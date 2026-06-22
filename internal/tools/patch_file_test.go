package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchFile(t *testing.T) {
	write := func(t *testing.T, content string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "patch-*.txt")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		if _, err := f.WriteString(content); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		_ = f.Close()
		return f.Name()
	}

	read := func(t *testing.T, path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(b)
	}

	t.Run("single edit", func(t *testing.T) {
		p := write(t, "hello world\n")
		msg, err := PatchFile(p, []PatchEdit{{OldString: "hello", NewString: "goodbye"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(msg, "1 replacement") {
			t.Errorf("got %q, want replacement count in message", msg)
		}
		if got := read(t, p); got != "goodbye world\n" {
			t.Errorf("file content = %q, want %q", got, "goodbye world\n")
		}
	})

	t.Run("multi edit applied atomically", func(t *testing.T) {
		p := write(t, "foo bar baz\n")
		_, err := PatchFile(p, []PatchEdit{
			{OldString: "foo", NewString: "FOO"},
			{OldString: "baz", NewString: "BAZ"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := read(t, p); got != "FOO bar BAZ\n" {
			t.Errorf("file content = %q, want %q", got, "FOO bar BAZ\n")
		}
	})

	t.Run("replace_all replaces every occurrence", func(t *testing.T) {
		p := write(t, "a a a\n")
		msg, err := PatchFile(p, []PatchEdit{{OldString: "a", NewString: "b", ReplaceAll: true}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(msg, "3 replacement") {
			t.Errorf("got %q, want 3 replacements in message", msg)
		}
		if got := read(t, p); got != "b b b\n" {
			t.Errorf("file content = %q, want %q", got, "b b b\n")
		}
	})

	t.Run("fails when old_string not found", func(t *testing.T) {
		p := write(t, "hello world\n")
		_, err := PatchFile(p, []PatchEdit{{OldString: "nothere", NewString: "x"}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error %q should mention 'not found'", err)
		}
		// File must be unchanged.
		if got := read(t, p); got != "hello world\n" {
			t.Errorf("file was modified after failed patch; got %q", got)
		}
	})

	t.Run("fails when old_string is not unique", func(t *testing.T) {
		p := write(t, "aa aa\n")
		_, err := PatchFile(p, []PatchEdit{{OldString: "aa", NewString: "bb"}})
		if err == nil {
			t.Fatal("expected error for non-unique old_string")
		}
		if !strings.Contains(err.Error(), "2 occurrences") {
			t.Errorf("error %q should mention occurrence count", err)
		}
		if got := read(t, p); got != "aa aa\n" {
			t.Errorf("file was modified after failed patch; got %q", got)
		}
	})

	t.Run("fails on empty old_string", func(t *testing.T) {
		p := write(t, "hello\n")
		_, err := PatchFile(p, []PatchEdit{{OldString: "", NewString: "x"}})
		if err == nil {
			t.Fatal("expected error for empty old_string")
		}
	})

	t.Run("fails with no edits", func(t *testing.T) {
		p := write(t, "hello\n")
		_, err := PatchFile(p, nil)
		if err == nil {
			t.Fatal("expected error for empty edits")
		}
	})

	t.Run("second edit failing leaves file unchanged", func(t *testing.T) {
		p := write(t, "alpha beta\n")
		_, err := PatchFile(p, []PatchEdit{
			{OldString: "alpha", NewString: "ALPHA"},
			{OldString: "missing", NewString: "X"},
		})
		if err == nil {
			t.Fatal("expected error for missing old_string in second edit")
		}
		// File must be unchanged — no partial application.
		if got := read(t, p); got != "alpha beta\n" {
			t.Errorf("file was partially modified; got %q", got)
		}
	})

	t.Run("returns diff output", func(t *testing.T) {
		p := write(t, "line1\nline2\nline3\n")
		msg, err := PatchFile(p, []PatchEdit{{OldString: "line2", NewString: "LINE2"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Diff may be empty if diff(1) is not available, so only check when non-empty.
		if strings.Contains(msg, "@@") && !strings.Contains(msg, "LINE2") {
			t.Errorf("diff output missing replacement; got:\n%s", msg)
		}
	})

	t.Run("path required", func(t *testing.T) {
		_, err := PatchFile("", []PatchEdit{{OldString: "x", NewString: "y"}})
		if err == nil {
			t.Fatal("expected error for empty path")
		}
	})

	t.Run("non-existent file errors cleanly", func(t *testing.T) {
		_, err := PatchFile(filepath.Join(t.TempDir(), "nosuchfile.txt"), []PatchEdit{{OldString: "x", NewString: "y"}})
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestPatchFileViaExecute(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "exec-patch-*.go")
	if err != nil {
		t.Fatal(err)
	}
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	argsJSON := `{"path":"` + f.Name() + `","edits":[{"old_string":"hello","new_string":"world"}]}`
	msg, err := Execute("patch_file", argsJSON)
	if err != nil {
		t.Fatalf("Execute patch_file: %v", err)
	}
	if !strings.Contains(msg, "1 replacement") {
		t.Errorf("Execute result %q missing replacement count", msg)
	}
	b, _ := os.ReadFile(f.Name())
	if !strings.Contains(string(b), "world") {
		t.Errorf("file content after Execute: %q", string(b))
	}
}
