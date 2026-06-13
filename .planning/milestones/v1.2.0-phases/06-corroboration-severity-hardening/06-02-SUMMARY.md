---
phase: 06-corroboration-severity-hardening
plan: "02"
subsystem: internal/policyloader
tags: [corroboration, critical_block_at, policyloader, tdd, CORR-01]
dependency_graph:
  requires: [06-01]
  provides: [CriticalBlockAt-PolicyRule, ValidateSchema-CriticalBlockAt-bound, ThresholdsFromPolicyFiles-CriticalBlockAt-merge]
  affects: [internal/policyloader/loader.go, internal/policyloader/validate.go, internal/policyloader/test.go]
tech_stack:
  added: []
  patterns: [non-zero-override, error-accumulation, tdd-red-green]
key_files:
  created: []
  modified:
    - internal/policyloader/loader.go
    - internal/policyloader/validate.go
    - internal/policyloader/test.go
    - internal/policyloader/test_test.go
decisions:
  - "CriticalBlockAt zero = use-default (not block unconditionally) — mirrors WarnAt/BlockAt/QuarantineAt non-zero override pattern in ThresholdsFromPolicyFiles"
  - "Load-time ValidateSchema bound: CriticalBlockAt != 0 && < 1 rejected; upper bound (<= global BlockAt) deferred to eval-time validateCorroborationThresholds which has the fully-resolved BlockAt"
  - "QuarantineAt defaults to CriticalBlockAt+1 when the existing override has QuarantineAt==0 — provides a sensible default without requiring operators to specify both fields"
  - "No schema_version bump — critical_block_at is an optional additive field on existing corroboration_threshold rule type; files without it remain valid via DisallowUnknownFields (now a known field)"
  - "Both thresholdsFromPolicyFile (unexported, used by RunPolicyTest) and ThresholdsFromPolicyFiles (exported, used by live check/gateway/watch/scan) extended identically — dry-run parity maintained"
metrics:
  duration_seconds: 171
  completed_date: "2026-06-03T19:17:17Z"
  tasks_completed: 2
  files_changed: 4
---

# Phase 06 Plan 02: Policy-File Configurable critical_block_at Override Summary

Operators can now configure the critical-severity corroboration block threshold via policy files using `critical_block_at` on any `corroboration_threshold` rule — allowing reversion to 2-source behavior (`critical_block_at: 2`) without a Beekeeper release if false positives emerge (CORR-01 Q3 / PITFALLS.md severity-inflation recovery path).

## What Was Built

### Task 1: RED (09b58e7)

Added `TestThresholdsFromPolicyFilesCriticalBlockAt` to `internal/policyloader/test_test.go`. The test constructs a `PolicyFile` with a `corroboration_threshold` rule with `CriticalBlockAt: 1` and asserts that `ThresholdsFromPolicyFiles` returns `SeverityOverrides["critical"].BlockAt == 1` and `QuarantineAt >= 1`. Compile failure (`unknown field CriticalBlockAt in struct literal of type PolicyRule`) confirmed RED state.

### Task 2: GREEN (fd14e85)

**`internal/policyloader/loader.go`:**
- Added `CriticalBlockAt int` with JSON tag `critical_block_at,omitempty` to `PolicyRule`, after `QuarantineAt` (same inline-comment style as sibling fields)
- Existing policy files without `critical_block_at` remain valid — the field zero-values to 0 (use-default); `DisallowUnknownFields` continues to reject all other unknown fields since `critical_block_at` is now a known field

**`internal/policyloader/validate.go`:**
- Extended `ValidateSchema` rule loop with a bound check: when `r.RuleType == "corroboration_threshold" && r.CriticalBlockAt != 0`, rejects `r.CriticalBlockAt < 1` with a descriptive accumulation error (error-accumulation pattern, no early return)
- Comment documents why the upper bound is deferred to eval-time `validateCorroborationThresholds`

**`internal/policyloader/test.go`:**
- Extended inner loop in both `thresholdsFromPolicyFile` (unexported) AND `ThresholdsFromPolicyFiles` (exported) identically: `if r.CriticalBlockAt > 0` allocates `SeverityOverrides` map if nil, sets `existing.BlockAt = r.CriticalBlockAt`, defaults `existing.QuarantineAt = r.CriticalBlockAt + 1` when 0, and writes back
- Mirrors the established non-zero-override pattern exactly

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 (RED) | 09b58e7 | test(06-02): add failing critical_block_at policy-file merge test |
| 2 (GREEN) | fd14e85 | feat(06-02): policy-file configurable critical_block_at corroboration override (CORR-01) |

## Test Results

- `TestThresholdsFromPolicyFilesCriticalBlockAt` — PASS (new, target test)
- All 28 policyloader tests — PASS (no regressions)
- `go build ./...` — PASS (CriticalBlockAt field addition compiles across all consumers)
- Existing `TestValidateSchema_*` tests still pass — no `DisallowUnknownFields` regression

## Deviations from Plan

None — plan executed exactly as written.

## TDD Gate Compliance

- RED gate: `test(06-02)` commit 09b58e7 — compile failure on undefined `CriticalBlockAt` field confirmed
- GREEN gate: `feat(06-02)` commit fd14e85 — all policyloader tests pass
- TDD cycle compliant: RED → GREEN

## Threat Model Coverage

| Threat | Mitigation | Evidence |
|--------|------------|---------|
| T-06-06 (Tampering: critical_block_at:0) | `ValidateSchema` treats 0 as use-default; rejects negative values at load time; eval-time `validateCorroborationThresholds` (plan 06-01) fails closed | `TestValidateSchema_*` tests pass; `TestValidateCorroborationThresholdsRejectsBlockAtZero` (plan 06-01) proven |
| T-06-07 (Spoofing: looser-than-global override) | Upper bound enforced at eval time by `validateCorroborationThresholds` (plan 06-01) which has the resolved global BlockAt | `TestValidateCorroborationThresholdsRejectsLooserOverride` (plan 06-01) proven |
| T-06-08 (Tampering: unknown-field injection) | `LoadPolicyFile` uses `DisallowUnknownFields`; `critical_block_at` is now a known field — no new attack surface | No `DisallowUnknownFields` regression in test suite |

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries beyond what is documented in the plan's threat model.

## Self-Check: PASSED

- `internal/policyloader/loader.go` — modified (CriticalBlockAt field added)
- `internal/policyloader/validate.go` — modified (ValidateSchema CriticalBlockAt bound check)
- `internal/policyloader/test.go` — modified (both merge loops extended)
- `internal/policyloader/test_test.go` — modified (TestThresholdsFromPolicyFilesCriticalBlockAt added)
- Commits 09b58e7 and fd14e85 present in git log
