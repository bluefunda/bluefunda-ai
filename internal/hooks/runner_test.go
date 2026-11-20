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

func TestUserPromptSubmit_NoScripts(t *testing.T) {
	r := New(t.TempDir(), "sess", "/work")
	if got := r.UserPromptSubmit("hi"); got.Block || got.AdditionalContext != "" {
		t.Errorf("expected zero Result with no scripts, got %+v", got)
	}
}

func TestUserPromptSubmit_ReceivesPromptAndJoinsContext(t *testing.T) {
	hooksDir := t.TempDir()
	// a.sh proves the prompt arrives on stdin; b.sh adds a second context.
	writeScript(t, hooksDir, "user-prompt-submit", "a.sh",
		`grep -q '"prompt":"fix the build"' && echo '{"additional_context":"branch: main"}'`)
	writeScript(t, hooksDir, "user-prompt-submit", "b.sh", `echo '{"additional_context":"ticket: ABC-1"}'`)

	got := New(hooksDir, "sess", "/work").UserPromptSubmit("fix the build")
	if got.Block {
		t.Fatalf("unexpected block: %+v", got)
	}
	if want := "branch: main\n\nticket: ABC-1"; got.AdditionalContext != want {
		t.Errorf("AdditionalContext = %q, want %q", got.AdditionalContext, want)
	}
}

func TestUserPromptSubmit_BlocksWithSystemMessage(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "user-prompt-submit", "a.sh", `echo '{"system_message":"looks like a secret"}'; exit 2`)
	writeScript(t, hooksDir, "user-prompt-submit", "b.sh", `echo '{"additional_context":"never reached"}'`)

	got := New(hooksDir, "sess", "/work").UserPromptSubmit("AKIA...")
	if !got.Block || got.SystemMessage != "looks like a secret" {
		t.Errorf("got %+v, want block with system_message", got)
	}
	if got.AdditionalContext != "" {
		t.Errorf("blocked result must not carry context, got %q", got.AdditionalContext)
	}
}

func TestUserPromptSubmit_BlockReasonFallsBackToStderr(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "user-prompt-submit", "a.sh", `echo "no prompts on Fridays" >&2; exit 2`)

	got := New(hooksDir, "sess", "/work").UserPromptSubmit("deploy")
	if !got.Block || got.SystemMessage != "no prompts on Fridays" {
		t.Errorf("got %+v, want block with stderr reason", got)
	}
}

func TestUserPromptSubmit_OtherFailureDoesNotBlock(t *testing.T) {
	hooksDir := t.TempDir()
	writeScript(t, hooksDir, "user-prompt-submit", "a.sh", `exit 1`)
	writeScript(t, hooksDir, "user-prompt-submit", "b.sh", `echo '{"additional_context":"still runs"}'`)

	got := New(hooksDir, "sess", "/work").UserPromptSubmit("hi")
	if got.Block || got.AdditionalContext != "still runs" {
		t.Errorf("got %+v, want non-blocking with later context", got)
	}
}
