package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

func withIdentity(t *testing.T, id app.Identity) {
	t.Helper()
	app.Set(id)
	t.Cleanup(func() { app.Set(app.Bai) })
}

func TestProductIdentity_BrandsHeaderPlaceholderAndAccent(t *testing.T) {
	withIdentity(t, app.Identity{Name: "abaper", DisplayName: "ABAPer", Placeholder: "Type your ABAP request", Accent: "#FF5F1F"})

	m := newTestModel("")
	if !strings.Contains(m.renderHeader(), "ABAPer") {
		t.Errorf("header should show the product display name: %q", m.renderHeader())
	}
	if m.textarea.Placeholder != "Type your ABAP request" {
		t.Errorf("placeholder = %q", m.textarea.Placeholder)
	}
	if m.theme.Accent != lipgloss.Color("#FF5F1F") {
		t.Errorf("accent = %q, want the product accent", m.theme.Accent)
	}
}

func TestRunSlashCommand_ShowsOutputAsSystemMessage(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	var gotArgs string
	m.cfg.CustomCommands = []SlashCommand{{
		Name: "/source",
		Run: func(args string) (string, error) {
			gotArgs = args
			return "REPORT zhello.", nil
		},
	}}

	m.textarea.SetValue("/source ZHELLO PROG/P")
	nm, cmd := m.submitInput()
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("expected an async command for a Run slash command")
	}
	m, _ = update(m, cmd())

	if gotArgs != "ZHELLO PROG/P" {
		t.Errorf("handler args = %q", gotArgs)
	}
	last := m.messages[len(m.messages)-1]
	if last.Content != "REPORT zhello." {
		t.Errorf("last message = %q, want handler output", last.Content)
	}
}

func TestRunSlashCommand_ErrorIsReported(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.cfg.CustomCommands = []SlashCommand{{
		Name: "/activate",
		Run:  func(string) (string, error) { return "", errors.New("no active SAP system") },
	}}

	m.textarea.SetValue("/activate")
	nm, cmd := m.submitInput()
	m = nm.(Model)
	m, _ = update(m, cmd())

	last := m.messages[len(m.messages)-1]
	if !strings.Contains(last.Content, "/activate failed: no active SAP system") {
		t.Errorf("last message = %q", last.Content)
	}
}
