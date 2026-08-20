package tools

import (
	"fmt"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/memory"
)

// memoryManager builds a Manager rooted at the current working directory.
// Tool calls, like all other filesystem tools in this package, operate
// relative to the process cwd (which reflects --dir / worktree chdir).
func memoryManager() *memory.Manager {
	return memory.New(".")
}

// MemoryRead returns the full content of a memory entry by key.
func MemoryRead(key string) (string, error) {
	e, err := memoryManager().Read(key)
	if err != nil {
		return "", err
	}
	return e.Content, nil
}

// MemoryList returns a one-line preview of every memory entry across both
// scopes, formatted for a tool result.
func MemoryList() (string, error) {
	entries, err := memoryManager().List()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "no memory entries", nil
	}
	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "%s [%s]: %s\n", e.Key, e.Scope, e.Preview())
	}
	return sb.String(), nil
}

// MemoryWrite creates or overwrites a project-scoped memory entry. When
// supersedes is non-empty, that key's entry is marked superseded after the
// write succeeds — it stays on disk but drops out of recall.
func MemoryWrite(key, content, supersedes string) (string, error) {
	mgr := memoryManager()
	e, err := mgr.Write(key, content)
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("wrote memory %q (%d bytes) to %s", e.Key, len(content), e.Path)
	if supersedes != "" {
		if err := mgr.Supersede(supersedes); err != nil {
			return "", fmt.Errorf("wrote %q but failed to mark %q superseded: %w", key, supersedes, err)
		}
		msg += fmt.Sprintf("; marked %q superseded", supersedes)
	}
	return msg, nil
}

// MemoryDelete removes a project-scoped memory entry.
func MemoryDelete(key string) (string, error) {
	if err := memoryManager().Delete(key); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted memory %q", key), nil
}
