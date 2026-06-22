// Package agent provides an embedded Go client for the bai agentic loop.
//
// Unlike sdk.Client (which drives a bai subprocess), this package runs the
// full loop in-process — no binary in PATH required. Use it from IDE extensions,
// test harnesses, or any Go program that needs programmatic agent access.
//
//	runner := agent.New(agent.Options{
//	    Model:    "auto",
//	    MaxTurns: 10,
//	    OnEvent: func(ev agent.Event) {
//	        if ev.Type == "text" {
//	            fmt.Print(ev.Text)
//	        }
//	    },
//	})
//	if err := runner.Run(context.Background(), "list the Go files here"); err != nil {
//	    log.Fatal(err)
//	}
package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	intagent "github.com/bluefunda/bluefunda-ai/internal/agent"
	"github.com/bluefunda/bluefunda-ai/internal/auth"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

// ── Public types ──────────────────────────────────────────────────────────────

// Event is a streaming event from the agentic loop. Type mirrors the
// --output-format stream-json event types used by the bai CLI.
type Event struct {
	Type       string // "text", "tool_use", "tool_result", "result", "error"
	Text       string // Type == "text"
	ToolID     string // Type == "tool_use" or "tool_result"
	ToolName   string // Type == "tool_use" or "tool_result"
	ToolInput  string // Type == "tool_use"
	ToolOutput string // Type == "tool_result"
	StopReason string // Type == "result": "end_turn" or "max_turns"
	InputToks  int32  // Type == "result"
	Err        error  // Type == "error"
}

// ToolCall is passed to OnToolCall when the LLM issues a tool request.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // JSON
}

// ToolResult is returned by OnToolCall.
type ToolResult struct {
	Output string
}

// Message is one turn in the conversation history.
type Message struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

// Options configures the embedded agent runner.
type Options struct {
	// Model is the LLM alias: "auto" (default), "fast", "think", or a full model ID.
	Model string
	// MaxTurns caps agentic iterations. 0 → 20.
	MaxTurns int
	// WorkDir is the working directory for tool operations. Defaults to os.Getwd().
	WorkDir string
	// OnEvent is called for every streaming event. May be nil.
	OnEvent func(Event)
	// OnToolCall intercepts tool execution. Return DefaultExecute(tc) to use the
	// default local tools implementation. May be nil (defaults to DefaultExecute).
	OnToolCall func(ToolCall) (ToolResult, error)
	// Allow and Deny are permission policy lists (same syntax as .bai/settings.yaml).
	Allow []string
	Deny  []string
}

// ── Runner ────────────────────────────────────────────────────────────────────

// Runner holds session state across Run / Continue calls.
type Runner struct {
	opts    Options
	chatID  string
	history []intagent.Message
	conn    *caigrpc.Conn
	cfg     *config.Config
}

// New creates a new Runner. The underlying gRPC connection is established on
// the first Run or Continue call.
func New(opts Options) *Runner {
	if opts.Model == "" {
		opts.Model = "auto"
	}
	return &Runner{opts: opts, chatID: uuid.New().String()}
}

// WithSystemPrompt injects a system message at the beginning of history.
// Must be called before Run.
func (r *Runner) WithSystemPrompt(prompt string) *Runner {
	r.history = append([]intagent.Message{{Role: "system", Content: prompt}}, r.history...)
	return r
}

// WithHistory seeds the runner with pre-existing conversation history.
// Must be called before Run.
func (r *Runner) WithHistory(msgs []Message) *Runner {
	r.history = toInternal(msgs)
	return r
}

// Run executes a prompt in the current session (continues history if any).
func (r *Runner) Run(ctx context.Context, prompt string) error {
	if err := r.ensureConn(); err != nil {
		return err
	}
	if r.opts.WorkDir != "" {
		if err := os.Chdir(r.opts.WorkDir); err != nil {
			return fmt.Errorf("chdir to %s: %w", r.opts.WorkDir, err)
		}
	}

	history := append(r.history, intagent.Message{Role: "user", Content: prompt})

	toolSchemas, err := tools.LocalToolSchemas()
	if err != nil {
		return fmt.Errorf("build tool schemas: %w", err)
	}

	loopOpts := intagent.LoopOptions{
		Conn:        r.conn,
		Model:       r.opts.Model,
		ToolSchemas: toolSchemas,
		History:     history,
		MaxTurns:    r.opts.MaxTurns,
		ChatID:      r.chatID,
		Allow:       r.opts.Allow,
		Deny:        r.opts.Deny,
		EventFn:     r.wrapEventFn(),
		ToolFn:      r.wrapToolFn(),
	}

	newHistory, err := intagent.RunLoop(ctx, loopOpts)
	r.history = newHistory
	return err
}

// Continue appends a follow-up prompt to the existing history and runs another turn.
func (r *Runner) Continue(ctx context.Context, prompt string) error {
	return r.Run(ctx, prompt)
}

// History returns the current conversation history.
func (r *Runner) History() []Message {
	return fromInternal(r.history)
}

// Close releases the underlying gRPC connection.
func (r *Runner) Close() error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// ── Default helpers ───────────────────────────────────────────────────────────

// DefaultExecute routes a tool call through the built-in local tools executor.
func DefaultExecute(tc ToolCall) (ToolResult, error) {
	out, err := tools.Execute(tc.Name, tc.Arguments)
	return ToolResult{Output: out}, err
}

// DefaultTools returns the JSON schema array for all built-in tools.
// Pass this to a custom runner that extends the default tool set.
func DefaultTools() (string, error) {
	return tools.LocalToolSchemas()
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (r *Runner) ensureConn() error {
	if r.conn != nil {
		// Refresh token if near expiry.
		if r.conn.TS.NearExpiry(2 * time.Minute) {
			_ = r.conn.TS.EnsureValidToken()
		}
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Auth.AccessToken == "" {
		return fmt.Errorf("not signed in — run `bai login`")
	}
	r.cfg = cfg

	refreshFunc := func() (string, error) {
		tok, err := auth.Refresh(cfg.Domain, cfg.Realm, cfg.Auth.RefreshToken)
		if err != nil {
			return "", err
		}
		cfg.Auth.AccessToken = tok.AccessToken
		cfg.Auth.RefreshToken = tok.RefreshToken
		cfg.Auth.TokenExpiry = tok.Expiry()
		_ = config.Save(cfg)
		return tok.AccessToken, nil
	}
	ts := caigrpc.NewTokenSource(cfg, refreshFunc)
	conn, err := caigrpc.Dial(cfg.BFFURL, ts)
	if err != nil {
		return fmt.Errorf("dial BFF: %w", err)
	}
	r.conn = conn
	return nil
}

func (r *Runner) wrapEventFn() func(intagent.Event) {
	if r.opts.OnEvent == nil {
		return func(intagent.Event) {}
	}
	return func(ev intagent.Event) {
		r.opts.OnEvent(Event{
			Type:       string(ev.Kind),
			Text:       ev.Text,
			ToolID:     ev.ToolID,
			ToolName:   ev.ToolName,
			ToolInput:  ev.ToolInput,
			ToolOutput: ev.ToolOutput,
			StopReason: ev.StopReason,
			InputToks:  ev.InputToks,
			Err:        ev.Err,
		})
	}
}

func (r *Runner) wrapToolFn() intagent.ToolExecutor {
	onToolCall := r.opts.OnToolCall
	if onToolCall == nil {
		return nil // loop will use tools.Execute directly
	}
	return func(ctx context.Context, name, args string) (string, error) {
		res, err := onToolCall(ToolCall{Name: name, Arguments: args})
		return res.Output, err
	}
}

func toInternal(msgs []Message) []intagent.Message {
	out := make([]intagent.Message, len(msgs))
	for i, m := range msgs {
		out[i] = intagent.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			out[i].ToolCalls = append(out[i].ToolCalls, intagent.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
	}
	return out
}

func fromInternal(msgs []intagent.Message) []Message {
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[i] = Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			out[i].ToolCalls = append(out[i].ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
	}
	return out
}
