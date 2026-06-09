---
gsd_state_version: 1.0
milestone: v1.3.0
milestone_name: Web Presence & Documentation
status: executing
stopped_at: "Phase 18 (Full Content Authoring, DOCS-02–09) ✅ COMPLETE & verified 2026-06-09 — executed inline on main (6/6 plans, ~14 commits). Replaced the 8 thin Phase-13 docs stubs with the complete accurate doc set. W1 18-01: accuracy_spec.py (AC-1 source_doc+path-exists / AC-2 unenforced labels / AC-3 no phantom cmds) + UnenforcedCallout amber component + mdx-components register. W2 18-02..05 (8 sections): getting-started, installation (exact cosign/SLSA from THREAT-MODEL §7), configuration, cli-reference (exhaustive tree), security (posture+co-located gaps), integration (per-harness point-of-use caveats), troubleshooting, audit-log. W3 18-06: full suite GREEN (pnpm build + accuracy_spec AC-1..3 + seo/home/gfx all exit 0) + maintainer AC-5 accuracy review APPROVED. **KEY DEVIATION (accuracy correction):** RESEARCH §7 OQ-1 + docs/nudge.md field-ref were WRONG — verified internal/config/config.go legalNudgeModes={soft,hard,block} & ValidateNudgeConfig ACCEPTS block (explicit test); documented nudge mode:block accurately as a valid/settable mode + on-install default per D-04 (code wins). Stub inaccuracies all corrected (--target not --hook; no check --input; hooks status→diag; catalogs rebuild→catalogs sync; audit log single beekeeper.ndjson no rotation). Docs STYLING (stock Fumadocs theme) kept SEPARATE → still deferred (.planning/todos/pending/docs-styling-polish.md). Tracking hand-managed (skipped phase.complete — frontmatter caveat). NEXT: /gsd-plan-phase 19 (Test Suite & CI, QA-01/02) — the LAST v1.3.0 phase."
last_updated: "2026-06-09T22:30:00.000Z"
last_activity: "2026-06-09 — Phase 18 (Full Content Authoring) COMPLETE & verified: all 8 docs sections authored, accuracy_spec AC-1..3 green, maintainer AC-5 sign-off vs THREAT-MODEL.md. DOCS-02..09 done. Next: /gsd-plan-phase 19 (Test Suite & CI — last v1.3.0 phase)."
progress:
  total_phases: 9
  completed_phases: 8
  total_plans: 24
  completed_plans: 24
  percent: 89
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-03)

**Core value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Current focus:** Phase 18 (Full Content Authoring, DOCS-02–09) ✅ COMPLETE & verified 2026-06-09 — replaced the 8 thin Phase-13 docs stubs with the complete, ACCURATE doc set (quickstart, install, config, security+gaps, integration, CLI ref, troubleshooting, audit-log). 6/6 plans executed inline on main; accuracy_spec AC-1..3 green; maintainer AC-5 sign-off vs THREAT-MODEL.md. Docs styling (stock Fumadocs theme) kept SEPARATE → still deferred (`.planning/todos/pending/docs-styling-polish.md`). **NEXT: /gsd-plan-phase 19 (Test Suite & CI, QA-01/02) — the LAST v1.3.0 phase.** (SITE-03 live deploy still DEFERRED → Vercel.) Milestone 8/9 phases (89%).

> ⏸ **v1.1.0 "Pollen" is PARKED, not closed** — paused at the 05-05 maintainer release checkpoint. To resume the release: see `.planning/phases/05-contribution-back-milestone-close/.continue-here.md` and `docs/release-runbook.md`. The four signed-tag releases remain in the "Deferred Items" table below. Do not archive v1.1.0 until the runbook is run + 05-05 Task 3 completes.

## Current Position

Phase: 18 — Full Content Authoring (✅ COMPLETE & verified 2026-06-09 — executed inline on main; 6/6 plans; DOCS-02–09 done)
Plan: 18-01 ✅ / 18-02 ✅ / 18-03 ✅ / 18-04 ✅ / 18-05 ✅ / 18-06 ✅ (6/6). W1 18-01 accuracy_spec.py + UnenforcedCallout + register → W2 18-02..05 all 8 docs sections authored (getting-started, installation, configuration, cli-reference, security, integration, troubleshooting, audit-log) → W3 18-06 full suite GREEN (pnpm build + accuracy_spec AC-1..3 + seo/home/gfx) + maintainer AC-5 sign-off vs docs/THREAT-MODEL.md. Stub inaccuracies corrected (--target/check-input/hooks-status/catalogs-rebuild/audit-rotation). KEY DEVIATION: nudge mode:block documented as valid/settable (verified config.go legalNudgeModes={soft,hard,block}) — RESEARCH OQ-1 + nudge.md were wrong; code wins (D-04). Docs STYLING kept separate (still deferred).
Status: ✅ Complete & verified — NEXT: /gsd-plan-phase 19 (Test Suite & CI, QA-01/02 — last v1.3.0 phase)
Last activity: 2026-06-09 — Phase 18 COMPLETE (all 8 docs authored; accuracy gate green; maintainer AC-5 approved). Phase 17 ✅ COMPLETE earlier same day.

## Phase Summary (v1.3.0)

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| 10 | Hook-Block Protocol Compliance & Multi-Harness Enforcement | (seed) | ✅ Shipped 2026-06-05 (v1.3.0 seed) |
| 11 | Scaffold & Toolchain Isolation | SITE-01, SITE-02 | ✅ Complete & verified 2026-06-07 (1/1 plan; 4/4 SCs) |
| 12 | Design System | DSYS-01–04 | ✅ Complete & verified 2026-06-08 (3/3 plans; verifier 8/8 must-haves, 4/4 reqs; Playwright-verified both themes) |
| 13 | Docs Content Pipeline | DOCS-01 | ✅ Complete & verified 2026-06-08 (3/3 plans; verifier 4/4 SCs PASSED; FOWT UAT passed; code review 0-crit/3-warn) |
| 14 | Changelog Pipeline | CHG-01–03 | ✅ Complete & verified 2026-06-08 (2/2 plans; verifier 7/7 must-haves + 3/3 SCs PASSED; code review 2-crit/3-warn ALL fixed inline before verify) |
| 15 | Marketing Home | HOME-01–05 (SITE-03 deferred→Vercel) | ✅ Complete & verified 2026-06-08 (3/3 plans; verifier 5/5 must-haves + SC-1..5; code review 2-crit/3-warn fixed inline; both-theme Playwright proof) |
| 16 | 3D Layer | GFX-01–04 | ✅ Complete & verified 2026-06-09 (3/3 plans; GFX-01..04 green on prod build, FCP 560ms; maintainer UAT approved after 3 hive rounds) |
| 17 | SEO & Static Assets | SEO-01 | ✅ Complete & verified 2026-06-09 (3/3 plans; seo_spec SC-1..3 + gfx + home green; maintainer-approved flat-hive OG card) |
| 18 | Full Content Authoring | DOCS-02–09 | ✅ Complete & verified 2026-06-09 (6/6 plans; accuracy_spec AC-1..3 green; maintainer AC-5 sign-off vs THREAT-MODEL.md; nudge mode:block deviation per D-04; styling deferred) |
| 19 | Test Suite & CI | QA-01, QA-02 | Not started |

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

- Total plans completed: 73
- Average duration: ~10 min/plan

## Accumulated Context

### Decisions

Recent decisions from Phase 11 (v1.3.0 - Scaffold & Toolchain Isolation, plan 11-01):

- Phase 11-01: web/ scaffolded with create-next-app@16.2.7 — App Router at web/app/ (no src/), Tailwind v4 CSS-first, Biome (no ESLint), TS `@/*`→`./*`. @16 had moved past the RESEARCH `--help` check, driving the deviations below.
- Phase 11-01: packageManager pinned to `pnpm@11.1.3` by hand (--use-pnpm no longer writes it). corepack `prepare pnpm@11.5.2 --activate` blocked by EPERM on C:\Program Files\nodejs → A3 fallback to active 11.1.3.
- Phase 11-01: authoritative lockfile is the REPO-ROOT `pnpm-lock.yaml` (pnpm workspace mode), NOT web/pnpm-lock.yaml as RESEARCH assumed; stale member lockfile deleted.
- Phase 11-01: create-next-app@16 generated a web/pnpm-workspace.yaml build-approval stub; pnpm 11.1.3 ABORTS (exit 1) a non-interactive install until the gate is decided. Resolved by deleting the web file + an explicit DENY at the root (`allowBuilds: sharp: false`, `ignoredBuiltDependencies: unrs-resolver`) — no untrusted lifecycle scripts run on install.
- Phase 11-01: root `.gitignore` web entries — `web/.source` has NO trailing slash so `git check-ignore` matches before Phase 13 creates the dir; existing `*.out` is an extension rule (not the out/ dir), so `web/out/` is still required.
- Phase 11-01: Geist scaffold fonts kept in layout.tsx because globals.css (left untouched) references their CSS vars; design-system fonts arrive in Phase 12.

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

Last session: 2026-06-09 (continued → EXECUTED + COMPLETED Phase 18). Ran `/gsd-execute-phase 18` INLINE on main (use_worktrees=false; subagents lack node/pnpm — I acted as the executor). 6/6 plans, ~14 atomic commits. **W1 18-01** (`71e3851`/`f61bb07`): `web/tests/accuracy_spec.py` (pure-Python AC-1 source_doc+referenced-path-exists / AC-2 unenforced-feature labels / AC-3 no phantom commands — cloned from seo_spec.py) + `web/components/docs/unenforced-callout.tsx` (`UnenforcedCallout`, amber `var(--amber)` dual-theme clone of breaking-change-callout) registered in mdx-components.tsx. RED until content landed (correct). **W2 18-02..05** authored all 8 docs sections from the source-of-truth (read THREAT-MODEL.md whole + harness-support-matrix.md + nudge.md + config.go + main.go): getting-started, installation (EXACT cosign/SLSA/SBOM from THREAT-MODEL §7, lowercase `bantuson` source-uri), configuration, cli-reference (exhaustive H2-per-command tree), security (posture + ALL known gaps co-located on one page), integration (Hermes fail-open / Kilo-Trae UNGUARDED / Claude-Code-only / `--bind 0.0.0.0` caveats each at point-of-use), troubleshooting, audit-log. **All stub inaccuracies corrected:** `hooks install --target` (not `--hook`), removed `check --input`, `hooks status`→`diag`, `catalogs rebuild`→`catalogs sync`, audit log = single `beekeeper.ndjson` (no rotation). **W3 18-06** (`e5847fd`): full suite GREEN (`pnpm build` exit 0, all 8 sections in out/docs/; accuracy_spec AC-1..3 + seo_spec + home_spec + gfx_spec all exit 0) → maintainer reviewed the served build at localhost:3000 vs docs/THREAT-MODEL.md and **APPROVED** the AC-5 accuracy gate. **CRITICAL ACCURACY DEVIATION (reusable):** the planner/RESEARCH §7 OQ-1 AND `docs/nudge.md`'s field-ref were WRONG — they claimed `config set nudge.mode` only accepts soft|hard and that block isn't settable. I verified `internal/config/config.go`: `legalNudgeModes={soft,hard,block}`, `ValidateNudgeConfig` ACCEPTS block (explicit `ValidateNudgeConfig{Mode:"block"}`→nil test + `TestEvaluateBlockMode`); `ensureNudgeBlockDefault` sets `mode=block` on first `hooks install`; `require_hardened:true` is a SEPARATE no-hardened-PM block trigger. Documented `mode:block` accurately per D-04 (code wins) — recorded in 18-03-SUMMARY. (The plan's `config set nudge.mode block`==0 grep still passed: I show the `soft` opt-down form, never that literal.) `docs/nudge.md` mode field-ref is now stale relative to the web docs — Go-side doc fix is out of scope. Tracking hand-managed (skipped phase.complete SDK verb — frontmatter caveat); REQUIREMENTS DOCS-02..09→Complete, ROADMAP Phase-18 row→6/6 Complete, STATE reconciled (8/9 phases, 24/24 plans, 89%). Docs **styling** (stock Fumadocs theme — the "looks old/basic" concern) remains separately DEFERRED (`.planning/todos/pending/docs-styling-polish.md`). **NEXT: `/gsd-plan-phase 19`** (Test Suite & CI, QA-01/02 — the LAST v1.3.0 phase; the docs-styling-polish backlog item is also still pending).

Last session: 2026-06-09 (continued → PLANNED Phase 18). Ran `/gsd-plan-phase 18` (Full Content Authoring, DOCS-02–09). Two maintainer decisions up front via AskUserQuestion: (1) **keep docs styling SEPARATE** — Phase 18 is pure content authoring; the deferred "docs look old/basic" Fumadocs-theme redesign (`.planning/todos/pending/docs-styling-polish.md`) becomes its own future phase with its own UI-SPEC, so NO UI-SPEC here despite ROADMAP "UI hint: yes"; (2) **research-first, inline** (Phase 13–17 precedent). Pipeline, all hand-managed inline on main: 18-CONTEXT.md (D-01..D-05) → gsd-phase-researcher (sonnet, HIGH conf) → 18-VALIDATION.md → gsd-pattern-mapper (sonnet, 11/11 analogs) → gsd-planner (opus, 6 plans) → gsd-plan-checker (sonnet, PASSED all dims; 7c N/A; one non-blocking advisory on 18-01 T2's deliberate pnpm-build verify). **The research paid off the research-first choice** — it caught real inaccuracies in the existing Phase-13 stubs that the plans now actively correct: `beekeeper hooks install` flag is `--target` NOT `--hook`; `beekeeper check --input` does not exist; `beekeeper hooks status` does not exist (→`beekeeper diag`); `beekeeper catalogs rebuild` does not exist (→`catalogs sync`); audit log is a single `beekeeper.ndjson` with NO daily rotation (stub claimed rotated/compressed); the `allow_remote_gateway` config gate the `--bind` help text promises is NOT implemented. Accuracy gate (DOCS-09) = a new pure-Python `web/tests/accuracy_spec.py` (AC-1 source_doc frontmatter + referenced path exists / AC-2 unenforced-feature labels present where `release_age`/`lifecycle_script_allowlist` mentioned / AC-3 no phantom commands) cloned from seo_spec.py, PLUS a blocking maintainer review vs `docs/THREAT-MODEL.md`. New reusable component `UnenforcedCallout` (amber dual-theme clone of breaking-change-callout.tsx, `var(--amber)` raw token NOT `--color-bk-*`). Waves: W1 18-01 infra → W2 18-02/03/04/05 parallel (8 docs sections, zero file overlap) → W3 18-06 full-suite gate + AC-5 human checkpoint (autonomous:false). 5 doc commits (d945920 CONTEXT, 1ed1593 RESEARCH, 553d592 VALIDATION, 5deeeb1 PATTERNS, 03b876c plans+ROADMAP). STATE+ROADMAP hand-updated (skipped state.planned-phase — frontmatter caveat); REQUIREMENTS unchanged (DOCS-02..09 already mapped). Resolver FINE for slug `full-content-authoring`. **NEXT: `/gsd-execute-phase 18`** (INLINE on main — subagents lack node/pnpm; execute W1 before W2; 18-06 T2 is a never-auto-approvable human accuracy checkpoint).

Last session: 2026-06-09 (continued → EXECUTED + COMPLETED Phase 17). Ran `/gsd-execute-phase 17` inline on main (use_worktrees=false; subagents lack node/pnpm). 3/3 plans + atomic commits. 17-01: BASE_URL/SITE_NAME const + seo_spec.py harness (ran red Wave-0 then green). 17-02: layout metadataBase + title template + shared OG/twitter; home + docs + changelog canonicals. 17-03: sitemap.ts (13 locs) + robots.ts force-static (flat out/ files; A1 fallback unneeded). **KEY DEVIATION (reusable Next.js fact):** nested catch-all routes must set ONLY `alternates.canonical` and NOT return `openGraph` — a nested route's openGraph REPLACES the inherited one wholesale (shallow merge), stripping the root `app/opengraph-image.png` file-convention og:image from every doc/changelog page (SC-2 failed on 12 pages until openGraph.url was removed). **OG card:** 3 maintainer rounds (animated-style rejected → legibility fix → flat hero-hive.svg approved); authored as HTML + rendered to a 1200×630 PNG via the installed Playwright (no new dep); Next dev server caches the generated OG image so a `.next` clear + restart was needed for live review. **+2 maintainer hero tweaks** (committed separately, style(home) `b162e92`): removed the 'v1.3.0 shipped: self-protection + TUI policy editor' pill; moved 'Read the docs' onto the go-install chip's row. Final gate GREEN (build + seo_spec + gfx_spec + home_spec all exit 0). Docs-styling concern (maintainer: docs 'look old and basic') captured to `.planning/todos/pending/docs-styling-polish.md` (deferred, NOT a Phase-17 redesign). Tracking hand-managed (skipped state.begin-phase/phase.complete — frontmatter caveat). Resolver fine for slug `seo-static-assets`. **NEXT: Phase 18 (Full Content Authoring, DOCS-02–09).**

Last session: 2026-06-09 (continued → PLANNED Phase 17). After finalizing Phase 16, ran `/gsd-plan-phase 17` (SEO & Static Assets). Maintainer chose: skip discuss / capture inline, research-first, canonical base URL = `https://beekeeper.vercel.app` (the Vercel project URL; SITE-03 deploy still deferred). Pipeline: 17-CONTEXT.md (inline, D-01..D-05) → researcher (sonnet, HIGH conf — confirmed: static `app/opengraph-image.png` not Edge `ImageResponse`; `sitemap.ts`/`robots.ts` need `export const dynamic='force-static'`; `metadataBase` is the one missing root change; canonical/og on Fumadocs catch-all routes; pure-Python `seo_spec.py` file-walk for verification) → 17-VALIDATION.md → 17-PATTERNS.md → planner (opus) 3 plans/2 waves → plan-checker PASSED (12/12 dims; the only blocker was the recurring Dim-11 RESEARCH `## Open Questions (RESOLVED)` annotation gap — fixed inline, NO plan change, re-verified clean). Commits: 4ec560c CONTEXT, eda3615 RESEARCH, fc862a5 VALIDATION, 9d87f80 PATTERNS, d4c880f plans, + RESEARCH annotation fix. ROADMAP + STATE hand-updated (did NOT run state.planned-phase — frontmatter-corruption caveat). UI-SPEC gate false-positived (ROADMAP UI-hint:no) → proceeded as --skip-ui. **NEXT: `/gsd-execute-phase 17`** (17-02 Task 3 = blocking OG-card human checkpoint; execute INLINE on main — subagents lack node/pnpm; phase-resolver is fine for slug `seo-static-assets`, unlike `16-3d-layer`).

Last session: 2026-06-09 (resumed → FINALIZED Phase 16). Restored from HANDOFF.json + 16-3d-layer/.continue-here.md: Phase 16 was executed & green but paused at the finalization gate. Maintainer requested a hive rework (round 3) — applied: darker back depth ring + short amber center→cone light-streaks (`Spoke`) in hero-canvas.tsx, defense-in-depth cards reordered Default→Privileged→Optional (how-it-works.tsx), command-field scrollbars removed (no-scrollbar @utility in globals.css). Maintainer approved on :3000 ("Good → finalize"). Full gate re-run GREEN on the production build (pnpm build exit 0; gfx_spec.py GFX-01..04 PASS, FCP 560ms; home_spec.py PASS). 16-03-SUMMARY.md written; STATE/ROADMAP/REQUIREMENTS reconciled (GFX-01..04 done); HANDOFF.json + 16-3d-layer/.continue-here.md deleted (one-shot); dev servers stopped. Committed inline on main. **Phase 16 COMPLETE. NEXT: Phase 17 (SEO & Static Assets, SEO-01).** NOTE for future sessions: GSD phase-resolver can't find "16-3d-layer" (greedy `3d` token) → tracking is hand-managed; do NOT spawn gsd-executor (subagents lack node/pnpm).

Last session: 2026-06-07 (resumed) — restored from HANDOFF.json (2026-06-07T11:56). v1.3.0-seed work complete & committed (e1b907b/307fbad/999fa39/95a99e6 + wip e6962e6); tree clean. Proceeding to next action: formalize v1.3.0 milestone.

Last session: 2026-06-06 (resumed)
Stopped at: Big dogfood session. (1) handoff #9 scan→pollen E2E validated. (2) #13 scan audit-bloat FIXED. (3) Wired beekeeper as a LIVE Claude Code hook (~/.claude/settings.json, merged w/ GSD; backup .pre-beekeeper-20260606.bak). (4) Found pnpm-detect flaky on Windows (42-92%). (5) Built nudge `mode=block` (detection-INDEPENDENT enforcement, offers pnpm+bun) — PROVEN blocking live npm install via the hook. (6) Fixed pkgparse compound/env bypass (was defeating nudge AND catalog block). 3 commits on main: 6691585 fix(scan), 96f0a7b fix(pkgparse), 0028392 feat(nudge). Live config: %APPDATA%/beekeeper/config.json mode=block.
Resume file: memory/nudge-block-mode-and-bypass-fix.md + HANDOFF.json. Open: block-as-default decision; block-mode false-positive on quoted "&& npm install" (e.g. commit msgs → use git commit -F); optional /gsd-code-review, /gsd-new-milestone v1.3.0.

## Operator Next Steps

- **v1.2.0 (current):** ALL phases (6,7,8,9) ✅ **complete & verified**. Milestone audit (`v1.2.0-MILESTONE-AUDIT.md`, `tech_debt` — NO blockers) → maintainer scope "Everything" → **Phase 9 "v1.2.0 Tech-Debt Cleanup" EXECUTED (5/5 plans + fix `ef4ea97`) + VERIFIED passed 9/9** (`09-VERIFICATION.md`). All audit findings resolved: CLEAN-01 hermetic CORR E2E (signed non-wildcard fixture, blocks offline), CLEAN-02 LoadLayered Nudge-pointer merge (root fix), CLEAN-03 Phase-6 Nyquist COMPLIANT, CLEAN-04 handler comment, HARDEN-01/02/03 SPATH evasion edges (ancestor-symlink dual-form + Windows ADS/trailing-dot + verb word-boundary), DRIFT-01 live version_drift npm registry query (fail-open, floors never bumped). Release gates GREEN (go build + full unit suite + `-tags e2e` 15.4s + `-tags fuzz` incl. policy post-fix + TestPathImportsArePure + TestOverlayAllowCannotDowngradePathBlock). Phase 9 ran SEQUENTIALLY on main (`use_worktrees=false`); executors were told NOT to do phase-number-keyed SDK tracking writes — orchestrator updated STATE/ROADMAP/REQUIREMENTS MANUALLY. **NEXT:** re-run `/gsd-complete-milestone v1.2.0` — fold `v1.2.0-MILESTONE-AUDIT.md` + `09-VERIFICATION.md` into the archive (optionally `/gsd-verify-work 9` for conversational UAT first). (Phase 8's 08-05 executor had hit a mid-run session limit AFTER committing its 3 task commits — closed out manually by writing its SUMMARY, no re-execution.) Known INFO (now folded into Phase 9): gateway `realMetadataFetch` (drift.go) is a production stub returning empty, so `version_drift` emits nothing live until a real registry query is added (floor auto-update is Out-of-Scope; drift is informational-only). Also confirmed broken this session: `phase-plan-index 8` flattens all 8 plans into "wave 1" (ignores the `wave:` frontmatter) — execution used the authoritative PLAN-frontmatter wave order W1{01,05} W2{02,03} W3{04} W4{06} W5{07,08}. Code review done (08-REVIEW.md): 0 critical, 4 warning — ALL FIXED (WR-01 redact nudge command fields in audit log, WR-02 parseInt overflow guard, WR-03 parallelize PM probes so detection stays ~2s within the hook budget, WR-04 bound drift goroutine) + re-confirmed (build + tests + live E2E green); 5 Info deferred. Caveat: `go test -race` is CI-only (CGO disabled locally) — the CI race pass on internal/nudge is the authoritative confirmation for WR-03's concurrency change. NOTE (still applies to execute): the `gsd-sdk` init.plan-phase / state.begin-phase / phase.complete resolvers map bare phase numbers (7, 8) to ARCHIVED v1.0.0 dirs under `.planning/milestones/v1.0.0-phases/` and corrupt STATE frontmatter progress — the live v1.2.0 phases are in `.planning/phases/NN-slug/`. Pass explicit paths to agents and update STATE/ROADMAP/REQUIREMENTS tracking MANUALLY (do not trust phase-number-keyed SDK writes). `init.execute-phase`, `roadmap.get-phase`, and `init.phase-op` resolved correctly; `init.plan-phase`/`state.begin-phase` did not. CONFIRMED this session: `init.plan-phase 8` → `.planning/milestones/v1.0.0-phases/08-tui-dashboard` (wrong); plan-phase was driven with explicit live paths instead.
- **v1.1.0 (parked release):** when ready, run `docs/release-runbook.md` (push `../pollen` + cut signed tags `pollen.2/.3/.4/.5` + cosign verify + create/push `bantuson/beekeeper`), then finish 05-05 Task 3 (tracking + verify) and close v1.1.0 via `/gsd-complete-milestone`. Resume context: 05-05 `.continue-here.md` + `docs/release-runbook.md`. Do NOT close v1.1.0 before this runs.
- **Still pending (from v1.0.0 close):** the beekeeper GitHub remote is created as part of the v1.1.0 runbook (Step 1: `gh repo create bantuson/beekeeper`).
