---
phase: 23-corpus-store-adjudication-engine
reviewed: 2026-06-14T00:00:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - cmd/beekeeper/catalogs_daemon.go
  - internal/audit/sink.go
  - internal/audit/sink_test.go
  - internal/catalog/state.go
  - internal/check/handler.go
  - internal/check/handler_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/corpus/adjudicator.go
  - internal/corpus/adjudicator_test.go
  - internal/corpus/emitter.go
  - internal/corpus/emitter_test.go
  - internal/corpus/fingerprint.go
  - internal/corpus/fingerprint_test.go
  - internal/corpus/fuzz_test.go
  - internal/corpus/signer.go
  - internal/corpus/signer_test.go
  - internal/corpus/store.go
  - internal/corpus/store_test.go
findings:
  critical: 1
  warning: 6
  info: 5
  total: 12
status: issues_found
---

# Phase 23: Code Review Report

**Reviewed:** 2026-06-14T00:00:00Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Phase 23 wires the corpus store, emitter, signer, fingerprinting, and the
adjudication engine. The architecture honors the project hard rules well: the
hook hot path (`writeCorpusRecord`) is strictly best-effort and never alters the
exit code; `RunAdjudicationBatch` lives only in `runCatalogsSync` and is wrapped
fail-soft; `internal/policy` is untouched (the corpus package is the impure
consumer); redaction runs first in `StoreSink.Write`; files are opened 0600 with
`SetOwnerOnly`; the ENV-02 purge gate and the SCHEMA-04 typed-const ActionHint
guard are robust (and fuzzed). No new external deps were introduced.

The independent bug pass surfaced one BLOCKER (a cross-process read-modify-write
race on the shared `state.json` salt that can corrupt the watch daemon's source
state and silently rotate the HMAC salt), six warnings (a dead/misleading
`StoreSink` lazy-open in the adjudicator, non-deterministic batch iteration that
weakens the ctx-deadline guarantee, an unredacted superseding-record path, a
fingerprint salt-fallback that masks a programming error, an O(n²) cluster
recount, and a Windows append-atomicity assumption), and several quality items.

## Critical Issues

### CR-01: Cross-process read-modify-write race on shared `state.json` can corrupt watch-daemon source state and rotate the HMAC salt

**File:** `internal/corpus/fingerprint.go:79-102` (`LoadOrCreateSalt`), interacting with `internal/check/handler.go:586-587`, `internal/catalog/state.go:133-149` (`SaveState`)

**Issue:**
`LoadOrCreateSalt` does an unsynchronized load → mutate → `SaveState` (full-file
overwrite via temp-file + rename) on `~/.beekeeper/state.json`. The same file is
concurrently written by the catalog watch daemon (`SourceState` Hash/Count/
Degraded/ETag/LastSuccess) and read-modified-written by `runCatalogsSync`
(`catalogs_daemon.go:144-159`). `beekeeper check` is a one-shot process invoked
once per agent tool call, so many `check` processes can run concurrently (sub-agents,
parallel tool calls), and each calls `LoadOrCreateSalt` from `writeCorpusRecord`
when `cfg.Corpus.Enabled`.

Two concrete failure modes:

1. **Salt double-generation / rotation.** Two concurrent first-run `check`
   processes both observe `CorpusLocalSalt == ""`, both generate a *different*
   random salt, and both `SaveState`. Last-writer-wins. The losing process has
   already written corpus records fingerprinted under salt A while the persisted
   salt is now B. RepoFingerprint/FleetNodeID are then unstable across the very
   records the non-reversibility design (T-23-02) assumes are stable per install.

2. **Lost `SourceState` updates / Degraded clobber.** `SaveState` marshals and
   rewrites the *entire* `WatchState`. A `check` process that loaded state before
   the watch daemon wrote a `Degraded` mark (CTLG-08) will rename its stale copy
   over the fresh one, silently dropping the degradation flag — directly
   undermining the anti-poisoning sanity gate. `state.go:127-149` even documents
   that the atomic write exists specifically so a crash "never leaves a
   partially-written state.json that could mask a prior Degraded mark," but
   atomicity of a single write does not protect against a stale-read overwrite.

**Fix:**
Salt provisioning must not happen on the concurrent hot path via a full-state
rewrite. Options, in order of preference:

- Provision the salt **once at install/first-run** (e.g. in `hooks install` /
  `offerCatalogSyncDaemon`) so `check` only ever *reads* it, never writes
  `state.json`. `writeCorpusRecord` should treat a missing salt as "corpus
  fingerprinting unavailable, log + skip fingerprint" rather than generating and
  persisting one inline.
- If lazy creation must stay, store the salt in its **own** owner-only file
  (e.g. `StateDir()/corpus/salt`) created with `O_CREATE|O_EXCL` so the first
  writer wins atomically and concurrent writers fall back to reading the
  existing file — never touching the shared `state.json`.
- At minimum, guard the `state.json` read-modify-write with an OS file lock
  (advisory lock / `O_EXCL` lock file) shared across `LoadOrCreateSalt`,
  `runCatalogsSync`, and the watch daemon, and re-read under the lock before
  mutating.

## Warnings

### WR-01: `RunAdjudicationBatch` opens a `StoreSink` it never writes through, then closes it — dead resource churn and a misleading invariant

**File:** `internal/corpus/adjudicator.go:304-443`

**Issue:**
The batch lazily opens `sink = NewStoreSink(corpusPath)` on the first record that
needs a superseding entry (lines 398-403), but every actual write goes through
`appendCorpusRecord` (line 430), never through `sink`. `NewStoreSink` opens an
`O_APPEND` handle *and* runs `platform.SetOwnerOnly` (a DACL syscall on Windows),
all for a handle that is only ever `Close()`d. Worse, on Windows the unused
`sink` holds an open write handle to `corpusPath` at the same time
`appendCorpusRecord` opens its own `O_APPEND|O_WRONLY` handle to the same path —
an unnecessary second concurrent writer that the file-sharing comment elsewhere
(line 280) warns about. The `sink != nil { sink.Close() }` cleanups (lines
312-314, 432-434, 440-442) maintain an invariant that has no functional purpose.

**Fix:**
Remove the `StoreSink` lazy-open machinery entirely from `RunAdjudicationBatch`
and rely solely on `appendCorpusRecord`. Delete `sink`, `openErr`, the
lazy-open block, and the three `sink.Close()` cleanups. This also removes the
spurious second open handle.

### WR-02: Non-deterministic `map` iteration over `latestByCluster` makes the ctx-deadline "writes whatever completed" guarantee non-reproducible and can starve clusters

**File:** `internal/corpus/adjudicator.go:309` (`for _, rec := range latestByCluster`)

**Issue:**
The batch iterates a Go `map`, whose order is randomized per run. The documented
contract (lines 233-234) is that on ctx cancellation the batch "returns ctx.Err()
(writing whatever completed)." With random iteration order, *which* clusters get
adjudicated before the 5s deadline is nondeterministic across runs. A corpus
large enough that the batch routinely hits the deadline could perpetually
adjudicate a random subset and never make guaranteed forward progress on the
oldest unresolved clusters. It also makes the behavior untestable/flaky for any
test that relies on ordering, and makes superseding-record output order
non-reproducible (a reproducibility concern given the project's reproducible-build
discipline).

**Fix:**
Collect cluster keys into a slice and sort them (e.g. by the record's
`Timestamp`, oldest first, or lexically by ClusterID) before the processing loop,
so the deadline always advances the same deterministic frontier and the oldest
unresolved incidents are adjudicated first.

### WR-03: Superseding records are appended without re-running redaction

**File:** `internal/corpus/adjudicator.go:430,446-468` (`appendCorpusRecord`)

**Issue:**
`StoreSink.Write` documents redaction-first as a "non-negotiable" invariant
(store.go:88-90), but `appendCorpusRecord` marshals and appends the
`CorpusRecord` with no redaction step. The mitigating assumption is that the
record was already redacted when first written by `writeCorpusRecord` /
`StoreSink.Write`. That holds for records produced by Beekeeper's own writers,
but `RunAdjudicationBatch` reads arbitrary NDJSON lines off disk
(`json.Unmarshal`, line 270) and re-emits a shallow copy. If the corpus file is
ever appended to by another tool/version, or a future field carries unredacted
content, the superseding path silently persists it. Defense-in-depth here is
cheap and the redaction-first invariant is supposed to be uniform.

**Fix:**
Run `audit.RedactRecordWithDefaults(superseding.AuditRecord)` before
`appendCorpusRecord` (or route superseding writes through a shared
redact-then-marshal helper), so every write path — store and adjudicator —
honors the same redaction-first contract.

### WR-04: `hmacHex` silently falls back to raw-string key on invalid hex salt, masking a programming error

**File:** `internal/corpus/fingerprint.go:51-65`

**Issue:**
When `hex.DecodeString(salt)` fails or yields zero bytes, `hmacHex` uses
`[]byte(salt)` as the HMAC key. The doc-comment for `RepoFingerprint`
(fingerprint.go:21-24) states an empty/invalid salt "is a programming error" and
"MUST NOT be used in production," yet the code quietly proceeds and emits a
*stable-looking* fingerprint that is keyed by a non-secret string (or, for empty
salt, by an empty key). That produces reversible fingerprints (dictionary attack
on repo paths becomes feasible — the exact T-23-02 threat this design exists to
prevent) with no signal that anything is wrong. `LoadOrCreateSalt` always returns
64-char hex, so the fallback only fires on misuse — which is precisely when you
want a loud failure, not a silent weak-crypto downgrade.

**Fix:**
Either return an error from the fingerprint helpers on an undecodable/empty salt
(propagate to the caller, which logs + skips corpus fingerprinting), or at
minimum require a non-empty decoded key and `panic`/log-and-skip on violation.
Do not silently key HMAC with attacker-guessable bytes.

### WR-05: Per-record O(n²) cluster-occurrence recount in the downstream_clean path

**File:** `internal/corpus/adjudicator.go:358-379`

**Issue:**
For every unresolved record that reaches the downstream_clean branch, the code
re-scans all `allRecords` to count occurrences of its ClusterID. With M
unresolved records and N total records this is O(M·N). N is capped at
`maxRecordsToScan` (50,000), so worst case is ~2.5 billion comparisons inside a
5s deadline — the batch will simply time out and silently under-adjudicate via
WR-02's nondeterministic frontier. (Flagged as a correctness/robustness issue,
not pure perf: it directly interacts with the bounded-deadline contract.)

**Fix:**
Build a single `map[string]int` cluster-occurrence count in one pass over
`allRecords` (right after Step 2's `latestByCluster` collapse) and look it up in
O(1) per record.

### WR-06: Corpus append relies on O_APPEND write atomicity that is not guaranteed on Windows for the documented record sizes

**File:** `internal/check/handler.go:561-563,631-646`; `internal/corpus/adjudicator.go:457-467`

**Issue:**
Both `writeCorpusRecordDirect` and `appendCorpusRecord` assert that a single
`O_APPEND` write of a `<4KB` NDJSON record is atomic, and `writeCorpusRecord`'s
comment (handler.go:561-563) leans on this for concurrent one-shot `check`
processes. POSIX guarantees append atomicity up to `PIPE_BUF`/filesystem limits;
Windows (the stated primary dev platform) provides no such guarantee for
`FILE_APPEND_DATA` writes from multiple processes — interleaved/partial writes
can corrupt an NDJSON line. Records also are not hard-capped at 4KB: a tool call
with large `SentryFilesAccessed`/`Reason` content can exceed it. A torn line is
silently dropped by the reader (`json.Unmarshal` skip on malformed line,
adjudicator.go:270-272), so corruption is invisible but lossy.

**Fix:**
Document the platform caveat and, for the concurrent `check` path, serialize
appends with an OS file lock (or a single `O_APPEND` write of the full
`record+"\n"` buffer guarded by a cross-process lock) rather than asserting
atomicity. At minimum, bound record size before write and assert the buffer is
written in a single `f.Write` call (already true) with a documented size cap.

## Info

### IN-01: `RunAdjudicationBatch` early-returns `ctx.Err()` as a non-nil error for the normal deadline case, forcing the caller to log an expected condition as an error

**File:** `internal/corpus/adjudicator.go:310-316`; caller `cmd/beekeeper/catalogs_daemon.go:117-120`

**Issue:** A 5s-deadline hit is an expected, designed outcome ("writes whatever
completed"), but it surfaces as `context.DeadlineExceeded` which
`runCatalogsSync` prints to stderr as `corpus adjudication batch: ...`. Operators
will see a scary-looking error on every large-corpus sync.

**Fix:** Return `nil` (or a sentinel that the caller treats as benign) when the
batch stops solely due to ctx deadline after writing partial progress, reserving
the error return for genuine I/O failures.

### IN-02: Duplicated `stateFile` computation in `runCatalogsSync`

**File:** `cmd/beekeeper/catalogs_daemon.go:61` and `:115`

**Issue:** `stateFile := filepath.Join(stateDir, "state.json")` is computed at
line 61 and shadowed/recomputed identically at line 115 inside the corpus block.
The inner one is redundant and the shadowing is mildly confusing.

**Fix:** Reuse the outer `stateFile`; delete line 115.

### IN-03: `appendCorpusRecord` and `writeCorpusRecordDirect` are near-identical duplicates split only to dodge an import

**File:** `internal/corpus/adjudicator.go:446-468` and `internal/check/handler.go:625-646`

**Issue:** Two byte-for-byte-similar "marshal + O_APPEND + write line" helpers
exist because handler.go must not import the adjudicator (ADJ-01/Pitfall 3). The
duplication is justified by the import constraint but should be consolidated into
a third location (e.g. a small unexported writer in a corpus file that does not
pull in adjudicator symbols) to avoid drift (e.g. WR-03's redaction fix needs to
land in both).

**Fix:** Extract one shared `appendCorpusRecordLine(path, rec)` in store.go (no
adjudicator deps) and call it from both sites.

### IN-04: `appendCorpusRecord` opens without `O_CREATE`; `writeCorpusRecordDirect` opens with it — inconsistent and a latent failure

**File:** `internal/corpus/adjudicator.go:457` vs `internal/check/handler.go:636`

**Issue:** `appendCorpusRecord` uses `os.O_APPEND|os.O_WRONLY` (no `O_CREATE`),
relying on the file already existing because the batch only runs after reading
it. That's true today, but if the corpus file is deleted between the Step-1 read
and the superseding append, the append fails with a confusing ENOENT instead of
recreating. The sibling writer in handler.go includes `O_CREATE`. Inconsistent.

**Fix:** Add `os.O_CREATE` to `appendCorpusRecord`'s open flags for consistency
and resilience (the file should not be re-truncated; `O_CREATE` without
`O_TRUNC` is safe).

### IN-05: `NewMultiSinkWithCorpus` aliases the base MultiSink's backing array via `append`

**File:** `internal/audit/sink.go:144`

**Issue:** `allSinks := append(ms.sinks, corpusSink)` may mutate `ms.sinks`'s
backing array in place if it has spare capacity, or return a new array if not —
nondeterministic aliasing. Today `ms` is freshly built by `NewMultiSink` and then
discarded, so it is harmless, but it is a fragile pattern (a future caller
re-using the base sink would observe surprising shared/unshared state).

**Fix:** Copy explicitly: `allSinks := append(append([]Sink{}, ms.sinks...), corpusSink)`.

## Structural Findings (fallow)

No `<structural_findings>` block was provided with this review; none included.

---

_Reviewed: 2026-06-14T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
