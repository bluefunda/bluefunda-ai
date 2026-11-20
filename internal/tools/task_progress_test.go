package tools

import (
	"context"
	"strings"
	"testing"
)

func TestActivityLine(t *testing.T) {
	cases := map[string]string{
		`{"command":"go test\n ./..."}`: "· bash go test ./...",
		`{}`:                            "· bash",
		`not json`:                      "· bash",
	}
	for in, want := range cases {
		if got := activityLine("bash", in); got != want {
			t.Errorf("activityLine(%q) = %q, want %q", in, got, want)
		}
	}
	long := activityLine("read_file", `{"path":"`+strings.Repeat("x", 100)+`"}`)
	if !strings.HasSuffix(long, "...") || len([]rune(long)) > 80 {
		t.Errorf("long arg not truncated: %q", long)
	}
}

func TestProgressContext(t *testing.T) {
	progressFrom(context.Background())("ignored") // no reporter: must not panic
	var got []string
	ctx := WithProgress(context.Background(), func(s string) { got = append(got, s) })
	progressFrom(ctx)("a")
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("got %q", got)
	}
}

func TestTailBuffer(t *testing.T) {
	b := &tailBuffer{max: 5}
	_, _ = b.Write([]byte("hello "))
	_, _ = b.Write([]byte("world"))
	if b.String() != "world" {
		t.Errorf("tail = %q, want the last 5 bytes", b.String())
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "tool call") != "1 tool call" || plural(3, "tool call") != "3 tool calls" {
		t.Error("plural")
	}
}
