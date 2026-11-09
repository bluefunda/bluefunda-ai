package commands

import (
	"github.com/bluefunda/go-update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update abaper to the latest version",
	Long: `Check for a newer release of abaper and upgrade automatically.

The installation method (Homebrew, dpkg, rpm, or standalone binary) is
detected from the current executable path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return update.Run(update.Config{
			BinaryName:     "abaper",
			CurrentVersion: version,
			GitHubOwner:    "bluefunda",
			GitHubRepo:     "abaper",
			HomebrewCask:   "abaper",
		})
	},
}
