---
phase: 02-policy-engine-multi-source-catalogs
plan: "05"
subsystem: catalog-socket
tags: [socket, purl, bearer-auth, backoff, disk-cache, degraded-mode, import-cycle-fix]
dependency_graph:
  requires: [02-01]
  provides: [SocketAdapter.LookupAll, QuerySocket, catalog.Indexer, config.SocketAPIToken]
  affects: [internal/catalog, internal/config, internal/policy, internal/check]
tech_stack:
  added: []
  patterns: [bearer-token-auth, exponential-backoff, cache-first-24h-ttl, atomic-write, graceful-degrade]
key_files:
  created:
    - internal/catalog/socket.go
    - internal/catalog/socket_test.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/catalog/loader.go
    - internal/policy/types.go
    - internal/policy/engine_test.go
    - internal/check/handler.go
    - internal/check/handler_test.go
decisions:
  - "catalog.Indexer interface added to catalog/loader.go to break the catalog↔policy import cycle"
  - "policy.CatalogLookup removed from policy/types.go; check/handler.go now uses catalog.Indexer"
  - "dead fakeCatalog/nxConsoleEntry removed from policy/engine_test.go (never called; caused test-time cycle)"
  - "Token never written to cache, logs, or error messages (T-02-05-01)"
  - "Socket disabled gracefully (not degraded) when token is empty"
  - "degraded=true returned on 5xx or transport errors; degraded=false when token empty"
metrics:
  duration: "~18 min"
  completed: "2026-05-26T09:45:00Z"
  tasks_completed: 2
  files_modified: 7
  files_created: 2
---

# Phase 2 Plan 05: Socket PURL Catalog Adapter Summary

Socket PURL API adapter (CTLG-03) with Bearer token auth, 24h disk cache, exponential backoff on 429, and graceful degradation — the third independent signed corroboration source for PLCY-01.

## What Was Built

### `internal/config/config.go` — Socket token field

```go
type SocketConfig struct {
    APIToken string `json:"api_token"`
}

type Config struct {
    FailMode string       `json:"fail_mode"`
    Socket   SocketConfig `json:"socket"`
}

func (c Config) SocketAPIToken() string { return c.Socket.APIToken }
```

Absent or empty `api_token` → `SocketAPIToken()` returns `""` → Socket source disabled (not an error).
All existing fail_mode semantics and tests unchanged.

### `internal/catalog/socket.go` — Socket PURL adapter

Key public API:

```go
// QuerySocket queries the Socket PURL API for threat data.
// token == ""      → (nil, false, nil)   Socket disabled, not an error
// cache hit        → (entries, false, nil)
// 200 OK           → (entries, false, nil) + cache written atomically
// 429 (retried)    → backoff with Retry-After honored, up to 5 retries
// 429 exhausted    → (nil, true, err)      degraded=true
// 5xx / transport  → (nil, true, err)      degraded=true
func QuerySocket(ctx context.Context, client *http.Client, cacheDir, token, ecosystem, pkg, version string) ([]Entry, bool, error)

// SocketAdapter implements policy.MultiCatalogLookup (pre-resolution path for Plan 08).
type SocketAdapter struct {
    Client   *http.Client
    CacheDir string
    Token    string
    Ctx      context.Context
}

func (a SocketAdapter) LookupAll(ecosystem, pkg string) []policy.CatalogMatch
// → []policy.CatalogMatch with CatalogSource "socket", Signed true, CatalogVersion "socket-api"
// → nil on degradation or error (aggregator records degradation to audit)
```

### Degraded vs Disabled Distinction

| Condition | `degraded` | `err` | Meaning |
|-----------|-----------|-------|---------|
| `token == ""` | false | nil | Socket not configured; not an error |
| Unsupported ecosystem | false | nil | PURL type unmapped; not an error |
| Cache hit | false | nil | Fresh data served |
| 200 OK | false | nil | Success |
| 429 exhausted | true | non-nil | Rate limited; caller degrades this source |
| 5xx / transport | true | non-nil | Service unavailable; caller degrades this source |

### Backoff Strategy

- Cache-first: 24h TTL is the primary rate-limit defense (T-02-05-02)
- On HTTP 429: read `Retry-After` header (seconds); fallback to exponential backoff starting at 1s, doubling, capped at 60s
- Maximum 5 retries; context cancellation respected via `select` on `time.After`/`ctx.Done()`
- Tests use injectable `backoffBase` parameter (`querySocket` inner function) to avoid real sleeps

### Deprecation / Migration Documentation

`socket.go` contains prominent TODO block:

```
// TODO(2026-07-30): Migrate to POST https://api.socket.dev/v0/packages before
// removal.  The migration touches only this file — change socketPURLURL, the
// request body builder (purlFor → buildPackagesRequest), and the response
// parser (parseSocketResponse).
```

Literal strings "2026-07-30" and "/v0/packages" present in file per acceptance criteria.

### Cache Layout

```
<cacheDir>/socket-cache/<ecosystem>/<pkg>/<version>.json
```

Version `""` stored as `_any`. Directory created with `0o700` (owner-only). Cache writes use `writeFileAtomic` (temp + rename). Token is NEVER written to cache (T-02-05-01).

## Import Cycle Fix (Rule 1 — Blocking Bug)

**Problem:** `catalog` adapters need to return `policy.CatalogMatch`; the old `policy.CatalogLookup` interface returned `catalog.Entry` — creating a mutual import cycle.

**Fix applied:**
1. Added `catalog.Indexer` interface to `internal/catalog/loader.go` (canonical single-source lookup interface, identical to the old `policy.CatalogLookup` semantics)
2. Removed `policy.CatalogLookup` from `internal/policy/types.go` (and the `catalog` import)
3. Updated `internal/check/handler.go` to use `catalog.Indexer` (comment notes Plan 08 will update to `MultiCatalogLookup`)
4. Updated `internal/check/handler_test.go` to use `catalog.Indexer` (removed unused `policy` import)
5. Removed dead `fakeCatalog`/`nxConsoleEntry` from `internal/policy/engine_test.go` (never called; caused test-time import cycle)

**Result:** `go build ./internal/catalog/... ./internal/policy/...` passes. `go test ./internal/catalog/... ./internal/policy/...` passes (all 23 policy tests + all 7 socket tests). The pre-existing transient break in `internal/check` (Plan 08 fix) is unchanged.

## Tests (9 new / 0 regressions)

| Test | Verifies |
|------|---------|
| TestSocketTokenLoads | `{"socket":{"api_token":"tok_abc"}}` → SocketAPIToken()=="tok_abc", FailMode defaults to "closed" |
| TestSocketTokenAbsentIsEmpty | Config with only fail_mode → SocketAPIToken()=="" |
| TestQuerySocketNoTokenDisabled | Empty token → (nil, false, nil), NO HTTP request made |
| TestQuerySocketParsesResponse | 200 OK → entries with CatalogSource "socket", Signed true |
| TestQuerySocketCacheHit | Second call with server closed → served from disk cache |
| TestQuerySocket429Backoff | 429→429→200 → success within retry budget, test <3s via injected backoff |
| TestQuerySocket5xxDegrades | 500 → degraded=true, non-nil err, nil entries, token not in error msg |
| TestSocketAdapterLookupAllMapsMatches | LookupAll → []policy.CatalogMatch with CatalogSource "socket", Signed true |
| TestQuerySocketCacheWriteIsAtomic | Cache dir created, cache file valid JSON with non-zero CachedAt |

## Threat Mitigations Implemented

| Threat ID | Mitigation |
|-----------|-----------|
| T-02-05-01 | Bearer token only in request header; never written to cache, audit, or error messages. Verified: TestQuerySocket5xxDegrades checks token absence from error string |
| T-02-05-02 | Cache-first 24h TTL as primary defense; Retry-After honored; exponential backoff capped at 5 retries; ctx cancellation respected |
| T-02-05-03 | 5xx/transport errors → degraded=true (warn-only for this source); check proceeds on remaining sources |
| T-02-05-04 | "2026-07-30" and "/v0/packages" migration TODO in socket.go; adapter structured for single-file swap |

## Commits

| Task | Type | Hash | Description |
|------|------|------|-------------|
| 1 | feat | 1e1a3c7 | add Socket API token to config (SocketConfig nested struct) |
| 2 | feat | cefd6a1 | Socket PURL adapter with Bearer auth, backoff, cache, graceful degrade |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Broke catalog↔policy import cycle**
- **Found during:** Task 2 (go build ./internal/catalog/... failed with import cycle)
- **Issue:** `catalog` adapters return `policy.CatalogMatch`; `policy.CatalogLookup` returned `catalog.Entry` → mutual import cycle
- **Fix:** Added `catalog.Indexer` in `catalog/loader.go`; removed `policy.CatalogLookup` + `catalog` import from `policy/types.go`; updated `check/handler.go` + `check/handler_test.go` to use `catalog.Indexer`
- **Files modified:** `internal/catalog/loader.go`, `internal/policy/types.go`, `internal/check/handler.go`, `internal/check/handler_test.go`
- **Commit:** cefd6a1

**2. [Rule 1 - Bug] Removed dead test helpers causing test-time import cycle**
- **Found during:** Task 2 (go test ./internal/policy/... failed with cycle in test)
- **Issue:** `fakeCatalog`/`nxConsoleEntry` in `engine_test.go` imported `catalog` but were never called; caused test-time cycle via `catalog` → `policy` → `catalog`
- **Fix:** Removed dead functions and the `catalog` import from `engine_test.go`
- **Files modified:** `internal/policy/engine_test.go`
- **Commit:** cefd6a1

## Known Stubs

None. `QuerySocket` is fully implemented. `SocketAdapter.LookupAll` is fully wired. The Plan 08 aggregator will combine Bumblebee + OSV + Socket into one `MultiCatalogLookup` — this is planned integration, not a stub.

## Threat Flags

None. No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries beyond what the plan's threat model covers.

## Self-Check: PASSED

- `internal/catalog/socket.go`: FOUND
- `internal/catalog/socket_test.go`: FOUND
- `internal/config/config.go` (modified): FOUND
- `internal/config/config_test.go` (modified): FOUND
- `internal/catalog/loader.go` (modified): FOUND
- `internal/policy/types.go` (modified): FOUND
- `internal/policy/engine_test.go` (modified): FOUND
- `internal/check/handler.go` (modified): FOUND
- `internal/check/handler_test.go` (modified): FOUND
- Commit 1e1a3c7: FOUND (config extension)
- Commit cefd6a1: FOUND (Socket adapter + cycle fix)
- `go test ./internal/config/... -count=1`: PASS (8/8)
- `go test ./internal/catalog/... -run TestQuerySocket -count=1`: PASS (7/7)
- `go test ./internal/policy/... -count=1`: PASS (23/23)
- `go vet ./internal/catalog/... ./internal/config/... ./internal/policy/...`: PASS
- `go build ./internal/catalog/... ./internal/config/... ./internal/policy/...`: PASS
- `socket.go` contains "2026-07-30": CONFIRMED
- `socket.go` contains "/v0/packages" migration TODO: CONFIRMED
- Bearer token not in error messages: CONFIRMED (TestQuerySocket5xxDegrades)
- Cache dir uses 0o700: CONFIRMED
