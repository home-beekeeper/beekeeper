---
phase: 06-corroboration-severity-hardening
verified: 2026-06-03T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification: false
---

# Phase 6: Corroboration Severity Hardening — Verification Report

**Phase Goal:** A critical-severity catalog match blocks at a single trusted source — so `ai-figure` (Shai-Hulud / OSV `MAL-2026-4126`) is blocked, not warned — with an anti-poisoning sanity gate that prevents a degraded or flooded catalog from triggering false escalations.
**Verified:** 2026-06-03
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `beekeeper check` with a critical-severity signed bumblebee + OSV `ai-figure` match returns exit 1 (block) and `decision:"block"` | VERIFIED | `TestRunCheckAiFigureBlocks` PASS: output shows `"Level":"block"`, `"Allow":false`, ExitCode=1. Integration test drives full `RunCheck` stdin→exit-code path. |
| 2 | Degraded catalog (`CatalogHealthy=false` / bumblebee `Degraded:true` in state.json) suppresses escalation — same match returns warn | VERIFIED | `TestRunCheckCriticalDegradedCatalogWarn` PASS: output shows `"Level":"warn"`, `"Allow":true`, ExitCode=0. state.json with `Degraded:true` written via `catalog.SaveState`, then `resolveCatalogHealthy` reads it and sets `CatalogHealthy=false`. |
| 3 | A catalog entry with `versions:["*"]` and `severity:"critical"` still requires 2-source corroboration | VERIFIED | `TestCorroborationAllVersionsCriticalWildcardStaysWarn` PASS: `Version:"*"` on all matches causes `findSeverityOverride` to return nil via the all-versions guard; level stays "warn" with one signed source. |
| 4 | `validateCorroborationThresholds` rejects `BlockAt < 1` with a descriptive error; `corroborate` fails closed to "block" on misconfigured thresholds | VERIFIED | `TestValidateCorroborationThresholdsRejectsBlockAtZero` PASS: `SeverityOverrides["critical"]={BlockAt:0}` returns non-nil error from `validateCorroborationThresholds`; `corroborate` returns "block" (fail-closed). `TestValidateCorroborationThresholdsRejectsLooserOverride` PASS: override `BlockAt:3 > global BlockAt:2` also rejected. |
| 5 | Table-driven unit tests in `internal/policy/` cover the Shai-Hulud fixture (1-source critical → block), degraded-catalog regression (healthy=false → warn), and all-versions guard (wildcard + critical → warn) | VERIFIED | All 6 new tests in `corroboration_test.go` PASS: `TestCorroborationShaiHuludCriticalBlock`, `TestCorroborationDegradedCatalogNoEscalation`, `TestCorroborationAllVersionsCriticalWildcardStaysWarn`, `TestValidateCorroborationThresholdsRejectsBlockAtZero`, `TestValidateCorroborationThresholdsRejectsLooserOverride`, `TestDefaultThresholdsIncludeSeverityOverrides`. |

**Score:** 5/5 truths verified

### Self-Defense Non-Negotiable

| Check | Status | Evidence |
|-------|--------|----------|
| Escalation + sanity bound shipped atomically (same plan 06-01) | VERIFIED | Both `SeverityOverrides` escalation and `CatalogHealthy` gate implemented in the same commit/plan; `corroboration.go` `findSeverityOverride` checks `catalogHealthy` before any override lookup. |
| `internal/policy` stays pure (no I/O imports) | VERIFIED | `TestCorroborationImportsArePure` PASS: `corroboration.go` imports only `"fmt"` and `"sort"`; forbidden packages (`os`, `net`, `net/http`, `io`, `sync`, `time`, `context`) absent. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/policy/types.go` | `SeverityThreshold` type + `SeverityOverrides`/`CatalogHealthy` fields + updated defaults | VERIFIED | `SeverityThreshold struct{BlockAt int; QuarantineAt int}` at line 91; `SeverityOverrides map[string]SeverityThreshold` and `CatalogHealthy bool` on `CorroborationThresholds`; `DefaultCorroborationThresholds()` seeds `CatalogHealthy:true` and `SeverityOverrides["critical"]={BlockAt:1,QuarantineAt:2}`. |
| `internal/policy/corroboration.go` | `findSeverityOverride` helper, extended `validateCorroborationThresholds`, severity-aware escalation table | VERIFIED | `func findSeverityOverride` at line 63; extended bounds loop in `validateCorroborationThresholds` lines 40-50; `effectiveBlockAt`/`effectiveQuarantineAt` computed at lines 160-165 before the escalation switch. |
| `internal/policy/corroboration_test.go` | 6 new Wave-0 unit tests | VERIFIED | All 6 test functions present and PASS: `TestCorroborationShaiHuludCriticalBlock`, `TestCorroborationDegradedCatalogNoEscalation`, `TestCorroborationAllVersionsCriticalWildcardStaysWarn`, `TestValidateCorroborationThresholdsRejectsBlockAtZero`, `TestValidateCorroborationThresholdsRejectsLooserOverride`, `TestDefaultThresholdsIncludeSeverityOverrides`. |
| `internal/policyloader/loader.go` | `CriticalBlockAt int` field with `critical_block_at` JSON tag on `PolicyRule` | VERIFIED | `CriticalBlockAt int \`json:"critical_block_at,omitempty"\`` at line 53. |
| `internal/policyloader/test.go` | `ThresholdsFromPolicyFiles` + `thresholdsFromPolicyFile` merge `CriticalBlockAt` into `SeverityOverrides["critical"]` | VERIFIED | Both merge loops present at lines 46-57 and 90-101. |
| `internal/policyloader/test_test.go` | `TestThresholdsFromPolicyFilesCriticalBlockAt` unit test | VERIFIED | Test at lines 118-149; PASS confirmed. |
| `internal/policyloader/validate.go` | Validates `critical_block_at >= 1` for non-zero values | VERIFIED | Validation at lines 56-62: checks `CriticalBlockAt != 0 && CriticalBlockAt < 1`. |
| `internal/check/sanity.go` | `resolveCatalogHealthy(cacheDir)` helper reading `state.json` `SourceState.Degraded` | VERIFIED | Full implementation at lines 34-47: reads `catalog.LoadState`, checks `bumblebee.Degraded`, defaults true on missing/error. |
| `internal/check/handler_test.go` | 3 integration tests proving live wiring (block, degraded-warn, healthy-thread) | VERIFIED | `TestRunCheckAiFigureBlocks`, `TestRunCheckCriticalDegradedCatalogWarn`, `TestRunCheckCriticalBlockWithHealthyCatalog` all PASS. |
| `internal/gateway/sanity.go` | `resolveCatalogHealthy` for gateway consumer | VERIFIED | File exists, verbatim implementation, `catalog.LoadState` wired. |
| `internal/watch/sanity.go` | `resolveCatalogHealthy` for watch consumer | VERIFIED | File exists, verbatim implementation, `catalog.LoadState` wired. |
| `internal/scan/sanity.go` | `resolveCatalogHealthy` for scan consumer | VERIFIED | File exists, verbatim implementation, `catalog.LoadState` wired. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `corroboration.go corroborate()` | `findSeverityOverride(matches, t.SeverityOverrides, t.CatalogHealthy)` | `effectiveBlockAt`/`effectiveQuarantineAt` computed before the escalation switch | VERIFIED | Line 162: `if ov := findSeverityOverride(matches, t.SeverityOverrides, t.CatalogHealthy); ov != nil { effectiveBlockAt = ov.BlockAt ... }` |
| `corroboration.go corroborate()` | `validateCorroborationThresholds(t)` | fail-closed return "block" on non-nil error | VERIFIED | Line 102: `if err := validateCorroborationThresholds(t); err != nil { return "block", false, 0, nil, nil }` |
| `internal/check/handler.go runCheck` | `thresholds.CatalogHealthy = resolveCatalogHealthy(cacheDir)` | one line between `ThresholdsFromPolicyFiles` and `policy.Evaluate` | VERIFIED | Line 252: `thresholds.CatalogHealthy = resolveCatalogHealthy(cacheDir)` confirmed present with CORR-02 comment. |
| `internal/gateway/policy.go applyPolicy` | `resolveCatalogHealthy(cfg.CacheDir)` | before `policy.Evaluate` call | VERIFIED | Line 121: confirmed by grep. |
| `internal/watch/handler.go` | `resolveCatalogHealthy(h.CacheDir)` | before `policy.Evaluate` call | VERIFIED | Line 134: confirmed by grep. |
| `internal/scan/scanner.go` | `resolveCatalogHealthy(cfg.CacheDir)` | before `policy.Evaluate` call | VERIFIED | Line 282: confirmed by grep. |
| `internal/policyloader/test.go ThresholdsFromPolicyFiles` | `policy.CorroborationThresholds.SeverityOverrides["critical"]` | non-zero `CriticalBlockAt` merge | VERIFIED | Both `ThresholdsFromPolicyFiles` and `thresholdsFromPolicyFile` merge `CriticalBlockAt>0` into `SeverityOverrides["critical"]`. |

### Data-Flow Trace (Level 4)

The critical escalation data flow was traced end-to-end via integration test output:

1. `RunCheck` stdin: `npm install ai-figure-test@1.0.0`
2. `catalog.BuildIndex` → signed bumblebee entry (`CatalogSignature:"sha256:corr02-test-sig"` → `Signed:true`)
3. `policyloader.ThresholdsFromPolicyFiles([])` → `DefaultCorroborationThresholds()` with `SeverityOverrides["critical"]={BlockAt:1,QuarantineAt:2}`, `CatalogHealthy:true`
4. `resolveCatalogHealthy(cacheDir)` → `true` (no state.json or `Degraded:false`)
5. `findSeverityOverride` → returns `{BlockAt:1,QuarantineAt:2}` (critical severity, no wildcard, healthy)
6. `effectiveBlockAt=1`; `signedCount=1 >= 1` → `"block"`, `Allow:false`, `ExitCode=1`

Confirmed: real data flows from catalog entry to block decision. Not a stub or hardcoded return.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Critical signed match blocks (SC1) | `go test ./internal/check/... -run TestRunCheckAiFigureBlocks` | PASS — `"Level":"block"`, `ExitCode=1` | PASS |
| Degraded catalog suppresses to warn (SC2) | `go test ./internal/check/... -run TestRunCheckCriticalDegradedCatalogWarn` | PASS — `"Level":"warn"`, `ExitCode=0` | PASS |
| Healthy explicit state.json still blocks (SC2 wiring proof) | `go test ./internal/check/... -run TestRunCheckCriticalBlockWithHealthyCatalog` | PASS — `"Level":"block"`, `ExitCode=1` | PASS |
| Wildcard stays warn (SC3) | `go test ./internal/policy/... -run TestCorroborationAllVersionsCriticalWildcardStaysWarn` | PASS | PASS |
| BlockAt<1 rejected, fails closed (SC4) | `go test ./internal/policy/... -run TestValidateCorroborationThresholdsRejectsBlockAtZero` | PASS | PASS |
| purity enforced | `go test ./internal/policy/... -run TestCorroborationImportsArePure` | PASS | PASS |
| Full internal/policy suite | `go test ./internal/policy/... -count=1` | PASS (3.347s) | PASS |
| Full internal/check suite | `go test ./internal/check/... -count=1` | PASS (20.508s) | PASS |
| Full policyloader suite | `go test ./internal/policyloader/... -count=1` | PASS (2.148s) | PASS |
| Gateway/watch/scan suites | `go test ./internal/gateway/... ./internal/watch/... ./internal/scan/...` | PASS (all three) | PASS |
| Full build (no import cycles) | `go build ./...` | Exit 0, no output | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CORR-01 | 06-01, 06-02 | Critical-severity match escalates to block at single trusted source via `SeverityOverrides["critical"]={BlockAt:1}` | SATISFIED | `findSeverityOverride` + `SeverityOverrides` in types/corroboration; `CriticalBlockAt` in policyloader; `TestCorroborationShaiHuludCriticalBlock` PASS; `TestRunCheckAiFigureBlocks` PASS |
| CORR-02 | 06-01, 06-03 | Escalation gated on catalog sanity; `validateCorroborationThresholds` rejects unsafe overrides; wildcard guard prevents false positives | SATISFIED | `CatalogHealthy` field + `resolveCatalogHealthy` wired to all 4 consumers; `TestCorroborationDegradedCatalogNoEscalation` + `TestRunCheckCriticalDegradedCatalogWarn` PASS; wildcard guard in `findSeverityOverride`; `TestValidateCorroborationThresholdsRejectsBlockAtZero` PASS |

Both CORR-01 and CORR-02 are fully satisfied. REQUIREMENTS.md traceability table marks both as "Complete" (lines 34-35).

### Anti-Patterns Found

No blockers or warnings found. Full scan of all Phase 6 modified files:

| File | Pattern | Finding |
|------|---------|---------|
| `internal/policy/corroboration.go` | TBD/FIXME/XXX | None |
| `internal/policy/types.go` | TBD/FIXME/XXX | None |
| `internal/check/sanity.go` | TBD/FIXME/XXX | None |
| `internal/check/handler.go` | Stub patterns | None — `thresholds.CatalogHealthy = resolveCatalogHealthy(cacheDir)` is live wiring |
| `internal/policyloader/loader.go` | Stub patterns | None |
| Gateway/watch/scan sanity.go files | Stub patterns | None — all three are verbatim live implementations |

### Selftest Alignment Audit (Plan 06-01 Task 3)

`TestSelftestAllFixturesPass` PASS. All selftest entries carry `Severity:"critical"` with empty `CatalogSignature` (unsigned). The bumblebee adapter sets `Signed:false` for unsigned entries. Since the escalation path requires `signedCount >= effectiveBlockAt && hasSignedSource`, and `signedCount=0` means `hasSignedSource=false`, the critical escalation does NOT fire for these unsigned entries. All critical-severity selftest fixtures correctly remain at "warn". No expectation changes required — recorded in selftest.go lines 120-125 comment and confirmed by PASS.

### Human Verification Required

None. All roadmap success criteria are verified programmatically via test execution. The full end-to-end decision chain (stdin to exit code) is exercised by integration tests `TestRunCheckAiFigureBlocks` and `TestRunCheckCriticalDegradedCatalogWarn`, eliminating the need for manual smoke testing.

### Gaps Summary

No gaps. All 5 roadmap success criteria verified. All 3 plans (06-01, 06-02, 06-03) delivered their stated artifacts, all key links are live (not dormant), and the behavioral test suite proves end-to-end wiring across all 4 enforcement consumers.

---

_Verified: 2026-06-03T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
