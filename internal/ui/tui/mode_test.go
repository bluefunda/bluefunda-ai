package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Shift+Tab cycles default → accept edits → plan → default, telling the
// session about each change, and the footer shows the mode.
func TestShiftTabCyclesModes(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	m.cfg.IsCode = true
	var edits, plan []bool
	m.cfg.SetAcceptEditsFn = func(b bool) { edits = append(edits, b) }
	m.cfg.SetPlanModeFn = func(b bool) { plan = append(plan, b) }

	press := func() {
		nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
		m = nm.(Model)
	}
	press()
	if m.mode() != modeAcceptEdits || !strings.Contains(m.renderFooter(), "accept edits on") {
		t.Fatalf("after 1: mode %v, footer %q", m.mode(), m.renderFooter())
	}
	press()
	if m.mode() != modePlan || m.cfg.AcceptEdits || !strings.Contains(m.renderFooter(), "plan mode on") {
		t.Fatalf("after 2: mode %v", m.mode())
	}
	press()
	if m.mode() != modeDefault || strings.Contains(m.renderFooter(), "(shift+tab to cycle)") {
		t.Fatalf("after 3: mode %v, footer %q", m.mode(), m.renderFooter())
	}
	if want := []bool{true, false}; len(edits) != 2 || edits[0] != want[0] || edits[1] != want[1] {
		t.Errorf("accept-edits calls = %v", edits)
	}
	if len(plan) != 2 || !plan[0] || plan[1] {
		t.Errorf("plan calls = %v", plan)
	}
}

func TestShiftTabLeavesAuto(t *testing.T) {
	m := newTestModel("")
	m.cfg.IsCode, m.cfg.AutoApply = true, true
	var auto []bool
	m.cfg.SetAutoApplyFn = func(b bool) { auto = append(auto, b) }
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if nm.(Model).mode() != modeDefault || len(auto) != 1 || auto[0] {
		t.Errorf("mode %v, auto calls %v", nm.(Model).mode(), auto)
	}
}

// Chat sessions have no tools, so no modes.
func TestShiftTabIgnoredInChat(t *testing.T) {
	m := newTestModel("")
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if nm.(Model).mode() != modeDefault {
		t.Error("mode changed in a chat session")
	}
}
