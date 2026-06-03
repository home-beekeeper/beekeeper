---
phase: 06-corroboration-severity-hardening
plan: "03"
subsystem: internal/check, internal/gateway, internal/watch, internal/scan
tags: [corroboration, sanity-gate, catalog-healthy, resolveCatalogHealthy, tdd, CORR-02, integration-tests]
dependency_graph:
  requires:
    - phase: 06-01
      provides: "CatalogHealthy bool on CorroborationThresholds; findSeverityOverride; DefaultCorroborationThresholds with SeverityOverrides"
    - phase: 06-02
      provides: "critical_block_at policy-file field; ThresholdsFromPolicyFiles CriticalBlockAt support"
  provides:
    - resolveCatalogHealthy(cacheDir) helper in each consumer package (check/gateway/watch/scan)
    - CatalogHealthy threaded at all four policy.Evaluate call sites
    - Integration tests proving wiring is live (not dormant): TestRunCheckAiFigureBlocks, TestRunCheckCriticalDegradedCatalogWarn, TestRunCheckCriticalBlockWithHealthyCatalog
  affects: [internal/check/handler.go, internal/gateway/policy.go, internal/watch/handler.go, internal/scan/scanner.go]
tech_stack:
  added: []
  patterns: [caller-resolved-IO, per-package-helper-no-shared-I/O, integration-test-through-RunCheck, fail-safe-default-true]
key_files:
  created:
    - internal/check/sanity.go
    - internal/gateway/sanity.go
    - internal/watch/sanity.go
    - internal/scan/sanity.go
  modified:
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/gateway/policy.go
    - internal/watch/handler.go
    - internal/scan/scanner.go
key_decisions:
  - "resolveCatalogHealthy copies to each consumer package (check/gateway/watch/scan) — no shared internal/sanity or internal/catalog import added to policy; no import cycle"
  - "buildCriticalTestIndex uses CatalogSignature non-empty → Signed:true in adapter, enabling single-source block via SeverityOverrides[critical]{BlockAt:1}"
  - "RED state: TestRunCheckCriticalDegradedCatalogWarn FAILS before wiring because DefaultCorroborationThresholds CatalogHealthy:true ignores state.json"
  - "Integration tests drive full stdin → RunCheck → policy.Evaluate path (not just corroborate directly) — F1-class wiring gap cannot hide"
requirements-completed: [CORR-02]
duration: ~15min
completed: "2026-06-03"
---

# Phase 06 Plan 03: Catalog Sanity Wiring Across All Four Enforcement Consumers Summary

**resolveCatalogHealthy(cacheDir) threaded into check/gateway/watch/scan via per-package sanity.go; three RunCheck integration tests prove the escalation is live end-to-end (CORR-02 closed)**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-06-03T~20:00Z
- **Completed:** 2026-06-03T~20:15Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments

- Created `resolveCatalogHealthy(cacheDir)` helper in all four consumer packages (check, gateway, watch, scan), each reading `filepath.Dir(cacheDir)/state.json` via `catalog.LoadState` and returning `!src.Degraded` for bumblebee; defaults `true` on missing/unreadable file
- Wired `thresholds.CatalogHealthy = resolveCatalogHealthy(...)` between `ThresholdsFromPolicyFiles` and `policy.Evaluate` in all four call sites
- Three RunCheck integration tests prove the wiring is live: block on healthy, warn on degraded (RED confirmed before Task 2), block on explicit-healthy (round-trip)
- Full phase-gate suite green: `go test ./internal/policy/... ./internal/policyloader/... ./internal/check/... ./internal/gateway/... ./internal/watch/... ./internal/scan/...`

## Task Commits

1. **Task 1: RED — 3 failing RunCheck integration tests** - `faf0d0e` (test)
2. **Task 2: GREEN — resolveCatalogHealthy helper + check call-site wiring** - `e991ec6` (feat)
3. **Task 3: Wire CatalogHealthy into gateway/watch/scan** - `b14fc50` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `internal/check/sanity.go` — resolveCatalogHealthy for check package; reads state.json Degraded flag
- `internal/gateway/sanity.go` — resolveCatalogHealthy for gateway package (same body)
- `internal/watch/sanity.go` — resolveCatalogHealthy for watch package (same body)
- `internal/scan/sanity.go` — resolveCatalogHealthy for scan package (same body)
- `internal/check/handler.go` — one-line CatalogHealthy wiring between ThresholdsFromPolicyFiles and policy.Evaluate
- `internal/check/handler_test.go` — buildCriticalTestIndex helper + 3 new integration tests
- `internal/gateway/policy.go` — one-line CatalogHealthy wiring
- `internal/watch/handler.go` — one-line CatalogHealthy wiring
- `internal/scan/scanner.go` — one-line CatalogHealthy wiring

## Decisions Made

- `resolveCatalogHealthy` is duplicated per consumer package rather than extracted to a shared package. This avoids circular imports (internal/policy is pure; a shared internal/sanity would be a new package that either imports catalog creating a new dep or is trivially small). The duplication is intentional and documented.
- `buildCriticalTestIndex` creates a **signed** critical bumblebee entry (`CatalogSignature: "sha256:corr02-test-sig"`) so the adapter sets `Signed:true`, enabling `signedCount=1 >= effectiveBlockAt=1` → block. An unsigned entry would always warn regardless of CatalogHealthy wiring (signedCount=0 → never reaches block branch).
- RED state targets `TestRunCheckCriticalDegradedCatalogWarn`: this is the test that FAILS before wiring (asserts "warn", gets "block" because DefaultCorroborationThresholds CatalogHealthy:true ignores state.json). The other two tests happen to pass before wiring since no state.json and healthy state.json both result in CatalogHealthy:true.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None — all wiring is live. The `resolveCatalogHealthy` helpers read actual `catalog.LoadState` I/O; the integration tests drive the full RunCheck pipeline.

## Threat Model Coverage

All four STRIDE threats mitigated:
- T-06-09 (Tampering: default on missing state.json): defaults true (healthy) on absence/error — confirmed by TestRunCheckAiFigureBlocks (no state.json → block)
- T-06-10 (DoS: per-invocation state.json read): bounded to one small JSON file read per check; mirrored by existing policy-file read on same path
- T-06-11 (EoP: wiring dormant): TestRunCheckCriticalDegradedCatalogWarn drives full stdin→exit-code path, proving wiring is live; RED confirmed the dormant state
- T-06-12 (Tampering: import cycle): `go build ./...` clean; resolveCatalogHealthy in caller tier only; `TestCorroborationImportsArePure` in 06-01 remains passing

## TDD Gate Compliance

- RED gate: `test(06-03)` commit faf0d0e — TestRunCheckCriticalDegradedCatalogWarn FAIL confirmed (asserts warn, got block)
- GREEN gate: `feat(06-03)` commit e991ec6 — all 3 tests pass (plus selftest)
- TDD cycle compliant: RED → GREEN → remaining consumers (Task 3)

## Self-Check: PASSED

- `internal/check/sanity.go` — FOUND
- `internal/gateway/sanity.go` — FOUND
- `internal/watch/sanity.go` — FOUND
- `internal/scan/sanity.go` — FOUND
- `internal/check/handler.go` contains `resolveCatalogHealthy(cacheDir)` — VERIFIED
- `internal/gateway/policy.go` contains `resolveCatalogHealthy(cfg.CacheDir)` — VERIFIED
- `internal/watch/handler.go` contains `resolveCatalogHealthy(h.CacheDir)` — VERIFIED
- `internal/scan/scanner.go` contains `resolveCatalogHealthy(cfg.CacheDir)` — VERIFIED
- Commits faf0d0e, e991ec6, b14fc50 present in git log — VERIFIED
- `go build ./...` clean — VERIFIED
- Full phase-gate suite green — VERIFIED
