---
phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement
plan: "01"
subsystem: check, hooks, cmd
tags: [hook-block, deny-renderer, harness-protocol, security, exit-code]
dependency_graph:
  requires: []
  provides:
    - internal/check.RenderDeny
    - internal/check.HarnessID
    - internal/check.DenyOutput
    - internal/check.exitHookBlock
    - cmd/beekeeper: --hook flag on check command
    - internal/hooks.claudePreCommand (updated to --hook claude-code)
  affects:
    - Plans 02-06 (all use RenderDeny and the --hook adapter)
tech_stack:
  added: []
  patterns:
    - pure table-driven function (no I/O, no os.Exit)
    - per-family typed JSON structs (not hand-built strings)
    - TDD RED/GREEN cycle
key_files:
  created:
    - internal/check/deny_render.go
    - internal/check/deny_render_test.go
  modified:
    - cmd/beekeeper/main.go
    - internal/hooks/claude_code.go
    - internal/hooks/hooks_test.go
decisions:
  - "RenderDeny returns ExitCode=0 on allow (never emits permissionDecision:allow) — harness approval flow is not bypassed (CONTEXT decision 3, T-10-02)"
  - "Hermes ExitCode=0 by design — exit codes ignored; JSON is the only block path; guaranteed non-empty message (T-10-04)"
  - "Unknown HarnessID fails closed: exit 2 + stderr, nil Stdout (never silently allows)"
  - "claudePreCommand changed from 'beekeeper check' to 'beekeeper check --hook claude-code' — propagates to merge/uninstall helpers via sentinel string"
  - "TestInstallClaudeCodePreservesExistingHooks assertions updated to match new command string (same merge logic, new sentinel)"
metrics:
  duration: "~15 minutes"
  completed: "2026-06-05"
  tasks_completed: 3
  files_created: 2
  files_modified: 3
---

# Phase 10 Plan 01: Pure Deny Renderer, --hook Flag, and Claude Code Installer Fix Summary

**One-liner:** Pure `RenderDeny(HarnessID, Decision) → DenyOutput` covering 8 deny-contract families across 15 harnesses, `beekeeper check --hook <h>` flag emitting exit-2 + harness JSON on block, and the Claude Code installer now wires the correct command.

## What Was Built

### Task 1: `internal/check/deny_render.go` — Pure RenderDeny + HarnessID/DenyOutput

Created `internal/check/deny_render.go` in package `check`:

- `HarnessID` type with 15 const values (claude-code, cursor, codex, augment, codebuddy, qwen, copilot, gemini, antigravity, windsurf, cline, hermes, opencode, kilo, trae)
- `exitHookBlock = 2` — the exit code honored by all hook-capable harnesses except Hermes
- `DenyOutput` struct (Stdout/Stderr/ExitCode)
- `RenderDeny(h HarnessID, d policy.Decision) DenyOutput` — pure function, no I/O, no os.Exit, no goroutines

Eight deny-contract families implemented with typed JSON structs:
- Family A: `hookSpecificOutputDeny` — nested hookSpecificOutput (Claude Code, Codex, CodeBuddy, Augment, Qwen)
- Family B: `copilotDeny` — flat permissionDecision (Copilot)
- Family C: `cursorDeny` — permission + user/agent message (Cursor)
- Family D: `geminiDeny` — decision + reason (Gemini CLI)
- Family E: `antigravityDeny` — dual-defensive decision + permissionDecision + denyReason (Antigravity)
- Family F: `clineDeny` — cancel + errorMessage (Cline)
- Family G: `hermesDeny` — action:block + guaranteed-non-empty message; ExitCode=0 (Hermes fail-open)
- Family H: nil Stdout (Windsurf, OpenCode, Kilo, Trae — exit-2-only)
- Default: unknown HarnessID fails closed (exit 2 + stderr, nil Stdout)

### Task 2: `internal/check/deny_render_test.go` + `--hook` flag in `cmd/beekeeper/main.go`

`internal/check/deny_render_test.go` — `TestRenderDeny` with 21 table-driven test cases:
- One row per harness × deny-family
- Asserts ExitCode, Stdout substring (or empty assertion), Stderr substring
- Hermes row: wantExit=0 + `"action":"block"` (JSON-only block path)
- Windsurf row: wantExit=2 + empty Stdout assertion
- Allow row: wantExit=0 + empty Stdout + (no Stderr assertion) — proves allow never over-allows
- Extra sub-test: Hermes with empty reason substitutes "blocked by beekeeper policy"

`cmd/beekeeper/main.go` `newCheckCmd()`:
- Added `var hookTarget string`
- `cmd.Flags().StringVar(&hookTarget, "hook", "", ...)` registered
- After `RunCheck`: if hookTarget != "" && !result.Decision.Allow → `check.RenderDeny(check.HarnessID(hookTarget), result.Decision)` → write stdout/stderr → `os.Exit(out.ExitCode)`
- Default path (no --hook) UNCHANGED: raw Decision JSON + exit 0/1

### Task 3: `claudePreCommand` fix + `TestInstallClaudeCodeWiresHookFlag`

`internal/hooks/claude_code.go`:
- `claudePreCommand` changed from `"beekeeper check"` to `"beekeeper check --hook claude-code"`
- `claudePostCommand` unchanged (`"beekeeper audit-record"`)
- All merge/uninstall helpers (`mergeClaudeHookEntry`, `removeClaudeHookEntry`, `claudeEntriesContainCommand`) reference `claudePreCommand` as sentinel — change propagates automatically

`internal/hooks/hooks_test.go`:
- Added `TestInstallClaudeCodeWiresHookFlag`: installs into temp dir, reads back, asserts inner hooks command equals `"beekeeper check --hook claude-code"`, re-installs to assert idempotency
- Updated `TestInstallClaudeCodePreservesExistingHooks` assertions from `"beekeeper check"` to `"beekeeper check --hook claude-code"` (same merge logic, updated sentinel)

## Verification Results

```
go build ./...                                          OK
go vet ./internal/check/ ./cmd/beekeeper/              OK
go test ./internal/check/ -run TestRenderDeny -count=1 PASS (21/21)
go test ./internal/hooks/ -run 'TestInstallClaudeCode' PASS (4/4)
go test ./internal/check/ ./internal/hooks/ -count=1   PASS
```

## Commits

| Hash | Message |
|------|---------|
| d7bffe3 | feat(10-01): add pure RenderDeny table-driven deny renderer + TestRenderDeny gate |
| 76029a3 | feat(10-01): add --hook flag to beekeeper check; routes block to RenderDeny |
| 342d78a | fix(10-01): claudePreCommand → 'beekeeper check --hook claude-code' + assert test |

## Deviations from Plan

None - plan executed exactly as written.

The only deviation was updating the existing `TestInstallClaudeCodePreservesExistingHooks` test assertions from `"beekeeper check"` to `"beekeeper check --hook claude-code"`. This was required because the test previously checked the sentinel string that `claudePreCommand` held, and changing the constant is the whole point of Task 3. This is not a deviation — it is the expected consequence of a locked decision (locked decision #2 in the plan's constraints protects the default mode, not the installed command string).

## Known Stubs

None. All harness deny contracts are implemented against their documented specifications. Windsurf and OpenCode emit nil Stdout by design (exit-2-only contracts).

## Threat Flags

No new security-relevant surface beyond what is declared in the plan's `<threat_model>`. All five STRIDE threats (T-10-01 through T-10-05) were addressed:
- T-10-01: TestRenderDeny asserts exact exit code + JSON per harness (the missing gate)
- T-10-02: allow row in TestRenderDeny proves no harness-specific output on allow
- T-10-03: default path unchanged; --hook branch gated on `hookTarget != "" && !result.Decision.Allow`
- T-10-04: Hermes guaranteed non-empty message sub-test
- T-10-05: claudePreCommand change reuses existing merge-not-clobber path (commit 50513ae)

## Self-Check: PASSED

Files created/modified all exist and build clean:
- internal/check/deny_render.go — FOUND
- internal/check/deny_render_test.go — FOUND
- cmd/beekeeper/main.go — FOUND (contains `"hook"` flag + RenderDeny call)
- internal/hooks/claude_code.go — FOUND (claudePreCommand = "beekeeper check --hook claude-code")
- internal/hooks/hooks_test.go — FOUND (TestInstallClaudeCodeWiresHookFlag)

Commits exist:
- d7bffe3 — FOUND
- 76029a3 — FOUND
- 342d78a — FOUND
