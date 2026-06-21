package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/config"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

const namespacePrefix = "mcp__"

// Manager starts and owns a set of MCP server clients for one bai code session.
type Manager struct {
	clients map[string]*Client // keyed by server name
}

// NewManager starts all MCP servers defined in cfg.MCPServers.
// Servers that fail to start are skipped with a warning printed to stderr.
func NewManager(ctx context.Context, cfg *config.ProjectConfig) *Manager {
	m := &Manager{clients: make(map[string]*Client)}
	if cfg == nil {
		return m
	}
	for name, srv := range cfg.MCPServers {
		if srv.Command == "" {
			fmt.Printf("[bai] mcp %s: missing command — skipping\n", name)
			continue
		}
		c, err := Start(ctx, name, srv.Command, srv.Args, srv.Env)
		if err != nil {
			fmt.Printf("[bai] mcp %s: failed to start: %v\n", name, err)
			continue
		}
		m.clients[name] = c
		fmt.Printf("[bai] mcp %s: started (%d tools)\n", name, len(c.Tools()))
	}
	return m
}

// Close stops all running MCP servers.
func (m *Manager) Close() {
	for _, c := range m.clients {
		c.Stop()
	}
}

// ToolSchemas returns the combined JSON schema for all MCP tools, ready to
// append to the local tools schema. Tools are namespaced as
// mcp__<server>__<tool_name>.
func (m *Manager) ToolSchemas() []tools.ToolSchema {
	var schemas []tools.ToolSchema
	for name, c := range m.clients {
		for _, t := range c.Tools() {
			// namespace: mcp__<server>__<tool>
			qualifiedName := namespacePrefix + name + "__" + t.Name

			params := t.InputSchema
			if params == nil {
				params = json.RawMessage(`{"type":"object","properties":{}}`)
			}

			schemas = append(schemas, tools.ToolSchema{
				Type: "function",
				Function: tools.FunctionDef{
					Name:        qualifiedName,
					Description: fmt.Sprintf("[%s] %s", name, t.Description),
					Parameters:  params,
				},
			})
		}
	}
	return schemas
}

// Execute routes a namespaced tool call to the correct MCP server and returns
// the text result. Returns an error if the tool name is not recognised.
func (m *Manager) Execute(ctx context.Context, qualifiedName, argsJSON string) (string, error) {
	// qualifiedName = mcp__<server>__<tool>
	rest := strings.TrimPrefix(qualifiedName, namespacePrefix)
	idx := strings.Index(rest, "__")
	if idx < 0 {
		return "", fmt.Errorf("invalid mcp tool name: %s", qualifiedName)
	}
	serverName := rest[:idx]
	toolName := rest[idx+2:]

	c, ok := m.clients[serverName]
	if !ok {
		return "", fmt.Errorf("mcp server %q not running", serverName)
	}
	return c.Call(ctx, toolName, argsJSON)
}

// IsMCPTool reports whether the tool name belongs to a local MCP server.
func IsMCPTool(name string) bool {
	return strings.HasPrefix(name, namespacePrefix)
}
