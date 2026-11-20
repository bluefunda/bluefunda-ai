package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// RewindPoint is one earlier message /rewind can go back to.
type RewindPoint struct {
	Turn    int
	Prompt  string
	At      time.Time
	Files   int  // files changed since; 0 = nothing to restore
	CanChat bool // false once the conversation was compacted past it
}

// rewindAction is what to restore.
type rewindAction struct {
	label      string
	code, chat bool
}

// rewindDoneMsg carries the outcome of a rewind.
type rewindDoneMsg struct {
	summary, prompt string
	chat            bool
	err             error
}

// rewindPicker is the /rewind (Esc Esc) dialog: pick a message, then what
// to restore, as in Claude Code.
type rewindPicker struct {
	points  []RewindPoint // newest first
	idx     int
	actions []rewindAction // set once a message is picked
	actIdx  int
}

// doubleEsc is how quickly a second Esc must follow the first to open it.
const doubleEsc = 800 * time.Millisecond

// openRewind shows the picker, or says why it can't.
func (m Model) openRewind() Model {
	if m.cfg.RewindPointsFn == nil || m.cfg.RewindFn == nil {
		m.messages = append(m.messages, newSystemMessage("Rewind is not available in this session."))
		return m
	}
	if m.streaming {
		m.messages = append(m.messages, newSystemMessage("Wait for the response to finish (or Ctrl+C) before rewinding."))
		return m
	}
	pts := m.cfg.RewindPointsFn()
	if len(pts) == 0 {
		m.messages = append(m.messages, newSystemMessage("Nothing to rewind yet."))
		return m
	}
	rev := make([]RewindPoint, len(pts))
	for i, p := range pts {
		rev[len(pts)-1-i] = p
	}
	m.rewind = &rewindPicker{points: rev}
	return m
}

func rewindActions(p RewindPoint) []rewindAction {
	var acts []rewindAction
	if p.CanChat && p.Files > 0 {
		acts = append(acts, rewindAction{"Restore code and conversation", true, true})
	}
	if p.CanChat {
		acts = append(acts, rewindAction{"Restore conversation only", false, true})
	}
	if p.Files > 0 {
		acts = append(acts, rewindAction{fmt.Sprintf("Restore code only (%s)", plural(p.Files, "file")), true, false})
	}
	return append(acts, rewindAction{label: "Cancel"})
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (m Model) handleRewindKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	r := m.rewind
	list, idx := len(r.points), &r.idx
	if r.actions != nil {
		list, idx = len(r.actions), &r.actIdx
	}
	switch msg.String() {
	case "up", "k", "shift+tab":
		*idx = max(*idx-1, 0)
	case "down", "j", "tab":
		*idx = min(*idx+1, list-1)
	case "esc", "ctrl+c":
		if r.actions != nil {
			r.actions = nil // back to the messages
		} else {
			m.rewind = nil
		}
	case "enter":
		if r.actions == nil {
			r.actions, r.actIdx = rewindActions(r.points[r.idx]), 0
			return m, nil
		}
		act, p := r.actions[r.actIdx], r.points[r.idx]
		m.rewind = nil
		if !act.code && !act.chat {
			return m, nil
		}
		fn := m.cfg.RewindFn
		return m, func() tea.Msg {
			summary, err := fn(p.Turn, act.code, act.chat)
			return rewindDoneMsg{summary: summary, prompt: p.Prompt, chat: act.chat, err: err}
		}
	}
	return m, nil
}

// rewound shows the outcome; after a conversation rewind the message goes
// back into the input to edit and resend, as in Claude Code.
func (m Model) rewound(msg rewindDoneMsg) Model {
	if msg.err != nil {
		m.messages = append(m.messages, newSystemMessage("Rewind failed: "+msg.err.Error()))
		return m
	}
	m.messages = append(m.messages, newSystemMessage("⏪ "+msg.summary))
	if msg.chat {
		m.textarea.SetValue(msg.prompt)
		m.textarea.CursorEnd()
	}
	return m
}

func (m Model) renderRewind() string {
	th, r := m.theme, m.rewind
	var rows []string
	if r.actions == nil {
		rows = append(rows, "  "+th.ToolDim.Render("Rewind to before which message?  ·  ↑↓ choose  ·  Enter select  ·  Esc cancel"))
		const visible = 8
		start := max(0, min(r.idx-visible/2, len(r.points)-visible))
		for i := start; i < min(start+visible, len(r.points)); i++ {
			p := r.points[i]
			meta := ago(p.At)
			if p.Files > 0 {
				meta += " · " + plural(p.Files, "file") + " changed since"
			}
			if !p.CanChat {
				meta += " · code only"
			}
			prompt := strings.Join(strings.Fields(p.Prompt), " ")
			line := "  " + truncate(prompt, max(m.width-len(meta)-14, 20)) + th.ToolDim.Render("  "+meta)
			if i == r.idx {
				line = th.SlashSelected.Width(m.width - 6).Render(line)
			}
			rows = append(rows, line)
		}
	} else {
		p := r.points[r.idx]
		rows = append(rows, "  "+th.ToolDim.Render("Rewind to before: ")+truncate(strings.Join(strings.Fields(p.Prompt), " "), max(m.width-30, 20)))
		for i, a := range r.actions {
			line := fmt.Sprintf("  %d. %s", i+1, a.label)
			if i == r.actIdx {
				line = th.SlashSelected.Width(m.width - 6).Render(line)
			}
			rows = append(rows, line)
		}
		rows = append(rows, "  "+th.ToolDim.Render("Shell commands' changes are not tracked."))
	}
	return th.SlashMenu.Width(m.width - 4).Render(strings.Join(rows, "\n"))
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
