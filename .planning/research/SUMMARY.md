# Research Summary: Beekeeper v1.3.0 "Web Presence & Documentation"

**Project:** Beekeeper — real-time safety harness for autonomous coding agents
**Domain:** Greenfield static marketing + documentation website (in-repo `web/`) for an existing Go security CLI
**Researched:** 2026-06-07
**Confidence:** HIGH (four research files cross-verified against official 2026 docs + npm versions)

---

## Executive Summary

v1.3.0 adds a greenfield `web/` subdirectory to the existing Go repo: a fully **static** Next.js 16 site (`output: 'export'`) with three surfaces — a marketing home with a Three.js hero, Fumadocs-powered documentation, and a versioned changelog with per-release supply-chain verification. The stack is locked and version-verified as of 2026-06-07. The build emits a self-contained `out/`; **Cloudflare Pages** is the recommended deploy target (no subpath complexity, global CDN).

The single most consequential rule: **Three.js / R3F must never appear outside the `dynamic(..., { ssr: false })` boundary** in `Hero3D.tsx`. This passes in `next dev` and crashes `next build` (`ReferenceError: window is not defined`). The same "works in dev, breaks in production" class applies to Fumadocs search (needs `force-static` Route Handler **and** `search={{type:'static'}}`), dynamic routes (need `generateStaticParams`), and the `globals.css` import order (`shadcn.css` before `preset.css` — wrong order = invisible text, no build error). **Every phase gate must run `pnpm next build` and inspect `out/` — not just `next dev`.**

The hardest non-technical constraint is **content accuracy** — this is a security tool, so documenting an unenforced or fail-open behavior as working is a *trust* failure, not a UX nit. Several real gaps must be surfaced at point-of-use: `release_age` / `lifecycle_script_allowlist` are parsed but not enforced; Hermes is structurally fail-OPEN; Kilo/Trae leave native tools unguarded; the `--bind 0.0.0.0` gateway has no `allow_remote_gateway` config gate in the shipped binary. A `source_doc:` frontmatter convention + a mandatory content review against `docs/THREAT-MODEL.md` are the process controls.

---

## Key Findings

### Recommended Stack

**Core technologies (versions verified 2026-06-07):**
- **Next.js 16** (App Router, Turbopack default) with `output: 'export'`; **React 19.2** (required by Next 16). Node 22+ (fumadocs-mdx requirement).
- **Tailwind v4** (CSS-first `@theme`, no `tailwind.config.js`) — mandatory for fumadocs-ui v16.
- **shadcn/ui** (CLI-managed) — the design-system base; auto-detects Tailwind v4.
- **Fumadocs v16** (`fumadocs-core`/`fumadocs-ui`/`fumadocs-mdx`) for docs; **Orama static-mode search** (`force-static` route + `search:{type:'static'}`).
- **React-Three-Fiber 9 + @react-three/drei 10 + three** (R3F v10/drei v11 are alpha — do NOT use); `transpilePackages:['three','@react-three/fiber','@react-three/drei']`.
- **motion** (ex-Framer-Motion) for non-3D ambient accents only; **Biome** for lint/format (Next 16 removed `next lint`); **pnpm** workspace; `images.unoptimized:true`.
- **Setup order is load-bearing:** `create-next-app` → `shadcn init` → fumadocs install. **Do NOT add:** Turborepo/Nx (single web package), BurntSushi/toml, ESLint/Prettier, a CMS.

### Expected Features

**Must have (table stakes):**
- *Home:* hero (Three.js hive OR static SVG fallback), dual CTA (`go install` copy chip + docs link), origin story (Nx Console compromise), how-it-works (3 steps), feature cards (shipped features only), 15-harness support matrix (honest live-verification caveat), fail-closed + corroboration callouts, footer.
- *Docs:* getting started, installation (`go install` + releases + cosign verify), configuration (layered config, policy-as-code, sensitive paths, nudge), **security posture + known gaps (ship together)**, CLI reference, integration guides (hooks/MCP gateway), troubleshooting, static search.
- *Changelog:* v1.0.0 / v1.2.0 / v1.3.0 prose, per-release verification commands, **v1.3.0 exit-1→exit-2 breaking-change red callout**.

**Should have (competitive / differentiators):** the Three.js hero (a brand differentiator, not a luxury); a "security changes in this release" callout; honesty block on the home page linking to known-gaps.

**Defer / Out of scope (anti-features):** in-browser interactive demo/playground (impossible to sandbox an OS-hook tool; trust-destroying for a security product), blog, AI docs chatbot (hallucination risk), inflated star counts, auto-generated changelog from commits.

### Architecture Approach

**Major components / boundaries:**
- `Hero3D.tsx` = the `dynamic(ssr:false)` boundary (never imports R3F directly); `HeroCanvasInner.tsx` + `HiveScene.tsx` (`'use client'`) hold ALL three/r3f/drei imports.
- `app/globals.css` order: `tailwindcss` → `fumadocs-ui/css/shadcn.css` → `fumadocs-ui/css/preset.css` → `@source '...fumadocs-ui/dist/**/*.js'` → `@theme {…}`.
- `app/api/search/route.ts`: `export const dynamic='force-static'`; RootProvider `search={{type:'static'}}`.
- `web/content/docs/**` = hand-authored MDX with `source_doc:` frontmatter pointing to Go-side docs (copy-with-citation; never symlink — breaks Windows CI; fumadocs-mdx can't ingest raw MD).
- Marketing `SiteNav`/`SiteFooter` are separate from the Fumadocs `DocsLayout` ancestry.
- pnpm workspace isolates `web/` from the Go module (no hoisting into repo root); CI as a separate path-filtered `web.yml` job; static `out/` is the Playwright/Lighthouse target.

### Critical Pitfalls (each owned by a phase)
1. R3F imported outside `ssr:false` → build crash, invisible in dev. **(Phase: 3D layer)**
2. Fumadocs search 404 — missing `force-static` or wrong RootProvider mode; silent until deploy. **(Phase: Docs pipeline)**
3. CSS import order wrong → invisible/wrong-color text in production, no build error. **(Phase: Design system)**
4. Content accuracy — unenforced/fail-open features documented as working = trust failure. **(Phase: Content authoring review gate)**
5. `generateStaticParams` missing on dynamic routes → 404 on deploy, passes in dev. **(Phase: Docs + Changelog pipelines)**
6. R3F LCP/perf, WebGL disposal leaks, reduced-motion gate (mount, don't just pause), canvas a11y. **(Phase: 3D layer)**

---

## Implications for Roadmap

### Phase 1: Scaffold & Toolchain Isolation
**Rationale:** Everything depends on a building, isolated `web/`. **Delivers:** pnpm workspace, `next.config.ts` (deploy target + `transpilePackages`), `.gitignore` (`.source/`), CI path filters. **Avoids:** pnpm hoisting into the Go repo, GitHub-Pages subpath surprises. Gate: `next build` succeeds on empty app.

### Phase 2: Design System (shadcn + Tailwind v4 + Fumadocs CSS + theme)
**Rationale:** CSS/theme foundation must be correct before any content renders. **Uses:** shadcn/ui, Tailwind v4, fumadocs-ui CSS. **Implements:** `globals.css` import order, `ThemeProvider`+`RootProvider`. Gate: `next build` green, correct contrast in both themes.

### Phase 3: Docs Content Pipeline
**Delivers:** `lib/source.ts`, `[[...slug]]` with `generateStaticParams`, Orama `force-static` search, DocsLayout, first MDX. **Avoids:** search 404, missing static params. Gate: `out/api/search/index.json` non-empty.

### Phase 4: Changelog Pipeline
**Delivers:** `changelog/[version]` route + `generateStaticParams`, stub entries (v1.0.0/v1.2.0/v1.3.0). Parallelizable after Phase 2.

### Phase 5: Marketing Sections (no R3F)
**Delivers:** all Server-Component sections, **static SVG hero**, `ReducedMotionProvider`, `og-image.png`, copyable `go install` chip. Gate: `out/index.html` correct (SVG is the LCP candidate).

### Phase 6: 3D Layer (R3F)
**Rationale:** Purely additive on top of a working site. **Delivers:** `Hero3D` dynamic boundary, hive scene, ambient accents, reduced-motion gate, WebGL error boundary, disposal cleanup. Gates: Lighthouse LCP < 2.5s, R3F chunk budget, no black canvas across nav cycles.

### Phase 7: SEO & Static Assets
**Delivers:** sitemap/robots, finalized `metadataBase`. **Avoids:** `next/og` runtime (use one static OG PNG). Needs domain decision.

### Phase 8: Full Content Authoring + Accuracy Gate
**Delivers:** all docs + changelog prose. **Non-negotiables:** Hermes fail-OPEN + Tier-3 unguarded + `release_age` unenforced + gateway-bind gap surfaced at point-of-use; every `source_doc:` file reviewed against its source before close. Highest trust-risk phase.

### Phase 9: Test Suite & CI
**Delivers:** `web.yml` (path-filtered, separate from `go.yml`), Vitest unit tests, Playwright E2E against `out/`. **Avoids:** Go/web CI cross-triggering.

### Phase Ordering Rationale
Foundation (build + CSS) before content; content surfaces before the additive 3D layer; 3D and SEO can overlap; content authoring and CI last. Hard gate between Phase 2 (CSS green) and Phase 3, and between Phase 5 (marketing green with SVG) and Phase 6 (R3F) — this prevents conflating CSS bugs with 3D bugs.

### Research Flags
- **3D layer:** WebGL disposal correctness + Safari context-limit need Playwright-on-`out/` against real browsers; hive scene geometry needs 1–2 design iterations (spike before estimating).
- **SEO:** domain name is an open question — `metadataBase` cannot be finalized until decided.
- Phases 1–5, 8, 9 are well-documented standard patterns (no execution-time research needed).

---

## Confidence Assessment

**Overall confidence:** HIGH

### Gaps to Address
- **Deploy target + domain name** — recommend Cloudflare Pages; domain TBD (use placeholder until Phase 7).
- **OG images** — single static PNG for v1.3.0 (`next/og` unavailable in static export); per-page OG deferred.
- **drei@10 / three versions** — MEDIUM (npm 403 during research; confirmed via web search) — verify with `pnpm info` on first install.
- **Hive scene visual design** — budget iteration in the 3D phase.

## Sources
### Primary (HIGH confidence)
- Official Next.js 16 docs (static export, App Router), Fumadocs theme + search docs, shadcn Tailwind v4 docs, R3F install docs + GitHub issues, npm registry versions.
- Beekeeper `docs/THREAT-MODEL.md`, `docs/harness-support-matrix.md`, `README.md`, `.planning/PROJECT.md` (content-accuracy ground truth).
### Secondary (MEDIUM confidence)
- Evil Martians dev-tool landing-page study; comparable OSS security-CLI docs (cosign, Trivy); WebSearch version cross-checks (drei/three/motion).
