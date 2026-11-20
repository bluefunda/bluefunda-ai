package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

// Permission dialog: shown in place of the input box while a tool call waits
// for approval. It mirrors Claude Code's prompt — a preview of exactly what
// will run or change, then a numbered option list navigable with ↑/↓, 1-3,
// or the legacy y/a/n keys.

// approvalChoice is one row of the permission dialog.
type approvalChoice int

const (
	choiceYes approvalChoice = iota
	choiceAlways
	choiceNoFeedback
	numApprovalChoices
)

// approvalPreviewLines caps how many preview lines (command, diff, file
// content) the dialog shows before summarising the rest; previewLineBudget
// lowers it further so the dialog fits short terminals.
const approvalPreviewLines = 12

// dialogChromeLines is the dialog's height without its preview: border (2),
// header and title (2), blank line (1), three options (3) and the key hint (1).
const dialogChromeLines = 9

// minViewportLines is the conversation height the dialog always leaves.
const minViewportLines = 4

// previewLineBudget returns how many preview lines fit on screen alongside the
// header, footer, dialog chrome, a minimal conversation view and — in
// feedback mode — the input box.
func (m Model) previewLineBudget() int {
	avail := m.height - headerHeight - footerHeight - 2 - minViewportLines - dialogChromeLines
	if m.approvalFeedback {
		avail -= m.effectiveInputRows() + 2
	}
	return max(1, min(approvalPreviewLines, avail))
}

func (c approvalChoice) label(alwaysScope string) string {
	switch c {
	case choiceYes:
		return "Yes"
	case choiceAlways:
		if alwaysScope == "" {
			alwaysScope = "this exact call"
		}
		return "Yes, and don't ask again for " + alwaysScope + " this session"
	case choiceNoFeedback:
		return "No, and tell " + app.Current().DisplayName + " what to do differently"
	}
	return ""
}

// handleApprovalKey drives the permission dialog. In feedback mode (option 3)
// keys go to the input box until Enter sends the feedback or Esc skips it.
func (m Model) handleApprovalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.quit = true
		if m.pendingApproval != nil {
			m.pendingApproval.ReplyCh <- ApprovalDecision{Approved: false}
			m.pendingApproval = nil
		}
		return m, tea.Quit
	}

	if m.approvalFeedback {
		switch msg.String() {
		case "enter":
			text := strings.TrimSpace(m.textarea.Value())
			m.textarea.Reset()
			return m.answerApproval(ApprovalDecision{Approved: false, Feedback: text})
		case "esc":
			m.textarea.Reset()
			return m.answerApproval(ApprovalDecision{Approved: false})
		}
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	switch strings.ToLower(msg.String()) {
	case "up", "k", "shift+tab":
		m.approvalIdx = (m.approvalIdx + numApprovalChoices - 1) % numApprovalChoices
		return m, nil
	case "down", "j", "tab":
		m.approvalIdx = (m.approvalIdx + 1) % numApprovalChoices
		return m, nil
	case "enter":
		return m.chooseApproval(m.approvalIdx)
	case "1", "y":
		return m.chooseApproval(choiceYes)
	case "2", "a":
		return m.chooseApproval(choiceAlways)
	case "3":
		return m.chooseApproval(choiceNoFeedback)
	case "n", "esc":
		return m.answerApproval(ApprovalDecision{Approved: false})
	}
	return m, nil
}

func (m Model) chooseApproval(c approvalChoice) (tea.Model, tea.Cmd) {
	switch c {
	case choiceYes:
		return m.answerApproval(ApprovalDecision{Approved: true})
	case choiceAlways:
		return m.answerApproval(ApprovalDecision{Approved: true, AlwaysAllow: true})
	case choiceNoFeedback:
		m.approvalFeedback = true
		m.textarea.Reset()
		m.textarea.Placeholder = "Tell " + app.Current().DisplayName + " what to do instead…"
		m.textarea.Focus()
		m = m.handleResize()
		return m, nil
	}
	return m, nil
}

// answerApproval records the decision in the transcript, closes the dialog,
// unblocks the agent goroutine and resumes the stream pump.
func (m Model) answerApproval(d ApprovalDecision) (tea.Model, tea.Cmd) {
	ap := m.pendingApproval
	if ap == nil {
		return m, nil
	}
	name := ap.ToolName
	var note string
	switch {
	case d.Approved && d.AlwaysAllow:
		scope := ap.AlwaysLabel
		if scope == "" {
			scope = "this exact call"
		}
		note = "✓ " + name + " allowed (won't ask again for " + scope + " this session)"
	case d.Approved:
		// Plain yes: the call's own row shows it running; no note needed.
	case d.Feedback != "":
		note = "✗ " + name + " declined: " + d.Feedback
	default:
		note = "✗ " + name + " declined"
	}
	if note != "" {
		// Under the call's row, not as a message of its own: a message here
		// would end the response and start a new one for the next events.
		if i := m.streamingAssistant(); i >= 0 {
			m.messages[i].addToolEvent(ToolEvent{Kind: ToolNote, Name: name, Summary: note})
		} else {
			m.messages = append(m.messages, newSystemMessage("  "+note))
		}
	}

	m.pendingApproval = nil
	m.approvalIdx = 0
	if m.approvalFeedback {
		m.approvalFeedback = false
		m.textarea.Placeholder = app.Current().Placeholder
	}
	m = m.handleResize()

	ch, stop, replyCh := m.streamCh, m.streamStop, ap.ReplyCh
	return m, func() tea.Msg {
		replyCh <- d // unblocks the agent goroutine
		return waitForStreamEvent(ch, stop)()
	}
}

// renderApproval draws the permission dialog.
func (m Model) renderApproval() string {
	th := m.theme
	ap := m.pendingApproval
	width := max(m.width-4, 20)
	inner := width - 4 // border + padding

	title, preview := approvalPreview(ap.ToolName, ap.Args, inner, m.previewLineBudget(), th)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(th.Warning).Bold(true).Render("Permission required"))
	b.WriteString(th.ToolDim.Render("  ·  "))
	b.WriteString(th.ToolName.Render(ap.ToolName))
	b.WriteByte('\n')
	b.WriteString(title)
	b.WriteByte('\n')
	if preview != "" {
		b.WriteString(preview)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	if m.approvalFeedback {
		b.WriteString(th.ToolDim.Render("Type what to do instead below  ·  Enter send  ·  Esc decline without a note"))
	} else {
		for c := approvalChoice(0); c < numApprovalChoices; c++ {
			label := fmt.Sprintf("%d. %s", c+1, c.label(ap.AlwaysLabel))
			if c == m.approvalIdx {
				b.WriteString(lipgloss.NewStyle().Foreground(th.AccentBold).Bold(true).Render("❯ " + label))
			} else {
				b.WriteString("  " + lipgloss.NewStyle().Foreground(th.Foreground).Render(label))
			}
			b.WriteByte('\n')
		}
		b.WriteString(th.ToolDim.Render("↑↓ select  ·  Enter confirm  ·  1-3 / y a n  ·  Esc decline"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Warning).
		Padding(0, 1).
		Width(width).
		Render(b.String())
}

// approvalPreview returns a one-line description of what the call will do and
// a multi-line preview (full command, diff, or file content), both styled.
func approvalPreview(toolName, argsJSON string, width, maxLines int, th Theme) (title, preview string) {
	var args map[string]any
	_ = json.Unmarshal([]byte(argsJSON), &args)
	str := func(k string) string { s, _ := args[k].(string); return s }
	dim := th.ToolDim
	code := lipgloss.NewStyle().Foreground(th.Foreground)

	switch toolName {
	case "bash":
		return dim.Render("Run shell command"), indentBlock(capLines(wrapText(str("command"), width-2), maxLines, dim), code, "  ")
	case "write_file":
		content := str("content")
		n := strings.Count(content, "\n")
		if content != "" && !strings.HasSuffix(content, "\n") {
			n++
		}
		return dim.Render("Write ") + th.ToolArg.Render(str("path")) + dim.Render(fmt.Sprintf("  (%d lines)", n)),
			indentBlock(capLines(truncLines(content, width-2), maxLines, dim), dim, "  ")
	case "edit_file":
		return dim.Render("Edit ") + th.ToolArg.Render(str("path")),
			diffPreview([][2]string{{str("old_string"), str("new_string")}}, width, maxLines, th)
	case "patch_file":
		var pairs [][2]string
		if edits, ok := args["edits"].([]any); ok {
			for _, e := range edits {
				if em, ok := e.(map[string]any); ok {
					o, _ := em["old_string"].(string)
					n, _ := em["new_string"].(string)
					pairs = append(pairs, [2]string{o, n})
				}
			}
		}
		return dim.Render("Patch ") + th.ToolArg.Render(str("path")) + dim.Render(fmt.Sprintf("  (%d edits)", len(pairs))),
			diffPreview(pairs, width, maxLines, th)
	case "web_fetch":
		return dim.Render("Fetch ") + th.ToolArg.Render(str("url")), ""
	case "edit_notebook":
		return dim.Render("Edit notebook ") + th.ToolArg.Render(str("path")), ""
	}

	// MCP, plugin and pack tools: show the arguments, pretty-printed.
	if len(args) == 0 {
		return dim.Render("Call tool (no arguments)"), ""
	}
	pretty, err := json.MarshalIndent(args, "", "  ")
	if err != nil {
		pretty = []byte(argsJSON)
	}
	return dim.Render("Call tool with:"), indentBlock(capLines(truncLines(string(pretty), width-2), maxLines, dim), code, "  ")
}

// diffPreview renders old/new string pairs as -/+ lines, capped overall.
func diffPreview(pairs [][2]string, width, maxLines int, th Theme) string {
	del := lipgloss.NewStyle().Foreground(th.ErrorColor)
	add := lipgloss.NewStyle().Foreground(th.Success)
	var lines []string
	for i, p := range pairs {
		if i > 0 {
			lines = append(lines, th.ToolDim.Render("  ⋯"))
		}
		for _, l := range strings.Split(strings.TrimSuffix(p[0], "\n"), "\n") {
			lines = append(lines, del.Render("- "+truncate(l, width-4)))
		}
		for _, l := range strings.Split(strings.TrimSuffix(p[1], "\n"), "\n") {
			lines = append(lines, add.Render("+ "+truncate(l, width-4)))
		}
	}
	if len(lines) > maxLines {
		more := len(lines) - maxLines
		lines = append(lines[:maxLines:maxLines], th.ToolDim.Render(fmt.Sprintf("  … %d more lines", more)))
	}
	return "  " + strings.Join(lines, "\n  ")
}

// wrapText hard-wraps s to width runes per line (commands can be long and
// must be shown in full, not truncated).
func wrapText(s string, width int) []string {
	width = max(width, 10)
	var out []string
	for _, line := range strings.Split(s, "\n") {
		r := []rune(line)
		for len(r) > width {
			out = append(out, string(r[:width]))
			r = r[width:]
		}
		out = append(out, string(r))
	}
	return out
}

// truncLines splits s into lines, truncating each to width runes.
func truncLines(s string, width int) []string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = truncate(l, width)
	}
	return lines
}

func truncate(s string, width int) string {
	r := []rune(s)
	if width < 2 || len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}

// capLines keeps at most n lines, replacing the rest with a dim summary line.
func capLines(lines []string, n int, dim lipgloss.Style) []string {
	if len(lines) <= n {
		return lines
	}
	more := len(lines) - n
	return append(lines[:n:n], dim.Render(fmt.Sprintf("… %d more lines", more)))
}

// indentBlock styles and indents each line.
func indentBlock(lines []string, style lipgloss.Style, indent string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.HasPrefix(l, "\x1b") { // already styled (summary line)
			out[i] = indent + l
			continue
		}
		out[i] = indent + style.Render(l)
	}
	return strings.Join(out, "\n")
}
