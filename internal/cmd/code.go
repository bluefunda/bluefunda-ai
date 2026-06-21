package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/audit"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/hooks"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
	"github.com/bluefunda/bluefunda-ai/internal/session"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

var (
	codeModel        string
	codeDir          string
	codeAutoApply    bool
	codeAuto         bool
	codeMaxTurns     int
	codePrint        bool
	codeOutputFormat string
	codeResume       string
	codeContinue     bool
)

var codeCmd = &cobra.Command{
	Use:   "code [prompt]",
	Short: "Agentic coding session with local file system access",
	Long: `Start an interactive coding session where the AI can read and write files,
run commands, and search your project. Tools that modify the filesystem or
run shell commands require your approval before execution (use --auto to
skip confirmation).

Use --print / -p for non-interactive (headless) mode: reads the prompt from
the argument or stdin and writes output to stdout.`,
	Example: `  bai code                                     interactive coding session
  bai code "fix the failing tests"             start with a prompt
  bai code -c                                  resume most recent session
  bai code --resume <id>                       resume a specific session
  bai code --auto "add godoc to all exports"   auto-approve all tools
  bai code --max-turns 50 "refactor auth"      increase turn limit
  bai code -p "explain main.go"                headless, output to stdout
  echo "list TODOs" | bai code -p              pipe prompt from stdin
  bai code -p "…" --output-format stream-json  NDJSON event stream`,
	Args: cobra.ArbitraryArgs,
	RunE: runCode,
}

func init() {
	codeCmd.Flags().StringVar(&codeModel, "model", "", "LLM model to use")
	codeCmd.Flags().StringVar(&codeDir, "dir", ".", "Working directory for file operations")
	codeCmd.Flags().BoolVar(&codeAutoApply, "auto-apply", false, "Execute write/bash tools without prompting")
	codeCmd.Flags().BoolVar(&codeAuto, "auto", false, "Same as --auto-apply")
	codeCmd.Flags().IntVar(&codeMaxTurns, "max-turns", 20, "Maximum agentic loop iterations before stopping")
	codeCmd.Flags().BoolVarP(&codePrint, "print", "p", false, "Non-interactive mode: print output to stdout")
	codeCmd.Flags().StringVar(&codeOutputFormat, "output-format", "text", "Output format for --print: text, json, stream-json")
	codeCmd.Flags().StringVar(&codeResume, "resume", "", "Resume a previous session by ID")
	codeCmd.Flags().BoolVarP(&codeContinue, "continue", "c", false, "Resume the most recent code session")
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

// cliPayloadVersion is incremented on any breaking change to cliCodePayload.
// cai-llm-router validates this and returns a clear error on mismatch so
// users know to upgrade their bai client.
const cliPayloadVersion = 1

// cliCodePayload is encoded in Prompt to work around proto fields 8+ being stripped
// by the load balancer between cli.bluefunda.com:443 and the BFF gRPC endpoint.
type cliCodePayload struct {
	V       int           `json:"v"` // payload format version — must match cliPayloadVersion in cai-llm-router
	History []codeMessage `json:"history"`
	Tools   string        `json:"tools"`
}

func runCode(cmd *cobra.Command, args []string) error {
	if codeAuto {
		codeAutoApply = true
	}

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

	initialPrompt := strings.Join(args, " ")

	if err := os.Chdir(codeDir); err != nil {
		return fmt.Errorf("chdir to %s: %w", codeDir, err)
	}

	// Apply project-level max_turns override if the flag wasn't set explicitly.
	if projCfgEarly := config.FindProjectConfig("."); projCfgEarly != nil {
		if projCfgEarly.MaxTurns > 0 {
			if !cmd.Flags().Changed("max-turns") {
				codeMaxTurns = projCfgEarly.MaxTurns
			}
		}
	}

	toolSchemas, err := tools.LocalToolSchemas()
	if err != nil {
		return fmt.Errorf("build tool schemas: %w", err)
	}

	workDir, _ := os.Getwd()
	p := printer(cfg)

	// --- Local MCP servers (#85) ---
	// Start servers defined in .bai/settings.yaml mcp_servers and merge their
	// tools into the schema sent to the LLM on every agentic turn.
	projCfg := config.FindProjectConfig(".")
	mcpMgr := mcp.NewManager(context.Background(), projCfg)
	defer mcpMgr.Close()
	if extra := mcpMgr.ToolSchemas(); len(extra) > 0 {
		merged, mergeErr := tools.MergeSchemas(toolSchemas, extra)
		if mergeErr == nil {
			toolSchemas = merged
		}
	}

	// --- Session persistence (#82) ---
	// --continue/-c resumes the most recent session for this working directory.
	sessionID := codeResume
	if sessionID == "" && codeContinue {
		if infos, err := session.List(workDir); err == nil && len(infos) > 0 {
			sessionID = infos[0].ID
		}
	}
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	sessPath, _ := session.Path(workDir, sessionID)

	// --- Audit logging (#81) ---
	auditLog, _ := audit.NewLogger(sessionID)
	defer auditLog.Close()
	auditLog.LogSessionStart(model, workDir, Version)

	// --- Hook runner (#80) ---
	hooksDir := hooks.FindHooksDir(".")
	hookRunner := hooks.New(hooksDir, sessionID, workDir)

	// --- History: context + optional resume (#82) ---
	var history []codeMessage
	if ctx := loadContextFiles("."); ctx != "" {
		history = append(history, codeMessage{Role: "system", Content: ctx})
	}
	if codeResume != "" || codeContinue {
		if msgs, err := session.Load(sessPath); err == nil {
			for _, m := range msgs {
				history = append(history, codeMessage{
					Role:       m.Role,
					Content:    m.Content,
					ToolCallID: m.ToolCallID,
				})
			}
		}
	}

	// --- Headless print mode (#77) ---
	if codePrint || !isTerminal() {
		if initialPrompt == "" {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			initialPrompt = strings.TrimSpace(string(b))
		}
		if initialPrompt == "" {
			return fmt.Errorf("prompt required in --print mode (pass as argument or pipe via stdin)")
		}
		return runCodePrint(conn, cfg, model, toolSchemas, initialPrompt, history,
			codeMaxTurns, codeAutoApply, codeOutputFormat, sessPath, auditLog, hookRunner, mcpMgr, p)
	}

	// --- Interactive TUI mode ---
	chatID := uuid.New().String()

	// autoApplyState and toolSchemasState are shared between the TUI (/auto,
	// /chat, /code toggles) and the submit closure, so changes take effect on
	// the very next turn.
	autoApplyState := codeAutoApply
	toolSchemasState := toolSchemas
	maxTurnsState := codeMaxTurns
	turns := 0
	isFirstTurn := true

	submitFn := func(cid, mdl, input string, isNew bool) <-chan tui.StreamEvent {
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
		currentAutoApply := autoApplyState // snapshot at turn start
		currentSchemas := toolSchemasState // snapshot (empty = chat mode)

		go func() {
			defer close(ch)
			newHistory, loopErr := agenticLoopTUI(
				conn, cfg, cid, mdl, currentSchemas,
				history, isFirstTurn && isNew, currentAutoApply,
				maxTurnsState, auditLog, hookRunner, mcpMgr, p, ch,
			)
			history = newHistory
			turns++
			isFirstTurn = false
			// Persist session after every turn (#82).
			session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
			if loopErr != nil {
				if !caigrpc.IsAuthError(loopErr) {
					ch <- tui.StreamEvent{Kind: "error", ErrMsg: ui.RewriteError(loopErr)}
				}
			}
			ch <- tui.StreamEvent{Kind: "done"}
		}()

		return ch
	}

	tuiCfg := tui.SessionConfig{
		ChatID:         chatID,
		Model:          model,
		IsCode:         true,
		WorkDir:        workDir,
		AutoApply:      codeAutoApply,
		InitialPrompt:  initialPrompt,
		RepoName:       gitRepoName(),
		CustomCommands: loadCustomSlashCommands("."),
		SetAutoApplyFn: func(enabled bool) {
			autoApplyState = enabled
		},
		SetCodeModeFn: func(enabled bool) {
			if enabled {
				toolSchemasState = toolSchemas // restore full tool schemas
			} else {
				toolSchemasState = "" // clear tools → chat mode
			}
		},
	}
	m := tui.New(tuiCfg, submitFn)
	err = tui.Run(m)
	auditLog.LogSessionEnd(turns, "end_turn")
	return err
}

// runCodePrint runs the agentic loop in headless mode, writing output to stdout.
func runCodePrint(
	conn *caigrpc.Conn,
	cfg *config.Config,
	model, toolSchemas, prompt string,
	history []codeMessage,
	maxTurns int,
	autoApply bool,
	outputFormat string,
	sessPath string,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	p *ui.Printer,
) error {
	history = append(history, codeMessage{Role: "user", Content: prompt})
	chatID := uuid.New().String()

	ch := make(chan tui.StreamEvent, 128)

	var jsonEvents []map[string]any

	go func() {
		defer close(ch)
		newHistory, loopErr := agenticLoopTUI(
			conn, cfg, chatID, model, toolSchemas,
			history, true, autoApply,
			maxTurns, auditLog, hookRunner, mcpMgr, p, ch,
		)
		session.Save(sessPath, toSessionMsgs(newHistory)) //nolint:errcheck
		if loopErr != nil {
			ch <- tui.StreamEvent{Kind: "error", ErrMsg: ui.RewriteError(loopErr)}
		}
		ch <- tui.StreamEvent{Kind: "done"}
	}()

	var textBuf strings.Builder
	exitCode := 0

	for ev := range ch {
		switch ev.Kind {
		case "chunk":
			switch outputFormat {
			case "stream-json":
				enc := json.NewEncoder(os.Stdout)
				enc.Encode(map[string]any{"type": "text", "text": ev.Chunk}) //nolint:errcheck
			case "json":
				textBuf.WriteString(ev.Chunk)
			default:
				fmt.Print(ev.Chunk)
			}
		case "tool_call":
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(os.Stdout)
				enc.Encode(map[string]any{"type": "tool_use", "name": ev.ToolName, "input": ev.ToolArgs}) //nolint:errcheck
			}
		case "tool_exec":
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(os.Stdout)
				enc.Encode(map[string]any{"type": "tool_result", "name": ev.ToolName, "status": ev.Status, "duration_ms": ev.DurationMs}) //nolint:errcheck
			}
		case "error":
			exitCode = 1
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(os.Stdout)
				enc.Encode(map[string]any{"type": "error", "error": ev.ErrMsg}) //nolint:errcheck
			} else {
				fmt.Fprintln(os.Stderr, "error:", ev.ErrMsg)
			}
		case "done":
			result := map[string]any{"type": "result", "stop_reason": "end_turn"}
			switch outputFormat {
			case "stream-json":
				enc := json.NewEncoder(os.Stdout)
				enc.Encode(result) //nolint:errcheck
			case "json":
				result["text"] = textBuf.String()
				jsonEvents = append(jsonEvents, result)
				enc := json.NewEncoder(os.Stdout)
				enc.Encode(jsonEvents) //nolint:errcheck
			}
		}
		_ = jsonEvents
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// isTerminal returns true when stdout is connected to an interactive terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// toSessionMsgs converts internal codeMessages to the exported session.Message type.
func toSessionMsgs(msgs []codeMessage) []session.Message {
	out := make([]session.Message, len(msgs))
	for i, m := range msgs {
		out[i] = session.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
	}
	return out
}

const (
	rateLimitInitialDelay = 10 * time.Second
	rateLimitMaxDelay     = 5 * time.Minute
	rateLimitMaxRetries   = 3
)

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
	maxTurns int,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) ([]codeMessage, error) {
	if maxTurns <= 0 {
		maxTurns = 20
	}

	rateLimitRetries := 0
	var lastPromptTokens int32 // prompt token count from the most recent iteration

	for iteration := 0; iteration < maxTurns; iteration++ {
		// Compact context before starting a new iteration if we're approaching the
		// context window limit. Skip the very first iteration (no usage yet).
		if iteration > 0 && lastPromptTokens > compactionThreshold {
			ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf(
				"\n⚡ Context at ~%dk tokens — compacting history...\n",
				lastPromptTokens/1000,
			)}
			if compacted, compactErr := compactHistory(conn, chatID, model, history); compactErr == nil {
				history = compacted
				lastPromptTokens = 0
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: "✅ History compacted. Continuing...\n\n"}
			}
		}

		req := buildCodeRequest(chatID, model, toolSchemas, history, isFirstTurn && iteration == 0)

		ctx, cancel := context.WithCancel(context.Background())
		stream, err := conn.Client.Chat(ctx, req)
		if err != nil {
			cancel()
			// Rate limit backoff (#83)
			if st, ok := status.FromError(err); ok && st.Code() == codes.ResourceExhausted {
				if rateLimitRetries >= rateLimitMaxRetries {
					return history, err
				}
				delay := rateLimitDelay(rateLimitRetries)
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n⏳ Rate limited. Retrying in %s (attempt %d/%d)...\n", delay.Round(time.Second), rateLimitRetries+1, rateLimitMaxRetries)}
				time.Sleep(delay)
				rateLimitRetries++
				iteration-- // don't count this iteration
				continue
			}
			return history, err
		}
		rateLimitRetries = 0

		// Pump this iteration's stream into ch, collecting tool calls.
		toolCalls, usage, err := pumpCodeStream(stream, cancel, ch)
		if usage.PromptTokens > 0 {
			lastPromptTokens = usage.PromptTokens
		}
		if err != nil {
			// Rate limit backoff on stream errors too (#83)
			if st, ok := status.FromError(err); ok && st.Code() == codes.ResourceExhausted {
				if rateLimitRetries < rateLimitMaxRetries {
					delay := rateLimitDelay(rateLimitRetries)
					ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n⏳ Rate limited. Retrying in %s (attempt %d/%d)...\n", delay.Round(time.Second), rateLimitRetries+1, rateLimitMaxRetries)}
					time.Sleep(delay)
					rateLimitRetries++
					iteration--
					continue
				}
			}
			return history, err
		}
		rateLimitRetries = 0

		if len(toolCalls) == 0 {
			return history, nil
		}

		// Build assistant tool-call turn in history
		assistantMsg := codeMessage{Role: "assistant"}
		for _, tc := range toolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, codeToolCall{
				ID:       tc.ID,
				Type:     "function",
				Function: codeFuncCall{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		history = append(history, assistantMsg)

		// Execute tools — run concurrently when all can be auto-approved,
		// fall back to sequential when any requires a TUI approval prompt.
		toolResults := executeTools(toolCalls, autoApply, auditLog, hookRunner, mcpMgr, p, ch)
		for i, tc := range toolCalls {
			r := toolResults[i]
			history = append(history, codeMessage{
				Role:       "tool",
				Content:    r.result,
				ToolCallID: tc.ID,
			})
			if r.err != nil {
				ch <- tui.StreamEvent{
					Kind:     "tool_exec",
					ToolName: tc.Name,
					Status:   "error",
					Summary:  r.err.Error(),
				}
			}
		}
	}

	ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n⚠  Reached --max-turns limit (%d). Use --max-turns N to increase.\n", maxTurns)}
	return history, nil
}

// rateLimitDelay returns exponential backoff delay for the given retry index.
func rateLimitDelay(retry int) time.Duration {
	d := rateLimitInitialDelay * (1 << retry)
	if d > rateLimitMaxDelay {
		d = rateLimitMaxDelay
	}
	return d
}

// iterationUsage holds token counts reported by the backend for one agentic iteration.
type iterationUsage struct {
	PromptTokens     int32
	CompletionTokens int32
}

// pumpCodeStream reads a gRPC stream for one agentic iteration, forwarding
// content/tool events to ch and collecting tool_call events for return.
func pumpCodeStream(
	stream interface {
		Recv() (*pb.ChatEvent, error)
	},
	cancelFn context.CancelFunc,
	ch chan<- tui.StreamEvent,
) ([]ui.ToolCallEvent, iterationUsage, error) {
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
				return toolCalls, iterationUsage{}, nil
			}
			return toolCalls, iterationUsage{}, fmt.Errorf("stream recv: %w", err)
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
			return toolCalls, iterationUsage{
				PromptTokens:     ev.GetUsagePromptTokens(),
				CompletionTokens: ev.GetUsageCompletionTokens(),
			}, nil

		case "error", "stream_error":
			return toolCalls, iterationUsage{}, fmt.Errorf("%s", ev.GetError())

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

// compactionThreshold is the prompt-token count above which bai code compacts
// the conversation history before the next agentic iteration. 100k is safely
// below the 128k floor across all supported models (claude-sonnet 200k,
// gpt-4.1 128k) leaving room for the compaction summary itself.
const compactionThreshold int32 = 100_000

// compactHistory sends the full history to the LLM with a summarisation prompt
// (no tools, single turn) and returns a trimmed history containing only system
// messages, the summary, and the last four messages (two exchanges).
func compactHistory(
	conn *caigrpc.Conn,
	chatID, model string,
	history []codeMessage,
) ([]codeMessage, error) {
	const summaryPrompt = "Summarise the conversation so far in a single assistant message. " +
		"Include: the overall goal, key decisions, every file created or modified (with paths), " +
		"tool results that matter, and any open questions. Be concise but complete enough that " +
		"the session can continue without the original messages."

	summaryHistory := append(append([]codeMessage(nil), history...),
		codeMessage{Role: "user", Content: summaryPrompt},
	)

	// No tool schemas — one-shot text response only.
	req := buildCodeRequest(chatID, model, "", summaryHistory, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := conn.Client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	tf := &tui.ExportedThinkFilter{}
	for {
		ev, recvErr := stream.Recv()
		if recvErr != nil {
			break
		}
		switch ev.GetType() {
		case "content", "stream_chunk":
			buf.WriteString(tf.Filter(ev.GetContent()))
		case "done", "stream_end":
			goto summarised
		case "error", "stream_error":
			return nil, fmt.Errorf("compaction: %s", ev.GetError())
		}
	}
summarised:
	summary := strings.TrimSpace(buf.String())
	if summary == "" {
		return history, nil // nothing to compact
	}

	// Collect system messages from the original history.
	compact := make([]codeMessage, 0, 8)
	for _, m := range history {
		if m.Role == "system" {
			compact = append(compact, m)
		}
	}

	// Add the summary as an assistant message, then keep the last 4 messages
	// (the two most recent exchanges) for continuity.
	compact = append(compact, codeMessage{
		Role:    "assistant",
		Content: "[Conversation summary]\n" + summary,
	})
	tail := history
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	compact = append(compact, tail...)
	return compact, nil
}

// toolResult holds the outcome of one tool call.
type toolResult struct {
	result string
	err    error
}

// executeTools runs toolCalls sequentially or concurrently depending on whether
// any call requires interactive TUI approval. When all tools can be auto-approved
// (read-only ops, safe bash prefixes, --auto-apply) they execute in parallel,
// preserving result order. Sequential fallback is used when any tool needs a
// user prompt, to avoid multiple concurrent approval dialogs.
func executeTools(
	toolCalls []ui.ToolCallEvent,
	autoApply bool,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) []toolResult {
	// Fast path: single tool — no parallelism overhead.
	if len(toolCalls) <= 1 {
		results := make([]toolResult, len(toolCalls))
		for i, tc := range toolCalls {
			r, e := executeWithApprovalTUI(tc, autoApply, auditLog, hookRunner, mcpMgr, p, ch)
			results[i] = toolResult{result: r, err: e}
		}
		return results
	}

	// Check whether all tools can run without interactive approval.
	allAutoApproved := autoApply
	if !allAutoApproved {
		allAutoApproved = true
		for _, tc := range toolCalls {
			needsApproval := tools.NeedsApproval(tc.Name) || mcp.IsMCPTool(tc.Name)
			if needsApproval && (tc.Name != "bash" || !tools.IsSafeBashCommand(tc.Arguments)) {
				allAutoApproved = false
				break
			}
		}
	}

	if !allAutoApproved {
		// Sequential: approval dialogs must not overlap.
		results := make([]toolResult, len(toolCalls))
		for i, tc := range toolCalls {
			r, e := executeWithApprovalTUI(tc, autoApply, auditLog, hookRunner, mcpMgr, p, ch)
			results[i] = toolResult{result: r, err: e}
		}
		return results
	}

	// Parallel: all tools are auto-approved — run concurrently, collect in order.
	results := make([]toolResult, len(toolCalls))
	var wg sync.WaitGroup
	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, t ui.ToolCallEvent) {
			defer wg.Done()
			r, e := executeWithApprovalTUI(t, true, auditLog, hookRunner, mcpMgr, p, ch)
			results[idx] = toolResult{result: r, err: e}
		}(i, tc)
	}
	wg.Wait()
	return results
}

// executeWithApprovalTUI runs a tool, running hooks, audit logging, and the
// TUI approval flow as needed. MCP tools (mcp__<server>__<tool>) are routed
// through mcpMgr; all other tools go through the local tools.Execute.
func executeWithApprovalTUI(
	tc ui.ToolCallEvent,
	autoApply bool,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) (string, error) {
	// --- Pre-tool hooks (#80) ---
	hookResult := hookRunner.PreToolUse(tc.Name, tc.Arguments)
	if hookResult.Block {
		msg := hookResult.SystemMessage
		if msg == "" {
			msg = "Tool blocked by pre-tool hook."
		}
		auditLog.LogToolCall(tc.Name, tc.Arguments, false, false)
		auditLog.LogToolResult(tc.Name, 0, false)
		return msg, nil
	}
	// Use modified input if hook rewrote it.
	argsJSON := tc.Arguments
	if hookResult.ModifiedInput != nil {
		if b, err := json.Marshal(hookResult.ModifiedInput); err == nil {
			argsJSON = string(b)
		}
	}

	// --- Approval gate ---
	// MCP tools always require approval (they can do anything).
	needsApproval := tools.NeedsApproval(tc.Name) || mcp.IsMCPTool(tc.Name)
	autoApproved := false
	// Safe bash commands are auto-approved to reduce friction.
	if needsApproval && tc.Name == "bash" && tools.IsSafeBashCommand(argsJSON) {
		needsApproval = false
		autoApproved = true
	}
	approved := !needsApproval || autoApply
	if needsApproval && !autoApply {
		replyCh := make(chan bool, 1)
		ch <- tui.StreamEvent{
			Kind:     "approval",
			ToolName: tc.Name,
			ToolArgs: argsJSON,
			ReplyCh:  replyCh,
		}
		approved = <-replyCh
		if !approved {
			auditLog.LogToolCall(tc.Name, argsJSON, false, false)
			auditLog.LogToolResult(tc.Name, 0, false)
			return "User declined to execute this tool.", nil
		}
	}

	auditLog.LogToolCall(tc.Name, argsJSON, approved, autoApproved)

	// --- Execute ---
	start := time.Now()
	var result string
	var execErr error
	if mcp.IsMCPTool(tc.Name) {
		result, execErr = mcpMgr.Execute(context.Background(), tc.Name, argsJSON)
	} else {
		result, execErr = tools.Execute(tc.Name, argsJSON)
	}
	elapsed := time.Since(start)
	auditLog.LogToolResult(tc.Name, elapsed.Milliseconds(), execErr == nil)

	// --- Post-tool hooks (#80) ---
	hookRunner.PostToolUse(tc.Name, argsJSON, result)

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
	payload := cliCodePayload{V: cliPayloadVersion, History: history, Tools: toolSchemas}
	payloadJSON, _ := json.Marshal(payload)
	return &pb.ChatRequest{
		ChatId:    chatID,
		Model:     "cli/" + model,
		IsNewChat: isNewChat,
		Prompt:    string(payloadJSON),
	}
}
