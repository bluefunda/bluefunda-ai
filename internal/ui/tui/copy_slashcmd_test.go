package tui

import (
	"errors"
	"testing"
)

func TestNthMessageFromEnd(t *testing.T) {
	msgs := []ChatMessage{
		newUserMessage("first prompt"),
		newAssistantMessage(),
		newSystemMessage("a notice"),
		newUserMessage("second prompt"),
		newAssistantMessage(),
	}
	msgs[1].Content = "first reply"
	msgs[4].Content = "second reply"

	cases := []struct {
		name string
		n    int
		want string
		ok   bool
	}{
		{"n=1 is the most recent, skipping the system notice", 1, "second reply", true},
		{"n=2 is the one before that", 2, "second prompt", true},
		{"n=3 skips the system notice entirely", 3, "first reply", true},
		{"out of range", 10, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg, ok := nthMessageFromEnd(msgs, c.n)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && msg.Content != c.want {
				t.Errorf("Content = %q, want %q", msg.Content, c.want)
			}
		})
	}
}

func TestNthMessageFromEnd_Empty(t *testing.T) {
	if _, ok := nthMessageFromEnd(nil, 1); ok {
		t.Error("expected no match in an empty slice")
	}
}

func TestCodeBlocksIn(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"no blocks", "just prose, no fences", nil},
		{"one block", "intro\n```go\nfmt.Println(\"hi\")\n```\noutro", []string{"fmt.Println(\"hi\")"}},
		{"multiple blocks in order", "```go\na()\n```\ntext\n```python\nb()\n```", []string{"a()", "b()"}},
		{"unclosed trailing fence (streaming-safe)", "prose\n```go\nin progress", []string{"in progress"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := codeBlocksIn(c.content)
			if len(got) != len(c.want) {
				t.Fatalf("codeBlocksIn() = %+v, want %+v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("block %d = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func withStubClipboard(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	orig := copyToClipboardFn
	copyToClipboardFn = fn
	t.Cleanup(func() { copyToClipboardFn = orig })
}

func TestCopySlashCommand_BareCopyUsesLastResponse(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.messages = append(m.messages, newUserMessage("prompt"), newAssistantMessage())
	m.messages[len(m.messages)-1].Content = "the response"

	var gotText string
	withStubClipboard(t, func(text string) (string, error) {
		gotText = text
		return "clipboard", nil
	})

	m.textarea.SetValue("/copy")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if gotText != "the response" {
		t.Errorf("copied text = %q, want %q", gotText, "the response")
	}
	if !containsMessage(m, "Copied (clipboard).") {
		t.Errorf("expected a copied confirmation, got %+v", m.messages)
	}
}

func TestCopySlashCommand_NumberedMessage(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.messages = append(m.messages, newUserMessage("first"), newAssistantMessage())
	m.messages[len(m.messages)-1].Content = "first reply"
	m.messages = append(m.messages, newUserMessage("second"), newAssistantMessage())
	m.messages[len(m.messages)-1].Content = "second reply"

	var gotText string
	withStubClipboard(t, func(text string) (string, error) {
		gotText = text
		return "clipboard", nil
	})

	// Numbering counts every non-system message from the end, not just
	// assistant turns: 1 = "second reply", 2 = the user's "second" prompt,
	// 3 = "first reply".
	m.textarea.SetValue("/copy 3")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if gotText != "first reply" {
		t.Errorf("copied text = %q, want %q", gotText, "first reply")
	}
}

func TestCopySlashCommand_CodeBlock(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.messages = append(m.messages, newUserMessage("prompt"), newAssistantMessage())
	m.messages[len(m.messages)-1].Content = "intro\n```go\nfunc main() {}\n```\noutro"

	var gotText string
	withStubClipboard(t, func(text string) (string, error) {
		gotText = text
		return "clipboard", nil
	})

	m.textarea.SetValue("/copy code")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if gotText != "func main() {}" {
		t.Errorf("copied text = %q, want %q", gotText, "func main() {}")
	}
}

func TestCopySlashCommand_NothingToCopy(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true

	withStubClipboard(t, func(text string) (string, error) {
		t.Fatal("clipboard should not be invoked when there's nothing to copy")
		return "", nil
	})

	m.textarea.SetValue("/copy 5")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !containsMessage(m, "Nothing to copy.") {
		t.Errorf("expected a 'nothing to copy' message, got %+v", m.messages)
	}
}

func TestCopySlashCommand_ClipboardErrorSurfaced(t *testing.T) {
	m := newTestModel("")
	m.vpReady = true
	m.messages = append(m.messages, newUserMessage("prompt"), newAssistantMessage())
	m.messages[len(m.messages)-1].Content = "reply"

	withStubClipboard(t, func(text string) (string, error) {
		return "", errors.New("no clipboard utility found")
	})

	m.textarea.SetValue("/copy")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !containsMessage(m, "Copy failed: no clipboard utility found") {
		t.Errorf("expected the clipboard error surfaced, got %+v", m.messages)
	}
}
