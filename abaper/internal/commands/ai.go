package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/bluefunda/abaper/pkg/output"
	"github.com/bluefunda/bluefunda-ai/sdk/agent"
	"github.com/spf13/cobra"
)

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI-powered developer assistance",
	Long:  "Interact with ABAPer AI features including chat and agentic coding.",
}

var aiChatCmd = &cobra.Command{
	Use:   "chat [prompt]",
	Short: "Send a prompt to the AI assistant and stream the response",
	Long: `Send a single prompt to the ABAPer AI assistant and stream the response.
For an interactive multi-turn agentic session use: abaper ai code`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAIChat,
}

func init() {
	aiChatCmd.Flags().String("model", "fast", "LLM model alias: auto, fast, think")
	aiChatCmd.Flags().String("context-file", "", "ABAP source file to include as context")

	aiCmd.AddCommand(aiChatCmd)
}

func runAIChat(cmd *cobra.Command, args []string) error {
	model, _ := cmd.Flags().GetString("model")
	contextFile, _ := cmd.Flags().GetString("context-file")
	outputFmt, _ := cmd.Flags().GetString("output")

	prompt := args[0]

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	systemPrompt := "You are an ABAP expert assistant."
	if contextFile != "" {
		data, err := os.ReadFile(contextFile)
		if err != nil {
			return fmt.Errorf("read context file: %w", err)
		}
		systemPrompt += fmt.Sprintf("\n\nABAP source context:\n```abap\n%s\n```", string(data))
	}

	if outputFmt == "json" {
		return runAIChatJSON(ctx, model, systemPrompt, prompt)
	}
	return runAIChatText(ctx, model, systemPrompt, prompt)
}

func runAIChatText(ctx context.Context, model, systemPrompt, prompt string) error {
	runner := agent.New(agent.Options{
		Model:    model,
		MaxTurns: 1,
		OnEvent: func(ev agent.Event) {
			switch ev.Type {
			case "text":
				fmt.Print(ev.Text)
			case "tool_use":
				fmt.Fprintf(os.Stderr, "\n[tool: %s]\n", ev.ToolName)
			case "result":
				fmt.Println()
			case "error":
				fmt.Fprintf(os.Stderr, "\nError: %v\n", ev.Err)
			}
		},
	})
	defer func() { _ = runner.Close() }()
	runner.WithSystemPrompt(systemPrompt)

	if err := runner.Run(ctx, prompt); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		if isNotSignedIn(err) {
			return fmt.Errorf("%w\n\nRun: bai login", err)
		}
		return err
	}
	return nil
}

func runAIChatJSON(ctx context.Context, model, systemPrompt, prompt string) error {
	var fullText string
	var stopReason string

	runner := agent.New(agent.Options{
		Model:    model,
		MaxTurns: 1,
		OnEvent: func(ev agent.Event) {
			switch ev.Type {
			case "text":
				fullText += ev.Text
			case "result":
				stopReason = ev.StopReason
			}
		},
	})
	defer func() { _ = runner.Close() }()
	runner.WithSystemPrompt(systemPrompt)

	if err := runner.Run(ctx, prompt); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		if isNotSignedIn(err) {
			return fmt.Errorf("%w\n\nRun: bai login", err)
		}
		return err
	}

	output.PrintJSON(map[string]any{
		"content":     fullText,
		"stop_reason": stopReason,
	})
	return nil
}

func isNotSignedIn(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not signed in") || strings.Contains(msg, "bai login")
}
