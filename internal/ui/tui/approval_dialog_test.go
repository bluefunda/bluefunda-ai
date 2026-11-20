package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// approvalModel returns a model with a pending approval for toolName/args.
func approvalModel(t *testing.T, toolName, args string) (Model, chan ApprovalDecision) {
	t.Helper()
	m := newTestModel("")
	m.streaming = true // approvals only arrive mid-turn
	m.streamCh = make(chan StreamEvent)
	m.streamStop = make(chan struct{})
	reply := make(chan ApprovalDecision, 1)
	m.handleStreamEvent(StreamEvent{Kind: "approval", ToolName: toolName, ToolArgs: args, ReplyCh: reply})
	if m.pendingApproval == nil {
		t.Fatal("approval event did not open the dialog")
	}
	return m, reply
}

func press(t *testing.T, m Model, keys ...tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		m, cmd = update(m, k)
	}
	return m, cmd
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func awaitDecision(t *testing.T, cmd tea.Cmd, reply chan ApprovalDecision) ApprovalDecision {
	t.Helper()
	if cmd != nil {
		go cmd()
	}
	select {
	case d := <-reply:
		return d
	case <-time.After(time.Second):
		t.Fatal("no ApprovalDecision sent")
		return ApprovalDecision{}
	}
}

func TestApprovalDialog_ArrowsAndEnter(t *testing.T) {
	m, reply := approvalModel(t, "bash", `{"command":"go test ./..."}`)
	m, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter})
	d := awaitDecision(t, cmd, reply)
	if !d.Approved || !d.AlwaysAllow {
		t.Errorf("option 2 should approve and always-allow, got %+v", d)
	}
	if m.pendingApproval != nil {
		t.Error("dialog should close after answering")
	}
}

func TestApprovalDialog_UpWrapsToLastOption(t *testing.T) {
	m, _ := approvalModel(t, "bash", `{"command":"ls"}`)
	m, _ = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.approvalIdx != choiceNoFeedback {
		t.Errorf("up from the first option should wrap to the last, got %d", m.approvalIdx)
	}
}

func TestApprovalDialog_DeclineWithFeedback(t *testing.T) {
	m, reply := approvalModel(t, "bash", `{"command":"rm -rf build"}`)
	m, _ = press(t, m, runes("3"))
	if !m.approvalFeedback {
		t.Fatal("option 3 should switch to feedback mode")
	}
	if !strings.Contains(m.View(), "Type what to do instead") {
		t.Error("feedback mode should explain what to do")
	}
	m, cmd := press(t, m, runes("use make clean instead"), tea.KeyMsg{Type: tea.KeyEnter})
	d := awaitDecision(t, cmd, reply)
	if d.Approved || d.Feedback != "use make clean instead" {
		t.Errorf("got %+v, want decline carrying the note", d)
	}
	if m.approvalFeedback || m.textarea.Value() != "" {
		t.Error("feedback mode and input should reset after sending")
	}
	if last := m.messages[len(m.messages)-1].Content; !strings.Contains(last, "declined: use make clean instead") {
		t.Errorf("transcript note = %q", last)
	}
}

func TestApprovalDialog_FeedbackEscDeclinesWithoutNote(t *testing.T) {
	m, reply := approvalModel(t, "bash", `{"command":"rm -rf build"}`)
	m, _ = press(t, m, runes("3"), runes("partial"))
	_, cmd := press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if d := awaitDecision(t, cmd, reply); d.Approved || d.Feedback != "" {
		t.Errorf("got %+v, want plain decline", d)
	}
}

func TestApprovalDialog_ShowsFullCommand(t *testing.T) {
	long := "go test ./internal/... -run TestSomethingVeryLong -count=1 -race && echo " + strings.Repeat("x", 80) + " END_MARKER"
	m, _ := approvalModel(t, "bash", `{"command":"`+long+`"}`)
	view := m.View()
	if !strings.Contains(view, "END_MARKER") {
		t.Error("the end of a long command must be visible, not truncated")
	}
	if strings.Contains(view, "Enter send") {
		t.Error("the input box hint should be replaced by the dialog")
	}
}

func TestApprovalDialog_EditShowsDiff(t *testing.T) {
	m, _ := approvalModel(t, "edit_file", `{"path":"main.go","old_string":"return nil","new_string":"return err"}`)
	view := m.renderApproval()
	for _, want := range []string{"main.go", "- return nil", "+ return err"} {
		if !strings.Contains(view, want) {
			t.Errorf("edit preview missing %q", want)
		}
	}
}

func TestApprovalDialog_WriteShowsLineCountAndCapsPreview(t *testing.T) {
	content := strings.Repeat("line\\n", 40)
	m, _ := approvalModel(t, "write_file", `{"path":"big.txt","content":"`+content+`"}`)
	view := m.renderApproval()
	if !strings.Contains(view, "(40 lines)") || !strings.Contains(view, "more lines") {
		t.Errorf("write preview should count lines and cap the preview:\n%s", view)
	}
}

// The view must fit the terminal even with a tall dialog open.
func TestApprovalDialog_FitsTerminal(t *testing.T) {
	m, _ := approvalModel(t, "patch_file", `{"path":"a.go","edits":[`+
		strings.TrimSuffix(strings.Repeat(`{"old_string":"a\nb\nc","new_string":"d\ne\nf"},`, 6), ",")+`]}`)
	if h := lipgloss.Height(m.View()); h > m.height {
		t.Errorf("view is %d lines tall in a %d-line terminal", h, m.height)
	}
}

func TestApprovalDialog_FeedbackModeFitsTerminal(t *testing.T) {
	m, _ := approvalModel(t, "write_file", `{"path":"a.txt","content":"`+strings.Repeat("x\\n", 50)+`"}`)
	m, _ = press(t, m, runes("3"))
	if h := lipgloss.Height(m.View()); h > m.height {
		t.Errorf("feedback view is %d lines tall in a %d-line terminal", h, m.height)
	}
}

func TestApprovalDialog_AlwaysOptionNamesItsScope(t *testing.T) {
	m := newTestModel("")
	m.streaming = true
	m.streamCh = make(chan StreamEvent)
	m.streamStop = make(chan struct{})
	m.handleStreamEvent(StreamEvent{Kind: "approval", ToolName: "bash", ToolArgs: `{"command":"gh pr create"}`,
		ReplyCh: make(chan ApprovalDecision, 1), AlwaysLabel: "`gh pr` commands"})
	if v := m.renderApproval(); !strings.Contains(v, "don't ask again for `gh pr` commands this session") {
		t.Errorf("option 2 should name its scope:\n%s", v)
	}
	m.pendingApproval.AlwaysLabel = ""
	if v := m.renderApproval(); !strings.Contains(v, "this exact call") {
		t.Errorf("without a scope, option 2 should say exact call:\n%s", v)
	}
}
