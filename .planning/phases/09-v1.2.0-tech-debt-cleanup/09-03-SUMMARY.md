---
phase: 09-v1.2.0-tech-debt-cleanup
plan: 03
subsystem: config-and-spath
tags: [clean-02, clean-04, harden-01, layered-config, nudge-merge, spath, dual-form, fail-closed, defense-in-depth]

# Dependency graph
requires:
  - phase: 09-v1.2.0-tech-debt-cleanup
    plan: 01
    provides: "canonicalizePathForms(raw) []string — dual-form (lexical + EvalSymlinks-resolved) canonicalizer (HARDEN-01 helper; consumer wiring deferred to this plan)"
provides:
  - "config.LoadLayered now merges the Nudge *NudgeConfig pointer at its root (mergeNudge) and guarantees a non-nil, ValidateNudgeConfig-validated cfg.Nudge (fail-closed) — CLEAN-02 root-cause fix"
  - "handler.go runCheck + integration_test.go runCheckWithIndex iterate canonicalizePathForms (block on ANY form) — HARDEN-01 wired end-to-end through the live check pipeline"
  - "Corrected handler.go decision-merge comment: overlay -> SPATH -> NUDGE (NUDGE last) — CLEAN-04"
affects: [spath, credential-block, layered-config, nudge]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pointer-field layered merge: mergeNudge mirrors mergeLlamaFirewall src-wins-if-set discipline; a present project nudge object is authoritative for the §11 project-disable while a partial override never zeros lower-layer fields"
    - "Dual-form SPATH matching wired at the call site: iterate canonicalizePathForms, EvaluatePath each, mergeDecisions most-restrictive-wins (block on either lexical or symlink-resolved form)"
    - "Consumer nil-guards reclassified as defense-in-depth (kept, re-commented) once the root cause is fixed — protect direct zero-Config construction without being load-bearing"

key-files:
  created: []
  modified:
    - "internal/config/layered.go — mergeNudge() + Nudge call in merge(); LoadLayered non-nil+validated Nudge guarantee (fail-closed)"
    - "internal/config/layered_test.go — TestMerge_Nudge* merge-mechanics tests + TestLoadLayeredNudge{Defaulting,ProjectNudgeOverrideWins,ProjectNudgeModeOverride} consumer-facing assertions"
    - "cmd/beekeeper/nudge.go — status/check nil-guards + defaultNudgeConfigHelper re-commented as defense-in-depth (CLEAN-02); no runtime change"
    - "internal/check/handler.go — corrected merge-order comment (CLEAN-04) + dual-form SPATH loop (HARDEN-01) + nudge nil-guard re-commented as defense-in-depth"
    - "internal/check/integration_test.go — runCheckWithIndex dual-form SPATH mirror (lockstep, CR-02) + TestIntegrationAncestorSymlinkCredentialBlocks end-to-end regression"

key-decisions:
  - "Enabled-merge resolution reconciles §11 project-disable with partial overrides: a disable-only / bare project nudge object IS the Enabled assertion (enabled:false wins); when the project sets OTHER nudge fields, only an explicit enable applies so a mode-only override never silently disables (accepted LlamaFirewall.Enabled-style limitation, documented in mergeNudge)"
  - "LoadLayered defaults+validates Nudge just before the final FailMode validate() — mirrors Load's single-file contract so layered and single-file paths produce identical, fail-closed Nudge resolution"
  - "Consumer nil-guards (nudge.go x2, handler.go) KEPT and re-commented as defense-in-depth rather than removed — they protect direct config.Config{} construction in tests/callers that bypass LoadLayered"
  - "canonicalizePath left unchanged; the SPATH loops switched to canonicalizePathForms (the 09-01 helper) — dual-form blocks on either lexical or symlink-resolved form (fail-closed, ancestor-symlink-proof)"

patterns-established:
  - "Pointer-field merge in a layered config: src-wins-if-set with a documented bool-disambiguation rule for the 'present section is authoritative for its enable flag' case"
  - "Wire a deferred security helper (canonicalizePathForms) into BOTH the production loop and its test mirror in the same commit so the mirror never drifts (CR-02 discipline)"

requirements-completed: [CLEAN-02, CLEAN-04, HARDEN-01]

# Metrics
duration: 16min
completed: 2026-06-04
---

# Phase 9 Plan 03: LoadLayered Nudge Merge + Merge-Order Comment + HARDEN-01 Wiring Summary

**Fixes the CLEAN-02 root cause (config.LoadLayered now merges the Nudge pointer and guarantees a non-nil, validated cfg.Nudge so a partial project override or a future consumer never silently disables the nudge), corrects the stale CLEAN-04 merge-order comment to overlay -> SPATH -> NUDGE, and lands the HARDEN-01 dual-form fix end-to-end by switching both runCheck and runCheckWithIndex SPATH loops to canonicalizePathForms so an ancestor-symlink credential read blocks through the live pipeline.**

## Performance

- **Duration:** ~16 min
- **Tasks:** 3 (all `auto`; Tasks 1-2 `tdd="true"`)
- **Files modified:** 5 (exactly the plan's `files_modified` set)

## Accomplishments

- **CLEAN-02 — LoadLayered Nudge pointer merge (root cause).** `merge()` now calls a new `mergeNudge(dst, src *NudgeConfig) *NudgeConfig` for the `Config.Nudge` pointer field, mirroring the `mergeLlamaFirewall` src-wins-if-set convention. `LoadLayered` guarantees a non-nil, `ValidateNudgeConfig`-validated `cfg.Nudge` just before the final FailMode `validate()` — an absent nudge across all layers resolves to `DefaultNudgeConfig()`; an invalid merged nudge (e.g. project `mode:"aggressive"`) is rejected fail-closed. The §11 project-disable (`nudge.enabled:false`) wins, while a partial project override (`mode:"hard"`) never zeros the lower-layer Enabled/floors. The consumer-layer workaround (`defaultNudgeConfigHelper` + the `cfg.Nudge == nil` guards in `nudge.go` and `handler.go`) is retained but re-commented as defense-in-depth (protects direct `config.Config{}` construction), no longer load-bearing.
- **CLEAN-04 — Corrected merge-order comment.** The stale `handler.go` comment claiming "the sensitive-path block is merged LAST" is replaced with the real, numbered order: `ApplyPolicyOverlay (FIRST) -> SPATH -> NUDGE (LAST)`, all merged most-restrictive-wins, with the CR-02 rationale that a `package_allowlist` allow can never downgrade a SPATH or NUDGE block.
- **HARDEN-01 — Dual-form SPATH wired end-to-end.** Both the production `runCheck` (handler.go) and the test-mirror `runCheckWithIndex` (integration_test.go) SPATH loops now iterate `canonicalizePathForms(rawPath)` (the 09-01 helper) instead of the single `canonicalizePath`, calling `policy.EvaluatePath` on EACH form and merging most-restrictive-wins. A block on either the lexically-cleaned or the EvalSymlinks-resolved form blocks (fail-closed), so an ancestor-directory symlink can no longer resolve a `/.aws/` or `/.ssh/` fragment out of the path and dodge the blocklist. A new end-to-end regression (`TestIntegrationAncestorSymlinkCredentialBlocks`) plants an ancestor symlink and asserts the credential read blocks through the live `runCheckWithIndex` Result/exit code (not just the unit-tested helper).

## Task Commits

1. **Task 1: Merge the Nudge pointer in LoadLayered.merge() (CLEAN-02)** — `564104b` (feat)
2. **Task 2: Layered-config tests + document consumer guards as defense-in-depth (CLEAN-02)** — `350b44f` (test)
3. **Task 3: Correct merge-order comment (CLEAN-04) + wire HARDEN-01 dual-form SPATH** — `a06524a` (feat)

**Plan metadata:** STATE.md / ROADMAP.md / REQUIREMENTS.md are orchestrator-owned on this phase (phase-number-keyed `gsd-sdk` resolvers map bare `9` to the archived v1.0.0 phase 9 dir — see 09-CONTEXT.md tooling note). This executor wrote only the SUMMARY in the live dir.

## Files Created/Modified

- `internal/config/layered.go` — Added `mergeNudge()` (src-wins-if-set field merge with the documented Enabled-disambiguation rule) and the `dst.Nudge = mergeNudge(...)` call in `merge()`; added the non-nil + `ValidateNudgeConfig` guard in `LoadLayered` before the final `validate()`.
- `internal/config/layered_test.go` — Added `TestMerge_Nudge{DefaultedAtLayeredRoot,ProjectDisableWins,ProjectModeOverride,InvalidRejected}` (merge mechanics) and `TestLoadLayeredNudge{Defaulting,...}` consumer-facing assertions (Test A/B/C) that assert directly on `LoadLayered` output with no consumer helper.
- `cmd/beekeeper/nudge.go` — Re-commented the `status` and `check` `cfg.Nudge == nil` guards and `defaultNudgeConfigHelper`'s doc as defense-in-depth referencing CLEAN-02; no runtime change.
- `internal/check/handler.go` — Rewrote the decision-merge comment (CLEAN-04: overlay -> SPATH -> NUDGE); switched the `runCheck` SPATH loop to `canonicalizePathForms` (HARDEN-01); re-commented the nudge nil-guard as defense-in-depth (T-09-07).
- `internal/check/integration_test.go` — Switched the `runCheckWithIndex` SPATH loop to `canonicalizePathForms` in lockstep (CR-02); added `TestIntegrationAncestorSymlinkCredentialBlocks`; added the `path/filepath` import.

## Decisions Made

- **Enabled-merge disambiguation (the one non-trivial design point).** The RED phase exposed that an unconditional `out.Enabled = src.Enabled` clobbered the inherited default `true` to `false` for a `mode`-only project override (JSON-absent `enabled` decodes to Go-zero `false`). Resolved by `srcHasOtherSignal`: a disable-only / bare project nudge object IS the Enabled assertion (so `enabled:false` wins for §11), but when the project object carries OTHER non-zero fields, only an explicit `enabled:true` applies — a project that wants to disable AND set other fields must set `enabled:false` explicitly. This is the same accepted limitation documented on `mergeLlamaFirewall.Enabled`, and it is spelled out in the `mergeNudge` doc comment.
- **Guards kept, not removed.** The plan offered "remove OR document"; chose document — the guards protect direct `config.Config{}` construction (tests/callers that bypass `LoadLayered`) from a nil-pointer deref. Removing them would have been a latent foot-gun for exactly those callers.
- **canonicalizePath unchanged.** Per 09-01, the single-form helper stays for backward compat; only the two SPATH call sites adopt `canonicalizePathForms`.

## Deviations from Plan

None — plan executed exactly as written. The Enabled-merge fix during Task 1 GREEN was an intra-task RED→GREEN iteration (the failing `TestMerge_NudgeProjectModeOverride` caught the clobber before commit), not a deviation: TDD working as intended. `internal/policy` stayed pure (no edits there); no architectural changes; no out-of-scope fixes.

## Issues Encountered

- **Task 1 (self-corrected during GREEN, not a deviation):** First `mergeNudge` draft applied `src.Enabled` unconditionally, which a mode-only project override clobbered to `false`. The RED test `TestMerge_NudgeProjectModeOverride` failed and pinpointed it; fixed with the `srcHasOtherSignal` discriminator so both the §11 disable case and the partial-override case pass. No production behavior shipped with the bug.

## TDD Gate Compliance

- **Task 1** (`tdd="true"`): RED — four `TestMerge_Nudge*` tests failed against pre-fix code (`cfg.Nudge == nil` after merge dropped the pointer; invalid nudge not rejected). GREEN — `mergeNudge` + the LoadLayered guard turned them green (with the Enabled-disambiguation iteration). Impl + test committed together (`564104b`), acceptable for this coherent root-cause unit; the RED→GREEN intent is documented above.
- **Task 2** (`tdd="true"`): the three named consumer-facing assertions (`TestLoadLayeredNudge*`) assert on `LoadLayered` output with no consumer helper — they pass because Task 1 fixed the root cause; this task's deliverable is the proof + the guard reclassification. Committed as `test(...)` (`350b44f`).
- **Task 3** (`auto`, not tdd): `TestIntegrationAncestorSymlinkCredentialBlocks` is a regression that would NOT block on pre-fix single-form code (the single-form path can lose the `/.aws/` shape via the ancestor symlink); it blocks now via the dual-form lexical form.

## Threat Surface

No new security-relevant surface introduced beyond the plan's `<threat_model>`. The three registered threats are mitigated:
- **T-09-07** (LoadLayered Nudge merge): `mergeNudge` + non-nil/validated guarantee; consumer guards retained as defense-in-depth.
- **T-09-08** (ancestor-symlink credential read): dual-form `canonicalizePathForms` wired at both call sites; block on any form; fail-closed preserved.
- **T-09-09** (stale merge-order comment): comment corrected to the real overlay -> SPATH -> NUDGE order.

## Verification

- `go build ./...` — clean.
- `go vet ./internal/config/... ./internal/check/... ./cmd/beekeeper/...` — clean.
- `go test ./internal/check/ ./internal/config/ ./internal/policy/` — all green.
- `go test ./cmd/beekeeper/` — green.
- `TestPathImportsArePure` — green (internal/policy purity preserved; no policy edits).
- `TestOverlayAllowCannotDowngradePathBlock` — green (CR-02 ordering: overlay before SPATH; blocks on `/.ssh/` with `sensitive-path-policy` rule ID).
- `TestIntegrationAncestorSymlinkCredentialBlocks` — **ran (not skipped)** on this Windows dev host (symlink privilege available) and blocked on `/.aws/`, proving HARDEN-01 end-to-end.
- `TestLoadLayeredNudge*` and `TestMerge_Nudge*` — green (defaulting + project disable + mode override + invalid-rejected).

_Note: `go test -race` is CGO-gated / CI-only on this Windows box; plain `go test` is authoritative locally, CI confirms the race pass._

## Next Phase Readiness

- CLEAN-02, CLEAN-04, HARDEN-01 are complete in code and proven by tests that bite the pre-fix code. The remaining Phase 9 items (CLEAN-01 CORR E2E hermetic seed, CLEAN-03 Phase-6 Nyquist reconcile, HARDEN-02/03 already effective from Plan 01, DRIFT-01 real registry fetch) are owned by other plans.
- No blockers.

## Self-Check: PASSED

- `09-03-SUMMARY.md` exists in the live phase dir `.planning/phases/09-v1.2.0-tech-debt-cleanup/`.
- All task commits reachable: `564104b` (CLEAN-02 merge), `350b44f` (tests + guards), `a06524a` (CLEAN-04 + HARDEN-01).
- All five modified source files present and committed.

---
*Phase: 09-v1.2.0-tech-debt-cleanup*
*Completed: 2026-06-04*
</content>
</invoke>
