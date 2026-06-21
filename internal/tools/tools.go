package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolCall represents a tool invocation request from the LLM.
type ToolCall struct {
	ID        string `json:"tool_call_id"`
	Name      string `json:"tool_name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolSchema is the JSON schema definition sent to the LLM.
type ToolSchema struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef describes a tool function.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// LocalToolSchemas returns the JSON-encoded tool schemas to send to the LLM.
func LocalToolSchemas() (string, error) {
	schemas := []ToolSchema{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "read_file",
				Description: "Read a local file. For large files use offset and limit to read a window of lines — lines are returned with line numbers. Omit offset/limit to read the full file.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path":   {"type": "string", "description": "Absolute or relative file path"},
						"offset": {"type": "integer", "description": "First line to return (1-based). Default: 0 (start of file)."},
						"limit":  {"type": "integer", "description": "Maximum number of lines to return. Default: 0 (all lines)."}
					},
					"required": ["path"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "edit_file",
				Description: "Replace a unique string in a file with a new string. Use for surgical edits without rewriting the entire file. Fails if old_string appears more than once (add surrounding context to make it unique) or not at all.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path":        {"type": "string", "description": "Absolute or relative file path"},
						"old_string":  {"type": "string", "description": "The exact string to replace (must appear exactly once unless replace_all is true)"},
						"new_string":  {"type": "string", "description": "The replacement string"},
						"replace_all": {"type": "boolean", "description": "Replace all occurrences instead of requiring uniqueness. Default: false."}
					},
					"required": ["path", "old_string", "new_string"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "write_file",
				Description: "Write content to a local file, creating it if it does not exist and overwriting it if it does. Prefer edit_file for modifying existing files.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path":    {"type": "string", "description": "Absolute or relative file path"},
						"content": {"type": "string", "description": "Full file content to write"}
					},
					"required": ["path", "content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "list_dir",
				Description: "List the files and directories at a given path (one level deep).",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "Directory path to list"}
					},
					"required": ["path"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "search_files",
				Description: "Search for files matching a glob pattern under a directory.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"dir":     {"type": "string", "description": "Root directory to search from"},
						"pattern": {"type": "string", "description": "Glob pattern, e.g. '*.go' or '**/*.ts'"}
					},
					"required": ["dir", "pattern"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "bash",
				Description: "Run a shell command and return combined stdout and stderr. Keep commands short and non-interactive.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"command": {"type": "string", "description": "Shell command to execute"}
					},
					"required": ["command"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "search_content",
				Description: "Search file contents using a regular expression. Returns matching lines as 'filepath:linenum: content'. Prefers ripgrep when available. Use this before read_file to locate relevant sections in large files.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pattern":   {"type": "string", "description": "Regular expression to search for"},
						"directory": {"type": "string", "description": "Directory to search in (default: current directory)"},
						"glob":      {"type": "string", "description": "Optional glob to filter files, e.g. \"*.go\" or \"*.ts\""}
					},
					"required": ["pattern"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "web_fetch",
				Description: "Fetch a URL and return its content as plain text. Useful for reading documentation, specs, package READMEs, or any web resource. HTML is stripped. Content capped at 50 000 characters.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"url": {"type": "string", "description": "Full URL to fetch (https:// assumed if scheme is omitted)"}
					},
					"required": ["url"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "web_search",
				Description: "Search the web using DuckDuckGo and return the top results (title, URL, snippet). Use to look up APIs, error messages, or library documentation.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {"type": "string", "description": "Search query"}
					},
					"required": ["query"]
				}`),
			},
		},
	}

	b, err := json.Marshal(schemas)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MergeSchemas appends extra ToolSchemas to the JSON-encoded base schema string
// and returns the combined JSON.
func MergeSchemas(base string, extra []ToolSchema) (string, error) {
	var schemas []ToolSchema
	if err := json.Unmarshal([]byte(base), &schemas); err != nil {
		return "", fmt.Errorf("parse base schemas: %w", err)
	}
	schemas = append(schemas, extra...)
	b, err := json.Marshal(schemas)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// NeedsApproval returns true for tools that modify state and require user confirmation.
func NeedsApproval(toolName string) bool {
	switch toolName {
	case "write_file", "bash":
		return true
	}
	return false
}

// safeBashPrefixes lists command prefixes that are safe to auto-approve without a
// TUI confirmation prompt. Commands that mutate shared state (rm, git push, git reset)
// are deliberately excluded.
var safeBashPrefixes = []string{
	"git status", "git log", "git diff", "git show", "git branch", "git stash list",
	"git remote", "git tag", "git describe", "git rev-parse",
	"go vet", "go build", "go test", "go mod tidy", "go mod verify", "go mod graph",
	"go run", "go generate", "go list", "golangci-lint",
	"make build", "make test", "make lint", "make vet", "make check", "make clean",
	"ls", "ls -", "find ", "cat ", "head ", "tail ", "wc ", "stat ",
	"echo ", "pwd", "which ", "env", "printenv",
	"grep ", "rg ", "sed ", "awk ",
	"npm test", "npm run", "npx ",
}

// IsSafeBashCommand returns true when the bash command JSON matches a known-safe
// prefix and should be executed without a user approval prompt.
func IsSafeBashCommand(argumentsJSON string) bool {
	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return false
	}
	cmd := strings.TrimSpace(fmt.Sprintf("%v", args["command"]))
	for _, prefix := range safeBashPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}
