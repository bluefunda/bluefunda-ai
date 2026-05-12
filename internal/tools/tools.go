package tools

import "encoding/json"

// ToolCall represents a tool invocation request from the LLM.
type ToolCall struct {
	ID        string `json:"tool_call_id"`
	Name      string `json:"tool_name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolSchema is the JSON schema definition sent to the LLM.
type ToolSchema struct {
	Type     string       `json:"type"`
	Function FunctionDef  `json:"function"`
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
				Description: "Read the full contents of a local file. Use this before editing a file to understand its current state.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "Absolute or relative file path"}
					},
					"required": ["path"]
				}`),
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "write_file",
				Description: "Write content to a local file. Creates the file if it does not exist; overwrites it if it does.",
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
	}

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
