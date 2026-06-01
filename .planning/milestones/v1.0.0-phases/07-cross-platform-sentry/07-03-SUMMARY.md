---
phase: 7
plan: 3
subsystem: sentry/darwin
tags: [sentry, darwin, macos, eslogger, launchd, ipc, daemon, cli]
dependency_graph:
  requires:
    - 07-01 (darwin parser + eslogger + launchd)
    - 05-03 (sentry correlation engine)
    - 05-04 (IPC server + proto)
  provides:
    - RunDaemon: darwin Sentry daemon main loop
    - protect_darwin: macOS protect install/uninstall/status CLI
    - runSentryDaemon/Rules*: darwin-tagged sentry subcommands
  affects:
    - internal/ipc/server.go (verifyPeerUID extracted to platform files)
    - cmd/beekeeper/protect_other.go (build tag narrowed)
tech_stack:
  added: []
  patterns:
    - eslogger subprocess drain goroutine pattern (mirrors linux fanotify reader)
    - darwin LOCAL_PEERCRED via GetsockoptXucred for IPC peer auth
    - launchd plist install (WritePlist + LaunchctlLoad)
    - correlationEngineLoop mirrors linux/daemon.go exactly
key_files:
  created:
    - internal/sentry/darwin/daemon.go
    - internal/sentry/darwin/daemon_test.go
    - cmd/beekeeper/protect_darwin.go
    - internal/ipc/peer_linux.go
    - internal/ipc/peer_darwin.go
  modified:
    - internal/ipc/server.go (verifyPeerUID extracted)
    - cmd/beekeeper/protect_other.go (build tag narrowed to !linux && !darwin)
decisions:
  - alertToAuditRecord copied field-for-field from linux/daemon.go; audit schema is cross-platform invariant
  - verifyPeerUID split into peer_linux.go (SO_PEERCRED) + peer_darwin.go (LOCAL_PEERCRED/GetsockoptXucred); ipc/server.go no longer imports golang.org/x/sys/unix directly
  - copyFileDarwin named with Darwin suffix to avoid duplicate symbol with protect_linux.go's copyFile (both compiled into the same package on respective platforms)
  - CoverageGapNotes() called in install, status-fallback, and status-success paths (all three output surfaces for SMAC-04)
metrics:
  duration: ~15min
  completed: "2026-05-28"
  tasks_completed: 2
  files_created: 5
  files_modified: 2
---

# Phase 7 Plan 3: macOS Sentry Daemon + Launchd CLI Summary

**One-liner:** macOS Sentry daemon supervising eslogger subprocess via RunDaemon, launchd install/uninstall/status CLI with CoverageGapNotes (SMAC-01, SMAC-03, SMAC-04), and darwin-specific IPC peer auth via LOCAL_PEERCRED/GetsockoptXucred.

## What Was Built

### Task 1: internal/sentry/darwin/daemon.go

`RunDaemon` is the macOS Sentry daemon entry point:

1. Opens audit writer via `audit.NewWriter`
2. Starts eslogger subprocess (`EsloggerCommand(ctx, DefaultEsloggerEvents)`) with stdout pipe
3. Drains stdout via `go drainEslogger(stdout, events)` goroutine
4. Creates IPC server at `~/.beekeeper/sentry.sock` via `ipc.NewServer`
5. Loads baseline path and starts `correlationEngineLoop` goroutine
6. Starts IPC server goroutine (`ipcSrv.Serve`)
7. Blocks on `select{ctx.Done / drainDone}`

`handleIPCConn` mirrors linux/daemon.go exactly:
- `CmdStatusRequest` → `StatusResponse{TierReason: "macOS eslogger (no entitlement)", EventsDropped from darwin.EventsDropped}`
- `CmdRulesListRequest/Enable/Disable` → identical to linux pattern

`correlationEngineLoop` mirrors linux/daemon.go:
- `sentry.LoadBaseline` → 7-day default on first run
- `sentry.NewRuleState()` + process tree map
- `sentry.EvaluateEvent` (SMAC-03: shared correlation engine)
- GC stale tree entries at 10-minute cutoff
- `alertToAuditRecord` + `auditWriter.Write`

`alertToAuditRecord` copied field-for-field from linux/daemon.go:
- `sentry_alert` (enforcement) vs `sentry_alert_baseline` (baseline mode)
- `block` for non-baseline + QuarantineRec=true, `warn` otherwise
- `CatalogMatches: []audit.CatalogProvenance{}` — non-nil empty (CTLG-09)

`daemon_test.go` (4 tests, darwin build tag):
- `TestAlertToAuditRecordEnforcement`: critical + !baseline → sentry_alert + block
- `TestAlertToAuditRecordBaseline`: BaselineMode=true → sentry_alert_baseline + warn
- `TestAlertToAuditRecordPreservesParentChain`: parent chain passthrough
- `TestDaemonStateInitialRules`: 5 rules all enabled at init

### Task 2: cmd/beekeeper/protect_darwin.go

- `runProtectInstall`: root check → `darwin.WritePlist` → `darwin.LaunchctlLoad` → `waitForSocket` → prints `CoverageGapNotes()` (SMAC-04)
- `runProtectUninstall`: `LaunchctlUnload` → remove plist + socket
- `runProtectStatus`: IPC connect → `StatusResponse` display + `CoverageGapNotes()` in both connected and fallback paths (SMAC-04)
- `runSentryDaemon`: `darwin.RunDaemon`
- `runSentryRulesList/Enable/Disable`: identical pattern to protect_linux.go
- `waitForSocket`: local 200ms-poll helper (cannot import linux-tagged code)
- `copyFileDarwin`: local copy helper (avoids duplicate symbol with linux's `copyFile`)

### Task 2b: cmd/beekeeper/protect_other.go

Build tag narrowed from `!linux` to `!linux && !darwin` so darwin now uses the real implementation.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed pre-existing cross-compile failure in ipc/server.go**
- **Found during:** Task 1 verification (GOOS=darwin go build)
- **Issue:** `ipc/server.go` (build tag `linux || darwin`) used `unix.GetsockoptUcred` and `unix.SO_PEERCRED` which are Linux-only symbols not present in `golang.org/x/sys/unix` on darwin
- **Fix:** Extracted `verifyPeerUID` from `server.go` into two platform files: `peer_linux.go` (SO_PEERCRED, original implementation) and `peer_darwin.go` (LOCAL_PEERCRED/GetsockoptXucred — darwin-native equivalent returning Xucred.Uid)
- **Files modified:** `internal/ipc/server.go`, new `internal/ipc/peer_linux.go`, new `internal/ipc/peer_darwin.go`
- **Commit:** 61ae8e3

**2. [Rule 1 - Bug] Restored accidentally deleted windows sentry files**
- **Found during:** `git status` review pre-commit
- **Issue:** `internal/sentry/windows/*.go` files from commit 7030ce5 were missing from working tree (deleted during previous session's context exhaustion)
- **Fix:** `git checkout -- internal/sentry/windows/` to restore tracked files
- **Impact:** No code change; files were tracked in git, just missing from disk

## Verification Results

```
go build ./...                         — PASS (Windows, darwin files excluded)
GOOS=darwin go build ./...             — PASS
GOOS=linux go build ./...              — PASS
GOOS=darwin go vet ./cmd/beekeeper/... — PASS
GOOS=darwin go vet ./internal/sentry/darwin/... — PASS
GOOS=darwin go vet ./internal/ipc/... — PASS
go test ./...                          — PASS (20 packages, 0 failures)
```

Acceptance checks:
- `grep -c "func RunDaemon" internal/sentry/darwin/daemon.go` = 1 ✓
- `grep -c "sentry.EvaluateEvent" internal/sentry/darwin/daemon.go` = 1 ✓ (SMAC-03)
- `grep -c "sentry.LoadBaseline" internal/sentry/darwin/daemon.go` = 2 ✓
- `grep -c "sentry_alert_baseline" internal/sentry/darwin/daemon.go` = 2 ✓
- `grep -c "CoverageGapNotes" cmd/beekeeper/protect_darwin.go` = 3 ✓ (SMAC-04)
- `grep "^//go:build" cmd/beekeeper/protect_other.go` = `!linux && !darwin` ✓

## Requirements Satisfied

- **SMAC-01**: macOS Sentry daemon loop (`RunDaemon`) implemented with eslogger subprocess management and IPC server
- **SMAC-03**: Shared `sentry.EvaluateEvent` correlation engine used (not duplicated); `sentry.LoadBaseline` for 7-day learning mode default
- **SMAC-04**: `CoverageGapNotes()` printed in install, status (connected), and status (fallback/launchctl) paths — all three output surfaces

## Self-Check: PASSED

Files created/verified:
- `internal/sentry/darwin/daemon.go` — FOUND
- `internal/sentry/darwin/daemon_test.go` — FOUND
- `cmd/beekeeper/protect_darwin.go` — FOUND
- `internal/ipc/peer_linux.go` — FOUND
- `internal/ipc/peer_darwin.go` — FOUND

Commit 61ae8e3 — FOUND in git log
