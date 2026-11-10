# BlueRequests

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/bluefunda/bluerequests.svg)](https://pkg.go.dev/github.com/bluefunda/bluerequests)
[![Release](https://img.shields.io/github/v/release/bluefunda/bluerequests)](https://github.com/bluefunda/bluerequests/releases)
[![CI](https://github.com/bluefunda/bluerequests/actions/workflows/ci.yml/badge.svg)](https://github.com/bluefunda/bluerequests/actions/workflows/ci.yml)

**`breq`** — A terminal-native CLI for the BlueRequests change and release management platform. Manage SAP transport requests, change orders, and release workflows from the command line or an interactive TUI dashboard.

## Features

- **Interactive TUI dashboard** — Browse, filter, and act on change requests in a full-screen terminal UI
- **Change request management** — Create, update, stage, comment on, and archive change requests
- **Release workflows** — Drive approval and release pipelines for SAP transports
- **Event streaming** — Subscribe to real-time platform events
- **Shell completions** — bash, zsh, fish, PowerShell
- **Multi-format output** — Table, JSON, and quiet modes for scripting and automation
- **gRPC-native** — All operations go through the BlueRequests BFF service with TLS
- **macOS + Linux** — Native binaries for amd64 and arm64

## Installation

### Homebrew (macOS)

```bash
brew tap bluefunda/tap
brew install --cask breq
```

### One-line installer (macOS and Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/bluefunda/bluerequests/main/install.sh | sh
```

### Debian / Ubuntu

```bash
curl -sL https://github.com/bluefunda/bluerequests/releases/latest/download/breq_linux_amd64.deb -o breq.deb
sudo dpkg -i breq.deb
```

### RHEL / Fedora / Rocky

```bash
sudo dnf install https://github.com/bluefunda/bluerequests/releases/latest/download/breq_linux_amd64.rpm
```

### From source

```bash
go install github.com/bluefunda/bluerequests/cmd/breq@latest
```

### Manual download

Download the latest binary for your platform from the [Releases](https://github.com/bluefunda/bluerequests/releases) page.

## Quick Start

```bash
# Authenticate with the bluerequests platform
breq login

# Open the interactive TUI dashboard
req

# List change requests
breq cr list

# Create a change request
breq cr create --project PROJ-1 --description "Hotfix for order processing"

# View a specific change request
breq cr get --id <id>

# Advance a change request to the next stage
breq cr stage --id <id>

# Check connection health
breq health
```

## TUI Usage

Launch the interactive dashboard by running `breq` with no arguments:

```
req
```

**Key bindings:**

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate list |
| `Enter` | Open detail view |
| `Esc` | Go back |
| `q` | Quit |
| `/` | Filter |
| `r` | Refresh |

## CLI Reference

```
breq [command] [flags]

Commands:
  login       Authenticate via OAuth device flow
  cr          Manage change requests (list, get, create, update, delete, stage, comment)
  a4h         On-demand SAP ABAP trial (A4H) session (status, start, extend, terminate, watch)
  events      Subscribe to or publish platform events
  rpc         Low-level gRPC request/reply (advanced)
  completion  Generate shell completions
  user        Show current user account info
  health      Check gRPC connection health
  version     Print version information

Global Flags:
  --bff string      BFF gRPC address (overrides config)
  --domain string   Domain override
  -o, --output      Output format: table, json, quiet
```

### Change Request Commands

```bash
# List change requests
breq cr list
breq cr list --project PROJ-1
breq cr list --status pending --severity high

# Get a specific change request
breq cr get --id <id>

# Create a change request
breq cr create --project PROJ-1 --description "My change"

# Update fields
breq cr update --id <id> --description "Updated description"

# Advance workflow stage
breq cr stage --id <id>

# Archive (soft delete)
breq cr delete --id <id>

# Comment management
breq cr comment list --id <id>
breq cr comment add --id <id> --message "Approved for prod"
breq cr comment update --id <id> --comment-id <cid> --message "Revised"
breq cr comment delete --id <id> --comment-id <cid>
```

### A4H Session Commands

Start your own temporary SAP ABAP trial system, from the terminal. It takes about four minutes to be ready, ends
after three hours (extend by one hour at a time) or one hour of inactivity, and is deleted when it ends.

```bash
breq a4h status                 # state "none" if you have no session
breq a4h start --wait           # start (or show the running one) and follow progress until SAP is ready
breq a4h extend                 # +1 hour
breq a4h terminate              # end it and delete the system (asks first; -y to skip, required when piped)
breq a4h watch                  # live screen: progress, host/ports, time remaining; s start, e extend, x end
```

`status`, `start`, `extend` and `terminate` honour `-o table|json|quiet` (`quiet` prints only the state, e.g.
`sap_running`). Once SAP is ready, `status` shows the host, SAP GUI instance (00, port 3200) and the ABAP
Development Tools URLs (`:50000` HTTP, `:50001` HTTPS). States: `none`, `provisioning_vm`, `vm_running`,
`waiting_for_sap`, `sap_running` (usable), `terminating`, `terminated`.

### Shell Completion

```bash
# bash
breq completion bash > /etc/bash_completion.d/req

# zsh
breq completion zsh > "${fpath[1]}/_req"

# fish
breq completion fish > ~/.config/fish/completions/req.fish
```

## Workflow Concepts

**Stages** represent the approval lifecycle of a change request — typically Draft → Review → Approved → Released. The `breq cr stage` command advances a change request to the next stage.

**Events** are emitted at each stage transition and can be subscribed to with `breq events`.

**Projects** group related change requests. Use `--project` to filter requests by project.

## Configuration

`breq` reads its configuration from `~/.breq/config.yaml`:

```yaml
endpoint: grpc.bluefunda.com:443    # BFF gRPC address
domain: your-tenant.bluefunda.com   # Tenant domain
defaults:
  output: table                      # Default output format
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BREQ_INSTALL_DIR` | Custom install directory for `install.sh` |
| `BLUEFUNDA_TOKEN` | Bearer token (alternative to `breq login`) |

## Development

### Prerequisites

- Go 1.25+
- `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` (for proto regeneration)
- [goreleaser](https://goreleaser.com/) (for releases)

### Build

```bash
make build      # Build breq binary
make test       # Run tests with race detector
make vet        # Run go vet
make fmt        # Format code
make snapshot   # Build release snapshot with goreleaser
```

### Project Layout

```
bluerequests/
├── cmd/breq/          # Entry point
├── internal/
│   ├── a4h/          # A4H session states, steps, polling, formatting (shared by cmd and tui)
│   ├── auth/         # OAuth2 device flow (RFC 8628)
│   ├── cmd/          # Cobra command definitions
│   ├── config/       # Config loader (~/.breq/config.yaml)
│   ├── grpc/         # gRPC connection + interceptors
│   ├── tui/          # BubbleTea TUI dashboard
│   └── ui/           # Output formatting
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

Copyright 2025 BlueFunda, Inc.
