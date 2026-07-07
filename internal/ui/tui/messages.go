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

// appendChunk appends streaming text to an assistant message.
func (m *ChatMessage) appendChunk(chunk string) {
	m.Content += chunk
	m.rendered = ""
}

// addToolEvent appends a tool event to the message.
func (m *ChatMessage) addToolEvent(ev ToolEvent) {
	m.ToolEvents = append(m.ToolEvents, ev)
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
