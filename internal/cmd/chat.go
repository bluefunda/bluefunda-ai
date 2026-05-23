package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// chatCmd is kept for backward compatibility but hidden from help.
var chatCmd = &cobra.Command{
	Use:    "chat",
	Short:  "Chat operations",
	Hidden: true,
}

// --- chat list ---

var chatListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all chat sessions",
	RunE:  runChatList,
}

func runChatList(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetChatIds(ctx, &pb.GetChatIdsRequest{})
	if err != nil {
		return fmt.Errorf("get chats: %w", err)
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"CHAT_ID", "TITLE", "MODEL", "CREATED"}
	rows := make([][]string, 0, len(resp.GetChats()))
	for _, c := range resp.GetChats() {
		rows = append(rows, []string{
			c.GetChatId(),
			truncate(c.GetChatTitle(), 40),
			c.GetModel(),
			c.GetCreatedAt(),
		})
	}
	p.Table(headers, rows)
	return nil
}

// --- chat start (interactive REPL) ---

var (
	chatModel     string
	chatNew       bool
	chatMCPServer string
	chatDemo      bool
)

var chatStartCmd = &cobra.Command{
	Use:    "start [chatId | prompt]",
	Short:  "Start an interactive session",
	Long:   "Start an interactive session. Pass an existing chat UUID to resume it, a quoted prompt to auto-submit it, or omit the argument to start blank. Use --new to force a new session.",
	Args:   cobra.MaximumNArgs(1),
	Hidden: true,
	RunE:   runChatStart,
}

func init() {
	chatStartCmd.Flags().StringVar(&chatModel, "model", "", "LLM model to use")
	chatStartCmd.Flags().BoolVar(&chatNew, "new", false, "Force new session (generate UUID)")
	chatStartCmd.Flags().StringVar(&chatMCPServer, "mcp-server", "", "MCP server name")
	chatStartCmd.Flags().BoolVar(&chatDemo, "demo", false, "Run with a mock backend (no auth required)")

	chatCmd.AddCommand(chatListCmd, chatStartCmd, chatHistoryCmd, chatContextCmd, chatTitleCmd, chatStopCmd)
}

func runChatStart(cmd *cobra.Command, args []string) error {
	if chatDemo {
		return runChatDemo()
	}

	var chatID, initialPrompt string
	if len(args) > 0 && !chatNew {
		if _, err := uuid.Parse(args[0]); err == nil {
			chatID = args[0] // resume existing session by UUID
		} else {
			initialPrompt = args[0] // treat as initial prompt
		}
	} else if len(args) > 0 {
		initialPrompt = args[0] // chatNew=true, still use as prompt
	}

	return runChatSession(chatID, initialPrompt, chatModel, chatMCPServer)
}

// runChatSession is the shared entry point for interactive sessions.
// Called from the root command (bai) and the hidden chat start command.
// chatID is optional — empty means generate a new UUID (new session).
func runChatSession(chatID, initialPrompt, model, mcpServer string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	if model == "" {
		model = cfg.Defaults.Model
	}
	if model == "" {
		model = "openai"
	}

	if chatID == "" {
		chatID = uuid.New().String()
	}

	p := printer(cfg)
	var titleWg sync.WaitGroup

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

		req := &pb.ChatRequest{
			ChatId:        cid,
			Prompt:        input,
			Model:         model,
			IsNewChat:     isNew,
			McpServerName: mcpServer,
		}

		ctx, cancel := context.WithCancel(context.Background())
		stream, err := conn.Client.Chat(ctx, req)
		if err != nil {
			cancel()
			if caigrpc.IsAuthError(err) {
				if authErr := reAuthenticate(cfg, p); authErr == nil {
					ctx2, cancel2 := context.WithCancel(context.Background())
					stream, err = conn.Client.Chat(ctx2, req)
					if err != nil {
						cancel2()
					} else {
						ch := tui.PumpGRPCStream(stream, cancel2)
						if isNew {
							titleWg.Add(1)
							go generateTitle(conn, &titleWg, cid, input)
						}
						return ch
					}
				}
			}
			ch := make(chan tui.StreamEvent, 1)
			ch <- tui.StreamEvent{Kind: "error", ErrMsg: err.Error()}
			close(ch)
			return ch
		}

		if isNew {
			titleWg.Add(1)
			go generateTitle(conn, &titleWg, cid, input)
		}
		return tui.PumpGRPCStream(stream, cancel)
	}

	cfg2 := tui.SessionConfig{
		ChatID:        chatID,
		Model:         model,
		InitialPrompt: initialPrompt,
	}
	m := tui.New(cfg2, submitFn)
	if err := tui.Run(m); err != nil {
		return err
	}

	titleWg.Wait()
	return nil
}

func generateTitle(conn *caigrpc.Conn, wg *sync.WaitGroup, chatID, prompt string) {
	defer wg.Done()
	tCtx, tCancel := caigrpc.ContextWithTimeout()
	defer tCancel()
	_, _ = conn.Client.GenerateTitle(tCtx, &pb.GenerateTitleRequest{ChatId: chatID, Prompt: prompt})
}

// --- chat history ---

var chatHistoryCmd = &cobra.Command{
	Use:   "history <chatId>",
	Short: "Get message history for a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runChatHistory,
}

func runChatHistory(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetChatHistory(ctx, &pb.GetChatHistoryRequest{ChatId: args[0]})
	if err != nil {
		return fmt.Errorf("get history: %w", err)
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
		return nil
	}

	headers := []string{"ROLE", "CONTENT", "CREATED"}
	rows := make([][]string, 0, len(resp.GetMessages()))
	for _, m := range resp.GetMessages() {
		rows = append(rows, []string{
			m.GetRole(),
			truncate(m.GetContent(), 80),
			m.GetCreatedAt(),
		})
	}
	p.Table(headers, rows)
	return nil
}

// --- chat context ---

var chatContextCmd = &cobra.Command{
	Use:   "context <chatId>",
	Short: "Get context for a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runChatContext,
}

func runChatContext(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GetChatContext(ctx, &pb.GetChatContextRequest{ChatId: args[0]})
	if err != nil {
		return fmt.Errorf("get context: %w", err)
	}

	printer(cfg).ProtoJSON(resp)
	return nil
}

// --- chat title ---

var chatTitlePrompt string

var chatTitleCmd = &cobra.Command{
	Use:   "title <chatId>",
	Short: "Generate a title for a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runChatTitle,
}

func init() {
	chatTitleCmd.Flags().StringVar(&chatTitlePrompt, "prompt", "", "Prompt hint for title generation")
}

func runChatTitle(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.GenerateTitle(ctx, &pb.GenerateTitleRequest{
		ChatId: args[0],
		Prompt: chatTitlePrompt,
	})
	if err != nil {
		return fmt.Errorf("generate title: %w", err)
	}

	if resp.GetError() != "" {
		return fmt.Errorf("title generation: %s", resp.GetError())
	}

	p := printer(cfg)
	if p.Format == ui.FormatJSON {
		p.ProtoJSON(resp)
	} else {
		p.Success(resp.GetGeneratedTitle())
	}
	return nil
}

// --- chat stop ---

var chatStopCmd = &cobra.Command{
	Use:   "stop <chatId>",
	Short: "Stop a streaming session",
	Args:  cobra.ExactArgs(1),
	RunE:  runChatStop,
}

func runChatStop(cmd *cobra.Command, args []string) error {
	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := caigrpc.ContextWithTimeout()
	defer cancel()

	resp, err := conn.Client.StopChat(ctx, &pb.StopChatRequest{ChatId: args[0]})
	if err != nil {
		return fmt.Errorf("stop session: %w", err)
	}

	p := printer(cfg)
	if resp.GetSuccess() {
		p.Success("Session stopped")
	} else {
		p.Error("Failed to stop session")
	}
	return nil
}

// --- demo mode ---

func runChatDemo() error {
	submitFn := func(chatID, input string, isNew bool) <-chan tui.StreamEvent {
		ch := make(chan tui.StreamEvent, 128)
		go func() {
			defer close(ch)
			demoRespond(input, ch)
		}()
		return ch
	}

	cfg := tui.SessionConfig{
		ChatID: uuid.New().String(),
		Model:  "demo",
	}
	return tui.Run(tui.New(cfg, submitFn))
}

// demoRespond simulates a realistic assistant response for UI preview.
func demoRespond(input string, ch chan<- tui.StreamEvent) {
	time.Sleep(180 * time.Millisecond)
	ch <- tui.StreamEvent{Kind: "heartbeat"}
	time.Sleep(120 * time.Millisecond)

	lc := strings.ToLower(input)
	if strings.Contains(lc, "read") || strings.Contains(lc, "file") ||
		strings.Contains(lc, "code") || strings.Contains(lc, "list") {
		ch <- tui.StreamEvent{Kind: "heartbeat"}
		time.Sleep(80 * time.Millisecond)
		ch <- tui.StreamEvent{
			Kind:     "tool_call",
			ToolName: "read_file",
			ToolArgs: `{"path":"src/main.go"}`,
		}
		time.Sleep(240 * time.Millisecond)
		ch <- tui.StreamEvent{
			Kind:       "tool_exec",
			ToolName:   "read_file",
			Status:     "ok",
			DurationMs: 12,
			Summary:    "214 lines",
		}
		time.Sleep(80 * time.Millisecond)
	}

	response := buildDemoResponse(input)
	words := strings.Fields(response)
	buf := ""
	for i, w := range words {
		buf += w
		if i < len(words)-1 {
			buf += " "
		}
		if len(buf) >= 6 || i == len(words)-1 {
			ch <- tui.StreamEvent{Kind: "chunk", Chunk: buf}
			buf = ""
			time.Sleep(22 * time.Millisecond)
		}
	}
	ch <- tui.StreamEvent{Kind: "done"}
}

func buildDemoResponse(input string) string {
	lc := strings.ToLower(input)
	switch {
	case strings.Contains(lc, "hello") || strings.Contains(lc, "hi"):
		return "Hello! I'm your AI pair programmer. How can I help you today?\n\nYou can ask me to:\n- **Read** and explain code files\n- **Write** or edit code\n- **Run** shell commands\n- **Search** your project\n\nType `/help` to see available commands."

	case strings.Contains(lc, "help"):
		return "Here's what I can do:\n\n## File Operations\n- `read_file` — read any file\n- `write_file` — create or update files\n- `list_dir` — browse directories\n- `search_files` — glob pattern search\n\n## Shell\n- `bash` — run shell commands\n\n## Slash Commands\nType `/` to see the command palette."

	case strings.Contains(lc, "code") || strings.Contains(lc, "file"):
		return "I've read the file. Here's a summary:\n\n```go\nfunc main() {\n    if err := cmd.Execute(); err != nil {\n        os.Exit(1)\n    }\n}\n```\n\nThis is the entry point. It delegates to the cobra command tree in `internal/cmd/`. Want me to explore any specific part?"

	case strings.Contains(lc, "test"):
		return "Running the test suite:\n\n```\nok   internal/cmd       0.031s\nok   internal/config    0.004s\nok   internal/grpc      0.002s\nok   internal/ui        0.008s\n```\n\nAll **4 packages** passed."

	default:
		return fmt.Sprintf("I understand you're asking about: *%s*\n\nThis is a **demo mode** response — connect to a real backend with `bai --new` to get actual AI responses.\n\nTry asking me to read a file or help with code.", input)
	}
}

// --- helpers ---

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
