package cmd

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"github.com/muesli/termenv"
)

func TestNoColorFlag_DisablesColorOutput(t *testing.T) {
	origFlag, origNoColor := rootNoColor, color.NoColor
	defer func() { rootNoColor, color.NoColor = origFlag, origNoColor }()

	rootNoColor = true
	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err != nil {
		t.Fatalf("PersistentPreRunE: %v", err)
	}

	if !color.NoColor {
		t.Error("color.NoColor = false, want true after --no-color")
	}
	if got := lipgloss.ColorProfile(); got != termenv.Ascii {
		t.Errorf("lipgloss.ColorProfile() = %v, want termenv.Ascii", got)
	}
}
