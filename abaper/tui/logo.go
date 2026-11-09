package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bluefunda/banner/pkg/banner"
	"github.com/bluefunda/abaper/styles"
)

func renderLogo(version, model string) string {
	// Always render with color inside the TUI.
	banner.NoColor = false

	bold  := lipgloss.NewStyle().Foreground(styles.ColorFg).Bold(true)
	muted := lipgloss.NewStyle().Foreground(styles.ColorMuted)

	// git describe produces "v1.5.1-2-g946a2df-dirty"; trim to just the tag.
	v := version
	if v == "" {
		v = "dev"
	}
	if idx := strings.IndexByte(v, '-'); idx > 0 {
		v = v[:idx] // strip "-N-gHASH-dirty" suffix
	}
	v = strings.TrimPrefix(v, "v") // we add our own "v" below

	info1 := bold.Render("ABAPer") + " " + muted.Render("v"+v)
	info2 := muted.Render(model)
	info3 := muted.Render(shortCWD())

	return banner.AbaperCompact(info1, info2, info3)
}

func shortCWD() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(cwd, home) {
		cwd = "~" + cwd[len(home):]
	}
	parts := strings.Split(filepath.ToSlash(cwd), "/")
	if len(parts) > 3 {
		cwd = "…/" + strings.Join(parts[len(parts)-2:], "/")
	}
	return cwd
}
