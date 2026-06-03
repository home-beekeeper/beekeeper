---
phase: 05-contribution-back-milestone-close
plan: "01"
subsystem: testing
tags: [windows, sentry, etw, rule-engine, honeypot, filepath, tdd]

# Dependency graph
requires:
  - phase: 07-sentry-os-daemons
    provides: internal/sentry EvaluateEvent, SentryEvent, SENTRY-005 exfil-fusion rule
  - phase: 04-windows-extension-mcp-coverage
    provides: Windows ETW daemon, internal/sentry/windows package structure
provides:
  - isSensitivePath normalized with filepath.ToSlash before substring matching
  - TestIsSensitivePathWindows all-OS regression for backslash credential paths
  - TestHoneypotExfilFusion Windows E2E honeypot proving SENTRY-005 fires on synthetic credential exfil scenario
  - exeBaseName helper stripping .exe suffix for cross-platform editorExes/credentialCLIs lookups
affects: [sentry, windows-ci, ptest-05, milestone-close]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "filepath.ToSlash normalization before substring matching for cross-platform path comparison"
    - "exeBaseName() strips .exe suffix so editorExes map matches Windows ETW-emitted cursor.exe paths"
    - "Synthetic SentryEvent structs via EvaluateEvent for OS-gated rule-engine tests without live daemons"
    - "RFC 5737 TEST-NET-3 (203.0.113.0/24) for test network destinations — never dialled"

key-files:
  created:
    - internal/sentry/windows/honeypot_test.go
  modified:
    - internal/sentry/rules.go
    - internal/sentry/rules_test.go

key-decisions:
  - "isSensitivePath: filepath.ToSlash normalises backslash before defaultSensitivePaths loop — forward-slash canonical form kept, ETW backslash paths now match"
  - "exeBaseName() strips .exe suffix (case-insensitive) so cursor.exe maps to editorExes['cursor'] on Windows"
  - "isCredentialCLI updated to use exeBaseName() — consistent Windows exe-suffix handling for both editor and CLI lookups"
  - "Honeypot test uses string-literal credential path only — no os.WriteFile, no net.Dial; RFC 5737 IP never dialled"
  - "TestHoneypotExfilFusion lives in internal/sentry/windows/ under //go:build windows — runs on dev box and windows-latest CI"

patterns-established:
  - "Windows ETW path normalization: always filepath.ToSlash before any substring/comparison operation"
  - "Exe base-name lookup: always exeBaseName() (strips .exe) not filepath.Base() directly"
  - "OS-gated synthetic rule-engine tests: //go:build windows + EvaluateEvent direct call, no daemon"

requirements-completed: [PTEST-05]

# Metrics
duration: 25min
completed: 2026-06-03
---

# Phase 05 Plan 01: Windows Sentry Honeypot (PTEST-05) Summary

**Windows credential-path evasion fixed with filepath.ToSlash + .exe-suffix normalization; TestHoneypotExfilFusion proves SENTRY-005 fires on synthetic backslash .aws/credentials exfil scenario**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-06-03T10:00:00Z
- **Completed:** 2026-06-03T10:25:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Fixed the Windows credential-path evasion bug: `isSensitivePath` now calls `filepath.ToSlash` before substring matching so ETW-emitted backslash paths like `C:\Users\x\.aws\credentials` match the forward-slash entries in `defaultSensitivePaths`. On Unix the call is a no-op.
- Added `exeBaseName()` helper that strips `.exe` suffix (case-insensitive) from executable base names; `isEditorDescendant` and `isCredentialCLI` now use it so Windows ETW paths like `cursor.exe` correctly match `editorExes["cursor"]`.
- Created `internal/sentry/windows/honeypot_test.go` (`//go:build windows`, `TestHoneypotExfilFusion`) — synthetic cursor.exe process tree + backslash `.aws\credentials` read + RFC 5737 outbound connection + recently installed extension asserts SENTRY-005 fires via `EvaluateEvent` directly with no live ETW daemon, no real file writes, and no real network.

## Task Commits

Each task committed atomically:

1. **Task 1 (RED): TestIsSensitivePathWindows regression** - `7fb76d6` (test)
2. **Task 1 (GREEN): isSensitivePath filepath.ToSlash fix** - `5ccaf66` (fix)
3. **Task 2: Windows honeypot + exeBaseName fix** - `10ab2ba` (feat)

## Files Created/Modified

- `internal/sentry/rules.go` — `filepath.ToSlash` normalization in `isSensitivePath`; new `exeBaseName()` helper; `isEditorDescendant` and `isCredentialCLI` updated to call `exeBaseName`
- `internal/sentry/rules_test.go` — `TestIsSensitivePathWindows` table-driven all-OS regression (4 cases)
- `internal/sentry/windows/honeypot_test.go` — `TestHoneypotExfilFusion` Windows E2E honeypot (synthetic, `//go:build windows`)

## Decisions Made

- Kept `defaultSensitivePaths` in forward-slash canonical form; normalization happens at match time via `filepath.ToSlash` — no data duplication, single source of truth.
- Added `exeBaseName()` as a new internal helper rather than patching `filepath.Base` call sites ad hoc — consistent, testable, documented.
- `isCredentialCLI` updated alongside `isEditorDescendant` for complete Windows exe-suffix coverage (not just the honeypot scenario).
- Honeypot test uses `203.0.113.1` (RFC 5737 TEST-NET-3) — never a real routable address and never dialled; credentials path is a string literal only.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] isEditorDescendant silent miss on Windows .exe-suffixed paths**
- **Found during:** Task 2 (TestHoneypotExfilFusion)
- **Issue:** `filepath.Base("C:\Program Files\cursor\cursor.exe")` returns `cursor.exe` on Windows; `editorExes["cursor.exe"]` is false — so `isEditorDescendant` returned false for the Windows editor process tree, causing SENTRY-005 to never fire.
- **Fix:** Added `exeBaseName(exe string) string` that strips `.exe` suffix (case-insensitive); updated `isEditorDescendant` and `isCredentialCLI` to call `exeBaseName` instead of `filepath.Base`.
- **Files modified:** `internal/sentry/rules.go`
- **Verification:** `go test ./internal/sentry/...` all pass; `TestHoneypotExfilFusion` passes; `go vet ./internal/sentry/...` clean.
- **Committed in:** `10ab2ba` (Task 2 commit, included alongside honeypot test)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug discovered during Task 2 execution)
**Impact on plan:** Auto-fix necessary for SENTRY-005 to fire on real Windows ETW events. The `.exe`-suffix bug mirrors the forward-slash bug from Task 1 — same root cause (ETW emits Windows-native paths; rule engine was Unix-only hardened). Both fixed in this plan.

## Issues Encountered

None beyond the Rule 1 auto-fix above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- PTEST-05 complete: `TestHoneypotExfilFusion` passes on Windows dev box; will pass on `windows-latest` CI.
- Both Windows path normalization bugs (forward-slash credential paths, .exe-suffix editor detection) are fixed and regression-locked.
- Ready to proceed with remaining Phase 05 plans (SYNC-01 UPSTREAM.md, BKINT-02 Pollen pin, SDEF-01 pollen-self catalog, D-4 signed release batch).

## Known Stubs

None — no stubs or placeholder values introduced.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced. The threat mitigations from the plan's threat model (T-05-01, T-05-02) are both addressed.

---
*Phase: 05-contribution-back-milestone-close*
*Completed: 2026-06-03*
