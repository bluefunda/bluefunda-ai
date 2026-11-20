package tui

import (
	"testing"
)

func TestMouseSlashCommandToggles(t *testing.T) {
	m := newTestModel("")
	if m.mouseOn {
		t.Fatal("mouse capture must be off by default so text can be selected")
	}
	m.textarea.SetValue("/mouse")
	nm, cmd := m.handleSlashCommand("/mouse")
	m = nm.(Model)
	if !m.mouseOn || cmd == nil {
		t.Fatalf("first /mouse should enable capture and return a command (on=%v)", m.mouseOn)
	}
	nm, cmd = m.handleSlashCommand("/mouse")
	m = nm.(Model)
	if m.mouseOn || cmd == nil {
		t.Fatalf("second /mouse should disable capture (on=%v)", m.mouseOn)
	}
}

func TestIsSSH(t *testing.T) {
	for _, k := range []string{"SSH_TTY", "SSH_CONNECTION", "SSH_CLIENT"} {
		t.Setenv(k, "")
	}
	if isSSH() {
		t.Error("no SSH variables set: want false")
	}
	t.Setenv("SSH_CONNECTION", "10.0.0.1 5000 10.0.0.2 22")
	if !isSSH() {
		t.Error("SSH_CONNECTION set: want true")
	}
}
