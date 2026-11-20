package tui

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCurrentMention(t *testing.T) {
	cases := map[string]struct {
		q  string
		ok bool
	}{
		"look at @int":    {"int", true},
		"@":               {"", true},
		"line\n@x":        {"x", true},
		"@done and more":  {"", false},
		"mail me@host":    {"", false},
		"no mention here": {"", false},
	}
	for in, want := range cases {
		if q, ok := currentMention(in); q != want.q || ok != want.ok {
			t.Errorf("currentMention(%q) = %q, %v", in, q, ok)
		}
	}
}

func TestMatchMentions_Ranking(t *testing.T) {
	paths := []string{"docs/model.md", "internal/ui/tui/model.go", "model.go", "internal/", "cmd/remodel.go"}
	got := matchMentions(paths, "model")
	want := []string{"model.go", "docs/model.md", "internal/ui/tui/model.go", "cmd/remodel.go"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := matchMentions(paths, "int"); got[0] != "internal/" {
		t.Errorf("a directory whose name starts with the query should come first: %q", got)
	}
}

// Typing "@mod" opens the picker; Tab puts the path in and closes it; Enter
// on a directory keeps it open for the directory's contents.
func TestMentionPicker(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	m.cfg.FilesFn = func() []string { return []string{"internal/", "internal/model.go", "README.md"} }
	typeText := func(s string) {
		for _, r := range s {
			nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = nm.(Model)
		}
	}
	typeText("see @inter")
	if !m.showMention || m.mentionMatches[0] != "internal/" {
		t.Fatalf("picker: %v %q", m.showMention, m.mentionMatches)
	}
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.textarea.Value() != "see @internal/" || !m.showMention {
		t.Fatalf("after a directory: %q, open %v", m.textarea.Value(), m.showMention)
	}
	typeText("mod")
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m = nm.(Model)
	if m.textarea.Value() != "see @internal/model.go " || m.showMention {
		t.Errorf("after a file: %q, open %v", m.textarea.Value(), m.showMention)
	}
}
