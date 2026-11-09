# abaper — Parity & SDK Work Plan (Handoff)

> **Status update (2026-11-09): historical — do not resume from the steps below.**
> Parity items A, B and C are implemented in `rest/server/server.go`
> (`/api/v1/activate` alias, save-mode branch in `createObjectHandler`,
> `/api/v1/packages/contents`), and a top-level `sdk/` package exists.
> Remaining SDK work (tests, packaging, choosing `lib` vs `sdk` as the public
> entry point) is tracked in #157. Still open from this doc and not tracked
> elsewhere: the abaper-ts decommission checks in "Parity acceptance".

Status snapshot as of 2026-07-02. Captures pending work toward three objectives.
Written mid-task because usage limits were approaching — this is the resume point.

## Objectives

1. **abaper has parity with abaper-ts** — ✅ A/B/C built (decommission checks still pending)
2. **abaper calls the bai SDK** — ✅ MET
3. **abaper exposes its own SDK (like bai)** — 🟡 `sdk/` exists; finish in #157

---

## Already shipped (context)

- PR #85 (merged): bump `github.com/bluefunda/bluefunda-ai` v1.25.0 → **v1.34.0**. Closed dependabot PR #83.
- PR #87 (merged, released v1.11.1): fixed `abaper list objects/packages` — route/decode/key bugs (issue #86).
- PR #90 (merged): first tests in the repo — `rest/server`, `internal/client`, `internal/config`, `internal/commands/list`. Also fixed `Client.Activate()` posting to a nonexistent `/api/v1/activate` route (pointed it at `/api/v1/objects/activate`). **NOTE:** see Parity item A below — the *server* actually needs to serve the bare `/api/v1/activate` path too, because every external client uses it.
- CI: shared `bluefunda/release-foundry` `go-ci.yml` runs `go build`, `golangci-lint` (v2.11.4, errcheck enabled), and `go test -race` on every PR. No CI changes needed for new tests.

## Environment / system config

- Local build: `PATH=/usr/local/go/bin:$PATH make build` (Go not on default PATH in this shell). Installed binary at `~/.local/bin/abaper`.
- SAP system configured & connected: **`abap.bluefunda.com`**, client `001`, user **`developer`** (active system in `~/.abaper/systems.json`). `abaper system test` → success. Use this to verify ADT calls live.
- Also configured: `a4h` (a4h.bluefunda.com).
- ⚠️ AI commands (`ai chat`, `ai code`) fail with **"LLM routing failed"** — backend/gateway routing issue, NOT an SDK integration problem. Separate from these objectives.

---

## Objective 2 — bai SDK: DONE

`github.com/bluefunda/bluefunda-ai/sdk/agent` imported in:
- `internal/commands/ai.go`
- `internal/commands/ai_code.go`
- `tui/chat.go`

On v1.34.0. Nothing further required.

---

## Objective 1 — Parity with abaper-ts

### Verified route inventory
- abaper-ts `v1.1.1` (Express 5): 27 routes, in `src/routes/*.ts`.
- abaper Go `v1.11.1`: 21 routes, in `rest/server/server.go` (`mux.HandleFunc` calls ~line 105–139).

### Client usage check (DECISIVE — grepped abaper-editor, abaper-bff, abaper-gw, abaper-mcp, abaper-vscode)
- **traces/* (6 routes): ZERO usage** across all clients → **OUT OF SCOPE, do not build.**
- **snapshots/* (5 routes): ZERO usage** across all clients → **OUT OF SCOPE, do not build.**
- abaper-cli was NOT checked out locally — pull & re-grep before final abaper-ts decommission to be safe.

### What actually must be built (3 items — all consumed by editor + mcp, routed via bff + gw)

#### A. Serve bare `POST /api/v1/activate` (path mismatch)
- **Every** client calls `/api/v1/activate`; NONE call `/objects/activate`. Go only registers `/objects/activate`.
- Evidence: editor `src/services/api.ts:175`, mcp `apiclient.go:334`, bff `internal/transport/http/handler.go:122`, gw `krakend.tmpl:43`.
- **Fix:** register `/api/v1/activate` as an alias to the existing `activateObjectHandler` in `rest/server/server.go`. Keep `/objects/activate` too (Go CLI uses it after PR #90).
- abaper-ts response shape (`src/routes/activate.ts`):
  ```json
  { "success": <bool>, "error": "<only if failed>",
    "data": { "object_name", "object_type", "activated": <bool>,
              "messages": [{"severity":"error|warning|info","text","line"}] } }
  ```
  Current Go `activateObjectHandler` returns `sendSuccess(w, result)` where result is `*types.ActivationResult` (`object_name`, `object_type`, `success`, `messages[]`). **Shape differs** (`activated` vs `success`, top-level `error` on failure). Align the handler's response to the abaper-ts shape for a true drop-in, or confirm editor tolerates `success`. CHECK editor's `ActivationResult` type in `abaper-editor/src/types/adt.ts`.

#### B. Save-mode `POST /api/v1/objects/create` (shape mismatch)
- editor `saveObject` (`api.ts:154`) and mcp `UpdateObject` (`apiclient.go:316`) POST to `objects/create` with `source` and **no `description`** to mean SAVE. Go splits create/save into two endpoints; nobody calls `/objects/save`.
- abaper-ts logic (`src/routes/objects.ts:150`): `if (source && !description)` → SAVE (lock → setObjectSource → unlock), returns:
  ```json
  { "success": true, "data": { "object_name","object_type","created":false,
    "source_inserted":true,"source_length":<n>,"message":"<TYPE> <NAME> saved successfully" } }
  ```
  else CREATE (createObject, then if source: write signed source + activate).
- **Fix:** in Go `createObjectHandler` (`rest/server/server.go` ~line 269), when `req.Source != "" && req.Description == ""`, branch to update/save (call `UpdateProgram/UpdateClass/...` matching object type) instead of Create. Mirror the response shape above. Keep `/objects/save` for back-com. On CREATE-with-source, abaper-ts also appends a signature line + activates — decide whether to replicate the signature/activation (editor may rely on it).

#### C. `POST /api/v1/packages/contents` (missing route — NOT a quick win)
- Used by editor `getPackageContents` (`api.ts:140`) to build the Explorer tree; routed via bff `handler.go:131` + gw `krakend.tmpl:52`.
- **Request:** `{ "package_name": "<PKG>" }` (note key is `package_name`).
- **Response:**
  ```json
  { "success": true, "data": {
      "nodes": [{"name","type","description","expandable","uri"}],
      "objectTypes": [{"type","label"}] } }
  ```
  editor types: `abaper-editor/src/types/adt.ts` → `PackageNode {name,type,description,expandable,uri}`, `PackageContentsResult {nodes, objectTypes}`. Consumed in `src/components/panels/ExplorerPanel.tsx:66,190` — uses `node.expandable` and `node.uri` for lazy tree expansion and open-on-click.
- **⚠️ Gap:** abaper-ts uses ADT `client.nodeContents('DEVC/K', pkg)` (hierarchical: sub-packages `expandable`, real `uri`, `objectTypes` from SAP). Go's `GetPackageContents` (`internal/adt/client.go:383`) instead does a **flat `quickSearch` by packageName** — returns `[]ADTObject` with **no URI and no expandable/objectTypes**. Mapping that into `{nodes,objectTypes}` would be degraded → editor tree navigation (expand sub-packages, open objects by uri) would break.
- **Proper fix (medium effort):** add a faithful `nodeContents` ADT method to the Go client:
  - `POST {baseURL}/sap/bc/adt/repository/nodestructure?parent_type=DEVC/K&parent_name=<PKG>&withShortDescriptions=true`
  - Parse the node-structure XML (`SEU_ADT_REPOSITORY_OBJ_NODE` rows: `OBJECT_NAME`, `OBJECT_TYPE`, `DESCRIPTION`, `OBJECT_URI`, and the object-types table with `OBJECT_TYPE_LABEL`).
  - Add a new type (e.g. `types.ADTNodeStructure{Nodes []ADTNode, ObjectTypes []ADTObjectType}`) or extend `ADTObject` with `URI`.
  - New REST handler `packageContentsHandler` → register `/api/v1/packages/contents`.
  - **VERIFY LIVE** against `abap.bluefunda.com` (nodestructure behavior varies by SAP kernel; the trial has silently ignored some ADT ops before — see memory).
  - `ADTObject` currently (`types/adt.go:13`) has no `URI` field — add one (xml attr) if reusing it.

### Parity acceptance
Decommission abaper-ts only after A + B + C are built, verified live, AND abaper-cli is confirmed not to need traces/snapshots. Then redeploy the Go server (Komodo stack `abaper` on apps node `10.0.1.8:8013`, `komo execute deploy-stack abaper`) and repoint/verify bff + gw before taking abaper-ts down.

---

## Objective 3 — abaper SDK (like bai's `sdk/agent`)

### bai's pattern (`~/src/bluefunda-ai/sdk/agent/`)
- `agent.New(Options) *Runner`; own public types (`Event`, `ToolCall`, ...); wraps `internal/*` via unexported adapters; has `doc.go` + `example_test.go`. Public façade, internals hidden.

### abaper today
- `types/adt.go` — public `ADTClient` interface (~44 methods) + all data structs. The intended contract.
- `internal/adt/client.go` — concrete `ADTClientImpl`, constructor `NewADTClient(config *types.ADTConfig) *ADTClientImpl`. **Under internal/ → not externally importable.** Deps: only `types` + `go.uber.org/zap` + stdlib (clean, easy to lift).
- `lib/wrapper.go` — **already a de-facto SDK**: `NewADTClient(*types.ADTConfig) types.ADTClient` and `CreateADTClient(host, client, user, pass) (types.ADTClient, error)`. Undocumented, easy to miss.
- No top-level `sdk/` package exists.

### Gap = presentation, not capability. Recommended (mirror bai)
Introduce **`sdk/`** package:
- `sdk.New(Options) (types.ADTClient, error)` with `Options{Host, Client, Username, Password, Language, AllowSelfSigned, Timeouts...}` wrapping `internal/adt` (like bai wraps `internal/agent`).
- Add `sdk/doc.go` + `sdk/example_test.go` (godoc runnable examples: create client → GetProgram/CreateClass/Activate).
- Keep `types.ADTClient` as the returned interface; keep concrete impl hidden.
- Optionally deprecate/redirect `lib/` to `sdk/`.
- Low risk, contained. Do AFTER parity quick-wins A + B (per chosen sequencing).

---

## Chosen sequencing (user decision)
1. **Parity quick-wins first**: A (activate alias) + B (save-mode create) now; C (packages/contents) requires the nodeContents method.
2. **Skip traces/snapshots** (zero usage confirmed).
3. **Then SDK** (objective 3).

## Immediate next steps on resume
1. Create GitHub issue: "feat(rest): abaper-ts parity — activate alias, save-mode create, packages/contents" (note traces/snapshots explicitly out of scope + usage evidence).
2. Branch `feat/abaper-ts-parity`. Implement A + B (quick), then C (nodeContents + route).
3. Check `abaper-editor/src/types/adt.ts` `ActivationResult` shape before altering activate response.
4. Verify live via `abaper serve` against `abap.bluefunda.com`, curl each endpoint.
5. Add handler tests (extend `rest/server/server_test.go` + `fake_adt_client_test.go`).
6. Separate branch `feat/sdk` for objective 3.
7. Before decommission: pull abaper-cli, re-grep traces/snapshots; redeploy Go server; verify bff/gw.

## Key file references
- `rest/server/server.go` — route table (~105–139) + handlers.
- `internal/adt/client.go` — ADT client (`GetPackageContents` L383, `NewADTClient` L64).
- `types/adt.go` — `ADTClient` interface (L231) + structs (`ADTObject` L13, no URI field).
- `lib/wrapper.go` — de-facto SDK.
- abaper-ts refs: `src/routes/{activate,objects,packages}.ts`.
- editor refs: `src/services/api.ts` (140 pkgs, 154 save, 175 activate), `src/types/adt.ts`, `src/components/panels/ExplorerPanel.tsx`.
