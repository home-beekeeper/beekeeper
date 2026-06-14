# Phase 24: First Responder Corpus Binding — Research

**Researched:** 2026-06-14
**Domain:** Integration/wiring — corpus adjudication → TUI quarantine card, Sentry TargetList watch, local catalog overlay
**Confidence:** HIGH — all four seams verified against live source files in Phase 23-shipped code; no speculative assumptions

---

## Summary

Phase 23 (Corpus Store & Adjudication Engine) is **COMPLETE and VERIFIED** as of 2026-06-14. The `internal/corpus` package has `Adjudicate`, `RunAdjudicationBatch`, `OperatorAdjudication` (Phase 24 stub), and `AppendCorpusRecordLine` all wired and tested. Phase 24 is a **pure integration/wiring phase** that binds the confirmed-malicious signal from the corpus into THREE existing seams without standing up any new IPC, new daemon, or new package.

The highest-risk work in this phase is discovering that two of the three seams (TUI quarantine card and Sentry TargetList) already have a clean caller in `internal/watch/firstresponder.go` (RunFirstResponder) that can be extended without import cycles. The third seam (local catalog overlay) does **not** exist yet and must be built as a new file in `internal/catalog/` — this is the most significant new construction in the phase.

**Primary recommendation:** Wire FRB-01/03 (TUI quarantine card + restore) and FRB-04 (Sentry watch) by extending `RunFirstResponder` in `internal/watch/firstresponder.go` to accept a corpus-adjudication reader path alongside the existing scan-hit path. Wire FRB-05 (local catalog overlay) by adding `internal/catalog/local_overlay.go` with a separate `local-overlay.json` file and `local-overlay.idx` index that `catalog.NewMultiIndex` merges alongside `bumblebee.idx` — the overlay survives sync because `SyncConditional` only writes to `bumblebee.json` + `bumblebee.idx`.

**No new IPC.** The notification path for "a confirmed-malicious adjudication happened" is a **file read** of `corpus/beekeeper-corpus.ndjson` — the same file `RunAdjudicationBatch` already reads. A dedicated new function in `internal/corpus` reads only the latest resolved records (those with `true_label == "malicious"` and `confidence_tier == "enforce"`) and returns them as a typed slice. The caller (`runCatalogsSync` batch pass, or the `catalogs sync` command) drives the three seam updates after the adjudication pass completes.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Signal source: confirmed-malicious adjudications | `internal/corpus/adjudicator.go` (ReadMaliciousRecords) | `corpus/beekeeper-corpus.ndjson` (NDJSON file) | No new IPC — caller reads the file directly; IPC would add complexity with no benefit in a local-only v1 |
| TUI quarantine card arming (FRB-01/03) | `internal/watch/firstresponder.go` (RunFirstResponder extension) | `internal/tui/incidents.go` (CatalogQuarantineIncidentFromRecord) | FirstResponder already owns the quarantine + TUI arm path; extend rather than duplicate |
| Sentry watch elevation (FRB-04) | `internal/watch/firstresponder.go` (RunFirstResponder extension) | `internal/sentry/targets.go` (AddTarget/SaveTargets) | Same seam FirstResponder already uses for scan hits; reuse the F-4-hardened gate |
| Corroboration threshold gate (FRB-04) | `internal/corpus/types.go` (CorpusRecord.PushEnvelope.SourceCount) | `internal/watch/firstresponder.go` (Threshold field) | source_count >= 2 = enforce — already stored on the adjudicated CorpusRecord |
| Local catalog overlay (FRB-05) | `internal/catalog/local_overlay.go` (NEW) | `internal/catalog/index.go` (BuildIndex reuse) | No existing overlay mechanism; must be created using the same BuildIndex pattern as bumblebee.idx |
| Overlay survival across sync | `internal/catalog/sync.go` (SyncConditional writes only bumblebee.*) | `internal/catalog/multi.go` (NewMultiIndex: add OverlayIndex) | SyncConditional never touches local-overlay.json / local-overlay.idx; overlay persists automatically |
| Owner-only overlay file | `internal/platform` (SetOwnerOnly) | `internal/catalog/local_overlay.go` | Same pattern as corpus file and sentry-targets.json |
| Purge gate (FRB-02 — non-auto) | `internal/quarantine/quarantine.go` (Purge: human-confirmed) | `internal/tui/quarantine_panel.go` (`p` keypress confirmation) | Purge is already human-gated at the TUI layer; Phase 24 MUST NOT add any automatic Purge call |

---

## Seam 1: Phase 23 Corpus Adjudication Output (Signal Source)

**Package:** `internal/corpus`
**File:** `internal/corpus/adjudicator.go`
**Key functions (VERIFIED: live source):**

```go
// Adjudicate — pure inner function (no I/O). Returns AdjudicationResult.
func Adjudicate(rec CorpusRecord, signals AdjudicationSignals) AdjudicationResult

// RunAdjudicationBatch — impure batch driver. Reads corpus NDJSON, writes superseding records.
func RunAdjudicationBatch(ctx context.Context, corpusPath, stateFile string,
    idx policy.MultiCatalogLookup, t policy.CorroborationThresholds, cleanWindowDays int) error

// OperatorAdjudication — Phase 24 stub for forensic_review / breach_confirmation /
// user_override / benign_explained. Returns AdjudicationResult.
func OperatorAdjudication(source, trueLabel, verdict string,
    matches []policy.CatalogMatch, t policy.CorroborationThresholds) (AdjudicationResult, error)

// AppendCorpusRecordLine — shared writer for superseding records (both adjudicator
// and hot path use this). Redaction-first, O_APPEND, single Write, size cap.
func AppendCorpusRecordLine(corpusPath string, rec CorpusRecord) error
```

**The signal:** A `CorpusRecord` with `TrueLabel == "malicious"` and `PushEnvelope.ConfidenceTier == "enforce"` (meaning `PushEnvelope.SourceCount >= 2`) is the FRB-04 signal. A record with `TrueLabel == "malicious"` at any confidence tier is the FRB-01 signal (arm the card).

**How First Responder learns about adjudications (NO NEW IPC):**

Phase 24 adds a new exported function to `internal/corpus`:

```go
// ReadMaliciousRecords returns the latest-per-cluster CorpusRecords whose
// TrueLabel is "malicious". These are the confirmed-malicious adjudications
// that First Responder uses to arm the TUI quarantine card and Sentry watch.
// Reads the corpus NDJSON from corpusPath (same file RunAdjudicationBatch writes).
// Returns nil if the file does not exist (no corpus yet — not an error).
func ReadMaliciousRecords(corpusPath string) ([]CorpusRecord, error)
```

This function implements the same "latest record per ClusterID" logic from `RunAdjudicationBatch` but filters to `TrueLabel == "malicious"` only. The CALLER is `runCatalogsSync` (or the post-adjudication step in the batch pass), which calls `ReadMaliciousRecords` AFTER `RunAdjudicationBatch` completes, then passes the slice to the seam extension in `internal/watch/firstresponder.go`.

**Alternate design considered and rejected:** A callback or channel from `RunAdjudicationBatch`. Rejected because: (1) the adjudicator is already designed as a batch file-reader; (2) adding a callback would require passing a function type through the batch; (3) reading the file twice (adjudication pass then first-responder pass) is trivial at v1 corpus sizes (<10MB) and keeps the concerns separate. [VERIFIED: RunAdjudicationBatch design in 23-03-SUMMARY.md]

**CorpusRecord fields used by FRB-01/04/05:**

```go
type CorpusRecord struct {
    AuditRecord         // ToolName, Decision, Timestamp, ClusterID
    TrueLabel           string     // "malicious" | "benign" | "policy_correct" | "unresolved"
    ConfidenceTier      string     // on PushEnvelope: "watch" | "enforce"
    SourceCount         int        // on PushEnvelope: distinct signed sources
    PushEnvelope        *PushEnvelope  // carries PackageOrExtensionID, Version, SourceCount, ConfidenceTier
}
// PushEnvelope.Signature.PackageOrExtensionID — format: "ecosystem:package" or "package"
// PushEnvelope.Signature.Version — installed version string
// PushEnvelope.SourceCount — FRB-04 gate: >= 2 = enforce = elevate Sentry watch
```

[VERIFIED: internal/corpus/types.go, internal/corpus/emitter.go, internal/corpus/adjudicator.go]

---

## Seam 2: TUI Quarantine Card / Crossref / Quarantine (FRB-01/02/03)

**Package:** `internal/watch` (orchestration), `internal/tui` (rendering), `internal/quarantine` (physical move)
**Key files (VERIFIED: live sources):**

### 2a. RunFirstResponder — the natural extension point (FRB-01)

`internal/watch/firstresponder.go` — `RunFirstResponder(ctx, FirstResponderConfig) error`

The existing function:
1. Calls `CrossReference` (scan-hit path) to get `[]ScanHit`
2. For each `ScanHit` with `CorroborationCount >= threshold`: adds to `sentry.TargetList` (F-4 gate) AND optionally calls `quarantine.MoveTyped`
3. Saves `sentry.TargetList` via `sentry.SaveTargets`

**Phase 24 extension pattern:** Add a `CorpusMaliciousRecords []corpus.CorpusRecord` field to `FirstResponderConfig` (or accept them as a separate parameter). The extension loop processes these records alongside existing `ScanHit` values, calling the same `sentry.AddTarget` path (FRB-04) and a NEW audit-write path for TUI arming (FRB-01).

```go
// FirstResponderConfig — Phase 24 adds:
type FirstResponderConfig struct {
    // ... existing fields (unchanged) ...
    // CorpusPath is the corpus NDJSON path. When non-empty and CorpusEnabled is
    // true, RunFirstResponder reads malicious adjudications from this path.
    CorpusPath    string
    CorpusEnabled bool
    // CorpusSentryThreshold is the minimum SourceCount to elevate a Sentry watch.
    // Default 2 (enforce tier). Maps to PushEnvelope.SourceCount >= threshold.
    CorpusSentryThreshold int
}
```

**Why extend FirstResponderConfig and not add a new function:** `RunFirstResponder` already has all the seam wiring (sentry targets path, quarantine dir, audit path). A new function would duplicate all that plumbing. Extending the config struct is the least-invasive approach and keeps the first-responder logic in one place.

### 2b. TUI quarantine card arming mechanism

`internal/tui/incidents.go` — `CatalogQuarantineIncidentFromRecord(rec audit.AuditRecord, pending bool) IncidentModel`

This existing function (Phase-20 built, verified live) builds a quarantine incident card from an `AuditRecord`. It is triggered in `App.Update` by `newRecordsMsg` containing `sentry_alert` records. The card type is determined by `rec.RecordType`.

**FRB-01 arming path:** When a confirmed-malicious adjudication is found by `ReadMaliciousRecords`, `RunFirstResponder` writes an audit record with `RecordType = "corpus_quarantine_alert"` (a new record type for FRB-01). The TUI's existing `watchAuditLog` goroutine picks it up via `newRecordsMsg`. The `App.Update` handler for `newRecordsMsg` is extended to handle `corpus_quarantine_alert` by calling `CatalogQuarantineIncidentFromRecord`.

OR (simpler alternative): Reuse the existing `"pending-quarantine"` record type from FirstResponder (already handled in `recordToRow` in `alerts_panel.go`) with a new `Reason` field indicating the corpus adjudication source. This avoids touching `App.Update` logic.

**Both options are valid.** The planner should prefer the simpler `pending-quarantine` record type reuse unless FRB-03 (locked TUI semantic) requires a distinct visual treatment. The existing `BadgeBlock` (coral) badge is already applied for `"catalog_quarantine"` and `"pending-quarantine"` record types in `alerts_panel.go`.

**Locked TUI color semantics (from `internal/tui/styles.go`):**

```go
colorRed   = lipgloss.Color("#f85149")  // red = attacker action
colorCoral = lipgloss.Color("#f0883e")  // coral = Beekeeper response
```

- `BadgeCrit()` → red → sentry critical / attacker-initiated
- `BadgeBlock()` → coral → Beekeeper blocking response (quarantine card, HELD badge)
- `BadgeHeld()` → coral → item in quarantine

**FRB-03 constraint:** Restore remains available. This is already implemented in `quarantine_panel.go` (`r` keypress → `quarantine.Restore`). Phase 24 must NOT remove or disable the restore path. The constraint is satisfied if the corpus-adjudication quarantine write uses the same `quarantine.MoveTyped` call as the scan-hit path — the entry lands in the same directory structure and is reversible via `quarantine.Restore`.

### 2c. What identifies a "matching install present locally"?

From `internal/watch/crossref.go`, `ScanHit` carries:
- `Package string` — pollen normalized name
- `Ecosystem string` — npm / pypi / cargo / etc.
- `InstalledPath string` — on-disk path from pollen's `project_path`

The corpus record carries:
- `PushEnvelope.Signature.PackageOrExtensionID` — format `"ecosystem:package"` or `"package"`
- `PushEnvelope.Signature.Version` — the version flagged

**Matching logic:** Extract ecosystem + package from `PackageOrExtensionID` and compare against the scan hit's `Ecosystem + Package` combination. If a matching `ScanHit` is present (from the CrossReference call), use its `InstalledPath`; if absent (package not locally installed), arm a `pending-quarantine` record instead. This is the same `PathResolved` branching logic already in `RunFirstResponder`.

**Key constraint:** Phase 24 does NOT need to run a new scan — the `CrossReference` result from the same `RunFirstResponder` call provides the local install inventory. Corpus records that do NOT appear in the scan-hit result are treated as pending (path unknown — package may have already been removed).

[VERIFIED: internal/watch/firstresponder.go, internal/watch/crossref.go, internal/tui/quarantine_panel.go, internal/tui/incidents.go, internal/quarantine/quarantine.go]

---

## Seam 3: Sentry Watch / TargetList (FRB-04)

**Package:** `internal/sentry`
**File:** `internal/sentry/targets.go`
**Key functions (VERIFIED: live source):**

```go
// TargetList — in-memory list. AddTarget is pure (no I/O), idempotent.
type TargetList struct {
    Entries []TargetEntry `json:"targets"`
}
type TargetEntry struct {
    Name            string `json:"name"`
    Path            string `json:"path,omitempty"`
    ExpectedProcess string `json:"expected_process,omitempty"`
}

// AddTarget — pure, idempotent. Duplicate names silently skipped.
func (tl *TargetList) AddTarget(name, path, expectedProcess string)

// LoadTargets — reads sentry-targets.json. Missing file → empty TargetList (not error).
func LoadTargets(path string) (*TargetList, error)

// SaveTargets — writes 0600 JSON. Creates parent directory.
func SaveTargets(path string, tl *TargetList) error
```

**DETECTION-ONLY invariant (VERIFIED in source comment):** The file's package-level comment explicitly states: "DETECTION-ONLY: the TargetList only tightens correlation thresholds for matching process subtrees. It NEVER triggers kill, isolate, or network-cut." This satisfies FRB-04's "no kill/isolate" requirement. [VERIFIED: targets.go line 16]

**F-4 corroboration gate (VERIFIED: firstresponder.go line 121):**

```go
// RunFirstResponder line 121:
if targets != nil && hit.CorroborationCount >= threshold {
    expectedProcess := ecosystemToProcess(hit.Ecosystem)
    targets.AddTarget(hit.Package, hit.InstalledPath, expectedProcess)
}
```

For FRB-04, the corpus-derived equivalent:

```go
// corpus record → TargetList gating:
if record.PushEnvelope != nil && record.PushEnvelope.SourceCount >= cfg.CorpusSentryThreshold {
    // CorpusSentryThreshold default = 2 (enforce tier).
    expectedProcess := ecosystemToProcess(parseEcosystem(record.PushEnvelope.Signature.PackageOrExtensionID))
    targets.AddTarget(parsePkgName(record.PushEnvelope.Signature.PackageOrExtensionID),
        matchedInstalledPath, expectedProcess)
}
```

**The threshold for the Sentry watch (FRB-04) is `source_count >= 2` (enforce tier).** This is exactly `PushEnvelope.SourceCount >= 2`, which is set by `BuildPushEnvelope` and frozen at emission time (ENV-02). The threshold lives in `FirstResponderConfig.CorpusSentryThreshold` (default 2), matching the hardcoded default in the existing `RunFirstResponder` scan-hit path. [VERIFIED: firstresponder.go threshold=2 default, emitter.go SourceCount frozen at emission]

**Where sentry-targets.json lives:** `FirstResponderConfig.SentryTargetsPath` — e.g. `StateDir()/sentry-targets.json`. Already wired in `RunFirstResponder`. Phase 24 reuses the same path with the same `LoadTargets`/`SaveTargets` pattern.

[VERIFIED: internal/sentry/targets.go, internal/watch/firstresponder.go]

---

## Seam 4: Local Catalog Overlay (FRB-05)

**Package:** `internal/catalog`
**Does a local overlay mechanism exist?** NO — confirmed by grep across `internal/catalog/`. There is no `local_overlay.go`, no `overlay.idx`, no `LocalEntry` type, and no overlay parameter in `NewMultiIndex`. **FRB-05 requires new construction.** [VERIFIED: Glob of internal/catalog/*.go, grep for "overlay"]

**How `catalogs sync` rebuilds the mmap index (VERIFIED: sync.go `SyncConditional`):**

`SyncConditional` writes ONLY two files:
1. `<catalogDir>/bumblebee.json` — raw catalog cache (JSON array of source files)
2. `<catalogDir>/bumblebee.idx` — the mmap binary index (`BuildIndex`)

It never touches any other file in `catalogDir`. An overlay at `<catalogDir>/local-overlay.json` + `<catalogDir>/local-overlay.idx` is therefore IMMUNE to clobber by `catalogs sync`. [VERIFIED: catalog/sync.go lines 138-143]

**How the mmap index is built (VERIFIED: catalog/index.go `BuildIndex`):**

```go
func BuildIndex(path string, entries []Entry) error
// Writes binary index: sorted by sha256(ecosystem + "::" + lower(package))[:32]
// Entry struct: ID, Name, Ecosystem, Package, Versions []string, Severity, SourceURL, CatalogSignature, CatalogSource

// Entry — the existing schema type (schema_version "0.1.0")
type Entry struct {
    ID               string   `json:"id"`
    Name             string   `json:"name"`
    Ecosystem        string   `json:"ecosystem"`
    Package          string   `json:"package"`
    Versions         []string `json:"versions"`
    Severity         string   `json:"severity"`
    SourceURL        string   `json:"source_url"`
    CatalogSignature string   `json:"catalog_signature"`   // "" for overlay entries
    CatalogSource    string   `json:"catalog_source"`      // "local-overlay"
}
```

**How `MultiIndex.LookupAll` works (VERIFIED: catalog/multi.go):**

```go
// MultiIndex aggregates Bumblebee + OSV + Socket.
type MultiIndex struct {
    Bumblebee *Index
    OSV       policy.MultiCatalogLookup
    Socket    policy.MultiCatalogLookup
}
// LookupAll queries each non-nil source and concatenates results.
// The policy engine's corroborate() deduplicates by CatalogSource name.
```

**FRB-05 implementation design — two-file local overlay:**

```
<catalogDir>/
  bumblebee.json          # synced by SyncConditional (CLOBBERED by sync) 
  bumblebee.idx           # synced by SyncConditional (CLOBBERED by sync)
  local-overlay.json      # written by Phase 24 (SURVIVES sync — never touched by SyncConditional)
  local-overlay.idx       # written by Phase 24 (SURVIVES sync — never touched by SyncConditional)
```

New file: `internal/catalog/local_overlay.go`

```go
// AddLocalOverlayEntry appends an entry to the local overlay and rebuilds local-overlay.idx.
// Called by the FRB-05 corpus wiring after a confirmed-malicious adjudication.
// The overlay file is owner-only (0600 via platform.SetOwnerOnly).
// BuildIndex is called with all existing overlay entries + the new one.
// idempotent: if an identical entry (same ecosystem+package) already exists, it is not duplicated.
func AddLocalOverlayEntry(catalogDir string, e Entry) error

// LoadLocalOverlay reads local-overlay.json and returns its entries.
// Missing file is not an error (first run).
func LoadLocalOverlay(catalogDir string) ([]Entry, error)
```

**How the overlay integrates into `NewMultiIndex`:**

Option A (preferred): Add a fourth field to `MultiIndex`:
```go
type MultiIndex struct {
    Bumblebee *Index
    Overlay   *Index          // NEW — opened from local-overlay.idx; nil if absent
    OSV       policy.MultiCatalogLookup
    Socket    policy.MultiCatalogLookup
}
```

`LookupAll` queries `Overlay` via `bumblebeeMultiAdapter` (same adapter already in `multi.go`), with `CatalogSource = "local-overlay"`. Because `corroborate()` deduplicates by source name, a local-overlay match counts as one independent source — consistent with corroboration semantics.

**Overlay entry construction from CorpusRecord:**

```go
overlayEntry := catalog.Entry{
    ID:            "local-overlay-" + rec.AuditRecord.ClusterID,
    Name:          pkgName,         // from PushEnvelope.Signature.PackageOrExtensionID
    Ecosystem:     ecosystem,       // parsed from PackageOrExtensionID
    Package:       pkgName,
    Versions:      []string{rec.PushEnvelope.Signature.Version},
    Severity:      "critical",      // confirmed malicious → critical
    SourceURL:     "",              // no external source (local adjudication)
    CatalogSignature: "",          // unsigned — local-overlay entries are warn-only per CTLG-07
    CatalogSource: "local-overlay",
}
```

**IMPORTANT — unsigned overlay entries:** `CatalogSignature` is empty → per `CTLG-07` (catalog/sync.go), unsigned entries are warn-only. This means a local-overlay-only match contributes `source_count:1` → `confidence_tier:"watch"`. For `confidence_tier:"enforce"`, a second independent source (bumblebee or OSV or Socket) must also match. This is correct behavior: the local overlay is an ADDITIONAL hint, not a standalone enforce signal. A malicious package confirmed by the corpus AND by bumblebee → 2 sources → enforce.

**Owner-only requirement (FRB-05, STORE-03 pattern):** `local-overlay.json` and `local-overlay.idx` must be 0600 / Windows owner-DACL. Use `platform.SetOwnerOnly` after each write. [VERIFIED: same pattern in corpus/store.go, audit/writer.go, sentry/targets.go]

[VERIFIED: catalog/sync.go, catalog/index.go, catalog/multi.go, catalog/schema.go]

---

## Seam 5: Reversibility + Non-Auto-Purge Invariant (FRB-02)

**Purge is already human-gated. Confirm in live code:**

`internal/tui/quarantine_panel.go` lines 99-108:
```go
case "p", "P":
    if len(p.items) > 0 {
        p.confirmPurge = true  // sets a confirmation prompt; p/P alone does NOT purge
    }
// ...
case "y", "Y":
    p.confirmPurge = false
    return p, p.doPurge()    // purge only fires after explicit [y] confirmation
```

`internal/quarantine/quarantine.go` — `Purge(quarantineDir string) (purged []string, err error)` — the function itself has no confirmation; the CLI/TUI layer owns the gate. [VERIFIED]

`internal/tui/incidents.go` — `CatalogQuarantineIncidentFromRecord` — purge action is `{Key: "p", Cls: "danger", Lbl: "purge (irreversible)"}` — `danger` class renders in `styleCoral` (Beekeeper response color); requires explicit keypress. [VERIFIED: incidents.go lines 196-199]

**FRB-02 landmine: Phase 24 MUST NOT:**
- Call `quarantine.Purge` or `quarantine.MoveTyped` automatically from the adjudication path
- Add any corpus adjudication hook that calls `Purge`
- Route the `OperatorAdjudication` result to any path that directly invokes `Purge`

The ONLY path from corpus adjudication to physical artifact removal is: corpus record → `ReadMaliciousRecords` → `RunFirstResponder` → `quarantine.MoveTyped` (for quarantine, which is reversible) OR human keypress → `quarantine.Purge` (irreversible). Purge is never reachable from the adjudication batch.

**Restore (FRB-03):** `quarantine.Restore(quarantineDir, id) error` is already implemented with all path traversal guards. No changes needed. Corpus-adjudication quarantine entries are physically identical to scan-hit quarantine entries (same `beekeeper-manifest.json` format), so `Restore` works without modification. [VERIFIED: quarantine.go]

---

## Notification Path: Confirmed-Malicious Adjudication → First Responder

**The complete no-new-IPC flow (FRB-01/04):**

```
runCatalogsSync (cmd/beekeeper/catalogs_daemon.go)
  │
  ├── corpus.RunAdjudicationBatch(ctx5s, corpusPath, stateFile, idx, thresholds, cleanDays)
  │     [writes superseding records with true_label="malicious" to corpus NDJSON]
  │
  └── [NEW Phase 24] corpus.ReadMaliciousRecords(corpusPath)
        [reads latest-per-cluster records where TrueLabel=="malicious"]
        │
        └── firstResponderFn(ctx, FirstResponderConfig{
                CorpusPath:            corpusPath,
                CorpusEnabled:         cfg.Corpus.Enabled,
                CorpusSentryThreshold: 2,
                SentryTargetsPath:     filepath.Join(stateDir, "sentry-targets.json"),
                QuarantineDir:         filepath.Join(stateDir, "quarantine"),
                AuditPath:             filepath.Join(auditDir, "beekeeper.ndjson"),
                ... /* existing fields for scan-hit path unchanged */
            })
              │
              ├── CrossReference(ctx, crossRefCfg)   [existing scan-hit path — UNCHANGED]
              │     → []ScanHit for quarantine / sentry watch from scan hits
              │
              ├── [NEW Phase 24] For each malicious CorpusRecord:
              │     ├── Match against ScanHit by ecosystem+package → InstalledPath
              │     ├── if SourceCount >= CorpusSentryThreshold (=2):
              │     │     targets.AddTarget(pkgName, installedPath, expectedProcess)  [FRB-04]
              │     ├── if InstalledPath resolved:
              │     │     quarantine.MoveTyped(quarantineDir, installedPath, manifest)  [FRB-01 arm]
              │     │     writeCorpusQuarantineAudit("catalog_quarantine", ...)         [FRB-01 notify TUI]
              │     └── else:
              │           writeCorpusQuarantineAudit("pending-quarantine", ...)         [FRB-01 notify TUI pending]
              │
              └── sentry.SaveTargets(cfg.SentryTargetsPath, targets)   [FRB-04 persist]

[SEPARATE, in same runCatalogsSync call]
[NEW Phase 24] AddLocalOverlayEntry(catalogDir, overlayEntry)  [FRB-05]
```

**TUI notification path (no IPC):**

The `watchAuditLog` goroutine in `internal/tui/model.go` (line 451: `go watchAuditLog(p, m.auditPath)`) polls the audit log and sends new records as `newRecordsMsg`. When `RunFirstResponder` writes a `"catalog_quarantine"` or `"pending-quarantine"` record to the audit log, the TUI picks it up within the next poll cycle. `App.Update` already handles `newRecordsMsg` and routes sentry_alert + critical → `IncidentFromRecord`. FRB-01 extends this handling to also route `catalog_quarantine` / `pending-quarantine` → `CatalogQuarantineIncidentFromRecord`. This path already works for scan-hit quarantine alerts — the corpus-adjudication path reuses the exact same audit record types. [VERIFIED: model.go lines 102-119, incidents.go CatalogQuarantineIncidentFromRecord]

---

## Common Pitfalls

### Pitfall 1: Import cycle — corpus imports tui or tui imports corpus (HIGH RISK)

**What goes wrong:** If `internal/corpus` imports `internal/tui` for TUI notification, or if `internal/tui` imports `internal/corpus` for CorpusRecord types, a circular import is created. Go will refuse to build.

**How to avoid:** The notification path is `corpus → audit log file → tui (file read)`. No direct import between `corpus` and `tui`. The TUI reads `audit.AuditRecord` (not `corpus.CorpusRecord`) from the NDJSON log. `RunFirstResponder` in `internal/watch` reads `corpus.CorpusRecord` (via `ReadMaliciousRecords`) and writes `audit.AuditRecord` to the log. The import chain is: `cmd/beekeeper/catalogs_daemon.go` → `internal/corpus` (read malicious records) → `internal/watch` (RunFirstResponder) → `internal/audit` (write audit record) → TUI picks it up via file poll. No cycle.

**Current import constraints (VERIFIED: Phase 23 design):**
- `handler.go` MUST NOT import `corpus/adjudicator` (ADJ-01 / Pitfall 3) — unchanged
- `internal/policy` MUST remain pure (no I/O) — unchanged
- `internal/corpus` already imports `internal/audit` and `internal/policy` — no cycle risk for Phase 24

### Pitfall 2: Clobbering the local overlay with `catalogs sync` (HIGH RISK)

**What goes wrong:** If the overlay file is named `bumblebee-local.json` or is placed in a subdirectory that `SyncConditional` happens to write to, a sync would overwrite it.

**How to avoid:** Name the files `local-overlay.json` and `local-overlay.idx`. Confirm that `SyncConditional` ONLY writes `bumblebee.json` and `bumblebee.idx` (VERIFIED: sync.go lines 138-143 — `os.WriteFile(filepath.Join(catalogDir, "bumblebee.json"), ...)` and `BuildIndex(filepath.Join(catalogDir, "bumblebee.idx"), ...)`). The `local-overlay.*` names are never touched by sync.

**Required test:** Write a local overlay entry, run a mock `SyncConditional`, assert the overlay files are unchanged.

### Pitfall 3: Overlay entries treated as signed → enforce tier from single overlay source

**What goes wrong:** If `CatalogSignature` is set on an overlay entry (even to a dummy value), `VerifySignature` may return true → the entry is counted as a signed source → `source_count:1` → `confidence_tier:"watch"` is correct, but if somehow the logic counts the overlay+itself → `source_count:2` → `confidence_tier:"enforce"` from a single-source overlay.

**How to avoid:** Always set `CatalogSignature = ""` on overlay entries. Per CTLG-07, unsigned entries count as warn-only. The corroboration engine in `policy/corroboration.go` uses `m.Signed` (which comes from `CatalogSignature != ""`) to determine signed source count. An overlay entry with empty `CatalogSignature` → `Signed: false` → not counted in the signed source set → `source_count:1` unless bumblebee or another signed source ALSO matches.

### Pitfall 4: Automatic purge path introduced via corpus adjudication → RunFirstResponder

**What goes wrong:** `RunFirstResponder` is extended to call `quarantine.MoveTyped` (the reversible quarantine move) for corpus-adjudicated packages. If the planner accidentally adds a `quarantine.Purge` call (e.g., to remove an already-quarantined-and-now-confirmed entry), this violates FRB-02.

**How to avoid:** Phase 24 ONLY calls `quarantine.MoveTyped` (reversible) from the corpus path, NEVER `quarantine.Purge`. Purge remains exclusively TUI-keyboard-gated. The required test for FRB-02 asserts that no `Purge` function is called from any corpus-adjudication code path.

### Pitfall 5: ReadMaliciousRecords called on hot path

**What goes wrong:** If `ReadMaliciousRecords` is called from `internal/check/handler.go` or any synchronous hook path, it adds file I/O to the sub-100ms budget.

**How to avoid:** `ReadMaliciousRecords` must only be called from `runCatalogsSync` (off the hot path), never from `handler.go`. Apply the same `ADJ-01 / Pitfall 3` rule that already gates `RunAdjudicationBatch`.

### Pitfall 6: Overlay index rebuild is O(n) per new entry

**What goes wrong:** `AddLocalOverlayEntry` reads all existing entries, appends the new one, and calls `BuildIndex`. With N entries this is O(N) per addition. For v1 (likely < 20 overlay entries), this is fine. For large fleets it would degrade.

**How to avoid:** The v1 cap is explicitly acceptable (PRD §3.4 is local-only; no fleet push in v1). Document the known limitation and add a cap (e.g., `maxOverlayEntries = 1000`) with a logged warning when exceeded. No optimization needed in Phase 24.

### Pitfall 7: Multiple concurrent `catalogs sync` processes writing the overlay simultaneously

**What goes wrong:** Two `beekeeper catalogs sync` processes run simultaneously (e.g., OS scheduler fires while user manually syncs). Both call `AddLocalOverlayEntry` → both read the existing file → both call `BuildIndex` → last-writer-wins, potentially losing one entry.

**How to avoid:** Use `writeFileAtomic` (already in `catalog/index.go`) for the overlay JSON write. For the index, `BuildIndex` already uses `writeFileAtomic` internally. The race window is at the read-then-write between `LoadLocalOverlay` and `BuildIndex`. Acceptable for v1 (same race exists for `state.json`; `SaveState` uses `writeFileAtomic` for the same reason). Document as a known gap; no file lock needed in Phase 24.

---

## Local Catalog Overlay — Survival Across `catalogs sync`

**Definitive proof that local-overlay.* files survive sync:**

`SyncConditional` (catalog/sync.go) writes EXACTLY these paths:
```
catalogDir/bumblebee.json    ← WriteFile
catalogDir/bumblebee.idx     ← BuildIndex (atomic rename via .tmp-)
```

No glob, no `os.RemoveAll(catalogDir)`, no wildcard deletion. Any file in `catalogDir` whose name is NOT `bumblebee.json` or `bumblebee.idx` is completely untouched. `local-overlay.json` and `local-overlay.idx` survive permanently until explicitly deleted by the operator or a future `beekeeper catalogs overlay clear` command (Phase 25 LAUNCH-01 concern, not Phase 24).

**How lookups merge (VERIFIED: catalog/multi.go LookupAll):**

With the Phase 24 extension to `MultiIndex`:
```
MultiIndex.LookupAll(ecosystem, pkg):
  matches := bumblebeeAdapter.LookupAll(ecosystem, pkg)    // bumblebee.idx
  matches += overlayAdapter.LookupAll(ecosystem, pkg)       // local-overlay.idx (NEW)
  matches += OSV.LookupAll(ecosystem, pkg)                  // OSV REST (if configured)
  matches += Socket.LookupAll(ecosystem, pkg)               // Socket PURL (if configured)
  return matches
```

`corroborate()` in `policy/corroboration.go` deduplicates by `CatalogSource` name. A package in both bumblebee AND local-overlay: `source_count: 2` → `confidence_tier: "enforce"`. A package ONLY in local-overlay: `source_count: 1` → `confidence_tier: "watch"` (unsigned, single source). This is the correct behavior — the overlay SUPPLEMENTS but does not replace the bumblebee catalog.

---

## Architecture Patterns

### System Architecture Diagram

```
PHASE 23 OUTPUT (already running)
┌──────────────────────────────────────────────────────────────┐
│  runCatalogsSync                                              │
│    └─ corpus.RunAdjudicationBatch(ctx5s, corpusPath, ...)    │
│         writes superseding records: true_label="malicious"   │
│         to beekeeper-corpus.ndjson (append-only NDJSON)      │
└──────────────────────────────────────────────────────────────┘

PHASE 24 ADDITIONS (new wiring in same runCatalogsSync call)
┌──────────────────────────────────────────────────────────────┐
│  [NEW] corpus.ReadMaliciousRecords(corpusPath)               │
│         → []CorpusRecord{true_label:"malicious"}             │
│                                                              │
│  [NEW] firstResponderFn(ctx, FirstResponderConfig{           │
│           CorpusPath: corpusPath, CorpusEnabled: true,       │
│           ...existing fields...})                            │
│    ├── CrossReference(ctx, crossRefCfg)  [UNCHANGED]        │
│    │    → []ScanHit (installed packages)                     │
│    │                                                          │
│    ├── For each malicious CorpusRecord:               FRB-01│
│    │    ├── Match ScanHit by ecosystem+package               │
│    │    ├── if SourceCount>=2: AddTarget(...)         FRB-04│
│    │    ├── if PathResolved: MoveTyped(...)           FRB-01│
│    │    │    └─ writeAudit("catalog_quarantine")      FRB-01│
│    │    └── else: writeAudit("pending-quarantine")    FRB-01│
│    │                                                          │
│    └── SaveTargets(sentryTargetsPath, targets)        FRB-04│
│                                                              │
│  [NEW] AddLocalOverlayEntry(catalogDir, overlayEntry) FRB-05│
│         → rebuilds local-overlay.json + local-overlay.idx    │
│         → SURVIVES next catalogs sync                        │
└──────────────────────────────────────────────────────────────┘

TUI NOTIFICATION (no IPC — file poll, already running)
┌──────────────────────────────────────────────────────────────┐
│  watchAuditLog goroutine (model.go:451)                      │
│    → reads new audit records → newRecordsMsg                 │
│    → App.Update handles "catalog_quarantine":                │
│         CatalogQuarantineIncidentFromRecord(rec, false)      │
│         → IncidentModel with [r]estore / [p]urge buttons     │
│         → coral badge (Beekeeper response, FRB-03 locked)    │
└──────────────────────────────────────────────────────────────┘

SENTRY DAEMON (reads targets file on startup — no new IPC)
┌──────────────────────────────────────────────────────────────┐
│  Sentry daemon reads sentry-targets.json on startup          │
│  TargetList.MatchesPID(pid, processTree) → bool             │
│  True → tighten correlation thresholds (DETECTION ONLY)      │
│  NEVER: kill / isolate / network-cut                         │
└──────────────────────────────────────────────────────────────┘

CATALOG LOOKUP (hook handler — beekeeper check)
┌──────────────────────────────────────────────────────────────┐
│  MultiIndex.LookupAll(ecosystem, pkg)                        │
│    ├── bumblebee.idx (mmap, O(log n))                        │
│    ├── local-overlay.idx (mmap, O(log n))   [NEW FRB-05]    │
│    ├── OSV (per-request, if configured)                      │
│    └── Socket (per-request, if configured)                   │
│  → policy.Evaluate → corroborate() dedup by CatalogSource   │
└──────────────────────────────────────────────────────────────┘
```

### Recommended File Structure (Phase 24 creates/modifies)

```
internal/corpus/
  [PHASE 24 NEW]
  reader.go              # ReadMaliciousRecords(corpusPath) — reads confirmed adjudications
  reader_test.go         # TestReadMaliciousRecords* (round-trip, empty file, wrong label)

internal/catalog/
  [PHASE 24 NEW]
  local_overlay.go       # AddLocalOverlayEntry, LoadLocalOverlay, overlayEntry construction
  local_overlay_test.go  # TestLocalOverlay* (survival across sync, idempotency, 0600 perms)
  multi.go               # MODIFIED: MultiIndex gains Overlay *Index field; LookupAll queries it
  multi_test.go          # MODIFIED: TestMultiIndex gains overlay test case

internal/watch/
  firstresponder.go      # MODIFIED: FirstResponderConfig gains CorpusPath/CorpusEnabled fields;
                         #   RunFirstResponder extended to process corpus malicious records
  firstresponder_test.go # MODIFIED: TestFirstResponder* gains corpus-adjudication test cases

cmd/beekeeper/
  catalogs_daemon.go     # MODIFIED: runCatalogsSync calls ReadMaliciousRecords + firstResponderFn
                         #   + AddLocalOverlayEntry after adjudication batch pass

internal/tui/
  model.go               # POSSIBLY MODIFIED: App.Update handling for corpus_quarantine records
                         # (may be no-op if existing "catalog_quarantine" / "pending-quarantine"
                         #  record types are reused without a new handler branch)
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Reading confirmed-malicious records | Custom NDJSON scanner | `corpus.ReadMaliciousRecords(corpusPath)` (new function reusing `RunAdjudicationBatch` scan logic) | Same latest-per-cluster collapse; avoids duplicating the NDJSON scan logic |
| Mmap index for overlay | Custom binary format | `catalog.BuildIndex(path, entries)` | Already tested, handles all edge cases including Windows mmap constraints |
| Overlay survival across sync | Atomic file rename trick | Name files `local-overlay.*` (SyncConditional only writes `bumblebee.*`) | Zero code — just correct naming; SyncConditional never globs catalogDir |
| Sentry watch addition | New IPC channel | `sentry.AddTarget` + `sentry.SaveTargets` | Already the mechanism; the daemon reads targets on startup |
| TUI notification | New message type | Existing `"catalog_quarantine"` / `"pending-quarantine"` audit record types | `CatalogQuarantineIncidentFromRecord` and `recordToRow` already handle these |
| Purge confirmation gate | New CLI prompt | Existing TUI `confirmPurge = true` guard | Already double-gated: p key → prompt → y key → Purge; phase 24 must not bypass this |
| Local overlay entry permissions | chmod manually | `platform.SetOwnerOnly(path)` | Cross-platform (Windows DACL + POSIX chmod); already used by audit, corpus, sentry targets |

---

## Validation Architecture

> `workflow.nyquist_validation` is not explicitly set to false in config — include this section.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None — `go test ./...` |
| Quick run command | `go test ./internal/corpus/... ./internal/catalog/... ./internal/watch/... -short` |
| Full suite command | `go test ./... -count=1` |
| Build verification | `go build ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File |
|--------|----------|-----------|-------------------|------|
| FRB-01 | Confirmed-malicious adjudication arms TUI quarantine card via audit record | integration | `go test ./internal/watch/... -run TestFirstResponderCorpusMaliciousArmsCard` | `watch/firstresponder_test.go` (extended) |
| FRB-01 | `ReadMaliciousRecords` returns only TrueLabel=="malicious" records | unit | `go test ./internal/corpus/... -run TestReadMaliciousRecords` | `corpus/reader_test.go` (new) |
| FRB-01 | Latest-per-cluster record wins (superseding adjudication takes precedence) | unit | `go test ./internal/corpus/... -run TestReadMaliciousRecordsLatestPerCluster` | `corpus/reader_test.go` (new) |
| FRB-02 | No Purge call from corpus adjudication path | negative/static | `grep -rn 'quarantine.Purge\|Purge(' internal/corpus/ internal/watch/firstresponder.go` — must return 0 matches | CI gate |
| FRB-02 | TUI purge remains human-confirmed (p key → y key required) | unit | `go test ./internal/tui/... -run TestQuarantinePanelPurgeRequiresConfirmation` | `tui/quarantine_panel_test.go` (existing, verify unchanged) |
| FRB-03 | Restore available for corpus-adjudication quarantine entries | unit | `go test ./internal/quarantine/... -run TestRestoreCorpusQuarantineEntry` | `quarantine/quarantine_test.go` (existing pattern, new fixture) |
| FRB-04 | Sentry watch added only when source_count >= 2 (enforce tier) | unit | `go test ./internal/watch/... -run TestFirstResponderCorpusSentryGate` | `watch/firstresponder_test.go` (new case) |
| FRB-04 | Single-source (watch tier) does NOT elevate Sentry watch | unit | `go test ./internal/watch/... -run TestFirstResponderCorpusSingleSourceNoSentry` | `watch/firstresponder_test.go` (new case) |
| FRB-04 | TargetList.AddTarget idempotent for corpus entries | unit | `go test ./internal/sentry/... -run TestTargetListAddTargetIdempotent` | `sentry/targets_test.go` (existing, verify) |
| FRB-05 | Local overlay entry survives `catalogs sync` (mock SyncConditional) | unit | `go test ./internal/catalog/... -run TestLocalOverlaySurvivesSync` | `catalog/local_overlay_test.go` (new) |
| FRB-05 | Local overlay entry appears in `MultiIndex.LookupAll` result | unit | `go test ./internal/catalog/... -run TestMultiIndexQueriesOverlay` | `catalog/multi_test.go` (extended) |
| FRB-05 | Overlay file is 0600 / owner-only | unit | `go test ./internal/catalog/... -run TestLocalOverlayFilePermissions` | `catalog/local_overlay_test.go` (new) |
| FRB-05 | Overlay entry with empty CatalogSignature → source_count:1 (warn, not enforce alone) | unit | `go test ./internal/catalog/... -run TestLocalOverlayUnsignedIsWarnTier` | `catalog/local_overlay_test.go` (new) |
| FRB-05 | Overlay + bumblebee match → source_count:2, confidence_tier:"enforce" | unit | `go test ./internal/catalog/... -run TestLocalOverlayPlusBumblebeeIsEnforce` | `catalog/local_overlay_test.go` (new) |

### Synthetic Nx Console Incident Fixture

The evaluator gate (PRD §4 Phase 2) requires a confirmed local Nx Console match that arms the card and does not auto-purge. The synthetic fixture:

```go
// Synthetic Nx Console corpus record (confirmed malicious, enforce tier):
corpusRec := corpus.CorpusRecord{
    AuditRecord: audit.AuditRecord{
        RecordID:    "test-nx-console-001",
        ClusterID:   "nx-console-cluster-001",
        ToolName:    "@nrwl/nx-console",
        RecordType:  "policy_decision",
        Decision:    "block",
        Timestamp:   time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339),
    },
    TrueLabel:           "malicious",
    AdjudicationSource:  "catalog_confirmation",
    CorpusSchemaVersion: "1.0",
    PushEnvelope: &corpus.PushEnvelope{
        Signature: corpus.EnvelopeSignature{
            PackageOrExtensionID: "npm:@nrwl/nx-console",
            Version:              "17.3.0",
        },
        TrueLabel:      "malicious",
        ConfidenceTier: "enforce",
        SourceCount:    2,
        ActionHint:     corpus.ActionHintWatchAndBlock,
    },
}
```

The evaluator gate asserts:
1. `ReadMaliciousRecords` returns the above record.
2. `RunFirstResponder` (with a fake CrossReference returning a matching ScanHit) writes a `"catalog_quarantine"` audit record.
3. `sentry-targets.json` contains `@nrwl/nx-console` (FRB-04: source_count=2 >= threshold=2).
4. `quarantine.List(quarantineDir)` contains one entry for `@nrwl/nx-console` (FRB-01: armed).
5. NO `quarantine.Purge` was called (FRB-02: auto-purge absent).
6. `quarantine.Restore(quarantineDir, entryID)` succeeds (FRB-03: reversible).
7. `MultiIndex.LookupAll("npm", "@nrwl/nx-console")` returns >= 1 match with `CatalogSource=="local-overlay"` (FRB-05).

### Sampling Rate

- **Per task commit:** `go test ./internal/corpus/... ./internal/catalog/... ./internal/watch/... -short`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite + FRB evaluator gate before `/gsd-verify-work`

### Wave 0 Gaps (files that must exist before implementation)

- `internal/corpus/reader_test.go` — new file (RED skeletons before implementation)
- `internal/catalog/local_overlay_test.go` — new file (RED skeletons before implementation)
- `internal/watch/firstresponder_test.go` — EXTEND (add corpus-adjudication test cases)

---

## Recommended Plan Shape

**Total plans: 3 (3 waves). Zero parallel plans — each wave has dependencies on the prior.**

### Plan 24-01: Corpus Reader + Local Catalog Overlay (FRB-01 signal source + FRB-05)

**Wave 1 — Foundation: the two "new construction" items**

Deliverables:
- `internal/corpus/reader.go` — `ReadMaliciousRecords(corpusPath string) ([]CorpusRecord, error)`. Uses the same latest-per-cluster collapse as `RunAdjudicationBatch`. Returns only records where `TrueLabel == "malicious"`. Returns nil for missing file (not error).
- `internal/corpus/reader_test.go` — unit tests: empty file, records with wrong label filtered out, latest-per-cluster wins (superseding adjudication), redaction safe (no secret leakage).
- `internal/catalog/local_overlay.go` — `AddLocalOverlayEntry(catalogDir string, e Entry) error` (read existing + append + BuildIndex atomic); `LoadLocalOverlay(catalogDir string) ([]Entry, error)` (missing file → nil, nil).
- `internal/catalog/multi.go` — MODIFIED: `MultiIndex` gains `Overlay *Index` field; `NewMultiIndex` signature extended to accept overlay (or add `NewMultiIndexWithOverlay`); `LookupAll` queries overlay via `bumblebeeMultiAdapter`; `Close` closes overlay.
- `internal/catalog/local_overlay_test.go` — unit tests: survival across mock sync (verify `bumblebee.*` write does NOT touch `local-overlay.*`); file permissions 0600; unsigned entry → SourceCount:1 warn; overlay + bumblebee → SourceCount:2 enforce; idempotent add.
- `internal/catalog/multi_test.go` — EXTENDED: overlay LookupAll case.

Dependencies: Phase 23 types (frozen), `catalog.BuildIndex` (existing), `catalog.OpenIndex` (existing), `platform.SetOwnerOnly` (existing).

Commit strategy: Wave-0 RED skeletons → reader.go GREEN → local_overlay.go + multi.go extension GREEN.

### Plan 24-02: First Responder Corpus Wiring (FRB-01/03/04)

**Wave 2 — wire the signal into quarantine + Sentry watch**

Deliverables:
- `internal/watch/firstresponder.go` — MODIFIED:
  - `FirstResponderConfig` gains `CorpusPath string`, `CorpusEnabled bool`, `CorpusSentryThreshold int` (default 2).
  - `RunFirstResponder` extended: after CrossReference, if `CorpusEnabled && CorpusPath != ""`: call `corpus.ReadMaliciousRecords(cfg.CorpusPath)`, then for each malicious record: (a) match against `ScanHit` by ecosystem+package, (b) if `SourceCount >= CorpusSentryThreshold`: `targets.AddTarget(...)` [FRB-04], (c) if `PathResolved`: `quarantine.MoveTyped(...)` + writeCorpusQuarantineAudit("catalog_quarantine") [FRB-01], (d) else: writeCorpusQuarantineAudit("pending-quarantine") [FRB-01 pending].
  - New helper `parsePackageID(id string) (ecosystem, pkg string)` — splits `"npm:@nrwl/nx-console"` into `("npm", "@nrwl/nx-console")`.
- `internal/watch/firstresponder_test.go` — EXTENDED:
  - `TestFirstResponderCorpusMaliciousArmsCard` — confirms audit record written with `"catalog_quarantine"` type.
  - `TestFirstResponderCorpusSentryGate` — SourceCount=2 → target added; SourceCount=1 → target NOT added.
  - `TestFirstResponderCorpusSingleSourceNoSentry` — enforce that single-source (watch tier) corpus record does NOT add to sentry targets.
  - `TestFirstResponderCorpusNoPurge` — no `quarantine.Purge` call, only `quarantine.MoveTyped`.
  - `TestFirstResponderCorpusPendingQuarantine` — no matching ScanHit → pending-quarantine record.

Dependencies: Plan 24-01 (corpus.ReadMaliciousRecords must exist), `internal/quarantine` (existing), `internal/sentry` (existing), `internal/audit` (existing).

### Plan 24-03: Catalog Sync Wiring + TUI Validation (FRB-05 overlay + evaluator gate)

**Wave 3 — wire everything into runCatalogsSync and validate end-to-end**

Deliverables:
- `cmd/beekeeper/catalogs_daemon.go` — MODIFIED:
  - After `corpus.RunAdjudicationBatch(...)`: call `corpus.ReadMaliciousRecords(corpusPath)` → call `firstResponderFn(ctx, FirstResponderConfig{CorpusPath: corpusPath, CorpusEnabled: cfg.Corpus.Enabled, ...})` [FRB-01/04].
  - For each malicious record with `PushEnvelope.Signature.PackageOrExtensionID` populated: call `catalog.AddLocalOverlayEntry(dir, overlayEntry)` [FRB-05].
  - Errors from both calls: logged to stderr, sync continues (same non-fatal pattern as adjudication batch pass).
- `cmd/beekeeper/catalogs_daemon_test.go` (or existing test) — EXTENDED: integration test verifying the full `runCatalogsSync` FRB path with a synthetic confirmed-malicious corpus record.
- `internal/tui/model.go` — MODIFIED (if needed): `App.Update` newRecordsMsg handler checks for `"catalog_quarantine"` record type from corpus wiring → calls `CatalogQuarantineIncidentFromRecord(rec, false)`. Verify this is not already handled (it IS already handled by `recordToRow` in `alerts_panel.go`; the incident card path in `App.Update` may need extending for corpus records).
- **Evaluator gate test:** Synthetic Nx Console round-trip (as described in Validation Architecture above) — passes before `/gsd-verify-work`.

Dependencies: Plans 24-01 + 24-02 (ReadMaliciousRecords + RunFirstResponder extension + AddLocalOverlayEntry must all exist).

### Dependency Order

```
24-01 (reader + overlay) → 24-02 (firstresponder) → 24-03 (sync wiring + E2E gate)
```

No parallel plans in Phase 24 — each plan's deliverables are required by the next.

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | — |
| V3 Session Management | No | — |
| V4 Access Control | Yes | 0600 / owner-DACL on all new files (`local-overlay.json`, `local-overlay.idx`); corpus path validation via `ResolveCorpusPath` (already enforced) |
| V5 Input Validation | Yes | `parsePackageID` must validate format; overlay entry fields must not leak corpus HMAC fingerprints |
| V6 Cryptography | No | Overlay entries intentionally have no signature (unsigned = warn-only per CTLG-07) |

### Known Threat Patterns for Phase 24 Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Agent reads local-overlay.json to learn what packages are flagged | Information Disclosure | 0600 owner-only + self-protection `EvaluateSelfPath` blocks agent reads of `StateDir`; overlay is under `CatalogDir` which is in `StateDir` |
| Agent writes local-overlay.json to inject false catalog entries | Tampering | 0600 owner-only; corpus path under StateDir; self-protection guard covers StateDir |
| Overlay entry for a safe package causes false-positive block | Tampering (poisoning) | Overlay entries are unsigned → warn-only (source_count:1 → warn); enforce requires 2+ signed sources; local-only, no fleet push |
| `ReadMaliciousRecords` blocked by corrupted corpus NDJSON | Denial of Service | Malformed lines are skipped silently (same pattern as `RunAdjudicationBatch` scanner); missing file returns nil (not error) |
| `AddLocalOverlayEntry` race with concurrent sync → clobbered overlay | Integrity | `writeFileAtomic` for JSON; `BuildIndex` already uses `writeFileAtomic`; documented as known gap; acceptable for v1 |
| corpus adjudication path triggers auto-purge | Tampering / Blast Radius | Phase 24 ONLY calls `quarantine.MoveTyped` (reversible); `quarantine.Purge` unreachable from any corpus path; grep gate in CI |

---

## Open Questions

1. **TUI model.go extension scope** — Does `App.Update` need a new handler branch for `"catalog_quarantine"` records emitted by corpus wiring, or is the existing `recordToRow` path in `alerts_panel.go` sufficient? The existing `recordToRow` handles `"catalog_quarantine"` but the App-level incident card (shown in calm mode for critical events) currently only checks for `"sentry_alert"`. For corpus-adjudication cards, the planner should decide whether to surface a calm-mode incident card or only a panel alert. **Recommendation:** Start with panel-only (alert panel) for Phase 24; calm-mode incident card can be added in Phase 25 if LAUNCH-01 E2E requires it.

2. **`CorpusSentryThreshold` config key** — Should the Sentry watch threshold for corpus-adjudicated packages be separate from the scan-hit corroboration threshold (`FirstResponderConfig.Threshold`)? **Recommendation:** Yes — expose `CorpusSentryThreshold` separately (default 2), matching the `source_count >= 2 = enforce` rule from FRB-04. The scan-hit threshold uses `CorroborationCount` (number of distinct catalog sources that matched the scan hit), which is semantically different from `PushEnvelope.SourceCount` (distinct sources at adjudication time). Keep them separate to avoid confusion.

3. **`NewMultiIndex` vs `NewMultiIndexWithOverlay`** — Should the overlay field be added to the existing `NewMultiIndex` signature (potentially breaking all callers), or should a new `NewMultiIndexWithOverlay` overload be added? **Recommendation:** Add an optional `overlayPath string` parameter to a new `NewMultiIndexWithOverlay(bumblebee *Index, osv, socket policy.MultiCatalogLookup, overlayPath string) *MultiIndex`. Keep `NewMultiIndex` as a thin wrapper calling `NewMultiIndexWithOverlay` with `overlayPath=""`. This is non-breaking and follows the `NewMultiSinkWithCorpus` pattern from Phase 23.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25+ toolchain | All | ✓ | 1.25 (CLAUDE.md) | — |
| `internal/corpus` (Phase 23 shipped) | ReadMaliciousRecords | ✓ | Phase 23 complete | — |
| `internal/catalog/index.go` (BuildIndex, OpenIndex) | local_overlay.go | ✓ | Existing | — |
| `internal/sentry/targets.go` (AddTarget, SaveTargets) | firstresponder.go extension | ✓ | Existing | — |
| `internal/quarantine/quarantine.go` (MoveTyped, Restore) | firstresponder.go extension | ✓ | Existing | — |
| `internal/watch/firstresponder.go` (RunFirstResponder) | Wave 2 | ✓ | Existing | — |
| `charm.land/bubbletea/v2` (TUI) | tui/model.go | ✓ | Phase 23+ (existing) | — |

**Missing dependencies with no fallback:** None.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `SyncConditional` ONLY writes `bumblebee.json` and `bumblebee.idx` — confirmed by reading sync.go lines 138-143 | Seam 4 | If a future sync also writes `local-overlay.*`, the overlay would be clobbered. Mitigate by using a subdirectory (`catalog/overlays/local-overlay.*`) instead of the top-level catalogDir |
| A2 | The existing `"catalog_quarantine"` audit record type is sufficient for FRB-01 TUI arming without a new record type | Seam 2 | If a distinct visual treatment is required for corpus-adjudication vs scan-hit quarantine in the TUI, a new record type `"corpus_quarantine"` would be needed; `App.Update` and `recordToRow` would need extension |
| A3 | `bumblebeeMultiAdapter` can wrap the overlay `*Index` for `LookupAll` semantics (same adapter used in `multi.go` for the bumblebee index) | Seam 4 | If the adapter has bumblebee-specific logic that cannot be reused for the overlay, a separate `overlayMultiAdapter` would be needed; likely identical code |

**If these assumptions are wrong:** No locked decisions change; only implementation details shift. The planner should re-read the relevant file before executing the affected task.

---

## Project Constraints (from CLAUDE.md)

- **Go 1.25+, single static binary, no CGO in core** — Zero new go.mod entries required; all new code uses stdlib + existing internal packages. [VERIFIED: Phase 23 zero-new-deps pattern]
- **`internal/policy` MUST stay a pure function library** — Phase 24 does not touch `internal/policy`. [SATISFIED]
- **Fail closed by default** — `ReadMaliciousRecords` missing file → nil (not error); `AddLocalOverlayEntry` errors → logged, sync continues; `RunFirstResponder` corpus errors → logged, existing scan-hit path unaffected. [REQUIRED]
- **Windows is primary dev machine** — All file writes use `platform.SetOwnerOnly` for owner-DACL; overlay path uses `filepath.Join`; no hardcoded paths. [REQUIRED]
- **Bubble Tea v2 import path:** `charm.land/bubbletea/v2` — NOT `github.com/charmbracelet`. Windows resize polling workaround already in place. Any TUI extension in `model.go` must use this import path. [REQUIRED]
- **Hook handler: fail closed, sub-100ms** — `ReadMaliciousRecords` and `AddLocalOverlayEntry` are called ONLY from `runCatalogsSync` (off-hot-path). `handler.go` MUST NOT import `corpus/reader.go` or `catalog/local_overlay.go`. [REQUIRED]
- **Reproducible builds** — No new binary-embedded data; no new runtime generation. [OK]
- **self-protection** — Both `local-overlay.json` and `local-overlay.idx` live under `CatalogDir` which is inside `StateDir`. The existing `EvaluateSelfPath` guard in self-protection covers the StateDir prefix, so agents cannot read or write these files. [VERIFIED: selfprotect.go pattern]

---

## Sources

### Primary (HIGH confidence — verified against live source files 2026-06-14)

- `internal/corpus/adjudicator.go` — `Adjudicate`, `RunAdjudicationBatch`, `OperatorAdjudication`, `AppendCorpusRecordLine`, `AdjudicationSignals`, `AdjudicationResult` [VERIFIED]
- `internal/corpus/store.go` — `StoreSink`, `AppendCorpusRecordLine`, `ResolveCorpusPath` [VERIFIED]
- `internal/corpus/emitter.go` — `MapToCorpusRecord`, `BuildPushEnvelope`, `AdjudicationResult`, `corroborationTierAndCount` [VERIFIED]
- `internal/corpus/types.go` — `CorpusRecord`, `PushEnvelope`, `EnvelopeSignature` [VERIFIED]
- `internal/sentry/targets.go` — `TargetList`, `TargetEntry`, `AddTarget`, `LoadTargets`, `SaveTargets` — DETECTION-ONLY comment [VERIFIED]
- `internal/tui/quarantine_panel.go` — `QuarantinePanel`, `doPurge`, `doRestore`, `confirmPurge` guard [VERIFIED]
- `internal/tui/incidents.go` — `IncidentFromRecord`, `CatalogQuarantineIncidentFromRecord`, `buildCatalogQuarantineDesc`, purge/restore action buttons [VERIFIED]
- `internal/tui/model.go` — `App`, `newRecordsMsg`, `watchAuditLog`, `quarantineAlertMsg`, incident card flow [VERIFIED]
- `internal/tui/alerts_panel.go` — `recordToRow` handling for `catalog_quarantine` / `pending-quarantine` / `sentry_alert` [VERIFIED]
- `internal/tui/styles.go` — `colorRed`/`colorCoral` locked semantic, `BadgeCrit`/`BadgeBlock`/`BadgeHeld` [VERIFIED]
- `internal/catalog/multi.go` — `MultiIndex`, `NewMultiIndex`, `LookupAll`, `bumblebeeMultiAdapter` [VERIFIED]
- `internal/catalog/sync.go` — `SyncConditional` ONLY writes `bumblebee.json` + `bumblebee.idx` [VERIFIED]
- `internal/catalog/index.go` — `BuildIndex`, `OpenIndex`, `Index`, `writeFileAtomic` [VERIFIED]
- `internal/catalog/schema.go` — `Entry` struct fields [VERIFIED]
- `internal/catalog/state.go` — `WatchState`, `LoadState`, `SaveState` [VERIFIED]
- `internal/quarantine/quarantine.go` — `MoveTyped`, `Restore`, `Purge`, `Manifest` [VERIFIED]
- `internal/watch/firstresponder.go` — `RunFirstResponder`, `FirstResponderConfig`, F-4 gate, ecosystem-to-process mapping [VERIFIED]
- `internal/watch/crossref.go` — `ScanHit`, `CrossRefConfig` [VERIFIED]
- `cmd/beekeeper/catalogs_daemon.go` — `runCatalogsSync`, Phase 23 adjudication batch pass wiring [VERIFIED]

### Secondary (HIGH confidence — Phase 23 execution summaries)

- `.planning/phases/23-corpus-store-adjudication-engine/23-01-SUMMARY.md` — StoreSink, salt storage, CorpusLocalSalt moved to dedicated salt file
- `.planning/phases/23-corpus-store-adjudication-engine/23-02-SUMMARY.md` — MapToCorpusRecord, BuildPushEnvelope, AdjudicationResult, corroborationTierAndCount
- `.planning/phases/23-corpus-store-adjudication-engine/23-03-SUMMARY.md` — RunAdjudicationBatch, OperatorAdjudication stub, appendCorpusRecord, writeCorpusRecordDirect, NewMultiSinkWithCorpus
- `.planning/phases/23-corpus-store-adjudication-engine/23-RESEARCH.md` — OQ-3 adjudicator lifecycle, seam map, pitfalls

### Tertiary (MEDIUM confidence — PRD + requirements)

- `beekeeper-corpus-milestone-prd.md` §3.4 First Responder integration — confirms: purge = human-confirmed; restore available; red/coral semantic locked; watch_and_block = only fleet action
- `.planning/REQUIREMENTS.md` FRB-01..05 — requirement text used directly for test map

---

## Metadata

**Confidence breakdown:**
- Signal source seam (corpus reader): HIGH — `RunAdjudicationBatch` logic confirmed in live code; `ReadMaliciousRecords` is a straightforward subset
- TUI quarantine card seam: HIGH — `CatalogQuarantineIncidentFromRecord`, `QuarantinePanel`, `recordToRow` all confirmed in live code; color semantics locked
- Sentry TargetList seam: HIGH — `AddTarget`, `SaveTargets`, F-4 gate all confirmed in live `firstresponder.go` and `targets.go`
- Local catalog overlay: MEDIUM — NO EXISTING overlay mechanism; `BuildIndex` + `SyncConditional` scope confirmed; overlay design derived from existing patterns but requires new construction
- Plan shape: HIGH — follows Phase 23 wave structure; 3 sequential plans; dependencies are definitive

**Research date:** 2026-06-14
**Valid until:** 2026-07-14 (frozen Phase 23 types; stable Go stdlib; MultiIndex extension is additive and non-breaking)
