package cmd

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose your environment",
	RunE:  runDoctor,
}

type checkResult struct {
	label  string
	status string // "ok" | "warn" | "error" | "info"
	detail string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var checks []checkResult
	warnings := 0
	errors := 0

	out := cmd.OutOrStdout()
	fmt.Fprintln(out)

	// 1. Config file
	cfg, err := config.Load()
	if err != nil {
		checks = append(checks, checkResult{"Config file", "error", err.Error()})
		errors++
	} else {
		checks = append(checks, checkResult{"Config file", "ok", "~/.bai/config.yaml"})
	}

	// 2. Authentication
	if cfg.Auth.AccessToken == "" {
		checks = append(checks, checkResult{"Authentication", "error", "not signed in — run `bai login`"})
		errors++
		printChecks(out, checks, warnings, errors)
		return nil
	}
	if !cfg.TokenValid() {
		checks = append(checks, checkResult{"Authentication", "warn", "token expired — run `bai login`"})
		warnings++
	} else {
		dur := time.Until(cfg.Auth.TokenExpiry).Round(time.Minute)
		checks = append(checks, checkResult{"Authentication", "ok", fmt.Sprintf("token valid (expires in %v)", dur)})
	}

	// 3. Backend reachable
	start := time.Now()
	if err := caigrpc.Ping(cfg.BFFURL); err != nil {
		checks = append(checks, checkResult{"Backend reachable", "error", cfg.BFFURL + " — " + err.Error()})
		errors++
	} else {
		latency := time.Since(start).Round(time.Millisecond)
		checks = append(checks, checkResult{"Backend reachable", "ok", fmt.Sprintf("%s  (%v)", cfg.BFFURL, latency)})
	}

	// 4. Account info — validates auth end-to-end
	conn, _, connErr := bffConn()
	if connErr == nil {
		defer conn.Close()
		ctx, cancel := caigrpc.ContextWithTimeout()
		defer cancel()

		resp, err := conn.Client.GetUserInfo(ctx, &pb.GetUserInfoRequest{})
		if err != nil {
			checks = append(checks, checkResult{"Account", "error", "could not fetch account info"})
			errors++
		} else {
			checks = append(checks, checkResult{"Account", "ok", resp.GetEmail()})
		}
	}

	// 5. Default model
	if cfg.Defaults.Model == "" || cfg.Defaults.Model == "openai" {
		checks = append(checks, checkResult{"Model default", "warn", `not configured — run: bai config set model=claude-sonnet`})
		warnings++
	} else {
		checks = append(checks, checkResult{"Model default", "ok", cfg.Defaults.Model})
	}

	// 6. MCP servers (informational)
	checks = append(checks, checkResult{"MCP servers", "info", "run `bai mcp list` to view available integrations"})

	printChecks(out, checks, warnings, errors)
	return nil
}

func printChecks(out interface{ Write([]byte) (int, error) }, checks []checkResult, warnings, errors int) {
	okStyle := color.New(color.FgGreen)
	warnStyle := color.New(color.FgYellow)
	errStyle := color.New(color.FgRed)
	infoStyle := color.New(color.Faint)
	dimStyle := color.New(color.Faint)

	for _, c := range checks {
		var icon string
		switch c.status {
		case "ok":
			icon = okStyle.Sprint("✓")
		case "warn":
			icon = warnStyle.Sprint("!")
		case "error":
			icon = errStyle.Sprint("✗")
		default:
			icon = infoStyle.Sprint("ℹ")
		}
		label := dimStyle.Sprintf("%-20s", c.label)
		fmt.Fprintf(out, "  %s  %s %s\n", icon, label, c.detail)
	}

	fmt.Fprintln(out)
	switch {
	case errors > 0:
		_, _ = errStyle.Fprintf(out, "  %d error(s)", errors)
		if warnings > 0 {
			fmt.Fprintf(out, ", %d warning(s)", warnings) //nolint:errcheck
		}
		fmt.Fprintln(out)
	case warnings > 0:
		_, _ = warnStyle.Fprintf(out, "  %d warning(s)\n", warnings)
	default:
		_, _ = okStyle.Fprintln(out, "  All checks passed")
	}
	fmt.Fprintln(out)
}
