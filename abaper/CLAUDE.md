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
