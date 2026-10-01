package tui

import (
	"strings"
	"testing"
)

// containsMessage reports whether any message in m.messages contains substr.
func containsMessage(m Model, substr string) bool {
	for _, msg := range m.messages {
		if strings.Contains(msg.Content, substr) {
			return true
		}
	}
	return false
}

func TestPlanSlashCommand_TogglesOnAndOff(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.cfg.IsCode = true

	var setCalls []bool
	m.cfg.SetPlanModeFn = func(enabled bool) { setCalls = append(setCalls, enabled) }

	m.textarea.SetValue("/plan")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !m.cfg.PlanMode {
		t.Fatal("expected PlanMode to be true after the first /plan")
	}
	if !containsMessage(m, "Plan mode enabled") {
		t.Errorf("expected an 'enabled' announcement, got %+v", m.messages)
	}

	m.textarea.SetValue("/plan")
	nm, _ = m.submitInput()
	m = nm.(Model)

	if m.cfg.PlanMode {
		t.Fatal("expected PlanMode to be false after the second /plan")
	}
	if !containsMessage(m, "Plan mode disabled") {
		t.Errorf("expected a 'disabled' announcement, got %+v", m.messages)
	}

	if len(setCalls) != 2 || setCalls[0] != true || setCalls[1] != false {
		t.Errorf("expected SetPlanModeFn called with [true, false], got %v", setCalls)
	}
}

func TestPlanSlashCommand_UnavailableOutsideCodeSessions(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.cfg.IsCode = false

	m.textarea.SetValue("/plan")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if m.cfg.PlanMode {
		t.Error("expected PlanMode to remain false outside code sessions")
	}
	if !containsMessage(m, "/plan is only available in code sessions") {
		t.Errorf("expected an unavailable message, got %+v", m.messages)
	}
}
