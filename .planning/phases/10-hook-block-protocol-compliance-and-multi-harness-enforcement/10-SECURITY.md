---
phase: 10
slug: hook-block-protocol-compliance-and-multi-harness-enforcement
status: verified
threats_open: 0
asvs_level: 1
created: 2026-06-05
---

# Phase 10 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> **Result: SECURED — 23/23 threats closed (0 open).** Verified by gsd-security-auditor against the implementation (ASVS L1, block_on: high). Register authored at plan time; mitigations verified present in code/tests.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| harness → `beekeeper check --hook <h>` | The agent harness invokes the hook on an untrusted tool call; the deny signal it gets back is the only thing between a hijacked agent and execution | tool-call JSON (stdin) |
| `beekeeper check` → harness (deny output) | The exit code + stdout/stderr must match the harness's documented deny contract EXACTLY or the harness fails open and runs the tool | exit code + deny JSON / stderr |
| installer → harness config file (e.g. `~/.claude/settings.json`) | The installer writes the command + event(s) + feature flags the harness executes on every tool call; a wrong event name or missing flag = silent fail-open | settings.json / config.toml / hooks.json / config.yaml |
| installer → existing user hooks | The installer must MERGE, never clobber, a user's pre-existing hooks | user hook entries |
| Kilo/Trae native tools → no interception | These harnesses have NO pre-exec hook; only MCP tools route through the gateway; native Bash/file tools are UNGUARDED | (no boundary — documented gap) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation (verified) | Status |
|-----------|----------|-----------|-------------|------------------------|--------|
| T-10-01 | Tampering (silent over-allow) | RenderDeny exit/JSON shape | mitigate | `deny_render_test.go` `TestRenderDeny` (21 rows) asserts exact `ExitCode` + deny JSON per harness; the gate the exit-1 bug slipped (HPC-06) | closed |
| T-10-02 | Elevation (bypass approval) | non-block path under --hook | mitigate | `deny_render.go:138-139` returns ExitCode 0 + nil Stdout on allow; allow-row test asserts no `permissionDecision:"allow"` ever emitted | closed |
| T-10-03 | DoS (default-path regression) | newCheckCmd --hook branch | mitigate | `main.go:321` gates RenderDeny on `hookTarget!="" && !Allow`; default `os.Exit` path untouched; `RunCheck`→`RunCheckTo(...,os.Stdout)` unchanged | closed |
| T-10-04 | Tampering (Hermes fail-open) | Hermes render family | mitigate | `deny_render.go:226-243` Hermes ExitCode=0; block carried only by non-empty `{"action":"block","message":...}`; default message when reason empty | closed |
| T-10-05 | Spoofing (clobber Claude hooks) | claudePreCommand change | accept | Reuses shipped merge path (`claude_code.go:84` `mergeClaudeHookEntry`, commit 50513ae); `TestInstallClaudeCodePreservesExistingHooks` seeds 5 foreign hooks | closed |
| T-10-06 | Info Disclosure (real key leak) | live canary command | mitigate | `10-02-SUMMARY.md` — canary reads only nonexistent suffixed paths; PASS = no "RAN:" line; real keys never touched | closed |
| T-10-07 | Tampering (corrupt user settings) | install/uninstall on live settings | mitigate | `10-02-SUMMARY.md` — byte-exact backup + cmp verify (3837 bytes) + backup removal; merge path preserves the 5 GSD hooks | closed |
| T-10-08 | Repudiation (unproven block claim) | "block works" assertion | mitigate | `10-02-SUMMARY.md` — audit block record @2026-06-05T13:20:06Z with NO following PostToolUse tool_result | closed |
| T-10-09 | Tampering (Cursor hook never fires) | Cursor preToolUse bug | mitigate | `cursor.go:32-36` three real events replace `preToolUse`; `TestInstallCursor/correct_event_names` asserts `preToolUse` absent + 3 events present | closed |
| T-10-10 | Tampering (Cursor fail-open) | Cursor failClosed | mitigate | `cursor.go:91` `FailClosed:true`; test asserts it for every event (Cursor defaults to fail-OPEN) | closed |
| T-10-11 | Tampering (Codex hook disabled) | Codex [features] flag | mitigate | `codex.go:101` `ensureCodexFeaturesFlag` idempotent `[features] hooks=true` (no TOML dep); 6 subtests cover all entry conditions | closed |
| T-10-12 | Spoofing (clobber Augment/CodeBuddy/Qwen) | three installers | mitigate | All reuse `mergeClaudeHookEntry`+`PatchSettings`; `preserves_existing_hooks` subtests seed a foreign hook + assert survival | closed |
| T-10-13 | Tampering (Copilot wrong deny family) | Copilot flat vs nested | mitigate | `deny_render.go:163-173` flat `permissionDecision` (no wrapper); `TestRenderDeny` copilot row asserts flat form at exit 2 | closed |
| T-10-14 | Tampering (Windsurf fail-open + OS key) | Windsurf exit-2-only | mitigate | `deny_render.go:249-250` exit 2 + nil Stdout; `windsurf.go` GOOS branch (powershell on Windows); `os_correct_key` test asserts | closed |
| T-10-15 | Tampering (Antigravity ambiguous deny) | dual-defensive family | mitigate | `deny_render.go:201-211` emits BOTH `decision:"deny"` and `permissionDecision:"deny"`+denyReason; test asserts both | closed |
| T-10-16 | Spoofing (clobber Copilot/Gemini/Antigravity/Windsurf) | four installers | mitigate | Merge trinity or filtered-append per installer; `preserves_existing_hooks`/`preserves_foreign_hooks` subtests assert survival | closed |
| T-10-17 | Tampering (Hermes silent allow — empty message) | Hermes empty message | mitigate | `deny_render.go:231-233` guarantees non-empty message; subtest asserts `"message":""` absent; `hermes.go` prints fail-open reminder | closed |
| T-10-18 | Info Disclosure (Windows Cline overclaim) | Cline on Windows | mitigate | `cline.go` `//go:build !windows`; `cline_windows.go:34` returns "macOS/Linux only" error; matrix Honesty Note #3 | closed |
| T-10-19 | Elevation (OpenCode subagent bypass) | OpenCode plugin | accept | `opencode_plugin.go:13-17` T-10-19 comment + `printOpenCodeCaveats` prints #5894/#2319 on install; matrix OpenCode row; MCP routes via gateway | closed |
| T-10-20 | Spoofing (destroy foreign Cline hook) | Cline file overwrite | mitigate | `cline.go:60-81` `containsClineCommand` guard + backup before overwrite; uninstall checks marker; `cline_test.go` asserts both | closed |
| T-10-21 | Repudiation (overclaim support) | support matrix | mitigate | matrix Honesty Notes #1/#2 (only Claude Code live-verified); README honest; `TestInstallGatewayTargetKiloTraeUNGUARDED` enforces "UNGUARDED" text | closed |
| T-10-22 | Elevation (Kilo/Trae native tools unguarded) | Kilo/Trae native tools | accept | `kilo_trae.go:51/99` "UNGUARDED"; matrix Tier-3 rows + Honesty Note #4; build-time honesty gate test; no code can close without upstream hook | closed |
| T-10-23 | Info Disclosure (docs drift) | docs drift | accept | matrix sourced from RESEARCH §2-3; "Research date: 2026-06-05" recorded for reconciliation | closed |
| T-10-SC | Tampering (supply chain — no new deps) | package/module installs | accept | `go.mod` has no new Phase-10 module dep; Codex TOML + Hermes YAML + Cline script + OpenCode plugin are stdlib-only in-repo writes | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-10-01 | T-10-05 | claude_code.go merge path already shipped (50513ae) + regression-tested; only the command constant changed | bantuson | 2026-06-05 |
| AR-10-02 | T-10-19 | OpenCode plugin cannot intercept subagent `task` (#5894) / MCP (#2319) calls — upstream limitation; documented on install + in matrix; MCP routes via gateway | bantuson | 2026-06-05 |
| AR-10-03 | T-10-22 | Kilo/Trae have no pre-exec hook (Kilo FR #5827); native tools UNGUARDED — no code can close without upstream support; documented plainly in guide + matrix | bantuson | 2026-06-05 |
| AR-10-04 | T-10-23 | Harness deny contracts may change upstream; matrix records its RESEARCH source date for future reconciliation | bantuson | 2026-06-05 |
| AR-10-05 | T-10-SC | No new package-manager installs and no new Go module dependency; all config patches are in-repo stdlib writes | bantuson | 2026-06-05 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-06-05 | 23 | 23 | 0 | gsd-security-auditor (ASVS L1, block_on: high) |

**Notable:** the audit was preceded by a live pre-flight that surfaced a real defect — `--hook` mode leaked the raw Decision JSON before the deny form, which would have silently allowed on Hermes (fail-open). Fixed in commit `f315c81` (`RunCheckTo(..., io.Discard)`) with regression tests `TestHookModeEmitsOnlyHarnessDenyForm` / `TestHermesHookNoRawDecisionLeak`. This is folded into T-10-04/T-10-17.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-06-05
