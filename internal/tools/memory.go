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

// MemoryWrite creates or overwrites a project-scoped memory entry.
func MemoryWrite(key, content string) (string, error) {
	e, err := memoryManager().Write(key, content)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote memory %q (%d bytes) to %s", e.Key, len(content), e.Path), nil
}

// MemoryDelete removes a project-scoped memory entry.
func MemoryDelete(key string) (string, error) {
	if err := memoryManager().Delete(key); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted memory %q", key), nil
}
