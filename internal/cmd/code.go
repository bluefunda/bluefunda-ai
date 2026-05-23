package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
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

	// history is shared between TUI turns via closure.
	var history []codeMessage
	isFirstTurn := true

	submitFn := func(cid, input string, isNew bool) <-chan tui.StreamEvent {
		if conn.TS.NearExpiry(2 * time.Minute) {
			if err := conn.TS.EnsureValidToken(); err != nil {
				if authErr := reAuthenticate(cfg, p); authErr != nil {
					ch := make(chan tui.StreamEvent, 1)
					ch <- tui.StreamEvent{Kind: "error", ErrMsg: "auth: " + authErr.Error()}
					close(ch)
					return ch
				}
			}
		}

		history = append(history, codeMessage{Role: "user", Content: input})
		ch := make(chan tui.StreamEvent, 64)

		go func() {
			defer close(ch)
			newHistory, loopErr := agenticLoopTUI(
				conn, cfg, cid, model, toolSchemas,
				history, isFirstTurn && isNew, codeAutoApply,
				p, ch,
			)
			history = newHistory
			isFirstTurn = false
			if loopErr != nil {
				if !caigrpc.IsAuthError(loopErr) {
					ch <- tui.StreamEvent{Kind: "error", ErrMsg: loopErr.Error()}
				}
			}
			ch <- tui.StreamEvent{Kind: "done"}
		}()

		return ch
	}

	workDir, _ := os.Getwd()
	tuiCfg := tui.SessionConfig{
		ChatID:    chatID,
		Model:     model,
		IsCode:    true,
		WorkDir:   workDir,
		AutoApply: codeAutoApply,
	}
	m := tui.New(tuiCfg, submitFn)
	return tui.Run(m)
}

// agenticLoopTUI runs the agentic loop for one user turn, sending tool events
// and content chunks to ch for TUI rendering. It handles approval prompts by
// sending StreamEvent{Kind:"approval"} and blocking on the reply channel.
func agenticLoopTUI(
	conn *caigrpc.Conn,
	cfg *config.Config,
	chatID, model, toolSchemas string,
	history []codeMessage,
	isFirstTurn bool,
	autoApply bool,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
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

		// Pump this iteration's stream into ch, collecting tool calls.
		toolCalls, err := pumpCodeStream(stream, cancel, ch)
		if err != nil {
			return history, err
		}

		if len(toolCalls) == 0 {
			return history, nil
		}

		// Build assistant tool-call turn in history
		assistantMsg := codeMessage{Role: "assistant"}
		for _, tc := range toolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, codeToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: codeFuncCall{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		history = append(history, assistantMsg)

		// Execute tools with TUI approval
		for _, tc := range toolCalls {
			result, execErr := executeWithApprovalTUI(tc, autoApply, p, ch)
			history = append(history, codeMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
			if execErr != nil {
				ch <- tui.StreamEvent{
					Kind:   "tool_exec",
					ToolName: tc.Name,
					Status: "error",
					Summary: execErr.Error(),
				}
			}
		}
	}

	ch <- tui.StreamEvent{Kind: "chunk", Chunk: "\n⚠  Maximum tool iterations reached.\n"}
	return history, nil
}

// pumpCodeStream reads a gRPC stream for one agentic iteration, forwarding
// content/tool events to ch and collecting tool_call events for return.
func pumpCodeStream(
	stream interface {
		Recv() (*pb.ChatEvent, error)
	},
	cancelFn context.CancelFunc,
	ch chan<- tui.StreamEvent,
) ([]ui.ToolCallEvent, error) {
	defer cancelFn()
	tf := &tui.ExportedThinkFilter{}
	var toolCalls []ui.ToolCallEvent

	for {
		ev, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				if tail := tf.Flush(); tail != "" {
					ch <- tui.StreamEvent{Kind: "chunk", Chunk: tail}
				}
				return toolCalls, nil
			}
			return toolCalls, fmt.Errorf("stream recv: %w", err)
		}

		switch ev.GetType() {
		case "content", "stream_chunk":
			filtered := tf.Filter(ev.GetContent())
			if filtered != "" {
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: filtered}
			}

		case "done", "stream_end":
			if tail := tf.Flush(); tail != "" {
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: tail}
			}
			return toolCalls, nil

		case "error", "stream_error":
			return toolCalls, fmt.Errorf("%s", ev.GetError())

		case "tool_call":
			var tc ui.ToolCallEvent
			data := ev.GetData()
			if data == "" {
				data = ev.GetContent()
			}
			if jsonErr := json.Unmarshal([]byte(data), &tc); jsonErr == nil {
				toolCalls = append(toolCalls, tc)
				ch <- tui.StreamEvent{
					Kind:     "tool_call",
					ToolID:   tc.ID,
					ToolName: tc.Name,
					ToolArgs: tc.Arguments,
				}
			}

		case "stream_progress":
			data := ev.GetData()
			if data == "" {
				data = ev.GetContent()
			}
			var prog struct {
				Tools     []string `json:"tools"`
				Iteration int      `json:"iteration"`
			}
			if jsonErr := json.Unmarshal([]byte(data), &prog); jsonErr == nil {
				ch <- tui.StreamEvent{
					Kind:      "progress",
					Iteration: prog.Iteration,
					Tools:     prog.Tools,
				}
			}

		case "stream_tool_execution":
			data := ev.GetData()
			if data == "" {
				data = ev.GetContent()
			}
			var te struct {
				ToolName      string `json:"tool_name"`
				Status        string `json:"status"`
				DurationMs    int64  `json:"duration_ms"`
				ResultSummary string `json:"result_summary"`
			}
			if jsonErr := json.Unmarshal([]byte(data), &te); jsonErr == nil {
				ch <- tui.StreamEvent{
					Kind:       "tool_exec",
					ToolName:   te.ToolName,
					Status:     te.Status,
					DurationMs: te.DurationMs,
					Summary:    te.ResultSummary,
				}
			}

		case "stream_heartbeat":
			ch <- tui.StreamEvent{Kind: "heartbeat"}
		}
	}
}

// executeWithApprovalTUI runs a tool, using the TUI approval flow when needed.
func executeWithApprovalTUI(tc ui.ToolCallEvent, autoApply bool, p *ui.Printer, ch chan<- tui.StreamEvent) (string, error) {
	if tools.NeedsApproval(tc.Name) && !autoApply {
		replyCh := make(chan bool, 1)
		ch <- tui.StreamEvent{
			Kind:     "approval",
			ToolName: tc.Name,
			ToolArgs: tc.Arguments,
			ReplyCh:  replyCh,
		}
		if approved := <-replyCh; !approved {
			return "User declined to execute this tool.", nil
		}
	}

	start := time.Now()
	result, execErr := tools.Execute(tc.Name, tc.Arguments)
	elapsed := time.Since(start)

	status := "ok"
	if execErr != nil {
		status = "error"
	}
	summary := result
	if execErr != nil {
		summary = execErr.Error()
	}
	if len(summary) > 80 {
		summary = summary[:77] + "..."
	}
	ch <- tui.StreamEvent{
		Kind:       "tool_exec",
		ToolName:   tc.Name,
		Status:     status,
		DurationMs: elapsed.Milliseconds(),
		Summary:    summary,
	}

	if execErr != nil {
		return "Error: " + execErr.Error(), execErr
	}
	return result, nil
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

