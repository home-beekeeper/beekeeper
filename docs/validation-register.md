# Validation Register (Tier C — manual live-block procedures)

**Phase 21 (VAL-06).** This is the honest, manual register for everything that
**cannot** be automatically validated: a true live block on each of the 16
non-Claude-Code harnesses, and the gated-22M-model LlamaFirewall end-to-end test.

Only **Claude Code** is live-block-verified by an automated test
(`internal/check/e2e_test.go::TestE2ELiveBinary/SPATH_hook_claude_code_exit2` —
the documented true-block reference, VAL-05). Every other harness is
**irreducibly manual**: it requires its real client installed and driven, so this
register records the exact procedure and a sign-off field rather than a CI gate.

See [validation-posture.md](validation-posture.md) for the Tier A/B/C model this
register is the Tier-C half of, and
[harness-support-matrix.md](harness-support-matrix.md) for the authoritative
per-harness tiers and deny mechanisms (the **Expected** rows below are sourced
from `internal/check/deny_render.go` and that matrix).

> **How to use:** install the real harness, run its **Install** step, perform the
> **Drive** action (a canary credential read — `~/.aws/credentials` or
> `~/.ssh/id_rsa`), confirm the **Expected** result, then fill **Result** and
> **Verified by / date**. An unchecked row is UNVERIFIED by design.

---

## Tier 1 — full hook-block (exit 2 + harness-specific deny JSON)

### Cursor (Tier 1)
- **Prereq:** Cursor installed; `~/.cursor` config dir; `failClosed:true` set (Cursor is fail-open by default).
- **Install:** `beekeeper hooks install --target cursor`
- **Drive:** have the agent read `~/.aws/credentials` (a `beforeReadFile` / `beforeShellExecution` event).
- **Expected:** exit 2 + stdout `{"permission":"deny","user_message":"...","agent_message":"..."}` (Family C); tool does not execute.
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### Codex (Tier 1)
- **Prereq:** Codex installed; `~/.codex`; `[features] hooks=true` in `config.toml`.
- **Install:** `beekeeper hooks install --target codex`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout `hookSpecificOutput.permissionDecision:"deny"` (Family A).
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### Augment (Tier 1)
- **Prereq:** Augment installed; `~/.augment`.
- **Install:** `beekeeper hooks install --target augment`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout `hookSpecificOutput.permissionDecision:"deny"` (Family A).
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### CodeBuddy (Tier 1)
- **Prereq:** CodeBuddy installed; `~/.codebuddy`.
- **Install:** `beekeeper hooks install --target codebuddy`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout `hookSpecificOutput.permissionDecision:"deny"` (Family A).
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### Qwen Code (Tier 1)
- **Prereq:** Qwen Code installed; `~/.qwen`.
- **Install:** `beekeeper hooks install --target qwen`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout `hookSpecificOutput.permissionDecision:"deny"` (Family A).
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### Gemini CLI (Tier 1)
- **Prereq:** Gemini CLI installed; `~/.gemini`.
- **Install:** `beekeeper hooks install --target gemini`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout `{"decision":"deny","reason":"..."}` (Family D, Gemini-native field).
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### Copilot (Tier 1)
- **Prereq:** Copilot CLI installed; `~/.copilot` (or `.github/hooks/*.json`).
- **Install:** `beekeeper hooks install --target copilot`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout FLAT `{"permissionDecision":"deny","permissionDecisionReason":"..."}` (Family B, not nested).
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### Antigravity (Tier 1)
- **Prereq:** Antigravity installed; `~/.gemini/antigravity`.
- **Install:** `beekeeper hooks install --target antigravity`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout dual `{"decision":"deny","permissionDecision":"deny","denyReason":"..."}` (Family E, emitted defensively).
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

### Windsurf (Tier 1)
- **Prereq:** Windsurf installed; `~/.codeium/windsurf`.
- **Install:** `beekeeper hooks install --target windsurf`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** **exit 2 ONLY** (no stdout JSON deny form — Family H). Windsurf is fail-open on a non-2 exit.
- **Result:** ☐ blocked  ☐ allowed (FAIL)
- **Verified by / date:** ______________

---

## Tier 2 — hook-block with caveats

### Hermes (Tier 2 — fail-OPEN seam)
- **Prereq:** Hermes installed; `~/.hermes`.
- **Install:** `beekeeper hooks install --target hermes`
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** **exit 0** + stdout `{"action":"block","message":"..."}` (Family G). Exit codes are IGNORED by Hermes; the JSON is the ONLY block path. A hook timeout / crash / non-JSON output ALLOWS the call (fail-open) — note this in the result.
- **Result:** ☐ blocked (JSON honored)  ☐ allowed (FAIL / fail-open)
- **Verified by / date:** ______________

### Cline (Tier 2 — macOS/Linux only)
- **Prereq:** Cline installed on **macOS or Linux** (Windows is unsupported — the installer errors); `.clinerules/hooks/` or `~/Documents/Cline/Rules/Hooks/`.
- **Install:** `beekeeper hooks install --target cline` (macOS/Linux only).
- **Drive:** agent reads `~/.aws/credentials`.
- **Expected:** exit 2 + stdout `{"cancel":true,"errorMessage":"..."}` (Family F).
- **Result:** ☐ blocked  ☐ allowed (FAIL)  ☐ N/A (Windows — unsupported)
- **Verified by / date:** ______________

### OpenCode (Tier 2 — plugin, bypass gaps)
- **Prereq:** OpenCode installed; `~/.config/opencode`. Beekeeper ships a JS plugin to `~/.config/opencode/plugins/beekeeper.js`.
- **Install:** `beekeeper hooks install --target opencode`
- **Drive:** agent reads `~/.aws/credentials` via a normal tool call (NOT a subagent `task` call — those bypass the plugin, #5894).
- **Expected:** exit 2 (plugin `throw new Error(...)`; Family H deny shape). Does not catch subagent `task` calls or (historically) MCP calls — note any bypass.
- **Result:** ☐ blocked  ☐ allowed (FAIL)  ☐ bypass (task/MCP)
- **Verified by / date:** ______________

---

## Tier 3 — MCP gateway only (native tools UNGUARDED)

> For all four Tier-3 targets, `beekeeper hooks install --target <name>` PRINTS
> the MCP client config (no hook file is written). Only MCP tool calls routed
> through the gateway (`http://127.0.0.1:7837/mcp`) are intercepted; **native
> Bash/file/shell tools are UNGUARDED**.

### Kilo (Tier 3 — native tools UNGUARDED)
- **Prereq:** Kilo installed; `~/.config/kilo`; `beekeeper gateway start` running.
- **Install:** `beekeeper hooks install --target kilo` (prints MCP config for `kilo.json`).
- **Drive:** (a) an **MCP-routed** credential-read tool call; (b) a **native** Bash `cat ~/.aws/credentials`.
- **Expected:** (a) MCP call blocked via the gateway; (b) native call **UNGUARDED — NOT blocked** (upstream FR #5827, no pre-exec hook).
- **Result:** ☐ MCP blocked  ☐ native UNGUARDED (expected)  ☐ unexpected
- **Verified by / date:** ______________

### Trae (Tier 3 — native tools UNGUARDED)
- **Prereq:** Trae installed; `~/.trae`; gateway running.
- **Install:** `beekeeper hooks install --target trae` (prints MCP config for `~/.trae/mcp.json`).
- **Drive:** (a) an MCP-routed credential read; (b) a native shell `cat ~/.aws/credentials`.
- **Expected:** (a) MCP call blocked; (b) native call **UNGUARDED** (gated only by Trae's "Auto-run & security" UI).
- **Result:** ☐ MCP blocked  ☐ native UNGUARDED (expected)  ☐ unexpected
- **Verified by / date:** ______________

### Continue (Tier 3 — native tools UNGUARDED)
- **Prereq:** Continue installed; `~/.continue`; gateway running.
- **Install:** `beekeeper hooks install --target continue` (prints `mcpServers` streamable-http config for `~/.continue/config.yaml`).
- **Drive:** (a) an MCP-routed credential read; (b) a native/non-MCP tool call.
- **Expected:** (a) MCP call blocked; (b) native/non-MCP call **UNGUARDED**.
- **Result:** ☐ MCP blocked  ☐ native UNGUARDED (expected)  ☐ unexpected
- **Verified by / date:** ______________

### OpenClaw (Tier 3 — native tools UNGUARDED)
- **Prereq:** OpenClaw installed; `~/.openclaw`; gateway running.
- **Install:** `beekeeper hooks install --target openclaw` (prints MCP config for `~/.openclaw/config.json`).
- **Drive:** (a) an MCP-routed credential read; (b) a native/non-MCP tool call.
- **Expected:** (a) MCP call blocked; (b) native/non-MCP call **UNGUARDED**.
- **Result:** ☐ MCP blocked  ☐ native UNGUARDED (expected)  ☐ unexpected
- **Verified by / date:** ______________

---

## Gated-model end-to-end (Tier C — human-gated)

### LlamaFirewall Llama-Prompt-Guard-2-22M e2e (Tier C — PENDING human HF-license gate)
- **Prereq:** accept the `meta-llama/Llama-Prompt-Guard-2-22M` license on huggingface.co; `huggingface-cli login`; run `beekeeper llamafirewall install` (bootstraps the CPU-only venv + pre-pulls the gated 22M model into `HF_HOME` under the Beekeeper state dir); set `BEEKEEPER_LLMF_E2E=1`.
- **Run:** `go test -tags e2e -run TestLlamaFirewallE2E ./internal/llamafirewall/`
- **Expected:** benign prompt → allow; prompt-injection → injection verdict; unsafe code → CodeShield unsafe; sidecar crash → fail-closed (never silently "clean").
- **Result / sign-off:** **PENDING** — Claude cannot accept the Llama license (a human-only web action). This entry may remain PENDING past phase close (D-07 / deferred); it is the only Tier-C item that is gated on an external human action.
- **Verified by / date:** ______________

---

## Sign-off summary

| Surface | Count | Status |
|---------|-------|--------|
| Claude Code live block | 1 | ✅ Automated (VAL-05 e2e — the true-block reference) |
| Non-Claude-Code harnesses | 16 | ☐ UNVERIFIED by design (manual; fill rows above) |
| Gated-22M-model LlamaFirewall e2e | 1 | ⏳ PENDING (human HF-license gate) |

*An unsigned row is honest: it means that harness has NOT been live-block-verified, only contract-shape unit-tested (`internal/hooks` installer conformance + `internal/check` golden deny contract). See [validation-posture.md](validation-posture.md).*
