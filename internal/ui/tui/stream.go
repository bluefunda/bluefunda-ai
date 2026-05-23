package tui

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"google.golang.org/grpc"
)

// toolCallJSON matches the stream tool_call data payload.
type toolCallJSON struct {
	ID        string `json:"tool_call_id"`
	Name      string `json:"tool_name"`
	Arguments string `json:"arguments"`
}

// progressJSON matches stream_progress payload.
type progressJSON struct {
	Tools     []string `json:"tools"`
	Iteration int      `json:"iteration"`
}

// toolExecJSON matches stream_tool_execution payload.
type toolExecJSON struct {
	ToolName      string `json:"tool_name"`
	Status        string `json:"status"`
	DurationMs    int64  `json:"duration_ms"`
	ResultSummary string `json:"result_summary"`
}

// PumpGRPCStream reads a gRPC ChatEvent stream and converts events into
// StreamEvents on the returned channel. The channel is closed when the stream
// ends (EOF, error, or done event). cancelFn is called on Ctrl+C signal.
func PumpGRPCStream(
	stream grpc.ServerStreamingClient[pb.ChatEvent],
	cancelFn context.CancelFunc,
) <-chan StreamEvent {
	ch := make(chan StreamEvent, 32)
	go func() {
		defer close(ch)
		defer cancelFn()
		pump(stream, ch)
	}()
	return ch
}

func pump(stream grpc.ServerStreamingClient[pb.ChatEvent], ch chan<- StreamEvent) {
	tf := &thinkFilter{}

	for {
		ev, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Flush any buffered think content before done
				if tail := tf.Flush(); tail != "" {
					ch <- StreamEvent{Kind: "chunk", Chunk: tail}
				}
				return
			}
			ch <- StreamEvent{Kind: "error", ErrMsg: err.Error()}
			return
		}

		switch ev.GetType() {
		case "content", "stream_chunk":
			filtered := tf.Filter(ev.GetContent())
			if filtered != "" {
				ch <- StreamEvent{Kind: "chunk", Chunk: filtered}
			}

		case "error", "stream_error":
			ch <- StreamEvent{Kind: "error", ErrMsg: ev.GetError()}
			return

		case "done", "stream_end":
			if tail := tf.Flush(); tail != "" {
				ch <- StreamEvent{Kind: "chunk", Chunk: tail}
			}
			ch <- StreamEvent{Kind: "done"}
			return

		case "tool_call":
			data := eventData(ev)
			var tc toolCallJSON
			if err := json.Unmarshal([]byte(data), &tc); err == nil {
				ch <- StreamEvent{
					Kind:     "tool_call",
					ToolName: tc.Name,
					ToolArgs: tc.Arguments,
					ToolID:   tc.ID,
				}
			}

		case "stream_progress":
			data := eventData(ev)
			var p progressJSON
			if err := json.Unmarshal([]byte(data), &p); err == nil {
				ch <- StreamEvent{
					Kind:      "progress",
					Iteration: p.Iteration,
					Tools:     p.Tools,
				}
			}

		case "stream_tool_execution":
			data := eventData(ev)
			var te toolExecJSON
			if err := json.Unmarshal([]byte(data), &te); err == nil {
				ch <- StreamEvent{
					Kind:       "tool_exec",
					ToolName:   te.ToolName,
					Status:     te.Status,
					DurationMs: te.DurationMs,
					Summary:    te.ResultSummary,
				}
			}

		case "stream_heartbeat":
			ch <- StreamEvent{Kind: "heartbeat"}

		case "tool_result", "stream_start":
			// no display

		default:
			// unknown; skip
		}
	}
}

func eventData(ev *pb.ChatEvent) string {
	if d := ev.GetData(); d != "" {
		return d
	}
	return ev.GetContent()
}

// ── ThinkFilter ──────────────────────────────────────────────────────────────
// Strips <think>…</think> blocks from streamed content. Copied from the
// original ui/stream.go so the tui package is self-contained.

// ExportedThinkFilter is the exported alias for external callers (e.g. cmd).
type ExportedThinkFilter = thinkFilter

type thinkFilter struct {
	inside     bool
	buf        string
	suppressed string
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
			if pl := partialSuffix(f.buf, "</think>"); pl > 0 {
				f.suppressed += f.buf[:len(f.buf)-pl]
				f.buf = f.buf[len(f.buf)-pl:]
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
		if pl := partialSuffix(f.buf, "<think>"); pl > 0 {
			out.WriteString(f.buf[:len(f.buf)-pl])
			f.buf = f.buf[len(f.buf)-pl:]
			return out.String()
		}
		out.WriteString(f.buf)
		f.buf = ""
	}

	return out.String()
}

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
