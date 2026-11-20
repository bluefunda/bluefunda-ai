package tui

import (
	"encoding/json"
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

	// printed is set once the message has been written to the terminal
	// scrollback; View no longer draws it.
	printed bool

	// narrationEnd is len(Content) when the most recent tool call arrived.
	// Text before it preceded further tool calls in the same turn, so it is
	// treated as narration and collapsed by default (#330); text after it is
	// the answer (or narration still streaming).
	narrationEnd int

	// Thinking is <think> text the model streamed for this turn, shown as a
	// collapsed "Thought for Ns" line (expand with Ctrl+O). It is display-only:
	// never copied, never part of Content.
	Thinking      string
	thinkingSince time.Time     // start of the current thinking segment (zero = not thinking)
	thinkingTotal time.Duration // finished thinking segments
}

// appendThinking adds streamed think text, starting the timer if needed.
func (m *ChatMessage) appendThinking(s string) {
	if m.thinkingSince.IsZero() {
		m.thinkingSince = time.Now()
	}
	m.Thinking += s
}

// endThinking stops the thinking timer (called when real output, a tool call
// or the end of the turn arrives).
func (m *ChatMessage) endThinking() {
	if !m.thinkingSince.IsZero() {
		m.thinkingTotal += time.Since(m.thinkingSince)
		m.thinkingSince = time.Time{}
	}
}

// isThinking reports whether think text is streaming right now.
func (m *ChatMessage) isThinking() bool { return !m.thinkingSince.IsZero() }

// thinkingDuration is the total time spent thinking, including a segment
// still in progress.
func (m *ChatMessage) thinkingDuration() time.Duration {
	d := m.thinkingTotal
	if m.isThinking() {
		d += time.Since(m.thinkingSince)
	}
	return d
}

// formatThinkDuration renders a thinking duration as "4s" or "1m 12s".
func formatThinkDuration(d time.Duration) string {
	secs := int(d.Round(time.Second) / time.Second)
	if secs < 60 {
		return fmt.Sprintf("%ds", max(secs, 1))
	}
	return fmt.Sprintf("%dm %ds", secs/60, secs%60)
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
	m.endThinking()
	m.Content += chunk
	m.rendered = ""
}

// addToolEvent appends a tool event to the message.
func (m *ChatMessage) addToolEvent(ev ToolEvent) {
	m.endThinking()
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
	m.endThinking()
	m.Streaming = false
	m.rendered = ""
}

// summaryArgKeys are the argument names shown, in priority order, as a tool
// call's one-line summary (Claude Code shows e.g. Bash(go test ./...)).
var summaryArgKeys = []string{"command", "path", "url", "pattern", "query", "key", "prompt"}

// toolArgSummary returns the most telling argument of a tool call as plain
// text — the command, path, URL, … — falling back to compact JSON.
func toolArgSummary(argsJSON string) string {
	if argsJSON == "" || argsJSON == "{}" {
		return ""
	}
	var args map[string]any
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return argsJSON
	}
	for _, k := range summaryArgKeys {
		if v, ok := args[k].(string); ok && v != "" {
			return strings.Join(strings.Fields(v), " ") // one line
		}
	}
	return argsJSON
}

// visibleToolRows is how many of the most recent tool calls an assistant
// message shows while collapsed; older ones fold into one summary line.
const visibleToolRows = 5

// toolRow is one tool call as displayed: the call merged with its result.
type toolRow struct {
	name, args string
	done       bool
	ok         bool
	durationMs int64
	summary    string
}

// toolRows merges the message's tool events into one row per call: a call
// is paired with the first later result for the same tool. Progress events
// are dropped — the rows themselves show progress.
func toolRows(events []ToolEvent) []toolRow {
	var rows []toolRow
	for _, ev := range events {
		switch ev.Kind {
		case ToolCall:
			rows = append(rows, toolRow{name: ev.Name, args: ev.Args})
		case ToolExec:
			paired := false
			for i := range rows {
				if !rows[i].done && rows[i].name == ev.Name {
					rows[i].done, rows[i].ok = true, ev.Status == "ok"
					rows[i].durationMs, rows[i].summary = ev.DurationMs, ev.Summary
					paired = true
					break
				}
			}
			if !paired { // result without a call event (e.g. server-side tool)
				rows = append(rows, toolRow{name: ev.Name, done: true, ok: ev.Status == "ok", durationMs: ev.DurationMs, summary: ev.Summary})
			}
		}
	}
	return rows
}

// renderToolRows draws a message's tool calls, one line each. Collapsed, only
// the last visibleToolRows are shown under a "… N earlier tool calls" line.
func renderToolRows(events []ToolEvent, expanded bool, width int, th Theme) string {
	rows := toolRows(events)
	if len(rows) == 0 {
		return ""
	}
	var sb strings.Builder
	if !expanded && len(rows) > visibleToolRows {
		hidden := rows[:len(rows)-visibleToolRows]
		failed := 0
		for _, r := range hidden {
			if r.done && !r.ok {
				failed++
			}
		}
		line := fmt.Sprintf("… %d earlier tool calls", len(hidden))
		if failed > 0 {
			line += fmt.Sprintf(", %d failed", failed)
		}
		sb.WriteString("  " + th.ToolDim.Render(line+"  (ctrl+o to expand)") + "\n")
		rows = rows[len(rows)-visibleToolRows:]
	}
	for _, r := range rows {
		sb.WriteString(formatToolRow(r, width, th))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// formatToolRow renders one call: icon, tool, its key argument, then the
// duration (and the error summary when it failed).
func formatToolRow(r toolRow, width int, th Theme) string {
	var icon string
	switch {
	case !r.done:
		icon = th.ToolIcon.Render("●")
	case r.ok:
		icon = th.ToolSuccess.Render("✓")
	default:
		icon = th.ToolError.Render("✗")
	}
	line := "  " + icon + " " + th.ToolName.Render(r.name)
	tail := ""
	if r.done {
		tail = fmt.Sprintf("  · %.1fs", float64(r.durationMs)/1000)
		if !r.ok && r.summary != "" {
			tail += " — " + r.summary
		}
	}
	room := max(width-len([]rune(r.name))-6-len([]rune(tail)), 10)
	if a := toolArgSummary(r.args); a != "" {
		line += th.ToolArg.Render("  " + truncate(a, room))
	}
	if tail != "" {
		style := th.ToolDim
		if !r.ok {
			style = th.ToolError
		}
		line += style.Render(truncate(tail, max(width-10, 20)))
	}
	return line
}
