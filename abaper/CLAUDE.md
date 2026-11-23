# CLAUDE.md — abaper/

abaper is part of the bluefunda-ai module. Follow the repository root
[AGENTS.md](../AGENTS.md) for build, test, commit and PR rules.

This directory holds the SAP code that is specific to the abaper binary:

- `cmd/abaper/` — entry point; calls `internal/cmd.ExecuteABAPer` with the abaper-only commands
- `internal/serve/` — `abaper serve` (REST server) and `abaper lsp` (language server)
- `internal/adt/`, `types/` — direct SAP ADT REST client
- `internal/lsp/`, `lsp/` — ABAP language server
- `rest/` — REST server used by abaper-editor
- `lib/`, `sdk/` — Go embedding APIs for the ADT client

The CLI commands, the gateway client, the SAP system registry, the ABAP tool
pack and the verify gate are shared with `bai abap` and live in
`../internal/abap` and `../internal/cmd/abap.go`.

`abaper serve` (REST backend): SAP credentials are per request. Every request must carry `X-SAP-Host`,
`X-SAP-Client`, `X-SAP-User` and `X-SAP-Password`; header-less requests get 401. `serve` ignores `SAP_*`
and stored systems unless started with `--allow-static-sap` (local single-user use only: logs a warning,
binds `127.0.0.1`). Do not reintroduce a shared/default SAP identity for `serve`.
