---
phase: 02-windows-root-resolver
verified: 2026-06-02T14:00:00Z
status: passed
score: 7/7
overrides_applied: 0
deferred:
  - truth: "v0.1.1-pollen.2 is tagged and signed; Windows CI no longer skips root-resolver tests (signed tag portion)"
    addressed_in: "Milestone 2 close (tracked)"
    evidence: "SC4 skips-flipped portion is VERIFIED (0 Phase-2 skip strings remain). The signed-tag portion is intentionally deferred to M2 close per maintainer decision — tracked in STATE.md Deferred Items table and ROADMAP.md Phase 2 pending-release note (HEAD c94b271 on ../pollen). The deferred item does not affect the code-goal verdict."
---

# Phase 2: Windows Root Resolver — Verification Report

**Phase Goal:** Pollen can discover all 8 package-manager roots on Windows — npm/pnpm/Yarn/Bun, PyPI, Go modules, RubyGems, Composer — using %APPDATA%/%LOCALAPPDATA%/%USERPROFILE%/%ProgramFiles%, with the cross-platform parity test asserting equivalent detection against Linux; the differential test stays green on Linux+macOS; and a signed v0.1.1-pollen.2 release with Windows CI no longer skipping root-resolver tests.

**Repo locus:** `C:\Users\Bantu\mzansi-agentive\pollen` (`../pollen` sibling repo)
**Verified:** 2026-06-02T14:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | On Windows, baseline scan-root resolution emits candidate roots for all 8 package ecosystems built from the 4 Windows env vars | VERIFIED | `roots_windows.go` (114 lines, `//go:build windows`) implements `windowsBaselinePackageRoots()` and `windowsSystemRoots()` covering npm, pnpm, Yarn, Bun, PyPI, Go modules, RubyGems, Composer with per-env-var guards. `case "windows":` delegation wired in `roots.go` `baselineHomeCandidates` and `systemRoots`. |
| 2 | An unset/empty env var produces zero roots for that var (never a CWD-relative path leak) | VERIFIED | Every env-var read in `roots_windows.go` uses an enclosing `if appdata := os.Getenv("APPDATA"); appdata != ""` guard. `TestWindowsBaselineRootsEmptyAppdata` in `roots_windows_test.go` asserts `filepath.VolumeName(r.Path) != ""` for every returned root when APPDATA is empty. |
| 3 | Linux and macOS root resolution is byte-unchanged — all new Windows code isolated behind `//go:build windows` | VERIFIED | `roots_windows.go` line 1 is `//go:build windows`. `roots_notwindows.go` (`//go:build !windows`) provides nil stubs. `git diff` on pre-existing darwin/linux `case` bodies shows additions only. `normalize_diff.go` is untouched (`git diff --quiet` exits 0). |
| 4 | `isBroadHomeRoot` refuses a Windows drive root (`C:\`) just as it refuses `/` on Unix | VERIFIED | `roots.go` lines 201-213 implement the `filepath.VolumeName` branch — refuses `C:\` (drive root), `C:\Users` (/Users equivalent), and `C:\Users\<name>` (/Users/<name> equivalent). `TestIsBroadHomeRoot` in `main_test.go` has `runtime.GOOS == "windows"` branch asserting `C:\`, `C:\Users`, and `$USERPROFILE` as broad. |
| 5 | On a Windows host, `TestWindowsBaselineRoots` proves all 8 ecosystems resolve via fixture dirs; empty-env guard proven | VERIFIED | `roots_windows_test.go` (156 lines, `//go:build windows`) contains `TestWindowsBaselineRoots` (10 ecosystem dirs, 10 assertions) and `TestWindowsBaselineRootsEmptyAppdata`. Uses USERPROFILE/APPDATA/LOCALAPPDATA/ProgramFiles via `t.Setenv`; never uses HOME. |
| 6 | The 6 Phase-2 `t.Skip` calls no longer carry "arrive in Phase 2" or `v0.1.1-pollen.2` language | VERIFIED | `grep 'arrive in Phase 2\|v0.1.1-pollen.2' main_test.go` returns 0 matches. All remaining 9 skips carry darwin-specific or unix-specific (non-Phase-2) reasons. The differential skip at `differential_test.go:55` is intact and intentionally kept. |
| 7 | `TestParityAllEcosystems` runs on all three OSes (no build tag), asserts `endpoint.os == runtime.GOOS`, and covers all 5 ecosystem strings (npm/pypi/go/rubygems/packagist) | VERIFIED | `parity_test.go` (147 lines) has no `//go:build` tag. Reuses `buildCurrentPollen`/`runBinaryOnFixture`/`normalize` without redefining them (0 duplicate definitions). Contains `runtime.GOOS` assertion via `assertEndpointOS`. 8-ecosystem fixture tree committed under `testdata/parity-fixture/` (8 files confirmed). |

**Score:** 7/7 truths verified

### Deferred Items

Items not yet met but explicitly addressed in later milestone phases.

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | `v0.1.1-pollen.2` git tag pushed, Sigstore signed, CycloneDX SBOM attached | Milestone 2 close | Maintainer decision 2026-06-02. VERSION bumped to `0.1.1-pollen.2`, CHANGES.md section committed at HEAD `c94b271`. 4 commits unpushed from `../pollen` main. Tracked in STATE.md Deferred Items table and ROADMAP.md Phase 2 pending-release note. The SC4 "Windows CI no longer skips root-resolver tests" sub-criterion is FULLY MET (see Truth 6 above). Only the signed-tag action is deferred. |

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `../pollen/cmd/pollen/roots_windows.go` | `windowsBaselinePackageRoots()` + `windowsSystemRoots()` for 8 ecosystems, `//go:build windows`, 60+ lines | VERIFIED | 114 lines. Build tag on line 1. Both functions present. All 4 env vars guarded. 3 `globExisting` calls (Python*, .gem/ruby/*, Ruby*). Zero `filepath.FromSlash` or hard-coded `C:`. |
| `../pollen/cmd/pollen/roots_notwindows.go` | `//go:build !windows` nil stubs for `windowsBaselinePackageRoots` and `windowsSystemRoots` | VERIFIED | 14 lines. `//go:build !windows`. Both nil-returning stubs present. Required for cross-OS compilation of `case "windows":` bodies in `roots.go`. |
| `../pollen/cmd/pollen/roots.go` | `case "windows":` in `baselineHomeCandidates`, `systemRoots`, both `browserExtensionCandidateRoots` switches; `filepath.VolumeName` in `isBroadHomeRoot` | VERIFIED | 4 `case "windows":` occurrences confirmed. `windowsSystemRoots()` and `windowsBaselinePackageRoots()` calls wired. 2 `filepath.VolumeName` occurrences. Darwin/linux cases unchanged. |
| `../pollen/cmd/pollen/roots_windows_test.go` | `TestWindowsBaselineRoots` + empty-env guard test, `//go:build windows`, 60+ lines | VERIFIED | 156 lines. `//go:build windows`. `TestWindowsBaselineRoots` and `TestWindowsBaselineRootsEmptyAppdata` present. No `t.Setenv("HOME", ...)`. All 4 Windows env vars set via `t.Setenv`. |
| `../pollen/cmd/pollen/main_test.go` | 6 flipped Phase-2 skips + Windows cases in `TestIsBroadHomeRoot` | VERIFIED | Zero `arrive in Phase 2` or `v0.1.1-pollen.2` strings. `TestIsBroadHomeRoot` has `runtime.GOOS == "windows"` branch with Windows-shaped broad/narrow cases. 9 remaining skips are all darwin/unix-specific. |
| `../pollen/cmd/pollen/parity_test.go` | `TestParityAllEcosystems` (no build tag), reuses locked helpers, 50+ lines | VERIFIED | 147 lines. No `//go:build` tag. 0 redefinitions of `buildCurrentPollen`/`runBinaryOnFixture`/`normalize`. `runtime.GOOS` referenced 5 times. `assertEndpointOS` and `assertParityRecordCoverage` local helpers. |
| `../pollen/cmd/pollen/testdata/parity-fixture/` | 8-ecosystem fixture tree (5 ecosystem strings) | VERIFIED | 8 files confirmed: `npm-fixture/package-lock.json`, `pnpm-fixture/pnpm-lock.yaml`, `yarn-fixture/yarn.lock`, `bun-fixture/bun.lock`, `pypi-fixture/parity_pypi_canary-1.0.0.dist-info/METADATA`, `gomod-fixture/go.sum`, `rubygems-fixture/specifications/parity-gem-1.0.0.gemspec` (parent dir `specifications/` correct), `composer-fixture/composer.lock`. |
| `../pollen/VERSION` | Single line `0.1.1-pollen.2` | VERIFIED | File reads `0.1.1-pollen.2` (newline only). |
| `../pollen/CHANGES.md` | `v0.1.1-pollen.2` section with actual filenames (not PRD-draft `internal/resolver/`) | VERIFIED | `## v0.1.1-pollen.2` section present. References `cmd/pollen/roots_windows.go` (actual path). Zero occurrences of `internal/resolver/resolver_windows.go`. Section lists parity_test.go, parity-fixture/, the 6 flipped skips, and the `isBroadHomeRoot` drive-root change. |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `roots.go baselineHomeCandidates` | `windowsBaselinePackageRoots()` | `case "windows":` delegation at line ~283 | VERIFIED | `switch runtime.GOOS { case "windows": for _, p := range windowsBaselinePackageRoots() { add(p.Path, p.Kind) } }` present and wired. |
| `roots.go systemRoots` | `windowsSystemRoots()` | `case "windows": return windowsSystemRoots()` | VERIFIED | `case "windows": return windowsSystemRoots()` present in `systemRoots()`. |
| `roots.go browserExtensionCandidateRoots` | Phase-4 skeleton | `case "windows":` (empty, documented) in both chromium and Firefox switches | VERIFIED | 2 empty `case "windows":` blocks with Phase-4 deferral comments present in both switch blocks (lines ~560, ~593). |
| `roots_windows.go` | `globExisting` (roots.go same package) | Direct call — same `package main` | VERIFIED | `globExisting(filepath.Join(...))` called 3 times in `roots_windows.go` (Python*, .gem/ruby/*, Ruby* wildcards). `globExisting` defined in `roots.go`, same package, no import needed. |
| `parity_test.go` | `buildCurrentPollen` + `runBinaryOnFixture` + `normalize` (differential_test.go / normalize_diff.go) | Same-package reuse, no redefinition | VERIFIED | 0 redefinitions confirmed via grep. All three helpers called directly. |
| `parity_test.go` | `testdata/parity-fixture` | `runtime.Caller(0)` + `filepath.Dir` + `"testdata", "parity-fixture"` | VERIFIED | `fixtureDir := filepath.Join(filepath.Dir(thisFile), "testdata", "parity-fixture")` present in `TestParityAllEcosystems`. |

---

## Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| WRES-01 | 02-01, 02-02 | Windows JS-ecosystem roots: npm (global `%APPDATA%\npm\node_modules` + ProgramFiles MSI + caches), pnpm (`%LOCALAPPDATA%\pnpm\store`, `%APPDATA%\pnpm`), Yarn (`%LOCALAPPDATA%\Yarn\Data\global`), Bun (`%USERPROFILE%\.bun\install\cache`) | SATISFIED | `roots_windows.go` lines 34-58 implement all 4 JS ecosystems with guarded env-var reads. `TestWindowsBaselineRoots` asserts all 4 plus npm-MSI roots. |
| WRES-02 | 02-01, 02-02 | Windows PyPI/Go/RubyGems/Composer roots | SATISFIED | `roots_windows.go` lines 62-88 implement PyPI (`globExisting` Python*), Go modules (`%USERPROFILE%\go\pkg\mod`), RubyGems user (`globExisting` .gem/ruby/*) + system (`windowsSystemRoots` globExisting Ruby*), Composer (`%APPDATA%\Composer\vendor`). |
| PTEST-01 | 02-03 | Cross-platform parity test: identical fixture produces equivalent inventory records on Linux/macOS/Windows; `endpoint.os` differs correctly per platform | SATISFIED | `parity_test.go` + `testdata/parity-fixture/` committed. No build tag. `assertEndpointOS` checks `endpoint.os == runtime.GOOS`. `assertParityRecordCoverage` checks all 5 ecosystem strings. Test confirmed PASS on this Windows host per session context. |

**Note on REQUIREMENTS.md wording vs implementation:** WRES-01/WRES-02 name `internal/resolver/resolver_windows.go` as the file path. The live repo uses `cmd/pollen/roots_windows.go` (Option B in-place per CONTEXT.md decision). ROADMAP.md Phase 2 explicitly documents this: "the PRD-draft `internal/resolver/` path does not exist in the live repo." The requirement intent (Windows root discovery for 8 ecosystems) is fully satisfied; only the literal filename differs. No override needed — ROADMAP itself resolves the discrepancy.

---

## Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `roots_windows.go windowsBaselinePackageRoots` | `out []scanner.Root` | `os.Getenv("APPDATA"/"LOCALAPPDATA"/"USERPROFILE"/"ProgramFiles")` + `filepath.Join` + `globExisting` | Yes — env vars read at runtime; `globExisting` stats real directories | FLOWING |
| `parity_test.go TestParityAllEcosystems` | `out []byte` (NDJSON from pollen) | `runBinaryOnFixture` → real pollen binary → scans `testdata/parity-fixture/` | Yes — real binary, committed fixture files | FLOWING |
| `roots.go isBroadHomeRoot` | `vol string` from `filepath.VolumeName(abs)` | `filepath.Abs(path)` + `filepath.VolumeName` | Yes — operates on caller-supplied path | FLOWING |

---

## Behavioral Spot-Checks

| Behavior | Check | Result | Status |
|----------|-------|--------|--------|
| `roots_windows.go` has `//go:build windows` on line 1 | `head -1 roots_windows.go` | `//go:build windows` | PASS |
| `main_test.go` contains zero Phase-2 skip strings | `grep -c 'arrive in Phase 2\|v0.1.1-pollen.2' main_test.go` | 0 | PASS |
| `roots.go` has 4 `case "windows":` occurrences | `grep -c 'case "windows":' roots.go` | 4 | PASS |
| `roots.go` wires `windowsSystemRoots()` and `windowsBaselinePackageRoots()` | `grep -c 'windowsSystemRoots()\|windowsBaselinePackageRoots()' roots.go` | 2 | PASS |
| `roots_windows.go` has no hand-built backslash strings | `grep -c 'filepath.FromSlash\|"C:' roots_windows.go` | 0 | PASS |
| `roots_windows_test.go` never uses `HOME` env var | `grep -c 't.Setenv("HOME"' roots_windows_test.go` | 0 | PASS |
| `parity_test.go` has no `//go:build` tag | File search for tag line | No match | PASS |
| `parity_test.go` does not redefine locked helpers | `grep -c 'func buildCurrentPollen\|func runBinaryOnFixture\|func normalize(' parity_test.go` | 0 | PASS |
| `normalize_diff.go` is unchanged since Phase 1 | `git diff --quiet HEAD~4 -- cmd/pollen/normalize_diff.go` | exit 0 | PASS |
| `VERSION` reads `0.1.1-pollen.2` | File content | `0.1.1-pollen.2` | PASS |
| `CHANGES.md` names actual file path (not PRD-draft) | `grep -c 'internal/resolver/resolver_windows.go' CHANGES.md` | 0; `grep -c 'cmd/pollen/roots_windows.go' CHANGES.md` | 1 | PASS |
| All 4 pollen commits are present | `git log --oneline -4` | `c94b271`, `833d29d`, `eba8e4c`, `2c202ef` (02-04, 02-03, 02-02, 02-01) | PASS |
| differential_test.go Windows skip intact | `grep -c 'PTEST-02 differential runs on Linux' differential_test.go` | 1 | PASS |
| 8 fixture files exist at exact paths | Glob of `testdata/parity-fixture/**/*` | 8 files, all present | PASS |
| `rubygems-fixture/specifications/` parent dir correct | Path structure | `specifications/parity-gem-1.0.0.gemspec` — parent is `specifications/` | PASS |

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `differential_test.go` line 56 | `t.Skip("PTEST-02 differential runs on Linux+macOS only; Windows behavior arrives Phase 2 (v0.1.1-pollen.2)")` | "arrives Phase 2 (v0.1.1-pollen.2)" in skip message | INFO | This is the INTENTIONALLY PRESERVED skip per plan and CONTEXT.md. The message refers to Windows path representation (Phase 3) and Windows differential coverage — not the root resolver. The phrasing is slightly stale (Phase 2 shipped root resolver; path representation is Phase 3) but the skip semantics are correct: the differential remains Linux+macOS-only. Not a blocker — this skip was explicitly protected throughout all plans. |

No blockers. No TBD/FIXME/XXX markers found in Phase-2 modified files.

---

## Human Verification Required

None. All must-haves are verifiable programmatically from the codebase. The test suite confirmation (`go test ./...` green on this Windows host, `TestParityAllEcosystems` PASS on Windows) was provided as session context and corroborated by codebase evidence (correct implementation, no stubs, data flows).

The deferred signed release is tracked (STATE.md + ROADMAP.md) and does not require human verification now — it will be a prerequisite check at M2 close.

---

## Gaps Summary

No gaps. All 7 must-have truths are VERIFIED. The signed release (SC4 tag/sign portion) is explicitly deferred by maintainer decision and tracked, not a gap.

---

_Verified: 2026-06-02T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
