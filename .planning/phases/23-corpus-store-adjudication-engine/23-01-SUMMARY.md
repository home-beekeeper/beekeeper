---
phase: 23-corpus-store-adjudication-engine
plan: "01"
subsystem: corpus
tags: [corpus, store, fingerprint, hmac, ndjson, permissions, phase-23, store-01, store-02, store-03, store-04, store-05, env-01]
dependency_graph:
  requires:
    - internal/audit (RedactRecordWithDefaults, Sink interface, AuditRecord, Writer pattern)
    - internal/platform (SetOwnerOnly, StateDir)
    - internal/catalog (LoadState, SaveState, WatchState)
    - internal/config (CorpusConfig)
    - internal/corpus/types.go (CorpusRecord, PushEnvelope — Phase 22 frozen)
  provides:
    - internal/corpus/store.go (StoreSink, ResolveCorpusPath)
    - internal/corpus/fingerprint.go (RepoFingerprint, FleetNodeID, LoadOrCreateSalt)
    - internal/corpus/store_test.go (STORE-01/02/03/04 test suite)
    - internal/corpus/fingerprint_test.go (STORE-05 + salt idempotency tests)
    - internal/config/config.go (CorpusConfig.DownstreamCleanDays field + CorpusDownstreamCleanDays accessor)
    - internal/catalog/state.go (WatchState.CorpusLocalSalt field)
  affects:
    - 23-02 (emitter imports StoreSink and fingerprint functions)
    - 23-03 (adjudicator imports StoreSink; reads CorpusDownstreamCleanDays)
tech_stack:
  added:
    - crypto/hmac (stdlib HMAC-SHA256 for non-reversible fingerprints)
    - crypto/rand (stdlib CSPRNG for per-install salt generation)
    - crypto/sha256 (stdlib SHA-256 via HMAC)
    - encoding/hex (stdlib salt encoding)
  patterns:
    - audit.Writer pattern mirrored: O_APPEND|O_CREATE|O_WRONLY 0600 + platform.SetOwnerOnly + sync.Mutex + json.Encoder
    - Redaction-first invariant: RedactRecordWithDefaults is the first call in StoreSink.Write
    - HMAC-SHA256 with per-install hex-encoded salt (LoadOrCreateSalt → state.json)
    - catalog.LoadState/SaveState round-trip for salt persistence
key_files:
  created:
    - internal/corpus/store.go
    - internal/corpus/fingerprint.go
    - internal/corpus/store_test.go
    - internal/corpus/fingerprint_test.go
  modified:
    - internal/config/config.go (added DownstreamCleanDays field + CorpusDownstreamCleanDays accessor)
    - internal/config/config_test.go (added TestCorpusDownstreamCleanDays)
    - internal/catalog/state.go (added CorpusLocalSalt field)
decisions:
  - "StoreSink mirrors audit.Writer exactly (long-lived file + mutex + encoder) rather than per-record open — Pitfall 4 avoided"
  - "fingerprint.go created in same commit as Task 2 because both files are in package corpus; fingerprint_test.go blocked compilation of store_test.go"
  - "PushEnvelope in StoreSink.Write is a minimal non-nil placeholder; 23-02 seam comment marks the swap point for MapToCorpusRecord"
  - "ResolveCorpusPath uses filepath.Separator boundary check (not HasPrefix alone) to avoid sibling-dir false positives (T-23-04)"
  - "hmacHex decodes hex salt with raw-byte fallback; callers always pass hex-encoded salts via LoadOrCreateSalt"
metrics:
  duration: "8 minutes"
  completed: "2026-06-13"
  tasks_completed: 3
  tasks_total: 3
  files_created: 4
  files_modified: 3
---

# Phase 23 Plan 01: Corpus Store & Fingerprint — Summary

**One-liner:** Append-only owner-only redaction-first NDJSON StoreSink (audit.Sink) + HMAC-SHA256 RepoFingerprint/FleetNodeID with per-install salt persisted to state.json

## What Was Built

### internal/corpus/store.go

- `NewStoreSink(corpusPath string) (*StoreSink, error)` — opens corpus file with `O_APPEND|O_CREATE|O_WRONLY 0600`, calls `platform.SetOwnerOnly` immediately, returns long-lived sink with `sync.Mutex` + `json.Encoder`
- `StoreSink.Write(rec audit.AuditRecord) error` — redaction-first (`RedactRecordWithDefaults` is the FIRST operation), maps to `CorpusRecord` with `TrueLabel:"unresolved"` + non-nil `PushEnvelope` placeholder, encodes under mutex, re-enforces owner-only permissions (T-23-03)
- `StoreSink.Close() error` — closes file under mutex
- `ResolveCorpusPath(cfg config.CorpusConfig, stateDir string) (string, error)` — defaults to `stateDir/corpus/beekeeper-corpus.ndjson`; validates `cfg.Path` is under `stateDir` using separator-bounded prefix check (T-23-04)

### internal/corpus/fingerprint.go

- `RepoFingerprint(repoPath, salt string) string` — HMAC-SHA256(repoPath, saltBytes); 64-char hex; non-reversible without salt (T-23-02)
- `FleetNodeID(hostname, goos, salt string) string` — HMAC-SHA256(`hostname\x00goos`, saltBytes); NUL separator prevents prefix collisions
- `LoadOrCreateSalt(stateFile string) (string, error)` — reads `WatchState.CorpusLocalSalt` from `state.json`; generates 32 bytes via `crypto/rand` on first run; persists via `catalog.SaveState`; idempotent

### Config/State seam additions

- `internal/config/config.go`: `CorpusConfig.DownstreamCleanDays int` (OQ-1: 30-day configurable window) + `Config.CorpusDownstreamCleanDays()` accessor (default 30)
- `internal/catalog/state.go`: `WatchState.CorpusLocalSalt string` (per-install salt storage; `omitempty` for backward compatibility)

## Verification Results

| Test | Result |
|------|--------|
| `TestStoreAppendOnly` (STORE-01) | PASS |
| `TestStoreRedactsSecretsBeforeWrite` (STORE-02) | PASS |
| `TestStoreFilePermissions` (STORE-03) | PASS |
| `TestStoreEmitsPushEnvelopeShape` (STORE-04) | PASS |
| `TestFingerprintNonReversibility` (STORE-05) | PASS |
| `TestLoadOrCreateSalt` (STORE-05 salt) | PASS |
| `TestCorpusConfig` (existing) | PASS |
| `TestCorpusDownstreamCleanDays` (new) | PASS |
| `go build ./...` | EXIT 0 |
| `go vet ./internal/corpus/...` | EXIT 0 |
| `go mod tidy && git diff --exit-code go.mod` | NO CHANGE |
| `go list -f '{{.Imports}}' ./internal/corpus/` | No net/* imports |

## Deviations from Plan

**1. [Rule 3 - Blocking] fingerprint.go created alongside store.go (Task 2) rather than as a separate Task 3 commit**

- **Found during:** Task 2 GREEN phase
- **Issue:** `fingerprint_test.go` (created in Task 1) is in `package corpus` and references `RepoFingerprint`, `FleetNodeID`, and `LoadOrCreateSalt`. When `go test` compiled the package to run STORE tests, it also compiled `fingerprint_test.go` and failed on undefined symbols — blocking Task 2 verification.
- **Fix:** Created `fingerprint.go` before running Task 2's test verification. This allowed both Task 2 STORE tests and Task 3 fingerprint tests to pass GREEN in the same run.
- **Impact:** Task 3 is functionally complete; its implementation was done in one pass with Task 2 rather than in a separate step. The commit history reflects this: fingerprint.go has its own separate commit (97f87fb) after store.go (b043faa).

None — plan executed as designed except the ordering note above.

## Threat Mitigations Verified

| Threat | Mitigation | Test |
|--------|-----------|------|
| T-23-01: Credential-shaped strings in corpus NDJSON | `RedactRecordWithDefaults` is the FIRST call in `StoreSink.Write` | `TestStoreRedactsSecretsBeforeWrite` (AKIAIOSFODNN7EXAMPLE absent from persisted bytes) |
| T-23-02: Reversible `repo_fingerprint` | HMAC-SHA256 with per-install random salt — never bare SHA-256 | `TestFingerprintNonReversibility` (salt-A ≠ salt-B) |
| T-23-03: Agent tampering with corpus file | 0600 / owner-DACL via `platform.SetOwnerOnly` on open + re-enforced per write | `TestStoreFilePermissions` |
| T-23-04: Corpus file outside StateDir | `ResolveCorpusPath` rejects paths not under stateDir | boundary check in `ResolveCorpusPath` |
| T-23-SC: Supply-chain (new packages) | Zero new go.mod entries — stdlib only | `go mod tidy && git diff --exit-code go.mod` shows no change |

## Known Stubs

One intentional seam exists in `store.go`:

- **`StoreSink.Write` PushEnvelope construction** (lines 98-103): creates a minimal non-nil `PushEnvelope` placeholder (`TrueLabel:"unresolved"`, `ConfidenceTier:"watch"`, `SourceCount:0`, `ActionHint:ActionHintWatchAndBlock`). This satisfies STORE-04 (push_envelope non-nil from first write) but does NOT populate the full `EnvelopeSignature`, IOC block, or `BehaviorSigHash`. Plan 23-02 replaces this inline construction by calling `MapToCorpusRecord(redacted, cfg)` and `BuildPushEnvelope`. A doc comment in `store.go` marks the exact swap point for the 23-02 executor.

This stub is intentional and fully documented. It does not prevent the plan's goal (STORE-01/02/03/04/05 all pass). The full emitter is 23-02's deliverable.

## Commits

| Task | Hash | Message |
|------|------|---------|
| Task 1 | 644c243 | feat(23-01): Wave-0 test skeletons + DownstreamCleanDays + CorpusLocalSalt |
| Task 2 | b043faa | feat(23-01): StoreSink implementing audit.Sink (STORE-01/02/03/04) |
| Task 3 | 97f87fb | feat(23-01): HMAC-SHA256 fingerprints + per-install salt persistence (STORE-05) |

## Self-Check: PASSED

- [x] `internal/corpus/store.go` — FOUND
- [x] `internal/corpus/fingerprint.go` — FOUND
- [x] `internal/corpus/store_test.go` — FOUND
- [x] `internal/corpus/fingerprint_test.go` — FOUND
- [x] Commit 644c243 — FOUND
- [x] Commit b043faa — FOUND
- [x] Commit 97f87fb — FOUND
- [x] `go test ./internal/corpus/... ./internal/config/... ./internal/catalog/... -count=1` — EXIT 0
- [x] `go build ./...` — EXIT 0
- [x] `go mod tidy && git diff --exit-code go.mod` — NO CHANGE
