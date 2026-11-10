package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

// baiRequestsIdentity is breq's identity as mounted under `bai requests`:
// same config dir, keychain service, realm and BFF — so `breq login` and
// `bai requests login` are one TRM sign-in — but hints name `bai requests`.
func baiRequestsIdentity() app.Identity {
	id := Breq
	id.Name = "bai requests"
	return id
}

// requestsCmd is `bai requests`: the breq command set inside bai. TRM uses
// its own Keycloak realm and BFF, so the process switches to the breq
// identity for the command and switches back before bai's post-run hooks.
var requestsCmd = &cobra.Command{
	Use:     "requests",
	Aliases: []string{"breq"},
	Short:   "BlueRequests change and release management (same as the breq CLI)",
	Long: `Change requests, events, A4H sessions and more on the BlueRequests (TRM)
platform — the same commands as the breq CLI.

BlueRequests uses its own sign-in (realm "trm"), shared with breq:
  bai requests login`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
			return err
		}
		app.Set(baiRequestsIdentity())
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		app.Set(app.Bai)
		return rootCmd.PersistentPostRunE(cmd, args)
	},
}

func init() {
	login := &cobra.Command{
		Use:   "login",
		Short: "Sign in to BlueRequests (shared with `breq login`)",
		Args:  cobra.NoArgs,
		RunE:  runLogin,
	}
	login.Flags().StringVar(&loginRealm, "realm", "", "Keycloak realm (default: trm)")
	logout := &cobra.Command{
		Use:   "logout",
		Short: "Sign out of BlueRequests (shared with `breq logout`)",
		Args:  cobra.NoArgs,
		RunE:  runLogout,
	}
	requestsCmd.AddCommand(login, logout)
	requestsCmd.AddCommand(NewRequestsCommands()...)
}
