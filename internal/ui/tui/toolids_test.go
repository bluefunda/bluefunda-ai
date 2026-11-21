package tui

import (
	"strings"
	"testing"
)

// Two calls to the same tool finishing out of order: each result lands on
// its own call because they're paired by ID, not by name.
func TestToolRows_PairByID(t *testing.T) {
	rows := toolRows([]ToolEvent{
		{Kind: ToolCall, ID: "a", Name: "bash", Args: `{"command":"slow"}`},
		{Kind: ToolCall, ID: "b", Name: "bash", Args: `{"command":"fast"}`},
		{Kind: ToolExec, ID: "b", Name: "bash", Status: "error", Summary: "boom"},
		{Kind: ToolNote, ID: "a", Name: "bash", Summary: "allowed"},
	})
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].done || rows[0].note != "allowed" {
		t.Errorf("call a = %+v, want running with the note", rows[0])
	}
	if !rows[1].done || rows[1].ok || rows[1].summary != "boom" {
		t.Errorf("call b = %+v, want the error result", rows[1])
	}
}

// Events without IDs (server-side tools, older producers) still pair by name.
func TestToolRows_NameFallback(t *testing.T) {
	rows := toolRows([]ToolEvent{
		{Kind: ToolCall, Name: "read_file"},
		{Kind: ToolExec, Name: "read_file", Status: "ok"},
		{Kind: ToolExec, Name: "web_search", Status: "ok"},
	})
	if len(rows) != 2 || !rows[0].done || !rows[1].done || rows[1].name != "web_search" {
		t.Errorf("rows = %+v", rows)
	}
	if rows[0].queued {
		t.Error("without start events no call should show as queued")
	}
}

// Calls from one round run one at a time: the ones not started yet are
// queued, the started one is running.
func TestToolRows_QueuedUntilStarted(t *testing.T) {
	events := []ToolEvent{
		{Kind: ToolCall, ID: "a", Name: "bash", Args: `{"command":"one"}`},
		{Kind: ToolCall, ID: "b", Name: "read_file", Args: `{"path":"x.go"}`},
		{Kind: ToolStart, ID: "a", Name: "bash"},
	}
	rows := toolRows(events)
	if rows[0].queued || !rows[1].queued {
		t.Fatalf("rows = %+v, want a running and b queued", rows)
	}
	out := renderToolRows(events, true, 80, DefaultTheme())
	if !strings.Contains(out, "○") || !strings.Contains(out, "queued") {
		t.Errorf("queued call not shown as queued:\n%s", out)
	}
	rows = toolRows(append(events, ToolEvent{Kind: ToolExec, ID: "a", Name: "bash", Status: "ok"},
		ToolEvent{Kind: ToolStart, ID: "b", Name: "read_file"}))
	if !rows[0].done || rows[1].queued {
		t.Errorf("rows = %+v, want a done and b running", rows)
	}
}

// A running subagent's latest activity shows under its task row and goes
// away once the task finishes.
func TestToolRows_SubagentActivity(t *testing.T) {
	events := []ToolEvent{
		{Kind: ToolCall, ID: "t1", Name: "task", Args: `{"prompt":"find foo"}`},
		{Kind: ToolStart, ID: "t1", Name: "task"},
		{Kind: ToolActivity, ID: "t1", Name: "task", Summary: "1 tool call · search_content foo"},
		{Kind: ToolActivity, ID: "t1", Name: "task", Summary: "2 tool calls · read_file a.go"},
	}
	out := renderToolRows(events, false, 80, DefaultTheme())
	if !strings.Contains(out, "2 tool calls · read_file a.go") || strings.Contains(out, "search_content") {
		t.Errorf("want only the latest activity:\n%s", out)
	}
	out = renderToolRows(append(events, ToolEvent{Kind: ToolExec, ID: "t1", Name: "task", Status: "ok"}), true, 80, DefaultTheme())
	if strings.Contains(out, "tool calls") {
		t.Errorf("activity should go once the task is done:\n%s", out)
	}
}
