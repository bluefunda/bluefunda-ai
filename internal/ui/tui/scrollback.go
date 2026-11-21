package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// The conversation is rendered the way Claude Code renders it: each finished
// message is printed once into the terminal's normal scrollback, where the
// terminal's own scrolling, search and selection work on it and nothing
// redraws it. View draws only the live part below it — the streaming
// response, the permission dialog, the input box and the status lines.

// scrollbackDoneMsg reports that a scrollback write has been handed to the
// renderer, so the next one may start.
type scrollbackDoneMsg struct{}

// fitWidth hard-wraps every line of s to the terminal width. A line wider
// than the terminal would be wrapped by the terminal itself, taking more rows
// than the renderer counted, and the next redraw would overwrite the wrong
// rows. Rendering aims to stay narrower; this catches what doesn't (long
// URLs, keys, hashes).
func (m Model) fitWidth(s string) string {
	if m.width <= 0 {
		return s
	}
	return ansi.Hardwrap(s, m.width, true)
}

// contentWidth is the width messages are rendered at.
func (m Model) contentWidth() int {
	return max(m.width-4, 20)
}

// flushScrollback prints, in order, the finished messages that have not been
// printed yet, stopping at the first one still streaming. Only one write is
// in flight at a time: tea commands run concurrently, so two writes issued
// back to back could otherwise land out of order.
func (m Model) flushScrollback() (Model, tea.Cmd) {
	if !m.ready || m.printing {
		return m, nil
	}
	width := m.contentWidth()
	var parts []string
	for i := range m.messages {
		msg := &m.messages[i]
		if msg.printed {
			continue
		}
		if msg.Streaming {
			if m.streaming {
				break
			}
			// The turn ended without finalizing it (Ctrl+C, a stream
			// error): finish it here rather than block every later write.
			msg.finishStreaming()
		}
		parts = append(parts, m.renderMessageAt(i, width))
		msg.printed = true
	}
	if len(parts) == 0 {
		return m, nil
	}
	m.printing = true
	text := strings.TrimRight(strings.Join(parts, "\n"), "\n")
	return m, tea.Sequence(tea.Println(m.fitWidth(text)), func() tea.Msg { return scrollbackDoneMsg{} })
}

// renderLive renders the messages not yet printed to scrollback.
func (m *Model) renderLive() string {
	width := m.contentWidth()
	var sb strings.Builder
	for i := range m.messages {
		if m.messages[i].printed {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(m.renderMessageAt(i, width))
	}
	return sb.String()
}

// printDetails reprints the last response with thinking, narration and every
// tool call expanded (Ctrl+O while idle). Returns nil if there is none.
func (m *Model) printDetails() tea.Cmd {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		if msg.Role != RoleAssistant || !msg.printed {
			continue
		}
		if msg.Thinking == "" && len(msg.ToolEvents) == 0 && msg.narrationEnd == 0 {
			return nil // nothing was collapsed
		}
		head := "\n  " + m.theme.ToolDim.Render("── last response in full (ctrl+o to collapse new ones) ──")
		return tea.Println(m.fitWidth(head + m.renderMessageAt(i, m.contentWidth())))
	}
	return nil
}
