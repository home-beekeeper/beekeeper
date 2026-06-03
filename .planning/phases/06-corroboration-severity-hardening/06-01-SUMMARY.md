---
phase: 06-corroboration-severity-hardening
plan: "01"
subsystem: internal/policy
tags: [corroboration, severity-escalation, sanity-gate, all-versions-guard, tdd, CORR-01, CORR-02]
dependency_graph:
  requires: []
  provides: [SeverityThreshold, SeverityOverrides, CatalogHealthy, findSeverityOverride, validateCorroborationThresholds-extended]
  affects: [internal/policy/types.go, internal/policy/corroboration.go, internal/check/selftest.go]
tech_stack:
  added: []
  patterns: [caller-resolved-IO, fail-closed, pure-function-library, all-versions-guard]
key_files:
  created: []
  modified:
    - internal/policy/types.go
    - internal/policy/corroboration.go
    - internal/policy/corroboration_test.go
    - internal/policy/engine_test.go
    - internal/check/selftest.go
decisions:
  - "CatalogHealthy defaults true — escalation is on by default; callers explicitly suppress on confirmed degradation (fail-safe for feature, not security fail-open)"
  - "findSeverityOverride placed inside corroboration.go; all-versions guard is first check after catalog-healthy gate — prevents wildcard-mis-tagged entries from triggering single-source block"
  - "validateCorroborationThresholds extended with per-severity bounds loop (BlockAt>=1, <=globalBlockAt, QuarantineAt>=BlockAt); fail-closed return 'block' on violation"
  - "engine_test.go TestEvaluateSingleSignedSourceWarns and TestSignedCatalogStillWarnWithSingleSource expectations updated to 'block' (correct Phase-6 behavior for critical + signed single source)"
  - "selftest fixtures stay at 'warn' — all selftestEntries have empty CatalogSignature (Signed:false) so signedCount=0 and hasSignedSource=false; critical escalation gate does not fire"
metrics:
  duration_seconds: 440
  completed_date: "2026-06-03T19:09:01Z"
  tasks_completed: 3
  files_changed: 5
---

# Phase 06 Plan 01: Corroboration Severity Escalation + Sanity Gate Summary

Implemented pure-logic core of corroboration severity escalation (CORR-01) with anti-poisoning sanity bounds (CORR-02) as a single atomic deliverable. A `critical`-severity catalog match escalates to block at a single trusted signed source via `SeverityOverrides["critical"]={BlockAt:1}`, gated by `CatalogHealthy`, guarded against all-versions wildcard entries, and bounds-checked so an override can never be `BlockAt < 1` or looser than the global threshold.

## What Was Built

### Task 1: RED (c564ad6)
Added 6 failing unit tests to `internal/policy/corroboration_test.go` covering the full escalation + sanity-bound contract. Tests referenced undefined symbols (`SeverityThreshold`, `SeverityOverrides`, `CatalogHealthy`) — compile failure confirmed RED state.

### Task 2: GREEN (6db63b1)

**`internal/policy/types.go`:**
- Added `SeverityThreshold` struct (fields: `BlockAt int`, `QuarantineAt int`) before `CorroborationThresholds`
- Extended `CorroborationThresholds` with `SeverityOverrides map[string]SeverityThreshold` and `CatalogHealthy bool`
- Updated `DefaultCorroborationThresholds()` to set `CatalogHealthy: true` and `SeverityOverrides: {"critical": {BlockAt:1, QuarantineAt:2}}`

**`internal/policy/corroboration.go`:**
- Extended `validateCorroborationThresholds` with per-severity bounds loop: rejects `BlockAt < 1`, `BlockAt > t.BlockAt`, `QuarantineAt < BlockAt`
- Added `findSeverityOverride` pure helper: returns nil when `!catalogHealthy`, when any non-dissented match has `Version=="*"`, or when no severity key matches overrides; otherwise returns most-restrictive override (lowest BlockAt)
- Replaced escalation table with `effectiveBlockAt`/`effectiveQuarantineAt` computed from override (falling back to global values)
- Imports remain exactly `"fmt"` and `"sort"` — purity constraint preserved

**`internal/policy/engine_test.go` (Rule 1 auto-fix):**
- `TestEvaluateSingleSignedSourceWarns`: updated expectation from `"warn"` to `"block"` — correct Phase-6 behavior for critical signed single source
- `TestSignedCatalogStillWarnWithSingleSource`: same update — critical + 1 signed → block under new defaults

### Task 3: Selftest Audit (b1ad691)

Audited `internal/check/selftest.go` and `corpus/fixtures.json` for critical-severity entries. All 3 `selftestEntries` carry `Severity: "critical"` but empty `CatalogSignature`. The `bumblebeeAdapter` sets `Signed: e.CatalogSignature != ""`, so all entries produce `Signed: false`. This means `signedCount=0` and `hasSignedSource=false`, so the critical escalation path (`signedCount >= effectiveBlockAt && hasSignedSource`) does not fire. All 10 fixtures correctly remain at their existing expectations — no changes required. Added an audit comment documenting the analysis.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 (RED) | c564ad6 | test(06-01): add failing corroboration severity escalation + sanity-bound tests |
| 2 (GREEN) | 6db63b1 | feat(06-01): per-severity corroboration escalation with sanity-gated all-versions guard (CORR-01/02) |
| 3 (Selftest) | b1ad691 | test(06-01): align selftest critical-severity expectations with Phase-6 escalation |

## Test Results

All 6 new tests pass:
- `TestCorroborationShaiHuludCriticalBlock` — PASS (SC1: critical single-signed-source blocks)
- `TestCorroborationDegradedCatalogNoEscalation` — PASS (SC2: degraded catalog suppresses escalation)
- `TestCorroborationAllVersionsCriticalWildcardStaysWarn` — PASS (SC3: wildcard requires 2 sources)
- `TestValidateCorroborationThresholdsRejectsBlockAtZero` — PASS (SC4: BlockAt<1 fail-closed)
- `TestValidateCorroborationThresholdsRejectsLooserOverride` — PASS
- `TestDefaultThresholdsIncludeSeverityOverrides` — PASS

Purity test: `TestCorroborationImportsArePure` — PASS (imports remain only `"fmt"` and `"sort"`)

Full suites: `go test ./internal/policy/...` — PASS, `go test ./internal/check/... -run TestSelftest` — PASS

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated engine_test.go expectations for critical single-source tests**
- **Found during:** Task 2 GREEN verification (`go test ./internal/policy/...`)
- **Issue:** `TestEvaluateSingleSignedSourceWarns` and `TestSignedCatalogStillWarnWithSingleSource` both use `nxConsoleMatch("bumblebee", true)` which has `Severity:"critical"` and `Signed:true`. With the new `DefaultCorroborationThresholds()` setting `SeverityOverrides["critical"]={BlockAt:1}`, one signed critical source now correctly blocks. The old assertions `want "warn"` reflected the old behavior — the new `"block"` expectation is the correct Phase-6 behavior.
- **Fix:** Updated both test expectations from `"warn"/"Allow:true"` to `"block"/"Allow:false"` with explanatory comments referencing CORR-01.
- **Files modified:** `internal/policy/engine_test.go`
- **Commit:** 6db63b1

## Task 3 Finding: No Critical Fixture Changes Required

The three `selftestEntries` in `selftest.go` all carry `Severity: "critical"` but have no `CatalogSignature` field set (empty string). The `bumblebeeAdapter` derives `Signed: e.CatalogSignature != ""` = `false`. With `signedCount=0` and `hasSignedSource=false`, the critical escalation condition `signedCount >= effectiveBlockAt && hasSignedSource` evaluates to `false`. All 10 selftest fixtures correctly remain at "warn" or "allow" as before.

## TDD Gate Compliance

- RED gate: `test(06-01)` commit c564ad6 — 6 failing tests confirmed (build failure on undefined symbols)
- GREEN gate: `feat(06-01)` commit 6db63b1 — all 6 tests pass, full suite green
- TDD cycle compliant: RED → GREEN → selftest audit

## Threat Model Coverage

All 5 STRIDE threats in the plan's threat register are mitigated:
- T-06-01 (catalog poisoning block storm): `CatalogHealthy=false` suppresses escalation — proven by `TestCorroborationDegradedCatalogNoEscalation`
- T-06-02 (wildcard mis-tag): `Version=="*"` guard returns nil — proven by `TestCorroborationAllVersionsCriticalWildcardStaysWarn`
- T-06-03 (BlockAt<1 override): `validateCorroborationThresholds` rejects + fails closed — proven by `TestValidateCorroborationThresholdsRejectsBlockAtZero`
- T-06-04 (looser override): `validateCorroborationThresholds` rejects `BlockAt > t.BlockAt` — proven by `TestValidateCorroborationThresholdsRejectsLooserOverride`
- T-06-05 (dissent sentinels): `findSeverityOverride` skips `m.Dissented` — accepted per Phase-11 decision

## Self-Check: PASSED

- `internal/policy/types.go` — modified (SeverityThreshold type + updated CorroborationThresholds + DefaultCorroborationThresholds)
- `internal/policy/corroboration.go` — modified (findSeverityOverride + extended validateCorroborationThresholds + updated escalation table)
- `internal/policy/corroboration_test.go` — modified (6 new test functions)
- `internal/check/selftest.go` — modified (audit comment)
- Commits c564ad6, 6db63b1, b1ad691 present in git log
