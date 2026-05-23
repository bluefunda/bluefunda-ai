package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check BFF connectivity (gRPC)",
	RunE:  runHealth,
}

func runHealth(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	if cfg.BFFURL == "" {
		return fmt.Errorf("bff_url not configured; run 'ai login' or pass --bff")
	}

	if err := caigrpc.Ping(cfg.BFFURL); err != nil {
		return fmt.Errorf("BFF unhealthy at %s: %w", cfg.BFFURL, err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "BFF healthy")
	return nil
}
