---
phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement
plan: "04"
subsystem: hooks
tags: [hooks, installers, copilot, gemini, antigravity, windsurf, deny-contract, hpc-02, hpc-03]
dependency_graph:
  requires: ["10-01", "10-03"]
  provides: [TargetCopilot, TargetGemini, TargetAntigravity, TargetWindsurf, installCopilot, installGemini, installAntigravity, installWindsurf]
  affects: [internal/hooks/hooks.go, internal/hooks/hooks_test.go]
tech_stack:
  added: []
  patterns:
    - merge-not-clobber trinity (mergeClaudeHookEntry) reused for Copilot + Antigravity
    - typed-struct containsXxxHookByCommand pattern from codex.go reused for Gemini
    - typed-struct map[string][]hook pattern from cursor.go reused for Windsurf
    - runtime.GOOS branch for Windows powershell key vs Linux/macOS command key
key_files:
  created:
    - internal/hooks/copilot.go
    - internal/hooks/gemini.go
    - internal/hooks/antigravity.go
    - internal/hooks/windsurf.go
  modified:
    - internal/hooks/hooks.go
    - internal/hooks/hooks_test.go
decisions:
  - "Antigravity settings path: ~/.gemini/antigravity/hooks.json (primary); .agents/hooks.json project-local alternative documented in comment"
  - "Gemini hooks stored in settings.json 'hooks' array (NOT a separate hooks.json file) per RESEARCH row 9"
  - "Windsurf exit-2-only deny: no stdout JSON form; RenderDeny(HarnessWindsurf) returns nil Stdout"
  - "Copilot event key is preToolUse (camelCase) — correct for Copilot, unlike Cursor which uses beforeShellExecution"
metrics:
  duration: "~7 minutes (12:00 to 12:07 UTC)"
  completed: "2026-06-05"
  tasks_completed: 3
  tasks_total: 3
  files_created: 4
  files_modified: 2
---

# Phase 10 Plan 04: Copilot + Gemini + Antigravity + Windsurf Installers Summary

Adds the four remaining Tier-1 documented harness installers that use NON-Claude deny families: Copilot (flat permissionDecision), Gemini CLI (decision/reason), Antigravity (dual-defensive), and Windsurf (exit-2-only). Each wires `beekeeper check --hook <name>` so the Plan 01 RenderDeny adapter emits the right per-family signal on block. HPC-02 + HPC-03 requirements fulfilled for these four harnesses.

## Tasks Completed

| # | Name | Commit | Key Files |
|---|------|--------|-----------|
| 1 | Copilot + Antigravity installers (settings.json merge family) | `8962495` | copilot.go, antigravity.go, hooks.go |
| 2 | Gemini + Windsurf installers (typed-struct families) | `e90d944` | gemini.go, windsurf.go |
| 3 | Contract-shape tests for all four new harnesses | `083e155` | hooks_test.go (+600 lines) |

## What Was Built

### `internal/hooks/copilot.go`
Installs `beekeeper check --hook copilot` into `~/.copilot/settings.json` under the `preToolUse` event (camelCase — correct for Copilot, distinct from Cursor's `beforeShellExecution`). Uses the same `mergeClaudeHookEntry` / `removeClaudeHookEntry` / `editorinit.PatchSettings` merge trinity as `claude_code.go`. The flat `{"permissionDecision":"deny","permissionDecisionReason":"..."}` deny JSON is emitted at runtime by `RenderDeny(HarnessCopilot)`, not by the installer.

### `internal/hooks/antigravity.go`
Installs `beekeeper check --hook antigravity` into `~/.gemini/antigravity/hooks.json` under the `PreToolUse` event. Uses the same merge trinity. Project-local alternative `.agents/hooks.json` documented in a comment. The dual-defensive deny (both `decision:"deny"` and `permissionDecision:"deny"+denyReason`) is emitted at runtime by `RenderDeny(HarnessAntigravity)`.

### `internal/hooks/gemini.go`
Installs `beekeeper check --hook gemini` into `~/.gemini/settings.json` hooks array. Uses a typed `geminiHooksFile` struct with a `Hooks []geminiHookEntry` field (adapting from the `codexHooksFile` pattern). Hook entry: `{Event: "BeforeTool", Matcher: ".*", Command: "beekeeper check --hook gemini"}`. `containsGeminiHookByCommand` guards idempotency; filtered-append uninstall preserves foreign hooks.

### `internal/hooks/windsurf.go`
Installs `beekeeper check --hook windsurf` into `~/.codeium/windsurf/hooks.json` for all three event keys: `pre_run_command`, `pre_mcp_tool_use`, `pre_read_code`. Uses a typed `windsurfHooksFile` with `Hooks map[string][]windsurfHook`. Critical: on `runtime.GOOS == "windows"` sets the `PowerShell` field; on Linux/macOS sets `Command`. Windsurf is exit-2-ONLY (no stdout-JSON deny form); `RenderDeny(HarnessWindsurf)` returns nil Stdout accordingly.

### `internal/hooks/hooks.go` (modified)
Added constants: `TargetCopilot`, `TargetAntigravity`, `TargetGemini`, `TargetWindsurf`. Extended `fileTargets` and `allTargets` slices. Added `InstallTo` and `UninstallTo` switch cases for all four new targets. Updated unknown-target error strings.

### `internal/hooks/hooks_test.go` (modified, +600 lines)
Added: `TestInstallCopilot` (5 subtests), `TestInstallGemini` (4 subtests), `TestInstallAntigravity` (5 subtests), `TestInstallWindsurf` (5 subtests including `os_correct_key`), `TestInstallDispatchNewTargets` (4 targets). Every installer test has `preserves_existing_hooks` (or `preserves_foreign_hooks`) and `idempotent` subtests. The Windsurf `os_correct_key` subtest asserts `PowerShell` field on Windows and `Command` field elsewhere.

## Verification Results

```
go build ./...                          PASS
GOOS=windows go build ./internal/hooks/ PASS  
go vet ./internal/hooks/               PASS
go test ./internal/hooks/ -count=1     PASS (all tests including 18 new)
```

## Deviations from Plan

None — plan executed exactly as written.

The only notable implementation decision was the Gemini file structure: Gemini hooks live inside `settings.json` as a top-level `hooks` array (not under a nested `hooks.{PreToolUse:...}` map). The installer reads and writes the entire `settings.json` as a `geminiHooksFile` struct, which correctly handles the `hooks` array alongside any other top-level keys in the file.

## Threat Mitigations Applied

| Threat | Mitigation Applied |
|--------|-------------------|
| T-10-13: Copilot flat vs nested deny | RenderDeny(HarnessCopilot) emits FLAT `permissionDecision` (Plan 01); installer wires `--hook copilot` |
| T-10-14: Windsurf fail-open + OS key | RenderDeny(HarnessWindsurf) emits exit 2 nil-stdout; installer uses `powershell` key on Windows; `os_correct_key` test asserts |
| T-10-15: Antigravity ambiguous deny | RenderDeny(HarnessAntigravity) emits BOTH deny fields; installer wires `--hook antigravity` |
| T-10-16: Clobber user hooks | Copilot/Antigravity: merge trinity; Gemini/Windsurf: filtered-append; all have `preserves_*_hooks` test subtests |

## Known Stubs

None. All four installers wire real commands into real config file locations with real deny families. No placeholder data.

## Threat Flags

None. No new network endpoints, auth paths, file access outside the four declared config file locations, or schema changes at trust boundaries. All config file locations are documented in RESEARCH.md.

## Self-Check: PASSED

| Item | Status |
|------|--------|
| internal/hooks/copilot.go | FOUND |
| internal/hooks/antigravity.go | FOUND |
| internal/hooks/gemini.go | FOUND |
| internal/hooks/windsurf.go | FOUND |
| Commit 8962495 (Task 1) | FOUND |
| Commit e90d944 (Task 2) | FOUND |
| Commit 083e155 (Task 3) | FOUND |
| go build ./... | PASS |
| GOOS=windows go build ./internal/hooks/ | PASS |
| go test ./internal/hooks/ -count=1 | PASS |
