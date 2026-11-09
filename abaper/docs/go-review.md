# Go Code Review: `github.com/bluefunda/abaper`

---

## Executive Summary

**Idiomatic Go Score: 4 / 10**

### Biggest Strengths
- The internal `client` package (`internal/client/abaper_api.go`) is genuinely well-structured: clean generics use, proper context propagation, solid response decoding.
- `LSPBackend` interface is small and focused — the right instinct.
- `document.Manager` is a textbook thread-safe value store.
- `HybridBackend` implements a clean circuit-breaker pattern with proper mutex usage.

### Biggest Architectural Risks
1. `ADTClient` is a 30-method god interface that poisons every consumer and makes the codebase unmaintainable as it grows.
2. `internal/adt/client.go` is 3400 lines of copy-paste — a maintenance liability and testing black hole.
3. Zero tests: not a stylistic issue, a correctness risk. The complexity of the lock/unlock/activate flow demands tests.
4. `context.Context` is entirely absent from the ADT client, so no request can be cancelled, timed out, or traced.
5. `log.Fatal` is called inside library code, silently killing the host process.

### Top 5 Highest ROI Refactors

| # | Refactor | Impact |
|---|----------|--------|
| 1 | Split `ADTClient` into capability interfaces | Testability, composability, LSP/REST decoupling |
| 2 | Add `context.Context` to every HTTP call | Cancellation, tracing, deadline propagation |
| 3 | Collapse 8 identical `getSource*` methods into one private helper | Remove 600 lines of duplication |
| 4 | Return `*ADTClientImpl` from constructors, not `ADTClient` | Stop hiding public API, fix interface inversion |
| 5 | Replace `log.Fatal` and `fmt.Println` in library code | Correctness, embeddability |

---

## Detailed Findings

---

### Finding 1 — God Interface `ADTClient`

**Severity: Critical**
**File: `types/adt.go:148-199`**

The interface has 30+ methods spanning object retrieval, creation, session management, LSP features, and testing. This is the defining architectural problem of the SDK.

**Why it matters in Go:** Go interfaces work because they're small. `io.Reader` has one method. `http.RoundTripper` has one. A 30-method interface cannot be satisfied by a fake in tests, cannot be partially implemented, and cannot evolve independently. Every consumer — the REST server, the LSP backend, the lib wrapper — must depend on the entire surface, even though each uses ~5 methods.

**Concrete symptom today:** `internal/lsp/backend/adt_backend.go` wraps `ADTClient` and only uses 5 methods (`SyntaxCheck`, `GetCompletionProposals`, `GetNavigationTarget`, `ActivateObject`, `IsAuthenticated`). But to satisfy the interface, any fake for testing must implement all 30.

**Recommended decomposition:**

```go
// SourceReader retrieves ABAP object source (read path).
type SourceReader interface {
    GetSource(ctx context.Context, objectType, objectName string) (*SourceCode, error)
    GetFunction(ctx context.Context, functionName, functionGroup string) (*SourceCode, error)
}

// SourceWriter creates/updates ABAP objects (write path).
type SourceWriter interface {
    CreateObject(ctx context.Context, opts CreateOptions) error
    UpdateSource(ctx context.Context, objectType, objectName, source string) error
}

// PackageBrowser searches packages and objects.
type PackageBrowser interface {
    SearchObjects(ctx context.Context, pattern string, types []string) ([]ADTObject, error)
    ListPackages(ctx context.Context, pattern string) ([]ADTPackage, error)
    GetPackageContents(ctx context.Context, name string) (*ADTPackage, error)
}

// ObjectActivator activates and tests objects.
type ObjectActivator interface {
    ActivateObject(ctx context.Context, objectType, objectName string) (*ActivationResult, error)
    RunUnitTests(ctx context.Context, objectType, objectName string) (*UnitTestResult, error)
}

// LangFeatures provides LSP-style language intelligence.
type LangFeatures interface {
    SyntaxCheck(ctx context.Context, objectType, objectName, source string) (*SyntaxCheckResult, error)
    Complete(ctx context.Context, objectType, objectName, source string, line, col int) ([]CompletionProposal, error)
    Navigate(ctx context.Context, objectType, objectName, source string, line, col int) (*NavigationTarget, error)
}
```

The concrete `ADTClientImpl` implements all of them. Consumers declare only what they use:

```go
// ADTBackend only needs this:
type ADTBackend struct {
    lang LangFeatures
    act  ObjectActivator
}
```

---

### Finding 2 — No `context.Context` in ADT Client

**Severity: Critical**
**File: `internal/adt/client.go` (every method)**

Every `http.NewRequest` call in the 3400-line file uses the non-context version:

```go
// Current — cannot be cancelled
req, err := http.NewRequest("GET", url, nil)
```

This means: no deadline propagation, no graceful shutdown, no tracing integration, and the LSP server (which correctly uses `*glsp.Context`) cannot pass cancellation into ADT operations.

**Fix:** Every exported and unexported method must accept and propagate `context.Context`:

```go
func (c *ADTClientImpl) getSource(ctx context.Context, endpoint string, name string) (*SourceCode, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
    ...
}
```

The `testConnectivity` method even creates a *new* `http.Client` just to set a shorter timeout (line 1422) — the correct pattern is `context.WithTimeout`.

---

### Finding 3 — 8 Identical Source-Retrieval Methods

**Severity: High**
**File: `internal/adt/client.go:186-623`**

`GetProgram`, `GetClass`, `GetInclude`, `GetInterface`, `GetStructure`, `GetTable`, `GetFunctionGroup` are byte-for-byte duplicates differing only in URL and the `ObjectType` string in the result. Each is ~45 lines. That's 315 lines of copy-paste where 20 lines of a helper would do.

**Current pattern (repeated 8×):**
```go
func (c *ADTClientImpl) GetProgram(programName string) (*types.ADTSourceCode, error) {
    if !c.IsAuthenticated() { return nil, fmt.Errorf("...") }
    programName = strings.ToUpper(strings.TrimSpace(programName))
    url := fmt.Sprintf("%s/programs/programs/%s/source/main", c.baseURL, programName)
    req, err := http.NewRequest("GET", url, nil)
    // ... same 40 lines
}
```

**Fix:**
```go
func (c *ADTClientImpl) getSource(ctx context.Context, objectType, name, endpoint string) (*types.ADTSourceCode, error) {
    if !c.IsAuthenticated() {
        return nil, fmt.Errorf("not authenticated")
    }
    name = strings.ToUpper(strings.TrimSpace(name))
    req, err := http.NewRequestWithContext(ctx, http.MethodGet,
        fmt.Sprintf("%s%s", c.baseURL, fmt.Sprintf(endpoint, name)), nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    c.addAuthHeaders(req)
    req.Header.Set("Accept", "text/plain")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request: %w", err)
    }
    defer resp.Body.Close()

    if err := checkStatus(resp, name); err != nil {
        return nil, err
    }
    src, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read body: %w", err)
    }
    return &types.ADTSourceCode{
        ObjectName: name,
        ObjectType: objectType,
        Source:     string(src),
        ETag:       resp.Header.Get("ETag"),
    }, nil
}

func (c *ADTClientImpl) GetProgram(ctx context.Context, name string) (*types.ADTSourceCode, error) {
    return c.getSource(ctx, "PROG", name, ADT_PROGRAMS_ENDPOINT)
}
```

The same unification applies to the lock/unlock/setSource cluster (`lockProgram`, `lockClass`, `lockObject`, `lockSource` — four lock implementations for the same operation).

---

### Finding 4 — `log.Fatal` Inside Library Code

**Severity: Critical**
**File: `internal/adt/client.go:732`**

```go
var result types.ADTSearchResult
if err := xml.Unmarshal(responseBody, &result); err != nil {
    log.Fatal(err)  // KILLS THE PROCESS
}
```

`log.Fatal` calls `os.Exit(1)`. Inside a library function (`SearchObjects`), this silently terminates any host program — an LSP server, a REST server, anything embedding this SDK. The fix is trivial:

```go
if err := xml.Unmarshal(responseBody, &result); err != nil {
    return nil, fmt.Errorf("parse search response: %w", err)
}
```

Also on line 2392:
```go
fmt.Println("responseBody", string(responseBody))  // debug print left in production code
```

---

### Finding 5 — Constructor Returns Interface (Inverted)

**Severity: High**
**File: `internal/adt/client.go:53`, `lib/wrapper.go:14`**

```go
func NewADTClient(config *types.ADTConfig) types.ADTClient {
```

Go convention: **accept interfaces, return concrete types.** Returning an interface from a constructor hides the concrete type, preventing callers from accessing methods not in the interface (`CreateProgramWithSource`, `CreateProgramWithOptions`, `CreateClassWithSource`, `CreateClassWithOptions`) without an unsafe type assertion.

The fix:
```go
func NewADTClient(config *ADTConfig) *ADTClientImpl {
```

Callers who want the interface assign it: `var client types.SourceReader = NewADTClient(cfg)`. The concrete type satisfies each interface automatically.

---

### Finding 6 — Global HTTP Mux in REST Server

**Severity: High**
**File: `rest/server/server.go:47-76`**

```go
http.HandleFunc("/api/v1/objects/get", rs.corsHandler(rs.getObjectHandler))
// ... 16 more registrations on the global mux
http.ListenAndServe(":"+port, nil)
```

Using `http.HandleFunc` (global default mux) means: only one `RestServer` can exist per process, routes from different tests/instances collide, and the server is untestable with `httptest.NewServer`. The idiomatic pattern:

```go
func (rs *RestServer) Handler() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/objects/get", rs.corsHandler(rs.getObjectHandler))
    // ...
    return mux
}

func (rs *RestServer) Start(port string) error {
    return http.ListenAndServe(":"+port, rs.Handler())
}
```

---

### Finding 7 — `HybridBackend` Goroutine Lifecycle Leak

**Severity: High**
**File: `internal/lsp/backend/hybrid.go:22-31`**

```go
func NewHybridBackend(client types.ADTClient, workDir string) *HybridBackend {
    h := &HybridBackend{...}
    go h.connectionMonitor()  // goroutine started, not tied to any lifetime
    return h
}
```

`Stop()` exists but is not on the `LSPBackend` interface, so callers holding `LSPBackend` cannot stop the goroutine. The `lsp.Server` stores `backend backend.LSPBackend` (the interface), calls no lifecycle methods, and the monitor runs forever.

**Fix:** Accept a `context.Context`:

```go
func NewHybridBackend(ctx context.Context, client SourceReader, workDir string) *HybridBackend {
    h := &HybridBackend{...}
    go func() {
        h.connectionMonitor(ctx)
    }()
    return h
}

func (h *HybridBackend) connectionMonitor(ctx context.Context) {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            ...
        }
    }
}
```

---

### Finding 8 — Error Control Flow via String Matching

**Severity: High**
**File: `internal/adt/client.go:2037`**

```go
func (c *ADTClientImpl) CheckObjectExists(objectType, objectName string) (bool, error) {
    _, err := c.GetObjectSource(objectType, objectName)
    if err != nil {
        if strings.Contains(err.Error(), "not found") {  // fragile!
            return false, nil
        }
        return false, err
    }
    return true, nil
}
```

Relying on error message text is the most fragile form of error handling. The Go pattern is sentinel errors:

```go
var ErrNotFound = errors.New("object not found")

// In getSource:
if resp.StatusCode == http.StatusNotFound {
    return nil, fmt.Errorf("%w: %s %s", ErrNotFound, objectType, name)
}

// In CheckObjectExists:
if errors.Is(err, ErrNotFound) {
    return false, nil
}
```

---

### Finding 9 — `ADTConfig` Uses `int` for Durations

**Severity: Medium**
**File: `types/adt.go:57-67`**

```go
type ADTConfig struct {
    ConnectTimeout int `json:"connect_timeout"`
    RequestTimeout int `json:"request_timeout"`
}
```

The unit (seconds) is implicit. Go has `time.Duration` exactly for this. Callers must know the undocumented unit. Fix:

```go
type ADTConfig struct {
    ConnectTimeout time.Duration `json:"connect_timeout"`
    RequestTimeout time.Duration `json:"request_timeout"`
}
```

---

### Finding 10 — Dead Code Mixed with Live Code

**Severity: Medium**
**File: `internal/adt/client.go`**

The file contains multiple generations of the same feature, creating confusion about which path is correct:

- `insertProgramSource` (line 2149) — marked `// DEPRECATED: use setProgramSource`
- `lockProgram` / `unlockProgram` / `updateProgramSource` (lines 2179–2338) — old object-specific locking
- `lockObject` / `unlockObject` / `setObjectSource` (lines 1852–1968) — new generic locking
- `setSourceUsingWorkingPattern` vs `setProgramSource` — two competing implementations for the same operation

Dead code should be deleted. It confuses readers, hinders maintenance, and suggests there's no single correct implementation path.

---

### Finding 11 — Manual XML Construction via `fmt.Sprintf`

**Severity: Medium**
**File: `internal/adt/client.go:1094-1106`, `1715-1727`**

```go
xmlPayload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<class:abapClass xmlns:class="http://www.sap.com/adt/oo/classes"
                 adtcore:description="%s"
                 adtcore:name="%s"
    `, escapeXML(description), name, ...)
```

This requires a hand-rolled `escapeXML` function when `encoding/xml` provides this. Manual XML formatting is fragile — `escapeXML` doesn't handle all edge cases (e.g., `\r`, `\n` in attribute values). Use struct marshalling instead of string formatting.

---

### Finding 12 — SCREAMING_SNAKE_CASE Constants

**Severity: Low**
**File: `internal/adt/client.go:21-38`**

```go
const (
    ADT_PROGRAMS_ENDPOINT = "/programs/programs/%s/source/main"
    ADT_CLASSES_ENDPOINT  = "/oo/classes/%s/source/main"
)
```

Go convention is `camelCase` for unexported identifiers:

```go
const (
    programsEndpoint = "/programs/programs/%s/source/main"
    classesEndpoint  = "/oo/classes/%s/source/main"
)
```

---

### Finding 13 — `lib` Package Provides No Value

**Severity: Low**
**File: `lib/wrapper.go`**

```go
func NewADTClient(config *types.ADTConfig) types.ADTClient {
    return adt.NewADTClient(config)  // pure passthrough
}
```

This re-exports `internal/adt` through a thin wrapper. If `adt` should be importable externally, move it out of `internal/`. If it shouldn't, the `lib` re-export defeats the purpose of `internal/`. Pick one.

---

### Finding 14 — `APIResponse` Has Two Definitions

**Severity: Low**
**File: `internal/client/abaper_api.go:17`, `rest/models/api.go`**

There are two separate `APIResponse` structs — the generic one in `internal/client` and the non-generic one in `rest/models`. They represent the same wire format. Unify them.

---

## Interface Refactor Opportunities

### `ADTClient` — Break into 5 Capability Interfaces

See Finding 1. Net effect: the LSP backend depends on a 2-method interface instead of a 30-method one. Testing becomes trivial with a 3-method fake rather than a 30-method stub.

### `LSPBackend` — Remove `IsConnected()`

`IsConnected()` leaks HybridBackend's fallback strategy into the interface. The callers use it only for diagnostic source labels, not behavioral branching. Remove it from the interface; internalize it to the `HybridBackend`.

### `RestServer` — Accept Typed Interfaces Per Handler

```go
type RestServer struct {
    reader   SourceReader
    writer   SourceWriter
    searcher PackageBrowser
}
```

Instead of depending on the full `ADTClient`, each handler group uses only the capability it needs.

### `addAuthHeaders` — Become an `http.RoundTripper`

Auth headers, CSRF tokens, and SAP session headers are applied by `addAuthHeaders` at each call site. These belong in a `RoundTripper` decorator that applies them transparently:

```go
type adtTransport struct {
    base      http.RoundTripper
    creds     basicAuth
    csrfToken func() string
    client    string
    language  string
}

func (t *adtTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req = req.Clone(req.Context())
    req.SetBasicAuth(t.creds.user, t.creds.pass)
    req.Header.Set("X-CSRF-Token", t.csrfToken())
    req.Header.Set("sap-client", t.client)
    req.Header.Set("sap-language", t.language)
    return t.base.RoundTrip(req)
}
```

---

## Stdlib Alignment Score

| Standard | Score | Notes |
|----------|-------|-------|
| `io` | 3/10 | No `io.Reader`/`Writer` composition. Source retrieval returns `string` instead of providing a reader. |
| `net/http` | 3/10 | No `context.Context`, global mux, `http.NewRequest` (no context), no `RoundTripper` for auth headers. |
| `context` | 1/10 | Entirely absent from the ADT client. |
| `database/sql` | 6/10 | The `ADTConfig` + constructor pattern loosely resembles `sql.Open`, but returning an interface breaks the analogy. |
| `slog` | 5/10 | Mixed: `zap` is used internally (fine), but `log.Fatal` and `log.Println` coexist with structured logging. |

---

## "What Would the Go Team Change?"

**1. The interface would be deleted.** The Go team's first question on seeing a 30-method interface is "who implements this?" If the answer is "one struct," the interface is probably wrong. They would define interfaces at the call sites.

**2. `context.Context` would be parameter one on every public method.** There are no exceptions in Go stdlib for network operations. The Go team would treat the absence of context as a blocking issue.

**3. The 3400-line file would be split.** Standard library files are rarely more than 500 lines. They would separate authentication, source CRUD, and package operations into different files within the same package, with a shared private `do(ctx context.Context, req *http.Request) (*http.Response, error)` helper.

**4. The lock/unlock pattern would become an `http.RoundTripper`.** Rather than calling `lockObject` / `unlockObject` manually in every write method, a `LockingTransport` wrapping `http.RoundTripper` could handle CSRF tokens and session type headers transparently.

**5. `CreateADTClient` that authenticates in the constructor would be removed.** Go constructors don't perform I/O. The function would return `*ADTClient, error` where the error is only configuration validation, and `Authenticate(ctx)` would be a separate explicit call.

**6. `ETag` and `Version` duplication would be eliminated.** `ADTSourceCode.Version` and `ADTSourceCode.ETag` are always set to the same value. Keep one.

**7. Zero tests would block any PR.** The `TestConnection` → `Authenticate` → `getCSRFToken` → `validateSession` four-hop handshake, the lock/unlock/activate sequence, and the XML parsing fallback chain all demand `httptest.NewServer`-based tests.
