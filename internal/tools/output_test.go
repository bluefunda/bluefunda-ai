package tools

import (
	"strings"
	"testing"
)

func TestCapOutput(t *testing.T) {
	if got := CapOutput("short", 100); got != "short" {
		t.Errorf("short output changed: %q", got)
	}
	big := "BEGIN" + strings.Repeat("x", 10_000) + "END"
	got := CapOutput(big, 300)
	if !strings.HasPrefix(got, "BEGIN") || !strings.HasSuffix(got, "END") {
		t.Error("start and end must be kept")
	}
	if !strings.Contains(got, "characters omitted") {
		t.Error("missing truncation note")
	}
	if len(got) > 300+300 {
		t.Errorf("capped output too long: %d", len(got))
	}
	// Cutting through a multi-byte rune must not leave invalid UTF-8.
	if s := CapOutput(strings.Repeat("é", 1000), 101); !strings.Contains(s, "omitted") || strings.ContainsRune(s, '�') {
		t.Error("multi-byte cut produced invalid text")
	}
}

func TestExecute_EmptyArgumentsMeanNone(t *testing.T) {
	// Models sometimes call no-argument tools with "" instead of "{}".
	if _, err := Execute("list_dir", ""); err != nil && strings.Contains(err.Error(), "parse arguments") {
		t.Errorf("empty arguments should be accepted: %v", err)
	}
}
