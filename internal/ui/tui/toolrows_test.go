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

func TestToolRows_CollapseToSummaryAndCtrlOExpands(t *testing.T) {
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
	m.handleStreamEvent(StreamEvent{Kind: "tool_call", ToolName: "read_file", ToolArgs: `{"path":"main.go"}`})
	m.handleStreamEvent(StreamEvent{Kind: "tool_exec", ToolName: "read_file", Status: "ok"})
	m.handleStreamEvent(StreamEvent{Kind: "tool_call", ToolName: "bash", ToolArgs: `{"command":"RUNNING"}`})

	conv := m.renderLive()
	if !strings.Contains(conv, "Ran 12 commands, read 1 file · 1 failed") {
		t.Errorf("expected one summary line for finished calls:\n%s", conv)
	}
	if n := strings.Count(conv, "step-"); n != 0 {
		t.Errorf("finished calls should be folded, %d listed:\n%s", n, conv)
	}
	if !strings.Contains(conv, "RUNNING") {
		t.Errorf("the running call should stay listed:\n%s", conv)
	}
	if strings.Contains(conv, "↻") {
		t.Error("progress lines should not be drawn")
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	conv = m.renderLive()
	if n := strings.Count(conv, "step-"); n != 12 || strings.Contains(conv, "Ran 12") {
		t.Errorf("ctrl+o should list all 12 calls, got %d:\n%s", n, conv)
	}
}

func TestToolRows_FewCallsListedIndividually(t *testing.T) {
	m := newTestModel("")
	for _, c := range []string{"first", "second"} {
		m.handleStreamEvent(StreamEvent{Kind: "tool_call", ToolName: "bash", ToolArgs: `{"command":"` + c + `"}`})
		m.handleStreamEvent(StreamEvent{Kind: "tool_exec", ToolName: "bash", Status: "ok"})
	}
	if conv := m.renderLive(); !strings.Contains(conv, "first") || !strings.Contains(conv, "second") {
		t.Errorf("two calls should each get a line:\n%s", conv)
	}
}

func TestToolSummary_Phrasing(t *testing.T) {
	rows := []toolRow{
		{name: "bash"}, {name: "edit_file"}, {name: "write_file"},
		{name: "memory_list"}, {name: "memory_list"}, {name: "list_dir"},
	}
	want := "Ran 1 command, edited 2 files, called memory_list 2 times, listed 1 directory"
	if got := toolSummary(rows); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// An approval note must not split the response: before the fix, the note
// was its own message and the next tool events started a second
// "Assistant" block.
func TestApprovalNoteStaysInResponse(t *testing.T) {
	m := newTestModel("")
	m.streaming = true
	reply := make(chan ApprovalDecision, 1)
	m.handleStreamEvent(StreamEvent{Kind: "tool_call", ToolName: "bash", ToolArgs: `{"command":"git fetch"}`})
	m.handleStreamEvent(StreamEvent{Kind: "approval", ToolName: "bash", ToolArgs: `{"command":"git fetch"}`, ReplyCh: reply, AlwaysLabel: "`git fetch` commands"})
	nm, _ := m.answerApproval(ApprovalDecision{Approved: true, AlwaysAllow: true})
	m = nm.(Model)
	m.handleStreamEvent(StreamEvent{Kind: "tool_exec", ToolName: "bash", Status: "ok"})
	m.handleStreamEvent(StreamEvent{Kind: "chunk", Chunk: "done"})

	assistants := 0
	for _, msg := range m.messages {
		if msg.Role == RoleAssistant {
			assistants++
		}
		if msg.Role == RoleSystem && strings.Contains(msg.Content, "allowed") {
			t.Error("approval note should not be a separate message")
		}
	}
	if assistants != 1 {
		t.Errorf("want one response, got %d assistant messages", assistants)
	}
	if conv := m.renderLive(); !strings.Contains(conv, "won't ask again for `git fetch` commands") {
		t.Errorf("note should be shown under the call:\n%s", conv)
	}
}
