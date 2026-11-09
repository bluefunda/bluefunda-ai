# ABAPer

[![Go Reference](https://pkg.go.dev/badge/github.com/bluefunda/abaper.svg)](https://pkg.go.dev/github.com/bluefunda/abaper)
[![CI](https://github.com/bluefunda/abaper/actions/workflows/ci.yml/badge.svg)](https://github.com/bluefunda/abaper/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**An AI pair programmer for SAP ABAP — in your terminal.**

ABAPer connects an AI agent to your SAP system over ADT, so you can generate, deploy, test, and refactor ABAP objects without leaving the command line. Ask it to write a report and deploy it; have it read a class, find the bug, and activate the fix. It's also a Go SDK and LSP server, so you can automate SAP workflows in code or bring ABAP intelligence to any editor.

```bash
sh -c "$(curl -fsSL https://bluefunda.com/abaper/install.sh)"
abaper login
abaper                # opens the interactive AI chat
```

---

## Why ABAPer

- **The agent talks to SAP, not just about it.** It deploys, activates, and runs unit tests through ADT — real objects on your system.
- **Terminal-native.** Fits next to git and your shell, and runs headless in CI/CD where the Web IDE and editor plugins can't.
- **Two modes of AI.** `ai chat` for quick answers, `ai code` for an agentic loop that reads and edits local ABAP source and deploys it.
- **Scriptable and embeddable.** Drive ADT from Go with the SDK — no platform account needed — or embed the LSP server in any editor.
- **One ecosystem.** Same backend and AI as the [Web IDE](https://github.com/bluefunda/abaper-editor) and [VS Code extension](https://github.com/bluefunda/abaper-vscode) — pick the surface that fits the task.

---

## Get going

```bash
# 1. Authenticate with the ABAPer platform
abaper login

# 2. Connect your SAP system (stored at ~/.abaper/systems.json, 0600)
abaper system add --host https://my-sap:44300 -u DEVELOPER -p secret
abaper system test

# 3. Work — interactively or by command
abaper                                  # interactive AI chat (TUI)
abaper ai code "write a report that reads MARA and prints MATNR"
abaper deploy --type program --name ZMY_REPORT --source-file report.abap
```

---

## What you can do

### AI assistance

```bash
abaper ai chat "Explain SELECT FOR ALL ENTRIES"
abaper ai chat "Optimise this" --context-file program.abap
abaper ai code "add an authorization check to ZCL_ORDER"   # agentic: reads, edits, deploys
```

`ai code` runs an agentic loop (default 20 turns) that can read and edit local ABAP files and deploy them. When it finishes, abaper syntax-checks, activates and unit-tests every object the agent deployed, gives the agent up to two rounds to fix failures, and prints the list of SAP objects it touched (`--no-verify` skips the gate). The agent never renames an object to work around a conflict; it stops and reports it. It uses the embedded [bluefunda-ai](https://github.com/bluefunda/bluefunda-ai) SDK — sign in once with `bai login`.

### Object operations

```bash
abaper generate --type class --name ZCL_MY_CLASS --source-file my_class.abap   # create on SAP
abaper deploy   --type program --name ZMY_REPORT --source-file report.abap     # upload + activate
abaper check    --type class --name ZCL_MY_CLASS --source-file my_class.abap   # syntax check, no save
abaper test     --type class --name ZCL_MY_CLASS                               # run unit tests
abaper list objects  --package ZDEV
abaper search "ZMY_*"
```

Object types: `program`, `class`, `interface`.

### SAP system management

```bash
abaper system add --host https://my-sap:44300 --name DEV --client 200 -u USER -p PASS
abaper system list     # ● marks the active system
abaper system use DEV
abaper system test
```

Manage systems from inside the TUI too: `/system add`, `/system list`, `/system edit <name>`.

---

## Go SDK

Drive SAP ADT operations directly from Go — no ABAPer platform account required.

```bash
go get github.com/bluefunda/abaper/lib
```

```go
import "github.com/bluefunda/abaper/lib"

client, err := lib.CreateADTClient(
    "https://my-sap.example.com:44300", "100", "DEVELOPER", "secret",
)

src, err := client.GetClass(ctx, "ZCL_MY_CLASS")
err = client.CreateProgram(ctx, "ZMY_PROG", "My program", "$TMP", source)
result, err := client.ActivateObject(ctx, "PROG", "ZMY_PROG")
results, err := client.SearchObjects(ctx, "ZMY_*", []string{"PROG", "CLAS"})
```

Key interfaces in `types/adt.go`: `SourceReader`, `SourceWriter`, `PackageBrowser`, `ObjectActivator`, `LangFeatures`, and the full `ADTClient`.

### LSP server

Bring SAP language intelligence to any LSP-capable editor:

```go
import "github.com/bluefunda/abaper/lsp"

srv := lsp.NewServer(adtClient, "/path/to/workspace")
srv.RunStdio()       // stdio transport (most editors); or srv.RunTCP(":2087")
```

---

## Install

**One-line installer (macOS / Linux)** — installs to `/usr/local/bin`, else `~/.local/bin`; override with `ABAPER_INSTALL_DIR`:

```bash
sh -c "$(curl -fsSL https://bluefunda.com/abaper/install.sh)"
```

**Homebrew (macOS)**

```bash
brew tap bluefunda/tap && brew install --cask abaper
```

**Debian / Ubuntu**

```bash
VER=$(curl -fsSL https://api.github.com/repos/bluefunda/abaper/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -sL "https://github.com/bluefunda/abaper/releases/download/v${VER}/abaper_${VER}_linux_${ARCH}.deb" -o abaper.deb
sudo dpkg -i abaper.deb
```

**RHEL / Fedora / Rocky**

```bash
VER=$(curl -fsSL https://api.github.com/repos/bluefunda/abaper/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
sudo dnf install "https://github.com/bluefunda/abaper/releases/download/v${VER}/abaper_${VER}_linux_${ARCH}.rpm"
```

**Docker**

```bash
docker run --rm -v ~/.abaper:/root/.abaper ghcr.io/bluefunda/abaper status
```

**From source** (Go 1.26+)

```bash
go install github.com/bluefunda/abaper/cmd/abaper@latest
```

Or download a binary from the [releases page](https://github.com/bluefunda/abaper/releases).

---

## Pick your surface

ABAPer is available across multiple clients — all share the same backend and AI.

| Client | Repo | Best for |
|--------|------|----------|
| **CLI / TUI** | **this repo** | Terminal workflows, scripting, CI/CD |
| **Go SDK** | **this repo** (`lib/`) | Embedding ADT operations in Go |
| **Web IDE** | [abaper-editor](https://github.com/bluefunda/abaper-editor) | Browser-based editing, no install |
| **VS Code** | [abaper-vscode](https://github.com/bluefunda/abaper-vscode) | VS Code users |

---

## Configuration

Priority: CLI flags → environment variables → `~/.abaper/config.yaml`

```yaml
# ~/.abaper/config.yaml
base_url: https://api.bluefunda.com
realm: trm
org: default
```

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--base-url` | `ABAPER_BASE_URL` | `https://api.bluefunda.com` | API gateway URL |
| `--realm` | `ABAPER_REALM` | `trm` | Keycloak realm |
| `--org` | `ABAPER_ORG` | `default` | Organisation |
| `-o`, `--output` | — | `text` | Output format: `text`, `json` |

SAP credentials are stored at `~/.abaper/systems.json` (0600) and sent as `X-SAP-*` headers so the gateway can initialise the SAP MCP server for AI tools.

---

## How it fits together

```
abaper CLI / TUI / SDK
      │  Bearer token + X-SAP-{Host,Client,User,Password} headers
      ▼
abaper-gw  (KrakenD gateway — authenticates, routes, starts SAP MCP server)
      │
      ▼
SAP ADT REST API    ◄── also reachable directly via lib/ (no gateway)
```

AI features run through the embedded [bluefunda-ai](https://github.com/bluefunda/bluefunda-ai) agent SDK.

```
cmd/abaper/   CLI entry point (Cobra)
tui/          Bubble Tea TUI — chat + SAP system form
internal/     Cobra commands, gateway client, config, ADT client, LSP internals
types/        Shared interfaces and ADT data structures
lsp/          Public LSP server wrapper
rest/         REST API server (CLI feature parity, no AI)
lib/          Library wrapper for Go embedding
```

---

## Development

**Prerequisites:** Go 1.26+, golangci-lint

```bash
make build        # build for current platform
make build-all    # cross-compile for all platforms
make test         # go test -race ./...
make lint         # golangci-lint
make vet          # go vet
```

Releases are automated via [release-please](https://github.com/googleapis/release-please): merge PRs with [conventional commit](https://www.conventionalcommits.org/) titles, then merge the release PR to trigger GoReleaser → multi-platform binaries + Docker image at `ghcr.io/bluefunda/abaper`.

---

## License

[Apache License 2.0](LICENSE) — Copyright © BlueFunda, Inc.
