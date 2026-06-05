---
phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement
plan: "03"
subsystem: hooks/installer
tags: [hooks, installer, cursor, codex, augment, codebuddy, qwen, contract-shape-tests]
dependency_graph:
  requires: ["10-01"]
  provides: ["HPC-02", "HPC-03"]
  affects: ["internal/hooks", "cmd/beekeeper"]
tech_stack:
  added: []
  patterns:
    - "Claude merge-not-clobber trinity (mergeClaudeHookEntry/removeClaudeHookEntry/editorinit.PatchSettings) extended to Augment, CodeBuddy, Qwen"
    - "ensureCodexFeaturesFlag: targeted TOML string-patch without new library dependency"
    - "cursorEvents slice replacing single 'preToolUse' string for correct Cursor v1.7+ event iteration"
key_files:
  created:
    - internal/hooks/augment.go
    - internal/hooks/codebuddy.go
    - internal/hooks/qwen.go
  modified:
    - internal/hooks/cursor.go
    - internal/hooks/codex.go
    - internal/hooks/hooks.go
    - internal/hooks/hooks_test.go
    - cmd/beekeeper/main.go
decisions:
  - "Cursor event fix: replaced single 'preToolUse' key (nonexistent in Cursor) with three real events beforeShellExecution/beforeMCPExecution/beforeReadFile; FailClosed:true retained (Cursor is fail-OPEN by default)"
  - "ensureCodexFeaturesFlag uses targeted line/section string patching — no new TOML library; preserves all existing content; idempotent under all four entry conditions (absent, no-features, features-no-hooks, already-correct)"
  - "Augment/CodeBuddy/Qwen reuse the mergeClaudeHookEntry/removeClaudeHookEntry trinity from claude_code.go; beekeeperClaudePreEntryWith/beekeeperClaudePostEntryWith helpers added in augment.go for shared use"
  - "installCodex calls ensureCodexFeaturesFlag after hooks.json write; backup of config.toml taken before flag patch"
  - "TargetAugment/TargetCodeBuddy/TargetQwen constants added to hooks.go; fileTargets and allTargets extended; InstallTo/UninstallTo switch cases added; error strings updated"
metrics:
  duration: "~45 minutes"
  completed: "2026-06-05T11:55:00Z"
  tasks_completed: 3
  tasks_total: 3
  files_changed: 8
---

# Phase 10 Plan 03: Installer Correctness (Cursor+Codex Bugs + Augment/CodeBuddy/Qwen) Summary

Fixed two buggy Tier-1 installers (Cursor nonexistent-event bug, Codex missing `[features] hooks=true`) and added three new Claude-schema-family installers (Augment, CodeBuddy, Qwen) with full merge-not-clobber safety and contract-shape test coverage.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Fix Cursor event-name bug + Codex features flag | 95ee9ed | cursor.go, codex.go |
| 2 | Add Augment/CodeBuddy/Qwen installers + dispatch | a4211d0 | augment.go, codebuddy.go, qwen.go, hooks.go, main.go |
| 3 | Contract-shape tests for fixed + new Tier-1 installers | 978fc4e | hooks_test.go |

## What Was Built

### Task 1: Installer bug fixes

**cursor.go** — The installer previously wrote `"preToolUse"` as the event key, which does not exist in Cursor. This silently disabled the hook — it never fired. Fix replaces the single key with a `cursorEvents` slice iterating all three real Cursor v1.7+ events: `beforeShellExecution`, `beforeMCPExecution`, `beforeReadFile`. Each event gets the beekeeper hook entry with `FailClosed: true` (required since Cursor is fail-OPEN by default). Command updated to `"beekeeper check --hook cursor"`. `uninstallCursor` now iterates all three event keys.

**codex.go** — The installer wrote `"beekeeper check"` (exits 1 on block, which Codex ignores) and never ensured `[features] hooks=true` in `config.toml` (required by Codex PR #18385; without it hooks are silently ignored). Two fixes:
1. Command updated to `"beekeeper check --hook codex"`.
2. `codexConfigPath(homeDir)` and `ensureCodexFeaturesFlag(configPath, out)` helpers added. The flag patcher uses careful line/section string scanning (no new TOML library) and is idempotent under all four conditions: file absent, file present without `[features]`, `[features]` present without `hooks`, already correct.

### Task 2: New installers

**augment.go**, **codebuddy.go**, **qwen.go** — Three new installers for the Claude-schema harnesses (identical nested `hookSpecificOutput.permissionDecision:"deny"` contract). Each file provides `installXxx`/`uninstallXxx` using `editorinit.ReadSettings` → `mergeClaudeHookEntry` → `backupSettings` → `editorinit.PatchSettings("hooks", ...)`. Shared helpers `beekeeperClaudePreEntryWith` and `beekeeperClaudePostEntryWith` added in augment.go to avoid duplicating the entry shape.

**hooks.go** — `TargetAugment`, `TargetCodeBuddy`, `TargetQwen` constants added. `fileTargets` and `allTargets` extended. `InstallTo`/`UninstallTo` switch cases wired. Path helpers defined in respective files. Error strings for unknown targets updated.

**cmd/beekeeper/main.go** — Two `--target` flag usage strings updated to list augment, codebuddy, qwen (only the two flag strings; `newCheckCmd` untouched per plan constraint).

### Task 3: Contract-shape tests

**TestInstallCursor** — Existing subtests fixed (command, event key assertions). New `correct_event_names` subtest: asserts `"preToolUse"` is absent and all three real events are present with the correct command and `failClosed:true` (T-10-09 regression gate).

**TestUninstallCursor** — Rewritten with `removes_from_all_events` and `preserves_foreign_hooks` subtests. Verifies beekeeper hook removed from all three events; foreign `"preToolUse"` data preserved.

**TestInstallCodex** — Existing subtests fixed (command string). Six new config.toml subtests: absent→created, existing no-features, existing features-no-hooks, already-correct; plus idempotency assertion (T-10-11 regression gate).

**TestInstallAugment**, **TestInstallCodeBuddy**, **TestInstallQwen** — Five subtests each: `from_absent`, `preserves_existing_hooks` (seeds foreign PreToolUse entry, asserts it survives), `idempotent`, `dry_run`, `uninstall_only_removes_beekeeper` (T-10-12 regression gate).

## Verification Results

```
go build ./... → clean
go vet ./internal/hooks/ → clean
go test ./internal/hooks/ -count=1 → PASS (all 46 test functions / subtests green)
go test ./cmd/... -count=1 → PASS
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Added beekeeperClaudePreEntryWith/beekeeperClaudePostEntryWith helpers in augment.go**
- **Found during:** Task 2
- **Issue:** The plan said to reuse the shared claude_code.go helpers, but `beekeeperClaudePreEntry()` and `beekeeperClaudePostEntry()` in claude_code.go return entries using the claude-specific `claudePreCommand`/`claudePostCommand` constants. To call them from augment/codebuddy/qwen with different command strings, parametric helpers were needed.
- **Fix:** Added `beekeeperClaudePreEntryWith(cmd)` and `beekeeperClaudePostEntryWith(cmd)` in augment.go. The merge/uninstall helpers (`mergeClaudeHookEntry`, `removeClaudeHookEntry`) are reused directly as planned.
- **Files modified:** `internal/hooks/augment.go`
- **Commit:** a4211d0

**2. [Rule 1 - Bug] Fixed existing TestInstallCursor and TestInstallCodex tests**
- **Found during:** Task 3
- **Issue:** The existing tests asserted the OLD (buggy) behavior — `preToolUse` event key and `"beekeeper check"` command. After Task 1 fixed the installers, the old tests became regressions.
- **Fix:** Updated assertions in `TestInstallCursor/from_absent`, `TestInstallCursor/merge_with_existing`, `TestUninstallCursor`, `TestInstallCodex/from_absent`, `TestInstallCodex/merge_with_existing` to match the correct post-fix behavior.
- **Files modified:** `internal/hooks/hooks_test.go`
- **Commit:** 978fc4e

## Threat Mitigations Shipped

| Threat | Mitigation |
|--------|-----------|
| T-10-09 (Cursor preToolUse → hook never fires) | `correct_event_names` test asserts preToolUse is ABSENT and three real events are present; regression can never ship again |
| T-10-10 (Cursor fail-open on crash) | `FailClosed: true` in every cursorHook entry; test asserts it |
| T-10-11 (Codex hook disabled without features flag) | `ensureCodexFeaturesFlag` writes `[features] hooks=true` idempotently; six test cases cover all entry conditions |
| T-10-12 (installer clobber user hooks) | All three new harnesses use `mergeClaudeHookEntry` + `editorinit.PatchSettings`; `preserves_existing_hooks` subtest seeds foreign hook and asserts survival |

## Known Stubs

None. All installers write the correct config and the tests assert the exact contract.

## Self-Check: PASSED

- `internal/hooks/augment.go` — FOUND
- `internal/hooks/codebuddy.go` — FOUND
- `internal/hooks/qwen.go` — FOUND
- `internal/hooks/cursor.go` contains `beforeShellExecution` — FOUND
- `internal/hooks/codex.go` contains `ensureCodexFeaturesFlag` — FOUND
- Commit 95ee9ed — FOUND
- Commit a4211d0 — FOUND
- Commit 978fc4e — FOUND
