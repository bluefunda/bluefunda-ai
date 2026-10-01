package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/hooks"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
)

func TestSummarizeMCPStatus_NoneConfigured(t *testing.T) {
	status, detail := summarizeMCPStatus(nil)
	if status != "info" {
		t.Errorf("expected info, got %q", status)
	}
	if detail != "none configured — add mcp_servers to .bai/settings.yaml" {
		t.Errorf("unexpected detail: %q", detail)
	}
}

func TestSummarizeMCPStatus_AllStarted(t *testing.T) {
	statuses := []mcp.ServerStatus{
		{Name: "a", Started: true, Tools: 2, Resources: 1, Prompts: 0},
		{Name: "b", Started: true, Tools: 1, Resources: 0, Prompts: 3},
	}
	status, detail := summarizeMCPStatus(statuses)
	if status != "ok" {
		t.Errorf("expected ok, got %q", status)
	}
	if detail != "2 server(s) connected (3 tools, 1 resources, 3 prompts)" {
		t.Errorf("unexpected detail: %q", detail)
	}
}

func TestSummarizeMCPStatus_SomeFailed(t *testing.T) {
	statuses := []mcp.ServerStatus{
		{Name: "good", Started: true, Tools: 2},
		{Name: "bad", Started: false, Err: "missing url for type http"},
	}
	status, detail := summarizeMCPStatus(statuses)
	if status != "warn" {
		t.Errorf("expected warn, got %q", status)
	}
	if detail != "1/2 server(s) failed to start: bad" {
		t.Errorf("unexpected detail: %q", detail)
	}
}

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
