package cmd

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// A manual /compact keeps only the system messages and the summary, and the
// user's focus reaches the summary request.
func TestCompactHistory_ManualKeepsOnlySummary(t *testing.T) {
	bff := &scriptedBFF{replies: [][]*pb.ChatEvent{textReply("the summary")}}
	conn := startScriptedBFF(t, bff)
	history := []codeMessage{
		{Role: "system", Content: "ctx"},
		{Role: "user", Content: "one"},
		{Role: "assistant", Content: "two"},
		{Role: "user", Content: "three"},
		{Role: "assistant", Content: "four"},
	}
	got, err := compactHistory(conn, "c", "", history, "the API decisions", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Content != "ctx" || got[1].Content != "[Conversation summary]\nthe summary" {
		t.Errorf("compacted = %+v", got)
	}
	sent := bff.sentHistory(t, 0)
	if last := sent[len(sent)-1].Content; !strings.Contains(last, "focus on: the API decisions") {
		t.Errorf("summary request lacks the focus: %q", last)
	}
}

// /compact refuses while a turn holds the lock instead of rewriting the
// history the turn is using.
func TestCompactCommand_RefusesDuringTurn(t *testing.T) {
	var mu sync.Mutex
	ran := false
	cmd := compactCommand(&mu, func(string) (string, error) { ran = true; return "ok", nil })

	mu.Lock()
	if _, err := cmd.Run(""); !errors.Is(err, errTurnRunning) || ran {
		t.Errorf("during a turn: err = %v, ran = %v", err, ran)
	}
	mu.Unlock()

	if out, err := cmd.Run("focus"); err != nil || out != "ok" || !ran {
		t.Errorf("idle: out = %q, err = %v", out, err)
	}
	if !mu.TryLock() {
		t.Error("lock not released after /compact")
	}
}

func TestHumanTokens(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1.0k", 12345: "12.3k"} {
		if got := humanTokens(n); got != want {
			t.Errorf("humanTokens(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestMergeSlashCommands_FirstWins(t *testing.T) {
	user := []tui.SlashCommand{{Name: "/init", Prompt: "mine"}}
	got := mergeSlashCommands(user, nil, promptCommands())
	n := 0
	for _, c := range got {
		if c.Name == "/init" {
			n++
			if c.Prompt != "mine" {
				t.Errorf("a .bai/commands file should override the built-in /init")
			}
		}
	}
	if n != 1 {
		t.Errorf("/init listed %d times", n)
	}
}

// Every built-in prompt command has a description and a prompt that takes
// the text typed after it.
func TestPromptCommands(t *testing.T) {
	for _, c := range promptCommands() {
		if c.Description == "" || !strings.Contains(c.Prompt, "$ARGUMENTS") {
			t.Errorf("%s: description %q, prompt without $ARGUMENTS", c.Name, c.Description)
		}
	}
}

func TestPermissionsReport(t *testing.T) {
	sa := newSessionAllowSet()
	sa.add("write_file", `{"path":"a"}`)
	out := permissionsReport([]ruleSource{
		{".bai/settings.yaml", []string{"bash:make test"}, nil},
		{"empty", nil, nil},
		{"--allowed-tools / --disallowed-tools", nil, []string{"bash:rm *"}},
	}, sa.describe())
	for _, want := range []string{".bai/settings.yaml\n  allow  bash:make test", "deny   bash:rm *", "Allowed for this session", "write_file"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "empty") {
		t.Errorf("a source with no rules should be left out:\n%s", out)
	}
	if none := permissionsReport(nil, nil); !strings.Contains(none, "(none)") {
		t.Errorf("no rules: %s", none)
	}
}

func TestSessionAllowDescribe(t *testing.T) {
	var nilSet *sessionAllowSet
	if nilSet.describe() != nil {
		t.Error("nil set")
	}
	sa := newSessionAllowSet()
	sa.add("bash", `{"command":"make deploy"}`)
	sa.add("web_fetch", `{"url":"x"}`)
	sa.exact["bash\x00"+`{"command":"rm -rf build"}`] = true
	got := sa.describe()
	if len(got) < 3 || !strings.HasPrefix(got[0], "bash:make") || !slices.Contains(got, "web_fetch") ||
		!slices.Contains(got, "bash rm -rf build (this exact call)") {
		t.Errorf("describe = %q", got)
	}
}

func TestAgentsReport(t *testing.T) {
	out := agentsReport(append(tools.BuiltinAgentTypes(),
		tools.AgentType{Name: "rev", Description: "Reviews", Tools: []string{"read_file"}, Model: "think", Source: "/x/rev.md"}))
	for _, want := range []string{"general  (built-in)", "tools: all tools", "rev  (/x/rev.md)", "tools: read_file", "model: think"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestStatusLines(t *testing.T) {
	lines := statusLines("Context: x", []string{"a"}, nil, []mcp.ServerStatus{{Name: "gh", Started: true}, {Name: "db"}})
	got := strings.Join(lines, "\n")
	for _, want := range []string{"Context: x", "1 allow, 0 deny", "gh, db (failed)", "explore"} {
		if !strings.Contains(got, want) {
			t.Errorf("status lacks %q:\n%s", want, got)
		}
	}
}

// Accept-edits mode approves file edits but nothing else.
func TestAcceptEditsMode(t *testing.T) {
	sa := newSessionAllowSet()
	if sa.contains("write_file", `{"path":"a"}`) {
		t.Fatal("edits approved before the mode is on")
	}
	sa.setAcceptEdits(true)
	for _, tool := range editTools {
		if !sa.contains(tool, `{"path":"a"}`) {
			t.Errorf("%s not approved in accept-edits mode", tool)
		}
	}
	if sa.contains("bash", `{"command":"rm -rf x"}`) {
		t.Error("accept-edits must not approve bash")
	}
	sa.setAcceptEdits(false)
	if sa.contains("edit_file", `{"path":"a"}`) {
		t.Error("still approving after the mode is off")
	}
}

func TestShellContext(t *testing.T) {
	got := shellContext("cat x.md", "a\n```go\ncode\n```\n")
	if !strings.Contains(got, "$ cat x.md\n````\na\n") || !strings.HasSuffix(got, "\n````") {
		t.Errorf("fence must outlast the output's own fences:\n%s", got)
	}
}
