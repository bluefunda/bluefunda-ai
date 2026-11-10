package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/config"
)

// requestsTarget is the BlueRequests BFF for this process. The breq binary
// uses its own config (endpoint, --bff, BREQ_BFF). Inside bai, `bai requests`
// keeps bai's sign-in and talks to the BlueRequests BFF instead of bai's:
// BAI_REQUESTS_BFF, else breq's default.
func requestsTarget(cfg *config.Config) string {
	if app.Current().Name == Breq.Name {
		return cfg.EffectiveBFFURL()
	}
	if v := app.Getenv("REQUESTS_BFF"); v != "" {
		return v
	}
	return Breq.BFFURL
}

// requestsCmd is `bai requests`: the breq command set inside bai, signed in
// with the bai session — `bai login` covers it.
var requestsCmd = &cobra.Command{
	Use:     "requests",
	Aliases: []string{"breq"},
	Short:   "BlueRequests change and release management (same as the breq CLI)",
	Long: `Change requests, events, A4H sessions and more on the BlueRequests (TRM)
platform — the same commands as the breq CLI.

Uses your bai sign-in; run ` + "`bai login`" + ` if you aren't signed in.`,
}

func init() {
	// Kept so `bai requests login` from the breq habit still works.
	login := &cobra.Command{
		Use:    "login",
		Short:  "Same as `bai login`",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE:   runLogin,
	}
	login.Flags().StringVar(&loginRealm, "realm", config.DefaultRealm, "Keycloak realm")
	logout := &cobra.Command{
		Use:    "logout",
		Short:  "Same as `bai logout`",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE:   runLogout,
	}
	requestsCmd.AddCommand(login, logout)
	requestsCmd.AddCommand(NewRequestsCommands()...)
}
