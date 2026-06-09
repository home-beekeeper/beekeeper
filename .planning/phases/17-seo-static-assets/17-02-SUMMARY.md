---
phase: 17-seo-static-assets
plan: 02
subsystem: web
tags: [seo, metadata, opengraph, twitter-card, static-export, og-image]

requires:
  - phase: 17-seo-static-assets (plan 01)
    provides: BASE_URL/SITE_NAME constants + seo_spec.py harness
provides:
  - Per-page static metadata (SC-1): title template + description + absolute trailing-slash canonical on every out/ page
  - Shared OG/social card (SC-2): committed 1200x630 web/app/opengraph-image.png + og:image/twitter:card site-wide
affects: [17-03 (shares the seo_spec gate), 18-content, SITE-03 social-preview UAT]

tech-stack:
  added: []
  patterns:
    - "metadataBase: new URL(BASE_URL) once in root layout → relative canonical/og resolve absolute"
    - "static app/opengraph-image.png file convention auto-injects og:image/dims + twitter:image (NOT next/og ImageResponse — Edge, forbidden under output:export)"
    - "nested catch-all routes set ONLY alternates.canonical — never openGraph — so the inherited file-convention og:image is not stripped by shallow openGraph replacement"
    - "OG card authored as HTML, rendered to PNG via the installed Playwright (no new dep)"

key-files:
  modified:
    - web/app/layout.tsx
    - web/app/page.tsx
    - web/app/docs/[[...slug]]/page.tsx
    - web/app/changelog/[[...slug]]/page.tsx
  created:
    - web/app/opengraph-image.png
    - web/app/opengraph-image.alt.txt

key-decisions:
  - "Nested docs/changelog routes return ONLY alternates.canonical (dropped openGraph.url): a nested route's openGraph replaces the inherited one wholesale, which stripped the root file-convention og:image from every doc page (SC-2 failure). Deviation from the plan's openGraph.url instruction — SC-2 (og:image everywhere) outranks per-route og:url, which is not an SC."
  - "OG card = the flat hero-hive.svg honeycomb design (maintainer chose the static loading-state render over an animated-style hive), authored as HTML and rendered to a 1200x630 PNG via Playwright"
  - "Home title resolves to single 'Beekeeper' (layout title.default, no template) — page.tsx sets no title (Pitfall 3)"

requirements-completed: []  # SEO-01 completed jointly with 17-03

duration: ~UAT-paced (multiple OG-card iterations)
completed: 2026-06-09
---

# Phase 17 Plan 02: Per-page Metadata + Shared OG Card Summary

**Every `out/` page now emits a title, description, and absolute trailing-slash canonical (SC-1), plus a site-wide `og:image`/`twitter:card` referencing a committed 1200x630 OG card (SC-2) — all build-time, static-export-safe, no Edge runtime.**

## Performance
- **Duration:** UAT-paced (3 OG-card design iterations: animated-style → legibility fix → flat hero hive)
- **Completed:** 2026-06-09
- **Tasks:** 3 (2 auto + 1 blocking human-verify checkpoint)
- **Files:** 4 modified + 2 created

## Accomplishments
- **layout.tsx** — `metadataBase: new URL(BASE_URL)`, title template `%s | Beekeeper` (default `Beekeeper`), shared `openGraph` (siteName/type/locale) + `twitter` (`summary_large_image`). NO `openGraph.images`/`twitter.images` (the file convention owns og:image — Pitfall 5). Fonts/Providers/skip-link untouched.
- **page.tsx** — home `description` + `alternates.canonical "/"` + `openGraph.url "/"`; no `title` (avoids "Beekeeper | Beekeeper", Pitfall 3).
- **docs + changelog [[...slug]] routes** — extended `generateMetadata` with `alternates.canonical` (trailing-slash, e.g. `/docs/getting-started/`, `/changelog/v1.2.0/`). Deliberately NO `openGraph` override (see Decisions).
- **OG card** — `web/app/opengraph-image.png` (1200x630, IHDR-verified) + `web/app/opengraph-image.alt.txt`. Brand-consistent: dark bg, "Beekeeper" wordmark, amber/teal tagline, the flat hero-hive honeycomb (maintainer-chosen), mono sub-line. Authored as HTML, rendered via the installed Playwright (no new dep).

## Verification
- `cd web && pnpm build && python tests/seo_spec.py` → SC-1 + SC-2 PASS (home `<title>Beekeeper</title>`, absolute trailing-slash canonical, `og:image`+`twitter:card` on all 13 content pages; `out/opengraph-image.png` present). Final full suite green (seo + gfx + home).
- Maintainer visually approved the OG card on the live dev server (round 3 — the flat hero hive).
- No new npm/pip packages.

## Task Commits
1. **Task 1: layout + home metadata** — `0aa1c6f` (feat)
2. **Task 2: docs + changelog canonicals** — `9070269` (feat)
3. **Task 3: OG card** — `3fe71e6` (initial) → `3b158d3` (legible wordmark) → `a2cb134` (flat hero hive, approved)

## Deviations from Plan
- **[Rule 3 — Blocking] Dropped `openGraph.url` from the nested docs/changelog routes.** Plan Task 2 said add `alternates.canonical` AND `openGraph.url`. But under Next's shallow metadata merge, a nested route's `openGraph` REPLACES the inherited one wholesale, stripping the root `app/opengraph-image.png` file-convention `og:image` from every doc/changelog page → SC-2 (og:image site-wide) failed on 12 pages. Fixed by returning ONLY `alternates.canonical` (no openGraph) so og:image + twitter inherit from the root layout. SC-2 outranks a per-route og:url (not an SC). Caught by `seo_spec.py`.
- **OG card design** — went through 3 maintainer iterations (animated-style hive rejected → legibility fix → flat hero-hive.svg design approved). Final card uses the static loading-state honeycomb the maintainer preferred.

## Issues Encountered
- Next **dev server caches the generated opengraph-image** — a changed `app/opengraph-image.png` was not reflected until the dev server + `.next/` cache were restarted/cleared (relevant for live OG-card review).

## User Setup Required
None. (Post-SITE-03: validate the live social-card preview in the Twitter Card Validator / LinkedIn Post Inspector — deferred, not a Phase-17 gate.)

## Next Phase Readiness
- SC-1 + SC-2 complete. Combined with 17-03 (SC-3 sitemap/robots) the full `seo_spec.py` is green.

---
*Phase: 17-seo-static-assets*
*Completed: 2026-06-09*
