---
phase: "06"
plan: "05"
subsystem: llamafirewall
tags: [llmf, audit, hook-handler, gateway, integration]
dependency_graph:
  requires: [06-04]
  provides: [LLMF-02, LLMF-03, LLMF-04, AUDT-01]
  affects: [internal/audit, internal/check, internal/gateway, internal/llamafirewall]
tech_stack:
  added: []
  patterns: [Scannable interface injection, GatewayScanner interface injection]
key_files:
  created:
    - internal/llamafirewall/integration.go
    - internal/llamafirewall/integration_test.go
  modified:
    - internal/audit/types.go
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/gateway/policy.go
decisions:
  - Scannable interface in check package allows mock injection in tests without importing supervisor
  - GatewayScanner mirrors Scannable for gateway package isolation
  - Injection detection writes llmf_alert record but does not block PostToolUse (LLMF-02 spec)
  - Sidecar unavailable + fail-closed returns exit 1 to block tool result propagation
metrics:
  duration: "~8 minutes"
  completed: "2026-05-28"
  tasks_completed: 4
  files_changed: 6
---

# Phase 6 Plan 05: LLMF Handler+Gateway Integration Summary

Wire LlamaFirewall supervisor into hook handler and MCP gateway with PromptGuard 2 (LLMF-02), CodeShield (LLMF-03), AlignmentCheck (LLMF-04), and LLMF provenance fields in AuditRecord (AUDT-01).

## Tasks Completed

| Task | Description | Status |
|------|-------------|--------|
| 1 | Append 5 LLMF fields to AuditRecord | Done |
| 2 | integration.go: ShouldScanPrompt, ShouldScanCode, BuildWarningPayload | Done |
| 3 | integration_test.go: 5 unit tests | Done |
| 4 | handler.go: Scannable interface + RunAuditRecordWithLLMF + helpers | Done |
| 4b | handler_test.go: 5 TestHandlerLLMF* tests + mockScanner | Done |
| 5 | gateway/policy.go: GatewayScanner interface + ScanProxiedResponse | Done |

## Test Results

- `go test ./internal/llamafirewall/... -run "TestShouldScan|TestBuildWarning"` — 5/5 PASS
- `go test ./internal/check/... -run "TestHandlerLLMF"` — 5/5 PASS
- `go build ./...` — PASS
- `go vet ./...` — PASS

## Decisions Made

1. **Scannable interface** lives in `internal/check` — avoids circular import; supervisor satisfies it at runtime, mocks satisfy it in tests.
2. **GatewayScanner interface** in `internal/gateway/policy.go` mirrors Scannable for the same reason; keeping gateway isolated from check package.
3. **Injection (LLMF-02)**: writes `llmf_alert` record but returns exit 0 — PostToolUse hooks must not block agent; the alert record is the forensic signal.
4. **Fail-closed on sidecar unavailable**: returns exit 1 when `cfg.FailClosed()` is true, blocks tool result propagation to agent context.
5. **CodeShieldAction "warn"**: writes alert record and sets `Decision.Level="warn"` but returns 0; "block" returns 1.

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — all functions are fully wired. ScanProxiedResponse and RunAuditRecordWithLLMF are callable with a real `*llamafirewall.Supervisor`; the gateway daemon wiring (calling ScanProxiedResponse on proxied responses) is deferred to the gateway daemon plan.

## Threat Flags

None — no new network endpoints or trust boundaries introduced. ScanProxiedResponse is a pure function called within existing gateway request flow.

## Self-Check: PASSED

- internal/llamafirewall/integration.go: EXISTS
- internal/llamafirewall/integration_test.go: EXISTS
- internal/audit/types.go contains LLMFScanned: VERIFIED
- internal/check/handler.go contains RunAuditRecordWithLLMF: VERIFIED
- internal/gateway/policy.go contains ScanProxiedResponse: VERIFIED
- All tests: PASS
- Build+vet: PASS
