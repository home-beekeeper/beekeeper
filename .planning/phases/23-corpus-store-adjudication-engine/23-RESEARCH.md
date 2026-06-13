# Phase 23: Corpus Store & Adjudication Engine — Research

**Researched:** 2026-06-13
**Domain:** Append-only local corpus store (as an audit.Sink) + off-hot-path adjudication engine with corroboration-gated confidence tiers
**Confidence:** HIGH — all findings verified against Phase-22-shipped code; zero new external dependencies; all seams confirmed in live source files

---

## Summary

Phase 22 (Schema & Envelope Lock) is **COMPLETE and FROZEN** at `CorpusSchemaVersion = "1.0"` with maintainer sign-off. Phase 23 builds the impure I/O layer on top of that frozen foundation: the append-only `beekeeper-corpus.ndjson` store (as an `audit.Sink`), the emitter adapter that maps `AuditRecord` → `CorpusRecord`, the off-hot-path adjudication engine that assigns the outcome layer, and the HMAC-SHA256 fingerprint population (STORE-05). The push-envelope builder (`BuildPushEnvelope`) with its purge-rejection gate (ENV-02) and the ENV-03 fuzz/property test also land here.

**The three open questions that previously blocked planning are now LOCKED** (see REQUIREMENTS.md Locked Decisions table): OQ-1 = 30-day configurable `downstream_clean` window; OQ-2 = per-package stable `ScanClusterID` (already shipped in `internal/corpus/behavior_sig.go`); OQ-3 = the automatic adjudication loop runs in `runCatalogsSync` (the no-daemon invocation path) as a bounded batch pass, because the `catalogs daemon` is an OS scheduler (schtasks/systemd-user/launchd) that fires one-shot `beekeeper catalogs sync` invocations — it is NOT a long-lived goroutine process. See the "Adjudicator Lifecycle" section below for the definitive resolution.

**Primary recommendation:** Implement `internal/corpus/store.go`, `emitter.go`, `adjudicator.go`, `fingerprint.go`, and `signer.go` as a single impure package that consumes the frozen Phase-22 types. Wire `corpus.StoreSink` into `audit.NewMultiSink` (Phase-23 adds a `corpusPath` parameter). Wire the bounded adjudication batch pass into `runCatalogsSync` in `cmd/beekeeper/catalogs_daemon.go`. Gate the phase on the evaluator test: four-layer round-trip, redaction proof, source_count dedup, and off-hot-path BenchmarkRunCheck p99 < 100ms.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Corpus record write (all six source_surfaces) | `internal/corpus` via `audit.Sink` | `internal/audit` MultiSink fan-out | StoreSink added to MultiSink graph; no per-surface code change |
| Emitter: AuditRecord → CorpusRecord mapping | `internal/corpus/emitter.go` (impure adapter) | `internal/policy` (read-only pure dep) | Impure adapter pattern preserves `internal/policy` purity |
| Adjudication (automatic sources) | `cmd/beekeeper/catalogs_daemon.go` → `runCatalogsSync` | `internal/corpus/adjudicator.go` (engine) | OQ-3: sync-invocation batch pass, not a long-lived goroutine |
| Adjudication (operator sources) | CLI/TUI write path | `internal/corpus/adjudicator.go` (shared pure logic) | Synchronous writes; no daemon dependency |
| Corpus file permissions (0600) | `internal/corpus/store.go` | `internal/audit` writer pattern | Same `platform.SetOwnerOnly` call as audit log |
| HMAC fingerprints (STORE-05) | `internal/corpus/fingerprint.go` | `internal/platform.StateDir()` (salt storage) | Per-install salt in `state.json` under `corpus.local_salt` |
| Push-envelope builder + purge gate | `internal/corpus/emitter.go` → `BuildPushEnvelope` | `internal/corpus/types.go` (ActionHint constraint) | Type-level guard (Phase 22) + builder-level error (Phase 23) |
| Redaction before every corpus write | `internal/corpus/store.go` | `internal/audit.RedactRecordWithDefaults` | Cross-package-safe wrapper shipped in Phase 22 |
| ENV-03 fuzz / property gate | `internal/corpus/fuzz_test.go` | — | Property: no envelope escapes with non-allowlisted action_hint |
| Benchmark: hot-path p99 < 100ms | `internal/check/benchmark_test.go` | — | BenchmarkRunCheck with corpus enabled; launch gate (LAUNCH-03) |

---

## Frozen Contract from Phase 22

All symbols below are verified against live source files in `internal/corpus/`. Phase 23 CONSUMES these; it MUST NOT change them.

### `internal/corpus/types.go` (commit `996fa46`)

```go
// CorpusRecord — unnamed AuditRecord embed; all AuditRecord fields promoted to JSON top level.
type CorpusRecord struct {
    audit.AuditRecord  // unnamed embed — JSON promotion; no json tag

    // Outcome layer (always present; TrueLabel NOT omitempty)
    TrueLabel          string      `json:"true_label"`              // "unresolved" initially
    AdjudicationSource string      `json:"adjudication_source,omitempty"`
    WasCorrect         *bool       `json:"was_correct,omitempty"`   // nil = unresolved
    ResolvedAt         string      `json:"resolved_at,omitempty"`   // RFC3339

    // Context layer
    BaselineDeviation  string      `json:"baseline_deviation,omitempty"`
    RepoFingerprint    string      `json:"repo_fingerprint,omitempty"`   // HMAC-SHA256; Phase 23 STORE-05
    FleetNodeID        string      `json:"fleet_node_id,omitempty"`      // HMAC-SHA256; Phase 23 STORE-05
    Scope              CorpusScope `json:"scope"`                        // MarshalJSON: "" → "org_only"

    // Schema + envelope
    CorpusSchemaVersion string       `json:"corpus_schema_version"` // const "1.0"
    PushEnvelope        *PushEnvelope `json:"push_envelope,omitempty"` // nil until Phase 23 emitter
}

type PushEnvelope struct {
    Signature      EnvelopeSignature `json:"signature"`
    TrueLabel      string            `json:"true_label"`
    ConfidenceTier string            `json:"confidence_tier"`   // "watch" | "enforce"
    SourceCount    int               `json:"source_count"`
    Scope          CorpusScope       `json:"scope"`
    ActionHint     ActionHint        `json:"action_hint"`       // typed — compile-time guard
    Signing        *SigningBlock      `json:"signing,omitempty"` // nil in v1
}

type EnvelopeSignature struct {
    PackageOrExtensionID  string   `json:"package_or_extension_id"`
    Version               string   `json:"version"`
    BehaviorSignatureHash string   `json:"behavior_signature_hash"`
    IOCs                  IOCBlock `json:"iocs,omitempty"`
}

type IOCBlock struct {
    Domains          []string `json:"domains,omitempty"`
    DNSTunnelPattern string   `json:"dns_tunnel_pattern,omitempty"`
    DeadDropPattern  string   `json:"dead_drop_pattern,omitempty"`
}

type SigningBlock struct {
    Issuer    string `json:"issuer"`
    Signature string `json:"signature"`
    IssuedAt  string `json:"issued_at"`
    Nonce     string `json:"nonce"`
}
```

[VERIFIED: live source `internal/corpus/types.go`]

### `internal/corpus/action_hint.go` (commit `996fa46`)

```go
type ActionHint string
const ActionHintWatchAndBlock ActionHint = "watch_and_block"
// No auto_purge constant exists; grep internal/corpus/ → 0 results [VERIFIED]
```

[VERIFIED: live source + grep check in 22-03-SUMMARY.md]

### `internal/corpus/scope.go` (commit `996fa46`)

```go
type CorpusScope string
const ScopeOrgOnly CorpusScope = "org_only"
const ScopeCommunityShareable CorpusScope = "community_shareable"

func (s CorpusScope) MarshalJSON() ([]byte, error) // "" → "org_only"
func PromoteScope(r *CorpusRecord) error            // always returns non-nil error in v1
```

[VERIFIED: live source `internal/corpus/scope.go`]

### `internal/corpus/behavior_sig.go` (commit `7daca1d`)

```go
// FROZEN normalization rules — changing any rule is a breaking schema change
func BehaviorSigHash(actionType, targetResource, networkDestination string) string
    // SHA-256(normalize(actionType) + NUL + normalize(targetResource) + NUL + normalize(networkDest))
    // Returns 64-char lowercase hex string

func ScanClusterID(packageOrExtID, version, repoFingerprint string) string
    // SHA-256(pkg + NUL + version + NUL + repoFingerprint)[:16]
    // Returns 16-char lowercase hex string (stable key per OQ-2)
```

[VERIFIED: live source `internal/corpus/behavior_sig.go`]

### `internal/corpus/schema_version.go` (commit `7daca1d`)

```go
const CorpusSchemaVersion = "1.0"
// IPv6 normalization quirk accepted at freeze sign-off — tracked in todos/pending/
```

[VERIFIED: live source `internal/corpus/schema_version.go`]

### `internal/audit/redact.go` (commit `e963df9`)

```go
// Cross-package-safe entrypoint for corpus store redaction.
// MUST be called before every corpus NDJSON write (Pitfall 2 / F-1 security finding).
func RedactRecordWithDefaults(rec AuditRecord) AuditRecord
    // Delegates to RedactRecord(rec, DefaultRedactPatterns())
    // Safe for corpus package import: no unexported types in signature
```

[VERIFIED: live source `internal/audit/redact.go`]

### `internal/policy/corroboration.go` (commit `ffdccda`)

```go
type CorroborationOutcome struct {
    SourceCount    int    // distinct SIGNED sources
    ConfidenceTier string // "watch" (< BlockAt) | "enforce" (>= BlockAt)
    // CRITICAL: tier derived from count >= t.BlockAt, NEVER from level == "block"
    // A single-source critical-severity block has level="block" but count=1
    // → must emit confidence_tier:"watch" in the corpus (Pitfall 3 / 2FA invariant)
}

func CorroborateOutcome(matches []CatalogMatch, t CorroborationThresholds) CorroborationOutcome
    // Pure function — no I/O, no goroutines
    // Imports only "fmt" and "sort" (confirmed in TestCorroborationImportsArePure)
```

[VERIFIED: live source `internal/policy/corroboration.go`]

### `internal/config/config.go` (commit `ffdccda`)

```go
type CorpusConfig struct {
    Enabled bool   `json:"enabled"`          // default false; zero value = safe default
    Path    string `json:"path,omitempty"`   // default: StateDir()/corpus/beekeeper-corpus.ndjson
    Scope   string `json:"scope,omitempty"`  // "org_only" (default) | "community_shareable"
}
// Config.Corpus CorpusConfig `json:"corpus,omitempty"`
```

[VERIFIED: live source `internal/config/config.go` lines 467-486]

### `internal/audit/sink.go` (existing — Phase 23 modifies)

```go
// Sink interface — Phase 23 adds corpus.StoreSink as a consumer
type Sink interface {
    Write(rec AuditRecord) error
    Close() error
}

// MultiSink — Phase 23 adds corpus.StoreSink to the sink graph
// Current signature (Phase 23 will extend):
func NewMultiSink(auditPath string, cfg config.AuditConfig) (Sink, error)
// Phase 23 must also accept CorpusConfig and conditionally add StoreSink
```

[VERIFIED: live source `internal/audit/sink.go`]

### `internal/audit/types.go` — additive Phase 22 fields (commit `992a41f`)

New `omitempty` fields on `AuditRecord` that the corpus emitter reads:
- `SourceSurface string json:"source_surface,omitempty"` — branch key
- `ClusterID string json:"cluster_id,omitempty"` — binds correlated events
- `RulesetVersion string json:"ruleset_version,omitempty"` — catalog snapshot version

[VERIFIED: live source `internal/audit/types.go` lines 86-108]

### `internal/sentry/targets.go` (existing — Phase 23 does NOT modify)

```go
func (tl *TargetList) AddTarget(name, path, expectedProcess string) // pure, idempotent
func LoadTargets(path string) (*TargetList, error)                  // I/O: reads JSON file
func SaveTargets(path string, tl *TargetList) error                 // I/O: writes 0600 JSON
// Called by adjudicator on confirmed-malicious; targets.go has NO code changes in Phase 23
```

[VERIFIED: live source `internal/sentry/targets.go`]

---

## Adjudicator Lifecycle (OQ-3 Locked)

**Critical finding from live code inspection:** The `catalogs daemon` is NOT a long-lived goroutine process. It is an OS scheduler registrar (`installCatalogDaemon` in `catalogs_daemon_windows/linux/darwin.go`) that registers a **user-level OS job** (schtasks on Windows, systemd --user timer on Linux, launchd LaunchAgent on macOS) that fires one-shot `beekeeper catalogs sync` invocations on an hourly heartbeat. There is no long-lived daemon Go process to host a persistent background goroutine. [VERIFIED: live source `cmd/beekeeper/catalogs_daemon.go`]

**OQ-3 resolution — definitive implementation:**

### Automatic adjudication sources (`catalog_confirmation`, `downstream_clean`)

These live in `runCatalogsSync` (`cmd/beekeeper/catalogs_daemon.go`), as a **bounded batch pass called at the START of each sync invocation**, before the catalog HTTP fetch. This is the no-daemon fallback — correct for both cases:
- User has `catalogs daemon install` → OS fires `beekeeper catalogs sync` hourly → batch pass runs
- User runs `beekeeper catalogs sync --force` manually → batch pass runs
- User has no daemon → batch pass runs whenever they manually sync

The batch pass must be bounded and fail-closed: if it takes longer than a configurable deadline (default 5s), it is abandoned. A corpus batch-pass error MUST NOT cause `runCatalogsSync` to return an error (the catalog sync proceeds regardless).

```
runCatalogsSync (cmd/beekeeper/catalogs_daemon.go):
  1. Load CorpusConfig from cfg
  2. If cfg.Corpus.Enabled:
     a. corpus.RunAdjudicationBatch(ctx, corpusPath, thresholds, deadline=5s)
        -- reads unresolved records from corpus NDJSON
        -- for each: checks catalog_confirmation (re-query mmap index for same pkg/version)
        -- for each: checks downstream_clean (30-day window, configurable)
        -- writes outcome update records (append-only NDJSON superseding records)
     b. Errors logged to stderr; sync continues
  3. Proceed with existing catalog HTTP sync logic (unchanged)
  4. Run enforceSelfQuarantine (unchanged)
```

The `corpus.RunAdjudicationBatch` function receives a `context.Context` with a 5s deadline — the caller sets `context.WithTimeout(ctx, 5*time.Second)` before passing it. The function exits when context is cancelled, writing whatever it completed.

### Operator-driven adjudication sources (`forensic_review`, `breach_confirmation`, `user_override`, `benign_explained`)

These are **synchronous CLI/TUI writes** — they do not depend on any daemon or adjudication batch pass. They will be wired in Phase 24 (TUI) and can be stubbed in Phase 23 as direct `CorpusRecord` outcome-layer append operations.

### Corpus record write (from `beekeeper check` hot path)

The `corpus.StoreSink.Write(AuditRecord)` call happens **synchronously** inside `audit.MultiSink.Write` — the same call that writes to `beekeeper.ndjson`. The write is synchronous (NDJSON append) but does NOT block on adjudication. The corpus record is written with `TrueLabel: "unresolved"` immediately. Adjudication happens later in `runCatalogsSync`.

**Fail-closed invariant for hot path:** A `corpus.StoreSink.Write` error MUST NOT change the hook exit code. `MultiSink.Write` already accumulates errors and returns the last non-nil error — the calling code in `writeAuditWithAC` logs the error to stderr and continues. Phase 23 must verify this invariant with a test that injects a corpus write error and asserts the hook exits with its policy-derived code.

### Pre-exit bounded flush (for `beekeeper check`)

Because `beekeeper check` is a one-shot process, there is no goroutine lifetime to worry about — the corpus write is synchronous in the `MultiSink.Write` call, which completes before `finalizeWithAC` returns. No `sync.WaitGroup` + deadline is needed for the STORE layer. The bounded deadline pattern is needed for the ADJUDICATION batch pass in `runCatalogsSync`, not for the store write.

This is a **deviation from the pre-Phase-22 research** (SUMMARY.md and ARCHITECTURE.md described a channel-fed async goroutine with WaitGroup). The correct architecture is:
- **Store write** = synchronous within `MultiSink.Write` (fast NDJSON append, <1ms)
- **Adjudication** = batch pass in `runCatalogsSync` (off-hot-path, bounded 5s deadline)

The benchmark requirement (BenchmarkRunCheck p99 < 100ms with corpus enabled) validates that the synchronous NDJSON append to the corpus file does not blow the budget. A fast local NDJSON append should add <1ms; the benchmark catches any regression from antivirus interception or slow disk.

---

## Standard Stack

### Core (zero new go.mod dependencies)

| Package | Source | Purpose | Phase 23 use |
|---------|--------|---------|-------------|
| `crypto/hmac` + `crypto/sha256` | stdlib | HMAC-SHA256 for `repo_fingerprint` / `fleet_node_id` | `internal/corpus/fingerprint.go` |
| `crypto/rand` | stdlib | Per-install salt generation | Once on first run; stored in `state.json` |
| `crypto/ed25519` | stdlib | Ed25519 keygen / sign for `SigningBlock` | `internal/corpus/signer.go` — block defined; transport v1.1+ |
| `encoding/json` | stdlib | NDJSON marshal/unmarshal for corpus store | `internal/corpus/store.go`, `adjudicator.go` |
| `sync` | stdlib | `sync.Mutex` for corpus writer concurrent access | `internal/corpus/store.go` |
| `os` + `path/filepath` | stdlib | O_APPEND|O_CREATE|O_WRONLY file operations | `internal/corpus/store.go` |
| `time` | stdlib | `time.Now()` for `ResolvedAt`, `downstream_clean` window | `adjudicator.go` |
| `internal/audit` | existing | `RedactRecordWithDefaults`, `Sink` interface, `AuditRecord`, `Writer` | All corpus files |
| `internal/policy` | existing | `CorroborateOutcome`, `CorroborationThresholds`, `CatalogMatch` | `adjudicator.go` — pure dep only |
| `internal/platform` | existing | `StateDir()` for corpus file path, salt storage | `store.go`, `fingerprint.go` |
| `internal/config` | existing | `CorpusConfig` | `store.go`, `cmd/beekeeper/catalogs_daemon.go` |

[VERIFIED: zero new go.mod entries required — all primitives are stdlib or existing internal packages]

### Corpus store file path

**Default:** `StateDir()/corpus/beekeeper-corpus.ndjson`

The `corpus/` subdirectory is created with `os.MkdirAll(dir, 0o700)` on first write. The file is opened with `O_APPEND|O_CREATE|O_WRONLY` and `0600` permissions (owner-only), matching the existing `audit.Writer` pattern.

**Windows owner-DACL:** The existing `internal/audit/writer.go` uses `platform.SetOwnerOnly(path)` which is already cross-platform (Windows uses owner-DACL; Linux/macOS use chmod 0600). Phase 23 replicates the same call for the corpus file. [VERIFIED: audit writer pattern]

### HMAC salt storage

`state.json` under `corpus.local_salt` (32 random bytes, hex-encoded). Generated once via `crypto/rand.Read` on first corpus store init. The `internal/catalog.LoadState`/`SaveState` seam already manages `state.json` at `StateDir()/state.json` — Phase 23 adds a `corpus.local_salt` key to the same file using the same load/save pattern.

---

## Architecture Patterns

### System Architecture Diagram

```
HOT PATH (synchronous, sub-100ms, fail-closed)
┌─────────────────────────────────────────────────────────────────────────┐
│  source surface produces AuditRecord (hook/gateway/shim/watch/sentry)  │
│                                                                         │
│   audit.MultiSink.Write(AuditRecord)                                   │
│       ├── audit.WriterSink.Write()  → beekeeper.ndjson (existing)      │
│       └── corpus.StoreSink.Write()  → beekeeper-corpus.ndjson (NEW)    │
│             ├── call audit.RedactRecordWithDefaults(rec)                 │
│             ├── map AuditRecord → CorpusRecord{true_label:"unresolved"} │
│             ├── call BehaviorSigHash() + ScanClusterID() (frozen)       │
│             ├── populate PushEnvelope (scope=org_only, action_hint=...)  │
│             └── O_APPEND json.Encoder.Encode(corpusRec) + mutex         │
│                                                                         │
│   handler.go exits → policy-derived exit code (0/1/2)                  │
│   corpus write error: logged to stderr, NEVER changes exit code         │
└─────────────────────────────────────────────────────────────────────────┘

OFF-HOT-PATH ADJUDICATION BATCH (runCatalogsSync — bounded 5s deadline)
┌─────────────────────────────────────────────────────────────────────────┐
│  corpus.RunAdjudicationBatch(ctx5s, corpusPath, cfg, thresholds)       │
│       ├── read unresolved CorpusRecords from corpus NDJSON tail         │
│       ├── for each unresolved record:                                   │
│       │     ├── catalog_confirmation: re-query mmap index               │
│       │     │     → if confirmed: adjudication_source="catalog_confirm" │
│       │     ├── downstream_clean: check 30d window, no follow-on alerts │
│       │     │     → if clean: adjudication_source="downstream_clean"    │
│       │     ├── call policy.CorroborateOutcome(matches, thresholds)     │
│       │     │     → CorroborationOutcome{SourceCount, ConfidenceTier}   │
│       │     └── write superseding CorpusRecord (same cluster_id)        │
│       │           {true_label, adjudication_source, resolved_at,        │
│       │            was_correct, source_count, confidence_tier}          │
│       └── context cancelled → abandon; errors → stderr (sync continues) │
└─────────────────────────────────────────────────────────────────────────┘
```

### Recommended File Structure (Phase 23 creates)

```
internal/corpus/
  [FROZEN Phase 22]
  types.go            # CorpusRecord, PushEnvelope, ActionHint, CorpusScope
  scope.go            # CorpusScope, PromoteScope
  action_hint.go      # ActionHint, ActionHintWatchAndBlock
  schema_version.go   # CorpusSchemaVersion = "1.0"
  behavior_sig.go     # BehaviorSigHash, ScanClusterID (frozen normalizers)

  [NEW Phase 23]
  store.go            # StoreSink: audit.Sink impl; O_APPEND NDJSON; 0600; RedactRecordWithDefaults
  emitter.go          # MapToCorpusRecord: AuditRecord→CorpusRecord; BuildPushEnvelope
  adjudicator.go      # RunAdjudicationBatch; pure Adjudicate() function; AdjudicationSignals
  fingerprint.go      # RepoFingerprint(); FleetNodeID(); HMAC-SHA256 with per-install salt
  signer.go           # Ed25519 keygen/sign/key-persistence (block format; no transport in v1)
  store_test.go       # StoreSink: redaction proof; 0600 permission; O_APPEND idempotency
  emitter_test.go     # MapToCorpusRecord: source_count dedup; watch/enforce table; cluster_id
  adjudicator_test.go # RunAdjudicationBatch: synthetic unresolved → resolved; 30d window gate
  fingerprint_test.go # Two-key non-reversibility; same repo + same key = same fingerprint
  fuzz_test.go        # FuzzBuildPushEnvelope: action_hint never outside allowed set (ENV-03)

cmd/beekeeper/
  catalogs_daemon.go  # MODIFIED: runCatalogsSync adds bounded adjudication batch pass
```

### Pattern 1: Corpus Store as audit.Sink

**What:** `corpus.StoreSink` implements `audit.Sink`. Added to `audit.MultiSink` at daemon/check startup when `cfg.Corpus.Enabled == true`.

**Current `NewMultiSink` signature** (must be extended):
```go
// CURRENT (Phase 22):
func NewMultiSink(auditPath string, cfg config.AuditConfig) (Sink, error)

// PHASE 23 approach — add corpus path parameter:
func NewMultiSinkWithCorpus(auditPath string, auditCfg config.AuditConfig,
    corpusCfg config.CorpusConfig, stateDir string) (Sink, error)
// OR: add an optional variadic functional option
// OR: keep NewMultiSink unchanged and build corpus.StoreSink separately at the call site
```

The planner should choose the least-invasive approach. The recommended approach is to keep `NewMultiSink` unchanged and build `corpus.StoreSink` separately at the call site (`writeAuditWithAC` already constructs a fresh `audit.Writer` per invocation — the corpus store init should happen at process startup in `cmd/beekeeper/main.go`, not per-invocation). See the "Don't Hand-Roll" section for the per-invocation anti-pattern.

**Fail-closed invariant** (from live `MultiSink.Write`):
```go
func (m *MultiSink) Write(rec AuditRecord) error {
    var lastErr error
    for _, s := range m.sinks {
        if err := s.Write(rec); err != nil {
            lastErr = err  // accumulate; never short-circuit
        }
    }
    return lastErr  // non-nil only if at least one sink errored
}
```

A `corpus.StoreSink.Write` error is returned from `MultiSink.Write`, which propagates to `writeAuditWithAC`, which logs it to stderr and continues. The hook decision is unaffected. [VERIFIED: live source `internal/audit/sink.go`]

### Pattern 2: CorpusRecord writer — O_APPEND NDJSON

**What:** Same `O_APPEND|O_CREATE|O_WRONLY` + `sync.Mutex` pattern as `audit.Writer`. Each corpus record is one JSON line.

**Important:** The current `writeAuditWithAC` in `handler.go` opens a NEW `audit.Writer` per invocation (`audit.NewWriter(auditPath)`) rather than sharing a long-lived writer. Phase 23 MUST NOT replicate this per-invocation pattern for the corpus store — it should use a long-lived `corpus.StoreSink` initialized at process startup and passed through the call chain. Opening and closing a file per-invocation is a performance anti-pattern that also breaks O_APPEND atomicity under concurrent check invocations.

**Resolution:** The `cmd/beekeeper/main.go` check command should initialize `corpus.StoreSink` once and pass it into `RunCheckTo`'s sink graph. This requires a minor refactor of how the audit sink is assembled in the check command's `RunE`.

### Pattern 3: Superseding records (append-only corrections)

ADJ-07 requires that corrections are **superseding records**, not in-place mutations. The adjudication engine appends a new `CorpusRecord` with the same `ClusterID` and the outcome layer populated. Consumers (Phase 25 E2E test, Phase 24 TUI binding) take the **latest record by ClusterID** from the NDJSON file.

```go
// Append a superseding outcome record to the corpus store.
// The original "unresolved" record is preserved (append-only).
// Consumers of the corpus NDJSON take the latest record for each cluster_id.
func (a *Adjudicator) writeOutcomeUpdate(rec CorpusRecord, outcome AdjudicationResult) error {
    updated := rec // shallow copy
    updated.TrueLabel = outcome.TrueLabel
    updated.AdjudicationSource = outcome.AdjudicationSource
    updated.WasCorrect = outcome.WasCorrect
    updated.ResolvedAt = outcome.ResolvedAt
    updated.PushEnvelope.ConfidenceTier = outcome.ConfidenceTier
    updated.PushEnvelope.SourceCount = outcome.SourceCount
    // RecordID is NEW (new UUID) — the original record's RecordID is unchanged
    updated.AuditRecord.RecordID = newRecordID()
    return a.store.Write(updated.AuditRecord) // routes through StoreSink
}
```

### Anti-Patterns to Avoid

- **Per-invocation corpus file open:** `audit.NewWriter` is called per-invocation today; do NOT replicate for the corpus store. Initialize `StoreSink` once at process startup.
- **Synchronous adjudication in handler.go:** Never call `corpus.RunAdjudicationBatch` from `internal/check/handler.go`. It does I/O and reads the corpus file — incompatible with the sub-100ms budget.
- **Corpus error blocking hook:** A `StoreSink.Write` error must be logged to stderr and ignored by the hook handler. Use the existing `MultiSink` error-accumulation pattern.
- **Re-implementing source_count:** Do NOT recount matches from log events. Call `policy.CorroborateOutcome(matches, thresholds)` and use `outcome.SourceCount`. Three Bumblebee hits → `SourceCount: 1`. [VERIFIED: Pitfall 3 / REQUIREMENTS.md ADJ-04]
- **Opening corpus NDJSON file from multiple goroutines without mutex:** The `StoreSink.Write` method must guard the `json.Encoder` with a `sync.Mutex`, matching `audit.Writer.mu`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-package redaction entrypoint | Custom `regexp` in corpus | `audit.RedactRecordWithDefaults(rec)` | Already shipped Phase 22; `redactPattern` type is unexported — custom regexp misses the synchronized compile-once behavior |
| HMAC keyed hash | Bare `sha256.Sum256(path)` | `crypto/hmac.New(sha256.New, salt)` | Bare SHA-256 is reversible by dictionary attack on repo paths |
| Corroboration source count | Count match events | `policy.CorroborateOutcome(matches, t).SourceCount` | Deduplication by source name already in `corroborate()`; re-implementing risks Pitfall 3 (double-count) |
| Ed25519 signing | External sigstore Go library | `crypto/ed25519` stdlib | sigstore is CI-only; importing it as a Go library adds 30+ transitive deps |
| Corpus writer mutex | Per-record file open/close | `sync.Mutex` + long-lived `os.File` | Per-record file open breaks O_APPEND atomicity under concurrent invocations |
| NDJSON format | Custom binary encoding | `json.NewEncoder(f).Encode(rec)` | Same format as audit log; downstream tools (jq, etc.) work out of the box |
| State.json salt storage | Separate salt file | `catalog.LoadState`/`SaveState` with `corpus.local_salt` key | Same file already managed by existing seam; avoids new file proliferation |

**Key insight:** Every "new" problem in Phase 23 has an existing solution already in the codebase — the main risk is not knowing those solutions exist.

---

## Common Pitfalls

### Pitfall 1: Corpus bypasses RedactRecord — secrets leak into NDJSON (HIGH RISK)

**What goes wrong:** The corpus store appends `json.Encode(corpusRec)` without calling `audit.RedactRecordWithDefaults` first. Credential-shaped strings in `target_resource`, `reason`, Sentry fields, or nudge fields reach the NDJSON file raw. This is the exact mistake from the F-1 security finding on 2026-06-12.

**How to avoid:** `StoreSink.Write(rec AuditRecord)` MUST call `rec = audit.RedactRecordWithDefaults(rec)` as its FIRST operation before any further processing. The corpus emitter maps the redacted `AuditRecord` into a `CorpusRecord`.

**Required test:** Write a corpus record where `AuditRecord.Reason` contains `AKIAIOSFODNN7EXAMPLE` (AWS key pattern). Assert the persisted NDJSON does NOT contain the raw key string.

**Warning signs:** `corpus.StoreSink.Write` imports `os` and `encoding/json` but NOT `internal/audit`. [CITED: PITFALLS.md §2]

### Pitfall 2: source_count double-counting → single-source enforce (HIGH RISK / 2FA invariant)

**What goes wrong:** The adjudication engine counts match events rather than distinct source names. Three Bumblebee events = `source_count: 3` = `confidence_tier: "enforce"`. One compromised catalog source can now drive enforce weight.

**How to avoid:** Always use `policy.CorroborateOutcome(matches, t).SourceCount`. Never count from log events.

**Critical nuance:** The existing `CorroborateOutcome` counts SIGNED sources only (`signedSet` in `corroborate()`). For the corpus, a single-source critical-severity block (severity override `BlockAt:1`) produces `SourceCount:1` → `ConfidenceTier:"watch"` even though `level=="block"`. The corpus records the **corroboration tier**, not the enforcement action. [VERIFIED: `CorroborateOutcome` source + 22-01-SUMMARY.md decision notes]

**Required test:** (1) matches=`["bumblebee","bumblebee","bumblebee"]` → `source_count:1, confidence_tier:"watch"`. (2) matches=`["bumblebee","osv"]` → `source_count:2, confidence_tier:"enforce"`.

### Pitfall 3: Adjudication blocks the hot path (HIGH RISK)

**What goes wrong:** `corpus.RunAdjudicationBatch` is called from `handler.go`/`runCheck`, blocking the sub-100ms hook exit on corpus file I/O.

**How to avoid:** `RunAdjudicationBatch` lives ONLY in `runCatalogsSync`. The `handler.go` and `finalizeWithAC` MUST NOT import `corpus` or call any function that reads the corpus file.

**Required test:** BenchmarkRunCheck with `cfg.Corpus.Enabled=true` and a real `StoreSink` wired in. p99 must be < 100ms. A corpus write error injection test must confirm hook exit code is unaffected.

### Pitfall 4: Per-invocation corpus file open

**What goes wrong:** `StoreSink.Write` calls `os.OpenFile(corpusPath, ...)` on every record, matching the current `writeAuditWithAC` pattern. This is 10-50ms overhead per record on Windows with antivirus interception, and breaks `O_APPEND` atomic-append under concurrent invocations.

**How to avoid:** Initialize `corpus.StoreSink` once at process startup (in the check command's `RunE`), pass it into the sink graph. Keep a long-lived `os.File` handle with `sync.Mutex`. Mirror the `audit.Writer` pattern, not `writeAuditWithAC`.

### Pitfall 5: HMAC salt not generated on first run

**What goes wrong:** `RepoFingerprint` and `FleetNodeID` return empty strings because the per-install salt was never generated. `ScanClusterID` uses `repoFingerprint=""` → stable within session but not across reinstall.

**How to avoid:** `corpus.StoreSink` constructor must: (1) load `state.json`, (2) check for `corpus.local_salt`, (3) generate `crypto/rand.Read(32 bytes)` if absent, (4) persist `state.json`. This happens ONCE at startup, not per-record.

**Required test:** Non-reversibility — same repo path + salt-A ≠ same repo path + salt-B.

### Pitfall 6: downstream_clean 30-day window not configurable

**What goes wrong:** `downstream_clean` window is hardcoded to 30 days. OQ-1 explicitly requires it to be configurable.

**How to avoid:** Add `DownstreamCleanDays int` to `CorpusConfig` (default 30). `RunAdjudicationBatch` reads from `cfg.Corpus.DownstreamCleanDays`. The REQUIREMENTS.md wording "30 days, configurable" is the locked decision. [CITED: REQUIREMENTS.md OQ-1]

### Pitfall 7: Corpus file outside StateDir — self-protection bypass

**What goes wrong:** The corpus file path is constructed incorrectly and lives outside `platform.StateDir()`. The self-protection guard in `selfprotect.go` only covers the `StateDir` prefix — a corpus file outside it is writable by the agent.

**How to avoid:** Default path MUST be `platform.StateDir() + "/corpus/beekeeper-corpus.ndjson"`. If `cfg.Corpus.Path` is set, validate it is under `StateDir`. Log a warning and refuse to open it outside `StateDir`. [CITED: PITFALLS.md "Corpus file path outside StateDir"]

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing + testing/fuzz (stdlib) |
| Config | `go test ./internal/corpus/...` |
| Quick run | `go test ./internal/corpus/... -short` |
| Full suite | `go test ./...` |
| Fuzz | `go test -fuzz=FuzzBuildPushEnvelope -fuzztime=30s ./internal/corpus/...` |
| Benchmark | `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File |
|--------|----------|-----------|-------------------|------|
| ADJ-01 | Adjudication off hot path; corpus error does not change hook exit code | unit + benchmark | `go test ./internal/check/... -run TestCorpusWriteErrorDoesNotChangeExitCode` + `go test -bench=BenchmarkRunCheck` | `check/handler_test.go` (existing + new case) |
| ADJ-01 | BenchmarkRunCheck p99 < 100ms with corpus enabled | benchmark | `go test -bench=BenchmarkRunCheck -benchtime=10s ./internal/check/...` | `check/handler_test.go` |
| ADJ-02 | Initial `TrueLabel` = "unresolved"; valid transition to malicious/benign/policy_correct | unit | `go test ./internal/corpus/... -run TestAdjudicationTrueLabelTransition` | `corpus/adjudicator_test.go` |
| ADJ-03 | 6 adjudication_source values with correct confidence mapping | unit table | `go test ./internal/corpus/... -run TestAdjudicationSources` | `corpus/adjudicator_test.go` |
| ADJ-04 | source_count = DISTINCT sources (3x Bumblebee → source_count:1) | unit | `go test ./internal/corpus/... -run TestSourceCountDedup` | `corpus/emitter_test.go` |
| ADJ-05 | confidence_tier: 1 source → "watch"; ≥2 sources → "enforce" | unit table | `go test ./internal/corpus/... -run TestConfidenceTierTable` | `corpus/emitter_test.go` |
| ADJ-06 | was_correct derived from true_label vs verdict; resolved_at set when leaving "unresolved" | unit | `go test ./internal/corpus/... -run TestWasCorrectAndResolvedAt` | `corpus/adjudicator_test.go` |
| ADJ-07 | Corrections are superseding records (new RecordID, same ClusterID); downstream_clean requires 30d window | unit | `go test ./internal/corpus/... -run TestSupersedingRecords` + `TestDownstreamCleanWindow` | `corpus/adjudicator_test.go` |
| STORE-01 | Corpus NDJSON is append-only (new records append; no truncation) | unit | `go test ./internal/corpus/... -run TestStoreAppendOnly` | `corpus/store_test.go` |
| STORE-02 | RedactRecord called before every write; credential-shaped string is redacted | unit | `go test ./internal/corpus/... -run TestStoreRedactsSecretsBeforeWrite` | `corpus/store_test.go` |
| STORE-03 | Corpus file has 0600 permissions after creation | unit | `go test ./internal/corpus/... -run TestStoreFilePermissions` | `corpus/store_test.go` |
| STORE-04 | Records persist in push-envelope shape from first write | unit | `go test ./internal/corpus/... -run TestStoreEmitsPushEnvelopeShape` | `corpus/store_test.go` |
| STORE-05 | repo_fingerprint / fleet_node_id: same path+key → same value; different keys → different values | unit | `go test ./internal/corpus/... -run TestFingerprintNonReversibility` | `corpus/fingerprint_test.go` |
| ENV-01 | Local records emit in frozen push-envelope shape; no transport | unit (schema round-trip) | `go test ./internal/corpus/... -run TestPushEnvelopeEmitted` | `corpus/emitter_test.go` |
| ENV-02 | BuildPushEnvelope returns error for purge-class intent; auto_purge never emitted | unit (negative) | `go test ./internal/corpus/... -run TestBuildPushEnvelopeRejectsPurge` | `corpus/emitter_test.go` |
| ENV-03 | Fuzz: no envelope escapes with non-allowlisted action_hint | fuzz | `go test -fuzz=FuzzBuildPushEnvelope ./internal/corpus/...` | `corpus/fuzz_test.go` |

### Evaluator Gate (Phase 23 Definition of Done — PRD §4 Phase 1)

All of the following must pass before Phase 23 is marked complete:

1. **Four-layer round-trip:** A synthetic Nx Console Sentry incident records all four layers (behavior + decision + outcome + context) in the corpus NDJSON. The `TrueLabel` starts as "unresolved" and transitions to "malicious" after `RunAdjudicationBatch` with a catalog-confirmed match.

2. **source_count dedup:** Three Bumblebee match events yield `source_count:1, confidence_tier:"watch"`. Two distinct sources yield `source_count:2, confidence_tier:"enforce"`.

3. **Redaction proof:** An `AuditRecord` with `AKIAIOSFODNN7EXAMPLE` in `Reason` produces a corpus NDJSON line where that string does not appear.

4. **HMAC non-reversibility:** Same repo path + salt-A ≠ same repo path + salt-B (two distinct per-install keys).

5. **Off-hot-path proof:** `BenchmarkRunCheck` with `cfg.Corpus.Enabled=true` p99 < 100ms; injected corpus write error does not change hook exit code.

6. **ENV-03 fuzz gate:** `FuzzBuildPushEnvelope` runs for at least 30s with no corpus violation (action_hint outside `{watch_and_block}`).

7. **Full suite green:** `go test ./... -count=1` exits 0; `go build ./...` exits 0; `go mod tidy && git diff --exit-code go.mod` shows no change.

### Sampling Rate

- **Per task commit:** `go test ./internal/corpus/... ./internal/check/... -short`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite + benchmark + fuzz seed before `/gsd-verify-work`

---

## Suggested Plan Breakdown

### Wave 1 — Foundation (Plans 23-01, 23-02 in parallel after Wave 0)

**Wave 0 (prerequisite — pre-wave gate test setup):**
- `corpus/store_test.go` skeleton (Wave-0 gap file)
- `corpus/emitter_test.go` skeleton
- `corpus/fuzz_test.go` stub (`FuzzBuildPushEnvelope`)
- `corpus/fingerprint_test.go` stub
- Existing `internal/check/handler_test.go` has `TestCorpusWriteErrorDoesNotChangeExitCode` gap

**Plan 23-01 — Corpus store + fingerprint + permissions (STORE-01/02/03/04/05, partial ENV-01)**

Deliverables:
- `internal/corpus/store.go` — `StoreSink` implementing `audit.Sink`; `O_APPEND|O_CREATE|O_WRONLY` 0600; `RedactRecordWithDefaults` called first; mutex-guarded encoder; long-lived file handle
- `internal/corpus/fingerprint.go` — `RepoFingerprint(repoPath, salt string) string` + `FleetNodeID(hostname, goos, salt string) string` via `crypto/hmac` + `crypto/sha256`; salt generation + persistence in `state.json` under `corpus.local_salt`
- Tests: `store_test.go` (append-only, redaction proof, 0600 permission, push-envelope shape in NDJSON), `fingerprint_test.go` (non-reversibility)

Dependencies: Phase 22 types (already in `internal/corpus/`), `audit.RedactRecordWithDefaults` (already shipped)

**Plan 23-02 — Emitter adapter + BuildPushEnvelope + ENV-03 fuzz (STORE-01 wiring, ENV-01/02/03, ADJ-04/05 partial)**

Deliverables:
- `internal/corpus/emitter.go` — `MapToCorpusRecord(rec AuditRecord, cfg CorpusConfig) CorpusRecord`; calls `BehaviorSigHash()`, surface-specific `ClusterID` derivation, `RepoFingerprint`/`FleetNodeID` population, `PushEnvelope` construction; `BuildPushEnvelope(rec CorpusRecord, outcome AdjudicationResult) (PushEnvelope, error)` — returns error for purge-class intent
- `internal/corpus/signer.go` — Ed25519 keygen/sign stub; `SigningBlock` population (v1 local-only; no transport); key persisted to `StateDir()/corpus-signing.key` at 0600
- `internal/corpus/fuzz_test.go` — `FuzzBuildPushEnvelope` property gate (ENV-03)
- Tests: `emitter_test.go` (source_count dedup, watch/enforce table, BuildPushEnvelope purge rejection, push-envelope round-trip)

Dependencies: Plan 23-01 (store + fingerprint)

### Wave 2 — Adjudication engine + lifecycle wiring (Plan 23-03)

**Plan 23-03 — Adjudicator + runCatalogsSync wiring + MultiSink integration (ADJ-01..07, ENV-01 complete)**

Deliverables:
- `internal/corpus/adjudicator.go` — `RunAdjudicationBatch(ctx, corpusPath, mmap index, thresholds, cleanWindowDays)` bounded batch pass; reads unresolved records from corpus NDJSON tail; for each: `catalog_confirmation` (re-query mmap index), `downstream_clean` (30d configurable window); calls `policy.CorroborateOutcome`; writes superseding `CorpusRecord` via `StoreSink.Write`; 6 `adjudication_source` values; pure `Adjudicate(rec CorpusRecord, signals AdjudicationSignals) AdjudicationResult` inner function
- `cmd/beekeeper/catalogs_daemon.go` — MODIFIED: `runCatalogsSync` adds bounded adjudication batch pass (5s deadline context) when `cfg.Corpus.Enabled`
- `internal/audit/sink.go` — MODIFIED: extend `NewMultiSink` (or add `NewMultiSinkWithCorpus`) to conditionally include `corpus.StoreSink` when `cfg.Corpus.Enabled`
- `internal/check/handler.go` (or `cmd/beekeeper/main.go`) — wire `StoreSink` into the sink graph for `beekeeper check` (long-lived handle, not per-invocation)
- Tests: `adjudicator_test.go` (four-layer round-trip, superseding records, 30d window), `check/handler_test.go` additions (corpus error injection, BenchmarkRunCheck benchmark)

Dependencies: Plans 23-01 and 23-02

### Dependency Notes

- Plans 23-01 and 23-02 can be PARTIALLY parallel (23-01 foundation ↛ 23-02 wait, but 23-02 imports `store.go`) — author 23-01 first, then 23-02 with the real `StoreSink`
- Plan 23-03 requires both 23-01 and 23-02 complete (adjudicator uses emitter + store)
- The MultiSink wiring and handler.go plumbing in 23-03 may require a small refactor to how the check command assembles its sink graph — the planner should read `cmd/beekeeper/main.go` RunE for the check command to understand the current wiring before designing the sink initialization
- `fuzz_test.go` ENV-03 gate lives in 23-02 but its release-gate status makes it a BLOCKER for phase completion

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | — |
| V3 Session Management | No | — |
| V4 Access Control | Yes | 0600 file permissions + `platform.SetOwnerOnly`; corpus path restricted to `StateDir` prefix; self-protection guard covers StateDir |
| V5 Input Validation | Yes | `audit.RedactRecordWithDefaults` before every write; `ActionHint` typed constant (compile-time); `BuildPushEnvelope` purge-rejection; fuzz gate ENV-03 |
| V6 Cryptography | Yes | HMAC-SHA256 stdlib (`crypto/hmac`); Ed25519 stdlib (`crypto/ed25519`); `crypto/rand` for salt + nonce; never roll custom crypto |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Credential-shaped string persisted to corpus | Information Disclosure | `audit.RedactRecordWithDefaults` as first operation in `StoreSink.Write` |
| Agent reads corpus file to learn Beekeeper's detection knowledge | Information Disclosure | 0600 owner-only + self-protection `EvaluateSelfPath` blocks agent reads of `StateDir` |
| Agent writes corpus file to inject false adjudications | Tampering | 0600 owner-only + self-protection guard; corpus path under `StateDir`; O_APPEND only (no truncation) |
| Single-source critical escalation → enforce tier in corpus | Tampering (bypasses 2FA) | `CorroborateOutcome` count-based tier (verified: count >= BlockAt, not level == "block") |
| `auto_purge` emitted in push envelope | Tampering (blast-radius) | `ActionHint` typed constant + `BuildPushEnvelope` error return for purge-class intent + ENV-03 fuzz gate |
| `repo_fingerprint` reversible by dictionary attack | Information Disclosure | HMAC-SHA256 with per-install salt (never bare SHA-256) |
| `community_shareable` scope leakage from uninitialized records | Information Disclosure | `CorpusScope.MarshalJSON` zero-value guarantee (Phase 22, type-level) |
| Corpus file outside StateDir (self-protection bypass) | Tampering | Validate `cfg.Corpus.Path` is under `StateDir`; refuse otherwise |

---

## Project Constraints (from CLAUDE.md)

These directives apply to Phase 23 implementation:

- **Go 1.25+, single static binary, no CGO in core** — `crypto/ed25519` and `crypto/hmac` are stdlib (no CGO). Zero new go.mod entries. [VERIFIED]
- **`internal/policy` MUST stay a pure function library** — `adjudicator.go` calls `policy.CorroborateOutcome` as a read-only dep; `internal/policy` imports remain only "fmt" and "sort". [VERIFIED: TestCorroborationImportsArePure]
- **Fail closed by default** — A `StoreSink.Write` error MUST NOT change the hook exit code. Corpus batch-pass errors MUST NOT cause `runCatalogsSync` to fail. [REQUIRED]
- **Windows is primary dev machine** — `platform.SetOwnerOnly(path)` for corpus file permissions (cross-platform). Salt stored in `state.json` under `StateDir()` (not `~/.beekeeper` hardcoded). [REQUIRED]
- **Zero new external dependencies** — Only stdlib + existing internal packages. [VERIFIED]
- **Hook handler: fail closed, sub-100ms** — Adjudication batch pass in `runCatalogsSync` only; never in `handler.go`. BenchmarkRunCheck gate confirms p99 < 100ms. [REQUIRED]
- **Bubble Tea v2 import path** — Not directly relevant to Phase 23; Phase 24 concern. [N/A]
- **eBPF pre-compiled** — Not relevant to Phase 23. [N/A]
- **Reproducible builds** — No new binary-embedded data; no new runtime generation. `-trimpath -buildvcs=false` continue to apply. [OK]

---

## Open Questions

1. **MultiSink extension approach** — The cleanest way to add `corpus.StoreSink` to `audit.MultiSink` without breaking existing call sites. Options: (A) add a separate `NewMultiSinkWithCorpus` overload, (B) pass a `[]audit.Sink` extras param, (C) build `corpus.StoreSink` outside `NewMultiSink` and compose at call site. The planner should read `cmd/beekeeper/main.go` check command RunE to understand the current sink construction pattern and choose the least-invasive approach.

2. **Long-lived StoreSink in beekeeper check** — `beekeeper check` is a one-shot process. The "long-lived StoreSink" is effectively process-lifetime, which is equivalent to one per-invocation open. The distinction matters for the CONCURRENT case (multiple parallel `beekeeper check` instances). For a one-shot binary, per-invocation open is acceptable IF the `O_APPEND` flag is set (atomic appends at the OS level for records < 4KB). The planner should decide whether to match the existing `audit.Writer` per-invocation pattern or initialize the `StoreSink` in the Cobra command's `RunE` and pass it through. The benchmark will catch any latency regression.

3. **Downstream_clean window implementation** — The 30-day (configurable) window requires reading the corpus NDJSON to find follow-on alerts with the same `ClusterID` after the original event's `Timestamp`. The planner should specify whether `RunAdjudicationBatch` does a full scan or uses a tail approach. Given corpus file sizes in v1 (unlikely to exceed 10MB), a full scan with a configurable cap (`maxRecordsToScan = 50000`) is acceptable and simpler than maintaining an index.

---

## Environment Availability

All dependencies are stdlib or already-present in go.mod. No external services required for Phase 23.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25+ toolchain | All | ✓ | 1.25 (CLAUDE.md) | — |
| `crypto/hmac` + `crypto/sha256` + `crypto/ed25519` + `crypto/rand` | fingerprint.go, signer.go | ✓ | stdlib | — |
| `encoding/json` + `sync` + `os` + `time` | store.go, adjudicator.go | ✓ | stdlib | — |
| `internal/audit` (existing) | All corpus files | ✓ | Phase 22 shipped | — |
| `internal/policy` (existing) | adjudicator.go | ✓ | Phase 22 (CorroborateOutcome exported) | — |
| `internal/platform` (existing) | store.go, fingerprint.go | ✓ | Existing | — |
| `internal/catalog` mmap index | adjudicator.go (catalog_confirmation) | ✓ | Existing | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `writeAuditWithAC` per-invocation `audit.NewWriter` pattern will be preserved; `StoreSink` init happens in Cobra RunE for the check command | Adjudicator Lifecycle, Pattern 1 | If the main.go check command already has a long-lived sink, the plan may need to wire into the existing sink rather than create a parallel one |
| A2 | Corpus NDJSON records are < 4KB each, making O_APPEND atomically safe for parallel check invocations on the same machine | Pattern 2 | If records exceed 4KB (e.g. long agent_lineage arrays), O_APPEND is no longer atomic → explicit mutex required at the StoreSink level (already planned) |
| A3 | `catalog.LoadState` / `SaveState` seam accepts arbitrary keys under the `state.json` structure, making `corpus.local_salt` storable without modifying the `State` struct | Foundation | If `State` struct is typed and does not have a corpus sub-struct, Phase 23 must add one (similar to how CorpusConfig was added to Config) |

If A3 applies: read `internal/catalog/state.go` before implementing fingerprint.go to confirm the State struct's extensibility.

---

## State of the Art

| Old Approach (pre-research) | Current Approach | When Changed | Impact |
|-----------------------------|------------------|--------------|--------|
| Long-lived daemon goroutine for adjudication | `runCatalogsSync` batch pass (one-shot OS scheduler) | Phase 22 live code inspection | Simpler architecture; no goroutine lifecycle management; adjudication happens on sync cadence, not continuously |
| `json:",inline"` for AuditRecord embed | Unnamed embed (`audit.AuditRecord` without field name) | Phase 22 (22-02-SUMMARY) | Correct Go idiom; `json:",inline"` is YAML/mapstructure only |
| Per-invocation redact pattern compilation | `sync.Once` pre-compilation via `DefaultRedactPatterns()` | Phase 22 (WR-05) | Zero per-call regexp overhead; already shipped |

**Deprecated/outdated from prior research:**
- SUMMARY.md §Phase 23 describes "async goroutine, channel-fed, 1024-record buffer, WaitGroup+200ms deadline" — this is SUPERSEDED by the sync-invocation batch pass in `runCatalogsSync`. The live `catalogs daemon` is not a long-lived goroutine host. The "200ms deadline" WaitGroup is unnecessary for the store write (synchronous NDJSON append). The 5s bounded deadline applies to the ADJUDICATION batch pass in `runCatalogsSync`.
- ARCHITECTURE.md references `internal/corpus/engine.go` — this file is NOT needed. The engine concept is split into `adjudicator.go` (pure `Adjudicate` function) and the batch pass in `runCatalogsSync`.

---

## Sources

### Primary (HIGH confidence — verified against live source files)

- `internal/corpus/types.go` — `CorpusRecord`, `PushEnvelope`, `ActionHint`, `CorpusScope`, `SigningBlock`, `IOCBlock`, `EnvelopeSignature` — all types Phase 23 consumes [VERIFIED: live source]
- `internal/corpus/behavior_sig.go` — `BehaviorSigHash`, `ScanClusterID`, frozen normalization rules [VERIFIED: live source]
- `internal/corpus/scope.go` — `CorpusScope.MarshalJSON`, `PromoteScope` always-error stub [VERIFIED: live source]
- `internal/corpus/schema_version.go` — `CorpusSchemaVersion = "1.0"` + IPv6 quirk note [VERIFIED: live source]
- `internal/audit/types.go` — `AuditRecord` (embed target + Phase 22 additive fields) [VERIFIED: live source]
- `internal/audit/sink.go` — `Sink` interface, `MultiSink` fan-out, `NewMultiSink` current signature [VERIFIED: live source]
- `internal/audit/redact.go` — `RedactRecordWithDefaults`, `RedactRecord`, `DefaultRedactPatterns` [VERIFIED: live source]
- `internal/policy/corroboration.go` — `CorroborationOutcome`, `CorroborateOutcome` exported wrapper, tier derivation contract [VERIFIED: live source]
- `internal/config/config.go` — `CorpusConfig` struct (Enabled/Path/Scope) [VERIFIED: live source lines 467-486]
- `internal/check/handler.go` — `runCheck`, `finalizeWithAC`, `writeAuditWithAC` — sub-100ms hot path; pre-exit audit write pattern; no adjudication room [VERIFIED: live source]
- `internal/sentry/targets.go` — `AddTarget`, `SaveTargets`, `LoadTargets` — called by Phase 24 adjudicator; no file changes in Phase 23 [VERIFIED: live source]
- `cmd/beekeeper/catalogs_daemon.go` — `runCatalogsSync`, `catalogSyncDue`, `offerCatalogSyncDaemon` — OQ-3 adjudicator lifecycle confirmed as sync-invocation batch pass [VERIFIED: live source]

### Secondary (HIGH confidence — Phase 22 execution summaries)

- `.planning/phases/22-schema-envelope-lock/22-01-SUMMARY.md` — Phase 22 Plan 01 decisions (CorroborateOutcome count-based tier, CorpusConfig value type, AuditRecord additive fields)
- `.planning/phases/22-schema-envelope-lock/22-02-SUMMARY.md` — Phase 22 Plan 02 decisions (unnamed embed over json:inline, CorpusScope.MarshalJSON zero-value guarantee, TaskGroup commit strategy)
- `.planning/phases/22-schema-envelope-lock/22-03-SUMMARY.md` — Phase 22 Plan 03 decisions (BehaviorSigHash normalization rules, ScanClusterID 16-char truncation, SCHEMA-06 gate structure)

### Tertiary (MEDIUM confidence — milestone research)

- `.planning/research/PITFALLS.md` — 8 pitfalls; #2 (RedactRecord bypass), #3 (source_count double-count), #5 (adjudication on hot path), #6 (reversible fingerprints) directly relevant to Phase 23
- `.planning/research/STACK.md` — technology stack decisions (zero new deps, Ed25519 stdlib, HMAC-SHA256)
- `.planning/research/ARCHITECTURE.md` — component breakdown (partially superseded by live code inspection for OQ-3 lifecycle)
- `beekeeper-corpus-milestone-prd.md` §3.2/§3.3/§3.5/§4 — authoritative scope; PRD §4 Phase 1 evaluator gate

---

## Metadata

**Confidence breakdown:**
- Frozen Phase 22 contract: HIGH — all symbols verified against live source files
- OQ-3 adjudicator lifecycle: HIGH — resolved by live code inspection of `catalogs_daemon.go` (OS scheduler, not long-lived goroutine); explicitly supersedes pre-Phase-22 research assumption
- Adjudication engine design: HIGH — `CorroborateOutcome` verified; `corroborate()` deduplication logic confirmed; tier mapping confirmed
- HMAC fingerprinting: HIGH — stdlib `crypto/hmac`+`crypto/sha256`; per-install salt pattern; same contract used by existing codebase salt patterns
- Performance bound: MEDIUM — BenchmarkRunCheck p99 < 100ms is the launch gate; actual NDJSON append latency on Windows (with possible antivirus interception) is not pre-measured; the benchmark will confirm
- Plan breakdown: HIGH — wave structure follows existing milestone conventions; plan 23-03 dependency on 23-01+23-02 is definitive

**Research date:** 2026-06-13
**Valid until:** 2026-07-13 (stable Go stdlib + frozen Phase 22 types; unlikely to change)

---

*Phase 23 research: Beekeeper v1.4.0 Adjudicated Corpus (Local Loop)*
*Researched: 2026-06-13*
*Supersedes open questions from .planning/research/SUMMARY.md (OQ-3 resolved by live code inspection)*
