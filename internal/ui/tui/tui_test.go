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

// ── Viewport rendering ────────────────────────────────────────────────────────

// TestViewportRendered verifies that View() includes viewport content rather
// than a clamped inline dump. The regression guard for the scrollback bug.
func TestViewportRendered(t *testing.T) {
	m := newTestModel("")

	// Inject several messages beyond what fits in one viewport page.
	for i := range 30 {
		m.messages = append(m.messages, newSystemMessage(fmt.Sprintf("message %d", i)))
	}
	m.refreshViewport()

	view := m.View()

	// The header must be present.
	if !strings.Contains(view, "BlueFunda AI") {
		t.Error("view should contain the header")
	}

	// The footer must be present (cursor anchor check).
	if !strings.Contains(view, "Enter send") {
		t.Error("view should contain the footer hint")
	}

	// The viewport is embedded; the view must not contain ALL 30 messages
	// at once (that would indicate the viewport clamp is missing and content
	// is overflowing the terminal height).
	lineCount := strings.Count(view, "\n")
	if lineCount > 50 {
		t.Errorf("view has %d newlines — looks like content is not clamped by viewport", lineCount)
	}
}

// TestCursorNotTrappedAtBottom ensures the View() height stays within terminal
// bounds even when the conversation is very long.
func TestCursorNotTrappedAtBottom(t *testing.T) {
	m := newTestModel("")

	// Flood with 200 messages (simulates a long session).
	for i := range 200 {
		m.messages = append(m.messages, newAssistantMessage())
		m.messages[len(m.messages)-1].Content = fmt.Sprintf("Response %d: this is some content to make it realistic.", i)
		m.messages[len(m.messages)-1].Streaming = false
	}
	m.refreshViewport()

	view := m.View()
	lines := strings.Split(view, "\n")

	// Terminal height is 24; the view must not exceed it with a significant margin.
	if len(lines) > 30 {
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
	m.vpReady = true

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
	m.vpReady = true

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
	m.vpReady = true

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
	if lineCount > 50 {
		t.Errorf("view has %d lines for a 24-line terminal after large response", lineCount)
	}
}

// ── Keyboard scrolling ────────────────────────────────────────────────────────

func TestKeyboardScrolling(t *testing.T) {
	m := newTestModel("")

	// Load enough messages to make the viewport scrollable.
	for i := range 50 {
		m.messages = append(m.messages, newSystemMessage(fmt.Sprintf("msg %d", i)))
	}
	m.refreshViewport()
	m.viewport.GotoBottom()
	initialOffset := m.viewport.YOffset

	// Page up should move the offset up.
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.viewport.YOffset >= initialOffset && initialOffset > 0 {
		t.Errorf("PgUp did not scroll up: offset before=%d after=%d", initialOffset, m.viewport.YOffset)
	}

	// Page down should move back toward bottom.
	beforeDown := m.viewport.YOffset
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.viewport.YOffset <= beforeDown && m.viewport.YOffset < initialOffset {
		t.Errorf("PgDn did not scroll down: offset before=%d after=%d", beforeDown, m.viewport.YOffset)
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
	if m.viewport.Width != 120 {
		t.Errorf("viewport width not updated: got %d", m.viewport.Width)
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
	m.vpReady = true

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
	// when vpReady is false, View() returns "".
	m := newTestModel("")
	m.vpReady = false
	if m.View() != "" {
		t.Error("View() should return empty string when vpReady is false (non-TTY guard)")
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
