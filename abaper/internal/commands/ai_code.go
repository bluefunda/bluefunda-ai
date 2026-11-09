package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/bluefunda/abaper/internal/agentrun"
	"github.com/bluefunda/abaper/internal/client"
	"github.com/bluefunda/bluefunda-ai/sdk/agent"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var aiCodeCmd = &cobra.Command{
	Use:   "code [prompt]",
	Short: "Agentic ABAP coding assistant — reads, writes, deploys",
	Long: `Run an agentic loop that can read and edit local ABAP source files
and deploy them to SAP via the abaper CLI. The agent iterates until the
task is complete or the turn limit is reached.

When the loop ends, abaper itself syntax-checks, activates and unit-tests
every object the agent deployed. Failures are fed back to the agent for up
to --repair-turns rounds; the command exits non-zero if any remain.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAICode,
}

func init() {
	aiCodeCmd.Flags().String("model", "fast", "LLM model alias: auto, fast, think")
	aiCodeCmd.Flags().Int("max-turns", 20, "Maximum agentic loop iterations")
	aiCodeCmd.Flags().String("context-file", "", "Seed the session with an ABAP source file")
	aiCodeCmd.Flags().Bool("verbose", false, "Show every tool call and its output instead of a collapsed progress line")
	aiCodeCmd.Flags().Bool("no-verify", false, "Skip the post-run syntax check / activation / unit test gate")
	aiCodeCmd.Flags().Int("repair-turns", 2, "Rounds the agent gets to fix objects that fail verification")

	aiCmd.AddCommand(aiCodeCmd)
}

func runAICode(cmd *cobra.Command, args []string) error {
	model, _ := cmd.Flags().GetString("model")
	maxTurns, _ := cmd.Flags().GetInt("max-turns")
	contextFile, _ := cmd.Flags().GetString("context-file")
	verbose, _ := cmd.Flags().GetBool("verbose")
	noVerify, _ := cmd.Flags().GetBool("no-verify")
	repairTurns, _ := cmd.Flags().GetInt("repair-turns")

	prompt := strings.Join(args, " ")

	if contextFile != "" {
		data, err := os.ReadFile(contextFile)
		if err != nil {
			return fmt.Errorf("read context file: %w", err)
		}
		prompt = fmt.Sprintf("%s\n\nSource file (%s):\n```abap\n%s\n```", prompt, contextFile, string(data))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Collapse tool-call noise into a single progress line unless the caller
	// asked for --verbose, or stdout isn't a terminal (piped/redirected output
	// can't overwrite a line in place, so fall back to the plain per-call log).
	collapse := !verbose && term.IsTerminal(int(os.Stdout.Fd()))
	toolCalls := 0
	progressDrawn := false

	clearProgressLine := func() {
		if progressDrawn {
			fmt.Fprint(os.Stderr, "\r\x1b[K")
			progressDrawn = false
		}
	}

	tracker := agentrun.NewTracker()

	runner := agent.New(agent.Options{
		Model:    model,
		MaxTurns: maxTurns,
		OnEvent: func(ev agent.Event) {
			tracker.Observe(ev)
			switch ev.Type {
			case "text":
				clearProgressLine()
				fmt.Print(ev.Text)
			case "tool_use":
				toolCalls++
				if collapse {
					fmt.Fprintf(os.Stderr, "\r\x1b[KWorking… (%d tool call%s)", toolCalls, plural(toolCalls))
					progressDrawn = true
				} else {
					fmt.Fprintf(os.Stderr, "\n[%s] %s\n", ev.ToolName, truncate(ev.ToolInput, 200))
				}
			case "tool_result":
				if !collapse {
					fmt.Fprintf(os.Stderr, "  -> %s\n", truncate(ev.ToolOutput, 200))
				}
			case "result":
				clearProgressLine()
				fmt.Println()
				if ev.StopReason == "max_turns" {
					fmt.Fprintf(os.Stderr, "[max turns reached]\n")
				}
			case "error":
				clearProgressLine()
				fmt.Fprintf(os.Stderr, "\nError: %v\n", ev.Err)
			}
		},
	})
	defer func() { _ = runner.Close() }()

	runner.WithSystemPrompt(agentrun.SystemPrompt)

	run := func(p string) error {
		err := runner.Run(ctx, p)
		clearProgressLine()
		if err != nil && strings.Contains(err.Error(), "not signed in") {
			return fmt.Errorf("%w\n\nRun: bai login", err)
		}
		return err
	}

	if err := run(prompt); err != nil {
		if ctx.Err() != nil {
			return nil // user cancelled
		}
		fmt.Fprintln(os.Stderr, "\n"+agentrun.Summary(tracker.Objects()))
		return err
	}

	if !noVerify && len(tracker.Objects()) > 0 {
		if err := verifyAndRepair(ctx, run, tracker, repairTurns); err != nil {
			fmt.Fprintln(os.Stderr, "\n"+agentrun.Summary(tracker.Objects()))
			return err
		}
	}

	fmt.Fprintln(os.Stderr, "\n"+agentrun.Summary(tracker.Objects()))
	for _, o := range tracker.Objects() {
		if !o.OK {
			return fmt.Errorf("%s %s was not deployed: %s", o.Type, o.Name, o.Detail)
		}
	}
	return nil
}

// verifyAndRepair runs the post-loop gate and hands failures back to the
// agent for up to repairTurns rounds.
func verifyAndRepair(ctx context.Context, run func(string) error, tracker *agentrun.Tracker, repairTurns int) error {
	c, err := client.NewClient()
	if err != nil {
		return fmt.Errorf("verification: %w", err)
	}
	for round := 0; ; round++ {
		fmt.Fprintln(os.Stderr, "\nVerifying SAP objects…")
		results := agentrun.Verify(c, tracker.Objects())
		fmt.Fprintln(os.Stderr, agentrun.Report(results))
		if agentrun.AllPassed(results) {
			return nil
		}
		if round >= repairTurns {
			return fmt.Errorf("verification failed after %d repair round(s)", repairTurns)
		}
		fmt.Fprintf(os.Stderr, "\nRepair round %d of %d…\n", round+1, repairTurns)
		if err := run(agentrun.RepairPrompt(results)); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "… (truncated)"
}
