---
phase: 09-v1.2.0-tech-debt-cleanup
plan: "02"
subsystem: check/e2e
tags: [e2e, corroboration, hermetic, release-gate, CLEAN-01]
requirements: [CLEAN-01]

dependency_graph:
  requires: []
  provides: [hermetic-CORR-E2E-gate]
  affects: [internal/check/e2e_test.go]

tech_stack:
  added: []
  patterns:
    - "Signed non-wildcard catalog fixture for hermetic corroboration E2E (mirrors TestRunCheckAiFigureBlocks unit test pattern)"

key_files:
  created: []
  modified:
    - internal/check/e2e_test.go

decisions:
  - "Use sha256:e2e-corr-test-sig as CatalogSignature value (any non-empty string sets Signed:true in the bumblebee adapter; mirrors corr02-test-sig from unit test)"
  - "Set Versions:[1.0.0] (not [*]) so the all-versions wildcard guard in findSeverityOverride does not suppress the critical escalation"
  - "Change stdin command to npm install ai-figure@1.0.0 to match the non-wildcard version entry"
  - "Leave os.Environ() inheritance in runCase unchanged — network-independence is achieved by fixture quality, not egress blocking"

metrics:
  duration: "~20 minutes"
  completed: "2026-06-04"
  tasks_total: 2
  tasks_completed: 2
  files_changed: 1
---

# Phase 9 Plan 02: Hermetic CORR E2E Release Gate (CLEAN-01) Summary

**One-liner:** Seeded a signed, non-wildcard `ai-figure@1.0.0` catalog fixture so `TestE2ELiveBinary/CORR_aifigure_critical_block` blocks via local corroboration (SeverityOverrides[critical].BlockAt=1) with no OSV network dependency.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Seed signed non-wildcard ai-figure fixture | e2a821a | internal/check/e2e_test.go |
| 2 | Prove network-independence + full E2E gate green | (verify only) | — |

## What Was Done

### Task 1: Fixture change in `CORR_aifigure_critical_block`

Changed the seeded `catalog.Entry` in the CORR E2E sub-case from an unsigned wildcard entry to a signed, version-specific entry that mirrors `buildCriticalTestIndex` from `handler_test.go`:

**Before (broken — network-dependent):**
```go
catalog.Entry{
    ID:            "e2e-ai-figure-critical",
    Package:       "ai-figure",
    Versions:      []string{"*"},       // wildcard guard suppresses escalation
    Severity:      "critical",
    CatalogSource: "bumblebee",
    // CatalogSignature absent → Signed:false → unsigned source
}
// stdin: npm install ai-figure  (no version → matches wildcard but escalation suppressed)
```

**After (hermetic — network-independent):**
```go
catalog.Entry{
    ID:               "e2e-ai-figure-critical-signed",
    Package:          "ai-figure",
    Versions:         []string{"1.0.0"},                 // non-wildcard: escalation fires
    Severity:         "critical",
    CatalogSource:    "bumblebee",
    CatalogSignature: "sha256:e2e-corr-test-sig",        // Signed:true in adapter
}
// stdin: npm install ai-figure@1.0.0  (version-matching install)
```

### Task 2: Full E2E battery result

```
=== RUN   TestE2ELiveBinary/SPATH_credential_block    PASS (1.01s)
=== RUN   TestE2ELiveBinary/CORR_aifigure_critical_block  PASS (3.80s)
=== RUN   TestE2ELiveBinary/NUDGE_pnpm_add_chalk      PASS (3.54s)
=== SKIP  TestE2ELiveBinary/NUDGE_bun_add_chalk       (bun not installed — expected)
PASS  14.42s
```

## Network-Independence Proof

The CORR case block is now network-independent by fixture construction, not by egress blocking. The reasoning is by parity with `TestRunCheckAiFigureBlocks` (handler_test.go:709-734):

- That unit test uses an identical fixture pattern: `CatalogSignature: "sha256:corr02-test-sig"`, `Versions: ["1.0.0"]`, severity `critical`, source `bumblebee`
- It passes with zero network access (no OSV call, no outbound connections, pure in-process `RunCheck`)
- The E2E fixture now satisfies the same conditions: one signed critical source → `findSeverityOverride` returns the override (wildcard guard does not fire) → `effectiveBlockAt=1` → `signedCount(1) >= 1` → decision `"block"`
- The OSV adapter may or may not be reachable; its response cannot change the outcome because the local signed source already meets the block threshold

The corroboration escalation path (`internal/policy/corroboration.go`) and `SeverityOverrides[critical].BlockAt=1` are untouched. The production block threshold is not weakened — only the test fixture was made more accurate.

## Deviations from Plan

None — plan executed exactly as written. The fixture change was a one-task surgical edit to `e2e_test.go` with no production code changes.

## Threat Model Verification

| Threat ID | Status |
|-----------|--------|
| T-09-05 (release gate flake via live OSV) | Mitigated — block now fires from local signed fixture |
| T-09-06 (block weakening) | Verified not weakened — SeverityOverrides, wildcard guard, and unsigned guard are untouched |

`git diff --name-only HEAD~1 HEAD` shows only `internal/check/e2e_test.go` (no production `.go` files changed).

## Self-Check: PASSED

- [x] `internal/check/e2e_test.go` modified (fixture + comment updated)
- [x] Commit e2a821a exists
- [x] `go test -tags e2e -run TestE2ELiveBinary ./internal/check/ -v` PASS (all sub-cases)
- [x] `go vet -tags e2e ./internal/check/...` clean (no output)
- [x] SPATH and NUDGE sub-cases unchanged and green
- [x] No production files modified
