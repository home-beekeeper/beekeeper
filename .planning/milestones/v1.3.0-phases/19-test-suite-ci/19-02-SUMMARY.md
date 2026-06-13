---
phase: 19-test-suite-ci
plan: 02
subsystem: testing
tags: [playwright, vitest, e2e, seo, fumadocs-search, next-themes, static-export]

requires:
  - phase: 19-test-suite-ci (19-01)
    provides: "@playwright/test@1.57.0, vitest config, test:e2e/test:postbuild scripts"
  - phase: 15-marketing-home
    provides: home page (port source home_spec.py)
  - phase: 16-3d-layer
    provides: hero SVG fallback (port source gfx_spec.py; 3D removed in 9e4a671)
  - phase: 17-seo-static-assets
    provides: sitemap/robots/og (port source seo_spec.py), BASE_URL constant
provides:
  - Playwright config serving the real static out/ on pinned port 4199 (chromium-only)
  - home.spec.ts (port of home_spec.py) + the 4 QA-02 critical-path E2E tests
  - gfx.spec.ts (port of gfx_spec.py) — SVG fallback + canvas regression gate
  - postbuild/seo.test.ts (port of seo_spec.py) running after pnpm build, BASE_URL imported from source
affects: [19-03, test-suite-ci]

tech-stack:
  added: []
  patterns: [playwright webServer serve out --listen 4199, Fumadocs static-search E2E ([data-search-full] + input[placeholder=Search] + result buttons), next-themes storageKey bk-theme, postbuild vitest stage via include+filter]

key-files:
  created:
    - web/playwright.config.ts
    - web/tests/e2e/home.spec.ts
    - web/tests/e2e/gfx.spec.ts
    - web/tests/postbuild/seo.test.ts
  modified:
    - web/vitest.config.mts
    - web/package.json
    - web/.gitignore

key-decisions:
  - "Vitest CLI positional only filters within include and exclude always wins — so tests/postbuild must be in include (not exclude) for test:postbuild to resolve; the default test script is scoped to tests/unit to keep the pre-build run from triggering the postbuild SEO test"
  - "Fumadocs static search E2E: open via [data-search-full], type into input[placeholder=Search], results render as buttons (not links) — verified via a throwaway probe"
  - "next-themes storageKey is bk-theme (read from app/providers.tsx), NOT theme as the PATTERNS snippet showed"
  - "Kept the .py's above-fold check for the Read the docs link (stronger than the plan's visible-only) to preserve parity"

patterns-established:
  - "Single Playwright webServer (serve out --listen 4199) serves all e2e specs via baseURL-relative goto"
  - "Post-build Vitest stage: tests/postbuild in include, run only by test:postbuild after pnpm build"

requirements-completed: [QA-01, QA-02]

duration: ~45min
completed: 2026-06-10
---

# Phase 19 (Wave 2) — 19-02: Playwright E2E + post-build SEO

**Playwright E2E (chromium, webServer serving the real static out/ on port 4199) porting home/gfx specs PLUS the four QA-02 critical paths — docs nav, docs search returns a result and navigates, theme persists across reload, three changelog headings — and a post-build Vitest SEO file-walk importing BASE_URL from source. 12 E2E + 29 postbuild cases green.**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-06-10
- **Tasks:** 3 (+1 infra fix)
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments
- `playwright.config.ts`: chromium-only, webServer `serve out --listen 4199` (port pinned), baseURL 127.0.0.1:4199.
- `home.spec.ts`: faithful home_spec.py port (verbatim HARNESS_NAMES + KNOWN_GAP_MARKERS) + all 4 QA-02 critical paths.
- `gfx.spec.ts`: faithful gfx_spec.py port (FORBIDDEN_SERVER_SYMBOLS verbatim; the /hero-hive.svg reduced-motion fallback is the QA-02 SVG contract).
- `postbuild/seo.test.ts`: seo_spec.py port in the post-build stage (OQ-1), BASE_URL imported from `@/lib/metadata` (eliminates the Python keep-in-sync mirror).
- Full local gate green: `pnpm test` 33/33, `pnpm test:postbuild` 29/29, `pnpm test:e2e` 12/12, `pnpm typecheck` 0, `pnpm lint` 0.

## Task Commits

1. **infra fix: test:postbuild include + gitignore Playwright output** — `69afea8` (fix)
2. **Task 1: Playwright config (chromium, webServer 4199)** — `d63a08b` (test, committed end of session 1)
3. **Task 2: home.spec.ts + 4 QA-02 paths** — `abf0691` (test)
4. **Task 3: gfx.spec.ts + postbuild/seo.test.ts** — `c81fa99` (test)

## Files Created/Modified
- `web/playwright.config.ts` - chromium project + webServer (serve out --listen 4199).
- `web/tests/e2e/home.spec.ts` - home_spec.py port + 4 QA-02 critical paths.
- `web/tests/e2e/gfx.spec.ts` - gfx_spec.py port (SVG fallback + canvas regression gate).
- `web/tests/postbuild/seo.test.ts` - seo_spec.py post-build port (BASE_URL imported).
- `web/vitest.config.mts` - include tests/postbuild; drop it from exclude.
- `web/package.json` - `test` scoped to `vitest run tests/unit`.
- `web/.gitignore` - ignore playwright-results/ (+ test-results/, playwright-report/).

## Decisions Made
- **Vitest include/exclude+filter semantics:** see key-decisions. This was the one real surprise — the plan's config (postbuild in exclude) made `vitest run tests/postbuild` find nothing.
- **Live-DOM facts captured via a throwaway probe** (deleted before commit): search trigger `[data-search-full]`, dialog input `input[placeholder="Search"]`, results render as `<button>`.
- **next-themes storageKey = `bk-theme`** (the PATTERNS snippet's `theme` was wrong; the plan explicitly warned to read the real key).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule: blocking — broken script] test:postbuild could not resolve its tests**
- **Found during:** Task 3 (running `pnpm test:postbuild`)
- **Issue:** vitest.config exclude listed `tests/postbuild`; a CLI positional filter only narrows files in `include`, and exclude wins → "No test files found".
- **Fix:** include tests/postbuild, drop it from exclude, scope `test` to `tests/unit`.
- **Files modified:** web/vitest.config.mts, web/package.json
- **Verification:** `pnpm test` 33 unit; `pnpm test:postbuild` 29.
- **Committed in:** `69afea8`

**2. [Rule: hygiene] gitignore Playwright output**
- **Found during:** Task 2/3 e2e runs (playwright-results/ tripped `pnpm lint`)
- **Issue:** the E2E outputDir is committed/linted otherwise.
- **Fix:** added playwright-results/ (+ test-results/, playwright-report/) to web/.gitignore.
- **Files modified:** web/.gitignore
- **Verification:** `pnpm lint` 0 over 58 files.
- **Committed in:** `69afea8`

**3. [Rule: parity — do not weaken] Read-the-docs above-fold retained**
- **Found during:** Task 2
- **Issue:** the plan/PATTERNS port relaxed the Read-the-docs check to visible-only; the .py asserts it is above the fold.
- **Fix:** kept the stronger above-fold (boundingBox) assertion to preserve parity (D-01).
- **Files modified:** web/tests/e2e/home.spec.ts
- **Verification:** test passes — the link is on the install-chip row (above fold).
- **Committed in:** `abf0691`

---

**Total deviations:** 3 (1 broken-script fix, 1 hygiene, 1 parity-strengthening)
**Impact on plan:** All necessary for a working, parity-preserving suite. No scope creep — every planned spec + QA-02 path is present.

## Issues Encountered
- Advisory: vite-tsconfig-paths notice (Vite 8 native support) persists; plugin retained per plan.

## User Setup Required
None.

## Next Phase Readiness
- Wave 3 (19-03) can author web.yml gating the proven 6-step order (lint→typecheck→test→build→test:postbuild→test:e2e), isolate ci.yml, re-run the 4 .py specs green on the same out/ to prove parity, then retire them.

---
*Phase: 19-test-suite-ci*
*Completed: 2026-06-10*
