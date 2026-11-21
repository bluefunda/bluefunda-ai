package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/checkpoint"
)

// Two turns, the second edits a file: rewinding both restores the file,
// cuts the conversation back to before the second message and drops it
// from the list.
func TestRewind(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(f, []byte("v0"), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := checkpoint.New()
	history := []codeMessage{{Role: "system", Content: "ctx"}}
	turn := func(prompt string) {
		cp.Begin(prompt, prompt+" (expanded)", len(history))
		history = append(history, codeMessage{Role: "user", Content: prompt + " (expanded)"}, codeMessage{Role: "assistant", Content: "ok"})
	}
	turn("first")
	turn("second")
	cp.BeforeEdit(f)
	if err := os.WriteFile(f, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	saved := false
	out, err := rewind(cp, &history, 2, true, true, func() { saved = true })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f); string(b) != "v0" {
		t.Errorf("file = %q, want v0", b)
	}
	if len(history) != 3 || history[2].Content != "ok" || !saved {
		t.Errorf("history = %+v, saved = %v", history, saved)
	}
	if !strings.Contains(out, "Restored 1 file") || !strings.Contains(out, "before: second") {
		t.Errorf("summary = %q", out)
	}
	if pts := cp.Points(); len(pts) != 1 || pts[0].Prompt != "first" {
		t.Errorf("points = %+v", pts)
	}
}

// After compaction rewrote the conversation, only code can be restored.
func TestRewind_ConversationChanged(t *testing.T) {
	cp := checkpoint.New()
	history := []codeMessage{}
	cp.Begin("hi", "hi", 0)
	history = append(history, codeMessage{Role: "user", Content: "hi"})
	pt := cp.Points()[0]
	if !conversationAt(history, pt) {
		t.Fatal("should be reachable before compaction")
	}
	history = []codeMessage{{Role: "system", Content: "[Conversation summary]"}}
	if conversationAt(history, pt) {
		t.Error("should not be reachable after compaction")
	}
	if _, err := rewind(cp, &history, pt.Turn, false, true, func() {}); err == nil {
		t.Error("conversation rewind should fail after compaction")
	}
	if _, err := rewind(cp, &history, 99, true, false, func() {}); err == nil {
		t.Error("unknown turn should fail")
	}
}

// Edit tools save the file to the session's checkpoints before running.
func TestEditToolsCheckpointFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("x.txt", []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := checkpoint.New()
	sessionCheckpoints = cp
	t.Cleanup(func() { sessionCheckpoints = nil })
	cp.Begin("edit it", "edit it", 0)

	if !slices.Contains(editTools, "write_file") {
		t.Fatal("write_file should be an edit tool")
	}
	saveBeforeEdit("write_file", `{"path":"x.txt","content":"after"}`)
	if err := os.WriteFile("x.txt", []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	saveBeforeEdit("read_file", `{"path":"x.txt"}`) // not an edit: ignored
	if _, err := cp.RestoreCode(1); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile("x.txt"); string(b) != "before" {
		t.Errorf("x.txt = %q", b)
	}
}
