---
phase: 08-tui-dashboard
plan: "08"
subsystem: tui
tags: [gap-closure, health-pips, llamafirewall, cross-platform]
dependency_graph:
  requires: [08-05]
  provides: [TUI-08-pip]
  affects: [model.go, health.go, base.go, model_test.go]
tech_stack:
  added: []
  patterns: [platform-tagged-files, comma-ok-type-assertion, pid-liveness-signal0]
key_files:
  created:
    - internal/tui/pid_alive_unix.go
    - internal/tui/pid_alive_windows.go
  modified:
    - internal/tui/model.go
    - internal/tui/health.go
    - internal/tui/base.go
    - internal/tui/model_test.go
decisions:
  - "Platform-split pid_alive_{unix,windows}.go to handle signal-0 vs OpenProcess liveness check without CGO"
  - "Comma-ok type assertions throughout probeLlamaFirewall to prevent panic on malformed/partial state.json (T-08-08-01 mitigation)"
  - "LlamaFirewallOK seeded true in NewApp so cold-start render shows green pip before first healthTick"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-29T13:54:09Z"
  tasks_completed: 3
  tasks_total: 3
  files_created: 2
  files_modified: 4
---

# Phase 8 Plan 08: LlamaFirewall Health Pip (Gap Closure TUI-08) Summary

LlamaFirewall boolean health pip added as the 5th component in the TUI health row, with cross-platform PID liveness probe using graceful degradation on missing/malformed state.json.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add LlamaFirewallOK to HealthState and NewApp default | d13c33f | internal/tui/model.go |
| 2 | probeLlamaFirewall in health.go, wired into refreshHealthState | fd3e7db | internal/tui/health.go, internal/tui/pid_alive_unix.go, internal/tui/pid_alive_windows.go |
| 3 | Render llamafirewall pip in base.go and cover with a test | 6d1690f | internal/tui/base.go, internal/tui/model_test.go |

## What Was Built

**HealthState extension (model.go):** Added `LlamaFirewallOK bool` field to the `HealthState` struct after `CatalogsOK`. Updated `NewApp` to seed `LlamaFirewallOK: true` so the pip renders green before the first 10s healthTick fires.

**probeLlamaFirewall probe (health.go):** Reads `state.json` from `stateDir`, json-decodes into `map[string]any`, extracts the `"llamafirewall"` sub-object and its `"pid"` field using comma-ok type assertions throughout. Returns false on any error (missing file, bad JSON, missing field, wrong type, dead PID) — never panics. Wired into `refreshHealthState` as `LlamaFirewallOK: probeLlamaFirewall(stateDir)`.

**Cross-platform PID liveness (pid_alive_unix.go / pid_alive_windows.go):** Platform-tagged helper `pidAlive(pid int) bool`. On Unix: `os.FindProcess` + `proc.Signal(syscall.Signal(0))` (kill -0 semantics, EPERM treated as alive). On Windows: `windows.OpenProcess(SYNCHRONIZE)` — confirms process existence without requiring elevated rights. No CGO.

**healthRow 5th pip (base.go):** Added `pip(a.health.LlamaFirewallOK, "llamafirewall")` segment after `catalogs fresh` pip, separated by standard `"  "` spacer. Health row now shows: hooks / gateway / sentry / catalogs fresh / llamafirewall / last block string.

**Test coverage (model_test.go):** Added `TestAppHealthLlamaFirewallPip` that: (1) asserts `LlamaFirewallOK=true` at cold start, (2) calls `a.Update(healthTick(...))` and asserts no panic + non-nil model + re-arm cmd, (3) asserts `renderBase` output contains `"llamafirewall"` label.

## Deviations from Plan

### Auto-added: Platform-tagged pid_alive files (Rule 2 - Missing Critical Functionality)

The plan specified `health.go` for `probeLlamaFirewall` but cross-platform PID liveness without CGO requires different OS primitives:
- Unix: `syscall.Signal(0)` via `proc.Signal` — not valid on Windows (sends CTRL_C_EVENT equivalent)
- Windows: `golang.org/x/sys/windows.OpenProcess` — not available on Unix

**Fix:** Added two new files `pid_alive_unix.go` (`//go:build !windows`) and `pid_alive_windows.go` (`//go:build windows`) following the existing `resize_other.go` / `resize_windows.go` pattern in the tui package. The `probeLlamaFirewall` function in `health.go` calls the `pidAlive(pid)` helper that resolves at compile time. This is required for correctness — a single-file implementation would either be Unix-only or require CGO.

**Files added:** `internal/tui/pid_alive_unix.go`, `internal/tui/pid_alive_windows.go`

## Scope Note: TUI-08 Numerics Out of Scope

TUI-08 also names "CPU/memory" and "inference latency" numerics. Per the plan objective and the gap closure specification, this plan delivers only the boolean LlamaFirewall pip (the 5th health component). CPU/mem/latency surfacing is not part of this gap's `missing` list, is not present in the existing boolean-pip HealthState, and is explicitly noted as out of scope in the plan.

## Security / Threat Model

| Threat | Disposition | Implementation |
|--------|-------------|----------------|
| T-08-08-01: Malformed state.json crashes probe | Mitigated | Comma-ok assertions on all type coercions; `json.Unmarshal` error checked; missing file returns false |
| T-08-08-02: Stale PID reuse shows false-positive green | Accepted | Pip is informational display only; no policy decision derived |
| T-08-08-03: PID check blocks healthTick | Mitigated | Local FindProcess + signal-0 is sub-ms; no network/IPC/file scan |
| T-08-08-04: state.json fields leak into TUI | Accepted | Only boolean liveness result surfaces; no state.json content rendered |

## Verification Results

- `go build ./...` — PASSED
- `go vet ./...` — PASSED
- `go test ./internal/tui/... -count=1` — PASSED (all tests green)
- `grep -n "LlamaFirewallOK bool" internal/tui/model.go` — line 30
- `grep -n "func probeLlamaFirewall" internal/tui/health.go` — line 98
- `grep -n "LlamaFirewallOK: probeLlamaFirewall(stateDir)" internal/tui/health.go` — line 27
- `grep -n "llamafirewall" internal/tui/base.go` — line 62
- `grep -n "TestAppHealthLlamaFirewallPip" internal/tui/model_test.go` — line 172

## Self-Check: PASSED

Files exist:
- internal/tui/model.go — FOUND
- internal/tui/health.go — FOUND
- internal/tui/base.go — FOUND
- internal/tui/model_test.go — FOUND
- internal/tui/pid_alive_unix.go — FOUND
- internal/tui/pid_alive_windows.go — FOUND

Commits exist:
- d13c33f (Task 1) — FOUND
- fd3e7db (Task 2) — FOUND
- 6d1690f (Task 3) — FOUND
