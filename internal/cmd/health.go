package cmd

import "github.com/spf13/cobra"

// healthCmd is kept for backward compatibility but hidden from help.
// Use `bai doctor` instead.
var healthCmd = &cobra.Command{
	Use:    "health",
	Short:  "Check connectivity (use `bai doctor` instead)",
	Hidden: true,
	RunE:   runDoctor,
}
