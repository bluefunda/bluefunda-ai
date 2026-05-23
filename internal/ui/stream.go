package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"google.golang.org/grpc"
)

// ToolCallEvent carries a tool invocation streamed from the backend.
type ToolCallEvent struct {
	ID        string `json:"tool_call_id"`
	Name      string `json:"tool_name"`
	Arguments string `json:"arguments"`
}

// progressEvent matches the stream_progress JSON payload.
type progressEvent struct {
	Tools     []string `json:"tools"`
	Iteration int      `json:"iteration"`
}

// toolExecEvent matches the stream_tool_execution JSON payload.
type toolExecEvent struct {
	ToolName      string `json:"tool_name"`
	Status        string `json:"status"`
	DurationMs    int64  `json:"duration_ms"`
	ResultSummary string `json:"result_summary"`
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// streamRenderer manages terminal output state during a streaming response.
// It handles the interleaving of content chunks, tool call lines, and spinner
// updates so they don't corrupt each other.
type streamRenderer struct {
	tf           *thinkFilter
	spinnerShown bool      // a spinner line is displayed without a trailing newline
	spinnerStart time.Time // when the current silent phase began
	spinnerIdx   int
	needNewline  bool // content printed without trailing newline
	contentSeen  bool // any content chunks received this stream
}

func newStreamRenderer() *streamRenderer {
	return &streamRenderer{tf: &thinkFilter{}}
}

// isTerminal reports whether stdout is an interactive terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// clearSpinner erases the spinner line if one is currently displayed.
func (r *streamRenderer) clearSpinner() {
	if r.spinnerShown {
		fmt.Print("\r\033[K")
		r.spinnerShown = false
	}
}

// ensureNewline moves to a fresh line if content was printed without one.
func (r *streamRenderer) ensureNewline() {
	if r.needNewline {
		fmt.Println()
		r.needNewline = false
	}
}

// showHeartbeat updates the working spinner. Only shown before content starts.
func (r *streamRenderer) showHeartbeat() {
	if r.contentSeen || !isTerminal() {
		return
	}
	if !r.spinnerShown {
		r.ensureNewline()
		r.spinnerStart = time.Now()
	} else {
		fmt.Print("\r\033[K")
	}
	elapsed := time.Since(r.spinnerStart)
	frame := spinnerFrames[r.spinnerIdx%len(spinnerFrames)]
	r.spinnerIdx++
	label := fmt.Sprintf("  %s  working...", frame)
	if elapsed >= 2*time.Second {
		label = fmt.Sprintf("  %s  working...  (%ds)", frame, int(elapsed.Seconds()))
	}
	fmt.Print(dim(label)) // no newline — stays on current line for in-place update
	r.spinnerShown = true
}

// printChunk writes a content delta to stdout, clearing any spinner first.
func (r *streamRenderer) printChunk(chunk string) {
	r.clearSpinner()
	filtered := r.tf.Filter(chunk)
	if filtered == "" {
		return
	}
	r.contentSeen = true
	fmt.Print(filtered)
	r.needNewline = !strings.HasSuffix(filtered, "\n")
}

// printToolCall renders a tool invocation line in dim style.
func (r *streamRenderer) printToolCall(data string) {
	r.clearSpinner()
	r.ensureNewline()
	var tc ToolCallEvent
	if err := json.Unmarshal([]byte(data), &tc); err != nil {
		fmt.Println(dim("  ⚙  " + data))
		return
	}
	fmt.Println(dim("  ⚙  " + tc.Name + formatToolArgs(tc.Arguments)))
}

// printProgress renders a stream_progress line (agentic loop iteration).
func (r *streamRenderer) printProgress(data string) {
	r.clearSpinner()
	r.ensureNewline()
	var ev progressEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return
	}
	fmt.Println(dim(fmt.Sprintf("  ↻  [%d]  %s", ev.Iteration, strings.Join(ev.Tools, ", "))))
}

// printToolExec renders a stream_tool_execution completion line.
func (r *streamRenderer) printToolExec(data string) {
	r.clearSpinner()
	r.ensureNewline()
	var ev toolExecEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		return
	}
	secs := fmt.Sprintf("%.1fs", float64(ev.DurationMs)/1000)
	icon := dimGreen("  ✓  ")
	if ev.Status != "ok" {
		icon = dimRed("  ✗  ")
	}
	tail := ""
	if ev.ResultSummary != "" {
		s := ev.ResultSummary
		if len(s) > 60 {
			s = s[:57] + "..."
		}
		tail = dim("  —  " + s)
	}
	fmt.Println(icon + dim(ev.ToolName+"  "+secs) + tail)
}

// printError renders an error event.
func (r *streamRenderer) printError(msg string) {
	r.clearSpinner()
	r.ensureNewline()
	Error(msg)
}

// flush drains the think filter and ensures the cursor ends on a fresh line.
func (r *streamRenderer) flush() {
	r.clearSpinner()
	if tail := r.tf.Flush(); tail != "" {
		fmt.Print(tail)
		r.needNewline = !strings.HasSuffix(tail, "\n")
	}
	r.ensureNewline()
}

// eventData returns the structured payload from a ChatEvent, preferring Data
// over Content (structured events use data; text chunks use content).
func eventData(ev *pb.ChatEvent) string {
	if d := ev.GetData(); d != "" {
		return d
	}
	return ev.GetContent()
}

// thinkFilter strips <think>...</think> blocks from streamed content.
// It handles tags that span multiple chunks. If a <think> block is never
// closed (e.g. Sarvam), Flush() returns the suppressed content.
type thinkFilter struct {
	inside     bool   // true when inside a <think> block
	buf        string // partial tag buffer
	suppressed string // content inside unclosed <think> (recovered on Flush)
}

func (f *thinkFilter) Filter(chunk string) string {
	f.buf += chunk
	var out strings.Builder

	for len(f.buf) > 0 {
		if f.inside {
			idx := strings.Index(f.buf, "</think>")
			if idx >= 0 {
				f.suppressed = ""
				f.buf = f.buf[idx+len("</think>"):]
				f.inside = false
				continue
			}
			if partialLen := partialSuffix(f.buf, "</think>"); partialLen > 0 {
				f.suppressed += f.buf[:len(f.buf)-partialLen]
				f.buf = f.buf[len(f.buf)-partialLen:]
			} else {
				f.suppressed += f.buf
				f.buf = ""
			}
			return out.String()
		}

		idx := strings.Index(f.buf, "<think>")
		if idx >= 0 {
			out.WriteString(f.buf[:idx])
			f.buf = f.buf[idx+len("<think>"):]
			f.inside = true
			f.suppressed = ""
			continue
		}
		if partialLen := partialSuffix(f.buf, "<think>"); partialLen > 0 {
			out.WriteString(f.buf[:len(f.buf)-partialLen])
			f.buf = f.buf[len(f.buf)-partialLen:]
			return out.String()
		}
		out.WriteString(f.buf)
		f.buf = ""
	}

	return out.String()
}

// Flush returns any remaining buffered content.
// If still inside an unclosed <think> block, returns the suppressed content
// since some models (e.g. Sarvam) emit <think> without a closing tag.
func (f *thinkFilter) Flush() string {
	var result string
	if f.inside {
		result = f.suppressed + f.buf
	} else {
		result = f.buf
	}
	f.buf = ""
	f.suppressed = ""
	f.inside = false
	return result
}

// partialSuffix returns the length of the longest suffix of s that is a prefix of tag.
func partialSuffix(s, tag string) int {
	maxLen := len(tag) - 1
	if maxLen > len(s) {
		maxLen = len(s)
	}
	for n := maxLen; n > 0; n-- {
		if strings.HasSuffix(s, tag[:n]) {
			return n
		}
	}
	return 0
}

// RenderGRPCStream reads ChatEvent messages from a gRPC server stream,
// prints content chunks to stdout, and calls cancelFn on Ctrl+C.
func RenderGRPCStream(stream grpc.ServerStreamingClient[pb.ChatEvent], cancelFn context.CancelFunc) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	doneCh := make(chan error, 1)

	go func() {
		doneCh <- renderGRPCLoop(stream)
	}()

	select {
	case err := <-doneCh:
		return err
	case <-sigCh:
		fmt.Println()
		cancelFn()
		return nil
	}
}

func renderGRPCLoop(stream grpc.ServerStreamingClient[pb.ChatEvent]) error {
	_, err := renderGRPCLoopWithTools(stream, false)
	return err
}

// StreamWithTools reads a ChatEvent stream, prints content to stdout, and returns
// any tool_call events collected during the stream. Used by the agentic code loop.
func StreamWithTools(stream grpc.ServerStreamingClient[pb.ChatEvent], cancelFn context.CancelFunc) ([]ToolCallEvent, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	type result struct {
		calls []ToolCallEvent
		err   error
	}
	doneCh := make(chan result, 1)

	go func() {
		calls, err := renderGRPCLoopWithTools(stream, true)
		doneCh <- result{calls, err}
	}()

	select {
	case r := <-doneCh:
		return r.calls, r.err
	case <-sigCh:
		fmt.Println()
		cancelFn()
		return nil, nil
	}
}

func renderGRPCLoopWithTools(stream grpc.ServerStreamingClient[pb.ChatEvent], collectTools bool) ([]ToolCallEvent, error) {
	r := newStreamRenderer()
	var toolCalls []ToolCallEvent

	for {
		ev, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				r.flush()
				return toolCalls, nil
			}
			return toolCalls, fmt.Errorf("stream recv: %w", err)
		}

		switch ev.GetType() {
		case "content", "stream_chunk":
			r.printChunk(ev.GetContent())

		case "error", "stream_error":
			r.printError(ev.GetError())

		case "done", "stream_end":
			r.flush()
			return toolCalls, nil

		case "tool_call":
			if collectTools {
				var tc ToolCallEvent
				if err := json.Unmarshal([]byte(ev.GetData()), &tc); err == nil {
					toolCalls = append(toolCalls, tc)
				}
			} else {
				r.printToolCall(ev.GetData())
			}

		case "stream_progress":
			r.printProgress(eventData(ev))

		case "stream_tool_execution":
			r.printToolExec(eventData(ev))

		case "stream_heartbeat":
			r.showHeartbeat()

		case "tool_result", "stream_start":
			// no display needed

		default:
			// unknown event type; ignore
		}
	}
}
