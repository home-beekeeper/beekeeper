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

- 🚧 **v1.3.0 — "Web Presence & Documentation"** — Phases 10–21 (in progress)
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
- [x] **Phase 18.1: Docs Theme Restyle** — Fumadocs chrome brand-aligned (white border killed, teal/amber accents, sidebar duplication fixed) — **✅ complete & maintainer-approved 2026-06-09 (quick task; DSYS-05; command-card copy split deferred to backlog)**
- [x] **Phase 19: Test Suite & CI** — path-filtered web.yml, Vitest unit tests, Playwright E2E against out/ ✅ Complete & verified 2026-06-10
- [ ] **Phase 20: Runtime Hardening II (Tiers 1–3)** — Tier 1 background catalog sync + TUI scheduler · Tier 2 LlamaFirewall opt-in actually works · Tier 3 Sentry coverage (006/007/008 + file-write) + honesty edits (researched; `20-RESEARCH.md` + `20-PLAN.md`)
- [ ] **Phase 21: Full-System Validation & CI Calibration** — Go-core release gate: close every local (Tier-A) test-coverage gap to 100% (gate-enforced), cross-platform CI matrix (Tier-B: 2 Linux kernels + macOS + Windows; eBPF/eslogger/ETW/peer-cred/-race), 17-harness installer+deny-contract conformance, fuzz gate, Claude Code live e2e, and a `docs/validation-register.md` manual register for the 16 live harnesses + gated-model e2e (Tier-C)

---

## v1.3.0 Web Presence & Documentation — Phase Details (Phases 11–21)

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

  1. [x] The `.github/workflows/web.yml` job triggers only on `web/**` and `pnpm-workspace.yaml` changes, never on Go file changes; Go CI never triggers on web changes — ✅ web.yml `paths:` include + ci.yml mirror `paths-ignore` on both triggers (SC-1, static-verified)
  2. [x] The CI job runs Biome lint/format check, `tsc --noEmit`, Vitest unit tests, `pnpm build`, and Playwright E2E against `out/` — all as required gates before merge — ✅ 6 ordered gates lint→typecheck→test→build→test:postbuild→test:e2e
  3. [x] Playwright E2E tests verify: home page renders with SVG hero fallback, docs navigation and search return results, theme toggle persists across reload, all three changelog version pages build and render their headings — ✅ all four QA-02 paths green (12/12 e2e)

**Plans**: 3 plans (3 waves) — ✅ COMPLETE & verified 2026-06-10 (executed INLINE on main per D-05; verifier PASSED 3/3 SCs; code review 0-crit/6-warn — WR-01 pinned-serve fix applied, rest dispositioned)
**Wave 1**

- [x] 19-01-PLAN.md — Wave 1 foundation: Vitest toolchain + 5 pre-build unit tests (cn/useReducedMotion/InstallChip/metadata + accuracy port, verbatim constants); 8 pinned devDeps, no Go-module touch (QA-01) — ✅ 33/33 unit green

**Wave 2** *(blocked on 19-01)*

- [x] 19-02-PLAN.md — Wave 2 E2E + post-build: playwright.config.ts (chromium, serve out --listen 4199) + home.spec.ts (port + 4 QA-02 paths) + gfx.spec.ts (port) + postbuild/seo.test.ts (port) (QA-01, QA-02) — ✅ 12/12 e2e + 29/29 postbuild green

**Wave 3** *(blocked on 19-02)*

- [x] 19-03-PLAN.md — Wave 3 CI gate + parity-retire: web.yml (6 ordered gates, first-party actions, least-privilege; static-verified D-03) + ci.yml paths-ignore (bidirectional, SC-1) + 4 .py specs proven green then DELETED + full-suite gate (QA-01, QA-02) — ✅ parity proven, .py retired

**Cross-cutting constraints:** the 4 Python specs are RETIRED only after the JS ports prove parity on the same out/ build (D-01); CI is build-verified LOCALLY (D-03 — web.yml YAML statically inspected, not run live); execution runs INLINE on main (D-05).
**UI hint**: no

### Phase 20: Runtime Hardening II (Tiers 1–3)

**Goal**: Close three runtime gaps from the 2026-06-10 audit — manual-only catalog sync, a non-functional (silently fail-open) LlamaFirewall opt-in, and narrow/overstated Sentry coverage — and make docs/threat-model match reality. Researched: `phases/20-runtime-hardening/20-RESEARCH.md` + `20-PLAN.md`. Analysis: `analysis/sentry-coverage-2026-06.md`.
**Depends on**: none (Go runtime + sidecar + honesty text; independent of the web phases). Tiers are independent — any order/parallel; Tier 3 is largest.
**Requirements**: CSYNC-01..06, LLMF-01..06, SENT-01..11

**Tier 1 — catalog sync (CSYNC):** new unprivileged `beekeeper catalogs daemon` (user-level systemd --user / launchd LaunchAgent / Windows schtasks) running `catalogs sync` on an hourly heartbeat gated by a config interval (5h/10h/24h); `CatalogSyncConfig` + project-layer-can't-disable; `SourceState` timestamps + ETag conditional sync (only the GitHub *list* call is rate-metered); wire the dead TUI `s sync all` + a schedule selector.

**Tier 2 — LlamaFirewall (LLMF):** `//go:embed` + installer (move `sidecar/`→`internal/llamafirewall/assets/`); **fix the silent fail-open** (`UserMessage(role=…)` TypeError→swallowed→"clean"; build scanners + construct once); switch IPC to loopback-TCP+token (delete the Windows-pipe fork, works on all 3 OSes); de-stub CodeShield, **remove** AlignmentCheck (cloud key); venv + HF_HOME cache + gated-22M model; fix the `$HOME` vs `StateDir` Windows bug; real-sidecar `//go:build e2e`.

**Tier 3 — Sentry + honesty (SENT):** watchlist expansion; **SENTRY-006** agent-descendant (`isMonitoredDescendant` refactor); **SENTRY-007** generalized exfil fusion + `isExternalDest`; **SENTRY-008** + new `EventFileWrite` ingestion (Linux 2nd fanotify group ≥5.9 · macOS write/rename + new-file union fix · Windows correct Kernel-File IDs 16/30/27); honesty edits (PROJECT/THREAT-MODEL §8 stale-fix/home). **Stretch:** DNS (Linux kprobe + Windows ETW DNS-Client). **OUT (residual/v2):** memory-read, macOS-DNS, Windows missing-PPID, legit-endpoint exfil.

**Success Criteria** (what must be TRUE):

  1. **T1:** injected-clock test proves interval-gated background sync; project-layer can't disable; TUI `s`/selector perform a real sync.
  2. **T2:** gated `//go:build e2e` proves benign-allow / injection / CodeShield-unsafe / crash-fail-closed; no stub-only decision path; Windows StateDir bug fixed.
  3. **T3:** SENTRY-006/007/008 fire on target + not on baselines; `EventFileWrite` builds on all 3 GOOS; watchlist trips SENTRY-001 on a 2-cloud-cred read.
  4. THREAT-MODEL §8 + home no longer overstate Sentry/LlamaFirewall; `go test ./... ` + `go vet` + `TestRulesImportsArePure` green; `pnpm build` + web specs green.

**Plans**: 6 plans in 4 waves (20-06 DNS is an OPTIONAL stretch)

Plans:
- [x] 20-01-PLAN.md — Tier 1 catalog sync (CSYNC): config schema + project-cant-disable; SourceState timestamps + ETag conditional sync; unprivileged catalogs daemon (systemd --user / LaunchAgent / schtasks) + interval gate; TUI real sync + selector + first-run sync [wave 1] ✅ 2026-06-10 (CSYNC-01..06; 4 commits 0540e52/482e8b9/6280a5f/8415c73)
- [~] 20-02-PLAN.md — Tier 2 LlamaFirewall (LLMF): embed + InstallSidecar; fix the silent fail-open API; loopback-TCP+token IPC; real CodeShield + remove AlignmentCheck; venv/22M-model/HF cache + StateDir fix; gated e2e [wave 2, blocking human checkpoint] ✅ CODE 2026-06-10 (LLMF-01..06; commits d306f19/b116c5b/923c4de/f36546d) — BLOCKING human HF-license live-bootstrap verify still pending
- [x] 20-03-PLAN.md — Tier 3 W1 Sentry rules (SENT): watchlist expansion; SENTRY-006 + isMonitoredDescendant; SENTRY-007 + isExternalDest; TestRulesImportsArePure [wave 1] ✅ 2026-06-10 (SENT-01..04; commits 0e6b5f1/3e5b3a2)
- [x] 20-04-PLAN.md — Tier 3 W2 file-write (SENT): EventFileWrite + SENTRY-008 + persistenceWritePaths; per-OS ingestion (Linux 2nd fanotify group >=5.9 / macOS write+rename+new-file union fix / Windows correct Kernel-File IDs 16/30/27) [wave 2] ✅ 2026-06-10 (SENT-05..08; commits f285ecf/c5c05db/fc5a781/03dcaec)
- [x] 20-05-PLAN.md — Tier 3 W3 honesty + tests (SENT): PROJECT/THREAT-MODEL §8 + home honesty edits; synthetic 006/007/008 + watchlist rules_test [wave 3] ✅ 2026-06-10 (SENT-09/10; commit 304fedc; synthetic cases already from 20-03/04, verified green)
- [x] 20-06-PLAN.md — Tier 3 W4 STRETCH/OPTIONAL DNS: EventDNSQuery; Linux kprobe udp/tcp_sendmsg dport53 bpf2go; Windows ETW DNS-Client ID 3006; macOS deferred v2 [wave 4] ✅ 2026-06-10 (SENT-11; commit d5719c5; eBPF QNAME parse CI-validated)
**UI hint**: no (TUI/threat-model/marketing text only)

### Phase 21: Full-System Validation & CI Calibration

**Goal**: The Go safety harness ships fully validated — every behavior is verified at the correct tier: **Tier A** (locally testable) is at 100% coverage and gate-enforced on the Windows dev box; **Tier B** (platform/kernel/build-tag) is covered by a cross-platform CI matrix; **Tier C** (irreducible: true live block on the 16 non-Claude-Code harnesses, and the gated-model e2e) is captured in a signed-off manual register. "Fully validated" = 100% of what can be tested locally + a CI matrix for everything platform-bound + a documented manual register for the rest, with zero silent gaps.
**Depends on**: the whole Go core; Phase 20 (validates its Sentry/LlamaFirewall hardening). Independent of the web phases.
**Requirements**: VAL-01, VAL-02, VAL-03, VAL-04, VAL-05, VAL-06, VAL-07, VAL-08
**Origin**: derived 2026-06-10 from the `/understand` knowledge-graph audit (per-package coverage + stub/gated/fail-open signals) — see the surfaced Tier-A gaps below. Relaxes the v1.3.0 web-only fence for the pre-ship release gate.
**Success Criteria** (what must be TRUE):

  1. `go test ./...` is green on the Windows dev box AND a coverage-gate check passes: every Go production file has a linked test OR a documented, reason-coded no-test allowlist entry (pure type/const/build-metadata + platform stubs) — zero unaccounted gaps. The surfaced Tier-A gaps are closed: `internal/ipc` server/client + Windows-pipe peer-auth, `check`/`watch`/`scan`/`gateway` `sanity.go`, `editorinit/lookup.go`, `hooks/protected.go`, and the TUI model logic.
  2. A local 17-harness conformance suite is green: for every target, the installer writes the correct config (keys, idempotent, backup-on-overwrite) and the deny renderer emits the exact per-harness block contract (exit code + JSON/stdout, golden-file asserted), including the Hermes fail-open seam and the Kilo/Trae UNGUARDED honesty cases.
  3. A cross-platform CI matrix (ubuntu-20.04/kernel-5.4 + ubuntu-22.04/kernel-5.15 + macos-latest + windows-latest) is green: build (native + 3 GOOS), vet, test, `-race` (CGO), eBPF generate+load (CI-only bytecode, never committed), eslogger (macOS), ETW (Windows), and Unix peer-cred auth.
  4. The fuzz suite (policy engine, IPC proto parser, catalog parser, MCP message parser, Sentry rule evaluator) runs in CI as a blocking release gate.
  5. A Claude Code live end-to-end test denies a canary credential read (`~/.ssh` + `~/.aws`) end-to-end — the documented true-block reference.
  6. `docs/validation-register.md` exists, enumerates all 16 non-Claude-Code harness live-block procedures + the LlamaFirewall gated-22M-model e2e (each with exact steps, expected result, sign-off), and is signed off; the README harness count is corrected (15→17 / 14→16) and the validation posture (allowlist + Tier A/B/C tiering) is documented.

**Cross-cutting constraints**: execution runs INLINE on main (Go subagents are fine here, unlike the web phases — but the executor is on Windows, so Tier-B paths are CI-validated, not local). The coverage-gate's no-test allowlist is self-defense (VAL-08): it fails closed on unjustified growth. CI is build-verified as far as the unpushed repo allows — the matrix YAML is statically validated; its live GitHub run is confirmed at first push.

**Plans**: 4 plans (3 waves) — planned 2026-06-11 (research-first; Tier A/B/C model per D-01; coverage gate = presence+reason-coded-allowlist per D-02, NOT a %-threshold; apply-all-surfaced-fixes scope per D-03, bounded to audit findings/failing tests)
**Wave 1** *(21-01 and 21-02 run in parallel — they share no files)*

- [ ] 21-01-PLAN.md — Wave 1: VAL-01/VAL-08 coverage gate — `internal/coveragegate` package (go/build prod-file walker + fail-closed reason-coded allowlist parser, six-code taxonomy) + `coverage-allowlist.txt` + `TestCoverageManifest`/`TestAllowlistFailsClosed` + the Tier-A gap tests (`catalog.ResolveHealthy` covering the 4 sanity.go delegators, `ipc` server/client/peer Tier-B, `hooks/protected.go`, `tui/model.go` functions) (VAL-01, VAL-08)
- [ ] 21-02-PLAN.md — Wave 1: VAL-02 17-harness conformance — golden-file deny contract (`deny_render_golden_test.go` + `testdata/deny/*.golden` + `-update`, all 15 HarnessIDs + unknown fail-closed, Hermes exit-0/Kilo-Trae UNGUARDED explicit rows) + 17-target installer-config conformance (`conformance_test.go` over `allTargets`: keys/idempotent/backup, gateway-print-no-file, Cline !windows) (VAL-02)

**Wave 2** *(blocked on 21-01 — the new ipc Tier-B tests must exist before the matrix runs them)*

- [ ] 21-03-PLAN.md — Wave 2: VAL-04 Sentry fuzz + VAL-03 CI matrix delta — `internal/sentry/fuzz_test.go` `FuzzEvaluateEvent` (//go:build fuzz, no-panic + valid-severity-only) + `ci.yml` delta only (3×GOOS cross-build step + `fuzz-sentry` blocking job → `release-gate.needs`; pre-existing eBPF/kernel/eslogger/-race jobs preserved; web/release isolation intact) (VAL-03, VAL-04)

**Wave 3** *(blocked on Wave 2)*

- [ ] 21-04-PLAN.md — Wave 3: VAL-05 e2e + VAL-06 register + VAL-07 honesty — `internal/check/e2e_test.go` `--hook claude-code` exit-2 canary case + `docs/validation-register.md` (16 non-CC harnesses + gated-22M-model e2e, sign-off fields, gated entry PENDING) + `docs/validation-posture.md` (Tier A/B/C + allowlist taxonomy) + README 15→17/14→16 fix + blocking maintainer register sign-off (autonomous:false) (VAL-05, VAL-06, VAL-07)

**Cross-cutting constraints**: 21-01 and 21-02 run in parallel (zero file overlap); 21-03 depends on 21-01 (the matrix runs the new ipc Tier-B tests); execution runs INLINE on main (Go subagents fine); Tier-B paths (eBPF/eslogger/ETW/-race/peer-cred) are CI-validated, NOT run on the Windows dev box; the CI matrix YAML is statically authored + locally build-verified (live GitHub run at first push, D-05); the coverage allowlist is the phase's self-defense surface (VAL-08, fails closed); zero external packages installed (RESEARCH Package Legitimacy Audit: N/A).
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
| **18.1 Docs Theme Restyle** | **v1.3.0** | **quick-task** | **Complete (borders/accents/duplication; command-card split → backlog)** | **2026-06-09** |
| 19. Test Suite & CI | v1.3.0 | 3/3 | ✅ Complete & verified 2026-06-10 (executed INLINE on main; verifier PASSED 3/3 SCs; unit 33 + postbuild 29 + e2e 12 green; 4 .py specs retired after parity; code review 0-crit/6-warn, WR-01 pinned-serve fix applied) | QA-01, QA-02 |
| **20. Runtime Hardening II (Tiers 1–3)** | **v1.3.0** | **6/6** | **All plans CODE-complete (20-01..06 ✅). ONLY remaining: 20-02 LLMF human HF-license live-bootstrap verify (gated 22M model)** | **—** |
| **21. Full-System Validation & CI Calibration** | **v1.3.0** | **0/4** | **Planned 2026-06-11 (4 plans / 3 waves; VAL-01..08 all covered; research-first; plan-checker pending). W1 coverage-gate + 17-harness conformance (parallel) → W2 Sentry fuzz + CI-matrix delta → W3 Claude-Code e2e + register + honesty docs. NEXT: /gsd-execute-phase 21** | **—** |
