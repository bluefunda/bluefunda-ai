// Package plugins loads and executes CLI-based tool plugins defined in
// .bai/plugins/<name>/plugin.yaml (project-level) and
// ~/.bai/plugins/<name>/plugin.yaml (user-level).
//
// Plugin tools are namespaced as plugin__<name> in the LLM tool schema.
package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

const (
	namePrefix     = "plugin__"
	defaultTimeout = 60 * time.Second
)

// Manifest is the parsed content of a plugin.yaml file.
type Manifest struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	InputSchema json.RawMessage `yaml:"input_schema"`
	Executor    ExecutorConfig  `yaml:"executor"`
	// Approval controls whether the TUI approval dialog is shown.
	// "always" — force dialog even with --auto.
	// "never"  — auto-approve without dialog.
	// "auto"   — (default) behave like bash: require dialog unless --auto is set.
	Approval string `yaml:"approval"`
}

// ExecutorConfig describes how to run the plugin.
type ExecutorConfig struct {
	Type    string            `yaml:"type"`    // only "cli" is supported
	Command []string          `yaml:"command"` // each element is a Go template string
	Timeout string            `yaml:"timeout"` // e.g. "60s"; defaults to 60s
	Env     map[string]string `yaml:"env"`     // extra env vars; values are Go template strings
}

// Plugin is a loaded, validated plugin ready for execution.
type Plugin struct {
	Manifest Manifest
	// ToolName is the namespaced name sent to the LLM (plugin__<manifest.Name>).
	ToolName string
	// SourcePath is the plugin.yaml file that defined this plugin.
	SourcePath string
}

// IsPluginTool returns true when the tool name has the plugin__ prefix.
func IsPluginTool(name string) bool {
	return strings.HasPrefix(name, namePrefix)
}

// ── Manager ───────────────────────────────────────────────────────────────────

// Manager holds all loaded plugins for a session.
type Manager struct {
	plugins []*Plugin
	byTool  map[string]*Plugin // keyed by ToolName
}

// NewManager loads plugins from user-level and project-level directories.
// Malformed plugin.yaml files are logged to stderr and skipped.
func NewManager(cwd string) *Manager {
	m := &Manager{byTool: make(map[string]*Plugin)}
	for _, p := range loadAll(cwd) {
		m.plugins = append(m.plugins, p)
		m.byTool[p.ToolName] = p
	}
	return m
}

// Get returns the plugin for the given tool name, or nil if not found.
func (m *Manager) Get(toolName string) *Plugin {
	return m.byTool[toolName]
}

// All returns all loaded plugins.
func (m *Manager) All() []*Plugin {
	return m.plugins
}

// ToolSchemas returns the JSON-encoded tool schemas for all loaded plugins.
func (m *Manager) ToolSchemas() []tools.ToolSchema {
	var schemas []tools.ToolSchema
	for _, p := range m.plugins {
		schema := p.Manifest.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		schemas = append(schemas, tools.ToolSchema{
			Type: "function",
			Function: tools.FunctionDef{
				Name:        p.ToolName,
				Description: p.Manifest.Description,
				Parameters:  schema,
			},
		})
	}
	return schemas
}

// ApprovalMode returns the approval policy for the plugin ("always", "never", "auto").
func (m *Manager) ApprovalMode(toolName string) string {
	p := m.byTool[toolName]
	if p == nil {
		return "auto"
	}
	switch p.Manifest.Approval {
	case "always", "never":
		return p.Manifest.Approval
	default:
		return "auto"
	}
}

// Execute runs the plugin and returns the combined stdout+stderr output.
func (m *Manager) Execute(ctx context.Context, toolName, argumentsJSON string) (string, error) {
	p := m.byTool[toolName]
	if p == nil {
		return "", fmt.Errorf("plugin %q not found", toolName)
	}
	return executePlugin(ctx, p, argumentsJSON)
}

// ── Loader ────────────────────────────────────────────────────────────────────

func loadAll(cwd string) []*Plugin {
	var all []*Plugin

	// User-level plugins.
	if home, err := os.UserHomeDir(); err == nil {
		all = append(all, loadDir(filepath.Join(home, ".bai", "plugins"))...)
	}

	// Project-level plugins: walk upward to git root.
	abs, _ := filepath.Abs(cwd)
	for {
		all = append(all, loadDir(filepath.Join(abs, ".bai", "plugins"))...)
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}

	return deduplicate(all)
}

// loadDir reads all <dir>/*/plugin.yaml files.
func loadDir(dir string) []*Plugin {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var plugins []*Plugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "plugin.yaml")
		p, err := loadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[bai] plugin %s: %v — skipping\n", path, err)
			continue
		}
		plugins = append(plugins, p)
	}
	return plugins
}

// manifestYAML is the YAML-parseable form of Manifest. input_schema is parsed
// as interface{} because json.RawMessage cannot directly unmarshal YAML maps.
type manifestYAML struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	InputSchema interface{}    `yaml:"input_schema"`
	Executor    ExecutorConfig `yaml:"executor"`
	Approval    string         `yaml:"approval"`
}

func loadFile(path string) (*Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var my manifestYAML
	if err := yaml.Unmarshal(data, &my); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if my.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if my.Executor.Type != "cli" {
		return nil, fmt.Errorf("executor.type %q not supported (only \"cli\")", my.Executor.Type)
	}
	if len(my.Executor.Command) == 0 {
		return nil, fmt.Errorf("executor.command must have at least one element")
	}
	// Convert input_schema from interface{} (YAML) to json.RawMessage.
	var schemaJSON json.RawMessage
	if my.InputSchema != nil {
		if b, err := json.Marshal(my.InputSchema); err == nil {
			schemaJSON = b
		}
	}
	m := Manifest{
		Name:        my.Name,
		Description: my.Description,
		InputSchema: schemaJSON,
		Executor:    my.Executor,
		Approval:    my.Approval,
	}
	return &Plugin{
		Manifest:   m,
		ToolName:   namePrefix + m.Name,
		SourcePath: path,
	}, nil
}

// deduplicate removes plugins with the same ToolName, keeping the last one
// (project-level overrides user-level since project dirs are appended after user).
func deduplicate(all []*Plugin) []*Plugin {
	seen := make(map[string]bool)
	// Iterate in reverse so the last occurrence wins.
	out := make([]*Plugin, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		p := all[i]
		if !seen[p.ToolName] {
			seen[p.ToolName] = true
			out = append([]*Plugin{p}, out...)
		}
	}
	return out
}

// ── Executor ──────────────────────────────────────────────────────────────────

func executePlugin(ctx context.Context, p *Plugin, argumentsJSON string) (string, error) {
	// Parse the LLM's JSON arguments into a map for template rendering.
	var args map[string]any
	if argumentsJSON != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
			return "", fmt.Errorf("parse arguments: %w", err)
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	// Validate required fields from the input schema.
	if err := validateRequired(p.Manifest.InputSchema, args); err != nil {
		return "", err
	}

	// Build template data: all input fields + env accessor map.
	data := map[string]any{}
	for k, v := range args {
		data[k] = v
	}
	envMap := make(map[string]string)
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	data["env"] = envMap

	// Render command args.
	rendered, err := renderTemplate(p.Manifest.Executor.Command, data)
	if err != nil {
		return "", fmt.Errorf("render command: %w", err)
	}
	if len(rendered) == 0 {
		return "", fmt.Errorf("command is empty after rendering")
	}

	// Resolve timeout.
	timeout := defaultTimeout
	if p.Manifest.Executor.Timeout != "" {
		if d, err := time.ParseDuration(p.Manifest.Executor.Timeout); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, rendered[0], rendered[1:]...)

	// Build environment: inherit current env, then apply plugin overrides.
	cmd.Env = os.Environ()
	for k, vTemplate := range p.Manifest.Executor.Env {
		rendered, err := renderString(vTemplate, data)
		if err != nil {
			continue
		}
		cmd.Env = append(cmd.Env, k+"="+rendered)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // non-zero exit is fine — return output to LLM
	return out.String(), nil
}

// renderTemplate renders each element of tmplStrs through Go's text/template.
func renderTemplate(tmplStrs []string, data map[string]any) ([]string, error) {
	out := make([]string, len(tmplStrs))
	for i, s := range tmplStrs {
		r, err := renderString(s, data)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

func renderString(tmplStr string, data map[string]any) (string, error) {
	t, err := template.New("").Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", tmplStr, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", tmplStr, err)
	}
	return buf.String(), nil
}

// validateRequired checks that all required fields in the JSON schema are present.
func validateRequired(schema json.RawMessage, args map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	var s struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil // malformed schema — skip validation
	}
	for _, field := range s.Required {
		if _, ok := args[field]; !ok {
			return fmt.Errorf("missing required field: %q", field)
		}
	}
	return nil
}
