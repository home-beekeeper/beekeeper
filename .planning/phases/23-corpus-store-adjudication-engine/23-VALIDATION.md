---
phase: 23
slug: corpus-store-adjudication-engine
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-13
planned: 2026-06-13
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Seeded from `23-RESEARCH.md` §Validation Architecture (Go testing + testing/fuzz + benchmark; HIGH confidence, code-grounded against the Phase-22-shipped types). Per-task IDs reconciled to the 3-plan / 2-wave breakdown (23-01/02/03); the requirement→test map below is authoritative for coverage.

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
- **Phase gate:** `go test ./... && go build ./... && go vet ./... && go test -fuzz=FuzzBuildPushEnvelope -fuzztime=30s ./internal/corpus/... && [ "$(grep -r "auto_purge" internal/corpus/ | grep -v '_test.go' | wc -l)" -eq 0 ] && go mod tidy && git diff --exit-code go.mod`
- **Max feedback latency:** ~60 s

---

## Per-Requirement Verification Map

> Task IDs assigned to the 3-plan breakdown. Commands/behaviors are fixed. Source: `23-RESEARCH.md` §Validation Architecture → Phase Requirements → Test Map.

| Req ID | Owning plan (task) | Behavior (secure/observable) | Test Type | Automated Command | File Exists |
|--------|--------------------|------------------------------|-----------|-------------------|-------------|
| ADJ-01 | 23-03 (T1/T3) | Corpus write/adjudication off the hot path; an injected corpus write error does NOT change the hook exit code; `internal/policy` stays I/O-free | unit | `go test ./internal/check/... -run TestCorpusWriteErrorDoesNotChangeExitCode` + `go test ./internal/policy/... -run TestCorroborationImportsArePure` | ❌ W0 (23-03 T1) |
| ADJ-01 | 23-03 (T1/T3) | `BenchmarkRunCheck` p99 < 100 ms with `cfg.Corpus.Enabled=true` | benchmark | `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` | ❌ W0 (23-03 T1) |
| ADJ-02 | 23-03 (T1/T2) | Initial `TrueLabel` = `"unresolved"`; valid transitions to `malicious`/`benign`/`policy_correct` | unit | `go test ./internal/corpus/... -run TestAdjudicationTrueLabelTransition` | ❌ W0 (23-03 T1) |
| ADJ-03 | 23-03 (T1/T2) | 6 `adjudication_source` values map to documented confidence | unit table | `go test ./internal/corpus/... -run TestAdjudicationSources` | ❌ W0 (23-03 T1) |
| ADJ-04 | 23-02 (T1/T2) | `source_count` = count of DISTINCT sources (3× Bumblebee → `source_count:1`), never event count | unit | `go test ./internal/corpus/... -run TestSourceCountDedup` | ❌ W0 (23-02 T1) |
| ADJ-05 | 23-02 (T1/T2) | `confidence_tier`: 1 source → `"watch"`; ≥2 → `"enforce"`; single-source critical stays `"watch"` | unit table | `go test ./internal/corpus/... -run TestConfidenceTierTable` | ❌ W0 (23-02 T1) |
| ADJ-06 | 23-03 (T1/T2) | `was_correct` from `true_label` vs `verdict` (`policy_correct`→true); `resolved_at` set when leaving `unresolved` | unit | `go test ./internal/corpus/... -run TestWasCorrectAndResolvedAt` | ❌ W0 (23-03 T1) |
| ADJ-07 | 23-03 (T1/T2) | Append-only superseding records (new RecordID, same ClusterID); `downstream_clean` benign only after 30-day (configurable) window | unit | `go test ./internal/corpus/... -run TestSupersedingRecords` + `-run TestDownstreamCleanWindow` | ❌ W0 (23-03 T1) |
| STORE-01 | 23-01 (T1/T2) | Corpus NDJSON is append-only (records append; never truncated) | unit | `go test ./internal/corpus/... -run TestStoreAppendOnly` | ❌ W0 (23-01 T1) |
| STORE-02 | 23-01 (T1/T2) | `RedactRecordWithDefaults` before every write; AWS-key-shaped field absent from persisted NDJSON | unit | `go test ./internal/corpus/... -run TestStoreRedactsSecretsBeforeWrite` | ❌ W0 (23-01 T1) |
| STORE-03 | 23-01 (T1/T2) | Corpus file owner-only (0600 / Windows owner-DACL via `platform.SetOwnerOnly`) | unit | `go test ./internal/corpus/... -run TestStoreFilePermissions` | ❌ W0 (23-01 T1) |
| STORE-04 | 23-01 (T1/T2) | Records persist in push-envelope shape from the first write | unit | `go test ./internal/corpus/... -run TestStoreEmitsPushEnvelopeShape` | ❌ W0 (23-01 T1) |
| STORE-05 | 23-01 (T1/T3) | `repo_fingerprint`/`fleet_node_id` HMAC-SHA256 with per-install salt; same id+salt stable; different salts differ | unit | `go test ./internal/corpus/... -run TestFingerprintNonReversibility` | ❌ W0 (23-01 T1) |
| ENV-01 | 23-02 (T1/T2) | Local records emit in the frozen push-envelope shape; NO transport | unit (round-trip) | `go test ./internal/corpus/... -run TestPushEnvelopeEmitted` | ❌ W0 (23-02 T1) |
| ENV-02 | 23-02 (T1/T2) | `BuildPushEnvelope` errors on purge-class intent; `auto_purge` never emitted; tier/count frozen at emission | unit (negative) | `go test ./internal/corpus/... -run TestBuildPushEnvelopeRejectsPurge` | ❌ W0 (23-02 T1) |
| ENV-03 | 23-02 (T1/T3) | Fuzz/property gate: no constructed envelope escapes with a non-allowlisted `action_hint` (release BLOCKER) | fuzz | `go test -fuzz=FuzzBuildPushEnvelope -fuzztime=30s ./internal/corpus/...` | ❌ W0 (23-02 T1) |

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

- [ ] `internal/corpus/store_test.go` (new) — STORE-01/02/03/04 — **23-01 Task 1**
- [ ] `internal/corpus/fingerprint_test.go` (new) — STORE-05 — **23-01 Task 1**
- [ ] `internal/corpus/emitter_test.go` (new) — ADJ-04/05, ENV-01/02 — **23-02 Task 1**
- [ ] `internal/corpus/fuzz_test.go` (new) — `FuzzBuildPushEnvelope` stub (ENV-03) — **23-02 Task 1**
- [ ] `internal/corpus/adjudicator_test.go` (new) — ADJ-02/03/06/07 — **23-03 Task 1**
- [ ] `internal/check/handler_test.go` (existing) — add `TestCorpusWriteErrorDoesNotChangeExitCode` + `BenchmarkRunCheck` corpus case (ADJ-01) — **23-03 Task 1**

*Skeletons/stubs land as the FIRST task of each owning plan so no implementation task ships without an automated verify.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| BenchmarkRunCheck p99 on this Windows machine | ADJ-01 / LAUNCH-03 (proven in P25) | NDJSON append latency under Windows AV interception is environment-dependent; the automated benchmark asserts the bound but the maintainer should eyeball the reported p99 once | Run `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` and confirm reported p99 < 100 ms |

*All functional phase behaviors have automated verification; the entry above is an environment-sensitivity eyeball, not an unautomated requirement.*

---

## Validation Sign-Off

- [x] All tasks have an `<automated>` verify or a Wave 0 dependency (each plan's Task 1 front-loads the skeletons)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (every task carries an `<automated>` command)
- [x] Wave 0 covers all MISSING references (6 skeleton files mapped to owning-plan Task 1)
- [x] No watch-mode flags
- [x] Feedback latency < 60 s
- [x] `nyquist_compliant: true` set in frontmatter (task IDs reconciled to 23-01/02/03)

**Approval:** planner-reconciled 2026-06-13 (3 plans / 2 waves). `wave_0_complete` flips true once each plan's Task 1 skeletons are committed.
