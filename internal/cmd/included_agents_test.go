package cmd

import (
	"context"
	"io"
	"testing"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/audit"
	"github.com/bluefunda/bluefunda-ai/internal/hooks"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
	"github.com/bluefunda/bluefunda-ai/internal/plugins"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

func refNames(refs []*pb.MCPServerRef) []string {
	var out []string
	for _, r := range refs {
		out = append(out, r.GetName())
	}
	return out
}

// Only a listed, available built-in agent is included; other servers need
// connecting, and a user the server doesn't list it for never gets it.
func TestNewAgentSet(t *testing.T) {
	builtIn := builtInAgentNames[0]
	a := newAgentSet([]*pb.MCPInfo{
		{Name: "github-mcp", IsAvailable: true},
		{Name: builtIn, IsAvailable: true},
		{Name: builtIn, IsAvailable: true},
	})
	if got := a.active(); len(got) != 1 || got[0] != builtIn {
		t.Errorf("active = %q", got)
	}
	if got := newAgentSet([]*pb.MCPInfo{{Name: builtIn, IsAvailable: false}}).active(); got != nil {
		t.Errorf("an unavailable agent must not be included: %q", got)
	}
	if got := newAgentSet(nil).active(); got != nil {
		t.Errorf("not listed: %q", got)
	}
	var none *agentSet
	if none.active() != nil || none.isIncluded(builtIn) || none.isOff(builtIn) {
		t.Error("a nil set has nothing")
	}
}

// An included agent can be switched off and back on for the session; other
// names can't be.
func TestAgentSetOnOff(t *testing.T) {
	builtIn := builtInAgentNames[0]
	a := newAgentSet([]*pb.MCPInfo{{Name: builtIn, IsAvailable: true}})
	if !a.setOn(builtIn, false) || a.active() != nil || !a.isOff(builtIn) {
		t.Fatal("switch off failed")
	}
	if !a.setOn(builtIn, true) || len(a.active()) != 1 || a.isOff(builtIn) {
		t.Fatal("switch on failed")
	}
	if a.setOn("github-mcp", false) {
		t.Error("only included agents can be switched")
	}
}

func TestMCPServerRefs(t *testing.T) {
	got := refNames(mcpServerRefs("abaper-mcp", []string{"x-mcp", "abaper-mcp"}))
	if len(got) != 2 || got[0] != "abaper-mcp" || got[1] != "x-mcp" {
		t.Errorf("refs = %q, want the selected server then the included one, no duplicates", got)
	}
	if mcpServerRefs("", nil) != nil {
		t.Error("nothing selected and nothing included: no list")
	}
}

// In chat mode (no local tools) every turn carries the included agents; a
// turn with local tools carries none, since the backend uses those alone.
func TestAgenticLoop_SendsIncludedAgentsInChatMode(t *testing.T) {
	builtIn := builtInAgentNames[0]
	sessionAgents = newAgentSet([]*pb.MCPInfo{{Name: builtIn, IsAvailable: true}})
	t.Cleanup(func() { sessionAgents = nil })
	bff := &scriptedBFF{replies: [][]*pb.ChatEvent{textReply("a"), textReply("b")}}
	conn := startScriptedBFF(t, bff)
	run := func(tools string) {
		ch := make(chan tui.StreamEvent, 64)
		go func() {
			for range ch { //nolint:revive // drain
			}
		}()
		defer close(ch)
		if _, err := agenticLoopTUI(conn, nil, "chat-1", "", tools, []codeMessage{{Role: "user", Content: "q"}}, true,
			nil, nil, true, false, nil, 3, 0, 0, "", &audit.Logger{},
			hooks.New("", "test-session", "."), mcp.NewManager(context.Background(), nil),
			plugins.NewManager(t.TempDir()), &ui.Printer{Out: io.Discard, Err: io.Discard}, ch); err != nil {
			t.Fatal(err)
		}
	}
	run("")
	run("[]")
	bff.mu.Lock()
	defer bff.mu.Unlock()
	if got := refNames(bff.reqs[0].GetMcpServers()); len(got) != 1 || got[0] != builtIn {
		t.Errorf("chat mode sent %q", got)
	}
	if got := bff.reqs[1].GetMcpServers(); len(got) != 0 {
		t.Errorf("a turn with local tools sent %q", refNames(got))
	}
}
