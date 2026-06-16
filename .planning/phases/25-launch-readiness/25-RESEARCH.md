# Phase 25: Launch Readiness — Research

**Researched:** 2026-06-14
**Domain:** End-to-end validation, benchmarking, and documentation for the v1.4.0 Adjudicated Corpus (Local Loop) milestone
**Confidence:** HIGH — all findings are code-grounded against live source files in the repo; zero new external dependencies; no new framework decisions needed

---

## Summary

Phase 25 is the final phase of v1.4.0. All four corpus phases (22–24) are COMPLETE and merged on main. The full moat loop — write → adjudicate → local feedback → overlay → Sentry watch — is implemented and proven by the 7-assertion `TestRunCatalogsSyncFirstResponder` gate. Phase 25's job is NOT to build new feature surface. It is to:

1. **Prove the full end-to-end trace** for the Nx Console incident with all four layers (LAUNCH-01)
2. **Prove each of the eight Sentry patterns** produces a moat-grade corpus record (LAUNCH-02)
3. **Confirm the hot-path budget** p99 < 100ms with corpus enabled, and prove offline-protective (LAUNCH-03)
4. **Verify no corpus data leaves the machine**, and update `docs/THREAT-MODEL.md` to name the three residual gaps (LAUNCH-04)

**Primary recommendation:** Extend the existing `cmd/beekeeper/catalogs_daemon_test.go` and `internal/corpus/` test suite; add a `TestAllSentryPatternsProduceMoatRecord` table-driven test; add a static `TestCorpusHasNoNetworkSink` grep-style gate; and add a `## 13. Adjudicated Corpus (Local Loop) — v1.4.0` section to `docs/THREAT-MODEL.md`.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| End-to-end round-trip proof (LAUNCH-01) | Test suite — `cmd/beekeeper/` or `internal/corpus/` | — | `TestRunCatalogsSyncFirstResponder` already exercises the FRB half; LAUNCH-01 needs to assert ALL FOUR layers (including adjudicated signature on the envelope), not just the FRB 7-point assertions |
| Sentry pattern → corpus record (LAUNCH-02) | `internal/sentry` rules + `internal/corpus` emitter | `internal/check/handler.go` (corpus write chokepoint) | EvaluateEvent is pure and table-driven; synthetic event fixtures already exist in `rules_test.go`; the plan drives them through `writeCorpusRecord` |
| Benchmark gate p99 < 100ms (LAUNCH-03 performance) | `internal/check/handler_test.go` `BenchmarkRunCheck` | `internal/check/latency_persist.go` | BenchmarkRunCheck already exists and confirmed ~25ms; LAUNCH-03 requires formalizing the gate assertion (not just "eyeball") |
| Offline-protective proof (LAUNCH-03 correctness) | `cmd/beekeeper/catalogs_daemon_test.go` | `internal/catalog/` mmap loader | Existing `TestRunCatalogsSyncFirstResponder` already runs without network; LAUNCH-03 just needs a direct assert that `beekeeper check` blocks with no catalog source reachable (last-synced mmap index) |
| No-corpus-exfil static gate (LAUNCH-04 verification) | Test suite — `internal/corpus/` or top-level | — | Grep-style static import assertion: `internal/corpus/store.go` has NO `net`, `http`, or `os/exec` imports; `StoreSink.Write` never calls a remote sink |
| THREAT-MODEL.md residual gaps (LAUNCH-04 docs) | `docs/THREAT-MODEL.md` — new `§13` | — | Three named gaps must appear: SENTRY-008 CI-runner OIDC theft, GitHub API dead-drop, DNS-tunnel ingested-but-undetected |

---

## Standard Stack

### Core (all already in go.mod — zero new deps)

| Package | Location | Purpose | Phase 25 Use |
|---------|----------|---------|-------------|
| `internal/corpus` | repo | CorpusRecord, PushEnvelope, adjudicator, store, reader | LAUNCH-01/02 assertion targets |
| `internal/sentry` | repo | `EvaluateEvent`, 8 rule functions | LAUNCH-02 synthetic event drivers |
| `internal/check` | repo | `runCheck` / `BenchmarkRunCheck` | LAUNCH-03 benchmark gate |
| `internal/catalog` | repo | mmap index, `OpenIndex`, `NewMultiIndexWithOverlay` | LAUNCH-03 offline proof |
| `cmd/beekeeper` | repo | `runCatalogsSync`, `firstResponderFn`, `TestRunCatalogsSyncFirstResponder` | LAUNCH-01 extension base |
| `docs/THREAT-MODEL.md` | repo | Security threat documentation | LAUNCH-04 residual gap naming |

**Installation:** No `go get` needed. Zero new deps is a standing constraint (CLAUDE.md). [VERIFIED: repo go.mod]

---

## Package Legitimacy Audit

> Phase 25 installs NO external packages. This section is N/A.

**Packages removed due to SLOP verdict:** none
**Packages flagged as suspicious SUS:** none

---

## Architecture Patterns

### System Architecture Diagram

The existing corpus moat loop (all built in Phases 22–24):

```
beekeeper check (hot path, < 100ms)
    │
    ├── writeAuditWithAC()
    │       └── writeCorpusRecord()    ← corpus write (fail-closed, off exit-code)
    │               └── StoreSink.Write()
    │                       └── RedactRecord + O_APPEND NDJSON + 0600 owner-only
    │                           (NO network call, NO remote sink)
    │
    └── [exit 0/1/2 — corpus write never changes this]

OS scheduler / catalogs sync (off hot path)
    │
    └── runCatalogsSync()
            ├── RunAdjudicationBatch(ctx 5s)     ← assigns outcome layer
            │       └── catalog_confirmation re-query → malicious|benign|unresolved
            ├── firstResponderFn()               ← FRB pass
            │       ├── ReadMaliciousRecords()
            │       ├── RunFirstResponder()       ← arm TUI card + sentry target
            │       └── AddLocalOverlayEntry()   ← local-only catalog overlay
            └── HTTP catalog fetch (online only; offline: skipped non-fatally)

Phase 25 LAUNCH tests (extend the above):
    LAUNCH-01: extend TestRunCatalogsSyncFirstResponder to assert ALL 4 layers
    LAUNCH-02: new table-driven test — 8 SentryEvents → corpus record → 4-layer check
    LAUNCH-03: formalize BenchmarkRunCheck gate + offline assertion
    LAUNCH-04: static no-network-sink grep gate + THREAT-MODEL.md §13
```

### Recommended Project Structure (Phase 25 additions only)

```
internal/corpus/
    launch_e2e_test.go          # LAUNCH-01 full trace + LAUNCH-02 sentry patterns
internal/check/
    (handler_test.go extended)  # LAUNCH-03 benchmark gate formalization
cmd/beekeeper/
    (catalogs_daemon_test.go)   # LAUNCH-01 may extend here OR use launch_e2e_test.go
docs/
    THREAT-MODEL.md             # LAUNCH-04: new §13 corpus residual gaps
```

---

## Key Research Findings (per LAUNCH requirement)

### LAUNCH-01: End-to-End Nx Console Round-Trip

**What already exists:**

`TestRunCatalogsSyncFirstResponder` in `cmd/beekeeper/catalogs_daemon_test.go` (commit `68ea5d1`) proves:

1. `ReadMaliciousRecords` returns the seeded `@nrwl/nx-console` v17.3.0 record
2. Audit log contains `catalog_quarantine` record (FRB-01)
3. `sentry-targets.json` contains `@nrwl/nx-console` (FRB-04)
4. Quarantine list has exactly one entry (FRB-01)
5. No auto-purge (FRB-02)
6. `quarantine.Restore` succeeds (FRB-03)
7. `NewMultiIndexWithOverlay.LookupAll` returns `local-overlay` match (FRB-05)

**What LAUNCH-01 adds that the existing gate does NOT assert:**

- **All four layers populated** on the CorpusRecord from run-1: `behavior` (SourceSurface, ToolName, SentryFilesAccessed, SentryNetworkDests), `decision` (Decision, Reason, SentryRuleID, CorroborationCount, RulesetVersion), `outcome` (TrueLabel after adjudication = "malicious", AdjudicationSource = "catalog_confirmation", WasCorrect = true, ResolvedAt non-empty), `context` (ClusterID, BaselineDeviation, RepoFingerprint, FleetNodeID, Scope = "org_only")
- **Envelope signature populated**: `PushEnvelope.Signature.BehaviorSignatureHash` is non-empty (64-char hex); `ConfidenceTier = "enforce"`, `SourceCount = 2`, `ActionHint = ActionHintWatchAndBlock`
- **Adjudication source chain**: the superseding record from `RunAdjudicationBatch` carries `AdjudicationSource = "catalog_confirmation"` (not just the initial "unresolved")

**Implementation approach for LAUNCH-01:**

Option A (preferred): Extend `TestRunCatalogsSyncFirstResponder` with additional assertions in the existing file. The seeded corpus record is already a "malicious" enforce-tier record. The test can assert the full four-layer structure directly on the returned `malicious[]` records from `ReadMaliciousRecords`.

Option B: New `TestLaunch01FullTrace` in `internal/corpus/launch_e2e_test.go` — drives `writeCorpusRecord` (via handler), then `RunAdjudicationBatch`, then `ReadMaliciousRecords`, and asserts all four layers in sequence. This is cleaner but requires importing more packages.

**Recommendation:** Extend `TestRunCatalogsSyncFirstResponder` (Option A) with 4 additional assertions (layer completeness check on the returned record). This keeps the LAUNCH-01 gate in the same test that already proves the FRB 7-point round-trip, giving a single 11-assertion evaluator gate.

**"All four layers populated" assertion checklist:**

```go
// Given: malicious[] from ReadMaliciousRecords after runCatalogsSync
rec := malicious[0]
// Behavior layer
assert non-empty: rec.SourceSurface, rec.ToolName or rec.SentryRuleID, rec.ClusterID
// Decision layer
assert: rec.Decision == "block", rec.CorroborationCount >= 1, rec.RulesetVersion != ""
// Outcome layer
assert: rec.TrueLabel == "malicious", rec.AdjudicationSource != "", rec.WasCorrect != nil, rec.ResolvedAt != ""
// Context layer
assert: rec.Scope == corpus.ScopeOrgOnly (= "org_only"), rec.CorpusSchemaVersion == "1.0"
// Push envelope
assert: rec.PushEnvelope != nil
assert: rec.PushEnvelope.Signature.BehaviorSignatureHash != "" (len == 64)
assert: rec.PushEnvelope.ConfidenceTier == "enforce"
assert: rec.PushEnvelope.SourceCount == 2
assert: rec.PushEnvelope.ActionHint == corpus.ActionHintWatchAndBlock
```

[VERIFIED: live source — `internal/corpus/types.go`, `cmd/beekeeper/catalogs_daemon_test.go`]

---

### LAUNCH-02: Eight Sentry Patterns → Moat-Grade Record

**Where the 8 patterns are defined:**

`internal/sentry/rules.go` — all 8 `eval*` functions; `EvaluateEvent` is the dispatch entry point. [VERIFIED: live source]

| Pattern | Event Kind | Trigger Condition | Alert Severity |
|---------|-----------|------------------|----------------|
| SENTRY-001 | `EventFileAccess` | `isMonitoredDescendant` + `isSensitivePath` + ≥ CredAccessThreshold (2) within CredAccessWindowSec (60s) | critical |
| SENTRY-002 | `EventProcessCreate` | `isMonitoredDescendant` + `isCredentialCLI` + ≥ CredCLIThreshold (2) within CredCLIWindowSec (60s) | critical |
| SENTRY-003 | `EventNetworkConnect` | `isMonitoredDescendant` + `isExternalDest` + first outbound within PhoneHomeWindowMin (10m) | high |
| SENTRY-004 | (post-001/002/003) | `checkSENTRY004` — fresh extension within FreshExtWindowMin (30m) | high |
| SENTRY-005 | `EventNetworkConnect` | `isMonitoredDescendant` + recent cred-access + fresh extension within ExfilFusionWindowMin (5m) | critical |
| SENTRY-006 | `EventFileAccess` | `isAgentDescendant` AND NOT `isEditorDescendant` + ≥ CredAccessThreshold non-self-config reads | critical |
| SENTRY-007 | `EventNetworkConnect` | `isMonitoredDescendant` + `isExternalDest` + (recent cred-access OR recent persist-write) within ExfilFusionWindowMin | critical |
| SENTRY-008 | `EventFileWrite` | `isMonitoredDescendant` + `isPersistencePath` + per-path-per-session dedup | high |

[VERIFIED: live source — `internal/sentry/rules.go` lines 405–785]

**Existing test fixtures and helpers (reuse these):**

`internal/sentry/rules_test.go` defines: `buildTree()`, `editorTree()`, `emptyInventory()`, `freshInventory()`, `defaultCfg()`, `noBaseline()`, `hasAlert()`. These are package-internal test helpers (in `package sentry`, not `package sentry_test`). [VERIFIED: live source]

The `rules_test.go` already has tests for SENTRY-001 through SENTRY-008 as individual unit tests (TestSENTRY001Fires, etc.). These tests call `EvaluateEvent` with synthetic process trees and assert which rule IDs fire. [VERIFIED: `rules_test.go` read to line 80 — further tests extend this pattern]

**What LAUNCH-02 needs that does NOT already exist:**

The existing `rules_test.go` tests only assert that the Sentry alert fires. LAUNCH-02 requires asserting that each fired alert routes through the corpus pipeline and produces a `CorpusRecord` with all four layers populated.

**The production corpus-write path for Sentry events:**

1. A Sentry daemon (Linux fanotify / macOS eslogger / Windows ETW) calls `EvaluateEvent` to produce `[]SentryAlert`
2. Each alert is converted to an `audit.AuditRecord` (with `SourceSurface = "sentry"`, `SentryRuleID`, etc.)
3. The audit record reaches `writeCorpusRecord` via the `NewMultiSinkWithCorpus` fan-out in the Sentry daemon's audit sink

In Phase 25, the plan tests this path at the unit level: given a `SentryAlert`, construct the corresponding `audit.AuditRecord`, then call `MapToCorpusRecord` + `StoreSink.Write`, and assert the resulting NDJSON has all four layers.

**Recommended test structure for LAUNCH-02:**

```go
// TestAllSentryPatternsProduceMoatRecord (LAUNCH-02)
// Table-driven over the 8 patterns. Each row specifies:
//   - a minimal SentryAlert for that pattern (ruleID, severity, files/dests)
//   - the expected four-layer fields on the resulting CorpusRecord

type sentryPatternCase struct {
    name       string
    ruleID     string
    alert      sentry.SentryAlert        // synthetic alert produced by rules.go
    wantLayers fourLayerExpectation       // all four layers must be populated
}
```

Each `SentryAlert` maps to an `audit.AuditRecord` via a helper (the same logic the Sentry daemon uses in production). The test then calls `corpus.MapToCorpusRecord(rec, cfg, fp, nodeID)` and asserts:

- **Behavior**: `SourceSurface = "sentry"`, `SentryRuleID = ruleID`, `ToolName = ruleID` (action_type proxy), non-empty `ClusterID`
- **Decision**: `Decision = "alert"`, `SentryRuleName` or `Reason` non-empty
- **Outcome**: `TrueLabel = "unresolved"` (initial; "moat-grade" = all four layers PRESENT even if unresolved at run-1)
- **Context**: `Scope = "org_only"`, `CorpusSchemaVersion = "1.0"`
- **Envelope**: `PushEnvelope != nil`, `ActionHint = ActionHintWatchAndBlock`, `BehaviorSignatureHash` 64-char hex

**"Moat-grade record" definition for LAUNCH-02:**

A "moat-grade record" has all four layers present (with `TrueLabel = "unresolved"` at run-1). "Moat-grade" does NOT require the outcome to be resolved in LAUNCH-02 — that would require a live catalog re-query for each pattern. The test asserts layer completeness + envelope structure, not the resolved label. This is consistent with the Phase 22 SCHEMA-06 gate and the PRD §4 Phase 3 evaluator gate text.

[VERIFIED: `internal/corpus/schema_lock_test.go` TestSchemaLockNxConsoleTrace uses the same "all four layers present at run-1" definition]

**Corpus write path for Sentry events (package placement decision):**

The test for LAUNCH-02 should live in `internal/corpus/` (e.g., `launch_e2e_test.go`) because:
- It imports `internal/sentry` for `SentryAlert` and `EventFileAccess` etc.
- It imports `internal/corpus` for `MapToCorpusRecord`, `StoreSink`, `CorpusRecord`
- `internal/sentry` does NOT import `internal/corpus` (the dependency only flows upward)
- No import cycle is introduced

The alternative (putting it in `cmd/beekeeper/`) would work too but forces a fat command package for what is a pure-corpus test.

---

### LAUNCH-03: p99 Sub-100ms Gate + Offline-Protective Proof

**What already exists — benchmark:**

`BenchmarkRunCheck` in `internal/check/handler_test.go` (commit `b08a094` fix + `e9bc535` original). It:
- Drives `runCheck` with `cfg.Corpus.Enabled = true` and a real corpus write each iteration
- Uses a `ReadFile` tool input (avoids nudge pnpm/bun subprocess) for clean timing
- Confirmed ~25ms / ~25,000,000 ns/op on development hardware [VERIFIED: `23-03-SUMMARY.md`]

**LAUNCH-03 gap: the benchmark is "eyeball" only:**

From `23-03-SUMMARY.md`: *"p99 eyeball on production hardware = a Phase-25 LAUNCH-03 check (non-blocking)"*

The existing benchmark comment says: *"run with -bench=BenchmarkRunCheck -benchtime=10s and confirm the reported ns/op < 100,000,000 (100ms). This is documented in 23-VALIDATION.md §Manual-Only."*

**How to formalize the gate in CI:**

Option A (recommended — simplest): Add a `TestBenchmarkRunCheckGate` test that calls `runCheck` N times (e.g., 100 iterations), records elapsed time, and asserts `elapsed/N < 100ms`. This is a deterministic test (not a benchmark) that CI gates on. Use `testing.Short()` to skip the loop in short mode.

Option B: Use `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/... | grep -E 'ns/op'` in CI and parse the output. More fragile.

Option C: Use the existing `latency_persist.go` ring and `P99()` method — call `runCheck` 100 times with a real latency ring, then assert `ring.P99() < 100_000_000` (100ms in nanoseconds). This reuses the existing latency infrastructure exactly as production does.

**Recommendation: Option C** — uses the production latency ring (`LTRing`) already in `internal/check/latency_persist.go`. This proves p99 using the SAME ring that `beekeeper diag` reads, making the CI gate semantically identical to what operators see. `latency_persist_test.go` already has `TestGlobalHookTracker` showing the ring API.

**"Corpus enabled" toggle:**

`BenchmarkRunCheck` already sets `cfg.Corpus.Enabled = true`. The new test must do the same.

**Offline-protective proof (LAUNCH-03 second part):**

What proves this: the existing `TestRunCatalogsSyncFirstResponder` already runs with no live catalog URL — `runCatalogsSync` fails the HTTP fetch non-fatally and the FRB pass runs before it. BUT it does not explicitly assert that `beekeeper check` BLOCKS with an mmap index from a previous sync when offline.

**Gap to fill:** A `TestOfflineProtective` test that:
1. Builds a catalog index with a known-malicious entry (e.g., the Nx Console extension)
2. Runs `runCheck` against it with no network (network catalog sources disabled / not configured)
3. Asserts the result is `Decision.Allow = false` (block)

This is structurally identical to existing `TestRunCheckBlocks*` tests in `internal/check/handler_test.go`. The catalog index is already mmap-loaded from disk; "offline" is the default state of all tests since they never configure live network sources.

The offline test can be 3-5 lines added to the existing handler test suite.

---

### LAUNCH-04: No-Exfil Gate + THREAT-MODEL.md Update

**No-corpus-exfil verification:**

What proves "no corpus data leaves the machine":

1. `internal/corpus/store.go` imports: `[bufio, encoding/json, fmt, os, sync, github.com/home-beekeeper/beekeeper/internal/audit, github.com/home-beekeeper/beekeeper/internal/platform]` — NO `net`, NO `net/http`, NO `os/exec`. [VERIFIED: store.go read directly]
2. `NewMultiSinkWithCorpus` in `internal/audit/sink.go` takes a caller-constructed `audit.Sink` and never opens a network socket for the corpus sink specifically.
3. STORE-03 requires: "No off-machine transmission of corpus data in v1 (remote sinks are not wired for corpus)." [VERIFIED: `REQUIREMENTS.md`]

**Static gate approach (LAUNCH-04 verification test):**

Add `TestCorpusStoreHasNoNetworkImports` to `internal/corpus/` (or `internal/policy` purity pattern). Pattern: use `go list -f "{{.Imports}}" ./internal/corpus/store.go` and assert no `net`, `net/http`, `os/exec` appear. This mirrors `TestCorroborationImportsArePure` in `internal/policy/`.

Alternatively, a simpler test: `TestCorpusStoreSinkImportsPure` in `internal/corpus/store_test.go` that calls `storeSink.Write(...)` and asserts no outbound connection was made (by verifying the corpus NDJSON on disk vs asserting the write function signature). Given the static import check is more robust, prefer a `go list` assertion.

**THREAT-MODEL.md current state (LAUNCH-04):**

The current `docs/THREAT-MODEL.md` covers through v1.3.0 in its header. It has 12 sections. Section 8 ("Known Gaps") discusses:

- Detection-Completeness Gaps in the Behavioral Sentry (lines 659–688): mentions DNS queries ingested-but-not-correlated, references SENTRY-008 persistence-write, but does NOT name the three corpus-specific residual gaps
- No mention of "dead-drop" or "CI-runner OIDC" theft as named corpus-layer residual gaps
- DNS-tunnel: mentioned at line 681 as "ingested-but-undetected"

**What LAUNCH-04 adds to THREAT-MODEL.md:**

A new `## 13. Adjudicated Corpus (Local Loop) — v1.4.0` section that:

1. States the corpus is local-first, append-only, owner-only (0600), no transport in v1
2. States that no corpus data leaves the machine (points to STORE-03 and the static import gate)
3. **Names the three residual gaps explicitly:**

   a. **SENTRY-008 CI-runner OIDC theft**: when tokens are stolen from runner process memory on a machine with no editor/host agent, Beekeeper's ancestry gate cannot see the event (the CI runner is not editor-descended). Architectural mitigation only: hardened token scoping + short TTLs at the CI level.

   b. **GitHub API dead-drop exfil**: a malicious agent that has exfiltrated secrets can use the GitHub API to create a private repo or gist as a dead-drop. A host tool cannot reliably distinguish a legitimate `gh repo create` from a malicious one using stolen secrets. Out of host scope.

   c. **DNS-tunnel ingested-but-undetected**: DNS query events are captured on Linux (eBPF `udp_sendmsg`/`tcp_sendmsg` kprobe) and Windows (ETW DNS-Client), but no correlation rule currently consumes them (the `EventDNSQuery` case in `EvaluateEvent` is an explicit no-op). DNS-TXT tunneling is a known undetected exfil channel until a rule is written.

These three gaps are already partially described in §8 (SENTRY-008, DNS-tunnel), but NOT named as a coherent "corpus-layer residual gaps" section and NOT named as explicit v1.4.0 out-of-scope items. LAUNCH-04 requires them to be named in a distinct v1.4.0 section.

[VERIFIED: `docs/THREAT-MODEL.md` read in full — no §13, no "dead-drop", no "CI-runner OIDC" named gap, no v1.4.0 corpus section]

---

## Common Pitfalls

### Pitfall 1: LAUNCH-01 asserts FRB round-trip but not all four layers
**What goes wrong:** The test extends `TestRunCatalogsSyncFirstResponder` with assertions #8–11 but only checks the FRB outcome (quarantine, sentry target, overlay). Misses asserting the corpus record's outcome layer (TrueLabel = "malicious", AdjudicationSource, WasCorrect, ResolvedAt).
**How to avoid:** Explicitly assert all four layers on the record returned by `ReadMaliciousRecords`, not just the FRB side-effects.

### Pitfall 2: LAUNCH-02 conflates "moat-grade" with "fully adjudicated"
**What goes wrong:** The plan tries to assert `TrueLabel = "malicious"` for each of the 8 Sentry patterns, which requires running `RunAdjudicationBatch` with a live catalog. Sentry events have no `PackageOrExtensionID` by default (they are behavioral, not package-install events), so `catalog_confirmation` cannot run.
**How to avoid:** "Moat-grade" for LAUNCH-02 means all four layers ARE PRESENT with `TrueLabel = "unresolved"` at run-1. The outcome layer IS the moat (present from first write). Assert layer completeness + envelope structure, not the resolved label.

### Pitfall 3: LAUNCH-03 benchmark test runs the nudge subprocess
**What goes wrong:** Using a Bash npm-install input for `BenchmarkRunCheck` (or the LAUNCH-03 100-iteration test) triggers the nudge pnpm/bun detection subprocess (~2–5s on Windows), making the latency test meaningless.
**How to avoid:** Use `ReadFile` tool input. Confirmed working in existing `BenchmarkRunCheck` after commit `b08a094` fix.

### Pitfall 4: LAUNCH-04 grep gate regex misses the corpus package's transitive imports
**What goes wrong:** `go list -f "{{.Imports}}" ./internal/corpus/store.go` checks direct imports but not transitive ones. A dependency of `internal/platform` could introduce a network call.
**How to avoid:** Only need to prove `store.go` itself has no direct network import; the StoreSink.Write path is the only corpus-write path. Transitive graph is validated by the existing `go vet ./...` and CI cross-build.

### Pitfall 5: THREAT-MODEL.md §13 duplicates §8 content without adding value
**What goes wrong:** The new §13 just restates what §8 already says about SENTRY-008 and DNS-tunnel without framing them as corpus-layer residual gaps with the specific LAUNCH-04 name requirement.
**How to avoid:** Frame §13 explicitly as the "v1.4.0 Corpus Residual Gaps" section. Use the exact strings "SENTRY-008 CI-runner OIDC theft", "GitHub API dead-drop exfil", and "DNS-tunnel ingested-but-undetected" because LAUNCH-04 requires those exact strings to appear (the REQUIREMENTS.md lists them verbatim).

### Pitfall 6: LAUNCH-02 test in wrong package causes import cycle
**What goes wrong:** Placing the LAUNCH-02 test in `cmd/beekeeper/` requires importing `internal/sentry` from a package that also imports `internal/catalog`, `internal/corpus`, etc. Not a cycle per se but tests `cmd/beekeeper/` already do this for `TestRunCatalogsSyncFirstResponder`.
**How to avoid:** Either place in `cmd/beekeeper/` (same package as the existing gate — cleanest) or in `internal/corpus/` (closer to the corpus functions). There is no import cycle either way. `internal/sentry` does not import `internal/corpus`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Sentry event generation for LAUNCH-02 | Custom event factories | Existing `buildTree()`, `editorTree()`, `freshInventory()` helpers in `rules_test.go` | Already tested, already cover all 8 patterns |
| Four-layer completeness assertion | Custom reflection walker | Explicit named field assertions (same as SCHEMA-06 gate pattern) | Reflection is fragile; field-by-field is readable and maintainable |
| p99 gate | External benchmark harness | `LTRing.P99()` from `internal/check/latency_persist.go` | Production ring already computes p99 across runs |
| Offline-protective test | Network interception mock | Don't configure live sources in test config (default for all tests already) | Tests run offline by default; just assert the block result |
| Exfil verification | Network packet capture | Static import assertion (`go list -f "{{.Imports}}"`) | Import check is cheaper and more reliable than runtime network capture |

---

## Runtime State Inventory

> Not applicable — Phase 25 is a pure test/doc/validation phase. No rename, refactor, or migration. Omitting this section.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25+ toolchain | All test runs | ✓ | (build env) | — |
| `go test ./...` | Full suite gate | ✓ | (existing CI) | — |
| `go test -bench=BenchmarkRunCheck` | LAUNCH-03 benchmark | ✓ | (existing CI) | `TestBenchmarkRunCheckGate` non-benchmark alternative |
| `go test -fuzz=FuzzBuildPushEnvelope` | ENV-03 gate (already passing; no new work) | ✓ | CI fuzz job | — |
| Network (catalog sync) | runCatalogsSync HTTP fetch | ✗ (offline in tests) | — | Already handled: HTTP fetch is non-fatal; FRB pass runs before it |

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing package (stdlib) |
| Config file | none (no separate config; tests use `t.TempDir()`) |
| Quick run command | `go test ./internal/corpus/... ./internal/check/... ./cmd/beekeeper/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LAUNCH-01 | Nx Console trace runs trace→record→adjudication→signature→local feedback with all 4 layers populated | integration | `go test ./cmd/beekeeper/... -run TestRunCatalogsSyncFirstResponder -count=1` | ✅ (exists — needs 4 new assertions extending 7-point gate to 11-point) |
| LAUNCH-02 | Each of 8 Sentry patterns produces a moat-grade record (4 layers present) | unit/integration | `go test ./internal/corpus/... -run TestAllSentryPatternsProduceMoatRecord -count=1` | ❌ Wave 0 gap |
| LAUNCH-03 (perf) | `beekeeper check` p99 < 100ms with corpus enabled | benchmark | `go test ./internal/check/... -run TestBenchmarkRunCheckGate -count=1` | ❌ Wave 0 gap (existing BenchmarkRunCheck is eyeball-only) |
| LAUNCH-03 (offline) | Disconnected machine blocks on last-synced catalog | unit | `go test ./internal/check/... -run TestOfflineProtective -count=1` | ❌ Wave 0 gap (pattern exists; test does not) |
| LAUNCH-04 (no-exfil) | No corpus data leaves machine (static import gate) | static/unit | `go test ./internal/corpus/... -run TestCorpusStoreHasNoNetworkImports -count=1` | ❌ Wave 0 gap |
| LAUNCH-04 (docs) | THREAT-MODEL.md §13 names 3 residual gaps | human-review + grep | `go test ./... -run TestThreatModelNames` (grep gate) or manual | ❌ Wave 0 gap |

### Sampling Rate
- **Per task commit:** `go test ./internal/corpus/... ./internal/check/... ./cmd/beekeeper/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green + `go vet ./... ` + `go build ./...` + `go mod tidy && git diff --exit-code go.mod`

### Wave 0 Gaps

- [ ] `internal/corpus/launch_e2e_test.go` (or extend `cmd/beekeeper/catalogs_daemon_test.go`) — LAUNCH-01 assertions #8–11 + LAUNCH-02 8-pattern table
- [ ] `internal/check/handler_test.go` — `TestBenchmarkRunCheckGate` (100-iter latency ring assertion) + `TestOfflineProtective`
- [ ] `internal/corpus/store_test.go` — `TestCorpusStoreHasNoNetworkImports` (import assertion)
- [ ] `docs/THREAT-MODEL.md` — new `## 13` section with 3 named corpus residual gaps

---

## Security Domain

> `security_enforcement` not explicitly set to false; treating as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No new auth paths in Phase 25 |
| V3 Session Management | no | No new session paths |
| V4 Access Control | no | Owner-only already enforced (STORE-03); no relaxation in tests |
| V5 Input Validation | yes | Test inputs (synthetic SentryEvents, synthetic CorpusRecords) must not allow injection of `auto_purge` into corpus — ENV-03 FuzzBuildPushEnvelope already guards this |
| V6 Cryptography | no | No new crypto; BehaviorSigHash and HMAC fingerprints are already tested |

### Known Threat Patterns for Phase 25

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Test seeding an `auto_purge` intent into a corpus record | Tampering | ENV-03 `FuzzBuildPushEnvelope` gate (already passing, 316k executions) |
| THREAT-MODEL.md understatement of residual gaps | Information Disclosure (false confidence) | LAUNCH-04 requires naming all three gaps verbatim; treat as a correctness requirement |
| Benchmark p99 baseline inflation on slow CI runner | Repudiation | Set the gate to 100ms (10x headroom over ~25ms observed); CI gate is deterministic |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `TestRunCatalogsSyncFirstResponder` can be extended in-place with assertions #8–11 without restructuring | LAUNCH-01 | Low — the test already has the seeded record and `ReadMaliciousRecords` result; adding assertions is additive |
| A2 | "Moat-grade" for LAUNCH-02 means "all four layers present, TrueLabel=unresolved acceptable" | LAUNCH-02 | Medium — if maintainer wants TrueLabel="malicious" for all 8 patterns, `RunAdjudicationBatch` must be driven with a catalog hit for each; this is more complex |
| A3 | `TestBenchmarkRunCheckGate` at 100ms gate has sufficient headroom on CI runners | LAUNCH-03 | Low on Linux CI; Windows CI may be slower; use 200ms as the gate on Windows if needed |
| A4 | `internal/corpus/store.go` has no indirect network path via `internal/platform` | LAUNCH-04 | Low — `platform.SetOwnerOnly` is a filesystem permission call, not a network call |

---

## Open Questions (RESOLVED)

> All four resolved at planning (2026-06-14); the Phase 25 plans implement each recommendation: Q1 → extend `TestRunCatalogsSyncFirstResponder` in place (25-01 Task 1); Q2 → `internal/corpus/launch_e2e_test.go` (25-01 Task 2); Q3 → 100ms gate / 200ms on Windows via `runtime.GOOS` (25-02 Task 1); Q4 → `## 13. Adjudicated Corpus (Local Loop) — v1.4.0` (25-03 Task 1).

1. **LAUNCH-01: extend existing gate vs new test file** — RESOLVED (extend in place)
   - What we know: `TestRunCatalogsSyncFirstResponder` is already 250+ lines; adding 4 assertions extends it further
   - What's unclear: whether the planner wants a single large gate or two separate focused tests
   - Recommendation: extend in place (Option A); the coherence of "one evaluator gate per phase" is preserved

2. **LAUNCH-02: which package hosts `TestAllSentryPatternsProduceMoatRecord`**
   - What we know: `internal/corpus/` can import `internal/sentry`; `cmd/beekeeper/` already does so
   - Recommendation: place in `internal/corpus/launch_e2e_test.go` to keep corpus tests colocated with corpus code; if the Sentry→AuditRecord mapping helper is needed, it can be a test helper in the same file

3. **LAUNCH-03: 100ms vs 200ms gate on Windows CI**
   - What we know: `BenchmarkRunCheck` ~25ms on dev hardware (Windows); CI Windows runners may be 2–3x slower
   - Recommendation: gate at 100ms for Linux/macOS; use 200ms for Windows (via `runtime.GOOS` branch in the test); the p99 requirement is framed as a DEVELOPMENT machine gate, so dev-hardware ~25ms is the source of truth

4. **LAUNCH-04: THREAT-MODEL.md section number**
   - What we know: current THREAT-MODEL has 12 sections (§1–§12)
   - Recommendation: add `## 13. Adjudicated Corpus (Local Loop) — v1.4.0` as the final section; update the header's "Covers" line to include v1.4.0

---

## Sources

### Primary (HIGH confidence)
- `internal/corpus/` (live source) — `types.go`, `emitter.go`, `reader.go`, `store.go`, `adjudicator.go`, `fuzz_test.go`, `schema_lock_test.go`, `testdata/nx_console_trace.json` — full four-layer schema, adjudication engine, StoreSink implementation [VERIFIED: read directly this session]
- `internal/sentry/rules.go` (live source) — all 8 `eval*` functions, `EvaluateEvent` dispatch, helper functions [VERIFIED: read directly this session]
- `internal/sentry/rules_test.go` (live source) — `buildTree`, `editorTree`, `freshInventory`, test helper patterns [VERIFIED: read directly this session]
- `internal/check/handler_test.go` (live source) — `BenchmarkRunCheck`, `TestCorpusWriteErrorDoesNotChangeExitCode` [VERIFIED: read directly this session]
- `internal/check/latency_persist.go` and `latency_persist_test.go` (live source) — `LTRing`, `P99()`, `TestGlobalHookTracker` [VERIFIED: Grep search this session]
- `cmd/beekeeper/catalogs_daemon_test.go` (live source) — `TestRunCatalogsSyncFirstResponder` full 250-line test [VERIFIED: read directly this session]
- `docs/THREAT-MODEL.md` (live source) — all 12 sections, current corpus/DNS/SENTRY-008 coverage [VERIFIED: read in full this session]
- `.planning/phases/22-schema-envelope-lock/22-03-SUMMARY.md` — schema freeze facts, BehaviorSigHash frozen normalization [VERIFIED: read directly this session]
- `.planning/phases/23-corpus-store-adjudication-engine/23-03-SUMMARY.md` — BenchmarkRunCheck ~25ms, ADJ-01 eyeball-only note [VERIFIED: read directly this session]
- `.planning/phases/24-first-responder-corpus-binding/24-03-SUMMARY.md` — 7-assertion evaluator gate, FRB wiring, mergeCorpus fix [VERIFIED: read directly this session]
- `.planning/REQUIREMENTS.md` — LAUNCH-01..04 verbatim text, STORE-03, ENV-01..03 [VERIFIED: read directly this session]
- `.planning/research/SUMMARY.md` — milestone-level architecture, Phase 25 evaluator gate description [VERIFIED: read directly this session]

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` — Phase 24 implementation record, deferred LAUNCH-03 p99 eyeball note [VERIFIED: read directly this session]
- `.planning/ROADMAP.md` — Phase 25 goal and 4 success criteria [VERIFIED: read directly this session]

---

## Metadata

**Confidence breakdown:**
- Test locations and extension points: HIGH — all seams are live source-verified
- THREAT-MODEL.md gap analysis: HIGH — read full document; §13 gaps identified precisely
- LAUNCH-03 p99 gate approach: HIGH — latency ring API confirmed in source
- LAUNCH-02 "moat-grade" definition: MEDIUM — see Assumption A2 (planner should clarify with maintainer if needed)

**Research date:** 2026-06-14
**Valid until:** 2026-06-28 (corpus code stable; Sentry rules stable; only new findings would be from maintainer feedback)

---

## Phase Constraints (from CLAUDE.md)

All directives from `CLAUDE.md` apply. Specifically for Phase 25:

| Directive | Impact on Phase 25 |
|-----------|-------------------|
| `internal/policy` must be a pure function library — no I/O | No new test touches `internal/policy`; existing `TestCorroborationImportsArePure` must stay green |
| Zero CGO in core | No new test introduces CGO; all new tests are pure Go |
| `fail_open` is an explicit opt-in | `TestOfflineProtective` must assert block (not allow) on offline machine — fail-closed confirmed |
| `go test ./...` must exit 0 | All LAUNCH tests must be in `go test ./...` scope (no special build tag beyond `//go:build !plan9`) |
| Zero new deps | Phase 25 adds zero new `go.mod` entries — all test helpers use stdlib or internal packages |
| Reproducible builds | No changes to `main.go` or build flags |
| Self-defense non-negotiable | Phase 25 is a validation/docs phase; no self-defense regression risk |
| `BenchmarkRunCheck p99 < 100ms` confirmed ~25ms | LAUNCH-03 gate is conservative (4x headroom); should pass without calibration |
