package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withTempHome sets HOME to a temp dir so session files go there, not ~/.bai.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

// --- Save / Load roundtrip ---

func TestSaveLoad_Roundtrip(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()

	path, err := Path(cwd, "test-session")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "do something", ToolCalls: []ToolCall{
			{ID: "tc1", Type: "function", Function: FuncCall{Name: "bash", Arguments: `{"command":"ls"}`}},
		}},
		{Role: "tool", Content: "file.go", ToolCallID: "tc1"},
	}

	if err := Save(path, msgs); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), len(loaded))
	}
	for i, m := range msgs {
		if loaded[i].Role != m.Role {
			t.Errorf("[%d] Role = %q, want %q", i, loaded[i].Role, m.Role)
		}
		if loaded[i].Content != m.Content {
			t.Errorf("[%d] Content = %q, want %q", i, loaded[i].Content, m.Content)
		}
		if loaded[i].ToolCallID != m.ToolCallID {
			t.Errorf("[%d] ToolCallID = %q, want %q", i, loaded[i].ToolCallID, m.ToolCallID)
		}
	}
}

func TestSave_Atomic(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()
	path, _ := Path(cwd, "atomic-test")

	first := []Message{{Role: "user", Content: "first"}}
	second := []Message{{Role: "user", Content: "second"}, {Role: "assistant", Content: "reply"}}

	if err := Save(path, first); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := Save(path, second); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 messages after overwrite, got %d", len(loaded))
	}
	if loaded[0].Content != "second" {
		t.Errorf("expected 'second', got %q", loaded[0].Content)
	}
}

func TestLoad_NonExistent(t *testing.T) {
	withTempHome(t)
	_, err := Load("/no/such/path/session.jsonl")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestSave_EmptyMessages(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()
	path, _ := Path(cwd, "empty-session")

	if err := Save(path, []Message{}); err != nil {
		t.Fatalf("Save empty: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 messages, got %d", len(loaded))
	}
}

// --- Path / Dir ---

func TestPath_ContainsSessionID(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()
	path, err := Path(cwd, "my-session-id")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if filepath.Base(path) != "my-session-id.jsonl" {
		t.Errorf("expected basename 'my-session-id.jsonl', got %q", filepath.Base(path))
	}
}

func TestDir_CreatesMkdirAll(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()
	dir, err := Dir(cwd)
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("expected Dir to create directory at %q", dir)
	}
}

func TestDir_SameCwdSameHash(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()
	d1, _ := Dir(cwd)
	d2, _ := Dir(cwd)
	if d1 != d2 {
		t.Errorf("same cwd should produce same dir: %q vs %q", d1, d2)
	}
}

func TestDir_DifferentCwdDifferentHash(t *testing.T) {
	withTempHome(t)
	cwd1, cwd2 := t.TempDir(), t.TempDir()
	d1, _ := Dir(cwd1)
	d2, _ := Dir(cwd2)
	if d1 == d2 {
		t.Errorf("different cwds should produce different dirs, both got %q", d1)
	}
}

// --- List ---

func TestList_Empty(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()
	// Ensure the dir exists but has no .jsonl files.
	Dir(cwd) //nolint:errcheck

	infos, err := List(cwd)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(infos))
	}
}

func TestList_MultipleFiles_NewestFirst(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()

	// Write three sessions with different mtimes.
	for _, id := range []string{"sess-a", "sess-b", "sess-c"} {
		path, _ := Path(cwd, id)
		Save(path, []Message{{Role: "user", Content: id}}) //nolint:errcheck
	}

	// Touch sess-c so it's clearly the newest.
	path, _ := Path(cwd, "sess-c")
	future := time.Now().Add(5 * time.Second)
	os.Chtimes(path, future, future) //nolint:errcheck

	infos, err := List(cwd)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(infos))
	}
	if infos[0].ID != "sess-c" {
		t.Errorf("expected newest session first (sess-c), got %q", infos[0].ID)
	}
}

func TestList_InfoFields(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()

	path, _ := Path(cwd, "info-session")
	msgs := []Message{
		{Role: "user", Content: "hello world"},
		{Role: "assistant", Content: "hi"},
	}
	Save(path, msgs) //nolint:errcheck

	infos, err := List(cwd)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("expected at least one session")
	}

	info := infos[0]
	if info.ID != "info-session" {
		t.Errorf("ID = %q, want %q", info.ID, "info-session")
	}
	if info.Turns != 2 {
		t.Errorf("Turns = %d, want 2", info.Turns)
	}
	if info.LastMsg == "" {
		t.Error("LastMsg should be non-empty")
	}
	if info.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestList_LastMsgSnippet_Truncated(t *testing.T) {
	withTempHome(t)
	cwd := t.TempDir()
	path, _ := Path(cwd, "long-msg")
	longContent := string(make([]byte, 100))
	for i := range longContent {
		longContent = longContent[:i] + "x" + longContent[i+1:]
	}
	Save(path, []Message{{Role: "user", Content: longContent}}) //nolint:errcheck

	infos, _ := List(cwd)
	if len(infos) == 0 {
		t.Fatal("expected session")
	}
	if len(infos[0].LastMsg) > 63 {
		t.Errorf("LastMsg should be truncated to ≤63 chars, got %d: %q", len(infos[0].LastMsg), infos[0].LastMsg)
	}
}

// --- ListAll ---

func TestListAll_AcrossCWDs(t *testing.T) {
	withTempHome(t)
	cwd1, cwd2 := t.TempDir(), t.TempDir()

	p1, _ := Path(cwd1, "session-x")
	p2, _ := Path(cwd2, "session-y")
	Save(p1, []Message{{Role: "user", Content: "from cwd1"}}) //nolint:errcheck
	Save(p2, []Message{{Role: "user", Content: "from cwd2"}}) //nolint:errcheck

	infos, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(infos) < 2 {
		t.Errorf("expected at least 2 sessions across cwds, got %d", len(infos))
	}
}

func TestListAll_NoSessionsDir(t *testing.T) {
	// Fresh home with no ~/.bai/sessions dir yet.
	withTempHome(t)
	infos, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if infos != nil {
		t.Errorf("expected nil slice with no sessions, got %v", infos)
	}
}
