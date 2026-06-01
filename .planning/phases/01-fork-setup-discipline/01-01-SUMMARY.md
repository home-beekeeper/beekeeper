---
phase: "01-fork-setup-discipline"
plan: "01"
subsystem: "pollen-fork"
tags: ["fork", "module-rename", "trademark", "windows-build", "go-embed"]
dependency_graph:
  requires: []
  provides: ["pollen-repo-at-../pollen", "module-github.com/bantuson/pollen", "cmd/pollen-binary"]
  affects: ["plans 01-02 through 01-05 (all depend on pollen repo existing)"]
tech_stack:
  added: []
  patterns: ["git-mv for history-preserving rename", "PowerShell bulk module-path rewrite", "t.Skip with structured Phase 2 reason comments"]
key_files:
  created:
    - "../pollen/ (entire repo)"
    - "../pollen/go.mod (module github.com/bantuson/pollen)"
    - "../pollen/cmd/pollen/main.go"
    - "../pollen/cmd/pollen/selftest.go"
    - "../pollen/cmd/pollen/version.go"
    - "../pollen/cmd/pollen/roots.go"
    - "../pollen/cmd/pollen/main_test.go"
    - "../pollen/internal/** (38 files, module path rewritten)"
  modified: []
decisions:
  - "No non-test files needed Windows build fixes (GOOS=windows go build ./... passed clean)"
  - "6 Unix-specific test functions in cmd/pollen/main_test.go got t.Skip with 'Phase 2 (v0.1.1-pollen.2)' structured reasons"
  - "scanner_test.go TestEndToEndScan: hardcoded /proj/ /dup/ path separators replaced with filepath.Separator to pass on Windows"
  - "BUMBLEBEE_USERS_DIR and BUMBLEBEE_TEST_DEVICE_ID env var names renamed to POLLEN_ prefix (FORK-04)"
  - "upstream remote named 'upstream' at time of clone (origin removed); plan 05 will bind origin to github.com/bantuson/pollen"
  - "pollen.exe build artifact committed accidentally, then removed in follow-up commit; .gitignore updated"
metrics:
  duration: "~20 minutes"
  completed: "2026-06-01"
  tasks_completed: 3
  files_changed: 42
---

# Phase 01 Plan 01: Fork Setup & Module Rename Summary

Pollen repo created at `../pollen` as a bounded Apache-2.0 fork of `perplexityai/bumblebee` v0.1.1. Module path rewritten to `github.com/bantuson/pollen`, `cmd/bumblebee` renamed to `cmd/pollen`, trademark strings fixed, host build and Windows cross-compile green, selftest emits 3 findings, full test suite passes.

## One-liner

Apache-2.0 fork of bumblebee v0.1.1 with module path rewrite to github.com/bantuson/pollen, cmd/pollen rename, and Windows-clean test suite (6 root-resolver test skips with Phase 2 structured reasons).

## Tasks Completed

| Task | Name | Status | Pollen Commit |
|------|------|--------|---------------|
| 1 | Clone upstream, init repo, rewrite module path | Done | `1fdd433` |
| 2 | Rename cmd/bumblebee → cmd/pollen, trademark fixes | Done | `18b0c70`, `9de5d61`, `c3d10a4` |
| 3 | Build, cross-compile, selftest, test suite | Done | `49f97a3` |

## Key Facts

### Pinned Upstream SHA

`c24089804ee66ece4bec6f14638cb98985389cdb` (tag v0.1.1, 2026-05-22)

Preserved in pollen git history: `git rev-list --all | grep c24089804ee66ece4bec6f14638cb98985389cdb` returns the commit.

### go.sum State

No `go.sum` file exists — upstream bumblebee v0.1.1 has zero external dependencies (confirmed). `go mod tidy` ran clean and produced no go.sum. `go mod verify` passes.

### Open Question 1 Resolution: Windows Build Fixes

**Non-test files:** ZERO non-test files needed Windows build fixes. `GOOS=windows GOARCH=amd64 go build ./...` passed clean on the first attempt.

**Test files modified for Windows:** Two test files received Windows-specific fixes:

1. `cmd/pollen/main_test.go` — 6 test functions with Unix `HOME` env var dependencies or Unix-style broad-home path detection (`isBroadHomeRoot`) that failed on Windows. Each received a structured `t.Skip` with reason:
   ```
   t.Skip("... reason ...; Windows root-resolver tests arrive in Phase 2 (v0.1.1-pollen.2)")
   ```
   Functions skipped on Windows:
   - `TestIsBroadHomeRoot`
   - `TestResolveRootsProjectIncludesCodeDir`
   - `TestResolveRootsBaselineIncludesUserLocalPython`
   - `TestResolveRootsBaselineRefusesBroadHome`
   - `TestResolveRootsProjectRefusesBroadHome`
   - `TestResolveRootsDeepAllowsBroadHome`

2. `internal/scanner/scanner_test.go` — `TestEndToEndScan` used hardcoded Unix path separator `/proj/` and `/dup/` in string comparisons. Fixed to use `filepath.Separator` — this is NOT a skip; the test now passes on all platforms (Rule 1 bug fix).

### Verification Results

| Check | Result |
|-------|--------|
| `go.mod` first line | `module github.com/bantuson/pollen` |
| Stale `perplexityai/bumblebee` imports in .go files | None |
| `git remote get-url upstream` | `https://github.com/perplexityai/bumblebee.git` |
| `cmd/pollen/` exists | Yes |
| `cmd/bumblebee/` exists | No (removed by git mv) |
| `selftest/fixtures/{npm,pypi,mcp}-fixture` intact | Yes |
| `pollen-selftest-` temp prefix | Yes (selftest.go line 51) |
| `bumblebee-selftest-` prefix | Gone |
| `fileDefault = "0.1.1-pollen.1"` | Yes (version.go) |
| FORK-04 trademark grep gate | CLEAN — `grep -ri "bumblebee" cmd/ --include="*.go"` returns nothing |
| `go build -trimpath -buildvcs=false ./cmd/pollen` | Exit 0 |
| `GOOS=windows GOARCH=amd64 go build ./...` | Exit 0 |
| `./dist/pollen selftest` | `selftest OK (3 findings)` Exit 0 |
| `go test -count=1 ./...` | All 19 packages PASS |

### Pollen Repo Commit Hashes

| Commit | Message |
|--------|---------|
| `c240898` | initial public release (v0.1.1) — upstream pinned commit |
| `1fdd433` | fork: import upstream bumblebee @ c24089804ee... ; rewrite module path -> github.com/bantuson/pollen |
| `18b0c70` | rename: cmd/bumblebee -> cmd/pollen; trademark fixes (selftest temp prefix, help text, version default) [FORK-04] |
| `9de5d61` | chore: remove accidentally committed pollen.exe build artifact |
| `c3d10a4` | fix: rename BUMBLEBEE_ env var test overrides to POLLEN_ [FORK-04] |
| `49f97a3` | build: cross-compile clean for windows; tag darwin/linux-only tests [FORK-01] |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Windows path separator in TestEndToEndScan**
- **Found during:** Task 3
- **Issue:** `strings.Contains(r.SourceFile, "/proj/")` failed on Windows because `filepath.Join` produces `\proj\` not `/proj/`
- **Fix:** Replaced hardcoded `/proj/` and `/dup/` with `filepath.Separator` + `"proj"` + `filepath.Separator` pattern
- **Files modified:** `internal/scanner/scanner_test.go`
- **Commit:** `49f97a3`

**2. [Rule 2 - Missing Critical] Added POLLEN_ env var namespace for test injection overrides**
- **Found during:** Task 2
- **Issue:** Test-injection env var names used `BUMBLEBEE_USERS_DIR` and `BUMBLEBEE_TEST_DEVICE_ID` — trademark discipline violation (FORK-04)
- **Fix:** Renamed to `POLLEN_USERS_DIR` and `POLLEN_TEST_DEVICE_ID`
- **Files modified:** `cmd/pollen/roots.go`, `cmd/pollen/main_test.go`
- **Commit:** `c3d10a4`

**3. [Rule 1 - Bug] Removed accidentally committed pollen.exe build artifact**
- **Found during:** Task 2 commit staging
- **Issue:** `pollen.exe` build artifact was committed with the rename commit before .gitignore was updated
- **Fix:** `git rm --cached pollen.exe` + added `pollen.exe` to `.gitignore`
- **Files modified:** `.gitignore`
- **Commit:** `9de5d61`

**4. [Rule 3 - Blocking] Added Windows t.Skip for 6 Unix-root-resolver tests**
- **Found during:** Task 3
- **Issue:** 6 test functions in `cmd/pollen/main_test.go` failed on Windows due to Unix `HOME` env override not affecting `os.UserHomeDir()` on Windows, and Unix-style path constants (`/Users`, `/home`, `/root`) in `isBroadHomeRoot`
- **Fix:** Added `t.Skip` with structured Phase 2 reason comment per plan instructions
- **Files modified:** `cmd/pollen/main_test.go`
- **Commit:** `49f97a3`

## Known Stubs

None — this plan contains no stub or placeholder code. The pollen binary is fully functional for Unix/macOS. Windows root resolution is intentionally deferred to Phase 2 (v0.1.1-pollen.2) with explicit t.Skip labels, not silent stubs.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced. The pollen binary is a read-only CLI scanner. The upstream remote (`https://github.com/perplexityai/bumblebee.git`) is configured but no code is fetched from it at runtime.

## Self-Check: PASSED

Files exist:
- `C:/Users/Bantu/mzansi-agentive/pollen/go.mod` — FOUND
- `C:/Users/Bantu/mzansi-agentive/pollen/cmd/pollen/main.go` — FOUND
- `C:/Users/Bantu/mzansi-agentive/pollen/cmd/pollen/selftest.go` — FOUND
- `C:/Users/Bantu/mzansi-agentive/pollen/cmd/pollen/version.go` — FOUND
- `C:/Users/Bantu/mzansi-agentive/pollen/cmd/pollen/selftest/catalog.json` — FOUND

Pollen commits verified:
- `1fdd433` — FOUND (fork commit)
- `18b0c70` — FOUND (rename commit)
- `49f97a3` — FOUND (build fix commit)
