package slash

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bluefunda/abaper/styles"
)

// SelectedMsg is sent when the user selects a slash command.
type SelectedMsg struct {
	Cmd  Command
	Args []string
}

// CancelMsg is sent when the user presses Esc in the slash menu.
type CancelMsg struct{}

// MenuModel is the slash command picker overlay.
type MenuModel struct {
	filter   string
	cursor   int
	filtered []Command
	width    int
}

// NewMenu creates a new slash menu with the given display width.
func NewMenu(width int) *MenuModel {
	m := &MenuModel{width: width}
	m.refilter()
	return m
}

// Height returns the total rendered line count (for viewport sizing).
func (m *MenuModel) Height() int {
	return len(m.filtered) + 6 // filter + 2 separators + help + 2 border lines
}

func (m *MenuModel) refilter() {
	// Only match on the command name (first word); the rest is args.
	word := ""
	if parts := strings.Fields(m.filter); len(parts) > 0 {
		word = strings.ToLower(parts[0])
	}
	if word == "" {
		m.filtered = Registry
	} else {
		var out []Command
		for _, c := range Registry {
			if strings.Contains(strings.ToLower(c.Name), word) ||
				strings.Contains(strings.ToLower(c.Description), word) {
				out = append(out, c)
			}
		}
		m.filtered = out
	}
	if m.cursor >= len(m.filtered) && len(m.filtered) > 0 {
		m.cursor = len(m.filtered) - 1
	}
	if len(m.filtered) == 0 {
		m.cursor = 0
	}
}

// Update processes key events for the slash menu.
func (m *MenuModel) Update(msg tea.Msg) (*MenuModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		return m, func() tea.Msg { return CancelMsg{} }

	case tea.KeyEnter:
		if len(m.filtered) > 0 {
			cmd := m.filtered[m.cursor]
			args := []string{}
			if m.filter != "" && m.filter != cmd.Name {
				// remaining text after command name becomes first arg
				remainder := strings.TrimPrefix(m.filter, cmd.Name)
				remainder = strings.TrimSpace(remainder)
				if remainder != "" {
					args = strings.Fields(remainder)
				}
			}
			return m, func() tea.Msg { return SelectedMsg{Cmd: cmd, Args: args} }
		}

	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}

	case tea.KeyDown:
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.refilter()
		}

	case tea.KeySpace:
		m.filter += " "
		m.refilter()

	case tea.KeyRunes:
		m.filter += string(keyMsg.Runes)
		m.refilter()
	}

	return m, nil
}

// View renders the slash menu as a bordered popup.
func (m *MenuModel) View() string {
	w := max(m.width, 42)
	innerW := w - 4 // account for border + padding

	muted := lipgloss.NewStyle().Foreground(styles.ColorMuted)
	accent := lipgloss.NewStyle().Foreground(styles.ColorAccent).Bold(true)
	fg := lipgloss.NewStyle().Foreground(styles.ColorFg)
	cursor := lipgloss.NewStyle().Foreground(styles.ColorAccent).Bold(true)
	sep := muted.Render(strings.Repeat("─", innerW))

	var sb strings.Builder

	// Filter line
	filterText := "/ " + m.filter + "▌"
	sb.WriteString(fg.Render(filterText))
	sb.WriteString("\n")
	sb.WriteString(sep)
	sb.WriteString("\n")

	// Command list
	if len(m.filtered) == 0 {
		sb.WriteString(muted.Render("  no matching commands"))
		sb.WriteString("\n")
	} else {
		for i, cmd := range m.filtered {
			nameStr := "/" + cmd.Name
			// Pad name to fixed width for alignment
			nameCol := nameStr + strings.Repeat(" ", max(0, 14-len(nameStr)))
			desc := cmd.Description

			if i == m.cursor {
				line := " ❯ " + cursor.Render(nameCol) + " " + fg.Render(desc)
				sb.WriteString(line)
			} else {
				line := "   " + accent.Render(nameCol) + " " + muted.Render(desc)
				sb.WriteString(line)
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString(sep)
	sb.WriteString("\n")
	sb.WriteString(muted.Render("  ↑↓ navigate   Enter select   Esc cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorAccent).
		Padding(0, 1).
		Width(w).
		Render(sb.String())
}

