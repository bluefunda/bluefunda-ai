package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/audit"
	"github.com/bluefunda/bluefunda-ai/internal/config"
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

func TestSessionAllowSet_AddWithoutPersistPathDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	s := newSessionAllowSet() // persistPath left empty — opt-in off
	s.add("bash", `{"command":"go test ./..."}`)
	if _, err := os.Stat(filepath.Join(dir, ".bai", "settings.local.yaml")); !os.IsNotExist(err) {
		t.Error("expected no local config file to be written when persistence is not enabled")
	}
}

func TestSessionAllowSet_AddWithPersistPathWritesLiteralRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bai", "settings.local.yaml")
	s := newSessionAllowSet()
	s.persistPath = path
	s.add("bash", `{"command":"go test ./..."}`)

	lc, _ := config.FindLocalConfig(dir)
	if lc == nil {
		t.Fatal("expected local config to be created")
	}
	want := "bash:go test ./..."
	if len(lc.Permissions.Allow) != 1 || lc.Permissions.Allow[0] != want {
		t.Errorf("Permissions.Allow = %+v, want [%q]", lc.Permissions.Allow, want)
	}
}

func TestSessionAllowSet_AddWithPersistPathSkipsGlobMetacharacters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bai", "settings.local.yaml")
	s := newSessionAllowSet()
	s.persistPath = path
	// The command itself contains '*' — persisting "bash:rm -rf *" verbatim
	// would be misread as a wildcard by globMatch, broadening far beyond what
	// was approved. Must stay session-only instead.
	s.add("bash", `{"command":"echo * matches everything here"}`)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no local config file to be written for a literal containing '*'")
	}
	// Session-scoped behavior must still work regardless.
	if !s.contains("bash", `{"command":"echo * matches everything here"}`) {
		t.Error("expected session-scoped contains() to still work")
	}
}

func TestResolveMaxTurns(t *testing.T) {
	cases := []struct {
		name            string
		flagValue       int
		projectMaxTurns int
		want            int
	}{
		{"flag left at default, project set — project wins", defaultMaxTurnsFlag, 10, 10},
		{"flag left at default, no project value — stays default", defaultMaxTurnsFlag, 0, defaultMaxTurnsFlag},
		{"flag explicitly set to something else — flag wins", 5, 10, 5},
		{"project value non-positive — ignored", defaultMaxTurnsFlag, -1, defaultMaxTurnsFlag},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveMaxTurns(c.flagValue, c.projectMaxTurns); got != c.want {
				t.Errorf("resolveMaxTurns(%d, %d) = %d, want %d", c.flagValue, c.projectMaxTurns, got, c.want)
			}
		})
	}
}
