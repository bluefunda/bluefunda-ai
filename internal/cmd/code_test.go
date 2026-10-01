package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/audit"
	"github.com/bluefunda/bluefunda-ai/internal/hooks"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
	"github.com/bluefunda/bluefunda-ai/internal/plugins"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

func TestLocalTimezoneName_TZEnvOverride(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	if got := localTimezoneName(); got != "America/New_York" {
		t.Errorf("localTimezoneName() = %q, want %q", got, "America/New_York")
	}
}

func TestLocalTimezoneName_EmptyTZFallsBackToLocaltime(t *testing.T) {
	t.Setenv("TZ", "")
	// Can't control /etc/localtime in a test, so just assert it never returns
	// the misleading literal "Local" that time.Local.String() would produce.
	if got := localTimezoneName(); got == "Local" {
		t.Errorf("localTimezoneName() returned literal %q, want an IANA zone or empty string", got)
	}
}

// TestExecuteTools_ParallelUnderAutoApply verifies the auto-approved path in
// executeTools runs tool calls concurrently rather than one at a time, and
// that results are still returned in the original call order.
func TestExecuteTools_ParallelUnderAutoApply(t *testing.T) {
	const n = 4
	toolCalls := make([]ui.ToolCallEvent, n)
	for i := range toolCalls {
		toolCalls[i] = ui.ToolCallEvent{
			ID:        fmt.Sprintf("call-%d", i),
			Name:      "bash",
			Arguments: fmt.Sprintf(`{"command":"sleep 0.3 && echo done-%d"}`, i),
		}
	}

	auditLog := &audit.Logger{}
	hookRunner := hooks.New("", "test-session", ".")
	mcpMgr := mcp.NewManager(context.Background(), nil)
	pluginMgr := plugins.NewManager(t.TempDir())
	printer := &ui.Printer{Out: io.Discard, Err: io.Discard}
	ch := make(chan tui.StreamEvent, n)

	start := time.Now()
	results := executeTools(toolCalls, nil, nil, true, false, nil, auditLog, hookRunner, mcpMgr, pluginMgr, printer, ch)
	elapsed := time.Since(start)

	// Serial execution of 4 x 300ms sleeps would take ~1.2s; concurrent
	// execution should finish close to a single 300ms sleep.
	if elapsed > 900*time.Millisecond {
		t.Errorf("executeTools took %s for %d concurrent 300ms tools; expected well under serial time", elapsed, n)
	}

	for i, r := range results {
		if r.err != nil {
			t.Fatalf("tool %d: unexpected error: %v", i, r.err)
		}
		want := fmt.Sprintf("done-%d", i)
		if !strings.Contains(r.result, want) {
			t.Errorf("tool %d: result = %q, want it to contain %q (result order not preserved)", i, r.result, want)
		}
	}
}

func TestIsPlanModeBlocked(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		args    string
		blocked bool
	}{
		{"read-only tool allowed", "read_file", `{"path":"x"}`, false},
		{"safe bash allowed", "bash", `{"command":"git status"}`, false},
		{"unsafe bash blocked", "bash", `{"command":"rm -rf /"}`, true},
		{"write_file blocked", "write_file", `{"path":"x","content":"y"}`, true},
		{"edit_file blocked", "edit_file", `{"path":"x"}`, true},
		{"mcp read-only tool allowed", "mcp__github__list_resources", `{}`, false},
		{"mcp real tool blocked", "mcp__github__create_issue", `{}`, true},
		{"task blocked", "task", `{"prompt":"do something"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPlanModeBlocked(c.tool, c.args); got != c.blocked {
				t.Errorf("isPlanModeBlocked(%q, %q) = %v, want %v", c.tool, c.args, got, c.blocked)
			}
		})
	}
}

func TestSessionAllowSet_AddAndContains(t *testing.T) {
	s := newSessionAllowSet()
	if s.contains("bash", `{"command":"git status"}`) {
		t.Fatal("expected a fresh set to contain nothing")
	}
	s.add("bash", `{"command":"git status"}`)
	if !s.contains("bash", `{"command":"git status"}`) {
		t.Error("expected the added tool+args pair to be contained")
	}
}

func TestSessionAllowSet_DifferentArgsTrackedIndependently(t *testing.T) {
	s := newSessionAllowSet()
	s.add("bash", `{"command":"git status"}`)
	if s.contains("bash", `{"command":"git log"}`) {
		t.Error("expected a different command to not be contained")
	}
}

func TestSessionAllowSet_SameArgsDifferentToolNotMatched(t *testing.T) {
	s := newSessionAllowSet()
	s.add("bash", `{"path":"x"}`)
	if s.contains("write_file", `{"path":"x"}`) {
		t.Error("expected the same args under a different tool name to not match")
	}
}

func TestSessionAllowSet_NilIsEmptyAndNoopAdd(t *testing.T) {
	var s *sessionAllowSet
	if s.contains("bash", `{"command":"git status"}`) {
		t.Error("expected a nil set to contain nothing")
	}
	s.add("bash", `{"command":"git status"}`) // must not panic
}
