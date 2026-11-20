package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/hooks"
)

func promptHookRunner(t *testing.T, body string) *hooks.Runner {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hook scripts require a POSIX shell")
	}
	hooksDir := t.TempDir()
	dir := filepath.Join(hooksDir, "user-prompt-submit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "h.sh"), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return hooks.New(hooksDir, "sess", ".")
}

func TestApplyPromptHooks_NoHooksPassesPromptThrough(t *testing.T) {
	msg, blocked := applyPromptHooks(hooks.New("", "sess", "."), "hello")
	if msg != "hello" || blocked != "" {
		t.Errorf("got (%q, %q), want prompt unchanged", msg, blocked)
	}
}

func TestApplyPromptHooks_Block(t *testing.T) {
	r := promptHookRunner(t, `echo "contains a secret" >&2; exit 2`)
	msg, blocked := applyPromptHooks(r, "my key is AKIA...")
	if msg != "" {
		t.Errorf("blocked prompt must not produce a message, got %q", msg)
	}
	if !strings.Contains(blocked, "contains a secret") {
		t.Errorf("blocked = %q, want the hook's reason", blocked)
	}
}

// Context is appended in tags, and stripping it restores exactly what the
// user typed, so /retry and `bai chat retry` don't stack a second copy.
func TestApplyPromptHooks_ContextRoundTrip(t *testing.T) {
	r := promptHookRunner(t, `echo '{"additional_context":"branch: main"}'`)
	prompt := "fix the build\nplease"
	msg, blocked := applyPromptHooks(r, prompt)
	if blocked != "" {
		t.Fatalf("unexpected block: %q", blocked)
	}
	if !strings.HasPrefix(msg, prompt) || !strings.Contains(msg, "<user-prompt-submit-hook>\nbranch: main\n</user-prompt-submit-hook>") {
		t.Errorf("msg = %q, want prompt followed by tagged context", msg)
	}
	if got := stripPromptContext(msg); got != prompt {
		t.Errorf("stripPromptContext = %q, want %q", got, prompt)
	}
}

func TestStripPromptContext_LeavesOtherTextAlone(t *testing.T) {
	for _, s := range []string{
		"plain prompt",
		"talking about </user-prompt-submit-hook> mid-prompt",
		"",
	} {
		if got := stripPromptContext(s); got != s {
			t.Errorf("stripPromptContext(%q) = %q, want unchanged", s, got)
		}
	}
}
