---
phase: 24-first-responder-corpus-binding
plan: "01"
subsystem: corpus-reader + catalog-overlay + multi-index
tags: [frb, corpus, catalog, overlay, mmap, first-responder]
dependency_graph:
  requires:
    - 23-03-SUMMARY.md  # corpus store + adjudication engine (Phase 23)
  provides:
    - ReadMaliciousRecords (corpus signal source for FRB-01/04/05)
    - AddLocalOverlayEntry / LoadLocalOverlay (FRB-05 storage)
    - NewMultiIndexWithOverlay (FRB-05 MultiIndex extension)
  affects:
    - internal/catalog/multi.go (additive Overlay field + LookupAll + Close)
    - internal/corpus/reader.go (new file)
    - internal/catalog/local_overlay.go (new file)
tech_stack:
  added: []
  patterns:
    - NDJSON scan + latest-per-cluster collapse (mirrors RunAdjudicationBatch)
    - atomic write via writeFileAtomic + platform.SetOwnerOnly (owner-only files)
    - additive MultiIndex extension (mirrors Phase-23 NewMultiSinkWithCorpus)
key_files:
  created:
    - internal/corpus/reader.go
    - internal/corpus/reader_test.go
    - internal/catalog/local_overlay.go
    - internal/catalog/local_overlay_test.go
  modified:
    - internal/catalog/multi.go
decisions:
  - "ReadMaliciousRecords reuses clusterKeyOf + maxRecordsToScan from adjudicator.go (same package, no export)"
  - "Overlay uses bumblebeeMultiAdapter with CatalogSource override to local-overlay (reuses existing adapter)"
  - "No dissent sentinel for Overlay — it is optional, not a configured required source"
  - "RED stubs committed first (Task 1) before production code (Tasks 2+3) per TDD protocol"
metrics:
  duration: "11 minutes"
  completed: "2026-06-14"
  tasks_completed: 3
  tasks_total: 3
  files_created: 4
  files_modified: 1
---

# Phase 24 Plan 01: Corpus Reader + Local Catalog Overlay Summary

**One-liner:** ReadMaliciousRecords NDJSON reader with latest-per-cluster collapse + owner-only local-overlay.json/idx two-file store that survives catalogs sync, wired into MultiIndex via additive NewMultiIndexWithOverlay.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Wave-0 RED test skeletons | 8c0a8b6 | corpus/reader_test.go, catalog/local_overlay_test.go, reader.go stub, local_overlay.go stub, multi.go (Overlay field + stub constructor) |
| 2 | ReadMaliciousRecords implementation (FRB-01) | 8be5570 | internal/corpus/reader.go |
| 3 | Local overlay + MultiIndex extension (FRB-05) | 2988ada | internal/catalog/local_overlay.go, internal/catalog/multi.go |

## What Was Built

### `internal/corpus/reader.go` — ReadMaliciousRecords (FRB-01 signal source)

Reads confirmed-malicious adjudication records from the corpus NDJSON file.
Mirrors the `RunAdjudicationBatch` scan loop exactly:

- Opens the file; missing file → `(nil, nil)` (no corpus yet = not an error)
- `bufio.Scanner` with 1MB line buffer, capped at `maxRecordsToScan` (50 000)
- Malformed JSON lines → `continue` (silent skip, never aborts)
- `latest-per-cluster collapse`: uses the existing unexported `clusterKeyOf` helper; last NDJSON line wins
- Filters collapsed map to `TrueLabel == "malicious"` only
- Explicit `f.Close()` before returning (Windows cannot rename open-for-read files)
- Imports: stdlib only (`bufio`, `encoding/json`, `fmt`, `os`); no `internal/tui`, no `internal/check`

### `internal/catalog/local_overlay.go` — Two-file owner-only overlay (FRB-05)

`LoadLocalOverlay(catalogDir string) ([]Entry, error)`:
- Reads `<catalogDir>/local-overlay.json`; missing → `(nil, nil)`; malformed JSON → error

`AddLocalOverlayEntry(catalogDir string, e Entry) error`:
- Idempotent on `EqualFold(ecosystem + package)` — duplicate add returns nil
- Capped at `maxOverlayEntries = 1000`; at cap logs warning and returns nil
- Marshals entries as JSON, writes via `writeFileAtomic` (same-package, unexported)
- Enforces `platform.SetOwnerOnly` on both `local-overlay.json` and `local-overlay.idx`
- Rebuilds binary index via `BuildIndex` (same-package, unexported)
- Survives `catalogs sync` because `SyncConditional` writes ONLY `bumblebee.*` (verified)

### `internal/catalog/multi.go` — Additive MultiIndex extension (FRB-05)

- `MultiIndex.Overlay *Index` field added between `Bumblebee` and `OSV`
- `NewMultiIndexWithOverlay`: calls `NewMultiIndex` then opens overlay via `OpenIndex`; error → `Overlay stays nil` (silently degraded)
- `LookupAll`: queries `Overlay` after `Bumblebee` using `bumblebeeMultiAdapter`; sets `CatalogSource = "local-overlay"` on each match; no dissent sentinel (overlay is optional)
- `Close`: closes `m.Overlay` (if non-nil) before `m.Bumblebee`
- `NewMultiIndex` signature **unchanged** — zero caller breakage (additive extension)

## Test Coverage

All 8 named test functions pass GREEN:

**corpus/reader_test.go:**
- `TestReadMaliciousRecords` (5 sub-tests: filters, latest-wins, missing-nil, malformed-skip, redaction-safety)
- `TestReadMaliciousRecordsLatestPerCluster`

**catalog/local_overlay_test.go:**
- `TestLocalOverlaySurvivesSync` — overlay files byte-unchanged after simulated bumblebee.* write
- `TestLocalOverlayFilePermissions` — SKIPPED on Windows (SetOwnerOnly applies DACL); would be 0600 on Unix
- `TestMultiIndexQueriesOverlay` — LookupAll returns >= 1 match with CatalogSource=="local-overlay"
- `TestLocalOverlayUnsignedIsWarnTier` — CatalogSignature=="" + Signed=false on overlay match
- `TestLocalOverlayPlusBumblebeeIsEnforce` — both "bumblebee" and "local-overlay" sources returned
- `TestLocalOverlayIdempotentAdd` — two identical adds = exactly 1 entry

## Verification Results

```
go test ./internal/corpus/... ./internal/catalog/... -count=1
ok  github.com/bantuson/beekeeper/internal/corpus   1.791s
ok  github.com/bantuson/beekeeper/internal/catalog  3.672s

go build ./...  exit 0
go mod tidy && git diff --exit-code go.mod  → PASS: zero new deps
grep -rn 'internal/tui|internal/check' reader.go local_overlay.go → 0 actual imports
NewMultiIndex callers (catalogs_daemon.go line 105): unchanged
```

## Deviations from Plan

None — plan executed exactly as written. The RED stub pattern required a thin `reader.go` stub before the full implementation, matching the TDD protocol in the plan. The `TestLocalOverlayFilePermissions` SKIP on Windows is expected behavior (documented in the test body with structured reason per plan instructions).

## Threat Surface Scan

No new network endpoints, auth paths, or external trust boundaries introduced. All new files live under `internal/` and write to `CatalogDir` (already within `StateDir` scope guarded by `EvaluateSelfPath`). No threat flags.

## Known Stubs

None — all functions are fully implemented. `ReadMaliciousRecords`, `LoadLocalOverlay`, `AddLocalOverlayEntry`, and `NewMultiIndexWithOverlay` are complete production implementations.

## Self-Check: PASSED

Files exist:
- `internal/corpus/reader.go` ✓
- `internal/corpus/reader_test.go` ✓
- `internal/catalog/local_overlay.go` ✓
- `internal/catalog/local_overlay_test.go` ✓

Commits exist:
- `8c0a8b6` (test: RED skeletons) ✓
- `8be5570` (feat: ReadMaliciousRecords) ✓
- `2988ada` (feat: local overlay + MultiIndex) ✓
