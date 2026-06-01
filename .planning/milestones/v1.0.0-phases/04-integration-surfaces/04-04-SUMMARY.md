---
phase: 04-integration-surfaces
plan: "04"
subsystem: shim
tags: [shim, intg-06, tdd, cross-platform, path-intercept]
dependency_graph:
  requires: [04-02]
  provides: [internal/shim]
  affects: [cmd/beekeeper/main.go]
tech_stack:
  added: []
  patterns:
    - os-native-shim-scripts
    - filter-path-entries-for-lookpath-exclusion
    - tdd-red-green-refactor
key_files:
  created:
    - internal/shim/shim.go
    - internal/shim/shim_unix.go
    - internal/shim/shim_windows.go
    - internal/shim/shim_test.go
  modified: []
decisions:
  - "FindRealBinary exported as public function (via internal delegation) for test accessibility without exposing osLookPath var"
  - "TestShimRealBinary uses .cmd extension on Windows so exec.LookPath can resolve the binary (Windows requires PATHEXT-recognized extensions)"
  - "Uninstall does not take io.Writer parameter — plan spec has no out parameter; count tracking removed in REFACTOR phase"
metrics:
  duration: "5m"
  completed: "2026-05-26T21:43:42Z"
  tasks_completed: 1
  tasks_total: 1
---

# Phase 4 Plan 04: Shim Layer Summary

Shim layer for INTG-06: OS-native wrapper scripts in `~/.beekeeper/shims/` that intercept npm, pnpm, pip, cargo, go, gem, composer, npx, and pipx invocations via PATH prepend, routing each call through `beekeeper check` before forwarding to the real binary.

## Tasks

| # | Name | Commit | Status |
|---|------|--------|--------|
| 1 (RED) | Failing tests for shim package | 9749bfb | Done |
| 1 (GREEN) | Implement Install/Uninstall/Status + OS-specific writers | b958794 | Done |
| 1 (REFACTOR) | Remove unused removed counter | 1fcc41f | Done |

## What Was Built

### `internal/shim/shim.go`

- `DefaultTools`: canonical list of 9 tools (npm, pnpm, pip, cargo, go, gem, composer, npx, pipx)
- `Install(shimDir, tools, out)`: creates shimDir (MkdirAll), iterates tools, calls `findRealBinary` (excluding shimDir), calls platform-specific `writeShellScript`, prints PATH instructions; tools not in PATH are silently skipped with a "not found" message
- `Uninstall(shimDir)`: removes all files in shimDir; no-op if shimDir doesn't exist
- `Status(shimDir, tools, out)`: reports "shimmed"/"not shimmed" per tool based on shim file existence
- `FindRealBinary(shimDir, tool)`: exported thin wrapper over internal `findRealBinary`
- `findRealBinary`: temporarily removes shimDir from PATH via `os.Setenv`, calls `osLookPath` (injectable for tests), restores original PATH via `defer`
- `filterPathEntries`: removes a single entry from the PATH list using `filepath.Clean` for canonical comparison (handles Windows path case-insensitivity)
- `printPathInstructions`: prints bash/zsh/fish/PowerShell snippets to stdout

### `internal/shim/shim_unix.go` (`//go:build !windows`)

- `shimFilePath(shimDir, tool)`: returns `shimDir/tool` (no extension)
- `writeShellScript`: creates POSIX shell script with `exec` keyword for signal/exit-code preservation; chmod 0755 via `os.WriteFile` mode parameter

### `internal/shim/shim_windows.go` (`//go:build windows`)

- `shimFilePath(shimDir, tool)`: returns `shimDir/tool.cmd`
- `writeShellScript`: creates .cmd batch file with `\r\n` CRLF line endings throughout; real binary path is double-quoted (handles spaces in paths)

### `internal/shim/shim_test.go`

- `TestShimInstallUnix` (skipped on Windows): verifies exec keyword, real binary path embedded, executable mode bit
- `TestShimInstallWindows` (skipped on non-Windows): verifies CRLF bytes present, double-quoted real binary path
- `TestShimRealBinary` (cross-platform): verifies shimDir excluded from LookPath — fake binary in shimDir is bypassed, real binary in separate dir is found
- `TestShimUninstall`: verifies all shim files removed after Uninstall call
- `TestShimIdempotent` (skipped on Windows): verifies second Install overwrites without error, still only one shim file
- `TestShimStatus`: verifies "shimmed"/"not shimmed" reporting per tool
- `TestShimToolNotFound`: verifies Install returns no error for missing tools, "not found" in output, no shim file created
- `TestShimInstallCreatesDir` (skipped on Windows): verifies shimDir is created if it doesn't exist
- `TestShimPathInstructions` (skipped on Windows): verifies PATH instructions printed after install

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestShimRealBinary fixed for Windows cross-platform compatibility**
- **Found during:** GREEN phase test run
- **Issue:** Test created plain files without extension (e.g. "testbinshim") in temp dirs. On Windows, `exec.LookPath` requires PATHEXT-recognized extensions (.exe, .cmd, .bat) to resolve a binary. The test created files without extensions that LookPath could not find, causing `FindRealBinary` to return an error.
- **Fix:** Added conditional in `TestShimRealBinary` — uses `.cmd` extension on Windows, no extension on Unix. Binary name uses a unique string (`testbinshim204`) to avoid colliding with real PATH entries.
- **Files modified:** `internal/shim/shim_test.go`
- **Commit:** b958794 (included in GREEN phase commit)

## TDD Gate Compliance

| Gate | Commit | Status |
|------|--------|--------|
| RED (test) | 9749bfb | `test(04-04): add failing tests for shim install/uninstall/status/idempotent/tool-not-found` |
| GREEN (feat) | b958794 | `feat(04-04): implement shim layer — install/uninstall/status + OS-specific writers` |
| REFACTOR | 1fcc41f | `refactor(04-04): remove unused removed counter in Uninstall` |

## Threat Mitigations Applied

| Threat ID | Mitigation | File | Verified By |
|-----------|-----------|------|-------------|
| T-04-04-01 | `filterPathEntries` removes shimDir from PATH before LookPath — no self-referential shims | shim.go | TestShimRealBinary |
| T-04-04-02 | Real binary path double-quoted in Windows .cmd template | shim_windows.go | TestShimInstallWindows content check |
| T-04-04-03 | `Uninstall` removes all shim files | shim.go | TestShimUninstall |
| T-04-04-04 | Code comment documents not goroutine-safe; CLI-only usage is single-goroutine (accepted) | shim.go | N/A (accepted) |
| T-04-04-05 | `\r\n` CRLF line endings throughout Windows batch file | shim_windows.go | TestShimInstallWindows CRLF check |
| T-04-04-06 | Args passing via $* / %* is correct behavior (accepted) | N/A | N/A (accepted) |

## Known Stubs

None — all behavior is fully implemented and wired.

## Threat Flags

No new threat surface introduced beyond what is in the plan's threat model.

## Verification Results

| Check | Result |
|-------|--------|
| `go test ./internal/shim/... -count=1` | PASS |
| `go build ./internal/shim/...` | PASS |
| `go vet ./internal/shim/...` | PASS |
| `go build ./...` (full project) | PASS |
| `exec` keyword in shim_unix.go | VERIFIED (signal preservation) |
| `\r\n` CRLF in shim_windows.go | VERIFIED (cmd.exe compatibility) |
| `findRealBinary` excludes shimDir | VERIFIED (TestShimRealBinary) |
| Tools not in PATH silently skipped | VERIFIED (TestShimToolNotFound) |
| PATH instructions printed after install | VERIFIED (TestShimPathInstructions on Unix) |
| No gateway binding (`"0.0.0.0"`) in shim.go | VERIFIED |

## Self-Check: PASSED

| Item | Status |
|------|--------|
| internal/shim/shim.go | FOUND |
| internal/shim/shim_unix.go | FOUND |
| internal/shim/shim_windows.go | FOUND |
| internal/shim/shim_test.go | FOUND |
| .planning/phases/04-integration-surfaces/04-04-SUMMARY.md | FOUND |
| Commit 9749bfb (RED) | FOUND |
| Commit b958794 (GREEN) | FOUND |
| Commit 1fcc41f (REFACTOR) | FOUND |
