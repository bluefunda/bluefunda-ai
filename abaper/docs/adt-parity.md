# ADT Parity Analysis: abaper-ts vs abaper Go SDK

> **Purpose:** Gap analysis for replacing `abaper-ts` with the `abaper` Go SDK as the ADT integration
> layer for `abaper-editor`. No code changes — discovery only.
>
> **Date:** 2026-06-02

---

## 1. Parity Matrix

### 1.1 Connection Management

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| Single-system connection (env config) | `getClient()` singleton | `NewADTClient(config)` | ✅ Present | Both support basic auth + CSRF fetch |
| Multi-system connection (per-request creds) | `getClientFromRequest(req)` via `X-SAP-*` headers | One `ADTClient` per config object (callers create multiple) | ⚠️ Partial | TS has connection pooling keyed on `host\|client\|username`; Go has no built-in pool — callers must manage instances |
| Connection pool with idle eviction | `evictIdleConnections()` — 30-min TTL | ❌ Not present | ❌ Missing | Go SDK is single-client; pooling must be added in the consumer layer (e.g., rest server) |
| Test connection / health check | `testConnectionForRequest(req)` → `{ authenticated, status }` | `TestConnection()` → `error` | ✅ Present | Equivalent behaviour |
| Session reset on stale session (400/401) | `resetClientForRequest(req)` → `withRetryForRequest()` | `doRequest()` auto-retries on 401/403 | ✅ Present | Both retry once; TS resets full client, Go re-authenticates in-place |
| SSL / self-signed cert support | `createSSLConfig(allowSelfSigned)` | `ADTConfig.AllowSelfSigned` | ✅ Present | Both support InsecureSkipVerify |
| Connection status query | `isConnected()`, `getConnectedSystems()` | `IsAuthenticated()` | ⚠️ Partial | Go has per-client boolean; TS tracks all pooled systems |

### 1.2 Session Management / CSRF

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| CSRF token fetch | `abap-adt-api` internal (per login) | `getCSRFToken()` — fetched during `Authenticate()` | ✅ Present | Same `X-CSRF-Token: Fetch` pattern |
| CSRF token refresh on 403 | `withRetryForRequest` resets client + re-logins | `doRequest()` calls `getCSRFToken()` and retries | ✅ Present | Go refreshes token only, TS resets full client — Go approach is lighter |
| Stateful session mode | `abap-adt-api`: `stateful` default | `SetSessionType(stateful)` default | ✅ Present | Both default to stateful |
| HTTP cookie jar (session cookies) | `abap-adt-api` internal | Go `http.Client` cookie jar | ✅ Present | Both maintain session cookies |
| Concurrent request safety | Pool keyed per system; single client per system (not goroutine-safe internally) | No mutex on CSRF token / authenticated state | ⚠️ Partial | Neither is safe for concurrent mutations; Go documentation notes this. TS inherits whatever node-fetch provides. In practice both are used single-threaded per connection. |
| Idle session eviction | Timer-driven, 30 min | ❌ None | ❌ Missing | Go SDK has no TTL; callers must manage lifecycle |

### 1.3 Object CRUD

| Capability | abaper-ts endpoint | Go SDK method | Status | Delta / Notes |
|---|---|---|---|---|
| Read program source | `GET /programs/programs/{NAME}/source/main` | `GetProgram()` | ✅ Present | Go returns structured `ADTSourceCode` with ETag |
| Read class source | `GET /oo/classes/{NAME}/source/main` | `GetClass()` | ✅ Present | |
| Read include source | `GET /programs/includes/{NAME}/source/main` | `GetInclude()` | ✅ Present | |
| Read interface source | `GET /oo/interfaces/{NAME}/source/main` | `GetInterface()` | ✅ Present | |
| Read function module source | `GET /functions/groups/{GROUP}/fmodules/{FUNC}/source/main` | `GetFunction(name, group)` | ✅ Present | |
| Read function group source | `GET /functions/groups/{GROUP}/source/main` | `GetFunctionGroup()` | ✅ Present | |
| Read structure / table definition | `GET /ddic/structures/{NAME}`, `GET /ddic/tables/{NAME}` | `GetStructure()`, `GetTable()` | ✅ Present | |
| Read CDS / DDL source | `GET /ddic/ddl/sources/{NAME}/source/main` | ❌ None | ❌ Missing | TS (via abap-adt-api) handles DDLS; Go SDK does not |
| Generic read by type string | `getObject(type, name)` → auto-routes | `GetObjectSource(type, name)` | ✅ Present | Go supports PROG/CLAS/INCL/INTF; TS also supports DDLS |
| Check object exists | N/A (returns 404 naturally) | `CheckObjectExists(type, name)` | ⚠️ Partial | Convenience method in Go; TS infers from HTTP 404 |
| Write / update program source | `saveObject()` lock→PUT→unlock | `UpdateProgram(name, source)` | ✅ Present | Both lock before write |
| Write / update class source | same | `UpdateClass(name, source)` | ✅ Present | |
| Write / update include source | same | `UpdateInclude(name, source)` | ✅ Present | |
| Write / update interface source | same | `UpdateInterface(name, source)` | ✅ Present | |
| Write / update function module source | `saveObject()` with FUNC type | ❌ None (`UpdateFunction` missing) | ❌ Missing | TS can update any object type generically; Go only has typed update methods for PROG/CLAS/INCL/INTF |
| Write / update function group source | same | ❌ None | ❌ Missing | |
| Write / update structure/table | same | ❌ None | ❌ Missing | |
| Create program | `createObject()` POST + source + activate | `CreateProgram()` / `CreateProgramWithOptions()` | ✅ Present | Go is more feature-complete (transport, responsible, activate flag) |
| Create class | same | `CreateClass()` / `CreateClassWithOptions()` | ✅ Present | |
| Create include | same | `CreateInclude()` | ❌ Missing | Method stub — returns "not implemented" |
| Create interface | same | `CreateInterface()` | ❌ Missing | Stub |
| Create function group | same | `CreateFunctionGroup()` | ❌ Missing | Stub |
| Create structure | same | `CreateStructure()` | ❌ Missing | Stub |
| Create table | same | `CreateTable()` | ❌ Missing | Stub |
| Delete object | `DELETE` via abap-adt-api | ❌ None | ❌ Missing | Neither the REST API nor CLI appears to expose object deletion |
| Object lock / unlock (explicit) | `client.lock()` / `client.unLock()` | `lockObject()` / `unlockObject()` (private) | ⚠️ Partial | Both lock internally; neither exposes lock/unlock as public API. TS exposes via abap-adt-api if needed; Go hides in private methods |
| ETag / optimistic locking | Passed via `etag` field, used in PUT | Returned in `ADTSourceCode.ETag`; used in `setObjectSource` | ✅ Present | Both honour ETags |

### 1.4 Activation

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| Single object activation | `activateObject(name, type)` | `ActivateObject(ctx, type, name)` | ✅ Present | Both return messages with severity |
| Activation with source save first | `activateObject(name, type, source)` — lock→write→activate | ❌ Not standalone | ⚠️ Partial | Go's `CreateProgramWithOptions` does save+activate together; no standalone "save source then activate" helper in Go |
| Mass activation | ❌ Not present | ❌ Not present | — | Neither supports mass activation |
| Dry-run / activation check | ❌ Not present | ❌ Not present | — | Neither supports `precheck` activation |
| Activation message detail (line numbers) | ✅ Yes (line field) | ✅ Yes (line field) | ✅ Present | |

### 1.5 Syntax Check

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| Syntax check (source, line/column errors) | `POST /syntax-check` → messages with line/col | `SyntaxCheck(ctx, type, name, source)` | ✅ Present | Go uses `/checkruns?reporters=abapCheckRun`; TS uses `/syntax-check`. Both return messages with line + column. |
| End line / end column in messages | ✅ Present (`end_line`, `end_col`) | ✅ Present (`EndLine`, `EndCol`) | ✅ Present | |
| Message code field | ✅ Present | ✅ Present | ✅ Present | |

### 1.6 Code Formatting

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| Pretty-print / format ABAP source | `POST /format` → `{ source }` | ❌ Not present | ❌ Missing | TS calls `client.prettyPrinter(source)` → `POST /repository/formatters/format`; Go SDK has no equivalent |

### 1.7 Code Completion & Navigation

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| Code completion proposals | `POST /completion` → `[{ identifier, kind, insert_text }]` | `GetCompletionProposals(ctx, type, name, source, line, col)` | ✅ Present | Go returns `CompletionProposal.Kind` as string ("keyword", "function", etc.); TS returns raw SAP kind code |
| Go-to-definition / navigation target | Route exists but **not used by editor** | `GetNavigationTarget(ctx, type, name, source, line, col)` | ✅ Present | Go is ahead here — TS doesn't expose this to the editor |

### 1.8 Unit Tests

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| Run unit tests for an object | `POST /unit-tests` (**not used by editor**) | `RunUnitTests(ctx, type, name)` | ✅ Present | Both run all tests in the object; no test selection |
| Test result detail (class/method breakdown) | ✅ Yes | ✅ Yes | ✅ Present | |

### 1.9 Transport & Package Operations

| Capability | abaper-ts | Go SDK | Status | Delta / Notes |
|---|---|---|---|---|
| Get transport info for an object | `POST /transports/info` | `GetTransports()` stub — returns empty | ❌ Missing | TS calls `client.transportInfo(url, pkg)` → `GET /transportinfo`; Go has stub only |
| Create transport request | `POST /transports/create` | ❌ Not present | ❌ Missing | |
| List packages by pattern | `POST /objects/list` (type=packages) | `ListPackages(ctx, pattern)` | ✅ Present | |
| Get package contents / tree | `POST /packages/contents` | `GetPackageContents(ctx, name)` | ✅ Present | |
| Search objects | `POST /objects/search` | `SearchObjects(ctx, pattern, types)` | ✅ Present | |

### 1.10 Capabilities Present Only in Go SDK (No TS Equivalent)

| Capability | Go SDK method | Notes |
|---|---|---|
| Get type info (domain / data element) | `GetTypeInfo(ctx, typeName)` | Not in abaper-ts at all |
| Get transaction metadata | `GetTransaction(ctx, name)` | Stub-level but present |
| Get table contents (data rows) | `GetTableContents(ctx, name, maxRows)` | Requires Z-service on SAP |

### 1.11 Capabilities Present Only in abaper-ts (No Go Equivalent)

| Capability | abaper-ts | Notes |
|---|---|---|
| Runtime tracing (SAT-equivalent) | `POST /traces/start`, `/list`, `/results`, `/statements`, `/stop`, `/delete` | Not used by editor; significant TS-only feature |
| Breakpoint debug snapshots | `POST /snapshots/start`, `/status`, `/list`, `/cancel`, `/delete` | Not used by editor; complex async listener architecture |
| CDS / DDL source read | `getObject(type="DDLS", name)` | Simple gap in Go SDK |
| Code formatting (pretty-print) | `POST /format` | Used by editor; significant gap |
| Connection pool (multi-system) | `connectionPool.ts` | Architectural gap; must be added in Go consumer layer |
| Idle eviction / session TTL | `evictIdleConnections()` | Operational concern for multi-tenant deployments |

---

## 2. Connection & Session Model Comparison

| Aspect | abaper-ts | abaper Go SDK |
|---|---|---|
| **Auth mechanism** | HTTP Basic Auth (per request, via abap-adt-api) | HTTP Basic Auth (`req.SetBasicAuth`) per request |
| **CSRF token fetch** | On client creation (`new ADTClient()` → `client.login()`) | On `Authenticate()` call — `GET /discovery` with `X-CSRF-Token: Fetch` |
| **CSRF token refresh** | On 400 (stale session): reset entire client, re-login | On 403: `getCSRFToken()` only — no full re-login |
| **Token storage** | Inside `abap-adt-api` ADTClient instance (opaque) | `c.csrfToken` field on `ADTClientImpl` |
| **Session model** | Stateful (via `client.stateful = session_types.stateful`) | Stateful default (`X-sap-adt-sessiontype: stateful`) |
| **Cookie jar** | Node.js http.CookieJar via abap-adt-api | Go `http.CookieJar` on the `http.Client` |
| **Retry on 401** | Full client reset + re-authenticate + retry once | `Authenticate()` + retry once |
| **Retry on 403/CSRF stale** | Full client reset + re-authenticate + retry once | Token refresh only + retry once (lighter) |
| **Concurrent safety** | Not safe (single client per pooled entry, no mutex) | Not safe (shared `csrfToken` / `authenticated` without mutex) |
| **Connection pooling** | Yes — `Map<string, ADTClient>` keyed on `host\|client\|username` | No — one `ADTClientImpl` per config; callers create multiple |
| **Idle eviction** | Yes — 30-minute idle timeout, timer-based | No |
| **Reconnect on network error** | No auto-reconnect; throws | No auto-reconnect; returns error |
| **Timeout config** | Implicitly via Node.js http (no explicit config) | `ConnectTimeout` (30s default), `RequestTimeout` (60s default) |
| **TLS / self-signed** | `createSSLConfig(allowSelfSigned)` | `ADTConfig.AllowSelfSigned` → `InsecureSkipVerify` |
| **Multi-system support** | Header-driven (`X-SAP-*`), pool per system | Manual — caller creates one client per system |
| **Debug logging** | Optional `adtDebugCallback` (LOG_LEVEL=debug) | `ADTConfig.Debug` flag |

**Key difference:** abaper-ts delegates almost all HTTP mechanics to the `abap-adt-api` library, which is a well-tested external package with its own session, CSRF, and retry logic. The Go SDK implements everything from scratch. This means the Go SDK's session handling is more transparent but has edge cases not yet hardened (e.g., concurrent goroutine safety, CSRF token refresh under load).

---

## 3. Prioritised Gap List

### P1 — Blocks Migration (editor uses it today, Go SDK missing)

| # | Gap | What TS Does | Effort Estimate |
|---|---|---|---|
| P1-1 | **Code formatting (pretty-print)** | `POST /repository/formatters/format` via abap-adt-api | S — single HTTP call, plain text request/response |
| P1-2 | **Transport info for object** | `GET /transportinfo?lockHandle=…` via abap-adt-api | S — parse XML transport list |
| P1-3 | **Create transport request** | `POST /transports` via abap-adt-api | S — parse transport number from response |
| P1-4 | **Update function module source** | `saveObject()` with FUNC type → lock→PUT→unlock | M — add `UpdateFunction(name, group, source)` method |
| P1-5 | **Connection pool (multi-system)** | `connectionPool.ts` + idle eviction | M — needed by rest server for multi-tenant; not in SDK itself but must be in consumer |
| P1-6 | **CDS / DDL source read** | `getObject(type="DDLS", name)` | S — add `GetDDLSource(name)` mirroring existing source-read pattern |

### P2 — Needed Soon (used by TS or blocks future work)

| # | Gap | What TS Does | Effort Estimate |
|---|---|---|---|
| P2-1 | **Update function group source** | `saveObject()` with FUGR type | S — same lock→PUT→unlock pattern |
| P2-2 | **Create include / interface / function group / structure / table** | `createObject()` generic | M each — implement stub methods that currently return "not implemented" |
| P2-3 | **Concurrent safety (mutex on CSRF state)** | Node.js single-threaded (implicit) | S — add `sync.RWMutex` around `csrfToken` / `authenticated` |
| P2-4 | **Idle session eviction** | 30-min pool TTL | S — if pool is added at the consumer layer, add TTL eviction there |
| P2-5 | **Activation with source pre-save** | `activateObject(name, type, source)` (lock→write→activate in one call) | S — compose existing lock/write/activate calls |

### P3 — Nice to Have (not used by editor today)

| # | Gap | What TS Does | Effort Estimate |
|---|---|---|---|
| P3-1 | Runtime tracing (SAT) | Full trace lifecycle (start/list/results/statements/delete) | L — significant feature |
| P3-2 | Breakpoint debug snapshots | Async listener + session store | L — complex async architecture |
| P3-3 | Object deletion | `DELETE` on ADT URI | S — not yet needed |
| P3-4 | Mass activation | Batch activate list of objects | M |
| P3-5 | Transport release / import | Beyond info+create | M |

---

## 4. Recommended Follow-on Issue Titles

Issues to open after Phani reviews this report (do not open automatically):

1. `[abaper] feat: add FormatSource (pretty-print) to Go ADT SDK` *(P1-1)*
2. `[abaper] feat: add GetTransportInfo and CreateTransport to Go ADT SDK` *(P1-2, P1-3)*
3. `[abaper] feat: add UpdateFunction / UpdateFunctionGroup to Go ADT SDK` *(P1-4, P2-1)*
4. `[abaper] feat: add GetDDLSource (CDS/DDLS) to Go ADT SDK` *(P1-6)*
5. `[abaper] feat: implement Create* stubs (include, interface, function group, structure, table)` *(P2-2)*
6. `[abaper] fix: add mutex for CSRF token / session state (concurrent safety)` *(P2-3)*
7. `[abaper-bff or rest] feat: connection pool with idle eviction for multi-system support` *(P1-5, P2-4)*
8. `[abaper] feat: add ActivateWithSource helper (save + activate in one call)` *(P2-5)*

---

## 5. Verification

```bash
# Report exists and is non-empty
test -s ~/src/abaper/docs/adt-parity.md && echo "PASS" || echo "FAIL"

# No Go or TS source files were modified
git -C ~/src/abaper diff --name-only HEAD | grep -E '\.(go|ts)$' \
  && echo "FAIL: source modified" || echo "PASS: no source changes"
```
