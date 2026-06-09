---
phase: 17
slug: seo-static-assets
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-09
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> SEO-01 success criteria are verified by a pure-Python static-`out/` file-walk
> spec (no browser needed), mirroring the existing `web/tests/gfx_spec.py` /
> `home_spec.py` harness pattern (self-contained scripts, run via `python`).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Python 3.12 standalone spec scripts (project pattern — NOT pytest). New: `web/tests/seo_spec.py` (stdlib `glob`/`re` over `out/**/*.html`; optional Pillow for the 1200×630 OG-dimension check, else PNG IHDR byte-parse to avoid a new dep) |
| **Config file** | none — `seo_spec.py` is self-contained like the existing specs |
| **Quick run command** | `cd web && python tests/seo_spec.py` (assumes a current `out/`) |
| **Full suite command** | `cd web && pnpm build && python tests/seo_spec.py && python tests/gfx_spec.py && python tests/home_spec.py` |
| **Estimated runtime** | seo_spec ~2–4s (file walk); full suite ~40–70s incl. `pnpm build` |

---

## Sampling Rate

- **After every task commit:** `cd web && pnpm build && python tests/seo_spec.py` (SEO assertions read the built `out/`, so a build precedes them)
- **After every plan wave:** Full suite command (seo + gfx + home regression)
- **Before `/gsd-verify-work`:** Full suite green
- **Max feedback latency:** ~70 seconds (one build + three specs)

---

## Per-Task Verification Map

> Task IDs are aligned by the planner; rows below are the requirement→assertion
> contract each task must satisfy (SEO-01 / SC-1..3). Threat refs are planner-defined
> `T-17-*` (this phase has a minimal surface — static public assets only).

| Task (SC) | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-----------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| seo_spec harness (Wave 0) | 01 | 1 | SEO-01 | — | N/A | infra | `python tests/seo_spec.py` (red until impl) | ❌ W0 | ⬜ pending |
| metadataBase + per-page title/desc/canonical (SC-1) | — | — | SEO-01 | — | canonical host pinned to `https://beekeeper.vercel.app`, no open-redirect base | static-HTML assert | `python tests/seo_spec.py` | ❌ W0 | ⬜ pending |
| shared static OG card + og/twitter tags (SC-2) | — | — | SEO-01 | T-17-01 | OG image is a committed static asset (no runtime/Edge fetch, no untrusted gen) | static-HTML + asset assert | `python tests/seo_spec.py` | ❌ W0 | ⬜ pending |
| sitemap.ts + robots.ts (`force-static`) (SC-3) | — | — | SEO-01 | T-17-02 | robots allows all + sitemap lists only public routes; no private paths leaked | built-artifact assert | `python tests/seo_spec.py` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Assertion detail (what `seo_spec.py` proves)

- **SC-1:** every `out/**/*.html` contains a non-empty `<title>`, a `<meta name="description" content="...">`, and a `<link rel="canonical" href="https://beekeeper.vercel.app/...">` whose href is absolute, uses the locked host, and ends with `/` (matches `trailingSlash: true`).
- **SC-2:** every page `<head>` has `<meta property="og:image">`, `og:title`, `og:url`, `og:type`, and `<meta name="twitter:card" content="summary_large_image">`; the referenced OG PNG exists under `out/` and is exactly 1200×630.
- **SC-3:** `out/sitemap.xml` exists, is well-formed, lists every expected public URL (home, all `/docs/*` sections, `/changelog/` + each version, top-level marketing routes) as absolute trailing-slash URLs; `out/robots.txt` contains `User-agent: *` + `Allow: /` + `Sitemap: https://beekeeper.vercel.app/sitemap.xml`.

---

## Wave 0 Requirements

- [ ] `web/tests/seo_spec.py` — SEO-01 SC-1..3 assertions over `out/` (the validation harness; red until the metadata/OG/sitemap tasks land)

*Built first so SC assertions exist before the implementation tasks; mirrors how `gfx_spec.py` was staged in Phase 16.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Social-card preview renders correctly when the URL is pasted into Twitter/LinkedIn (SC-2 visual) | SEO-01 | Real social crawlers fetch the live absolute URL; cannot be exercised headlessly against `out/`, and the site is not deployed (SITE-03 deferred) | After SITE-03 deploy: paste `https://beekeeper.vercel.app/` into the Twitter Card Validator / LinkedIn Post Inspector and confirm the 1200×630 card renders with title/description. Pre-deploy: maintainer visually confirms the committed `opengraph-image.png` looks right. |
| OG card visual quality / brand fit | SEO-01 | Subjective design acceptance of the static PNG | Maintainer reviews the committed 1200×630 PNG before phase completion (planner adds a `checkpoint:human-verify` task). |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (`seo_spec.py`)
- [ ] No watch-mode flags
- [ ] Feedback latency < 70s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
