package cmd

import "github.com/spf13/cobra"

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check connectivity (alias for `bai doctor`)",
	RunE:  runDoctor,
}
