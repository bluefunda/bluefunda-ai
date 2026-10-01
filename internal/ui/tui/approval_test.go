package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// sendApprovalKey sets up a pending approval and simulates pressing key,
// returning the ApprovalDecision sent on the reply channel (or failing the
// test if none arrives within a second). The returned tea.Cmd blocks forever
// past the reply send (resuming the stream pump), so it's run in a detached
// goroutine — fine for a short-lived test process.
func sendApprovalKey(t *testing.T, key string) ApprovalDecision {
	t.Helper()
	m := newTestModel("")
	m.vpReady = true
	m.streamCh = make(chan StreamEvent)
	m.streamStop = make(chan struct{})
	replyCh := make(chan ApprovalDecision, 1)
	m.pendingApproval = &ApprovalRequestMsg{
		ToolName: "bash",
		Args:     `{"command":"git status"}`,
		ReplyCh:  replyCh,
	}

	var keyMsg tea.KeyMsg
	switch key {
	case "esc":
		keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		keyMsg = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}

	_, cmd := update(m, keyMsg)
	if cmd != nil {
		go cmd()
	}

	select {
	case decision := <-replyCh:
		return decision
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for an ApprovalDecision on the reply channel")
		return ApprovalDecision{}
	}
}

func TestHandleApprovalKey_Yes(t *testing.T) {
	got := sendApprovalKey(t, "y")
	if !got.Approved || got.AlwaysAllow {
		t.Errorf("got %+v, want {Approved:true AlwaysAllow:false}", got)
	}
}

func TestHandleApprovalKey_AlwaysAllow(t *testing.T) {
	got := sendApprovalKey(t, "a")
	if !got.Approved || !got.AlwaysAllow {
		t.Errorf("got %+v, want {Approved:true AlwaysAllow:true}", got)
	}
}

func TestHandleApprovalKey_No(t *testing.T) {
	got := sendApprovalKey(t, "n")
	if got.Approved || got.AlwaysAllow {
		t.Errorf("got %+v, want {Approved:false AlwaysAllow:false}", got)
	}
}

func TestHandleApprovalKey_Esc(t *testing.T) {
	got := sendApprovalKey(t, "esc")
	if got.Approved {
		t.Errorf("got %+v, want Approved:false", got)
	}
}

func TestHandleApprovalKey_CtrlC(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	replyCh := make(chan ApprovalDecision, 1)
	m.pendingApproval = &ApprovalRequestMsg{ToolName: "bash", Args: "{}", ReplyCh: replyCh}

	nm, _ := update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !nm.quit {
		t.Error("expected ctrl+c to set quit")
	}
	select {
	case decision := <-replyCh:
		if decision.Approved {
			t.Errorf("got %+v, want Approved:false", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the decline decision on ctrl+c")
	}
}
