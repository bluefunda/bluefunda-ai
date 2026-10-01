package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

// mcpSessionHeader is the MCP Streamable HTTP session header name. The server
// issues a session id on the initialize response; the client echoes it back
// on every subsequent request for that session.
const mcpSessionHeader = "Mcp-Session-Id"

// httpClient manages one MCP server reached over the Streamable HTTP
// transport (a single URL; each JSON-RPC call is one POST, whose response is
// either a plain JSON body or a text/event-stream carrying one JSON-RPC event).
type httpClient struct {
	name    string
	url     string
	headers map[string]string
	hc      *http.Client
	tools   []Tool

	nextID    atomic.Int64
	sessionID atomic.Value // string
}

// StartHTTP connects to an MCP server over Streamable HTTP and performs the
// same initialize -> notifications/initialized -> tools/list handshake as
// Start does for stdio.
func StartHTTP(ctx context.Context, name, url string, headers map[string]string) (*httpClient, error) {
	c := &httpClient{
		name:    name,
		url:     url,
		headers: headers,
		hc:      &http.Client{},
	}
	c.sessionID.Store("")

	initCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	initResult, err := c.call(initCtx, initializeMethod, map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "bai", "version": "1.0"},
	})
	if err != nil {
		return nil, fmt.Errorf("mcp %s: initialize: %w", name, err)
	}
	_ = initResult

	if err := c.notify(initCtx, "notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("mcp %s: initialized notification: %w", name, err)
	}

	listResult, err := c.call(initCtx, toolsListMethod, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("mcp %s: tools/list: %w", name, err)
	}
	var toolsResp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(listResult, &toolsResp); err != nil {
		return nil, fmt.Errorf("mcp %s: parse tools/list: %w", name, err)
	}
	c.tools = toolsResp.Tools

	return c, nil
}

// Tools returns the tools this server exposes.
func (c *httpClient) Tools() []Tool {
	return c.tools
}

// Call invokes a tool by name with the given JSON arguments and returns the
// text content of the result.
func (c *httpClient) Call(ctx context.Context, toolName, argsJSON string) (string, error) {
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

// Stop ends the MCP session. Best-effort: per the MCP spec a client may send
// DELETE with the session header to let the server free session state; a
// failure here is not actionable, so it is ignored (mirrors stdio Client.Stop
// ignoring Process.Kill errors).
func (c *httpClient) Stop() {
	sid, _ := c.sessionID.Load().(string)
	if sid == "" {
		return
	}
	req, err := http.NewRequest(http.MethodDelete, c.url, nil)
	if err != nil {
		return
	}
	req.Header.Set(mcpSessionHeader, sid)
	resp, err := c.hc.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// call sends a JSON-RPC request over HTTP and returns the decoded result.
func (c *httpClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	resp, err := c.roundTrip(ctx, rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("rpc %s: no response", method)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc %s: %s", method, resp.Error.Message)
	}
	return resp.Result, nil
}

// notify sends a JSON-RPC notification (no ID, no response expected).
func (c *httpClient) notify(ctx context.Context, method string, params any) error {
	_, err := c.roundTrip(ctx, rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	return err
}

// roundTrip POSTs one JSON-RPC envelope and returns the matching response, or
// nil if none was expected/returned (e.g. a notification accepted with 202).
func (c *httpClient) roundTrip(ctx context.Context, req rpcRequest) (*rpcResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("mcp %s: build request: %w", c.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	if sid, _ := c.sessionID.Load().(string); sid != "" {
		httpReq.Header.Set(mcpSessionHeader, sid)
	}

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp %s: http post: %w", c.name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if sid := resp.Header.Get(mcpSessionHeader); sid != "" {
		c.sessionID.Store(sid)
	}

	if resp.StatusCode == http.StatusAccepted {
		return nil, nil // notification acknowledged, no body to parse
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("mcp %s: http %d: %s", c.name, resp.StatusCode, strings.TrimSpace(string(b)))
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/event-stream") {
		return parseSSEResponse(c.name, resp.Body, req.ID)
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		if err == io.EOF {
			return nil, nil // empty body — treat like an accepted notification
		}
		return nil, fmt.Errorf("mcp %s: decode response: %w", c.name, err)
	}
	return &rpcResp, nil
}

// parseSSEResponse scans a text/event-stream body for the JSON-RPC response
// whose ID matches wantID, ignoring unrelated server-initiated events. wantID
// nil (a notification) matches the first parseable JSON-RPC event.
func parseSSEResponse(name string, body io.Reader, wantID *int64) (*rpcResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	var dataLines []string
	tryMatch := func() (*rpcResponse, bool) {
		if len(dataLines) == 0 {
			return nil, false
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil
		var resp rpcResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return nil, false // not a JSON-RPC event — skip it
		}
		if wantID == nil || (resp.ID != nil && *resp.ID == *wantID) {
			return &resp, true
		}
		return nil, false
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case line == "":
			if resp, matched := tryMatch(); matched {
				return resp, nil
			}
		}
	}
	if resp, matched := tryMatch(); matched {
		return resp, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("mcp %s: sse scan: %w", name, err)
	}
	return nil, fmt.Errorf("mcp %s: sse stream ended without a matching response", name)
}
