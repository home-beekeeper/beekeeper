---
phase: quick-260623-w7y
plan: 01
subsystem: hooks
status: complete
tags: [hooks, fail-closed, abspath, migration, security]
key-decisions:
  - "Command format: quoted forward-slash absolute path — e.g. \"/path/to/beekeeper\" check --hook claude-code"
  - "Idempotency via stable suffix (check --hook <harness>) not full command — covers both old bare-name and new abspath forms"
  - "Migration: on re-install, replace stale entry in place rather than append or leave stale"
  - "BeekeeperHookMarkers() augmented with abspath-form markers (check --hook, audit-record) while retaining beekeeper check / beekeeper audit-record for backward compatibility"
  - "hookMarkerCount in conformance_test.go counts 'check --hook' (invariant in both raw forms) not 'beekeeper check' (absent from JSON-escaped abspath)"
  - "matchesBeekeeperCommand ANCHORED on the beekeeper program token (basename == beekeeper) not just the suffix substring — prevents migration from clobbering a third-party hook whose args coincidentally contain a weak suffix like audit-record (T-w7y-03)"
metrics:
  duration: "~3 hours (2 sessions: context cut + resume)"
  completed: "2026-06-24"
  tasks_completed: 3
  files_modified: 19
key-files:
  created:
    - internal/hooks/command.go
    - internal/hooks/command_test.go
  modified:
    - internal/hooks/claude_code.go
    - internal/hooks/augment.go
    - internal/hooks/antigravity.go
    - internal/hooks/copilot.go
    - internal/hooks/codebuddy.go
    - internal/hooks/qwen.go
    - internal/hooks/cursor.go
    - internal/hooks/codex.go
    - internal/hooks/gemini.go
    - internal/hooks/windsurf.go
    - internal/hooks/hermes.go
    - internal/hooks/cline.go
    - internal/hooks/cline_windows.go
    - internal/hooks/opencode_plugin.go
    - internal/hooks/protected.go
    - internal/hooks/conformance_test.go
    - internal/hooks/hooks_test.go
    - internal/hooks/edge_cases_test.go
    - internal/hooks/cline_test.go
---

# Quick Task 260623-w7y: Embed Absolute Binary Path in Agent Hook Commands

## One-liner

Eliminated the exit-127 fail-open vulnerability by embedding the running binary's absolute path (via os.Executable at install time, quoted+ToSlash'd) into all 13 harness hook commands, with stable-suffix-based idempotency, in-place migration of stale entries, and a green test suite proving abspath generation, fallback, migration, and foreign-hook preservation.

## Objective

Fix the fail-closed wiring gap: harness installers wrote the bare name `beekeeper` which resolves to exit 127 (command not found) when beekeeper is installed after the harness captures its PATH. Exit 127 is non-blocking — the agent runs unprotected. The fix embeds the absolute binary path so the hook resolves and runs regardless of PATH state.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Shared abspath command-builder + stable-suffix matcher | f818e0f | command.go, command_test.go |
| 2 | Route all 13 harness installers + suffix-based detect/migrate/uninstall | 3497f09 | 14 harness files + protected.go + conformance_test.go |
| 3 | Update + extend tests (migration, idempotency, dual-form uninstall, preservation) | 12e52c4 | hooks_test.go, edge_cases_test.go, cline_test.go |

## Hard Gates

All three hard gates passed on branch `fix/hook-abspath-failclose`:

```
go build ./...            -> exit 0  (BUILD: OK)
go vet ./...              -> exit 0  (VET: OK)
go test ./internal/hooks/ -count=1 -> PASS (61 tests, 0 failures)
```

## Implementation Summary

### Task 1 — Shared command builder (command.go)

- `var execResolver = os.Executable` — injectable test seam
- `resolveBeekeeperBin()` — calls execResolver, ToSlash+quote on success, returns `"beekeeper"` fallback on error
- `beekeeperCmd(args string)` — returns `<quoted-abspath> <args>`
- `matchesBeekeeperCommand(cmd, suffix string)` — stable-suffix anchor, matches both old bare-name and new abspath forms

### Task 2 — All 13 harness installers

Harness type groupings and changes:

**(a) JSON entry-array harnesses** (claude_code, augment, antigravity, copilot, codebuddy, qwen):
- Named command constants replaced by stable suffix constants (e.g. `claudeCheckSuffix = "check --hook claude-code"`)
- `claudeEntriesContainCommand`, `mergeClaudeHookEntry`, `removeClaudeHookEntry` now use `matchesBeekeeperCommand(c, suffix)`
- `mergeClaudeHookEntry` migrates stale entries in place (replaces bare-name command with new abspath command)

**(b) Cursor** — per-event struct, FailClosed:true preserved throughout; migration replaces stale `.Command` field

**(c) Codex** — per-event codexHookEntry struct; `ensureCodexFeaturesFlag` untouched; migration replaces stale command in inner `codexHookCmd`

**(d) Gemini** — flat array, migration replaces stale `geminiHookEntry.Command` in place

**(e) Windsurf** — OS-split Command/PowerShell fields; both fields checked by `matchesBeekeeperCommand`; Windows-uses-PowerShell-key invariant preserved; migration replaces whichever field is stale

**(f) Hermes** — YAML line scan; `hermesCheckSuffix = "check --hook hermes"`; `hasHermesBeekeeperHook` matches suffix in YAML line; `removeHermesBeekeeperHook` removes lines matching suffix; added `patchHermesConfigMigrate` for in-place abspath migration

**(g) Cline** — POSIX shell script via `clineScript()` function (now uses `beekeeperCmd`); `containsClineCommand` does per-line suffix scan; cline_windows.go stub updated to use `clineCheckSuffix`

**(h) OpenCode** — JS plugin template is now `openCodePluginContent()` function (embeds resolved abspath as spawnSync argv[0]); re-install rewrites content if different (migration; no early no-op)

**protected.go (Rule 1 fix)**: `BeekeeperHookMarkers()` augmented with `"check --hook"` and `"audit-record"` to detect abspath-form entries in raw file bytes (JSON-encoding produces `beekeeper\" check`, not `beekeeper check`). Legacy markers `"beekeeper check"` and `"beekeeper audit-record"` retained for backward compatibility.

**conformance_test.go (Rule 1 fix)**: `hookMarkerCount` now counts `"check --hook"` instead of `"beekeeper check"` — the former is present in both raw bare-name and JSON-escaped abspath forms; the latter is absent from abspath JSON files.

### Task 3 — Test updates + new tests

**Updated existing tests**: Replaced exact bare-name constant references (`augmentPreCommand`, `hermesPreCommand`, etc.) with new suffix constants. Replaced exact command equality assertions with `matchesBeekeeperCommand` or suffix-constant-based `claudeEntriesContainCommand` calls.

**New tests added**:
- `TestAbspathMigrationClaudeCode`: pre-seeded bare-name entry → exactly 1 abspath entry after re-install
- `TestAbspathMigrationCursor`: all 3 Cursor events migrated in place, FailClosed preserved
- `TestIdempotentReinstallWithAbspath`: 2nd install with same abspath → no duplicate
- `TestDualFormUninstallClaudeCode`: both bare-name and abspath forms removed by uninstall
- `TestPreservationRegressionClaudeCode`: foreign hook survives install + migration + uninstall
- `fakeExecResolver` helper: injects deterministic fake binary path via injectable seam

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] BeekeeperHookMarkers() incompatible with abspath JSON encoding**
- **Found during:** Task 2 implementation
- **Issue:** The plan required `"beekeeper check"` to be a literal substring of every produced command. With the abspath form `"/path/to/beekeeper" check`, the command string does NOT contain `beekeeper check` as a substring (the `"` separates them). JSON-encoding further transforms it to `beekeeper\" check` in raw file bytes, making the literal `"beekeeper check"` absent from any settings.json produced by an abspath install.
- **Fix:** Updated `BeekeeperHookMarkers()` to also return `"check --hook"` and `"audit-record"` — these ARE present in raw file bytes for both bare-name and abspath forms. Retained the original markers for backward compatibility with pre-existing bare-name installations.
- **Files modified:** `internal/hooks/protected.go`
- **Commit:** 3497f09

**2. [Rule 1 - Bug] conformance_test.go hookMarkerCount fails for abspath installs**
- **Found during:** Task 2 verification planning
- **Issue:** `hookMarkerCount` counted `"beekeeper check"` occurrences in raw file bytes. Abspath JSON produces `beekeeper\" check` (escaped inner quote), making the count 0 for every abspath-form install, which would cause `TestHarnessConformance` to fail the "at least 1 marker" assertion.
- **Fix:** Changed the marker string to `"check --hook"` which appears in both bare-name and abspath forms of every hook entry across all file formats (JSON, YAML, shell script).
- **Files modified:** `internal/hooks/conformance_test.go`
- **Commit:** 3497f09

**3. [Rule 1 - Bug] Test compilation failure — old constants removed from production, still referenced in tests**
- **Found during:** Task 2 → Task 3 transition
- **Issue:** `augmentPreCommand`, `codebuddyPreCommand`, `hermesPreCommand`, `clinePreCommand` etc. were removed from production files as part of Task 2. Test files still referenced them, causing `undefined: X` build errors.
- **Fix:** Replaced all references in test files with the new suffix constant names (`augmentCheckSuffix`, `hermesCheckSuffix`, etc.), and updated exact-equality assertions to suffix-based assertions.
- **Files modified:** `internal/hooks/hooks_test.go`, `internal/hooks/edge_cases_test.go`, `internal/hooks/cline_test.go`
- **Commit:** 12e52c4

### Post-review fix

**4. [Rule 1 - Bug] matchesBeekeeperCommand too loose → third-party hook clobber (review finding)**
- **Found during:** Coordinator code review after Task 3
- **Issue:** The shipped `matchesBeekeeperCommand` was `strings.HasSuffix(cmd, suffix) || strings.Contains(cmd, suffix)` — the `HasSuffix` clause was dead (subsumed by `Contains`), and the match was an UNANCHORED substring test. For the weak suffix `"audit-record"`, a user's unrelated PostToolUse hook like `audit-logger audit-record` matched. Because `mergeClaudeHookEntry` REPLACES any matched entry during migration, the installer would silently CLOBBER the user's own hook. This violated T-w7y-03 (the matcher was supposed to anchor on the beekeeper program token, not the suffix substring).
- **Fix:** Rewrote `matchesBeekeeperCommand` to (a) extract the leading program token (quoted abspath or first bare word), (b) require its basename to be `beekeeper` via new `programIsBeekeeper` (quotes/path/.exe stripped, lowercased — mirrors `internal/check/hookguard.go` `programBase`), and (c) require the remaining args to contain the stable suffix. Both real forms still match; third-party decoys no longer match. Hermes line-scan needed a dedicated `hermesLineIsBeekeeperCommand` to strip the `- command:` YAML prefix before anchoring. Added a package-wide `TestMain` so `execResolver` returns a beekeeper-named path (production reality; the test binary is `hooks.test`, not `beekeeper`).
- **New tests:** `anchoring_regression_decoys` table in command_test.go (third-party `audit-record` / `check --hook` decoys reject; bare + abspath forms match); strengthened `TestPreservationRegressionClaudeCode` to seed `linter check --hook ci` (Pre) + `audit-logger audit-record` (Post) and assert both survive install (incl. migration) AND uninstall — the test that catches the clobber.
- **Files modified:** `internal/hooks/command.go`, `internal/hooks/hermes.go`, `internal/hooks/command_test.go`, `internal/hooks/hooks_test.go`, `internal/hooks/edge_cases_test.go`
- **Commit:** 64b2e0b

## Self-Check

### Created files exist

- `internal/hooks/command.go` — present
- `internal/hooks/command_test.go` — present
- `.planning/quick/260623-w7y-embed-absolute-binary-path-in-agent-hook/260623-w7y-SUMMARY.md` — present (this file)

### Commits exist

- f818e0f — Task 1: feat(hooks): shared abspath command builder
- 3497f09 — Task 2: fix(hooks): embed absolute beekeeper path in all 13 harness installers
- 12e52c4 — Task 3: test(hooks): update assertions to suffix-match + add T3 migration tests
- 64b2e0b — Post-review: fix(hooks): anchor beekeeper command matcher to prevent third-party hook clobber

### Hard gate results (re-run after matcher-anchoring fix)

- `go build ./...` — exit 0 (BUILD: OK)
- `go vet ./...` — exit 0 (VET: OK)
- `go test ./internal/hooks/ -count=1` — PASS (273 test cases, 0 failures)
- `go test ./internal/check/` — PASS (cross-package consumer of BeekeeperHookMarkers)

## Self-Check: PASSED
