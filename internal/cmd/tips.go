package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/tips"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var tipsCmd = &cobra.Command{
	Use:   "tips",
	Short: "Manage the Contextual Tip Engine",
}

var tipsOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Permanently disable tips",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tips.Disable(); err != nil {
			return fmt.Errorf("disable tips: %w", err)
		}
		ui.Success("Tips disabled.")
		return nil
	},
}

var tipsOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Re-enable tips after 'bai tips off'",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tips.Enable(); err != nil {
			return fmt.Errorf("enable tips: %w", err)
		}
		ui.Success("Tips enabled.")
		return nil
	},
}

var tipsDismissCmd = &cobra.Command{
	Use:   "dismiss <family>",
	Short: "Suppress a tip family (backoff: 24h, then 72h, then 14d, then permanent)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		until, err := tips.DismissFamily(args[0])
		if err != nil {
			return fmt.Errorf("dismiss family: %w", err)
		}
		if until.Year() >= 9999 {
			ui.Success(fmt.Sprintf("Dismissed tip family %q permanently.", args[0]))
		} else {
			ui.Success(fmt.Sprintf("Dismissed tip family %q until %s.", args[0], until.Format("2006-01-02 15:04 MST")))
		}
		return nil
	},
}

var tipsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show what would be shown next, ignoring the anti-annoyance budget",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfig()
		listed, err := tips.ListEligible()
		if err != nil {
			return fmt.Errorf("list tips: %w", err)
		}
		rows := make([][]string, 0, len(listed))
		for _, l := range listed {
			rows = append(rows, []string{l.ID, l.Family, fmt.Sprintf("%.3f", l.Similarity), fmt.Sprintf("%t", l.Eligible)})
		}
		printer(cfg).Table([]string{"ID", "FAMILY", "SIMILARITY", "ELIGIBLE"}, rows)
		return nil
	},
}

func init() {
	tipsCmd.AddCommand(tipsOffCmd, tipsOnCmd, tipsDismissCmd, tipsListCmd)
}
