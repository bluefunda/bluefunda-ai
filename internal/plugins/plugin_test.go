package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePlugin creates a plugin.yaml at dir/<name>/plugin.yaml.
func writePlugin(t *testing.T, dir, name, content string) string {
	t.Helper()
	pluginDir := filepath.Join(dir, name)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(pluginDir, "plugin.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write plugin.yaml: %v", err)
	}
	return path
}

func TestLoadFile_Valid(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "hello", `
name: hello
description: say hello
executor:
  type: cli
  command: ["echo", "{{.message}}"]
approval: auto
`)
	p, err := loadFile(filepath.Join(dir, "hello", "plugin.yaml"))
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	if p.ToolName != "plugin__hello" {
		t.Errorf("ToolName = %q, want plugin__hello", p.ToolName)
	}
	if p.Manifest.Description != "say hello" {
		t.Errorf("Description = %q", p.Manifest.Description)
	}
}

func TestLoadFile_MissingName(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "noname", `
description: no name
executor:
  type: cli
  command: ["echo"]
`)
	_, err := loadFile(filepath.Join(dir, "noname", "plugin.yaml"))
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestLoadFile_UnsupportedType(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "bad", `
name: bad
executor:
  type: grpc
  command: []
`)
	_, err := loadFile(filepath.Join(dir, "bad", "plugin.yaml"))
	if err == nil {
		t.Error("expected error for unsupported executor type")
	}
}

func TestLoadFile_HTTPValid(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "httptest", `
name: httptest
description: call an api
executor:
  type: http
  url: "https://api.example.com/run"
  method: POST
`)
	p, err := loadFile(filepath.Join(dir, "httptest", "plugin.yaml"))
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	if p.Manifest.Executor.Type != "http" {
		t.Errorf("executor type = %q, want http", p.Manifest.Executor.Type)
	}
}

func TestLoadFile_HTTPMissingURL(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "nourl", `
name: nourl
executor:
  type: http
`)
	_, err := loadFile(filepath.Join(dir, "nourl", "plugin.yaml"))
	if err == nil {
		t.Error("expected error for http type with no url")
	}
}

func TestLoadFile_EmptyCommand(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "empty", `
name: empty
executor:
  type: cli
  command: []
`)
	_, err := loadFile(filepath.Join(dir, "empty", "plugin.yaml"))
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestIsPluginTool(t *testing.T) {
	if !IsPluginTool("plugin__hello") {
		t.Error("plugin__hello should be a plugin tool")
	}
	if IsPluginTool("bash") {
		t.Error("bash should not be a plugin tool")
	}
	if IsPluginTool("mcp__server__tool") {
		t.Error("mcp tool should not be a plugin tool")
	}
}

func TestManager_ToolSchemas(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "greet", `
name: greet
description: greet someone
input_schema:
  type: object
  properties:
    name:
      type: string
      description: name to greet
  required: [name]
executor:
  type: cli
  command: ["echo", "Hello {{.name}}"]
`)
	m := &Manager{byTool: make(map[string]*Plugin)}
	for _, p := range loadDir(dir) {
		m.plugins = append(m.plugins, p)
		m.byTool[p.ToolName] = p
	}

	schemas := m.ToolSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0].Function.Name != "plugin__greet" {
		t.Errorf("schema name = %q", schemas[0].Function.Name)
	}
	if schemas[0].Function.Description != "greet someone" {
		t.Errorf("schema description = %q", schemas[0].Function.Description)
	}
}

func TestManager_ApprovalMode(t *testing.T) {
	dir := t.TempDir()
	// NewManager looks in <cwd>/.bai/plugins/ — write there.
	baiPluginsDir := filepath.Join(dir, ".bai", "plugins")
	for _, tc := range []struct{ name, approval string }{
		{"always_tool", "always"},
		{"never_tool", "never"},
		{"auto_tool", "auto"},
	} {
		writePlugin(t, baiPluginsDir, tc.name, `
name: `+tc.name+`
executor:
  type: cli
  command: ["echo"]
approval: `+tc.approval+`
`)
	}
	m := NewManager(dir)
	for _, tc := range []struct{ toolName, want string }{
		{"plugin__always_tool", "always"},
		{"plugin__never_tool", "never"},
		{"plugin__auto_tool", "auto"},
		{"plugin__nonexistent", "auto"},
	} {
		got := m.ApprovalMode(tc.toolName)
		if got != tc.want {
			t.Errorf("ApprovalMode(%q) = %q, want %q", tc.toolName, got, tc.want)
		}
	}
}

func TestExecutePlugin_Echo(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "echo", `
name: echo
executor:
  type: cli
  command: ["echo", "hello {{.name}}"]
`)
	p, err := loadFile(filepath.Join(dir, "echo", "plugin.yaml"))
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	args, _ := json.Marshal(map[string]string{"name": "world"})
	out, err := executePlugin(context.Background(), p, string(args))
	if err != nil {
		t.Fatalf("executePlugin: %v", err)
	}
	if !strings.Contains(out, "world") {
		t.Errorf("output = %q, want 'world'", out)
	}
}

func TestExecutePlugin_MissingRequired(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "req", `
name: req
input_schema:
  type: object
  required: [name]
executor:
  type: cli
  command: ["echo", "{{.name}}"]
`)
	p, _ := loadFile(filepath.Join(dir, "req", "plugin.yaml"))
	_, err := executePlugin(context.Background(), p, `{}`)
	if err == nil {
		t.Error("expected error for missing required field")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention missing field name: %v", err)
	}
}

func TestExecutePlugin_EnvTemplate(t *testing.T) {
	t.Setenv("PLUGIN_TEST_VAR", "test_value")
	dir := t.TempDir()
	writePlugin(t, dir, "envtest", `
name: envtest
executor:
  type: cli
  command: ["echo", "{{.env.PLUGIN_TEST_VAR}}"]
`)
	p, _ := loadFile(filepath.Join(dir, "envtest", "plugin.yaml"))
	out, err := executePlugin(context.Background(), p, `{}`)
	if err != nil {
		t.Fatalf("executePlugin: %v", err)
	}
	if !strings.Contains(out, "test_value") {
		t.Errorf("output = %q, expected test_value", out)
	}
}

func TestExecutePlugin_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "fail", `
name: fail
executor:
  type: cli
  command: ["sh", "-c", "echo error output; exit 1"]
`)
	p, _ := loadFile(filepath.Join(dir, "fail", "plugin.yaml"))
	out, err := executePlugin(context.Background(), p, `{}`)
	// Non-zero exit should return output without error.
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "error output") {
		t.Errorf("output = %q, expected 'error output'", out)
	}
}

func TestExecutePlugin_HTTPPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			http.Error(w, "wrong content-type", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"result":"ok"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "httppost", `
name: httppost
executor:
  type: http
  url: "`+srv.URL+`/run"
  method: POST
input_schema:
  type: object
  properties:
    code: {type: string}
  required: [code]
`)
	p, err := loadFile(filepath.Join(dir, "httppost", "plugin.yaml"))
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	args, _ := json.Marshal(map[string]string{"code": "print('hello')"})
	out, err := executePlugin(context.Background(), p, string(args))
	if err != nil {
		t.Fatalf("executePlugin: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("output = %q, want json with ok", out)
	}
}

func TestExecutePlugin_HTTPGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query().Get("q")
		w.Write([]byte("got:" + q)) //nolint:errcheck
	}))
	defer srv.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "httpget", `
name: httpget
executor:
  type: http
  url: "`+srv.URL+`/search"
  method: GET
`)
	p, err := loadFile(filepath.Join(dir, "httpget", "plugin.yaml"))
	if err != nil {
		t.Fatalf("loadFile: %v", err)
	}
	args, _ := json.Marshal(map[string]string{"q": "abap"})
	out, err := executePlugin(context.Background(), p, string(args))
	if err != nil {
		t.Fatalf("executePlugin: %v", err)
	}
	if !strings.Contains(out, "abap") {
		t.Errorf("output = %q, expected query param echoed back", out)
	}
}

func TestExecutePlugin_HTTPNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	writePlugin(t, dir, "http404", `
name: http404
executor:
  type: http
  url: "`+srv.URL+`/missing"
`)
	p, _ := loadFile(filepath.Join(dir, "http404", "plugin.yaml"))
	out, err := executePlugin(context.Background(), p, `{}`)
	if err != nil {
		t.Fatalf("unexpected Go error (non-2xx should return content, not error): %v", err)
	}
	if !strings.Contains(out, "404") {
		t.Errorf("output = %q, expected 404 status in output", out)
	}
}

func TestExecutePlugin_HTTPHeaderTemplate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("X-Token")
		w.Write([]byte("token:" + tok)) //nolint:errcheck
	}))
	defer srv.Close()

	t.Setenv("MY_API_TOKEN", "secret123")
	dir := t.TempDir()
	writePlugin(t, dir, "httpheader", `
name: httpheader
executor:
  type: http
  url: "`+srv.URL+`/check"
  headers:
    X-Token: "{{.env.MY_API_TOKEN}}"
`)
	p, _ := loadFile(filepath.Join(dir, "httpheader", "plugin.yaml"))
	out, err := executePlugin(context.Background(), p, `{}`)
	if err != nil {
		t.Fatalf("executePlugin: %v", err)
	}
	if !strings.Contains(out, "secret123") {
		t.Errorf("output = %q, expected header value in response", out)
	}
}

func TestDeduplicate(t *testing.T) {
	a := &Plugin{ToolName: "plugin__x", SourcePath: "/user/.bai/plugins/x/plugin.yaml"}
	b := &Plugin{ToolName: "plugin__x", SourcePath: "/project/.bai/plugins/x/plugin.yaml"}
	c := &Plugin{ToolName: "plugin__y", SourcePath: "/project/.bai/plugins/y/plugin.yaml"}

	result := deduplicate([]*Plugin{a, b, c})
	if len(result) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(result))
	}
	// Last occurrence (project-level) should win.
	found := false
	for _, p := range result {
		if p.ToolName == "plugin__x" && p.SourcePath == b.SourcePath {
			found = true
		}
	}
	if !found {
		t.Error("project-level plugin should override user-level")
	}
}
