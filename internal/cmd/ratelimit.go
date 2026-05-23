package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var rateLimitCmd = &cobra.Command{
	Use:     "rate-limit",
	Aliases: []string{"rl"},
	Short:   "Query current rate limit status",
	Hidden:  true,
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
		rows = append(rows,
			[]string{"Plan", stats.GetPlanType()},
			[]string{"Hourly Usage", fmt.Sprintf("%.1f%%", stats.GetHourlyPercentage())},
			[]string{"Daily Usage", fmt.Sprintf("%.1f%%", stats.GetDailyPercentage())},
			[]string{"Monthly Usage", fmt.Sprintf("%.1f%%", stats.GetMonthlyPercentage())},
		)
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
