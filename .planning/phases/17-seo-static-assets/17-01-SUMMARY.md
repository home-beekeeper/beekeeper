---
phase: 17-seo-static-assets
plan: 01
subsystem: web
tags: [seo, metadata, static-export, validation, python-spec]

requires:
  - phase: 15-marketing-home / 13-docs / 14-changelog
    provides: the out/ routes (home + 8 docs + changelog landing + 3 versions) the spec walks
provides:
  - web/lib/metadata.ts — BASE_URL + SITE_NAME single-source-of-truth constants
  - web/tests/seo_spec.py — SC-1..3 file-walk validation harness (stdlib only)
affects: [17-02 (imports BASE_URL/SITE_NAME), 17-03 (imports BASE_URL), 19-test-ci]

tech-stack:
  added: []
  patterns:
    - "constants-only @/lib module (no imports, no side effects) as the host single-source-of-truth"
    - "pure-Python out/ file-walk spec (gfx_spec.py scaffold minus Playwright/http.server)"

key-files:
  created:
    - web/lib/metadata.ts
    - web/tests/seo_spec.py

key-decisions:
  - "Exported both BASE_URL and SITE_NAME (plan 17-01 contract) so layout.tsx title template + openGraph.siteName derive from one place; PATTERNS showed only BASE_URL"
  - "seo_spec.py mirrors BASE_URL as a Python literal (TS const not importable into Python) with a sync comment — the single coupling point"

requirements-completed: []  # SEO-01 is satisfied jointly by 17-02 + 17-03; this plan lays the foundation + gate

duration: ~10min
completed: 2026-06-09
---

# Phase 17 Plan 01: Foundation (BASE_URL constant + seo_spec.py harness) Summary

**The single-source-of-truth base-URL constant (`web/lib/metadata.ts`) and the stdlib-only SC-1..3 validation harness (`web/tests/seo_spec.py`) are in place; the spec runs RED against the current `out/` (correct Wave-0 state) and will go green once 17-02 + 17-03 land.**

## Performance
- **Duration:** ~10 min
- **Completed:** 2026-06-09
- **Tasks:** 2 (both auto)
- **Files created:** 2

## Accomplishments
- `web/lib/metadata.ts` — `export const BASE_URL = "https://beekeeper.vercel.app"` (locked Vercel host, D-01, verbatim, no trailing slash) + `export const SITE_NAME = "Beekeeper"`. Constants only, no imports/side-effects. The ONLY place the host literal appears.
- `web/tests/seo_spec.py` — pure-Python (`os`/`pathlib`/`re`/`sys`, NO Playwright/http.server) walk of `out/**/index.html` (excluding 404/_not-found) asserting:
  - **SC-1:** non-empty `<title>`, `<meta name="description">`, absolute `<link rel="canonical" href="https://beekeeper.vercel.app/...">` per page;
  - **SC-2:** `out/opengraph-image.png` exists + `og:image` + `twitter:card` per page;
  - **SC-3:** `out/sitemap.xml` exists, ≥13 `<loc>`, EVERY `<loc>` starts with BASE_URL (T-17-02 leak guard), `out/robots.txt` has `Allow: /` + references `sitemap.xml`.

## Verification
- **Task 1:** `node` assertion on `lib/metadata.ts` → `OK` (exit 0); BASE_URL + SITE_NAME present and exact.
- **Task 2:** `python tests/seo_spec.py` against the current `out/` → **exit 1 (RED)**, failing on missing canonical (all 13 content pages), missing `og:image`/`opengraph-image.png`, and missing `sitemap.xml`/`robots.txt` — the correct Wave-0 state. The harness walks `out/` correctly (resolves all home/docs/changelog `index.html`).
- **No npm/pip packages added** (Python stdlib only; no `pnpm install`).

## Task Commits
1. **Task 1: BASE_URL/SITE_NAME constant** — `4a78b9b` (feat)
2. **Task 2: seo_spec.py harness** — `3ca1014` (test)

## Deviations from Plan
- None. (Plan called for both BASE_URL and SITE_NAME; PATTERNS.md only showed BASE_URL — followed the plan contract and exported both.)

## User Setup Required
None.

## Next Phase Readiness
- 17-02 (metadata + OG card) and 17-03 (sitemap + robots) can now import `@/lib/metadata`. Both run in Wave 2 (parallel; share no files). The seo_spec.py gate is wired and red — it goes green once both Wave-2 plans land.

---
*Phase: 17-seo-static-assets*
*Completed: 2026-06-09*
