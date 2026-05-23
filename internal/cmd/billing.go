package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Billing and subscription operations",
}

// --- billing subscription ---

var billingSubCmd = &cobra.Command{
	Use:   "subscription",
	Short: "Show current subscription details",
	RunE:  runBillingSubscription,
}

func runBillingSubscription(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetStripeSubscription(ctx, &pb.GetStripeSubscriptionRequest{})
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	if resp.GetError() != "" {
		return fmt.Errorf("subscription: %s", resp.GetError())
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Plan", resp.GetPlanName()},
		{"Status", resp.GetSubscriptionStatus()},
		{"Expires", resp.GetExpirationDate()},
		{"Daily Tokens", fmt.Sprintf("%d", resp.GetDailyTokenLimit())},
		{"Monthly Tokens", fmt.Sprintf("%d", resp.GetMonthlyTokenLimit())},
	}
	p.Table(headers, rows)
	return nil
}

// --- billing plans ---

var billingPlansCmd = &cobra.Command{
	Use:   "plans",
	Short: "List available subscription plans",
	RunE:  runBillingPlans,
}

func runBillingPlans(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetStripePlans(ctx, &pb.GetStripePlansRequest{})
	if err != nil {
		return fmt.Errorf("get plans: %w", err)
	}

	if resp.GetError() != "" {
		return fmt.Errorf("plans: %s", resp.GetError())
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"NAME", "PRICE", "PERIOD", "FEATURES"}
	rows := make([][]string, 0, len(resp.GetPlans()))
	for _, plan := range resp.GetPlans() {
		price := fmt.Sprintf("$%.2f", float64(plan.GetPriceCents())/100)
		rows = append(rows, []string{
			plan.GetName(),
			price,
			plan.GetBillingPeriod(),
			truncate(strings.Join(plan.GetFeatures(), ", "), 50),
		})
	}
	p.Table(headers, rows)
	return nil
}

func init() {
	billingCmd.AddCommand(billingSubCmd, billingPlansCmd)
}
