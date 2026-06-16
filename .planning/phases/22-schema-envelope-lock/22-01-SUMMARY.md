---
phase: 22-schema-envelope-lock
plan: "01"
subsystem: audit/policy/config
tags: [schema, audit, policy, config, corpus, phase-22]
dependency_graph:
  requires: []
  provides:
    - audit.AuditRecord.SourceSurface (SCHEMA-01)
    - audit.AuditRecord.ClusterID (SCHEMA-02)
    - audit.AuditRecord.RulesetVersion (SCHEMA-05)
    - audit.RedactRecordWithDefaults (Phase-23 prerequisite)
    - policy.CorroborationOutcome
    - policy.CorroborateOutcome (Phase-23 corpus adjudication prerequisite)
    - config.CorpusConfig
    - config.Config.Corpus
  affects:
    - internal/corpus (Plans 02/03 — can now compile against these types)
    - Phase 23 corpus store (RedactRecordWithDefaults + CorroborateOutcome unblock)
tech_stack:
  added: []
  patterns:
    - omitempty additive fields on existing struct (backward-compatible schema extension)
    - exported wrapper over unexported type (cross-package escape hatch)
    - count-based tier derivation (NOT level-based — Pitfall 4 / 2FA invariant)
    - AuditConfig pattern replicated for CorpusConfig
key_files:
  created: []
  modified:
    - internal/audit/types.go
    - internal/audit/redact.go
    - internal/audit/redact_test.go
    - internal/policy/corroboration.go
    - internal/policy/corroboration_test.go
    - internal/config/config.go
    - internal/config/config_test.go
decisions:
  - "source_surface, cluster_id, ruleset_version placed on AuditRecord (not CorpusRecord) — additive omitempty so existing audit consumers unchanged; Sentry surface can set cluster_id directly (research Finding 1 placement matrix)"
  - "alert verdict documented as comment on Decision field (no Go type change) — keeps the change additive/non-breaking; corpus emitter sets it in Phase 23 (OQ-1 resolution)"
  - "CorroborateOutcome tier derived from count >= t.BlockAt, NEVER from level == block — a single-source critical-severity block (severity override) has level=block but count=1 and must map to confidence_tier:watch (Pitfall 4 / 2FA invariant)"
  - "CorpusConfig uses value type (not pointer) on Config — same pattern as AuditConfig; absent corpus key leaves Enabled:false (safe default) via Go zero-value semantics"
metrics:
  duration: "~25 minutes"
  completed: "2026-06-13"
  tasks_completed: 3
  tasks_total: 3
  files_modified: 6
  commits: 3
---

# Phase 22 Plan 01: Schema & Envelope Lock Prerequisites Summary

Additive prerequisite changes to three existing packages enabling the Phase 22 corpus types (Plans 02/03) and unblocking Phase 23. Three modified production files, three extended test files, zero new go.mod dependencies, zero behavior changes.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add additive schema fields to AuditRecord + document alert verdict | `992a41f` | internal/audit/types.go |
| 2 | Export RedactRecordWithDefaults from internal/audit | `e963df9` | internal/audit/redact.go, redact_test.go |
| 3 | Export CorroborateOutcome (policy) + add CorpusConfig (config) | `ffdccda` | internal/policy/corroboration.go, corroboration_test.go, internal/config/config.go, config_test.go |

## What Was Built

**Task 1 — AuditRecord schema additions (SCHEMA-01/02/05):**

Three additive `omitempty` fields added to `audit.AuditRecord`:
- `SourceSurface string json:"source_surface,omitempty"` — the branch key (hook|mcp_gateway|shim|file_watcher|sentry|scan)
- `ClusterID string json:"cluster_id,omitempty"` — binds correlated non-agent events; Sentry surface may set it directly
- `RulesetVersion string json:"ruleset_version,omitempty"` — catalog snapshot version at decision time

Decision field doc-comment updated from `// allow|warn|block` to `// allow|warn|block|alert (alert = Sentry detection-only, set by the corpus emitter in Phase 23)`. No type change; Decision remains `string`. `FromDecision` is unchanged.

**Task 2 — RedactRecordWithDefaults (T-22-02 / research Finding 7):**

`func RedactRecordWithDefaults(rec AuditRecord) AuditRecord` added to `internal/audit/redact.go`. The unexported `redactPattern` type used by `RedactRecord`/`DefaultRedactPatterns` cannot cross package boundaries, so `internal/corpus` cannot call those directly. This wrapper provides a cross-package-safe single-argument signature. Body: `return RedactRecord(rec, DefaultRedactPatterns())`.

`TestRedactRecordWithDefaults` proves: Bearer token in `rec.Reason` is redacted, JWT is redacted, input is not mutated.

**Task 3 — CorroborateOutcome + CorpusConfig:**

`CorroborationOutcome struct { SourceCount int; ConfidenceTier string }` and `CorroborateOutcome(matches []CatalogMatch, t CorroborationThresholds) CorroborationOutcome` added to `internal/policy/corroboration.go`. Tier derived from `count >= t.BlockAt` (NOT from `level == "block"`) — this is the 2FA invariant: a single-source critical-severity block has `level="block"` but `count=1` (< BlockAt=2) and must emit `confidence_tier:"watch"` in the corpus record. No new imports; `corroboration.go` still imports only `fmt` and `sort`.

`CorpusConfig struct { Enabled bool; Path string; Scope string }` and `Config.Corpus CorpusConfig json:"corpus,omitempty"` added to `internal/config/config.go`. Pattern mirrors `AuditConfig`. `Enabled` defaults false (zero value) so no behavior change in Phase 22 (store wiring is Phase 23, T-22-04).

Tests: `TestCorroborateOutcome` (enforce, watch-from-critical-single-source, no-match); `TestCorpusConfig` (JSON round-trip, missing key = zero value, community_shareable parses).

## Verification Results

```
go test ./internal/audit/... ./internal/policy/... ./internal/config/...
ok  github.com/home-beekeeper/beekeeper/internal/audit
ok  github.com/home-beekeeper/beekeeper/internal/policy
ok  github.com/home-beekeeper/beekeeper/internal/config

go build ./...  → exit 0
go mod tidy && git diff --exit-code go.mod → exit 0 (zero new dependencies)
internal/policy/corroboration.go imports: only "fmt" and "sort" (TestCorroborationImportsArePure passes)
```

## Requirements Covered

| Requirement | Coverage |
|-------------|----------|
| SCHEMA-01 | `source_surface` additive omitempty field on AuditRecord |
| SCHEMA-02 | `cluster_id` additive omitempty field on AuditRecord |
| SCHEMA-05 | `ruleset_version` additive omitempty field on AuditRecord |
| Phase-23 prereq (T-22-02) | `RedactRecordWithDefaults` exported with cross-package-safe signature |
| Phase-23 prereq (Finding 8) | `CorroborateOutcome` exported wrapper with correct tier mapping |
| CorpusConfig (Finding 10) | `config.CorpusConfig` + `Config.Corpus` field; round-trips from JSON |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — no placeholder values or TODO stubs in any modified file. All new fields are real struct fields with accurate json tags. `CorpusConfig.Enabled` defaults false intentionally (Phase 23 wires the store).

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries beyond what the plan's threat register covers (T-22-01 through T-22-SC, all mitigated or accepted per plan).

## Self-Check

### Files exist

- [x] `internal/audit/types.go` — modified (SourceSurface/ClusterID/RulesetVersion + alert comment)
- [x] `internal/audit/redact.go` — modified (RedactRecordWithDefaults)
- [x] `internal/audit/redact_test.go` — modified (TestRedactRecordWithDefaults)
- [x] `internal/policy/corroboration.go` — modified (CorroborationOutcome + CorroborateOutcome)
- [x] `internal/policy/corroboration_test.go` — modified (TestCorroborateOutcome)
- [x] `internal/config/config.go` — modified (CorpusConfig + Config.Corpus)
- [x] `internal/config/config_test.go` — modified (TestCorpusConfig)

### Commits exist

- [x] `992a41f` feat(22-01): add additive schema fields to AuditRecord + document alert verdict
- [x] `e963df9` feat(22-01): export RedactRecordWithDefaults cross-package redaction entrypoint
- [x] `ffdccda` feat(22-01): export CorroborateOutcome (policy) + add CorpusConfig (config)

## Self-Check: PASSED
