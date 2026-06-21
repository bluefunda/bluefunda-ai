package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var (
	cfgGateway string
	cfgBFF     string
	cfgDomain  string
	cfgOutput  string
	rootNew    bool
)

// Version is set at build time via -ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "bai [prompt]",
	Short: "Your AI pair programmer",
	Long:  "BlueFunda AI — start a session, ask a question, or jump straight into coding.",
	Example: `  bai                          start interactive session
  bai "fix the failing tests"  start with a message
  bai code                     agentic coding mode
  bai login                    sign in`,
	Args: cobra.ArbitraryArgs,
	RunE: runDefault,
}

func runDefault(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	if cfg.Auth.AccessToken == "" {
		fmt.Println("Not signed in. Run `bai login` to get started.")
		fmt.Println()
		fmt.Println("  bai login     sign in with your BlueFunda account")
		fmt.Println("  bai doctor    check configuration and connectivity")
		return nil
	}
	prompt := strings.Join(args, " ")
	return runChatSession("", prompt, "", "")
}

func init() {
	// Infrastructure flags — hidden from help but still functional for power users and scripts.
	rootCmd.PersistentFlags().StringVar(&cfgGateway, "gateway", "", "Gateway base URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&cfgBFF, "bff", "", "gRPC endpoint host:port (overrides config)")
	rootCmd.PersistentFlags().StringVar(&cfgDomain, "domain", "", "Auth domain (overrides config)")
	_ = rootCmd.PersistentFlags().MarkHidden("gateway")
	_ = rootCmd.PersistentFlags().MarkHidden("bff")
	_ = rootCmd.PersistentFlags().MarkHidden("domain")

	rootCmd.PersistentFlags().StringVarP(&cfgOutput, "output", "o", "", "Output format: table, json, quiet")
	rootCmd.Flags().BoolVar(&rootNew, "new", false, "Force a new session")
	rootCmd.Flags().BoolP("version", "v", false, "Print version and exit")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if v, _ := cmd.Flags().GetBool("version"); v {
			fmt.Println("bai version " + Version)
			os.Exit(0)
		}
		return nil
	}

	rootCmd.AddCommand(
		// Visible commands
		loginCmd,
		initCmd,
		codeCmd,
		configCmd,
		doctorCmd,
		mcpCmd,
		updateCmd,
		versionCmd,
		// Hidden commands
		sessionsCmd,
		// Hidden backward-compat commands
		chatCmd,
		healthCmd,
		modelCmd,
		userCmd,
		billingCmd,
		rateLimitCmd,
	)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// loadConfig loads the config and applies any flag overrides.
func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		ui.Error("Failed to load config: " + err.Error())
		return &config.Config{}
	}
	if cfgGateway != "" {
		cfg.GatewayURL = cfgGateway
	}
	if cfgBFF != "" {
		cfg.BFFURL = cfgBFF
	}
	if cfgDomain != "" {
		cfg.Domain = cfgDomain
	}
	return cfg
}

// outputFormat returns the effective output format from flags or config.
func outputFormat(cfg *config.Config) ui.OutputFormat {
	if cfgOutput != "" {
		switch cfgOutput {
		case "json":
			return ui.FormatJSON
		case "quiet":
			return ui.FormatQuiet
		default:
			return ui.FormatTable
		}
	}
	switch cfg.Defaults.Output {
	case "json":
		return ui.FormatJSON
	case "quiet":
		return ui.FormatQuiet
	default:
		return ui.FormatTable
	}
}
