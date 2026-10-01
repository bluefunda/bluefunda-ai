package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/config"
)

func TestNewManager_NilConfig(t *testing.T) {
	m := NewManager(context.Background(), nil)
	if len(m.clients) != 0 {
		t.Errorf("expected no clients for nil config, got %d", len(m.clients))
	}
}

func TestNewManager_HTTPServerByExplicitType(t *testing.T) {
	fake := &fakeMCPServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"remote": {Type: "http", URL: srv.URL},
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	if _, ok := m.clients["remote"]; !ok {
		t.Fatal("expected 'remote' client to be started over HTTP")
	}
	schemas := m.ToolSchemas()
	if len(schemas) != 1 || schemas[0].Function.Name != "mcp__remote__echo" {
		t.Errorf("unexpected tool schemas: %+v", schemas)
	}
}

func TestNewManager_HTTPServerByBareURL(t *testing.T) {
	fake := &fakeMCPServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"remote": {URL: srv.URL}, // no explicit type — inferred as http
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	if _, ok := m.clients["remote"]; !ok {
		t.Fatal("expected 'remote' client to be started over HTTP when only a URL is given")
	}
}

func TestNewManager_SkipsHTTPEntryMissingURL(t *testing.T) {
	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"broken": {Type: "http"},
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	if _, ok := m.clients["broken"]; ok {
		t.Error("expected 'broken' (http type, no url) to be skipped")
	}
}

func TestNewManager_SkipsStdioEntryMissingCommand(t *testing.T) {
	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"broken": {},
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	if _, ok := m.clients["broken"]; ok {
		t.Error("expected 'broken' (no command, no url) to be skipped")
	}
}

func TestExecute_RoutesToHTTPClient(t *testing.T) {
	fake := &fakeMCPServer{}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"remote": {Type: "http", URL: srv.URL},
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	out, err := m.Execute(context.Background(), "mcp__remote__echo", `{"name":"world"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "echoed: world" {
		t.Errorf("expected 'echoed: world', got %q", out)
	}
}
