package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// ANSI escape sequences for cursor shape (DECSCUSR).
// \x1b[5 q = blinking bar (|), \x1b[0 q = terminal default (usually block).
const (
	cursorBar     = "\x1b[5 q"
	cursorDefault = "\x1b[0 q"
)

// Run starts the BubbleTea program and blocks until the user exits.
func Run(m Model) error {
	// Switch to a blinking bar cursor for the session; restore on exit.
	fmt.Fprint(os.Stdout, cursorBar)
	defer fmt.Fprint(os.Stdout, cursorDefault)

	// WithMouseCellMotion passes mouse wheel events to the viewport bubble,
	// enabling scroll-by-wheel without requiring keyboard-only navigation.
	p := tea.NewProgram(m, tea.WithMouseCellMotion())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui run: %w", err)
	}

	if fm, ok := finalModel.(Model); ok && len(fm.messages) > 1 {
		fmt.Fprintln(os.Stdout, "")
	}
	return nil
}
