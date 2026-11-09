package output

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/bluefunda/abaper/styles"
)

// OK prints a success line prefixed with a colored [OK] tag.
func OK(format string, args ...any) {
	printTag("[OK]", styles.ColorSuccess, format, args...)
}

// Err prints a failure line prefixed with a colored [ERROR] tag.
func Err(format string, args ...any) {
	printTag("[ERROR]", styles.ColorError, format, args...)
}

// Info prints an informational line prefixed with a colored [INFO] tag.
func Info(format string, args ...any) {
	printTag("[INFO]", styles.ColorInfo, format, args...)
}

// Warn prints a cautionary line prefixed with a colored [WARN] tag.
func Warn(format string, args ...any) {
	printTag("[WARN]", styles.ColorWarning, format, args...)
}

func printTag(tag string, color lipgloss.TerminalColor, format string, args ...any) {
	styledTag := lipgloss.NewStyle().Foreground(color).Bold(true).Render(tag)
	fmt.Printf(styledTag+" "+format+"\n", args...)
}
