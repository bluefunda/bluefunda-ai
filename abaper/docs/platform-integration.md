# ABAPer Platform Integration

## Role in the ABAPer Platform

ABAPer (Go) serves two roles in the ABAPer platform:

### 1. CLI Tool (standalone)

The primary use case — a POSIX-compliant command-line tool for ABAP development:

```bash
abaper get class ZCL_TEST
abaper search objects "Z*"
abaper connect
```

### 2. REST API Backend (server mode)

When running in server mode (`abaper server -p 8085`), it provides REST endpoints consumed by the abaper-editor frontend via abaper-gw.

**Currently handles**: GitHub integration endpoints

```
abaper-editor → abaper-gw → abaper (Go :8085)
  /api/v1/github/oauth/callback
  /api/v1/github/user
  /api/v1/github/branches
  /api/v1/github/tree
  /api/v1/github/file
```

## ABAPer vs ABAPer TS

The platform has two backend services that handle ADT operations:

| | ABAPer (Go) | ABAPer TS (TypeScript) |
|---|---|---|
| **Language** | Go | TypeScript (Express) |
| **ADT Library** | Custom Go implementation | `abap-adt-api` (npm) |
| **Primary role** | CLI tool + GitHub REST API | ADT proxy REST API |
| **Deployment** | Docker `ghcr.io/bluefunda/abaper:latest` on apps node | Docker `bdadevops/abaper-ts:latest` on apps node |
| **Port** | 8085 | 8087 |
| **Used by** | CLI users, abaper-editor (GitHub features) | abaper-editor (all ADT ops), abaper-mcp |

### Why Two Services?

ABAPer TS was introduced to leverage the mature `abap-adt-api` npm library by Marcel Schork, which provides comprehensive ADT support including features not yet available in the Go implementation (code completion, navigation, transport management, unit test execution).

The Go service continues to handle:
- CLI operations (the primary user-facing tool)
- GitHub integration (OAuth, file browsing)
- Features using Go's native ADT client

Over time, ADT operations are being consolidated in abaper-ts for consistency.

## Service Architecture

```
abaper-editor (browser)
    │
    └── /abaper/* → abaper-gw (KrakenD :8083)
                      │
                      ├── /api/v1/objects/*     → abaper-ts (:8087)
                      ├── /api/v1/activate      → abaper-ts (:8087)
                      ├── /api/v1/syntax-check  → abaper-ts (:8087)
                      ├── /api/v1/format        → abaper-ts (:8087)
                      ├── /api/v1/unit-tests    → abaper-ts (:8087)
                      ├── /api/v1/completion    → abaper-ts (:8087)
                      ├── /api/v1/navigation    → abaper-ts (:8087)
                      ├── /api/v1/transports/*  → abaper-ts (:8087)
                      ├── /api/v1/packages/*    → abaper-ts (:8087)
                      │
                      └── /api/v1/github/*      → abaper (Go :8085)
```

## Docker Deployment

```yaml
# apps node containers
abaper:      ghcr.io/bluefunda/abaper:latest  # Port 8085
abaper-ts:   bdadevops/abaper-ts:latest  # Port 8087
abaper-mcp:  bdadevops/abaper-mcp:latest # Port 8015
```

All three run on the `apps` node within the same Docker network (`app-services_trm-network`), communicating via container names.
