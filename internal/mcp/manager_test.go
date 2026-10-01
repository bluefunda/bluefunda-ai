package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestToolSchemas_ExcludesResourceAndPromptToolsWhenNotCapable(t *testing.T) {
	fake := &fakeMCPServer{} // no resources/prompts capability
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"remote": {Type: "http", URL: srv.URL},
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	schemas := m.ToolSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected only the 'echo' tool schema, got %d: %+v", len(schemas), schemas)
	}
}

func TestToolSchemas_IncludesResourceAndPromptToolsWhenCapable(t *testing.T) {
	fake := &fakeMCPServer{resourcesCap: true, promptsCap: true}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"remote": {Type: "http", URL: srv.URL},
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	names := make(map[string]bool)
	for _, s := range m.ToolSchemas() {
		names[s.Function.Name] = true
	}
	for _, want := range []string{
		"mcp__remote__echo",
		"mcp__remote__list_resources",
		"mcp__remote__read_resource",
		"mcp__remote__list_prompts",
		"mcp__remote__get_prompt",
	} {
		if !names[want] {
			t.Errorf("expected tool schema %q, got %+v", want, names)
		}
	}
}

func TestExecute_RoutesResourceAndPromptCalls(t *testing.T) {
	fake := &fakeMCPServer{resourcesCap: true, promptsCap: true}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"remote": {Type: "http", URL: srv.URL},
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	if out, err := m.Execute(context.Background(), "mcp__remote__list_resources", `{}`); err != nil {
		t.Fatalf("list_resources: %v", err)
	} else if !strings.Contains(out, "file:///readme.md") {
		t.Errorf("expected list_resources output to mention the readme, got %q", out)
	}

	if out, err := m.Execute(context.Background(), "mcp__remote__read_resource", `{"uri":"file:///readme.md"}`); err != nil {
		t.Fatalf("read_resource: %v", err)
	} else if out != "resource content for file:///readme.md" {
		t.Errorf("unexpected read_resource output: %q", out)
	}

	if out, err := m.Execute(context.Background(), "mcp__remote__list_prompts", `{}`); err != nil {
		t.Fatalf("list_prompts: %v", err)
	} else if !strings.Contains(out, "greet") {
		t.Errorf("expected list_prompts output to mention 'greet', got %q", out)
	}

	if out, err := m.Execute(context.Background(), "mcp__remote__get_prompt", `{"name":"greet","arguments":{"who":"bai"}}`); err != nil {
		t.Fatalf("get_prompt: %v", err)
	} else if out != "user: Hello, bai!" {
		t.Errorf("unexpected get_prompt output: %q", out)
	}
}

func TestStatus_ReportsStartedAndFailedServers(t *testing.T) {
	fake := &fakeMCPServer{resourcesCap: true, promptsCap: true}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()

	cfg := &config.ProjectConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"good":   {Type: "http", URL: srv.URL},
			"broken": {Type: "http"}, // no URL — fails to start
		},
	}
	m := NewManager(context.Background(), cfg)
	defer m.Close()

	byName := make(map[string]ServerStatus)
	for _, s := range m.Status() {
		byName[s.Name] = s
	}
	if len(byName) != 2 {
		t.Fatalf("expected 2 statuses, got %d: %+v", len(byName), byName)
	}

	good := byName["good"]
	if !good.Started {
		t.Errorf("expected 'good' to have started, got %+v", good)
	}
	if good.Tools != 1 || good.Resources != 1 || good.Prompts != 1 {
		t.Errorf("expected 1 tool/resource/prompt for 'good', got %+v", good)
	}
	if good.Err != "" {
		t.Errorf("expected no error for 'good', got %q", good.Err)
	}

	broken := byName["broken"]
	if broken.Started {
		t.Error("expected 'broken' to have failed to start")
	}
	if broken.Err == "" {
		t.Error("expected a non-empty error for 'broken'")
	}
}
