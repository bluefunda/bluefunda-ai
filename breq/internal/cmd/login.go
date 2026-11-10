package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluerequests/internal/auth"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the TRM platform (OAuth2 device flow)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		p := printer(cfg)
		p.Info(fmt.Sprintf("Authenticating at %s (realm: %s)", cfg.Domain, cfg.Realm))

		tok, err := auth.LoginWithDevice(cfg.Domain, cfg.Realm)
		if err != nil {
			return fmt.Errorf("login: %w", err)
		}

		if err := saveAuthTokens(cfg, tok); err != nil {
			return fmt.Errorf("save tokens: %w", err)
		}

		p.Success("Logged in successfully.")
		return nil
	},
}
