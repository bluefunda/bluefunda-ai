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
		Short: "Start the ABAPer REST server (abaper-ts replacement)",
		Long: `Starts an HTTP server that exposes the ADT SDK over REST, mirroring the
abaper-ts API surface.

SAP credentials are per request: every request must carry the caller's own
X-SAP-Host, X-SAP-Client, X-SAP-User and X-SAP-Password headers. Requests
without them are rejected with 401. SAP_* environment variables and stored
systems are ignored.

For local, single-user development you can pass --allow-static-sap to serve
requests without X-SAP-* headers using your own active SAP system from
~/.abaper/systems.json (or --system, or SAP_HOST/SAP_USERNAME/SAP_PASSWORD/
SAP_CLIENT). The server then listens on 127.0.0.1 only. Never use it for a
server other people can reach.`,
		RunE: runServe,
	}
	cmd.Flags().String("port", "8080", "Port to listen on")
	cmd.Flags().String("system", "", "SAP system name or ID for the static identity (requires --allow-static-sap; default: active system)")
	cmd.Flags().Bool("allow-self-signed", false, "Allow self-signed TLS certificates")
	cmd.Flags().Bool("allow-static-sap", false, "Local single-user use only: serve requests without X-SAP-* headers with your own stored/env SAP system, listening on 127.0.0.1")
	return cmd
}

// staticSAPEnv lists the environment variables the static identity may come
// from. They are only read with --allow-static-sap.
var staticSAPEnv = []string{"SAP_HOST", "SAP_USERNAME", "SAP_PASSWORD", "SAP_CLIENT"}

// resolveStaticSystem decides the SAP identity used for requests without
// X-SAP-* headers. Without allowStatic there is none: no stored system and no
// SAP_* environment, so every request must carry its own credentials.
func resolveStaticSystem(allowStatic bool, systemName string, loadSystems func() (*abap.Systems, error), getenv func(string) string) (*abap.SAPSystem, error) {
	if !allowStatic {
		if systemName != "" {
			return nil, fmt.Errorf("--system needs --allow-static-sap (local single-user use only); without it every request must carry X-SAP-* headers")
		}
		return nil, nil
	}

	sysCfg, err := loadSystems()
	if err != nil {
		return nil, fmt.Errorf("load systems: %w", err)
	}

	var sys *abap.SAPSystem
	if systemName != "" {
		sys = sysCfg.Find(systemName)
		if sys == nil {
			return nil, fmt.Errorf("system %q not found — run: abaper system list", systemName)
		}
	} else {
		sys = sysCfg.ActiveSystem()
	}

	// Fall back to environment variables when no stored system is available.
	if sys == nil {
		host := getenv("SAP_HOST")
		user := getenv("SAP_USERNAME")
		pass := getenv("SAP_PASSWORD")
		client := getenv("SAP_CLIENT")
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
	if sys == nil {
		return nil, fmt.Errorf("--allow-static-sap: no SAP system found — run: abaper system add, or set SAP_HOST, SAP_USERNAME and SAP_PASSWORD")
	}
	return sys, nil
}

func runServe(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetString("port")
	systemName, _ := cmd.Flags().GetString("system")
	allowSelfSigned, _ := cmd.Flags().GetBool("allow-self-signed")
	allowStatic, _ := cmd.Flags().GetBool("allow-static-sap")

	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:errcheck

	sys, err := resolveStaticSystem(allowStatic, systemName, abap.LoadSystems, os.Getenv)
	if err != nil {
		return err
	}
	if !allowStatic {
		for _, name := range staticSAPEnv {
			if os.Getenv(name) != "" {
				logger.Warn("SAP_* environment is ignored by serve; every request must carry X-SAP-* headers (pass --allow-static-sap for local single-user use)",
					zap.String("variable", name))
				break
			}
		}
	}

	// Build connection pool (for per-request credentials via X-SAP-* headers).
	pool := server.NewPool(30*time.Minute, logger)
	pool.StartEviction(cmd.Context(), 5*time.Minute)

	cfg := &server.Config{AllowSelfSigned: allowSelfSigned}
	var staticClient types.ADTClient
	addr := ":" + port

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
		// A static identity serves anyone who can reach the port, so keep it
		// on loopback: local single-user use only.
		addr = "127.0.0.1:" + port
		logger.Warn("--allow-static-sap: requests without X-SAP-* headers use this static SAP identity; local single-user use only, listening on 127.0.0.1",
			zap.String("port", port),
			zap.String("sap_host", sys.Host),
			zap.String("sap_client", sys.Client),
			zap.String("sap_user", sys.Username))
	} else {
		logger.Info("Starting ABAPer REST server; every request must carry X-SAP-* headers", zap.String("port", port))
	}

	srv := server.NewRestServerWithPool(cfg, logger, pool, staticClient)

	fmt.Printf("ABAPer REST server listening on http://localhost:%s\n", port)
	if sys != nil {
		fmt.Printf("SAP system: %s (%s, client %s), static identity for local use only (--allow-static-sap)\n", sys.Name, sys.Host, sys.Client)
	} else {
		fmt.Println("SAP system: none, per-request credentials via X-SAP-* headers")
	}
	fmt.Println("Press Ctrl+C to stop.")

	srv.StartAddr(addr)
	return nil
}
