package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

// mentionRE finds @path mentions: an @ at the start or after whitespace.
var mentionRE = regexp.MustCompile(`(^|\s)@(\S+)`)

// maxDirEntries caps how much of a mentioned directory is listed.
const maxDirEntries = 200

// expandMentions attaches the files and directories the message mentions as
// @path, as Claude Code does: a file's contents (capped like tool output),
// a directory's listing. Mentions that aren't an existing path inside the
// workspace are left as plain text. The message itself is unchanged; the
// attachments follow it.
func expandMentions(msg string) string {
	var blocks []string
	seen := map[string]bool{}
	for _, m := range mentionRE.FindAllStringSubmatch(msg, -1) {
		path := resolveMention(m[2])
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if b := attachPath(path); b != "" {
			blocks = append(blocks, b)
		}
	}
	if len(blocks) == 0 {
		return msg
	}
	return msg + "\n\n" + strings.Join(blocks, "\n\n")
}

// resolveMention returns the mentioned path if it exists inside the
// workspace, trying it with trailing punctuation removed ("see @a.go,").
func resolveMention(raw string) string {
	for _, cand := range []string{raw, strings.TrimRight(raw, ".,;:!?)]}'\"")} {
		if cand == "" {
			continue
		}
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		if _, err := tools.ConfineToWorkspace(cand); err != nil {
			return ""
		}
		return cand
	}
	return ""
}

func attachPath(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return ""
		}
		var names []string
		for i, e := range entries {
			if i == maxDirEntries {
				names = append(names, fmt.Sprintf("… %d more", len(entries)-maxDirEntries))
				break
			}
			n := e.Name()
			if e.IsDir() {
				n += "/"
			}
			names = append(names, n)
		}
		return fenced("Contents of directory @"+path+":", strings.Join(names, "\n"))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "@" + path + " is a binary file (" + fmt.Sprint(len(data)) + " bytes); not attached."
	}
	return fenced("Contents of @"+path+":", tools.CapOutput(string(data), tools.MaxOutputChars))
}

// fenced puts body in a code fence longer than any backtick run inside it,
// so the body can't close the fence early.
func fenced(title, body string) string {
	fence := "```"
	for strings.Contains(body, fence) {
		fence += "`"
	}
	return title + "\n" + fence + "\n" + strings.TrimRight(body, "\n") + "\n" + fence
}

// maxMentionCandidates bounds the @ picker's list in very large trees.
const maxMentionCandidates = 20000

// mentionCandidates lists the files and directories the @ picker offers:
// git's tracked and untracked-but-not-ignored files when in a repository,
// otherwise a bounded walk that skips hidden and dependency directories.
func mentionCandidates() []string {
	var files []string
	out, err := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if f != "" {
				files = append(files, f)
			}
		}
	} else {
		_ = filepath.WalkDir(".", func(p string, d os.DirEntry, err error) error {
			if err != nil || len(files) >= maxMentionCandidates {
				return filepath.SkipDir
			}
			if d.IsDir() {
				if p != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "vendor") {
					return filepath.SkipDir
				}
				return nil
			}
			files = append(files, p)
			return nil
		})
	}
	if len(files) > maxMentionCandidates {
		files = files[:maxMentionCandidates]
	}
	dirs := map[string]bool{}
	for _, f := range files {
		for d := filepath.Dir(f); d != "." && d != "/" && !dirs[d]; d = filepath.Dir(d) {
			dirs[d] = true
		}
	}
	for d := range dirs {
		files = append(files, d+"/")
	}
	sort.Strings(files)
	return files
}
