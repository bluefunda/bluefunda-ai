package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

func usageBarASCII(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "#"
		} else {
			bar += "-"
		}
	}
	return bar
}

var rateLimitCmd = &cobra.Command{
	Use:     "rate-limit",
	Aliases: []string{"rl"},
	Short:   "Query current rate limit and token usage",
	RunE:    runRateLimit,
}

func runRateLimit(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.QueryRateLimit(ctx, &pb.QueryRateLimitRequest{})
	if err != nil {
		return fmt.Errorf("query rate limit: %w", err)
	}

	if resp.GetError() != "" {
		return fmt.Errorf("rate limit: %s", resp.GetError())
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Allowed", fmt.Sprintf("%t", resp.GetAllowed())},
		{"Remaining", fmt.Sprintf("%d", resp.GetRemaining())},
	}

	if stats := resp.GetUserStats(); stats != nil {
		rows = append(rows, []string{"Plan", stats.GetPlanType()})

		addUsageRow := func(label string, pct float64) {
			bar := usageBarASCII(pct, 10)
			alert := ""
			switch {
			case pct >= 100:
				pct = 100
				alert = " ✗"
			case pct >= 95:
				alert = " ⚠ 95%"
			case pct >= 90:
				alert = " ⚠ 90%"
			case pct >= 75:
				alert = " ⚠ 75%"
			case pct >= 50:
				alert = " ◆ 50%"
			}
			rows = append(rows, []string{label, fmt.Sprintf("[%s] %.1f%%%s", bar, pct, alert)})
		}

		rpmPct := stats.GetRpmPercentage()
		rpmBar := usageBarASCII(rpmPct, 10)
		rows = append(rows, []string{"RPM", fmt.Sprintf("%d/%d [%s] %.1f%%", stats.GetRpmUsed(), stats.GetRpmLimit(), rpmBar, rpmPct)})
		addUsageRow("Daily", stats.GetDailyPercentage())
		addUsageRow("Monthly", stats.GetMonthlyPercentage())
	}

	if usage := resp.GetTokenUsage(); usage != nil {
		rows = append(rows,
			[]string{"Input Tokens", fmt.Sprintf("%d", usage.GetInputTokens())},
			[]string{"Output Tokens", fmt.Sprintf("%d", usage.GetOutputTokens())},
			[]string{"Total Tokens", fmt.Sprintf("%d", usage.GetTotalTokens())},
		)
	}

	p.Table(headers, rows)
	return nil
}
