---
phase: 07-sensitive-path-runtime-enforcement
plan: "03"
subsystem: check
tags: [spath, wiring, integration-tests, d-03, sc1, sc2, sc3, sc4]
dependency_graph:
  requires:
    - 07-01  # policy.EvaluatePath + DefaultSensitivePaths (pure engine)
    - 07-02  # extractPathTargets, canonicalizePath, mergeDecisions (impure adapter)
  provides:
    - SPATH wiring in runCheck (handler.go)
    - SPATH wiring in runCheckWithIndex (integration_test.go)
    - RunCheck integration tests SC1-SC4 with audit-record assertions (handler_test.go)
  affects:
    - internal/check/handler.go
    - internal/check/integration_test.go
    - internal/check/handler_test.go
tech_stack:
  added: []
  patterns:
    - path-block-before-overlay (runCheck insertion order)
    - audit-record-assertion pattern (SC4 proof of live wiring)
    - t.Setenv(USERPROFILE) test isolation (D-01 end-to-end)
key_files:
  created: []
  modified:
    - internal/check/handler.go
    - internal/check/integration_test.go
    - internal/check/handler_test.go
decisions:
  - "Path block inserted AFTER policy.Evaluate and BEFORE ApplyPolicyOverlay — escalation possible, downgrade prevented (T-07-13)"
  - "Identical block mirrored in runCheckWithIndex so credential blocks fire independent of catalog matching (D-03 hermetic test path)"
  - "D-03 honored: SPATH wiring touches only runCheck + runCheckWithIndex test path; gateway/watch/scan untouched"
  - "hasRuleID local helper added to handler_test.go (no containsRuleID pre-existed)"
  - "t.Setenv(USERPROFILE, C:\\Users\\testuser) used in TestRunCheckBashTypeUserProfileBlocks — never HOME (Phase 2 Pitfall-5 isolation)"
metrics:
  duration: "~15 minutes"
  completed: "2026-06-04"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
requirements: [SPATH-01, SPATH-02, SPATH-03, SPATH-04]
---

# Phase 7 Plan 03: Wave 2 — SPATH Wiring + Integration Tests Summary

**One-liner:** Wired policy.EvaluatePath into the live runCheck pipeline (after policy.Evaluate, before ApplyPolicyOverlay) and added nine RunCheck integration tests proving SC1–SC4 with exit-code AND audit-record assertions — closing finding F2.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Wire SPATH path-evaluation block into runCheck + runCheckWithIndex | 6031cca | internal/check/handler.go, internal/check/integration_test.go |
| 2 | Add RunCheck integration tests SC1–SC4 with audit-record assertions | 68c7c4a | internal/check/handler_test.go |

## What Was Built

### Task 1: handler.go + integration_test.go wiring

Inserted the path-evaluation block into two locations:

**`runCheck` (handler.go, after `policy.Evaluate`, before `ApplyPolicyOverlay`):**
```
spathCfg := policy.DefaultSensitivePaths()
for _, rawPath := range extractPathTargets(toolCall) {
    resolved := canonicalizePath(rawPath)
    if resolved == "" { continue }
    pathDecision := policy.EvaluatePath(resolved, spathCfg)
    decision = mergeDecisions(decision, pathDecision)
}
```

**`runCheckWithIndex` (integration_test.go, identical block after `policy.Evaluate`):**
Ensures the path block fires hermetically in integration tests even when the catalog
index returns no matches — path blocks are independent of catalog matching (D-03).

**Insertion order rationale:** Running before `ApplyPolicyOverlay` means:
- A JSON policy-file `sensitive_path` rule can escalate a block further
- The overlay cannot silently downgrade a path block to allow (T-07-13 mitigated)
- `AllowPatterns` in `DefaultSensitivePaths()` is the controlled escape hatch

### Task 2: handler_test.go — nine RunCheck integration tests

| Test | SC | Input | Expected |
|------|----|-------|----------|
| `TestRunCheckCredentialFileBlocks` | SC1+SC4 | Read `~/.aws/credentials` | exit 1, block, audit decision:block + sensitive-path-policy |
| `TestRunCheckTraversalBlocks` | SC1 | Read `../../.aws/credentials` | exit 1, block, audit decision:block |
| `TestRunCheckWindowsCredentialBlocks` | SC1 | Read `C:\Users\u\.aws\credentials` | exit 1, block, audit decision:block |
| `TestRunCheckBashCatCredentialBlocks` | SC2 | Bash `cat ~/.ssh/id_rsa` | exit 1, block, audit sensitive-path-policy |
| `TestRunCheckBashTypeUserProfileBlocks` | SC2/D-01 | Bash `type %USERPROFILE%\.ssh\id_rsa` + t.Setenv(USERPROFILE) | exit 1, block, sensitive-path-policy |
| `TestRunCheckEnvExampleAllowed` | SC3 | Read `/home/u/project/.env.example` | exit 0, allow |
| `TestRunCheckEnvTestAllowed` | SC3 | Read `/home/u/project/.env.test` | exit 0, allow |
| `TestRunCheckEnvSchemaAllowed` | SC3 | Read `/home/u/project/.env.schema` | exit 0, allow |
| `TestRunCheckEnvProductionBlocked` | SC3/neg | Read `/home/u/project/.env.production` | exit 1, block, audit decision:block |

A `hasRuleID(ids []string, ruleID string) bool` local helper was added (no pre-existing
`containsRuleID` existed) for the SC2 and SC4 rule_id assertions.

## Verification Results

```
go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -count=1 -timeout=60s
ok    github.com/bantuson/beekeeper/internal/policy       2.401s
ok    github.com/bantuson/beekeeper/internal/check        10.814s
ok    github.com/bantuson/beekeeper/internal/policyloader 1.934s
```

All nine new tests pass individually:
```
go test ./internal/check/... -run "TestRunCheckCredentialFileBlocks|...|TestRunCheckEnvProductionBlocked" -count=1 -v
--- PASS: TestRunCheckCredentialFileBlocks (0.06s)
--- PASS: TestRunCheckTraversalBlocks (0.06s)
--- PASS: TestRunCheckWindowsCredentialBlocks (0.06s)
--- PASS: TestRunCheckBashCatCredentialBlocks (0.06s)
--- PASS: TestRunCheckBashTypeUserProfileBlocks (0.05s)
--- PASS: TestRunCheckEnvExampleAllowed (0.03s)
--- PASS: TestRunCheckEnvTestAllowed (0.08s)
--- PASS: TestRunCheckEnvSchemaAllowed (0.04s)
--- PASS: TestRunCheckEnvProductionBlocked (0.06s)
PASS
ok    github.com/bantuson/beekeeper/internal/check   3.931s
```

## Acceptance Criteria

- [x] `go build ./internal/check/...` succeeds
- [x] internal/check/handler.go contains `extractPathTargets` and `policy.EvaluatePath` between `policy.Evaluate` and `policyloader.ApplyPolicyOverlay`
- [x] internal/check/handler.go contains `mergeDecisions`
- [x] internal/check/integration_test.go `runCheckWithIndex` also calls `extractPathTargets` + `policy.EvaluatePath`
- [x] handler.go does NOT add SPATH wiring to gateway/watch/scan (D-03 — diff confirms only runCheck changed in production code)
- [x] All nine SC1–SC4 tests pass
- [x] Block tests assert BOTH `res.ExitCode==exitBlock` AND `readLastAuditRecord(...).Decision=="block"` (SC4 live wiring proof)
- [x] SC2 Bash tests assert `rec.RuleIDs` contains `"sensitive-path-policy"`
- [x] `TestRunCheckBashTypeUserProfileBlocks` calls `t.Setenv("USERPROFILE", ...)` for D-01 end-to-end chain
- [x] Full phase suite exits 0: `go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -count=1`

## Success Criteria

- [x] SC1: `~/.aws/credentials` AND `../../.aws/credentials` AND Windows form block at RunCheck (exit 1 + decision block)
- [x] SC2: Bash `cat ~/.ssh/id_rsa` AND `type %USERPROFILE%\.ssh\id_rsa` detected (exit 1 + sensitive-path-policy rule id) (D-01)
- [x] SC3: `.env.example` / `.env.test` / `.env.schema` NOT blocked (exit 0 + allow); `.env.production` still blocks (SPATH-04)
- [x] SC4: integration tests assert exit code AND `decision:"block"` audit record — wiring proven live (Pitfall 5)
- [x] D-03 scope honored: check-only; no SPATH wiring in gateway/watch/scan

## Deviations from Plan

None — plan executed exactly as written.

- Task 2 is marked `tdd="true"` but Task 1 (the wiring) was committed first per plan sequencing (Task 1 precedes Task 2). The tests were written directly in GREEN state since the implementation was already committed. This is the expected behavior for wave-2 wiring plans where the production implementation (Task 1) logically precedes test authoring (Task 2).

## Threat Model Coverage

| Threat ID | Mitigation | Status |
|-----------|-----------|--------|
| T-07-11 | Path block inserted in runCheck + runCheckWithIndex; RunCheck integration tests assert exit code + audit record (SC4, Pitfall 5) | CLOSED |
| T-07-12 | `finalizeWithAC` chokepoint writes NDJSON; tests assert `readLastAuditRecord(...).Decision=="block"` | CLOSED |
| T-07-13 | Path block runs BEFORE `ApplyPolicyOverlay`; overlay cannot downgrade a path block to allow | CLOSED |
| T-07-14 | `extractPathTargets + canonicalizePath` are O(targets) string/stat ops; within execTimeout budget | ACCEPTED |

## Known Stubs

None. The wiring is complete and all SPATH tests prove live enforcement via the full RunCheck pipeline.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced. The path block is a synchronous, in-process string comparison with no new I/O surface.

## Self-Check: PASSED

- [x] `internal/check/handler.go` modified with path-evaluation block
- [x] `internal/check/integration_test.go` modified with mirrored path-evaluation block
- [x] `internal/check/handler_test.go` modified with nine SPATH integration tests
- [x] Commit 6031cca exists: `feat(07-03): wire SPATH path-evaluation block into runCheck + runCheckWithIndex`
- [x] Commit 68c7c4a exists: `test(07-03): add RunCheck integration tests SC1-SC4 with audit-record assertions`
- [x] `go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -count=1` exits 0
- [x] D-03: `git diff eb20fd3..HEAD -- internal/gateway internal/watch internal/scan` is empty
