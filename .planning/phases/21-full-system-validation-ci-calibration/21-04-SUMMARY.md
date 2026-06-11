---
phase: 21-full-system-validation-ci-calibration
plan: 04
subsystem: testing
tags: [e2e, claude-code, validation-register, validation-posture, readme, honesty]

requires:
  - phase: 21-full-system-validation-ci-calibration
    provides: "21-01 coverage-allowlist taxonomy + 21-02 deny families (sourced into the docs)"
provides:
  - "Claude Code --hook exit-2 canary e2e (the documented true-block reference, VAL-05)"
  - "docs/validation-register.md: 16-harness + gated-model manual live-block register with sign-off"
  - "docs/validation-posture.md: Tier A/B/C model + allowlist taxonomy (auditable coverage claim)"
  - "README harness count corrected 15->17 / 14->16"
affects: [milestone-close (Tier-C register + posture are the honest release artifact)]

tech-stack:
  added: []
  patterns:
    - "Live e2e sub-case driving the real binary with --hook (exit-2 hook contract)"
    - "Manual validation register with per-row sign-off (Tier C)"

key-files:
  created:
    - docs/validation-register.md
    - docs/validation-posture.md
  modified:
    - internal/check/e2e_test.go
    - README.md

key-decisions:
  - "VAL-05 asserts exit 2 via --hook claude-code (the Phase-10 hook contract), NOT the exit-1 default-mode block (Pitfall 2)"
  - "The 16 non-Claude-Code harness rows + the gated-22M-model e2e stay UNSIGNED/PENDING by design — Tier C is irreducibly manual (D-07)"
  - "Maintainer approved the register/posture/README as the honest Tier-C artifact (blocking checkpoint)"

patterns-established:
  - "An unsigned register row honestly means contract-shape-tested but not live-block-verified"

requirements-completed: [VAL-05, VAL-06, VAL-07]

duration: ~40min
completed: 2026-06-11
---

# Phase 21 Plan 04: Live e2e + Tier-C Register + Honesty Docs Summary

**The Claude Code `--hook` exit-2 canary block proven end-to-end against the real binary, plus the honest Tier-C manual register (16 harnesses + the gated-model e2e) and the auditable validation posture — with the README harness count corrected to 17/16. Maintainer-approved.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 3 (2 autonomous + 1 blocking maintainer checkpoint)
- **Files created:** 2 docs; **modified:** 2 (e2e_test.go, README.md)
- **Production files modified:** 0 (test + docs only — D-03)

## Accomplishments
- **VAL-05** — a new `//go:build e2e` sub-case drives the real binary with `beekeeper check --hook claude-code` for canary reads of `~/.ssh/id_rsa` AND `~/.aws/credentials`, asserting **exit 2** (the hook deny contract, NOT exit 1) + Family-A `permissionDecision:"deny"` on stdout + audit `decision:"block"`. Proven green on Windows (the one locally-runnable harness). The existing exit-1 default-mode cases are preserved.
- **VAL-06** — `docs/validation-register.md` enumerates a live-block procedure for all 16 non-Claude-Code harnesses (Prereq/Install/Drive/Expected/Result/sign-off; deny families sourced from `deny_render.go` + `harness-support-matrix.md`) + the gated-22M-model LlamaFirewall e2e, whose sign-off honestly reads **PENDING** on the human HF-license gate.
- **VAL-07** — `docs/validation-posture.md` documents the Tier A/B/C model + the fail-closed coverage-allowlist taxonomy; `README.md` corrected to **17 / 16** with Tier 3 now listing Kilo/Trae/Continue/OpenClaw (sums to 17) and links to both new docs.
- **Maintainer sign-off (Task 3)** — the blocking honesty checkpoint: maintainer reviewed and **approved** the register/posture/README as honest, with the 16 harness rows + gated model left unsigned/PENDING by design.

## Task Commits
1. **Task 1: --hook exit-2 canary e2e** — `test(21-04)` (VAL-05)
2. **Task 2: register + posture + README fix** — `docs(21-04)` (VAL-06/07)
3. **Task 3: maintainer sign-off** — approved via AskUserQuestion checkpoint (no code change)

## Decisions Made
- **Exit 2, not exit 1.** The existing e2e proved the exit-1 default-mode block; Phase 10's hook contract is exit 2 via `--hook`. The new case drives `--hook claude-code` and asserts exit 2 + Family-A deny JSON (RESEARCH Pitfall 2). This made the documented true-block reference exit-2-accurate.
- **Unsigned-by-design.** Tier C is irreducibly manual; an unsigned register row honestly means contract-shape-tested but not live-block-verified. The gated-model entry stays PENDING on the human HF-license gate (D-07).

## Deviations from Plan
- **New sub-case rather than extending `runCase`.** The plan suggested extending `runCase` to take extra args; instead a self-contained sub-case builds its own `exec.Command(binPath, "check", "--hook", "claude-code")` capturing stdout (which `runCase` does not), avoiding changes to the 4 existing call sites. Same outcome, smaller blast radius.

## Issues Encountered
None — both canary paths blocked at exit 2 on the first run.

## Next Phase Readiness
- Phase 21 is complete (all 4 plans). The Go safety harness is validated at every tier: Tier A gate-enforced locally, Tier B authored for the CI matrix (CI-validated; live run at first push), Tier C captured in the signed register.
- Remaining external follow-ups (not Phase 21): the Phase-20 20-02 HF-license live bootstrap (which also closes the register's gated-model row), then `/gsd-complete-milestone v1.3.0`.

---
*Phase: 21-full-system-validation-ci-calibration*
*Completed: 2026-06-11*
