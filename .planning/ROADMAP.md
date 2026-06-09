# Roadmap: Beekeeper v1.3.0 — Web Presence & Documentation

> Phases 1–5 = parked v1.1.0 Pollen; Phase 10 = shipped v1.3.0 seed — preserved by the orchestrator below / in git history.

## Milestones

- ✅ **v1.0.0 — Comprehensive Standalone Release** — Phases 1–11 (shipped 2026-06-01)
  Full per-phase detail archived in [`milestones/v1.0.0-ROADMAP.md`](milestones/v1.0.0-ROADMAP.md).
  Audit: PASSED — [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).

- 🔄 **v1.1.0 — "Pollen"** — Phases 1–5 (PARKED at maintainer release checkpoint)
  Goal: Own Windows inventory compatibility via a bounded Apache-2.0 Bumblebee derivative so the Windows CI matrix goes fully green.

- ✅ **v1.2.0 — "Runtime Behavioral Hardening"** — Phases 6–9 (shipped 2026-06-04)
  Full per-phase detail archived in [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).

- 🚧 **v1.3.0 — "Web Presence & Documentation"** — Phases 10–19 (in progress)
  Phase 10 (seed: hook-block + self-protection + $VAR hardening) shipped 2026-06-05.
  Phases 11–19: greenfield Next.js site under `web/`.

## Phases

<details>
<summary>✅ v1.0.0 Comprehensive Standalone Release (Phases 1–11) — SHIPPED 2026-06-01</summary>

- [x] Phase 1: Foundation + Hook Handler (6/6 plans)
- [x] Phase 2: Policy Engine + Multi-Source Catalogs (9/9)
- [x] Phase 3: Editor Extension Defense (5/5)
- [x] Phase 4: Integration Surfaces (5/5)
- [x] Phase 5: Linux Sentry (5/5)
- [x] Phase 6: LlamaFirewall + Audit Sinks (5/5)
- [x] Phase 7: Cross-Platform Sentry (5/5)
- [x] Phase 8: TUI Dashboard (9/9)
- [x] Phase 9: Policy as Code + Self-Defense Capstone (5/5)
- [x] Phase 10: Cross-Phase Integration Closure (1/1)
- [x] Phase 11: v1.0.0 PRD-Gap Closure (pre-push) (1/1)

</details>

### v1.1.0 "Pollen" — Windows Inventory Compatibility (PARKED)

- [x] **Phase 1: Fork Setup & Discipline** — tag `v0.1.1-pollen.1` shipped
- [~] **Phase 2: Windows Root Resolver** — code complete; tag deferred to M2 close
- [~] **Phase 3: Windows Path Representation** — code complete & verified 2026-06-02; tag deferred
- [~] **Phase 4: Windows Extension & MCP Coverage** — code complete & verified 2026-06-02; tag deferred
- [ ] **Phase 5: Contribution-Back & Milestone Close** — not started (parked)

<details>
<summary>✅ v1.2.0 Runtime Behavioral Hardening (Phases 6–9) — SHIPPED 2026-06-04</summary>

- [x] Phase 6: Corroboration Severity Hardening (3/3 plans)
- [x] Phase 7: Sensitive-Path Runtime Enforcement (3/3)
- [x] Phase 8: Package-Manager Nudge + Behavioral Test Suite (8/8)
- [x] Phase 9: v1.2.0 Tech-Debt Cleanup (5/5 +1 fix)

Full detail: [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).

</details>

### v1.3.0 "Web Presence & Documentation"

- [x] **Phase 10: Hook-Block Protocol Compliance & Multi-Harness Enforcement** — SHIPPED 2026-06-05 (v1.3.0 seed)
- [x] **Phase 11: Scaffold & Toolchain Isolation** — ✅ Complete & verified 2026-06-07 — pnpm workspace + Next.js 16 static-export app under web/, Go-isolated (Vercel deploy is SITE-03 / Phase 15 — retargeted from Cloudflare 2026-06-08, deferred)
- [x] **Phase 12: Design System** — shadcn/ui + Tailwind v4 + Fumadocs CSS + theme toggle + reduced-motion (completed 2026-06-08)
- [x] **Phase 13: Docs Content Pipeline** — ✅ Complete & verified 2026-06-08 — Fumadocs static-export pipeline (fumadocs-mdx wiring, static Orama search, DocsLayout, 8-section skeleton); verifier 4/4 SCs + FOWT UAT passed
- [x] **Phase 14: Changelog Pipeline** — ✅ Complete & verified 2026-06-08 — second fumadocs-mdx changelog collection + per-version static pages (v1.0.0/v1.2.0/v1.3.0) + cosign/SLSA verify commands + red exit-1→exit-2 breaking-change callout; verifier 7/7 must-haves, 3/3 SCs
- [x] **Phase 15: Marketing Home** — ✅ Complete & verified 2026-06-08 (HOME-01..05) — hero+dual CTA+go-install chip, Nx origin story, 3-step how-it-works, 6 shipped-capability feature cards, 15-harness matrix, honesty/known-gaps callout; verifier 5/5 must-haves; code review 2-crit/3-warn fixed inline; both-theme Playwright proof. **SITE-03 (live deploy) DEFERRED → Vercel.**
- [x] **Phase 16: 3D Layer** — R3F hive hero + ambient accents behind dynamic(ssr:false), perf/a11y gates — **✅ complete & verified 2026-06-09 (3/3 plans; GFX-01..04 green; maintainer UAT approved after 3 hive rounds)**
- [x] **Phase 17: SEO & Static Assets** — sitemap, robots.txt, finalized metadata, OG image — **✅ complete & verified 2026-06-09 (3/3 plans; SEO-01; seo_spec SC-1..3 green; maintainer-approved OG card)**
- [x] **Phase 18: Full Content Authoring** — all 8 docs sections authored + accuracy gate — **✅ complete & verified 2026-06-09 (6/6 plans; DOCS-02..09; accuracy_spec AC-1..3 green; maintainer AC-5 sign-off vs THREAT-MODEL.md)**
- [ ] **Phase 18.1: Docs Theme Restyle** — restyle the Fumadocs docs chrome to the Phase-12 brand (DSYS-05; inserted 2026-06-09 from the docs-styling-polish backlog)
- [ ] **Phase 19: Test Suite & CI** — path-filtered web.yml, Vitest unit tests, Playwright E2E against out/

---

## v1.3.0 Web Presence & Documentation — Phase Details (Phases 11–19)

### Phase 11: Scaffold & Toolchain Isolation

**Goal**: A developer can run and build the `web/` Next.js app locally, and the Node toolchain is fully isolated from the Go module with zero cross-contamination.
**Depends on**: Phase 10 (v1.3.0 seed shipped)
**Requirements**: SITE-01, SITE-02
**Success Criteria** (what must be TRUE):

  1. `pnpm dev` starts in `web/` and serves a page at localhost without errors
  2. `pnpm build` completes and emits a non-empty `web/out/` directory with `index.html`
  3. `pnpm install` in `web/` never modifies `go.mod`, `go.sum`, or any Go-module file
  4. `.next/`, `out/`, `.source/`, and `node_modules/` are all gitignored; no build artifacts appear in `git status`**Plans**: 1 plan (1 wave)
- [x] 11-01-PLAN.md — Wave 1: create-next-app scaffold (pnpm/Tailwind v4/Biome/TS/App Router, no src) + static-export next.config.ts (transpilePackages stub) + pnpm-workspace isolation + root .gitignore/.gitattributes + Go-isolation & gitignore verify + dev-server human-verify (SITE-01, SITE-02)

**UI hint**: yes

### Phase 12: Design System

**Goal**: The site has a unified design system — shadcn/ui + Tailwind v4 + Fumadocs CSS — with a working theme toggle, reduced-motion gate, and WCAG 2.1 AA compliance verified in both themes.
**Depends on**: Phase 11
**Requirements**: DSYS-01, DSYS-02, DSYS-03, DSYS-04
**Success Criteria** (what must be TRUE):

  1. `pnpm build` succeeds with the correct `globals.css` import order (tailwindcss → shadcn.css → preset.css → @source → @theme) and Fumadocs components render in correct colors in both light and dark themes
  2. A visitor can switch between light and dark themes via the toggle; the chosen theme persists across page reloads with no flash-of-wrong-theme
  3. Setting `prefers-reduced-motion: reduce` in browser preferences disables all animation and 3D site-wide (verified by the ReducedMotionProvider)
  4. Both light and dark themes pass WCAG 2.1 AA color-contrast ratios and all interactive elements are reachable by keyboard with visible focus indicators

**Plans**: 3 plans (3 waves)
**Wave 1**

- [x] 12-01-PLAN.md — Wave 1: shadcn foundation — package-legitimacy gate + `shadcn init` (new-york/zinc/CSS-first, registry-clean) + add button/badge/separator/tooltip + install next-themes (DSYS-01)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 12-02-PLAN.md — Wave 2: providers — ReducedMotionProvider + `useReducedMotion()` hook + next-themes ThemeProvider (bk-theme, class strategy, disableTransitionOnChange) with Phase 13 RootProvider marker (DSYS-02, DSYS-03)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 12-03-PLAN.md — Wave 3: integration — canonical Beekeeper globals.css token cascade (overwrites CLI output; Fumadocs commented) + layout.tsx Inter/JetBrains fonts (opsz fallback) + suppressHydrationWarning + skip link + Providers wrap + manual smoke verify (DSYS-01–04)

**UI hint**: yes

### Phase 13: Docs Content Pipeline

**Goal**: A visitor can browse a Fumadocs-powered documentation site with sidebar navigation, table of contents, and working static search — all served from pre-built static files with no server runtime.
**Depends on**: Phase 12
**Requirements**: DOCS-01
**Success Criteria** (what must be TRUE):

  1. `pnpm build` emits `out/docs/` with HTML pages for every docs route; `out/api/search/index.json` is non-empty (Orama static index generated)
  2. The docs sidebar lists all top-level sections (getting-started, installation, configuration, integration, security, cli-reference, audit-log, troubleshooting) in the correct order
  3. The table of contents panel renders correctly for a docs page with multiple headings
  4. The search dialog opens, a query returns at least one result, and clicking a result navigates to the correct page

**Plans**: 3 plans (3 waves)
**Wave 1**

- [x] 13-01-PLAN.md — Wave 0/1 toolchain: install pinned fumadocs deps (ui/core/mdx + @types/mdx) + postinstall codegen; rename next.config.ts -> next.config.mjs with createMDX (preserve output:export/trailingSlash/images.unoptimized/transpilePackages); source.config.ts; tsconfig collections/* alias; pnpm build green + .source/ generated (DOCS-01)

**Wave 2** *(blocked on Wave 1)*

- [x] 13-02-PLAN.md — Wave 2 pipeline body: lib/source.ts loader (toFumadocsSource); docs DocsLayout + [[...slug]] page (generateStaticParams, Next-16 async params); static-search route (createFromSource/staticGET -> out/api/search/index.json); RootProvider wired (theme disabled, static search options); globals.css Section-3 @source glob restore (DOCS-01)

**Wave 3** *(blocked on Wave 2)*

- [x] 13-03-PLAN.md — Wave 3 seed skeleton (phase-complete gate): top-level + 8 section meta.json (forced sidebar order) + 8 seed index.mdx (getting-started = 4-heading TOC page); pnpm build emits all 8 out/docs/<section>/index.html + non-empty out/api/search/index.json; SC-2/3/4 Playwright-Python pass (DOCS-01)

**UI hint**: yes

### Phase 14: Changelog Pipeline

**Goal**: A visitor can read versioned release notes (v1.0.0, v1.2.0, v1.3.0) with per-release download and verification guidance, and the v1.3.0 entry displays a red breaking-change callout.
**Depends on**: Phase 12 (Design System) — reuses the Phase 13 docs-pipeline pattern (Phase 13 complete) as the implementation analog
**Requirements**: CHG-01, CHG-02, CHG-03
**Success Criteria** (what must be TRUE):

  1. `pnpm build` emits `out/changelog/v1.0.0/`, `out/changelog/v1.2.0/`, and `out/changelog/v1.3.0/` as separate static HTML pages
  2. Each changelog page includes copyable cosign/SLSA verification commands and a link to the corresponding GitHub Release
  3. The v1.3.0 changelog page displays a prominently styled red callout for the exit-1 to exit-2 breaking change with a migration note

**Plans**: 2 plans (2 waves)
**Wave 1**

- [x] 14-01-PLAN.md — Wave 1: changelog pipeline + v1.0.0/v1.2.0 — second fumadocs-mdx `changelog` collection + lib/changelog-source.ts loader (baseUrl /changelog) + app/changelog/[[...slug]] catch-all route + DocsLayout; VerifyCommands (cosign+SLSA, capital-B Bantuson per release-runbook Pitfall 4) + ReleaseLinks (canonical github.com/bantuson/beekeeper release tag) + BreakingChangeCallout (red, dual-theme tokens) components + MDX map; accurate v1.0.0 + v1.2.0 release notes (from MILESTONES.md); docs-nav Changelog link; build emits the two static pages (CHG-01, CHG-02)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 14-02-PLAN.md — Wave 2: v1.3.0 + phase-complete gate — accurate v1.3.0 release notes + prominent red BreakingChangeCallout (exit-1→exit-2 hook protocol change + accurate migration note: upgrade, re-run `hooks install --hook <harness>`, restart session; only Claude Code live-verified) + VerifyCommands/ReleaseLinks; pnpm build emits all three out/changelog/vX.Y.Z/index.html; SC-1/2/3 validated (grep + Playwright-on-static-export, red callout in both themes) (CHG-01, CHG-03)

**UI hint**: yes

### Phase 15: Marketing Home

**Goal**: A visitor sees a complete marketing home page — hero with dual CTA, origin story, how-it-works, feature highlights for shipped capabilities, the 15-harness support matrix, and an honesty callout — all as server-rendered static content with a static SVG hero (no 3D yet).
**Depends on**: Phase 12
**Requirements**: HOME-01, HOME-02, HOME-03, HOME-04, HOME-05 *(SITE-03 DEFERRED — deploy retargeted Cloudflare→Vercel, pending repo push / Vercel setup; page build-verified locally this phase)*
**Success Criteria** (what must be TRUE):

  1. The home page hero displays the headline, subhead, a copyable `go install` command chip, and a "Read the docs" link — all visible above the fold on a 1280px viewport
  2. The page explains the Nx Console compromise origin story and presents a 3-step how-it-works flow
  3. Feature cards cover only the six shipped capabilities (corroboration engine, fail-closed hooks, editor-extension defense, Sentry, LlamaFirewall, policy-as-code) with no aspirational claims
  4. The harness support matrix shows all 15 harnesses with honest tier labels and live-verification caveats, linking to the integration docs
  5. An honesty callout is visible on the home page linking to the security-posture / known-gaps documentation
  6. *(DEFERRED)* The static site is deployed to **Vercel** and reachable at a public URL — carried forward (SITE-03); Phase 15 verifies the page is built (`pnpm build` → `out/`), not a live URL

**Plans**: 3 plans (3 waves)
**Wave 1**

- [x] 15-01-PLAN.md — Wave 1: marketing chrome + hero — Section/SectionHead primitives, sticky SiteHeader (Docs/Changelog nav), SiteFooter, the go-install copy chip (InstallChip, client), Hero (headline + subhead + dual CTA), replace scaffold page.tsx; build emits out/index.html (HOME-01)

**Wave 2** *(blocked on Wave 1)*

- [x] 15-02-PLAN.md — Wave 2: content body — OriginStory (Nx Console origin + documented-2026-campaigns threat list), HowItWorks (reactive/proactive two-layer + 3-step quickstart), FeatureCards reconciled to the six SHIPPED capabilities; wire into page.tsx (HOME-02, HOME-03)

**Wave 3** *(blocked on Wave 2)*

- [x] 15-03-PLAN.md — Wave 3: added sections + gate — HarnessMatrix (all 15 harnesses, honest tiers/caveats → /docs/integration), HonestyCallout (4 known gaps → /docs/security), final page assembly + build gate + both-theme/above-the-fold Playwright proof (HOME-04, HOME-05)

**UI hint**: yes

### Phase 16: 3D Layer

**Goal**: The home hero features an interactive Three.js hive centerpiece and ambient 3D accents that load client-only behind a dynamic(ssr:false) boundary, meet the Lighthouse LCP < 2.5s budget, and fall back gracefully to a static SVG under reduced-motion, low-power, or no-WebGL conditions.
**Depends on**: Phase 15
**Requirements**: GFX-01, GFX-02, GFX-03, GFX-04
**Success Criteria** (what must be TRUE):

  1. `pnpm build` succeeds with no R3F/three/drei imports visible in server-rendered output; the R3F chunk loads lazily in the browser (verified: `<canvas>` element appears only after JS hydration)
  2. With `prefers-reduced-motion: reduce` or WebGL unavailable, the hero renders the static SVG fallback and no canvas is mounted — the page is fully usable
  3. The `<canvas>` element has `aria-hidden="true"` and a visible sr-only description; keyboard navigation is unaffected by the 3D layer
  4. Lighthouse audit against `out/index.html` shows LCP < 2.5s (SVG as LCP candidate) and no WebGL context leaks across navigation cycles

**Plans**: 3 plans (3 waves)
**Wave 1**

- [x] 16-01-PLAN.md — Wave 1: foundation — package-legitimacy gate + install pinned three@0.184.0/@react-three/fiber@9.6.1/drei@10.7.7/@types/three@0.184.1 (workspace-isolated) + author static public/hero-hive.svg LCP fallback + create tests/gfx_spec.py harness (GFX-01 server-clean grep + canvas-post-hydration; GFX-02/03/04 PENDING slots) (GFX-01, GFX-03)

**Wave 2** *(blocked on Wave 1)*

- [x] 16-02-PLAN.md — Wave 2: 3D layer — hero-canvas-wrapper.tsx (use-client dynamic ssr:false boundary) + hero-canvas.tsx (useCanMount3D capability gate: reduced-motion/WebGL/saveData; single aria-hidden Canvas frameloop=demand; HiveMesh/HexCell theme-aware teal/amber; sr-only sibling) + hero.tsx modify (server-rendered SVG LCP img + wrapper slot); gfx_spec.py GFX-02/03 assertions (GFX-01, GFX-02, GFX-03)

**Wave 3** *(blocked on Wave 2)*

- [x] 16-03-PLAN.md — Wave 3: perf + a11y gate — gfx_spec.py GFX-04 (Playwright LCP/FCP proxy 560ms < 2500ms + no-context-leak across nav) + lighthouse SKIPPED (proxy is the sole gate, RESEARCH OQ4) + maintainer UAT approved after 3 hive redesign rounds (depth ring + center→cone light-streaks; tier-ordered cards; no command scrollbars) (GFX-04)

**UI hint**: yes

### Phase 17: SEO & Static Assets

**Goal**: Every page emits correct static metadata, a shared OG/social card image, and the site produces `sitemap.xml` and `robots.txt` as part of the static build.
**Depends on**: Phase 15
**Requirements**: SEO-01
**Success Criteria** (what must be TRUE):

  1. Every page in `out/` has a `<title>`, `<meta name="description">`, and canonical `<link rel="canonical">` in its static HTML
  2. Every page references the shared OG image (1200×630 PNG); social-card preview renders correctly when the URL is pasted into Twitter/LinkedIn
  3. `out/sitemap.xml` lists all public page URLs and `out/robots.txt` allows all crawlers and references the sitemap

**Plans**: 3 plans (2 waves) — planned & verified 2026-06-09 (plan-checker PASSED, 12/12 dims; SEO-01 covered)
**Wave 1**

- [x] 17-01-PLAN.md — Wave 1 foundation: BASE_URL constant (web/lib/metadata.ts = https://beekeeper.vercel.app) + seo_spec.py SC-1..3 file-walk harness (Wave 0 deliverable) (SEO-01)

**Wave 2** *(blocked on Wave 1; 17-02 and 17-03 run in parallel — they share no files, both consume BASE_URL read-only)*

- [x] 17-02-PLAN.md — Wave 2: per-page metadata (SC-1) + shared OG card (SC-2) — metadataBase/title-template/canonical on layout+home+docs+changelog routes + static app/opengraph-image.png (1200×630, no Edge ImageResponse); maintainer approved the flat hero-hive card after 3 rounds (SEO-01)
- [x] 17-03-PLAN.md — Wave 2: crawler assets (SC-3) — app/sitemap.ts + app/robots.ts with `export const dynamic = 'force-static'`; sitemap enumerates routes via source.generateParams() + hardcoded /changelog/ (13 trailing-slash `<loc>` URLs) (SEO-01)

**Cross-cutting constraints:** the `web/lib/metadata.ts` BASE_URL constant (17-01) is the single source of truth consumed by both Wave-2 plans; `python tests/seo_spec.py` green on the production `out/` build is the shared gate for all three plans.

**UI hint**: no

### Phase 18: Full Content Authoring

**Goal**: A developer can follow every documentation path — quickstart to CLI reference to security posture to integration guides to troubleshooting — with content that is accurate to the shipped binary: every security claim cites its source, every unenforced feature is explicitly labeled, and known gaps are presented alongside the security posture.
**Depends on**: Phase 13, Phase 14, Phase 15
**Requirements**: DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-06, DOCS-07, DOCS-08, DOCS-09
**Success Criteria** (what must be TRUE):

  1. A new user can follow the Getting Started quickstart from zero to a working `beekeeper check` invocation, with no steps referencing unshipped behavior
  2. The installation docs cover `go install`, binary download from GitHub Releases, and cosign + SLSA verification with copyable commands
  3. The security posture and known-gaps pages are co-located and explicitly document: Hermes fail-open, Tier-3 unguarded tools, `release_age`/`lifecycle_script_allowlist` unenforced in v1.3.0, and the `--bind 0.0.0.0` gateway caveat
  4. Every integration guide for Tier-1/2/3 harnesses includes honest caveats at point-of-use; the CLI reference covers all subcommands and flags
  5. Every MDX file that derives content from a Go-side doc has a `source_doc:` frontmatter field, and all content has been reviewed against `docs/THREAT-MODEL.md` before publish

**Plans**: 6 plans (3 waves) — planned 2026-06-09 (research-first; pure-content scope per D-01; accuracy gate = `web/tests/accuracy_spec.py` AC-1..3 + human THREAT-MODEL.md review)
**Wave 1**

- [ ] 18-01-PLAN.md — Wave 1 infra: `web/tests/accuracy_spec.py` (AC-1 source_doc+path-exists / AC-2 unenforced labels / AC-3 no phantom commands; RED until content lands) + `web/components/docs/unenforced-callout.tsx` (amber dual-theme `UnenforcedCallout`) registered in `web/mdx-components.tsx` (DOCS-09)

**Wave 2** *(blocked on 18-01; 18-02/03/04/05 share no files -> run in parallel)*

- [ ] 18-02-PLAN.md — Wave 2: getting-started (zero->real `beekeeper check`, `hooks install --target`, Claude-Code-only caveat) + installation (go install + Releases + exact cosign/SLSA/SBOM from THREAT-MODEL section 7 + state-dir) (DOCS-02, DOCS-03, DOCS-09)
- [ ] 18-03-PLAN.md — Wave 2: configuration (layered config, policy-as-code, sensitive paths, soft/hard nudge + `require_hardened` blocking accurate to real validator, `<UnenforcedCallout>` for release_age/lifecycle) + cli-reference (exhaustive subcommand tree, no phantom/internal commands) (DOCS-04, DOCS-07, DOCS-09)
- [ ] 18-04-PLAN.md — Wave 2: security (posture + co-located known gaps — Hermes/Tier-3/unenforced/--bind) + integration (Tier1->2->3->gateway with point-of-use caveats, `--target` flag) (DOCS-05, DOCS-06, DOCS-09)
- [ ] 18-05-PLAN.md — Wave 2: troubleshooting (real commands only — `diag`/`version`; phantom `hooks status`/`catalogs rebuild`/`status` removed) + audit-log (single `beekeeper.ndjson`, corrected rotation claim, field-scoped redaction caveat) (DOCS-08, DOCS-09)

**Wave 3** *(blocked on Wave 2)*

- [ ] 18-06-PLAN.md — Wave 3 gate: full suite green (pnpm build + accuracy_spec + seo/home/gfx regression) + blocking maintainer AC-5 accuracy review against `docs/THREAT-MODEL.md` + served-docs render check (DOCS-09)

**Cross-cutting constraints:** `web/tests/accuracy_spec.py` (18-01) is the shared DOCS-09 gate consumed by every Wave-2 plan; `<UnenforcedCallout>` (18-01) must be registered before 18-03/18-04 use it; `pnpm build` green is the shared wave-boundary gate; execution runs INLINE on main (subagents lack node/pnpm).

**UI hint**: yes

### Phase 18.1: Docs Theme Restyle

**Goal**: The Fumadocs documentation section adopts the Beekeeper design system and matches the marketing home's visual quality — sidebar/nav, typography scale, code-block treatment, link/heading colors, admonitions, and spacing all reflect the Phase-12 brand (dark-first GitHub-dark palette, amber `#e3b341` brand / teal `#39c5cf` interactive, Inter + JetBrains Mono, 1180px/60px chrome) — while remaining dual-theme, reduced-motion, and WCAG-AA safe. Content is NOT changed (Phase 18 owns content).
**Depends on**: Phase 12 (design system), Phase 13 (docs pipeline), Phase 18 (content)
**Requirements**: DSYS-05
**Inserted**: 2026-06-09 (from the `.planning/todos/pending/docs-styling-polish.md` backlog item — maintainer flagged the stock Fumadocs theme as "looks old/basic" during the Phase-17 review)
**Success Criteria** (what must be TRUE):

  1. The docs `DocsLayout` chrome (sidebar, top nav, TOC, search trigger) is restyled to the Phase-12 brand and is visually consistent with the marketing home
  2. Code blocks, links, headings, and admonitions/callouts in docs use the brand tokens via raw theme-switched `var(--*)` tokens (NOT dark-only `--color-bk-*`)
  3. The restyle is dual-theme correct (light + dark, no flash-of-wrong-theme) and honors `prefers-reduced-motion`; WCAG 2.1 AA contrast holds in both themes (Playwright-proven, per the Phase-12 method)
  4. `pnpm build` stays green and the existing docs content, sidebar nav order, TOC, and Orama static search all still work (DOCS-01 regression); no content changes

**Plans**: TBD
**UI hint**: yes

### Phase 19: Test Suite & CI

**Goal**: A path-filtered web CI job gates merges with lint, type-check, unit tests, a static build, and E2E tests against the `out/` directory — completely isolated from Go CI.
**Depends on**: Phase 16, Phase 17, Phase 18
**Requirements**: QA-01, QA-02
**Success Criteria** (what must be TRUE):

  1. The `.github/workflows/web.yml` job triggers only on `web/**` and `pnpm-workspace.yaml` changes, never on Go file changes; Go CI never triggers on web changes
  2. The CI job runs Biome lint/format check, `tsc --noEmit`, Vitest unit tests, `pnpm build`, and Playwright E2E against `out/` — all as required gates before merge
  3. Playwright E2E tests verify: home page renders with SVG hero fallback, docs navigation and search return results, theme toggle persists across reload, all three changelog version pages build and render their headings

**Plans**: TBD
**UI hint**: no

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Foundation + Hook Handler | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 2. Policy Engine + Multi-Source Catalogs | v1.0.0 | 4/4 | Complete | 2026-05-27 |
| 3. Editor Extension Defense | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 4. Integration Surfaces | v1.0.0 | 3/3 | Complete | 2026-05-27 |
| 5. Linux Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 6. LlamaFirewall + Audit Sinks | v1.0.0 | 5/5 | Complete | 2026-06-03 |
| 7. Cross-Platform Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 8. TUI Dashboard | v1.0.0 | 9/9 | Complete | 2026-05-29 |
| 9. Policy as Code + Self-Defense Capstone | v1.0.0 | 5/5 | Complete | 2026-05-29 |
| 10. Cross-Phase Integration Closure | v1.0.0 | 6/6 | Complete | 2026-06-05 |
| 11. v1.0.0 PRD-Gap Closure (pre-push) | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| **1. Fork Setup & Discipline** | **v1.1.0** | **5/5** | **Complete** | **2026-05-26** |
| **2. Windows Root Resolver** | **v1.1.0** | **3/4** | **Code complete — release deferred to M2 close** | **—** |
| **3. Windows Path Representation** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred** | **2026-06-02** |
| **4. Windows Extension & MCP Coverage** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred** | **2026-06-02** |
| **5. Contribution-Back & Milestone Close** | **v1.1.0** | **0/5** | **PARKED** | **—** |
| **6. Corroboration Severity Hardening** | **v1.2.0** | **3/3** | **Complete** | **2026-06-03** |
| **7. Sensitive-Path Runtime Enforcement** | **v1.2.0** | **3/3** | **Complete** | **2026-06-04** |
| **8. Package-Manager Nudge + Behavioral Test Suite** | **v1.2.0** | **8/8** | **Complete** | **2026-06-04** |
| **9. v1.2.0 Tech-Debt Cleanup** | **v1.2.0** | **5/5** | **Complete** | **2026-06-04** |
| **10. Hook-Block Protocol Compliance** | **v1.3.0** | **6/6** | **Complete (seed)** | **2026-06-05** |
| **11. Scaffold & Toolchain Isolation** | **v1.3.0** | **1/1** | **Complete** | **2026-06-07** |
| **12. Design System** | **v1.3.0** | **3/3** | **Complete** | **2026-06-08** |
| **13. Docs Content Pipeline** | **v1.3.0** | **3/3** | **Complete** | **2026-06-08** |
| **14. Changelog Pipeline** | **v1.3.0** | **2/2** | **Complete** | **2026-06-08** |
| **15. Marketing Home** | **v1.3.0** | **3/3** | **Complete** (SITE-03 deferred→Vercel) | **2026-06-08** |
| **16. 3D Layer** | **v1.3.0** | **3/3** | **Complete** | **2026-06-09** |
| **17. SEO & Static Assets** | **v1.3.0** | **3/3** | **Complete** | **2026-06-09** |
| **18. Full Content Authoring** | **v1.3.0** | **6/6** | **Complete** | **2026-06-09** |
| **18.1 Docs Theme Restyle** | **v1.3.0** | **0/TBD** | **Inserted — not started** | **—** |
| **19. Test Suite & CI** | **v1.3.0** | **0/TBD** | **Not started** | **—** |
