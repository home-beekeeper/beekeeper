---
phase: "06"
plan: "04"
subsystem: llamafirewall
tags: [supervisor, ipc, sidecar, latency, cli]
dependency_graph:
  requires: [06-02]
  provides: [llamafirewall-supervisor, llamafirewall-client, latency-tracker]
  affects: [cmd/beekeeper/main.go, internal/llamafirewall]
tech_stack:
  added: []
  patterns:
    - Ring-buffer P95 latency tracking (fixed 100-sample window)
    - Exponential-backoff restart supervisor with fail-closed/open/warn modes
    - Build-tag-gated IPC clients (Unix socket / named pipe)
    - Python sidecar with length-prefixed JSON framing (same as proto.go)
key_files:
  created:
    - internal/llamafirewall/latency.go
    - internal/llamafirewall/latency_test.go
    - internal/llamafirewall/client.go
    - internal/llamafirewall/client_windows.go
    - internal/llamafirewall/supervisor.go
    - internal/llamafirewall/supervisor_test.go
    - sidecar/llamafirewall_sidecar.py
    - sidecar/requirements.txt
  modified:
    - cmd/beekeeper/main.go
decisions:
  - LlamaFirewallConfig type defined in llamafirewall package (struct mirrors config.LlamaFirewallConfig) — supervisor package independence
  - Sample-rate gating applied before IPC dial to avoid latency tracking skew on unsampled requests
  - Degradation via watchProcess goroutine only — Scan itself does not auto-degrade on transient error
  - PID state persistence to ~/.beekeeper/state.json uses merge pattern (read existing → update key → write)
metrics:
  duration: "8m"
  completed_date: "2026-05-28"
  tasks_completed: 8
  files_created: 9
  files_modified: 1
---

# Phase 06 Plan 04: LlamaFirewall Supervisor + Client + Sidecar Summary

**One-liner:** Supervisor with exponential-backoff restart, Unix/named-pipe IPC clients, ring-buffer P95 latency tracker, Python sidecar, and `beekeeper llamafirewall` CLI.

## What Was Built

### Task 1: LatencyTracker (LLMF-06)

`internal/llamafirewall/latency.go` — fixed 100-sample ring buffer for P95 computation and running sum for mean. Thread-safe via `sync.Mutex`. Four unit tests pass: empty P95=0, single-sample P95, ring-buffer eviction (200 samples: first 100@1ms, next 100@99ms → P95=99), mean=25 for [10,20,30,40].

### Task 2: Client — Unix socket (linux || darwin)

`internal/llamafirewall/client.go` — `Dial()` wraps `net.DialTimeout("unix", ...)`, `Scan()` sets write+read deadlines, `Close()` closes the connection. Uses `Encode`/`Decode` from proto.go.

### Task 3: Client — Windows named pipe

`internal/llamafirewall/client_windows.go` — `Dial()` opens `\\.\pipe\beekeeper-llamafirewall` via `windows.CreateFile`, `Scan()` uses `pipeReadWriter` adapter to implement `io.ReadWriter` over a `windows.Handle`. The `sockPath` argument is ignored on Windows.

### Task 4: Supervisor (LLMF-01)

`internal/llamafirewall/supervisor.go` — manages Python sidecar lifecycle:

- `Start()`: launches Python, polls for socket (2s timeout), dials client, persists PID to state.json, starts `watchProcess` goroutine.
- `watchProcess()`: blocks on `cmd.Wait()`, restarts with exponential backoff (2^retries, capped at 30s), sets `degraded=true` after MaxRetries (default 3).
- `Scan()`: reads degraded+failMode+sampleRate under mutex, short-circuits on degraded/unsampled, delegates to `client.Scan()`, records latency.
- `Stop()`: signals SIGINT, closes client.
- `StatusInfo()`: returns PID, start time, degraded flag, config snapshot, P95 latency.
- `persistState()`: reads/merges/writes `~/.beekeeper/state.json`.

### Task 5: Supervisor tests (linux build tag)

`internal/llamafirewall/supervisor_test.go` — 5 tests using mock Unix servers:

- `TestSupervisorFailsClosedAfterMaxRetries`: degraded=true + failMode="closed" → ErrSidecarUnavailable
- `TestSupervisorFailsOpenAfterMaxRetries`: degraded=true + failMode="open" → ResultClean, nil
- `TestSampleRateGating`: SampleRate=0.0 → no request sent to mock server
- `TestSupervisorScanSuccess`: mock server returns ScanResponse{LatencyMS:10} → round-trip verified
- `TestLatencyTrackerUpdatedOnScan`: after scan, sup.latency.P95() is non-zero

### Task 6-7: Python sidecar + requirements

`sidecar/llamafirewall_sidecar.py` — Unix socket server matching proto.go framing. Lazy-imports `llamafirewall` only on `scan_prompt` requests for fast startup. Binds socket at `~/.beekeeper/llamafirewall.sock` with 0o600 permissions. `scan_code` and `scan_alignment` are placeholders returning "clean".

`sidecar/requirements.txt` — `llama-firewall`, `torch`, `transformers`.

### Task 8: CLI (LLMF-01)

`cmd/beekeeper/main.go` — `newLlamaFirewallCmd()` adds:
- `llamafirewall enable` — sets `LlamaFirewall.Enabled=true` in config.json
- `llamafirewall disable` — sets `LlamaFirewall.Enabled=false` in config.json  
- `llamafirewall status` — reads state.json PID, probes liveness via Signal(0), reports sample rate / fail mode / uptime / degraded flag

## Verification Results

```
go test ./internal/llamafirewall/... -run "TestLatencyTracker" -v -count=1
  4/4 PASS

go build ./...
  PASS (no errors)

go vet ./internal/llamafirewall/...
  PASS (no warnings)

grep checks:
  type Supervisor        — FOUND in supervisor.go
  type LatencyTracker    — FOUND in latency.go
  ErrSidecarUnavailable  — FOUND in supervisor.go
  func Dial              — FOUND in client.go
  scan_prompt            — FOUND in llamafirewall_sidecar.py
  llama-firewall         — FOUND in requirements.txt

go test ./...  (17 packages)
  17/17 PASS
```

## Deviations from Plan

### Auto-added: LlamaFirewallConfig in supervisor.go

The plan said "define locally if needed". Since `internal/config` already had `LlamaFirewallConfig` added in Plan 03, the supervisor package defines its own matching `LlamaFirewallConfig` struct for package independence. The CLI wires them via `config.LlamaFirewall`. No external API change.

### Auto-fixed: config.go already had LlamaFirewallConfig (Plan 03 pre-empted)

Plan 04 noted config.go was being updated in Plan 03 concurrently. On inspection, config.go already had `LlamaFirewallConfig`, `LlamaFirewall LlamaFirewallConfig` field, and `LlamaFirewallSampleRate()` helper — no config change was needed.

## Known Stubs

| Stub | File | Reason |
|------|------|--------|
| `scan_code` returns "clean" immediately | sidecar/llamafirewall_sidecar.py:87 | CodeShield Python package not yet integrated; placeholder for future plan |
| `scan_alignment` returns "clean" immediately | sidecar/llamafirewall_sidecar.py:90 | Alignment scanner placeholder for future plan |

These stubs do not block the plan's goal (supervisor + IPC + CLI). Real CodeShield integration is a follow-on item.

## Threat Flags

None — this plan introduces no new network endpoints, auth paths, or trust-boundary schema changes. The Unix socket is restricted to owner (0o600) and the named pipe uses Windows ACLs via `CreateFile`.

## Self-Check: PASSED

Files exist:
- internal/llamafirewall/latency.go — FOUND
- internal/llamafirewall/latency_test.go — FOUND
- internal/llamafirewall/client.go — FOUND
- internal/llamafirewall/client_windows.go — FOUND
- internal/llamafirewall/supervisor.go — FOUND
- internal/llamafirewall/supervisor_test.go — FOUND
- sidecar/llamafirewall_sidecar.py — FOUND
- sidecar/requirements.txt — FOUND

Commit exists: 546d94c — FOUND
