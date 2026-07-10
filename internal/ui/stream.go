package ui

import "strings"

// ToolCallEvent carries a tool invocation collected from a gRPC stream.
// Used by the agentic loop in internal/cmd/code.go to route tool calls to
// local executors.
type ToolCallEvent struct {
	ID        string `json:"tool_call_id"`
	Name      string `json:"tool_name"`
	Arguments string `json:"arguments"`
}

// thinkFilter strips <think>...</think> blocks from streamed content.
// It handles tags that span multiple chunks. If a <think> block is never
// closed (e.g. Sarvam), Flush() returns the suppressed content.
type thinkFilter struct {
	inside     bool   // true when inside a <think> block
	buf        string // partial tag buffer
	suppressed string // content inside unclosed <think> (recovered on Flush if no prior output)
	hadOutput  bool   // true if Filter() ever returned non-empty content
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

	result := out.String()
	if result != "" {
		f.hadOutput = true
	}
	return result
}

// Flush returns any remaining buffered content.
// If still inside an unclosed <think> block and no real content was emitted
// before it, the suppressed content is returned as the actual response (handles
// older Sarvam behavior where the response is wrapped in an unclosed <think>).
// If real content was already streamed (hadOutput), the unclosed <think> is
// trailing chain-of-thought — discard it to prevent duplicate output.
func (f *thinkFilter) Flush() string {
	var result string
	if f.inside {
		if !f.hadOutput {
			result = f.suppressed + f.buf
		}
		// real content already streamed — discard trailing think block
	} else {
		result = f.buf
	}
	f.buf = ""
	f.suppressed = ""
	f.inside = false
	f.hadOutput = false
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
