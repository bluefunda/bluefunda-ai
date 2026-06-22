package cmd

import (
	"fmt"
	"os"

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
	// Agentic session flags — shared with the deprecated 'bai code' alias.
	rootModel      string
	rootFast       bool
	rootThink      bool
	rootAuto       bool
	rootAutoApply  bool
	rootMaxTurns   int
	rootDir        string
	rootContinue   bool
	rootResume     string
	rootPrint      bool
	rootNoTools    bool
	rootOutFormat  string
)

// Version is set at build time via -ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "bai [prompt]",
	Short: "Your AI pair programmer",
	Long:  "BlueFunda AI — interactive AI with local file and shell access.",
	Example: `  bai                                interactive session with tools
  bai "fix the failing tests"        start with a prompt
  bai --fast                         use Groq fast-responder
  bai --think                        extended thinking mode
  bai --auto "add godoc to exports"  auto-approve all tool calls
  bai -c                             resume most recent session
  bai login                          sign in`,
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
	// Sync root flags → shared code vars so the agentic loop can read them.
	codeModel = rootModel
	codeAuto = rootAuto
	codeAutoApply = rootAutoApply
	if rootMaxTurns > 0 {
		codeMaxTurns = rootMaxTurns
	}
	if rootDir != "" {
		codeDir = rootDir
	}
	codeContinue = rootContinue
	codeResume = rootResume
	codePrint = rootPrint
	codeOutputFormat = rootOutFormat
	if rootFast {
		codeModel = "fast"
	}
	if rootThink {
		codeModel = "think"
	}
	if rootNoTools {
		codeNoTools = true
	}
	return runAgenticSession(args)
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

	// Agentic session flags (also registered on the deprecated 'bai code' alias).
	rootCmd.Flags().StringVarP(&rootModel, "model", "m", "", "Model alias: auto, fast, think, openai, anthropic, ...")
	rootCmd.Flags().BoolVar(&rootFast, "fast", false, "Use Groq fast-responder (~300ms)")
	rootCmd.Flags().BoolVar(&rootThink, "think", false, "Enable extended thinking")
	rootCmd.Flags().BoolVar(&rootAuto, "auto", false, "Auto-approve all tool calls")
	rootCmd.Flags().BoolVar(&rootAutoApply, "auto-apply", false, "Same as --auto")
	rootCmd.Flags().IntVar(&rootMaxTurns, "max-turns", 20, "Max agentic loop iterations")
	rootCmd.Flags().StringVar(&rootDir, "dir", ".", "Working directory for file operations")
	rootCmd.Flags().BoolVarP(&rootContinue, "continue", "c", false, "Resume most recent session")
	rootCmd.Flags().StringVar(&rootResume, "resume", "", "Resume a specific session ID")
	rootCmd.Flags().BoolVarP(&rootPrint, "print", "p", false, "Headless mode: print output to stdout")
	rootCmd.Flags().StringVar(&rootOutFormat, "output-format", "text", "Output format for --print: text, json, stream-json")
	rootCmd.Flags().BoolVar(&rootNoTools, "no-tools", false, "Disable local tools (pure chat mode)")
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
		logoutCmd,
		initCmd,
		codeCmd,
		configCmd,
		doctorCmd,
		mcpCmd,
		updateCmd,
		versionCmd,
		completionCmd,
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
