package commands

import (
	"context"
	"fmt"

	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/internal/health"
	"github.com/bluefunda/abaper/pkg/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connection, authentication, and SAP system status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := config.Load()
	report := health.Check(context.Background(), true)

	systems := make([]map[string]any, len(report.Systems))
	for i, s := range report.Systems {
		sys := map[string]any{
			"name":      s.Name,
			"host":      s.Host,
			"active":    s.Active,
			"reachable": s.Reachable,
		}
		if s.Err != "" {
			sys["error"] = s.Err
		}
		systems[i] = sys
	}

	status := map[string]any{
		"base_url":      cfg.BaseURL,
		"realm":         cfg.Realm,
		"org":           cfg.Org,
		"authenticated": report.TokenValid,
		"api_reachable": report.GatewayReachable,
		"systems":       systems,
	}
	if report.GatewayStatus != "" {
		status["api_status"] = report.GatewayStatus
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	if outputFmt == "json" {
		output.PrintJSON(status)
		return nil
	}

	fmt.Printf("ABAPer CLI Status\n")
	fmt.Printf("  Base URL:       %s\n", cfg.BaseURL)
	fmt.Printf("  Realm:          %s\n", cfg.Realm)
	fmt.Printf("  Organization:   %s\n", cfg.Org)
	fmt.Printf("  Authenticated:  %v\n", report.TokenValid)
	fmt.Printf("  API Reachable:  %v\n", report.GatewayReachable)
	if report.GatewayStatus != "" {
		fmt.Printf("  API Status:     %v\n", report.GatewayStatus)
	}

	if len(report.Systems) == 0 {
		fmt.Printf("  SAP Systems:    none configured — run: abaper system add --help\n")
		return nil
	}
	fmt.Printf("  SAP Systems:\n")
	for _, s := range report.Systems {
		marker := " "
		if s.Active {
			marker = "●"
		}
		state := "✗ unreachable"
		if s.Reachable {
			state = "✓ reachable"
		}
		fmt.Printf("    %s %-20s %-30s %s\n", marker, s.Name, s.Host, state)
	}

	return nil
}
