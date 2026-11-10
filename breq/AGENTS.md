# AGENTS.md

Instructions for AI coding agents working on bluerequests.

## Project Overview

Go 1.25 CLI for the BlueFunda bluerequests change/release management platform. All requests flow through `trm-bff` via gRPC — the CLI never talks to NATS or backend services directly.

Binary name: `breq`
Module: `github.com/bluefunda/bluerequests`
Entry point: `cmd/breq/main.go`
Config location: `~/.breq/config.yaml`

## Build and Test Commands

```bash
make build          # go mod tidy + build binary to ./req
make vet            # go vet ./...
make fmt            # gofmt -w .
make tidy           # go mod tidy
make test           # go test -v -race -count=1 ./...
make test-cover     # Coverage report
make proto          # Regenerate protobuf bindings (requires protoc)
make snapshot       # goreleaser snapshot (test release build)
```

### Validation Sequence

```bash
make fmt && make vet && make test && make build
```

All must pass with zero errors before committing.

## Project Structure

```
cmd/breq/main.go                 # Entry point: invokes root command
api/proto/
  bff.proto                     # BFFService definition — keep in sync with trm-bff
  bff/
    bff.pb.go                   # Generated — do not edit by hand
    bff_grpc.pb.go              # Generated — do not edit by hand
internal/
  a4h/
    session.go                  # A4H session states, steps, poll cadence, formatting (shared by cmd and tui)
  cmd/
    root.go                     # Root cobra command, global flags (--bff, --realm, --output)
    helpers.go                  # bffConn(), printer(), token refresh utilities
    login.go                    # OAuth2 device authorization flow
    health.go                   # gRPC health check
    version.go                  # Version display
    user.go                     # User info command
    cr.go                       # Change request + comment commands (list/get/create/update/delete/stage)
    events.go                   # Event subscribe/publish commands
    rpc.go                      # Low-level request-reply command
    a4h.go                      # A4H session commands (status/start/extend/terminate/watch)
  tui/
    a4h.go                      # Live A4H session screen (breq a4h watch); the CR dashboard is model/list/detail.go
  grpc/
    conn.go                     # gRPC connection, TLS auto-detect, auth interceptors
  auth/
    auth.go                     # OAuth2 device authorization grant (RFC 8628)
  config/
    config.go                   # ~/.breq/config.yaml loader; token storage
  ui/
    output.go                   # Printer: table/json/quiet output modes
scripts/
  generate-proto.sh             # Runs protoc with module= path mode
```

## BFFService RPC Surface

| Command group      | RPC                      | Description                          |
|--------------------|--------------------------|--------------------------------------|
| `req user info`    | GetUserInfo              | Current user from Keycloak           |
| `req events sub`   | SubscribeEvents          | Stream realm-scoped events           |
| `req events pub`   | PublishEvent             | Publish event to NATS subject        |
| `req rpc request`  | RequestReply             | NATS request-reply                   |
| `breq cr list`      | ListChangeRequests       | List change requests with filters    |
| `breq cr get`       | GetChangeRequest         | Get a single change request by ID    |
| `breq cr create`    | CreateChangeRequest      | Create a new change request          |
| `breq cr update`    | UpdateChangeRequest      | Update fields on a change request    |
| `breq cr delete`    | DeleteChangeRequest      | Archive a change request             |
| `breq cr stage`     | UpdateChangeRequestStage | Advance the workflow stage           |
| `breq cr comment list`   | ListComments        | List comments on a change request    |
| `breq cr comment add`    | AddComment          | Add a comment                        |
| `breq cr comment update` | UpdateComment       | Edit a comment                       |
| `breq cr comment delete` | DeleteComment       | Delete a comment                     |
| `breq a4h status`   | GetA4HSession            | Show your on-demand SAP trial session |
| `breq a4h start`    | StartA4HSession          | Start (or return the running) session |
| `breq a4h extend`   | ExtendA4HSession         | Extend the session by one hour       |
| `breq a4h terminate`| TerminateA4HSession      | End the session, delete the system   |

## Adding a New Command

1. Create `internal/cmd/<name>.go` with a cobra command variable
2. Register it in `root.go` via `rootCmd.AddCommand(<name>Cmd)`
3. Use `bffConn()` from `helpers.go` to get the gRPC connection
4. All RPC calls must go through `trm-bff` — never connect to NATS or backend directly

## Proto Workflow

`api/proto/bff.proto` must stay in sync with `trm-bff/api/proto/bff.proto`. When adding RPCs:

1. Add the RPC and message types to both `bff.proto` files
2. Run `make proto` in bluerequests to regenerate client bindings
3. Implement the client call in `internal/cmd/*.go`
4. Implement the server handler in trm-bff `internal/transport/grpc/handler.go`

## Code Conventions

### Command pattern

```go
var fooCmd = &cobra.Command{
    Use:   "foo",
    Short: "Short description",
    RunE: func(cmd *cobra.Command, args []string) error {
        conn, cfg, err := bffConn()
        if err != nil {
            return err
        }
        defer conn.Close()
        // ...
    },
}
```

Register subcommands in the parent's `init()`. Register top-level commands in `root.go` → `init()`.

### Output contract
- **stdout**: Data only (tables, JSON) — used for piping
- **stderr**: Status messages via `p.OK()`, `p.Info()`, `p.Error()`, `p.Warn()`
- Support `table`, `json`, `quiet` output formats for commands that return data

### Error handling
- Use `RunE` (not `Run`) — return errors, do not `os.Exit()`
- Wrap errors: `fmt.Errorf("context: %w", err)`

## Git and Branch Conventions

Follows `bluefunda` org-level standards:

- **Conventional Commits**: `feat:`, `fix:`, `chore:`, `docs:`, `perf:`
- **Branch naming**: `<type>/<short-description>`
- **PRs**: squash-merged to `main`; title must use conventional commit format
- **CI**: `bluefunda/release-foundry` shared workflows
- **Release**: Release Please → GoReleaser → GitHub Release (linux/darwin amd64/arm64 + deb/rpm) + Homebrew tap
- **Required secrets**: `GH_PAT`, `HOMEBREW_TAP_TOKEN`

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/fatih/color` — Terminal colour output
- `google.golang.org/grpc` — gRPC client
- `google.golang.org/protobuf` — Protobuf runtime
- `gopkg.in/yaml.v3` — Config file parsing

## Identity is not the CLI's to assert

The CLI puts `x-realm` and `x-user-id` (from the token it decoded locally) on every call, and nginx forwards gRPC
to trm-bff without checking tokens. trm-bff therefore must not trust that metadata for anything that acts on a
user ID alone. The A4H methods verify the bearer token with Keycloak and ignore `x-user-id` (bluefunda/trm-bff#51);
when adding an RPC like that, do the same on the server. The CLI never sends a user ID in a request body.

## Do NOT

- Connect to NATS, trm-backend-go, or Keycloak directly from the CLI — all traffic goes through `trm-bff` via gRPC
- Edit `api/proto/bff/bff.pb.go` or `bff_grpc.pb.go` by hand — regenerate with `make proto`
- Commit tokens, credentials, or `~/.breq/` contents
- Modify `.github/workflows/` without explicit request
