# Agentic Loop & Local Filesystem — Implementation Plan

## Goal

Enable `cai-cli` to write code on the local filesystem via an agentic loop where the LLM can call tools (read file, write file, list directory, run bash), execute them locally on the user's machine, and iterate until the task is done.

---

## Full Stack Architecture

```
cai (React SPA)
  ↓  REST/SSE
cai-gw (KrakenD API gateway)
  ↓  HTTP
cai-bff (BFF — gRPC server implementing bff.proto)
  ↓  NATS pub/sub
cai-llm-router (LLM routing + agentic loop engine)
  ↓  HTTP (Streamable HTTP + SSE)
MCP servers / LLM providers
```

`cai-cli` connects directly to cai-bff via gRPC (bypassing cai-gw), using `cli.bluefunda.com:443`.

---

## Intended Agentic Loop Design

The agentic loop runs **on the CLI**, not in the backend. The backend is used as a single-turn LLM proxy per iteration. Each iteration:

```
CLI: ChatRequest { prompt, messages[], local_tools (JSON schemas), raw_tool_result }
  ↓ gRPC → cai-bff → NATS
cai-llm-router:
  call LLM with tool schemas from local_tools
  ↓ LLM emits tool_call → stream it as tool_call event, stop (CLI drives the loop)
  ↓ LLM emits content   → stream content, send done
CLI:
  ↓ tool_call event → execute tool locally → loop back with raw_tool_result
  ↓ content event   → present to user
```

---

## The Gap: raw_tool_result is Wired Halfway

`raw_tool_result` exists in `ChatRequest` (proto) and flows:
- `cai-cli` → gRPC → `cai-bff` → NATS → `cai-llm-router` ✓

But in `cai-llm-router/internal/handler/chat.go` `buildLLMRequest()` (line ~718),
`req.RawToolResult` is **never read** — it's silently dropped. The LLM never sees the tool result.

Same gap for tool schemas: there's no way to pass local tool definitions to the LLM
via the non-MCP path.

---

## Changes Required by Repo

### 1. `cai-llm-router` (unblocks everything)

**`internal/messages/request.go`**
- Add `LocalTools string` field (JSON-encoded `[]llmrouter.Tool`)
- Add `RawToolResultToolCallID string` field (so tool result maps to the right call)

**`internal/handler/chat.go` — `buildLLMRequest()`**

```go
// Consume raw_tool_result — inject as tool role message
if req.RawToolResult != "" {
    msgs = append(msgs, llmrouter.Message{
        Role:       llmrouter.RoleTool,
        Content:    req.RawToolResult,
        ToolCallID: req.RawToolResultToolCallID,
    })
}

// Consume local_tools — pass schemas to LLM
if req.LocalTools != "" {
    var tools []llmrouter.Tool
    if err := json.Unmarshal([]byte(req.LocalTools), &tools); err == nil {
        llmReq.Tools = tools
        llmReq.ToolChoice = &llmrouter.ToolChoice{Type: "auto"}
    }
}
```

**`internal/handler/chat.go` — `processInline()`**
- Add branch: when `LocalTools != ""` but no `MCPServerName`:
  - Single-turn LLM call (no backend agentic loop — CLI drives the loop)
  - If LLM returns tool_calls: stream them as `tool_call` events, then `done`
  - If LLM returns content: stream normally, then `done`

**`internal/messages/response.go`**
- Ensure `tool_call` event carries enough data for CLI to execute:
  `{ type: "tool_call", tool_call_id: "...", function_name: "...", arguments: "{...}" }`

---

### 2. `cai-bff` (pass-through additions)

**gRPC → NATS bridge**
- Pass `ChatRequest.local_tools` → `messages.ChatRequest.LocalTools`
- Pass `ChatRequest.raw_tool_result_tool_call_id` → `messages.ChatRequest.RawToolResultToolCallID`
- Confirm `raw_tool_result` already passes through (likely does, needs verification)

---

### 3. `cai-cli` (main new feature work)

**`api/proto/bff.proto` — `ChatRequest` additions**
```proto
string local_tools = 8;                  // JSON-encoded tool schemas
string raw_tool_result_tool_call_id = 9; // tool_call_id being answered
repeated ConversationMessage messages = 10; // full conversation history
```

**New: `internal/tools/filesystem.go`**
- `ReadFile(path string) (string, error)`
- `WriteFile(path, content string) error`
- `ListDir(path string) ([]string, error)`
- `SearchFiles(dir, pattern string) ([]string, error)`
- `Bash(cmd string) (string, error)` — requires user approval before execution

**New: `internal/tools/executor.go`**
- Tool schema definitions (JSON for LLM)
- Dispatch: tool_call_id + function_name → call the right local function
- User approval gate for destructive tools (`write_file`, `bash`)

**New: `internal/cmd/code.go` — `ai code` command**
```
ai code [--model <model>] [--dir <path>]
```
Agentic REPL:
1. Read user prompt
2. Build `ChatRequest` with `local_tools` schemas and message history
3. Stream from backend
4. On `tool_call` event: show tool + args, get user approval, execute, append tool result to history
5. On `content` event: print response to user
6. Loop back to step 2 with updated history (including tool results)

**`internal/ui/stream.go`**
- `RenderGRPCStream` currently returns only an error
- Change return type to `(toolCalls []ToolCallEvent, err error)` so the caller (agentic loop) can act on tool calls

---

## Local Tool Schemas (for LLM)

```json
[
  {
    "name": "read_file",
    "description": "Read the contents of a local file",
    "parameters": {
      "type": "object",
      "properties": {
        "path": { "type": "string", "description": "Absolute or relative file path" }
      },
      "required": ["path"]
    }
  },
  {
    "name": "write_file",
    "description": "Write content to a local file (creates or overwrites)",
    "parameters": {
      "type": "object",
      "properties": {
        "path": { "type": "string" },
        "content": { "type": "string" }
      },
      "required": ["path", "content"]
    }
  },
  {
    "name": "list_dir",
    "description": "List files and directories at a path",
    "parameters": {
      "type": "object",
      "properties": {
        "path": { "type": "string" }
      },
      "required": ["path"]
    }
  },
  {
    "name": "bash",
    "description": "Run a shell command and return stdout+stderr",
    "parameters": {
      "type": "object",
      "properties": {
        "command": { "type": "string" }
      },
      "required": ["command"]
    }
  }
]
```

---

## Build Order

1. **`cai-llm-router`** — fix `buildLLMRequest` to consume `raw_tool_result` + `local_tools`.
   Add single-turn-with-tools path in `processInline`. This is self-contained and testable via NATS directly.

2. **`cai-bff`** — add `local_tools` and `raw_tool_result_tool_call_id` pass-through.
   Verify existing `raw_tool_result` pass-through.

3. **`cai-cli`** — proto changes → implement local tools → implement `ai code` agentic REPL.

---

## Notes

- The MCP agentic loop in `cai-llm-router` (`handleMCPRequest`) is untouched — it continues
  serving the React web app's remote MCP tool use.
- The new local-tools path is a parallel, separate code path in `processInline`.
- `cai-gw` has no changes needed — it doesn't inspect chat request bodies.
- The `cai` React app is unaffected — it doesn't use `local_tools`.
