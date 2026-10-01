package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMemoryFixture writes a project-scope memory entry at
// <dir>/.bai/memory/<key>.md with the given body.
func writeMemoryFixture(t *testing.T, dir, key, body string) {
	t.Helper()
	memDir := filepath.Join(dir, ".bai", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, key+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMemorySlashCommand_EmptyShowsHint(t *testing.T) {
	t.Chdir(t.TempDir())

	m := newTestModel("")
	m.vpReady = true
	m.textarea.SetValue("/memory")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !lastMessageContains(m, "No memory entries") {
		t.Errorf("expected an empty-state hint, got %+v", m.messages)
	}
}

func TestMemorySlashCommand_ListsEntry(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeMemoryFixture(t, dir, "arch-notes", "Repo uses a monorepo layout.\nMore detail here.")

	m := newTestModel("")
	m.vpReady = true
	m.textarea.SetValue("/memory")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !lastMessageContains(m, "arch-notes") || !lastMessageContains(m, "Repo uses a monorepo layout") {
		t.Errorf("expected the listing to mention 'arch-notes' and its preview, got %+v", m.messages)
	}
}

func TestMemorySlashCommand_ShowsEntryContent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeMemoryFixture(t, dir, "arch-notes", "Full content of the note.")

	m := newTestModel("")
	m.vpReady = true
	m.textarea.SetValue("/memory arch-notes")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !lastMessageContains(m, "Full content of the note.") {
		t.Errorf("expected the full entry content, got %+v", m.messages)
	}
}

func TestMemorySlashCommand_MissingKeyReportsError(t *testing.T) {
	t.Chdir(t.TempDir())

	m := newTestModel("")
	m.vpReady = true
	m.textarea.SetValue("/memory does-not-exist")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !lastMessageContains(m, "Error reading does-not-exist") {
		t.Errorf("expected a read error, got %+v", m.messages)
	}
}

func TestMemorySlashCommand_Delete(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeMemoryFixture(t, dir, "arch-notes", "Will be deleted.")

	m := newTestModel("")
	m.vpReady = true
	m.textarea.SetValue("/memory delete arch-notes")
	nm, _ := m.submitInput()
	m = nm.(Model)

	if !lastMessageContains(m, "Deleted memory entry: arch-notes") {
		t.Errorf("expected a deletion confirmation, got %+v", m.messages)
	}

	// A follow-up listing should no longer show it.
	m.textarea.SetValue("/memory")
	nm, _ = m.submitInput()
	m = nm.(Model)
	if !lastMessageContains(m, "No memory entries") {
		t.Errorf("expected no entries after delete, got %+v", m.messages)
	}
}

func lastMessageContains(m Model, substr string) bool {
	for _, msg := range m.messages {
		if strings.Contains(msg.Content, substr) {
			return true
		}
	}
	return false
}
