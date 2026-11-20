package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func runSlash(t *testing.T, m Model, cmd string) Model {
	t.Helper()
	m.textarea.SetValue(cmd)
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func lastSystem(m Model) string { return m.messages[len(m.messages)-1].Content }

func TestMaxTurnsSlashCommand(t *testing.T) {
	m := newTestModel("")
	got := 0
	m.cfg.MaxTurns = 50
	m.cfg.SetMaxTurnsFn = func(n int) { got = n }

	m = runSlash(t, m, "/max-turns")
	if !strings.Contains(lastSystem(m), "50") {
		t.Errorf("bare /max-turns should show the limit, got %q", lastSystem(m))
	}
	m = runSlash(t, m, "/max-turns 120")
	if got != 120 || m.cfg.MaxTurns != 120 {
		t.Errorf("limit not applied: fn got %d, cfg %d", got, m.cfg.MaxTurns)
	}
	m = runSlash(t, m, "/max-turns zero")
	if !strings.Contains(lastSystem(m), "Usage") || got != 120 {
		t.Errorf("invalid input should show usage and keep the limit, got %q (limit %d)", lastSystem(m), got)
	}
}

func TestMaxTurnsSlashCommand_UnavailableOutsideCode(t *testing.T) {
	m := runSlash(t, newTestModel(""), "/max-turns 10")
	if !strings.Contains(lastSystem(m), "only available") {
		t.Errorf("got %q", lastSystem(m))
	}
}
