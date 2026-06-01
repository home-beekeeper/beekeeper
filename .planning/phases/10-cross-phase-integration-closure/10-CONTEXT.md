# Phase 10: Cross-Phase Integration Closure - Context

**Gathered:** 2026-05-29
**Status:** Ready for planning
**Source:** v1.0.0 milestone audit (`.planning/v1.0.0-MILESTONE-AUDIT.md`) — no discuss-phase (audit findings are the spec)

<domain>
## Phase Boundary

The v1.0.0 milestone audit found the per-phase work complete and individually verified, but **four cross-phase integration blockers** where exported components were never wired into the runtime path. This phase closes them. It adds **no new features** — it connects existing, tested components so shipped requirements are functional end-to-end.

All four blockers + both warnings are confirmed with direct grep evidence in the audit.
</domain>

<decisions>
## Locked decisions (from the audit)

### INT-BLOCK-2 — live `corroboration_threshold` (PLCY-07, CODE-01)
`internal/check/handler.go runCheck` calls `policy.Evaluate(..., policy.DefaultCorroborationThresholds(), ...)` unconditionally; `thresholdsFromPolicyFile` is only used in `policy test`. Fix: derive thresholds from the loaded policy files in the live check path and pass them to `Evaluate`. Remove the false comment at handler.go:237. Live and dry-run must agree.

### INT-BLOCK-3 — gateway multi-source corroboration (PLCY-01, CTLG-09, INTG-03/04)
`internal/gateway/gateway.go:75` builds `catalog.NewMultiIndex(bbIdx, nil, nil)`. Fix: construct OSV + Socket adapters from `Config.CacheDir`/`SocketToken` exactly as `internal/check/handler.go:194-213` does, and pass them in, so the gateway can produce a 2-source block.

### INT-BLOCK-1 — LlamaFirewall supervisor lifecycle + scan wiring (LLMF-01..06)
`llamafirewall.NewSupervisor`/`Supervisor.Start` have zero production callsites; `check.RunAuditRecordWithLLMF` and `gateway.ScanProxiedResponse` are orphaned. Fix: when `config.LlamaFirewall.Enabled`, the long-lived daemon host (gateway daemon, and a standalone `llamafirewall` serve path) starts/supervises the sidecar; the gateway proxy calls `ScanProxiedResponse`; `audit-record` routes to `RunAuditRecordWithLLMF` and scans via the running sidecar, failing per `fail_mode` (fail-closed default) when the sidecar is unreachable. LLMF stays disabled-by-default. **Design note:** the sidecar is long-lived; one-shot commands connect to a running sidecar socket rather than spawning their own. If the lifecycle host raises a genuine design choice the plan cannot resolve cleanly, the executor should checkpoint rather than guess.

### INT-BLOCK-4 — diag sidecar latency (CODE-06)
`llamafirewall.GlobalLatencyTracker` is read by `diag.go:92` but never `.Record()`'d. Fix: have `Supervisor.Scan` also record into `GlobalLatencyTracker` (or point `CollectDiag` at the active supervisor's tracker). After real sidecar calls, `beekeeper diag` must show a non-zero sidecar p95.

### INT-WARN-1 — overlay coverage (CODE-01)
`policyloader.LoadPolicyDir` + `ApplyPolicyOverlay` are applied only in `beekeeper check`. Apply them in the `gateway`, `watch`, and `scan` decision paths too (mirror `handler.go:244-253`).

### INT-WARN-2 — orphaned exports
`ScanProxiedResponse` and `RunAuditRecordWithLLMF` must have real callers after this phase (resolved by INT-BLOCK-1).

## Constraints (unchanged)
- `internal/policy` stays a pure library — no I/O. Threshold derivation + overlay live in `internal/policyloader` / callers.
- Fail-closed by default. Single static binary, no CGO in core. Build-tag pairs stay paired.
- `go build ./...`, `GOOS=windows go build ./...`, and `go test ./...` must stay green.

## Claude's Discretion
- Exact LLMF supervisor lifecycle host (gateway daemon vs a dedicated `llamafirewall serve`) and the one-shot connect-or-fail-closed mechanism — implement the most defensible design consistent with the disabled-by-default posture; checkpoint if genuinely ambiguous.
- Whether to export `thresholdsFromPolicyFile` or add a new `policyloader` helper for live threshold derivation.
</decisions>

<canonical_refs>
- `.planning/v1.0.0-MILESTONE-AUDIT.md` — the spec (gaps + recommended fixes + file:line evidence)
- `internal/check/handler.go` (runCheck — BLOCK-2, BLOCK-4 read path, overlay reference at 244-253)
- `internal/gateway/gateway.go` + `internal/gateway/proxy.go` + `internal/gateway/policy.go` (BLOCK-3, BLOCK-1 gateway scan, WARN-1)
- `internal/llamafirewall/supervisor.go` + `client.go` + `latency.go` (BLOCK-1, BLOCK-4)
- `internal/policyloader/test.go` (thresholdsFromPolicyFile) + `enforce.go` (overlay)
- `internal/watch/handler.go` + `internal/scan/scanner.go` (WARN-1)
- `cmd/beekeeper/main.go` (audit-record, gateway, llamafirewall commands)
- `CLAUDE.md` (architecture constraints)
</canonical_refs>

<deferred>
- No new features. Distributed mode, weighted corroboration, etc. remain out of scope.
</deferred>

---
*Phase: 10-cross-phase-integration-closure*
*Context derived from milestone audit on 2026-05-29 (no discuss-phase)*
