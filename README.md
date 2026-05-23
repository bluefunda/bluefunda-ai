# BlueFunda AI

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/bluefunda/bluefunda-ai.svg)](https://pkg.go.dev/github.com/bluefunda/bluefunda-ai)
[![Release](https://img.shields.io/github/v/release/bluefunda/bluefunda-ai)](https://github.com/bluefunda/bluefunda-ai/releases)
[![CI](https://github.com/bluefunda/bluefunda-ai/actions/workflows/ci.yml/badge.svg)](https://github.com/bluefunda/bluefunda-ai/actions/workflows/ci.yml)

**`bai`** — A terminal-native AI assistant for SAP operations. Runs interactive TUI sessions, drives agentic coding loops, and integrates with the BlueFunda AI platform via gRPC.

## Features

- **Interactive TUI** — Full-screen chat interface powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Agentic coding** — `bai ask` launches an autonomous loop that reads/writes files and runs tools
- **Streaming output** — Token-by-token rendering with syntax-highlighted code blocks
- **Model selection** — Switch between available LLM models on the fly
- **MCP integration** — Model Context Protocol tool support for external integrations
- **Multi-format output** — Table, JSON, and quiet modes for scripting
- **Shell completions** — bash, zsh, fish, PowerShell
- **macOS + Linux** — Native binaries for amd64 and arm64

## Installation

### Homebrew (macOS)

```bash
brew tap bluefunda/tap
brew install --cask bai
```

### One-line installer (macOS and Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/bluefunda/bluefunda-ai/main/install.sh | sh
```

### Debian / Ubuntu

```bash
curl -sL https://github.com/bluefunda/bluefunda-ai/releases/latest/download/bai_linux_amd64.deb -o bai.deb
sudo dpkg -i bai.deb
```

### RHEL / Fedora / Rocky

```bash
sudo dnf install https://github.com/bluefunda/bluefunda-ai/releases/latest/download/bai_linux_amd64.rpm
```

### From source

```bash
go install github.com/bluefunda/bluefunda-ai/cmd/bai@latest
```

### Manual download

Download the latest binary for your platform from the [Releases](https://github.com/bluefunda/bluefunda-ai/releases) page.

## Quick Start

```bash
# Authenticate with BlueFunda AI
bai login

# Open the interactive TUI
bai

# Ask a question directly
bai ask "How do I create a transport request in SAP?"

# Start a named chat session
bai chat start "SAP BASIS help"

# List available AI models
bai model list

# Check connection health
bai health
```

## TUI Usage

Launch the TUI by running `bai` with no arguments:

```
bai
```

**Key bindings:**

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Ctrl+C` | Quit |
| `/help` | Show slash commands |
| `/clear` | Clear current session |
| `/model` | Switch model |

**Slash commands** are available inside the TUI by typing `/` followed by the command name.

## CLI Reference

```
bai [command] [flags]

Commands:
  login       Authenticate via OAuth device flow
  ask         Start an agentic session (non-interactive)
  chat        Manage chat sessions (list, start, history, context, title, stop)
  model       List available AI models
  mcp         Manage MCP tool integrations
  user        Show current user account info
  billing     View billing and usage
  ratelimit   Show current rate limit status
  health      Check gRPC connection health
  version     Print version information
  completion  Generate shell completions

Global Flags:
  --bff string      BFF gRPC address (overrides config)
  --domain string   Domain override
  -o, --output      Output format: table, json, quiet
```

### Agentic Mode

```bash
# Run a coding task autonomously
bai ask "Refactor all ABAP function modules in ./src to use structured exceptions"

# Ask with a specific model
bai ask --model gpt-4o "Explain this ABAP program"
```

### Chat Management

```bash
bai chat list                          # List all chat sessions
bai chat start "My chat"               # Start a new session
bai chat history --id <id>             # View message history
bai chat context --id <id>             # Show context window
bai chat title --id <id> "New title"   # Rename a session
bai chat stop --id <id>                # Stop a session
```

## Configuration

`bai` reads its configuration from `~/.bai/config.yaml`:

```yaml
endpoint: grpc.bluefunda.com:443    # BFF gRPC address
domain: your-tenant.bluefunda.com   # Tenant domain
defaults:
  output: table                      # Default output format
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BAI_INSTALL_DIR` | Custom install directory for `install.sh` |
| `BLUEFUNDA_TOKEN` | Bearer token (alternative to `bai login`) |

## Development

### Prerequisites

- Go 1.25+
- `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` (for proto regeneration)
- [goreleaser](https://goreleaser.com/) (for releases)

### Build

```bash
make build      # Build bai binary
make test       # Run tests with race detector
make vet        # Run go vet
make fmt        # Format code
make snapshot   # Build release snapshot with goreleaser
```

### Project Layout

```
bluefunda-ai/
├── cmd/bai/          # Entry point
├── internal/
│   ├── auth/         # OAuth2 device flow (RFC 8628)
│   ├── cmd/          # Cobra command definitions
│   ├── config/       # Config loader (~/.bai/config.yaml)
│   ├── grpc/         # gRPC connection + interceptors
│   ├── tools/        # Agentic tool implementations
│   └── ui/           # Output formatting + BubbleTea TUI
├── api/proto/        # Protobuf definitions + generated code
└── scripts/          # Build utilities
```

### Regenerate Protobuf

```bash
make proto
```

### Running Tests

```bash
make test           # All tests with race detector
make test-cover     # Tests + coverage report
```

## Releases

Releases are automated via [Release Please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com/).

- Merge a `feat:` or `fix:` commit to `main` to trigger a release PR
- Merging the release PR publishes binaries to GitHub Releases, Homebrew tap, and package repositories

See [CHANGELOG.md](CHANGELOG.md) for the full release history.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow, code style, and how to submit pull requests.

## Security

To report a security vulnerability, see [SECURITY.md](SECURITY.md).

## License

Apache 2.0 — see [LICENSE](LICENSE).

Copyright 2024 BlueFunda, Inc.
