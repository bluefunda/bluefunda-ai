package cmd

import (
	"github.com/bluefunda/go-update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update bai to the latest version",
	Long: `Check for a newer release of bai and upgrade automatically.

The installation method (Homebrew, dpkg, rpm, or standalone binary) is
detected from the current executable path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return update.Run(update.Config{
			BinaryName:     "bai",
			CurrentVersion: Version,
			GitHubOwner:    "bluefunda",
			GitHubRepo:     "bluefunda-ai",
			HomebrewCask:   "bai",
		})
	},
}
