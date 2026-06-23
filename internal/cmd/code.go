package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
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
	"github.com/bluefunda/bluefunda-ai/internal/plugins"
	"github.com/bluefunda/bluefunda-ai/internal/session"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
	"github.com/bluefunda/bluefunda-ai/internal/worktree"
)

var (
	codeModel            string
	codeDir              string
	codeAutoApply        bool
	codeAuto             bool
	codeMaxTurns         int
	codeMaxContextTokens int     // 0 = use defaultCompactionThreshold
	codeMaxBudgetUSD     float64 // 0 = no limit
	codePrint            bool
	codeOutputFormat     string
	codeResume           string
	codeContinue         bool
	codeNoTools          bool
	codeWorktree         bool
)

// codeCmd is a deprecated alias for the root 'bai' command.
// It is hidden from help but preserved for backward compatibility.
var codeCmd = &cobra.Command{
	Use:    "code [prompt]",
	Short:  "Deprecated: use 'bai' directly",
	Hidden: true,
	Args:   cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Note: 'bai code' is now just 'bai'. This alias will be removed in a future release.")
		return runAgenticSession(args)
	},
}

func init() {
	// Keep flags on codeCmd so existing scripts using 'bai code --auto' etc. still work.
	codeCmd.Flags().StringVar(&codeModel, "model", "", "LLM model to use")
	codeCmd.Flags().StringVar(&codeDir, "dir", ".", "Working directory for file operations")
	codeCmd.Flags().BoolVar(&codeAutoApply, "auto-apply", false, "Execute write/bash tools without prompting")
	codeCmd.Flags().BoolVar(&codeAuto, "auto", false, "Same as --auto-apply")
	codeCmd.Flags().IntVar(&codeMaxTurns, "max-turns", 20, "Maximum agentic loop iterations before stopping")
	codeCmd.Flags().IntVar(&codeMaxContextTokens, "max-context-tokens", 0, "Max context tokens before auto-compaction (default 100000; env BAI_MAX_CONTEXT_TOKENS)")
	codeCmd.Flags().Float64Var(&codeMaxBudgetUSD, "max-budget-usd", 0, "Stop session when estimated cost exceeds this USD amount (0 = no limit; env BAI_MAX_BUDGET_USD)")
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

func runAgenticSession(args []string) error {
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
	model = resolveModelAlias(model)

	initialPrompt := strings.Join(args, " ")

	if err := os.Chdir(codeDir); err != nil {
		return fmt.Errorf("chdir to %s: %w", codeDir, err)
	}

	// --- Worktree isolation (#126) ---
	// When --worktree is set, create a detached git worktree and switch the
	// process cwd into it so all tool file/bash operations go there instead of
	// the user's main working tree.
	var (
		worktreePath string
		gitRootPath  string
	)
	if codeWorktree {
		if codePrint {
			return fmt.Errorf("--worktree is not supported in --print / headless mode")
		}
		var err error
		gitRootPath, err = worktree.GitRoot()
		if err != nil {
			return fmt.Errorf("--worktree requires a git repository: %w", err)
		}
		// sessionID is determined later; use a placeholder UUID for the worktree path.
		// We reassign sessionID below before using it for session persistence.
	}

	// Apply project-level max_turns override if the flag wasn't set explicitly.
	if projCfgEarly := config.FindProjectConfig("."); projCfgEarly != nil {
		if projCfgEarly.MaxTurns > 0 && codeMaxTurns == 20 {
			// project config wins when the user hasn't explicitly set --max-turns
			codeMaxTurns = projCfgEarly.MaxTurns
		}
	}

	// Resolve effective context token limit: CLI flag > BAI_MAX_CONTEXT_TOKENS > default.
	maxContextTokens := codeMaxContextTokens
	if maxContextTokens == 0 {
		if v := os.Getenv("BAI_MAX_CONTEXT_TOKENS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				maxContextTokens = n
			}
		}
	}
	if maxContextTokens == 0 {
		maxContextTokens = defaultCompactionThreshold
	}

	// Resolve effective budget limit: CLI flag > BAI_MAX_BUDGET_USD > 0 (no limit).
	maxBudgetUSD := codeMaxBudgetUSD
	if maxBudgetUSD == 0 {
		if v := os.Getenv("BAI_MAX_BUDGET_USD"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				maxBudgetUSD = f
			}
		}
	}

	var toolSchemas string
	if !codeNoTools {
		var err error
		toolSchemas, err = tools.LocalToolSchemas()
		if err != nil {
			return fmt.Errorf("build tool schemas: %w", err)
		}
	}

	workDir, _ := os.Getwd()
	p := printer(cfg)

	// --- Local MCP servers (#85) ---
	// Start servers defined in .bai/settings.yaml mcp_servers and merge their
	// tools into the schema sent to the LLM on every agentic turn.
	projCfg := config.FindProjectConfig(".")
	mcpMgr := mcp.NewManager(context.Background(), projCfg)

	// --- Permission policy (#123) ---
	// Load allow/deny lists from the project config (empty = no restrictions).
	var permAllow, permDeny []string
	if projCfg != nil {
		permAllow = projCfg.Permissions.Allow
		permDeny = projCfg.Permissions.Deny
	}
	defer mcpMgr.Close()
	if extra := mcpMgr.ToolSchemas(); len(extra) > 0 {
		merged, mergeErr := tools.MergeSchemas(toolSchemas, extra)
		if mergeErr == nil {
			toolSchemas = merged
		}
	}

	// --- Plugin tools (#128) ---
	// Load .bai/plugins/*/plugin.yaml from user-level and project directories.
	pluginMgr := plugins.NewManager(".")
	if extra := pluginMgr.ToolSchemas(); len(extra) > 0 {
		merged, mergeErr := tools.MergeSchemas(toolSchemas, extra)
		if mergeErr == nil {
			toolSchemas = merged
		}
		for _, p := range pluginMgr.All() {
			fmt.Printf("[bai] plugin %s: loaded (%s)\n", p.Manifest.Name, p.SourcePath)
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

	// Create the worktree now that we have a stable sessionID.
	if codeWorktree && gitRootPath != "" {
		var err error
		worktreePath, err = worktree.Create(gitRootPath, sessionID)
		if err != nil {
			return fmt.Errorf("create worktree: %w", err)
		}
		if err := os.Chdir(worktreePath); err != nil {
			_ = worktree.Remove(gitRootPath, worktreePath, true)
			return fmt.Errorf("chdir to worktree: %w", err)
		}
		// Recompute workDir — everything from here on uses the worktree path.
		workDir, _ = os.Getwd()
		p.Info(fmt.Sprintf("Isolated worktree: %s", worktreePath))
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
		return runCodePrint(conn, cfg, sessionID, model, toolSchemas, initialPrompt, history,
			codeMaxTurns, maxContextTokens, maxBudgetUSD, permAllow, permDeny, codeAutoApply, codeOutputFormat, sessPath, auditLog, hookRunner, mcpMgr, pluginMgr, p)
	}

	// --- Interactive TUI mode ---
	// Use sessionID as the chatID so all turns of the same session share one
	// identifier on the server. Previously a new UUID was generated per turn,
	// making server-side grouping impossible (#183).
	chatID := sessionID

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
				history, isFirstTurn && isNew, permAllow, permDeny, currentAutoApply,
				maxTurnsState, maxContextTokens, maxBudgetUSD, sessPath, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch,
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

	// --- Worktree teardown (#126) ---
	if worktreePath != "" {
		if wtErr := handleWorktreeEnd(worktreePath, gitRootPath, p); wtErr != nil {
			fmt.Fprintf(os.Stderr, "worktree cleanup: %v\n", wtErr)
		}
	}

	return err
}

// handleWorktreeEnd shows changed files and prompts the user to apply, discard,
// or keep the isolated worktree created by --worktree.
func handleWorktreeEnd(worktreePath, gitRootPath string, p *ui.Printer) error {
	changed, _ := worktree.ChangedFiles(worktreePath)

	if len(changed) == 0 {
		p.Info("No changes in worktree — cleaning up.")
		return worktree.Remove(gitRootPath, worktreePath, true)
	}

	fmt.Printf("\n%d file(s) changed in worktree:\n", len(changed))
	for _, f := range changed {
		fmt.Printf("  %s\n", f)
	}
	fmt.Printf("\n[A]pply to main tree  [D]iscard worktree  [K]eep worktree\n> ")

	var choice string
	_, _ = fmt.Scanln(&choice)
	choice = strings.ToLower(strings.TrimSpace(choice))

	switch {
	case strings.HasPrefix(choice, "a"):
		if err := worktree.Apply(worktreePath, gitRootPath); err != nil {
			return fmt.Errorf("apply changes: %w", err)
		}
		_ = worktree.Remove(gitRootPath, worktreePath, true)
		p.Success("Changes applied to main tree.")
	case strings.HasPrefix(choice, "k"):
		p.Info(fmt.Sprintf("Worktree kept at: %s", worktreePath))
	default:
		_ = worktree.Remove(gitRootPath, worktreePath, true)
		p.Info("Worktree discarded. Main tree unchanged.")
	}
	return nil
}

// runCodePrint runs the agentic loop in headless mode, writing output to stdout.
func runCodePrint(
	conn *caigrpc.Conn,
	cfg *config.Config,
	chatID, model, toolSchemas, prompt string,
	history []codeMessage,
	maxTurns int,
	maxContextTokens int,
	maxBudgetUSD float64,
	allow, deny []string,
	autoApply bool,
	outputFormat string,
	sessPath string,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
	p *ui.Printer,
) error {
	history = append(history, codeMessage{Role: "user", Content: prompt})

	ch := make(chan tui.StreamEvent, 128)

	var jsonEvents []map[string]any

	go func() {
		defer close(ch)
		newHistory, loopErr := agenticLoopTUI(
			conn, cfg, chatID, model, toolSchemas,
			history, true, allow, deny, autoApply,
			maxTurns, maxContextTokens, maxBudgetUSD, sessPath, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch,
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
	allow, deny []string,
	autoApply bool,
	maxTurns int,
	maxContextTokens int,
	maxBudgetUSD float64,
	sessPath string,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) ([]codeMessage, error) {
	if maxTurns <= 0 {
		maxTurns = 20
	}
	if maxContextTokens <= 0 {
		maxContextTokens = defaultCompactionThreshold
	}

	// Generate a chat title from the first user prompt on new sessions (#182).
	// Fire-and-forget: title generation failure is silently ignored.
	if isFirstTurn {
		for _, m := range history {
			if m.Role == "user" && m.Content != "" {
				prompt := m.Content
				if len(prompt) > 200 {
					prompt = prompt[:200]
				}
				go func(p string) {
					tCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, _ = conn.Client.GenerateTitle(tCtx, &pb.GenerateTitleRequest{ChatId: chatID, Prompt: p})
				}(prompt)
				break
			}
		}
	}

	rateLimitRetries := 0
	var lastPromptTokens int32 // prompt token count from the most recent iteration
	var estimatedCostUSD float64 // cumulative estimated cost for this session turn

	for iteration := 0; iteration < maxTurns; iteration++ {
		// Compact context when the client-side estimate or server-reported token count
		// exceeds the configured limit. Client-side estimate fires even on iteration 0
		// (e.g. resumed history already large). Server count is a secondary signal.
		estTokens := estimateTokens(history)
		if estTokens >= maxContextTokens || (iteration > 0 && lastPromptTokens > int32(maxContextTokens)) {
			fmt.Fprintf(os.Stderr, "[bai] compacting context (est. ~%dk tokens → summary)…\n", estTokens/1000)
			ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf(
				"\n⚡ Context at ~%dk tokens — compacting history...\n",
				estTokens/1000,
			)}
			if compacted, compactErr := compactHistory(conn, chatID, model, history); compactErr == nil {
				history = compacted
				lastPromptTokens = 0
				session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: "✅ History compacted. Continuing...\n\n"}
			} else {
				fmt.Fprintf(os.Stderr, "[bai] context compaction failed, dropping oldest turns\n")
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: "\n⚠ Compaction failed — dropping oldest turns...\n"}
				history = dropOldestTurns(history, maxContextTokens*4/5)
				lastPromptTokens = 0
				session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
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
			// Request entity too large (413) — history is still too big even after
			// compaction. Drop aggressively to half the threshold and retry once.
			if strings.Contains(err.Error(), "413") {
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: "\n⚠ Request too large — dropping older messages and retrying...\n"}
				history = dropOldestTurns(history, maxContextTokens/2)
				lastPromptTokens = 0
				session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
				iteration--
				continue
			}
			return history, err
		}
		rateLimitRetries = 0

		// Budget check: accumulate estimated cost and stop before the next iteration.
		if maxBudgetUSD > 0 {
			estimatedCostUSD += estimateIterationCost(model, usage, estimateTokens(history))
			if estimatedCostUSD >= maxBudgetUSD {
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf(
					"\n💰 Session budget limit $%.2f reached (est. $%.4f used). Stopping.\n",
					maxBudgetUSD, estimatedCostUSD,
				)}
				return history, fmt.Errorf("budget limit $%.2f reached (est. $%.4f used)", maxBudgetUSD, estimatedCostUSD)
			}
		}

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
		toolResults := executeTools(toolCalls, allow, deny, autoApply, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
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

// defaultCompactionThreshold is the default max context tokens before compaction.
// 100k is safely below the 128k floor across all supported models (claude-sonnet
// 200k, gpt-4.1 128k), leaving headroom for the compaction summary itself.
const defaultCompactionThreshold = 100_000

// estimateTokens counts characters in all message content and tool-call
// arguments and divides by 4 (standard chars-per-token approximation).
func estimateTokens(history []codeMessage) int {
	chars := 0
	for _, m := range history {
		chars += len(m.Content)
		for _, tc := range m.ToolCalls {
			chars += len(tc.Function.Arguments)
		}
	}
	return chars / 4
}

// dropOldestTurns removes non-system messages from the front of history until
// estimateTokens is below targetTokens. System messages are always preserved.
func dropOldestTurns(history []codeMessage, targetTokens int) []codeMessage {
	var sysMsgs, convo []codeMessage
	for _, m := range history {
		if m.Role == "system" {
			sysMsgs = append(sysMsgs, m)
		} else {
			convo = append(convo, m)
		}
	}
	for len(convo) > 0 && estimateTokens(append(sysMsgs, convo...)) > targetTokens {
		convo = convo[1:]
	}
	return append(sysMsgs, convo...)
}

// modelPrice holds approximate per-million-token pricing for one model.
type modelPrice struct {
	inputPerMToken  float64 // USD per 1M prompt tokens
	outputPerMToken float64 // USD per 1M completion tokens
}

// modelPricing returns approximate pricing for a model name or alias.
// Prices are estimates as of mid-2026 and may drift — they are used only
// for session-level budget enforcement, not billing.
func modelPricing(model string) modelPrice {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return modelPrice{15.00, 75.00}
	case strings.Contains(m, "haiku"):
		return modelPrice{0.25, 1.25}
	case strings.Contains(m, "gpt-4o"):
		return modelPrice{2.50, 10.00}
	case strings.Contains(m, "gpt-4.1") || strings.Contains(m, "gpt-4"):
		return modelPrice{2.00, 8.00}
	case m == "fast": // Groq llama-class
		return modelPrice{0.06, 0.18}
	default: // claude-sonnet, auto, think, unknown — conservative estimate
		return modelPrice{3.00, 15.00}
	}
}

// estimateIterationCost computes the approximate USD cost of one agentic iteration.
// When the backend provides usage counts they are used directly; otherwise prompt
// tokens are estimated from the current history size and completion tokens are
// estimated at 15% of prompt (conservative for code tasks).
func estimateIterationCost(model string, usage iterationUsage, historyTokens int) float64 {
	pt := int(usage.PromptTokens)
	ct := int(usage.CompletionTokens)
	if pt == 0 {
		pt = historyTokens
	}
	if ct == 0 {
		ct = pt * 15 / 100
	}
	p := modelPricing(model)
	return float64(pt)/1_000_000*p.inputPerMToken + float64(ct)/1_000_000*p.outputPerMToken
}

// compactionSafeTokens is the maximum estimated tokens sent to the LLM for
// the summarisation request. Capping the compaction input prevents the
// compaction request itself from triggering a 413 on the server when the
// history has grown very large.
const compactionSafeTokens = 50_000

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

	// Pre-truncate so the compaction request itself stays below server size limits.
	// We keep the most recent messages (most context-relevant) and system messages.
	safeHistory := history
	if estimateTokens(history) > compactionSafeTokens {
		safeHistory = dropOldestTurns(history, compactionSafeTokens)
	}

	summaryHistory := append(append([]codeMessage(nil), safeHistory...),
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
			if recvErr != io.EOF {
				return nil, fmt.Errorf("compaction recv: %w", recvErr)
			}
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
	allow, deny []string,
	autoApply bool,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) []toolResult {
	// Fast path: single tool — no parallelism overhead.
	if len(toolCalls) <= 1 {
		results := make([]toolResult, len(toolCalls))
		for i, tc := range toolCalls {
			r, e := executeWithApprovalTUI(tc, allow, deny, autoApply, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
			results[i] = toolResult{result: r, err: e}
		}
		return results
	}

	// Check whether all tools can run without interactive approval.
	// Denied tools return immediately (no dialog); allow-matched tools skip the prompt.
	allAutoApproved := autoApply
	if !allAutoApproved {
		allAutoApproved = true
		for _, tc := range toolCalls {
			action := tools.CheckPermissions(allow, deny, tc.Name, tc.Arguments)
			if action == tools.PermitDeny || action == tools.PermitAuto {
				continue
			}
			if plugins.IsPluginTool(tc.Name) && pluginMgr.ApprovalMode(tc.Name) == "never" {
				continue
			}
			needsApproval := tools.NeedsApproval(tc.Name) || mcp.IsMCPTool(tc.Name) || plugins.IsPluginTool(tc.Name)
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
			r, e := executeWithApprovalTUI(tc, allow, deny, autoApply, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
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
			r, e := executeWithApprovalTUI(t, allow, deny, true, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
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
	allow, deny []string,
	autoApply bool,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
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

	// --- Permission policy (#123) ---
	// Evaluate project allow/deny lists before the TUI approval gate.
	// Deny-first: a matching deny rule blocks execution immediately.
	// Allow match: skip the NeedsApproval prompt for this call.
	effectiveAutoApply := autoApply
	switch tools.CheckPermissions(allow, deny, tc.Name, argsJSON) {
	case tools.PermitDeny:
		auditLog.LogToolCall(tc.Name, argsJSON, false, false)
		auditLog.LogToolResult(tc.Name, 0, false)
		return fmt.Sprintf("Tool call blocked by permissions policy: %s", tc.Name), nil
	case tools.PermitAuto:
		effectiveAutoApply = true
	}

	// --- Approval gate ---
	// MCP and plugin tools always require approval by default (they can do anything).
	needsApproval := tools.NeedsApproval(tc.Name) || mcp.IsMCPTool(tc.Name) || plugins.IsPluginTool(tc.Name)
	autoApproved := false
	// Safe bash commands are auto-approved to reduce friction.
	if needsApproval && tc.Name == "bash" && tools.IsSafeBashCommand(argsJSON) {
		needsApproval = false
		autoApproved = true
	}
	// Plugin-specific approval mode overrides the default and the --auto flag.
	if plugins.IsPluginTool(tc.Name) {
		switch pluginMgr.ApprovalMode(tc.Name) {
		case "always":
			needsApproval = true
			effectiveAutoApply = false // force dialog even with --auto or allow rule
		case "never":
			needsApproval = false
			autoApproved = true
		}
	}
	approved := !needsApproval || effectiveAutoApply
	if needsApproval && !effectiveAutoApply {
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
	switch {
	case mcp.IsMCPTool(tc.Name):
		result, execErr = mcpMgr.Execute(context.Background(), tc.Name, argsJSON)
	case plugins.IsPluginTool(tc.Name):
		result, execErr = pluginMgr.Execute(context.Background(), tc.Name, argsJSON)
	default:
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
