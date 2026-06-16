# Phase 17: SEO & Static Assets - Context

**Gathered:** 2026-06-09
**Status:** Ready for planning
**Source:** Inline capture (discuss-phase skipped by maintainer choice — Phase 13/14/15 precedent)

<domain>
## Phase Boundary

Phase 17 makes the static-export marketing + docs site discoverable and shareable. In scope (from ROADMAP SC-1..3 / SEO-01):

- **Per-page static metadata** — every page in `out/` ships a `<title>`, `<meta name="description">`, and a canonical `<link rel="canonical">` in its static HTML.
- **Shared OG/social card** — a single 1200×630 OG image referenced by every page; the social-card preview renders correctly when a URL is pasted into Twitter/LinkedIn (Open Graph + Twitter Card tags).
- **Crawler assets** — `out/sitemap.xml` listing all public page URLs, and `out/robots.txt` that allows all crawlers and references the sitemap, both produced as part of `pnpm build`.

OUT of scope: analytics, structured-data/JSON-LD beyond basic OG (unless trivial), per-page custom OG images (one shared card is the requirement), and the live deploy itself (SITE-03 remains deferred → Vercel).
</domain>

<decisions>
## Implementation Decisions (locked)

### D-01 — Canonical base URL = `https://beekeeper.vercel.app`
The site is not deployed yet (SITE-03 deferred; repo `home-beekeeper/beekeeper` unpushed), but the **intended production host is the Vercel project URL `https://beekeeper.vercel.app`** (maintainer decision 2026-06-09). Bake this in NOW as a single source-of-truth constant used by `metadataBase`, every canonical link, absolute OG/Twitter image + `og:url`, and every `sitemap.xml` URL. One constant, one place to change if a custom domain is added later.

### D-02 — `trailingSlash: true` governs URL shape
`next.config.mjs` already sets `trailingSlash: true` (Phases 11/13). Canonical links and sitemap URLs MUST match the emitted route shape (e.g. `https://beekeeper.vercel.app/docs/getting-started/`, home = `https://beekeeper.vercel.app/`). Do not emit canonicals that 301 to a different slash form.

### D-03 — Keep the static export; everything must work at build time
The site stays `output: 'export'` (no architecture change across Phases 11–16). SEO assets must be produced during `pnpm build` with NO server/Edge runtime. This is the key research constraint: `next/og` `ImageResponse` (`opengraph-image.tsx`) runs on the Edge runtime and does NOT execute during `output: 'export'` — so the OG image is almost certainly a **pre-rendered static 1200×630 PNG** committed to the repo (e.g. `app/opengraph-image.png` static convention or `public/`), not generated at build. Research confirms the exact working pattern (see SEO-01 research).

### D-04 — Honesty obligation still applies
Authoring metadata against the future host is fine, but copy/descriptions must not claim the product is live/installable in ways that overclaim (the canonical `go install` path is still not resolvable; repo unpushed). Reuse the honest framing already established in Phases 14/15.

### D-05 — Process: research-first, plan inline
discuss-phase skipped (maintainer choice); research-first chosen (maintainer choice — static-export SEO is novel territory like Phase 16's R3F). No UI-SPEC (ROADMAP "UI hint: no"; design system already locked in Phase 12 — the OG card reuses brand tokens/fonts).

### Claude's Discretion
- OG image **production method** (static PNG asset vs any build-time generation that actually works under `output: 'export'`) and its **visual design** (reuse Phase-12 brand: amber #e3b341 brand, teal #39c5cf, dark `--bg #0a0d12`, Inter/JetBrains; honeycomb/hive motif consistent with the hero).
- `sitemap.xml` / `robots.txt` **mechanism** — Next `app/sitemap.ts` + `app/robots.ts` (`MetadataRoute`) convention vs static `public/sitemap.xml` + `public/robots.txt` — pick whichever reliably emits to `out/` under `output: 'export'` + `trailingSlash` (research to confirm; route handlers may need `export const dynamic = 'force-static'`).
- Per-page `title`/`description`/canonical via the Next Metadata API (`generateMetadata`/static `metadata`) in `layout.tsx` + per-route, including the Fumadocs docs/changelog catch-all routes.
- Verification approach (extend the existing Python+Playwright `web/tests/*_spec.py` harness, or a static `out/` grep spec) for SC-1..3.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` — Phase 17 section (goal, SC-1..3, Depends on Phase 15)
- `.planning/REQUIREMENTS.md` — SEO-01

### web/ stack & prior-phase constraints
- `web/next.config.mjs` — `output: 'export'`, `trailingSlash: true`, `images.unoptimized`, `transpilePackages`, `createMDX` wrapper (do NOT regress these)
- `web/app/layout.tsx` — current root metadata + font setup (where global metadata/metadataBase land)
- `web/app/globals.css` — Phase-12 design tokens (brand colors/fonts for the OG card)
- `web/source.config.ts` + `web/lib/changelog-source.ts` — docs + changelog Fumadocs sources (their catch-all routes also need metadata + sitemap entries)
- `beekeeper-docs.html` (repo root) — authoritative brand/visual source for the OG card

### Honesty / accuracy
- `docs/THREAT-MODEL.md` §8 + the Phase 14/15 honest-framing pattern — don't overclaim deployment/availability in meta descriptions

### Known deploy risk to keep in view
- Phase 13 WR-02: under `trailingSlash` the Orama search index emits at extensionless `out/api/search`; Vercel serving of extensionless JSON is unverified — note any sitemap/robots interaction, but the live-deploy fix is SITE-03 (deferred), not Phase 17.
</canonical_refs>

<specifics>
## Specific Ideas

- Base URL constant: `https://beekeeper.vercel.app` (no trailing slash on the origin).
- OG image spec: 1200×630 PNG, brand-consistent (hive/honeycomb + wordmark + one-line value prop), referenced via `og:image` + `twitter:image` (`summary_large_image`).
- robots.txt: `User-agent: *` `Allow: /` + `Sitemap: https://beekeeper.vercel.app/sitemap.xml`.
- sitemap.xml: enumerate home, `/docs/*` (all 8 sections), `/changelog` + each version, and any top-level marketing routes — matching the `trailingSlash` URL shape.
</specifics>

<deferred>
## Deferred Ideas

- SITE-03 live Vercel deploy + public-URL verification (carried from Phase 15; re-check the Phase-13 WR-02 extensionless `out/api/search` serving on Vercel at that time).
- Per-page bespoke OG images, JSON-LD/structured data beyond basic OG, analytics — out of scope for SEO-01.
</deferred>

---

*Phase: 17-seo-static-assets*
*Context gathered: 2026-06-09 via inline capture (discuss-phase skipped; research-first)*
