---
phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement
verified: 2026-06-05T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
---

# Phase 10: Hook-Block Protocol Compliance & Multi-Harness Enforcement — Verification Report

**Phase Goal:** Beekeeper's PreToolUse hook must ACTUALLY block a denied tool call across supported agent harnesses — not merely detect + audit it. Add `beekeeper check --hook <harness>` deny adapter, fix/extend per-harness installers (15 harnesses), route no-hook harnesses to the MCP gateway, and add the missing release gate that asserts the harness deny contract.
**Verified:** 2026-06-05
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth (from Success Criteria) | Status | Evidence |
|---|-------------------------------|--------|----------|
| 1 | On Claude Code, a credential-read tool call is BLOCKED live — not just audited | VERIFIED | 10-02-SUMMARY.md records two canary blocks (/.ssh/ and /.aws/) with audit block @13:20:06Z and NO following tool_result; this is the only locally-installable harness |
| 2 | `beekeeper check --hook <harness>` emits exit 2 + per-harness JSON on block; default mode still exits 1 with raw JSON | VERIFIED | `internal/check/deny_render.go` implements `RenderDeny` as a pure function; `cmd/beekeeper/main.go` branches on `hookTarget != "" && !result.Decision.Allow`; `TestRenderDeny` passes for all 15 harnesses; default path unchanged (exitBlock=1) |
| 3 | Installers write correct event names + config + feature flags and never clobber existing hooks | VERIFIED | `cursor.go` uses `beforeShellExecution/beforeMCPExecution/beforeReadFile` (not `preToolUse`); `codex.go` calls `ensureCodexFeaturesFlag`; all installers use merge-not-overwrite with targeted uninstall; all installer tests pass |
| 4 | No-hook harnesses documented + routed to MCP gateway; OpenCode plugin shipped; Hermes/Cline caveats documented; Tier 1/2/3 support matrix published | VERIFIED | `internal/hooks/kilo_trae.go` (printKiloGuide/printTraeGuide); `internal/hooks/opencode_plugin.go` (JS plugin template with `throw new Error`); `docs/harness-support-matrix.md` (15-harness table, honest tier definitions, 5 honesty notes); README with harness subsection |
| 5 | A release-gate test asserts the harness deny contract (exit 2 / deny JSON), closing the exit-1 gap | VERIFIED | `TestRenderDeny` (21 sub-tests covering all 15 harnesses × block families + allow path + Hermes empty-reason); `TestHookModeEmitsOnlyHarnessDenyForm` (5 harnesses full pipeline); `TestHermesHookNoRawDecisionLeak` — all pass |

**Score:** 5/5 truths verified

---

## Required Artifacts

| Artifact | Status | Evidence |
|----------|--------|----------|
| `internal/check/deny_render.go` | VERIFIED | Exists, 258 lines, `func RenderDeny(`, `exitHookBlock = 2`, `type HarnessID string`, 15 harness consts, 7 family structs, pure (no I/O, no os.Exit in function body) |
| `internal/check/deny_render_test.go` | VERIFIED | Exists, 251 lines, `func TestRenderDeny` with 21 cases; passes |
| `cmd/beekeeper/main.go` (--hook flag) | VERIFIED | `var hookTarget string`, `cmd.Flags().StringVar(&hookTarget, "hook", ...)`, branch `if hookTarget != "" && !result.Decision.Allow` calling `check.RenderDeny` |
| `internal/check/handler.go` (RunCheckTo) | VERIFIED | `func RunCheckTo(... decisionOut io.Writer)` exists; `RunCheck` delegates to it with `os.Stdout`; `--hook` mode passes `io.Discard` to suppress raw Decision JSON |
| `internal/hooks/claude_code.go` | VERIFIED | `claudePreCommand = "beekeeper check --hook claude-code"` (not bare `beekeeper check`) |
| `internal/hooks/cursor.go` | VERIFIED | Uses `cursorEvents = []string{"beforeShellExecution","beforeMCPExecution","beforeReadFile"}` (NOT `preToolUse`); `FailClosed: true` |
| `internal/hooks/codex.go` | VERIFIED | `ensureCodexFeaturesFlag` present; `beekeeperCodexPreToolUse()` command is `"beekeeper check --hook codex"` |
| `internal/hooks/augment.go` | VERIFIED | Exists with `installAugment`/`uninstallAugment`; tests pass |
| `internal/hooks/codebuddy.go` | VERIFIED | Exists with `installCodeBuddy`/`uninstallCodeBuddy`; tests pass |
| `internal/hooks/qwen.go` | VERIFIED | Exists with `installQwen`/`uninstallQwen`; tests pass |
| `internal/hooks/copilot.go` | VERIFIED | Exists with flat `permissionDecision` deny; tests pass |
| `internal/hooks/gemini.go` | VERIFIED | Exists with Gemini-native `decision:"deny"` form; tests pass |
| `internal/hooks/antigravity.go` | VERIFIED | Exists with dual-defensive `decision`+`permissionDecision`; tests pass |
| `internal/hooks/windsurf.go` | VERIFIED | Exists; exit-2-only (no stdout JSON); tests pass |
| `internal/hooks/hermes.go` | VERIFIED | Exists; YAML-patching installer; explicit fail-open documentation; tests pass |
| `internal/hooks/cline.go` + `cline_windows.go` | VERIFIED | `//go:build !windows` on cline.go; Windows stub returns clear error; `clinePreCommand = "beekeeper check --hook cline"` |
| `internal/hooks/opencode_plugin.go` | VERIFIED | Exists; JS plugin template with `tool.execute.before` and `throw new Error`; tests pass |
| `internal/hooks/kilo_trae.go` | VERIFIED | `printKiloGuide`/`printTraeGuide`; both include UNGUARDED language; wired via `printGatewayGuide` in `gateway_targets.go` |
| `docs/harness-support-matrix.md` | VERIFIED | Exists, 208 lines; all 15 harnesses; Tier 1/2/3 definitions; 5 honesty notes; UNGUARDED appears for Kilo/Trae |
| `internal/hooks/hooks_test.go` | VERIFIED | Contains `TestInstallClaudeCodeWiresHookFlag`, `TestInstallAugment`, `TestInstallCodeBuddy`, `TestInstallQwen`, `TestInstallCopilot`, `TestInstallGemini`, `TestInstallAntigravity`, `TestInstallWindsurf`, `TestInstallHermes`, `TestInstallOpenCodePlugin` — all pass |
| `internal/hooks/cline_test.go` | VERIFIED | `TestInstallCline` exists in dedicated file |
| `internal/check/handler_test.go` | VERIFIED | `TestHookModeEmitsOnlyHarnessDenyForm` (5 harnesses, full pipeline); `TestHermesHookNoRawDecisionLeak` — both pass |

---

## Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `cmd/beekeeper/main.go newCheckCmd` | `check.RenderDeny` | `if hookTarget != "" && !result.Decision.Allow` + `check.RenderDeny(check.HarnessID(hookTarget), result.Decision)` | WIRED |
| `cmd/beekeeper/main.go --hook mode` | `check.RunCheckTo(..., io.Discard)` | `if hookTarget != ""` branches to `RunCheckTo` suppressing raw Decision JSON | WIRED |
| `cmd/beekeeper/main.go default path` | `check.RunCheck(...)` | else branch, `os.Exit(result.ExitCode)` — exitBlock=1 unchanged | WIRED |
| `internal/hooks/claude_code.go` | `"beekeeper check --hook claude-code"` | `claudePreCommand` constant used by installer + idempotency + uninstall | WIRED |
| `internal/hooks/cursor.go installCursor` | Three correct event keys | `for _, event := range cursorEvents` (beforeShellExecution/beforeMCPExecution/beforeReadFile) | WIRED |
| `internal/hooks/codex.go installCodex` | `ensureCodexFeaturesFlag` | Called at end of install; writes `[features]\nhooks = true` | WIRED |
| `internal/hooks/gateway_targets.go Install` | `kilo_trae.go printKiloGuide/printTraeGuide` | `case TargetKilo: return printGatewayGuide(target, out)` → `printKiloGuide(out)` | WIRED |

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` exits 0 | `go build ./...` | exit 0, no output | PASS |
| `TestRenderDeny` passes (all 15 harnesses + allow path + Hermes empty-reason) | `go test ./internal/check/ -run TestRenderDeny -count=1` | 22 sub-tests PASS | PASS |
| `TestHookModeEmitsOnlyHarnessDenyForm` passes (Hermes fail-open leak closed) | `go test ./internal/check/ -run TestHookModeEmitsOnlyHarnessDenyForm -count=1` | 5 sub-tests PASS | PASS |
| `TestHermesHookNoRawDecisionLeak` passes | `go test ./internal/check/ -run TestHermesHookNoRawDecisionLeak -count=1` | PASS | PASS |
| All per-harness installer tests pass | `go test ./internal/hooks/... -run 'TestInstall...' -count=1` | 11 install-test functions, all subtests PASS | PASS |
| `TestInstallCursor/correct_event_names` passes | sub-test | PASS — verifies beforeShellExecution/beforeMCPExecution/beforeReadFile | PASS |
| `TestInstallCodex` passes (all 6 config.toml sub-tests) | sub-test | PASS — verifies `[features] hooks=true` patching | PASS |
| `TestInstallClaudeCodeWiresHookFlag` passes | sub-test | PASS — asserts installed command = `"beekeeper check --hook claude-code"` + idempotency | PASS |

---

## Anti-Patterns Found

| File | Pattern | Severity | Finding |
|------|---------|----------|---------|
| `internal/check/deny_render.go` | TBD/FIXME/XXX | — | None |
| `internal/hooks/*.go` | TBD/FIXME/XXX | — | None |
| `cmd/beekeeper/main.go` | TBD/FIXME/XXX | — | None |
| `docs/harness-support-matrix.md` | TBD/FIXME/XXX | — | None |
| `deny_render.go` os.Exit/I/O in function body | Purity violation | — | None — references are comments only; RenderDeny body contains no I/O calls |

No blockers or warnings found.

---

## HPC Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| HPC-01 | `beekeeper check --hook <harness>` pure table-driven deny renderer; exit 2 + stderr + per-harness JSON on block; default path (exit 1 + raw JSON) unchanged | SATISFIED | `deny_render.go` `RenderDeny`; `main.go` `--hook` flag with `RunCheckTo(io.Discard)`; `exitBlock=1` default path unchanged |
| HPC-02 | Per-harness installer correctness: Cursor event-name fix, Codex `[features] hooks=true`, merge-not-clobber + targeted uninstall for 15 harnesses | SATISFIED | `cursor.go` uses correct event names; `codex.go` calls `ensureCodexFeaturesFlag`; all 15 installers have merge-not-clobber + backup + targeted uninstall; all tests pass |
| HPC-03 | Per-harness deny-contract regression tests (the gate that was missing) | SATISFIED | `TestRenderDeny` (22 sub-tests covering all harness/family combinations); all per-harness installer tests asserting exact command strings and config formats |
| HPC-04 | Live Claude Code re-verification: credential read is actually DENIED end-to-end | SATISFIED | 10-02-SUMMARY.md records two live canary blocks (/.ssh/ /.aws/); audit block @13:20:06Z with NO following tool_result; byte-exact settings restore confirmed |
| HPC-05 | Kilo/Trae MCP-gateway routing; OpenCode plugin; Hermes/Cline caveats; Tier 1/2/3 support matrix | SATISFIED | `kilo_trae.go`; `opencode_plugin.go`; `hermes.go` and `cline.go`/`cline_windows.go` with explicit caveat docs; `docs/harness-support-matrix.md` |
| HPC-06 | Release-gate test proving harness deny contract holds for shipped binary | SATISFIED | `TestRenderDeny`; `TestHookModeEmitsOnlyHarnessDenyForm`; `TestHermesHookNoRawDecisionLeak` |

---

## Important Deviation (Verified Correct)

**Commit f315c81 — Hermes fail-open leak fix:**

The original plan had `--hook` mode calling `RunCheck` and then appending harness deny output. During live pre-flight, the correct issue was identified: Hermes ignores exit codes and parses the FIRST JSON object on stdout as the decision. The raw `{"Allow":false,...}` Decision JSON produced by `RunCheck` (to `os.Stdout`) would precede the Hermes deny form, causing Hermes to parse the wrong object and silently allow.

The fix: `RunCheckTo(..., io.Discard)` suppresses the raw Decision JSON in `--hook` mode; only the harness-specific deny form reaches stdout. `TestHookModeEmitsOnlyHarnessDenyForm` and `TestHermesHookNoRawDecisionLeak` assert this behavior across 5 harnesses including Hermes.

This is the correct implementation. The deviation from the initial plan design is an improvement, not a regression.

---

## Human Verification Required

None. All behavioral assertions are programmatically verifiable via test suite.

The only human-verified item (HPC-04 live canary) is recorded as completed in 10-02-SUMMARY.md with objective evidence (audit log timestamp + absence of PostToolUse record). No further human testing is required.

---

## Gaps Summary

None. All 5 success criteria verified. No gaps blocking goal achievement.

---

_Verified: 2026-06-05_
_Verifier: Claude (gsd-verifier)_
