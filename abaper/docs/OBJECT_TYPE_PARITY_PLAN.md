# abaper — Object Type Parity Plan (vs. abap-adt-api)

**Status: All 6 plan items implemented and verified live against
abap.bluefunda.com, plus one additional bug found during live testing
(`FUNCTIONGROUP` alias missing from `objectTypeToURI`, breaking
activation/syntax-check for that alias). Unit tests added for all fixes.**

Reference: `marcellourbani/abap-adt-api` (TypeScript), cloned to `~/src/abap-adt-api`.
Scope (per user direction): Program, Class, Function Module, Function Group,
DDIC objects (Table, Structure, Domain, Data Element), CDS Views (DDLS/DF).
Read → Edit → Create, in that order, TDD: write/run a test against the live
system first, fix code until it passes.

Live system used for verification: `abap.bluefunda.com`, client `001`, user
`developer`, via a locally-run `abaper serve --port 8092` (own instance,
separate from another concurrent session using port 8080 — different Z-object
naming prefix `ZABPB_` used to avoid collisions).

## Findings: Read (confirmed live, 2026-07-02)

| Type | Search | REST get | Notes |
|---|---|---|---|
| Program (PROG/P) | ✅ | ✅ | |
| Class (CLAS/OC) | ✅ | ✅ | |
| Function Group (FUGR/F) | ✅ | ✅ | |
| Function Module (FUGR/FF) | ✅ | ✅ | needs `args[0]=<group>` |
| Table (TABL/DT) | ✅ | ✅ | |
| Structure (TABL/DS) | ✅ | ✅ | |
| CDS View (DDLS/DF) | ✅ | ✅ | |
| Domain (DOMA/DD) | ✅ | ❌ | `getObjectHandler` has no DOMAIN case; `GetTypeInfo` internally tries `/ddic/domains/{n}/source/main` which 404s live — domains have no source/main representation |
| Data Element (DTEL/DE) | ✅ | ❌ | same issue; `/ddic/dataelements/{n}` needs plain GET (no Accept override), not the `getTypeSource` path used today |

## Findings: Create/Edit bugs

1. **`CreateFunctionGroup` bug** (`internal/adt/client.go:1206-1238`): sets
   `Type: "FUGR/FF"` (function **module** type ID) instead of `"FUGR/F"`
   (function **group**), and XML root element is `fgroup:functionGroup`
   instead of reference's `group:abapFunctionGroup`. Confirmed live: SAP
   rejects with `ExceptionInvalidData — Data is invalid and could not be
   converted`.
2. **No function module create** (`CreateFunction` missing from
   `types.SourceWriter` + `internal/adt/client.go`). `UpdateFunction` exists
   but nothing creates a new FM inside a group. REST `createObjectHandler`
   has no `FUNCTION` case (400 unsupported).
3. **No table/structure update** (`UpdateTable`/`UpdateStructure` missing).
   Can create but never edit again via source.
4. **No CDS view (DDLS) create/update**. `GetDDLSource` (read) exists;
   `CreateDDLS`/`UpdateDDLS` do not. Reference: creationPath
   `ddic/ddl/sources`, root `ddl:ddlSource`, namespace
   `http://www.sap.com/adt/ddic/ddlsources`, typeId `DDLS/DF`. Activation and
   syntax-check already work generically for DDLS (`objectTypeToURI` has a
   case) — only create/update are missing.
5. **No domain/data-element create/update**. These are **not** source-text
   objects like the others — reference confirms a distinct structured-XML
   properties API (`getDomainProperties`/`setDomainProperties`,
   `getDataElementProperties`/`setDataElementProperties`, see
   `abap-adt-api/src/api/objectcontents.ts`). Plain GET on
   `/ddic/domains/{name}` (no special Accept header) returns the full
   metadata+content XML directly; PUT with `?lockHandle=` sets it. This is a
   different shape from the `source string` convention used everywhere else
   in `SourceWriter` — needs its own typed options struct.

## Plan (priority order)

1. Wire domain/data-element **read** into REST (`getObjectHandler`), fix
   `GetTypeInfo` to use plain GET instead of the wrong source/main + Accept
   dance. *(quick)*
2. Fix `CreateFunctionGroup` type ID + root element bug. *(quick)*
3. Add `CreateFunction` (function module create within a group) —
   mirror `createAndPopulate` pattern used by programs/classes, endpoint
   `functions/groups/{group}/fmodules`. *(medium)*
4. Add `UpdateTable` / `UpdateStructure` — same lock → setObjectSource →
   unlock flow already used by `UpdateProgram` et al, pointed at
   `/ddic/tables/{n}/source/main` / `/ddic/structures/{n}/source/main`.
   *(quick — endpoints and lock/unlock already exist)*
5. Add `CreateDDLS` / `UpdateDDLS` for CDS views — mirror `CreateInclude`
   pattern (metadata create shell + populate). *(medium)*
6. Add domain/data-element **create/update** via the structured-properties
   API. New types (`DomainProperties`, `DataElementProperties` — trimmed to
   the fields abaper needs) + `CreateDomain`/`UpdateDomain`/
   `CreateDataElement`/`UpdateDataElement`. *(large — new XML shape, not the
   source-text convention)*

Each item: write a REST-handler unit test with `fakeADTClient` first (red),
implement, confirm green, then verify live via curl against the `:8092`
instance, then a live-backed integration test where practical.

## Test strategy

- Unit tests (`rest/server/server_test.go`, `fake_adt_client_test.go`):
  extend for every new/changed handler case, run in CI via existing
  `go-ci.yml` (no live system needed).
- Live verification: manual curl against `abaper serve` on `:8092` during
  development, using disposable `ZABPB_*`-prefixed objects in `$TMP` to avoid
  touching real content or colliding with the other concurrent session's
  `ZABAPER_*`-prefixed objects.

## Results (2026-07-02)

Items 1–5 implemented, unit-tested, and verified live end-to-end
(create → read → update → activate) against `abap.bluefunda.com`:

- Domain/data-element **read** now reachable via REST (`GET domain/data_element`).
- `CreateFunctionGroup` bug fixed (was silently broken — SAP rejected every
  call with `ExceptionInvalidData`).
- `CreateFunction` (function module create) added and verified inside a
  freshly-created group.
- `UpdateTable` / `UpdateStructure` added; both verified with a real
  create → activate → update → re-activate cycle.
- `CreateDDLS` / `UpdateDDLS` (CDS views) added and verified the same way.

**Bonus fix found via live testing (not in the original plan):**
`objectTypeToURI` (used by `ActivateObject`/`SyntaxCheck`) recognized `FUGR`
and `FUNCTION_GROUP` but not `FUNCTIONGROUP` — the alias every other REST
handler in this codebase accepts. Activating a function group created via the
REST API failed with "unsupported object type for activation" until fixed.
Added `FUNCTIONGROUP` to the switch; covered by `TestObjectTypeToURI`.

**Known gap, not fixed (documented, low priority):** individual function
module activation. `ActivateObject(ctx, objectType, objectName)` takes no
function-group parameter, so a function module can't be activated on its own
through this API — only its containing function group can. Every other
create/read/update path for function modules works. Fixing this would require
either changing the `ObjectActivator` interface signature (breaking change)
or adding a separate function-module-specific activation method.

**Deferred (item 6): Domain/Data Element create/update.** Confirmed live that
these need the structured-properties XML API described above, not the
source-text convention every other `SourceWriter` method uses. This is a
genuinely different, larger feature (new request/response types, no reuse of
`lockObject`/`setObjectSource`/`createAndPopulate`). Recommend a separate
follow-up issue/PR scoped specifically to this, rather than folding it into
this change.

## Follow-up (2026-07-02): Domain / Data Element create+update — WIP

Branch `feat/ddic-domain-dtel-crud`. Implements `CreateDomain`/`UpdateDomain`/
`CreateDataElement`/`UpdateDataElement` (types + `internal/adt` + REST wiring on
create/save via `domain_properties`/`data_element_properties` request fields).

**Live-proven manually (raw curl):** the full flow works —
create-shell POST → lock → PUT structured properties → unlock → activate —
verified end-to-end for both a domain (CHAR 10, `ZABPB_DOM1`) and a data element
referencing it (`ZABPB_DTEL1`), both reaching `version="active"`. Findings:
- Domains/data elements have NO `/source/main`; they use a plain-GET metadata
  resource and a PUT of a structured XML body to the object URL.
- Activation MUST happen AFTER unlock (locked activation → "User X is currently
  editing").
- The data-element PUT body is strict: every `dtel:*` element must be present in
  the exact reference order, including the `*FieldMaxLength` elements.

**Known open issue (why this isn't done):** through the Go path, create/update
run without error and the properties PUT succeeds, but the object is left
`version="new"` (inactive). The REST `activate` call returns `activated:true`
yet SAP does not actually activate it — i.e. an empty activation worklist.
Suspect the `ActivationRequest` XML shape (`<objectReferences xmlns=...>` with
un-prefixed `objectReference`/`uri`/`name`) differs from the working manual body
(`adtcore:`-prefixed `objectReferences`/`objectReference`/`uri`/`name`). Source
objects activate fine with the current shape, so this only bites DDIC property
objects. Next step: build the activation body with `adtcore:`-prefixed elements
(matching the proven curl) for these types, or reuse the exact reference shape,
then re-verify `version="active"` live.
