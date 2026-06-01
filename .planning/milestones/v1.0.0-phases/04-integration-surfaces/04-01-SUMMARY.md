---
phase: 04-integration-surfaces
plan: "01"
subsystem: hooks
tags: [hooks, integration, claude-code, cursor, codex, idempotency, backup]
dependency_graph:
  requires:
    - 03-editor-extension-defense (editorinit.PatchSettings — reused directly)
  provides:
    - internal/hooks package: Install/Uninstall dispatch for all 6 agent targets
  affects:
    - cmd/beekeeper/main.go (Plan 05 will wire Cobra subcommands)
tech_stack:
  added: []
  patterns:
    - editorinit.PatchSettings for JSONC-safe Claude Code settings.json writes
    - writeFileAtomic (temp-rename) for Cursor/Codex JSON writes
    - backupSettings before any file modification (timestamp-suffixed copy)
    - containsHookByCommand idempotency guard in all three file-writing installers
    - printGatewayGuide for gateway targets (no file I/O)
key_files:
  created:
    - internal/hooks/hooks.go
    - internal/hooks/claude_code.go
    - internal/hooks/cursor.go
    - internal/hooks/codex.go
    - internal/hooks/gateway_targets.go
    - internal/hooks/hooks_test.go
    - internal/hooks/testdata/claude_settings.json
    - internal/hooks/testdata/cursor_hooks.json
    - internal/hooks/testdata/codex_hooks.json
  modified: []
decisions:
  - "Cursor hooks.json uses preToolUse (camelCase) and failClosed:true — different schema from Claude Code's PascalCase hooks key"
  - "Codex hooks.json uses nested schema (codexHookEntry wraps inner codexHookCmd array) matching Claude Code structure but in a separate ~/.codex/ file"
  - "Gateway targets (continue, opencode, openclaw) receive only printed guide — no file write; printGatewayGuide dispatches per-target"
  - "backupSettings is a no-op for missing files (returns nil) so install on fresh machines does not fail"
  - "PatchSettings idempotency relies on overwriting the entire 'hooks' key — re-running replaces the key with identical content; no entry duplication possible"
  - "Cursor and Codex installers use containsCursorHookByCommand/containsCodexHookByCommand before append to preserve existing third-party hooks"
metrics:
  duration: "~18 minutes"
  completed: "2026-05-26T21:22:37Z"
  tasks_completed: 1
  tasks_total: 1
  files_created: 9
  files_modified: 0
---

# Phase 04 Plan 01: Hook Installer Package Summary

## One-liner

Hook installer for 6 agent targets — Claude Code via PatchSettings (JSONC-safe), Cursor/Codex via typed JSON writers with idempotency guards, gateway targets via printed MCP config guide.

## What Was Built

The `internal/hooks/` package implements `beekeeper hooks install/uninstall` business logic for all six supported agent integration targets. The Cobra CLI wiring is deferred to Plan 05.

### Package structure

| File | Purpose |
|------|---------|
| `hooks.go` | `Install`/`Uninstall` dispatch; `backupSettings`; `writeFileAtomic`; path helpers |
| `claude_code.go` | `installClaudeCode` — reuses `editorinit.PatchSettings` for JSONC-safe atomic write |
| `cursor.go` | `installCursor` — typed `cursorHooksFile` struct; `failClosed:true` enforced |
| `codex.go` | `installCodex` — nested `codexHooksFile` struct; trust reminder printed |
| `gateway_targets.go` | `printGatewayGuide` — Continue/OpenCode/OpenClaw config snippets printed |
| `hooks_test.go` | 9 tests: all targets, idempotency, dry-run, backup, merge, unknown-target error |
| `testdata/` | JSONC settings fixture + existing cursor/codex hooks fixtures for merge tests |

### Key behaviors verified

- **Claude Code** (`installClaudeCode`): reads JSONC via `PatchSettings`, merges `PreToolUse` + `PostToolUse`, atomic write, backup created, idempotent (re-running overwrites key with identical content).
- **Cursor** (`installCursor`): writes `~/.cursor/hooks.json` with `version:1`, `preToolUse` (camelCase), `failClosed:true`. Merges with existing hooks from other tools (e.g., `some-other-linter`). Never writes to editor preferences file.
- **Codex** (`installCodex`): writes `~/.codex/hooks.json` with nested schema (`PreToolUse`/`PostToolUse`). Prints trust reminder after write.
- **Gateway targets** (Continue, OpenCode, OpenClaw): no file written; formatted MCP config guide printed with port 7837 and `beekeeper gateway token` retrieval instruction.
- **Dry-run**: all file-writing installers print what would be written without modifying any file; no backup created.
- **Unknown target**: `Install` returns a descriptive error listing all valid targets.

## Deviations from Plan

None — plan executed exactly as written.

The plan's `printDryRun` helper signature was adapted from `printDryRun(path, label, value, out)` to pass `out io.Writer` consistently, and the formatted output is printed inline in each installer (cleaner than a generic helper with an opaque `any` value). This is a cosmetic implementation choice within the task's scope.

## Threat Mitigations Applied

All STRIDE threats from the plan's `<threat_model>` were addressed:

| Threat ID | Status | Implementation |
|-----------|--------|----------------|
| T-04-01-01 | Mitigated | `--target` is a closed enum; file paths derived from `os.UserHomeDir()` only |
| T-04-01-02 | Mitigated | `cursor.go` never constructs or references the editor preferences path (grep-verified) |
| T-04-01-03 | Mitigated | `backupSettings` called before every write; missing file returns nil |
| T-04-01-04 | Mitigated | `FailClosed: true` hardcoded in `cursor.go` struct literal |
| T-04-01-05 | Mitigated | `containsCursorHookByCommand`/`containsCodexHookByCommand` guard before every append |
| T-04-01-06 | Mitigated | `printGatewayGuide` prints only `beekeeper gateway token` command; no token value embedded |

## Known Stubs

None — all installers write real content. The gateway guide snippets use the configured default port (7837) which is the Plan 03/05 gateway default; this is intentional and documented in CONTEXT.md.

## Threat Flags

None — this plan only writes to user home directory config files (~/.claude/, ~/.cursor/, ~/.codex/). No new network endpoints, auth paths, or schema changes at trust boundaries.

## Self-Check: PASSED

Files created:
- internal/hooks/hooks.go: FOUND
- internal/hooks/claude_code.go: FOUND
- internal/hooks/cursor.go: FOUND
- internal/hooks/codex.go: FOUND
- internal/hooks/gateway_targets.go: FOUND
- internal/hooks/hooks_test.go: FOUND
- internal/hooks/testdata/claude_settings.json: FOUND
- internal/hooks/testdata/cursor_hooks.json: FOUND
- internal/hooks/testdata/codex_hooks.json: FOUND

Commits:
- a498830: feat(04-01): hook installer package — Install/Uninstall for all 6 targets: FOUND

Test results: 9/9 passed (all TestInstall* and TestUninstall* tests)
Build: go build ./... — PASSED
go vet ./internal/hooks/... — PASSED
