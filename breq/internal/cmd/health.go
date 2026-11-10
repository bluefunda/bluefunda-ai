package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check connectivity to the TRM BFF",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		p := printer(cfg)

		if cfg.BFFURL == "" {
			return fmt.Errorf("bff_url not configured; run 'breq login' or pass --bff")
		}

		if err := trmgrpc.Ping(cfg.BFFURL); err != nil {
			return fmt.Errorf("BFF unhealthy at %s: %w", cfg.BFFURL, err)
		}

		p.Success(fmt.Sprintf("BFF reachable at %s", cfg.BFFURL))
		return nil
	},
}
