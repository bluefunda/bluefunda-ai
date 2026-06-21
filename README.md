# BlueFunda AI

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/bluefunda/bluefunda-ai)](https://github.com/bluefunda/bluefunda-ai/releases)
[![CI](https://github.com/bluefunda/bluefunda-ai/actions/workflows/ci.yml/badge.svg)](https://github.com/bluefunda/bluefunda-ai/actions/workflows/ci.yml)

**`bai`** — A terminal-native AI assistant for the BlueFunda platform. Interactive TUI chat, agentic coding loops with local tool execution, and self-update via Homebrew or direct download.

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap bluefunda/tap
brew install bai
```

### Debian / Ubuntu

```bash
curl -sL https://github.com/bluefunda/bluefunda-ai/releases/latest/download/bai_linux_amd64.deb -o bai.deb
sudo dpkg -i bai.deb
```

### RHEL / Fedora / Rocky

```bash
sudo rpm -U https://github.com/bluefunda/bluefunda-ai/releases/latest/download/bai_linux_amd64.rpm
```

### Manual download

Download the latest binary from the [Releases](https://github.com/bluefunda/bluefunda-ai/releases) page.

## Quick Start

```bash
bai login          # authenticate with your BlueFunda account
bai                # open the interactive TUI
bai "explain X"    # start with a message
bai code           # agentic coding mode (reads/writes local files)
bai update         # self-update to the latest release
```

## Usage

### Interactive chat — `bai`

```bash
bai                          # auto model (router decides)
bai --fast                   # Groq fast-responder (~300ms)
bai --think                  # extended thinking
bai -m anthropic             # explicit model
bai "fix the failing tests"  # auto-submit prompt on open
bai --new                    # force a new session (don't resume)
```

**TUI key bindings:**

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Ctrl+C` | Quit |
| `/` | Slash command palette |
| `/model <name>` | Switch model mid-session |
| `/clear` | Clear session |
| `/help` | Show all slash commands |

### Agentic coding — `bai code`

Runs an autonomous tool-calling loop with access to local files and shell:

```bash
bai code                              # interactive TUI + local tools
bai code "add unit tests for auth"    # auto-submit prompt
bai code --model fast                 # use Groq for faster iterations
bai code --auto                       # auto-approve all tool calls
bai code --max-turns 30               # increase iteration budget
bai code --continue                   # resume most recent session
bai code --resume <session-id>        # resume a specific session
bai code --dir ~/myproject            # run in a specific directory
```

**Available local tools:** `read_file`, `write_file`, `edit_file`, `list_dir`, `search_files`, `grep`, `bash`

**Context auto-compaction:** when the conversation exceeds ~100k prompt tokens, `bai code` automatically summarises history and continues — no silent crashes on long sessions.

### Model aliases

| Alias | Behaviour |
|-------|-----------|
| `auto` | Let cai-llm-router's routing rules decide (default) |
| `fast` | Groq llama-3.3-70b — low latency (~300ms) |
| `think` | Extended thinking mode |
| `openai`, `anthropic`, `groq`, … | Pass-through to a specific provider |

Set the default in `~/.bai/config.yaml`:
```yaml
defaults:
  model: auto   # or fast, think, openai, anthropic, ...
```

Or via environment variable:
```bash
BAI_MODEL=fast bai
```

### Other commands

```bash
bai update          # self-update (runs brew update first if installed via Homebrew)
bai sessions        # list past chat and code sessions
bai model list      # list available LLM models
bai mcp list        # list MCP servers
bai login           # sign in (OAuth2 device flow)
bai logout          # sign out
bai version         # print version
bai doctor          # check connectivity and configuration
bai completion      # generate shell completions (bash/zsh/fish/PowerShell)
```

## Configuration

`~/.bai/config.yaml`:

```yaml
endpoint: grpc.bluefunda.com:443   # BFF gRPC address
realm: bluefunda                   # Keycloak realm
defaults:
  model: auto                      # default model alias
  output: table                    # table | json | quiet
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `BAI_MODEL` | Override default model (`auto`, `fast`, `think`, `openai`, …) |
| `BAI_BASE_URL` | Override gateway base URL |

### Project-level config — `.bai/settings.yaml`

Place in your project root to override defaults per project:

```yaml
max_turns: 30
mcp_servers:
  - name: my-server
    command: npx
    args: ["-y", "@my/mcp-server"]
```

### Context injection — `.bai/context.md`

Place a `context.md` in your project root and `bai code` will automatically inject it as the system prompt, giving the agent project-specific knowledge.

## Development

### Prerequisites

- Go 1.25+
- `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` (for proto regeneration only)

### Build

```bash
make build       # build bai binary
make test        # go test -race ./...
make vet         # go vet ./...
make fmt         # gofmt -w .
make proto       # regenerate protobuf code
make snapshot    # goreleaser snapshot build
```

### Project layout

```
cmd/bai/              # Entry point (delegates to internal/cmd)
api/proto/            # Protobuf source + generated Go code
internal/
  cmd/                # Cobra commands: root, chat, code, login, …
  config/             # Config loader (~/.bai/config.yaml)
  grpc/               # gRPC connection + auth interceptors
  ui/                 # Output formatting + BubbleTea TUI
  tools/              # Local tool implementations (read, write, bash, …)
  audit/              # Structured session audit logging
  hooks/              # Pre/post-tool hook pipeline
  session/            # Session persistence (~/.bai/sessions/)
  mcp/                # Local MCP client (stdio transport)
```

## Releases

Fully automated via [Release Please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com/). See [CHANGELOG.md](CHANGELOG.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md).

## License

Apache 2.0 — see [LICENSE](LICENSE).  
Copyright 2024–2026 BlueFunda, Inc.
