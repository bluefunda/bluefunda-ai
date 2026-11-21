package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TodoItem is one entry of the session's task list (Claude Code's
// TodoWrite). The model replaces the whole list on every call.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending | in_progress | completed
	ActiveForm string `json:"activeForm,omitempty"`
}

var (
	todoMu   sync.Mutex
	todoList []TodoItem
)

// Todos returns the current task list.
func Todos() []TodoItem {
	todoMu.Lock()
	defer todoMu.Unlock()
	return append([]TodoItem(nil), todoList...)
}

// TodoWrite validates and stores a new task list, replacing the old one.
func TodoWrite(argumentsJSON string) (string, error) {
	var args struct {
		Todos []TodoItem `json:"todos"`
	}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return "", fmt.Errorf("parse todos: %w", err)
	}
	counts := map[string]int{}
	for i, t := range args.Todos {
		if strings.TrimSpace(t.Content) == "" {
			return "", fmt.Errorf("todo %d: content is required", i+1)
		}
		switch t.Status {
		case "pending", "in_progress", "completed":
		default:
			return "", fmt.Errorf("todo %d: status %q must be pending, in_progress or completed", i+1, t.Status)
		}
		counts[t.Status]++
	}
	todoMu.Lock()
	todoList = args.Todos
	todoMu.Unlock()
	msg := fmt.Sprintf("Todo list updated: %d completed, %d in progress, %d pending.",
		counts["completed"], counts["in_progress"], counts["pending"])
	if counts["in_progress"] > 1 {
		msg += " Keep only one task in progress at a time."
	}
	return msg, nil
}
