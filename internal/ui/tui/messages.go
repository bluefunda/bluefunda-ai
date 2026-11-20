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
	ID         string // the call's ID; pairs results and notes with their call
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
	ToolNote     ToolEventKind = iota // approval outcome, shown under its call
	ToolStart    ToolEventKind = iota // the call began executing
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

// toolRow is one tool call as displayed: the call merged with its result.
type toolRow struct {
	id         string
	name, args string
	queued     bool // waiting for an earlier call of the same round to finish
	done       bool
	ok         bool
	durationMs int64
	summary    string
	note       string // approval outcome, e.g. "declined: use make clean"
}

// toolRows merges the message's tool events into one row per call. A result,
// start or note goes to the call with the same ID; events without an ID
// (server-side tools) fall back to the oldest unfinished call of that name.
// Once the producer reports starts, unstarted calls show as queued. Progress
// events are dropped — the rows themselves show progress.
func toolRows(events []ToolEvent) []toolRow {
	var rows []toolRow
	starts := false
	find := func(ev ToolEvent, open bool) int {
		if ev.ID != "" {
			for i := range rows {
				if rows[i].id == ev.ID {
					return i
				}
			}
		}
		for i := range rows {
			if rows[i].name == ev.Name && (!open || !rows[i].done) && (ev.ID == "" || rows[i].id == "") {
				return i
			}
		}
		return -1
	}
	for _, ev := range events {
		switch ev.Kind {
		case ToolCall:
			rows = append(rows, toolRow{id: ev.ID, name: ev.Name, args: ev.Args, queued: true})
		case ToolStart:
			starts = true
			if i := find(ev, true); i >= 0 {
				rows[i].queued = false
			}
		case ToolExec:
			i := find(ev, true)
			if i < 0 { // result without a call event (e.g. server-side tool)
				rows = append(rows, toolRow{id: ev.ID, name: ev.Name})
				i = len(rows) - 1
			}
			r := &rows[i]
			r.queued, r.done, r.ok = false, true, ev.Status == "ok"
			r.durationMs, r.summary = ev.DurationMs, ev.Summary
		case ToolNote:
			i := -1
			if ev.ID != "" {
				i = find(ev, false)
			}
			for j := len(rows) - 1; i < 0 && j >= 0; j-- {
				if rows[j].name == ev.Name {
					i = j
				}
			}
			if i >= 0 {
				rows[i].note = ev.Summary
			}
		}
	}
	if !starts {
		for i := range rows {
			rows[i].queued = false
		}
	}
	return rows
}

// collapsedToolRows is how many finished calls are still listed one per
// line while collapsed; more than that fold into one summary line.
const collapsedToolRows = 2

// renderToolRows draws a message's tool calls. Expanded (Ctrl+O), every call
// gets a line. Collapsed, as in Claude Code, finished calls fold into one
// line ("ran 4 commands, read 3 files · 1 failed") and only calls still
// running are listed.
func renderToolRows(events []ToolEvent, expanded bool, width int, th Theme) string {
	rows := toolRows(events)
	if len(rows) == 0 {
		return ""
	}
	var done, running []toolRow
	for _, r := range rows {
		if r.done {
			done = append(done, r)
		} else {
			running = append(running, r)
		}
	}
	var sb strings.Builder
	list := rows
	if !expanded && len(done) > collapsedToolRows {
		sb.WriteString(formatToolSummary(done, width, th))
		sb.WriteByte('\n')
		list = running
	}
	for _, r := range list {
		sb.WriteString(formatToolRow(r, width, th))
		sb.WriteByte('\n')
		if r.note != "" {
			sb.WriteString("    " + th.ToolDim.Render(truncate("⎿ "+r.note, max(width-6, 20))) + "\n")
		}
	}
	return sb.String()
}

// toolVerbs groups tools under the phrase the summary line uses for them.
var toolVerbs = map[string][2]string{ // name → {singular, plural} with %d
	"bash":           {"ran %d command", "ran %d commands"},
	"read_file":      {"read %d file", "read %d files"},
	"read_notebook":  {"read %d file", "read %d files"},
	"list_dir":       {"listed %d directory", "listed %d directories"},
	"search_files":   {"searched %d time", "searched %d times"},
	"search_content": {"searched %d time", "searched %d times"},
	"write_file":     {"edited %d file", "edited %d files"},
	"edit_file":      {"edited %d file", "edited %d files"},
	"patch_file":     {"edited %d file", "edited %d files"},
	"edit_notebook":  {"edited %d file", "edited %d files"},
	"web_fetch":      {"fetched %d page", "fetched %d pages"},
	"web_search":     {"searched the web %d time", "searched the web %d times"},
	"task":           {"ran %d subagent", "ran %d subagents"},
}

// toolSummary phrases finished calls by kind, in order of first use:
// "Ran 4 commands, read 3 files, called memory_list 2 times".
func toolSummary(rows []toolRow) string {
	type group struct {
		one, many string
		n         int
	}
	var groups []*group
	byKey := map[string]*group{}
	for _, r := range rows {
		one, many := "called "+r.name, "called "+r.name+" %d times"
		if v, ok := toolVerbs[r.name]; ok {
			one, many = v[0], v[1]
		}
		g := byKey[many] // tools sharing a phrase share a count
		if g == nil {
			g = &group{one: one, many: many}
			byKey[many] = g
			groups = append(groups, g)
		}
		g.n++
	}
	parts := make([]string, len(groups))
	for i, g := range groups {
		if g.n == 1 {
			parts[i] = strings.Replace(g.one, "%d", "1", 1)
		} else {
			parts[i] = fmt.Sprintf(g.many, g.n)
		}
	}
	s := strings.Join(parts, ", ")
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatToolSummary is the collapsed line for finished calls.
func formatToolSummary(rows []toolRow, width int, th Theme) string {
	failed := 0
	for _, r := range rows {
		if !r.ok {
			failed++
		}
	}
	icon := th.ToolSuccess.Render("✓")
	text := toolSummary(rows)
	if failed > 0 {
		icon = th.ToolError.Render("✓")
		text += fmt.Sprintf(" · %d failed", failed)
	}
	return "  " + icon + " " + th.ToolDim.Render(truncate(text, max(width-24, 20))+"  (ctrl+o to expand)")
}

// formatToolRow renders one call: icon, tool, its key argument, then the
// duration (and the error summary when it failed).
func formatToolRow(r toolRow, width int, th Theme) string {
	var icon string
	switch {
	case r.queued:
		icon = th.ToolDim.Render("○")
	case !r.done:
		icon = th.ToolIcon.Render("●")
	case r.ok:
		icon = th.ToolSuccess.Render("✓")
	default:
		icon = th.ToolError.Render("✗")
	}
	line := "  " + icon + " " + th.ToolName.Render(r.name)
	tail := ""
	if r.queued {
		tail = "  · queued"
	}
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
		if r.done && !r.ok {
			style = th.ToolError
		}
		line += style.Render(truncate(tail, max(width-10, 20)))
	}
	return line
}
