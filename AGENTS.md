# AGENTS.md

Instructions for AI coding agents working on `bluefunda-ai`.

## Project Overview

Go 1.25+ CLI for the BlueFunda AI platform.

- **Binary:** `bai`
- **Module:** `github.com/bluefunda/bluefunda-ai`
- **Config:** `~/.bai/config.yaml`
- **Tokens:** `~/.bai/tokens.yaml` (0600)

Two primary modes:
- `bai` — interactive TUI chat (BubbleTea), streams from cai-bff via gRPC
- `bai code` — agentic coding loop with local tool execution (read/write/bash), context auto-compaction at 100k prompt tokens

## Build and Test

```bash
make build        # build binary (runs go mod tidy first)
make test         # go test -race -count=1 ./...
make vet          # go vet ./...
make fmt          # gofmt -w .
make proto        # regenerate protobuf from api/proto/bff.proto
make snapshot     # goreleaser snapshot (test release build)
```

**Validation sequence before committing — must all pass:**
```bash
make fmt && make vet && make test && make build
```

## Project Structure

```
cmd/bai/main.go             # Entry point — delegates to internal/cmd.Execute()
api/proto/
  bff.proto                 # Source of truth for gRPC API (DO NOT hand-edit generated files)
  bff/bff.pb.go             # Generated — run make proto to regenerate
  bff/bff_grpc.pb.go        # Generated — run make proto to regenerate
internal/
  cmd/
    root.go                 # Root command (bai), global flags: --fast, --think, --model/-m, --new
    chat.go                 # Chat commands + resolveModelAlias()
    code.go                 # bai code: agentic loop, pumpCodeStream, compactHistory
    login.go                # OAuth2 device flow login/logout
    model.go                # bai model list
    mcp.go                  # bai mcp list/select
    update.go               # bai update (delegates to go-update library)
    init.go                 # bai init (scaffold .bai/ project config)
    version.go              # bai version
    health.go               # bai doctor / health
    cmd_test.go             # Integration tests (bufconn in-process gRPC)
  config/
    config.go               # YAML config load/save, defaults (default model: "auto")
    config_test.go
  grpc/
    conn.go                 # gRPC connection, TLS auto-detect, auth interceptors
  ui/
    output.go               # Printer: table/json/quiet output
    stream.go               # Streaming chat renderer (non-TUI)
    tui/
      model.go              # BubbleTea model: token tracking, slash commands
      stream.go             # PumpGRPCStream — gRPC → StreamEvent channel
  tools/
    filesystem.go           # read_file, write_file, edit_file, list_dir, search_files
    bash.go                 # bash tool with allowlist and timeout
    tools.go                # Tool registry, NeedsApproval, IsSafeBashCommand
  audit/
    audit.go                # Structured JSONL audit logging to ~/.bai/audit/
  hooks/
    hooks.go                # PreToolUse / PostToolUse hook pipeline
  session/
    session.go              # Session persistence: save/load/list
  mcp/
    manager.go              # Local MCP client manager (stdio transport)
scripts/
  generate-proto.sh         # Protobuf regeneration
```

## Key Patterns

### Model aliases (resolveModelAlias in chat.go)

| Alias | Sent to backend | Effect |
|-------|----------------|--------|
| `auto` / `""` | `""` | cai-llm-router routing rules decide |
| `fast` | `"groq"` | fast-responder agent (Groq llama-3.3-70b) |
| `think` | `":think"` | cai-bff strips suffix → `thinkingMode="think"` |
| anything else | pass-through | `openai`, `anthropic`, `groq:...`, etc. |

Apply `resolveModelAlias()` on any model string before sending to cai-bff.

### Context auto-compaction (code.go)

When `lastPromptTokens > compactionThreshold (100_000)` before a new iteration:
1. `compactHistory()` sends a no-tools summarisation request
2. Returns `[system msgs, summary assistant msg, last 4 msgs]`
3. `lastPromptTokens` resets to 0; loop continues

### cliCodePayload versioning (#84)

`bai code` encodes history + tool schemas in `Prompt` as JSON (LB strips proto fields 8+).
`cliPayloadVersion = 1` is set on every outgoing payload. cai-llm-router validates and returns a clear error on mismatch.

### Token usage tracking

`pumpCodeStream` returns `iterationUsage{PromptTokens, CompletionTokens}` from the `done`/`stream_end` gRPC event. The TUI `model.go` accumulates `totalPromptTokens` and `totalCompletionTokens` per session.

## Safe Modification Boundaries

**Safe to modify:**
- `internal/cmd/*.go` — add/modify commands
- `internal/ui/*.go` — output formatting
- `internal/config/config.go` — add config fields
- `internal/tools/` — add/modify local tools
- `internal/hooks/`, `internal/audit/`, `internal/session/` — extension points

**Modify with caution:**
- `api/proto/bff.proto` — changes require `make proto`; coordinate with cai-bff; note LB strips proto fields 8+ on the CLI→cai-bff path
- `Makefile` — affects CI
- `.goreleaser.yml` — affects binary distribution

**Do NOT modify:**
- `api/proto/bff/*.pb.go` — generated, run `make proto`
- `.github/workflows/*.yml` — shared workflows from `bluefunda/release-foundry`
- `cmd/bai/main.go` — 3-line delegation only

## Code Conventions

### Command pattern

```go
var fooCmd = &cobra.Command{
    Use:   "foo",
    Short: "Short description",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg := loadConfig()
        conn, _, err := bffConn()
        if err != nil { return err }
        defer conn.Close()
        // ...
    },
}
```

Register in `init()` in `root.go` → `rootCmd.AddCommand(...)`.

### Output contract
- **stdout:** data only (tables, JSON) — safe for piping
- **stderr:** status messages (`ui.OK`, `ui.Info`, `ui.Error`, `ui.Warn`)
- Support `table`, `json`, `quiet` for all data-returning commands

### Error handling
- Use `RunE` (not `Run`) — return errors, never `os.Exit()`
- Wrap: `fmt.Errorf("context: %w", err)`

### gRPC calls
```go
conn, cfg, err := bffConn()
if err != nil { return err }
defer conn.Close()
ctx, cancel := caigrpc.ContextWithTimeout()
defer cancel()
resp, err := conn.Client.SomeRPC(ctx, &pb.Request{...})
```

### Testing
- Tests use `bufconn` in-memory gRPC (no network, deterministic)
- Add canned responses to `testBFF` in `internal/cmd/cmd_test.go`
- Always run with race detector: `go test -race ./...`

## Git and Branch Conventions

- **Conventional Commits:** `feat:`, `fix:`, `chore:`, `docs:`, `perf:`, `security:`
- **Branch naming:** `feat/<desc>`, `fix/<desc>`, `chore/<desc>`
- **PRs:** conventional commit title, target `main`, squash-merged
- **Releases:** automated via release-please; do NOT manually tag

## Dependencies (direct)

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `google.golang.org/grpc` | gRPC client |
| `google.golang.org/protobuf` | Protobuf runtime |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `github.com/charmbracelet/bubbles` | TUI components |
| `github.com/fatih/color` | Terminal colours |
| `github.com/google/uuid` | UUID generation |
| `gopkg.in/yaml.v3` | Config file parsing |
| `github.com/bluefunda/go-update` | Self-update (brew/dpkg/rpm/binary) |

Keep dependencies minimal. Run `make tidy` after any `go.mod` change.

## Common Tasks

### Add a new CLI command
1. Create `internal/cmd/<service>.go`
2. Register in `root.go` → `init()` → `rootCmd.AddCommand(...)`
3. Support `table`, `json`, `quiet` output
4. Add test cases in `cmd_test.go`
5. Run full validation sequence

### Add a local tool for `bai code`
1. Implement in `internal/tools/`
2. Register in `tools.go` tool registry
3. Add to `NeedsApproval()` / `IsSafeBashCommand()` if applicable

### Add a proto field
1. Edit `api/proto/bff.proto`
2. **Check LB constraints:** fields 8+ on `ChatRequest` are stripped by the load balancer. Use fields ≤ 7 or encode as suffix/payload workaround.
3. Run `make proto`
4. Wire in `internal/cmd/` and update cai-bff counterpart

### Change config defaults
1. Edit `internal/config/config.go` — update the backfill in `Load()` and `defaultConfig()`
2. Update `internal/config/config_test.go` to match
