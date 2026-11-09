// Package serve is the `abaper serve` command: the ABAPer REST server over
// the direct ADT client (the abaper-ts replacement).
package serve

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/bluefunda/bluefunda-ai/abaper/internal/adt"
	"github.com/bluefunda/bluefunda-ai/abaper/rest/server"
	"github.com/bluefunda/bluefunda-ai/abaper/types"
	"github.com/bluefunda/bluefunda-ai/internal/abap"
)

// NewCommand returns the `serve` command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local ABAPer REST server (abaper-ts replacement)",
		Long: `Starts a local HTTP server that exposes the ADT SDK over REST,
mirroring the abaper-ts API surface. Useful for local development and
integration testing with abaper-editor.

The active SAP system from ~/.abaper/systems.json is used as the default
connection. Pass --system to select a different one. The server also accepts
per-request X-SAP-* headers for multi-system operation.`,
		RunE: runServe,
	}
	cmd.Flags().String("port", "8080", "Port to listen on")
	cmd.Flags().String("system", "", "SAP system name or ID to use (default: active system)")
	cmd.Flags().Bool("allow-self-signed", false, "Allow self-signed TLS certificates")
	return cmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetString("port")
	systemName, _ := cmd.Flags().GetString("system")
	allowSelfSigned, _ := cmd.Flags().GetBool("allow-self-signed")

	// Resolve SAP system from stored config.
	sysCfg, err := abap.LoadSystems()
	if err != nil {
		return fmt.Errorf("load systems: %w", err)
	}

	var sys *abap.SAPSystem
	if systemName != "" {
		sys = sysCfg.Find(systemName)
		if sys == nil {
			return fmt.Errorf("system %q not found — run: abaper system list", systemName)
		}
	} else {
		sys = sysCfg.ActiveSystem()
	}

	// Fall back to environment variables when no stored system is available.
	if sys == nil {
		host := os.Getenv("SAP_HOST")
		user := os.Getenv("SAP_USERNAME")
		pass := os.Getenv("SAP_PASSWORD")
		client := os.Getenv("SAP_CLIENT")
		if client == "" {
			client = "001"
		}
		if host != "" && user != "" && pass != "" {
			sys = &abap.SAPSystem{
				Name:     host,
				Host:     host,
				Client:   client,
				Username: user,
				Password: pass,
			}
		}
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	// Build connection pool (for multi-system requests via X-SAP-* headers).
	pool := server.NewPool(30*time.Minute, logger)
	pool.StartEviction(cmd.Context(), 5*time.Minute)

	cfg := &server.Config{AllowSelfSigned: allowSelfSigned}
	var staticClient types.ADTClient

	if sys != nil {
		adtCfg := types.ADTConfig{
			Host:            sys.Host,
			Client:          sys.Client,
			Username:        sys.Username,
			Password:        sys.Password,
			Language:        "EN",
			AllowSelfSigned: allowSelfSigned,
		}
		staticClient = adt.NewADTClient(&adtCfg)
		if err := staticClient.Authenticate(); err != nil {
			return fmt.Errorf("authenticate %s: %w", sys.Host, err)
		}
		cfg.ADTHost = sys.Host
		cfg.ADTClient = sys.Client
		cfg.ADTUsername = sys.Username
		cfg.ADTPassword = sys.Password
		logger.Info("Starting ABAPer REST server",
			zap.String("port", port),
			zap.String("sap_host", sys.Host),
			zap.String("sap_client", sys.Client),
			zap.String("sap_user", sys.Username))
	} else {
		logger.Info("Starting ABAPer REST server in multi-system mode", zap.String("port", port))
	}

	srv := server.NewRestServerWithPool(cfg, logger, pool, staticClient)

	fmt.Printf("ABAPer REST server listening on http://localhost:%s\n", port)
	if sys != nil {
		fmt.Printf("SAP system: %s (%s, client %s)\n", sys.Name, sys.Host, sys.Client)
	} else {
		fmt.Println("SAP system: none — multi-system mode (credentials via X-SAP-* headers)")
	}
	fmt.Println("Press Ctrl+C to stop.")

	srv.Start(port)
	return nil
}
