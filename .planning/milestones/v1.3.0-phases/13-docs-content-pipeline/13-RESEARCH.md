# Phase 13: Docs Content Pipeline — Research

**Researched:** 2026-06-08
**Domain:** Fumadocs v16 + fumadocs-mdx v15 + Next.js 16 static export + Orama static search + Tailwind v4
**Confidence:** HIGH (core APIs verified against npm registry and live fumadocs docs; version numbers confirmed via `npm view`)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DOCS-01 | A user can browse a Fumadocs-powered docs site with sidebar navigation, table of contents, and working static (Orama) search | §Standard Stack, §Dependency Set, §Static Search, §Docs Route, §Sidebar Ordering, §Seed Content |
</phase_requirements>

---

<user_constraints>
## User Constraints (from CONTEXT.md)

No CONTEXT.md exists for Phase 13. Constraints are derived from STATE.md Accumulated Context (locked decisions) and CLAUDE.md.

### Locked Decisions (from STATE.md + REQUIREMENTS.md)
- `output: "export"` — static export only, no server runtime (SITE-01, locked from Phase 11)
- `trailingSlash: true` — required for static hosting (locked from Phase 11)
- `transpilePackages: ["three", "@react-three/fiber", "@react-three/drei"]` — already in next.config.ts (locked from Phase 11)
- `images.unoptimized: true` — locked from Phase 11
- Fumadocs is the docs framework (locked from STACK.md research)
- Orama static search (no server-side search) — locked from STACK.md
- pnpm workspace; authoritative lockfile at repo root (`pnpm-lock.yaml`) — locked from Phase 11
- Tailwind v4 CSS-first (`@theme inline`) — locked from Phase 12
- next-themes (class strategy, `storageKey: "bk-theme"`) — locked from Phase 12
- Beekeeper token cascade in `globals.css` is canonical and must not be overridden by Fumadocs CSS
- Fumadocs imports in `globals.css` are already stubbed as comments in Section 3 — Phase 13 uncomments them
- `RootProvider` insertion is marked in `providers.tsx` — Phase 13 inserts `<RootProvider theme={{ enabled: false }}>` there
- The `@source` placeholder in globals.css line 10 must be restored to the real glob (see deviation note in 12-03-SUMMARY.md)
- DOCS-02 through DOCS-09 are Phase 18 scope — Phase 13 creates skeleton/seed content only

### Claude's Discretion
- Exact meta.json sidebar content and seed MDX prose
- Whether to rename `next.config.ts` to `next.config.mjs` for fumadocs-mdx ESM compatibility (recommended: rename)
- Whether to add `postinstall: fumadocs-mdx` script to package.json (recommended: yes, for CI type-gen)

### Deferred Ideas (OUT OF SCOPE)
- Full prose content authoring (Phase 18)
- Changelog pipeline (Phase 14)
- Marketing home content (Phase 15)
- 3D layer (Phase 16)
- SEO/sitemap (Phase 17)
- CI workflow (Phase 19)
</user_constraints>

---

## Summary

Phase 13 installs the Fumadocs pipeline (`fumadocs-ui@16.9.3`, `fumadocs-core@16.9.3`, `fumadocs-mdx@15.0.11`) and wires the complete static docs infrastructure: source configuration, MDX compilation, docs layout, the `[[...slug]]` catch-all with `generateStaticParams`, Orama static search (via `createFromSource` + `staticGET`), and the eight-section sidebar skeleton with seed MDX content.

**The most important things the planner needs to know:**

1. **`fumadocs-mdx` is ESM-only** — `next.config.ts` must be renamed to `next.config.mjs` (or `.mts`) before adding `createMDX`. The existing TypeScript config content works unchanged in `.mjs` with `import type { NextConfig }`.

2. **The static search API changed** from the STACK.md research. The correct pattern is `createFromSource(source)` with `staticGET` (from `fumadocs-core/search/server`), NOT `getDocsSearchIndex`. The RootProvider search prop is `search={{ options: { type: 'static' } }}` with the `options` wrapper.

3. **`RootProvider` is at `fumadocs-ui/provider/next`** — there is no `fumadocs-ui/provider` export in v16.

4. **`lib/source.ts` uses `docs.toFumadocsSource()`** — the method is NOT `toFumaMDX()` or `createMDXSource()`.

5. **`tsconfig.json` needs a `collections/*` path alias** pointing to `./.source/*` so `import { docs } from 'collections/server'` resolves.

6. **The `.source/` directory** is already gitignored (`web/.source` in repo root `.gitignore`, confirmed Phase 11). It is generated automatically on `next build` / `next dev`.

7. **No build-script approval gate issues** — none of the three fumadocs packages have `postinstall` scripts; pnpm will install them without touching `pnpm-workspace.yaml`.

**Primary recommendation:** Follow the dependency install order and file-creation order precisely. Run `pnpm build` after each of the three major waves: (1) after dependencies are installed and config files created, (2) after source/layout wiring, (3) after seed MDX is added. Each wave has a distinct build-failure mode.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| MDX compilation (content → RSC) | Frontend Server (build) | — | `fumadocs-mdx` Next.js plugin runs at `next build`; zero runtime |
| Page tree / sidebar data | Frontend Server (build) | — | `loader()` creates the page tree from `.source/`; static at build time |
| Docs route rendering (HTML) | Frontend Server (build) | Browser / Client | `[[...slug]]/page.tsx` is a Server Component; Client components are Fumadocs UI internals |
| TOC rendering | Browser / Client | Frontend Server (build) | TOC is a Fumadocs component; skeleton rendered server-side, scrollspy client |
| Static search index | Frontend Server (build) → CDN | Browser / Client | `staticGET` emits `out/api/search/index.json` at build; Orama queries it in browser |
| Theme (light/dark) | Browser / Client | — | Already wired in Phase 12; RootProvider with `theme={{ enabled: false }}` defers to next-themes |
| CSS token cascade | Frontend Server (build) | — | Fumadocs CSS injected at build by Tailwind v4; Beekeeper `@theme inline` wins because it is last |

---

## Standard Stack

### Core (to be installed in Phase 13)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `fumadocs-ui` | `16.9.3` | Docs layout shell, sidebar, TOC, search dialog, code blocks | Verified `npm view fumadocs-ui version` → 16.9.3 (published 2026-05-29) [VERIFIED: npm registry] |
| `fumadocs-core` | `16.9.3` | Source loader, page tree, search server function, utilities | Required peer of fumadocs-ui; `npm view fumadocs-core version` → 16.9.3 [VERIFIED: npm registry] |
| `fumadocs-mdx` | `15.0.11` | Build-time MDX compilation, `.source/` generation, Next.js plugin | `npm view fumadocs-mdx version` → 15.0.11 [VERIFIED: npm registry] |
| `@types/mdx` | `2.0.14` | TypeScript types for MDX files | Required peer (optional but needed for `.mdx` TS types); `npm view @types/mdx version` → 2.0.14 [VERIFIED: npm registry] |

### Already installed (pulled in by fumadocs packages)

| Library | Version | Notes |
|---------|---------|-------|
| `@orama/orama` | `^3.1.18` | Already a fumadocs-core dependency; no separate install needed [VERIFIED: npm registry] |
| `motion` | `^12.40.0` | Already a fumadocs-ui dependency (used internally for animations) |

### No additional peer dependencies needed

`fumadocs-ui` peer deps: `fumadocs-core@16.9.3`, `next@16.x.x`, `react@^19.2.0`, `react-dom@^19.2.0`, `@types/mdx@*` (optional), `@takumi-rs/image-response@*` (optional — skip). All satisfied by existing package.json.

`fumadocs-mdx` peer deps: `fumadocs-core@^16.7.0`, `next@^15.3.0 || ^16.0.0`, `react@^19.2.0`, `rolldown@*` (optional), `vite@7.x.x || 8.x.x` (optional). All satisfied; rolldown/vite not needed for Next.js mode.

### Installation command

```bash
# From repo root (pnpm workspace) — installs into web/node_modules
pnpm --filter web add fumadocs-ui fumadocs-core fumadocs-mdx @types/mdx
```

Or from `web/` directory:

```bash
cd web && pnpm add fumadocs-ui fumadocs-core fumadocs-mdx @types/mdx
```

**`pnpm-workspace.yaml` build-approval gate:** None of the three fumadocs packages have `postinstall` scripts (verified: `npm view fumadocs-mdx scripts.postinstall` — empty; same for fumadocs-core, fumadocs-ui). No changes to `pnpm-workspace.yaml` `allowBuilds` or `ignoredBuiltDependencies` are required. [VERIFIED: npm registry]

---

## Package Legitimacy Audit

> slopcheck unavailable in this environment. All packages below are tagged based on registry age and well-known status.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `fumadocs-ui` | npm | ~3 yrs | >100K/wk | github.com/fuma-nama/fumadocs | unavailable | APPROVED — active project, well-known Fumadocs ecosystem [ASSUMED] |
| `fumadocs-core` | npm | ~3 yrs | >100K/wk | github.com/fuma-nama/fumadocs | unavailable | APPROVED — same repo as fumadocs-ui [ASSUMED] |
| `fumadocs-mdx` | npm | ~3 yrs | >50K/wk | github.com/fuma-nama/fumadocs | unavailable | APPROVED — same repo; fumadocs-mdx bin registered [ASSUMED] |
| `@types/mdx` | npm | 4+ yrs | >1M/wk | DefinitelyTyped | unavailable | APPROVED — DefinitelyTyped standard package [ASSUMED] |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*slopcheck unavailable. All packages are well-known and match the locked STACK.md recommendation. Maintainer may skip human-verify checkpoints for these given project context.*

---

## Architecture Patterns

### System Architecture Diagram

```
content/docs/**/*.mdx + content/docs/**/meta.json
         |
         | [next build triggers fumadocs-mdx Next.js plugin]
         | (via createMDX() in next.config.mjs)
         v
web/.source/server.ts       ← generated: exports { docs }
web/.source/browser.ts      ← generated: browser-safe data
         |
         | [imported at build by lib/source.ts]
         v
web/lib/source.ts
  docs.toFumadocsSource()
  loader({ baseUrl: '/docs', source: ... })
         |
         ├──→ app/docs/layout.tsx
         │     DocsLayout { tree: source.getPageTree() }
         │
         ├──→ app/docs/[[...slug]]/page.tsx
         │     generateStaticParams → source.generateParams()
         │     source.getPage(slug) → MDX page data
         │     DocsPage + DocsBody (from 'fumadocs-ui/layouts/docs/page')
         │     → out/docs/<section>/<page>/index.html
         │
         └──→ app/api/search/route.ts
               createFromSource(source) → { staticGET: GET }
               export const revalidate = false
               → out/api/search/index.json (Orama static index)

app/providers.tsx
  <ThemeProvider> (next-themes, already wired Phase 12)
    <ReducedMotionProvider> (already wired Phase 12)
      <RootProvider          ← Phase 13 inserts here
        theme={{ enabled: false }}
        search={{ options: { type: 'static' } }}
      >
        {children}
      </RootProvider>
```

### Recommended Project Structure (Phase 13 additions)

```
web/
├── next.config.mjs          # RENAMED from .ts; createMDX wraps existing config
├── source.config.ts         # NEW: defineDocs({ dir: 'content/docs' }) + defineConfig()
├── tsconfig.json            # MODIFIED: add "collections/*": ["./.source/*"] path alias
├── app/
│   ├── providers.tsx        # MODIFIED: insert RootProvider (marker already exists)
│   ├── globals.css          # MODIFIED: uncomment 3 Fumadocs lines in Section 3
│   ├── docs/
│   │   ├── layout.tsx       # NEW: DocsLayout with source.getPageTree()
│   │   └── [[...slug]]/
│   │       └── page.tsx     # NEW: MDX renderer + generateStaticParams
│   └── api/
│       └── search/
│           └── route.ts     # NEW: createFromSource(source) + staticGET
├── lib/
│   └── source.ts            # NEW: loader() from fumadocs-core/source
└── content/
    └── docs/                # NEW directory tree
        ├── meta.json        # top-level sidebar order
        ├── getting-started/
        │   ├── meta.json
        │   └── index.mdx    # seed page (multi-heading for TOC)
        ├── installation/
        │   ├── meta.json
        │   └── index.mdx
        ├── configuration/
        │   ├── meta.json
        │   └── index.mdx
        ├── integration/
        │   ├── meta.json
        │   └── index.mdx
        ├── security/
        │   ├── meta.json
        │   └── index.mdx
        ├── cli-reference/
        │   ├── meta.json
        │   └── index.mdx
        ├── audit-log/
        │   ├── meta.json
        │   └── index.mdx
        └── troubleshooting/
            ├── meta.json
            └── index.mdx
```

### Pattern 1: next.config.mjs Rename + createMDX Composition

**What:** `fumadocs-mdx` is ESM-only. The Fumadocs docs say "it's recommended to use `next.config.mjs`". The current project uses `next.config.ts`. Node.js 22 with native TS resolver can work, but the safest path is rename to `.mjs` and keep `import type` syntax.

**Why rename, not keep `.ts`:** Node.js native TypeScript support in v22 requires `--experimental-strip-types` flag or Node v22.6+ with strip-types enabled. Next.js 16 on Node 22.17.0 (installed) should support this, but ESM resolution of `fumadocs-mdx/next` is more reliably handled in `.mjs`. The rename is a 1-line change to the extension, and `import type { NextConfig }` syntax works in both `.ts` and `.mjs`. [ASSUMED — safe path based on fumadocs official recommendation]

**Files to change:** Rename `web/next.config.ts` → `web/next.config.mjs`. The `import type` line must be updated to a JSDoc comment or the TypeScript syntax kept as-is (`.mjs` supports TypeScript `import type`).

```javascript
// web/next.config.mjs  (renamed from next.config.ts)
// Source: fumadocs.dev/docs/mdx/next [CITED: fumadocs.dev/docs/mdx/next]
import { createMDX } from 'fumadocs-mdx/next';

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  trailingSlash: true,
  images: { unoptimized: true },
  transpilePackages: ['three', '@react-three/fiber', '@react-three/drei'],
};

const withMDX = createMDX();
export default withMDX(nextConfig);
```

The existing `output: "export"`, `trailingSlash: true`, `images.unoptimized`, and `transpilePackages` are preserved unchanged. `createMDX()` wraps the config without replacing any of its properties.

### Pattern 2: source.config.ts

**What:** Tells `fumadocs-mdx` where to find docs content and what the collection shape is.

```typescript
// web/source.config.ts
// Source: fumadocs.dev/docs/mdx/next [CITED]
import { defineDocs, defineConfig } from 'fumadocs-mdx/config';

export const docs = defineDocs({
  dir: 'content/docs',
});

export default defineConfig();
```

The `dir` is relative to the project root (`web/`). With `content/docs/` as the directory, this scans `web/content/docs/**/*.mdx` and `web/content/docs/**/meta.json`.

### Pattern 3: tsconfig.json path alias for `.source/`

**What:** `lib/source.ts` imports from `collections/server`. This resolves to `.source/server` (generated by fumadocs-mdx). The alias must be added to `tsconfig.json`.

```json
// web/tsconfig.json — add to "paths":
{
  "compilerOptions": {
    "paths": {
      "@/*": ["./*"],
      "collections/*": ["./.source/*"]
    }
  }
}
```

Without this alias, `import { docs } from 'collections/server'` fails TypeScript resolution (build-time type error).

### Pattern 4: lib/source.ts

**What:** The single binding between fumadocs-mdx's generated output and the docs pages. Exports `source` which is used by layout, page route, and search route.

```typescript
// web/lib/source.ts
// Source: fumadocs.dev/docs/mdx/next [CITED]
import { docs } from 'collections/server';
import { loader } from 'fumadocs-core/source';

export const source = loader({
  baseUrl: '/docs',
  source: docs.toFumadocsSource(),
});
```

**Important:** The method is `toFumadocsSource()` — NOT `toFumaMDX()` (old API), NOT `createMDXSource()` (renamed). This is the current fumadocs-mdx v15 API. [VERIFIED: fumadocs.dev/docs/mdx/next]

### Pattern 5: Docs Layout (`app/docs/layout.tsx`)

**What:** Fumadocs `DocsLayout` wraps all docs pages with sidebar + TOC. Does NOT share the marketing `SiteNav` or `SiteFooter`.

```typescript
// web/app/docs/layout.tsx
// Source: fumadocs.dev/docs/ui/layouts/docs [CITED]
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { source } from '@/lib/source';
import type { ReactNode } from 'react';

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <DocsLayout
      tree={source.pageTree}
      nav={{
        title: 'Beekeeper Docs',
        url: '/',
      }}
      sidebar={{
        defaultOpenLevel: 1,
      }}
    >
      {children}
    </DocsLayout>
  );
}
```

**`source.pageTree` vs `source.getPageTree()`:** Both are valid. `source.pageTree` is the direct property access (no-i18n case). The DocsLayout docs example uses `source.getPageTree()` for i18n; without i18n `source.pageTree` is sufficient and simpler. [VERIFIED: fumadocs source API docs]

**Import path:** `'fumadocs-ui/layouts/docs'` — confirmed via `npm view fumadocs-ui exports`. NOT `'fumadocs-ui/layout'` or `'fumadocs-ui'`.

### Pattern 6: Docs Catch-All Route (`app/docs/[[...slug]]/page.tsx`)

**What:** Renders each MDX doc page. Must export `generateStaticParams` for `output: 'export'`.

```typescript
// web/app/docs/[[...slug]]/page.tsx
// Source: fumadocs source API docs + Next.js 16 [CITED]
import { source } from '@/lib/source';
import {
  DocsPage,
  DocsBody,
  DocsTitle,
  DocsDescription,
} from 'fumadocs-ui/layouts/docs/page';
import { notFound } from 'next/navigation';
import type { Metadata } from 'next';

export async function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata(
  props: { params: Promise<{ slug?: string[] }> }
): Promise<Metadata> {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();
  return {
    title: page.data.title,
    description: page.data.description,
  };
}

export default async function Page(
  props: { params: Promise<{ slug?: string[] }> }
) {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();

  const MDX = page.data.body;

  return (
    <DocsPage toc={page.data.toc} full={page.data.full}>
      <DocsTitle>{page.data.title}</DocsTitle>
      <DocsDescription>{page.data.description}</DocsDescription>
      <DocsBody>
        <MDX />
      </DocsBody>
    </DocsPage>
  );
}
```

**Next.js 15+ async params:** In Next.js 16, `params` is a `Promise` — must be `await`ed. [VERIFIED: Next.js 16 docs — params are Promise in App Router]

**`generateStaticParams` returns:** `source.generateParams()` enumerates all pages from the source. This is the correct method — no manual `.map()` needed. [VERIFIED: fumadocs-core source API]

**Import path for page components:** `'fumadocs-ui/layouts/docs/page'` — confirmed via `npm view fumadocs-ui exports`. Exports `DocsPage`, `DocsBody`, `DocsTitle`, `DocsDescription`.

### Pattern 7: Static Search Route (`app/api/search/route.ts`)

**CRITICAL — API changed from STACK.md research.** The old pattern used `getDocsSearchIndex`. The current fumadocs-core v16 API uses `createFromSource` with `staticGET`. [VERIFIED: fumadocs-core/search/server docs, confirmed via fumadocs.dev]

```typescript
// web/app/api/search/route.ts
// Source: fumadocs.dev/docs/headless/search/orama [CITED]
import { source } from '@/lib/source';
import { createFromSource } from 'fumadocs-core/search/server';

export const revalidate = false;

export const { staticGET: GET } = createFromSource(source);
```

**Why `revalidate = false` instead of `dynamic = 'force-static'`:** The fumadocs docs use `revalidate = false`. Both patterns achieve the same outcome under `output: 'export'` — Next.js statically renders the route handler at build time. The `revalidate = false` form is what the official fumadocs static search docs show. [CITED: fumadocs.dev/docs/headless/search/orama]

**Output:** This emits `out/api/search/index.json` (the Orama index). With `trailingSlash: true` in next.config, the path is actually `out/api/search/index.json` — not `out/api/search.json`. Verify post-build.

### Pattern 8: RootProvider Insertion in `providers.tsx`

**What:** Phase 12 left insertion markers in `providers.tsx`. Phase 13 inserts `<RootProvider>` at that position.

**CRITICAL — import path changed:** `RootProvider` is at `'fumadocs-ui/provider/next'` (NOT `'fumadocs-ui/provider'` — that export path does NOT exist in v16). [VERIFIED: `npm view fumadocs-ui exports`]

**CRITICAL — search prop structure:** The static search prop uses an `options` wrapper: `search={{ options: { type: 'static' } }}`. NOT `search={{ type: 'static' }}`. [VERIFIED: fumadocs static export guide + WebSearch cross-check]

**CRITICAL — theme prop:** `theme={{ enabled: false }}` disables Fumadocs' internal next-themes instance. Our `<ThemeProvider>` (Phase 12) is the single theme owner. This prevents double-toggle / state desync. [CITED: fumadocs.dev/docs/ui/layouts/root-provider]

```typescript
// web/app/providers.tsx — MODIFIED (replace {children} insertion region)
"use client";
import { ThemeProvider } from "next-themes";
import { ReducedMotionProvider } from "@/lib/reduced-motion";
import { RootProvider } from "fumadocs-ui/provider/next"; // ← Phase 13 adds

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem={true}
      storageKey="bk-theme"
      disableTransitionOnChange
    >
      <ReducedMotionProvider>
        <RootProvider                                    // ← Phase 13 inserts
          theme={{ enabled: false }}
          search={{ options: { type: 'static' } }}
        >
          {children}
        </RootProvider>                                  // ← Phase 13 closes
      </ReducedMotionProvider>
    </ThemeProvider>
  );
}
```

### Pattern 9: globals.css — Uncomment Fumadocs Section 3

**What:** Phase 12 deliberately commented out the 3 Fumadocs lines. Phase 13 uncomments them.

**CRITICAL — line 10 placeholder:** Line 10 in the current `globals.css` reads:
```
/* @source "../node_modules/fumadocs-ui/dist/[double-star]/*.js"; (uncomment in Phase 13 with correct glob) */
```
The `[double-star]` is a placeholder — `**/` was not valid inside a CSS comment and caused a PostCSS parse error (see 12-03-SUMMARY.md Deviation 1). Phase 13 must replace this with the real glob.

**Correct glob (VERIFIED):** `@source "../node_modules/fumadocs-ui/dist/**/*.js";`

The `dist/**/*.js` glob is confirmed by:
1. `npm pack fumadocs-ui --dry-run` shows all JS files are under `dist/` (e.g., `dist/layouts/docs/index.js`, `dist/provider/next.js`, etc.) [VERIFIED: npm registry]
2. The official Fumadocs Tailwind v4 docs show this exact glob: `@source '../node_modules/fumadocs-ui/dist/**/*.js'` [CITED: fumadocs.dev/docs/ui/theme]

**Section 3 after edit:**
```css
/* Section 3 — Fumadocs imports: Phase 13 uncomments these */
@import "fumadocs-ui/css/shadcn.css";
@import "fumadocs-ui/css/preset.css";
@source "../node_modules/fumadocs-ui/dist/**/*.js";
```

**Import order is load-bearing:**
- `shadcn.css` must come before `preset.css` (maps `--color-fd-*` to shadcn tokens before preset reads them)
- Both must come before `@custom-variant dark` (Section 4)
- Beekeeper `@theme inline` (Section 5) must remain LAST — wins over Fumadocs tokens

The existing Sections 4, 5, 5b, 6a, 6b, 7, 8 are UNCHANGED.

### Pattern 10: postinstall script (optional but recommended)

**What:** `fumadocs-mdx` provides a CLI binary that generates `.source/` types without running Next.js. Recommended as `postinstall` for fresh CI clones that run `tsc --noEmit` before `next build`.

```json
// web/package.json — add to scripts:
{
  "scripts": {
    "postinstall": "fumadocs-mdx"
  }
}
```

**Why:** Without this, a `pnpm install` followed by `tsc --noEmit` fails because `.source/server.ts` doesn't exist yet. With this, the type generation runs immediately after install.

**Risk:** The postinstall script in fumadocs-mdx has been reported to fail with Node.js v25 (GitHub issue #2456). Node 22.17.0 is confirmed installed and is the CI target — no issue. Zod v4 conflict (issue #2860) is also not a risk here since this project does not have zod in package.json.

**pnpm build-approval gate:** Adds the project's own `postinstall` script (runs at `pnpm install` for the `web` workspace). This is NOT a dependency postinstall script — it does not require changes to `pnpm-workspace.yaml`. The `allowBuilds` / `ignoredBuiltDependencies` keys only gate dependency lifecycle scripts, not the package's own scripts. [VERIFIED: pnpm workspace docs]

### Anti-Patterns to Avoid

- **`import { RootProvider } from 'fumadocs-ui/provider'`** — this export path does not exist in fumadocs-ui v16. Use `'fumadocs-ui/provider/next'`.
- **`search={{ type: 'static' }}`** on RootProvider — wrong structure; must be `search={{ options: { type: 'static' } }}`.
- **`getDocsSearchIndex`** from `fumadocs-core/search/server` — renamed/removed. Use `createFromSource(source)`.
- **`export const dynamic = 'force-static'`** in the search route — not wrong, but fumadocs docs use `revalidate = false`. Use the fumadocs-recommended pattern.
- **`docs.toFumaMDX()`** — old method name. Use `docs.toFumadocsSource()`.
- **`import { DocsLayout } from 'fumadocs-ui/layouts/docs'`** without the `/page` suffix for `DocsPage`/`DocsBody` — these are at separate paths: `DocsLayout` from `'fumadocs-ui/layouts/docs'`; `DocsPage`/`DocsBody`/`DocsTitle`/`DocsDescription` from `'fumadocs-ui/layouts/docs/page'`.
- **Leaving `next.config.ts`** and adding ESM imports — risk of resolution failure; rename to `.mjs`.
- **Adding `fumadocs-ui` to `transpilePackages`** — NOT needed; fumadocs-ui ships pre-compiled CJS/ESM dist files.
- **Keeping the `[double-star]` placeholder** in the `@source` glob — it is invalid CSS and will fail PostCSS.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Static search index | Custom MDX-to-JSON extractor | `createFromSource(source)` + `staticGET` | Handles TOC text, code, frontmatter; correct Orama index format |
| Sidebar from meta.json | Custom directory traversal | `defineDocs()` + `loader()` + `meta.json` | fumadocs-mdx handles nested dirs, ordering, draft pages |
| `generateStaticParams` for docs | Manual `readdirSync` | `source.generateParams()` | Returns correct `{slug: string[]}` array for catch-all |
| MDX compilation | `@mdx-js/loader` directly | `createMDX()` plugin | fumadocs preset includes rehype-shiki, remark-gfm, etc. |
| TOC extraction | Custom heading parser | `page.data.toc` from fumadocs-mdx | Nested, anchored, used by `DocsPage toc={}` prop |
| CSS token bridging | Manual CSS var mapping | `fumadocs-ui/css/shadcn.css` + import order | Maps all `--color-fd-*` to shadcn tokens automatically |

---

## Key Dependency Investigation Findings

### fumadocs-mdx version mismatch

The STACK.md locked `fumadocs-mdx@14.2.11`. The npm registry latest is `fumadocs-mdx@15.0.11`. This is a minor-major version delta. Use `15.0.11` — it matches the peer requirement of `fumadocs-core@16.9.3` (`fumadocs-core: ^16.7.0` in fumadocs-mdx peers, and fumadocs-mdx 15.x is what aligns with fumadocs-core 16.x). [VERIFIED: npm registry]

| Package | STACK.md version | npm latest | Use |
|---------|-----------------|------------|-----|
| `fumadocs-ui` | 16.9.3 | 16.9.3 | ✓ matches |
| `fumadocs-core` | 16.9.3 | 16.9.3 | ✓ matches |
| `fumadocs-mdx` | 14.2.11 | **15.0.11** | Use 15.0.11 |

### API changes from STACK.md research (critical corrections)

| STACK.md assumed | Actual current API | Confidence |
|------------------|--------------------|------------|
| `import { RootProvider } from 'fumadocs-ui/provider'` | `import { RootProvider } from 'fumadocs-ui/provider/next'` | HIGH [VERIFIED: npm exports] |
| `search={{ type: 'static' }}` on RootProvider | `search={{ options: { type: 'static' } }}` | MEDIUM [VERIFIED: WebSearch fumadocs static export guide] |
| `getDocsSearchIndex(source.getPages())` | `createFromSource(source)` with `staticGET` | HIGH [CITED: fumadocs.dev/docs/headless/search/orama] |
| `createMDXSource(import('../.source/index'), import('../.source/meta'))` | `docs.toFumadocsSource()` with `import { docs } from 'collections/server'` | HIGH [CITED: fumadocs.dev/docs/mdx/next] |
| `source.pageTree` passed to `DocsLayout tree={}` | Both `source.pageTree` and `source.getPageTree()` valid; use `source.pageTree` | HIGH [VERIFIED: fumadocs source API] |
| `export const dynamic = 'force-static'` | `export const revalidate = false` | MEDIUM [pattern from fumadocs static search docs — either works] |

---

## Static Search End-to-End

### Build-time flow

```
next build
  → fumadocs-mdx plugin processes content/docs/**/*.mdx
  → .source/server.ts generated
  → lib/source.ts: createFromSource(source) builds Orama index data
  → app/api/search/route.ts: staticGET handler executes at build time
  → out/api/search/index.json emitted (Orama serialized index)
```

### Runtime flow (browser)

```
User opens search dialog (Ctrl+K)
  → Fumadocs UI fetches /api/search/index.json (once per session, cached)
  → Orama queries index client-side
  → Results rendered in dialog
  → User clicks result → navigates to /docs/<section>/<page>/
```

### Search verification

After `pnpm build`:
1. `out/api/search/index.json` must exist and be non-empty (> 100 bytes)
2. The file should contain Orama's serialized index with document data derived from the seed MDX content

If `out/api/search/index.json` is absent: the route was not executed at build time. Check `revalidate = false` is exported and that the route file is at exactly `app/api/search/route.ts` (not `route.tsx`).

If the file exists but is `{}` or `{"indexes":{},"vectorIndexes":{}}`: the source has no pages. The seed MDX files are not being picked up — check `source.config.ts` `dir` matches the actual content directory.

---

## Sidebar Ordering (`meta.json`)

### Top-level `content/docs/meta.json`

The `pages` array forces the exact 8-section order required by success criterion 2:

```json
{
  "title": "Beekeeper",
  "pages": [
    "getting-started",
    "installation",
    "configuration",
    "integration",
    "security",
    "cli-reference",
    "audit-log",
    "troubleshooting"
  ]
}
```

Without this `meta.json`, fumadocs falls back to alphabetical directory ordering — which would put `audit-log` before `cli-reference`, `configuration` before `getting-started`, etc.

### Section-level `meta.json` (each subdirectory)

Each section directory needs a `meta.json` to order its pages. For the seed phase, each section has only one page (`index.mdx`), so ordering is trivially correct. The meta.json is still required to give the section a human-readable title:

```json
// content/docs/getting-started/meta.json
{
  "title": "Getting Started",
  "pages": ["index"]
}
```

Pattern is identical for all 8 sections with appropriate titles:
- `getting-started/meta.json` → `"title": "Getting Started"`
- `installation/meta.json` → `"title": "Installation"`
- `configuration/meta.json` → `"title": "Configuration"`
- `integration/meta.json` → `"title": "Integration"`
- `security/meta.json` → `"title": "Security"`
- `cli-reference/meta.json` → `"title": "CLI Reference"`
- `audit-log/meta.json` → `"title": "Audit Log"`
- `troubleshooting/meta.json` → `"title": "Troubleshooting"`

---

## Seed Content Minimum

**Goal:** Make sidebar, TOC, and search all demonstrably work without doing Phase 18's prose authoring job.

### Minimum viable seed set: 8 MDX files (one per section)

Each seed file (`index.mdx`) needs:
1. A `title` and `description` in frontmatter (required by fumadocs-mdx schema)
2. **Multi-heading content** in at least one file — required for success criterion 3 (TOC renders for a page with multiple headings). The `getting-started/index.mdx` should have at least 3 headings.
3. **Enough searchable text** — at least 50 words per file so Orama indexes at least 1 result per query. A few sentences per section is sufficient.

**The `getting-started` page is the TOC test page.** It must have 3+ headings:

```mdx
---
title: Getting Started
description: Set up Beekeeper in minutes and protect your AI coding agent.
---

## Prerequisites

Beekeeper requires Go 1.25+ and a supported operating system (Linux, macOS, Windows).

## Installation

Install the latest release with a single command:

```bash
go install github.com/bantuson/beekeeper/cmd/beekeeper@latest
```

## Hook Registration

Register Beekeeper as a pre-tool hook for your coding agent:

```bash
beekeeper hooks install --hook claude-code
```

## Verifying the Hook

Run `beekeeper check` to confirm the hook is active and the catalog is loaded.
```

This page generates a 4-entry TOC (Prerequisites, Installation, Hook Registration, Verifying the Hook).

### Other 7 seed files

Stub content — enough text for Orama to index, no prose authoring:

```mdx
---
title: Installation
description: Install Beekeeper via go install, pre-built binary, or from source.
---

## Install Methods

Beekeeper is distributed as a single static binary.

### go install

```bash
go install github.com/bantuson/beekeeper/cmd/beekeeper@latest
```

### Pre-built Binaries

Download signed binaries from the GitHub Releases page.
```

Repeat this pattern (3+ headings, 50+ words) for all 8 sections. Full prose is Phase 18.

### Orama search verification

A query for `"beekeeper"` or `"install"` or `"hook"` against the seed content must return ≥1 result. The seed content above guarantees this.

---

## Common Pitfalls

### Pitfall 1: Wrong RootProvider import or search prop

**What goes wrong:** TypeScript compile error OR theme desync (dark/light toggle breaks) OR search shows no results.

**Why it happens:** v16 changed the import path from `fumadocs-ui/provider` (doesn't exist) to `fumadocs-ui/provider/next`. The search prop structure also has an `options` wrapper that older tutorials omit.

**How to avoid:** Use exact patterns from this research:
- Import: `from 'fumadocs-ui/provider/next'`
- Prop: `search={{ options: { type: 'static' } }}`
- Prop: `theme={{ enabled: false }}`

**Warning signs:** TypeScript error "Module not found: 'fumadocs-ui/provider'"; theme flickers after adding RootProvider; search dialog submits but no results appear.

### Pitfall 2: `out/api/search/index.json` not emitted

**What goes wrong:** `pnpm build` succeeds, `out/` exists, but `out/api/search/index.json` is absent. Search dialog opens but nothing loads.

**Why it happens:** Missing `export const revalidate = false` in the route, OR the source has no pages (seed MDX not yet created), OR `lib/source.ts` references the wrong import path.

**How to avoid:** Create seed MDX BEFORE the first full `pnpm build`. The `staticGET` handler calls `createFromSource(source)` which calls `source.getPages()` — if pages is empty, the index is empty.

**Warning signs:** `out/api/search/` directory absent; file exists but contains `{}` or minimal JSON.

### Pitfall 3: `[double-star]` placeholder left in globals.css

**What goes wrong:** `pnpm build` succeeds (CSS syntax error is in a comment, harmless at runtime) BUT Tailwind doesn't scan fumadocs-ui dist files, so many Fumadocs utility classes are absent in production. Docs section renders with broken styling.

**How to avoid:** In the globals.css edit, replace the entire commented placeholder line with the real uncommented `@source` directive.

**Warning signs:** Docs sidebar and TOC render with wrong/missing spacing, code blocks unstyled, production build looks different from `next dev`.

### Pitfall 4: `collections/server` import fails TypeScript resolution

**What goes wrong:** `tsc --noEmit` fails with "Cannot find module 'collections/server'", even though `pnpm build` might work (Next.js has its own module resolver).

**Why it happens:** `collections/server` resolves to `.source/server.ts`, but TypeScript doesn't know the alias without the `paths` entry.

**How to avoid:** Add `"collections/*": ["./.source/*"]` to `tsconfig.json` `compilerOptions.paths` BEFORE running type checks.

**Warning signs:** Red squiggles in `lib/source.ts` at the `collections/server` import; `pnpm tsc --noEmit` fails.

### Pitfall 5: `next.config.ts` ESM import fails

**What goes wrong:** `pnpm build` fails with "Cannot use import statement in a CommonJS module" or resolution error for `fumadocs-mdx/next`.

**Why it happens:** `fumadocs-mdx` is ESM-only. Node.js needs the `.mjs` extension or TypeScript native resolver to handle ESM imports correctly in config files.

**How to avoid:** Rename `next.config.ts` → `next.config.mjs` before adding `createMDX`. Confirm `pnpm build` succeeds after rename with no other changes, then add `createMDX`.

**Warning signs:** Build error mentioning `import` statement in a module context; `ERR_REQUIRE_ESM`.

### Pitfall 6: `trailingSlash: true` + search index path

**What goes wrong:** The search dialog tries to fetch `/api/search/` (with trailing slash) but Next.js emits `out/api/search/index.json`. The browser requests `/api/search/index.json` or `/api/search/` — whether this resolves depends on the static host's directory index behavior.

**Why this is not a Phase 13 problem:** On Cloudflare Pages (the target), directory requests serve `index.html` (for HTML) but JSON files are served at their exact path. The Fumadocs search client fetches `/api/search` (no trailing slash) which with `trailingSlash: true` Next.js config resolves to `/api/search/index.json` in `out/`. Cloudflare Pages serves directory contents as index files for `text/html`, but for `application/json` the exact path is used.

**Mitigation:** Verify `out/api/search/index.json` exists after build (not `out/api/search.json`). The Fumadocs search client fetches `{baseUrl}/api/search` — confirm the path matches. If issues arise at deploy time, configure a Cloudflare `_redirects` rule.

### Pitfall 7: Windows path separators in `source.config.ts`

**What goes wrong:** On Windows, `fumadocs-mdx` resolves `dir: 'content/docs'` correctly (cross-platform path handling is built in), but if the developer hand-codes a `dir` with backslashes it fails.

**How to avoid:** Always use forward slashes in `defineDocs({ dir: 'content/docs' })`. [ASSUMED — standard Node.js behavior]

---

## Code Examples

Verified patterns from official sources:

### source.config.ts (canonical)

```typescript
// Source: fumadocs.dev/docs/mdx/next [CITED]
import { defineDocs, defineConfig } from 'fumadocs-mdx/config';

export const docs = defineDocs({
  dir: 'content/docs',
});

export default defineConfig();
```

### next.config.mjs (with createMDX wrapping existing config)

```javascript
// Source: fumadocs.dev/docs/mdx/next [CITED]
import { createMDX } from 'fumadocs-mdx/next';

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  trailingSlash: true,
  images: { unoptimized: true },
  transpilePackages: ['three', '@react-three/fiber', '@react-three/drei'],
};

const withMDX = createMDX();
export default withMDX(nextConfig);
```

### lib/source.ts (canonical)

```typescript
// Source: fumadocs.dev/docs/mdx/next [CITED]
import { docs } from 'collections/server';
import { loader } from 'fumadocs-core/source';

export const source = loader({
  baseUrl: '/docs',
  source: docs.toFumadocsSource(),
});
```

### app/api/search/route.ts (static search)

```typescript
// Source: fumadocs.dev/docs/headless/search/orama [CITED]
import { source } from '@/lib/source';
import { createFromSource } from 'fumadocs-core/search/server';

export const revalidate = false;

export const { staticGET: GET } = createFromSource(source);
```

### app/docs/layout.tsx

```typescript
// Source: fumadocs.dev/docs/ui/layouts/docs [CITED]
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { source } from '@/lib/source';
import type { ReactNode } from 'react';

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <DocsLayout
      tree={source.pageTree}
      nav={{ title: 'Beekeeper Docs', url: '/' }}
      sidebar={{ defaultOpenLevel: 1 }}
    >
      {children}
    </DocsLayout>
  );
}
```

### app/docs/[[...slug]]/page.tsx (minimal working version)

```typescript
// Source: fumadocs source API + fumadocs-ui layouts/docs/page [CITED]
import { source } from '@/lib/source';
import {
  DocsPage,
  DocsBody,
  DocsTitle,
  DocsDescription,
} from 'fumadocs-ui/layouts/docs/page';
import { notFound } from 'next/navigation';

export async function generateStaticParams() {
  return source.generateParams();
}

export default async function Page(props: {
  params: Promise<{ slug?: string[] }>;
}) {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();

  const MDX = page.data.body;

  return (
    <DocsPage toc={page.data.toc}>
      <DocsTitle>{page.data.title}</DocsTitle>
      <DocsDescription>{page.data.description}</DocsDescription>
      <DocsBody>
        <MDX />
      </DocsBody>
    </DocsPage>
  );
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `fumadocs-ui/provider` import | `fumadocs-ui/provider/next` | fumadocs-ui v16 | Framework-specific providers; old path missing from exports |
| `getDocsSearchIndex()` | `createFromSource()` + `staticGET` | fumadocs-core v15/v16 | Cleaner API; handles structured data automatically |
| `createMDXSource(import('.source/index'), import('.source/meta'))` | `docs.toFumadocsSource()` | fumadocs-mdx v14 | Single method; collections pattern replaces manual imports |
| `next.config.mjs` with Fumadocs | `next.config.mjs` or `.mts` (ESM-only) | Always; now enforced | TypeScript `.ts` config requires Node native resolver workaround |
| `fumadocs-mdx@14.x` | `fumadocs-mdx@15.x` | 2026 | v15 aligns with fumadocs-core v16; v14 is incompatible |
| `export const dynamic = 'force-static'` | `export const revalidate = false` | fumadocs search docs update | Both achieve same result; fumadocs docs now show `revalidate` |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Renaming `next.config.ts` → `next.config.mjs` resolves fumadocs-mdx ESM import on Node 22 | Pattern 1 | If `.mjs` rename still fails, try `.mts` extension or add `"type": "module"` to `web/package.json` (breaking change for other tools) |
| A2 | `source.generateParams()` returns the correct shape for `[[...slug]]` catch-all (not needing `lang` key) | Pattern 6 | If build emits no docs routes, check that generateStaticParams return includes all slugs; may need `.map(page => ({ slug: page.slugs }))` |
| A3 | `search={{ options: { type: 'static' } }}` is the correct RootProvider prop in fumadocs-ui v16 | Pattern 8 | If search fails to load index, check fumadocs-ui source for actual RootProvider search prop type |
| A4 | `out/api/search/index.json` is the exact output path (not `out/api/search.json`) with `trailingSlash: true` | Static Search section | If search 404s in browser, verify the actual path in `out/` and adjust search client `from` if needed |
| A5 | No `fumadocs-ui` or `fumadocs-core` in `transpilePackages` needed | Standard Stack | If ESM import errors appear for fumadocs at build time, add `fumadocs-ui` to `transpilePackages` in next.config.mjs |

---

## Open Questions

1. **`next.config.ts` → `.mjs` rename: does it break any existing tooling?**
   - What we know: Biome, TypeScript, pnpm all work with `.mjs` files
   - What's unclear: Whether Next.js `next-env.d.ts` or any type references point specifically to `.ts` vs `.mjs`
   - Recommendation: Rename as the first step of Wave 0; run `pnpm build` with no other changes to confirm the rename alone succeeds

2. **`DocsPage toc={}` prop: must it be non-empty for TOC to render?**
   - What we know: `page.data.toc` is populated by fumadocs-mdx from the MDX headings
   - What's unclear: Whether TOC renders at all if the page has no headings (crash vs empty panel)
   - Recommendation: Use the `getting-started/index.mdx` as the multi-heading test page; verify TOC panel appears in the built `out/docs/getting-started/index.html`

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | pnpm, next build, fumadocs-mdx | ✓ | 22.17.0 | — |
| pnpm | Package installs | ✓ | 11.1.3 | — |
| Internet (npm registry) | Package install | ✓ (assumed) | — | Install from offline cache |
| `web/node_modules` | Build | ✓ | Installed (Phase 12) | `pnpm install` |

**Missing dependencies with no fallback:** none — all packages are on the registry and confirmed installable.

---

## Validation Architecture

The four success criteria can be objectively validated as follows:

### Test Framework

| Property | Value |
|----------|-------|
| Framework | No Vitest/Playwright yet (Phase 19). Phase 13 validation is `pnpm build` + filesystem + Playwright Python (available from Phase 12 smoke tests). |
| Config file | none (Phase 19 installs test runner) |
| Quick run command | `cd web && pnpm build` |
| Full validation command | `cd web && pnpm build && python <validation-script>` |

### Phase Requirements → Test Map

| SC | Behavior | Test Type | How to Verify |
|----|----------|-----------|---------------|
| SC-1: `pnpm build` emits `out/docs/` + non-empty search index | Build output inspection | Filesystem assert | `pnpm build` exits 0; `ls out/docs/<section>/index.html` for all 8 sections; `out/api/search/index.json` exists and size > 100 bytes |
| SC-2: Sidebar lists all 8 sections in exact order | DOM inspection | Playwright or manual | Open `out/docs/getting-started/index.html` in browser (or Playwright); inspect sidebar DOM for nav links in exact order: getting-started, installation, configuration, integration, security, cli-reference, audit-log, troubleshooting |
| SC-3: TOC panel renders for multi-heading page | DOM inspection | Playwright or manual | Navigate to `out/docs/getting-started/index.html`; TOC panel must contain ≥3 entries corresponding to headings in `getting-started/index.mdx` |
| SC-4: Search dialog opens, query returns result, click navigates | Interactive E2E | Playwright against static `out/` | Serve `out/` with `pnpm start` (= `pnpm dlx serve out`); Playwright: open search dialog (Ctrl+K), type "beekeeper", assert at least 1 result card appears, click first result, assert URL changed to a docs path |

### Detailed Validation Sequence (per task commit)

**Per task commit (minimum gate):**
```bash
cd web && pnpm build
# Must exit 0 with no errors
```

**After source/layout wiring (Wave 2 gate):**
```bash
cd web && pnpm build
# Then verify:
ls out/api/search/index.json
# File must exist and be > 100 bytes
ls out/docs/
# Must show: getting-started/ installation/ configuration/ integration/ security/ cli-reference/ audit-log/ troubleshooting/
```

**After seed content (Wave 3 gate — phase complete gate):**
```bash
cd web && pnpm build
# Then verify each success criterion:
# SC-1: check all 8 out/docs/<section>/index.html exist
# SC-2: Playwright sidebar order check
# SC-3: Playwright TOC check on getting-started page
# SC-4: Playwright search E2E (serve out/, open dialog, query, assert result, click)
```

### Playwright Validation Snippets (from Phase 12 smoke pattern)

```python
# SC-2: Sidebar order (run against served static out/)
from playwright.sync_api import sync_playwright
with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page()
    page.goto("http://localhost:3000/docs/getting-started/")
    # sidebar nav items in order
    nav_items = page.locator("nav a").all_text_contents()
    expected = ["Getting Started", "Installation", "Configuration", "Integration",
                "Security", "CLI Reference", "Audit Log", "Troubleshooting"]
    # assert expected subset appears in order (sidebar may include subsections)
    ...

# SC-3: TOC on getting-started page
    toc_items = page.locator("[data-toc] a, aside a").all_text_contents()
    assert len(toc_items) >= 3, f"Expected ≥3 TOC entries, got: {toc_items}"

# SC-4: Search
    page.keyboard.press("Control+k")
    page.locator("[data-search-input], input[placeholder*='search' i]").fill("beekeeper")
    results = page.locator("[data-search-result], [data-hit], [role='option']")
    results.first.wait_for(timeout=5000)
    assert results.count() >= 1
    first_href = results.first.get_attribute("href") or ""
    results.first.click()
    assert "/docs/" in page.url or page.url.endswith("/docs/")
```

### Wave 0 Gaps

- [ ] `web/source.config.ts` — required before `pnpm build` can generate `.source/`
- [ ] `web/next.config.mjs` (rename from `.ts`) — required for fumadocs-mdx ESM import
- [ ] `web/tsconfig.json` path alias update — required for `collections/server` resolution
- [ ] 8 seed MDX files — required for `source.generateParams()` to return routes and Orama to have content to index

---

## Security Domain

> security_enforcement: absent in config.json — treated as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no (docs pipeline; no user input) | — |
| V6 Cryptography | no | — |

### Known Threat Patterns for this Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Supply-chain via fumadocs packages | Tampering | Official fumadocs packages from npm; no postinstall scripts confirmed [VERIFIED] |
| MDX content injection (hypothetical) | Tampering | All MDX is hand-authored in-repo; no remote/dynamic MDX sources; fumadocs-mdx does not fetch external content |
| Search index poisoning | Tampering | Search index is generated at build time from in-repo MDX; no external data sources |
| Content accuracy (security tool docs) | Information Disclosure | Seed content is clearly labeled as placeholder; Phase 18 content review gate addresses this |

**Phase 13 security posture:** Minimal surface. This phase adds a static content pipeline with no server runtime, no user input, no auth surfaces. The primary supply-chain gate is using official fumadocs packages with confirmed-clean postinstall behavior.

---

## Sources

### Primary (HIGH confidence)
- `fumadocs.dev/docs/mdx/next` — `createMDX`, `source.config.ts`, `lib/source.ts` with `toFumadocsSource()`, `collections/server` alias [CITED]
- `fumadocs.dev/docs/headless/search/orama` — `createFromSource`, `staticGET`, `revalidate = false` [CITED]
- `fumadocs.dev/docs/ui/layouts/root-provider` — `theme={{ enabled: false }}`, provider nesting [CITED]
- `npm view fumadocs-ui version` → `16.9.3` [VERIFIED: npm registry]
- `npm view fumadocs-core version` → `16.9.3` [VERIFIED: npm registry]
- `npm view fumadocs-mdx version` → `15.0.11` [VERIFIED: npm registry]
- `npm view fumadocs-ui exports` — confirmed `./layouts/docs`, `./layouts/docs/page`, `./provider/next` paths [VERIFIED: npm registry]
- `npm pack fumadocs-ui --dry-run` — confirmed `css/shadcn.css`, `css/preset.css`, `dist/**/*.js` layout [VERIFIED: npm registry]
- `npm view fumadocs-mdx scripts.postinstall` — empty, no postinstall [VERIFIED: npm registry]

### Secondary (MEDIUM confidence)
- WebSearch: `search={{ options: { type: 'static' } }}` as the RootProvider prop — multiple sites using fumadocs static export show this pattern [MEDIUM: cross-referenced from fumadocs static export guide]
- `fumadocs.dev/docs/deploying/static` — confirms `output: 'export'` + static search setup [CITED: MEDIUM — page content was partially accessible]
- `v14.fumadocs.dev/docs/headless/search/orama` — confirms `createFromSource` + `staticGET` pattern (v14 docs) [CITED]

### Tertiary (LOW confidence — marked [ASSUMED])
- `next.config.mjs` rename resolves ESM issue on Node 22.17.0 — based on fumadocs official recommendation and Node.js native TS resolver behavior

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified on npm registry with version pinning
- API patterns: HIGH — core patterns (createFromSource, staticGET, toFumadocsSource, provider/next) verified via official docs and npm exports
- CSS glob: HIGH — confirmed via `npm pack fumadocs-ui --dry-run`
- RootProvider search prop structure: MEDIUM — `options` wrapper confirmed by multiple sources but not directly from a type definition
- `.mjs` rename strategy: LOW/MEDIUM — official recommendation; exact behavior on Node 22.17.0 not empirically tested

**Research date:** 2026-06-08
**Valid until:** 2026-09-08 (fumadocs releases frequently; verify API on any major bumps)

---

## RESEARCH COMPLETE
