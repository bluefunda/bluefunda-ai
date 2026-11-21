package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// A code block has no border (selecting it copies only the code), and a line
// longer than the terminal wraps: no rendered line is wider than the width.
func TestCodeBlockFitsWidth(t *testing.T) {
	key := `expiry-time="20270213" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFcP02TxH+TK417qrvD60j35ezXD6lz3fFiH46Rl6gti user@host`
	const width = 60
	out := renderCodeBlock("", key, width, DefaultTheme())
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("line is %d wide, want <= %d:\n%s", w, width, out)
		}
	}
	if strings.ContainsAny(ansi.Strip(out), "╭│╰") {
		t.Errorf("code block should have no border:\n%s", ansi.Strip(out))
	}
	if tabbed := renderCodeBlock("go", "\tx := 1", width, DefaultTheme()); strings.Contains(tabbed, "\t") {
		t.Error("tabs should be expanded so wrapping counts them right")
	}
	if !strings.Contains(ansi.Strip(out), "@host") {
		t.Errorf("wrapped content lost text:\n%s", ansi.Strip(out))
	}
}

// Whatever is printed or drawn live is hard-wrapped to the terminal width.
func TestFitWidth(t *testing.T) {
	m := Model{width: 20}
	out := m.fitWidth(strings.Repeat("x", 50) + "\nshort")
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 20 {
			t.Fatalf("line %q wider than 20", line)
		}
	}
	if (Model{}).fitWidth("abc") != "abc" {
		t.Error("unknown width should leave text alone")
	}
}
