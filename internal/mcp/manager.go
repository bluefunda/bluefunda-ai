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

// Reserved per-server tool-name suffixes synthesized for resources/prompts
// support. An MCP server whose actual tool happens to be named one of these
// would be shadowed — an accepted edge case, same tradeoff the mcp__/__
// namespacing scheme already makes.
const (
	listResourcesTool = "list_resources"
	readResourceTool  = "read_resource"
	listPromptsTool   = "list_prompts"
	getPromptTool     = "get_prompt"
)

// mcpClient is satisfied by both the stdio Client and the Streamable HTTP
// httpClient, letting Manager hold either transport uniformly.
type mcpClient interface {
	Tools() []Tool
	Resources() []Resource
	Prompts() []Prompt
	Call(ctx context.Context, toolName, argsJSON string) (string, error)
	ReadResource(ctx context.Context, uri string) (string, error)
	GetPrompt(ctx context.Context, name string, args map[string]string) (string, error)
	Stop()
}

// Manager starts and owns a set of MCP server clients for one bai code session.
type Manager struct {
	clients map[string]mcpClient // keyed by server name
}

// NewManager starts all MCP servers defined in cfg.MCPServers.
// Servers that fail to start are skipped with a warning printed to stderr.
func NewManager(ctx context.Context, cfg *config.ProjectConfig) *Manager {
	m := &Manager{clients: make(map[string]mcpClient)}
	if cfg == nil {
		return m
	}
	for name, srv := range cfg.MCPServers {
		var (
			c   mcpClient
			err error
		)
		switch srv.EffectiveTransport() {
		case "http":
			if srv.URL == "" {
				fmt.Printf("[bai] mcp %s: missing url for type http — skipping\n", name)
				continue
			}
			c, err = StartHTTP(ctx, name, srv.URL, srv.Headers)
		default:
			if srv.Command == "" {
				fmt.Printf("[bai] mcp %s: missing command — skipping\n", name)
				continue
			}
			c, err = Start(ctx, name, srv.Command, srv.Args, srv.Env)
		}
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
		if len(c.Resources()) > 0 {
			schemas = append(schemas, resourceToolSchemas(name)...)
		}
		if len(c.Prompts()) > 0 {
			schemas = append(schemas, promptToolSchemas(name)...)
		}
	}
	return schemas
}

// resourceToolSchemas returns the synthetic list_resources/read_resource tool
// pair for a server that declared the resources capability.
func resourceToolSchemas(server string) []tools.ToolSchema {
	return []tools.ToolSchema{
		{
			Type: "function",
			Function: tools.FunctionDef{
				Name:        namespacePrefix + server + "__" + listResourcesTool,
				Description: fmt.Sprintf("[%s] List available MCP resources (URI, name, description).", server),
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: tools.FunctionDef{
				Name:        namespacePrefix + server + "__" + readResourceTool,
				Description: fmt.Sprintf("[%s] Read an MCP resource's content by URI (see list_resources).", server),
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {"uri": {"type": "string", "description": "Resource URI from list_resources"}},
					"required": ["uri"]
				}`),
			},
		},
	}
}

// promptToolSchemas returns the synthetic list_prompts/get_prompt tool pair
// for a server that declared the prompts capability.
func promptToolSchemas(server string) []tools.ToolSchema {
	return []tools.ToolSchema{
		{
			Type: "function",
			Function: tools.FunctionDef{
				Name:        namespacePrefix + server + "__" + listPromptsTool,
				Description: fmt.Sprintf("[%s] List available MCP prompt templates (name, description, arguments).", server),
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: tools.FunctionDef{
				Name:        namespacePrefix + server + "__" + getPromptTool,
				Description: fmt.Sprintf("[%s] Render an MCP prompt template by name (see list_prompts).", server),
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"name": {"type": "string", "description": "Prompt name from list_prompts"},
						"arguments": {"type": "object", "description": "Named arguments the prompt template expects", "additionalProperties": {"type": "string"}}
					},
					"required": ["name"]
				}`),
			},
		},
	}
}

// formatResources renders a resource list as readable text for the LLM.
func formatResources(resources []Resource) string {
	if len(resources) == 0 {
		return "(no resources)"
	}
	lines := make([]string, len(resources))
	for i, r := range resources {
		lines[i] = fmt.Sprintf("- %s (%s): %s", r.URI, r.Name, r.Description)
	}
	return strings.Join(lines, "\n")
}

// formatPrompts renders a prompt list as readable text for the LLM.
func formatPrompts(prompts []Prompt) string {
	if len(prompts) == 0 {
		return "(no prompts)"
	}
	lines := make([]string, len(prompts))
	for i, p := range prompts {
		lines[i] = fmt.Sprintf("- %s: %s", p.Name, p.Description)
	}
	return strings.Join(lines, "\n")
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

	switch toolName {
	case listResourcesTool:
		return formatResources(c.Resources()), nil
	case readResourceTool:
		var args struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("parse args: %w", err)
		}
		return c.ReadResource(ctx, args.URI)
	case listPromptsTool:
		return formatPrompts(c.Prompts()), nil
	case getPromptTool:
		var args struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("parse args: %w", err)
		}
		return c.GetPrompt(ctx, args.Name, args.Arguments)
	}

	return c.Call(ctx, toolName, argsJSON)
}

// IsMCPTool reports whether the tool name belongs to a local MCP server.
func IsMCPTool(name string) bool {
	return strings.HasPrefix(name, namespacePrefix)
}

// IsReadOnlyMCPTool reports whether a qualified MCP tool name is one of the
// synthetic read-only tools (list_resources/read_resource/list_prompts/
// get_prompt) rather than an actual, unknown-risk MCP server tool — used by
// plan mode to decide which MCP calls are safe to allow.
func IsReadOnlyMCPTool(name string) bool {
	if !IsMCPTool(name) {
		return false
	}
	for _, suffix := range []string{listResourcesTool, readResourceTool, listPromptsTool, getPromptTool} {
		if strings.HasSuffix(name, "__"+suffix) {
			return true
		}
	}
	return false
}
