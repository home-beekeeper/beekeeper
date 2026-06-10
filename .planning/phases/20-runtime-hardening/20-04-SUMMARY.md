---
phase: 20-runtime-hardening
plan: 04
subsystem: testing
tags: [sentry, fanotify, eslogger, etw, file-write, persistence, sentry-008]

requires:
  - phase: 20-runtime-hardening
    provides: "20-03 isMonitoredDescendant + SENTRY-007 PersistWriteByPID extension point"
provides:
  - EventFileWrite EventKind
  - persistenceWritePaths + isPersistencePath + evalSENTRY008 (closes SENTRY-007 persistence-write input)
  - Linux 2nd fanotify group (FAN_REPORT_DFID_NAME, kernel >=5.9 gate) -> EventFileWrite
  - macOS write/rename subscriptions + new-file create-parse union fix
  - Windows Kernel-File branch split (15->read, 16/30/27/19->write) + corrected comment
affects: [20-05, 20-06]

tech-stack:
  added: []
  patterns:
    - "Distinct EventFileWrite kind keeps the read-clustering path uncontaminated (D-T3-write)"
    - "Separate FAN_CLASS_NOTIF|FAN_REPORT_DFID_NAME group (incompatible with the existing FAN_CLASS_CONTENT permission group) with open_by_handle_at path resolution"

key-files:
  created:
    - internal/sentry/linux/fanotify_write.go
    - internal/sentry/darwin/testdata/create_newfile_event.json
    - internal/sentry/darwin/testdata/write_event.json
    - internal/sentry/darwin/testdata/rename_event.json
  modified:
    - internal/sentry/types.go
    - internal/sentry/rules.go
    - internal/sentry/rules_test.go
    - internal/sentry/linux/daemon.go
    - internal/sentry/darwin/parser.go
    - internal/sentry/darwin/eslogger.go
    - internal/sentry/darwin/parser_test.go
    - internal/sentry/windows/parser.go
    - internal/sentry/windows/parser_test.go

key-decisions:
  - "evalSENTRY008 populates PersistWriteByPID and dedups per-path-per-session by scanning that window — closes the 20-03 SENTRY-007 extension point without a separate state field."
  - "macOS: create stays EventFileAccess (path union fixed); write+rename are the new EventFileWrite cases (plan contract: existing create/open/network kinds unaffected). The eslogger subscription lives in eslogger.go (plan named a non-existent collector.go)."
  - "Windows: kept event 12=Create(open) as EventFileAccess alongside 15=Read rather than dropping it — dropping the open-handle event would silently narrow Windows credential detection (a security regression). Only 14=Close is excluded. Acceptance (15->access, 16/30/27->write, comment fixed) is fully met."
  - "Linux write group uses StartWriteWatch (returns a closer) so daemon.go needs no new unix import; $HOME is the open_by_handle_at mount reference."

patterns-established:
  - "kernelAtLeast(major,minor) via unix.Uname for the FAN_REPORT_DFID_NAME >=5.9 gate with graceful degrade"

requirements-completed: [SENT-05, SENT-06, SENT-07, SENT-08]

duration: ~70 min
completed: 2026-06-10
---

# Phase 20 Plan 04: File-Write Persistence Ingestion + SENTRY-008 (SENT-05..08) Summary

**A distinct EventFileWrite kind feeding SENTRY-008 (persistence-write detection) plus the per-OS write ingestion that fuels it: a separate Linux fanotify group, macOS write/rename + the new-file create-parse union fix, and the corrected Windows Kernel-File read/write split.**

## Performance

- **Duration:** ~70 min
- **Tasks:** 4
- **Files modified:** 9 modified + 4 created

## Accomplishments
- `EventFileWrite` appended to the EventKind iota; `persistenceWritePaths` + `isPersistencePath` + `evalSENTRY008` (high/warn, monitored-descendant + persistence path, per-path-per-session dedup) dispatched from a new `EventFileWrite` case — and recording into `PersistWriteByPID` so SENTRY-007 now fuses on a recent persistence write (extension point from 20-03 closed, proven by test).
- Linux: new `fanotify_write.go` opens a separate `FAN_CLASS_NOTIF|FAN_REPORT_DFID_NAME` group (kernel >=5.9 gated, graceful degrade), marks persistence parent dirs with `FAN_CREATE|FAN_MOVED_TO|FAN_ONDIR`, and resolves the dir handle via `open_by_handle_at` + entry name → `EventFileWrite`; wired into the daemon via `StartWriteWatch`.
- macOS: `esloggerCreateEvent` reads the destination union (fixes dropped new-file creates); `write`+`rename` parser cases → `EventFileWrite` and added to the eslogger subscription; three fixtures + tests.
- Windows: Kernel-File branch split — `15=Read`/`12=Create` → access, `16/30/27/19` → write, `14=Close` ignored; mislabeled comment fixed; native parser tests.

## Task Commits

1. **Task 1: EventFileWrite + SENTRY-008 + 007 fusion** - `f285ecf` (feat)
2. **Task 2: Linux 2nd fanotify group (>=5.9) -> EventFileWrite** - `c5c05db` (feat)
3. **Task 3: macOS write/rename + new-file union fix** - `fc5a781` (feat)
4. **Task 4: Windows Kernel-File branch split + comment fix** - `03dcaec` (feat)

## Decisions Made
See `key-decisions` frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] Windows event 12 kept as EventFileAccess**
- **Found during:** Task 4
- **Issue:** A literal reading of "12/14 are not file-access alerts" + the existing test (12→EventFileAccess) collided; dropping event 12 would narrow Windows credential detection to reads only (regression risk).
- **Fix:** Kept 12=Create(open) AND 15=Read → EventFileAccess; excluded only 14=Close. Acceptance criteria (15→access, 16/30/27→write, comment fixed) all met.
- **Verification:** Native `go test ./internal/sentry/windows/` green; existing TestParseFileCreateEvent unchanged + new read/write/close tests pass.
- **Committed in:** `03dcaec`

**2. [Rule 1 - Bug] eslogger subscription file is eslogger.go, not collector.go**
- **Found during:** Task 3
- **Issue:** The plan listed `internal/sentry/darwin/collector.go` (does not exist); the subscription list lives in `eslogger.go` (`DefaultEsloggerEvents`).
- **Fix:** Added `write`+`rename` to `DefaultEsloggerEvents` in eslogger.go.
- **Committed in:** `fc5a781`

---

**Total deviations:** 2 auto-fixed (1 missing-critical, 1 bug)
**Impact on plan:** No scope creep; both preserve correctness/coverage.

## Issues Encountered
- **ETW write/rename field key unverified (carried forward):** the exact `EventData` key for Kernel-File write/rename templates (FileName vs FilePath vs a write-specific key) could not be verified against a live golang-etw capture in this session (no live Windows ETW capture). The code uses the same `FileName`→`FilePath` fallback as reads and is flagged for CI/live validation (CLAUDE.md Phase-7 ETW research flag). Real per-OS capture (Linux fanotify, macOS eslogger, Windows ETW) is CI-validated.

## Next Phase Readiness
- Wave 2 plan 20-02 (LlamaFirewall) remains — it has a BLOCKING human checkpoint (HF gated-model license).
- Wave 3 plan 20-05 (honesty + synthetic tests) depends on 20-03 + 20-04 (both done) — SENTRY-006/007/008 now exist for its suite-level tests.
- `go build ./...` (+ all 3 GOOS) + `go test ./internal/sentry/...` + `go vet` all green.

---
*Phase: 20-runtime-hardening*
*Completed: 2026-06-10*
