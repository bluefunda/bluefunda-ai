# AGENTS.md — breq/

breq (BlueRequests) is part of the bluefunda-ai module. Follow the repository
root [AGENTS.md](../AGENTS.md) for build, test, commit and PR rules.

This directory holds only what is specific to the breq binary:

- `cmd/breq/` — entry point; calls `internal/cmd.ExecuteBREQ`
- `install.sh`, `README.md`, `CHANGELOG.md` (history up to v1.9.1, from bluefunda/bluerequests)

The code is shared with `bai requests` and lives in the core:

- `../internal/cmd/breq.go` — breq identity (`~/.breq`, realm `trm`, client `bluerequests`, BFF `requests.bluefunda.com:443`), root command, `NewRequestsCommands`
- `../internal/cmd/requests_*.go` — `cr`, `events`, `rpc`, `user`, `a4h` commands (constructors: the same tree mounts under two parents)
- `../internal/requests/` — TRM connection (`requests.Conn`) and status colours; `a4h/` session model; `tui/` dashboard (`cr watch`, `a4h watch`)
- `../api/proto/trm/` — TRM BFF proto. Its package is `trm.bff.v1` so it can coexist with bai's `bff.v1`; `scripts/generate-proto.sh` rewrites the client's method paths back to `/bff.v1.BFFService/*`, which is what the server serves. `internal/requests` tests guard this.
