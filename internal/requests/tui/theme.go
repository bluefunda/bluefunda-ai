package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Brand palette — matches the web app (sidebar #1b1e37, primary #1e64e7).
var (
	colPrimary  = lipgloss.Color("#1e64e7")
	colNavy     = lipgloss.Color("#1b1e37")
	colText     = lipgloss.Color("#e2e8f0")
	colMuted    = lipgloss.Color("#8892a4")
	colBorder   = lipgloss.Color("#2d3748")
	colSelected = lipgloss.Color("#1e3a5f")
	colGreen    = lipgloss.Color("#22c55e")
	colRed      = lipgloss.Color("#ef4444")

	// Severity — from web app CSS variables
	colSevLow      = lipgloss.Color("#f7aa16")
	colSevMedium   = lipgloss.Color("#f76900")
	colSevHigh     = lipgloss.Color("#ee1b1b")
	colSevCritical = lipgloss.Color("#ff4444")

	// Status — derived from web app status tag palette
	colStatusPlanned  = lipgloss.Color("#92d0e2")
	colStatusProgress = lipgloss.Color("#fa9d3f")
	colStatusQA       = lipgloss.Color("#a855f7")
	colStatusUAT      = lipgloss.Color("#3b82f6")
	colStatusRelease  = lipgloss.Color("#8b5cf6")
	colStatusDone     = lipgloss.Color("#22c55e")
)

var (
	sHeader = lipgloss.NewStyle().
		Background(colNavy).
		Foreground(colText).
		Padding(0, 2)

	sHeaderTitle = lipgloss.NewStyle().
			Foreground(colPrimary).
			Bold(true)

	sFooter = lipgloss.NewStyle().
		Background(colNavy).
		Foreground(colMuted).
		Padding(0, 2)

	sKeyName = lipgloss.NewStyle().
			Foreground(colPrimary).
			Bold(true)

	sColHeader = lipgloss.NewStyle().
			Foreground(colMuted).
			Bold(true)

	sRowSelected = lipgloss.NewStyle().
			Background(colSelected).
			Foreground(colText)

	sRowNormal = lipgloss.NewStyle().
			Foreground(colText)

	sMuted = lipgloss.NewStyle().
		Foreground(colMuted)

	sError = lipgloss.NewStyle().
		Foreground(colRed)

	sInfo = lipgloss.NewStyle().
		Foreground(colPrimary)

	sStageDone    = lipgloss.NewStyle().Foreground(colGreen)
	sStageCurrent = lipgloss.NewStyle().Foreground(colPrimary).Bold(true)
	sStagePending = lipgloss.NewStyle().Foreground(colMuted)

	sDetailLabel = lipgloss.NewStyle().
			Foreground(colMuted).
			Width(12)

	sDetailValue = lipgloss.NewStyle().
			Foreground(colText)

	sCommentAuthor = lipgloss.NewStyle().
			Foreground(colPrimary).
			Bold(true)

	sCommentMeta = lipgloss.NewStyle().
			Foreground(colMuted)

	sDivider = lipgloss.NewStyle().
			Foreground(colBorder)

	sPanelTitle = lipgloss.NewStyle().
			Foreground(colText).
			Bold(true).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colBorder)

	sFilterActive = lipgloss.NewStyle().
			Foreground(colPrimary).
			Bold(true)

	sFilterInactive = lipgloss.NewStyle().
			Foreground(colMuted)

	sTabActive = lipgloss.NewStyle().
			Foreground(colPrimary).
			Bold(true).
			Underline(true)

	sTabInactive = lipgloss.NewStyle().
			Foreground(colMuted)
)

// SeverityBadge returns a coloured severity string.
func SeverityBadge(s string) string {
	var c lipgloss.Color
	switch s {
	case "low":
		c = colSevLow
	case "medium":
		c = colSevMedium
	case "high":
		c = colSevHigh
	case "critical":
		c = colSevCritical
	default:
		c = colMuted
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(s)
}

// StatusBadge returns a coloured status string.
func StatusBadge(s string) string {
	var c lipgloss.Color
	switch strings.ToLower(s) {
	case "planned":
		c = colStatusPlanned
	case "inprogress", "in_progress":
		c = colStatusProgress
	case "completed", "imported":
		c = colStatusDone
	case "qa", "toc_importing_to_qa", "toc_imported_to_qa":
		c = colStatusQA
	case "uat", "uat_approved":
		c = colStatusUAT
	case "release_management", "toc_release_initiated", "toc_released",
		"transport_release_from_dev", "transport_released",
		"import_to_prod", "schedule_import", "import_is_scheduled":
		c = colStatusRelease
	default:
		c = colMuted
	}
	return lipgloss.NewStyle().Foreground(c).Render(s)
}
