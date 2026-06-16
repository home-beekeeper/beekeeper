---
phase: 24-first-responder-corpus-binding
plan: "03"
subsystem: catalogs-sync-frb-wiring
tags: [frb, corpus, first-responder, catalog-overlay, quarantine, sentry, evaluator-gate, config-merge-fix]
dependency_graph:
  requires:
    - 24-01-SUMMARY.md  # ReadMaliciousRecords + AddLocalOverlayEntry + corpus store
    - 24-02-SUMMARY.md  # RunFirstResponder corpus loop + FRB-02 static gate
  provides:
    - runCatalogsSync FRB wiring (firstResponderFn + ReadMaliciousRecords + AddLocalOverlayEntry)
    - buildOverlayEntry helper (unsigned CatalogSignature="", CatalogSource="local-overlay")
    - TestRunCatalogsSyncFirstResponder (7-assertion synthetic Nx Console evaluator gate)
    - TestRestoreCorpusQuarantineEntry (FRB-03 reversibility proof)
    - mergeCorpus + mergeAutoQuarantine (Rule-1 config-merge fix in layered.go)
  affects:
    - cmd/beekeeper/catalogs_daemon.go
    - cmd/beekeeper/catalogs_daemon_test.go
    - internal/quarantine/quarantine_test.go
    - internal/config/layered.go
tech_stack:
  added: []
  patterns:
    - Injectable seam (firstResponderFn var) mirrors scanOnDeltaFn pattern for testability
    - Non-fatal stderr-log pattern for all three new FRB calls (sync never returns an FRB error)
    - FRB pass ordered after RunAdjudicationBatch but before HTTP fetch (moat feedback survives offline)
    - buildOverlayEntry sets CatalogSignature="" (unsigned, warn-only tier) to prevent poisoning
    - mergeCorpus: Enabled true always wins; false cannot clobber lower-layer true (Go zero-bool ambiguity)
key_files:
  created:
    - cmd/beekeeper/catalogs_daemon_test.go   # TestRunCatalogsSyncFirstResponder evaluator gate
    - internal/quarantine/quarantine_test.go  # TestRestoreCorpusQuarantineEntry (new test added to existing file)
  modified:
    - cmd/beekeeper/catalogs_daemon.go        # FRB wiring block + firstResponderFn seam + buildOverlayEntry
    - internal/config/layered.go              # mergeCorpus + mergeAutoQuarantine (Rule-1 fix)
decisions:
  - "firstResponderFn injectable seam added to catalogs_daemon.go (var, mirrors scanOnDeltaFn) so cmd tests can stub it without touching RunFirstResponder"
  - "FRB pass ordered after RunAdjudicationBatch and before HTTP fetch: confirmed-malicious records exist before first-responder reads them; moat feedback runs even offline"
  - "buildOverlayEntry sets CatalogSignature='' (unsigned) so overlay entries contribute source_count:1 (warn-only); enforce requires a second independent signed source — prevents overlay poisoning (T-24-OVERLAY-UNSIGNED)"
  - "Rule-1 fix: mergeCorpus/mergeAutoQuarantine added because Go zero-bool means false cannot be distinguished from absent; Enabled=true always wins; false never clobbers a lower-layer true"
  - "Task 1 (restore reversibility) is test-only — corpus-adjudication quarantine entries are structurally identical to scan-hit entries, so quarantine.Restore required no production change"
  - "Task 4 (FRB-03 TUI checkpoint) confirmed: existing catalog_quarantine/pending-quarantine audit types already render with coral BadgeBlock/BadgeHeld; no new colors introduced"
metrics:
  duration: "approximately 35 minutes (Tasks 1-3 automated + Task 4 human checkpoint)"
  completed: "2026-06-14"
  tasks_completed: 4
  tasks_total: 4
  files_created: 1
  files_modified: 3
---

# Phase 24 Plan 03: Catalogs Sync FRB Wiring + Evaluator Gate Summary

**One-liner:** runCatalogsSync extended with the full FRB moat loop (firstResponderFn + corpus overlay), the 7-assertion synthetic Nx Console evaluator gate proven green, FRB-03 restore reversibility confirmed, and a Rule-1 config-merge fix (mergeCorpus + mergeAutoQuarantine) that prevented the corpus/first-responder pass from ever firing in production.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | FRB-03 restore reversibility for a corpus-quarantine entry | f09f957 | internal/quarantine/quarantine_test.go |
| 2 | Wire FRB corpus pass into runCatalogsSync (FRB-01/04/05) | eb70718 | cmd/beekeeper/catalogs_daemon.go |
| 3 | Synthetic Nx Console evaluator gate + fix Corpus config merge | 68ea5d1 | cmd/beekeeper/catalogs_daemon_test.go, internal/config/layered.go |
| 4 | Human-verify FRB-03 red/coral TUI semantic (checkpoint) | (no code) | Maintainer APPROVED |

## What Was Built

### `cmd/beekeeper/catalogs_daemon.go` — FRB wiring block + firstResponderFn seam

**`firstResponderFn` injectable seam** (package-level var):

```go
var firstResponderFn = func(ctx context.Context, cfg watch.FirstResponderConfig) error {
    return watch.RunFirstResponder(ctx, cfg)
}
```

Mirrors `scanOnDeltaFn`: production leaves it pointing at `watch.RunFirstResponder`; cmd tests replace it with a no-op closure or a fake to isolate FRB assertions.

**FRB wiring block in `runCatalogsSync`** (inside the existing `if cfg.Corpus.Enabled { ... }` block, after `RunAdjudicationBatch`):

1. **FRB-01/04 first-responder pass** — calls `firstResponderFn` with `watch.FirstResponderConfig` supplying `CorpusPath`, `CorpusEnabled`, `CorpusSentryThreshold: 2`, `SentryTargetsPath`, `QuarantineDir`, `AuditPath` (resolved via `platform.AuditDir()`), `IndexPath`, `CacheDir`, `Enabled` (from `cfg.AutoQuarantineEnabled()`), `Threshold: 2`. Error is non-fatal (logged to stderr, sync continues).

2. **FRB-05 overlay pass** — calls `corpus.ReadMaliciousRecords(corpusPath)`, then for each record with a non-empty `PackageOrExtensionID`, calls `catalog.AddLocalOverlayEntry(dir, buildOverlayEntry(rec))`. Errors are non-fatal per-record.

**`buildOverlayEntry(rec corpus.CorpusRecord) catalog.Entry`** — unexported helper:
- Parses ecosystem and package name by splitting `PushEnvelope.Signature.PackageOrExtensionID` on the first `:`
- Returns a `catalog.Entry` with `ID="local-overlay-"+rec.AuditRecord.ClusterID`, `Severity="critical"`, `CatalogSource="local-overlay"`, `CatalogSignature=""` (unsigned, warn-only — T-24-OVERLAY-UNSIGNED)
- No `quarantine.Purge` call; no `internal/tui` import

All three new calls run AFTER `RunAdjudicationBatch` and BEFORE the HTTP fetch, so the moat feedback loop runs even on an offline machine.

### `internal/quarantine/quarantine_test.go` — FRB-03 restore reversibility

**`TestRestoreCorpusQuarantineEntry`** — proves that a corpus-adjudication quarantine entry is physically identical to a scan-hit entry and is reversible via `quarantine.Restore` without any production change to `quarantine.go`:

- Creates a temp source artifact and temp quarantine dir
- Calls `quarantine.MoveTyped` with a corpus-style `Manifest` (Publisher "npm", Name "@nrwl/nx-console", Version "17.3.0", ArtifactType language-package, Reason "corpus adjudication: confirmed malicious", RuleIDs ["FRSP-02"])
- Asserts `quarantine.List` shows exactly one entry
- Calls `quarantine.Restore` for the entry ID
- Asserts the artifact is back at its `OriginalPath` and the quarantine entry is gone (FRB-03 reversibility)

No production code change to `internal/quarantine/quarantine.go` was required.

### `cmd/beekeeper/catalogs_daemon_test.go` — Synthetic Nx Console evaluator gate

**`TestRunCatalogsSyncFirstResponder`** — the phase evaluator gate; asserts all 7 FRB conditions via a BEEKEEPER_HOME-isolated temp dir (no network):

1. `corpus.ReadMaliciousRecords(corpusPath)` returns the seeded `@nrwl/nx-console` v17.3.0 enforce-tier record (FRB data is present)
2. The audit log contains a `catalog_quarantine` record (FRB-01: card armed)
3. `sentry.LoadTargets(stateDir/sentry-targets.json)` contains `@nrwl/nx-console` (FRB-04: SourceCount 2 >= threshold 2)
4. `quarantine.List(stateDir/quarantine)` contains exactly one entry for the package (FRB-01: artifact quarantined)
5. The quarantine entry survived (no auto-purge occurred — FRB-02)
6. `quarantine.Restore` succeeds and the artifact returns to its original path (FRB-03)
7. `NewMultiIndexWithOverlay(...).LookupAll("npm","@nrwl/nx-console")` returns >= 1 match with `CatalogSource=="local-overlay"` (FRB-05: overlay entry present)

Test uses the `firstResponderFn` seam: the test replaces it with a closure that calls `watch.RunFirstResponder` with a `CrossRefFn` that returns a synthetic `ScanHit` backed by a real temp file (so `MoveTyped` succeeds). The original seam is restored in a `defer`.

### `internal/config/layered.go` — Rule-1 config-merge fix

**`mergeCorpus(dst, src CorpusConfig) CorpusConfig`** — added to `merge()` and `mergeUntrusted()`:
- `Enabled=true` always wins (a higher-layer true cannot be clobbered by a lower-layer false — Go zero-bool cannot be distinguished from "absent")
- `Path`, `DownstreamCleanDays`, and other string/int fields override when non-zero

**`mergeAutoQuarantine(dst, src AutoQuarantineConfig) AutoQuarantineConfig`** — added to `merge()` and `mergeUntrusted()`:
- Same Enabled-wins semantics; `Threshold` and `DryRun` override when non-zero/true

Both helpers wired into both `merge()` (trusted layer) and `mergeUntrusted()` (env/flag layers).

### Task 4: TUI red/coral semantic (human-verified, FRB-03)

No code written. The maintainer confirmed that the corpus-armed quarantine card on the live TUI dashboard uses exclusively the existing locked palette: coral (#f0883e) for Beekeeper-response badges (HELD / quarantine), red (#f85149) only for attacker-action rows. No new colors were introduced. The [r]estore and [p]→y-gated [p]urge actions render correctly on corpus-adjudication cards.

## Verification Results

```
go test ./internal/quarantine/... -run TestRestoreCorpusQuarantineEntry -count=1
ok  github.com/home-beekeeper/beekeeper/internal/quarantine   PASS

go test ./cmd/beekeeper/... -run TestRunCatalogsSyncFirstResponder -count=1
ok  github.com/home-beekeeper/beekeeper/cmd/beekeeper   PASS (all 7 FRB assertions)

go test ./... -count=1
ok  github.com/home-beekeeper/beekeeper/...   PASS (27 packages)

go build ./...   OK
go vet ./...   OK
go mod tidy && git diff --exit-code go.mod   NO CHANGE (zero new deps)

grep -n 'Purge(\|internal/tui' cmd/beekeeper/catalogs_daemon.go   0 matches
grep -n 'CatalogSignature' cmd/beekeeper/catalogs_daemon.go
  => buildOverlayEntry sets CatalogSignature: "" (empty — unsigned)

Human checkpoint (Task 4): APPROVED by maintainer
  => TUI red/coral semantic preserved; restore + p->y-gated purge actions render
```

## Requirements Satisfied

| Requirement | Description | Status |
|-------------|-------------|--------|
| FRB-01 | Corpus-adjudication quarantine card armed in TUI via firstResponderFn | DONE |
| FRB-02 | No automatic purge in the command path (static gate + evaluator-gate assertion 5) | DONE |
| FRB-03 | Restore reverses a corpus-adjudication entry; TUI red/coral semantic human-verified | DONE |
| FRB-04 | Sentry watch target added when SourceCount >= 2 (evaluator-gate assertion 3) | DONE |
| FRB-05 | Local catalog overlay entry added per confirmed-malicious record; LookupAll returns match | DONE |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Config-merge omitted Corpus and AutoQuarantine fields**

- **Found during:** Task 3 (GREEN iteration on TestRunCatalogsSyncFirstResponder)
- **Issue:** `internal/config/layered.go` `merge()` and `mergeUntrusted()` did not include the `Corpus` or `AutoQuarantine` struct blocks. After `resolveConfig()` layered all config sources, `cfg.Corpus.Enabled` was always `false` (Go zero value) regardless of what was set in `config.json` or env vars. This meant the `if cfg.Corpus.Enabled { ... }` guard in `runCatalogsSync` was permanently skipped — the entire adjudication batch, first-responder pass, and local overlay pass would never have fired in production.
- **Fix:** Added `mergeCorpus()` helper (Enabled-wins semantics for Go zero-bool) and `mergeAutoQuarantine()` helper, wired both into `merge()` (trusted layer) and `mergeUntrusted()` (env/flag layers) in `internal/config/layered.go`.
- **Files modified:** `internal/config/layered.go`
- **Commit:** `68ea5d1` (combined with the evaluator-gate test)

### No Other Deviations

Tasks 1 and 2 executed exactly as planned. Task 4 was a blocking human-verify checkpoint with no code changes; maintainer approved.

## STRIDE Threat Coverage

| Threat ID | Status |
|-----------|--------|
| T-24-SYNC-FAILOPEN | Mitigated: all three new calls use non-fatal stderr-log pattern; sync never returns FRB error |
| T-24-NOPURGE-CMD | Mitigated: grep confirms 0 `Purge(` calls in catalogs_daemon.go; evaluator-gate assertion 5 confirms no auto-purge |
| T-24-OVERLAY-UNSIGNED | Mitigated: buildOverlayEntry sets CatalogSignature="" (warn-only tier, source_count:1) |
| T-24-CARD-COLOR | Mitigated: no new TUI colors; corpus path reuses existing catalog_quarantine/pending-quarantine types; human-verified |
| T-24-SC | Mitigated: go mod tidy shows no change; zero new dependencies |

## Known Stubs

None. All FRB wiring is fully implemented production code. The `buildOverlayEntry` helper, `firstResponderFn` seam, `mergeCorpus`, and `mergeAutoQuarantine` are complete. The evaluator gate passes against real implementations (not mocks) for `ReadMaliciousRecords`, `AddLocalOverlayEntry`, `quarantine.List`, `quarantine.Restore`, and `sentry.LoadTargets`.

## Self-Check: PASSED

Files exist:
- `cmd/beekeeper/catalogs_daemon.go` (firstResponderFn seam + FRB wiring block + buildOverlayEntry) ✓
- `cmd/beekeeper/catalogs_daemon_test.go` (TestRunCatalogsSyncFirstResponder, 7 assertions) ✓
- `internal/quarantine/quarantine_test.go` (TestRestoreCorpusQuarantineEntry added) ✓
- `internal/config/layered.go` (mergeCorpus + mergeAutoQuarantine in both merge paths) ✓

Commits exist:
- `f09f957` (test(24-03): TestRestoreCorpusQuarantineEntry FRB-03 reversibility) ✓
- `eb70718` (feat(24-03): wire FRB corpus pass into runCatalogsSync FRB-01/04/05) ✓
- `68ea5d1` (test(24-03): synthetic Nx Console evaluator gate + fix Corpus config merge) ✓
