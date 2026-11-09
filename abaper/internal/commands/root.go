package commands

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/abaper/internal/config"
	"github.com/bluefunda/abaper/tui"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "abaper",
	Short: "ABAPer CLI — AI pair programmer for SAP ABAP developers",
	Long: `ABAPer is an AI pair programmer for SAP ABAP developers.
Run bare to start the interactive TUI, or use a subcommand for non-interactive workflows.`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		go client.Track("cli_invoked", map[string]string{"command": cmd.Name()})
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(
			tui.New(version),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tui: %w", err)
		}
		return nil
	},
}

func init() {
	cobra.OnInitialize(config.Init)

	rootCmd.PersistentFlags().String("base-url", "", "ABAPer API base URL (default: https://api.bluefunda.com)")
	rootCmd.PersistentFlags().String("realm", "", "Keycloak realm (default: trm)")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text, json")

	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(signupCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(aiCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(systemCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(doctorCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("abaper version %s\n", version)
	},
}
