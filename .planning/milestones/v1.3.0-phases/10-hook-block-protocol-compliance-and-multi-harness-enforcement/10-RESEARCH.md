# Research: Hook Enforcement Protocol — Multi-Harness Deny Contracts

**Date:** 2026-06-05
**Source:** Live dogfood of shipped v1.2.0 on Windows (acting as the guarded agent) + 5 parallel doc-research agents.
**Status:** Research complete. Feeds the inserted GSD phase "Hook-Block Protocol Compliance & Multi-Harness Enforcement". NOT yet implemented.

---

## 1. The critical bug (why this phase exists)

`beekeeper check` (the PreToolUse hook) **fires, evaluates, and decides "block" — but the tool runs anyway** on Claude Code. Proven live:

- Audit log showed a PreToolUse `policy_decision=block` (`/.ssh/`) immediately followed by a PostToolUse `tool_result` record — the PostToolUse record only exists if the tool executed.
- The canary `cat ~/.ssh/...` printed `No such file` (it ran).

**Root cause:** `beekeeper check` exits **`1`** on block (`internal/check/handler.go: exitBlock=1` → `cmd/beekeeper/main.go: os.Exit(result.ExitCode)`) and prints its own `{"Allow":false,"Level":"block",...}` JSON. **No agent harness honors exit `1` as a deny, and none recognize Beekeeper's custom JSON.** Claude Code (and every harness below) treats exit `1` as a *non-blocking* soft error → the tool proceeds. Confirmed authoritative via claude-code-guide + official docs.

**Blast radius:** the hook-block path is non-functional on EVERY hook-based harness it's wired into (Claude Code, Cursor, Codex today; all future). No unit/e2e test caught it because they assert `ExitCode==1` (correct for Beekeeper's *internal* contract, wrong for *harness* hook protocols). Slipped the v1.0.0 + v1.2.0 release gates.

**Unaffected paths (still enforce):** the **shim** layer (Unix `exit 1` = block the wrapped `npm`/`pip` process — correct there) and the **MCP gateway** (intercepts in-flight, not via exit code). The *hook* path — the primary, most-advertised integration — is the broken one.

### Universal fix primitive
**Exit code `2` + reason on stderr blocks on 11 of the 13 hook-capable harnesses** (all except Hermes which is fail-open, and OpenCode which is a plugin API). Most also accept a richer per-harness JSON deny. So: add a `beekeeper check --hook <harness>` mode that emits **exit 2 + stderr baseline** plus the per-harness JSON; keep the default (no flag) at exit-1 raw for shim/gateway/tests.

---

## 2. The 15-harness interception matrix

Legend: **HOOK** = real external pre-exec deny hook; **PLUGIN** = code-plugin deny; **MCP-ONLY** = no hook, intercept via MCP gateway; **NONE** = no external interception.

| # | Harness | Config dir | Verdict | Pre-exec event | Deny contract | Fail mode / caveats |
|---|---------|-----------|---------|----------------|---------------|---------------------|
| 1 | **Claude Code** | `~/.claude` | HOOK | `PreToolUse` (settings.json) | exit **2** + stderr, OR stdout `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"…"}}` | hooks reload mid-session. **Locally testable (only one).** |
| 2 | **Antigravity** | `~/.gemini/antigravity` | HOOK | `PreToolUse` | exit **2**, OR stdout deny — field name **MED-confidence** (`decision:"deny"` vs `permissionDecision:"deny"` vs SDK `allow:false`+`denyReason`); emit both defensively | hooks.json lives in `.agents/hooks.json` or `~/.gemini/config/hooks.json`; `request-review`+`overwrite` bug → prefer hard deny |
| 3 | **Augment** | `~/.augment` | HOOK | `PreToolUse` | exit **2** + stderr, OR stdout `hookSpecificOutput.permissionDecision:"deny"` (Claude-identical) | layered settings.json; matchers support `mcp:*` |
| 4 | **Cline** | `.clinerules` | HOOK | `PreToolUse` script (NOT `.clinerules` text) | stdout `{"cancel":true,"errorMessage":"…"}` (exit 0), OR exit **2** | hook = executable file named `PreToolUse` (no ext) in `.clinerules/hooks/` or `~/Documents/Cline/Rules/Hooks/`. **macOS/Linux ONLY — no Windows support.** `.clinerules` files are prompt text only. |
| 5 | **CodeBuddy** | `~/.codebuddy` | HOOK | `PreToolUse` (settings.json) | exit **2**, OR stdout `hookSpecificOutput.permissionDecision:"deny"` | Claude-Code clone |
| 6 | **Codex** | `~/.codex` | HOOK | `PreToolUse` | exit **2** + stderr, OR stdout `hookSpecificOutput.permissionDecision:"deny"` (legacy `{"decision":"block"}`) | **needs `[features] hooks=true`** in config.toml; non-Bash/MCP coverage gated on PR #18385 (~Apr 2026); repo-local interactive bug #17532 |
| 7 | **Copilot** | `~/.copilot` | HOOK | `preToolUse` | stdout **FLAT** `{"permissionDecision":"deny","permissionDecisionReason":"…"}` (NOT nested), OR exit **2** | command hooks fail-CLOSED; `~/.copilot/settings.json` or `.github/hooks/*.json`; `COPILOT_HOME` override |
| 8 | **Cursor** | `~/.cursor` | HOOK | **`beforeShellExecution` / `beforeMCPExecution` / `beforeReadFile`** | stdout `{"permission":"deny","user_message":"…","agent_message":"…"}`, OR exit **2** | **fail-OPEN by default → must set `"failClosed":true`.** ⚠️ **Current beekeeper installer writes `preToolUse` — an event that does NOT exist in Cursor → hook never fires.** Must fix event names. v1.7+. |
| 9 | **Gemini CLI** | `~/.gemini` | HOOK | `BeforeTool` (settings.json `hooks`) | exit **2** + stderr, OR stdout `{"decision":"deny","reason":"…"}` (Gemini-native — **NOT** hookSpecificOutput); `"continue":false` aborts loop | matcher = regex on tool name |
| 10 | **Hermes** | `~/.hermes` | HOOK (**fail-OPEN**) | `pre_tool_call` (config.yaml YAML) | stdout `{"action":"block","message":"…"}` or `{"decision":"block","reason":"…"}` — **non-empty message required** | ⚠️ **non-zero exit / timeout / bad-JSON does NOT block.** Exit codes ignored → MUST emit block-JSON. Per-(event,command) consent allowlist. Gateway is more robust here. (= NousResearch/hermes-agent) |
| 11 | **Kilo** | `~/.config/kilo` | **MCP-ONLY** | — (no hooks; open FR #5827) | — | only built-in UI perms; intercept MCP tools via `mcp` in `kilo.json`; built-in Bash/file tools UNGUARDED |
| 12 | **OpenCode** | `~/.config/opencode` | **PLUGIN** | `tool.execute.before` (JS/TS plugin) | **`throw new Error(...)`** inside the plugin hook (no exit/JSON contract) | ship a plugin in `~/.config/opencode/plugins/` that shells to `beekeeper check` and throws on deny. ⚠️ does NOT catch subagent `task` calls (#5894) or (historically) MCP calls (#2319) |
| 13 | **Qwen Code** | `~/.qwen` | HOOK | `PreToolUse` (settings.json) | exit **2** + stderr, OR stdout `hookSpecificOutput.permissionDecision:"deny"` (Claude-identical) | Gemini-CLI fork that adopted Claude's schema |
| 14 | **Trae** | `~/.trae` | **MCP-ONLY** | — (no programmatic hook) | — | native cmds gated only by interactive "Auto-run & security"; intercept MCP tools via `~/.trae/mcp.json`. `.rules` = text only |
| 15 | **Windsurf** | `~/.codeium/windsurf` | HOOK | `pre_run_command` / `pre_mcp_tool_use` / `pre_read_code` / … (hooks.json) | **exit `2` ONLY** (no stdout-JSON deny form); stderr shown | fail-OPEN on non-2 exit; Windows uses `powershell` key |

**Local reality:** only `~/.claude` exists on this machine. All other config dirs are absent (not installed) → only Claude Code is live-verifiable; the rest are implemented against documented contracts.

---

## 3. Deny-contract families (drives the `--hook` adapter design)

- **Exit-2 universal baseline:** blocks on Claude Code, Antigravity, Augment, Cline, CodeBuddy, Codex, Copilot, Cursor, Gemini CLI, Qwen, Windsurf. → emit `exit 2` + reason to stderr always.
- **Nested `hookSpecificOutput.permissionDecision:"deny"`:** Claude Code, Codex, CodeBuddy, Augment, Qwen.
- **Flat `permissionDecision:"deny"`:** Copilot.
- **`permission:"deny"` (+user_message/agent_message):** Cursor.
- **`decision:"deny"`/`reason`:** Gemini CLI (Gemini-native); Antigravity (ambiguous — emit both forms).
- **`cancel:true`/`errorMessage`:** Cline.
- **`action`/`decision`:"block" + message/reason (REQUIRED; exit ignored):** Hermes (fail-open — JSON is the ONLY block path).
- **throw Error (plugin):** OpenCode.
- **No hook → MCP gateway:** Kilo, Trae (MCP tools only; native tools unguarded).

**Installer bugs to fix (beyond exit code):**
1. **Cursor:** event name `preToolUse` → must be `beforeShellExecution` (+ `beforeMCPExecution`, `beforeReadFile`); add `"failClosed":true` (already present); deny via `{"permission":"deny"}` / exit 2.
2. **Codex:** ensure `[features] hooks=true`; matcher; deny via exit 2 / hookSpecificOutput.
3. **Claude Code:** command → `beekeeper check --hook claude-code` (emit exit 2).
4. **Clobber-merge:** already fixed for Claude (this session); replicate merge-not-overwrite + targeted-uninstall for every settings.json-style installer.

**Honest support tiers for the docs/README:**
- **Tier 1 — full hook block (exit 2 + JSON), testable:** Claude Code.
- **Tier 1 — full hook block (documented, unverified locally):** Codex, Cursor, Augment, CodeBuddy, Qwen, Gemini CLI, Copilot, Antigravity, Windsurf.
- **Tier 2 — hook block with caveats:** Hermes (fail-open → JSON-only), Cline (no Windows), OpenCode (plugin + subagent bypass).
- **Tier 3 — MCP gateway only (no pre-exec hook):** Kilo, Trae (native tools unguarded; MCP tools via gateway).

---

## 4. Prerequisite fixes already done this session (uncommitted, on `main`)

These are complete + tested but NOT committed — fold into this phase or commit first:
- `internal/nudge/detect.go`: `detectionTimeout` 2s→3s (Windows pnpm.cmd; 0/24 misses, was 4/24).
- `internal/check/handler.go`: `execTimeout` 5s→8s (preserve nudge fail-open budget).
- `internal/hooks/claude_code.go` + `hooks_test.go`: installer clobber→**merge** fix (was overwriting the whole `hooks` key, would wipe the 11 GSD hooks) + targeted uninstall + `TestInstallClaudeCodePreservesExistingHooks`.

All gates green after these: `go build ./...`, full `go test ./...`, `-tags e2e` TestE2ELiveBinary, TestOverlayAllowCannotDowngradePathBlock.

---

## 5. Recommended phase shape (proposed requirements)

- **HPC-01** `beekeeper check --hook <harness>` flag: on block emit exit 2 + stderr reason (universal) + the per-harness JSON; non-block → exit 0 (defer to harness's own permission flow, do NOT emit `allow`); default (no flag) unchanged = raw Decision JSON + exit 1 (shim/gateway/tests). Pure, table-driven, unit-tested per harness.
- **HPC-02** Installer correctness per harness: correct command (`--hook <name>`), correct event name(s), config format/location, feature flags (Codex `[features] hooks=true`), merge-not-clobber + targeted uninstall for all settings.json-style targets. Add the 12 new targets.
- **HPC-03** Regression gate: per-harness tests asserting the exact deny output + exit code (the gate that was missing). Add a "hook protocol" table test.
- **HPC-04** Live re-verification on Claude Code: install via fixed installer, attempt a credential read, confirm it is actually DENIED (not just audited). (Only locally-testable harness.)
- **HPC-05** Gateway routing + honest docs: Kilo/Trae → MCP gateway path; OpenCode → ship a plugin; Hermes fail-open + Cline Windows caveat documented. Publish the Tier 1/2/3 support matrix in README/docs (no overclaiming).
- **HPC-06 (self-defense):** the missing release gate is itself the deliverable — a test that proves the *harness* deny contract, not just Beekeeper's internal exit code.

**Note:** likely splits into ≥2 phases during planning (adapter+gate+ClaudeCode live, then the long tail of harness installers + gateway/plugin paths). 15 harnesses × (research-confirmed contract + installer + test) is multi-session.

---

## 6. Source citations (per harness)
Claude Code: code.claude.com/docs/en/hooks · Cursor: cursor.com/docs/agent/hooks · Codex: developers.openai.com/codex/hooks (+ PR #18385, issue #17532) · Gemini CLI: geminicli.com/docs/hooks/reference · Antigravity: antigravity.google/docs/hooks (+ discuss.ai.google.dev overwrite bug) · Qwen: qwenlm.github.io/qwen-code-docs/en/users/features/hooks · CodeBuddy: codebuddy.ai/docs/cli/hooks · OpenCode: opencode.ai/docs/plugins (+ issues #5894, #2319) · Kilo: github.com/Kilo-Org/kilocode issue #5827 · Copilot: docs.github.com/en/copilot/reference/hooks-configuration · Augment: docs.augmentcode.com/cli/hooks · Hermes: github.com/NousResearch/hermes-agent hooks.md · Windsurf: docs.windsurf.com/windsurf/cascade/hooks · Cline: docs.cline.bot/customization/hooks · Trae: docs.trae.ai/ide (MCP only).
