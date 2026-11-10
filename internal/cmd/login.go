package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/auth"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var loginRealm string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in via browser (opens Keycloak)",
	RunE:  runLogin,
}

func init() {
	// ExecuteABAPer rebuilds this after switching the app identity, so the
	// help always names the binary being run.
	loginCmd.Long = loginLongHelp()
	loginCmd.Flags().StringVar(&loginRealm, "realm", config.DefaultRealm, "Keycloak realm to authenticate against")
}

func loginLongHelp() string {
	return fmt.Sprintf(`Log in via browser using the Keycloak device authorization flow.

By default, authenticates against the %q realm.
Use --realm to authenticate against a different realm:

  %s login --realm <name>`, config.DefaultRealm, app.Current().Name)
}

func runLogin(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	realm := loginRealm
	if realm == "" {
		realm = config.DefaultRealm
	}

	tok, err := auth.LoginWithDevice(cfg.EffectiveDomain(), realm)
	if err != nil {
		ui.Error("Login failed: " + err.Error())
		return err
	}

	cfg.Realm = realm

	if err := saveAuthTokens(cfg, tok); err != nil {
		ui.Error("Failed to save config: " + err.Error())
		return err
	}

	ui.Success("Logged in successfully (realm: " + realm + ")")
	return nil
}
