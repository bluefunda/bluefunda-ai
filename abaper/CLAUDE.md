# CLAUDE.md — abaper

## What is this?

CLI and Go SDK for the ABAPer platform and SAP ABAP development.

- **CLI**: Communicates with ABAPer APIs through the KrakenD gateway (`abaper-gw`) via OAuth2
- **SDK**: Direct SAP ADT REST client (`types/`, `internal/adt/`), LSP server (`lsp/`, `internal/lsp/`), REST server (`rest/`)

Module: `github.com/bluefunda/abaper` | Go 1.25.1 | License: Apache 2.0

## Build & Verify

```bash
make build        # Build for current platform
make build-all    # Cross-compile for all platforms
make test         # Run tests
make lint         # Run golangci-lint
make vet          # Run go vet
```

## CLI Commands

`login`, `logout`, `status`, `generate`, `deploy`, `check`, `test`, `list objects`, `list packages`, `ai chat`, `version`

## Key Patterns

- OAuth2 device authorization flow via Keycloak (`cai-cli` client ID)
- Config: CLI flags > env vars (`ABAPER_BASE_URL`, `ABAPER_REALM`) > `~/.abaper/config.yaml`
- Tokens stored at `~/.abaper/tokens.yaml` (0600 perms), auto-refreshed
- API calls go through gateway at `/abaper/api/v1/*`
- Response format: `{ "success": bool, "data": T, "error": string }`
- Headers: `Authorization: Bearer`, `X-Realm`, `X-SAP-*`

## SDK Architecture

- `types/adt.go` — `ADTClient` interface (~50 methods), all ADT data structs
- `internal/adt/client.go` — single `ADTClientImpl` (HTTP calls to SAP ADT REST)
- `internal/lsp/` — LSP server internals (backends, handlers, document manager)
- `lsp/` — public LSP server wrapper (`NewServer`, `RunStdio`, `RunTCP`)
- `rest/` — REST API server with `/api/v1/` endpoints
- `lib/` — library wrapper for embedding

## Conventions

- Commits: conventional format (`feat:`, `fix:`, `chore:`)
- Branches: `<type>/<short-description>`
- PRs: conventional commit title, target `main`, squash-merged
- Releases: release-please + GoReleaser for multi-platform binaries + Docker image

## How we work (operating rules)

- Every task starts from a GitHub issue. Read it fully first: `gh issue view <N> 2>/dev/null`.
- M/L issues: produce a short plan and WAIT for approval before editing. S: proceed.
- Respect the issue's "Out of scope" — do not refactor or touch anything outside it.
- Commit in small steps, Conventional Commits, referencing the issue: `feat(abaper): ... (#<N>)`.
- Before claiming done: run the issue's "Verify with" commands; confirm every acceptance box.
- Open the PR with "Closes #<N>" + a one-paragraph, release-note-ready summary.
- Secrets come from Vault/env, never hardcoded. Never run destructive git/deploy without asking.
