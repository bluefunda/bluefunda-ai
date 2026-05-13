package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	pb "github.com/bluefunda/cai-cli/api/proto/bff"
	"github.com/bluefunda/cai-cli/internal/config"
	caigrpc "github.com/bluefunda/cai-cli/internal/grpc"
	"github.com/bluefunda/cai-cli/internal/tools"
	"github.com/bluefunda/cai-cli/internal/ui"
)

var (
	codeModel     string
	codeDir       string
	codeAutoApply bool
)

var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "Agentic coding session with local file system access",
	Long: `Start an interactive coding session where the AI can read and write files,
run commands, and search your project. Tools that modify the filesystem or
run shell commands require your approval before execution (use --auto-apply
to skip confirmation).`,
	RunE: runCode,
}

func init() {
	codeCmd.Flags().StringVar(&codeModel, "model", "", "LLM model to use")
	codeCmd.Flags().StringVar(&codeDir, "dir", ".", "Working directory for file operations")
	codeCmd.Flags().BoolVar(&codeAutoApply, "auto-apply", false, "Execute write/bash tools without prompting")
}

// codeMessage mirrors the ConversationMsg format expected by cai-llm-router.
type codeMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []codeToolCall `json:"tool_calls,omitempty"`
}

// codeToolCall matches messages.ToolCall nested format in cai-llm-router.
type codeToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function codeFuncCall `json:"function"`
}

type codeFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// cliCodePayload is encoded in Prompt to work around proto fields 8+ being stripped
// by the load balancer between cli.bluefunda.com:443 and the BFF gRPC endpoint.
type cliCodePayload struct {
	History []codeMessage `json:"history"`
	Tools   string        `json:"tools"`
}

func runCode(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	model := codeModel
	if model == "" {
		model = cfg.Defaults.Model
	}
	if model == "" {
		model = "openai"
	}

	if err := os.Chdir(codeDir); err != nil {
		return fmt.Errorf("chdir to %s: %w", codeDir, err)
	}

	toolSchemas, err := tools.LocalToolSchemas()
	if err != nil {
		return fmt.Errorf("build tool schemas: %w", err)
	}

	chatID := uuid.New().String()
	p := printer(cfg)
	p.Info(fmt.Sprintf("Code session %s (model: %s, dir: %s)", chatID, model, codeDir))
	p.Info("Type /exit or Ctrl+D to quit. Use \\ at end of line for multiline.")
	fmt.Println()

	// history holds the full conversation for the current agentic session.
	// It is sent to the backend on every iteration so the LLM has context.
	var history []codeMessage
	isFirstTurn := true

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for {
		if conn.TS.NearExpiry(2 * time.Minute) {
			if err := conn.TS.EnsureValidToken(); err != nil {
				if authErr := reAuthenticate(cfg, p); authErr != nil {
					p.Error("Cannot restore session: " + authErr.Error())
					return authErr
				}
				fmt.Println()
			}
		}

		fmt.Print("You> ")
		var lines []string
		gotInput := false

		for scanner.Scan() {
			gotInput = true
			line := scanner.Text()
			if strings.HasSuffix(line, "\\") {
				lines = append(lines, strings.TrimSuffix(line, "\\"))
				fmt.Print("...> ")
				continue
			}
			lines = append(lines, line)
			break
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read input: %w", err)
		}
		if !gotInput {
			fmt.Println()
			return nil
		}

		input := strings.TrimSpace(strings.Join(lines, "\n"))
		if input == "" {
			continue
		}
		if input == "/exit" {
			return nil
		}

		// Append user message to history.
		history = append(history, codeMessage{Role: "user", Content: input})

		// Run the agentic loop for this user turn.
		history, err = agenticLoop(conn, cfg, chatID, model, toolSchemas, history, isFirstTurn, codeAutoApply, p)
		if err != nil {
			if caigrpc.IsAuthError(err) {
				p.Warn("Session expired. Please try again.")
			} else {
				p.Error("Error: " + err.Error())
			}
		}

		isFirstTurn = false
		fmt.Println()
	}
}

// agenticLoop runs one full agentic turn: sends a request to the LLM, handles
// any tool calls by executing them locally and feeding results back, and repeats
// until the LLM produces a content-only response.
func agenticLoop(
	conn *caigrpc.Conn,
	cfg *config.Config,
	chatID, model, toolSchemas string,
	history []codeMessage,
	isFirstTurn bool,
	autoApply bool,
	p *ui.Printer,
) ([]codeMessage, error) {
	const maxIterations = 20

	for iteration := 0; iteration < maxIterations; iteration++ {
		req := buildCodeRequest(chatID, model, toolSchemas, history, isFirstTurn && iteration == 0)

		ctx, cancel := context.WithCancel(context.Background())
		stream, err := conn.Client.Chat(ctx, req)
		if err != nil {
			cancel()
			return history, err
		}

		fmt.Print("\nAI> ")
		toolCalls, err := ui.StreamWithTools(stream, cancel)
		if err != nil {
			return history, err
		}

		if len(toolCalls) == 0 {
			// Final content response — the LLM is done for this user turn.
			return history, nil
		}

		// LLM wants to call tools. Execute each locally and collect results.
		// First record the assistant's tool-call turn in history.
		assistantMsg := codeMessage{
			Role:    "assistant",
			Content: "", // content was already streamed
		}
		for _, tc := range toolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, codeToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: codeFuncCall{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		history = append(history, assistantMsg)

		// Execute each tool and append the result to history.
		for _, tc := range toolCalls {
			result, execErr := executeWithApproval(tc, autoApply, p)
			history = append(history, codeMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
			if execErr != nil {
				p.Warn(fmt.Sprintf("Tool %s failed: %s", tc.Name, execErr.Error()))
			}
		}
	}

	p.Warn("Maximum tool iterations reached.")
	return history, nil
}

// buildCodeRequest constructs the gRPC ChatRequest for one agentic iteration.
// The history and tool schemas are encoded in Prompt as a JSON cliCodePayload and the
// model is prefixed with "cli/" so cai-llm-router can decode them. This works around
// the load balancer stripping proto fields 8+ (local_tools, code_messages) in transit.
func buildCodeRequest(chatID, model, toolSchemas string, history []codeMessage, isNewChat bool) *pb.ChatRequest {
	payload := cliCodePayload{History: history, Tools: toolSchemas}
	payloadJSON, _ := json.Marshal(payload)
	return &pb.ChatRequest{
		ChatId:    chatID,
		Model:     "cli/" + model,
		IsNewChat: isNewChat,
		Prompt:    string(payloadJSON),
	}
}

// executeWithApproval runs a tool, asking for user confirmation if needed.
func executeWithApproval(tc ui.ToolCallEvent, autoApply bool, p *ui.Printer) (string, error) {
	p.Info(fmt.Sprintf("\nTool: %s", tc.Name))
	if tc.Arguments != "" && tc.Arguments != "{}" {
		p.Info(fmt.Sprintf("Args: %s", tc.Arguments))
	}

	if tools.NeedsApproval(tc.Name) && !autoApply {
		fmt.Print("Apply? [y/N] ")
		var resp string
		if _, err := fmt.Scanln(&resp); err != nil {
			return "User declined to execute this tool.", nil
		}
		if strings.ToLower(strings.TrimSpace(resp)) != "y" {
			return "User declined to execute this tool.", nil
		}
	}

	result, err := tools.Execute(tc.Name, tc.Arguments)
	if err != nil {
		return "Error: " + err.Error(), err
	}
	return result, nil
}
