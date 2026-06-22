package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/keychain"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove stored credentials",
	RunE:  runLogout,
}

func runLogout(_ *cobra.Command, _ []string) error {
	cfg := loadConfig()
	cfg.ClearTokens()

	// Remove tokens from the OS keychain when available.
	if keychain.Available() {
		_ = keychain.Delete("access_token")
		_ = keychain.Delete("refresh_token")
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	ui.Success("Logged out successfully.")
	return nil
}
