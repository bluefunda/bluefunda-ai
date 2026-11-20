package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSplitNarration(t *testing.T) {
	msg := newAssistantMessage()
	msg.appendChunk("Let's run the tests.\n")
	msg.addToolEvent(ToolEvent{Kind: ToolCall, Name: "bash"})
	msg.addToolEvent(ToolEvent{Kind: ToolExec, Name: "bash", Status: "ok"})
	msg.appendChunk("Good. Now verify.\n")
	msg.addToolEvent(ToolEvent{Kind: ToolCall, Name: "read_file"})
	msg.appendChunk("All tests pass.")

	narration, answer := msg.splitNarration()
	if narration != "Let's run the tests.\nGood. Now verify.\n" {
		t.Errorf("narration = %q", narration)
	}
	if answer != "All tests pass." {
		t.Errorf("answer = %q", answer)
	}
	if got := narrationSummary(narration); !strings.Contains(got, "2 lines") || !strings.Contains(got, "ctrl+o") {
		t.Errorf("summary = %q", got)
	}
}

func TestSplitNarration_NoToolCalls(t *testing.T) {
	msg := newAssistantMessage()
	msg.appendChunk("Just an answer.")
	narration, answer := msg.splitNarration()
	if narration != "" || answer != "Just an answer." {
		t.Errorf("got narration=%q answer=%q", narration, answer)
	}
}

func TestSplitNarration_ToolCallBeforeAnyText(t *testing.T) {
	msg := newAssistantMessage()
	msg.addToolEvent(ToolEvent{Kind: ToolCall, Name: "bash"})
	msg.appendChunk("Done.")
	if narration, _ := msg.splitNarration(); narration != "" {
		t.Errorf("whitespace-free empty prefix must not count as narration, got %q", narration)
	}
}

func TestNarrationCollapsedAndCtrlOToggles(t *testing.T) {
	m := newTestModel("")
	for _, ev := range []StreamEvent{
		{Kind: "chunk", Chunk: "NARRATIONMARKER let's look around.\n"},
		{Kind: "tool_call", ToolName: "bash", ToolArgs: "{}"},
		{Kind: "chunk", Chunk: "ANSWERMARKER"},
	} {
		m.handleStreamEvent(ev)
	}

	conv := m.renderLive()
	if strings.Contains(conv, "NARRATIONMARKER") {
		t.Error("narration should be collapsed by default")
	}
	if !strings.Contains(conv, "narration hidden") {
		t.Error("collapsed indicator missing")
	}
	if !strings.Contains(conv, "ANSWERMARKER") {
		t.Error("answer after the last tool call must always be shown")
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	conv = m.renderLive()
	if !strings.Contains(conv, "NARRATIONMARKER") || !strings.Contains(conv, "ANSWERMARKER") {
		t.Error("ctrl+o should expand narration")
	}
	if strings.Contains(conv, "narration hidden") {
		t.Error("indicator should disappear when expanded")
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if strings.Contains(m.renderLive(), "NARRATIONMARKER") {
		t.Error("second ctrl+o should collapse again")
	}
}
