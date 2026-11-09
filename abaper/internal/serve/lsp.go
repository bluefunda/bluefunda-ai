package serve

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/abaper/internal/adt"
	"github.com/bluefunda/bluefunda-ai/abaper/internal/lsp/backend"
	"github.com/bluefunda/bluefunda-ai/abaper/lsp"
	"github.com/bluefunda/bluefunda-ai/abaper/types"
	"github.com/bluefunda/bluefunda-ai/internal/abap"
)

// NewLSPCommand returns the `lsp` command: the ABAP language server used by
// editor integrations (editors/zed runs `abaper lsp [--offline]`).
func NewLSPCommand() *cobra.Command {
	var (
		offline         bool
		tcp             string
		systemName      string
		activateOnSave  bool
		allowSelfSigned bool
	)
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Run the ABAP language server (stdio by default)",
		Long: `Runs the ABAP Language Server Protocol server for editors.

Offline mode offers keyword completion and abaplint diagnostics. Otherwise
the active SAP system (or --system) adds live syntax checks and activation.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := os.Getwd()
			if err != nil {
				return err
			}
			cfg := &lsp.Config{OfflineOnly: offline, ActivateOnSave: activateOnSave}

			var b backend.LSPBackend = backend.NewOfflineBackend(workDir)
			if !offline {
				sys, err := lspSystem(systemName)
				if err != nil {
					return err
				}
				if sys != nil {
					cfg.ADT = &types.ADTConfig{
						Host:            sys.Host,
						Client:          sys.Client,
						Username:        sys.Username,
						Password:        sys.Password,
						Language:        "EN",
						AllowSelfSigned: allowSelfSigned,
					}
					client := adt.NewADTClient(cfg.ADT)
					// A failed logon leaves the hybrid backend in offline
					// mode rather than refusing to start the editor's server.
					_ = client.Authenticate()
					b = backend.NewHybridBackend(context.Background(), client, workDir)
				}
			}

			srv := lsp.NewServer(cfg, b)
			if tcp != "" {
				return srv.RunTCP(tcp)
			}
			return srv.RunStdio()
		},
	}
	cmd.Flags().BoolVar(&offline, "offline", false, "Run without an SAP connection")
	cmd.Flags().StringVar(&tcp, "tcp", "", "Listen on a TCP address (e.g. :2087) instead of stdio")
	cmd.Flags().StringVar(&systemName, "system", "", "SAP system name or ID (default: the active system)")
	cmd.Flags().BoolVar(&activateOnSave, "activate-on-save", false, "Activate objects after save")
	cmd.Flags().BoolVar(&allowSelfSigned, "allow-self-signed", false, "Allow self-signed TLS certificates")
	return cmd
}

// lspSystem returns the named system, else the active one; nil when none is configured.
func lspSystem(name string) (*abap.SAPSystem, error) {
	reg, err := abap.LoadSystems()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return reg.ActiveSystem(), nil
	}
	sys := reg.Find(name)
	if sys == nil {
		return nil, fmt.Errorf("no SAP system named %q", name)
	}
	return sys, nil
}
