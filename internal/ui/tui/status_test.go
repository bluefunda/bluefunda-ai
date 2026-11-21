package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestStatusCommand(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	m.cfg.IsCode, m.cfg.PlanMode, m.cfg.Model = true, true, "think"
	m.cfg.StatusFn = func() []string { return []string{"Context:    ~1.2k tokens"} }
	m.textarea.SetValue("/status")
	nm, _ := m.submitInput()
	out := nm.(Model).messages[len(nm.(Model).messages)-1].Content
	for _, want := range []string{"Model:      think", "plan (read-only)", "no limit", "~1.2k tokens"} {
		if !strings.Contains(out, want) {
			t.Errorf("/status lacks %q:\n%s", want, out)
		}
	}
}

// An Instant command runs on the spot and shows only its output.
func TestInstantCommand(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	m.cfg.CustomCommands = []SlashCommand{{Name: "/agents", Instant: true, Run: func(a string) (string, error) { return "list " + a, nil }}}
	m.textarea.SetValue("/agents x")
	nm, cmd := m.submitInput()
	if cmd != nil {
		t.Error("an instant command should not start async work")
	}
	msgs := nm.(Model).messages
	if got := msgs[len(msgs)-1].Content; got != "list x" {
		t.Errorf("shown = %q", got)
	}
	if strings.Contains(msgs[len(msgs)-2].Content, "…") {
		t.Error("no progress line for an instant command")
	}
}

// Every line of a multi-line notice is indented, and an unset model reads
// as the server's default rather than blank.
func TestStatusLayout(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	m.cfg.Model = ""
	m.messages = append(m.messages, newSystemMessage(m.statusText()))
	out := m.renderMessageAt(len(m.messages)-1, 80)
	for _, line := range strings.Split(out, "\n") {
		if plain := ansi.Strip(line); plain != "" && !strings.HasPrefix(plain, "  ") {
			t.Errorf("line not indented: %q", plain)
		}
	}
	if !strings.Contains(out, "default (chosen by the server)") {
		t.Errorf("model line:\n%s", out)
	}
}
