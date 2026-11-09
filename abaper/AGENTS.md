# AGENTS.md — AI Coding Agent Instructions for abaper

## Project Overview

**abaper** is a unified Go CLI and SDK for ABAP development on the ABAPer platform and SAP systems.

Module: `github.com/bluefunda/abaper` | License: Apache 2.0

Two primary surfaces:

1. **CLI** — Cobra commands that communicate with the ABAPer platform gateway via OAuth2:
   - Commands: `login`, `logout`, `status`, `generate`, `deploy`, `test`, `list`, `ai`, `version`
   - Entry point: `cmd/abaper/main.go`

2. **SDK** — Reusable Go packages for SAP ABAP development tooling:
   - `types/` — `ADTClient` interface, all ABAP data structs
   - `internal/adt/` — ADT HTTP client implementation (direct SAP)
   - `internal/lsp/` — LSP server internals
   - `lsp/` — public LSP server wrapper
   - `rest/` — REST API server
   - `lib/` — library wrapper

---

## Build and Verification

```sh
# Build CLI binary
go build -o bin/abaper ./cmd/abaper

# Build all packages
go build ./...

# Vet
go vet ./...

# Lint
golangci-lint run

# Test
go test ./...
```

Linter config: `.golangci.yml`.

---

## Directory Structure

```
cmd/abaper/         # CLI entry point (package main)

internal/
  commands/         # Cobra command definitions (login, generate, deploy, etc.)
  client/           # ABAPer gateway HTTP client + OAuth2 device flow
  config/           # Viper-based config (~/.abaper/config.yaml, tokens.yaml)
  adt/              # ADT HTTP client (ADTClientImpl — ~50 methods, 3400 lines)
  lsp/
    abap/           # ABAP language knowledge (keywords, formatter, syntax)
    abaplint/       # abaplint CLI integration
    backend/        # LSPBackend interface + 3 implementations (Offline, ADT, Hybrid)
    document/       # Open document tracking + URI utilities
    handler/        # LSP protocol handlers (8 files)

types/              # ADTClient interface + all ADT data structs (package types)
lsp/                # Public LSP server wrapper (NewServer, RunStdio, RunTCP)
rest/               # REST API server + GitHub OAuth proxy
lib/                # Library wrapper for embedding
pkg/output/         # Output formatting utilities (JSON / table)

man/                # Unix man page
scripts/            # Build and release scripts
hurl/               # HTTP test files (SAP ADT API)
editors/zed/        # Zed editor extension (Rust)
examples/           # Docker examples
testdata/           # ABAP test fixtures
docs/               # Documentation and Hugo site
```

---

## Key Interfaces

### ADTClient (`types/adt.go`)

Central contract for SAP ABAP operations. ~50 methods:
- Retrieval: `GetProgram`, `GetClass`, `GetFunction`, `GetInclude`, `GetInterface`, `GetStructure`, `GetTable`
- Creation: `CreateProgram`, `CreateClass`, `CreateInclude`, etc.
- Update: `UpdateProgram`, `UpdateClass`, `UpdateInclude`, `UpdateInterface`
- Search/list: `SearchObjects`, `ListPackages`, `GetPackageContents`
- Connection: `TestConnection`, `IsAuthenticated`, `Authenticate`, `SetSessionType`
- LSP support: `SyntaxCheck`, `GetCompletionProposals`, `GetNavigationTarget`
- Activation/testing: `ActivateObject`, `RunUnitTests`

Single implementation: `internal/adt/client.go` (`ADTClientImpl`).

### LSPBackend (`internal/lsp/backend/interface.go`)

```go
type LSPBackend interface {
    SyntaxCheck(objectType, objectName, source string) (*types.SyntaxCheckResult, error)
    Complete(objectType, objectName, source string, line, column int) ([]types.CompletionProposal, error)
    Navigate(objectType, objectName, source string, line, column int) (*types.NavigationTarget, error)
    Format(source string) (string, error)
    Activate(objectType, objectName string) (*types.ActivationResult, error)
    IsConnected() bool
}
```

Three implementations: `OfflineBackend`, `ADTBackend`, `HybridBackend`.

---

## CLI Architecture (ABAPer Gateway)

- Config: CLI flags > env vars (`ABAPER_BASE_URL`, `ABAPER_REALM`) > `~/.abaper/config.yaml`
- Tokens: `~/.abaper/tokens.yaml` (0600 perms), auto-refreshed
- API calls: through ABAPer KrakenD gateway at `/abaper/api/v1/*`
- Auth: OAuth2 device authorization flow via Keycloak, client ID `cai-cli`
- Response: `{ "success": bool, "data": T, "error": string }`
- Headers: `Authorization: Bearer`, `X-Realm`, `X-SAP-*`

## SDK Constraints

- ADT types use both `xml` and `json` struct tags. Maintain both when adding fields.
- Object names are uppercased before ADT calls.
- LSP `HybridBackend` tries ADT first, falls back to offline on error, runs 60-second connection monitor.
- No vendored dependencies.

---

## Environment Variables

### CLI
| Variable          | Purpose                          |
|-------------------|----------------------------------|
| `ABAPER_BASE_URL` | ABAPer API base URL              |
| `ABAPER_REALM`    | Keycloak realm                   |
| `ABAPER_ORG`      | Organization                     |

### SDK (ADT)
| Variable       | Purpose                          |
|----------------|----------------------------------|
| `SAP_HOST`     | SAP system hostname              |
| `SAP_PORT`     | SAP system port                  |
| `SAP_CLIENT`   | SAP client number                |
| `SAP_USERNAME` | SAP username                     |
| `SAP_PASSWORD` | SAP password                     |
| `GH_PAT`       | GitHub personal access token     |

---

## Commit Conventions

- Conventional format: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `perf:`, `security:`, `infra:`
- Scoped: `feat(lsp): add go-to-definition for includes`
- Branches: `<type>/<short-description>`
- PRs target `main`, squash-merged

## Release

Automated via release-please (conventional commits drive changelog) + GoReleaser.
