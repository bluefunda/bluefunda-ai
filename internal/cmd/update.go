package cmd

import (
	"github.com/bluefunda/go-update"
	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update to the latest version",
	Long: `Check for a newer release and upgrade automatically.

The installation method (Homebrew, dpkg, rpm, or standalone binary) is
detected from the current executable path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return update.Run(update.Config{
			BinaryName:     app.Current().Name,
			CurrentVersion: Version,
			GitHubOwner:    "bluefunda",
			GitHubRepo:     "bluefunda-ai",
			HomebrewTap:    "bluefunda/tap",
			HomebrewCask:   app.Current().HomebrewCask,
		})
	},
}
