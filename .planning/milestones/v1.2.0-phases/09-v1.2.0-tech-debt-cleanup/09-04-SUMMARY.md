---
phase: 09-v1.2.0-tech-debt-cleanup
plan: "04"
subsystem: gateway/drift
tags: [drift, registry, nudge, fail-open, httptest]
requirements: [DRIFT-01]

dependency_graph:
  requires: []
  provides: [DRIFT-01]
  affects: [internal/gateway]

tech_stack:
  added: []
  patterns:
    - "npm dist-tags endpoint: GET /-/package/<pm>/dist-tags -> {latest: ...}"
    - "Per-PM fail-open: HTTP/parse error omits PM, returns nil error, continues"
    - "Package-level base URL var (npmDriftRegistryBase) overridable for httptest"

key_files:
  modified:
    - internal/gateway/drift.go
    - internal/gateway/drift_test.go

decisions:
  - "realMetadataFetch returns nil error even when individual PM fetches fail (per-PM fail-open contract); checkDrift handles partial maps gracefully — missing PM = no drift record for that PM only"
  - "npmDriftRegistryBase is its own var (not imported from internal/catalog) to keep packages decoupled; the two vars serve different consumers"
  - "5s HTTP client timeout (vs scheduler's 30s ctx) to avoid consuming the full check budget on a single slow PM"
  - "256KB io.LimitReader cap: dist-tags is ~100 bytes; cap defends against runaway/malicious response without allocating the full cap upfront (T-09-10)"

metrics:
  duration: "~10 minutes"
  completed: "2026-06-04"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 9 Plan 04: DRIFT-01 Live Version-Drift Registry Query Summary

**One-liner:** Real npm dist-tags HTTP fetch for pnpm/bun behind an overridable base-URL seam so the gateway weekly drift check emits live `version_drift` audit records.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Implement real npm dist-tags fetch in realMetadataFetch | e622676 | internal/gateway/drift.go |
| 2 | httptest-backed tests: parse, fail-open, end-to-end emit, floors unchanged | fe690b0 | internal/gateway/drift_test.go |

## What Was Built

**Task 1 — `internal/gateway/drift.go`**

- Added `var npmDriftRegistryBase = "https://registry.npmjs.org"` — a package-level var overridable in tests; deliberately separate from `internal/catalog`'s `npmRegistryBase` to keep packages decoupled.
- Added `driftDistTagsResponse` struct for `{"latest":"..."}` JSON decode.
- Replaced the empty-map stub `realMetadataFetch` with a real implementation:
  - Builds an `*http.Client` with a 5s timeout.
  - Iterates `{"pnpm", "bun"}`, GETs `<base>/-/package/<pm>/dist-tags`.
  - Decodes `{"latest":"..."}` via `io.LimitReader` (256KB cap) + `json.NewDecoder`.
  - Per-PM fail-open: non-200, network error, parse error, or empty `latest` → log to stderr, `continue`, omit that PM from the result map. Function returns `nil` error.
  - `metadataFetchFn` seam (`var metadataFetchFn = realMetadataFetch`) is unchanged.
- No new module dependency; `git diff go.mod go.sum` is empty.

**Task 2 — `internal/gateway/drift_test.go`**

Four new tests, all using `httptest.Server` + `npmDriftRegistryBase` override with defer-restore:

- `TestRealMetadataFetchParsesDistTags`: server returns `12.0.0`/`1.4.0`; asserts both parsed correctly.
- `TestRealMetadataFetchFailOpenOnError`: server returns HTTP 500 for pnpm, 200 for bun; asserts pnpm absent, bun present, nil error, no panic.
- `TestCheckDriftEndToEndRealFetch`: no `metadataFetchFn` override — calls `h.checkDrift` directly against httptest; asserts `record_type:"version_drift"` record emitted for pnpm (floor 11.0.0, server returns 12.0.0).
- `TestCheckDriftFloorsNeverBumped`: server returns `999.0.0`; asserts `h.cfg.Nudge.VersionFloors.Pnpm` is unchanged after `checkDrift` (PRD §7.1, T-09-12).

All pre-existing seam-level drift tests (`TestCheckDriftEmitsVersionDrift`, `TestCheckDriftNoDrift`, `TestCheckDriftFetchError`, `TestStartDriftSchedulerBoundedConcurrency`, `TestStartDriftSchedulerDisabled`) continue to pass.

## Verification Results

```
go test ./internal/gateway/ -run Drift -v
--- PASS: TestCheckDriftEmitsVersionDrift
--- PASS: TestCheckDriftNoDrift
--- PASS: TestStartDriftSchedulerBoundedConcurrency
--- PASS: TestStartDriftSchedulerDisabled
--- PASS: TestCheckDriftFetchError
--- PASS: TestRealMetadataFetchParsesDistTags
--- PASS: TestRealMetadataFetchFailOpenOnError
--- PASS: TestCheckDriftEndToEndRealFetch
--- PASS: TestCheckDriftFloorsNeverBumped
PASS ok github.com/home-beekeeper/beekeeper/internal/gateway

go build ./...    → clean
go vet ./internal/gateway/... → clean
git diff go.mod go.sum → empty (no new deps)
```

## Deviations from Plan

None — plan executed exactly as written.

The TDD order was adapted to the two-task split in the plan (Task 1 = implementation, Task 2 = httptest tests) rather than strict RED/GREEN/REFACTOR within a single task; this matches the plan's stated intent.

## Threat Model Coverage

All four STRIDE threats from the plan's `<threat_model>` are addressed:

| Threat ID | Status |
|-----------|--------|
| T-09-10 — DoS (slow/large response) | Mitigated: 5s client timeout + 256KB `io.LimitReader` + scheduler's existing 30s ctx + WR-04 one-in-flight guard |
| T-09-11 — SSRF | Mitigated: base URL is a hardcoded constant, never derived from agent input; test override is package-internal only |
| T-09-12 — Tampering (floor bump) | Mitigated: `emitVersionDrift` never mutates `VersionFloors`; `TestCheckDriftFloorsNeverBumped` asserts this |
| T-09-13 — DoS (fetch failure cascade) | Mitigated: per-PM fail-open omission + `checkDrift` fail-open on error; drift never blocks or gates a tool call |

## Known Stubs

None — `realMetadataFetch` is no longer a stub.

## Self-Check: PASSED

- `internal/gateway/drift.go` exists and compiles: FOUND
- `internal/gateway/drift_test.go` exists with new tests: FOUND
- Commit e622676 (Task 1): FOUND
- Commit fe690b0 (Task 2): FOUND
- `git diff go.mod go.sum` empty: VERIFIED
- `go build ./...` clean: VERIFIED
- `go test ./internal/gateway/ -run Drift` 9/9 PASS: VERIFIED
