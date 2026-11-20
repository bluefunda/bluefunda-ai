package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The latest task list renders as a checklist in place of todo_write's tool
// rows; other calls still show (and fold) as usual.
func TestRenderTodos(t *testing.T) {
	first := `{"todos":[{"content":"Read code","status":"in_progress"},{"content":"Fix bug","status":"pending"}]}`
	latest := `{"todos":[{"content":"Read code","status":"completed"},{"content":"Fix bug","status":"in_progress","activeForm":"Fixing the bug"}]}`
	events := []ToolEvent{
		{Kind: ToolCall, ID: "1", Name: "todo_write", Args: first},
		{Kind: ToolExec, ID: "1", Name: "todo_write", Status: "ok"},
		{Kind: ToolCall, ID: "2", Name: "read_file", Args: `{"path":"a.go"}`},
		{Kind: ToolExec, ID: "2", Name: "read_file", Status: "ok"},
		{Kind: ToolCall, ID: "3", Name: "todo_write", Args: latest},
		{Kind: ToolExec, ID: "3", Name: "todo_write", Status: "ok"},
		{Kind: ToolCall, ID: "4", Name: "todo_write", Args: `{"todos":[{"content":"bad","status":"nope"}]}`},
		{Kind: ToolExec, ID: "4", Name: "todo_write", Status: "error", Summary: "status must be"},
	}
	out := ansi.Strip(renderToolRows(events, false, 80, DefaultTheme()))
	for _, want := range []string{"Todos", "☒ Read code", "☐ Fixing the bug", "read_file", "status must be"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "bad") && strings.Contains(out, "☐ bad") {
		t.Errorf("a rejected list must not be shown as the list:\n%s", out)
	}
	if strings.Count(out, "Todos") != 1 {
		t.Errorf("only the latest list should show:\n%s", out)
	}
}
