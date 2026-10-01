package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeMCPServer is a minimal MCP Streamable HTTP server for tests. It
// understands initialize, notifications/initialized, tools/list, and
// tools/call, replying either as plain JSON or as a one-event SSE stream
// depending on sse.
type fakeMCPServer struct {
	sse          bool
	sessionID    string
	requestCount atomic.Int64
	sawSession   atomic.Bool // true once a non-initialize request carried the session header
}

func (f *fakeMCPServer) handler(w http.ResponseWriter, r *http.Request) {
	f.requestCount.Add(1)

	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Method != initializeMethod && r.Header.Get(mcpSessionHeader) == f.sessionID && f.sessionID != "" {
		f.sawSession.Store(true)
	}

	if req.ID == nil {
		// Notification (e.g. notifications/initialized): accept, no body.
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var result any
	switch req.Method {
	case initializeMethod:
		if f.sessionID != "" {
			w.Header().Set(mcpSessionHeader, f.sessionID)
		}
		result = map[string]any{"protocolVersion": protocolVersion}
	case toolsListMethod:
		result = map[string]any{"tools": []Tool{{Name: "echo", Description: "echoes input"}}}
	case toolsCallMethod:
		var params struct {
			Name      string `json:"name"`
			Arguments struct {
				Name string `json:"name"`
			} `json:"arguments"`
		}
		_ = json.Unmarshal(mustMarshal(req.Params), &params)
		if params.Name == "boom" {
			result = map[string]any{
				"isError": true,
				"content": []map[string]any{{"type": "text", "text": "it broke"}},
			}
		} else {
			result = map[string]any{
				"content": []map[string]any{{"type": "text", "text": "echoed: " + params.Arguments.Name}},
			}
		}
	default:
		http.Error(w, "unknown method", http.StatusNotImplemented)
		return
	}

	resultJSON, _ := json.Marshal(result)
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	respBytes, _ := json.Marshal(resp)

	if f.sse {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", respBytes)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestStartHTTP_JSONHandshakeAndCall(t *testing.T) {
	fake := &fakeMCPServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	c, err := StartHTTP(context.Background(), "test", srv.URL, nil)
	if err != nil {
		t.Fatalf("StartHTTP: %v", err)
	}
	defer c.Stop()

	tools := c.Tools()
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("expected one 'echo' tool, got %+v", tools)
	}

	out, err := c.Call(context.Background(), "echo", `{"name":"hello"}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != "echoed: hello" {
		t.Errorf("expected 'echoed: hello', got %q", out)
	}
}

func TestStartHTTP_SSEHandshakeAndCall(t *testing.T) {
	fake := &fakeMCPServer{sse: true}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	c, err := StartHTTP(context.Background(), "test", srv.URL, nil)
	if err != nil {
		t.Fatalf("StartHTTP: %v", err)
	}
	defer c.Stop()

	out, err := c.Call(context.Background(), "echo", `{"name":"sse-world"}`)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != "echoed: sse-world" {
		t.Errorf("expected 'echoed: sse-world', got %q", out)
	}
}

func TestStartHTTP_SessionIDRoundTrip(t *testing.T) {
	fake := &fakeMCPServer{sessionID: "sess-123"}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	c, err := StartHTTP(context.Background(), "test", srv.URL, nil)
	if err != nil {
		t.Fatalf("StartHTTP: %v", err)
	}
	defer c.Stop()

	if _, err := c.Call(context.Background(), "echo", `{"name":"x"}`); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !fake.sawSession.Load() {
		t.Error("expected the client to echo back the Mcp-Session-Id header on subsequent requests")
	}
}

func TestStartHTTP_ToolErrorSurfacesAsGoError(t *testing.T) {
	fake := &fakeMCPServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	c, err := StartHTTP(context.Background(), "test", srv.URL, nil)
	if err != nil {
		t.Fatalf("StartHTTP: %v", err)
	}
	defer c.Stop()

	_, err = c.Call(context.Background(), "boom", `{"name":"boom"}`)
	if err == nil {
		t.Fatal("expected an error for isError:true result")
	}
	if !strings.Contains(err.Error(), "it broke") {
		t.Errorf("expected error to contain tool's message, got: %v", err)
	}
}

func TestStartHTTP_NonSuccessStatusIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := StartHTTP(context.Background(), "test", srv.URL, nil)
	if err == nil {
		t.Fatal("expected an error for a 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to mention status 403, got: %v", err)
	}
}

func TestStartHTTP_CustomHeadersSent(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		(&fakeMCPServer{}).handler(w, r)
	}))
	defer srv.Close()

	_, err := StartHTTP(context.Background(), "test", srv.URL, map[string]string{"Authorization": "Bearer xyz"})
	if err != nil {
		t.Fatalf("StartHTTP: %v", err)
	}
	if gotAuth != "Bearer xyz" {
		t.Errorf("expected Authorization header to be forwarded, got %q", gotAuth)
	}
}
