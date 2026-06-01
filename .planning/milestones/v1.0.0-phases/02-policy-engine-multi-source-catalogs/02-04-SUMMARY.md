---
phase: 02-policy-engine-multi-source-catalogs
plan: "04"
subsystem: catalog-osv
tags: [osv, catalog, http-adapter, cache, corroboration, policy]
dependency_graph:
  requires: [02-01]
  provides: [OSVAdapter, QueryOSV, queryOSVWithURL, bumblebeeAdapter]
  affects: [internal/catalog, internal/policy, internal/check]
tech_stack:
  added: []
  patterns: [cache-first, atomic-write, injectable-url-for-testing, fail-safe-degradation]
key_files:
  created:
    - internal/catalog/osv.go
    - internal/catalog/osv_test.go
  modified:
    - internal/policy/types.go
    - internal/policy/engine.go
    - internal/policy/engine_test.go
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/check/selftest.go
decisions:
  - "queryOSV(url) internal core + queryOSVWithURL testable wrapper: injectable URL for httptest without exporting to production callers"
  - "OSVAdapter.baseURL field (unexported): test URL injection pattern without changing the public API"
  - "Remove CatalogLookup from policy/types.go: breaks catalog->policy->catalog import cycle; phase 2 callers use MultiCatalogLookup exclusively"
  - "bumblebeeAdapter.LookupAll returns per-version CatalogMatch entries: engine version filtering requires Version field populated"
  - "engine.go captures version from extract() (was discarded with _): version filtering applied after LookupAll to preserve phase 1 semantics"
  - "OSV API over TLS treated as signed source (Signed=true, CatalogVersion=osv-api): no separate signature field needed when responses come from the authoritative host over TLS"
metrics:
  duration: "~20 min"
  completed: "2026-05-26T09:46:20Z"
  tasks_completed: 2
  files_modified: 8
---

# Phase 2 Plan 04: OSV Catalog Adapter Summary

OSV REST API adapter (`internal/catalog/osv.go`) with 24h-TTL disk cache, cache-first lookup, atomic writes, and fail-safe degradation. OSVAdapter satisfies `policy.MultiCatalogLookup` as the second independent signed corroboration source alongside Bumblebee.

## What Was Built

**OSV ecosystem mapping (CASE-SENSITIVE, T-02-04-02):**
```go
func osvEcosystem(internal string) (string, bool)
// "npm"→"npm", "pypi"→"PyPI", "go"→"Go", "cargo"→"crates.io",
// "rubygems"→"RubyGems", "packagist"→"Packagist"
// unknown → ("", false) — caller returns (nil, nil), never queries with wrong name
```

**Cache layout:**
```
<cacheDir>/osv/<ecosystem>/<pkg>/<version>.json
// version="" → "_any.json" (avoids empty-filename collision)
```

**QueryOSV signature:**
```go
func QueryOSV(ctx context.Context, client *http.Client, cacheDir, ecosystem, pkg, version string) ([]Entry, error)
// 1. osvEcosystem() → unknown ecosystem: (nil, nil)
// 2. readOSVCache() → hit + age < 24h: return cached entries
// 3. POST https://api.osv.dev/v1/query, Content-Type: application/json
// 4. non-200 or transport error: (nil, err) — caller degrades, never fabricates allow
// 5. 200: io.LimitReader(4MB), unmarshal, convert to Entry, writeFileAtomic cache
```

**OSVAdapter for policy.MultiCatalogLookup:**
```go
type OSVAdapter struct {
    Client   *http.Client
    CacheDir string
    Ctx      context.Context
    baseURL  string // test injection; zero value = osvQueryURL
}
func (a *OSVAdapter) LookupAll(ecosystem, pkg string) []policy.CatalogMatch
// Returns []policy.CatalogMatch{CatalogSource:"osv", Signed:true, CatalogVersion:"osv-api"}
// On error: returns nil (degrade to no-match — Plan 08 aggregator logs degradation)
```

## Import Cycle Fix (Rule 1 — Blocking Issue)

`policy/types.go` had `import "catalog"` via the old Phase 1 `CatalogLookup` interface, causing a cycle when `catalog/osv.go` tried to import `policy`. Resolution:

1. **Removed `CatalogLookup` from `policy/types.go`**: Phase 1 compat interface that returned `catalog.Entry`; all Phase 2 callers use `MultiCatalogLookup` exclusively.
2. **Updated `handler.go`**: Replaced `policy.CatalogLookup` opener type with local `catalogIndex` interface (embeds `io.Closer + policy.MultiCatalogLookup`).
3. **Added `bumblebeeAdapter`** in `selftest.go`: Wraps `*catalog.Index` as `policy.MultiCatalogLookup + io.Closer` for the transitional period before Plan 08 wires the full aggregator.
4. **Removed dead code from `engine_test.go`**: `fakeCatalog`, `newFakeCatalog`, `nxConsoleEntry` were unused stubs importing `catalog`; removed to break the test import cycle.

## Engine Version Filtering Fix (Rule 1 — Bug)

With Phase 2's `LookupAll(ecosystem, pkg)` pattern, the engine originally discarded the extracted version (`ecosystem, pkg, _, ok`). `bumblebeeAdapter.LookupAll` returns one CatalogMatch per version listed in `Entry.Versions`. Without engine-level filtering, nrwl.angular-console@18.100.0 (remediated) matched against the @18.95.0 entry in the catalog.

**Fix**: Capture version in `extract()` call, filter `allMatches` by `m.Version`:
```go
if m.Version == "" || version == "" || m.Version == version {
    // include
}
```
This preserves Phase 1 version-matching semantics for the Bumblebee adapter.

## Threat Mitigations Implemented

| Threat ID | Mitigation |
|-----------|-----------|
| T-02-04-01 | `io.LimitReader(resp.Body, 4<<20)` — 4MB bound on OSV response |
| T-02-04-02 | `osvEcosystem()` explicit table; unknown → (nil,nil); never queries with wrong name |
| T-02-04-03 | Cache-first; ctx deadline propagates from hook handler; failure degrades source only |
| T-02-04-04 | Cache dir created with 0o700; documented as accepted (local attacker has broader compromise) |

## Tests (7/7 passing)

| Test | Verifies |
|------|---------|
| TestQueryOSVParsesVulns | POST with Content-Type JSON → Entry with CatalogSource "osv", severity "critical" |
| TestQueryOSVCacheHit | Second call with server torn down still returns cached result (cache-first proven) |
| TestQueryOSVUnmappedEcosystem | ecosystem "deb" → (nil, nil), server never called |
| TestQueryOSVNon200Degrades | HTTP 500 → non-nil error, nil entries (no fabricated allow) |
| TestOSVAdapterLookupAllMapsMatches | LookupAll → CatalogSource "osv", Signed true, CatalogVersion "osv-api" |
| TestQueryOSVEmptyResponse | Empty vulns → nil/empty entries, no error |
| TestOSVCacheExpiry | Cache entry >24h old → network call made (expiry enforced) |

## OSV API "Treated as Signed" Rationale

OSV API responses are assigned `Signed: true` because:
- All requests go to `api.osv.dev` over TLS with OS-level certificate validation
- OSV is the authoritative vulnerability database (Google OSS)
- No per-entry signature field is published; TLS transport provides the trust anchor
- This means ONE OSV match (Signed=true) + ONE Bumblebee match (Signed=false) = 1 signed source → "warn"
- TWO signed sources (OSV + another signed source) → "block" per PLCY-01

## Commits

| Task | Type | Hash | Description |
|------|------|------|-------------|
| 1 | feat | 28d0cd0 | OSV adapter types, ecosystem mapping, cache layout + import cycle fix |
| 2 | feat | f76d53a | OSV query with cache-first, atomic write, fail-safe degradation + tests |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Import Cycle] catalog→policy→catalog import cycle**
- **Found during:** Task 1 (go build)
- **Issue:** `policy/types.go` imported `catalog` via `CatalogLookup` interface returning `catalog.Entry`; `catalog/osv.go` needed to import `policy` for `MultiCatalogLookup` return type
- **Fix:** Removed `CatalogLookup` from `policy/types.go`; added `bumblebeeAdapter` in `check/selftest.go` as transitional bridge; updated `handler.go` to use local `catalogIndex` interface
- **Files modified:** `internal/policy/types.go`, `internal/check/handler.go`, `internal/check/handler_test.go`, `internal/check/selftest.go`, `internal/policy/engine_test.go`
- **Commits:** 28d0cd0, f76d53a

**2. [Rule 1 - Bug] Engine version filtering broken for bumblebeeAdapter**
- **Found during:** Task 2 (full test suite run — TestSelftestAllFixturesPass failed)
- **Issue:** `Evaluate()` discarded version (`_, ok`); `bumblebeeAdapter.LookupAll` returned all-version matches; remediated nrwl.angular-console@18.100.0 incorrectly matched @18.95.0 entry
- **Fix:** Capture version in `extract()` return; add post-LookupAll version filter in engine; update `bumblebeeAdapter.LookupAll` to return per-version `CatalogMatch` entries
- **Files modified:** `internal/policy/engine.go`, `internal/check/selftest.go`
- **Commits:** f76d53a

## Known Stubs

None. All OSV adapter logic is fully implemented and test-covered. The `bumblebeeAdapter` in `selftest.go` is a transitional bridge (not a stub) — it is the correct Bumblebee-only implementation for the period before Plan 08.

## Threat Flags

None. This plan operates entirely within `internal/catalog` and `internal/check/selftest.go`. No new network endpoints, auth paths, or schema changes at external trust boundaries.

## Self-Check: PASSED

- `internal/catalog/osv.go`: FOUND
- `internal/catalog/osv_test.go`: FOUND
- Commit 28d0cd0: FOUND
- Commit f76d53a: FOUND
- `go test ./internal/catalog/... -run 'TestQueryOSV|TestOSVAdapter' -count=1`: PASS (6/6)
- `go vet ./internal/catalog/...`: PASS
- `go build ./internal/catalog/...`: PASS
- `go test ./... -count=1`: PASS (all packages)
