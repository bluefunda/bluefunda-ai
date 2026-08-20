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
	results := executeTools(toolCalls, nil, nil, true, auditLog, hookRunner, mcpMgr, pluginMgr, printer, ch)
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
