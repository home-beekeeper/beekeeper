---
phase: 02-policy-engine-multi-source-catalogs
plan: 07
subsystem: catalog
tags: [go, watch-daemon, sanity-bounds, degraded-mode, state-persistence, ticker, injectable-snapshot]

# Dependency graph
requires:
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 04
    provides: OSV adapter (osv.go) — exists before Watch is written (sequencing only; Watch does not poll OSV)
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 05
    provides: Socket adapter (socket.go) — exists before Watch is written (sequencing only; Watch does not poll Socket)
  - phase: 01-foundation
    plan: 02
    provides: mmap Index (index.go) with Count() method; writeFileAtomic; catalog directory layout
provides:
  - "CheckSanity: pure delta/total bounds validator (no I/O); alert >1000, hard-block >10000, total >100000"
  - "WatchState + SourceState: per-source hash/count/degraded state persisted to state.json atomically"
  - "CatalogDelta: before/after snapshot type with HasChanges() via hash comparison"
  - "Watch: ticker-based daemon loop (5m–24h interval) with injectable SnapshotFunc for testability"
  - "computeDelta: delta detection + sanity gating + state persistence per tick"
  - "readBumblebeeSnapshot: production Snapshot implementation (reads bumblebee.json + .idx count)"
affects:
  - phase: 02-policy-engine-multi-source-catalogs plan 08 (catalogs watch CLI wiring + catalogs verify)
  - internal/catalog consumers that need to check SourceState.Degraded for corroboration weighting

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Injectable SnapshotFunc: testable without network by injecting a fake count/hash function"
    - "minPollInterval override field: unexported WatchConfig field allows tests to bypass 5m production floor"
    - "Ticker-loop with ctx.Done: mirrors tailAuditLog pattern from cmd/beekeeper/main.go"
    - "Missing-file-is-OK LoadState: returns empty WatchState{Sources: map} on first run (mirrors config.Load)"
    - "Degradation preservation: once set, Degraded=true persists across clean ticks until explicit clear"

key-files:
  created:
    - internal/catalog/sanity.go
    - internal/catalog/sanity_test.go
    - internal/catalog/state.go
    - internal/catalog/state_test.go
    - internal/catalog/watch.go
    - internal/catalog/watch_test.go
  modified: []

key-decisions:
  - "WatchConfig.Snapshot is injectable (not just SnapshotFunc as a global): production sets readBumblebeeSnapshot; tests inject fakes without any mock framework"
  - "minPollInterval unexported field in WatchConfig bypasses the 5m floor for tests; production callers never set it — avoids exportable test surface"
  - "Alert threshold also degrades the source (Degraded=true) not just emits a warning: a 1500-entry delta is suspicious enough to downgrade to warn-only corroboration weight"
  - "Degradation is sticky across ticks: clearing requires explicit operator action (catalogs verify --clear-degraded in Plan 08) — prevents self-healing after a poisoning event"
  - "Watch fires onDelta on Alert and Block even when HasChanges()==false: if the same hash triggers a sanity breach this tick, the caller must know immediately"
  - "TestWatchFiresOnDelta uses a buffered channel for synchronisation instead of ctx.Done() select: Windows goroutine scheduler does not reliably yield to ctx.Done() receives in the blocking-select pattern"

patterns-established:
  - "Ticker loop: time.NewTicker + defer Stop + select{ctx.Done/ticker.C} — same as tailAuditLog"
  - "State persistence: LoadState (missing→empty,nil) + SaveState (atomic via writeFileAtomic) — same idiom as config.Load"
  - "Sanity gating: CheckSanity called every tick; Block degrades; Alert degrades; clean tick preserves prior degradation"

requirements-completed: [CTLG-06, CTLG-08]

# Metrics
duration: 45min
completed: 2026-05-26
---

# Phase 2 Plan 07: Catalog Watch Daemon + Sanity Bounds Summary

**Ticker-based catalog watch daemon with injectable Snapshot seam, corroboration-degrading sanity bounds (alert >1000/block >10000 deltas), and atomic state.json persistence for cross-restart delta tracking**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-05-26T00:00:00Z
- **Completed:** 2026-05-26T00:45:00Z
- **Tasks:** 2
- **Files created:** 6

## Accomplishments

- `CheckSanity` is a pure function: no I/O, no goroutines; safe to call from hook handler or watch loop
- `WatchState` / `SourceState` persist hash+count+Degraded to `~/.beekeeper/state.json` atomically; first-run missing file returns empty state with nil error
- `Watch` mirrors the `tailAuditLog` ticker pattern; PollInterval clamped to [5m, 24h] in production; injectable `Snapshot` function makes the loop testable without network
- `computeDelta` runs `CheckSanity` on every tick; Block and Alert both set `Degraded=true`; degradation is sticky until explicit operator clear (Plan 08)
- 9 sanity tests, 8 state persistence tests, 9 Watch/computeDelta tests — all passing

## Task Commits

1. **Task 1: Pure sanity-bounds check + state.json persistence** - `0e347fb` (feat)
2. **Task 2: Watch daemon loop with delta detection + sanity gating** - `d0b12da` (feat)

## Files Created/Modified

- `internal/catalog/sanity.go` — `SanityConfig`, `DefaultSanityConfig()`, `SanityResult`, `CheckSanity(prevCount, newCount int, cfg SanityConfig) SanityResult` (pure function, no I/O)
- `internal/catalog/sanity_test.go` — 9 tests: within-bounds, alert delta, negative delta alert, block delta, total alert, block priority, zero-to-zero, custom config, default config values
- `internal/catalog/state.go` — `SourceState` (hash/count/degraded/reason), `WatchState` (map[string]SourceState), `LoadState`, `SaveState` (atomic), `CatalogDelta`, `HasChanges()`
- `internal/catalog/state_test.go` — 8 tests: missing file, save/load round-trip, degraded source round-trip, parent dir creation, nil Sources repair, CatalogDelta.HasChanges table test
- `internal/catalog/watch.go` — `SnapshotFunc`, `WatchConfig` (with injectable Snapshot + minPollInterval), `clampInterval`, `readBumblebeeSnapshot`, `computeDelta`, `Watch`
- `internal/catalog/watch_test.go` — 9 tests: computeDelta detects change/no-change/sanity-block/alert/degradation-preserved, Watch exits on cancel, fires on delta, clamp interval, first-run empty state

## Decisions Made

- **Injectable SnapshotFunc on WatchConfig** — The "current snapshot" is abstracted behind a function field rather than a global or method, keeping `computeDelta` pure from the network while still being testable. Production sets `readBumblebeeSnapshot(cfg.CatalogDir)` automatically when `Snapshot` is nil.
- **`minPollInterval` unexported field** — Tests need to bypass the 5m production floor without exposing a test-only API to callers. An unexported field on the struct is accessible within `package catalog` tests but invisible to external packages.
- **Alert also degrades** — The plan says "alert threshold → source warning-only". Warning-only in the corroboration model means `Degraded=true` with half-weight. This is stricter than just logging a warning but is correct per the threat model (T-02-07-01).
- **Buffered channel for test synchronisation** — `TestWatchFiresOnDelta` uses `called <- struct{}{}` instead of waiting on `ctx.Done()`. On Windows with GOMAXPROCS=2, blocking on `ctx.Done()` in the main goroutine can starve the Watch goroutine's ticker; a separate buffered channel avoids this scheduler quirk.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Poll interval not clamped for test WatchConfigs**
- **Found during:** Task 2 (TestWatchFiresOnDelta)
- **Issue:** `clampInterval` enforces a 5m minimum. Test configs pass 30ms intervals, which get clamped to 5m, causing tests to wait indefinitely (0 ticker fires in 2s).
- **Fix:** Added unexported `minPollInterval` field to `WatchConfig`; `clampInterval` takes it as parameter (zero = use production 5m floor). Tests set `minPollInterval: testPollInterval` via `testWatchConfig` helper. Production callers never set it.
- **Files modified:** `internal/catalog/watch.go`, `internal/catalog/watch_test.go`
- **Verification:** All 9 Watch/computeDelta tests pass with 30ms test intervals
- **Committed in:** `d0b12da` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug in test infrastructure caused by production guard misapplied to test context)
**Impact on plan:** Fix necessary for test correctness; no change to production behavior. The production 5m clamp is fully intact for callers that don't set `minPollInterval`.

## Issues Encountered

- **Windows goroutine scheduling** — `select { case <-ctx.Done(): }` in the test main goroutine does not reliably yield to a Watch goroutine's ticker on Windows with GOMAXPROCS=2. The pattern works on Linux/macOS. Fixed by using a separate buffered `called` channel for test synchronisation (documented in key-decisions above). This is a test-layer concern only — the production Watch loop is unaffected.

## Threat Surface Scan

| Flag | File | Description |
|------|------|-------------|
| None | — | No new network endpoints, auth paths, file access patterns, or schema changes beyond state.json (existing file under ~/.beekeeper, owner-only 0700) |

## Known Stubs

None — Watch does not render to UI and CatalogDelta/WatchState are fully wired data types.

## Self-Check

Files exist check:
- `internal/catalog/sanity.go` — FOUND
- `internal/catalog/sanity_test.go` — FOUND
- `internal/catalog/state.go` — FOUND
- `internal/catalog/state_test.go` — FOUND
- `internal/catalog/watch.go` — FOUND
- `internal/catalog/watch_test.go` — FOUND

Commits exist:
- `0e347fb` — Task 1: pure sanity-bounds check + state.json persistence
- `d0b12da` — Task 2: Watch daemon loop with delta detection and sanity gating

## Self-Check: PASSED

## Next Phase Readiness

- Plan 08 (`catalogs watch` + `catalogs verify` CLI wiring) can now consume `Watch`, `WatchState`, `SourceState`, and `CatalogDelta` directly
- `catalogs verify --clear-degraded` in Plan 08 should write `Degraded=false, DegradedReason=""` to the source's `SourceState` via `SaveState`
- The `onDelta` callback in Plan 08 should write a `catalog_delta` audit event and trigger a re-scan via `beekeeper check` on recently-installed packages

---
*Phase: 02-policy-engine-multi-source-catalogs*
*Completed: 2026-05-26*
