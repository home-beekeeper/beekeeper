# Phase 10 — CONTEXT

**Phase:** 10 — Hook-Block Protocol Compliance & Multi-Harness Enforcement
**Milestone:** seeds v1.3.0 (proposed). v1.2.0 archived (6–9), v1.1.0 parked.
**Created:** 2026-06-05 (from a live dogfood of shipped v1.2.0).
**Primary research:** `10-RESEARCH.md` (this folder) — READ FIRST. Has the bug, live evidence, the full 15-harness deny-contract matrix, and citations. This CONTEXT is the summary; the research doc is the source of truth.
**Live-test resume (HPC-04):** `10-LIVE-TEST-RESUME.md` (this folder) — exact procedure + machine state to re-prove the block live on Claude Code (canary-safe), the audit-log smoking-gun check, and a note that **CI does NOT test harness honoring** (local + Claude-Code-only).

## Why this phase exists (the vision)

Beekeeper is sold as a *runtime safety harness that blocks* hijacked/off-task agents. A live dogfood proved the headline is currently false on the hook path: **`beekeeper check` fires, evaluates, decides "block" — and the agent runs the tool anyway.** Audit log showed a PreToolUse `block` immediately followed by a PostToolUse `tool_result` (the tool ran). Root cause: it exits `1` and emits its own `{"Allow":false,...}` JSON; **no agent harness honors exit 1 as deny**, and none recognize Beekeeper's JSON. Every harness needs **exit code 2** or a per-harness deny JSON (`hookSpecificOutput.permissionDecision:"deny"`, `permission:"deny"`, `decision:"deny"`, `cancel:true`, etc.).

"Done" = on Claude Code (the only locally-installed harness) a credential-read tool call is **actually denied end-to-end** (tool never executes), proven live; the `--hook` adapter emits the right deny per harness (unit-tested); installers are correct per harness; no-hook harnesses are honestly routed to the gateway; and a release gate asserts the *harness* deny contract so exit-1 can never silently ship again.

## Locked technical decisions / constraints

1. **Universal baseline = exit 2 + stderr reason.** Blocks on 11/13 hook-capable harnesses. Add per-harness JSON on top for richness. Two exceptions: **Hermes** is fail-OPEN (ignores exit codes → MUST emit block-JSON), **OpenCode** is a plugin API (throw, not a CLI hook).
2. **Don't break the default mode.** `beekeeper check` with NO `--hook` flag must keep current behavior: raw Decision JSON to stdout, exit 0 (allow) / **1** (block). The shim relies on exit 1; tests assert it; gateway is a separate path. Only `--hook <harness>` changes output/exit.
3. **Non-block must not over-allow.** On allow/warn the hook exits 0 and emits nothing harness-specific — defer to the harness's own permission flow. Never emit `permissionDecision:"allow"` (that would bypass the user's normal approvals).
4. **Installer correctness is part of the bug.** Cursor's installer writes a non-existent event `preToolUse` (real events: `beforeShellExecution`/`beforeMCPExecution`/`beforeReadFile`) → never fires. Codex needs `[features] hooks=true`. Every settings.json-style installer must MERGE (not clobber) + targeted-uninstall (Claude Code already fixed this session — replicate the pattern).
5. **No-hook harnesses → MCP gateway, documented honestly.** Kilo, Trae have no external pre-exec hook (only MCP-tool interception via the gateway; native tools unguarded). OpenCode = ship a JS plugin that shells to `beekeeper check` and throws (caveat: subagent/MCP bypass). Publish a Tier 1/2/3 support matrix — no overclaiming.
6. **Windows caveat:** Cline hooks are macOS/Linux only — won't work on the Windows-primary dev box. Document it.
7. **Architecture:** keep the deny-rendering as a pure, table-driven function in `internal/check` (testable); `cmd/beekeeper` stays thin Cobra wiring (`--hook` flag → render → exit).

## Prerequisite fixes already done & tested this session (UNCOMMITTED, on `main`)

Fold into this phase (or commit first). All gates green (`go build ./...`, full `go test ./...`, `-tags e2e`, overlay-downgrade gate):
- `internal/nudge/detect.go` — `detectionTimeout` 2s→3s (Windows pnpm.cmd; 0/24 misses, was 4/24).
- `internal/check/handler.go` — `execTimeout` 5s→8s (preserve nudge fail-open budget).
- `internal/hooks/claude_code.go` + `hooks_test.go` — installer clobber→**merge** + targeted uninstall + `TestInstallClaudeCodePreservesExistingHooks`.

## Scope guidance

15 harnesses × (contract + installer + test) is multi-session — **expect to split into ≥2 plans/phases.** Suggested order: (1) the `--hook` adapter + Claude Code installer + the missing release gate + live Claude Code re-proof (the verifiable core); (2) the Tier-1 documented harnesses (Codex, Cursor, Augment, CodeBuddy, Qwen, Gemini, Copilot, Antigravity, Windsurf); (3) the long tail (Hermes fail-open, Cline/Windows, OpenCode plugin, Kilo/Trae gateway routing + support-matrix docs).

## Open questions for planning

- Milestone framing: keep as standalone Phase 10, or run `/gsd-new-milestone` (v1.3.0) and reparent? (Currently marked "v1.3.0 (next)" in ROADMAP.)
- `--hook` value naming: match installer target names (`claude-code`, `cursor`, `codex`, …).
- Antigravity deny field is MED-confidence (docs conflict) — emit both `decision:"deny"` and `permissionDecision:"deny"`+`denyReason` defensively until verified on a live install.
- Only Claude Code is installed locally; all other harness installers are implemented against documented contracts (unverified live) — reflect that in the support matrix + tests (contract-shape tests, not live-harness tests).
