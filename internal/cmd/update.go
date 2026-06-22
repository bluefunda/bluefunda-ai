package cmd

import (
	"os/exec"
	"runtime"

	"github.com/bluefunda/go-update"
	"github.com/spf13/cobra"
)

const homebrewTap = "bluefunda/tap"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update bai to the latest version",
	Long: `Check for a newer release of bai and upgrade automatically.

The installation method (Homebrew, dpkg, rpm, or standalone binary) is
detected from the current executable path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Homebrew >= 4.x requires taps to be explicitly trusted before
		// formulas can be loaded. Trust the tap silently; errors are
		// ignored (already trusted, or brew not on PATH on Linux).
		if runtime.GOOS == "darwin" {
			_ = exec.Command("brew", "trust", homebrewTap).Run()
		}

		return update.Run(update.Config{
			BinaryName:     "bai",
			CurrentVersion: Version,
			GitHubOwner:    "bluefunda",
			GitHubRepo:     "bluefunda-ai",
			HomebrewCask:   "bai",
		})
	},
}
