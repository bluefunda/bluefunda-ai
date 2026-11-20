package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func rewindModel(t *testing.T) (Model, *[]string) {
	t.Helper()
	m := newTestModel("")
	m.ready = true
	var calls []string
	m.cfg.RewindPointsFn = func() []RewindPoint {
		return []RewindPoint{
			{Turn: 1, Prompt: "first", At: time.Now(), Files: 2, CanChat: true},
			{Turn: 2, Prompt: "second", At: time.Now(), Files: 0, CanChat: true},
		}
	}
	m.cfg.RewindFn = func(turn int, code, chat bool) (string, error) {
		calls = append(calls, fmt.Sprintf("%d code=%v chat=%v", turn, code, chat))
		return "done", nil
	}
	return m, &calls
}

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// Esc Esc on an empty input opens the picker newest first; picking the
// older message and "code and conversation" rewinds both and puts the
// message back in the input.
func TestRewindPicker(t *testing.T) {
	m, calls := rewindModel(t)
	nm, _ := m.Update(key(tea.KeyEsc))
	nm, _ = nm.(Model).Update(key(tea.KeyEsc))
	m = nm.(Model)
	if m.rewind == nil || m.rewind.points[0].Prompt != "second" {
		t.Fatalf("picker not open newest-first: %+v", m.rewind)
	}
	if !strings.Contains(m.renderRewind(), "2 files changed since") {
		t.Errorf("render:\n%s", m.renderRewind())
	}
	nm, _ = m.Update(key(tea.KeyDown))
	nm, _ = nm.(Model).Update(key(tea.KeyEnter))
	m = nm.(Model)
	if len(m.rewind.actions) != 4 || !m.rewind.actions[0].code || !m.rewind.actions[0].chat {
		t.Fatalf("actions = %+v", m.rewind.actions)
	}
	nm, cmd := m.Update(key(tea.KeyEnter))
	m = nm.(Model)
	if m.rewind != nil || cmd == nil {
		t.Fatal("choosing an action should close the picker and run the rewind")
	}
	nm, _ = m.Update(cmd())
	m = nm.(Model)
	if len(*calls) != 1 || (*calls)[0] != "1 code=true chat=true" {
		t.Errorf("calls = %q", *calls)
	}
	if m.textarea.Value() != "first" || !strings.Contains(m.messages[len(m.messages)-1].Content, "done") {
		t.Errorf("input = %q", m.textarea.Value())
	}
}

// A message with no file changes offers only the conversation; Cancel and
// Esc do nothing.
func TestRewindActionsAndCancel(t *testing.T) {
	acts := rewindActions(RewindPoint{Files: 0, CanChat: true})
	if len(acts) != 2 || acts[0].code || !acts[0].chat || acts[1].label != "Cancel" {
		t.Errorf("acts = %+v", acts)
	}
	if acts := rewindActions(RewindPoint{Files: 3, CanChat: false}); len(acts) != 2 || acts[0].chat {
		t.Errorf("code-only acts = %+v", acts)
	}

	m, calls := rewindModel(t)
	m = m.openRewind()
	nm, _ := m.Update(key(tea.KeyEsc))
	if nm.(Model).rewind != nil || len(*calls) != 0 {
		t.Error("Esc should close the picker without rewinding")
	}
}

// A single Esc, or Esc with text in the input, doesn't open it.
func TestEscAloneDoesNotRewind(t *testing.T) {
	m, _ := rewindModel(t)
	nm, _ := m.Update(key(tea.KeyEsc))
	if nm.(Model).rewind != nil {
		t.Error("opened on one Esc")
	}
	m.textarea.SetValue("draft")
	nm, _ = m.Update(key(tea.KeyEsc))
	nm, _ = nm.(Model).Update(key(tea.KeyEsc))
	if nm.(Model).rewind != nil {
		t.Error("opened with a draft in the input")
	}
}
