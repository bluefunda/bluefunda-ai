package cmd

import (
	"errors"
	"strings"
	"sync"
	"testing"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
)

// A manual /compact keeps only the system messages and the summary, and the
// user's focus reaches the summary request.
func TestCompactHistory_ManualKeepsOnlySummary(t *testing.T) {
	bff := &scriptedBFF{replies: [][]*pb.ChatEvent{textReply("the summary")}}
	conn := startScriptedBFF(t, bff)
	history := []codeMessage{
		{Role: "system", Content: "ctx"},
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "user", Content: "three"},
		{Role: "assistant", Content: "four"},
	}
	got, err := compactHistory(conn, "c", "", history, "the API decisions", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Content != "ctx" || got[1].Content != "[Conversation summary]\nthe summary" {
		t.Errorf("compacted = %+v", got)
	}
	sent := bff.sentHistory(t, 0)
	if last := sent[len(sent)-1].Content; !strings.Contains(last, "focus on: the API decisions") {
		t.Errorf("summary request lacks the focus: %q", last)
	}
}

// /compact refuses while a turn holds the lock instead of rewriting the
// history the turn is using.
func TestCompactCommand_RefusesDuringTurn(t *testing.T) {
	var mu sync.Mutex
	ran := false
	cmd := compactCommand(&mu, func(string) (string, error) { ran = true; return "ok", nil })

	mu.Lock()
	if _, err := cmd.Run(""); !errors.Is(err, errTurnRunning) || ran {
		t.Errorf("during a turn: err = %v, ran = %v", err, ran)
	}
	mu.Unlock()

	if out, err := cmd.Run("focus"); err != nil || out != "ok" || !ran {
		t.Errorf("idle: out = %q, err = %v", out, err)
	}
	if !mu.TryLock() {
		t.Error("lock not released after /compact")
	}
}

func TestHumanTokens(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1.0k", 12345: "12.3k"} {
		if got := humanTokens(n); got != want {
			t.Errorf("humanTokens(%d) = %q, want %q", n, got, want)
		}
	}
}
