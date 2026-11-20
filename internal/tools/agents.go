package tools

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// AgentType is a named subagent profile the task tool can spawn (#408): a
// tool allow-list, an optional model and instructions prepended to the task.
// Mirrors Claude Code's subagents (.claude/agents/*.md).
type AgentType struct {
	Name         string
	Description  string
	Tools        []string // nil = every tool the child session has
	Model        string   // "" = the child's default model
	Instructions string
	Source       string // file path, or "built-in"
}

// DefaultAgentType is used when the task call names no subagent_type.
const DefaultAgentType = "general"

// readOnlyToolNames are the built-in tools that never change state. It backs
// IsReadOnlyTool and the explore/plan agent types.
var readOnlyToolNames = []string{
	"read_file", "list_dir", "search_files", "search_content", "read_notebook",
	"web_fetch", "web_search", "memory_read", "memory_list",
}

// BuiltinAgentTypes returns the agent types every session has.
func BuiltinAgentTypes() []AgentType {
	ro := append([]string(nil), readOnlyToolNames...)
	return []AgentType{
		{
			Name:        DefaultAgentType,
			Description: "General-purpose agent with every tool. Use for self-contained work that may change files or run commands.",
			Source:      "built-in",
		},
		{
			Name:        "explore",
			Description: "Read-only agent for finding code, reading files and answering questions about the codebase. Cannot change anything.",
			Tools:       ro,
			Instructions: "You are a read-only exploration agent. Search and read to answer the task. " +
				"You cannot modify files or run shell commands. Report concrete findings with file:line references.",
			Source: "built-in",
		},
		{
			Name:        "plan",
			Description: "Read-only agent that investigates and returns a step-by-step implementation plan without making changes.",
			Tools:       ro,
			Instructions: "You are a planning agent. Investigate the codebase read-only, then return a concise, " +
				"step-by-step implementation plan naming the files and functions to change. Do not attempt the change.",
			Source: "built-in",
		},
	}
}

var agentNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// agentFrontmatter is the YAML header of an agent file. tools accepts either
// a comma-separated string or a YAML list, as in Claude Code.
type agentFrontmatter struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Tools       yaml.Node `yaml:"tools"`
	Model       string    `yaml:"model"`
}

// ParseAgentFile parses one agent definition: YAML frontmatter between "---"
// lines, then the instructions. name and description are required.
func ParseAgentFile(path string, data []byte) (AgentType, error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(data, []byte("---\n"))
	if !ok {
		return AgentType{}, fmt.Errorf("%s: missing --- frontmatter", path)
	}
	head, body, ok := bytes.Cut(rest, []byte("\n---"))
	if !ok {
		return AgentType{}, fmt.Errorf("%s: unterminated frontmatter", path)
	}
	var fm agentFrontmatter
	if err := yaml.Unmarshal(head, &fm); err != nil {
		return AgentType{}, fmt.Errorf("%s: frontmatter: %w", path, err)
	}
	if !agentNameRE.MatchString(fm.Name) {
		return AgentType{}, fmt.Errorf("%s: name %q must be lowercase letters, digits, - or _", path, fm.Name)
	}
	if strings.TrimSpace(fm.Description) == "" {
		return AgentType{}, fmt.Errorf("%s: description is required", path)
	}
	toolList, err := parseToolsField(&fm.Tools)
	if err != nil {
		return AgentType{}, fmt.Errorf("%s: tools: %w", path, err)
	}
	return AgentType{
		Name:         fm.Name,
		Description:  strings.TrimSpace(fm.Description),
		Tools:        toolList,
		Model:        strings.TrimSpace(fm.Model),
		Instructions: strings.TrimSpace(strings.TrimPrefix(string(body), "\n")),
		Source:       path,
	}, nil
}

func parseToolsField(n *yaml.Node) ([]string, error) {
	var raw []string
	switch n.Kind {
	case 0: // absent
		return nil, nil
	case yaml.ScalarNode:
		raw = strings.Split(n.Value, ",")
	case yaml.SequenceNode:
		if err := n.Decode(&raw); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("must be a list or a comma-separated string")
	}
	var out []string
	for _, t := range raw {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// LoadAgentTypes returns the built-in agent types overlaid with the *.md
// definitions found in dirs, in order — later directories win on a name
// clash, so pass the user directory before the project one. Files that fail
// to parse are skipped and reported in errs.
func LoadAgentTypes(dirs ...string) (types []AgentType, errs []error) {
	byName := map[string]AgentType{}
	for _, t := range BuiltinAgentTypes() {
		byName[t.Name] = t
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		sort.Strings(matches)
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			t, err := ParseAgentFile(path, data)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			byName[t.Name] = t
		}
	}
	for _, t := range byName {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i].Name < types[j].Name })
	return types, errs
}

// The session's agent types, set once at startup. Unset means built-ins only.
var (
	agentTypesMu sync.RWMutex
	agentTypes   []AgentType
)

// SetAgentTypes records the agent types the task tool offers this session.
func SetAgentTypes(types []AgentType) {
	agentTypesMu.Lock()
	defer agentTypesMu.Unlock()
	agentTypes = append([]AgentType(nil), types...)
}

// AgentTypes returns the session's agent types (built-ins if none were set).
func AgentTypes() []AgentType {
	agentTypesMu.RLock()
	defer agentTypesMu.RUnlock()
	if agentTypes == nil {
		return BuiltinAgentTypes()
	}
	return append([]AgentType(nil), agentTypes...)
}

// lookupAgentType resolves a subagent_type ("" = the default type).
func lookupAgentType(name string) (AgentType, error) {
	if name == "" {
		name = DefaultAgentType
	}
	types := AgentTypes()
	names := make([]string, 0, len(types))
	for _, t := range types {
		if t.Name == name {
			return t, nil
		}
		names = append(names, t.Name)
	}
	return AgentType{}, fmt.Errorf("unknown subagent_type %q; available: %s", name, strings.Join(names, ", "))
}

// taskPrompt prefixes the agent type's instructions to the task prompt.
func taskPrompt(t AgentType, prompt string) string {
	if t.Instructions == "" {
		return prompt
	}
	return t.Instructions + "\n\n## Task\n\n" + prompt
}
