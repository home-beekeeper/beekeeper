---
plan: 05-05
status: complete
wave: 3
---
# 05-05 Summary: Daemon Wiring + CLI + LVH CI

## Artifacts
- `internal/sentry/linux/systemd.go` — WriteUnitFile, SystemctlDaemonReload, SystemctlEnableNow, SystemctlDisableNow, SystemctlIsActive, WaitForSocket, IsSystemdRunning
- `internal/sentry/linux/systemd_test.go` — TestWriteUnitFile (content verification), TestIsSystemdRunningReturnsValue
- `internal/sentry/linux/daemon.go` — RunDaemon, correlationEngineLoop, alertToAuditRecord, handleIPCConn
- `internal/sentry/linux/daemon_test.go` — TestAlertToAuditRecord, TestAlertToAuditRecordBaseline, TestAuditRecordHasSentryFields
- `cmd/beekeeper/main.go` — protect + sentry commands added (newProtectCmd, newSentryCmd)
- `cmd/beekeeper/protect_linux.go` — Linux implementations of runProtectInstall/Uninstall/Status, runSentryDaemon, runSentryRulesList/Enable/Disable, copyFile
- `cmd/beekeeper/protect_other.go` — !linux stubs for all protect/sentry dispatch functions
- `.github/workflows/ci.yml` — fuzz-ipc + test-sentry-kernel-5-4 + test-sentry-kernel-5-15 + release-gate jobs

## Verification
- `go build ./...` passes on Windows (exit 0)
- `go vet ./cmd/beekeeper/...` passes (exit 0)
- All grep patterns confirmed:
  - newProtectCmd in main.go
  - newSentryCmd in main.go
  - little-vm-helper in ci.yml
  - 5.4-main in ci.yml
  - 5.15-main in ci.yml
  - sentry_alert_baseline in daemon.go
  - DropCapabilities in daemon.go

## Notes
- `config.Load` takes a path argument (not zero-arg); resolved via `platform.ConfigPath()` in protect_linux.go
- `ipc.Server.Serve` takes `Handler func(conn net.Conn)`; handleIPCConn uses `net.Conn` parameter
- `sentry.defaultSensitivePaths` is unexported; replicated as `daemonSensitivePaths` in daemon.go
- `cfg.Policy.SensitivePaths` does not exist in Config struct; daemon uses `daemonSensitivePaths` directly
- Platform dispatch via build-tagged files (protect_linux.go / protect_other.go) keeps main.go clean
