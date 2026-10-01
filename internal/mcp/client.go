// Package mcp implements a minimal MCP (Model Context Protocol) client over
// the stdio and Streamable HTTP transports. It speaks JSON-RPC 2.0 and
// supports initialize, tools/list, tools/call, and — when a server declares
// the capability during initialize — resources/list, resources/read,
// prompts/list, and prompts/get.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	protocolVersion     = "2024-11-05"
	initializeMethod    = "initialize"
	toolsListMethod     = "tools/list"
	toolsCallMethod     = "tools/call"
	resourcesListMethod = "resources/list"
	resourcesReadMethod = "resources/read"
	promptsListMethod   = "prompts/list"
	promptsGetMethod    = "prompts/get"
	startupTimeout      = 10 * time.Second
	callTimeout         = 60 * time.Second
)

// Tool describes a capability exposed by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Resource describes a piece of readable content an MCP server exposes.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// Prompt describes a reusable prompt template an MCP server exposes.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments"`
}

// PromptArgument describes one named input a Prompt accepts.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// serverCapabilities is the subset of the initialize response's capabilities
// object this client interprets. A non-nil field means the server declares
// that capability, per the MCP spec; this client does not otherwise inspect
// capability contents.
type serverCapabilities struct {
	Resources json.RawMessage `json:"resources"`
	Prompts   json.RawMessage `json:"prompts"`
}

// parseCapabilities extracts serverCapabilities from a raw initialize result.
// A malformed or missing capabilities object simply yields a zero value
// (no resources/prompts support assumed), matching the tolerant handling the
// existing tools/list parsing already applies to unexpected server responses.
func parseCapabilities(initResult json.RawMessage) serverCapabilities {
	var resp struct {
		Capabilities serverCapabilities `json:"capabilities"`
	}
	_ = json.Unmarshal(initResult, &resp)
	return resp.Capabilities
}

// Client manages one MCP server subprocess over stdio.
type Client struct {
	name      string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	scanner   *bufio.Scanner
	tools     []Tool
	resources []Resource
	prompts   []Prompt

	mu      sync.Mutex
	nextID  atomic.Int64
	pending map[int64]chan rpcResponse
}

// rpcRequest is the JSON-RPC 2.0 request envelope.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"` // nil for notifications
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcResponse is the JSON-RPC 2.0 response envelope.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Start launches the MCP server subprocess and performs the initialize handshake.
func Start(ctx context.Context, name, command string, args []string, env map[string]string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	// Inject env overrides on top of the current environment.
	cmd.Env = os.Environ()
	for k, v := range env {
		// Expand ${VAR} references from the current environment.
		expanded := os.ExpandEnv(v)
		cmd.Env = append(cmd.Env, k+"="+expanded)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp %s: stdin pipe: %w", name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp %s: stdout pipe: %w", name, err)
	}
	cmd.Stderr = os.Stderr // forward server stderr for debugging

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %s: start %q: %w", name, command, err)
	}

	c := &Client{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		scanner: bufio.NewScanner(stdout),
		pending: make(map[int64]chan rpcResponse),
	}
	c.scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	// Read loop: route responses to pending callers.
	go c.readLoop()

	// Initialize handshake.
	initCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	initResult, err := c.call(initCtx, initializeMethod, map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "bai", "version": "1.0"},
	})
	if err != nil {
		c.Stop()
		return nil, fmt.Errorf("mcp %s: initialize: %w", name, err)
	}
	caps := parseCapabilities(initResult)

	// Send initialized notification (no response expected).
	if err := c.notify("notifications/initialized", nil); err != nil {
		c.Stop()
		return nil, fmt.Errorf("mcp %s: initialized notification: %w", name, err)
	}

	// List available tools.
	listResult, err := c.call(initCtx, toolsListMethod, map[string]any{})
	if err != nil {
		c.Stop()
		return nil, fmt.Errorf("mcp %s: tools/list: %w", name, err)
	}

	var toolsResp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(listResult, &toolsResp); err != nil {
		c.Stop()
		return nil, fmt.Errorf("mcp %s: parse tools/list: %w", name, err)
	}
	c.tools = toolsResp.Tools

	if caps.Resources != nil {
		raw, err := c.call(initCtx, resourcesListMethod, map[string]any{})
		if err != nil {
			c.Stop()
			return nil, fmt.Errorf("mcp %s: resources/list: %w", name, err)
		}
		resources, err := parseResourcesListResult(raw)
		if err != nil {
			c.Stop()
			return nil, fmt.Errorf("mcp %s: parse resources/list: %w", name, err)
		}
		c.resources = resources
	}

	if caps.Prompts != nil {
		raw, err := c.call(initCtx, promptsListMethod, map[string]any{})
		if err != nil {
			c.Stop()
			return nil, fmt.Errorf("mcp %s: prompts/list: %w", name, err)
		}
		prompts, err := parsePromptsListResult(raw)
		if err != nil {
			c.Stop()
			return nil, fmt.Errorf("mcp %s: parse prompts/list: %w", name, err)
		}
		c.prompts = prompts
	}

	return c, nil
}

// Tools returns the tools this server exposes.
func (c *Client) Tools() []Tool {
	return c.tools
}

// Resources returns the resources this server exposes (empty if it did not
// declare the resources capability during initialize).
func (c *Client) Resources() []Resource {
	return c.resources
}

// Prompts returns the prompts this server exposes (empty if it did not
// declare the prompts capability during initialize).
func (c *Client) Prompts() []Prompt {
	return c.prompts
}

// ReadResource fetches a resource's content by URI.
func (c *Client) ReadResource(ctx context.Context, uri string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	raw, err := c.call(callCtx, resourcesReadMethod, map[string]any{"uri": uri})
	if err != nil {
		return "", err
	}
	return parseResourceReadResult(raw)
}

// GetPrompt renders a prompt template by name with the given arguments.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	raw, err := c.call(callCtx, promptsGetMethod, map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", err
	}
	return parsePromptGetResult(raw)
}

// Call invokes a tool by name with the given JSON arguments and returns the
// text content of the result.
func (c *Client) Call(ctx context.Context, toolName, argsJSON string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	raw, err := c.call(callCtx, toolsCallMethod, map[string]any{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	return parseToolCallResult(raw)
}

// parseToolCallResult decodes a tools/call JSON-RPC result — shared between
// the stdio and HTTP transports, since the result shape is transport-agnostic —
// into the joined text content, surfacing isError as a Go error.
func parseToolCallResult(raw json.RawMessage) (string, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse result: %w", err)
	}
	if result.IsError {
		texts := make([]string, 0, len(result.Content))
		for _, c := range result.Content {
			if c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		return "", fmt.Errorf("%s", strings.Join(texts, "\n"))
	}

	var parts []string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

// parseResourcesListResult decodes a resources/list JSON-RPC result.
func parseResourcesListResult(raw json.RawMessage) ([]Resource, error) {
	var resp struct {
		Resources []Resource `json:"resources"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}
	return resp.Resources, nil
}

// parsePromptsListResult decodes a prompts/list JSON-RPC result.
func parsePromptsListResult(raw json.RawMessage) ([]Prompt, error) {
	var resp struct {
		Prompts []Prompt `json:"prompts"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}
	return resp.Prompts, nil
}

// parseResourceReadResult decodes a resources/read JSON-RPC result into
// joined text content. A content entry carrying only a base64 blob (no text)
// becomes a one-line placeholder rather than being decoded into the LLM's
// context — this client has no use for arbitrary binary resource content.
func parseResourceReadResult(raw json.RawMessage) (string, error) {
	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
			Blob     string `json:"blob"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse result: %w", err)
	}
	var parts []string
	for _, c := range result.Contents {
		switch {
		case c.Text != "":
			parts = append(parts, c.Text)
		case c.Blob != "":
			parts = append(parts, fmt.Sprintf("[binary resource: %s, base64, %d bytes]", c.MimeType, len(c.Blob)))
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// parsePromptGetResult decodes a prompts/get JSON-RPC result into a joined,
// role-prefixed transcript of its messages. Non-text content parts (e.g.
// embedded images) are skipped, matching parseToolCallResult's convention.
func parsePromptGetResult(raw json.RawMessage) (string, error) {
	var result struct {
		Description string `json:"description"`
		Messages    []struct {
			Role    string `json:"role"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse result: %w", err)
	}
	var parts []string
	for _, m := range result.Messages {
		if m.Content.Type == "text" && m.Content.Text != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", m.Role, m.Content.Text))
		}
	}
	return strings.Join(parts, "\n"), nil
}

// Stop terminates the MCP server subprocess.
func (c *Client) Stop() {
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill() //nolint:errcheck
	}
}

// call sends a JSON-RPC request and waits for the response.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	ch := make(chan rpcResponse, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params}
	if err := c.send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc %s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// notify sends a JSON-RPC notification (no ID, no response expected).
func (c *Client) notify(method string, params any) error {
	return c.send(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
}

// send marshals and writes a request to the server's stdin.
func (c *Client) send(req rpcRequest) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", b)
	return err
}

// readLoop reads newline-delimited JSON from the server's stdout and routes
// responses to waiting callers.
func (c *Client) readLoop() {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.ID == nil {
			continue // notification — ignore for now
		}
		c.mu.Lock()
		ch, ok := c.pending[*resp.ID]
		if ok {
			delete(c.pending, *resp.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
	// Server exited: drain all pending callers with an error.
	c.mu.Lock()
	for id, ch := range c.pending {
		ch <- rpcResponse{Error: &rpcError{Message: "MCP server exited"}}
		delete(c.pending, id)
	}
	c.mu.Unlock()
}
