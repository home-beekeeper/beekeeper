---
phase: 24-first-responder-corpus-binding
plan: "02"
subsystem: watch-corpus-binding
tags: [frb, corpus, first-responder, quarantine, sentry, tdd]
dependency_graph:
  requires:
    - 24-01-SUMMARY.md  # ReadMaliciousRecords + local overlay (FRB-01 signal source)
  provides:
    - RunFirstResponder corpus loop (FRB-01/02/04 enforcement)
    - parsePackageID helper (ecosystem:pkg splitting)
    - writeCorpusFirstResponderAudit helper (corpus-path audit discipline)
    - TestCorpusPathHasNoPurgeCall (FRB-02 static gate)
  affects:
    - internal/watch/firstresponder.go (corpus fields + loop + helpers)
    - internal/watch/firstresponder_corpus_test.go (five corpus test cases)
    - internal/watch/nopurge_test.go (FRB-02 static gate)
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN: corpus test RED skeletons before implementation
    - O(1) hitMap for corpus-to-scan-hit path-resolution (ecosystem+pkg keyed)
    - audit.RedactRecord discipline on corpus audit helper (T-24-AUDIT-REDACT)
    - Static source-read gate test (grep-hygiene via runtime.Caller + os.ReadFile)
key_files:
  created:
    - internal/watch/firstresponder_corpus_test.go
    - internal/watch/nopurge_test.go
  modified:
    - internal/watch/firstresponder.go
decisions:
  - "Enabled=false on scan-hit path in corpus tests to isolate the corpus path as the only writer"
  - "CrossRefFn returns nil in TestFirstResponderCorpusSingleSourceNoSentry to prevent scan-hit sentry interference"
  - "parsePackageID iterates rune-by-rune to find first colon — handles scoped npm names (@org/pkg) intact"
  - "writeCorpusFirstResponderAudit added as sibling helper (no ScanHit required) for pending-quarantine path"
  - "FRB-02 static gate uses runtime.Caller(0) to resolve repo root portably in tests"
metrics:
  duration: "18 minutes"
  completed: "2026-06-14"
  tasks_completed: 3
  tasks_total: 3
  files_created: 2
  files_modified: 1
---

# Phase 24 Plan 02: First Responder Corpus Binding Summary

**One-liner:** RunFirstResponder extended with a corpus-adjudication loop that arms the TUI quarantine card (FRB-01), gates Sentry watch on SourceCount >= 2 (FRB-04), and is statically and behaviorally proven to never call quarantine.Purge (FRB-02).

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Wave-0 RED corpus test cases | c0e8663 | internal/watch/firstresponder_corpus_test.go |
| 2 | Extend RunFirstResponder with corpus loop (FRB-01/02/04) | 99b8932 | internal/watch/firstresponder.go, firstresponder_corpus_test.go |
| 3 | FRB-02 static no-purge gate + full verification | f2d3e44 | internal/watch/nopurge_test.go |

## What Was Built

### `internal/watch/firstresponder.go` — Corpus fields + loop + helpers

**New FirstResponderConfig fields:**

- `CorpusPath string` — path to `beekeeper-corpus.ndjson`; when non-empty and `CorpusEnabled=true`, the corpus loop runs
- `CorpusEnabled bool` — gates the corpus processing path
- `CorpusSentryThreshold int` — minimum `PushEnvelope.SourceCount` to elevate a Sentry watch target; defaults to 2 (enforce tier) when <= 0

**`parsePackageID(id string) (ecosystem, pkg string)`** — iterates the id rune-by-rune to find the first ':' and splits there. Handles scoped npm names correctly: `"npm:@nrwl/nx-console"` → `("npm", "@nrwl/nx-console")`. A colon-less id returns `("", id)`.

**`writeCorpusFirstResponderAudit(auditPath, recordType, ecosystem, pkg, version string)`** — sibling helper to `writeFirstResponderAudit` that constructs a FRSP-02 audit record from raw corpus fields (no `ScanHit` required, enabling the pending-quarantine path). Applies `audit.RedactRecord` before write (T-24-AUDIT-REDACT discipline).

**Corpus loop in `RunFirstResponder`** (runs after the scan-hit loop, before `SaveTargets`):

- Guarded by `cfg.CorpusEnabled && cfg.CorpusPath != ""`
- Calls `corpus.ReadMaliciousRecords(cfg.CorpusPath)`; read error → log + skip corpus block (scan-hit results unaffected — fail-closed primary path)
- Builds `hitMap` from the already-computed `hits` slice (O(1) ecosystem+pkg lookup)
- For each malicious record:
  - Skips records with nil `PushEnvelope` or empty `PackageOrExtensionID`
  - Parses ecosystem+pkg via `parsePackageID`
  - Resolves `InstalledPath` by looking up the `hitMap`
  - **FRB-04**: `targets.AddTarget(pkg, installedPath, ecosystemToProcess(ecosystem))` only when `SourceCount >= corpusThreshold` (default 2)
  - **FRB-01 arm**: if path resolved → `quarantine.MoveTyped` + `"catalog_quarantine"` audit; else → `"pending-quarantine"` audit (no move)
  - **FRB-02**: `quarantine.Purge` is never called anywhere in the corpus path

### `internal/watch/firstresponder_corpus_test.go` — Five corpus test cases

| Test | Behavior Verified |
|------|-------------------|
| `TestFirstResponderCorpusMaliciousArmsCard` | enforce-tier corpus record + matching local install → `catalog_quarantine` audit + artifact in quarantine dir |
| `TestFirstResponderCorpusSentryGate` | SourceCount=2 >= threshold=2 → target in sentry-targets.json |
| `TestFirstResponderCorpusSingleSourceNoSentry` | SourceCount=1 < threshold=2 → NO target added |
| `TestFirstResponderCorpusNoPurge` | Artifact moved into quarantine (reversible); quarantine.List returns entry (not purged) |
| `TestFirstResponderCorpusPendingQuarantine` | No local install → `pending-quarantine` audit; quarantine dir empty |

**Key test design decisions:**
- `Enabled: false` on scan-hit path in card/nopurge tests so only the corpus path quarantines the artifact (prevents double-move)
- `CrossRefFn` returns `nil` in the single-source test to prevent the scan-hit path from adding a Sentry target and masking the corpus-gate negative assertion

### `internal/watch/nopurge_test.go` — FRB-02 static gate

`TestCorpusPathHasNoPurgeCall` reads `internal/watch/firstresponder.go` and `internal/corpus/reader.go` at test time via `os.ReadFile`. It strips comment lines (lines whose trimmed prefix is `//`) then asserts neither `"quarantine.Purge("` nor `".Purge("` appear on any non-comment source line. Uses `runtime.Caller(0)` to resolve the repo root portably without hardcoded paths.

## Verification Results

```
go test ./internal/watch/... -run TestFirstResponderCorpus -count=1
ok  github.com/bantuson/beekeeper/internal/watch   PASS (5 tests)

go test ./internal/watch/... -run TestCorpusPathHasNoPurgeCall -count=1
ok  github.com/bantuson/beekeeper/internal/watch   PASS

go test ./internal/watch/... ./internal/corpus/... ./internal/catalog/... -count=1
ok  github.com/bantuson/beekeeper/internal/watch    PASS
ok  github.com/bantuson/beekeeper/internal/corpus   PASS
ok  github.com/bantuson/beekeeper/internal/catalog  PASS

go build ./...   OK
go vet ./internal/watch/...  OK
go mod tidy && git diff --exit-code go.mod  NO CHANGE (zero new deps)
grep -n 'Purge(' internal/watch/firstresponder.go  0 matches
grep -n 'internal/tui' internal/watch/firstresponder.go  0 matches
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test isolation: scan-hit path needed to be disabled in corpus tests**

- **Found during:** Task 2 (GREEN iteration)
- **Issue:** `TestFirstResponderCorpusMaliciousArmsCard`, `TestFirstResponderCorpusNoPurge` had `Enabled: true` in the config, causing the scan-hit loop to quarantine the artifact FIRST. When the corpus loop then tried `MoveTyped` on the already-moved path, it got an OS error (file not found). The artifact was already in quarantine so the `catalog_quarantine` assertion passed, but via the scan-hit path rather than the corpus path.
- **Fix:** Set `Enabled: false` (scan-hit auto-quarantine disabled) in those two tests to isolate the corpus path as the sole mover. The scan-hit results are still returned by `CrossRefFn` (needed for install-path resolution in the hitMap lookup), but the scan-hit quarantine action is skipped.
- **Files modified:** `internal/watch/firstresponder_corpus_test.go`

**2. [Rule 1 - Bug] Test isolation: CrossRefFn must return nil in single-source test**

- **Found during:** Task 2 (GREEN iteration)
- **Issue:** `TestFirstResponderCorpusSingleSourceNoSentry` had `CrossRefFn` returning `nxConsoleScanHits("")` — a hit with `CorroborationCount: 2`. The scan-hit path's F-4 gate added a Sentry target for it (CorroborationCount >= threshold), so `sentry-targets.json` already contained `@nrwl/nx-console` before the corpus path ran. The single-source corpus negative assertion failed.
- **Fix:** Changed `CrossRefFn` to return `nil, nil` in that test. The test is specifically for the corpus FRB-04 gate; it should not depend on scan-hit behavior.
- **Files modified:** `internal/watch/firstresponder_corpus_test.go`

## Threat Surface Scan

No new network endpoints, auth paths, or external trust boundaries introduced. The corpus loop is purely file-read + file-write within `StateDir`. All audit writes are routed through `audit.RedactRecord` before reaching disk. No threat flags beyond those already modeled in the plan's STRIDE register.

## Known Stubs

None — all functions are fully implemented. `parsePackageID`, `writeCorpusFirstResponderAudit`, and the corpus loop in `RunFirstResponder` are complete production implementations.

## Self-Check: PASSED

Files exist:
- `internal/watch/firstresponder.go` ✓ (CorpusPath/CorpusEnabled/CorpusSentryThreshold fields + corpus loop + helpers)
- `internal/watch/firstresponder_corpus_test.go` ✓ (5 corpus test functions)
- `internal/watch/nopurge_test.go` ✓ (TestCorpusPathHasNoPurgeCall)

Commits exist:
- `c0e8663` (test: RED corpus test cases) ✓
- `99b8932` (feat: corpus loop + helpers) ✓
- `f2d3e44` (test: FRB-02 static gate) ✓
