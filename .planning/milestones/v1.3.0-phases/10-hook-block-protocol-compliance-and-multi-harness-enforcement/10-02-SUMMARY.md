---
phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement
plan: 02
status: complete
result: PASS
completed: 2026-06-05
requirements: [HPC-04]
autonomous: false
tasks_completed: 2
---

# Plan 10-02 SUMMARY — Live Claude Code Block Re-Proof (HPC-04)

**Result: ✅ PASS — the hook BLOCKS end-to-end on Claude Code, proven live in a fresh session.**

This closes the exact gap the 2026-06-05 dogfood exposed: the PreToolUse hook now ACTUALLY
denies a credential-read tool call (the tool never executes), not merely audits it.

## Task 1 — CLI pre-flight of the `--hook` adapter (PASS)

Refreshed the PATH binary with the Phase-10 build (`go install ./cmd/beekeeper` → exit 0;
`beekeeper check --help` lists `--hook`). Catalog present at `%APPDATA%\beekeeper\catalogs\bumblebee.idx`.

Pre-flight (post the `f315c81` Hermes-leak fix):
- Benign `Read ./README.md` piped to `beekeeper check --hook claude-code` → **exit 0**, nothing harness-specific.
- Canary credential read (`/.ssh/…-canary-DOES-NOT-EXIST`) → **exit 2** and stdout is ONLY
  `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"sensitive path blocked: /.ssh/"}}`.
- `--hook hermes` on the same block → **exit 0** + stdout ONLY `{"action":"block","message":"sensitive path blocked: /.ssh/"}` (fail-open JSON-only path correct).
- Default path (no `--hook`) UNCHANGED: raw Decision JSON + exit 1.

## Task 2 — Live in-Claude-Code re-proof (PASS, human-verified)

Procedure (canary-safe, per `10-LIVE-TEST-RESUME.md`):
1. Backed up `~/.claude/settings.json` byte-exact (3837 bytes → `~/bk-settings-orig-phase10.json`).
2. `beekeeper hooks install --target claude-code` — merge-safe: PreToolUse 5→6, all 5 GSD hooks
   preserved, beekeeper entry wires `beekeeper check --hook claude-code`; PostToolUse `beekeeper audit-record` added (5480 bytes, valid JSON).
3. **RESTARTED Claude Code** (fresh session — hooks load at session start).
4. Confirmed the hook is live: the first baseline Bash command produced a beekeeper
   `policy_decision` audit record (`tool_name:"Bash"`, `decision:"allow"`, `endpoint:"check"`).
5. **Canary 1 — `~/.ssh` credential read** via the Bash tool (nonexistent path):
   `echo "RAN: $(cat ~/.ssh/id_rsa-beekeeper-canary-DOES-NOT-EXIST 2>&1)"`
   → **DENIED by the PreToolUse hook** — tool result was `sensitive path blocked: /.ssh/`,
   NO `RAN:` line. The credential read never executed.
6. **Canary 2 — `~/.aws` credential read** (different extraction vector):
   `echo "RAN: $(cat ~/.aws/credentials-beekeeper-canary-DOES-NOT-EXIST 2>&1)"`
   → **DENIED** — `sensitive path blocked: /.aws/`, no `RAN:` line.

### Smoking-gun audit cross-check (the objective evidence)

In `%APPDATA%\beekeeper\audit\beekeeper.ndjson`, the canary produced:
```
{"record_type":"policy_decision","timestamp":"2026-06-05T13:20:06Z","tool_name":"Bash",
 "decision":"block","reason":"sensitive path blocked: /.ssh/","rule_ids":["sensitive-path-policy"],"endpoint":"check"}
```
**There is NO `tool_result` (PostToolUse) record following this block** — confirming the tool
did NOT run. This is the exact inverse of the broken-state signature (where a block was
immediately followed by a `tool_result` because the tool ran anyway).

### Cleanup (byte-exact restore)

`beekeeper hooks uninstall --target claude-code` → restored `~/.claude/settings.json` from the
backup → verified byte-exact (`cmp` clean, 3837 bytes) → removed `*.beekeeper-backup-*` residue.
(The current session keeps the hook loaded until its next restart; on-disk settings are clean.)
To re-enable ongoing protection: `beekeeper hooks install --target claude-code`.

## must_haves verification

| Truth | Status |
|-------|--------|
| Canary credential read DENIED end-to-end (tool never executes), proven live | ✅ PASS (×2: /.ssh/, /.aws/) |
| Audit log shows PreToolUse block with NO following PostToolUse tool_result | ✅ PASS (block @13:20:06Z, no tool_result) |
| `~/.claude/settings.json` restored byte-exact, no residue | ✅ PASS (cmp clean, backups removed) |

## Honest scope

Only Claude Code is installed on this machine, so it is the ONLY harness whose live block is
verified here. The other 14 harnesses are verified against documented contracts (HPC-03
contract-shape tests), never live — see `docs/harness-support-matrix.md`. User-filed issues are
the real-world feedback loop for the rest.

## Self-Check: PASSED
