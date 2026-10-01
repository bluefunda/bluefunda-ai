package hooks

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeScript creates an executable shell script at <dir>/<phase>/<name> with
// the given body and returns the hooks root directory (dir).
func writeScript(t *testing.T, hooksDir, phase, name, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hook scripts require a POSIX shell")
	}
	dir := filepath.Join(hooksDir, phase)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestSessionStart_NoScripts(t *testing.T) {
	r := New(t.TempDir(), "sess", "/work")
	if got := r.SessionStart("auto", "1.0"); got != "" {
		t.Errorf("expected empty string with no scripts, got %q", got)
	}
}

func TestSessionStart_JoinsAdditionalContext(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "session-start", "a.sh", `echo '{"additional_context":"hello from hook"}'`)

	r := New(hooksDir, "sess", "/work")
	got := r.SessionStart("auto", "1.0")
	if got != "hello from hook" {
		t.Errorf("expected 'hello from hook', got %q", got)
	}
}

func TestSessionEnd_DoesNotPanicWithScript(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "session-end", "notify.sh", `cat >/dev/null`)

	r := New(hooksDir, "sess", "/work")
	r.SessionEnd(3, "end_turn") // must not panic or hang
}

func TestStop_NoScripts(t *testing.T) {
	r := New(t.TempDir(), "sess", "/work")
	if res := r.Stop(); res.Block {
		t.Error("expected no block with no scripts configured")
	}
}

func TestStop_BlocksOnExitCode2(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "stop", "guard.sh", `echo '{"system_message":"tests not run yet"}'; exit 2`)

	r := New(hooksDir, "sess", "/work")
	res := r.Stop()
	if !res.Block {
		t.Fatal("expected Stop to block")
	}
	if res.SystemMessage != "tests not run yet" {
		t.Errorf("expected system message 'tests not run yet', got %q", res.SystemMessage)
	}
}

func TestStop_PassesThroughOnExitZero(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "stop", "ok.sh", `exit 0`)

	r := New(hooksDir, "sess", "/work")
	if res := r.Stop(); res.Block {
		t.Error("expected no block on exit 0")
	}
}

func TestPreCompact_NoScripts(t *testing.T) {
	r := New(t.TempDir(), "sess", "/work")
	if res := r.PreCompact(50000); res.Block {
		t.Error("expected no block with no scripts configured")
	}
}

func TestPreCompact_BlocksOnExitCode2(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "pre-compact", "backup.sh", `echo '{"system_message":"archiving first"}'; exit 2`)

	r := New(hooksDir, "sess", "/work")
	res := r.PreCompact(120000)
	if !res.Block {
		t.Fatal("expected PreCompact to block")
	}
	if res.SystemMessage != "archiving first" {
		t.Errorf("expected system message 'archiving first', got %q", res.SystemMessage)
	}
}

func TestPreCompact_PassesThroughOnExitZero(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "pre-compact", "noop.sh", `exit 0`)

	r := New(hooksDir, "sess", "/work")
	if res := r.PreCompact(120000); res.Block {
		t.Error("expected no block on exit 0")
	}
}

func TestFindLifecycleScripts_EmptyHooksDir(t *testing.T) {
	r := New("", "sess", "/work")
	if got := r.findLifecycleScripts("stop"); got != nil {
		t.Errorf("expected nil for empty hooksDir, got %v", got)
	}
}
