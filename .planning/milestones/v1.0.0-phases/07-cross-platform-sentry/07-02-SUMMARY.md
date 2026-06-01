---
phase: 07-cross-platform-sentry
plan: 02
status: complete
commit: 7030ce5
subsystem: sentry/windows
tags: [etw, windows, sentry, ingestion]
dependency_graph:
  requires: [internal/sentry/types.go]
  provides: [internal/sentry/windows]
  affects: []
tech_stack:
  added: [github.com/tekert/golang-etw v0.6.2]
  patterns: [adapter-for-testability, atomic-counter, callback-based-etw]
key_files:
  created:
    - internal/sentry/windows/etw.go
    - internal/sentry/windows/parser.go
    - internal/sentry/windows/conflict.go
    - internal/sentry/windows/etw_test.go
    - internal/sentry/windows/parser_test.go
    - internal/sentry/windows/conflict_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - Used etwEventSummary adapter struct so parser tests do not depend on unexported etw.Event fields
  - Used etw.ERROR_ALREADY_EXISTS (syscall.Errno(183)) exported by tekert library rather than windows.ERROR_ALREADY_EXISTS
  - Consumer.EventCallback assigned directly (not via ProcessEvents method — no such method exists)
  - c.Start() is non-blocking; blocked with <-ctx.Done() + c.Stop() + c.Wait() for context-cancellable lifecycle
metrics:
  duration: ~15min
  completed: 2026-05-28
  tasks: 1
  files: 8
---

# Phase 07 Plan 02: Windows ETW Ingestion Layer Summary

Windows ETW ingestion via tekert/golang-etw v0.6.2 (no CGO) mapping Kernel-Process, Kernel-File, and Kernel-Network events to sentry.SentryEvent with atomic drop counter and NT Kernel Logger conflict probe.

## Completed

- **go.mod**: `github.com/tekert/golang-etw v0.6.2` added (not indirect; no CGO; locked per CLAUDE.md)
- **etw.go**: `ProviderGUIDs` map (4 GUIDs), `EventsLost` atomic uint64, `StartETWConsumer` (ctx-cancellable, callback-based), `DefaultKernelProviders`
- **parser.go**: `parseETWEvent` adapter + `parseETWEventSummary` core logic; normalizes Kernel-Process (event 1), Kernel-File (events 12/14/15), Kernel-Network (events 10/11/12); `ErrUnknownEvent`; PID-0 guard; `toUint32`/`toUint16` helpers; `normalizeGUID`
- **conflict.go**: `ProbeKernelLoggerConflict` calls `sess.Start()` on NT Kernel Logger, returns `(true, nil)` on ERROR_ALREADY_EXISTS (183); `ConflictMessage` for status output
- **Tests**: 18 pass, 1 skipped (NT Kernel Logger EDR-protected on dev machine — expected behavior)

## API deviations from RESEARCH.md

| Assumed | Actual |
|---------|--------|
| `c.ProcessEvents(func(*etw.Event))` method | No such method; set `c.EventCallback = func(*etw.Event) error` directly |
| `e.System.Provider.Guid` is a string | `e.System.Provider.Guid` is type `etw.GUID` with `.String()` returning `{lowercase-guid}` |
| `etw.NewConsumer` blocks | `c.Start()` is non-blocking; must call `c.Wait()` after `c.Stop()` |
| `windows.ERROR_ALREADY_EXISTS` for conflict check | `etw.ERROR_ALREADY_EXISTS` (= `syscall.Errno(183)`) is exported by the library |

## Verification

- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test -tags windows ./internal/sentry/windows/... -count=1 -v` — 18 PASS, 1 SKIP

## Requirements satisfied

- **SWIN-02**: ETW ingestion via `github.com/tekert/golang-etw` (no CGO); `parseETWEvent` normalizes Kernel-Process/File/Network events to `sentry.SentryEvent`
- **SWIN-03**: `ProbeKernelLoggerConflict` detects NT Kernel Logger conflict via `ERROR_ALREADY_EXISTS` (errno 183); `ConflictMessage` for protect status
- **SWIN-04**: `EventsLost` atomic counter incremented on full-channel drop; accessible via `beekeeper diag` (wired in 07-04)

## Self-Check: PASSED

- `internal/sentry/windows/etw.go` — exists
- `internal/sentry/windows/parser.go` — exists
- `internal/sentry/windows/conflict.go` — exists
- commit `7030ce5` — verified in git log
