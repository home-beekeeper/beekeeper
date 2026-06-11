---
phase: 19-test-suite-ci
plan: 03
subsystem: testing
tags: [github-actions, ci, path-filter, parity, playwright, vitest, retirement]

requires:
  - phase: 19-test-suite-ci (19-01, 19-02)
    provides: full JS suite (vitest unit, playwright e2e, postbuild seo) + scripts
provides:
  - .github/workflows/web.yml — path-filtered Web CI with the 6 ordered QA-01 gates
  - .github/workflows/ci.yml bidirectional paths-ignore (web isolation, SC-1)
  - retirement of the 4 Python specs (parity proven) — JS toolchain is now the sole web test surface
affects: [test-suite-ci, milestone-v1.3.0-close]

tech-stack:
  added: []
  patterns: [path-filtered CI per workspace (paths include vs paths-ignore), .py->JS parity-then-retire]

key-files:
  created:
    - .github/workflows/web.yml
  modified:
    - .github/workflows/ci.yml
  deleted:
    - web/tests/home_spec.py
    - web/tests/gfx_spec.py
    - web/tests/seo_spec.py
    - web/tests/accuracy_spec.py

key-decisions:
  - "Verified web.yml/ci.yml by STATIC inspection only (D-03 — repo unpushed, no live GitHub run)"
  - "Retired the 4 .py specs only after BOTH directions proved green on the same out/ + byte-identical constants (D-01)"

patterns-established:
  - "web.yml paths: include filter + ci.yml paths-ignore mirror = bidirectional CI isolation (no cross-spin)"

requirements-completed: [QA-01, QA-02]

duration: ~35min
completed: 2026-06-10
---

# Phase 19 (Wave 3) — 19-03: Path-filtered web CI + Python-spec retirement

**A path-filtered `web.yml` running the six ordered QA-01 gates (lint→typecheck→unit→build→postbuild→e2e), bidirectional `paths-ignore` isolation on the Go `ci.yml`, and retirement of the four Python specs after JS parity was proven green on the same `out/` with byte-identical constants.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-06-10
- **Tasks:** 2
- **Files modified:** 6 (1 created, 1 modified, 4 deleted)

## Accomplishments
- `.github/workflows/web.yml`: path-filtered (web/**, pnpm-workspace.yaml, itself) Web CI on ubuntu-latest, least-privilege `permissions: contents: read`, first-party actions only, the 6 ordered gates with `playwright install chromium --with-deps` + failure-artifact upload.
- `.github/workflows/ci.yml`: `paths-ignore` (web/**, pnpm-workspace.yaml, web.yml) on BOTH push and pull_request — the only change; all 8 Go jobs unchanged (SC-1 bidirectional isolation).
- Proved parity: all 4 .py specs green on the fresh out/, the JS suite green on the same out/, all 6 constant arrays byte-identical, then retired the 4 .py files.
- Final full JS suite green in CI order: lint (58 files) → typecheck → unit (33) → build → postbuild (29) → e2e (12).

## Task Commits

1. **Task 1: web.yml + ci.yml paths-ignore** — `aaab69c` (ci)
2. **e2e robustness (dual-theme de-flake + webServer 60s)** — `bdc142a` (fix)
3. **Task 2: retire the 4 Python specs** — `1bb1108` (test)

## Files Created/Modified
- `.github/workflows/web.yml` - path-filtered Web CI, 6 ordered gates (NEW).
- `.github/workflows/ci.yml` - paths-ignore on both triggers (MODIFY, on-block only).
- `web/tests/{home,gfx,seo,accuracy}_spec.py` - DELETED (parity proven).

## Decisions Made
- **Static-only CI verification (D-03):** the repo is unpushed, so web.yml/ci.yml were verified by a pyyaml parse + structural assertions (path filters on both triggers; gate order lint<typecheck<test<build<test:postbuild<test:e2e; ci.yml jobs intact), not a live GitHub run.
- **Retire-after-parity (D-01):** deletion happened only after the .py baseline AND the JS suite were both green on the same out/ and the 6 constant arrays matched byte-for-byte.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule: flake — discovered in the full-suite gate] dual-theme e2e + webServer timeout**
- **Found during:** Task 2 (final full CI-order gate)
- **Issue:** (a) the dual-theme test raced next-themes (manual class toggle) and once read equal backgrounds; (b) `pnpm dlx serve` cold-start exceeded the 15s webServer timeout right after a fresh build.
- **Fix:** drive the theme via next-themes (localStorage bk-theme + reload); raise webServer.timeout to 60s.
- **Files modified:** web/tests/e2e/home.spec.ts, web/playwright.config.ts
- **Verification:** e2e re-run 3x, 12/12 each.
- **Committed in:** `bdc142a` (scoped to 19-02 artifacts)

---

**Total deviations:** 1 (flake hardening of two Wave-2 artifacts, discovered by Wave-3's full-suite gate)
**Impact on plan:** Necessary for a non-flaky merge gate. No scope creep; all planned deliverables present.

## Issues Encountered
- The webServer cold-start timeout (above) — resolved by the 60s bump.
- Advisory: vite-tsconfig-paths Vite-8-native notice persists (plugin retained per plan).

## User Setup Required
None.

## Next Phase Readiness
- QA-01 + QA-02 fully delivered by the JS toolchain; the four Python specs are retired.
- web.yml is statically verified but NOT live-run (repo unpushed — D-03). When the repo is pushed (SITE-03 / Vercel), the first PR touching web/ will exercise web.yml live; Go CI will exercise ci.yml's paths-ignore.
- Phase 19 is the last v1.3.0 web phase. Remaining milestone items: the Phase-20 HF-license human gate (independent), then /gsd-complete-milestone v1.3.0 (Phase 21 also defined).

---
*Phase: 19-test-suite-ci*
*Completed: 2026-06-10*
