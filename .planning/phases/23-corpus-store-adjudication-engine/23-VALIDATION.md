---
phase: 23
slug: corpus-store-adjudication-engine
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-13
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Seeded from `23-RESEARCH.md` §Validation Architecture (Go testing + testing/fuzz + benchmark; HIGH confidence, code-grounded against the Phase-22-shipped types). Per-task IDs (`23-NN-NN`) are filled by the planner; the requirement→test map below is authoritative for coverage.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testing/fuzz (stdlib) |
| **Config file** | none (Go convention) |
| **Quick run command** | `go test ./internal/corpus/... ./internal/check/... -short` |
| **Full suite command** | `go test ./...` |
| **Fuzz command** | `go test -fuzz=FuzzBuildPushEnvelope -fuzztime=30s ./internal/corpus/...` |
| **Benchmark command** | `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` |
| **Estimated runtime** | ~30–60 s full suite; +30 s fuzz seed; +10 s benchmark |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/corpus/... ./internal/check/... -short`
- **After every plan wave:** `go test ./...`
- **Before `/gsd-verify-work`:** Full suite green + `go build ./...` + fuzz seed + benchmark gate (below)
- **Phase gate:** `go test ./... && go build ./... && go vet ./... && go test -fuzz=FuzzBuildPushEnvelope -fuzztime=30s ./internal/corpus/... && [ "$(grep -r "auto_purge" internal/corpus/ | wc -l)" -eq 0 ] && go mod tidy && git diff --exit-code go.mod`
- **Max feedback latency:** ~60 s

---

## Per-Requirement Verification Map

> Task IDs (`23-NN-NN`) assigned by the planner; commands/behaviors are fixed. Source: `23-RESEARCH.md` §Validation Architecture → Phase Requirements → Test Map.

| Req ID | Behavior (secure/observable) | Test Type | Automated Command | File Exists |
|--------|------------------------------|-----------|-------------------|-------------|
| ADJ-01 | Corpus write/adjudication off the hot path; an injected corpus write error does NOT change the hook exit code; `internal/policy` stays I/O-free | unit | `go test ./internal/check/... -run TestCorpusWriteErrorDoesNotChangeExitCode` + `go test ./internal/policy/... -run TestCorroborationImportsArePure` (existing) | ❌ W0 |
| ADJ-01 | `BenchmarkRunCheck` p99 < 100 ms with `cfg.Corpus.Enabled=true` | benchmark | `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` | ❌ W0 |
| ADJ-02 | Initial `TrueLabel` = `"unresolved"`; valid transitions to `malicious`/`benign`/`policy_correct` | unit | `go test ./internal/corpus/... -run TestAdjudicationTrueLabelTransition` | ❌ W0 |
| ADJ-03 | 6 `adjudication_source` values map to the documented confidence (forensic/breach=high; catalog/benign_explained=medium; downstream_clean/user_override=weak) | unit table | `go test ./internal/corpus/... -run TestAdjudicationSources` | ❌ W0 |
| ADJ-04 | `source_count` = count of DISTINCT sources (3× Bumblebee events → `source_count:1`), never event count | unit | `go test ./internal/corpus/... -run TestSourceCountDedup` | ❌ W0 |
| ADJ-05 | `confidence_tier`: 1 source → `"watch"`; ≥2 sources → `"enforce"`; single-source critical block stays `"watch"` | unit table | `go test ./internal/corpus/... -run TestConfidenceTierTable` | ❌ W0 |
| ADJ-06 | `was_correct` derived from `true_label` vs `verdict` (`policy_correct`→true); `resolved_at` (RFC3339) set when leaving `unresolved`, absent while unresolved | unit | `go test ./internal/corpus/... -run TestWasCorrectAndResolvedAt` | ❌ W0 |
| ADJ-07 | Corrections are append-only superseding records (new RecordID, same ClusterID, references prior); `downstream_clean` labels benign only after a 30-day (configurable) follow-on-free window | unit | `go test ./internal/corpus/... -run TestSupersedingRecords` + `-run TestDownstreamCleanWindow` | ❌ W0 |
| STORE-01 | Corpus NDJSON is append-only (records append; existing content never truncated/rewritten) | unit | `go test ./internal/corpus/... -run TestStoreAppendOnly` | ❌ W0 |
| STORE-02 | `RedactRecordWithDefaults` called before every write; an AWS-key-shaped field is absent from the persisted NDJSON line | unit | `go test ./internal/corpus/... -run TestStoreRedactsSecretsBeforeWrite` | ❌ W0 |
| STORE-03 | Corpus file is owner-only (0600 / Windows owner-DACL via `platform.SetOwnerOnly`), identical to the audit log | unit | `go test ./internal/corpus/... -run TestStoreFilePermissions` | ❌ W0 |
| STORE-04 | Records persist in push-envelope shape from the first write (no later migration) | unit | `go test ./internal/corpus/... -run TestStoreEmitsPushEnvelopeShape` | ❌ W0 |
| STORE-05 | `repo_fingerprint`/`fleet_node_id` are HMAC-SHA256 with a per-install salt: same id+salt → same value; different salts → different values (non-reversible; two installs of same repo differ) | unit | `go test ./internal/corpus/... -run TestFingerprintNonReversibility` | ❌ W0 |
| ENV-01 | Local records emit in the frozen push-envelope shape; NO transport is stood up | unit (round-trip) | `go test ./internal/corpus/... -run TestPushEnvelopeEmitted` | ❌ W0 |
| ENV-02 | `BuildPushEnvelope` returns an error for any purge-class intent; `auto_purge` is never emitted; `confidence_tier`/`source_count` frozen at emission | unit (negative) | `go test ./internal/corpus/... -run TestBuildPushEnvelopeRejectsPurge` | ❌ W0 |
| ENV-03 | Fuzz/property gate: no constructed envelope escapes with a non-allowlisted `action_hint` (release gate / BLOCKER) | fuzz | `go test -fuzz=FuzzBuildPushEnvelope -fuzztime=30s ./internal/corpus/...` | ❌ W0 |

---

## Evaluator Gate (Phase 23 Definition of Done — PRD §4 Phase 1)

All of the following must pass before Phase 23 is marked complete:

1. **Four-layer round-trip:** A synthetic Nx Console Sentry incident records all four layers (behavior + decision + outcome + context) in the corpus NDJSON. `TrueLabel` starts `"unresolved"` and transitions to `"malicious"` after the adjudication batch pass with a catalog-confirmed match.
2. **source_count dedup + tier:** Three Bumblebee match events yield `source_count:1, confidence_tier:"watch"`. Two distinct sources yield `source_count:2, confidence_tier:"enforce"`.
3. **Redaction proof:** An `AuditRecord` carrying `AKIAIOSFODNN7EXAMPLE` produces a corpus NDJSON line where that string does not appear.
4. **HMAC non-reversibility:** same repo path + salt-A ≠ same repo path + salt-B.
5. **Off-hot-path proof:** `BenchmarkRunCheck` with corpus enabled p99 < 100 ms; an injected corpus write error does not change the hook exit code.
6. **ENV-03 fuzz gate:** `FuzzBuildPushEnvelope` runs ≥ 30 s with no `action_hint` outside `{watch_and_block}`.
7. **Full suite green:** `go test ./... -count=1` exits 0; `go build ./...` exits 0; `go mod tidy && git diff --exit-code go.mod` shows no change.

---

## Wave 0 Requirements

- [ ] `internal/corpus/store_test.go` (new) — STORE-01/02/03/04
- [ ] `internal/corpus/fingerprint_test.go` (new) — STORE-05
- [ ] `internal/corpus/emitter_test.go` (new) — ADJ-04/05, ENV-01/02
- [ ] `internal/corpus/adjudicator_test.go` (new) — ADJ-02/03/06/07
- [ ] `internal/corpus/fuzz_test.go` (new) — `FuzzBuildPushEnvelope` stub (ENV-03)
- [ ] `internal/check/handler_test.go` (existing) — add `TestCorpusWriteErrorDoesNotChangeExitCode` + `BenchmarkRunCheck` corpus case (ADJ-01)

*Skeletons/stubs land in Wave 0 (or as the first task of each owning plan) so no implementation task ships without an automated verify.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| BenchmarkRunCheck p99 on this Windows machine | ADJ-01 / LAUNCH-03 (proven in P25) | NDJSON append latency under Windows AV interception is environment-dependent; the automated benchmark asserts the bound but the maintainer should eyeball the reported p99 once | Run `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` and confirm reported p99 < 100 ms |

*All functional phase behaviors have automated verification; the entry above is an environment-sensitivity eyeball, not an unautomated requirement.*

---

## Validation Sign-Off

- [ ] All tasks have an `<automated>` verify or a Wave 0 dependency
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60 s
- [ ] `nyquist_compliant: true` set in frontmatter (after planner reconciles task IDs)

**Approval:** pending
