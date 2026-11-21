package tui

import (
	"fmt"
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

// A notice with a line wider than the terminal word-wraps; no line
// overflows and no blank lines appear that weren't in the text.
func TestSystemMessageWraps(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	text := "Header:\n\n" + strings.Repeat("read_file, list_dir, search_files, ", 6) + "\nlast"
	m.messages = append(m.messages, newSystemMessage(text))
	out := ansi.Strip(m.renderMessageAt(len(m.messages)-1, 60))
	lines := strings.Split(out, "\n")
	blank := 0
	for _, l := range lines {
		if ansi.StringWidth(l) > 62 {
			t.Errorf("line wider than the terminal: %q", l)
		}
		if strings.TrimSpace(l) == "" {
			blank++
		}
	}
	if blank != 1 {
		t.Errorf("want only the text's one blank line, got %d:\n%s", blank, out)
	}
	if strings.Contains(out, "searc\nh") {
		t.Errorf("broke inside a word:\n%s", out)
	}
}

func TestFooterFitsWidth(t *testing.T) {
	m := newTestModel("")
	m.cfg.IsCode = true
	for _, w := range []int{40, 80, 120, 200} {
		m.width = w
		f := m.renderFooter()
		if ansi.StringWidth(f) > w {
			t.Errorf("width %d: footer is %d wide: %q", w, ansi.StringWidth(f), ansi.Strip(f))
		}
		if strings.HasSuffix(strings.TrimSpace(ansi.Strip(f)), "·") {
			t.Errorf("width %d: dangling separator", w)
		}
	}
	m.width = 200
	if !strings.Contains(ansi.Strip(m.renderFooter()), "Shift+Tab mode") {
		t.Error("wide terminal should show every hint")
	}
}

func TestWelcomeShowsDefaultModel(t *testing.T) {
	m := New(SessionConfig{ChatID: "test-0000-0000-0000-000000000000"}, nil)
	if !strings.Contains(m.messages[0].Content, "model: default") {
		t.Errorf("welcome = %q", m.messages[0].Content)
	}
}

// /mcp off|on switches an included agent; the list marks included ones.
func TestMCPIncludeToggle(t *testing.T) {
	m := newTestModel("")
	m.ready = true
	var calls []string
	m.cfg.MCPIncludeFn = func(name string, on bool) bool {
		calls = append(calls, fmt.Sprint(name, on))
		return name == "inc-mcp"
	}
	for _, in := range []string{"/mcp off inc-mcp", "/mcp on inc-mcp", "/mcp off other"} {
		m.textarea.SetValue(in)
		nm, _ := m.submitInput()
		m = nm.(Model)
	}
	if strings.Join(calls, ",") != "inc-mcpfalse,inc-mcptrue,otherfalse" {
		t.Errorf("calls = %q", calls)
	}
	if last := m.messages[len(m.messages)-1].Content; !strings.Contains(last, "isn't an included agent") {
		t.Errorf("last = %q", last)
	}
	list := formatMCPList([]MCPInfo{{Name: "inc-mcp", Available: true, Included: true}, {Name: "off-mcp", Included: true, Off: true}})
	if !strings.Contains(list, "inc-mcp") || !strings.Contains(list, "(included)") || !strings.Contains(list, "off this session") {
		t.Errorf("list:\n%s", list)
	}
}
