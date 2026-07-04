package tui

import "testing"

// newTestModel returns a model that is laid out enough for commitScrollback to run.
func newTestModel() Model {
	m := New(SessionConfig{ChatID: "test-chat-id-123456", Model: "auto"}, nil)
	m.width = 80
	m.vpReady = true
	return m
}

func TestCommitScrollbackMarksFinishedMessages(t *testing.T) {
	m := newTestModel()
	m.messages = append(m.messages,
		newUserMessage("hello"),
		newSystemMessage("some system note"),
	)

	cmd := m.commitScrollback()
	if cmd == nil {
		t.Fatal("expected a tea.Println cmd for pending messages, got nil")
	}
	for i, msg := range m.messages {
		if !msg.printed {
			t.Errorf("message %d (role %v) should be marked printed", i, msg.Role)
		}
	}

	// A second call has nothing new to commit.
	if cmd := m.commitScrollback(); cmd != nil {
		t.Error("expected nil cmd when all messages are already printed")
	}
}

func TestCommitScrollbackLeavesLiveTurnInline(t *testing.T) {
	m := newTestModel()
	m.messages = append(m.messages, newUserMessage("do a thing"))
	live := newAssistantMessage() // Streaming == true
	live.Content = "partial resp"
	m.messages = append(m.messages, live)

	m.commitScrollback()

	last := len(m.messages) - 1
	if m.messages[last].printed {
		t.Error("live streaming assistant turn must not be committed to scrollback")
	}
	if !m.messages[last-1].printed {
		t.Error("the completed user message before the live turn should be committed")
	}

	// Once the turn finishes, the next commit picks it up.
	m.messages[last].finishStreaming()
	if cmd := m.commitScrollback(); cmd == nil {
		t.Error("expected the finished assistant turn to be committed")
	}
	if !m.messages[last].printed {
		t.Error("finished assistant turn should now be marked printed")
	}
}

// renderActiveMessages must exclude committed messages so the inline block only
// ever holds the live turn — this is what keeps history from being truncated.
func TestRenderActiveMessagesExcludesPrinted(t *testing.T) {
	m := newTestModel()
	m.messages = append(m.messages, newUserMessage("committed msg"))
	m.commitScrollback()

	if got := m.renderActiveMessages(); got != "" {
		t.Errorf("expected empty active block after commit, got %q", got)
	}
}
