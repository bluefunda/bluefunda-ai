package agent

import (
	"testing"

	intagent "github.com/bluefunda/bluefunda-ai/internal/agent"
)

func TestNew_Defaults(t *testing.T) {
	r := New(Options{})
	if r.opts.Model != "auto" {
		t.Errorf("default model = %q, want %q", r.opts.Model, "auto")
	}
	if r.chatID == "" {
		t.Error("chatID should be set on New")
	}
}

func TestNew_CustomModel(t *testing.T) {
	r := New(Options{Model: "fast"})
	if r.opts.Model != "fast" {
		t.Errorf("model = %q, want %q", r.opts.Model, "fast")
	}
}

func TestWithSystemPrompt(t *testing.T) {
	r := New(Options{})
	r.WithSystemPrompt("you are a test bot")
	if len(r.history) != 1 {
		t.Fatalf("expected 1 history message, got %d", len(r.history))
	}
	if r.history[0].Role != "system" {
		t.Errorf("role = %q, want system", r.history[0].Role)
	}
	if r.history[0].Content != "you are a test bot" {
		t.Errorf("content = %q", r.history[0].Content)
	}
}

func TestWithHistory(t *testing.T) {
	r := New(Options{})
	r.WithHistory([]Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	})
	if len(r.history) != 2 {
		t.Fatalf("expected 2 history messages, got %d", len(r.history))
	}
}

func TestHistory_RoundTrip(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	r := New(Options{})
	r.WithHistory(msgs)
	got := r.History()
	if len(got) != len(msgs) {
		t.Fatalf("len = %d, want %d", len(got), len(msgs))
	}
	for i, m := range msgs {
		if got[i].Role != m.Role || got[i].Content != m.Content {
			t.Errorf("[%d] got %+v, want %+v", i, got[i], m)
		}
	}
}

func TestToFromInternal(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "hello", ToolCalls: []ToolCall{{ID: "1", Name: "bash", Arguments: `{"command":"ls"}`}}},
		{Role: "tool", Content: "result", ToolCallID: "1"},
	}
	internal := toInternal(msgs)
	if len(internal) != 2 {
		t.Fatalf("toInternal len = %d", len(internal))
	}
	if len(internal[0].ToolCalls) != 1 {
		t.Errorf("tool calls not converted")
	}
	back := fromInternal(internal)
	if back[0].ToolCalls[0].Name != "bash" {
		t.Errorf("ToolCall.Name not round-tripped")
	}
	if back[1].ToolCallID != "1" {
		t.Errorf("ToolCallID not round-tripped")
	}
}

func TestDefaultExecute_UnknownTool(t *testing.T) {
	// DefaultExecute should return an error for unknown tools without panicking.
	_, err := DefaultExecute(ToolCall{Name: "nonexistent_tool", Arguments: `{}`})
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestWrapEventFn_NilSafe(t *testing.T) {
	r := New(Options{OnEvent: nil})
	fn := r.wrapEventFn()
	// Must not panic when OnEvent is nil.
	fn(intagent.Event{Kind: intagent.EventText, Text: "hello"})
}

func TestWrapEventFn_Called(t *testing.T) {
	var got []Event
	r := New(Options{OnEvent: func(ev Event) { got = append(got, ev) }})
	fn := r.wrapEventFn()
	fn(intagent.Event{Kind: intagent.EventText, Text: "world"})
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].Text != "world" || got[0].Type != "text" {
		t.Errorf("event = %+v", got[0])
	}
}

func TestWrapToolFn_NilReturnsNil(t *testing.T) {
	r := New(Options{OnToolCall: nil})
	fn := r.wrapToolFn()
	if fn != nil {
		t.Error("expected nil ToolFn when OnToolCall is nil")
	}
}

func TestWrapToolFn_Called(t *testing.T) {
	called := false
	r := New(Options{OnToolCall: func(tc ToolCall) (ToolResult, error) {
		called = true
		return ToolResult{Output: "ok"}, nil
	}})
	fn := r.wrapToolFn()
	if fn == nil {
		t.Fatal("expected non-nil ToolFn")
	}
	out, err := fn(nil, "bash", `{"command":"ls"}`)
	if err != nil || out != "ok" || !called {
		t.Errorf("fn = %q, %v, called = %v", out, err, called)
	}
}

func TestDefaultTools(t *testing.T) {
	schemas, err := DefaultTools()
	if err != nil {
		t.Fatalf("DefaultTools: %v", err)
	}
	if schemas == "" {
		t.Error("DefaultTools returned empty schema")
	}
	// Should include known tool names.
	for _, name := range []string{"bash", "read_file", "edit_file", "patch_file", "web_search"} {
		if !contains(schemas, name) {
			t.Errorf("DefaultTools missing %q", name)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
