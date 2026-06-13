---
phase: 17-seo-static-assets
plan: 03
subsystem: web
tags: [seo, sitemap, robots, static-export, force-static, metadata-route]

requires:
  - phase: 17-seo-static-assets (plan 01)
    provides: BASE_URL constant + seo_spec.py SC-3 assertions
  - phase: 13-docs / 14-changelog
    provides: source.generateParams() route enumeration (docs + changelog)
provides:
  - SC-3 crawler assets — out/sitemap.xml (13 absolute trailing-slash <loc>) + out/robots.txt (allow-all + sitemap ref)
affects: [SITE-03 deploy (sitemap/robots served from out/ root)]

tech-stack:
  added: []
  patterns:
    - "MetadataRoute.Sitemap/Robots + `export const dynamic = 'force-static'` is MANDATORY under output:export (without it the build errors with a generateStaticParams complaint)"
    - "sitemap enumerates dynamic Fumadocs routes via source.generateParams() (the same proven call the catch-all routes use) + a hardcoded /changelog/ landing"
    - "route-handler form emits FLAT out/sitemap.xml + out/robots.txt under trailingSlash (the public/ fallback was NOT needed)"

key-files:
  created:
    - web/app/sitemap.ts
    - web/app/robots.ts

key-decisions:
  - "Used app/sitemap.ts + app/robots.ts route handlers (NOT the static public/ fallback) — both emit flat extensionless files under trailingSlash, so Assumption A1's public/robots.txt fallback was unnecessary"
  - "Changelog landing /changelog/ enumerated as a hardcoded sitemap entry (not relying on a slug=[] serialization, Open Question 3)"

requirements-completed: [SEO-01]  # SC-3 closes SEO-01 jointly with 17-02's SC-1/SC-2

duration: ~10min
completed: 2026-06-09
---

# Phase 17 Plan 03: Crawler Assets (sitemap.xml + robots.txt) Summary

**`pnpm build` now emits `out/sitemap.xml` (13 absolute trailing-slash URLs, all BASE_URL-derived) and `out/robots.txt` (allow-all + absolute sitemap reference) as flat files under `output:'export'` — closing SC-3 and, with 17-02, completing SEO-01.**

## Performance
- **Duration:** ~10 min
- **Completed:** 2026-06-09
- **Tasks:** 2 (both auto)
- **Files created:** 2

## Accomplishments
- **web/app/sitemap.ts** — `MetadataRoute.Sitemap` with `export const dynamic = "force-static"`. Enumerates home `/` + 8 docs sections (via `docsSource.generateParams()`) + `/changelog/` (hardcoded landing) + 3 versions (via `changelogSource.generateParams()`). Every `<loc>` is an absolute `https://beekeeper.vercel.app/...` trailing-slash URL. Emits a flat `out/sitemap.xml` with **13 `<loc>`** entries.
- **web/app/robots.ts** — `MetadataRoute.Robots` with `force-static`: `User-Agent: *` / `Allow: /` / `Sitemap: https://beekeeper.vercel.app/sitemap.xml`. Emits a flat `out/robots.txt` (74 bytes).

## Verification
- `cd web && pnpm build` exit 0 (force-static present in both → no generateStaticParams error). `out/sitemap.xml` and `out/robots.txt` both emit as **FLAT files** (not `…/index.html`) — the A1 public/ fallback was NOT needed.
- `python tests/seo_spec.py` SC-3 PASS: sitemap has 13 `<loc>` (≥13), every `<loc>` starts with BASE_URL (T-17-02 leak guard), robots has `Allow: /` + references `sitemap.xml`. Full suite green (seo + gfx + home).
- No new npm packages.

## Task Commits
1. **Task 1: sitemap.ts** — `a326561` (feat)
2. **Task 2: robots.ts** — `3990224` (feat)

## Deviations from Plan
- None. (Assumption A1's `public/robots.txt` fallback was contingency-only; the route-handler form emitted flat files, so it was not used — recorded as planned.)

## Sitemap URL inventory (13)
`/`, `/docs/{audit-log,cli-reference,configuration,getting-started,installation,integration,security,troubleshooting}/`, `/changelog/`, `/changelog/{v1.0.0,v1.2.0,v1.3.0}/` — all absolute, trailing-slash, BASE_URL-derived.

## User Setup Required
None.

## Next Phase Readiness
- SEO-01 complete (SC-1/SC-2 from 17-02, SC-3 here). Phase 17 done. SITE-03 (live deploy) remains deferred → Vercel; sitemap/robots will be served from the `out/` root.

---
*Phase: 17-seo-static-assets*
*Completed: 2026-06-09*
