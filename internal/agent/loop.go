// Package agent provides an in-process agentic loop used by sdk/agent.
// It mirrors the logic in internal/cmd/code.go but replaces TUI channels and
// approval dialogs with plain callbacks, making it embeddable in editors and
// test harnesses without spawning a bai subprocess.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// ── Public types ──────────────────────────────────────────────────────────────

// EventKind identifies the kind of streaming event emitted by the loop.
type EventKind string

const (
	EventText       EventKind = "text"       // streamed LLM text chunk
	EventToolUse    EventKind = "tool_use"   // tool call issued by the LLM
	EventToolResult EventKind = "tool_result" // tool execution result
	EventResult     EventKind = "result"     // session complete
	EventError      EventKind = "error"      // non-fatal error
)

// Event is one streaming event from the agentic loop.
type Event struct {
	Kind       EventKind
	Text       string    // EventText
	ToolID     string    // EventToolUse, EventToolResult
	ToolName   string    // EventToolUse, EventToolResult
	ToolInput  string    // EventToolUse
	ToolOutput string    // EventToolResult
	StopReason string    // EventResult
	InputToks  int32     // EventResult
	OutputToks int32     // EventResult
	Err        error     // EventError
}

// Message is one turn in the conversation history.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"-"` // encoded separately for the wire format
}

// ToolCall is a tool invocation from the LLM.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ToolExecutor executes a tool and returns its output. A non-nil error is
// reported back to the LLM as a tool failure (the loop does not stop).
type ToolExecutor func(ctx context.Context, name, argumentsJSON string) (string, error)

// LoopOptions configures a single RunLoop invocation.
type LoopOptions struct {
	// Conn is the authenticated gRPC connection to the BFF. Required.
	Conn *caigrpc.Conn
	// Model is the resolved model alias (e.g. "auto", "fast").
	Model string
	// ToolSchemas is the JSON-encoded array of tool schemas. Empty = no tools.
	ToolSchemas string
	// History seeds the conversation (system message + prior turns).
	History []Message
	// MaxTurns caps agentic iterations. 0 defaults to 20.
	MaxTurns int
	// ChatID is a stable identifier for this session. Generated if empty.
	ChatID string
	// Allow/Deny are the permission policy lists (see internal/tools/permissions).
	Allow []string
	Deny  []string
	// EventFn is called synchronously for every streaming event. May be nil.
	EventFn func(Event)
	// ToolFn executes each tool call. Defaults to tools.Execute when nil.
	ToolFn ToolExecutor
}

// ── Core loop ─────────────────────────────────────────────────────────────────

const (
	defaultMaxTurns        = 20
	rateLimitInitialDelay  = 10 * time.Second
	rateLimitMaxDelay      = 5 * time.Minute
	rateLimitMaxRetries    = 3
	compactionThreshold    = int32(100_000)
	cliPayloadVersion      = 1
)

// RunLoop executes the agentic loop synchronously. It appends the user prompt
// to history, runs up to MaxTurns iterations, and returns the final history.
func RunLoop(ctx context.Context, opts LoopOptions) ([]Message, error) {
	if opts.MaxTurns <= 0 {
		opts.MaxTurns = defaultMaxTurns
	}
	if opts.ChatID == "" {
		opts.ChatID = uuid.New().String()
	}
	if opts.ToolFn == nil {
		opts.ToolFn = func(_ context.Context, name, args string) (string, error) {
			return tools.Execute(name, args)
		}
	}
	emit := opts.EventFn
	if emit == nil {
		emit = func(Event) {}
	}

	history := opts.History
	rateLimitRetries := 0
	var lastPromptTokens int32

	for iteration := 0; iteration < opts.MaxTurns; iteration++ {
		// Auto-compact context when approaching token limit.
		if iteration > 0 && lastPromptTokens > compactionThreshold {
			emit(Event{Kind: EventText, Text: fmt.Sprintf("\n⚡ Context at ~%dk tokens — compacting history...\n", lastPromptTokens/1000)})
			if compacted, err := compact(ctx, opts.Conn, opts.ChatID, opts.Model, history); err == nil {
				history = compacted
				lastPromptTokens = 0
				emit(Event{Kind: EventText, Text: "✅ History compacted. Continuing...\n\n"})
			}
		}

		req := buildRequest(opts.ChatID, opts.Model, opts.ToolSchemas, history, iteration == 0)
		iterCtx, cancel := context.WithCancel(ctx)
		stream, err := opts.Conn.Client.Chat(iterCtx, req)
		if err != nil {
			cancel()
			if st, ok := status.FromError(err); ok && st.Code() == codes.ResourceExhausted {
				if rateLimitRetries >= rateLimitMaxRetries {
					return history, err
				}
				d := rateLimitBackoff(rateLimitRetries)
				emit(Event{Kind: EventText, Text: fmt.Sprintf("\n⏳ Rate limited. Retrying in %s (%d/%d)...\n", d.Round(time.Second), rateLimitRetries+1, rateLimitMaxRetries)})
				time.Sleep(d)
				rateLimitRetries++
				iteration--
				continue
			}
			return history, err
		}
		rateLimitRetries = 0

		toolCalls, usage, err := pumpStream(stream, cancel, emit)
		if usage.promptTokens > 0 {
			lastPromptTokens = usage.promptTokens
		}
		if err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.ResourceExhausted && rateLimitRetries < rateLimitMaxRetries {
				d := rateLimitBackoff(rateLimitRetries)
				emit(Event{Kind: EventText, Text: fmt.Sprintf("\n⏳ Rate limited. Retrying in %s (%d/%d)...\n", d.Round(time.Second), rateLimitRetries+1, rateLimitMaxRetries)})
				time.Sleep(d)
				rateLimitRetries++
				iteration--
				continue
			}
			return history, err
		}
		rateLimitRetries = 0

		if len(toolCalls) == 0 {
			emit(Event{Kind: EventResult, StopReason: "end_turn", InputToks: lastPromptTokens})
			return history, nil
		}

		// Append assistant tool-call message.
		assistantMsg := Message{Role: "assistant"}
		assistantMsg.ToolCalls = toolCalls
		history = append(history, assistantMsg)

		// Execute tools (parallel when all auto-approved by policy).
		results := executeTools(ctx, toolCalls, opts.Allow, opts.Deny, opts.ToolFn, emit)
		for i, tc := range toolCalls {
			history = append(history, Message{
				Role:       "tool",
				Content:    results[i],
				ToolCallID: tc.ID,
			})
		}
	}

	emit(Event{Kind: EventText, Text: fmt.Sprintf("\n⚠  Reached max-turns limit (%d).\n", opts.MaxTurns)})
	emit(Event{Kind: EventResult, StopReason: "max_turns"})
	return history, nil
}

// ── Wire format ───────────────────────────────────────────────────────────────

// wireMessage is the JSON wire format for cai-llm-router (same as codeMessage).
type wireMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	ToolCalls  []wireToolCall  `json:"tool_calls,omitempty"`
}

type wireToolCall struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function wireFuncCall  `json:"function"`
}

type wireFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type cliPayload struct {
	V       int           `json:"v"`
	History []wireMessage `json:"history"`
	Tools   string        `json:"tools"`
}

func toWire(msgs []Message) []wireMessage {
	out := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		out[i] = wireMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			out[i].ToolCalls = append(out[i].ToolCalls, wireToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: wireFuncCall{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
	}
	return out
}

func buildRequest(chatID, model, toolSchemas string, history []Message, isNew bool) *pb.ChatRequest {
	payload := cliPayload{V: cliPayloadVersion, History: toWire(history), Tools: toolSchemas}
	payloadJSON, _ := json.Marshal(payload)
	return &pb.ChatRequest{
		ChatId:    chatID,
		Model:     "cli/" + model,
		IsNewChat: isNew,
		Prompt:    string(payloadJSON),
	}
}

// ── Stream pump ───────────────────────────────────────────────────────────────

type streamUsage struct {
	promptTokens     int32
	completionTokens int32
}

func pumpStream(
	stream interface{ Recv() (*pb.ChatEvent, error) },
	cancelFn context.CancelFunc,
	emit func(Event),
) ([]ToolCall, streamUsage, error) {
	defer cancelFn()
	tf := &tui.ExportedThinkFilter{}
	var toolCalls []ToolCall

	for {
		ev, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				if tail := tf.Flush(); tail != "" {
					emit(Event{Kind: EventText, Text: tail})
				}
				return toolCalls, streamUsage{}, nil
			}
			return toolCalls, streamUsage{}, fmt.Errorf("stream recv: %w", err)
		}

		switch ev.GetType() {
		case "content", "stream_chunk":
			if filtered := tf.Filter(ev.GetContent()); filtered != "" {
				emit(Event{Kind: EventText, Text: filtered})
			}

		case "done", "stream_end":
			if tail := tf.Flush(); tail != "" {
				emit(Event{Kind: EventText, Text: tail})
			}
			return toolCalls, streamUsage{
				promptTokens:     ev.GetUsagePromptTokens(),
				completionTokens: ev.GetUsageCompletionTokens(),
			}, nil

		case "error", "stream_error":
			return toolCalls, streamUsage{}, fmt.Errorf("%s", ev.GetError())

		case "tool_call":
			data := ev.GetData()
			if data == "" {
				data = ev.GetContent()
			}
			var raw struct {
				ID        string `json:"tool_call_id"`
				Name      string `json:"tool_name"`
				Arguments string `json:"arguments"`
			}
			if json.Unmarshal([]byte(data), &raw) == nil {
				tc := ToolCall{ID: raw.ID, Name: raw.Name, Arguments: raw.Arguments}
				toolCalls = append(toolCalls, tc)
				emit(Event{Kind: EventToolUse, ToolID: tc.ID, ToolName: tc.Name, ToolInput: tc.Arguments})
			}
		}
	}
}

// ── Tool execution ────────────────────────────────────────────────────────────

func executeTools(
	ctx context.Context,
	toolCalls []ToolCall,
	allow, deny []string,
	executor ToolExecutor,
	emit func(Event),
) []string {
	results := make([]string, len(toolCalls))

	// Check if all tools pass the permission policy without a deny block.
	allSafe := true
	for _, tc := range toolCalls {
		if tools.CheckPermissions(allow, deny, tc.Name, tc.Arguments) == tools.PermitDeny {
			allSafe = false
			break
		}
	}

	if len(toolCalls) <= 1 || !allSafe {
		for i, tc := range toolCalls {
			results[i] = executeSingle(ctx, tc, allow, deny, executor, emit)
		}
		return results
	}

	// Parallel execution when multiple tools and none are denied.
	var wg sync.WaitGroup
	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, t ToolCall) {
			defer wg.Done()
			results[idx] = executeSingle(ctx, t, allow, deny, executor, emit)
		}(i, tc)
	}
	wg.Wait()
	return results
}

func executeSingle(
	ctx context.Context,
	tc ToolCall,
	allow, deny []string,
	executor ToolExecutor,
	emit func(Event),
) string {
	switch tools.CheckPermissions(allow, deny, tc.Name, tc.Arguments) {
	case tools.PermitDeny:
		msg := fmt.Sprintf("Tool call blocked by permissions policy: %s", tc.Name)
		emit(Event{Kind: EventToolResult, ToolID: tc.ID, ToolName: tc.Name, ToolOutput: msg})
		return msg
	}

	start := time.Now()
	out, err := executor(ctx, tc.Name, tc.Arguments)
	elapsed := time.Since(start)

	result := out
	if err != nil {
		result = "Error: " + err.Error()
	}
	emit(Event{
		Kind: EventToolResult, ToolID: tc.ID, ToolName: tc.Name,
		ToolOutput: result,
	})
	_ = elapsed
	return result
}

// ── Context compaction ────────────────────────────────────────────────────────

func compact(ctx context.Context, conn *caigrpc.Conn, chatID, model string, history []Message) ([]Message, error) {
	summarisePrompt := "Summarise this conversation concisely. Include: the original goal, key decisions made, files changed, and any important context needed to continue. Be thorough but brief."
	req := buildRequest(chatID, model, "", append(history, Message{Role: "user", Content: summarisePrompt}), false)

	iterCtx, cancel := context.WithCancel(ctx)
	stream, err := conn.Client.Chat(iterCtx, req)
	if err != nil {
		cancel()
		return nil, err
	}

	var summaryText string
	_, _, err = pumpStream(stream, cancel, func(ev Event) {
		if ev.Kind == EventText {
			summaryText += ev.Text
		}
	})
	if err != nil {
		return nil, fmt.Errorf("compact: %w", err)
	}
	if summaryText == "" {
		return nil, fmt.Errorf("compact: empty summary")
	}

	// Trimmed history: system messages + summary + last 4 messages.
	var sys []Message
	for _, m := range history {
		if m.Role == "system" {
			sys = append(sys, m)
		}
	}
	tail := history
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	result := append(sys, Message{Role: "assistant", Content: "[Summary] " + summaryText})
	return append(result, tail...), nil
}

func rateLimitBackoff(retry int) time.Duration {
	d := rateLimitInitialDelay * (1 << retry)
	if d > rateLimitMaxDelay {
		d = rateLimitMaxDelay
	}
	return d
}
