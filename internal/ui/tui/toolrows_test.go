package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestToolRows_PairsCallsWithResultsAndDropsProgress(t *testing.T) {
	rows := toolRows([]ToolEvent{
		{Kind: ToolCall, Name: "bash", Args: `{"command":"a"}`},
		{Kind: ToolCall, Name: "bash", Args: `{"command":"b"}`},
		{Kind: ToolProgress, Iteration: 1, Tools: []string{"bash"}},
		{Kind: ToolExec, Name: "bash", Status: "ok", DurationMs: 400},
		{Kind: ToolExec, Name: "bash", Status: "error", Summary: "exit 1"},
		{Kind: ToolCall, Name: "read_file", Args: `{"path":"x"}`},
	})
	if len(rows) != 3 {
		t.Fatalf("want 3 rows (one per call, no progress row), got %d: %+v", len(rows), rows)
	}
	if !rows[0].done || !rows[0].ok || rows[0].durationMs != 400 {
		t.Errorf("first call should take the first result: %+v", rows[0])
	}
	if !rows[1].done || rows[1].ok || rows[1].summary != "exit 1" {
		t.Errorf("second call should take the error: %+v", rows[1])
	}
	if rows[2].done {
		t.Errorf("read_file has no result yet: %+v", rows[2])
	}
}

func TestToolRows_CollapseOlderCallsAndCtrlOExpands(t *testing.T) {
	m := newTestModel("")
	for i := range 12 {
		m.handleStreamEvent(StreamEvent{Kind: "tool_call", ToolName: "bash", ToolArgs: fmt.Sprintf(`{"command":"step-%02d"}`, i)})
		status := "ok"
		if i == 2 {
			status = "error"
		}
		m.handleStreamEvent(StreamEvent{Kind: "tool_exec", ToolName: "bash", Status: status})
		m.handleStreamEvent(StreamEvent{Kind: "progress", Iteration: i, Tools: []string{"bash"}})
	}
	conv := m.renderConversation()
	if !strings.Contains(conv, "7 earlier tool calls, 1 failed") {
		t.Errorf("expected a folded summary line:\n%s", conv)
	}
	if strings.Contains(conv, "step-06") || !strings.Contains(conv, "step-07") || !strings.Contains(conv, "step-11") {
		t.Errorf("collapsed view should show only the last 5 calls:\n%s", conv)
	}
	if strings.Contains(conv, "↻") {
		t.Error("progress lines should not be drawn")
	}
	if n := strings.Count(conv, "step-"); n != 5 {
		t.Errorf("want 5 call lines, got %d", n)
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	conv = m.renderConversation()
	if n := strings.Count(conv, "step-"); n != 12 || strings.Contains(conv, "earlier tool calls") {
		t.Errorf("ctrl+o should list all 12 calls, got %d:\n%s", n, conv)
	}
}
