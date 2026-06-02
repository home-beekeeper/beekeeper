---
phase: 02-windows-root-resolver
plan: 01
subsystem: inventory
tags: [go, windows, root-resolver, pollen, cross-platform, package-manager]

# Dependency graph
requires:
  - phase: 01-fork-setup-discipline
    provides: pollen fork baseline with GOOS=windows build passing and cross-repo commit discipline
provides:
  - windowsBaselinePackageRoots() in roots_windows.go covering npm/pnpm/Yarn/Bun/PyPI/Go/RubyGems/Composer
  - windowsSystemRoots() for ProgramFiles-based global roots
  - case "windows": delegation in all 3 roots.go GOOS switches
  - isBroadHomeRoot Windows drive-root (C:\) detection via filepath.VolumeName
  - roots_notwindows.go stubs enabling tri-GOOS build
affects: [02-02, 02-03, 03-windows-path-representation, 04-windows-extension-mcp]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "//go:build windows + //go:build !windows stub pair for OS-specific functions in package main"
    - "env-var-level guard: if appdata := os.Getenv(\"APPDATA\"); appdata != \"\" (Pitfall 1 prevention)"
    - "filepath.VolumeName drive-root detection added unconditionally to isBroadHomeRoot (no-op on Unix)"

key-files:
  created:
    - ../pollen/cmd/pollen/roots_windows.go
    - ../pollen/cmd/pollen/roots_notwindows.go
  modified:
    - ../pollen/cmd/pollen/roots.go

key-decisions:
  - "Added roots_notwindows.go (//go:build !windows) stubs because Go compiles all switch case bodies regardless of GOOS value — case \"windows\": bodies that reference windowsBaselinePackageRoots/windowsSystemRoots fail on Linux/Darwin without stubs"
  - "env-var guards are per-variable (if appdata := ...) not a single upfront check — Windows has multiple independent env vars unlike Unix HOME"
  - "browserExtensionCandidateRoots Windows case is intentionally empty (Phase 4 WEXT-02 owns it); skeleton added to both Chromium and Firefox switches"
  - "GOPATH not consulted for Go modules root — mirrors upstream Unix behavior (uses %USERPROFILE%\\go\\pkg\\mod only)"

patterns-established:
  - "roots_windows.go / roots_notwindows.go build-tag pair: use this pattern for any future OS-specific functions in package main that are called from untagged switch cases"
  - "globExisting called directly from roots_windows.go (same package, no import) for Python*/Ruby* wildcard expansion"

requirements-completed: [WRES-01, WRES-02]

# Metrics
duration: 15min
completed: 2026-06-02
---

# Phase 02 Plan 01: Windows Root Resolver Summary

**Windows package-root discovery for all 8 ecosystems via filepath.Join+env-var guards, isolated behind //go:build windows, with tri-GOOS build and go vet green**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-06-02T09:50:00Z
- **Completed:** 2026-06-02T10:02:28Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified in pollen repo)

## Accomplishments

- `windowsBaselinePackageRoots()` covers all 8 ecosystems (npm/pnpm/Yarn/Bun via APPDATA/LOCALAPPDATA/USERPROFILE; PyPI/Go/RubyGems/Composer via same vars with globExisting for wildcards)
- `windowsSystemRoots()` covers ProgramFiles npm MSI and RubyGems system glob — isolated in `roots_windows.go` (//go:build windows)
- `roots.go` gains `case "windows":` delegation in `baselineHomeCandidates`, `systemRoots`, and both `browserExtensionCandidateRoots` switches (Phase 4 skeleton); `isBroadHomeRoot` gains `filepath.VolumeName` drive-root detection
- WRES-01 and WRES-02 requirements satisfied; committed to pollen repo as feat(02-01)

## Task Commits

Tasks 1 and 2 had no intermediate commits per cross-repo discipline (code in ../pollen). Task 3 is the single pollen commit.

1. **Task 1: Create roots_windows.go** - no intermediate commit (pollen repo, per cross-repo protocol)
2. **Task 2: Wire case "windows": into roots.go** - no intermediate commit (pollen repo, per cross-repo protocol)
3. **Task 3: Commit to pollen repo** - `2c202ef` (feat(02-01): Windows root resolver for 8 ecosystems)

## Files Created/Modified

- `../pollen/cmd/pollen/roots_windows.go` - Windows-only root-discovery functions for 8 ecosystems (//go:build windows)
- `../pollen/cmd/pollen/roots_notwindows.go` - No-op stubs enabling tri-GOOS compilation (//go:build !windows)
- `../pollen/cmd/pollen/roots.go` - Added case "windows": delegation and drive-root detection

## Decisions Made

- Added `roots_notwindows.go` (//go:build !windows) with no-op stubs: Go compiles all switch case bodies at build time regardless of `runtime.GOOS` value. The `case "windows":` bodies in `roots.go` reference `windowsBaselinePackageRoots`/`windowsSystemRoots` which are defined only in `roots_windows.go`. Without stubs, GOOS=linux and GOOS=darwin builds fail with "undefined" errors. This is the standard Go cross-platform idiom.
- env-var guards are per-variable: `if appdata := os.Getenv("APPDATA"); appdata != ""` before every block that uses `appdata`. Windows has multiple independent env vars (not a single HOME), so a single upfront check is insufficient.
- GOPATH not consulted for Go modules path — mirrors upstream Unix behavior which uses `~/go/pkg/mod` without reading GOPATH.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added roots_notwindows.go to fix Linux/Darwin compilation**
- **Found during:** Task 2 verification (`GOOS=linux go build ./cmd/pollen/`)
- **Issue:** Go compiles all switch case bodies regardless of GOOS; `case "windows":` blocks in roots.go reference `windowsBaselinePackageRoots` and `windowsSystemRoots` which exist only in the //go:build windows file. Linux/Darwin builds produced "undefined: windowsBaselinePackageRoots" and "undefined: windowsSystemRoots" errors.
- **Fix:** Created `cmd/pollen/roots_notwindows.go` (//go:build !windows) with no-op stubs returning nil for both functions.
- **Files modified:** `../pollen/cmd/pollen/roots_notwindows.go` (created)
- **Verification:** `GOOS=linux go build ./cmd/pollen/` and `GOOS=darwin go build ./cmd/pollen/` both exit 0
- **Committed in:** 2c202ef (Task 3 pollen commit, alongside roots_windows.go and roots.go)

---

**Total deviations:** 1 auto-fixed (Rule 3 - blocking compilation issue)
**Impact on plan:** Required for tri-GOOS build to pass. No scope creep — stub file is minimal (14 lines) and follows the standard Go cross-platform build-tag idiom.

## Issues Encountered

- Plan's claim that "Go only compiles the case body for the platform" is incorrect — Go always compiles all switch case bodies. The standard solution (//go:build !windows stub file) was applied immediately as a Rule 3 auto-fix.

## Known Stubs

- `roots_notwindows.go` contains intentional no-op stubs for `windowsBaselinePackageRoots()` and `windowsSystemRoots()` — these are compiler shims, not user-facing stubs. They are never called at runtime on non-Windows.
- `browserExtensionCandidateRoots` Windows cases are intentionally empty (Phase 4 WEXT-02 owns the implementation).

## Threat Surface Scan

No new network endpoints, auth paths, or trust-boundary changes. The Windows root additions read OS env vars (process-owner-controlled) and call `filepath.Glob`/`os.Stat` — same trust surface as existing Unix root resolution. T-02-01 (empty-env-var → relative path injection) mitigated by per-var guards. T-02-02 (junction/symlink) and T-02-03 (absent root DoS) mitigated by existing `filterExistingRoots` discipline (caller side).

## Next Phase Readiness

- Plan 02-02 (flip Phase-2 Windows skips in main_test.go + roots_windows_test.go) can proceed immediately
- Plan 02-03 (parity test + fixture tree) can proceed
- roots_windows.go is cleanly isolated — extractable as a contribution-back PR to upstream Bumblebee (Phase 5)

## Self-Check

- [ ] roots_windows.go exists
- [ ] roots_notwindows.go exists
- [ ] roots.go modified

---
*Phase: 02-windows-root-resolver*
*Completed: 2026-06-02*
