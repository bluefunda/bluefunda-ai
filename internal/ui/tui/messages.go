package tui

import (
	"fmt"
	"strings"
	"time"
)

// MsgRole identifies who authored a message.
type MsgRole int

const (
	RoleUser      MsgRole = iota
	RoleAssistant MsgRole = iota
	RoleSystem    MsgRole = iota
)

// ToolEvent holds a single tool-lifecycle event within an assistant turn.
type ToolEvent struct {
	Kind       ToolEventKind
	Name       string
	Args       string
	Status     string // "ok" | "error"
	DurationMs int64
	Summary    string
	Iteration  int
	Tools      []string // for progress events
}

type ToolEventKind int

const (
	ToolCall     ToolEventKind = iota
	ToolExec     ToolEventKind = iota
	ToolProgress ToolEventKind = iota
)

// ChatMessage is one logical message in the conversation.
type ChatMessage struct {
	Role        MsgRole
	Content     string
	ToolEvents  []ToolEvent
	Streaming   bool
	Timestamp   time.Time
	rendered    string // cached glamour-rendered form of Content (assistant only)
	renderWidth int    // width used when rendered was produced
	renderFull  bool   // whether rendered covers the full Content or only the answer

	// narrationEnd is len(Content) when the most recent tool call arrived.
	// Text before it preceded further tool calls in the same turn, so it is
	// treated as narration and collapsed by default (#330); text after it is
	// the answer (or narration still streaming).
	narrationEnd int
}

func newUserMessage(text string) ChatMessage {
	return ChatMessage{Role: RoleUser, Content: text, Timestamp: time.Now()}
}

func newAssistantMessage() ChatMessage {
	return ChatMessage{Role: RoleAssistant, Streaming: true, Timestamp: time.Now()}
}

func newSystemMessage(text string) ChatMessage {
	return ChatMessage{Role: RoleSystem, Content: text, Timestamp: time.Now()}
}

// nthMessageFromEnd returns the nth ChatMessage counting backward from the
// end of msgs (1 = most recent), skipping RoleSystem notices — those are CLI
// chrome, not conversation content a user would want to copy (#288). ok is
// false if n is out of range.
func nthMessageFromEnd(msgs []ChatMessage, n int) (msg ChatMessage, ok bool) {
	count := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleSystem {
			continue
		}
		count++
		if count == n {
			return msgs[i], true
		}
	}
	return ChatMessage{}, false
}

// lastUserMessageIndex returns the index of the last RoleUser message in
// msgs, or -1 if none exists.
func lastUserMessageIndex(msgs []ChatMessage) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser {
			return i
		}
	}
	return -1
}

// appendChunk appends streaming text to an assistant message.
func (m *ChatMessage) appendChunk(chunk string) {
	m.Content += chunk
	m.rendered = ""
}

// addToolEvent appends a tool event to the message.
func (m *ChatMessage) addToolEvent(ev ToolEvent) {
	m.ToolEvents = append(m.ToolEvents, ev)
	if ev.Kind == ToolCall && m.narrationEnd != len(m.Content) {
		m.narrationEnd = len(m.Content)
		m.rendered = ""
	}
}

// splitNarration splits Content into the narration that preceded the turn's
// last tool call and the answer that follows it. The split is a heuristic:
// the BFF streams reasoning as plain content with no distinct event type, so
// "text followed by another tool call" is the only signal available (#330).
func (m *ChatMessage) splitNarration() (narration, answer string) {
	end := m.narrationEnd
	if end > len(m.Content) {
		end = len(m.Content)
	}
	narration = m.Content[:end]
	if strings.TrimSpace(narration) == "" {
		return "", m.Content
	}
	return narration, m.Content[end:]
}

// narrationSummary is the one-line indicator shown in place of collapsed
// narration.
func narrationSummary(narration string) string {
	lines := 0
	for _, l := range strings.Split(strings.TrimSpace(narration), "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	unit := "lines"
	if lines == 1 {
		unit = "line"
	}
	return fmt.Sprintf("✻ %d %s of narration hidden (ctrl+o to expand)", lines, unit)
}

// finishStreaming marks the message as no longer streaming.
func (m *ChatMessage) finishStreaming() {
	m.Streaming = false
	m.rendered = ""
}

// formatToolEventLine renders a ToolEvent as a compact single line.
func formatToolEventLine(ev ToolEvent, th Theme) string {
	switch ev.Kind {
	case ToolCall:
		icon := th.ToolIcon.Render("●")
		name := th.ToolName.Render(ev.Name)
		args := ""
		if ev.Args != "" && ev.Args != "{}" {
			short := ev.Args
			if len(short) > 60 {
				short = short[:57] + "..."
			}
			args = th.ToolArg.Render("  " + short)
		}
		return fmt.Sprintf("  %s %s%s", icon, name, args)

	case ToolExec:
		var icon string
		if ev.Status == "ok" {
			icon = th.ToolSuccess.Render("  ✓")
		} else {
			icon = th.ToolError.Render("  ✗")
		}
		dur := fmt.Sprintf("%.1fs", float64(ev.DurationMs)/1000)
		line := fmt.Sprintf("%s  %s", icon, th.ToolDim.Render(ev.Name+"  "+dur))
		if ev.Summary != "" {
			s := ev.Summary
			if len(s) > 55 {
				s = s[:52] + "..."
			}
			line += th.ToolDim.Render("  —  " + s)
		}
		return line

	case ToolProgress:
		icon := th.ToolDim.Render("  ↻")
		label := fmt.Sprintf("  [%d]  %s", ev.Iteration, strings.Join(ev.Tools, ", "))
		return icon + th.ToolDim.Render(label)
	}
	return ""
}
