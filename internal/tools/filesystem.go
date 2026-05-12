package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const bashTimeout = 30 * time.Second

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
		return ReadFile(path)
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
	case "bash":
		command, _ := args["command"].(string)
		return Bash(command)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// ReadFile returns the contents of a file.
func ReadFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(b), nil
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
