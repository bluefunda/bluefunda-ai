package sdk

import (
	"context"
	"testing"
)

func TestNewClient_defaults(t *testing.T) {
	c := NewClient(Options{})
	if c.opts.BinaryPath != "bai" {
		t.Errorf("BinaryPath = %q, want %q", c.opts.BinaryPath, "bai")
	}
	if c.opts.MaxTurns != 20 {
		t.Errorf("MaxTurns = %d, want 20", c.opts.MaxTurns)
	}
}

func TestNewClient_customOpts(t *testing.T) {
	c := NewClient(Options{
		BinaryPath:  "/usr/local/bin/bai",
		Model:       "fast",
		WorkDir:     "/tmp",
		MaxTurns:    5,
		AutoApprove: true,
	})
	if c.opts.BinaryPath != "/usr/local/bin/bai" {
		t.Errorf("BinaryPath = %q, want /usr/local/bin/bai", c.opts.BinaryPath)
	}
	if c.opts.Model != "fast" {
		t.Errorf("Model = %q, want fast", c.opts.Model)
	}
	if c.opts.MaxTurns != 5 {
		t.Errorf("MaxTurns = %d, want 5", c.opts.MaxTurns)
	}
}

func TestSend_missingBinary(t *testing.T) {
	c := NewClient(Options{BinaryPath: "/nonexistent/bai-test-binary"})
	_, err := c.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestStop_noop(t *testing.T) {
	c := NewClient(Options{})
	if err := c.Stop(); err != nil {
		t.Errorf("Stop on idle client: %v", err)
	}
}
