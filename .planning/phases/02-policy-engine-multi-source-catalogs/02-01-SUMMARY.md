---
phase: 02-policy-engine-multi-source-catalogs
plan: "01"
subsystem: policy-engine
tags: [policy, corroboration, multi-source, pure-function, tdd]
dependency_graph:
  requires: [01-06]
  provides: [MultiCatalogLookup interface, CorroborationThresholds, corroborate() function]
  affects: [internal/policy, internal/check (transient break — Plan 08 fix)]
tech_stack:
  added: []
  patterns: [TDD-red-green, pure-function-purity-test, AST-import-gate]
key_files:
  created:
    - internal/policy/corroboration.go
    - internal/policy/corroboration_test.go
  modified:
    - internal/policy/types.go
    - internal/policy/engine.go
    - internal/policy/engine_test.go
decisions:
  - "corroborate() returns agreed as sorted union of all matched sources (signed+unsigned); dissented is nil in Phase 2"
  - "buildRuleIDs maps agreed source names to rule ID constants; falls back to bumblebee rule for unknown sources"
  - "Evaluate deduplicates by CatalogSource before passing to corroborate — same as corroborate() logic"
  - "Version field on returned CatalogMatch is pre-set by the adapter (not re-stamped by engine)"
metrics:
  duration: "~9 min"
  completed: "2026-05-26T09:25:00Z"
  tasks_completed: 3
  files_modified: 5
---

# Phase 2 Plan 01: Policy Engine Corroboration Extension Summary

Extends the Phase 1 single-source warn-only policy engine to full corroboration-based block enforcement (PLCY-01) and adds CTLG-09 provenance fields. This is the foundational type and logic change that every other Phase 2 plan depends on.

## What Was Built

**New `Evaluate` signature (PLCY-01 contract):**
```go
func Evaluate(tc ToolCall, idx MultiCatalogLookup, t CorroborationThresholds) Decision
```

**`MultiCatalogLookup` interface (Wave 2/3 contract):**
```go
type MultiCatalogLookup interface {
    LookupAll(ecosystem, pkg string) []CatalogMatch
}
```

**Corroboration decision table (PLCY-01):**
| Signed sources | Quarantine | Level | Allow |
|---------------|-----------|-------|-------|
| 0 (zero matches) | false | allow | true |
| 0 (unsigned only) | false | warn | true |
| 1 | false | warn | true |
| 2 | false | block | false |
| ≥3 | true | block | false |

**New rule ID constants:**
- `ruleOSVCatalogMatch = "osv-catalog-match"`
- `ruleSocketCatalogMatch = "socket-catalog-match"`

## Corroboration Semantics

- "Independent" = distinct `CatalogSource` values; same source appearing in multiple matches counts as ONE.
- Only SIGNED sources escalate to block. Unsigned sources are warn-only (0.5 weight). Two unsigned sources with zero signed → "warn", never "block".
- `CorroborationThresholds` is configurable per-ecosystem. Default: `{WarnAt:1, BlockAt:2, QuarantineAt:3}`.
- `SourcesAgreed` is the sorted union of ALL matched sources (signed and unsigned).
- `SourcesDissented` is always `nil` in Phase 2 (reserved for Phase 3+).
- `Quarantine` is set `true` when `signedCount >= QuarantineAt`.

## CTLG-09 Provenance Fields Added

**`CatalogMatch` additions:** `Corroborated bool`, `Dissented bool`, `CatalogVersion string`

**`Decision` additions:** `CorroborationCount int`, `SourcesAgreed []string`, `SourcesDissented []string`, `Quarantine bool`

## Tests (23/23 passing)

| Test | Verifies |
|------|---------|
| TestCorroborationZeroMatches | Zero matches → allow |
| TestCorroborationOneSignedSource | 1 signed → warn, count 1 |
| TestCorroborationTwoSignedSources | 2 signed → block, count 2 |
| TestCorroborationThreeSignedSources | 3 signed → block+quarantine |
| TestCorroborationTwoUnsignedNeverBlocks | Unsigned-only → warn (T-02-01-01) |
| TestCorroborationSameSourceTwiceCounts | Deduplication by CatalogSource (T-02-01-02) |
| TestCorroborationOneSignedOneUnsigned | 1 signed + 1 unsigned → warn |
| TestCorroborationImportsArePure | AST purity gate (T-02-01-04) |
| TestEvaluateSingleSignedSourceWarns | Evaluate: 1 signed → warn |
| TestEvaluateTwoSignedSourcesBlock | Evaluate: 2 signed → block |
| TestEvaluateThreeSignedSourcesQuarantine | Evaluate: 3 signed → quarantine |
| TestEvaluateNoMatchAllows | Evaluate: empty LookupAll → allow |
| TestEvaluateUnsignedNeverBlocks | Evaluate: unsigned → warn (T-02-01-01) |
| TestEngineImportsArePure | AST purity gate on engine.go (T-02-01-04) |
| + 9 migrated Phase 1 tests | Existing semantics preserved |

## Threat Mitigations Implemented

| Threat ID | Mitigation |
|-----------|-----------|
| T-02-01-01 | `hasSignedSource` gate: unsigned sources never reach block; TestCorroborationTwoUnsignedNeverBlocks + TestEvaluateUnsignedNeverBlocks |
| T-02-01-02 | CatalogSource deduplication in corroborate(): same source duplicated counts once; TestCorroborationSameSourceTwiceCounts |
| T-02-01-03 | `SourcesAgreed` records exact catalog_source names on every Decision; downstream audit (Plan 08) consumes this for attribution |
| T-02-01-04 | TestCorroborationImportsArePure + TestEngineImportsArePure enforce pure-library import allowlist at AST level |

## Commits

| Task | Type | Hash | Description |
|------|------|------|-------------|
| 1 | feat | a827ea1 | extend types.go with corroboration provenance + MultiCatalogLookup |
| 2 RED | test | 1bcf702 | add failing corroboration tests (RED phase) |
| 2 GREEN | feat | 80ffcf1 | implement pure corroboration logic (GREEN phase) |
| 3 RED | test | 6eda3c8 | migrate engine_test.go to MultiCatalogLookup (RED phase) |
| 3 GREEN | feat | e100e03 | rewire Evaluate onto MultiCatalogLookup + corroboration (GREEN phase) |

## TDD Gate Compliance

- Task 2: RED commit `1bcf702` (test) precedes GREEN commit `80ffcf1` (feat). Gate met.
- Task 3: RED commit `6eda3c8` (test) precedes GREEN commit `e100e03` (feat). Gate met.

## Intentional Transient Build Break

`go build ./...` currently fails with:
```
internal\check\handler.go:143:40: not enough arguments in call to policy.Evaluate
internal\check\selftest.go:89:36: not enough arguments in call to policy.Evaluate
```

This is expected and documented in the plan. `internal/check/handler.go` and `internal/check/selftest.go` call `policy.Evaluate(toolCall, idx)` with the Phase 1 two-argument signature. **Plan 08 (Wave 3) rewires the check handler** to pass a `MultiCatalogLookup` and `CorroborationThresholds`. The fix is intentionally deferred.

`go build ./internal/policy/...` and `go test ./internal/policy/...` both pass with 0 failures.

## Backward Compatibility

The Phase 1 `CatalogLookup` interface is retained unchanged in `types.go`. No existing Phase 1 types were removed. All Phase 1 `CatalogMatch` and `Decision` fields are preserved; Phase 2 adds new fields only.

## Known Stubs

None. All corroboration logic is fully implemented and test-covered. The `SourcesDissented` field is reserved (always `nil`) — this is intentional design, not a stub.

## Threat Flags

None. This plan operates entirely within `internal/policy` (pure library, no new network endpoints, no auth paths, no file access patterns, no schema changes at trust boundaries).

## Self-Check: PASSED

- `internal/policy/corroboration.go`: FOUND
- `internal/policy/corroboration_test.go`: FOUND
- `internal/policy/types.go` (modified): FOUND
- `internal/policy/engine.go` (modified): FOUND
- `internal/policy/engine_test.go` (modified): FOUND
- Commit a827ea1: FOUND (extend types.go)
- Commit 1bcf702: FOUND (RED corroboration tests)
- Commit 80ffcf1: FOUND (GREEN corroboration)
- Commit 6eda3c8: FOUND (RED engine tests)
- Commit e100e03: FOUND (GREEN engine)
- `go test ./internal/policy/... -count=1`: PASS (23/23)
- `go vet ./internal/policy/...`: PASS
- `go build ./internal/policy/...`: PASS
