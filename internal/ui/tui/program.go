package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the BubbleTea program and blocks until the user exits.
func Run(m Model) error {
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui run: %w", err)
	}

	if fm, ok := finalModel.(Model); ok && len(fm.messages) > 1 {
		fmt.Fprintln(os.Stdout, "")
	}
	return nil
}
