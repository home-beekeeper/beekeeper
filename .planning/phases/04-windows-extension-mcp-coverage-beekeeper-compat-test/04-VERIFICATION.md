---
phase: 04-windows-extension-mcp-coverage-beekeeper-compat-test
verified: 2026-06-02T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification: false
---

# Phase 4: Windows Extension & MCP Coverage + Beekeeper Compat Test — Verification Report

**Phase Goal:** Pollen enumerates all Windows editor-extension directories (VS Code family), browser-extension profile paths (Chromium + Firefox), and MCP host-config files (Claude Desktop, Cursor, Windsurf, Cline, Gemini CLI); and beekeeper's Pollen compatibility test runs on all three OSes with a Windows skip count of zero.
**Verified:** 2026-06-02
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (derived from ROADMAP SC1–SC5)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | On Windows, `resolveRoots(baseline)` returns all five editor-extension roots (VS Code, Insiders, Cursor, Windsurf, VSCodium via both `.vscodium` and `.vscode-oss`) | VERIFIED | `TestWindowsExtensionMCPRoots` PASS; `extensionRootSegments` in `editorext.go` line 47 contains `.vscode-oss/extensions`; `baselineHomeCandidates` in `roots.go` line 254 contains `.vscode-oss/extensions`; test asserts all six extension dirs |
| 2 | On Windows, `resolveRoots(baseline)` returns per-profile browser-extension roots for Chrome/Chromium/Edge/Brave (under `%LOCALAPPDATA%`) and the Firefox Profiles parent (under `%APPDATA%`) | VERIFIED | `TestWindowsExtensionMCPRoots` PASS; `browserExtensionCandidateRoots` in `roots.go` lines 573–586 fills `case "windows":` for Chromium-family from `os.Getenv("LOCALAPPDATA")` with env-var guard; lines 622–625 adds Firefox Profiles parent from `os.Getenv("APPDATA")` |
| 3 | On Windows, `resolveRoots(baseline)` returns MCP config roots for all five hosts: Claude Desktop (`%APPDATA%\Claude`), Cline (`%APPDATA%\cline`), Cursor (`%USERPROFILE%\.cursor`), Windsurf (`%USERPROFILE%\.windsurf`), and Gemini CLI (`%USERPROFILE%\.gemini`) | VERIFIED | `TestWindowsExtensionMCPRoots` PASS; `baselineHomeCandidates` line 267 adds `.windsurf` unconditionally (cross-platform gap fix); `case "windows":` block lines 281–284 adds `%APPDATA%\Claude` and `%APPDATA%\cline` with APPDATA env-var guard; `.cursor` and `.gemini` covered by existing cross-platform dotfile roots |
| 4 | Beekeeper's scan seam invokes `pollen` (not `bumblebee`) via a mockable injectable var; `exec.LookPath("bumblebee")` is absent from `internal/scan/scanner.go` | VERIFIED | `scanner.go` declares `var lookPollenFn` (line 56), `var runPollenFn` (line 60), `func defaultRunPollen` (line 67) targeting `exec.LookPath("pollen")`; `grep -rn bumblebee internal/scan/` returns nothing; `pollen_unavailable` key replaces `bumblebee_unavailable`; out-of-scope catalog-source `bumblebee` literals in `internal/tui`, `internal/catalog` intentionally preserved |
| 5 | `TestPollenCompatibility` (PTEST-04) passes all five Pollen record types through beekeeper `Scan`, asserts `scanner_name=pollen`, no `scan_error`, no double-counting — with zero `t.Skip` on any OS | VERIFIED | `go test ./internal/scan/ -count=1 -v` output: all four tests PASS (including `TestPollenCompatibility`); grep for `t.Skip` in `scanner_test.go` returns zero skip calls; `TestPollenCompatibility` is fixture-driven with no binary spawn, OS filesystem access, or build tags |

**Score:** 5/5 truths verified

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | `v0.1.1-pollen.4` signed git tag + cosign signing + CycloneDX SBOM push | M2 close (D-06) | Phase 4 ROADMAP SC5 explicitly states "DEFERRED to M2 close per D-06"; CONTEXT.md decision record; `git tag --list v0.1.1-pollen.4` returns empty (confirmed); VERSION reads `0.1.1-pollen.4`, CHANGES.md has the v0.1.1-pollen.4 section with the D-06 deferral note |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `../pollen/cmd/pollen/roots.go` | Filled `case "windows":` blocks in `browserExtensionCandidateRoots` + Windows MCP case + `.windsurf` unconditional + `.vscode-oss` segment | VERIFIED | File exists; chromium `case "windows":` fills Chrome/Chromium/Edge/Brave via `LOCALAPPDATA` guard; Firefox `case "windows":` adds Profiles parent via `APPDATA` guard; `.windsurf` added unconditionally at line 267; `%APPDATA%\Claude` and `%APPDATA%\cline` in Windows MCP switch |
| `../pollen/internal/ecosystem/editorext/editorext.go` | `.vscode-oss/extensions` in `extensionRootSegments`; `hostFromExtRoot` maps `.vscode-oss` to `"vscodium"` | VERIFIED | Line 47: `.vscode-oss/extensions` present; line 134: `case strings.Contains(p, "/.vscode-oss"): return "vscodium"` |
| `../pollen/cmd/pollen/roots_windows_test.go` | `TestWindowsExtensionMCPRoots` asserts all five editor roots, Chromium+Firefox browser roots, and all five MCP hosts | VERIFIED | Function exists at line 130; asserts VS Code, Insiders, Cursor, Windsurf, `.vscode-oss`, `.vscodium`, Chrome, Brave, Firefox Profiles, Claude, Cline, Cursor MCP, Windsurf MCP, Gemini; `//go:build windows` header; zero `t.Skip` |
| `internal/scan/scanner.go` | `var runPollenFn` / `lookPollenFn` / `defaultRunPollen` targeting `pollen`; `pollen_unavailable` status | VERIFIED | `var lookPollenFn` at line 56; `var runPollenFn` at line 60; `defaultRunPollen` at line 67; `exec.LookPath("pollen")` present; `"pollen_unavailable":true` at line 137; `go build ./...` exits 0 |
| `internal/scan/scanner_test.go` | `TestPollenCompatibility` (5 fixtures, assertions) + `TestScanPollenUnavailable`; zero `runBumblebeeFn` references | VERIFIED | `TestPollenCompatibility` at line 171; `TestScanPollenUnavailable` at line 93; `grep runBumblebeeFn scanner_test.go` returns nothing; all four tests pass `go test ./internal/scan/ -count=1 -v` |
| `../pollen/VERSION` | Contains exactly `0.1.1-pollen.4` | VERIFIED | File reads `0.1.1-pollen.4` (single line) |
| `../pollen/CHANGES.md` | `## v0.1.1-pollen.4` section with D-06 deferral note and Modified/Added lists | VERIFIED | Section at line 9; D-06 deferral blockquote present; `### Modified` lists `roots.go` and `editorext.go`; `### Added` lists `roots_windows_test.go` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `roots.go browserExtensionCandidateRoots case "windows":` | `os.Getenv("LOCALAPPDATA")` | env-var-guarded `filepath.Join` | WIRED | Line 573: `if localappdata := os.Getenv("LOCALAPPDATA"); localappdata != "" {` |
| `roots.go baselineHomeCandidates case "windows":` | `%APPDATA%\Claude` and `%APPDATA%\cline` | `os.Getenv("APPDATA")` guard | WIRED | Lines 281–284: `if appdata := os.Getenv("APPDATA"); appdata != "" {` with both MCP roots added |
| `roots.go baselineHomeCandidates cross-platform MCP block` | `add(filepath.Join(home, ".windsurf"), model.RootKindMCPConfig)` | unconditional add before `switch runtime.GOOS` | WIRED | Line 267 present; adds the `.windsurf` root on all platforms |
| `internal/scan/scanner.go Scan()` | `runPollenFn(ctx, cfg.Deep)` | injectable package var (renamed from `runBumblebeeFn`) | WIRED | Line 109: `ch, ok := runPollenFn(ctx, cfg.Deep)` |
| `internal/scan/scanner.go defaultRunPollen` | `exec.LookPath("pollen")` | `lookPollenFn` | WIRED | Line 68: `bin, err := lookPollenFn()` calling `exec.LookPath("pollen")` |

### Data-Flow Trace (Level 4)

N/A — this phase produces path-enumeration and test code, not components that render dynamic data to a UI. The data flow for `resolveRoots` → `filterExistingRoots` → returned `[]scanner.Root` is verified by `TestWindowsExtensionMCPRoots` (live fixture directories are planted and the returned roots are checked by path + kind). `TestPollenCompatibility` verifies the fixture NDJSON flows through `Scan` to `out io.Writer` without rejection.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `TestWindowsExtensionMCPRoots` passes on Windows | `go test ./cmd/pollen/ -run TestWindowsExtensionMCPRoots -count=1 -v` | `--- PASS: TestWindowsExtensionMCPRoots (0.03s)` | PASS |
| `TestPollenCompatibility` (PTEST-04) passes with zero skips | `go test ./internal/scan/ -count=1 -v` + grep for SKIP | All 4 tests PASS; grep for SKIP returns no output | PASS |
| Zero `t.Skip` in beekeeper `internal/scan` tests | `go test ./internal/scan/ -count=1 -v \| grep -i skip` | Empty output (no skips) | PASS |
| Pollen and beekeeper repos build clean | `go build ./...` in both repos | Exit 0 (no output) in both repos | PASS |
| No `bumblebee` literals remain in `internal/scan/scanner.go` | `grep -rn bumblebee internal/scan/` | Empty output | PASS |
| Out-of-scope catalog-source `bumblebee` literals preserved | `grep -rn bumblebee internal/tui/ internal/catalog/` | Returns `bumblebee.idx`, `CatalogSource:"bumblebee"` entries (unchanged) | PASS |
| `editorext.go` contains both `.vscodium` and `.vscode-oss` | `grep -n "vscodium\|vscode-oss" editorext.go` | Both present in `extensionRootSegments`; `hostFromExtRoot` maps `.vscode-oss` → `"vscodium"` | PASS |
| VERSION file reads `0.1.1-pollen.4` | File read | `0.1.1-pollen.4` | PASS |
| No signed tag created (D-06 deferral) | `git tag --list v0.1.1-pollen.4` | Empty (no tag) | PASS |

### Probe Execution

Step 7c: SKIPPED — no probe scripts declared in PLANs or SUMMARY files for this phase. Behavioral verification is fully covered by `TestWindowsExtensionMCPRoots` and `TestPollenCompatibility` (see spot-checks above).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WEXT-01 | 04-01-PLAN.md | Windows editor-extension paths — VS Code, Insiders, Cursor, Windsurf, VSCodium (both `.vscodium` and `.vscode-oss`) | SATISFIED | `extensionRootSegments` has both; `baselineHomeCandidates` has `.vscode-oss`; `TestWindowsExtensionMCPRoots` asserts all six variants; test PASS |
| WEXT-02 | 04-01-PLAN.md | Windows browser-extension paths — Chrome/Chromium/Edge/Brave per-profile + Firefox per-profile parent | SATISFIED | `browserExtensionCandidateRoots` `case "windows":` fills all four Chromium families via LOCALAPPDATA guard and Firefox Profiles parent via APPDATA guard; `TestWindowsExtensionMCPRoots` asserts Chrome, Brave, Firefox roots |
| WEXT-03 | 04-01-PLAN.md | Windows MCP host-config paths — Claude Desktop, Cline, Cursor, Windsurf, Gemini | SATISFIED | `%APPDATA%\Claude` and `%APPDATA%\cline` in Windows MCP switch; `.windsurf` added unconditionally; `.cursor` and `.gemini` covered by cross-platform roots; `TestWindowsExtensionMCPRoots` asserts all five MCP hosts |
| BKINT-01 | 04-02-PLAN.md | Beekeeper consumes Pollen behind a mockable interface; switching back is a one-line target change | SATISFIED | `runPollenFn` injectable var in `scanner.go`; `exec.LookPath("pollen")` is the only binary reference; reverting to `bumblebee` is a one-string change; unit tests inject mock without binary spawn |
| PTEST-04 | 04-02-PLAN.md | `TestPollenCompatibility` passes all five record types, asserts `scanner_name=pollen`, no `scan_error`, no double-counting; zero `t.Skip` on all OSes | SATISFIED | `TestPollenCompatibility` function present and passes; fixture-driven (no binary spawn); zero `t.Skip`; all assertions on scan_error, scanner_name count, source_file uniqueness confirmed in code review |

### Anti-Patterns Found

No anti-patterns found in phase-modified files. Specific checks performed:

- `grep -n "TBD\|FIXME\|XXX" roots.go editorext.go roots_windows_test.go scanner.go scanner_test.go` — no results
- `grep -n "TODO\|HACK\|PLACEHOLDER" ...` — no results
- `grep -n "return null\|return \[\]\|return \{\}" scanner.go` — only `return nil` in error paths (correct behavior)
- Hardcoded empty values: none introduced; all path construction uses `filepath.Join + os.Getenv`
- `t.Skip` calls: zero in new and modified test code (confirmed by grep)

### Human Verification Required

None — all phase deliverables are verifiable programmatically on this Windows dev machine. The `//go:build windows` test (`TestWindowsExtensionMCPRoots`) ran directly. The `TestPollenCompatibility` test is fixture-driven and OS-agnostic. No visual UI, real-time behavior, or external service integration is introduced in this phase.

### Gaps Summary

No gaps found. All five must-have truths are VERIFIED, all artifacts are substantive and wired, all tests pass on the Windows development machine, and the intentionally deferred signed release tag (D-06) is correctly classified as a tracked deferral to M2 close — not a phase failure.

The `bumblebee` catalog-source literals intentionally preserved in `internal/tui`, `internal/gateway`, `internal/watch`, and `internal/catalog` are confirmed out-of-scope for BKINT-01, as specified in the plan and verified by code inspection.

---

_Verified: 2026-06-02_
_Verifier: Claude (gsd-verifier)_
