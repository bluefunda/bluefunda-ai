package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// View implements tea.Model — the top-level render function.
func (m Model) View() string {
	if !m.vpReady || m.width == 0 {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteByte('\n')

	// Conversation viewport
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')

	// Slash menu (shown above input when active)
	if m.showSlash && len(m.slashMatches) > 0 {
		b.WriteString(m.renderSlashMenu())
		b.WriteByte('\n')
	}

	// Approval prompt
	if m.pendingApproval != nil {
		b.WriteString(m.renderApproval())
		b.WriteByte('\n')
	}

	// Input area
	b.WriteString(m.renderInput())
	b.WriteByte('\n')

	// Footer hints
	b.WriteString(m.renderFooter())

	return b.String()
}

// ──────────────────────────────────────────────
//  Header
// ──────────────────────────────────────────────

func (m Model) renderHeader() string {
	th := m.theme
	left := th.AssistantLabel.Render("BlueFunda AI") +
		th.ToolDim.Render("  ·  ") +
		th.ToolDim.Render(m.cfg.Model)
	if m.cfg.RepoName != "" {
		left += th.ToolDim.Render("  ·  " + m.cfg.RepoName)
	}
	if m.cfg.IsCode {
		left += th.ToolDim.Render("  ·  code")
	}

	right := th.ToolDim.Render(m.cfg.ChatID[:8])

	spacer := strings.Repeat(" ", max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2))
	line := " " + left + spacer + right

	return th.Header.Width(m.width).Render(line)
}

// ──────────────────────────────────────────────
//  Messages
// ──────────────────────────────────────────────

func (m *Model) renderMessages() string {
	if len(m.messages) == 0 {
		return ""
	}

	var sb strings.Builder
	innerWidth := m.width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	for i := range m.messages {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(m.renderMessageAt(i, innerWidth))
	}

	return sb.String()
}

func (m *Model) renderMessageAt(idx, width int) string {
	msg := m.messages[idx]
	th := m.theme

	switch msg.Role {
	case RoleSystem:
		return "  " + th.SystemMsg.Render(msg.Content)

	case RoleUser:
		label := th.UserLabel.Render("You")
		lines := strings.Split(msg.Content, "\n")
		for i, line := range lines {
			if i == 0 {
				lines[i] = "  " + line
			} else {
				lines[i] = "     " + line
			}
		}
		return "\n" + "  " + label + "\n" + th.UserContent.Render(strings.Join(lines, "\n"))

	case RoleAssistant:
		var sb strings.Builder
		label := th.AssistantLabel.Render("Assistant")
		if msg.Streaming {
			frame := spinnerFrames[m.spinnerIdx%len(spinnerFrames)]
			label = th.AssistantLabel.Render("Assistant") + " " +
				th.Spinner.Render(frame)
		}
		sb.WriteString("\n  " + label + "\n")

		for _, ev := range msg.ToolEvents {
			sb.WriteString(formatToolEventLine(ev, th))
			sb.WriteByte('\n')
		}

		if msg.Content != "" {
			sb.WriteString(m.renderMarkdownAt(idx, width))
		} else if msg.Streaming && len(msg.ToolEvents) == 0 {
			sb.WriteString("  " + th.ToolDim.Render("working..."))
		}

		return sb.String()
	}
	return ""
}

// renderMarkdownAt renders message i with prose via glamour and code blocks
// via our custom syntax-highlighted renderer. Results are cached by (content,
// width) so we don't re-render on every spinner tick.
func (m *Model) renderMarkdownAt(i, width int) string {
	msg := &m.messages[i]
	if msg.rendered != "" && msg.renderWidth == width {
		return msg.rendered
	}

	rendered := renderMixed(msg.Content, width, m.theme)

	msg.rendered = rendered
	msg.renderWidth = width
	return rendered
}

// renderMixed splits the content into prose and fenced code segments, renders
// each appropriately, and joins them back.
func renderMixed(content string, width int, th Theme) string {
	segs := splitMarkdown(content)
	var sb strings.Builder

	for _, seg := range segs {
		if seg.isCode {
			sb.WriteString(renderCodeBlock(seg.lang, seg.body, width, th))
			sb.WriteByte('\n')
		} else {
			body := seg.body
			if strings.TrimSpace(body) == "" {
				sb.WriteString(body)
				continue
			}
			rendered, err := glamourRender(body, width)
			if err != nil || rendered == "" {
				rendered = indentLines(body, "  ")
			}
			sb.WriteString(rendered)
		}
	}
	return sb.String()
}

// ──────────────────────────────────────────────
//  Input area
// ──────────────────────────────────────────────

func (m Model) renderInput() string {
	th := m.theme

	var inner string
	if m.streaming {
		frame := spinnerFrames[m.spinnerIdx%len(spinnerFrames)]
		inner = "  " + th.Spinner.Render(frame) + "  " +
			th.ToolDim.Render("Generating...")
		lines := strings.Repeat("\n", inputMinLines-1)
		inner += lines
	} else {
		inner = m.textarea.View()
	}

	return th.InputBorder.
		Width(m.width - 2).
		Render(inner)
}

// ──────────────────────────────────────────────
//  Slash command menu
// ──────────────────────────────────────────────

func (m Model) renderSlashMenu() string {
	th := m.theme

	var rows []string
	for i, cmd := range m.slashMatches {
		name := lipgloss.NewStyle().Foreground(th.AccentBold).Render(cmd.Name)
		desc := lipgloss.NewStyle().Foreground(th.Secondary).Render("  " + cmd.Description)
		line := "  " + name + desc
		if i == m.slashIdx {
			line = th.SlashSelected.Width(m.width - 6).Render(line)
		}
		rows = append(rows, line)
	}

	// Show at most 6 entries
	if len(rows) > 6 {
		start := m.slashIdx - 2
		if start < 0 {
			start = 0
		}
		end := start + 6
		if end > len(rows) {
			end = len(rows)
			start = end - 6
		}
		rows = rows[start:end]
	}

	content := strings.Join(rows, "\n")
	return th.SlashMenu.Width(m.width - 4).Render(content)
}

// ──────────────────────────────────────────────
//  Approval prompt
// ──────────────────────────────────────────────

func (m Model) renderApproval() string {
	th := m.theme
	ap := m.pendingApproval
	tool := th.ToolIcon.Render("●") + "  " +
		th.ToolName.Render(ap.ToolName)
	args := ""
	if ap.Args != "" && ap.Args != "{}" {
		a := ap.Args
		if len(a) > 70 {
			a = a[:67] + "..."
		}
		args = "\n     " + th.ToolArg.Render(a)
	}
	warnStyle := lipgloss.NewStyle().Foreground(th.Warning)
	prompt := warnStyle.Render("  Apply? ") + th.ToolDim.Render("[y/N]")

	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(m.theme.Warning).
		PaddingLeft(1).
		Render(fmt.Sprintf("  %s%s\n%s", tool, args, prompt))
}

// ──────────────────────────────────────────────
//  Footer
// ──────────────────────────────────────────────

func (m Model) renderFooter() string {
	th := m.theme
	hint := "Enter send  ·  Shift+Enter newline  ·  /help commands  ·  Ctrl+C quit"
	if m.streaming {
		hint = "Ctrl+C to interrupt"
	}
	return "  " + th.Footer.Render(hint)
}

// ──────────────────────────────────────────────
//  Markdown rendering
// ──────────────────────────────────────────────

// glamourRenderer is a lazily-initialised per-width renderer cache.
var glamourRendererCache = map[int]*glamour.TermRenderer{}

func glamourRender(content string, width int) (string, error) {
	r, ok := glamourRendererCache[width]
	if !ok {
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(width),
			glamour.WithEmoji(),
		)
		if err != nil {
			return "", err
		}
		glamourRendererCache[width] = r
	}
	return r.Render(content)
}

// indentLines is a fallback plain-text renderer used when glamour fails.
func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
