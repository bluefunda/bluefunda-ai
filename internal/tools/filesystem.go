package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const webTimeout = 30 * time.Second

const bashTimeout = 120 * time.Second

// PatchEdit is one old→new replacement within a patch_file call.
type PatchEdit struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

// Execute dispatches a tool call to the appropriate local implementation.
// Arguments is the JSON string from the LLM tool call.
func Execute(name, argumentsJSON string) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	switch name {
	case "read_file":
		path, _ := args["path"].(string)
		offset, _ := args["offset"].(float64)
		limit, _ := args["limit"].(float64)
		return ReadFile(path, int(offset), int(limit))
	case "edit_file":
		path, _ := args["path"].(string)
		oldStr, _ := args["old_string"].(string)
		newStr, _ := args["new_string"].(string)
		replaceAll, _ := args["replace_all"].(bool)
		return EditFile(path, oldStr, newStr, replaceAll)
	case "patch_file":
		var patchArgs struct {
			Path  string      `json:"path"`
			Edits []PatchEdit `json:"edits"`
		}
		if err := json.Unmarshal([]byte(argumentsJSON), &patchArgs); err != nil {
			return "", fmt.Errorf("parse patch_file arguments: %w", err)
		}
		return PatchFile(patchArgs.Path, patchArgs.Edits)
	case "write_file":
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		return WriteFile(path, content)
	case "list_dir":
		path, _ := args["path"].(string)
		return ListDir(path)
	case "search_files":
		dir, _ := args["dir"].(string)
		pattern, _ := args["pattern"].(string)
		return SearchFiles(dir, pattern)
	case "search_content":
		pattern, _ := args["pattern"].(string)
		dir, _ := args["directory"].(string)
		glob, _ := args["glob"].(string)
		return SearchContent(pattern, dir, glob)
	case "web_fetch":
		rawURL, _ := args["url"].(string)
		return WebFetch(rawURL)
	case "web_search":
		query, _ := args["query"].(string)
		return WebSearch(query)
	case "bash":
		command, _ := args["command"].(string)
		return Bash(command)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// ReadFile returns the contents of a file, optionally starting at line offset
// and reading at most limit lines. offset=0, limit=0 reads the whole file.
// Lines are returned prefixed with their 1-based line number.
func ReadFile(path string, offset, limit int) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if offset <= 0 && limit <= 0 {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		return string(b), nil
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	lineNum := 0
	written := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= offset {
			continue
		}
		fmt.Fprintf(&sb, "%4d\t%s\n", lineNum, scanner.Text())
		written++
		if limit > 0 && written >= limit {
			break
		}
	}
	return sb.String(), nil
}

// EditFile replaces the first (or all) occurrence(s) of oldStr with newStr in
// the file at path. Returns an error if oldStr does not appear exactly once
// when replaceAll is false.
func EditFile(path, oldStr, newStr string, replaceAll bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if oldStr == "" {
		return "", fmt.Errorf("old_string is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	content := string(b)
	count := strings.Count(content, oldStr)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", path)
	}
	if !replaceAll && count > 1 {
		return "", fmt.Errorf("old_string matches %d occurrences in %s; add more surrounding context to make it unique", count, path)
	}

	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		updated = strings.Replace(content, oldStr, newStr, 1)
	}

	// Atomic write: temp file in same directory, then rename.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bai-edit-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(updated); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("rename to %s: %w", path, err)
	}

	n := 1
	if replaceAll {
		n = count
	}
	return fmt.Sprintf("edited %d occurrence(s) in %s", n, path), nil
}

// PatchFile applies multiple old→new string replacements to a file atomically.
// All edits are validated before any change is written: if any old_string is
// absent or non-unique (and replace_all is false), the whole call fails.
// Returns a summary line and a unified diff of the changes.
func PatchFile(path string, edits []PatchEdit) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if len(edits) == 0 {
		return "", fmt.Errorf("edits must contain at least one entry")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	original := string(b)

	// Validate every edit before touching the file.
	for i, e := range edits {
		if e.OldString == "" {
			return "", fmt.Errorf("edits[%d]: old_string is required", i)
		}
		count := strings.Count(original, e.OldString)
		if count == 0 {
			return "", fmt.Errorf("edits[%d]: old_string not found in %s", i, path)
		}
		if !e.ReplaceAll && count > 1 {
			return "", fmt.Errorf("edits[%d]: old_string matches %d occurrences in %s; add surrounding context to make it unique, or set replace_all: true", i, count, path)
		}
	}

	// Apply edits in order.
	content := original
	totalReplaced := 0
	for _, e := range edits {
		if e.ReplaceAll {
			n := strings.Count(content, e.OldString)
			content = strings.ReplaceAll(content, e.OldString, e.NewString)
			totalReplaced += n
		} else {
			content = strings.Replace(content, e.OldString, e.NewString, 1)
			totalReplaced++
		}
	}

	if content == original {
		return fmt.Sprintf("no changes made to %s (all replacements were no-ops)", path), nil
	}

	// Atomic write.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bai-patch-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("rename to %s: %w", path, err)
	}

	diff := unifiedDiff(path, original, content)
	return fmt.Sprintf("patched %s (%d replacement(s))\n\n%s", path, totalReplaced, diff), nil
}

// unifiedDiff returns a unified diff of original→updated content using the
// system diff(1) command. Returns an empty string if diff is unavailable.
func unifiedDiff(path, original, updated string) string {
	origTmp, err := os.CreateTemp("", ".bai-orig-*")
	if err != nil {
		return ""
	}
	defer func() { _ = os.Remove(origTmp.Name()) }()
	if _, err := origTmp.WriteString(original); err != nil || origTmp.Close() != nil {
		return ""
	}

	newTmp, err := os.CreateTemp("", ".bai-new-*")
	if err != nil {
		return ""
	}
	defer func() { _ = os.Remove(newTmp.Name()) }()
	if _, err := newTmp.WriteString(updated); err != nil || newTmp.Close() != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "diff", "-u",
		"--label", "a/"+path,
		"--label", "b/"+path,
		origTmp.Name(), newTmp.Name())
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run() // diff exits 1 when there are differences — expected
	return out.String()
}

// WriteFile writes content to a file, creating parent directories as needed.
func WriteFile(path, content string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("create dirs for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
}

// ListDir lists the immediate contents of a directory.
func ListDir(path string) (string, error) {
	if path == "" {
		path = "."
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("list %s: %w", path, err)
	}
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			sb.WriteString(e.Name() + "/\n")
		} else {
			sb.WriteString(e.Name() + "\n")
		}
	}
	return sb.String(), nil
}

// SearchFiles walks dir and returns paths matching pattern (supports ** globs).
func SearchFiles(dir, pattern string) (string, error) {
	if dir == "" {
		dir = "."
	}
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	// Normalise: strip leading **/ so filepath.Match can compare base names too.
	basePattern := strings.TrimPrefix(pattern, "**/")

	var matches []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		name := filepath.Base(path)
		ok, _ := filepath.Match(basePattern, name)
		if ok {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search %s: %w", dir, err)
	}
	if len(matches) == 0 {
		return "no files found", nil
	}
	return strings.Join(matches, "\n"), nil
}

// Bash runs a shell command with a timeout and returns combined stdout+stderr.
func Bash(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		// Return output even on non-zero exit so the LLM can see the error.
		output := out.String()
		if output == "" {
			output = err.Error()
		}
		return output, nil
	}
	return out.String(), nil
}

const searchContentMaxResults = 200

// SearchContent searches file contents for a regex pattern.
// Prefers ripgrep (rg) when available; falls back to pure-Go line scanning.
// Returns matching lines as "filepath:linenum: content", capped at 200 results.
func SearchContent(pattern, dir, glob string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if dir == "" {
		dir = "."
	}

	// Fast path: delegate to ripgrep when available.
	if rgPath, err := exec.LookPath("rg"); err == nil {
		return searchContentRg(rgPath, pattern, dir, glob)
	}

	return searchContentGo(pattern, dir, glob)
}

func searchContentRg(rgPath, pattern, dir, glob string) (string, error) {
	args := []string{"--line-number", "--no-heading", "--max-count", "1",
		"--max-filesize", "10M", pattern, dir}
	if glob != "" {
		args = append([]string{"--glob", glob}, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, rgPath, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // rg exits 1 when no matches; that's fine

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return "no matches found", nil
	}
	if len(lines) > searchContentMaxResults {
		lines = lines[:searchContentMaxResults]
		lines = append(lines, fmt.Sprintf("... (truncated at %d results)", searchContentMaxResults))
	}
	return strings.Join(lines, "\n"), nil
}

func searchContentGo(pattern, dir, glob string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	var results []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if glob != "" {
			matched, _ := filepath.Match(glob, filepath.Base(path))
			if !matched {
				return nil
			}
		}
		if info.Size() > 10*1024*1024 { // skip files > 10 MB
			return nil
		}
		if len(results) >= searchContentMaxResults {
			return filepath.SkipAll
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()

		lineNum := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", path, lineNum, line))
				if len(results) >= searchContentMaxResults {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("search %s: %w", dir, err)
	}
	if len(results) == 0 {
		return "no matches found", nil
	}
	if len(results) >= searchContentMaxResults {
		results = append(results, fmt.Sprintf("... (truncated at %d results)", searchContentMaxResults))
	}
	return strings.Join(results, "\n"), nil
}

// WebFetch fetches a URL and returns its content as plain text. HTML tags are
// stripped to reduce token usage. Content is capped at 50 000 characters.
func WebFetch(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), webTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "bai/1.0 (https://bluefunda.com)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 512 KB cap
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	text := stripHTML(string(body))
	if len(text) > 50_000 {
		text = text[:50_000] + "\n\n... (truncated at 50 000 chars)"
	}
	return fmt.Sprintf("URL: %s\nStatus: %d\n\n%s", rawURL, resp.StatusCode, text), nil
}

// stripHTML removes HTML tags and normalises whitespace.
func stripHTML(s string) string {
	// Remove script and style blocks.
	for _, tag := range []string{"script", "style", "head"} {
		open := "<" + tag
		close := "</" + tag + ">"
		for {
			start := strings.Index(strings.ToLower(s), open)
			if start < 0 {
				break
			}
			end := strings.Index(strings.ToLower(s[start:]), close)
			if end < 0 {
				s = s[:start]
				break
			}
			s = s[:start] + s[start+end+len(close):]
		}
	}
	// Strip remaining tags.
	inTag := false
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			out.WriteByte(' ')
		case !inTag:
			out.WriteRune(r)
		}
	}
	// Collapse whitespace.
	lines := strings.Split(out.String(), "\n")
	var cleaned []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			cleaned = append(cleaned, l)
		}
	}
	return strings.Join(cleaned, "\n")
}

// WebSearch searches the web using DuckDuckGo's Lite JSON API (no API key
// required) and returns the top results as title + URL + snippet.
func WebSearch(query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	apiURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) +
		"&format=json&no_html=1&skip_disambig=1"

	ctx, cancel := context.WithTimeout(context.Background(), webTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "bai/1.0 (https://bluefunda.com)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var ddg struct {
		AbstractText   string `json:"AbstractText"`
		AbstractURL    string `json:"AbstractURL"`
		AbstractSource string `json:"AbstractSource"`
		RelatedTopics  []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
			Topics   []struct {
				Text     string `json:"Text"`
				FirstURL string `json:"FirstURL"`
			} `json:"Topics"`
		} `json:"RelatedTopics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ddg); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Search: %s\n\n", query)

	if ddg.AbstractText != "" {
		fmt.Fprintf(&sb, "## %s\n%s\n%s\n\n", ddg.AbstractSource, ddg.AbstractText, ddg.AbstractURL)
	}

	count := 0
	for _, t := range ddg.RelatedTopics {
		if count >= 8 {
			break
		}
		if t.Text != "" && t.FirstURL != "" {
			fmt.Fprintf(&sb, "- %s\n  %s\n", t.Text, t.FirstURL)
			count++
		}
		for _, sub := range t.Topics {
			if count >= 8 {
				break
			}
			if sub.Text != "" && sub.FirstURL != "" {
				fmt.Fprintf(&sb, "- %s\n  %s\n", sub.Text, sub.FirstURL)
				count++
			}
		}
	}

	if sb.Len() == len(fmt.Sprintf("Search: %s\n\n", query)) {
		return fmt.Sprintf("No results found for: %s\nTry web_fetch with a specific URL instead.", query), nil
	}
	return sb.String(), nil
}
