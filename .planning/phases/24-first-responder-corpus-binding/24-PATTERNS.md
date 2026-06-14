# Phase 24: First Responder Corpus Binding — Pattern Map

**Mapped:** 2026-06-14
**Files analyzed:** 5 (2 new, 3 modified)
**Analogs found:** 5 / 5

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/corpus/reader.go` (NEW) | utility/reader | batch file-I/O | `internal/corpus/adjudicator.go` `RunAdjudicationBatch` scan loop | exact — same NDJSON scanner, same latest-per-cluster collapse |
| `internal/catalog/local_overlay.go` (NEW) | utility/store | file-I/O + CRUD | `internal/catalog/index.go` (`BuildIndex`/`writeFileAtomic`) + `internal/corpus/store.go` (`SetOwnerOnly` pattern) | role-match + data-flow-match |
| `internal/catalog/multi.go` (MODIFIED) | aggregator | request-response | itself — existing `MultiIndex`/`LookupAll`/`bumblebeeMultiAdapter` | exact (additive extension) |
| `internal/watch/firstresponder.go` (MODIFIED) | orchestrator | event-driven | itself — existing `RunFirstResponder` body | exact (additive extension) |
| `cmd/beekeeper/catalogs_daemon.go` (MODIFIED) | command wiring | batch | itself — existing `runCatalogsSync` Phase 23 adjudication batch pass | exact (continuation of same pattern) |

---

## Pattern Assignments

### `internal/corpus/reader.go` (NEW — utility, batch file-I/O)

**Analog:** `internal/corpus/adjudicator.go` — `RunAdjudicationBatch` (lines 253–438)

**Imports pattern** (analog lines 22–34):
```go
import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
)
// No internal imports needed — CorpusRecord is in the same package (corpus).
// Do NOT import internal/tui. Do NOT import internal/check.
```

**Core scan + latest-per-cluster pattern** (analog lines 255–302):
```go
// Open — missing file is nil/nil (not an error).
f, err := os.Open(corpusPath)
if err != nil {
    if os.IsNotExist(err) {
        return nil, nil // no corpus file yet — not an error
    }
    return nil, fmt.Errorf("open corpus for reading: %w", err)
}
defer f.Close()

var allRecords []CorpusRecord
scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB line buffer (matches adjudicator)
scanned := 0
for scanner.Scan() {
    if scanned >= maxRecordsToScan { // reuse the existing const (50_000)
        break
    }
    line := scanner.Bytes()
    if len(line) == 0 {
        continue
    }
    var rec CorpusRecord
    if err := json.Unmarshal(line, &rec); err != nil {
        continue // skip malformed lines — do not abort
    }
    allRecords = append(allRecords, rec)
    scanned++
}
if err := scanner.Err(); err != nil {
    return nil, fmt.Errorf("scan corpus: %w", err)
}
f.Close() // close before any further work (Windows cannot write to open-for-read files)

// Latest-per-cluster collapse (analog lines 290–302):
latestByCluster := make(map[string]CorpusRecord)
for _, rec := range allRecords {
    id := clusterKeyOf(rec) // reuse the existing unexported helper in adjudicator.go
    if id == "" {
        continue
    }
    latestByCluster[id] = rec // last write wins (NDJSON append order)
}

// Filter to malicious-only:
var out []CorpusRecord
for _, rec := range latestByCluster {
    if rec.TrueLabel == "malicious" {
        out = append(out, rec)
    }
}
return out, nil
```

**Key constraints:**
- `ReadMaliciousRecords` lives in `internal/corpus/reader.go` — same package as `clusterKeyOf` and `maxRecordsToScan`, so both can be reused without export.
- No context parameter — this is a fast read (< 10MB corpus). The caller (`runCatalogsSync`) already has the 5s deadline on `RunAdjudicationBatch`; `ReadMaliciousRecords` runs after that and is a pass over an already-drained small file.
- MUST NOT be called from `internal/check/handler.go` (ADJ-01 / Pitfall 5).

---

### `internal/catalog/local_overlay.go` (NEW — utility/store, file-I/O + CRUD)

**Analog A:** `internal/catalog/index.go` — `BuildIndex` + `writeFileAtomic` (lines 62–138)
**Analog B:** `internal/corpus/store.go` — `platform.SetOwnerOnly` pattern (lines 51–75)

**Imports pattern** (derived from both analogs):
```go
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.com/bantuson/beekeeper/internal/platform"
)
// catalog package — no internal/tui, no internal/corpus imports here.
```

**LoadLocalOverlay pattern** (modeled on `os.Open` + missing-file → nil):
```go
func LoadLocalOverlay(catalogDir string) ([]Entry, error) {
    path := filepath.Join(catalogDir, "local-overlay.json")
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil // first run — not an error
        }
        return nil, fmt.Errorf("read local overlay: %w", err)
    }
    var entries []Entry
    if err := json.Unmarshal(data, &entries); err != nil {
        return nil, fmt.Errorf("parse local overlay: %w", err)
    }
    return entries, nil
}
```

**AddLocalOverlayEntry: read→deduplicate→append→BuildIndex→SetOwnerOnly pattern**
(BuildIndex analog: `index.go` lines 62–113; SetOwnerOnly analog: `store.go` lines 61–67):
```go
func AddLocalOverlayEntry(catalogDir string, e Entry) error {
    entries, err := LoadLocalOverlay(catalogDir)
    if err != nil {
        return err
    }

    // Idempotency: skip if an entry with the same ecosystem+package already exists.
    for _, existing := range entries {
        if strings.EqualFold(existing.Ecosystem, e.Ecosystem) &&
            strings.EqualFold(existing.Package, e.Package) {
            return nil // already present
        }
    }

    // Cap guard (Pitfall 6 from RESEARCH — v1 acceptable; log warning when exceeded).
    const maxOverlayEntries = 1000
    if len(entries) >= maxOverlayEntries {
        log.Printf("beekeeper: local overlay: entry cap %d reached, skipping %s/%s", maxOverlayEntries, e.Ecosystem, e.Package)
        return nil
    }

    entries = append(entries, e)

    // Write JSON atomically.
    jsonPath := filepath.Join(catalogDir, "local-overlay.json")
    data, err := json.Marshal(entries)
    if err != nil {
        return fmt.Errorf("marshal local overlay: %w", err)
    }
    if err := writeFileAtomic(jsonPath, data); err != nil { // reuse unexported writeFileAtomic in index.go (same package)
        return fmt.Errorf("write local overlay json: %w", err)
    }
    // Enforce owner-only on JSON (analog: store.go lines 61–67).
    if err := platform.SetOwnerOnly(jsonPath); err != nil {
        return fmt.Errorf("enforce owner-only on local overlay json: %w", err)
    }

    // Rebuild the mmap binary index (reuses BuildIndex from index.go — same package).
    idxPath := filepath.Join(catalogDir, "local-overlay.idx")
    if err := BuildIndex(idxPath, entries); err != nil {
        return fmt.Errorf("rebuild local overlay index: %w", err)
    }
    // Enforce owner-only on index (BuildIndex uses writeFileAtomic → file is new; set perms).
    if err := platform.SetOwnerOnly(idxPath); err != nil {
        return fmt.Errorf("enforce owner-only on local overlay index: %w", err)
    }

    return nil
}
```

**Key constraints:**
- `writeFileAtomic` and `BuildIndex` are both in `package catalog` (same package as `local_overlay.go`) — no export needed.
- `CatalogSignature` MUST be `""` on all overlay entries (Pitfall 3 from RESEARCH). `CatalogSource` MUST be `"local-overlay"`.
- Both `local-overlay.json` and `local-overlay.idx` are named so `SyncConditional` (which writes only `bumblebee.json` + `bumblebee.idx`) never touches them.

---

### `internal/catalog/multi.go` (MODIFIED — aggregator, request-response)

**Analog:** itself (lines 1–157)

**Existing MultiIndex struct** (lines 15–25):
```go
type MultiIndex struct {
    Bumblebee *Index
    OSV       policy.MultiCatalogLookup
    Socket    policy.MultiCatalogLookup
}
```

**Phase 24 extension — add `Overlay *Index` field:**
```go
type MultiIndex struct {
    Bumblebee *Index
    Overlay   *Index          // NEW: local-overlay.idx; nil when absent (first run)
    OSV       policy.MultiCatalogLookup
    Socket    policy.MultiCatalogLookup
}
```

**Existing `NewMultiIndex`** (lines 30–36) — keep as thin wrapper:
```go
func NewMultiIndex(bumblebee *Index, osv, socket policy.MultiCatalogLookup) *MultiIndex {
    return &MultiIndex{Bumblebee: bumblebee, OSV: osv, Socket: socket}
}
// Add non-breaking overload (pattern from Phase 23 NewMultiSinkWithCorpus):
func NewMultiIndexWithOverlay(bumblebee *Index, osv, socket policy.MultiCatalogLookup, overlayPath string) *MultiIndex {
    m := NewMultiIndex(bumblebee, osv, socket)
    if overlayPath != "" {
        if idx, err := OpenIndex(overlayPath); err == nil {
            m.Overlay = idx
        }
        // err → Overlay stays nil; silently degraded (no overlay match = no block)
    }
    return m
}
```

**Existing `LookupAll` loop** (lines 52–96) — insert overlay query after Bumblebee, before OSV:
```go
func (m *MultiIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
    var matches []policy.CatalogMatch

    if m.Bumblebee != nil {
        adapter := &bumblebeeMultiAdapter{idx: m.Bumblebee}
        got := adapter.LookupAll(ecosystem, pkg)
        if len(got) > 0 {
            matches = append(matches, got...)
        } else {
            matches = append(matches, policy.CatalogMatch{CatalogSource: "bumblebee", Dissented: true})
        }
    }

    // NEW Phase 24 — overlay query using the same bumblebeeMultiAdapter:
    if m.Overlay != nil {
        adapter := &bumblebeeMultiAdapter{idx: m.Overlay}
        got := adapter.LookupAll(ecosystem, pkg)
        if len(got) > 0 {
            // Override CatalogSource to "local-overlay" so corroborate() counts it separately.
            for i := range got {
                got[i].CatalogSource = "local-overlay"
            }
            matches = append(matches, got...)
        }
        // No dissent sentinel for overlay — it is optional, not a configured source.
    }

    // ... existing OSV + Socket blocks unchanged ...
}
```

**Existing `Close`** (lines 100–105) — extend to also close Overlay:
```go
func (m *MultiIndex) Close() error {
    if m.Overlay != nil {
        _ = m.Overlay.Close()
    }
    if m.Bumblebee != nil {
        return m.Bumblebee.Close()
    }
    return nil
}
```

**Existing `bumblebeeMultiAdapter`** (lines 110–157) — reused as-is for overlay lookup. The adapter is generic enough: `idx.Lookup(ecosystem, pkg)` works on any `*Index`, including `local-overlay.idx`.

---

### `internal/watch/firstresponder.go` (MODIFIED — orchestrator, event-driven)

**Analog:** itself (lines 1–264)

**Existing `FirstResponderConfig`** (lines 22–45):
```go
type FirstResponderConfig struct {
    Enabled           bool
    DryRun            bool
    Threshold         int
    QuarantineDir     string
    AuditPath         string
    IndexPath         string
    CacheDir          string
    SocketToken       string
    SentryTargetsPath string
    CrossRefFn        func(ctx context.Context, cfg CrossRefConfig) ([]ScanHit, error)
}
```
**Phase 24 extension — add corpus fields:**
```go
    // CorpusPath is the beekeeper-corpus.ndjson path. When non-empty and
    // CorpusEnabled is true, RunFirstResponder reads confirmed-malicious
    // adjudications from this path via corpus.ReadMaliciousRecords.
    CorpusPath            string
    CorpusEnabled         bool
    // CorpusSentryThreshold is the minimum PushEnvelope.SourceCount to elevate
    // a Sentry watch entry. Default 2 (enforce tier).
    CorpusSentryThreshold int
```

**Existing F-4 Sentry gate + targets pattern** (lines 99–124) — the new corpus loop copies this shape exactly:
```go
// Existing scan-hit path (copy shape for corpus loop):
if targets != nil && hit.CorroborationCount >= threshold {
    expectedProcess := ecosystemToProcess(hit.Ecosystem)
    targets.AddTarget(hit.Package, hit.InstalledPath, expectedProcess)
}
```

**Existing quarantine + audit pattern** (lines 143–167) — the corpus path copies this:
```go
// Existing: MoveTyped + writeFirstResponderAudit("catalog_quarantine")
_, moveErr := quarantine.MoveTyped(cfg.QuarantineDir, hit.InstalledPath, m)
if moveErr != nil {
    log.Printf("beekeeper first-responder: quarantine move failed for %s/%s: %v (artifact left in place)", hit.Ecosystem, hit.Package, moveErr)
    writeFirstResponderAudit(cfg.AuditPath, "quarantine_error", hit)
    continue
}
writeFirstResponderAudit(cfg.AuditPath, "catalog_quarantine", hit)
```

**Existing pending-quarantine path** (line 138–140):
```go
// Path unknown: emit pending-quarantine (same record type reused for corpus path).
writeFirstResponderAudit(cfg.AuditPath, "pending-quarantine", hit)
```

**Existing SaveTargets pattern** (lines 171–175):
```go
if targets != nil && cfg.SentryTargetsPath != "" {
    if saveErr := sentry.SaveTargets(cfg.SentryTargetsPath, targets); saveErr != nil {
        log.Printf("beekeeper first-responder: save sentry targets failed: %v", saveErr)
    }
}
```

**New helper to add** — `parsePackageID` (NOT hand-rolled; modeled on adjudicator.go lines 351–362):
```go
// parsePackageID splits "ecosystem:package" or "package" into (ecosystem, pkg).
// Handles scoped npm names correctly ("npm:@org/pkg" → "npm", "@org/pkg").
func parsePackageID(id string) (ecosystem, pkg string) {
    for i, c := range id {
        if c == ':' {
            return id[:i], id[i+1:]
        }
    }
    return "", id // no colon — treat whole string as package name
}
```

**Import additions for modified file:**
```go
// Add to existing import block:
"github.com/bantuson/beekeeper/internal/corpus"
// (internal/quarantine, internal/sentry, internal/audit already imported)
```

**Critical constraint — MUST NOT call `quarantine.Purge`** from any corpus code path. Phase 24 only calls `quarantine.MoveTyped` (reversible). `quarantine.Purge` remains exclusively TUI-keyboard-gated. The CI grep gate (FRB-02) enforces this.

---

### `cmd/beekeeper/catalogs_daemon.go` (MODIFIED — command wiring, batch)

**Analog:** itself (lines 78–123) — the existing Phase 23 adjudication batch pass

**Existing non-fatal stderr-log pattern** (lines 83–122) — ALL new calls in `runCatalogsSync` must follow the same shape:
```go
if cfg.Corpus.Enabled {
    corpusPath, cpErr := corpus.ResolveCorpusPath(cfg.Corpus, stateDir)
    if cpErr != nil {
        fmt.Fprintf(os.Stderr, "beekeeper: corpus: resolve corpus path for adjudication: %v\n", cpErr)
    } else {
        // ... existing RunAdjudicationBatch call (UNCHANGED) ...
        if batchErr := corpus.RunAdjudicationBatch(batchCtx, corpusPath, stateFile, idx, thresholds, cleanDays); batchErr != nil {
            fmt.Fprintf(os.Stderr, "beekeeper: corpus adjudication batch: %v\n", batchErr)
            // non-fatal: sync continues
        }

        // Phase 24 additions — same non-fatal pattern:

        // FRB-01/04: first-responder corpus wiring.
        if frErr := firstResponderFn(cmd.Context(), watch.FirstResponderConfig{
            CorpusPath:            corpusPath,
            CorpusEnabled:         cfg.Corpus.Enabled,
            CorpusSentryThreshold: 2,
            SentryTargetsPath:     filepath.Join(stateDir, "sentry-targets.json"),
            QuarantineDir:         filepath.Join(stateDir, "quarantine"),
            AuditPath:             filepath.Join(auditDir, "beekeeper.ndjson"),
            IndexPath:             filepath.Join(dir, "bumblebee.idx"),
            CacheDir:              dir,
            Enabled:               cfg.AutoQuarantine.Enabled,
            Threshold:             2,
        }); frErr != nil {
            fmt.Fprintf(os.Stderr, "beekeeper: corpus first-responder: %v\n", frErr)
            // non-fatal: sync continues
        }

        // FRB-05: local catalog overlay — one entry per malicious corpus record.
        malicious, rdErr := corpus.ReadMaliciousRecords(corpusPath)
        if rdErr != nil {
            fmt.Fprintf(os.Stderr, "beekeeper: corpus: read malicious records for overlay: %v\n", rdErr)
        } else {
            for _, rec := range malicious {
                if rec.PushEnvelope == nil || rec.PushEnvelope.Signature.PackageOrExtensionID == "" {
                    continue
                }
                // build overlayEntry inline (Entry is from internal/catalog)
                if ovErr := catalog.AddLocalOverlayEntry(dir, buildOverlayEntry(rec)); ovErr != nil {
                    fmt.Fprintf(os.Stderr, "beekeeper: catalog overlay: add entry: %v\n", ovErr)
                    // non-fatal: continue to next record
                }
            }
        }
    }
}
```

**Existing import block** (lines 3–17) — additions needed:
```go
// Add to existing imports:
"github.com/bantuson/beekeeper/internal/watch"
// internal/catalog and internal/corpus already imported
```

---

## Shared Patterns

### Owner-Only File Creation (applies to all new files in `local_overlay.go`)
**Source:** `internal/corpus/store.go` lines 51–75 (`NewStoreSink`)
```go
// Pattern: create with 0o600, then enforce via platform.SetOwnerOnly.
// os.OpenFile alone is insufficient on Windows — SetOwnerOnly applies DACL.
if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { ... }
f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
// ... then:
if err := platform.SetOwnerOnly(path); err != nil { ... }
```
**Apply to:** Every file created by `AddLocalOverlayEntry` (`local-overlay.json`, `local-overlay.idx`).

### Atomic Write (applies to `local_overlay.go`)
**Source:** `internal/catalog/index.go` lines 118–139 (`writeFileAtomic`)
```go
func writeFileAtomic(path string, data []byte) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
    // ... write to tmp, sync, close, then os.Rename(tmpName, path)
}
```
**Apply to:** `local-overlay.json` write in `AddLocalOverlayEntry`. `BuildIndex` already uses `writeFileAtomic` internally for `local-overlay.idx`.

### Non-Fatal Error Logging in `runCatalogsSync`
**Source:** `cmd/beekeeper/catalogs_daemon.go` lines 118–122
```go
if batchErr := corpus.RunAdjudicationBatch(...); batchErr != nil {
    fmt.Fprintf(os.Stderr, "beekeeper: corpus adjudication batch: %v\n", batchErr)
    // non-fatal: continue
}
```
**Apply to:** All three Phase 24 call sites in `runCatalogsSync` (`firstResponderFn`, `ReadMaliciousRecords`, `AddLocalOverlayEntry` loop). Errors MUST NOT return from `runCatalogsSync`.

### Malformed-Line Skip in NDJSON Scanner
**Source:** `internal/corpus/adjudicator.go` lines 277–280
```go
if err := json.Unmarshal(line, &rec); err != nil {
    continue // skip malformed lines — do not abort the batch
}
```
**Apply to:** `ReadMaliciousRecords` in `internal/corpus/reader.go`.

### Missing-File → nil (not error)
**Source:** `internal/corpus/adjudicator.go` lines 255–260
```go
if os.IsNotExist(err) {
    return nil // no corpus file yet — nothing to adjudicate
}
```
**Apply to:** `ReadMaliciousRecords` (returns `nil, nil`) and `LoadLocalOverlay` (returns `nil, nil`).

---

## No Analog Found

All five files have strong analogs. No files require falling back to RESEARCH.md patterns alone.

---

## Import Boundary Rules (extracted from RESEARCH.md + CLAUDE.md)

| Rule | Enforced By |
|------|-------------|
| `internal/corpus/reader.go` MUST NOT import `internal/tui` | ADJ-01 / Pitfall 1 |
| `internal/corpus/reader.go` MUST NOT import `internal/check` | ADJ-01 |
| `internal/policy` MUST remain pure (no I/O) — Phase 24 does not touch it | CLAUDE.md |
| `handler.go` MUST NOT import `internal/corpus/reader.go` | Pitfall 5 |
| Overlay entries MUST have `CatalogSignature = ""` | Pitfall 3 |
| `quarantine.Purge` MUST NOT be called from any corpus path | FRB-02 / Pitfall 4 |

---

## Metadata

**Analog search scope:** `internal/corpus/`, `internal/catalog/`, `internal/watch/`, `cmd/beekeeper/`
**Files scanned:** 7 source files read in full
**Pattern extraction date:** 2026-06-14
