---
gsd_state_version: 1.0
milestone: v1.2.0
milestone_name: "Runtime Behavioral Hardening"
status: executing
stopped_at: "Completed 06-01-PLAN.md — CORR-01/02 severity escalation + sanity gate"
last_updated: "2026-06-03T19:09:01Z"
last_activity: 2026-06-03
progress:
  total_phases: 5
  completed_phases: 4
  total_plans: 20
  completed_plans: 20
  percent: 95
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-03)

**Core value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Current focus:** Phase 6 — Corroboration Severity Hardening

> ⏸ **v1.1.0 "Pollen" is PARKED, not closed** — paused at the 05-05 maintainer release checkpoint. To resume the release: see `HANDOFF.json`, `.planning/phases/05-contribution-back-milestone-close/.continue-here.md`, and `docs/release-runbook.md`. The four signed-tag releases remain in the "Deferred Items" table below. Do not archive v1.1.0 until the runbook is run + 05-05 Task 3 completes.

## Current Position

Phase: 6 (Corroboration Severity Hardening) — EXECUTING
Plan: 2 of 3
Status: Plan 06-01 complete — CORR-01/02 severity escalation + sanity gate shipped
Last activity: 2026-06-03 -- 06-01 complete

Progress (v1.2.0): [██████████] 95%

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
| 6 | Corroboration Severity Hardening | CORR-01, CORR-02 | Not started |
| 7 | Sensitive-Path Runtime Enforcement | SPATH-01–04 | Not started |
| 8 | Package-Manager Nudge + Behavioral Test Suite | NUDGE-01–08, BTEST-01–03 | Not started |

## Performance Metrics

**Velocity (v1.0.0):**

- Total plans completed: 61
- Average duration: ~10 min/plan

## Accumulated Context

### Decisions

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

### Open Research Flags (v1.2.0)

- **Before Phase 8 plan:** commit Flag 2 (NUDGE detection cache: Position A file-cache vs Position B gateway/shim-only) and Flag 4 (installPrefixes: extract `internal/pkgparse/` vs accept third copy with cross-reference comments). Both decisions determine `detect.go` signature, test strategy, and BTEST E2E fixtures — must be resolved in `/gsd-discuss-phase 8`.
- **During Phase 8:** Windows corepack-shimmed pnpm `cmd.exe` startup time under the 2s detection timeout — live CI timing needed.
- **Flag 5 (PRD corrections):** `minimumReleaseAge` default is 1440 minutes (not 60); Node 22 is Maintenance LTS (Node 24 is Active LTS) — apply before implementation in Phase 8.

### Blockers/Concerns

- Phase 8 (NUDGE): two unresolved architectural decisions (Flag 2 + Flag 4) must be settled in discuss/plan before `detect.go` is written — surfaced in Phase 8 success criteria and must not be deferred to implementation
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

## Session Continuity

Last session: 2026-06-03T19:11:32.307Z
Stopped at: Roadmap creation complete — ready to plan Phase 6.
Resume file: None

## Operator Next Steps

- **v1.2.0 (current):** run `/gsd-plan-phase 6` to plan Phase 6 (Corroboration Severity Hardening — CORR-01/02, pure internal/policy change, no architectural decisions outstanding). Then Phase 7 (SPATH wiring), then Phase 8 (NUDGE — resolve Flag 2 + Flag 4 in discuss/plan before planning).
- **v1.1.0 (parked release):** when ready, run `docs/release-runbook.md` (push `../pollen` + cut signed tags `pollen.2/.3/.4/.5` + cosign verify + create/push `bantuson/beekeeper`), then finish 05-05 Task 3 (tracking + verify) and close v1.1.0 via `/gsd-complete-milestone`. Resume context: `HANDOFF.json` + 05-05 `.continue-here.md`. Do NOT close v1.1.0 before this runs.
- **Still pending (from v1.0.0 close):** the beekeeper GitHub remote is created as part of the v1.1.0 runbook (Step 1: `gh repo create bantuson/beekeeper`).
