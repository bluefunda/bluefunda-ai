# AGENTS.md

Instructions for AI coding agents working on bluefunda-ai.

## Project Overview

Go 1.25 CLI for the BlueFunda AI platform. Uses Cobra for commands, gRPC for backend communication with a BFF service, and OAuth2 device flow (Keycloak) for authentication.

Binary name: `bai`
Module: `github.com/bluefunda/bluefunda-ai`
Config location: `~/.bai/config.yaml`

## Build and Test Commands

```bash
# MUST-RUN before submitting any change
make build          # Build binary (runs go mod tidy first)
make test           # go test -v -race -count=1 ./...
make vet            # go vet ./...
make fmt            # gofmt -w .

# Other targets
make test-cover     # Coverage report
make proto          # Regenerate protobuf code from api/proto/bff.proto
make snapshot       # goreleaser snapshot (test release build)
```

### Validation Sequence

Run these in order before committing:

```bash
make fmt
make vet
make test
make build
```

All four must pass with zero errors.

## Project Structure

```
cmd/bai/main.go             # Entry point (delegates to internal/cmd.Execute)
api/proto/
  bff.proto                  # Source-of-truth service definition (DO NOT hand-edit generated files)
  bff/                       # Generated Go code (bff.pb.go, bff_grpc.pb.go)
internal/
  cmd/                       # Cobra command tree and CLI logic
    root.go                  # Root command, global flags, loadConfig(), outputFormat()
    chat.go                  # Chat commands (list, start, history, context, title, stop)
    model.go                 # Model commands
    mcp.go                   # MCP commands
    user.go                  # User commands
    billing.go               # Billing commands
    ratelimit.go             # Rate limit command
    login.go                 # OAuth device flow login
    health.go                # gRPC health check
    version.go               # Version display
    cmd_test.go              # Integration tests (bufconn in-process gRPC)
  grpc/
    conn.go                  # gRPC connection, TLS auto-detect, auth interceptors
    conn_test.go             # Transport tests
  auth/
    auth.go                  # OAuth2 device authorization grant (RFC 8628)
  config/
    config.go                # YAML config load/save, defaults, token validation
    config_test.go           # Config tests
  ui/
    output.go                # Printer: table/json/quiet output modes
    output_test.go           # Output formatting tests
    stream.go                # Streaming chat renderer
scripts/
  generate-proto.sh          # Protobuf code generation script
```

## Safe Modification Boundaries

### Safe to modify (application logic)
- `internal/cmd/*.go` — Add/modify CLI commands
- `internal/ui/*.go` — Change output formatting
- `internal/config/config.go` — Add config fields
- `internal/auth/auth.go` — Modify auth flow
- `internal/grpc/conn.go` — Modify connection/interceptor logic

### Modify with caution
- `api/proto/bff.proto` — Source-of-truth for the gRPC API; changes require `make proto` to regenerate
- `Makefile` — Build system; changes affect CI
- `.goreleaser.yml` — Release pipeline; changes affect binary distribution
- `go.mod` / `go.sum` — Run `make tidy` after any dependency change

### Do NOT modify
- `api/proto/bff/*.pb.go` — Generated files. Run `make proto` instead.
- `.github/workflows/*.yml` — CI/CD pipelines (uses shared workflows from `bluefunda/release-foundry`)
- `cmd/bai/main.go` — Entry point; should remain a 3-line delegation to `internal/cmd.Execute()`

## Code Conventions

### Command pattern
All commands follow: `bai <service> <operation> [flags]`

```go
var fooCmd = &cobra.Command{
    Use:   "foo",
    Short: "Short description",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg := loadConfig()
        // ... use cfg, outputFormat(cfg), etc.
    },
}
```

Register new commands in `internal/cmd/root.go` → `init()` → `rootCmd.AddCommand(...)`.

### Output contract
- **stdout**: Data only (tables, JSON). Used for piping.
- **stderr**: Status messages via `ui.OK()`, `ui.Info()`, `ui.Error()`, `ui.Warn()`.
- **json mode**: Valid JSON via `protojson` with camelCase field names.
- **quiet mode**: One value per line, no headers.

Always support all three output formats (`table`, `json`, `quiet`) for new commands that return data.

### Error handling
- Use `RunE` (not `Run`) on Cobra commands — return errors, do not `os.Exit()`.
- Wrap errors with `fmt.Errorf("context: %w", err)`.
- Linter allows ignoring errors from: `fmt.Fprint*`, `tabwriter.Flush`, `json.Encoder.Encode`, `conn.Close()` (see `.golangci.yml`).

### gRPC calls
- Get a connection via `grpc.NewConn(cfg)` — handles TLS auto-detect and auth interceptors.
- Always `defer conn.Close()`.
- Create a client with `pb.NewBFFServiceClient(conn.ClientConn())`.
- Use `context.Background()` for unary RPCs; the connection layer handles timeouts (30s default).

### Testing
- Tests use `bufconn` in-memory gRPC — no network, deterministic, fast.
- Add canned responses to the `testBFF` struct in `internal/cmd/cmd_test.go`.
- Run with race detector: `go test -race ./...`
- Test file naming: `*_test.go` in the same package.

### Version injection
Version is set at build time via ldflags:
```
-X github.com/bluefunda/bluefunda-ai/internal/cmd.Version=$(VERSION)
```
Do not hardcode version strings.

## Git and Branch Conventions

### Org-level standards

All `bluefunda` repos follow these shared conventions (defined in `bluefunda/.github`):

- **PR template**: Org-level template at `bluefunda/.github/.github/PULL_REQUEST_TEMPLATE.md`. Each repo carries an identical copy in `.github/PULL_REQUEST_TEMPLATE.md`.
- **CI**: Reusable workflows from `bluefunda/release-foundry`. All Go repos use `go-ci.yml` (lint + test + build).
- **Conventional Commits**: Enforced via Release Please across all repos.
- **Branch protection**: `main` is protected — requires PR + CI pass, no direct pushes.
- **Required secrets**: `GH_PAT` (GitHub PAT for private `bluefunda/*` modules). Additional per-repo secrets documented below.

### Commit messages
Use **Conventional Commits** — Release Please parses these for automated versioning:
- `feat:` — New feature (minor version bump)
- `fix:` — Bug fix (patch version bump)
- `perf:` — Performance improvement
- `security:` — Security fix
- `infra:` — Infrastructure/tooling change
- `chore:` — Maintenance (no version bump)
- `docs:` — Documentation only
- `test:` — Test only

Scopes are optional: `feat(auth): add SSO login`

### Branch naming
- `main` — Protected release branch, requires PR + CI pass
- Feature branches: `feat/<short-description>` or `fix/<short-description>`
- Do NOT push directly to `main`

### Pull requests
PR titles MUST use conventional commit format: `feat: ...`, `fix: ...`, etc.

PR body must follow the org template (`.github/PULL_REQUEST_TEMPLATE.md`):
```
## Summary
<1-3 sentences: what changed and why>

## Type
- [x] feature | fix | performance | security | infrastructure

## Customer Impact
<Who benefits? What problem does this solve?>

## Test Plan
- [x] `make vet` passes
- [x] `make test` passes (race detector enabled)
- [x] `make build` succeeds
- [x] <specific verification for the change>
```

### Release process
Fully automated — do NOT manually tag or create releases:
1. Merge conventional commits to `main`
2. Release Please opens a version-bump PR
3. Merging that PR triggers GoReleaser → binaries → GitHub Release → Homebrew tap

### CI/CD secrets (this repo)
- `GH_PAT` — GitHub PAT for private module access (org-wide)
- `HOMEBREW_TAP_TOKEN` — Token for patching Homebrew tap with private asset URLs

## Dependencies

Only 6 direct dependencies — keep it minimal:
- `github.com/spf13/cobra` — CLI framework
- `google.golang.org/grpc` — gRPC client
- `google.golang.org/protobuf` — Protobuf runtime
- `github.com/fatih/color` — Terminal colors
- `github.com/google/uuid` — UUID generation
- `gopkg.in/yaml.v3` — Config file parsing

Do not add dependencies without clear justification. Run `make tidy` after any change to `go.mod`.

## Common Tasks

### Add a new CLI command
1. Create `internal/cmd/<service>.go` with Cobra command
2. Register in `internal/cmd/root.go` → `init()` → `rootCmd.AddCommand(...)`
3. Support `table`, `json`, `quiet` output formats
4. Add test cases in `internal/cmd/cmd_test.go` (add canned response to `testBFF`)
5. Run full validation sequence

### Add a new RPC to the proto
1. Edit `api/proto/bff.proto`
2. Run `make proto`
3. Implement the command in `internal/cmd/`
4. Add tests

### Modify config schema
1. Edit `internal/config/config.go` — add field to `Config` struct with `yaml` tag
2. Add default backfill in `Load()` if needed
3. Update tests in `internal/config/config_test.go`
