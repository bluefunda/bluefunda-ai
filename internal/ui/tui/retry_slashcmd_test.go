package tui

import (
	"errors"
	"testing"
)

func TestLastUserMessageIndex(t *testing.T) {
	cases := []struct {
		name string
		msgs []ChatMessage
		want int
	}{
		{"empty", nil, -1},
		{"no user messages", []ChatMessage{newSystemMessage("hi")}, -1},
		{"single user", []ChatMessage{newUserMessage("a")}, 0},
		{"finds last of several", []ChatMessage{
			newUserMessage("a"),
			newAssistantMessage(),
			newUserMessage("b"),
			newAssistantMessage(),
		}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastUserMessageIndex(c.msgs); got != c.want {
				t.Errorf("lastUserMessageIndex() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestRetrySlashCommand_UnavailableWhenRetryFnNil(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.cfg.RetryFn = nil

	m.textarea.SetValue("/retry")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !containsMessage(m, "Retry is not available in this session.") {
		t.Errorf("expected an unavailable message, got %+v", m.messages)
	}
}

func TestRetrySlashCommand_PropagatesError(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.cfg.RetryFn = func(modelOverride string) (<-chan StreamEvent, string, error) {
		return nil, "", errors.New("no previous turn to retry")
	}

	m.textarea.SetValue("/retry")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !containsMessage(m, "no previous turn to retry") {
		t.Errorf("expected the error surfaced as a system message, got %+v", m.messages)
	}
	if m.streaming {
		t.Error("expected streaming to remain false after a failed retry")
	}
}

func TestRetrySlashCommand_ReplacesLastResponseInPlace(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.messages = append(m.messages, newUserMessage("first prompt"), newAssistantMessage())

	var gotModelOverride string
	m.cfg.RetryFn = func(modelOverride string) (<-chan StreamEvent, string, error) {
		gotModelOverride = modelOverride
		ch := make(chan StreamEvent, 1)
		close(ch)
		return ch, "first prompt", nil
	}

	beforeLen := len(m.messages)

	m.textarea.SetValue("/retry")
	nm, _ := m.submitInput()
	m = nm.(Model)

	// The retry truncates back to before the old user+assistant pair, then
	// appends exactly one fresh user message — net length unchanged, and the
	// old assistant response is gone (replaced in place, not piled up after).
	if len(m.messages) != beforeLen-1 {
		t.Fatalf("expected net length %d (old pair dropped, one fresh message added), got %d: %+v", beforeLen-1, len(m.messages), m.messages)
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != RoleUser || last.Content != "first prompt" {
		t.Fatalf("expected the last message to be the fresh retried user message, got %+v", last)
	}
	if !m.streaming {
		t.Error("expected streaming to be true after a successful retry")
	}
	if gotModelOverride != "" {
		t.Errorf("expected no model override for bare /retry, got %q", gotModelOverride)
	}
}

func TestRetrySlashCommand_ModelOverride(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.messages = append(m.messages, newUserMessage("first prompt"))

	var gotModelOverride string
	m.cfg.RetryFn = func(modelOverride string) (<-chan StreamEvent, string, error) {
		gotModelOverride = modelOverride
		ch := make(chan StreamEvent, 1)
		close(ch)
		return ch, "first prompt", nil
	}

	m.textarea.SetValue("/retry --model gpt-4")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if gotModelOverride != "gpt-4" {
		t.Errorf("expected model override %q, got %q", "gpt-4", gotModelOverride)
	}
	if !containsMessage(m, "Retrying with model: gpt-4") {
		t.Errorf("expected a model-switch announcement, got %+v", m.messages)
	}
}

func TestRetrySlashCommand_BadUsageDoesNotCallRetryFn(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.cfg.RetryFn = func(modelOverride string) (<-chan StreamEvent, string, error) {
		t.Fatal("RetryFn should not be called for malformed input")
		return nil, "", nil
	}

	m.textarea.SetValue("/retry foo")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !containsMessage(m, "Usage: /retry [--model <name>]") {
		t.Errorf("expected a usage message, got %+v", m.messages)
	}
}
