package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/hooks"
)

func TestCountHookScripts_NoHooksDir(t *testing.T) {
	if got := countHookScripts(filepath.Join(t.TempDir(), "missing"), hooks.Phases); got != 0 {
		t.Errorf("expected 0 for a missing hooks dir, got %d", got)
	}
}

func TestCountHookScripts_RecursesIntoPhaseSubdirectories(t *testing.T) {
	hooksDir := t.TempDir()
	mustWriteFile(t, filepath.Join(hooksDir, "pre-tool", "bash.sh"))
	mustWriteFile(t, filepath.Join(hooksDir, "post-tool", "wildcard.sh"))
	mustWriteFile(t, filepath.Join(hooksDir, "stop", "guard.sh"))

	// Regression check: before the fix, countHookScripts (then inline in
	// runDoctor) only looked at hooksDir itself, where pre-tool/post-tool/stop
	// are directories (skipped), so it always reported 0 even with scripts
	// present one level down.
	if got := countHookScripts(hooksDir, hooks.Phases); got != 3 {
		t.Errorf("expected 3 hook scripts across subdirectories, got %d", got)
	}
}

func TestCountHookScripts_EmptyPhaseDirsIgnored(t *testing.T) {
	hooksDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hooksDir, "session-start"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := countHookScripts(hooksDir, hooks.Phases); got != 0 {
		t.Errorf("expected 0 for an empty phase directory, got %d", got)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
