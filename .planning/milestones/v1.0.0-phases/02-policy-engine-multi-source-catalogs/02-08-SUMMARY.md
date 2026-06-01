---
phase: 02-policy-engine-multi-source-catalogs
plan: "08"
subsystem: catalog-integration
tags: [go, multi-source, corroboration, audit-provenance, baseline, cli-wiring, ctlg-09, plcy-01]

# Dependency graph
requires:
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 01
    provides: MultiCatalogLookup interface + corroborate() engine
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 04
    provides: OSVAdapter.LookupAll
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 05
    provides: SocketAdapter.LookupAll
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 07
    provides: Watch, WatchState, SourceState, CatalogDelta, LoadState/SaveState
provides:
  - "MultiIndex: concrete policy.MultiCatalogLookup aggregating Bumblebee+OSV+Socket with nil-safe skipping"
  - "NewMultiIndex constructor: bumblebee *Index + osv + socket policy.MultiCatalogLookup"
  - "AuditRecord Phase 2 fields: CorroborationCount, SourcesAgreed, SourcesDissented, Quarantine"
  - "CatalogProvenance Phase 2 fields: Corroborated, Dissented, CatalogVersion"
  - "FromDecision extended: maps all corroboration Decision fields; catalog_matches always [] not null"
  - "internal/baseline Store: atomic Load/Save with owner-only 0600 perms (T-02-08-04)"
  - "RunCheck new signature: cacheDir param for OSV+Socket cache path"
  - "Hook handler wired to MultiIndex: Bumblebee + OSVAdapter + SocketAdapter share 5s ctx deadline"
  - "catalogs watch CLI: foreground daemon, SIGINT/SIGTERM, DefaultSanityConfig"
  - "catalogs verify --source: clears Degraded flag in state.json (CTLG-08)"
affects:
  - internal/check (RunCheck signature, full multi-source path)
  - internal/audit (CTLG-09 provenance shape)
  - cmd/beekeeper (check + catalogs subcommands)
  - all consumers of check.RunCheck (selftest, bench, main.go)

# Tech tracking
tech-stack:
  added:
    - internal/baseline (new package)
  patterns:
    - "bumblebeeMultiAdapter inside catalog package: wraps *Index as policy.MultiCatalogLookup without cross-package dep on selftest"
    - "MultiIndex.Close() delegates to Bumblebee.Close(): satisfies io.Closer for catalogIndex interface"
    - "nil-adapter skipping: nil OSV or Socket sub-adapters are silently skipped (no-match, never error)"
    - "writeBaselineAtomic: CreateTemp+Rename mirrors catalog/index.go writeFileAtomic"
    - "signal.NotifyContext in watch CLI: wraps cmd.Context() with SIGINT/SIGTERM cancellation"
    - "Test isolation via fictional package name: avoids live OSV hits on real packages like express during tests"

key-files:
  created:
    - internal/catalog/multi.go
    - internal/catalog/multi_test.go
    - internal/audit/types_test.go
    - internal/baseline/store.go
    - internal/baseline/store_test.go
  modified:
    - internal/audit/types.go
    - internal/audit/writer_test.go
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/check/handler_bench_test.go
    - internal/check/selftest.go
    - cmd/beekeeper/main.go

key-decisions:
  - "bumblebeeMultiAdapter inside multi.go: selftest.go has a similar bumblebeeAdapter; kept separate so catalog package does not import check, avoiding a new import cycle"
  - "OSVAdapter always wired (non-nil) in runCheck: OSV is enabled by default; Socket is nil when empty token"
  - "httpClient{Timeout: 4s} in runCheck: slightly below the 5s execTimeout to give OSV/Socket a chance to complete without running over the handler deadline"
  - "Test fictional package for allow assertions: express@4.18.2 has real OSV CVEs; tests use 'beekeeper-test-clean-package-xyz-not-real' to ensure stable allow decisions"
  - "TestAuditRecordJSONKeys updated to 14 keys: Phase 2 CTLG-09 adds corroboration_count, sources_agreed, sources_dissented"
  - "catalogs verify --source as required flag: avoids silent no-op if user omits it"

requirements-completed: [CTLG-09]

# Metrics
duration: ~60min
completed: 2026-05-26
---

# Phase 2 Plan 08: Integration — Multi-Source Corroboration Pipeline Summary

**MultiIndex aggregating Bumblebee+OSV+Socket wired into the hook handler, full CTLG-09 audit provenance on every record, owner-only baseline persistence, and catalogs watch/verify CLI completing the Plan 01 transient build break resolution.**

## Performance

- **Duration:** ~60 min
- **Started:** 2026-05-26T00:00:00Z
- **Completed:** 2026-05-26T01:00:00Z
- **Tasks:** 3
- **Files created:** 5
- **Files modified:** 7

## Accomplishments

### Task 1: Multi-source aggregator + audit provenance extension (CTLG-09)

- `internal/catalog/multi.go`: `MultiIndex` struct aggregating `*Index` (Bumblebee mmap), `policy.MultiCatalogLookup` (OSV), and `policy.MultiCatalogLookup` (Socket); nil sub-adapters silently skipped
- `bumblebeeMultiAdapter` inside `multi.go`: wraps `*Index` for `LookupAll` semantics without importing `check` package
- `MultiIndex.Close()`: delegates to `Bumblebee.Close()`, satisfying `io.Closer` for the `catalogIndex` interface
- Extended `internal/audit/types.go`:
  - `CatalogProvenance` gets `Corroborated`, `Dissented`, `CatalogVersion` (CTLG-09)
  - `AuditRecord` gets `CorroborationCount`, `SourcesAgreed`, `SourcesDissented`, `Quarantine`
  - `FromDecision` maps all Phase 2 Decision fields; `catalog_matches`, `sources_agreed`, `sources_dissented` always serialize as `[]` not `null`
- Tests: `TestMultiIndexAggregatesAllSources`, `TestMultiIndexSkipsNilSources`, `TestMultiIndexMissReturnsNil`, `TestMultiIndexNilBumblebeeSkipped`, `TestFromDecisionMapsProvenance`, `TestFromDecisionEmptyMatchesSerializesEmptyArray`, `TestFromDecisionQuarantineFlag`, `TestFromDecisionPhase1FieldsIntact`

### Task 2: Baseline store + hook handler rewire

- `internal/baseline/store.go`: `Store{path}` with `NewStore` (MkdirAll 0700), `Load` (missing → empty counters, non-nil map), `Save` (atomic via `writeBaselineAtomic` + `platform.SetOwnerOnly` → 0600)
- `internal/check/handler.go`:
  - `catalogOpener` now returns `*catalog.Index` (raw Bumblebee mmap); `defaultOpener` calls `catalog.OpenIndex`
  - `RunCheck` gains `cacheDir string` parameter
  - `runCheck` constructs `OSVAdapter{Client, CacheDir, Ctx}` always (OSV enabled by default)
  - `SocketAdapter` nil when `cfg.SocketAPIToken() == ""` (Socket disabled gracefully)
  - `catalog.NewMultiIndex(bbIdx, osvAdapter, socketAdapter)` aggregates all three
  - `policy.Evaluate(toolCall, multiIdx, policy.DefaultCorroborationThresholds())` — multi-source evaluation
  - All HTTP adapters share the handler `ctx` (5s deadline bounds their calls — T-02-08-01)
  - All Phase 1 fail-closed paths preserved (panic→block, timeout→block, oversized→block, missing-index→block)

### Task 3: CLI wiring

- `newCheckCmd`: passes `catalogDir` to `check.RunCheck` (new `cacheDir` param)
- `catalogs watch`: foreground daemon with `signal.NotifyContext` (SIGINT/SIGTERM), `DefaultSanityConfig()`, production `Snapshot` (nil → `readBumblebeeSnapshot`)
- `catalogs verify --source`: loads `state.json`, clears `Degraded=false, DegradedReason=""`, saves atomically
- `go build ./... && go vet ./... && go test ./...`: all pass — Plan 01 transient break resolved

## Task Commits

1. **Task 1: Multi-source aggregator + audit provenance extension** — `169b143` (feat)
2. **Task 2: Baseline store + rewire hook handler** — `6899819` (feat)
3. **Task 3: CLI wiring** — `f3ca38f` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestHookHandlerAllow used express@4.18.2 which has real OSV CVEs**
- **Found during:** Task 2 (first test suite run after wiring OSV adapter)
- **Issue:** `express` package has multiple real OSV vulnerabilities (MODERATE/LOW severity). Once the OSV adapter was wired, the "allow" test became "warn" because OSV returned real hits. The test expected "allow" based on Phase 1 Bumblebee-only evaluation.
- **Fix:** Changed all handler test calls that use real packages (`express@4.18.2`) to use a clearly fictional package name (`beekeeper-test-clean-package-xyz-not-real@1.0.0`) that will never appear in any real threat catalog.
- **Files modified:** `internal/check/handler_test.go`, `internal/check/handler_bench_test.go`
- **Committed in:** `6899819` (Task 2 commit)

**2. [Rule 1 - Bug] TestAuditRecordJSONKeys asserted exactly 11 JSON keys**
- **Found during:** Task 3 (full test suite run)
- **Issue:** Phase 1 test asserted exactly 11 JSON keys in `AuditRecord`. Phase 2 added 3 new mandatory fields (`corroboration_count`, `sources_agreed`, `sources_dissented`) → 14 keys total.
- **Fix:** Updated `wantKeys` from 11 to 14 (added the 3 CTLG-09 fields). The `quarantine` field uses `omitempty` so it does not appear when false, keeping count at 14 not 15.
- **Files modified:** `internal/audit/writer_test.go`
- **Committed in:** `f3ca38f` (Task 3 commit)

## CTLG-09 Compliance

Every `AuditRecord` JSON line now contains:
```json
{
  "catalog_matches": [],           // always present, never null
  "corroboration_count": 0,        // number of independent signed sources that agreed
  "sources_agreed": [],            // always present, never null
  "sources_dissented": [],         // always present, never null
  "quarantine": true               // only present when true (omitempty)
}
```

Per-match provenance in `catalog_matches`:
```json
{
  "catalog_source": "bumblebee",
  "catalog_version": "bumblebee",
  "corroborated": true,
  "dissented": false
}
```

## Threat Mitigations Implemented

| Threat ID | Mitigation |
|-----------|-----------|
| T-02-08-01 | OSV/Socket adapters share the handler `ctx`; slow sources cancelled at 5s deadline; degrade to no-match (nil LookupAll), never block the whole check |
| T-02-08-02 | `FromDecision` maps `corroboration_count + sources_agreed/dissented + per-match corroborated/dissented`; `catalog_matches` always `[]` (CTLG-09 closed) |
| T-02-08-03 | All Phase 1 fail-closed tests retained and pass; panic/timeout/oversized/missing-index paths unchanged in `handler.go` |
| T-02-08-04 | `baseline.Store.Save` calls `platform.SetOwnerOnly(s.path)` after atomic write; `TestSaveEnforcesOwnerOnly` asserts 0600 on Unix |

## Known Stubs

None. All functionality is fully implemented and test-covered.

## Threat Surface Scan

No new network endpoints, auth paths, or schema changes at external trust boundaries beyond what the plan's threat model covers. The `catalogs watch` and `catalogs verify` commands access the local `state.json` file (pre-existing, owner-only directory) and make no new external connections.

## Self-Check

Files exist check:
- `internal/catalog/multi.go` — FOUND
- `internal/catalog/multi_test.go` — FOUND
- `internal/audit/types_test.go` — FOUND
- `internal/baseline/store.go` — FOUND
- `internal/baseline/store_test.go` — FOUND
- `internal/audit/types.go` (modified) — FOUND
- `internal/audit/writer_test.go` (modified) — FOUND
- `internal/check/handler.go` (modified) — FOUND
- `cmd/beekeeper/main.go` (modified) — FOUND

Commits exist:
- `169b143` — Task 1: multi-source aggregator + audit provenance extension
- `6899819` — Task 2: baseline store + rewire hook handler
- `f3ca38f` — Task 3: CLI wiring

Build and test results:
- `go build ./...`: PASS
- `go vet ./...`: PASS
- `go test ./... -count=1`: PASS (7/7 packages)

## Self-Check: PASSED

---
*Phase: 02-policy-engine-multi-source-catalogs*
*Completed: 2026-05-26*
