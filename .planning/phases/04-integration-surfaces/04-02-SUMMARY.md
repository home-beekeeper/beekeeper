---
phase: 04-integration-surfaces
plan: "02"
subsystem: policy-engine / audit / check-handler
tags: [INTG-07, multi-agent, agent-context, audit-lineage, hook-handler]
dependency_graph:
  requires: [03-04, 04-01]
  provides: [AgentContext, readAgentContext, RunAuditRecord, AuditRecord.AgentLineage]
  affects: [internal/policy, internal/audit, internal/check, internal/watch, internal/scan]
tech_stack:
  added: []
  patterns:
    - "Pure value struct passed by value across I/O tier boundary (AgentContext)"
    - "hookInput local extension struct for Claude Code stdin fields"
    - "Env-var precedence over stdin field (T-04-02-01 mitigation)"
    - "finalizeWithAC pattern carries AgentContext through audit chokepoint"
key_files:
  created: []
  modified:
    - internal/policy/types.go
    - internal/policy/engine.go
    - internal/policy/engine_test.go
    - internal/audit/types.go
    - internal/audit/types_test.go
    - internal/audit/writer_test.go
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/check/selftest.go
    - internal/check/integration_test.go
    - internal/scan/scanner.go
    - internal/watch/handler.go
decisions:
  - "AgentContext is a pure value struct with no methods or I/O — lives in internal/policy/types.go to enforce the purity contract"
  - "hookInput local struct (in handler.go) captures Claude Code stdin agent_id without polluting the policy ToolCall type"
  - "BEEKEEPER_AGENT_ID env var takes precedence over stdin agent_id (T-04-02-01: orchestration layer has final say)"
  - "RunAuditRecord always returns 0 — PostToolUse hook failures must not disrupt the agent (T-04-02-04)"
  - "finalize() retained as a zero-AgentContext shim to avoid breaking the panic recover path; finalizeWithAC() is the canonical path"
  - "Comma-separated lineage format documented as Assumption A8 (T-04-02-03: accepted limitation)"
metrics:
  duration: "~25 minutes"
  completed_date: "2026-05-26"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 12
---

# Phase 4 Plan 02: Multi-Agent Context Propagation (INTG-07) Summary

**One-liner:** AgentContext pure struct + Evaluate depth enforcement (maxAgentDepth=10) + AuditRecord lineage fields + env-var-driven readAgentContext + RunAuditRecord exits-0 PostToolUse handler.

## What Was Built

### Task 1: AgentContext + Extended Evaluate + AuditRecord Lineage

**`internal/policy/types.go`** — Added `AgentContext` pure value struct:
```go
type AgentContext struct {
    AgentID       string
    ParentAgentID string
    Depth         int
    Lineage       []string
}
```
No methods, no I/O, no json tags. Passed by value across the I/O tier boundary.

**`internal/policy/engine.go`** — Extended `Evaluate` signature to accept `AgentContext` as fourth parameter. Added `const maxAgentDepth = 10`. Depth check is the FIRST operation in `Evaluate`:
- Negative depth normalized to 0 (T-04-02-01)
- `Depth > 10` returns immediate block with `RuleIDs: ["INTG-07"]` before any corroboration logic

**`internal/audit/types.go`** — Extended `AuditRecord` with four `omitempty` fields:
```go
AgentID       string   `json:"agent_id,omitempty"`
ParentAgentID string   `json:"parent_agent_id,omitempty"`
AgentDepth    int      `json:"agent_depth,omitempty"`
AgentLineage  []string `json:"agent_lineage,omitempty"`
```
Updated `FromDecision` to accept `policy.AgentContext` as fifth parameter and map all four fields. Zero `AgentContext{}` produces no agent fields in JSON output (omitempty handles it).

**All callers updated:** `handler.go`, `selftest.go`, `integration_test.go`, `scanner.go`, `watch/handler.go`, plus all test files (`types_test.go`, `writer_test.go`, `engine_test.go`).

### Task 2: Wire AgentContext + RunAuditRecord

**`internal/check/handler.go`** — Three major additions:

1. **`hookInput` struct** — Local extension of `policy.ToolCall` that also decodes Claude Code hook stdin `agent_id` field without polluting the pure policy type.

2. **`readAgentContext(stdinAgentID string) policy.AgentContext`** — Reads four env vars:
   - `BEEKEEPER_AGENT_ID` (env var wins over stdinAgentID; fall back to stdin if env empty)
   - `BEEKEEPER_PARENT_AGENT_ID`
   - `BEEKEEPER_AGENT_DEPTH` (negative → 0; invalid string → 0)
   - `BEEKEEPER_AGENT_LINEAGE` (comma-split → `[]string`)

3. **`RunAuditRecord(stdin io.Reader, auditPath string) int`** — PostToolUse hook handler:
   - Reads PostToolUse JSON from stdin (bounded at 1MB)
   - Tolerates malformed JSON — exits 0 always (T-04-02-04)
   - Writes `tool_result` audit record (RecordType overridden from `policy_decision`)
   - Used by `beekeeper audit-record` subcommand (registered in Plan 05)

**`runCheck` updated:** decodes stdin into `hookInput`, extracts `stdinAgentID`, calls `readAgentContext(stdinAgentID)`, passes `AgentContext` to both `Evaluate` and `finalizeWithAC`.

## Test Results

```
ok  github.com/mzansi-agentive/beekeeper/internal/policy    (includes TestAgentContextDepthBlock, TestAgentContextDepthNormalize, TestEngineImportsArePure)
ok  github.com/mzansi-agentive/beekeeper/internal/audit     (all Phase 2 tests still green; key count=14 preserved via omitempty)
ok  github.com/mzansi-agentive/beekeeper/internal/check     (TestReadAgentContext*, TestRunAuditRecord{MalformedStdin,Valid})
ok  github.com/mzansi-agentive/beekeeper/internal/scan
ok  github.com/mzansi-agentive/beekeeper/internal/watch
```

All 13 test packages pass. No regressions.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| Task 1 | c1051a2 | feat(04-02): AgentContext type + extended Evaluate + AuditRecord lineage fields (INTG-07) |
| Task 2 | 90acb4c | feat(04-02): wire AgentContext into check handler + add RunAuditRecord (INTG-07) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Five additional callers of Evaluate and FromDecision needed updating**

- **Found during:** Task 1 implementation (go build ./...)
- **Issue:** `internal/check/selftest.go`, `internal/check/integration_test.go`, `internal/scan/scanner.go`, `internal/watch/handler.go`, and `internal/audit/writer_test.go` all called the old 3-arg `Evaluate` and/or 4-arg `FromDecision` signatures
- **Fix:** Updated all callers to pass `policy.AgentContext{}` (zero value = backward-compatible)
- **Files modified:** selftest.go, integration_test.go, scanner.go, watch/handler.go, writer_test.go
- **Commit:** c1051a2

**2. [Rule 3 - Blocking] finalize() refactored into finalizeWithAC() to carry AgentContext**

- **Found during:** Task 2 implementation
- **Issue:** The existing `finalize()` chokepoint needed to pass `AgentContext` to `writeAudit`; the panic recover path also needed zero-value AgentContext
- **Fix:** Added `finalizeWithAC()` as the canonical path; `finalize()` retained as a shim calling `finalizeWithAC` with `AgentContext{}` so the panic recover path does not require ac in scope
- **Files modified:** internal/check/handler.go
- **Commit:** 90acb4c

## Success Criteria Verification

- [x] INTG-07 policy enforcement: Evaluate with Depth=11 → block with INTG-07 rule (TestAgentContextDepthBlock)
- [x] INTG-07 audit lineage: AuditRecord.AgentID/ParentAgentID/AgentDepth/AgentLineage fields present (omitempty)
- [x] beekeeper audit-record handler (RunAuditRecord): exits 0 always; writes tool_result record
- [x] internal/policy remains pure: TestEngineImportsArePure passes (no os/io/net imports in engine.go)
- [x] All existing tests green; no regressions across all 13 packages

## Known Stubs

None. AgentContext is fully wired from env vars through to the audit record.

## Threat Flags

No new threat surface introduced. All trust boundaries are in the existing handler.go I/O tier. The AgentContext purity constraint (T-04-02-06) is verified by `TestEngineImportsArePure` passing.

## Self-Check: PASSED

Files exist:
- internal/policy/types.go — contains `AgentContext`
- internal/policy/engine.go — contains `maxAgentDepth`
- internal/audit/types.go — contains `agent_id,omitempty`
- internal/check/handler.go — contains `readAgentContext`, `RunAuditRecord`

Commits exist:
- c1051a2: confirmed via git log
- 90acb4c: confirmed via git log
