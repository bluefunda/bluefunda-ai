package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestThinkFilter_TakeThinkingAcrossSplitTags(t *testing.T) {
	f := &thinkFilter{}
	var visible, thought strings.Builder
	for _, c := range []string{"<thi", "nk>plan the ", "fix</thi", "nk>Here is the fix."} {
		visible.WriteString(f.Filter(c))
		thought.WriteString(f.TakeThinking())
	}
	visible.WriteString(f.Flush())
	if visible.String() != "Here is the fix." {
		t.Errorf("visible = %q", visible.String())
	}
	if thought.String() != "plan the fix" {
		t.Errorf("thinking = %q", thought.String())
	}
	if f.TakeThinking() != "" {
		t.Error("TakeThinking should drain")
	}
}

func TestThinkingCollapsedLiveThenDoneThenExpanded(t *testing.T) {
	m := newTestModel("")
	m.streaming = true
	m.handleStreamEvent(StreamEvent{Kind: "thinking", Chunk: "THOUGHTMARKER considering options"})

	conv := m.renderLive()
	if !strings.Contains(conv, "Thinking…") || strings.Contains(conv, "THOUGHTMARKER") {
		t.Errorf("live thinking should show a collapsed line only:\n%s", conv)
	}
	if strings.Contains(conv, "working...") {
		t.Error("the generic working line should give way to the thinking line")
	}

	m.handleStreamEvent(StreamEvent{Kind: "chunk", Chunk: "ANSWER"})
	conv = m.renderLive()
	if !strings.Contains(conv, "Thought for") || !strings.Contains(conv, "ctrl+o") {
		t.Errorf("finished thinking should collapse to a summary:\n%s", conv)
	}
	if m.messages[len(m.messages)-1].Content != "ANSWER" {
		t.Error("thinking must not leak into the message content")
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if conv = m.renderLive(); !strings.Contains(conv, "THOUGHTMARKER") {
		t.Errorf("ctrl+o should expand the thinking text:\n%s", conv)
	}
}

func TestFormatThinkDuration(t *testing.T) {
	for in, want := range map[int]string{0: "1s", 4: "4s", 72: "1m 12s"} {
		if got := formatThinkDuration(timeSeconds(in)); got != want {
			t.Errorf("formatThinkDuration(%ds) = %q, want %q", in, got, want)
		}
	}
}

func timeSeconds(n int) time.Duration { return time.Duration(n) * time.Second }

func TestToolArgSummary(t *testing.T) {
	for in, want := range map[string]string{
		`{"command":"go test\n  ./..."}`:   "go test ./...",
		`{"path":"a.go","old_string":"x"}`: "a.go",
		`{"limit":3}`:                      `{"limit":3}`,
		`{}`:                               "",
		`not json`:                         "not json",
	} {
		if got := toolArgSummary(in); got != want {
			t.Errorf("toolArgSummary(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExpandedNarrationStaysSeparateFromAnswer(t *testing.T) {
	m := newTestModel("")
	m.handleStreamEvent(StreamEvent{Kind: "chunk", Chunk: "NARR.\n"})
	m.handleStreamEvent(StreamEvent{Kind: "tool_call", ToolName: "bash", ToolArgs: `{"command":"ls"}`})
	m.handleStreamEvent(StreamEvent{Kind: "chunk", Chunk: "ANS."})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})
	if strings.Contains(m.renderLive(), "NARR. ANS.") {
		t.Error("expanded narration and answer should render as separate paragraphs")
	}
}
