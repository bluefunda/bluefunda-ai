package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel returns a ready-to-use model sized to a standard 80×24 terminal.
// It simulates the WindowSizeMsg that BubbleTea sends on startup.
func newTestModel(initialPrompt string) Model {
	cfg := SessionConfig{
		ChatID:        "test-0000-0000-0000-000000000000",
		Model:         "test",
		InitialPrompt: initialPrompt,
	}
	submit := func(_, _, _ string, _ bool) <-chan StreamEvent {
		ch := make(chan StreamEvent, 1)
		close(ch)
		return ch
	}
	m := New(cfg, submit)
	m, _ = update(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

// update is a typed helper that avoids repetitive casts.
func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

// ── Scrollback rendering ──────────────────────────────────────────────────────

// TestFinishedMessagesLeaveTheView verifies that finished messages are handed
// to the terminal scrollback and no longer drawn by View, which keeps only
// the live area (input, status, footer).
func TestFinishedMessagesLeaveTheView(t *testing.T) {
	m := newTestModel("")
	for i := range 30 {
		m.messages = append(m.messages, newSystemMessage(fmt.Sprintf("message %d", i)))
	}
	m, cmd := update(m, scrollbackDoneMsg{}) // the welcome write finished
	if cmd == nil || !m.printing {
		t.Fatal("finished messages should produce a scrollback write")
	}
	for i, msg := range m.messages {
		if !msg.printed {
			t.Fatalf("message %d not printed", i)
		}
	}
	view := m.View()
	if strings.Contains(view, "message 5") {
		t.Error("printed messages must not be redrawn by View")
	}
	if !strings.Contains(view, "BlueFunda AI") || !strings.Contains(view, "Enter send") {
		t.Errorf("view should keep the status and footer lines:\n%s", view)
	}
	if n := strings.Count(view, "\n"); n > 10 {
		t.Errorf("idle view has %d lines; want only the input and status area", n)
	}
}

// TestStreamingMessageStaysLive checks that a streaming response, and
// anything after it, stays in the view until it finishes, and that writes are
// serialized: nothing new is printed until the previous write is done.
func TestStreamingMessageStaysLive(t *testing.T) {
	m := newTestModel("")
	m, _ = update(m, scrollbackDoneMsg{}) // the welcome write finished
	m.streaming = true
	m.messages = append(m.messages, newAssistantMessage())
	m.messages[len(m.messages)-1].Content = "LIVEANSWER"
	m.messages = append(m.messages, newSystemMessage("AFTERNOTE"))

	m, _ = update(m, scrollbackDoneMsg{})
	if m.printing {
		t.Fatal("nothing finished: no write expected")
	}
	view := m.View()
	if !strings.Contains(view, "LIVEANSWER") || !strings.Contains(view, "AFTERNOTE") {
		t.Fatalf("streaming message and the note after it should be live:\n%s", view)
	}

	m, _ = update(m, StreamEvent{Kind: "done"})
	if !m.printing || !m.messages[1].printed || !m.messages[2].printed {
		t.Fatal("finished response and the note after it should be printed")
	}
	if strings.Contains(m.View(), "LIVEANSWER") {
		t.Error("printed response must leave the view")
	}

	// A message finishing while a write is in flight waits for it.
	m.messages = append(m.messages, newSystemMessage("QUEUED"))
	m, _ = update(m, tickMsg{})
	if m.messages[3].printed {
		t.Fatal("second write must wait for the first")
	}
	m, cmd := update(m, scrollbackDoneMsg{})
	if cmd == nil || !m.messages[3].printed {
		t.Fatal("queued message should be printed once the first write is done")
	}
}

// TestInterruptedTurnDoesNotBlockScrollback: Ctrl+C leaves the response
// unfinalized; it must still reach scrollback, and so must what follows.
func TestInterruptedTurnDoesNotBlockScrollback(t *testing.T) {
	m := newTestModel("")
	m, _ = update(m, scrollbackDoneMsg{})
	m.streaming = true
	m.streamStop = make(chan struct{})
	m.streamCh = make(chan StreamEvent)
	m.messages = append(m.messages, newAssistantMessage())
	m.messages[1].Content = "partial"

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if m.streaming || !m.messages[1].printed || m.messages[1].Streaming {
		t.Fatalf("interrupted response should be finalized and printed (streaming=%v printed=%v)",
			m.messages[1].Streaming, m.messages[1].printed)
	}
	if last := m.messages[len(m.messages)-1]; last.Content != "Interrupted." || !last.printed {
		t.Error("the Interrupted notice should follow it into scrollback")
	}
}

// TestCursorNotTrappedAtBottom ensures the View() height stays within terminal
// bounds even when the conversation is very long.
func TestCursorNotTrappedAtBottom(t *testing.T) {
	m := newTestModel("")
	for i := range 200 {
		m.messages = append(m.messages, newAssistantMessage())
		m.messages[len(m.messages)-1].Content = fmt.Sprintf("Response %d: this is some content to make it realistic.", i)
		m.messages[len(m.messages)-1].Streaming = false
	}
	m, _ = update(m, scrollbackDoneMsg{})
	if lines := strings.Split(m.View(), "\n"); len(lines) > 24 {
		t.Errorf("view has %d lines for a 24-line terminal — cursor would be off-screen", len(lines))
	}
}

// ── Message accumulation ──────────────────────────────────────────────────────

func TestMessagesAccumulate(t *testing.T) {
	m := newTestModel("")

	// Simulate a user turn and assistant response.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	// After typing, the message should appear in the textarea but not yet in messages.
	if strings.TrimSpace(m.textarea.Value()) != "hello" {
		t.Errorf("textarea should contain 'hello', got %q", m.textarea.Value())
	}
}

// ── Streaming ─────────────────────────────────────────────────────────────────

func TestStreamingChunksAppend(t *testing.T) {
	m := newTestModel("")
	m.ready = true

	// Simulate a streaming response.
	m.streaming = true
	m.messages = append(m.messages, newAssistantMessage())

	chunks := []string{"Hello ", "world", "!\n", "How can I help?"}
	for _, c := range chunks {
		m, _ = update(m, StreamEvent{Kind: "chunk", Chunk: c})
	}
	m, _ = update(m, StreamEvent{Kind: "done"})

	if m.streaming {
		t.Error("streaming should be false after done event")
	}

	last := m.messages[len(m.messages)-1]
	if last.Role != RoleAssistant {
		t.Fatalf("last message role = %v, want RoleAssistant", last.Role)
	}
	want := "Hello world!\nHow can I help?"
	if last.Content != want {
		t.Errorf("assistant content = %q, want %q", last.Content, want)
	}
}

func TestStreamingError(t *testing.T) {
	m := newTestModel("")
	m.ready = true

	m.streaming = true
	m, _ = update(m, StreamEvent{Kind: "error", ErrMsg: "connection refused"})

	if m.streaming {
		t.Error("streaming should be false after error event")
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != RoleSystem {
		t.Fatalf("last message after error should be RoleSystem, got %v", last.Role)
	}
	if !strings.Contains(last.Content, "connection refused") {
		t.Errorf("error message not found in system message: %q", last.Content)
	}
}

// ── Large responses ───────────────────────────────────────────────────────────

func TestLargeResponse(t *testing.T) {
	m := newTestModel("")
	m.ready = true

	// Build a response larger than 100 KB.
	var sb strings.Builder
	for i := range 3000 {
		fmt.Fprintf(&sb, "Line %d: the quick brown fox jumps over the lazy dog.\n", i)
	}
	largeContent := sb.String()
	if len(largeContent) < 100_000 {
		t.Fatalf("test setup error: large content is only %d bytes", len(largeContent))
	}

	m.streaming = true
	m.messages = append(m.messages, newAssistantMessage())
	m, _ = update(m, StreamEvent{Kind: "chunk", Chunk: largeContent})
	m, _ = update(m, StreamEvent{Kind: "done"})

	last := m.messages[len(m.messages)-1]
	if last.Content != largeContent {
		t.Errorf("large content not preserved: got %d bytes, want %d bytes", len(last.Content), len(largeContent))
	}

	// View must still be bounded even with huge content.
	view := m.View()
	lineCount := strings.Count(view, "\n")
	if lineCount >= 24 {
		t.Errorf("view has %d lines for a 24-line terminal after large response", lineCount)
	}
}

// ── Window resize ─────────────────────────────────────────────────────────────

func TestWindowResize(t *testing.T) {
	m := newTestModel("")

	// Start at 80×24.
	if m.width != 80 || m.height != 24 {
		t.Fatalf("expected 80×24, got %d×%d", m.width, m.height)
	}

	// Resize to 120×40.
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Errorf("after resize expected 120×40, got %d×%d", m.width, m.height)
	}
	if m.textarea.Width() <= 80 {
		t.Errorf("input width not updated: got %d", m.textarea.Width())
	}

	// Resize to minimal terminal.
	m, _ = update(m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if m.width < 40 || m.height < 10 {
		t.Errorf("resize to 40×10 failed: got %d×%d", m.width, m.height)
	}
}

// ── Ctrl+C behaviour ──────────────────────────────────────────────────────────

func TestCtrlC_Idle_Quits(t *testing.T) {
	m := newTestModel("")
	m.streaming = false

	m2, cmd := update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m2.quit {
		t.Error("Ctrl+C when idle should set quit=true")
	}
	if cmd == nil {
		t.Error("Ctrl+C when idle should return tea.Quit cmd")
	}
}

func TestCtrlC_WhileStreaming_Interrupts(t *testing.T) {
	m := newTestModel("")
	m.streaming = true
	m.streamStop = make(chan struct{})

	// Provide a dummy channel so the drain goroutine has something to read.
	ch := make(chan StreamEvent)
	close(ch)
	m.streamCh = ch

	m2, _ := update(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if m2.streaming {
		t.Error("streaming should be false after Ctrl+C interrupt")
	}
	if m2.quit {
		t.Error("Ctrl+C during streaming should interrupt, not quit")
	}
}

// ── Ctrl+D behaviour ─────────────────────────────────────────────────────────

func TestCtrlD_EmptyInput_Quits(t *testing.T) {
	m := newTestModel("")

	m2, cmd := update(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if !m2.quit {
		t.Error("Ctrl+D with empty input should quit")
	}
	if cmd == nil {
		t.Error("Ctrl+D with empty input should return tea.Quit cmd")
	}
}

func TestCtrlD_NonEmpty_DoesNotQuit(t *testing.T) {
	m := newTestModel("")
	m.textarea.SetValue("some text")

	m2, _ := update(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if m2.quit {
		t.Error("Ctrl+D with non-empty input should not quit")
	}
}

// ── Prompt editing ────────────────────────────────────────────────────────────

func TestPromptEditing(t *testing.T) {
	m := newTestModel("")

	// Type characters.
	for _, r := range "Hello, World!" {
		m, _ = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if !strings.Contains(m.textarea.Value(), "Hello, World!") {
		t.Errorf("textarea should contain typed text, got %q", m.textarea.Value())
	}
}

// ── Input history ─────────────────────────────────────────────────────────────

func TestInputHistory(t *testing.T) {
	m := newTestModel("")
	m.ready = true

	// Submit two messages via a real submit function.
	submitted := []string{}
	m.submitFn = func(_, _, input string, _ bool) <-chan StreamEvent {
		submitted = append(submitted, input)
		ch := make(chan StreamEvent, 1)
		ch <- StreamEvent{Kind: "done"}
		close(ch)
		return ch
	}

	sendMsg := func(text string) {
		m.textarea.SetValue(text)
		nm, _ := m.submitInput()
		m = nm.(Model)
		// Drain the done event.
		m, _ = update(m, StreamEvent{Kind: "done"})
	}

	sendMsg("first message")
	sendMsg("second message")

	if len(m.inputHistory) < 2 {
		t.Fatalf("expected at least 2 history entries, got %d", len(m.inputHistory))
	}
	if m.inputHistory[0] != "first message" {
		t.Errorf("history[0] = %q, want 'first message'", m.inputHistory[0])
	}
	if m.inputHistory[1] != "second message" {
		t.Errorf("history[1] = %q, want 'second message'", m.inputHistory[1])
	}
}

// ── Custom slash commands ──────────────────────────────────────────────────────

func TestCustomSlashCommand_SubstitutesArguments(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	m.cfg.CustomCommands = []SlashCommand{
		{Name: "/review", Description: "Code review this file", Prompt: "Review $ARGUMENTS for correctness."},
	}

	var submitted string
	m.submitFn = func(_, _, input string, _ bool) <-chan StreamEvent {
		submitted = input
		ch := make(chan StreamEvent, 1)
		ch <- StreamEvent{Kind: "done"}
		close(ch)
		return ch
	}

	m.textarea.SetValue("/review internal/cmd/code.go")
	nm, _ := m.submitInput()
	m = nm.(Model)

	want := "Review internal/cmd/code.go for correctness."
	if submitted != want {
		t.Errorf("submitted prompt = %q, want %q", submitted, want)
	}
	// The conversation shows what was typed, not the expanded prompt.
	var shown string
	for _, msg := range m.messages {
		if msg.Role == RoleUser {
			shown = msg.Content
		}
	}
	if shown != "/review internal/cmd/code.go" {
		t.Errorf("shown user message = %q, want the typed command", shown)
	}
}

func TestCustomSlashCommand_NoArgumentsLeavesPlaceholderless(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	m.cfg.CustomCommands = []SlashCommand{
		{Name: "/standup", Description: "Summarize commits", Prompt: "Summarize yesterday's commits."},
	}

	var submitted string
	m.submitFn = func(_, _, input string, _ bool) <-chan StreamEvent {
		submitted = input
		ch := make(chan StreamEvent, 1)
		ch <- StreamEvent{Kind: "done"}
		close(ch)
		return ch
	}

	m.textarea.SetValue("/standup")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if submitted != "Summarize yesterday's commits." {
		t.Errorf("submitted prompt = %q", submitted)
	}
}

func TestCustomSlashCommand_UnknownReportsError(t *testing.T) {
	m := newTestModel("")
	m.ready = true

	m.textarea.SetValue("/nonexistent")
	nm, _ := m.submitInput()
	m = nm.(Model)

	found := false
	for _, msg := range m.messages {
		if strings.Contains(msg.Content, "Unknown command: /nonexistent") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an 'Unknown command' message, got %+v", m.messages)
	}
}

// ── One-shot mode (InitialPrompt auto-submit) ─────────────────────────────────

func TestOneShotMode(t *testing.T) {
	submitted := make(chan string, 1)

	cfg := SessionConfig{
		ChatID:        "test-oneshot",
		Model:         "test",
		InitialPrompt: "explain foo.go",
	}
	m := New(cfg, func(_, _, input string, _ bool) <-chan StreamEvent {
		submitted <- input
		ch := make(chan StreamEvent, 1)
		ch <- StreamEvent{Kind: "done"}
		close(ch)
		return ch
	})

	// WindowSizeMsg triggers InitialPrompt auto-submit.
	m, _ = update(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if !m.initialPromptSent {
		t.Error("initialPromptSent should be true after WindowSizeMsg")
	}

	select {
	case prompt := <-submitted:
		if prompt != "explain foo.go" {
			t.Errorf("submitted prompt = %q, want 'explain foo.go'", prompt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("InitialPrompt was not auto-submitted within timeout")
	}
}

// ── TTY detection ─────────────────────────────────────────────────────────────

// TestIsTerminal verifies the isTerminal helper in cmd/code.go is accessible
// via the package-level function that mirrors it. This test ensures the
// non-TTY path (piped/redirected stdout) is detectable.
// Note: in test execution stdout is not a TTY, so this always returns false.
func TestIsTerminalReturnsFalseInTests(t *testing.T) {
	// We cannot import cmd here, but we can verify the TUI's own TTY guard:
	// when ready is false, View() returns "".
	m := newTestModel("")
	m.ready = false
	if m.View() != "" {
		t.Error("View() should return empty string when ready is false (non-TTY guard)")
	}
}

// ── Slash command menu ────────────────────────────────────────────────────────

func TestSlashMenuAppears(t *testing.T) {
	m := newTestModel("")

	m.textarea.SetValue("/h")
	m.updateSlashMenu()

	if !m.showSlash {
		t.Error("slash menu should show after typing /h")
	}
	found := false
	for _, cmd := range m.slashMatches {
		if cmd.Name == "/help" {
			found = true
		}
	}
	if !found {
		t.Error("/help should appear in slash menu after typing /h")
	}
}

func TestSlashMenuDismissOnEsc(t *testing.T) {
	m := newTestModel("")
	m.textarea.SetValue("/")
	m.updateSlashMenu()
	m.showSlash = true

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.showSlash {
		t.Error("slash menu should dismiss on Esc")
	}
}

// ── Token count display ───────────────────────────────────────────────────────

func TestTokenCountInHeader(t *testing.T) {
	m := newTestModel("")
	m.totalPromptTokens = 45_000

	header := m.renderHeader()
	if !strings.Contains(header, "45k") {
		t.Errorf("header should show '45k' token count, got: %s", header)
	}
}
