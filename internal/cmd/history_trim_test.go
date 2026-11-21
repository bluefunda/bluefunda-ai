package cmd

import (
	"strings"
	"testing"
)

// validHistory reports why msgs would be rejected by a provider, or "".
func validHistory(msgs []codeMessage) string {
	seenUser := false
	calls := map[string]bool{}
	for _, m := range msgs {
		switch m.Role {
		case "user":
			seenUser = true
		case "assistant":
			if !seenUser {
				return "assistant before any user message"
			}
			for _, tc := range m.ToolCalls {
				calls[tc.ID] = true
			}
		case "tool":
			if !calls[m.ToolCallID] {
				return "tool result " + m.ToolCallID + " without its call"
			}
		}
	}
	if !seenUser {
		return "no user message"
	}
	return ""
}

// agenticTurn builds one user turn with n tool rounds, each result size bytes.
func agenticTurn(prompt string, n, size int) []codeMessage {
	msgs := []codeMessage{{Role: "user", Content: prompt}}
	for i := range n {
		id := prompt + "-" + string(rune('a'+i))
		msgs = append(msgs,
			codeMessage{Role: "assistant", ToolCalls: []codeToolCall{{ID: id, Type: "function", Function: codeFuncCall{Name: "bash", Arguments: `{"command":"ls"}`}}}},
			codeMessage{Role: "tool", ToolCallID: id, Content: strings.Repeat("x", size)},
		)
	}
	return msgs
}

func TestRecentTail_KeepsUserMessageInToolTurn(t *testing.T) {
	// The last 4 messages are all assistant/tool: the old code kept nothing.
	h := append([]codeMessage{{Role: "system", Content: "sys"}}, agenticTurn("task", 6, 10)...)
	tail := recentTail(h, 4)
	if why := validHistory(tail); why != "" {
		t.Fatalf("invalid tail: %s", why)
	}
	if tail[0].Content != "task" {
		t.Errorf("tail should start with the turn's user message, got %q", tail[0].Content)
	}
	if len(tail) != 5 { // user + 2 rounds
		t.Errorf("tail has %d messages, want 5", len(tail))
	}
}

func TestRecentTail_CapsHugeToolResult(t *testing.T) {
	h := agenticTurn("task", 1, 800_000)
	tail := recentTail(h, 4)
	if n := estimateTokens(tail); n > 10_000 {
		t.Errorf("tail still ~%d tokens; huge tool result not capped", n)
	}
}

func TestDropOldestTurns_StaysValid(t *testing.T) {
	sys := codeMessage{Role: "system", Content: "sys"}
	cases := map[string][]codeMessage{
		"earlier turns":          append(append([]codeMessage{sys}, agenticTurn("old", 5, 4000)...), agenticTurn("new", 2, 100)...),
		"one long turn":          append([]codeMessage{sys}, agenticTurn("only", 30, 4000)...),
		"one huge latest result": append([]codeMessage{sys}, agenticTurn("huge", 1, 400_000)...),
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			got := dropOldestTurns(h, 3_000)
			if why := validHistory(got); why != "" {
				t.Fatalf("invalid after drop: %s", why)
			}
			if got[0].Role != "system" {
				t.Error("system message dropped")
			}
			if n := estimateTokens(got); n > 3_000 {
				t.Errorf("still ~%d tokens, want <= 3000", n)
			}
		})
	}
}

func TestDropOldestTurns_KeepsLatestTurnWhenEarlierDropSuffices(t *testing.T) {
	h := append(agenticTurn("old", 5, 4000), agenticTurn("new", 2, 100)...)
	got := dropOldestTurns(h, 1_000)
	if got[0].Content != "new" || len(got) != 5 {
		t.Errorf("want the whole latest turn (5 messages starting at \"new\"), got %d starting %q", len(got), got[0].Content)
	}
}
