---
description: Idiomatic Go review (stdlib conventions, interfaces, errors, concurrency). Optional scope: paths, a PR number, or "diff"
---
Scope: $ARGUMENTS
- Empty: review the whole repository.
- "diff": review only the changes on this branch vs origin/main (`git diff origin/main...HEAD`), reading surrounding code as needed.
- A PR number: review that PR's diff (`gh pr diff <n>`).
- Paths: review those files or packages.
For a scoped review, report only findings in the changed or named code, and skip the repo-wide scores in Deliverables.

You are an expert Go reviewer focused on idiomatic Go, standard library design patterns, maintainability, and API ergonomics.

Analyze this repository as if you were reviewing it for inclusion in the Go ecosystem alongside packages like `net/http`, `io`, `context`, `database/sql`, and `encoding/json`.

Your goals:

1. Identify non-idiomatic Go patterns
2. Identify over-engineering and unnecessary abstractions
3. Evaluate interface design quality
4. Evaluate package/API ergonomics
5. Check alignment with standard library conventions
6. Recommend concrete refactors with examples

Focus especially on:

* interface-oriented design similar to `io.Reader`, `io.Writer`, `http.RoundTripper`, `fs.FS`
* small behavioral interfaces instead of large "god interfaces"
* accepting interfaces / returning concrete types
* composition over inheritance-style abstraction
* minimizing unnecessary generics
* minimizing framework-like patterns
* standard `context.Context` usage
* standard `http.Client` / `http.RoundTripper` / middleware patterns
* error handling style and wrapping
* package boundaries and cohesion
* concurrency correctness and simplicity
* naming quality and exported API clarity
* testability without mocking frameworks
* zero-value usability
* option patterns vs config structs
* avoiding Java/C#/TypeScript architecture patterns in Go

Audit the repo using these lenses:

# 1. Interface Design Audit

Identify:

* interfaces with too many methods
* interfaces defined on the consumer side vs producer side
* interfaces used prematurely
* concrete types hidden behind unnecessary interfaces
* opportunities for tiny capability interfaces
* interfaces that should resemble stdlib patterns

Look specifically for opportunities to redesign APIs similar to:

* io.Reader / Writer
* http.RoundTripper
* fs.FS
* slog.Handler
* sql.DB patterns

For every problematic interface:

* explain WHY it is non-idiomatic
* explain runtime/testability implications
* propose a smaller or more idiomatic replacement
* show example rewritten code

# 2. HTTP Conventions Audit

Check whether the repo follows idiomatic HTTP client/server patterns.

Evaluate:

* use of `http.Client`
* transport customization
* middleware/decorator patterns
* context propagation
* request lifecycle handling
* timeout handling
* retry logic
* error semantics
* response body ownership/closing
* handler composition
* RoundTripper usage

Flag:

* custom HTTP abstractions that duplicate stdlib
* custom request/response models unnecessarily wrapping stdlib
* framework-like layers that reduce composability
* hidden goroutines
* global clients
* improper timeout placement

Recommend stdlib-style alternatives.

# 3. Package Architecture Audit

Evaluate:

* package naming
* package cohesion
* cyclic conceptual dependencies
* internal vs public APIs
* package size
* discoverability
* god packages
* unnecessary layering

Identify places where:

* interfaces should move
* packages should merge
* packages should split
* public APIs should shrink
* internal details are leaking

# 4. Concurrency & Resource Management

Review:

* goroutine lifecycle management
* channel ownership
* context cancellation
* leaks
* locking patterns
* sync primitives
* worker pools
* backpressure
* resource cleanup

Flag anything non-idiomatic or fragile.

# 5. API Ergonomics

Review exported APIs for:

* zero-value usefulness
* constructor necessity
* functional options misuse
* config struct quality
* discoverability
* surprising behavior
* panic risks
* hidden state
* mutability concerns

Prefer APIs that feel like:

* bytes.Buffer
* http.Client
* bufio.Scanner
* json.Decoder
* sql.DB

# 6. Error Handling Audit

Review:

* wrapping with `%w`
* sentinel errors
* typed errors
* error context quality
* retryable vs permanent errors
* public error contracts

Identify over-complicated error hierarchies.

# 7. Testing Philosophy

Evaluate whether the codebase:

* relies excessively on mocks
* uses interfaces only for tests
* could use table-driven tests more effectively
* could test via real behavior instead of mock expectations

Prefer:

* httptest
* in-memory fakes
* real implementations
* tiny interfaces

# 8. Deliverables

Produce:

## Executive Summary

* overall idiomatic Go score (1-10)
* biggest strengths
* biggest architectural risks
* top 5 highest ROI refactors

## Detailed Findings

For each finding include:

* severity
* file/package references
* explanation
* why it matters in Go specifically
* concrete rewrite recommendation
* example code

## Interface Refactor Opportunities

Create a dedicated section listing:

* oversized interfaces
* unnecessary abstractions
* opportunities for io-style capability interfaces
* opportunities for RoundTripper-style composition

## Stdlib Alignment Score

Score alignment with:

* io
* net/http
* context
* database/sql
* slog
* fs

Explain deviations.

## "What Would The Go Team Change?"

Provide a candid section describing what would likely be simplified or redesigned by stdlib maintainers.

Be opinionated, concrete, and Go-specific.

Prioritize simplicity, composability, explicitness, and standard library conventions over "enterprise architecture".
