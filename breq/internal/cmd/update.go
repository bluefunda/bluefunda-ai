package cmd

import (
	"github.com/bluefunda/go-update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update breq to the latest version",
	Long: `Check for a newer release of breq and upgrade automatically.

The installation method (Homebrew, dpkg, rpm, or standalone binary) is
detected from the current executable path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return update.Run(update.Config{
			BinaryName:     "breq",
			CurrentVersion: Version,
			GitHubOwner:    "bluefunda",
			GitHubRepo:     "bluerequests",
			HomebrewCask:   "breq",
		})
	},
}
