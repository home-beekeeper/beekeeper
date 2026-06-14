---
phase: 25-launch-readiness
reviewed: 2026-06-15T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - cmd/beekeeper/catalogs_daemon_test.go
  - cmd/beekeeper/threatmodel_names_test.go
  - docs/THREAT-MODEL.md
  - internal/check/handler_test.go
  - internal/corpus/launch_e2e_test.go
  - internal/corpus/store_test.go
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 25: Code Review Report

**Reviewed:** 2026-06-15T00:00:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

Phase 25 ("Launch Readiness") ships tests + docs only over code built in Phases 22-24.
I reviewed the three new files in full (`internal/corpus/launch_e2e_test.go`,
`internal/corpus/store_test.go`, `cmd/beekeeper/threatmodel_names_test.go`), the new
THREAT-MODEL.md §13, and the Phase-25 diff additions to the two appended files
(`catalogs_daemon_test.go` assertions #8-11; `handler_test.go` `TestBenchmarkRunCheckGate`
+ `TestOfflineProtective`).

No security vulnerabilities or correctness bugs in product code (none was changed). The
material concern is **false-confidence gates**: two of the new assertions claim to prove
something they do not actually exercise, and would pass even if the underlying property
were broken. Because these tests exist specifically to be launch-readiness evaluator
gates, a tautological or mis-targeted assertion is worse than no test — it manufactures
confidence. One latency gate is also brittle under CI load and could fail-block a correct
build. No BLOCKERs: the defects are in test fidelity, not in shippable product behavior.

## Warnings

### WR-01: Assertion #9 is a tautology — proves nothing about the stored record

**File:** `cmd/beekeeper/catalogs_daemon_test.go:355-378`
**Issue:** Assertion #9 claims to prove "Envelope signature: BehaviorSignatureHash is a
64-char hex string (LAUNCH-01)". It does not. It calls
`corpus.BehaviorSigHash(actionType9, "", "")` to compute a *fresh* hash and asserts
`len(computedHash) == 64`. `BehaviorSigHash` is `hex.EncodeToString(sha256.Sum(...))`
(see `internal/corpus/behavior_sig.go:54-62`), which returns exactly 64 hex chars for
*any* input, including empty strings. The assertion can therefore never fail regardless
of the record's actual contents. The diff comment even admits "the existing seed does NOT
set BehaviorSignatureHash in the PushEnvelope.Signature block" — so the test recomputes to
dodge the empty stored field rather than assert on it. The actually-meaningful field,
`rec.PushEnvelope.Signature.BehaviorSignatureHash`, is never read. The launch gate claims
coverage of envelope-signature integrity that does not exist.

Contrast with `launch_e2e_test.go:230-244`, which asserts on the *stored*
`corpusRec.PushEnvelope.Signature.BehaviorSignatureHash` (length + hex-validity) — that is
the correct pattern, because in that test `MapToCorpusRecord` actually populates the field
(`emitter.go:166`). The catalogs_daemon seed bypasses `MapToCorpusRecord`, so the field
stays empty and the test papers over it.

**Fix:** Either (a) seed the `BehaviorSignatureHash` in the fixture and assert on the
stored value, or (b) drop the recompute-and-measure-length step and assert hex-validity of
the stored field, acknowledging it is currently empty (which would correctly turn the gate
red and surface the seed gap):
```go
got := rec.PushEnvelope.Signature.BehaviorSignatureHash
if len(got) != 64 {
    t.Errorf("[9] stored BehaviorSignatureHash = %q (%d chars); want 64-char hex", got, len(got))
}
// plus a hex-validity loop as in launch_e2e_test.go:238-244
```

### WR-02: TestOfflineProtective does not exercise the catalog/offline path it claims

**File:** `internal/check/handler_test.go:1599-1670`
**Issue:** The test's name, doc comment, and three paragraphs of inline rationale assert it
proves "a known-malicious entry in the last-synced mmap catalog is still blocked when no
live network catalog sources are configured" and that "the mmap index IS the disconnected
machine's sole defense boundary." The implementation does none of that. It builds the index
(`buildTestIndex`) but the stimulus is malformed JSON
(`"{bad json — offline fail-closed proof}"`), which short-circuits at the JSON-decode
fail-closed guard *before any catalog lookup occurs*. The built index is never consulted —
removing the `buildTestIndex(t, dir)` call (or passing a garbage index path) would not
change the result. The test proves only that malformed input fails closed, which is already
covered by existing decode-path tests. The LAUNCH-03 "offline machine blocks on last-synced
catalog" property remains unproven; a regression that broke catalog-backed offline blocking
would not turn this gate red.

**Fix:** Drive a tool input that matches a known-malicious entry in the test index with a
corroboration count that meets the block threshold, and assert `Allow == false` with no
network sources configured. If the existing `buildTestIndex` only seeds a single-source
(warn) entry, add a multi-source malicious fixture so the block path is genuinely
exercised. Keep a separate, honestly-named decode-path test for the malformed-JSON case.

### WR-03: p99 latency gate is brittle and can fail-block a correct build under CI load

**File:** `internal/check/handler_test.go:1572-1597`
**Issue:** `TestBenchmarkRunCheckGate` measures wall-clock p99 over 100 iterations and fails
if `p99 > budgetMS` (100ms Linux/macOS, 200ms Windows). Wall-clock timing of a sub-budget
operation on a shared CI runner is non-deterministic: GC pauses, noisy-neighbor scheduling,
or a cold disk on the audit/corpus append can spike a single sample over budget. With
nearest-rank p99 over N=100 the threshold is the 99th sorted sample (index 98) — a *single*
slow iteration out of 100 lands at or near the p99 slot and trips the gate. Because the
suite default (no `-short`) runs this, a correct build can be blocked by transient runner
latency, not by a real regression. This inverts the project's fail-closed intent into
fail-noisy. Secondarily, `time.Since(start).Milliseconds()` truncates to integer
milliseconds; the comment states ~25ms on dev hardware but the timed ReadFile path is
typically sub-millisecond, so most samples round to 0-1ms and the gate has almost no
resolution to detect a 2-3x regression that still lands under 100ms.

**Fix:** Make the gate robust: (a) measure with `time.Duration`/`Microseconds()` instead
of truncating to ms; (b) use a higher iteration count and report median plus p99, or compare
a *relative* delta (corpus-enabled vs corpus-disabled on the same run) rather than an
absolute wall-clock budget; (c) consider gating only when an explicit env flag (e.g.
`BEEKEEPER_PERF_GATE=1`) is set so transient CI latency cannot fail-block routine builds,
while still running the functional corpus-write coverage unconditionally.

### WR-04: Assertion #8 weakens the behavior-layer check to an OR that the seed guarantees

**File:** `cmd/beekeeper/catalogs_daemon_test.go:331-336`
**Issue:** The behavior-layer check is
`if rec.AuditRecord.SourceSurface == "" && rec.AuditRecord.ToolName == ""`. The fixture
hard-codes `ToolName: "@nrwl/nx-console"` (line 117) and never sets `SourceSurface`, so the
OR condition is structurally satisfied by the fixture's own constant and can only fail if
someone deletes the `ToolName` literal in the same file. The inline comment acknowledges
"The seed sets ToolName=...; SourceSurface is not set in the seed" — i.e. the author knows
only one branch is ever exercised. As written this is a near-tautology that does not
validate that the production write path *populates* a behavior-layer field; it validates
that a test constant is still present. (Lower severity than WR-01 because at least it reads
the stored field.)

**Fix:** Assert the specific field the LAUNCH-01 invariant requires
(`rec.AuditRecord.ToolName == "@nrwl/nx-console"`), or seed `SourceSurface` and assert it
explicitly, so the check reflects the real behavior-layer contract rather than an
either-branch that the fixture can never violate.

## Info

### IN-01: store.go no-network gate verifies only direct imports (documented but worth flagging)

**File:** `internal/corpus/store_test.go:220-247`
**Issue:** `TestCorpusStoreHasNoNetworkImports` parses `store.go` with `parser.ImportsOnly`
and checks only direct imports against `{net, net/http, os/exec}`. A transitively-imported
package could still reach the network, and the THREAT-MODEL.md §13 prose
(`docs/THREAT-MODEL.md:1228-1230`) states the no-exfil property somewhat more strongly than
the test proves. The test comment honestly scopes this ("only store.go's DIRECT imports
need proving; the transitive graph is validated by go vet ./... and the CI cross-build"),
so this is acceptable as-is — but go vet does not validate the absence of network imports,
so the cited transitive guarantee is weaker than implied.
**Fix:** Optional: note in the doc/comment that transitive no-exfil is an architectural
convention, not a machine-checked invariant; or add a build-tag-gated `go list -deps` check
if a hard guarantee is desired later.

### IN-02: Hardcoded sample credential pattern in redaction test

**File:** `internal/corpus/store_test.go:96`
**Issue:** `const secretKey = "AKIAIOSFODNN7EXAMPLE"` is AWS's well-known documentation
example key, used intentionally as redaction-test bait. Not a real secret, but secret
scanners (and the project's own catalog tooling) may flag the literal.
**Fix:** None required; optionally add a `// nolint:gosec` / scanner-ignore comment to
preempt false positives in self-scan runs.

### IN-03: Range-variable capture is redundant on Go 1.25

**File:** `internal/corpus/launch_e2e_test.go:103`
**Issue:** `tc := tc // capture range variable` is a no-op under Go 1.22+ loop semantics
(per-iteration variables), and CLAUDE.md pins Go 1.25+. Harmless but dead boilerplate.
**Fix:** Optional: remove the redundant capture line.

---

_Reviewed: 2026-06-15T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
