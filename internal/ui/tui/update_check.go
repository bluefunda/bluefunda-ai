package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bluefunda/bluefunda-ai/internal/updatecheck"
)

// UpdateAvailableMsg is sent when a newer bai version is found on GitHub.
type UpdateAvailableMsg struct{ Version string }

// checkForUpdateCmd fires a background goroutine that consults
// updatecheck.Check (24h-cached, see internal/updatecheck for #287). It sends
// UpdateAvailableMsg if a newer version is found, or returns nil (silently
// ignored by BubbleTea) on any error, when already up-to-date, or when
// disabled is true (the resolved update.check config value — see
// SessionConfig.DisableUpdateCheck).
func checkForUpdateCmd(current string, disabled bool) tea.Cmd {
	if disabled || current == "" || current == "dev" {
		return nil
	}
	return func() tea.Msg {
		if latest, available := updatecheck.Check(current); available {
			return UpdateAvailableMsg{Version: latest}
		}
		return nil
	}
}
