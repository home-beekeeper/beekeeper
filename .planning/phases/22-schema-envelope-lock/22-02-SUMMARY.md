---
phase: 22-schema-envelope-lock
plan: "02"
subsystem: corpus
tags: [schema, corpus, types, envelope, action-hint, scope, phase-22]
dependency_graph:
  requires:
    - audit.AuditRecord.SourceSurface (SCHEMA-01, from Plan 01)
    - audit.AuditRecord.ClusterID (SCHEMA-02, from Plan 01)
    - audit.AuditRecord.RulesetVersion (SCHEMA-05, from Plan 01)
  provides:
    - internal/corpus.CorpusRecord (SCHEMA-01/03)
    - internal/corpus.PushEnvelope (SCHEMA-03/04)
    - internal/corpus.ActionHint + ActionHintWatchAndBlock (SCHEMA-04)
    - internal/corpus.CorpusScope + MarshalJSON zero-value guard (SCOPE-01)
    - internal/corpus.PromoteScope always-error stub (SCOPE-02)
    - internal/corpus.CorpusSchemaVersion (SCHEMA-05)
  affects:
    - Phase 23 corpus store (CorpusRecord + PushEnvelope types now available)
    - Phase 23 BuildPushEnvelope builder (ActionHint type constraint enforced)
    - Phase 23 ENV-03 fuzz gate (type-level guard established here)
tech_stack:
  added: []
  patterns:
    - unnamed struct embedding for JSON field promotion (NOT json:",inline")
    - typed string constant as compile-time closed-enum guard (ActionHint)
    - MarshalJSON on named string type for zero-value serialization guarantee (CorpusScope)
    - always-error stub as v1 capability placeholder (PromoteScope)
key_files:
  created:
    - internal/corpus/action_hint.go
    - internal/corpus/scope.go
    - internal/corpus/schema_version.go
    - internal/corpus/types.go
    - internal/corpus/corpus_test.go
  modified: []
decisions:
  - "ActionHint compile-time guard via typed string constant (Option A from research Finding 2): type ActionHint string with single ActionHintWatchAndBlock const; PushEnvelope.ActionHint is typed ActionHint not string; assigning any unlisted string literal is a compile error"
  - "CorpusScope.MarshalJSON implements zero-value guarantee (Option A from research Finding 5): empty string maps to org_only regardless of how CorpusRecord is constructed; constructor enforcement alone is insufficient"
  - "PromoteScope always returns non-nil error in v1 and mutates nothing; it is the only code path that may change Scope, so automatic promotion is impossible (SCOPE-02)"
  - "Tasks 1+2 committed as a unit: scope.go references *CorpusRecord from types.go; both files needed for the package to compile"
  - "chore commit (a28c9d2) to remove unlisted purge action string from test comment: SCHEMA-04 success criteria requires grep -r for the unlisted string in internal/corpus/ to return 0 results"
metrics:
  duration: "~30 minutes"
  completed: "2026-06-13"
  tasks_completed: 3
  tasks_total: 3
  files_created: 5
  files_modified: 0
  commits: 3
---

# Phase 22 Plan 02: Schema & Envelope Lock (Wave 2) Summary

Pure `internal/corpus` package establishing the frozen four-layer `CorpusRecord` (unnamed-embeds `audit.AuditRecord`), the `PushEnvelope` wire format, the `ActionHint` typed-const compile-time blast-radius guard, and the `CorpusScope.MarshalJSON` zero-value scope guarantee. Zero new go.mod dependencies. Zero behavior changes to existing packages.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1+2 | ActionHint guard, CorpusScope, schema version + CorpusRecord + PushEnvelope | `996fa46` | internal/corpus/action_hint.go, scope.go, schema_version.go, types.go |
| 3 | Unit tests: embedding promotion, envelope round-trip, scope zero-value, PromoteScope error, ActionHint type safety | `7a03b49` | internal/corpus/corpus_test.go |
| chore | Remove unlisted purge action string from test comment (grep guard) | `a28c9d2` | internal/corpus/corpus_test.go |

## What Was Built

### Task 1: ActionHint Guard, CorpusScope, SchemaVersion

**`internal/corpus/action_hint.go`** (SCHEMA-04):

`type ActionHint string` with the single constant `ActionHintWatchAndBlock ActionHint = "watch_and_block"`. This is the compile-time blast-radius guard. `PushEnvelope.ActionHint` is typed `ActionHint` (not `string`), so assigning any unrecognized string literal is a compile error. No `auto_purge` constant or string literal exists anywhere in `internal/corpus` (`grep -r "auto_purge" internal/corpus/` returns 0 results).

The mechanism: Go named types reject untyped string constant assignment. `envelope.ActionHint = "some_string"` is a compile error. `envelope.ActionHint = ActionHintWatchAndBlock` is the only well-typed assignment. Explicit type conversion (`ActionHint("purge_string")`) would still compile but is visually conspicuous and leaves no guarded constant to grep for — a future contributor would have to write an explicit unsafe conversion with no backing constant.

**`internal/corpus/scope.go`** (SCOPE-01, SCOPE-02):

`type CorpusScope string` with constants `ScopeOrgOnly = "org_only"` and `ScopeCommunityShareable = "community_shareable"`. `MarshalJSON` maps the Go zero value `""` to `"org_only"` — this is the only mechanism that makes `CorpusRecord{}` (zero value) serialize as `"scope":"org_only"` without requiring a constructor call. `PromoteScope(*CorpusRecord) error` always returns a non-nil error and mutates nothing — the SCOPE-02 always-error stub establishing that PromoteScope is the only code path that may change scope.

**`internal/corpus/schema_version.go`**:

`const CorpusSchemaVersion = "1.0"` with a breaking-change warning in the doc comment.

### Task 2: CorpusRecord + PushEnvelope Wire Format

**`internal/corpus/types.go`** (SCHEMA-01, SCHEMA-03):

`type CorpusRecord struct` with UNNAMED embedded `audit.AuditRecord` — no field name, no json tag. Go's `encoding/json` promotes all `AuditRecord` fields to the top-level JSON object. This is the correct idiom; `json:",inline"` is a YAML/mapstructure convention that `encoding/json` does NOT support (Pitfall 1 from research).

Four-layer field layout:
- **behavior + decision layers**: promoted from embedded `audit.AuditRecord` (SourceSurface, ClusterID, RulesetVersion from Plan 01; AgentName, ToolName, Decision, RuleIDs, CorroborationCount, etc. from prior phases)
- **outcome layer**: `TrueLabel string json:"true_label"` (NOT omitempty — always present, "unresolved" initially), `AdjudicationSource omitempty`, `WasCorrect *bool omitempty` (nil = unresolved, distinct from false), `ResolvedAt omitempty`
- **context layer**: `BaselineDeviation omitempty`, `RepoFingerprint omitempty` (HMAC-SHA256 contract documented; population is Phase 23 STORE-05), `FleetNodeID omitempty` (same contract), `Scope CorpusScope json:"scope"` (always present, zero-value resolves via MarshalJSON)
- **schema + envelope**: `CorpusSchemaVersion string json:"corpus_schema_version"`, `PushEnvelope *PushEnvelope json:"push_envelope,omitempty"` (nil until Phase 23 emitter)

`type PushEnvelope struct` with `ActionHint ActionHint` (typed, SCHEMA-04 guard at field level), `Scope CorpusScope`, `Signing *SigningBlock json:"signing,omitempty"` (nil in v1 — zero-value in v1 freezes the wire format). Plus `EnvelopeSignature`, `IOCBlock`, `SigningBlock` all defined with correct json tags.

JSON namespace collision check (verified): `AuditRecord` has no `scope`, `baseline_deviation`, `repo_fingerprint`, `fleet_node_id`, `was_correct`, `true_label`, `adjudication_source`, `resolved_at`, or `corpus_schema_version` fields. No promoted-field collision exists.

### Task 3: Unit Tests

**`internal/corpus/corpus_test.go`** — five tests:

1. **TestCorpusRecordSchema**: Sets `rec.AuditRecord.SourceSurface = "sentry"`, marshals, asserts `"source_surface":"sentry"` at top level and NO `"AuditRecord"` nested key. Also asserts `"true_label"` always present and `"was_correct"` absent when `WasCorrect` is nil.

2. **TestPushEnvelopeRoundTrip**: Two sub-tests — `Signing nil` (signing key absent from JSON) and `Signing &SigningBlock{}` (signing key present). Full field round-trip verification.

3. **TestScopeZeroValue**: `CorpusRecord{}` (zero value) produces `"scope":"org_only"`; `CorpusScope("").MarshalJSON()` returns `"org_only"`; `ScopeCommunityShareable` round-trips correctly.

4. **TestPromoteScopeReturnsErrorInV1**: `err != nil` AND `rec.Scope == ScopeOrgOnly` (unchanged).

5. **TestActionHintTypeSafety**: `ActionHintWatchAndBlock == ActionHint("watch_and_block")`; PushEnvelope with `ActionHintWatchAndBlock` round-trips as `"action_hint":"watch_and_block"`.

## Verification Results

```
go build ./internal/corpus/...    → exit 0
go vet ./internal/corpus/...      → exit 0
go test ./internal/corpus/... -run TestCorpusRecordSchema|TestPushEnvelopeRoundTrip|TestScopeZeroValue|TestPromoteScopeReturnsErrorInV1|TestActionHintTypeSafety
→ PASS (5/5)
go test ./...                     → exit 0 (full suite green, no regressions)
go mod tidy && git diff go.mod    → no change (zero new dependencies)

grep -r "auto_purge" internal/corpus/    → 0 results (exit 1 = no matches)
go list -f '{{.Imports}}' ./internal/corpus/
→ [encoding/json errors github.com/home-beekeeper/beekeeper/internal/audit]
  (pure: no os, net, filesystem imports)
```

## Requirements Covered

| Requirement | Coverage |
|-------------|----------|
| SCHEMA-01 | `CorpusRecord` embeds `audit.AuditRecord` (unnamed, promoted); outcome layer with `TrueLabel` NOT omitempty; four-layer field set complete |
| SCHEMA-03 | `PushEnvelope` wire format frozen: signature, true_label, confidence_tier, source_count, scope, action_hint, signing; round-trips through JSON; `signing` nil in v1 |
| SCHEMA-04 | `type ActionHint string` + single `ActionHintWatchAndBlock` const; `PushEnvelope.ActionHint` typed `ActionHint`; `grep -r "auto_purge" internal/corpus/` = 0 results |
| SCOPE-01 | `CorpusScope.MarshalJSON` maps `""` → `"org_only"`; `CorpusRecord{}` serializes `"scope":"org_only"` (type-level guarantee, not constructor-only) |
| SCOPE-02 | `PromoteScope` always returns non-nil error in v1; mutates nothing; is the only code path that may change scope |

(SCHEMA-02 cluster_id + SCHEMA-05 ruleset_version fields promoted from AuditRecord covered by Plan 01. SCHEMA-05 `CorpusSchemaVersion = "1.0"` constant defined here. SCHEMA-06 schema-lock gate test is Plan 03.)

## Deviations from Plan

**1. [Rule — Plan note] Tasks 1+2 committed as a unit**

- **Found during:** Task 1 creation
- **Issue:** `scope.go` references `*CorpusRecord` from `types.go`; the package cannot compile until both files exist. The plan anticipated this and noted "treat the package-creation tasks as a unit."
- **Fix:** Created all four production files (action_hint.go, scope.go, schema_version.go, types.go) before committing, verifying `go build` and `go vet` pass with all four in place.
- **Deviation type:** Execution grouping only; no behavior or scope change.

**2. [Rule 2 — Guard hardening] Removed unlisted action hint string from test comment**

- **Found during:** Task 3 post-commit verification
- **Issue:** SCHEMA-04 success criteria requires `grep -r "auto_purge" internal/corpus/` to return 0 results. The initial `corpus_test.go` doc comment explained the compile-time guard by mentioning the guarded string in a comment — causing the grep to return matches.
- **Fix:** Rewrote the comment to describe the property without embedding the guarded string literal. Added a `chore` commit.
- **Commit:** `a28c9d2`
- **Files modified:** `internal/corpus/corpus_test.go`

No other deviations. Plan executed as written.

## Known Stubs

- `PromoteScope` is an intentional always-error stub per SCOPE-02 and the CONTEXT.md locked decision. The stub is the deliverable for Phase 22 — real promotion with anonymization is v2.0.
- `RepoFingerprint` and `FleetNodeID` fields are defined with a documented HMAC-SHA256 non-reversibility contract. The actual HMAC population (per-install secret generation) is Phase 23 STORE-05 — the fields are empty in all Phase 22 records.
- `PushEnvelope.Signing` is nil in v1 — the `SigningBlock` type is defined to freeze the wire format; population is v1.1+ when transport exists.
- `CorpusRecord.PushEnvelope` is nil until Phase 23 emitter.

These stubs are all **intentional Phase 22 boundary stubs**, not defects. They are documented in type/field comments and the research phase boundary list.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries beyond what the plan's threat register covers (T-22-05 through T-22-SC). The `internal/corpus` package is pure (imports: `encoding/json`, `errors`, `internal/audit` only).

## Self-Check

### Files exist

- [x] `internal/corpus/action_hint.go` — created (`996fa46`)
- [x] `internal/corpus/scope.go` — created (`996fa46`)
- [x] `internal/corpus/schema_version.go` — created (`996fa46`)
- [x] `internal/corpus/types.go` — created (`996fa46`)
- [x] `internal/corpus/corpus_test.go` — created (`7a03b49`, updated `a28c9d2`)

### Commits exist

- [x] `996fa46` feat(22-02): create internal/corpus package — ActionHint guard, CorpusScope, CorpusRecord, PushEnvelope
- [x] `7a03b49` test(22-02): add corpus unit tests — embedding promotion, envelope round-trip, scope zero-value, PromoteScope error, ActionHint type safety
- [x] `a28c9d2` chore(22-02): remove unlisted action hint string from test comment to satisfy grep guard

### Build and test green

- [x] `go build ./internal/corpus/...` exit 0
- [x] `go vet ./internal/corpus/...` exit 0
- [x] `go test ./internal/corpus/... -run TestCorpusRecordSchema|...` all 5 PASS
- [x] `go test ./...` exit 0 (full suite, no regressions)
- [x] `grep -r "auto_purge" internal/corpus/` returns 0 results
- [x] `go list -f '{{.Imports}}' ./internal/corpus/` shows only stdlib + internal/audit (pure package)
- [x] `go mod tidy && git diff go.mod` no change

## Self-Check: PASSED
