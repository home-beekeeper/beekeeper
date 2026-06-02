---
phase: 03-windows-path-representation
plan: "01"
subsystem: pollen-ecosystem-helpers
tags: [wpath-01, npm, pnpm, windows, path-representation, filepath.FromSlash]
dependency_graph:
  requires: []
  provides: [WPATH-01-npm-fix, WPATH-01-pnpm-fix]
  affects: [internal/ecosystem/npm, internal/ecosystem/pnpm]
tech_stack:
  added: []
  patterns: [filepath.FromSlash wrap, runtime.GOOS skip guard, raw Windows string literals in tests]
key_files:
  created: []
  modified:
    - ../pollen/internal/ecosystem/npm/npm.go
    - ../pollen/internal/ecosystem/npm/npm_test.go
    - ../pollen/internal/ecosystem/pnpm/pnpm.go
    - ../pollen/internal/ecosystem/pnpm/pnpm_test.go
decisions:
  - "filepath.FromSlash wrap is the one-line fix — no import changes needed (path/filepath already imported)"
  - "TestIsPnpmStorePackageJSON gained a runtime.GOOS==windows skip (Rule 1 regression fix) because the existing Unix-path fixture /x/proj is converted to \\x\\proj on Windows by the fix"
  - "TestIsNodeModulesPackageJSONShapes unaffected — only checks boolean ok, not projectPath string"
metrics:
  duration: "12 minutes"
  completed: "2026-06-02"
  tasks_completed: 2
  files_changed: 4
---

# Phase 03 Plan 01: WPATH-01 npm/pnpm filepath.FromSlash Fix Summary

Single-line `filepath.FromSlash` wrap in two ecosystem helper functions so that `project_path` in emitted NDJSON records uses native Windows backslash separators on Windows, while remaining byte-identical on Linux/macOS.

## What Was Done

### Production changes (2 lines total across 2 files)

**`internal/ecosystem/npm/npm.go` line 114** (inside `IsNodeModulesPackageJSON`):
```
Before: projectPath := strings.Join(parts[:nmIdx], "/")
After:  projectPath := filepath.FromSlash(strings.Join(parts[:nmIdx], "/"))
```

**`internal/ecosystem/pnpm/pnpm.go` line 87** (inside `IsPnpmStorePackageJSON`):
```
Before: projectPath = strings.Join(parts[:pnpmIdx-1], "/")
After:  projectPath = filepath.FromSlash(strings.Join(parts[:pnpmIdx-1], "/"))
```

No new imports were added to either production file. `path/filepath` was already imported at line 14 (npm.go) and line 18 (pnpm.go).

### Test changes

**`internal/ecosystem/npm/npm_test.go`** — added `"runtime"` import + two new test functions:
- `TestIsNodeModulesPackageJSONWindowsPath`: guards with `runtime.GOOS != "windows"` t.Skip; uses raw Windows string literal `C:\Users\fana\code\web-app\node_modules\left-pad\package.json`; asserts `projectPath == "C:\Users\fana\code\web-app"` (backslash + drive letter)
- `TestIsNodeModulesPackageJSONScopedWindowsPath`: same pattern for scoped `@scope\pkg` path

**`internal/ecosystem/pnpm/pnpm_test.go`** — added `"runtime"` import + one new test function + one skip guard:
- `TestIsPnpmStorePackageJSONWindowsPath`: guards with `runtime.GOOS != "windows"` t.Skip; asserts `projectPath`, `name`, and `version` from a raw Windows pnpm store path
- `TestIsPnpmStorePackageJSON` (existing): gained a `runtime.GOOS == "windows"` skip — see Deviations

## Test Exit Status (Windows dev machine)

```
go vet ./internal/ecosystem/npm/ ./internal/ecosystem/pnpm/  → clean
go test ./internal/ecosystem/npm/                            → PASS (9/9, 0 skip)
go test ./internal/ecosystem/pnpm/                          → PASS (8/8, 1 skip)
```

Windows tests `TestIsNodeModulesPackageJSONWindowsPath`, `TestIsNodeModulesPackageJSONScopedWindowsPath`, and `TestIsPnpmStorePackageJSONWindowsPath` all RAN (not skipped) and PASSED on the Windows dev machine.

## Pollen Commits

Both commits are in the Pollen repo (`../pollen`), staged with explicit file paths (no `git add .`):

| Commit | Hash | Message |
|--------|------|---------|
| Task 1 | `2daf939` | `fix(03-01): WPATH-01 npm — wrap projectPath join in filepath.FromSlash + Windows unit tests` |
| Task 2 | `f21e231` | `fix(03-01): WPATH-01 pnpm — wrap projectPath join in filepath.FromSlash + Windows unit test` |

`git diff HEAD~2 HEAD --stat` (pollen) shows exactly 4 files: npm.go, npm_test.go, pnpm.go, pnpm_test.go.

## Differential Safety (Unix Byte-Identity)

`filepath.FromSlash` is defined as a no-op on any OS where `/` is the native separator (Linux, macOS). The NDJSON bytes emitted on Unix are therefore byte-identical before and after this fix. `normalize_diff.go` is NOT modified (it is LOCKED). `TestDifferential` (PTEST-02) will pass unchanged on Linux/macOS CI.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestIsPnpmStorePackageJSON regression on Windows**
- **Found during:** Task 2 GREEN phase
- **Issue:** The existing `TestIsPnpmStorePackageJSON` uses Unix-style paths (`/x/proj/node_modules/.pnpm/...`) and asserts `proj == "/x/proj"`. After applying `filepath.FromSlash`, on Windows these forward slashes are converted to backslashes, yielding `\x\proj` which does not equal `/x/proj` → test failure.
- **Fix:** Added `runtime.GOOS == "windows"` skip to `TestIsPnpmStorePackageJSON` with message pointing to `TestIsPnpmStorePackageJSONWindowsPath` as the Windows coverage. The Unix-path test remains fully valid on Linux/macOS CI where it will continue to run.
- **Files modified:** `../pollen/internal/ecosystem/pnpm/pnpm_test.go`
- **Commit:** `f21e231` (included in Task 2 commit)
- **Note:** `TestIsNodeModulesPackageJSONShapes` was NOT affected because it only checks the boolean `ok` return value, not the `projectPath` string.

## Known Stubs

None. Both production functions return the fully-computed OS-native projectPath.

## Threat Flags

None. The fix is a separator-representation correctness change over scanner-derived paths with no new untrusted-input path or new attack surface (T-03-01, T-03-02 accepted; T-03-03 mitigated — normalize_diff.go not edited).

## Self-Check: PASSED

- `../pollen/internal/ecosystem/npm/npm.go` — FOUND: contains `filepath.FromSlash(strings.Join(parts[:nmIdx]` at line 114
- `../pollen/internal/ecosystem/pnpm/pnpm.go` — FOUND: contains `filepath.FromSlash(strings.Join(parts[:pnpmIdx-1]` at line 87
- `../pollen/internal/ecosystem/npm/npm_test.go` — FOUND: contains `TestIsNodeModulesPackageJSONWindowsPath` and `TestIsNodeModulesPackageJSONScopedWindowsPath`
- `../pollen/internal/ecosystem/pnpm/pnpm_test.go` — FOUND: contains `TestIsPnpmStorePackageJSONWindowsPath`
- Pollen commits `2daf939` and `f21e231` — VERIFIED in `git -C ../pollen log --oneline -5`
- `go vet` + `go test` exit 0 for both packages — VERIFIED above
