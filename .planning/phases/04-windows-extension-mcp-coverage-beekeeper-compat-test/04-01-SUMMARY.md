---
phase: 04-windows-extension-mcp-coverage-beekeeper-compat-test
plan: 01
subsystem: testing
tags: [go, windows, pollen, editor-extensions, browser-extensions, mcp, vscode-oss, windsurf]

requires:
  - phase: 03-windows-path-representation
    provides: "filepath.FromSlash discipline + Windows path representation for NDJSON records"
  - phase: 02-windows-root-resolver
    provides: "roots_windows.go build-tag pattern + env-var-guard discipline (t.Setenv isolation)"

provides:
  - "Windows Chromium-family browser-extension roots (Chrome, Chromium, Edge, Brave per-profile Extensions/ under LOCALAPPDATA)"
  - "Windows Firefox browser-extension root (APPDATA Mozilla/Firefox/Profiles parent)"
  - "VSCodium .vscode-oss/extensions segment added to extensionRootSegments and baselineHomeCandidates"
  - "Unconditional .windsurf MCP root added cross-platform (fixes Windsurf MCP mcp.json gap)"
  - "Windows MCP roots: %APPDATA%\\Claude and %APPDATA%\\cline under APPDATA-guarded case windows:"
  - "TestWindowsExtensionMCPRoots: Windows-only fixture test asserting all WEXT-01/02/03 root discovery"

affects: [04-02, 04-03, pollen-scan-consumption, beekeeper-compat-test]

tech-stack:
  added: []
  patterns:
    - "Per-variable env-var guard: if localappdata := os.Getenv(\"LOCALAPPDATA\"); localappdata != \"\" { ... } (Phase-2 pattern applied to browser roots)"
    - "Firefox Profiles parent only: add Profiles dir; walker recurses into per-profile subdirs (IsFirefoxExtensionsJSON matches per-profile file)"
    - "Dual VSCodium segments: keep .vscodium/extensions for upstream parity; add .vscode-oss/extensions for PRD §8.2 compliance"
    - "Unconditional cross-platform MCP root pattern: add .windsurf before switch runtime.GOOS so all OSes benefit"

key-files:
  created:
    - "../pollen/cmd/pollen/roots_windows_test.go (TestWindowsExtensionMCPRoots added)"
  modified:
    - "../pollen/cmd/pollen/roots.go"
    - "../pollen/internal/ecosystem/editorext/editorext.go"

key-decisions:
  - "Fill case windows: chromiumBases with LOCALAPPDATA-guarded Chrome/Chromium/Edge/Brave paths (not the home parameter)"
  - "Fill case windows: Firefox with APPDATA-guarded Mozilla/Firefox/Profiles parent path"
  - "Add .vscode-oss/extensions to both extensionRootSegments (editorext.go) and baselineHomeCandidates (roots.go); keep .vscodium for upstream parity"
  - "Add .windsurf MCP root unconditionally (cross-platform) before switch runtime.GOOS — .codeium/windsurf does not cover .windsurf"
  - "Windows MCP switch adds %APPDATA%\\Claude and %APPDATA%\\cline only; Cursor/.cursor, Windsurf/.windsurf, Gemini/.gemini already covered by cross-platform roots"
  - "Test isolation: t.Setenv(USERPROFILE/APPDATA/LOCALAPPDATA/ProgramFiles) — never HOME — per Phase-2 Pitfall 5 prevention"

patterns-established:
  - "Pattern: Fill existing case windows: skeletons rather than adding new _windows.go files when the switch already exists"
  - "Pattern: browserExtensionCandidateRoots Windows — use os.Getenv(LOCALAPPDATA) not home param for Chromium; os.Getenv(APPDATA) for Firefox"
  - "Pattern: MCP root gaps — check both .codeium/windsurf (IDE extension root) and .windsurf (MCP config root) are distinct"

requirements-completed: [WEXT-01, WEXT-02, WEXT-03]

duration: 25min
completed: 2026-06-02
---

# Phase 04 Plan 01: Windows Extension & MCP Coverage Summary

**Windows Chromium/Firefox browser-extension roots + VSCodium .vscode-oss segment + Windsurf MCP gap fix + all five MCP hosts, verified by TestWindowsExtensionMCPRoots**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-06-02T00:00:00Z
- **Completed:** 2026-06-02
- **Tasks:** 3 (all completed)
- **Files modified:** 3 (pollen repo)

## Accomplishments

- Filled the two empty `case "windows":` skeletons in `browserExtensionCandidateRoots` (WEXT-02): Chrome, Chromium, Edge, Brave under `%LOCALAPPDATA%` per-profile, and Firefox `%APPDATA%\Mozilla\Firefox\Profiles` parent
- Added `.vscode-oss/extensions` to `extensionRootSegments` (editorext.go) and `baselineHomeCandidates` (roots.go) alongside `.vscodium/extensions` — both VSCodium install variants now resolve (WEXT-01)
- Fixed the `.windsurf` MCP gap: added `add(filepath.Join(home, ".windsurf"), model.RootKindMCPConfig)` unconditionally before the OS switch so `%USERPROFILE%\.windsurf\mcp.json` is reachable on all platforms (WEXT-03)
- Added `case "windows":` MCP block in `baselineHomeCandidates` with APPDATA-guarded `%APPDATA%\Claude` and `%APPDATA%\cline` roots (WEXT-03)
- Wrote `TestWindowsExtensionMCPRoots` — Windows-only fixture test (//go:build windows, zero t.Skip) asserting all five editor roots, Chromium+Firefox browser roots, and all five MCP hosts

## Task Commits (all in pollen repo `../pollen`)

1. **Task 1: Fill Windows browser-extension skeletons** - `dbe4c52` (feat)
2. **Task 2: Add Windows MCP roots + .windsurf unconditional + .vscode-oss segment** - `77ad510` (feat)
3. **Task 3: TestWindowsExtensionMCPRoots** - `94fb651` (test)

## Files Created/Modified

- `../pollen/cmd/pollen/roots.go` — Filled Windows chromiumBases + Firefox switches; added .vscode-oss/extensions to editor seg list; added .windsurf unconditional MCP root; added case "windows": APPDATA MCP block
- `../pollen/internal/ecosystem/editorext/editorext.go` — Added .vscode-oss/extensions to extensionRootSegments (hostFromExtRoot unchanged)
- `../pollen/cmd/pollen/roots_windows_test.go` — Added TestWindowsExtensionMCPRoots (new test function)

## Decisions Made

- Add BOTH `.vscodium/extensions` (upstream parity) and `.vscode-oss/extensions` (PRD §8.2 locked path) to both lists — different VSCodium installation variants use different dirs
- `.windsurf` MCP root added unconditionally (benefits all OSes); `.codeium/windsurf` is the Windsurf IDE extension root, NOT the same directory as `.windsurf` where `mcp.json` lives
- Windows MCP case only needs `%APPDATA%\Claude` and `%APPDATA%\cline` — Cursor/Windsurf/Gemini already covered by cross-platform `os.UserHomeDir()` dotfile roots
- All paths use `filepath.Join + os.Getenv` only; no hardcoded backslash strings (T-04-02 mitigation)
- `hostFromExtRoot` in editorext.go left unchanged — `.vscode-oss` root resolves to host "vscode" (acceptable; the extension record is still emitted correctly)

## Deviations from Plan

None — plan executed exactly as written. All three tasks completed per spec. Both Firefox and Chromium Windows patterns matched research examples verbatim. `IsFirefoxExtensionsJSON` confirmed unchanged (already handles Windows via `filepath.ToSlash`).

## Issues Encountered

None. Build, vet, and full test suite (`go test ./...`) all green on first attempt.

## Threat Surface Scan

No new security-relevant surface beyond what the plan's threat model covers. The two threat mitigations from the plan were applied:
- T-04-01 (junction point information disclosure): accepted per Phase-2 T-02-02 precedent; `filterExistingRoots` uses `os.Stat` (read-only)
- T-04-02 (unset LOCALAPPDATA/APPDATA relative-path leak): mitigated via per-variable env-var guards on all new APPDATA/LOCALAPPDATA references

## Known Stubs

None — all paths are wired to real `os.Getenv` calls; no placeholder text or hardcoded empty values.

## Next Phase Readiness

- WEXT-01/02/03 all satisfied: pollen scan on Windows now enumerates all five editor-extension dirs, all four Chromium-family browser extension profiles + Firefox Profiles parent, and all five MCP host-config directories
- Ready for Phase 04 Plan 02 (BKINT-01 / PTEST-04 beekeeper compatibility test)

---
*Phase: 04-windows-extension-mcp-coverage-beekeeper-compat-test*
*Completed: 2026-06-02*
