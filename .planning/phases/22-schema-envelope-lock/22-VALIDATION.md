---
phase: 22
slug: schema-envelope-lock
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-13
---

# Phase 22 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Seeded from `22-RESEARCH.md` §Validation Architecture (Go testing; HIGH confidence, code-grounded). Per-task IDs are filled by the planner; the requirement→test map below is authoritative for coverage.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing package (stdlib) |
| **Config file** | none (Go convention) |
| **Quick run command** | `go test ./internal/corpus/... ./internal/audit/... ./internal/policy/... ./internal/config/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30–60 s full suite |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/corpus/... ./internal/audit/... ./internal/policy/... ./internal/config/...`
- **After every plan wave:** `go test ./...`
- **Before `/gsd-verify-work`:** Full suite green + `go build ./...` + the phase gate below
- **Phase gate:** `go test ./... && go build ./... && [ "$(grep -r "auto_purge" internal/corpus/ | wc -l)" -eq 0 ]`
- **Max feedback latency:** ~60 s

---

## Per-Requirement Verification Map

> Task IDs (`22-NN-NN`) assigned by the planner; commands/behaviors are fixed.

| Req ID | Behavior (secure/observable) | Test Type | Automated Command | File Exists |
|--------|------------------------------|-----------|-------------------|-------------|
| SCHEMA-01 | `CorpusRecord` embeds `AuditRecord` (unnamed embed, NOT `json:",inline"`); all four layers present; `source_surface` present; `TrueLabel` defaults `"unresolved"` (not `omitempty`); `WasCorrect *bool` nil = unresolved | unit | `go test ./internal/corpus/... -run TestCorpusRecordSchema` | ❌ W0 |
| SCHEMA-02 | `ScanClusterID(pkg, ver, fp)` stable across calls; distinct inputs → distinct IDs; NUL separator prevents collisions; agent surfaces per-event vs non-agent per-cluster | unit | `go test ./internal/corpus/... -run TestScanClusterID` | ❌ W0 |
| SCHEMA-03 | `PushEnvelope` JSON round-trip — all fields present; `signing` block nil/zero in v1 | unit | `go test ./internal/corpus/... -run TestPushEnvelopeRoundTrip` | ❌ W0 |
| SCHEMA-04 | `ActionHint` typed const — `ActionHintWatchAndBlock` is the only value; no `auto_purge` constant exists; `PushEnvelope.ActionHint` typed `ActionHint` not `string` | compile-time | `go build ./internal/corpus/...` succeeds AND `grep -r "auto_purge" internal/corpus/` returns 0 | ❌ W0 |
| SCHEMA-05 | `BehaviorSigHash` deterministic over `action_type` + normalized `target_resource` + normalized `network_destination`; NUL-separated; `ruleset_version` recorded | unit | `go test ./internal/corpus/... -run TestBehaviorSigHash` | ❌ W0 |
| SCHEMA-06 | Schema-lock gate — Nx Console trace maps to schema with no gaps; envelope represents a `watch_and_block` push with `confidence_tier:"enforce"` + `source_count:2` | unit (gate) | `go test ./internal/corpus/... -run TestSchemaLockNxConsoleTrace` | ❌ W0 |
| SCOPE-01 | `CorpusRecord{}` zero-value serializes `"scope":"org_only"`; `CorpusScope("").MarshalJSON()` → `"org_only"` | unit | `go test ./internal/corpus/... -run TestScopeZeroValue` | ❌ W0 |
| SCOPE-02 | `PromoteScope(&rec)` returns non-nil error in v1; `rec.Scope` unchanged after the error | unit | `go test ./internal/corpus/... -run TestPromoteScopeReturnsErrorInV1` | ❌ W0 |

**Cross-package prerequisite tests (Phase 22 enables Phase 23):**

| Item | Command | File |
|------|---------|------|
| `RedactRecordWithDefaults(rec)` exported wrapper applies default patterns (unexported `redactPattern` cannot cross packages) | `go test ./internal/audit/... -run TestRedactRecordWithDefaults` | extend `internal/audit/redact_test.go` |
| `CorroborateOutcome` wrapper maps `count>=BlockAt`→`enforce`; single-source critical block (`level==block`, `count==1`)→`watch` | `go test ./internal/policy/... -run TestCorroborateOutcome` | extend `internal/policy/corroboration_test.go` |
| No new `go.mod` imports | `go mod tidy && git diff --exit-code go.mod` | — |
| `internal/policy` stays I/O-free | `go test ./internal/policy/... -run TestPolicyImportsArePure` (existing) | — |

---

## Compile-Time `auto_purge` Guard (type-level guarantee, not a runtime test)

Validation = three checks, all at the phase gate:
1. `go build ./internal/corpus/...` succeeds.
2. `grep -r "auto_purge" internal/corpus/` returns no results (no constant exists).
3. `PushEnvelope.ActionHint` field type is `ActionHint` (not `string`); `go vet ./internal/corpus/...` passes.

Optional `internal/corpus/testdata/compile_fail/auto_purge_assignment.go` with `//go:build ignore` documents the intended compile error for future contributors (it must NOT compile — that is the property).

---

## Wave 0 Requirements

- [ ] `internal/corpus/` package created (new) with `corpus_test.go` — SCHEMA-01/02/03/05, SCOPE-01/02
- [ ] `internal/corpus/schema_lock_test.go` — SCHEMA-06 gate
- [ ] `internal/corpus/testdata/nx_console_trace.json` — Nx Console trace fixture
- [ ] `internal/audit/redact_test.go` (existing) — add `TestRedactRecordWithDefaults`
- [ ] `internal/policy/corroboration_test.go` (existing) — add `TestCorroborateOutcome`
- [ ] `internal/config/config_test.go` (existing) — add `TestCorpusConfig` (if `CorpusConfig` lands in 22)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Schema "freeze" sign-off | SCHEMA-06 | Freezing the format is a human gate (PRD §4 Phase 0: "Sign-off freezes the format") | Maintainer reviews the schema + envelope types + the Nx Console gate test and confirms no field gaps before Phase 23 builds on them |

---

## Validation Sign-Off

- [ ] All tasks have an `<automated>` verify or a Wave 0 dependency
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60 s
- [ ] `nyquist_compliant: true` set in frontmatter (after planner reconciles task IDs)

**Approval:** pending
