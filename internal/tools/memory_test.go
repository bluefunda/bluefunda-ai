package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp switches the process cwd to a fresh temp dir for the duration of
// the test and points $HOME at a separate temp dir so user-scope memory
// never touches the real ~/.bai/memory.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	home := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	t.Setenv("HOME", home)
	return dir
}

func TestMemoryWriteReadDelete(t *testing.T) {
	chdirTemp(t)

	msg, err := MemoryWrite("conventions", "run golangci-lint before committing")
	if err != nil {
		t.Fatalf("MemoryWrite: %v", err)
	}
	if !strings.Contains(msg, "conventions") {
		t.Errorf("MemoryWrite result = %q, want it to mention the key", msg)
	}
	if _, err := os.Stat(filepath.Join(".bai", "memory", "conventions.md")); err != nil {
		t.Errorf("expected file on disk: %v", err)
	}

	got, err := MemoryRead("conventions")
	if err != nil {
		t.Fatalf("MemoryRead: %v", err)
	}
	if got != "run golangci-lint before committing" {
		t.Errorf("MemoryRead = %q", got)
	}

	delMsg, err := MemoryDelete("conventions")
	if err != nil {
		t.Fatalf("MemoryDelete: %v", err)
	}
	if !strings.Contains(delMsg, "conventions") {
		t.Errorf("MemoryDelete result = %q, want it to mention the key", delMsg)
	}
	if _, err := MemoryRead("conventions"); err == nil {
		t.Error("expected error reading deleted key, got nil")
	}
}

func TestMemoryRead_NotFound(t *testing.T) {
	chdirTemp(t)
	if _, err := MemoryRead("nonexistent"); err == nil {
		t.Error("expected error for nonexistent key, got nil")
	}
}

func TestMemoryList(t *testing.T) {
	chdirTemp(t)

	if got, err := MemoryList(); err != nil || got != "no memory entries" {
		t.Errorf("MemoryList (empty) = %q, %v", got, err)
	}

	if _, err := MemoryWrite("alpha", "first note"); err != nil {
		t.Fatalf("MemoryWrite: %v", err)
	}
	if _, err := MemoryWrite("beta", "second note"); err != nil {
		t.Fatalf("MemoryWrite: %v", err)
	}

	got, err := MemoryList()
	if err != nil {
		t.Fatalf("MemoryList: %v", err)
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "first note") {
		t.Errorf("MemoryList = %q, want alpha entry with preview", got)
	}
	if !strings.Contains(got, "beta") || !strings.Contains(got, "second note") {
		t.Errorf("MemoryList = %q, want beta entry with preview", got)
	}
}

func TestExecute_MemoryTools(t *testing.T) {
	chdirTemp(t)

	writeArgs, _ := json.Marshal(map[string]string{"key": "known-bugs", "content": "issue #144 tracked here"})
	if _, err := Execute("memory_write", string(writeArgs)); err != nil {
		t.Fatalf("Execute memory_write: %v", err)
	}

	readArgs, _ := json.Marshal(map[string]string{"key": "known-bugs"})
	got, err := Execute("memory_read", string(readArgs))
	if err != nil {
		t.Fatalf("Execute memory_read: %v", err)
	}
	if got != "issue #144 tracked here" {
		t.Errorf("Execute memory_read = %q", got)
	}

	listOut, err := Execute("memory_list", "{}")
	if err != nil {
		t.Fatalf("Execute memory_list: %v", err)
	}
	if !strings.Contains(listOut, "known-bugs") {
		t.Errorf("Execute memory_list = %q, want it to list known-bugs", listOut)
	}

	deleteArgs, _ := json.Marshal(map[string]string{"key": "known-bugs"})
	if _, err := Execute("memory_delete", string(deleteArgs)); err != nil {
		t.Fatalf("Execute memory_delete: %v", err)
	}
	if _, err := Execute("memory_read", string(readArgs)); err == nil {
		t.Error("expected error reading deleted key via Execute, got nil")
	}
}

func TestNeedsApproval_MemoryTools(t *testing.T) {
	cases := map[string]bool{
		"memory_read":   false,
		"memory_list":   false,
		"memory_write":  true,
		"memory_delete": true,
	}
	for tool, want := range cases {
		if got := NeedsApproval(tool); got != want {
			t.Errorf("NeedsApproval(%q) = %v, want %v", tool, got, want)
		}
	}
}

func TestCheckPermissions_MemoryKeyGlob(t *testing.T) {
	writeArgs := `{"key":"secrets","content":"do not store this"}`
	deny := []string{"memory_write:secrets"}
	if got := CheckPermissions(nil, deny, "memory_write", writeArgs); got != PermitDeny {
		t.Errorf("CheckPermissions with deny on key = %v, want PermitDeny", got)
	}

	allow := []string{"memory_read", "memory_list"}
	readArgs := `{"key":"conventions"}`
	if got := CheckPermissions(allow, nil, "memory_read", readArgs); got != PermitAuto {
		t.Errorf("CheckPermissions memory_read with allow = %v, want PermitAuto", got)
	}
}

func TestLocalToolSchemas_IncludesMemoryTools(t *testing.T) {
	schemasJSON, err := LocalToolSchemas()
	if err != nil {
		t.Fatalf("LocalToolSchemas: %v", err)
	}
	var schemas []ToolSchema
	if err := json.Unmarshal([]byte(schemasJSON), &schemas); err != nil {
		t.Fatalf("unmarshal schemas: %v", err)
	}
	names := make(map[string]bool)
	for _, s := range schemas {
		names[s.Function.Name] = true
	}
	for _, want := range []string{"memory_read", "memory_list", "memory_write", "memory_delete"} {
		if !names[want] {
			t.Errorf("LocalToolSchemas missing %q", want)
		}
	}
}
