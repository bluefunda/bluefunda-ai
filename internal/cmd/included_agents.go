package cmd

import (
	"slices"
	"sync"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
)

// builtInAgentNames are agents with nothing of the user's own to connect,
// recognised by server name as the web app does. The server lists one only
// for the users entitled to it, so a listed built-in agent counts as
// connected: on in every chat, and switchable off for a session.
var builtInAgentNames = []string{"kb-mcp"}

// agentSet is a session's included agents and which the user switched off.
type agentSet struct {
	mu       sync.Mutex
	included []string
	off      map[string]bool
}

// newAgentSet keeps the listed, available built-in agents from servers.
func newAgentSet(servers []*pb.MCPInfo) *agentSet {
	a := &agentSet{off: map[string]bool{}}
	a.set(servers)
	return a
}

func (a *agentSet) set(servers []*pb.MCPInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.included = nil
	for _, s := range servers {
		if s.GetIsAvailable() && slices.Contains(builtInAgentNames, s.GetName()) && !slices.Contains(a.included, s.GetName()) {
			a.included = append(a.included, s.GetName())
		}
	}
}

// loadIncludedAgents asks the server, in the background, which agents this
// user has; until it answers (or if it fails) the session has none.
func loadIncludedAgents(conn *caigrpc.Conn) *agentSet {
	a := newAgentSet(nil)
	go func() {
		ctx, cancel := caigrpc.ContextWithTimeout()
		defer cancel()
		if resp, err := conn.Client.GetMcpInfo(ctx, &pb.GetMcpInfoRequest{}); err == nil {
			a.set(resp.GetMcpServers())
		}
	}()
	return a
}

// sessionAgents is the agentic session's included agents (nil outside one).
var sessionAgents *agentSet

// active returns the included agents not switched off.
func (a *agentSet) active() []string {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for _, n := range a.included {
		if !a.off[n] {
			out = append(out, n)
		}
	}
	return out
}

// isIncluded reports whether name is one of the session's included agents.
func (a *agentSet) isIncluded(name string) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Contains(a.included, name)
}

// setOn switches an included agent on or off for the session; it reports
// false if name isn't included.
func (a *agentSet) setOn(name string, on bool) bool {
	if !a.isIncluded(name) {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.off[name] = !on
	return true
}

// isOff reports whether an included agent was switched off.
func (a *agentSet) isOff(name string) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.off[name]
}

// mcpServerRefs is the mcp_servers list for a chat turn: the server the
// user selected, then the active included agents, without duplicates.
func mcpServerRefs(selected string, agents []string) []*pb.MCPServerRef {
	var refs []*pb.MCPServerRef
	seen := map[string]bool{}
	for _, n := range append([]string{selected}, agents...) {
		if n != "" && !seen[n] {
			seen[n] = true
			refs = append(refs, &pb.MCPServerRef{Name: n})
		}
	}
	return refs
}
