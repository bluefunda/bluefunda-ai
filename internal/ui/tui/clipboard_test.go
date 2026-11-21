package tui

import (
	"bytes"
	"strings"
	"testing"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
)

func TestEmitOSC52_WritesExpectedSequence(t *testing.T) {
	t.Setenv("TMUX", "")

	var buf bytes.Buffer
	if ok := emitOSC52(&buf, "hello"); !ok {
		t.Fatal("expected emitOSC52 to succeed for small input")
	}

	want := osc52.New("hello").Limit(maxOSC52Bytes).String()
	if buf.String() != want {
		t.Errorf("emitOSC52 wrote %q, want %q", buf.String(), want)
	}
}

func TestEmitOSC52_WrapsForTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	var buf bytes.Buffer
	if ok := emitOSC52(&buf, "hello"); !ok {
		t.Fatal("expected emitOSC52 to succeed for small input")
	}

	want := osc52.New("hello").Limit(maxOSC52Bytes).Tmux().String()
	if buf.String() != want {
		t.Errorf("emitOSC52 under $TMUX wrote %q, want the Tmux-wrapped sequence %q", buf.String(), want)
	}
}

func TestEmitOSC52_OversizedInputWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	oversized := strings.Repeat("x", maxOSC52Bytes+1)
	if ok := emitOSC52(&buf, oversized); ok {
		t.Fatal("expected emitOSC52 to return false for oversized input")
	}
	if buf.Len() != 0 {
		t.Errorf("expected nothing written for oversized input, got %d bytes", buf.Len())
	}
}

func TestIsSSH(t *testing.T) {
	for _, k := range []string{"SSH_TTY", "SSH_CONNECTION", "SSH_CLIENT"} {
		t.Setenv(k, "")
	}
	if isSSH() {
		t.Error("no SSH variables set: want false")
	}
	t.Setenv("SSH_CONNECTION", "10.0.0.1 5000 10.0.0.2 22")
	if !isSSH() {
		t.Error("SSH_CONNECTION set: want true")
	}
}
