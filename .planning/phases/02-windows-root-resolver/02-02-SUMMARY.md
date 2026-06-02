---
phase: 02-windows-root-resolver
plan: 02
subsystem: inventory
tags: [go, windows, root-resolver, pollen, cross-platform, testing, skip-discipline]

# Dependency graph
requires:
  - phase: 02-windows-root-resolver
    plan: 01
    provides: roots_windows.go with windowsBaselinePackageRoots/windowsSystemRoots + isBroadHomeRoot drive-root detection
provides:
  - roots_windows_test.go (//go:build windows) proving 8-ecosystem root discovery + empty-env guard
  - main_test.go with 6 Phase-2 Windows skips flipped (Windows-relevant tests run, Unix-specific tests carry non-Phase-2 reason)
  - isBroadHomeRoot C:\Users broad detection (Rule-1 auto-fix)
affects: [02-03, phase-2-CI-success-criterion-4]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "//go:build windows test file with t.Setenv(USERPROFILE/APPDATA/LOCALAPPDATA/ProgramFiles) + os.MkdirAll fixture dirs"
    - "runtime.GOOS gate in platform-agnostic test: if runtime.GOOS == windows { t.Setenv(USERPROFILE) } else { t.Setenv(HOME) }"
    - "Glob fixture pattern: create concrete versioned dir under wildcard parent (Python313, 3.3.0, Ruby33-x64)"

key-files:
  created:
    - ../pollen/cmd/pollen/roots_windows_test.go
  modified:
    - ../pollen/cmd/pollen/main_test.go
    - ../pollen/cmd/pollen/roots.go

key-decisions:
  - "isBroadHomeRoot C:\\\\Users and C:\\\\Users\\\\<name> added as broad cases (Rule-1 auto-fix): test asserted C:\\\\Users broad but implementation only had C:\\\\ drive-root; added Windows Users-dir and immediate-child detection mirroring /Users and /Users/<name> on Unix"
  - "TestWindowsBaselineRoots creates all 10 fixture dirs (8 user-package + 2 system) and asserts Kind on each; glob roots resolved via concrete versioned subdirs under wildcard parents"
  - "TestWindowsBaselineRootsEmptyAppdata asserts filepath.VolumeName(r.Path) != empty for all returned roots — proves T-02-06 Pitfall-1 guard holds"
  - "TestResolveRootsBaselineIncludesUserLocalPython keeps Windows skip with Unix-specific (non-Phase-2) reason pointing to TestWindowsBaselineRoots for Windows PyPI coverage"
  - "roots.go modification included in pollen commit (3 files total) because test assertions require the C:\\\\Users broad-detection fix"

patterns-established:
  - "Windows test env override: t.Setenv(USERPROFILE/APPDATA/LOCALAPPDATA/ProgramFiles) — never t.Setenv(HOME) in windows-tagged files"
  - "Glob root fixture pattern: create Python313, Ruby33-x64, 3.3.0 concrete dirs so filepath.Glob resolves the wildcard"
  - "runtime.GOOS adaptation in shared test: gate Unix path logic behind else branch, Windows path logic behind if windows block"

requirements-completed: [WRES-01, WRES-02]

# Metrics
duration: 25min
completed: 2026-06-02
---

# Phase 02 Plan 02: Windows Root-Resolver Tests + Skip Flips Summary

**Windows root-resolver unit tests (8 ecosystems, empty-env guard) + 6 Phase-2 skips flipped, with isBroadHomeRoot C:\\Users broad-detection auto-fix**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-06-02T09:48:00Z
- **Completed:** 2026-06-02T10:13:00Z
- **Tasks:** 3
- **Files modified:** 3 (1 created, 2 modified in pollen repo)

## Accomplishments

- `roots_windows_test.go` (`//go:build windows`): `TestWindowsBaselineRoots` proves all 8 ecosystems (npm, pnpm, Yarn, Bun, PyPI, Go modules, RubyGems user+system, Composer) resolve via `resolveRoots(baseline)` when env-var fixture dirs exist under `t.TempDir()`; `TestWindowsBaselineRootsEmptyAppdata` proves T-02-06 Pitfall-1 empty-APPDATA guard holds (no volume-less paths returned)
- `main_test.go`: all 6 Phase-2 Windows skips flipped — `TestIsBroadHomeRoot` runs on Windows with `C:\` / `C:\Users` / `USERPROFILE` as broad and `USERPROFILE\code` as narrow; `TestResolveRootsProjectIncludesCodeDir`, `TestResolveRootsBaselineRefusesBroadHome`, `TestResolveRootsProjectRefusesBroadHome`, `TestResolveRootsDeepAllowsBroadHome` adapted with `USERPROFILE` override; `TestResolveRootsBaselineIncludesUserLocalPython` keeps a skip with Unix-specific (non-Phase-2) reason
- `roots.go`: Rule-1 auto-fix — `isBroadHomeRoot` now detects `C:\Users` and `C:\Users\<name>` as broad (mirrors `/Users` and `/Users/<name>` on Unix)
- Pollen committed as `test(02-02): eba8e4c`; zero pushed

## Task Commits

Tasks 1 and 2 had no intermediate pollen commits (per cross-repo discipline). Task 3 is the single pollen commit.

1. **Task 1: Create roots_windows_test.go** — no intermediate commit (pollen repo, cross-repo protocol)
2. **Task 2: Flip 6 Phase-2 skips + Rule-1 fix to roots.go** — no intermediate commit (pollen repo, cross-repo protocol)
3. **Task 3: Commit to pollen repo** — `eba8e4c` (test(02-02): Windows root-resolver tests + flip 6 Phase-2 skips)

## Files Created/Modified

- `../pollen/cmd/pollen/roots_windows_test.go` — NEW; `//go:build windows`; 2 test functions covering 8 ecosystems + empty-env guard (156 lines)
- `../pollen/cmd/pollen/main_test.go` — 6 Phase-2 Windows skips flipped; `TestIsBroadHomeRoot` adapted with `runtime.GOOS` gates; 4 broad-home tests adapted with USERPROFILE override
- `../pollen/cmd/pollen/roots.go` — `isBroadHomeRoot` gains `C:\Users` and `C:\Users\<name>` broad detection (Rule-1 auto-fix)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] isBroadHomeRoot missing C:\\Users and C:\\Users\\<name> broad detection**
- **Found during:** Task 2 test execution — `TestIsBroadHomeRoot` failed with `isBroadHomeRoot("C:\\Users") = false, want true`
- **Issue:** Plan-01 added `C:\` drive-root detection via `filepath.VolumeName`. The test cases in PATTERNS.md correctly listed `C:\Users` as broad (mirrors Unix `/Users`), but `isBroadHomeRoot` only detected the drive root itself — not the Windows-equivalent of `/Users`
- **Fix:** Added Windows Users-dir detection in `isBroadHomeRoot`: if vol != "" { check `abs == vol+"\\"+"Users"` (C:\Users) and `filepath.Clean(parent) == usersDir` (C:\Users\<name>) }
- **Files modified:** `../pollen/cmd/pollen/roots.go`
- **Committed in:** `eba8e4c` (same task-3 commit alongside test files)

---

**Total deviations:** 1 auto-fixed (Rule 1 - behavioral bug in isBroadHomeRoot)

## Verification Results

- `go test ./cmd/pollen/ -run '^TestWindowsBaseline' -v`: PASS (TestWindowsBaselineRoots, TestWindowsBaselineRootsEmptyAppdata)
- `go test ./cmd/pollen/ -run '^TestIsBroadHomeRoot$|^TestResolveRoots' -v`: PASS (all 8 tests pass, 6 skip darwin/linux-specific)
- `GOOS=windows go vet ./cmd/pollen/`: exit 0
- `go test ./cmd/pollen/... -count=1`: PASS
- `grep -c 'arrive in Phase 2' cmd/pollen/main_test.go`: 0
- `grep -c 'v0.1.1-pollen.2' cmd/pollen/main_test.go`: 0
- `grep -c 'PTEST-02 differential runs on Linux' cmd/pollen/differential_test.go`: 1 (untouched)
- `-race` test: deferred (no gcc on Windows dev machine — pre-existing constraint from Phase 1, runs in CI with CGO_ENABLED=1)

## Known Stubs

None. All test assertions are live; `roots_windows_test.go` fixtures use real filesystem dirs.

## Threat Surface Scan

No new network endpoints, auth paths, or trust-boundary changes. Test files use `t.TempDir()` + `t.Setenv` exclusively — no real user-home paths touched. T-02-04 (test fixture escape) mitigated: all writes under auto-cleaned temp dirs. T-02-05 (silent Windows skips masking coverage) mitigated: 5 of 6 skips eliminated; 1 remaining skip is Unix-specific with clear reason. T-02-06 (empty-env CWD-relative path leak) verified by `TestWindowsBaselineRootsEmptyAppdata`.

## Next Phase Readiness

- Plan 02-03 (parity test + 8-ecosystem fixture tree) can proceed immediately
- Phase-2 Success Criterion 4 met: no Windows root-resolver test carries Phase-2/version deferral language
- `roots_windows_test.go` and `main_test.go` changes are isolated to `cmd/pollen/` — extractable for upstream contribution-back PR (Phase 5)

## Self-Check: PASSED

- `../pollen/cmd/pollen/roots_windows_test.go` exists: FOUND
- `../pollen/cmd/pollen/main_test.go` modified: FOUND
- `../pollen/cmd/pollen/roots.go` modified: FOUND
- Pollen commit `eba8e4c` exists: CONFIRMED (git log -1 --format=%H HEAD)
- No Phase-2 deferral language in main_test.go: CONFIRMED (grep count = 0)
- Differential skip intact: CONFIRMED (grep count = 1)
