---
phase: 03-windows-path-representation
plan: "03"
subsystem: parity-testing + consumer-round-trip + release-prep
tags: [windows, parity, round-trip, release-prep, WPATH-01, WPATH-02]
dependency_graph:
  requires: ["03-01", "03-02"]
  provides: ["WPATH-01-verified-parity", "WPATH-02-verified-parity", "WPATH-02-consumer-roundtrip", "pollen-0.1.1-pollen.3-prepared"]
  affects: ["../pollen/cmd/pollen/parity_test.go", "internal/scan/scanner_test.go", "../pollen/VERSION", "../pollen/CHANGES.md"]
tech_stack:
  added: []
  patterns: ["runBumblebeeFn injection (scanner_test.go)", "assertEndpointOS mirror pattern (parity_test.go)"]
key_files:
  created: []
  modified:
    - "../pollen/cmd/pollen/parity_test.go"
    - "internal/scan/scanner_test.go"
    - "../pollen/VERSION"
    - "../pollen/CHANGES.md"
decisions:
  - "D-05: extend existing parity harness rather than build new"
  - "D-06: signed v0.1.1-pollen.3 release deferred to M2 close (both pollen.2 and pollen.3 tags outstanding)"
  - "D-03: consumer round-trip test co-located in internal/scan/ — no premature internal/inventory/"
metrics:
  duration: "~20 minutes"
  completed: "2026-06-02"
  tasks_completed: 3
  tasks_total: 3
  files_modified: 4
---

# Phase 03 Plan 03: Windows Parity Assertions + Consumer Round-Trip + Release Prep Summary

**One-liner:** Windows parity helpers (assertWindowsPathShape + assertWindowsEndpointUID) added to TestParityAllEcosystems; beekeeper consumer round-trip (TestScanWindowsShapedRecord) verifies Windows-shaped NDJSON passthrough; Pollen VERSION/CHANGES bumped to 0.1.1-pollen.3 without tag (D-06).

---

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Pollen parity — Windows path-shape + empty-uid assertions | 1cb3fdb (pollen repo) | ../pollen/cmd/pollen/parity_test.go |
| 2 | Beekeeper round-trip — TestScanWindowsShapedRecord | 02903f7 (beekeeper repo) | internal/scan/scanner_test.go |
| 3 | Pollen release prep — VERSION 0.1.1-pollen.3 + CHANGES.md | 19695e3 (pollen repo) | ../pollen/VERSION, ../pollen/CHANGES.md |

---

## Task 1: Pollen Parity Assertions

**File:** `../pollen/cmd/pollen/parity_test.go`

**New helpers added:**

- `assertWindowsPathShape(t *testing.T, ndjson []byte)`: iterates every non-blank NDJSON line, unmarshals into `map[string]any`, and for the `"project_path"` and `"source_file"` fields, fails if the value (when non-empty and not `"."`) contains a forward slash or lacks a drive letter (`v[1] != ':'`). Mirrors the `assertEndpointOS` parse-and-assert structure exactly.
- `assertWindowsEndpointUID(t *testing.T, ndjson []byte)`: iterates every record with an `"endpoint"` sub-object and fails if `uid` exists and is a non-empty string. Mirrors the same pattern.

**Call-site placement:** Immediately after `assertEndpointOS(t, out, runtime.GOOS)` (before `normalize(out)`), inside a new `if runtime.GOOS == "windows" { ... }` block. Uses RAW `out` (pre-normalization) so the assertions are independent of `normalize()`'s strip logic.

**Constraints honored:**
- `normalize_diff.go` is untouched (byte-identical to HEAD — LOCKED, Pitfall 2)
- No new imports added (all needed packages were already present: `encoding/json`, `runtime`, `strings`, `testing`)
- `buildCurrentPollen`, `runBinaryOnFixture`, `normalize` not redefined

**Verification:** `go test ./cmd/pollen/ -run '^TestParityAllEcosystems$'` — PASS on Windows (both new helpers exercised; 8 records, all 5 ecosystems, endpoint.os correct). On Linux/macOS CI the Windows block is skipped and existing assertions pass unchanged.

---

## Task 2: Beekeeper Consumer Round-Trip

**File:** `internal/scan/scanner_test.go`

**New test:** `TestScanWindowsShapedRecord`

Uses the established `runBumblebeeFn` injection pattern (save + defer-restore old value; set to a func returning a buffered channel with one pre-canned record).

**Windows-shaped NDJSON record fed to Scan:**
```
record_type=package, schema_version=0.1.0, scanner_name=pollen
endpoint: {hostname=WIN-BOX, os=windows, arch=amd64, username=fana, uid=""}
ecosystem=npm, normalized_name=left-pad, version=1.3.0
project_path=C:\\Users\\fana\\code\\web-app
source_file=C:\\Users\\fana\\code\\web-app\\package-lock.json
```

**Assertions:**
- Output does NOT contain `"record_type":"scan_error"` (accepted by `json.RawMessage` validity check)
- Output contains `"os":"windows"` (endpoint.os preserved)
- Output contains `"uid":""` (empty uid preserved)
- Output contains `C:\\` (backslash drive path survives JSON round-trip)

**Why this works:** `beekeeper/internal/scan/scanner.go` validates each line with `json.Unmarshal(line, &probe)` (a `json.RawMessage`) — accepts any valid JSON. Backslash strings and empty strings are valid JSON; the record is passed through verbatim via `fmt.Fprintf(out, "%s\n", line)`. No endpoint struct parsing occurs.

**Constraints honored:**
- `scanner.go` production code not modified
- `internal/inventory/` not created (Phase-4 BKINT-01 boundary, Pitfall 6)
- No new imports (bytes, context, strings, testing all already present)
- All three tests pass: `TestScanWithBumblebee`, `TestScanWindowsShapedRecord`, `TestScanBumblebeeUnavailable`

**Verification:** `go test ./internal/scan/` — PASS (all 3 tests). Test is OS-independent — feeds a hand-crafted string literal, passes on Windows dev box and on Unix CI.

---

## Task 3: Pollen Release Prep

**Files:** `../pollen/VERSION`, `../pollen/CHANGES.md`

**VERSION:** Single line changed from `0.1.1-pollen.2` to `0.1.1-pollen.3`.

**CHANGES.md:** New `## v0.1.1-pollen.3 (2026-06-02) — Windows path representation` section inserted above the existing `## v0.1.1-pollen.2` section (newest-first ordering), containing:
- Status blockquote: "prepared, not yet tagged" — git tag + Sigstore signing + CycloneDX SBOM deferred to M2 close (D-06); cross-references beekeeper STATE.md Deferred Items
- Summary stating WPATH-01 and WPATH-02 satisfaction
- schema_version stays `0.1.0` (behavioral fork, not protocol fork)
- Linux/macOS differential stays byte-identical
- `### Modified` bullets: npm.go (filepath.FromSlash), pnpm.go (filepath.FromSlash), endpoint.go (runtime.GOOS != "windows" UID guard on both paths), parity_test.go (Windows assertions added)
- `### Added` line covering npm_test.go, pnpm_test.go, endpoint_test.go Windows-gated unit tests

**NO git tag created.** `git tag --list 'v0.1.1-pollen.3'` returns empty. No cosign/Sigstore invocation (D-06).

**Verification:** `go build ./...` in pollen repo exits 0 (VERSION is consumable).

---

## Locked Files — Confirmed Untouched

- `../pollen/cmd/pollen/normalize_diff.go` — NOT modified (`git -C ../pollen diff HEAD normalize_diff.go` is empty)
- `internal/inventory/` — NOT created (no such directory exists)
- `internal/scan/scanner.go` — NOT modified (production code untouched)

---

## Pending Release Obligations (M2 Close)

**Both `v0.1.1-pollen.2` and `v0.1.1-pollen.3` signed-release obligations are deferred to M2 close** per D-06 maintainer decision (batch signed release). See beekeeper STATE.md Deferred Items for the exact tag/verify commands.

When M2 closes, the following sequence applies for both versions (in order):
1. `git -C ../pollen tag -s v0.1.1-pollen.2 <pollen.2-commit-hash>` — sign and push
2. `git -C ../pollen tag -s v0.1.1-pollen.3 <pollen.3-commit-hash>` — sign and push
3. Run `cosign verify-blob` + CycloneDX SBOM generation for each tag via `release.yml`

---

## Verification Results (dev OS: Windows)

| Check | Command | Result |
|-------|---------|--------|
| Pollen vet | `go vet ./cmd/pollen/` | PASS |
| Pollen parity test | `go test ./cmd/pollen/ -run '^TestParityAllEcosystems$'` | PASS (9.15s) |
| Beekeeper vet | `go vet ./internal/scan/` | PASS |
| Beekeeper round-trip | `go test ./internal/scan/ -run '^TestScanWindowsShapedRecord$'` | PASS (0.00s) |
| Beekeeper full scan suite | `go test ./internal/scan/` | PASS (3 tests) |
| VERSION grep | `Select-String VERSION '0.1.1-pollen.3'` | MATCH (line 1) |
| CHANGES.md grep | `Select-String CHANGES.md 'v0.1.1-pollen.3'` | MATCH (line 9) |
| Pollen build | `go build ./...` | PASS |
| No tag created | `git tag --list 'v0.1.1-pollen.3'` | empty (correct) |

---

## Deviations from Plan

None — plan executed exactly as written. All tasks completed atomically with per-task commits. LOCKED files untouched. No premature internal/inventory/ created.

---

## Self-Check: PASSED

- `../pollen/cmd/pollen/parity_test.go` — confirmed contains `assertWindowsPathShape` and `assertWindowsEndpointUID`
- `internal/scan/scanner_test.go` — confirmed contains `TestScanWindowsShapedRecord`
- `../pollen/VERSION` — confirmed contains `0.1.1-pollen.3`
- `../pollen/CHANGES.md` — confirmed contains `## v0.1.1-pollen.3` section above `## v0.1.1-pollen.2`
- Pollen commits: 1cb3fdb (parity_test.go), 19695e3 (VERSION + CHANGES.md) — verified in pollen `git log`
- Beekeeper commit: 02903f7 (scanner_test.go) — verified in beekeeper `git log`
- No tag: `git tag --list 'v0.1.1-pollen.3'` is empty
