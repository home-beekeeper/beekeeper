---
phase: 19-test-suite-ci
plan: 01
subsystem: testing
tags: [vitest, playwright, jsdom, testing-library, vite-tsconfig-paths, biome, pnpm-workspace]

requires:
  - phase: 11-scaffold
    provides: pnpm workspace (repo-root lockfile), allowBuilds/ignoredBuiltDependencies posture, SITE-02 Go-isolation invariant
  - phase: 18-full-content-authoring
    provides: web/tests/accuracy_spec.py (the DOCS-09 gate this ports), web/content/docs/*.mdx (8 flat sections)
provides:
  - Vitest 4.1.8 unit runner + config (jsdom default, @/ alias via vite-tsconfig-paths, globals:false)
  - 5 pre-build unit tests (cn, useReducedMotion, InstallChip, metadata constants, accuracy port)
  - package.json test scripts (typecheck/test/test:watch/test:postbuild/test:e2e)
  - 8 pinned test devDependencies incl. @playwright/test@1.57.0 (consumed by 19-02)
affects: [19-02, 19-03, test-suite-ci]

tech-stack:
  added: [vitest@4.1.8, vite@8.0.16, "@vitejs/plugin-react@6.0.2", jsdom@29.1.1, vite-tsconfig-paths@6.1.1, "@testing-library/react@16.3.2", "@testing-library/dom@10.4.1", "@playwright/test@1.57.0"]
  patterns: [explicit-imports (globals:false), per-file @vitest-environment node docblock, afterEach(cleanup) for DOM tests, @/ alias in tests, verbatim constant copy for .py->ts ports]

key-files:
  created:
    - web/vitest.config.mts
    - web/tests/unit/utils.test.ts
    - web/tests/unit/reduced-motion.test.tsx
    - web/tests/unit/install-chip.test.tsx
    - web/tests/unit/metadata.test.ts
    - web/tests/unit/accuracy.test.ts
  modified:
    - web/package.json
    - pnpm-lock.yaml
    - web/app/sitemap.ts
    - web/components/docs/unenforced-callout.tsx

key-decisions:
  - "environmentMatchGlobs was REMOVED in Vitest 4 — use per-file `// @vitest-environment node` docblocks (the plan's mandated mechanism) for accuracy + metadata"
  - "globals:false breaks @testing-library auto-cleanup — register afterEach(cleanup) explicitly in DOM-rendering test files"
  - "Fixed pre-existing biome format drift (sitemap.ts, unenforced-callout.tsx) because Wave-3's web.yml makes `pnpm lint` the first CI gate"

patterns-established:
  - "Vitest port pattern: copy .py constant arrays VERBATIM so the .py can be retired with zero coverage loss (Wave 3)"
  - "node: import protocol in TS file-I/O tests (biome useNodejsImportProtocol)"

requirements-completed: [QA-01]

duration: ~35min
completed: 2026-06-10
---

# Phase 19 (Wave 1) — 19-01: Vitest toolchain + pre-build unit suite

**Vitest 4.1.8 unit runner wired into the web/ workspace (jsdom + @/ alias via vite-tsconfig-paths, globals:false) with five green pre-build unit tests — cn(), useReducedMotion, InstallChip, metadata constants, and a verbatim port of accuracy_spec.py.**

## Performance

- **Duration:** ~35 min
- **Completed:** 2026-06-10
- **Tasks:** 4 (+1 incidental drift fix)
- **Files modified:** 10 (6 created, 4 modified)

## Accomplishments
- Installed the 8-package JS-native test toolchain pinned EXACT, workspace-isolated, with **no Go-module change** (SITE-02 verified).
- `web/vitest.config.mts`: tsconfigPaths() before react(), jsdom default env, globals:false, unit-only include, e2e/postbuild excluded.
- Five unit tests, 33 cases green (9 component/lib + 24 accuracy = 8 docs sections × 3 ACs).
- accuracy.test.ts ports accuracy_spec.py with the three constant arrays copied byte-for-byte (parity prerequisite for Wave-3 retirement).
- `pnpm test`, `pnpm typecheck`, `pnpm lint` all exit 0.

## Task Commits

1. **Task 1: install pinned test devDependencies** — `0e06fab` (test)
2. **Task 2: vitest.config.mts + 5 package.json scripts** — `a4d1138` (test)
3. **Task 3: pre-build unit tests (cn/useReducedMotion/InstallChip/metadata)** — `8889256` (test)
4. **incidental: fix pre-existing biome format drift (lint gate)** — `0a82172` (style)
5. **Task 4: port accuracy_spec.py -> accuracy.test.ts** — `1abb86d` (test)

## Files Created/Modified
- `web/vitest.config.mts` - Vitest config (jsdom, @/ alias, globals:false, unit include).
- `web/tests/unit/utils.test.ts` - cn() merge / conflict / falsy-filter.
- `web/tests/unit/reduced-motion.test.tsx` - useReducedMotion via renderHook + mocked matchMedia.
- `web/tests/unit/install-chip.test.tsx` - install command text + copy-button aria-label.
- `web/tests/unit/metadata.test.ts` - BASE_URL / SITE_NAME (node env).
- `web/tests/unit/accuracy.test.ts` - DOCS-09 accuracy gate port (AC-1/2/3, verbatim constants).
- `web/package.json` - 8 pinned devDeps + 5 test scripts.
- `pnpm-lock.yaml` - lockfile for the new deps (repo-root workspace lockfile).
- `web/app/sitemap.ts`, `web/components/docs/unenforced-callout.tsx` - biome format-only drift fix.

## Decisions Made
- **environmentMatchGlobs removed in Vitest 4:** the plan/PATTERNS snippet used `environmentMatchGlobs` to route accuracy/metadata to the node env, but the option does not exist in the pinned vitest@4.1.8 (verified absent from its dist). The supported v4 mechanism is the per-file `// @vitest-environment node` docblock, which the plan ALSO mandated for both files — used that, omitted the dead option. All Task-2 acceptance criteria still hold.
- **node: import protocol** in accuracy.test.ts (biome `useNodejsImportProtocol` recommended rule).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule: pinned-version reality] environmentMatchGlobs omitted**
- **Found during:** Task 2 (vitest config)
- **Issue:** `environmentMatchGlobs` (in the plan + PATTERNS) was removed in Vitest 4; including it is dead/risky config.
- **Fix:** Rely on the per-file `// @vitest-environment node` docblock (the plan's mandated secondary mechanism for accuracy + metadata).
- **Files modified:** web/vitest.config.mts
- **Verification:** metadata + accuracy tests run in node env and pass; `pnpm typecheck` 0.
- **Committed in:** `a4d1138`

**2. [Rule: missing-critical] explicit afterEach(cleanup) in DOM tests**
- **Found during:** Task 3 (install-chip render)
- **Issue:** globals:false means @testing-library's automatic afterEach(cleanup) never registers, so the 2nd render accumulated a 2nd InstallChip and getByRole found multiple buttons.
- **Fix:** Added `afterEach(cleanup)` to install-chip.test.tsx and reduced-motion.test.tsx.
- **Files modified:** web/tests/unit/install-chip.test.tsx, web/tests/unit/reduced-motion.test.tsx
- **Verification:** 9/9 component+lib cases pass.
- **Committed in:** `8889256`

**3. [Rule: blocking — gate dependency] pre-existing biome format drift**
- **Found during:** Task 3 verification (`pnpm lint`)
- **Issue:** `pnpm lint` (biome) was never a gate in Phases 17/18, so format/organize-imports drift sat in app/sitemap.ts + components/docs/unenforced-callout.tsx. Wave-3's web.yml makes `pnpm lint` the FIRST CI gate, so it must be green.
- **Fix:** `biome check --write` on the two files (formatting + import order only — no logic change; verified by diff).
- **Files modified:** web/app/sitemap.ts, web/components/docs/unenforced-callout.tsx
- **Verification:** `pnpm lint` exits 0 over all 54 files.
- **Committed in:** `0a82172`

---

**Total deviations:** 3 (1 version-reality, 1 missing-critical, 1 gate-dependency drift fix)
**Impact on plan:** All necessary for correctness / for the QA-01 lint gate to be meaningful. No scope creep — the test surface is exactly as planned.

## Issues Encountered
- Advisory only: vite-tsconfig-paths prints a notice that Vite 8 supports `resolve.tsconfigPaths` natively. The plugin still works and the plan mandates it, so it is retained (a future cleanup could switch to native resolution).

## User Setup Required
None.

## Next Phase Readiness
- Wave 2 (19-02) can consume @playwright/test@1.57.0 (installed; chromium-1200 already cached so no browser download), the `test:e2e`/`test:postbuild` scripts, and the vitest config's e2e/postbuild excludes.
- The four Python specs remain in place; retirement is Wave 3 after JS parity is proven.

---
*Phase: 19-test-suite-ci*
*Completed: 2026-06-10*
