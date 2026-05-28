---
status: complete
phase: 07-cross-platform-sentry
source:
  - 07-01-SUMMARY.md
  - 07-02-SUMMARY.md
  - 07-03-SUMMARY.md
  - 07-04-SUMMARY.md
  - 07-05-SUMMARY.md
started: "2026-05-28T12:30:00.000Z"
updated: "2026-05-28T12:35:00.000Z"
mode: automated
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: |
  `go build ./...` exits 0 on Windows; `beekeeper version` prints version info without error.
result: pass
automated: true
evidence: "go build ./... exits 0; beekeeper version → version: dev / commit: none / date: unknown"

### 2. Full test suite (21 packages)
expected: |
  `go test ./...` runs all packages and reports 0 failures. All 19 packages that have tests
  return `ok`; 2 report `[no test files]`.
result: pass
automated: true
evidence: "19 packages ok (cached), 0 FAIL; internal/ipc and internal/sentry/windows re-ran fresh 1.5s/1.7s"

### 3. Cross-platform builds (GOOS=linux, GOOS=darwin)
expected: |
  `GOOS=linux go build ./...` and `GOOS=darwin go build ./...` both exit 0 — darwin and
  Windows files are correctly excluded by build tags on each platform.
result: pass
automated: true
evidence: "Both exits 0 with no output"

### 4. SMAC-02: eslogger parser (parseEsloggerLine)
expected: |
  `internal/sentry/darwin/parser.go` (//go:build darwin) exports `parseEsloggerLine`
  mapping exec/open/create/network_flow events to sentry.SentryEvent. Parser test fixtures
  exist in `testdata/`.
result: pass
automated: true
evidence: "grep -c func parseEsloggerLine internal/sentry/darwin/parser.go = 1; testdata/ contains 4 fixture files"

### 5. SMAC-04: CoverageGapNotes on all 3 output surfaces
expected: |
  `darwin.CoverageGapNotes()` returns a string containing "Keychain" and "Cocoa".
  `protect_darwin.go` calls it in install path, status-connected path, and status-fallback
  path (3 call sites).
result: pass
automated: true
evidence: "grep Keychain launchd.go = 1; grep Cocoa launchd.go = 1; grep CoverageGapNotes protect_darwin.go = 3"

### 6. SMAC-01: macOS RunDaemon + correlation engine
expected: |
  `internal/sentry/darwin/daemon.go` provides `RunDaemon`, `correlationEngineLoop`,
  `alertToAuditRecord`. `sentry.EvaluateEvent` is called (SMAC-03: shared rule engine).
result: pass
automated: true
evidence: "func RunDaemon = 1; sentry.EvaluateEvent = 1; sentry.LoadBaseline = 2"

### 7. SWIN-02: Windows ETW ingestion (tekert/golang-etw v0.6.2)
expected: |
  `internal/sentry/windows/etw.go` (//go:build windows) has `StartETWConsumer`,
  `EventsLost uint64` atomic counter, `ProviderGUIDs` map. `go.mod` declares
  `github.com/tekert/golang-etw`.
result: pass
automated: true
evidence: "StartETWConsumer = 1; EventsLost = 4 occurrences; golang-etw in go.mod = 1"

### 8. SWIN-03: NT Kernel Logger conflict probe
expected: |
  `internal/sentry/windows/conflict.go` has `ProbeKernelLoggerConflict` and
  `ConflictMessage`. `protect_windows.go` calls `ProbeKernelLoggerConflict` at both
  install and status time.
result: pass
automated: true
evidence: "func ProbeKernelLoggerConflict = 1; ProbeKernelLoggerConflict in protect_windows.go = 3"

### 9. SWIN-04: EventsLost surfaced in protect status
expected: |
  `protect_windows.go runProtectStatus` prints EventsLost count from IPC StatusResponse.
  "processed" and "lost" appear in the format string.
result: pass
automated: true
evidence: "grep EventsLost|processed.*lost protect_windows.go = 2"

### 10. SWIN-05: ipc/stub.go deleted; go-winio named pipe replaces it
expected: |
  `internal/ipc/stub.go` does NOT exist. `internal/ipc/pipe_windows.go` (//go:build windows)
  has `winio.ListenPipe`, SDDL `D:(A;;GRGW;;;<SID>)`, `NewServer`, `Connect`,
  `SendCommand`, `ReadResponse`. `go.mod` declares `github.com/Microsoft/go-winio`.
result: pass
automated: true
evidence: "stub.go: No such file or directory (confirmed deleted); winio.ListenPipe = 1; D:(A;;GRGW;;; = 2; go-winio in go.mod = 1"

### 11. SWIN-01: Windows Service under NT AUTHORITY\LocalService
expected: |
  `internal/sentry/windows/service.go` has `InstallService`, `UninstallService`,
  `QueryService`, `WaitForPipe`. Service config specifies `NT AUTHORITY\LocalService`
  and `mgr.StartAutomatic`.
result: pass
automated: true
evidence: "NT AUTHORITY = 2; func InstallService = 1; mgr.StartAutomatic confirmed in service.go"

### 12. SWIN-06: Windows RunDaemon via svc.Run + shared correlation engine
expected: |
  `internal/sentry/windows/daemon.go` dispatches via `svc.IsWindowsService()` and
  `svc.Run`. `sentry.EvaluateEvent` is called (shared rule engine). `sentry_alert_baseline`
  appears (7-day baseline mode). 5 rules initialized (SENTRY-005 present).
result: pass
automated: true
evidence: "svc.IsWindowsService = 2; svc.Run = confirmed; sentry.EvaluateEvent = 1; sentry_alert_baseline = 2; SENTRY-005 = 1"

### 13. Windows CLI admin guard
expected: |
  `protect_windows.go` checks for admin elevation (UAC) before install/uninstall via
  `isElevated()` / `GetCurrentProcessToken().IsElevated()`.
result: pass
automated: true
evidence: "grep IsElevated|isElevated protect_windows.go = 6"

### 14. protect_other.go dead-code state
expected: |
  `protect_other.go` first line is `//go:build !linux && !darwin && !windows` — it now
  only catches unsupported platforms (FreeBSD etc.) and is effectively dead code for v1.
result: pass
automated: true
evidence: "head -1 cmd/beekeeper/protect_other.go = //go:build !linux && !darwin && !windows"

### 15. Cross-platform alertToAuditRecord audit schema invariance
expected: |
  `sentry_alert_baseline` appears in darwin/daemon.go, windows/daemon.go, AND
  linux/daemon.go — all three platforms use the identical audit record schema.
result: pass
automated: true
evidence: "sentry_alert_baseline: darwin = 2, windows = 2, linux = 2"

### 16. SFDF-05 (part 1): CycloneDX SBOM in GoReleaser
expected: |
  `.goreleaser.yaml` has a `sboms:` section invoking syft to produce `cyclonedx-json`
  output per archive. Phase 1 `signs:` (cosign), `mod_timestamp`, and `-trimpath`
  sections are preserved.
result: pass
automated: true
evidence: "sboms: = 1; cyclonedx-json = 1; cosign = 3; mod_timestamp = 2; sigstore/cosign-installer@v3 in release.yml = 1"

### 17. SFDF-05 (part 2): SLSA Level 3 provenance job @v2.1.0
expected: |
  `.github/workflows/release.yml` has a `provenance` job using
  `slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.1.0`
  (full semver — never @v2). Goreleaser job exports `outputs.hashes`; provenance job
  consumes via `base64-subjects`.
result: pass
automated: true
evidence: "generator_generic_slsa3.yml@v2.1.0 = 1; @v2 truncated form = 0; base64-subjects = 1; anchore/sbom-action/download-syft@v0 = 1"

### 18. SMAC-02 CI release gate: test-eslogger-fields job
expected: |
  `.github/workflows/ci.yml` has a `test-eslogger-fields` job on `macos-latest` that
  captures live eslogger output with `sudo -n eslogger` and validates the parser.
  The job is in the `release-gate` needs list (schema drift blocks releases).
  `eslogger_fields_test.go` has `t.Skip` for local runs without the env var.
result: pass
automated: true
evidence: "test-eslogger-fields = 2 (job def + needs); sudo -n eslogger = 1; BEEKEEPER_ESLOGGER_FIXTURE = 1; t.Skip in test = 2"

### 19. go.mod completeness
expected: |
  `go.mod` declares both `github.com/tekert/golang-etw` (07-02, Windows ETW) and
  `github.com/Microsoft/go-winio` (07-04, Windows named pipe) as direct dependencies.
result: pass
automated: true
evidence: "golang-etw in go.mod = 1; go-winio in go.mod = 1"

## Summary

total: 19
passed: 19
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
