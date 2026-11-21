package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestShellCommand(t *testing.T) {
	for in, want := range map[string]string{"!git status": "git status", "! ls -la ": "ls -la"} {
		if got, ok := shellCommand(in); !ok || got != want {
			t.Errorf("shellCommand(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{"!", "!  ", "git status", "hello !x"} {
		if _, ok := shellCommand(in); ok {
			t.Errorf("%q should not be a shell command", in)
		}
	}
}

// "!cmd" runs the command through ShellFn instead of sending a message, and
// shows the command and its output.
func TestShellModeRunsCommand(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	var ran string
	m.cfg.ShellFn = func(c string) (string, error) { ran = c; return "on branch main\n", nil }
	m.submitFn = func(_, _, _ string, _ bool) <-chan StreamEvent {
		t.Fatal("a shell command must not start a turn")
		return nil
	}
	m.textarea.SetValue("!git status")
	nm, cmd := m.submitInput()
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("no command returned")
	}
	nm, _ = m.Update(cmd())
	m = nm.(Model)
	if ran != "git status" {
		t.Errorf("ran %q", ran)
	}
	n := len(m.messages)
	if m.messages[n-2].Content != "! git status" || !strings.Contains(m.messages[n-1].Content, "⎿ on branch main") {
		t.Errorf("messages = %q / %q", m.messages[n-2].Content, m.messages[n-1].Content)
	}
}

// Without ShellFn (chat sessions) "!x" is an ordinary message.
func TestShellModeOffInChat(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	sent := ""
	m.submitFn = func(_, _, in string, _ bool) <-chan StreamEvent {
		sent = in
		ch := make(chan StreamEvent)
		close(ch)
		return ch
	}
	m.textarea.SetValue("!important")
	m.submitInput()
	if sent != "!important" {
		t.Errorf("sent %q", sent)
	}
}

func TestFormatShellOutput(t *testing.T) {
	if got := formatShellOutput("", nil); !strings.Contains(got, "(no output)") {
		t.Errorf("empty: %q", got)
	}
	if got := formatShellOutput("", errors.New("boom")); !strings.Contains(got, "boom") {
		t.Errorf("error: %q", got)
	}
	var lines []string
	for i := range 50 {
		lines = append(lines, fmt.Sprint("line", i))
	}
	got := formatShellOutput(strings.Join(lines, "\n"), nil)
	if !strings.Contains(got, "… 10 more lines") || strings.Contains(got, "line45") {
		t.Errorf("long output not capped:\n%s", got)
	}
}
