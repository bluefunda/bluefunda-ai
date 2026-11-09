package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/abap"
	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/updatecheck"
)

// ABAPer is the product identity of the abaper binary. It keeps abaper's own
// config directory (~/.abaper, so existing systems.json files keep working),
// Keycloak client, Homebrew cask and orange brand.
var ABAPer = app.Identity{
	Name:            "abaper",
	DisplayName:     "ABAPer",
	Dir:             ".abaper",
	EnvPrefix:       "ABAPER_",
	ClientID:        "cai-cli",
	KeychainService: "abaper",
	HomebrewCask:    "abaper",
	Placeholder:     "Type your ABAP request or / for commands",
	Accent:          "#FF5F1F",
	AccentBold:      "#FF8C42",
}

// ExecuteABAPer runs the abaper binary: the ABAP command set at the root,
// the platform's login/logout/version/update, and the interactive ABAP agent
// when run bare. extra adds abaper-only commands (e.g. the REST server).
func ExecuteABAPer(extra ...*cobra.Command) error {
	app.Set(ABAPer)
	abap.Command = "abaper"
	// The task tool re-executes the current binary with bai's headless
	// flags, which abaper does not have.
	sessionDeny = append(sessionDeny, "task")
	loginCmd.Long = loginLongHelp()

	root := newABAPerRoot()
	root.AddCommand(extra...)
	return root.Execute()
}

func newABAPerRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "abaper",
		Short: "ABAPer — AI pair programmer for SAP ABAP developers",
		Long: `ABAPer is an AI pair programmer for SAP ABAP developers.
Run bare to start the interactive agent, or use a subcommand for scripted workflows.`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if codeAuto {
				codeAutoApply = true
			}
			return runABAPSession(nil)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			applyGlobalFlags()
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.CommandPath() == "abaper" {
				return nil // the TUI footer already shows update notices
			}
			cfg := loadConfig()
			updatecheck.MaybeNotify(Version, outputFormat(cfg) == ui.FormatQuiet, cfg.UpdateCheckEnabled())
			return nil
		},
	}
	root.PersistentFlags().StringVarP(&cfgOutput, "output", "o", "", "Output format: table, json, quiet")
	root.PersistentFlags().StringVar(&cfgProfile, "profile", "", "Activate a named backend profile for this session")
	root.PersistentFlags().BoolVar(&rootNoColor, "no-color", false, "Disable colored output")
	_ = root.PersistentFlags().MarkHidden("no-color")

	// Interactive session flags, bound to the same state as `bai code`.
	root.Flags().StringVarP(&codeModel, "model", "m", "", "Model alias: auto, fast, think, ...")
	root.Flags().BoolVar(&codeAuto, "auto", false, "Auto-approve all tool calls")
	root.Flags().StringVar(&codeDir, "dir", ".", "Working directory for file operations")
	root.Flags().IntVar(&codeMaxTurns, "max-turns", defaultMaxTurnsFlag, "Max agentic loop iterations")
	root.Flags().BoolVarP(&codeContinue, "continue", "c", false, "Resume the most recent session")
	root.Flags().StringVar(&codeResume, "resume", "", "Resume a specific session ID")
	root.Flags().BoolVar(&codePlanMode, "plan", false, "Start in plan mode (read-only tools)")

	root.AddCommand(loginCmd, logoutCmd, versionCmd, updateCmd)
	root.AddCommand(NewABAPCommands()...)
	root.AddCommand(abaperAICompatCmd())
	return root
}

// abaperAICompatCmd keeps `abaper ai code` and `abaper ai chat` working for
// scripts written against the standalone abaper CLI.
func abaperAICompatCmd() *cobra.Command {
	ai := &cobra.Command{Use: "ai", Short: "Deprecated: use `abaper code` or bare `abaper`", Hidden: true}
	chat := &cobra.Command{
		Use:   "chat",
		Short: "Open the interactive ABAP agent",
		Args:  cobra.ArbitraryArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runABAPSession(args) },
	}
	ai.AddCommand(abapCodeCmd(), chat)
	return ai
}
