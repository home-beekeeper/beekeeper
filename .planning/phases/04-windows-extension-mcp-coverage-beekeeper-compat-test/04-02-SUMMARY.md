---
phase: 04-windows-extension-mcp-coverage-beekeeper-compat-test
plan: 02
subsystem: scan
tags: [bkint-01, ptest-04, subprocess-seam, pollen-compat, windows-coverage]
dependency_graph:
  requires: []
  provides: [runPollenFn-seam, TestPollenCompatibility, pollen_unavailable-status]
  affects: [internal/scan/scanner.go, internal/scan/scanner_test.go]
tech_stack:
  added: []
  patterns: [injectable-var-rename, fixture-driven-compat-test, no-binary-spawn]
key_files:
  modified:
    - internal/scan/scanner.go
    - internal/scan/scanner_test.go
decisions:
  - "BKINT-01 implemented via in-place rename (not new internal/inventory/ package) — injectable-var seam already provides replaceability + testability with zero new infrastructure"
  - "runBumblebeeFn → runPollenFn + lookBumblebee → lookPollenFn + defaultRunBumblebee → defaultRunPollen; exec.LookPath target changed from 'bumblebee' to 'pollen'"
  - "pollen_unavailable scan_status key replaces bumblebee_unavailable; source field in scan_error updated to 'pollen'"
  - "TestPollenCompatibility uses Config{} (no ExtensionDirs) to prevent beekeeperScan from running, avoiding double-counting from beekeeper-own records (Pitfall 6)"
  - "Zero t.Skip enforced by fixture-driven design: no binary spawn, no OS filesystem access, runs identically on ubuntu/macos/windows"
metrics:
  duration: "~10 min"
  completed: "2026-06-02"
  tasks: 2
  files: 2
---

# Phase 04 Plan 02: Beekeeper Compat Test + Pollen Subprocess Seam Summary

Bumblebee→pollen subprocess seam rename (BKINT-01) in `internal/scan/scanner.go` plus the PTEST-04 compatibility test covering all five Pollen record types with zero `t.Skip` on any OS.

## What Was Built

### Task 1: Rename bumblebee→pollen subprocess seam (BKINT-01) — commit `45ea499`

In `internal/scan/scanner.go`, renamed the three injectable identifiers and updated all status strings:

- `lookBumblebee` → `lookPollenFn` with `exec.LookPath("pollen")` (was `"bumblebee"`)
- `runBumblebeeFn` → `runPollenFn` (calls `defaultRunPollen`)
- `defaultRunBumblebee` → `defaultRunPollen` (body identical; calls `lookPollenFn()`)
- `Scan()` call site: `runBumblebeeFn(ctx, cfg.Deep)` → `runPollenFn(ctx, cfg.Deep)`
- scan_error `"source":"bumblebee"` → `"source":"pollen"`, error text updated
- scan_status key `"bumblebee_unavailable":true` → `"pollen_unavailable":true`
- `fmt.Errorf` text updated: `"write bumblebee_unavailable status"` → `"write pollen_unavailable status"`
- Package and function doc comments updated: "Bumblebee CLI" → "Pollen CLI"

Out-of-scope literals left untouched (confirmed by grep):
- `bumblebee.idx`, `CatalogSource:"bumblebee"` in `internal/tui`, `internal/gateway`, `internal/watch`, `internal/catalog` — these are threat-intel catalog source names, not the subprocess
- `scanOnDeltaFn` in `cmd/beekeeper/main.go` — separate outer seam, not part of BKINT-01

`go build ./...` exits 0 after Task 1 (test files excluded from build; scanner_test.go updated in Task 2).

### Task 2: Update scan tests + add TestPollenCompatibility (PTEST-04) — commit `4f6312f`

In `internal/scan/scanner_test.go`:

**Mechanical renames (3 existing tests):**
- `TestScanWithBumblebee`: `runBumblebeeFn` → `runPollenFn`
- `TestScanWindowsShapedRecord`: `runBumblebeeFn` → `runPollenFn`
- `TestScanBumblebeeUnavailable` → `TestScanPollenUnavailable`: `runBumblebeeFn` → `runPollenFn`, assertion `"bumblebee_unavailable":true` → `"pollen_unavailable":true`

**New tests added:**

`TestScanPollenUnavailable`: injects `runPollenFn = func(...) { return nil, false }`, verifies `"pollen_unavailable":true` in output and that beekeeperScan still runs (finding record emitted).

`TestPollenCompatibility` (PTEST-04): fixture-driven, no binary spawn, no OS filesystem access — runs identically on ubuntu/macos/windows with zero `t.Skip`. Injects 5 Pollen record fixtures with Windows-shaped backslash+drive-letter paths:
1. npm package record
2. editor-extension record (WEXT-01 path: `.vscode\extensions\...`)
3. browser-extension record (WEXT-02 path: `AppData\Local\Google\Chrome\...`)
4. mcp-config record (WEXT-03 path: `AppData\Roaming\Claude\...`)
5. scan_summary record

Assertions (PTEST-04):
- No `"record_type":"scan_error"` (all five accepted as valid NDJSON)
- `strings.Count(out, "\"scanner_name\":\"pollen\"") >= 4` (attribution preserved on all non-summary records)
- Each non-summary record's `source_file` appears exactly once (no double-counting)

## Verification Results

```
go build ./...    → exit 0 (after Task 1 and Task 2)
go vet ./internal/scan/  → exit 0 (after Task 2)
go test ./internal/scan/ -v:
  === RUN   TestScanWithBumblebee     --- PASS (0.00s)
  === RUN   TestScanWindowsShapedRecord --- PASS (0.00s)
  === RUN   TestScanPollenUnavailable  --- PASS (0.08s)
  === RUN   TestPollenCompatibility   --- PASS (0.00s)
  PASS  ok  github.com/home-beekeeper/beekeeper/internal/scan  5.541s
go test ./internal/scan/ -v | grep SKIP  → (empty — zero skips)
go test ./...     → all 22 packages green
```

## Deviations from Plan

None — plan executed exactly as written.

Both tasks followed the plan's specified approach: mechanical identifier rename in Task 1, fixture-driven test additions in Task 2. No Rule 1/2/3 auto-fixes were needed.

## Known Stubs

None. All functionality is fully wired.

## Threat Flags

No new security surface introduced. The subprocess target name changed from `bumblebee` to `pollen` — this is the pre-existing T-04-03 threat (user PATH trust domain, identical risk profile). The T-04-04 fail-closed guard (malformed NDJSON → `scan_error`) is preserved and covered by existing behavior.

## Self-Check: PASSED

- `internal/scan/scanner.go` — FOUND (renamed seam, `runPollenFn`, `lookPollenFn`, `defaultRunPollen`)
- `internal/scan/scanner_test.go` — FOUND (`TestPollenCompatibility`, `TestScanPollenUnavailable`)
- Commit `45ea499` — FOUND (`feat(04-02): rename bumblebee→pollen subprocess seam`)
- Commit `4f6312f` — FOUND (`feat(04-02): update scan tests to runPollenFn + add TestPollenCompatibility`)
- `grep bumblebee internal/scan/scanner.go` — empty (no bumblebee literals remain)
- `grep runBumblebeeFn internal/scan/scanner_test.go` — empty (no old references)
- `go test ./internal/scan/ -v | grep SKIP` — empty (zero skips confirmed)
- Full suite `go test ./...` — all 22 packages green
