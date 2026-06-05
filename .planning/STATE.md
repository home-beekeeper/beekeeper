---
gsd_state_version: 1.0
milestone: v1.3.0
milestone_name: milestone
status: executing
stopped_at: Completed 10-06-PLAN.md
last_updated: "2026-06-05T12:44:15Z"
last_activity: "2026-06-05 -- Phase 10 plan 06 executed (2 tasks, 2 commits: c7018e0 8e1b50d)"
progress:
  total_phases: 6
  completed_phases: 4
  total_plans: 26
  completed_plans: 24
  percent: 92
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-03)

**Core value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Current focus:** Phase 10 — hook-block-protocol-compliance-and-multi-harness-enforcement

> ⏸ **v1.1.0 "Pollen" is PARKED, not closed** — paused at the 05-05 maintainer release checkpoint. To resume the release: see `HANDOFF.json`, `.planning/phases/05-contribution-back-milestone-close/.continue-here.md`, and `docs/release-runbook.md`. The four signed-tag releases remain in the "Deferred Items" table below. Do not archive v1.1.0 until the runbook is run + 05-05 Task 3 completes.

## Current Position

> **SUPERSEDED 2026-06-05 — read this first.** v1.2.0 is SHIPPED & ARCHIVED (tag `v1.2.0`; the "READY FOR CLOSE" lines below are stale pre-close text). **NEW critical finding:** a live dogfood proved Beekeeper's PreToolUse hook fires + decides "block" but harnesses run the tool anyway — `beekeeper check` exits `1`, but harnesses need exit `2` / a per-harness deny JSON. **The hook-block path is non-functional on every hook-based harness** (Claude Code/Cursor/Codex); shim + MCP-gateway paths are unaffected. **Phase 10 "Hook-Block Protocol Compliance & Multi-Harness Enforcement" inserted (seeds v1.3.0) — NOT yet planned.** Research + Context live in the phase folder: `.planning/phases/10-hook-block-protocol-compliance-and-multi-harness-enforcement/{10-RESEARCH.md, 10-CONTEXT.md}` (15-harness deny-contract matrix in 10-RESEARCH.md). **NEXT: `/gsd-plan-phase 10`** (read the research first; expect ≥2 sub-phases). The 4 prerequisite fixes are now on `main` (commits `adc1f77` nudge/timeout, `50513ae` hooks clobber→merge — fast-forward merged 2026-06-05; the `fix/windows-pnpm-timeout-and-hook-merge` branch is fully merged/redundant, deletable with `git branch -d`). **Phase 10 executes on `main`** (GSD runs the phase on the current branch; HEAD is on main). Planning artifacts (ROADMAP/STATE/phase-10 dir) remain uncommitted. Roadmap evolution: Phase 10 added 2026-06-05.

Phase: 10 (hook-block-protocol-compliance-and-multi-harness-enforcement) — COMPLETE
Plan: 6 of 6 (all plans complete; plan 02 is checkpoint-only)
Status: Phase 10 complete — all 6 plans executed (HPC-01 through HPC-05)
Last activity: 2026-06-05 -- Phase 10 plan 06 executed (2 tasks, 2 commits: c7018e0 8e1b50d)

Progress (v1.2.0): [██████████] 100% (4 of 4 phases complete & verified — milestone ready for close)

## Phase Summary (v1.1.0)

| Phase | Name | Tag | Requirements | Status |
|-------|------|-----|--------------|--------|
| 1 | Fork Setup & Discipline | v0.1.1-pollen.1 | FORK-01–04, PTEST-02–03, SDEF-02 | ✅ Complete |
| 2 | Windows Root Resolver | v0.1.1-pollen.2 | WRES-01–02, PTEST-01 | ✅ Code complete — signed release deferred to M2 close |
| 3 | Windows Path Representation | v0.1.1-pollen.3 | WPATH-01–02 | ✅ Code complete & verified — signed release deferred to M2 close |
| 4 | Windows Extension & MCP Coverage | v0.1.1-pollen.4 | WEXT-01–03, BKINT-01, PTEST-04 | ✅ Code complete & verified — signed release deferred to M2 close |
| 5 | Contribution-Back & Milestone Close | v0.1.1-pollen.5 | SYNC-01–02, BKINT-02, PTEST-05, SDEF-01 | Not started |

## Phase Summary (v1.2.0)

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| 6 | Corroboration Severity Hardening | CORR-01, CORR-02 | ✅ Complete (3/3 plans; +2 code-review fixes) |
| 7 | Sensitive-Path Runtime Enforcement | SPATH-01–04 | ✅ Complete (3/3 plans; +6 code-review fixes) |
| 8 | Package-Manager Nudge + Behavioral Test Suite | NUDGE-01–08, BTEST-01–03 | ✅ Complete & verified (8/8 plans; 5/5 SCs, 11/11 reqs; full suite + fuzz + live-binary E2E green) |
| 9 | v1.2.0 Tech-Debt Cleanup (inserted from audit) | CLEAN-01–04, HARDEN-01–03, DRIFT-01 | ✅ Complete & verified (5/5 plans + 1 fix; 9/9 must-haves; full suite + e2e + fuzz green) |

## Performance Metrics

**Velocity (v1.0.0):**

- Total plans completed: 64
- Average duration: ~10 min/plan

## Accumulated Context

### Decisions

Recent decisions from Phase 10 (v1.3.0 - plan 10-06):

- Phase 10-06: kilo_trae.go is a dedicated file for Tier-3 no-hook harness guides; guides upgraded with "UNGUARDED" text; TestInstallGatewayTargetKiloTraeUNGUARDED enforces the honesty gate (T-10-22)
- Phase 10-06: docs/harness-support-matrix.md is the authoritative 15-harness honesty document: Tier 1 testable = Claude Code; Tier 1 documented = 9 others; Tier 2 = Hermes/Cline/OpenCode; Tier 3 = Kilo/Trae
- Phase 10-06: README.md created (project had no README); links harness-support-matrix.md; states headline honestly without overclaiming universal protection

Recent decisions from Phase 10 (v1.3.0 - plan 10-05):

- Phase 10-05: Hermes YAML patching uses bufio.Scanner line scan (3 cases: append full block / append pre_tool_call: under hooks: / insert entry under existing pre_tool_call:); no gopkg.in/yaml.v3 dep
- Phase 10-05: Cline build-tag split — cline.go (!windows) real installer with executable PreToolUse 0o755; cline_windows.go (windows) stub with "macOS/Linux only" error; both GOOS builds clean
- Phase 10-05: TargetOpenCode moved from gatewayTargets to plugin installer; printOpenCodeGuide retained as MCP-fallback reference; TargetKilo/TargetTrae added to gatewayTargets + gateway_targets.go
- Phase 10-05: T-10-20 — installCline backs up + warns on foreign PreToolUse before overwriting; uninstallCline verifies clinePreCommand marker before removing

Recent decisions from Phase 10 (v1.3.0 - plan 10-03):

- Phase 10-03: Cursor event-name fixed (preToolUse → beforeShellExecution/beforeMCPExecution/beforeReadFile); FailClosed:true retained; command updated to --hook cursor
- Phase 10-03: ensureCodexFeaturesFlag uses targeted line/section string patching without new TOML library (CLAUDE.md constraint); idempotent under all 4 entry conditions
- Phase 10-03: Augment/CodeBuddy/Qwen reuse mergeClaudeHookEntry/removeClaudeHookEntry trinity; beekeeperClaudePreEntryWith/beekeeperClaudePostEntryWith added for parametric entry construction
- Phase 10-03: installCodex backs up config.toml before calling ensureCodexFeaturesFlag

Recent decisions from Phase 10 (v1.3.0 - plan 10-01):

- Phase 10-01: RenderDeny returns ExitCode=0 on allow — harness approval flow never bypassed; permissionDecision:"allow" never emitted (CONTEXT decision 3, T-10-02)
- Phase 10-01: Hermes ExitCode=0 by design (exit codes ignored by Hermes; block carried by JSON action:"block" with guaranteed non-empty message)
- Phase 10-01: Unknown HarnessID fails closed — exit 2 + stderr, nil Stdout (never silently allows)
- Phase 10-01: claudePreCommand changed from "beekeeper check" to "beekeeper check --hook claude-code"; propagates to merge/uninstall helpers via sentinel string

Recent decisions from Phase 02 (v1.1.0 Pollen - plan 02-02):

- Phase 02-02: isBroadHomeRoot gains C:\Users and C:\Users\<name> broad detection (Rule-1 auto-fix) — mirrors /Users and /Users/<name> on Unix; test asserted C:\Users broad but implementation only had C:\ drive-root
- Phase 02-02: roots_windows_test.go uses t.Setenv(USERPROFILE/APPDATA/LOCALAPPDATA/ProgramFiles) — never HOME — for Windows test isolation (Pitfall 5 prevention)
- Phase 02-02: glob root fixtures: create concrete versioned dirs (Python313, 3.3.0, Ruby33-x64) under wildcard parent so filepath.Glob resolves (needed for PyPI/RubyGems test assertions)
- Phase 02-02: TestResolveRootsBaselineIncludesUserLocalPython keeps Windows skip; reason updated to Unix-specific (non-Phase-2) language pointing to TestWindowsBaselineRoots for Windows PyPI coverage

Recent decisions from Phase 01 (v1.1.0 Pollen - plan 01-01):

- GOOS=windows go build ./... passes clean — no non-test files needed Windows fixes (Open Question 1 resolved)
- 6 Unix-root-resolver tests in cmd/pollen/main_test.go get t.Skip with "Phase 2 (v0.1.1-pollen.2)" structured reasons (not build tags — allows other tests in the file to run)
- scanner_test.go TestEndToEndScan: path separator bug fixed via filepath.Separator (not a skip; test passes on all OSes)
- BUMBLEBEE_ env var names renamed to POLLEN_ in roots.go + main_test.go (FORK-04 trademark)
- upstream remote configured at pollen repo clone; origin binding to github.com/bantuson/pollen deferred to plan 05

Recent decisions from Phase 11:

- VerifySignatureWithKey(entry, pubKey) added alongside VerifySignature — presence-only path unchanged for backward compat
- Dissent sentinels (CatalogMatch{Dissented:true}) emitted by MultiIndex.LookupAll for configured-but-no-match sources; corroborate() filters them into SourcesDissented — import cycle avoided
- scanOnDeltaFn injectable var follows runBumblebeeFn pattern for test-time mock without real scan binary
- GoReleaser before.hooks uses sh -c guard so non-Linux environments skip eBPF generate gracefully
- -buildvcs=false added to goreleaser build flags (reproducibility gap closure)

Recent decisions from Phase 7:

- go-winio import path is github.com/Microsoft/go-winio (capital M); lowercase fails at go get with module path mismatch
- PipePath is var not const to enable test-time substitution; production value unchanged
- GetCurrentProcessToken().IsElevated() replaces manual TOKEN_ELEVATION unsafe pointer dance
- ETW EnableProvider is the actual API (not AddProvider); Provider struct needs GUID value type from *MustParseGUID dereference
- TestQueryServiceWhenNotInstalled skips on non-admin (mgr.Connect returns Access Denied); covered by CI admin runners

Recent decisions from Phase 6:

- Remote sink errors are fire-and-forget (nil returned); local NDJSON write is never blocked by remote collector outage
- AuditConfig imported by audit/sink.go from internal/config — no import cycle (config imports only stdlib)
- LlamaFirewall injection detection (LLMF-02) exits 0 in hook handler — PostToolUse hooks must not block agent flow; llmf_alert is the forensic signal
- scan_code / scan_alignment are Python sidecar stubs; CodeShield model integration deferred
- Phase 06-01 (CORR-01): CatalogHealthy defaults true — escalation active by default; callers explicitly set false on confirmed catalog degradation
- Phase 06-01 (CORR-01): findSeverityOverride all-versions guard inside helper — Version=="*" returns nil, preventing wildcard mis-tagged critical entries from single-source block
- Phase 06-01 (CORR-02): validateCorroborationThresholds extended with per-severity bounds loop; fail-closed to "block" on violation (BlockAt<1, BlockAt>globalBlockAt, QuarantineAt<BlockAt)
- Phase 06-01 (CORR-01/02): escalation + sanity gate shipped atomically in one commit (STATE.md Blockers/Concerns constraint satisfied)
- [Phase ?]: critical_block_at operator configurability added to policyloader

Recent decisions from Phase 07 (v1.2.0 — Sensitive-Path Runtime Enforcement):

- runCheck ordering (CR-02 fix): ApplyPolicyOverlay runs BEFORE the sensitive-path block; the path block is merged LAST via mergeDecisions (most-restrictive-wins) so a package_allowlist allow can never downgrade a credential-read block. runCheckWithIndex (integration_test.go) mirrors the same block so tests prove wiring without a catalog match.
- canonicalizePath (internal/check/paths.go, impure adapter) order: expandWinEnvVars → expandHome → filepath.Abs → EvalSymlinks(fallback to Abs result on error so non-existent credential files still evaluate) → ToSlash. internal/policy stays pure (TestPathImportsArePure green).
- expandWinEnvVars: single-pass strings.Builder, targeted %VAR%→os.Getenv (NOT os.ExpandEnv), fail-closed on unresolved var (literal %VAR% kept, never empty). Satisfies SC2 `type %USERPROFILE%\.ssh\id_rsa` (D-01).
- extractBashCredentialPaths scans ALL read-verb occurrences (moving-offset loop, CR-01 fix) and firstShellToken skips leading flag tokens (handles `cat -n …` and `a && cat ~/.ssh/id_rsa`).
- isAllowedPath gained a basename branch (no-separator patterns match lastSegment) — required for AllowPatterns (.env.example/.test/.schema) to take effect (Pitfall 2). extractTargetPath (policyloader) reads file_path primary, path fallback (both `!= ""` guarded).
- DefaultSensitivePaths added /.cursor/, /.windsurf/, bare /.cargo/credentials (D-02). D-03: SPATH wired into `beekeeper check` ONLY (no gateway/watch/scan).
- [Phase ?]: Antigravity settings path: ~/.gemini/antigravity/hooks.json (primary); .agents/hooks.json documented as project-local alternative
- [Phase ?]: Windsurf: exit-2-only deny (no stdout JSON); installer uses runtime.GOOS branch for powershell vs command key
- [Phase ?]: Gemini hooks in settings.json top-level hooks array, not nested event map

### Open Research Flags (v1.2.0)

- **[RESOLVED in 08-RESEARCH.md, 2026-06-04]** Flag 2 → **Position B** (60s detection cache lives ONLY in the long-lived gateway/shim; the one-shot `beekeeper check` hook runs `nudge.DetectStateFn` fresh with a 2s timeout, no file-cache/hot-path I/O). Flag 4 → **EXTRACT** a new pure `internal/pkgparse/` collapsing the two existing install-parse copies (engine.go `installPrefixes` + enforce.go `installPrefixesOverlay`); pnpm/bun/yarn install verbs map to ecosystem "npm" (closes F3). discuss-phase was intentionally skipped (maintainer chose to plan directly); research committed both decisions so they did not leak into implementation.
- **During Phase 8:** Windows corepack-shimmed pnpm `cmd.exe` startup time under the 2s detection timeout — live CI timing needed.
- **Flag 5 (PRD corrections):** `minimumReleaseAge` default is 1440 minutes (not 60); Node 22 is Maintenance LTS (Node 24 is Active LTS) — apply before implementation in Phase 8.

### Blockers/Concerns

- [RESOLVED 2026-06-04] Phase 8 (NUDGE): Flag 2 + Flag 4 settled in 08-RESEARCH.md and encoded in the plans (Flag 5 PRD corrections — minimumReleaseAge weakness baseline 1440, Node 24 recommended / floor 22 — also baked in). detect.go signature locked (cache-free `DetectStateFn` seam + gateway-only Cache wrapper). No longer a blocker.
- PLCY-07 (Phase 6) self-defense: [RESOLVED in 06-01] escalation + sanity gate shipped atomically; all-versions guard + SeverityOverrides in one commit

## Deferred Items

Items acknowledged and carried forward:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Testing | `go test -race` requires CGO/C compiler | CI-only | Phase 1 (v1.0.0) |
| Build | `make verify-release` requires make on Windows | CI-only | Phase 1 (v1.0.0) |
| Watch | `notify.Config` wired to config preferences | Future phase | Phase 3 (v1.0.0) |
| Cursor | Windows extension-dir path (Assumption A1) | Live validation in v1.1.0 Phase 4 | Phase 3 (v1.0.0) |
| Distribution | Pollen binary releases (DIST-01) | v2 requirement | v1.1.0 scoping |
| Self-catalog | Separate `pollen-self` catalog (SELF-02) | v2 requirement | v1.1.0 scoping |
| Release | **`v0.1.1-pollen.2` signed tag (Phase 2 SC4)** — VERSION+CHANGES bumped and 4 commits prepared locally in `../pollen` (HEAD `c94b271`), **unpushed, untagged**. Cut the release at M2 close: `git push origin main` → confirm 3-OS CI green → `git tag -a v0.1.1-pollen.2` + push → cosign verify. Exact commands in `.planning/phases/02-windows-root-resolver/02-04-SUMMARY.md`. | **Deferred to M2 close** (maintainer decision 2026-06-02) | Phase 2 (v1.1.0) |
| Release | **`v0.1.1-pollen.3` signed tag (Phase 3 SC4)** — VERSION bumped to `0.1.1-pollen.3` + CHANGES.md WPATH section prepared locally in `../pollen` (commits incl. `1cb3fdb`, `19695e3`), **untagged, unsigned**. Cut at M2 close together with pollen.2: confirm 3-OS CI green → `git tag -a v0.1.1-pollen.3` + push → cosign verify. Details in `.planning/phases/03-windows-path-representation/03-03-SUMMARY.md`. | **Deferred to M2 close** (D-06, maintainer decision 2026-06-02) | Phase 3 (v1.1.0) |
| Release | **`v0.1.1-pollen.4` signed tag (Phase 4 SC5)** — VERSION bumped to `0.1.1-pollen.4` + CHANGES.md WEXT section prepared locally in `../pollen` (HEAD `a9db7b3`), **untagged, unsigned**. Cut at M2 close together with pollen.2 + pollen.3: confirm 3-OS CI green → `git tag -a v0.1.1-pollen.4` + push → cosign verify. Details in `.planning/phases/04-windows-extension-mcp-coverage-beekeeper-compat-test/04-03-SUMMARY.md`. | **Deferred to M2 close** (D-06, maintainer decision 2026-06-02) | Phase 4 (v1.1.0) |
| Phase 06 P01 | 440 | 3 tasks | 5 files |
| Phase 06 P02 | 171 | 2 tasks | 4 files |
| Phase 06-corroboration-severity-hardening P03 | 15 | 3 tasks | 9 files |
| Phase 10 P04 | 7 minutes | 3 tasks | 6 files |

## Session Continuity

Last session: 2026-06-05T12:44:15Z
Stopped at: Completed 10-06-PLAN.md (Phase 10 complete)
Resume file: None

## Operator Next Steps

- **v1.2.0 (current):** ALL phases (6,7,8,9) ✅ **complete & verified**. Milestone audit (`v1.2.0-MILESTONE-AUDIT.md`, `tech_debt` — NO blockers) → maintainer scope "Everything" → **Phase 9 "v1.2.0 Tech-Debt Cleanup" EXECUTED (5/5 plans + fix `ef4ea97`) + VERIFIED passed 9/9** (`09-VERIFICATION.md`). All audit findings resolved: CLEAN-01 hermetic CORR E2E (signed non-wildcard fixture, blocks offline), CLEAN-02 LoadLayered Nudge-pointer merge (root fix), CLEAN-03 Phase-6 Nyquist COMPLIANT, CLEAN-04 handler comment, HARDEN-01/02/03 SPATH evasion edges (ancestor-symlink dual-form + Windows ADS/trailing-dot + verb word-boundary), DRIFT-01 live version_drift npm registry query (fail-open, floors never bumped). Release gates GREEN (go build + full unit suite + `-tags e2e` 15.4s + `-tags fuzz` incl. policy post-fix + TestPathImportsArePure + TestOverlayAllowCannotDowngradePathBlock). Phase 9 ran SEQUENTIALLY on main (`use_worktrees=false`); executors were told NOT to do phase-number-keyed SDK tracking writes — orchestrator updated STATE/ROADMAP/REQUIREMENTS MANUALLY. **NEXT:** re-run `/gsd-complete-milestone v1.2.0` — fold `v1.2.0-MILESTONE-AUDIT.md` + `09-VERIFICATION.md` into the archive (optionally `/gsd-verify-work 9` for conversational UAT first). (Phase 8's 08-05 executor had hit a mid-run session limit AFTER committing its 3 task commits — closed out manually by writing its SUMMARY, no re-execution.) Known INFO (now folded into Phase 9): gateway `realMetadataFetch` (drift.go) is a production stub returning empty, so `version_drift` emits nothing live until a real registry query is added (floor auto-update is Out-of-Scope; drift is informational-only). Also confirmed broken this session: `phase-plan-index 8` flattens all 8 plans into "wave 1" (ignores the `wave:` frontmatter) — execution used the authoritative PLAN-frontmatter wave order W1{01,05} W2{02,03} W3{04} W4{06} W5{07,08}. Code review done (08-REVIEW.md): 0 critical, 4 warning — ALL FIXED (WR-01 redact nudge command fields in audit log, WR-02 parseInt overflow guard, WR-03 parallelize PM probes so detection stays ~2s within the hook budget, WR-04 bound drift goroutine) + re-confirmed (build + tests + live E2E green); 5 Info deferred. Caveat: `go test -race` is CI-only (CGO disabled locally) — the CI race pass on internal/nudge is the authoritative confirmation for WR-03's concurrency change. NOTE (still applies to execute): the `gsd-sdk` init.plan-phase / state.begin-phase / phase.complete resolvers map bare phase numbers (7, 8) to ARCHIVED v1.0.0 dirs under `.planning/milestones/v1.0.0-phases/` and corrupt STATE frontmatter progress — the live v1.2.0 phases are in `.planning/phases/NN-slug/`. Pass explicit paths to agents and update STATE/ROADMAP/REQUIREMENTS tracking MANUALLY (do not trust phase-number-keyed SDK writes). `init.execute-phase`, `roadmap.get-phase`, and `init.phase-op` resolved correctly; `init.plan-phase`/`state.begin-phase` did not. CONFIRMED this session: `init.plan-phase 8` → `.planning/milestones/v1.0.0-phases/08-tui-dashboard` (wrong); plan-phase was driven with explicit live paths instead.
- **v1.1.0 (parked release):** when ready, run `docs/release-runbook.md` (push `../pollen` + cut signed tags `pollen.2/.3/.4/.5` + cosign verify + create/push `bantuson/beekeeper`), then finish 05-05 Task 3 (tracking + verify) and close v1.1.0 via `/gsd-complete-milestone`. Resume context: `HANDOFF.json` + 05-05 `.continue-here.md`. Do NOT close v1.1.0 before this runs.
- **Still pending (from v1.0.0 close):** the beekeeper GitHub remote is created as part of the v1.1.0 runbook (Step 1: `gh repo create bantuson/beekeeper`).
