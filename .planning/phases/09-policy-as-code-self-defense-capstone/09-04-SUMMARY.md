---
phase: 09-policy-as-code-self-defense-capstone
plan: "04"
subsystem: check/llamafirewall
tags: [diag, latency, etw, catalog-freshness, build-tag-pair]
dependency_graph:
  requires: [09-03-PLAN]
  provides: [DiagReport, CollectDiag, LatencyTracker.P99, GlobalHookTracker, hook-latency-ring]
  affects: [internal/check, internal/llamafirewall]
tech_stack:
  added: []
  patterns: [build-tag-pair, persisted-ring-file, atomic-write, missing-file-is-ok]
key_files:
  created:
    - internal/check/diag.go
    - internal/check/diag_windows.go
    - internal/check/diag_other.go
    - internal/check/diag_test.go
    - internal/check/latency_persist.go
    - internal/check/latency_persist_test.go
  modified:
    - internal/llamafirewall/latency.go
    - internal/llamafirewall/latency_test.go
    - internal/check/handler.go
decisions:
  - GlobalLatencyTracker added to llamafirewall alongside existing tracker (package-level var, matches GlobalHookTracker symmetry)
  - hookLatencyFile placed under filepath.Dir(cacheDir) so ring is in ~/.beekeeper/ root regardless of catalog sub-dir layout
  - diag_other.go uses !windows (not linux && !darwin) to cover all non-Windows targets in one stub file
metrics:
  duration: ~20min
  completed: "2026-05-29"
  tasks_completed: 2
  files_created: 6
  files_modified: 3
---

# Phase 9 Plan 04: beekeeper diag — Data Assembly Layer Summary

**One-liner:** Hook latency p95/p99 + sidecar inference latency + per-source catalog freshness + ETW EventsLost assembled into DiagReport via persisted ring file and platform-dispatched build-tag pair.

## What Was Built

This plan implements the data-assembly layer for `beekeeper diag` (CODE-06, SWIN-04). The CLI command wiring and output formatting land in Plan 05; this plan provides everything that command will call.

### Task 1: LatencyTracker.P99() + GlobalHookTracker + Persisted Latency Ring

**Gap addressed:** `beekeeper check` is a one-shot process (one invocation per tool call), so in-memory `LatencyTracker` instances always reset to zero samples. Diag needs accumulated p95/p99 from real production check invocations.

**What was built:**

- `LatencyTracker.P99()` added to `internal/llamafirewall/latency.go` — identical structure to `P95()` but at the 0.99 percentile index. Reuses the existing `p95buf` ring buffer field; no new struct field.

- `GlobalLatencyTracker` package-level var added to `internal/llamafirewall/latency.go` for LlamaFirewall sidecar latency accumulation.

- `GlobalHookTracker` package-level `*llamafirewall.LatencyTracker` added to `internal/check/handler.go` for hook handler latency.

- `internal/check/latency_persist.go` — small persisted ring file (`~/.beekeeper/hook-latency.json`, last 100 samples):
  - `appendHookLatency(ringPath string, ms int64)` — load ring, append/rotate, write atomically.
  - `loadHookLatency(ringPath string) []int64` — read ring; missing or corrupt file returns nil (T-09-15).
  - `writeRingAtomic(path string, data []byte) error` — temp file + rename atomic write.

- `runCheck` in `handler.go` captures `start := time.Now()` at entry and has a deferred function that calls `GlobalHookTracker.Record(elapsedMS)` + `appendHookLatency(...)` just before returning. The deferred write is best-effort: a ring-write error is silently discarded and never alters the `Result` or the fail-closed decision (T-09-14 mitigation).

### Task 2: DiagReport + CollectDiag + ETW Build-Tag Pair

**Gap addressed:** Three data sources needed platform isolation (ETW is Windows-only) and a unified struct for the CLI formatter.

**What was built:**

- `internal/check/diag.go` — `DiagReport` struct, `CatalogSourceStatus` struct, and `CollectDiag(stateFile, hookLatencyRingPath string) DiagReport`:
  - Hook p95/p99: loads persisted ring → feeds temp `LatencyTracker` → P95()/P99().
  - Sidecar p95: reads `llamafirewall.GlobalLatencyTracker.P95()`.
  - Catalog freshness: `catalog.LoadState(stateFile).Sources` mapped to `[]CatalogSourceStatus` sorted alphabetically by name (deterministic output, includes `beekeeper-self` when present).
  - ETW events lost: dispatched to `eventsLost()`.

- `internal/check/diag_windows.go` (`//go:build windows`) — `eventsLost()` reads `atomic.LoadUint64(&winsentry.EventsLost)` (real ETW counter, SWIN-04).

- `internal/check/diag_other.go` (`//go:build !windows`) — `eventsLost()` returns 0 (ETW does not exist on Linux/macOS, T-09-17 mitigation).

Both files committed together as required by the build-tag-pair pattern.

## Test Results

| Test | Result |
|------|--------|
| `TestP99Empty` | PASS |
| `TestP99Single` | PASS |
| `TestP99GreaterOrEqualP95` | PASS |
| `TestP99NinetyNinthPercentile` | PASS |
| `TestAppendAndLoadHookLatency` | PASS |
| `TestHookLatencyRingRotation` | PASS |
| `TestLoadHookLatency_CorruptFile` | PASS |
| `TestGlobalHookTracker` | PASS |
| `TestAppendHookLatency_InvalidPath` | PASS |
| `TestEventsLost_NonWindows` | PASS |
| `TestCollectDiag` | PASS |
| `TestCollectDiag_MissingStateFile` | PASS |
| `TestCollectDiag_SortedSources` | PASS |
| All pre-existing check/llamafirewall tests | PASS |

**Build verification:**
- `go build ./...` — green
- `go build ./internal/check/...` — green
- `GOOS=windows go build ./internal/check/...` — green (build-tag pair compiles on both targets)
- `go vet ./internal/check/...` — clean

## Commits

| Hash | Task | Message |
|------|------|---------|
| `73d9ed9` | Task 1 | feat(09-04): add P99(), GlobalHookTracker, persisted hook latency ring |
| `e463f7c` | Task 2 | feat(09-04): DiagReport + CollectDiag + ETW build-tag pair (CODE-06, SWIN-04) |

## Deviations from Plan

**1. [Rule 2 - Missing Functionality] GlobalLatencyTracker added to llamafirewall**

The plan's `<interfaces>` block referenced `llamafirewall.GlobalLatencyTracker` as already existing (Phase 6 comment). Inspection showed it was absent. Added as a package-level `*LatencyTracker{}` var in `latency.go` alongside `GlobalHookTracker`. This is exactly the missing critical functionality that CollectDiag needs to read sidecar P95.

**2. [Rule 1 - Correctness] hookLatencyFile path derived from filepath.Dir(cacheDir)**

The plan says "under cacheDir/state dir". Actual cacheDir resolves to `~/.beekeeper/catalogs`. Using `filepath.Dir(cacheDir)` places the ring at `~/.beekeeper/hook-latency.json` — the beekeeper home root alongside `state.json`, not inside the catalogs sub-directory. This is more correct (ring is not catalog data) and consistent with `state.json` placement.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. Changes are limited to:
- Read-only access to `~/.beekeeper/state.json` (existing file, read-only in diag path).
- Read/write access to `~/.beekeeper/hook-latency.json` (new file, best-effort write from runCheck, read by diag).
- Atomic read of `windows.EventsLost` (existing var, atomic access — no race).

All surfaces are within the existing `~/.beekeeper/` trust boundary. No new trust boundary crossings.

## Self-Check: PASSED

- `internal/check/diag.go` — exists
- `internal/check/diag_windows.go` — exists
- `internal/check/diag_other.go` — exists
- `internal/check/latency_persist.go` — exists
- `internal/llamafirewall/latency.go` P99() — present (grep returns 1)
- Commits `73d9ed9` and `e463f7c` — present in git log
- `go build ./...` — green
- `go test ./internal/check/... ./internal/llamafirewall/... -count=1` — green
