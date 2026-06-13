---
phase: 23-corpus-store-adjudication-engine
verified: 2026-06-14T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 23: Corpus Store & Adjudication Engine — Verification Report

**Phase Goal:** Stand up the append-only local corpus store (as an `audit.Sink`) and the off-hot-path adjudication engine assigning the outcome layer with corroboration-gated confidence, emitting records in envelope shape.
**Verified:** 2026-06-14
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A synthetic incident records all four layers; two-source adjudication is `enforce`, one-source is `watch`, `source_count` from DISTINCT sources (ADJ-04/05) | VERIFIED | `TestSourceCountDedup` (3x Bumblebee → source_count:1, tier:"watch"); `TestConfidenceTierTable` (2 distinct sources → tier:"enforce"; single-source critical → "watch"); all PASS in fresh run |
| 2 | Records emit in push-envelope shape and persist append-only + owner-only; injected secret-shaped field is redacted (STORE-01/02/04, ENV-01) | VERIFIED | `TestStoreEmitsPushEnvelopeShape`, `TestStoreAppendOnly`, `TestStoreFilePermissions` (0600), `TestStoreRedactsSecretsBeforeWrite` (AKIAIOSFODNN7EXAMPLE absent from NDJSON); `TestPushEnvelopeEmitted` — all PASS |
| 3 | Adjudication runs off the hot path — `beekeeper check` is never blocked and `internal/policy` stays I/O-free (ADJ-01) | VERIFIED | `TestCorpusWriteErrorDoesNotChangeExitCode` PASS (block stays 1, allow stays 0 after corpus write error); `TestCorroborationImportsArePure` PASS; `BenchmarkRunCheck` ~22.8ms/op (well under 100ms gate); handler.go imports `internal/corpus` (store/fingerprint/emitter) but zero references to `adjudicator` or `RunAdjudicationBatch` (confirmed by grep) |
| 4 | `repo_fingerprint`/`fleet_node_id` are non-reversible HMAC values; two installs of the same repo differ (STORE-05) | VERIFIED | `TestFingerprintNonReversibility` PASS (salt-A ≠ salt-B for same repo path; same salt stable); `TestLoadOrCreateSalt` PASS (idempotent, 64-char hex salt persisted to state.json) |
| 5 | A fuzz/property test confirms no envelope escapes with a non-allowlisted `action_hint` (ENV-02/03) | VERIFIED | `FuzzBuildPushEnvelope` 20s run: 222,367 executions, zero failures; `TestBuildPushEnvelopeRejectsPurge` (8 subtests including "auto_purge", "PURGE", "Delete") all PASS; `auto_purge` is zero occurrences as a constructable literal in non-test corpus files (grep confirmed: 0 matches in production files) |

**Score:** 5/5 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/corpus/store.go` | StoreSink implementing audit.Sink; O_APPEND\|O_CREATE\|O_WRONLY 0600, RedactRecordWithDefaults-first, mutex-guarded encoder | VERIFIED | File exists, substantive (161 lines), NewStoreSink + Write (redaction-first Step 1) + Close + ResolveCorpusPath all implemented. Wired in handler.go via writeCorpusRecord. |
| `internal/corpus/fingerprint.go` | RepoFingerprint + FleetNodeID HMAC-SHA256 + per-install salt load/generate/persist | VERIFIED | File exists, substantive (103 lines), all three functions implemented using crypto/hmac+sha256+rand. Wired in handler.go via writeCorpusRecord. |
| `internal/corpus/store_test.go` | STORE-01/02/03/04 unit tests | VERIFIED | File exists; TestStoreAppendOnly, TestStoreRedactsSecretsBeforeWrite, TestStoreFilePermissions, TestStoreEmitsPushEnvelopeShape all PASS |
| `internal/corpus/fingerprint_test.go` | STORE-05 non-reversibility test | VERIFIED | File exists; TestFingerprintNonReversibility (6 subtests) + TestLoadOrCreateSalt all PASS |
| `internal/corpus/emitter.go` | MapToCorpusRecord + BuildPushEnvelope + AdjudicationResult type | VERIFIED | File exists, substantive (236 lines), all symbols present. corroborationTierAndCount delegates to policy.CorroborateOutcome (single-sourced). ActionHint assigned only from typed const. |
| `internal/corpus/signer.go` | Ed25519 keygen/sign + SigningBlock population | VERIFIED | File exists; LoadOrCreateSigningKey + SignEnvelope implemented. Key at 0600 + platform.SetOwnerOnly. TestSignEnvelopeRoundTrip PASS (ed25519.Verify confirms signature). |
| `internal/corpus/emitter_test.go` | ADJ-04/05 + ENV-01/02 unit tests | VERIFIED | File exists; TestSourceCountDedup, TestConfidenceTierTable (3 subtests), TestPushEnvelopeEmitted, TestBuildPushEnvelopeRejectsPurge (8 subtests) all PASS |
| `internal/corpus/fuzz_test.go` | FuzzBuildPushEnvelope ENV-03 property gate | VERIFIED | File exists; 16 seed entries including adversarial intent strings. 222,367 execs in 20s with zero failures. |
| `internal/corpus/adjudicator.go` | pure Adjudicate() + RunAdjudicationBatch() bounded batch pass + 6 adjudication_source values | VERIFIED | File exists, substantive (521 lines). 6 AdjSource* constants defined. Adjudicate() is pure (no I/O, no goroutines). RunAdjudicationBatch() has maxRecordsToScan=50000 cap + ctx deadline honoring. |
| `internal/corpus/adjudicator_test.go` | ADJ-02/03/06/07 unit tests | VERIFIED | File exists; TestAdjudicationTrueLabelTransition, TestAdjudicationSources, TestWasCorrectAndResolvedAt, TestSupersedingRecords, TestDownstreamCleanWindow all PASS |
| `internal/check/handler_test.go` (extended) | TestCorpusWriteErrorDoesNotChangeExitCode + BenchmarkRunCheck | VERIFIED | Both present and verified. TestCorpusWriteErrorDoesNotChangeExitCode PASS. BenchmarkRunCheck: ~22.8ms/op (hardware: Intel Celeron N4020, 1.10GHz). |
| `cmd/beekeeper/catalogs_daemon.go` (modified) | bounded 5s adjudication batch pass in runCatalogsSync | VERIFIED | RunAdjudicationBatch called with context.WithTimeout(cmd.Context(), 5s) before the HTTP catalog fetch. Error logged to stderr, sync continues. Confirmed by code inspection. |
| `internal/audit/sink.go` (modified) | NewMultiSinkWithCorpus for daemon/gateway surfaces | VERIFIED | Function exists (147 lines), takes corpusSink as audit.Sink interface (avoids import cycle). Three tests pass: TestNewMultiSinkWithCorpusFanout, TestNewMultiSinkWithCorpusErrorDoesNotBlockFileSink, TestNewMultiSinkWithCorpusNilCorpusSink. |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/corpus/store.go` | `internal/audit.RedactRecordWithDefaults` | First call in StoreSink.Write before any marshal | VERIFIED | Line 90: `redacted := audit.RedactRecordWithDefaults(rec)` — first statement in Write() body |
| `internal/corpus/store.go` | `internal/platform.SetOwnerOnly` | 0600 enforcement after open + re-enforce per write | VERIFIED | Called in NewStoreSink (line 64) and in Write (line 120) |
| `internal/corpus/fingerprint.go` | `internal/catalog.WatchState.CorpusLocalSalt` | LoadState/SaveState round-trip for per-install salt | VERIFIED | LoadOrCreateSalt reads `st.CorpusLocalSalt`; if empty generates 32 bytes via crypto/rand and saves via catalog.SaveState |
| `internal/check/handler.go (writeAuditWithAC)` | `internal/corpus.NewStoreSink` | corpus write gated on cfg.Corpus.Enabled, error never changes exit code | VERIFIED | Lines 542-544: if cfg.Corpus.Enabled → writeCorpusRecord(rec, cfg, stateDir). writeCorpusRecord opens NewStoreSink per-invocation, every error path returns without altering the decision. |
| `cmd/beekeeper/catalogs_daemon.go (runCatalogsSync)` | `internal/corpus.RunAdjudicationBatch` | bounded 5s-deadline batch pass when cfg.Corpus.Enabled | VERIFIED | Lines 82-121: RunAdjudicationBatch called with batchCtx (5s deadline). Non-nil error logged to stderr; sync continues. |
| `internal/corpus/adjudicator.go` | `internal/policy.CorroborateOutcome` | source_count + confidence_tier on outcome records (read-only pure dep) | VERIFIED | corroborationTierAndCount() in emitter.go wraps CorroborateOutcome; Adjudicate() calls this helper. TestCorroborationImportsArePure PASS — policy package is I/O-free. |
| `internal/corpus/emitter.go` | `internal/corpus.BehaviorSigHash` | behavior_signature_hash population from frozen normalizer | VERIFIED | Line 135: `behaviorHash := BehaviorSigHash(actionType, targetResource, networkDestination)` in MapToCorpusRecord |
| `internal/corpus/emitter.go` | `internal/corpus.ActionHintWatchAndBlock` | the only action_hint a built envelope may carry | VERIFIED | Line 230 in BuildPushEnvelope: `ActionHint: ActionHintWatchAndBlock` — typed const, never assigned from runtime string |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `internal/corpus/store.go` StoreSink.Write | rec (audit.AuditRecord) | Caller passes redacted AuditRecord from writeAuditWithAC | Yes — audit.FromDecision produces record from real ToolCall + Decision | FLOWING |
| `internal/corpus/adjudicator.go` RunAdjudicationBatch | allRecords ([]CorpusRecord) | bufio.Scanner over corpus NDJSON file; real OS file read | Yes — reads actual persisted corpus records | FLOWING |
| `cmd/beekeeper/catalogs_daemon.go` batch pass | idx (policy.MultiCatalogLookup) | catalog.OpenIndex → catalog.NewMultiIndex (mmap bumblebee.idx) | Yes — real mmap index; nil on missing index → catalog_confirmation finds nothing (safe) | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full suite passes | `go test ./... -count=1` | All 27 packages OK (0 failures) | PASS |
| corpus package unit tests | `go test ./internal/corpus/... -v -count=1` | All 29 tests + 16 fuzz seeds PASS | PASS |
| ADJ-01 fail-closed test | `go test ./internal/check/... -run TestCorpusWriteErrorDoesNotChangeExitCode -v -count=1` | PASS; block exits 1, allow exits 0 even after corpus error | PASS |
| policy purity gate | `go test ./internal/policy/... -run TestCorroborationImportsArePure -v -count=1` | PASS | PASS |
| Build | `go build ./...` | exit 0 (no output) | PASS |
| Vet | `go vet ./...` | exit 0 (no output) | PASS |
| Zero new deps | `go mod tidy && git diff --exit-code go.mod` | exit 0 (no change) | PASS |
| SCHEMA-04 tripwire | `grep -rl "auto_purge" internal/corpus/ | grep -v "_test.go" | wc -l` | 0 (deny-list entry built via strings.Join, not literal) | PASS |
| BenchmarkRunCheck ADJ-01 latency | `go test -bench=BenchmarkRunCheck -benchtime=3s ./internal/check/...` | ~22.8ms/op on Intel Celeron N4020 1.10GHz — well under 100ms gate | PASS |
| FuzzBuildPushEnvelope ENV-03 | `go test -fuzz=FuzzBuildPushEnvelope -fuzztime=20s ./internal/corpus/...` | 222,367 executions, zero failures | PASS |

---

### Probe Execution

Step 7c: SKIPPED — no `scripts/*/tests/probe-*.sh` found in the phase directory or referenced in PLAN/SUMMARY files. The validation is probe-free; the evaluator gate uses Go test commands directly.

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| ADJ-01 | 23-03 | Adjudication off hot path; corpus error never changes exit code; policy I/O-free | SATISFIED | TestCorpusWriteErrorDoesNotChangeExitCode PASS; TestCorroborationImportsArePure PASS; BenchmarkRunCheck ~22.8ms/op; handler.go has zero adjudicator imports |
| ADJ-02 | 23-03 | TrueLabel ∈ {malicious, benign, policy_correct, unresolved}; initial = unresolved | SATISFIED | TestAdjudicationTrueLabelTransition PASS |
| ADJ-03 | 23-03 | 6 adjudication_source values with documented confidence mapping | SATISFIED | TestAdjudicationSources PASS; all 6 AdjSource* consts defined |
| ADJ-04 | 23-02 | source_count from DISTINCT sources, never event count | SATISFIED | TestSourceCountDedup PASS; corroborationTierAndCount delegates to CorroborateOutcome |
| ADJ-05 | 23-02 | confidence_tier: 1 source → watch; ≥2 → enforce; single-source critical stays watch | SATISFIED | TestConfidenceTierTable (3 subtests) PASS including 2FA-invariant case |
| ADJ-06 | 23-03 | was_correct from true_label vs verdict; resolved_at RFC3339 when leaving unresolved | SATISFIED | TestWasCorrectAndResolvedAt PASS |
| ADJ-07 | 23-03 | Append-only superseding records; downstream_clean benign only after 30-day window | SATISFIED | TestSupersedingRecords + TestDownstreamCleanWindow PASS |
| STORE-01 | 23-01 | Append-only corpus NDJSON as audit.Sink | SATISFIED (see observation) | TestStoreAppendOnly PASS; StoreSink implements audit.Sink. Note: REQUIREMENTS.md prose says "all six source surfaces gain corpus writing with no per-surface code change" but v1 scope wires the check (hook) surface via writeAuditWithAC; NewMultiSinkWithCorpus is the mechanism for other surfaces in future phases. This deviation is explicitly documented in 23-03 as OQ-1 "Option C" and is consistent with the ROADMAP SC-2 gate which checks append-only + owner-only + redaction (not surface count). |
| STORE-02 | 23-01 | RedactRecordWithDefaults before every write | SATISFIED | TestStoreRedactsSecretsBeforeWrite PASS; RedactRecordWithDefaults is Step 1 in StoreSink.Write |
| STORE-03 | 23-01 | Owner-only 0600 / Windows DACL | SATISFIED | TestStoreFilePermissions PASS; platform.SetOwnerOnly called on open and re-enforced per write |
| STORE-04 | 23-01 | Records in push-envelope shape from first write | SATISFIED | TestStoreEmitsPushEnvelopeShape PASS; PushEnvelope non-nil in StoreSink.Write output |
| STORE-05 | 23-01 | HMAC-SHA256 non-reversible repo_fingerprint/fleet_node_id with per-install salt | SATISFIED | TestFingerprintNonReversibility PASS; TestLoadOrCreateSalt PASS |
| ENV-01 | 23-02 | Records in frozen push-envelope shape; no transport | SATISFIED | TestPushEnvelopeEmitted PASS; Signing nil in v1; no transport code |
| ENV-02 | 23-02 | BuildPushEnvelope errors on purge-class intent; auto_purge never emitted; tier/count frozen | SATISFIED | TestBuildPushEnvelopeRejectsPurge (8 subtests) PASS; ActionHint always typed const |
| ENV-03 | 23-02 | Fuzz/property gate: no non-allowlisted action_hint escapes | SATISFIED | FuzzBuildPushEnvelope 222,367 execs zero failures |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/corpus/store.go` | 94-106 | Minimal PushEnvelope placeholder in StoreSink.Write (SourceCount:0, BehaviorSigHash empty) | INFO | Documented intentional seam; handler.go bypasses this via writeCorpusRecordDirect which uses MapToCorpusRecord for the full four-layer record. The StoreSink.Write path still satisfies STORE-04 (push_envelope non-nil from first write). Seam comment present. |
| `internal/corpus/store.go` | 607-619 | handler.go opens NewStoreSink then immediately bypasses it via writeCorpusRecordDirect | INFO | Documented deviation (23-03-SUMMARY Deviation 1): StoreSink.Write would re-wrap the already-fully-mapped CorpusRecord with TrueLabel:"unresolved". Direct write preserves the full emitter output. No correctness issue; tested by BenchmarkRunCheck and TestCorpusWriteErrorDoesNotChangeExitCode. |
| `.planning/REQUIREMENTS.md` | Lines 28, 73 | OQ-3 prose says "Background goroutine in the long-lived catalogs daemon"; implementation correctly uses bounded batch pass in runCatalogsSync | INFO | Stale prose, acknowledged in phase instructions as a known drift. Implementation is correct. The REQUIREMENTS.md `Pending` status (not "Done") for all 16 requirements is a documentation-update gap, not an implementation gap. |

No TBD/FIXME/XXX markers found in corpus or modified check/audit files.

---

### Human Verification Required

#### 1. BenchmarkRunCheck p99 on production machine

**Test:** Run `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` on the target machine and eyeball the reported latency.
**Expected:** Reported ns/op converts to < 100ms. On the dev machine (Intel Celeron N4020, 1.10GHz) the result was ~22.8ms/op.
**Why human:** NDJSON append latency under Windows Defender AV interception is environment-dependent. The benchmark runs and reports a measurement, but the p99 gate on the final deployment machine (LAUNCH-03) needs human confirmation. The automated benchmark confirms the mechanism is exercised and reports plausible latency.

---

### Gaps Summary

No gaps block the phase goal. All five ROADMAP success criteria are verifiably true in the codebase:

1. The four-layer synthetic incident round-trip is proven by the adjudicator tests (TestAdjudicationTrueLabelTransition confirms unresolved → malicious transition; TestSupersedingRecords confirms append-only superseding with same ClusterID). The source-count / confidence-tier logic is proven by TestSourceCountDedup and TestConfidenceTierTable.

2. The push-envelope shape, append-only semantics, owner-only permissions, and redaction-first invariant are each proven by dedicated unit tests.

3. The hot-path isolation is proven three ways: (a) handler.go has zero adjudicator imports (grep: 0); (b) TestCorpusWriteErrorDoesNotChangeExitCode proves fail-closed semantics; (c) BenchmarkRunCheck at ~22.8ms/op is well within the 100ms gate.

4. HMAC non-reversibility is proven by TestFingerprintNonReversibility (same repo path + different salts → different fingerprints).

5. The ENV-03 fuzz gate ran 222,367 executions with zero failures.

The STORE-01 surface-coverage observation (check surface wired, other surfaces have the mechanism ready but not called) is documented as a v1 scope decision (OQ-1 Option C in 23-03) and is consistent with the ROADMAP SC-2 contract. It does not constitute a blocker.

The REQUIREMENTS.md OQ-3 prose drift and the `Pending` status on all 16 requirements are documentation-only gaps that do not affect the implementation.

---

_Verified: 2026-06-14_
_Verifier: Claude (gsd-verifier)_
