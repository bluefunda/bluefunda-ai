package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds every color and style used in the TUI so all rendering goes
// through a single source of truth. Claude Code dark palette.
type Theme struct {
	// Base colors
	Background lipgloss.Color
	Foreground lipgloss.Color
	Secondary  lipgloss.Color // dimmed text
	Muted      lipgloss.Color // very dim, borders
	Accent     lipgloss.Color // muted blue highlight
	AccentBold lipgloss.Color // brighter blue for active elements
	Success    lipgloss.Color
	Warning    lipgloss.Color
	ErrorColor lipgloss.Color
	ToolColor  lipgloss.Color // purple-ish for tool calls

	// Composed styles
	UserLabel      lipgloss.Style
	UserContent    lipgloss.Style
	AssistantLabel lipgloss.Style
	AssistantText  lipgloss.Style
	SystemMsg      lipgloss.Style

	// Tool rendering
	ToolIcon    lipgloss.Style
	ToolName    lipgloss.Style
	ToolArg     lipgloss.Style
	ToolSuccess lipgloss.Style
	ToolError   lipgloss.Style
	ToolDim     lipgloss.Style

	// Code blocks
	CodeBg    lipgloss.Color
	CodeLang  lipgloss.Style
	CodeBlock lipgloss.Style

	// Input area
	InputBorder   lipgloss.Style
	InputPrompt   lipgloss.Style
	InputHint     lipgloss.Style
	SlashMenu     lipgloss.Style
	SlashSelected lipgloss.Style

	// Header / footer
	Header  lipgloss.Style
	Footer  lipgloss.Style
	Divider lipgloss.Style

	// Spinner
	Spinner lipgloss.Style
}

// DefaultTheme returns the Claude Code–inspired dark theme.
func DefaultTheme() Theme {
	bg := lipgloss.Color("#0d0d0d")
	fg := lipgloss.Color("#e8e8e8")
	secondary := lipgloss.Color("#888888")
	muted := lipgloss.Color("#3a3a3a")
	accent := lipgloss.Color("#4a9eff")
	accentBold := lipgloss.Color("#6ab8ff")
	success := lipgloss.Color("#4caf72")
	warning := lipgloss.Color("#d4a54a")
	errColor := lipgloss.Color("#e05c5c")
	toolColor := lipgloss.Color("#9d7cd8")
	codeBg := lipgloss.Color("#1a1a1a")

	t := Theme{
		Background: bg,
		Foreground: fg,
		Secondary:  secondary,
		Muted:      muted,
		Accent:     accent,
		AccentBold: accentBold,
		Success:    success,
		Warning:    warning,
		ErrorColor: errColor,
		ToolColor:  toolColor,
		CodeBg:     codeBg,
	}

	t.UserLabel = lipgloss.NewStyle().
		Foreground(accentBold).
		Bold(true)

	t.UserContent = lipgloss.NewStyle().
		Foreground(fg).
		PaddingLeft(2)

	t.AssistantLabel = lipgloss.NewStyle().
		Foreground(success).
		Bold(true)

	t.AssistantText = lipgloss.NewStyle().
		Foreground(fg)

	t.SystemMsg = lipgloss.NewStyle().
		Foreground(secondary).
		Italic(true)

	t.ToolIcon = lipgloss.NewStyle().
		Foreground(toolColor)

	t.ToolName = lipgloss.NewStyle().
		Foreground(secondary)

	t.ToolArg = lipgloss.NewStyle().
		Foreground(muted)

	t.ToolSuccess = lipgloss.NewStyle().
		Foreground(success)

	t.ToolError = lipgloss.NewStyle().
		Foreground(errColor)

	t.ToolDim = lipgloss.NewStyle().
		Foreground(secondary)

	t.CodeBg = codeBg
	t.CodeLang = lipgloss.NewStyle().
		Foreground(secondary).
		Background(codeBg).
		PaddingLeft(1).
		PaddingRight(1)

	t.CodeBlock = lipgloss.NewStyle().
		Background(codeBg).
		Foreground(fg).
		PaddingLeft(2).
		PaddingRight(2).
		PaddingTop(0).
		PaddingBottom(0)

	t.InputBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(muted).
		PaddingLeft(1).
		PaddingRight(1)

	t.InputPrompt = lipgloss.NewStyle().
		Foreground(secondary)

	t.InputHint = lipgloss.NewStyle().
		Foreground(muted)

	t.SlashMenu = lipgloss.NewStyle().
		Background(lipgloss.Color("#1e1e1e")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(muted).
		PaddingLeft(1).
		PaddingRight(1)

	t.SlashSelected = lipgloss.NewStyle().
		Background(lipgloss.Color("#2a2a2a")).
		Foreground(accentBold)

	t.Header = lipgloss.NewStyle().
		Foreground(secondary).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(muted)

	t.Footer = lipgloss.NewStyle().
		Foreground(muted)

	t.Divider = lipgloss.NewStyle().
		Foreground(muted)

	t.Spinner = lipgloss.NewStyle().
		Foreground(accent)

	return t
}
