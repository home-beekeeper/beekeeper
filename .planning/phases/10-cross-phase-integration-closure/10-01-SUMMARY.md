---
plan: 10-01
phase: 10
subsystem: cross-phase-integration-closure
tags: [integration, policy, llamafirewall, gateway, watch, scan, corroboration, threshold]
dependency_graph:
  requires:
    - 09-06 (policyloader, policy-as-code, ThresholdsFromPolicyFiles path)
    - 06-04 (llamafirewall supervisor + client)
    - 06-05 (RunAuditRecordWithLLMF, ScanProxiedResponse)
    - 04-01 (MCP gateway Start, Config)
    - 02-08 (MultiIndex, OSVAdapter, SocketAdapter)
  provides:
    - Live corroboration_threshold policy rules affect beekeeper check decisions
    - Gateway enforces multi-source corroboration (Bumblebee+OSV+Socket)
    - Policy overlay enforced in gateway, watch, and scan (not only check)
    - LlamaFirewall supervisor started by gateway daemon when enabled; audit-record wired
    - GlobalLatencyTracker populated by Supervisor.Scan; beekeeper diag reports real p95
  affects:
    - internal/check/handler.go (threshold derivation)
    - internal/policyloader/test.go (exported helper)
    - internal/gateway/gateway.go, proxy.go, policy.go (multi-source + overlay + LLMF)
    - internal/watch/handler.go (overlay)
    - internal/scan/scanner.go (overlay)
    - internal/llamafirewall/supervisor.go (GlobalLatencyTracker population)
    - cmd/beekeeper/main.go (supervisor lifecycle + audit-record routing)
tech_stack:
  added: []
  patterns:
    - ThresholdsFromPolicyFiles: exported helper shared by all callers for threshold derivation
    - PolicyOverlay applied in gateway/watch/scan mirroring handler.go pattern
    - GatewayScanner injected into gateway.Config for optional LLMF scanning
    - llmfClientScanner adapts *Client to check.Scannable for one-shot audit-record
key_files:
  created: []
  modified:
    - internal/policyloader/test.go
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/gateway/gateway.go
    - internal/gateway/proxy.go
    - internal/gateway/policy.go
    - internal/gateway/gateway_test.go
    - internal/watch/handler.go
    - internal/watch/handler_test.go
    - internal/scan/scanner.go
    - internal/llamafirewall/supervisor.go
    - internal/llamafirewall/latency_test.go
    - cmd/beekeeper/main.go
decisions:
  - ThresholdsFromPolicyFiles exported from policyloader/test.go (not a new file) so all callers share one implementation; thresholdsFromPolicyFile (unexported, single-file) delegated to it
  - gateway.Config.Scanner GatewayScanner field (nil = disabled) preferred over passing supervisor directly to gateway.Start — lets main.go own the lifecycle, gateway owns the usage
  - llmfClientScanner adapter in main.go bridges *llamafirewall.Client to check.Scannable for one-shot commands; no new interface needed in llamafirewall package
  - watch/scan policy errors are non-fatal (missing dir = no-op); handler.go is fail-closed because it is the primary enforcement point
  - forwardAllowWithScan added for allow-path LLMF scanning when scanner present; transparent ReverseProxy preserved when scanner is nil (no overhead)
metrics:
  duration: ~45 minutes
  completed: "2026-06-01"
  tasks: 5
  files_modified: 13
---

# Phase 10 Plan 01: Cross-Phase Integration Closure Summary

Closed all four INT-BLOCK and both INT-WARN gaps from the v1.0.0 milestone audit. No new features — connected existing, tested components into the runtime path. `internal/policy` untouched; LLMF disabled-by-default behavior unchanged; native + Windows builds + full `go test ./...` stay green.

## Objective

Connect four cross-phase integration blockers (corroboration_threshold not live, gateway Bumblebee-only, LLMF never started, GlobalLatencyTracker never populated) and two warnings (policy overlay missing from gateway/watch/scan, orphaned LLMF exports) so shipped requirements work end-to-end.

## Tasks Completed

| Task | Commit | Description |
|------|--------|-------------|
| 10-01-01 INT-BLOCK-2 | d6bea7a | Export ThresholdsFromPolicyFiles; wire live threshold in check |
| 10-01-02 INT-BLOCK-3 + overlay | e6141d5 | Gateway multi-source corroboration + policy overlay |
| 10-01-03 INT-WARN-1 | 23f4183 | Apply policy overlay in watch and scan decision paths |
| 10-01-04 INT-BLOCK-1 + INT-WARN-2 | 25473d7 | LlamaFirewall supervisor lifecycle + scan wiring |
| 10-01-05 INT-BLOCK-4 | 9ccf333 | Populate GlobalLatencyTracker in Supervisor.Scan |

## What Was Fixed

### INT-BLOCK-2: corroboration_threshold policy rules now affect live beekeeper check

`runCheck` previously called `policy.Evaluate(..., policy.DefaultCorroborationThresholds(), ...)` unconditionally, ignoring any `corroboration_threshold` rules in loaded policy files. The `thresholdsFromPolicyFile` helper existed only in `policyloader/test.go` for `RunPolicyTest` (dry-run) but never reached the live path — creating a live/dry-run divergence.

Fix: exported `ThresholdsFromPolicyFiles([]PolicyFile) policy.CorroborationThresholds` from `policyloader/test.go`. In `runCheck`, policy files are now loaded **before** `policy.Evaluate`, and `ThresholdsFromPolicyFiles`-derived thresholds are passed instead of the unconditional defaults. `RunPolicyTest` updated to call `ThresholdsFromPolicyFiles` for parity.

### INT-BLOCK-3: Gateway now performs multi-source corroboration

`gateway.Start` previously passed `catalog.NewMultiIndex(bbIdx, nil, nil)` — Bumblebee-only. OSV + Socket adapters were never constructed despite `Config.CacheDir` and `Config.SocketToken` existing.

Fix: gateway.go mirrors `handler.go:194-213` to construct `OSVAdapter` and `SocketAdapter` and pass them to `NewMultiIndex`. The gateway now produces 2-source corroboration blocks matching the hook handler.

### INT-WARN-1: Policy overlay applied in gateway, watch, and scan

Previously `ApplyPolicyOverlay` was only applied in `beekeeper check`. `gateway/proxy.go`, `watch/handler.go`, and `scan/scanner.go` called `policy.Evaluate` directly without the overlay or policy-file-derived thresholds.

Fix: all three paths now load `policyloader.LoadPolicyDir`, derive thresholds via `ThresholdsFromPolicyFiles`, pass to `policy.Evaluate`, and apply `ApplyPolicyOverlay`. Missing dir is a no-op; malformed file is skipped.

### INT-BLOCK-1 + INT-WARN-2: LlamaFirewall supervisor now has production callers

`llamafirewall.NewSupervisor`/`Supervisor.Start` had zero production callers. `check.RunAuditRecordWithLLMF` and `gateway.ScanProxiedResponse` were exported but never called outside tests.

Fix:
- `newGatewayCmd` creates and starts the supervisor when `cfg.LlamaFirewallEnabled()`. The supervisor is passed as `gateway.Config.Scanner` (a new optional `GatewayScanner` field). With LLMF disabled (default), no supervisor is created — zero behavior change.
- `gatewayHandler.forwardWithWarningInjection` and `forwardAllowWithScan` call `ScanProxiedResponse` when scanner is non-nil.
- `newAuditRecordCmd` dials the running sidecar socket and routes to `RunAuditRecordWithLLMF` via `llmfClientScanner` when LLMF is enabled. Fail-closed on unreachable sidecar.

### INT-BLOCK-4: GlobalLatencyTracker populated by Supervisor.Scan

`check/diag.go:92` reads `llamafirewall.GlobalLatencyTracker.P95()` but no `Record()` callsite existed. `Supervisor.Scan` recorded into the per-instance `s.latency` only.

Fix: Added `GlobalLatencyTracker.Record(resp.LatencyMS)` in `Supervisor.Scan` immediately after the per-instance record. `beekeeper diag` now reports real sidecar p95 after production calls.

## Deviations from Plan

None — plan executed exactly as written. All acceptance criteria met.

## Verification

- `go test ./...` — 23/23 packages pass
- `go build ./...` — clean
- `GOOS=windows go build ./...` — clean
- `git diff 033a2cc..HEAD -- internal/policy/` — empty (untouched)
- Production callers confirmed: `llamafirewall.NewSupervisor` (main.go:1081), `check.RunAuditRecordWithLLMF` (main.go:1521), `ScanProxiedResponse` (proxy.go:288, proxy.go:346)
- `GlobalLatencyTracker.Record` production callsite: `supervisor.go:301`

## Self-Check: PASSED

All 5 task commits exist in git log. All modified files verified present. Full test suite green. `internal/policy` unchanged.
