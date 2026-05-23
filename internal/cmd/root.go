package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

var (
	cfgGateway string
	cfgBFF     string
	cfgDomain  string
	cfgOutput  string
)

// Version is set at build time via -ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "bai",
	Short:   "bai -- CLI for the BlueFunda AI platform",
	Long:    "bai is a command-line interface for interacting with the BlueFunda AI platform via gRPC.",
	Version: Version,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgGateway, "gateway", "", "Gateway base URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&cfgBFF, "bff", "", "BFF gRPC address host:port (overrides config)")
	rootCmd.PersistentFlags().StringVar(&cfgDomain, "domain", "", "Domain (overrides config)")
	rootCmd.PersistentFlags().StringVarP(&cfgOutput, "output", "o", "", "Output format: table, json, quiet")

	rootCmd.AddCommand(
		loginCmd,
		healthCmd,
		versionCmd,
		chatCmd,
		codeCmd,
		modelCmd,
		mcpCmd,
		userCmd,
		billingCmd,
		rateLimitCmd,
	)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// loadConfig loads the config and applies flag overrides.
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
