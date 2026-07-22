package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/memory"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

func testMemoryManager(t *testing.T) *memory.Manager {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	return memory.New(dir)
}

func TestListMemory_Empty(t *testing.T) {
	mgr := testMemoryManager(t)
	p, buf := testPrinter(ui.FormatTable)

	if err := listMemory(mgr, p); err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	if !strings.Contains(buf.String(), "no memory entries") {
		t.Errorf("expected 'no memory entries', got: %s", buf.String())
	}
}

func TestListMemory_Table(t *testing.T) {
	mgr := testMemoryManager(t)
	if _, err := mgr.Write("conventions", "run golangci-lint before committing"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	p, buf := testPrinter(ui.FormatTable)

	if err := listMemory(mgr, p); err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "conventions") || !strings.Contains(out, "project") {
		t.Errorf("expected key and scope in table output, got: %s", out)
	}
}

func TestListMemory_JSON(t *testing.T) {
	mgr := testMemoryManager(t)
	if _, err := mgr.Write("known-bugs", "race in compaction"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	p, buf := testPrinter(ui.FormatJSON)

	if err := listMemory(mgr, p); err != nil {
		t.Fatalf("listMemory: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"KEY"`) || !strings.Contains(out, "known-bugs") {
		t.Errorf("expected JSON with key field, got: %s", out)
	}
}

func TestShowMemory(t *testing.T) {
	mgr := testMemoryManager(t)
	if _, err := mgr.Write("conventions", "full memory content here"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var buf bytes.Buffer

	if err := showMemory(mgr, "conventions", &buf); err != nil {
		t.Fatalf("showMemory: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "full memory content here" {
		t.Errorf("showMemory output = %q", buf.String())
	}
}

func TestShowMemory_NotFound(t *testing.T) {
	mgr := testMemoryManager(t)
	var buf bytes.Buffer
	if err := showMemory(mgr, "nonexistent", &buf); err == nil {
		t.Error("expected error for nonexistent key, got nil")
	}
}

func TestDeleteMemory(t *testing.T) {
	mgr := testMemoryManager(t)
	if _, err := mgr.Write("temp", "temporary note"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	p, buf := testPrinter(ui.FormatTable)

	if err := deleteMemory(mgr, "temp", p); err != nil {
		t.Fatalf("deleteMemory: %v", err)
	}
	if !strings.Contains(buf.String(), "deleted") {
		t.Errorf("expected success message, got: %s", buf.String())
	}
	if _, err := mgr.Read("temp"); err == nil {
		t.Error("expected memory to be gone after delete")
	}
}

func TestDeleteMemory_NotFound(t *testing.T) {
	mgr := testMemoryManager(t)
	p, _ := testPrinter(ui.FormatTable)
	if err := deleteMemory(mgr, "nonexistent", p); err == nil {
		t.Error("expected error deleting nonexistent key, got nil")
	}
}

func TestConfirmMemoryDelete(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"n\n", false},
		{"\n", false},
		{"", false},
	}
	for _, c := range cases {
		var out bytes.Buffer
		got := confirmMemoryDelete(strings.NewReader(c.input), &out, "some-key")
		if got != c.want {
			t.Errorf("confirmMemoryDelete(%q) = %v, want %v", c.input, got, c.want)
		}
		if !strings.Contains(out.String(), "some-key") {
			t.Errorf("expected prompt to mention key, got: %s", out.String())
		}
	}
}
