package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/requests"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/updatecheck"
)

// Breq is the product identity of the breq (BlueRequests) binary. It keeps
// breq's config directory (~/.breq, so existing logins keep working),
// Keycloak realm and client, and its own BFF.
var Breq = app.Identity{
	Name:            "breq",
	DisplayName:     "BlueRequests",
	Dir:             ".breq",
	EnvPrefix:       "BREQ_",
	ClientID:        "bluerequests",
	KeychainService: "breq",
	HomebrewCask:    "breq",
	BFFURL:          "requests.bluefunda.com:443",
	Realm:           "trm",
}

// ExecuteBREQ runs the breq binary: the BlueRequests command set at the
// root plus the platform's login/logout/version/update.
func ExecuteBREQ() error {
	app.Set(Breq)
	loginCmd.Long = loginLongHelp()
	return newBreqRoot().Execute()
}

func newBreqRoot() *cobra.Command {
	root := &cobra.Command{
		Use:          "breq",
		Short:        "breq — CLI for the BlueRequests change and release management platform",
		Long:         "breq is a command-line interface for the BlueRequests (TRM) platform.",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			applyGlobalFlags()
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			updatecheck.MaybeNotify(Version, outputFormat(cfg) == ui.FormatQuiet, cfg.UpdateCheckEnabled())
			return nil
		},
	}
	root.PersistentFlags().StringVar(&cfgBFF, "bff", "", "BFF gRPC address host:port (overrides config)")
	root.PersistentFlags().StringVar(&cfgDomain, "domain", "", "Auth domain (overrides config)")
	root.PersistentFlags().StringVarP(&cfgOutput, "output", "o", "", "Output format: table, json, quiet")
	_ = root.RegisterFlagCompletionFunc("output", staticValues("table", "json", "quiet"))
	root.PersistentFlags().BoolVar(&rootNoColor, "no-color", false, "Disable colored output")
	_ = root.PersistentFlags().MarkHidden("no-color")

	root.AddCommand(loginCmd, logoutCmd, versionCmd, updateCmd, completionCmd)
	root.AddCommand(NewRequestsCommands()...)
	return root
}

// NewRequestsCommands returns the BlueRequests commands, built fresh so they
// can mount at the breq root and under `bai requests`.
func NewRequestsCommands() []*cobra.Command {
	return []*cobra.Command{
		newRequestsCRCmd(),
		newRequestsEventsCmd(),
		newRequestsRPCCmd(),
		newRequestsUserCmd(),
		newRequestsA4HCmd(),
		newRequestsHealthCmd(),
	}
}

// requestsConn opens an authenticated connection to the TRM BFF, using the
// running identity's config and login. Caller must Close it.
func requestsConn() (*requests.Conn, *config.Config, error) {
	conn, cfg, err := bffConn()
	if err != nil {
		return nil, cfg, err
	}
	return requests.Wrap(conn), cfg, nil
}

func newRequestsHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check connectivity to the BlueRequests BFF",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			target := cfg.EffectiveBFFURL()
			if err := caigrpc.Ping(target); err != nil {
				return fmt.Errorf("BFF unhealthy at %s: %w", target, err)
			}
			printer(cfg).Success("BFF reachable at " + target)
			return nil
		},
	}
}
