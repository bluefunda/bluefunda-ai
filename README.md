# BlueFunda AI

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/bluefunda/bluefunda-ai.svg)](https://pkg.go.dev/github.com/bluefunda/bluefunda-ai)
[![Release](https://img.shields.io/github/v/release/bluefunda/bluefunda-ai)](https://github.com/bluefunda/bluefunda-ai/releases)
[![CI](https://github.com/bluefunda/bluefunda-ai/actions/workflows/ci.yml/badge.svg)](https://github.com/bluefunda/bluefunda-ai/actions/workflows/ci.yml)

**`bai` is an AI pair programmer that lives in your terminal.**

Ask it to fix a test, refactor a package, or explain a codebase — it reads and writes your files, runs shell commands, and works the problem turn by turn until it's done. No browser, no copy-paste, no leaving the terminal.

```bash
curl -fsSL https://bluefunda.com/ai/install.sh | sh
bai login
bai "add table-driven tests for the parser package"
```

That's the whole setup. The installer picks the right binary for your OS, verifies the checksum, and drops `bai` on your `PATH`. After `bai login`, you're working.

---

## Why bai

- **It does the work, not just the talking.** bai edits files and runs commands in your project. You review and approve; it executes.
- **Stays in your flow.** It's a terminal tool. It fits next to git, your editor, and your shell — and into CI.
- **You stay in control.** Every tool call is yours to approve. Use `--auto` when you trust the task, `--worktree` to keep changes isolated, `--print` for a dry run.
- **Tuned for real work.** `--fast` for quick answers, `--think` for hard problems, and automatic context compaction so long sessions don't fall over.
- **Extensible.** Connect tools via MCP (Model Context Protocol) — including SAP operations through [ABAPer](https://github.com/bluefunda/abaper).
- **Embeddable.** Drop the agentic loop straight into your own Go programs with the [SDK](#embed-it-go-sdk).

---

## Everyday use

```bash
bai                                   # open an interactive session
bai "why is the build failing?"       # start with a prompt
bai --fast "rename this variable"     # quick, low-latency model
bai --think "design a retry strategy" # extended reasoning for hard problems
bai --auto "add godoc to all exports" # auto-approve tool calls
bai --worktree "try a risky refactor" # run in an isolated git worktree
bai -c                                # resume your most recent session
```

Inside a session, type `/` for slash commands — `/model` to switch models, `/clear` to reset, `/help` for the rest. `Enter` sends, `Ctrl+C` quits.

Scaffold project-level config and conventions once:

```bash
bai init        # creates .bai/ for the current project
```

---

## What's in the box

| Command | What it does |
|---------|--------------|
| `bai [prompt]` | Interactive agentic session with file + shell access |
| `bai login` / `logout` | Sign in / out via browser (OAuth device flow) |
| `bai doctor` | Check configuration and connectivity |
| `bai init` | Scaffold `.bai/` config for a project |
| `bai sessions` | List past sessions; resume with `-c` or `--resume` |
| `bai chat` | Manage named chat sessions (list, start, history, title, stop) |
| `bai model list` | List available LLM models |
| `bai mcp` | Connect and manage MCP tool integrations |
| `bai plugins` | Manage plugins |
| `bai billing` | View subscription and plans |
| `bai update` | Update bai to the latest release |
| `bai completion` | Generate shell completions (bash, zsh, fish, PowerShell) |

Useful flags: `--model`, `--fast`, `--think`, `--auto`, `--auto-apply`, `--max-turns`, `--max-budget-usd`, `--dir`, `--worktree`, `-c`/`--continue`, `--resume`, `--print`, `--no-tools`, `-o/--output {table,json,quiet}`.

---

## Embed it (Go SDK)

Run the full bai agentic loop in-process — no `bai` binary, no subprocess. Bring your own tools.

```go
import "github.com/bluefunda/bluefunda-ai/sdk/agent"

runner := agent.New(agent.Options{
    Model:    "auto",
    MaxTurns: 5,
    OnEvent: func(ev agent.Event) {
        switch ev.Type {
        case "text":     fmt.Print(ev.Text)
        case "tool_use": fmt.Printf("[tool: %s]\n", ev.ToolName)
        case "result":   fmt.Printf("\n--- done (%s) ---\n", ev.StopReason)
        }
    },
})
defer runner.Close()

runner.WithSystemPrompt("You are a concise Go assistant.")
err := runner.Run(ctx, "summarize this repository")
```

Register your own tools via `OnToolCall`, and fall back to `agent.DefaultExecute` for the built-in file and shell ops. The SDK reads credentials from `~/.bai/config.yaml` (written by `bai login`). For cross-language use, the subprocess-based `sdk.Client` is also available.

---

## Install

**macOS / Linux (one-liner)** — installs to `/usr/local/bin` or `~/.local/bin`, verifies SHA256:

```bash
curl -fsSL https://bluefunda.com/ai/install.sh | sh
```

**Homebrew (macOS)**

```bash
brew tap bluefunda/tap && brew install bai
```

**Debian / Ubuntu** — replace `amd64` with `arm64` as needed:

```bash
curl -sL https://github.com/bluefunda/bluefunda-ai/releases/latest/download/bai_linux_amd64.deb -o bai.deb
sudo dpkg -i bai.deb
```

**RHEL / Fedora / Rocky**

```bash
curl -sL https://github.com/bluefunda/bluefunda-ai/releases/latest/download/bai_linux_amd64.rpm -o bai.rpm
sudo rpm -i bai.rpm
```

**From source** (Go 1.26+)

```bash
go install github.com/bluefunda/bluefunda-ai/cmd/bai@latest
```

Or grab a binary from [Releases](https://github.com/bluefunda/bluefunda-ai/releases).

---

## Configuration

`bai` reads `~/.bai/config.yaml`:

```yaml
gateway: api.bluefunda.com          # API gateway
endpoint: grpc.bluefunda.com:443    # BFF gRPC address
domain: your-tenant.bluefunda.com   # tenant domain
defaults:
  model: auto                       # default model
  output: table                     # default output format
```

| Environment variable | Description |
|----------------------|-------------|
| `BAI_ACCESS_TOKEN` | Bearer token — skips `bai login` in CI/CD |
| `BAI_GATEWAY` | Override the gateway URL |
| `BAI_BFF` | Override the BFF gRPC address |
| `BAI_MODEL` | Override the default model |
| `BAI_INSTALL_DIR` | Custom install directory for `install.sh` |

---

## Development

**Prerequisites:** Go 1.26+, `protoc` (+ `protoc-gen-go`, `protoc-gen-go-grpc`) for proto regen, [goreleaser](https://goreleaser.com/) for releases.

```bash
make build      # build the bai binary
make test       # tests with race detector
make vet        # go vet
make fmt        # format
make proto      # regenerate protobuf
make snapshot   # release snapshot via goreleaser
```

```
cmd/bai/          Entry point
internal/
  auth/           OAuth2 device flow (RFC 8628)
  cmd/            Cobra command definitions
  config/         Config loader (~/.bai/config.yaml)
  grpc/           gRPC connection + interceptors
  tools/          Agentic tool implementations
  ui/             Output formatting + Bubble Tea TUI
sdk/agent/        Embeddable in-process agent SDK
api/proto/        Protobuf definitions + generated code
```

Releases are automated via [Release Please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com/): merge a `feat:` or `fix:` commit to `main`, then merge the release PR to publish. See [CHANGELOG.md](CHANGELOG.md).

---

## More

- **Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md)
- **Security:** report vulnerabilities via [SECURITY.md](SECURITY.md)
- **SAP / ABAP developers:** see [ABAPer](https://github.com/bluefunda/abaper) — the same agent, specialized for ABAP

## License

Apache 2.0 — see [LICENSE](LICENSE). Copyright © BlueFunda, Inc.
