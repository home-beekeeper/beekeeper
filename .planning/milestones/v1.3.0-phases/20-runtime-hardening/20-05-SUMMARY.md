---
phase: 20-runtime-hardening
plan: 05
subsystem: docs
tags: [sentry, honesty, threat-model, marketing, detection-only, rules-test]

requires:
  - phase: 20-runtime-hardening
    provides: "20-03 SENTRY-006/007 + agent ancestry; 20-04 SENTRY-008 + file-write — the rules these docs now describe and the synthetic tests already cover"
provides:
  - PROJECT.md Sentry Daemon + Out-of-Scope honesty corrections (and stale 20-02 LlamaFirewall bullets fixed)
  - docs/THREAT-MODEL.md §8 rewrite (stale don't-fire/no-drops removed; 5 honest residual gaps)
  - home honesty edits (how-it-works/feature-cards Sentry cards + new honesty-callout Sentry gap)
affects: [20-06]

tech-stack:
  added: []
  patterns:
    - "Documentation honesty is part of the threat model: marketing claims are graded against code reality, overstatement is a tracked gap"

key-files:
  modified:
    - .planning/PROJECT.md
    - docs/THREAT-MODEL.md
    - web/components/home/how-it-works.tsx
    - web/components/home/feature-cards.tsx
    - web/components/home/honesty-callout.tsx

key-decisions:
  - "Task 3's synthetic SENTRY-006/007/008 + watchlist rules_test cases already landed in 20-03/20-04 and comprehensively cover the fire/no-fire behaviors (006 fire + integrated-terminal no-double-fire; 007 fire + loopback/RFC1918/IPv4-mapped no-fire + warn-first; 008 fire + benign/non-monitored no-fire + per-path dedup; watchlist 2-cloud-cred SENTRY-001). Verified green here rather than duplicating — the AC greps (each >= 1) are satisfied."
  - "hero.tsx left unchanged: its 'a hijacked agent cannot act on your machine without Beekeeper's permission' is the hook/gateway enforcement thesis (true for the layer that blocks), not a Sentry-detection claim — per the plan's conditional (only adjust if it makes the blanket *Sentry* claim)."
  - "Also corrected the PROJECT.md LlamaFirewall bullets that 20-02 had just made stale (unix/named-pipe → loopback-TCP+token; AlignmentCheck/Together-AI removed; real CodeShield; silent fail-open fixed) — a doc-accuracy fix caused by this phase, folded in rather than left false."
  - "Out-of-Scope reframed to reflect 20-03/20-04: agent terminals + persistence writes are now IN scope (SENTRY-006/008); CI runners, DNS, process-memory, and legitimate-endpoint exfil remain the honest residual gaps."

patterns-established: []

requirements-completed: [SENT-09, SENT-10]

duration: ~35 min
completed: 2026-06-10
---

# Phase 20 Plan 05: Sentry Honesty Corrections + Synthetic-Test Verification (SENT-09/10) Summary

**Make the docs and home page tell the truth about the Sentry engine: PROJECT.md and THREAT-MODEL §8 no longer claim SENTRY-004/005 "do not fire" or that fanotify "does not count drops" (both false in code), the home page no longer implies Sentry catches "novel campaigns the catalog has never seen", and the residual gaps (CI/daemons, DNS, process-memory, legitimate-endpoint exfil, detection-only) are stated plainly. The synthetic rule tests were already comprehensive from 20-03/20-04 and are verified green.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3 (Task 3 satisfied by existing 20-03/20-04 tests)
- **Files:** 5 modified

## Accomplishments

- **Task 1 (PROJECT.md + THREAT-MODEL §8):** Sentry Daemon bullet now names SENTRY-001..005 + the editor-descendant gate + detection-only + scope; a new line records the v1.3.0 SENTRY-006/007/008 + agent ancestry + file-write + watchlist additions. Out of Scope gains the residual gaps. THREAT-MODEL §8 removed the stale don't-fire/no-drops text and now lists five honest residual gaps. Also fixed the PROJECT.md LlamaFirewall bullets 20-02 had made stale.
- **Task 2 (home):** how-it-works + feature-cards Sentry cards reframed to editor+agent-host-scoped, detection-only; honesty-callout adds a Sentry scope/detection-only gap entry (raw theme tokens, no em-dashes). hero left as-is (enforcement-thesis, not a Sentry claim).
- **Task 3 (tests):** the synthetic SENTRY-006/007/008 + watchlist cases already exist from 20-03/20-04 and cover all required fire/no-fire behaviors; verified green (no duplication added).

## Task Commits

1. **Tasks 1+2: honesty corrections (PROJECT/THREAT-MODEL + home)** - `304fedc` (docs)
2. **Task 3:** no code change — existing 20-03/20-04 synthetic cases satisfy the AC and pass.

## Deviations from Plan

- **Task 3 added no new test code:** the synthetic 006/007/008 + watchlist cases were already landed (and committed) under 20-03/20-04 and are comprehensive; this plan verifies them rather than writing duplicates. AC greps (`SENTRY-006/007/008` each >= 1 in rules_test.go) all pass (7/18/13).
- **Scope addition (doc accuracy):** fixed the PROJECT.md LlamaFirewall bullets that 20-02 made stale (in-scope as honesty, caused by this phase's sibling plan).
- **hero.tsx left unchanged** per the plan's conditional (its claim is the hook/gateway enforcement thesis, not a Sentry claim).

## Issues Encountered / Carried Forward

- The untracked prototype HTMLs (`beekeeper-docs.html`, `beekeeper-tui-prototype.html`) flagged by analysis §6.4 as "worst offenders" (stale R5 naming, "zero-day / anything pre-disclosure") are git-untracked and not part of the build/site; left out of scope (pre-existing, not shipped). Recommend deleting or aligning before they ever ship.

## Verification

- `go test ./internal/sentry/...` + `go vet ./internal/sentry/...` (native + GOOS=linux) all green.
- web `pnpm build` exit 0; `accuracy_spec` + `seo_spec` + `home_spec` + `gfx_spec` all PASS.
- AC greps: `do not fire in production today`=0, `detection-only` present in THREAT-MODEL §8 + feature-cards, `novel campaigns…`=0, `regardless of catalog knowledge`=0, Sentry gap in honesty-callout, no new `color-bk-` token.

## Next Phase Readiness

- 20-06 (Tier-3 W4 DNS, SENT-11) is the remaining OPTIONAL stretch (depends on 20-04 + 20-05). Droppable.
- The only outstanding non-optional item in Phase 20 is the 20-02 human HF-license live-bootstrap verification.

---
*Phase: 20-runtime-hardening*
*Completed: 2026-06-10*
