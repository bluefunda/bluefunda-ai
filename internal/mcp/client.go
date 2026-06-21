// Package mcp implements a minimal MCP (Model Context Protocol) client using
// the stdio transport. It speaks JSON-RPC 2.0 and supports the initialize,
// tools/list, and tools/call methods sufficient for bai code tool integration.
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
	protocolVersion  = "2024-11-05"
	initializeMethod = "initialize"
	toolsListMethod  = "tools/list"
	toolsCallMethod  = "tools/call"
	startupTimeout   = 10 * time.Second
	callTimeout      = 60 * time.Second
)

// Tool describes a capability exposed by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Client manages one MCP server subprocess over stdio.
type Client struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	tools   []Tool

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
	_ = initResult

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

	return c, nil
}

// Tools returns the tools this server exposes.
func (c *Client) Tools() []Tool {
	return c.tools
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

// Stop terminates the MCP server subprocess.
func (c *Client) Stop() {
	c.stdin.Close()
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
