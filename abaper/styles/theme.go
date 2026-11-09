// Package styles holds abaper's TUI/CLI color palette and shared lipgloss
// styles — the same dark palette as bluefunda-ai (bai), so both CLIs share
// a visual identity.
package styles

import "github.com/charmbracelet/lipgloss"

var (
	ColorBg         = lipgloss.AdaptiveColor{Dark: "#0d0d0d", Light: "#f5f5f0"}
	ColorFg         = lipgloss.AdaptiveColor{Dark: "#e8e8e8", Light: "#1a1a1a"}
	ColorMuted      = lipgloss.AdaptiveColor{Dark: "#888888", Light: "#999999"}
	ColorBorder     = lipgloss.AdaptiveColor{Dark: "#3a3a3a", Light: "#dddddd"}
	ColorAccent     = lipgloss.Color("#4a9eff")
	ColorAccentSoft = lipgloss.Color("#6ab8ff")
	ColorSuccess    = lipgloss.Color("#4caf72")
	ColorWarning    = lipgloss.Color("#d4a54a")
	ColorError      = lipgloss.Color("#e05c5c")
	ColorInfo       = lipgloss.Color("#4a9eff")
	ColorTool       = lipgloss.Color("#9d7cd8")
	ColorDiffAdd    = lipgloss.Color("#1e4d2b")
	ColorDiffAddFg  = lipgloss.Color("#4caf72")
	ColorDiffDel    = lipgloss.Color("#4d1e1e")
	ColorDiffDelFg  = lipgloss.Color("#e05c5c")
	ColorCodeBg     = lipgloss.AdaptiveColor{Dark: "#1a1a1a", Light: "#efefef"}
	ColorCodeBorder = lipgloss.Color("#4a9eff")
)

var (
	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	StyleAccentBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorAccent)

	StyleCode = lipgloss.NewStyle().
			Background(ColorCodeBg).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeftForeground(ColorCodeBorder).
			PaddingLeft(1)

	// StyleUserMsg labels the human turn — bold blue, mirroring bai's UserLabel.
	StyleUserMsg = lipgloss.NewStyle().
			Foreground(ColorAccentSoft).
			Bold(true)

	// StyleAssistantLabel labels the assistant turn — bold green, mirroring
	// bai's AssistantLabel (deliberately not the accent color).
	StyleAssistantLabel = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleTool    = lipgloss.NewStyle().Foreground(ColorTool)

	// StyleHeader is the TUI's top bar: bottom-border-only, like bai's header.
	StyleHeader = lipgloss.NewStyle().
			Foreground(ColorMuted).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder)

	// StyleInputBorder wraps the chat input textarea in a rounded box, like
	// bai's input area.
	StyleInputBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)
)
