package tui

import (
	"fmt"
	"io"
	"os"

	"github.com/atotto/clipboard"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
)

// maxOSC52Bytes caps the OSC 52 fallback payload — most terminals cap the
// escape sequence somewhere around 100KB and silently ignore anything larger.
const maxOSC52Bytes = 100_000

// copyToClipboardFn is the indirection callers (model.go) use, so tests can
// swap in a stub without requiring a real clipboard utility in CI.
var copyToClipboardFn = copyToClipboard

// copyToClipboard copies text to the system clipboard, falling back to an
// OSC 52 terminal escape sequence (works over SSH in terminals that support
// it — iTerm2, kitty, wezterm, Windows Terminal, and others) when no system
// clipboard utility is available (#288). method describes which path
// succeeded, for the caller to report back to the user. OSC 52 is
// best-effort: the escape sequence can't report success, so method's text
// says so explicitly rather than claiming a guaranteed copy.
func copyToClipboard(text string) (method string, err error) {
	if err := clipboard.WriteAll(text); err == nil {
		return "clipboard", nil
	}
	if !emitOSC52(os.Stdout, text) {
		return "", fmt.Errorf("no clipboard utility found and this terminal doesn't support OSC 52")
	}
	return "OSC 52 (sent to the terminal — depends on terminal support)", nil
}

// emitOSC52 writes the OSC 52 "set clipboard" escape sequence to w, via
// github.com/aymanbagabas/go-osc52 (already a transitive dependency through
// muesli/termenv, used elsewhere in this binary for color-profile detection —
// so this adds no new third-party code). Automatically wraps the sequence for
// tmux passthrough when $TMUX is set (tmux needs `allow-passthrough on` for
// that to reach the outer terminal). Returns false only when text exceeds
// maxOSC52Bytes — anything written within the limit is fire-and-forget, so
// there's no way to confirm the terminal actually acted on it.
func emitOSC52(w io.Writer, text string) bool {
	seq := osc52.New(text).Limit(maxOSC52Bytes)
	if os.Getenv("TMUX") != "" {
		seq = seq.Tmux()
	}
	n, err := seq.WriteTo(w)
	return err == nil && n > 0
}
