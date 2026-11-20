package tui

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// todoTool is the task-list tool; its calls render as a checklist instead
// of tool rows.
const todoTool = "todo_write"

type todoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// splitTodoRows separates task-list calls from the other tool rows and
// returns the latest list that was accepted (a call that failed validation
// doesn't replace it).
func splitTodoRows(rows []toolRow) (others []toolRow, todos []todoItem) {
	for _, r := range rows {
		if r.name != todoTool {
			others = append(others, r)
			continue
		}
		if r.done && !r.ok {
			others = append(others, r) // show the error like any failed call
			continue
		}
		var args struct {
			Todos []todoItem `json:"todos"`
		}
		if json.Unmarshal([]byte(r.args), &args) == nil {
			todos = args.Todos
		}
	}
	return others, todos
}

// renderTodos draws the task list as Claude Code does: done tasks struck
// through, the one in progress highlighted, the rest plain.
func renderTodos(todos []todoItem, width int, th Theme) string {
	if len(todos) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("  " + th.ToolName.Render("Todos") + "\n")
	room := max(width-8, 20)
	for i, t := range todos {
		prefix := "    "
		if i == 0 {
			prefix = "  ⎿ "
		}
		var line string
		switch t.Status {
		case "completed":
			line = th.ToolDim.Render("☒ ") + th.ToolDim.Strikethrough(true).Render(truncate(t.Content, room))
		case "in_progress":
			text := t.Content
			if t.ActiveForm != "" {
				text = t.ActiveForm
			}
			line = lipgloss.NewStyle().Bold(true).Render("☐ " + truncate(text, room))
		default:
			line = "☐ " + truncate(t.Content, room)
		}
		sb.WriteString(prefix + line + "\n")
	}
	return sb.String()
}
