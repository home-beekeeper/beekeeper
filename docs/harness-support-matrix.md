# Agent Harness Support Matrix

**Version:** Phase 10 (v1.3.0-seed), 2026-06-05, updated 2026-06-10 (added Continue + OpenClaw; 17 targets total, matching `internal/hooks/hooks.go` `allTargets`)
**Source:** 10-RESEARCH.md §2–3 (multi-harness deny-contract analysis); `internal/hooks/hooks.go`

---

## Overview

Beekeeper supports 17 agent harnesses across three support tiers. The tier
reflects how completely Beekeeper can intercept tool calls for that harness,
based on available upstream hook mechanisms.

**Coverage boundaries:**
- **Hook-based interception** (Tier 1/2): covers pre-exec tool calls. The harness
  must invoke the Beekeeper hook binary before executing each tool.
- **MCP gateway interception** (Tier 3): covers only MCP tools. Native built-in
  tools (Bash, file read/write, shell execution) bypass the gateway entirely.
- **No interception** (not implemented): harnesses without any hook mechanism and
  no MCP support.

---

## Harness Support Table

| # | Harness | Config Dir | Interception | Deny Mechanism | Tier | Caveats | Verification |
|---|---------|-----------|--------------|----------------|------|---------|--------------|
| 1 | **Claude Code** | `~/.claude` | HOOK (PreToolUse) | exit 2 + stderr; OR stdout `hookSpecificOutput.permissionDecision:"deny"` | **Tier 1** | Hooks reload mid-session; settings.json merge required | Live-verified (HPC-04) |
| 2 | **Codex** | `~/.codex` | HOOK (PreToolUse) | exit 2 + stderr; OR stdout `hookSpecificOutput.permissionDecision:"deny"` | **Tier 1** | Requires `[features] hooks=true` in config.toml; non-Bash/MCP coverage gated on PR #18385 | Documented contract |
| 3 | **Cursor** | `~/.cursor` | HOOK (beforeShellExecution / beforeMCPExecution / beforeReadFile) | exit 2 + stderr; OR stdout `{"permission":"deny","user_message":"...","agent_message":"..."}` | **Tier 1** | `failClosed:true` required (Cursor is fail-OPEN by default); three separate events (NOT preToolUse) | Documented contract |
| 4 | **Augment** | `~/.augment` | HOOK (PreToolUse) | exit 2 + stderr; OR stdout `hookSpecificOutput.permissionDecision:"deny"` | **Tier 1** | Matchers support `mcp:*`; layered settings.json | Documented contract |
| 5 | **CodeBuddy** | `~/.codebuddy` | HOOK (PreToolUse) | exit 2 + stderr; OR stdout `hookSpecificOutput.permissionDecision:"deny"` | **Tier 1** | Claude Code clone schema | Documented contract |
| 6 | **Qwen Code** | `~/.qwen` | HOOK (PreToolUse) | exit 2 + stderr; OR stdout `hookSpecificOutput.permissionDecision:"deny"` | **Tier 1** | Gemini-CLI fork that adopted Claude's schema | Documented contract |
| 7 | **Gemini CLI** | `~/.gemini` | HOOK (BeforeTool) | exit 2 + stderr; OR stdout `{"decision":"deny","reason":"..."}` | **Tier 1** | Gemini-native `decision` field (NOT `hookSpecificOutput`); matcher = regex on tool name | Documented contract |
| 8 | **Copilot** | `~/.copilot` | HOOK (preToolUse) | exit 2; OR stdout FLAT `{"permissionDecision":"deny","permissionDecisionReason":"..."}` | **Tier 1** | Flat JSON schema (NOT nested); `~/.copilot/settings.json` or `.github/hooks/*.json` | Documented contract |
| 9 | **Antigravity** | `~/.gemini/antigravity` | HOOK (PreToolUse) | exit 2; OR stdout `{"decision":"deny","permissionDecision":"deny","denyReason":"..."}` | **Tier 1** | Field name MED-confidence (docs conflict); Beekeeper emits both forms defensively | Documented contract |
| 10 | **Windsurf** | `~/.codeium/windsurf` | HOOK (pre_run_command / pre_mcp_tool_use / pre_read_code) | exit 2 ONLY (no stdout JSON deny form) | **Tier 1** | Fail-OPEN on non-2 exit; Windows uses `powershell` key; no JSON deny form | Documented contract |
| 11 | **Hermes** | `~/.hermes` | HOOK, fail-OPEN | stdout `{"action":"block","message":"..."}` ONLY; exit codes IGNORED | **Tier 2** | Non-zero exit / timeout / bad-JSON does NOT block; JSON is the ONLY block path; message field required non-empty | Documented contract |
| 12 | **Cline** | `.clinerules/hooks/` | HOOK (PreToolUse executable) | exit 2; OR stdout `{"cancel":true,"errorMessage":"..."}` | **Tier 2** | **macOS/Linux ONLY, no Windows support**; hook = executable file `PreToolUse` (no ext) in hooks dir | Documented contract |
| 13 | **OpenCode** | `~/.config/opencode` | PLUGIN (tool.execute.before) | `throw new Error(...)` inside JS plugin | **Tier 2** | Plugin does NOT catch subagent `task` calls (#5894) or historically MCP calls (#2319); Beekeeper ships a plugin to `~/.config/opencode/plugins/beekeeper.js` | Documented contract |
| 14 | **Kilo** | `~/.config/kilo` | MCP-GATEWAY-ONLY | MCP gateway intercept | **Tier 3** | **Native Bash/file tools UNGUARDED**: no pre-exec hook upstream (open FR #5827). Only MCP tools intercepted via gateway. | Documented contract |
| 15 | **Trae** | `~/.trae` | MCP-GATEWAY-ONLY | MCP gateway intercept | **Tier 3** | **Native shell/file tools UNGUARDED**: no programmatic pre-exec hook. Native commands gated only by Trae's "Auto-run & security" UI. Only MCP tools intercepted via gateway. | Documented contract |
| 16 | **Continue** | `~/.continue` | MCP-GATEWAY-ONLY | MCP gateway intercept | **Tier 3** | Wired via MCP client config (`~/.continue/config.yaml`, `mcpServers` streamable-http), NOT a pre-exec hook file. Only MCP tools routed through the gateway are intercepted; native/non-MCP tools are UNGUARDED. | Documented contract |
| 17 | **OpenClaw** | `~/.openclaw` | MCP-GATEWAY-ONLY | MCP gateway intercept | **Tier 3** | Wired via MCP client config (`~/.openclaw/config.json`), NOT a pre-exec hook file. Only MCP tools routed through the gateway are intercepted; native/non-MCP tools are UNGUARDED. | Documented contract |

---

## Tier Definitions

### Tier 1: Full hook-block (exit 2 + harness-specific deny JSON)

The harness has a pre-exec hook mechanism. Beekeeper installs a hook that
runs `beekeeper check --hook <name>` before each tool call. On block:
- Exits with code 2 (recognized as deny by the harness)
- Emits harness-specific deny JSON to stdout
- Emits human-readable reason to stderr

**Tier 1, locally live-verified:** Claude Code (verified on this machine via
HPC-04 live test; confirmed the pre-exec hook blocks a credential-read tool call).

**Tier 1, documented, unverified locally:** Codex, Cursor, Augment, CodeBuddy,
Qwen Code, Gemini CLI, Copilot, Antigravity, Windsurf. These are implemented
against documented contracts and validated by contract-shape unit tests, but
never tested against a running harness on this machine.

### Tier 2: Hook-block with caveats

The harness has a hook mechanism, but with a significant caveat that limits
coverage or reliability:

- **Hermes**, fail-OPEN harness: exit codes are ignored by Hermes. Block is
  only achieved by emitting `{"action":"block","message":"..."}` to stdout. Any
  hook timeout, crash, or non-JSON output causes Hermes to allow the tool call.
  MCP gateway is more robust for Hermes use cases.

- **Cline**, macOS/Linux ONLY: Cline's hook mechanism is not supported on
  Windows. The Beekeeper installer returns an explicit error on Windows. No
  Windows coverage for Cline hook blocking.

- **OpenCode**, plugin + bypass gaps: OpenCode uses a JS plugin API
  (`tool.execute.before`) rather than a CLI hook. The plugin does not intercept
  subagent `task` calls (issue #5894) and historically did not intercept MCP
  calls (issue #2319). Plugin-based block uses `throw new Error(...)`.

### Tier 3: MCP gateway only (native tools UNGUARDED)

These harnesses have no external pre-exec hook mechanism. Beekeeper cannot
install a binary hook that runs before each tool call.

- **Kilo**: no pre-exec hook (open upstream FR #5827). Native built-in tools
  (Bash, file read/write, shell commands) are **UNGUARDED**. Only MCP tools
  routed through the Beekeeper gateway (`http://127.0.0.1:7837/mcp`) are
  intercepted. Configure via `kilo.json`.

- **Trae**: no programmatic pre-exec hook. Native commands are gated only by
  Trae's interactive "Auto-run & security" UI. Beekeeper cannot intercept them.
  Native tools are **UNGUARDED**. Only MCP tools routed through the Beekeeper
  gateway are intercepted. Configure via `~/.trae/mcp.json`.

- **Continue**: integrated through its MCP client config
  (`~/.continue/config.yaml`, `mcpServers` streamable-http), not a pre-exec hook
  file. Beekeeper intercepts only the MCP tools Continue routes through the
  gateway; any native/non-MCP tools are **UNGUARDED**. `beekeeper hooks install
  --target continue` prints the config (no file is written).

- **OpenClaw**: integrated through its MCP client config
  (`~/.openclaw/config.json`), not a pre-exec hook file. Beekeeper intercepts only
  the MCP tools OpenClaw routes through the gateway; any native/non-MCP tools are
  **UNGUARDED**. `beekeeper hooks install --target openclaw` prints the config (no
  file is written).

---

## Honesty Notes

### 1. Only Claude Code is locally live-verified

The live-verified claim (HPC-04) means: on the development machine with
`~/.claude` installed, Beekeeper was confirmed to:
1. Fire the PreToolUse hook for a credential-read tool call
2. Block the tool call (tool did not execute, no PostToolUse record followed)
3. Audit the block in the NDJSON log

**The other 16 harnesses** are implemented against their published documentation
and source code. Each has contract-shape unit tests that verify:
- The installer writes the correct config format
- The deny output matches the harness-documented JSON schema
- The exit code is 2 (or 0 for Hermes) as required by each harness

These tests do NOT run a real harness. They verify Beekeeper's output matches
what each harness is documented to expect. Whether the harness actually honors
that contract is not tested in CI and is not tested locally (these harnesses are
not installed on the development machine).

### 2. CI does not test whether a harness honors the hook

The CI test suite (`go test -v ./...`) tests:
- `internal/hooks/...`: installer writes correct config
- `internal/check/...`: `beekeeper check` returns correct Decision and exit code

CI does NOT run any agent harness. It cannot verify that Claude Code, Cursor,
Codex, etc. actually block a tool call when `beekeeper check --hook <name>`
exits 2. That verification requires a running harness, which is local and
manual only.

The contract-shape tests (HPC-03/HPC-06) are the release gate for the deny
contract: they prove Beekeeper emits the right exit code and JSON. The
"harness honors it" check is Claude-Code-only and manual.

### 3. Cline is unsupported on Windows

The Cline hook mechanism requires an executable file at
`.clinerules/hooks/PreToolUse` (or `~/Documents/Cline/Rules/Hooks/PreToolUse`)
that is marked executable. Windows does not support Unix executable scripts
in this way. The Beekeeper Cline installer is guarded with `//go:build !windows`
and returns an explicit "macOS/Linux only" error on Windows.

### 4. Tier-3 native-tool gap (Kilo, Trae, Continue, OpenClaw)

The four Tier-3 targets are integrated through the MCP gateway, not a pre-exec
hook file (they are Beekeeper's `gatewayTargets`). This means:
- **Any native tool call** (Bash, file read, file write, shell command) that the
  agent makes outside MCP will NOT be intercepted by Beekeeper.
- Only MCP tool calls routed through the Beekeeper gateway are intercepted.
- For Kilo and Trae this is an upstream limitation (no pre-exec hook mechanism);
  for Kilo the upstream feature request is FR #5827. Continue and OpenClaw are
  wired via their MCP client config (`~/.continue/config.yaml`,
  `~/.openclaw/config.json`); `beekeeper hooks install` prints the config rather
  than writing a hook.
- Users of any Tier-3 target should be aware that Beekeeper provides PARTIAL
  coverage only. For full pre-exec coverage, use a Tier-1 harness.

### 5. Documented-contract caveat for all non-Claude-Code harnesses

All 16 non-Claude-Code harnesses are implemented based on:
- Official documentation from each harness vendor
- Source code review (where public)
- Corroborated across multiple sources

If a harness vendor changes their hook contract (event names, JSON schema, exit
code behavior), the Beekeeper installer and deny renderer may need updating.
Phase 10 documentation (10-RESEARCH.md) records the source citations and
confidence levels for each harness contract.

---

## Running `beekeeper hooks install`

```sh
# Install for a specific harness (use the target name from the Harness column):
beekeeper hooks install --target claude-code
beekeeper hooks install --target cursor
beekeeper hooks install --target codex
# ... etc.

# Dry-run (preview without writing):
beekeeper hooks install --target cursor --dry-run

# Uninstall (removes only Beekeeper entries, preserves other hooks):
beekeeper hooks uninstall --target cursor

# For Tier-3 targets (MCP gateway only — prints config instructions, no file written):
beekeeper hooks install --target kilo
beekeeper hooks install --target trae
beekeeper hooks install --target continue
beekeeper hooks install --target openclaw
```

---

## Source Citations

Claude Code: code.claude.com/docs/en/hooks | Cursor: cursor.com/docs/agent/hooks |
Codex: developers.openai.com/codex/hooks (+ PR #18385, issue #17532) |
Gemini CLI: geminicli.com/docs/hooks/reference |
Antigravity: antigravity.google/docs/hooks |
Qwen: qwenlm.github.io/qwen-code-docs/en/users/features/hooks |
CodeBuddy: codebuddy.ai/docs/cli/hooks |
OpenCode: opencode.ai/docs/plugins (+ issues #5894, #2319) |
Kilo: github.com/Kilo-Org/kilocode issue #5827 |
Copilot: docs.github.com/en/copilot/reference/hooks-configuration |
Augment: docs.augmentcode.com/cli/hooks |
Hermes: github.com/NousResearch/hermes-agent hooks.md |
Windsurf: docs.windsurf.com/windsurf/cascade/hooks |
Cline: docs.cline.bot/customization/hooks |
Trae: docs.trae.ai/ide (MCP only) |
Continue: docs.continue.dev/customize/deep-dives/mcp (MCP only) |
OpenClaw: docs.openclaw.ai/cli/mcp (MCP only)

**Research date:** 2026-06-05
