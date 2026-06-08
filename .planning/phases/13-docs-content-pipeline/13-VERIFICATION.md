---
phase: 13-docs-content-pipeline
verified: 2026-06-08T16:00:00Z
status: human_needed
score: 4/4 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Toggle light/dark theme on a /docs/ page"
    expected: "No flash-of-wrong-theme; a single theme toggle (bk-theme storageKey) controls the appearance — no double-toggle or Fumadocs-owned theme controller competing"
    why_human: "FOWT is a visual, transient render artifact — static HTML analysis and grep cannot catch a flash that occurs during client-side hydration. RootProvider has theme={{ enabled: false }} (verified in code), but the runtime behavior of next-themes interacting with the hydrated page can only be confirmed in a live browser."
---

# Phase 13: Docs Content Pipeline — Verification Report

**Phase Goal:** A visitor can browse a Fumadocs-powered documentation site with sidebar navigation, table of contents, and working STATIC (Orama) search — all served from pre-built static files with NO server runtime (`output: "export"`).
**Verified:** 2026-06-08T16:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `cd web && pnpm build` exits 0; `out/docs/` has HTML for every docs route; Orama static index is non-empty | VERIFIED | Build run: exit 0, 13 static pages generated. All 8 `out/docs/<section>/index.html` confirmed present. `out/api/search` = 93,388 bytes of valid Orama JSON (154+ "beekeeper" references across all 8 sections). |
| 2 | The docs sidebar lists all 8 top-level sections in exact order: getting-started → installation → configuration → integration → security → cli-reference → audit-log → troubleshooting | VERIFIED | Static HTML analysis: section names appear at strictly ascending character positions (1300, 10291, 12032, 13777, 15517, 17248, 18994, 20728). `web/content/docs/meta.json` `pages` array matches the required order exactly. Playwright evidence in 13-03-SUMMARY.md confirms the rendered sidebar order. |
| 3 | The table-of-contents panel renders for the getting-started page with >= 3 headings (4 required) | VERIFIED | Static HTML `out/docs/getting-started/index.html` contains 4 TOC `<a href="#...">` entries: prerequisites, installation, hook-registration, verifying-the-hook. `getting-started/index.mdx` has exactly 4 `## ` headings. Playwright confirmed 4 TOC entries. |
| 4 | The search dialog opens, a query returns >= 1 result, and clicking a result navigates to the correct docs page | VERIFIED (Playwright evidence) | Orama index contains all 8 sections (85–105 occurrences each), 93KB / 80 indexed documents. Playwright run in 13-03 confirms: 36 results for "beekeeper", first click navigated to `/docs/cli-reference/`. Cannot re-drive a served browser from this environment; Playwright evidence accepted per verification guidance. |

**Score:** 4/4 truths verified

---

### SC-1 Path Clarification

ROADMAP SC-1 reads "out/api/search/index.json is non-empty". The build emits `out/api/search` (flat file, no extension), not `out/api/search/index.json`. This is correct Next.js behavior with `trailingSlash: true` — route handlers are emitted as flat files in static export (Pitfall 6 from 13-RESEARCH.md, confirmed empirically). The verification guidance for this phase explicitly accepts `out/api/search` as satisfying SC-1. The flat file is 93,388 bytes of valid Orama JSON. SC-1 is VERIFIED.

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/next.config.mjs` | ESM Next config with createMDX, all 4 static-export keys | VERIFIED | `createMDX` import + `withMDX(nextConfig)`. Keys confirmed: `output:'export'`, `trailingSlash:true`, `images.unoptimized:true`, `transpilePackages:['three','@react-three/fiber','@react-three/drei']`. `next.config.ts` absent. |
| `web/source.config.ts` | fumadocs-mdx collection config pointing at `content/docs` | VERIFIED | `defineDocs({ dir: 'content/docs' })` + `export default defineConfig()`. Forward slash path. |
| `web/tsconfig.json` | `collections/*` path alias resolving to `./.source/*` | VERIFIED | `"collections/*": ["./.source/*"]` present in `compilerOptions.paths`. |
| `web/package.json` | Pinned fumadocs deps + postinstall script | VERIFIED | `fumadocs-ui: 16.9.3`, `fumadocs-core: 16.9.3`, `fumadocs-mdx: 15.0.11` (exact, no `^`). `postinstall: "fumadocs-mdx"` present. |
| `web/lib/source.ts` | loader() binding toFumadocsSource() | VERIFIED | Imports `docs` from `collections/server`, `loader` from `fumadocs-core/source`. Uses `toFumadocsSource()`. No `"use client"`. |
| `web/app/docs/layout.tsx` | DocsLayout shell with sidebar + TOC | VERIFIED | `DocsLayout` from `fumadocs-ui/layouts/docs`. `tree={source.pageTree}`. No `"use client"`. |
| `web/app/docs/[[...slug]]/page.tsx` | MDX page renderer with generateStaticParams | VERIFIED | `generateStaticParams` → `source.generateParams()`. Imports `DocsPage/DocsBody/DocsTitle/DocsDescription` from `fumadocs-ui/layouts/docs/page`. Awaits `props.params` (Next 16). |
| `web/app/api/search/route.ts` | Orama static search route (staticGET) | VERIFIED | `export const revalidate = false`. `export const { staticGET: GET } = createFromSource(source)`. No `dynamic = 'force-static'`. File is `.ts` not `.tsx`. |
| `web/app/providers.tsx` | RootProvider with theme disabled + static search options | VERIFIED | `RootProvider` from `fumadocs-ui/provider/next`. `theme={{ enabled: false }}`. `search={{ options: { type: "static" } }}`. Nested inside `ReducedMotionProvider` inside `ThemeProvider`. `"use client"` preserved. |
| `web/app/globals.css` | Fumadocs Section 3 imports + real @source glob | VERIFIED | `@import "fumadocs-ui/css/shadcn.css"` and `@import "fumadocs-ui/css/preset.css"` uncommented. `@source "../node_modules/fumadocs-ui/dist/**/*.js"` present. `[double-star]` placeholder absent. Beekeeper `@theme inline` cascade follows as Section 5 (last, wins). |
| `web/content/docs/meta.json` | Top-level sidebar order (8-section pages array) | VERIFIED | `"pages": ["getting-started","installation","configuration","integration","security","cli-reference","audit-log","troubleshooting"]` — exact required order. |
| `web/content/docs/getting-started/index.mdx` | TOC test page — 4 headings, seed prose for Orama | VERIFIED | 4 `##` headings: Prerequisites, Installation, Hook Registration, Verifying the Hook. Has `title:` and `description:` frontmatter. "beekeeper" appears in body. |
| `web/content/docs/troubleshooting/index.mdx` | 8th seed section page | VERIFIED | Present with `title:` and `description:` frontmatter, 4 headings, 86 body words. |
| All 8 section `meta.json` files | Section-level page ordering | VERIFIED | All 8 `content/docs/<section>/meta.json` files present with correct titles and `"pages": ["index"]`. |
| `web/.source/` | Generated codegen directory | VERIFIED | `browser.ts`, `server.ts`, `dynamic.ts`, `source.config.mjs` all present. |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `web/next.config.mjs` | `fumadocs-mdx/next` | `createMDX` import + `withMDX(nextConfig)` wrap | WIRED | Import and usage both present. |
| `web/source.config.ts` | `content/docs` | `defineDocs({ dir: 'content/docs' })` | WIRED | Forward slash path confirmed. |
| `web/lib/source.ts` | `collections/server` | `import { docs } from "collections/server"` | WIRED | tsconfig alias `"collections/*": ["./.source/*"]` resolves this. |
| `web/app/docs/[[...slug]]/page.tsx` | `web/lib/source.ts` | `source.getPage` / `source.generateParams()` | WIRED | Both `source.generateParams()` (in generateStaticParams) and `source.getPage(params.slug)` (in Page + generateMetadata) are present. |
| `web/app/api/search/route.ts` | `web/lib/source.ts` | `createFromSource(source)` | WIRED | `createFromSource(source)` on line 6; `source` imported from `@/lib/source`. |
| `web/app/providers.tsx` | `fumadocs-ui/provider/next` | `RootProvider` with `theme={{ enabled: false }}` + `search={{ options: { type: "static" } }}` | WIRED | Import and usage confirmed. |
| `web/content/docs/meta.json` | 8 section directories | `"pages"` array forcing order | WIRED | All 8 directory names in `pages` array; all 8 directories with `meta.json` and `index.mdx` confirmed. |
| `web/content/docs/*/index.mdx` | `out/api/search` | fumadocs-mdx → loader → createFromSource indexes seed text | WIRED | All 8 sections appear 82–105 times in the Orama index; 80 total indexed documents. |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `web/app/docs/[[...slug]]/page.tsx` | `page` | `source.getPage(params.slug)` → loader over `.source/` → fumadocs-mdx over `content/docs/*.mdx` | Yes — MDX files confirmed populated with seed content; 8 routes render to non-empty HTML | FLOWING |
| `web/app/api/search/route.ts` | search index | `createFromSource(source)` → Orama indexes all pages from loader | Yes — 93,388-byte index with 80 documents, all 8 sections present | FLOWING |
| `web/app/docs/layout.tsx` | `source.pageTree` | loader-generated page tree from meta.json + MDX | Yes — page tree embedded in static HTML contains all 8 sections in correct order | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `pnpm build` exits 0 | `cd web && pnpm build` | Exit 0; 13 static pages generated | PASS |
| All 8 `out/docs/<section>/index.html` exist | Node filesystem check | All 8 confirmed present | PASS |
| `out/api/search` non-empty (> 100 bytes) | `fs.statSync` | 93,388 bytes | PASS |
| `out/api/search` is valid JSON | `JSON.parse` | Parses successfully; keys: type, internalDocumentIDStore, index, docs, sorting | PASS |
| Orama index covers all 8 sections | Pattern match per section | 82–105 occurrences per section | PASS |
| getting-started has 4 TOC entries in static HTML | Node regex on index.html | 4 `a[href^="#"]` entries (prerequisites, installation, hook-registration, verifying-the-hook) | PASS |
| Sidebar section order preserved in static HTML | Character-position ordering | GS(1300) < Install(10291) < Config(12032) < Integ(13777) < Sec(15517) < CLI(17248) < Audit(18994) < Trouble(20728) | PASS |
| No server runtime in `out/` | `ls out/ \| grep server` | No server files present | PASS |
| `next.config.ts` deleted | File existence check | ABSENT (correct) | PASS |

---

### Probe Execution

No probe scripts discovered for Phase 13. Build gate and filesystem assertions serve as the equivalent verification. Step 7c: SKIPPED (no `scripts/*/tests/probe-*.sh` for this phase; `pnpm build` + filesystem asserts are the documented gate).

---

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| DOCS-01 | 13-01-PLAN.md, 13-02-PLAN.md, 13-03-PLAN.md | A user can browse a Fumadocs-powered docs site with sidebar navigation, table of contents, and working static (Orama) search | SATISFIED | All three technical pillars verified: sidebar (8-section forced order, confirmed in static HTML + Playwright), TOC (4 entries on getting-started, confirmed in static HTML), search (93KB Orama index with 80 docs + Playwright 36 results for "beekeeper"). Static-only: no server files in `out/`, `output: 'export'` preserved in `next.config.mjs`. |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `web/app/docs/[[...slug]]/page.tsx` | 19, 30 | `source.getPage(params.slug)` called twice (in `generateMetadata` and in `Page`) | Info (WR-03) | Minor: doubles lookup per static-export route pre-render pass. Not a correctness issue; TypeScript `notFound()→never` handles nullability correctly. |
| `pnpm-workspace.yaml` | 12–13 | `allowBuilds: esbuild: true` contradicts header comment "build scripts are DENIED" | Warning (WR-01) | Documentation/audit confusion. esbuild's postinstall runs and installs a platform-specific native binary — semantically different from `ignoredBuiltDependencies`. No runtime security impact for the static site, but meaningful posture gap for a security product. |
| `web/app/api/search/route.ts` | — | Emits `out/api/search` (flat file); no `_redirects` rule ensures Cloudflare Pages serves it | Warning (WR-02) | Latent deployment risk: Cloudflare Pages may treat the extensionless path differently from local `serve`. Only manifests post-deployment, not during local build/serve. |
| `web/package.json` | 15 | `@types/mdx` in `dependencies`, not `devDependencies` | Info (IN-01) | No runtime impact for static export. Inconsistent with other `@types/*` packages. |

No `TBD`, `FIXME`, or `XXX` debt markers found in any Phase 13 source files.

---

### Human Verification Required

#### 1. FOWT / Theme-Bridge Non-Regression

**Test:** Deploy or serve `out/` locally (`python -m http.server <port> --directory out`). Navigate to a `/docs/` page. Observe the initial render. Toggle light/dark theme using the theme switch.

**Expected:**
- No flash-of-wrong-theme during initial hydration (page renders in the correct theme immediately).
- A single theme toggle: one click → theme changes, no double-toggle or competing theme controller.
- The `bk-theme` localStorage key is the sole owner. No Fumadocs-internal theme state is active (RootProvider has `theme={{ enabled: false }}`).

**Why human:** FOWT is a transient hydration artifact. `providers.tsx` has `theme={{ enabled: false }}` on `RootProvider` (verified in code), but the runtime interaction between next-themes, the hydration script in `layout.tsx`, and the Fumadocs provider tree can only be confirmed in a live browser. This was listed as a manual-only verification in `13-VALIDATION.md`.

---

### Gaps Summary

No blocking gaps. All four ROADMAP success criteria are satisfied by the codebase evidence. The single human verification item (FOWT / theme-bridge non-regression) is a quality confirmation, not a blocker — the code wiring (`theme={{ enabled: false }}` on RootProvider, single `ThemeProvider` owner) is correct; only the runtime visual behavior requires a browser confirmation.

Two code-review warnings (WR-01: pnpm-workspace.yaml comment, WR-02: Cloudflare Pages search path) are latent concerns acknowledged in 13-REVIEW.md. Neither blocks the phase goal (static-export site with working docs, sidebar, TOC, and local-build search). WR-02 is a deployment concern appropriate for Phase 19 (CI pipeline, which would validate the deployed URL).

---

_Verified: 2026-06-08T16:00:00Z_
_Verifier: Claude (gsd-verifier)_
