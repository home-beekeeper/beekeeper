---
phase: 24-first-responder-corpus-binding
verified: 2026-06-14T18:30:00Z
status: passed
score: 10/10 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Launch the TUI dashboard after seeding a confirmed-malicious corpus record for npm:@nrwl/nx-console and triggering beekeeper catalogs sync --force, then open the quarantine / alerts panel"
    expected: "The corpus-armed quarantine card uses ONLY the existing locked palette: coral (#f0883e) for Beekeeper-response badges (HELD / quarantine), red (#f85149) only for attacker-action rows. No new colors are introduced. The [r]estore and [p]urge (p->y gated) actions render on the corpus-adjudication card."
    why_human: "Bubble Tea terminal rendering; color semantic is visual and cannot be verified programmatically. 24-03 plan Task 4 is a blocking human-verify checkpoint for FRB-03."
    result: "APPROVED by maintainer 2026-06-14 (responded 'Approved' to the FRB-03 checkpoint during /gsd-execute-phase 24, Task 4). Human gate satisfied — status closed to passed."
---

# Phase 24: First Responder Corpus Binding — Verification Report

**Phase Goal:** A confirmed-malicious adjudication arms the TUI quarantine card and elevates detection (Sentry watch + local catalog overlay) without ever auto-purging.
**Verified:** 2026-06-14T18:30:00Z
**Status:** passed (FRB-03 visual gate APPROVED by maintainer during execution)
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | ReadMaliciousRecords returns only TrueLabel=="malicious" records, latest-per-cluster | VERIFIED | `TestReadMaliciousRecords` (5 sub-tests) + `TestReadMaliciousRecordsLatestPerCluster` GREEN; `internal/corpus/reader.go` L83 filters `TrueLabel == "malicious"`, L77 uses `clusterKeyOf` last-write-wins |
| 2 | ReadMaliciousRecords returns nil (not error) for a missing corpus file and skips malformed NDJSON lines | VERIFIED | `TestReadMaliciousRecords/missing_file_returns_nil_nil` + `TestReadMaliciousRecords/malformed_line_skipped_valid_returned` GREEN; `reader.go` L38-41 `os.IsNotExist` → nil, L57-60 `json.Unmarshal` error → `continue` |
| 3 | A confirmed-malicious corpus record matched to a local install writes a catalog_quarantine audit record (arms the TUI card) | VERIFIED | `TestFirstResponderCorpusMaliciousArmsCard` GREEN; `firstresponder.go` L267 writes `"catalog_quarantine"` via `writeCorpusFirstResponderAudit`; `TestRunCatalogsSyncFirstResponder` assertion [2] also confirms end-to-end |
| 4 | NO automatic quarantine.Purge from the corpus path — behavioral test + static gate both pass | VERIFIED | `TestFirstResponderCorpusNoPurge` GREEN (quarantine entry survives); `TestCorpusPathHasNoPurgeCall` GREEN (static grep of `firstresponder.go` + `reader.go` finds zero non-comment `.Purge(` lines); `grep -n "Purge("` of all three corpus-path files returns 0 matches |
| 5 | Restore reverses a corpus-adjudication quarantine entry (TestRestoreCorpusQuarantineEntry) | VERIFIED | `TestRestoreCorpusQuarantineEntry` GREEN; uses `quarantine.MoveTyped` with corpus manifest (RuleIDs=["FRSP-02"], Reason="corpus adjudication: confirmed malicious"), then `quarantine.Restore` succeeds and artifact returns to `OriginalPath`; no production change to `quarantine.go` required |
| 6 | Sentry watch added ONLY when SourceCount >= CorpusSentryThreshold (default 2); single-source (SourceCount=1) does NOT add a target | VERIFIED | `TestFirstResponderCorpusSentryGate` GREEN (SourceCount=2 → target added); `TestFirstResponderCorpusSingleSourceNoSentry` GREEN (SourceCount=1 → no target); `firstresponder.go` L237: `rec.PushEnvelope.SourceCount >= corpusThreshold` gate |
| 7 | A local-only owner-only catalog overlay (local-overlay.json + local-overlay.idx) is added and survives catalogs sync | VERIFIED | `TestLocalOverlaySurvivesSync` GREEN; `local_overlay.go` L99 `writeFileAtomic` + L103 `platform.SetOwnerOnly`; `SyncConditional` writes only `bumblebee.*` (verified by comment at L9 of `local_overlay.go`); `TestLocalOverlayFilePermissions` SKIP on Windows (expected; SetOwnerOnly applies DACL) |
| 8 | MultiIndex.LookupAll queries the overlay index and returns CatalogSource=="local-overlay" | VERIFIED | `TestMultiIndexQueriesOverlay` GREEN; `multi.go` L101-109 queries Overlay via `bumblebeeMultiAdapter`, sets `got[i].CatalogSource = "local-overlay"`; `TestLocalOverlayPlusBumblebeeIsEnforce` GREEN (two distinct sources returned) |
| 9 | runCatalogsSync, after RunAdjudicationBatch, runs the first-responder corpus pass and adds a local overlay entry per malicious record — all non-fatal | VERIFIED | `catalogs_daemon.go` L131-175: `firstResponderFn` call (L140) + `corpus.ReadMaliciousRecords` (L160) + `catalog.AddLocalOverlayEntry` (L169) all wrapped in non-fatal stderr-log pattern; `TestRunCatalogsSyncFirstResponder` all 7 assertions GREEN |
| 10 | The synthetic Nx Console round-trip arms the card, adds the Sentry target, adds the local overlay, never purges, and is restorable — end to end | VERIFIED | `TestRunCatalogsSyncFirstResponder` GREEN with all 7 FRB evaluator-gate assertions passing (see Behavioral Spot-Checks section) |

**Score:** 10/10 truths verified

---

### Deferred Items

None. All 5 FRB requirements are addressed in this phase.

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/corpus/reader.go` | `ReadMaliciousRecords` NDJSON reader | VERIFIED | Exists, 88 lines, fully implemented; stdlib-only imports (`bufio`, `encoding/json`, `fmt`, `os`) |
| `internal/corpus/reader_test.go` | `TestReadMaliciousRecords` + `TestReadMaliciousRecordsLatestPerCluster` | VERIFIED | Exists with both test functions; 7 sub-tests all GREEN |
| `internal/catalog/local_overlay.go` | `AddLocalOverlayEntry` + `LoadLocalOverlay` owner-only overlay | VERIFIED | Exists, 119 lines; `AddLocalOverlayEntry` at L69, `LoadLocalOverlay` at L42 |
| `internal/catalog/local_overlay_test.go` | 6 overlay tests | VERIFIED | Exists with all 6 named tests; 5 PASS, 1 SKIP (Windows POSIX mode, expected) |
| `internal/catalog/multi.go` | `MultiIndex.Overlay` field + `NewMultiIndexWithOverlay` + LookupAll overlay query | VERIFIED | `Overlay *Index` at L22; `NewMultiIndexWithOverlay` at L50; overlay queried at L101-109 |
| `internal/watch/firstresponder.go` | `CorpusPath/CorpusEnabled/CorpusSentryThreshold` fields + corpus loop | VERIFIED | Fields at L53-61; corpus block at L197-274; `parsePackageID` at L330; `writeCorpusFirstResponderAudit` at L345 |
| `internal/watch/firstresponder_corpus_test.go` | 5 `TestFirstResponderCorpus*` tests | VERIFIED | Exists as a separate file (not inline in `firstresponder_test.go`); all 5 tests PASS |
| `internal/watch/nopurge_test.go` | `TestCorpusPathHasNoPurgeCall` static FRB-02 gate | VERIFIED | Exists; PASS; checks both `firstresponder.go` and `reader.go` for `.Purge(` on non-comment lines |
| `cmd/beekeeper/catalogs_daemon.go` | `firstResponderFn` seam + `buildOverlayEntry` + FRB wiring block | VERIFIED | `firstResponderFn` var at L29; `buildOverlayEntry` at L242; FRB wiring at L131-175 |
| `cmd/beekeeper/catalogs_daemon_test.go` | `TestRunCatalogsSyncFirstResponder` 7-assertion evaluator gate | VERIFIED | Exists; all 7 assertions PASS (FRB-01..05 end to end) |
| `internal/quarantine/quarantine_test.go` | `TestRestoreCorpusQuarantineEntry` FRB-03 reversibility | VERIFIED | Test added to existing file; PASS; no production change to `quarantine.go` |
| `internal/config/layered.go` | `mergeCorpus` + `mergeAutoQuarantine` (Rule-1 fix) | VERIFIED | `mergeCorpus` at L873; `mergeAutoQuarantine` at L890; wired into both `merge()` at L259 and `mergeUntrusted()` at L355 |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/corpus/reader.go` | same-package `clusterKeyOf` | reuses unexported helper | WIRED | `reader.go` L76: `clusterKeyOf(rec)` — same package, no import needed |
| `internal/catalog/local_overlay.go` | `platform.SetOwnerOnly` | enforces owner-only on overlay files | WIRED | `local_overlay.go` L103, L114: both `platform.SetOwnerOnly` calls present |
| `internal/catalog/multi.go` | `local-overlay.idx` via `OpenIndex` | `NewMultiIndexWithOverlay` opens overlay | WIRED | `multi.go` L53: `OpenIndex(overlayPath)` |
| `internal/watch/firstresponder.go` | `corpus.ReadMaliciousRecords` | reads malicious records when `CorpusEnabled` | WIRED | `firstresponder.go` L198: `corpus.ReadMaliciousRecords(cfg.CorpusPath)` |
| `internal/watch/firstresponder.go` | `sentry.TargetList.AddTarget` | `SourceCount >= CorpusSentryThreshold` gate (FRB-04) | WIRED | `firstresponder.go` L237-239: `targets.AddTarget(pkg, installedPath, ...)` behind `rec.PushEnvelope.SourceCount >= corpusThreshold` |
| `internal/watch/firstresponder.go` | `quarantine.MoveTyped` | reversible quarantine only — never Purge (FRB-02) | WIRED | `firstresponder.go` L260: `quarantine.MoveTyped(...)` — zero `Purge` calls confirmed by `TestCorpusPathHasNoPurgeCall` |
| `cmd/beekeeper/catalogs_daemon.go` | `firstResponderFn` | injectable seam calling `watch.RunFirstResponder` | WIRED | `catalogs_daemon.go` L29-31: `var firstResponderFn = func(...) { return watch.RunFirstResponder(...) }` |
| `cmd/beekeeper/catalogs_daemon.go` | `catalog.AddLocalOverlayEntry` | one overlay entry per malicious corpus record (FRB-05) | WIRED | `catalogs_daemon.go` L169: `catalog.AddLocalOverlayEntry(dir, buildOverlayEntry(rec))` |
| `cmd/beekeeper/catalogs_daemon.go` | `corpus.ReadMaliciousRecords` | drives the overlay loop | WIRED | `catalogs_daemon.go` L160: `corpus.ReadMaliciousRecords(corpusPath)` |
| `internal/config/layered.go` | `mergeCorpus` in `merge()` + `mergeUntrusted()` | Rule-1 fix: `cfg.Corpus.Enabled` propagates correctly | WIRED | `layered.go` L259 (`merge`) + L355 (`mergeUntrusted`) both call their respective merge helpers |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `firstresponder.go` corpus path | `malicious []CorpusRecord` | `corpus.ReadMaliciousRecords(cfg.CorpusPath)` | Yes — reads NDJSON from disk | FLOWING |
| `catalogs_daemon.go` FRB overlay pass | `malicious []CorpusRecord` | second `corpus.ReadMaliciousRecords(corpusPath)` call | Yes — reads same NDJSON | FLOWING |
| `catalogs_daemon.go` FRB first-responder pass | `firstResponderFn(...)` return | calls real `watch.RunFirstResponder` in production | Yes — drives quarantine + sentry via real implementations | FLOWING |
| `catalog/multi.go` LookupAll | `got []policy.CatalogMatch` from Overlay | `bumblebeeMultiAdapter{idx: m.Overlay}.LookupAll(...)` | Yes — reads mmap index built by `BuildIndex` over real `local-overlay.json` entries | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| FRB-01 signal: `ReadMaliciousRecords` returns only malicious records, latest-per-cluster | `go test ./internal/corpus/... -run TestReadMaliciousRecords -count=1` | PASS (5 sub-tests) | PASS |
| FRB-05 overlay survives sync | `go test ./internal/catalog/... -run TestLocalOverlaySurvivesSync -count=1` | PASS | PASS |
| FRB-05 MultiIndex overlay query | `go test ./internal/catalog/... -run TestMultiIndexQueriesOverlay -count=1` | PASS | PASS |
| FRB-01 arm TUI card (catalog_quarantine) | `go test ./internal/watch/... -run TestFirstResponderCorpusMaliciousArmsCard -count=1` | PASS | PASS |
| FRB-04 Sentry gate (SourceCount>=2) | `go test ./internal/watch/... -run TestFirstResponderCorpusSentryGate -count=1` | PASS | PASS |
| FRB-04 single-source no Sentry | `go test ./internal/watch/... -run TestFirstResponderCorpusSingleSourceNoSentry -count=1` | PASS | PASS |
| FRB-02 no Purge behavioral | `go test ./internal/watch/... -run TestFirstResponderCorpusNoPurge -count=1` | PASS | PASS |
| FRB-01 pending-quarantine (no local install) | `go test ./internal/watch/... -run TestFirstResponderCorpusPendingQuarantine -count=1` | PASS | PASS |
| FRB-02 no Purge static gate | `go test ./internal/watch/... -run TestCorpusPathHasNoPurgeCall -count=1` | PASS (both sub-tests) | PASS |
| FRB-03 restore reversibility | `go test ./internal/quarantine/... -run TestRestoreCorpusQuarantineEntry -count=1` | PASS | PASS |
| FRB-01..05 synthetic Nx Console evaluator gate (7 assertions) | `go test ./cmd/beekeeper/... -run TestRunCatalogsSyncFirstResponder -count=1` | PASS (all 7 assertions) | PASS |
| go build ./... | `go build ./...` | exit 0 | PASS |
| Zero new deps | `go mod tidy` diff | go.mod unchanged | PASS |

---

### Probe Execution

No declared probe scripts for this phase. The evaluator gate (`TestRunCatalogsSyncFirstResponder`) serves as the integration probe.

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| FRB-01 | 24-01, 24-02, 24-03 | Confirmed-malicious adjudication arms TUI quarantine card via existing crossref / quarantine seam (no new IPC) | SATISFIED | `writeCorpusFirstResponderAudit` writes `catalog_quarantine`; `quarantine.MoveTyped` arms the card; `TestFirstResponderCorpusMaliciousArmsCard` + evaluator gate assertion [2] + [4] PASS |
| FRB-02 | 24-02, 24-03 | Purge stays human-confirmed — never automatic, never fleet-pushable | SATISFIED | `TestCorpusPathHasNoPurgeCall` (static: zero `.Purge(` calls in corpus path files) + `TestFirstResponderCorpusNoPurge` (behavioral: entry survives) + evaluator gate assertion [5] PASS |
| FRB-03 | 24-03 | Restore available for reversibility; locked TUI red/coral semantic preserved | SATISFIED (automated) + HUMAN NEEDED (visual) | `TestRestoreCorpusQuarantineEntry` GREEN; no production change to `quarantine.go` needed; TUI semantic confirmed by maintainer during execution (24-03-SUMMARY.md Task 4 APPROVED) — see Human Verification section |
| FRB-04 | 24-02, 24-03 | Confirmed-malicious adjudication elevates DETECTION-ONLY Sentry watch gated at corroboration >= threshold | SATISFIED | `TestFirstResponderCorpusSentryGate` (SourceCount=2 → target added) + `TestFirstResponderCorpusSingleSourceNoSentry` (SourceCount=1 → not added) + evaluator gate assertion [3] PASS |
| FRB-05 | 24-01, 24-03 | Adds local-only catalog overlay entry; owner-only; survives `catalogs sync`; future installs caught without catalog lag | SATISFIED | `TestLocalOverlaySurvivesSync` + `TestMultiIndexQueriesOverlay` + `TestLocalOverlayUnsignedIsWarnTier` + `TestLocalOverlayPlusBumblebeeIsEnforce` GREEN; evaluator gate assertion [7] (CatalogSource=="local-overlay") PASS |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | Scanned all Phase 24 modified files for TBD/FIXME/XXX/TODO/placeholder/stub patterns; zero matches found |

Additional static checks:
- `grep -rn 'internal/tui' internal/corpus/reader.go internal/catalog/local_overlay.go internal/watch/firstresponder.go cmd/beekeeper/catalogs_daemon.go` — 0 actual import matches (only a comment in `local_overlay.go` saying "NO internal/tui")
- `grep -n 'Purge(' cmd/beekeeper/catalogs_daemon.go internal/watch/firstresponder.go internal/corpus/reader.go` — 0 matches
- `grep -n 'CatalogSignature' cmd/beekeeper/catalogs_daemon.go` — confirms `buildOverlayEntry` sets `CatalogSignature: ""` (unsigned, warn-only)

---

### Rule-1 Deviation Note (Config-Merge Fix)

The 24-03-SUMMARY.md documents a Rule-1 bug found during Task 3 green iteration: `internal/config/layered.go` `merge()` and `mergeUntrusted()` were missing `Corpus` and `AutoQuarantine` struct handling, causing `cfg.Corpus.Enabled` to always be `false` (Go zero value) after `resolveConfig()`. This would have permanently skipped the entire FRB adjudication pass in production.

**Fix:** `mergeCorpus()` (Enabled-wins semantics for Go zero-bool) and `mergeAutoQuarantine()` added and wired into both merge paths.
**Verified:** Both helpers present in `layered.go` at L873 and L890; wired at L259 (`merge`) and L355 (`mergeUntrusted`); `TestRunCatalogsSyncFirstResponder` would have failed without this fix.

This is a genuine production defect found and fixed within the phase (not deferred). It is tested directly by the evaluator gate.

---

### Human Verification Required

#### 1. TUI red/coral color semantic on corpus-armed quarantine card (FRB-03)

**Test:** Build the binary (`go build -o beekeeper ./cmd/beekeeper`). Seed a confirmed-malicious corpus record for npm:@nrwl/nx-console v17.3.0. Run `beekeeper catalogs sync --force` (or replay the `TestRunCatalogsSyncFirstResponder` temp dirs). Launch the TUI dashboard and open the quarantine / alerts panel.

**Expected:** The corpus-armed quarantine card uses ONLY the existing locked palette: coral (#f0883e) for Beekeeper-response badges (HELD / quarantine state), red (#f85149) only for attacker-action rows. No new colors are introduced. The [r]estore and [p]→y-gated [p]urge actions render correctly on corpus-adjudication cards.

**Why human:** Bubble Tea terminal rendering; color semantic is visual and cannot be verified programmatically. The corpus path reuses `catalog_quarantine`/`pending-quarantine` audit record types (no new type introduced), so the existing TUI rendering should apply unchanged — but this must be confirmed visually.

**Note:** 24-03-SUMMARY.md Task 4 records: "Maintainer confirmed: TUI red/coral semantic preserved; restore + p→y-gated purge actions render." This checkpoint was completed during execution on 2026-06-14 and APPROVED. This human_needed status reflects that the phase cannot automatically prove visual correctness — the status will resolve to `passed` once the maintainer re-confirms or acknowledges the previously-logged approval in the formal gate.

---

### Gaps Summary

No automated gaps found. All 10 must-haves are VERIFIED with passing tests and correct implementation.

The `status: human_needed` reflects the FRB-03 TUI visual checkpoint (Task 4 in 24-03-PLAN.md), which is a blocking human-verify gate by design. The 24-03-SUMMARY.md records maintainer APPROVAL during execution, but the formal gate must acknowledge it.

The `internal/check` package showed a timeout failure in one parallel full-suite run (`126.353s` vs `98.272s` in isolation) — this is a pre-existing Windows timing flakiness not caused by Phase 24 (zero files in `internal/check` were modified; confirmed with `git diff 23bdc40 HEAD -- internal/check/`).

---

_Verified: 2026-06-14T18:30:00Z_
_Verifier: Claude (gsd-verifier)_
