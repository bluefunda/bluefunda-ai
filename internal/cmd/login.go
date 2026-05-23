package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/auth"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var loginRealm string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in via browser (opens Keycloak)",
	Long: `Log in via browser using the Keycloak device authorization flow.

By default, authenticates against the "trm" realm.
Use --realm to authenticate against a different realm:

  ai login --realm individual`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&loginRealm, "realm", config.DefaultRealm,
		"Keycloak realm to authenticate against (e.g. individual, trm)")
}

func runLogin(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()

	tok, err := auth.LoginWithDevice(cfg.Domain, loginRealm)
	if err != nil {
		ui.Error("Login failed: " + err.Error())
		return err
	}

	cfg.Realm = loginRealm

	if err := saveAuthTokens(cfg, tok); err != nil {
		ui.Error("Failed to save config: " + err.Error())
		return err
	}

	ui.Success("Logged in successfully (realm: " + loginRealm + ")")
	return nil
}
