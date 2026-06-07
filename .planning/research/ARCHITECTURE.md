# Architecture Research: v1.3.0 Web Presence & Documentation

**Domain:** Next.js static-export marketing + documentation site added to an existing Go security CLI monorepo
**Researched:** 2026-06-07
**Confidence:** HIGH (grounded in locked stack from STACK.md, official Next.js/Fumadocs/R3F documentation, and direct inspection of the existing Go repo structure)

---

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                     beekeeper/ (repo root)                           │
│  ┌──────────────────────────┐  ┌──────────────────────────────────┐  │
│  │  Go module (unchanged)   │  │  web/ (new Node workspace member)│  │
│  │  go.mod, cmd/, internal/ │  │  package.json + pnpm-lock.yaml   │  │
│  │  docs/ (source of truth) │  │  next.config.ts, tsconfig.json   │  │
│  │  milestones/             │  │  biome.json                      │  │
│  └──────────────────────────┘  └──────────────────────────────────┘  │
│                pnpm-workspace.yaml  (new — one entry: 'web')          │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     web/ — Next.js App Router                        │
├────────────────────────┬────────────────────────────────────────────┤
│  Route Groups          │  Content Pipeline                           │
│  ┌──────────────────┐  │  ┌──────────────────┐  ┌────────────────┐  │
│  │ (marketing)/     │  │  │ content/docs/    │  │ content/       │  │
│  │   page.tsx (/)   │  │  │   *.mdx files    │  │ changelog/     │  │
│  └──────────────────┘  │  │   meta.json      │  │   *.mdx files  │  │
│  ┌──────────────────┐  │  └────────┬─────────┘  └───────┬────────┘  │
│  │ docs/            │  │           │ fumadocs-mdx        │           │
│  │   layout.tsx     │  │           └─────────────────────┘           │
│  │   [[...slug]]/   │  │                    │                         │
│  │     page.tsx     │  │           lib/source.ts                      │
│  └──────────────────┘  │                    │ build time              │
│  ┌──────────────────┐  │           ┌────────▼──────────┐             │
│  │ changelog/       │  │           │ fumadocs-core      │             │
│  │   page.tsx       │  │           │ (page tree, TOC)   │             │
│  └──────────────────┘  │           └────────────────────┘             │
│  ┌──────────────────┐  │                                              │
│  │ api/search/      │  │  ┌─────────────────────────────────────────┐ │
│  │   route.ts       │  │  │  Design System Layer                     │ │
│  │   (force-static) │  │  │  shadcn/ui (copy-owned) + Tailwind v4   │ │
│  └──────────────────┘  │  │  next-themes + motion                    │ │
├────────────────────────┘  └─────────────────────────────────────────┘ │
│                                                                        │
│  ┌────────────────────────────────────────────────────────────────┐   │
│  │  3D Layer (client-only)                                         │   │
│  │  dynamic(() => import('./HeroCanvasInner'), { ssr: false })     │   │
│  │  React-Three-Fiber + drei + three (transpilePackages)          │   │
│  │  Suspended behind fallback; aria-hidden; reduced-motion gate   │   │
│  └────────────────────────────────────────────────────────────────┘   │
│                                                                        │
│  next build → out/ (static HTML/CSS/JS/JSON)                          │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation |
|-----------|----------------|----------------|
| `pnpm-workspace.yaml` (repo root) | Declares `web/` as a workspace member; isolates Node toolchain from Go module | New file; single entry `packages: ['web']` |
| `web/next.config.ts` | `output: 'export'`, `trailingSlash`, `images.unoptimized`, `transpilePackages` for three/r3f/drei | Static, committed; no runtime involvement |
| `web/app/layout.tsx` | Root layout: `RootProvider` (Fumadocs + static search config), `ThemeProvider` (next-themes), HTML shell | Server Component at build time |
| `web/app/(marketing)/` | Route group: marketing home and all non-docs pages; shares marketing layout (nav + footer) | Route group — no URL segment added |
| `web/app/docs/` | Fumadocs `DocsLayout` + `[[...slug]]` catch-all; sidebar, TOC, built-in search dialog | Fumadocs-managed layout |
| `web/app/changelog/` | Versioned release notes; `generateStaticParams` enumerates all MDX files at build time | Static pages from MDX |
| `web/app/api/search/route.ts` | Orama static search index: `export const dynamic = 'force-static'`; emits JSON into `out/` | Force-static Route Handler |
| `web/lib/source.ts` | fumadocs-mdx `loader()` definition; the single source-of-truth for the docs page tree | Build-time only |
| `web/content/docs/` | Authoritative MDX content; MUST reflect shipped features only | Hand-authored; sourced from `docs/` |
| `web/content/changelog/` | Per-version release notes as MDX | Hand-authored from `milestones/` archives |
| `web/components/hero/Hero3D.tsx` | `dynamic(...)` boundary; ssr:false wrapper; static SVG fallback for reduced-motion | Gateway to client-only canvas |
| `web/components/hero/HeroCanvasInner.tsx` | `'use client'`; Canvas + R3F scene; all three/r3f/drei imports live here only | Never imported by server paths |
| `web/components/marketing/` | Section components (ProblemOrigin, HowItWorks, FeatureCards, HarnessMatrix, etc.) | Server Components (no interactivity beyond motion) |
| `web/components/ui/` | shadcn/ui copy-owned components (Button, Card, Badge, etc.) | Managed by `shadcn` CLI; never edited manually |
| `web/public/` | favicon.ico, og-image.png, robots.txt, sitemap.xml (generated), logo SVG | Static assets; no build step |

---

## Recommended Project Structure

```
beekeeper/                            ← Go module root — unchanged
  go.mod
  go.sum
  pnpm-workspace.yaml                 ← NEW: packages: ['web']
  docs/
    THREAT-MODEL.md                   ← content source of truth (stay here; web/ references it)
    harness-support-matrix.md         ← content source of truth
    release-runbook.md
    nudge.md
  milestones/                         ← content source of truth for changelog
    v1.0.0-ROADMAP.md
    v1.2.0-ROADMAP.md
  web/
    package.json                      ← "packageManager": "pnpm@9.x.x"
    pnpm-lock.yaml
    next.config.ts
    tsconfig.json
    biome.json
    vitest.config.ts
    playwright.config.ts
    public/
      favicon.ico
      og-image.png                    ← 1200×630, used by metadata API
      logo.svg                        ← static SVG logo (also fallback for hero)
      hero-fallback.svg               ← reduced-motion / no-WebGL hero placeholder
      robots.txt                      ← generated or hand-authored
    app/
      globals.css                     ← @import order: tailwindcss → shadcn.css → preset.css → @theme
      layout.tsx                      ← root layout: RootProvider + ThemeProvider
      (marketing)/                    ← route group — no URL prefix
        layout.tsx                    ← marketing layout: shared Nav + Footer
        page.tsx                      ← / — home page sections assembled here
      docs/
        layout.tsx                    ← Fumadocs DocsLayout: sidebar + TOC config
        [[...slug]]/
          page.tsx                    ← dynamic MDX renderer; generateStaticParams from source
      changelog/
        page.tsx                      ← changelog index (list of versions)
        [version]/
          page.tsx                    ← per-version release notes; generateStaticParams
      api/
        search/
          route.ts                    ← force-static Orama index; GET → JSON
    components/
      ui/                             ← shadcn/ui copy-owned (CLI-generated, never hand-edited)
        button.tsx
        card.tsx
        badge.tsx
        separator.tsx
        ... (others as needed)
      hero/
        Hero3D.tsx                    ← dynamic(..., { ssr: false }) boundary + SVG fallback
        HeroCanvasInner.tsx           ← 'use client'; Canvas; R3F scene (hive)
        HiveScene.tsx                 ← three.js geometry: hexcells, agents, Beekeeper node
        AmbientHex.tsx                ← small reusable ambient canvas (used in other sections)
      marketing/
        SiteNav.tsx                   ← top navigation; shadcn NavigationMenu; dark-mode toggle
        SiteFooter.tsx                ← Apache 2.0, GitHub, SECURITY.md, docs links
        HeroSection.tsx               ← assembles Hero3D + headline + dual CTA
        ProblemOriginSection.tsx      ← Nx Console incident origin story
        HowItWorksSection.tsx         ← 3-step linear flow diagram
        FeatureCardsSection.tsx       ← 8 capability cards (shipped features only)
        HarnessMatrixSection.tsx      ← 15-harness tier table from harness-support-matrix.md
        CorroborationSection.tsx      ← 2FA-for-threat-intel explainer + AmbientHex accent
        FailClosedCallout.tsx         ← "3.58ms/op on Celeron N4020" trust signal
        HonestyBlock.tsx              ← known gaps linking to security posture docs
        SupplyChainCallout.tsx        ← SLSA L3 + Sigstore + CycloneDX badges
      docs/
        DocsCallout.tsx               ← :::note[Windows] / :::warning inline blocks
      shared/
        ThemeToggle.tsx               ← next-themes toggle button
        CopyableCodeChip.tsx          ← one-click copy for go install command
        ReducedMotionProvider.tsx     ← useReducedMotion hook; shared across hero + ambient
    content/
      docs/                           ← fumadocs-mdx sources; directory = sidebar group
        meta.json                     ← top-level sidebar order
        getting-started/
          meta.json
          quickstart.mdx
          how-it-works.mdx
        installation/
          meta.json
          go-install.mdx
          binary-download.mdx
          verification.mdx            ← cosign + SLSA verify steps from THREAT-MODEL.md §7
        configuration/
          meta.json
          layered-config.mdx
          config-reference.mdx        ← all fields, defaults, fail_mode warning
          policy-as-code.mdx          ← policy files, validate, test; v1 enforcement gaps noted
          sensitive-paths.mdx
        integration/
          meta.json
          claude-code.mdx             ← Tier 1
          codex.mdx                   ← Tier 1
          cursor.mdx                  ← Tier 1
          augment.mdx                 ← Tier 1
          codebuddy.mdx               ← Tier 1
          qwen.mdx                    ← Tier 1
          gemini-cli.mdx              ← Tier 1
          copilot.mdx                 ← Tier 1
          antigravity.mdx             ← Tier 1
          windsurf.mdx                ← Tier 1
          hermes.mdx                  ← Tier 2 (fail-OPEN warning first)
          cline.mdx                   ← Tier 2 (Windows-only caveat first)
          opencode.mdx                ← Tier 2
          kilo.mdx                    ← Tier 3 (unguarded native tools warning first)
          trae.mdx                    ← Tier 3 (unguarded native tools warning first)
          mcp-gateway.mdx             ← gateway guide (remote-bind warning prominent)
        security/
          meta.json
          corroboration-model.mdx
          fail-closed-defaults.mdx
          self-protection.mdx
          build-pipeline.mdx          ← SLSA L3, Sigstore, SBOM, reproducible builds
          known-gaps.mdx              ← honest limitations from THREAT-MODEL.md §8
        cli-reference/
          meta.json
          check.mdx
          catalogs.mdx
          hooks.mdx
          gateway.mdx
          audit.mdx
          policy.mdx
          scan.mdx
          diag.mdx
          selftest.mdx
          protect.mdx
          dashboard.mdx
          nudge.mdx
          version.mdx
        audit-log/
          meta.json
          schema.mdx
          sinks.mdx
          query-export.mdx
          redaction-scope.mdx
        troubleshooting/
          meta.json
          hook-not-firing.mdx
          catalog-stale.mdx
          self-quarantine-event.mdx
          windows-state-dir.mdx
      changelog/
        v1.0.0.mdx
        v1.2.0.mdx
        v1.3.0.mdx                    ← exit-code breaking change migration note required
    lib/
      source.ts                       ← fumadocs-mdx loader; exports `source` used by docs pages
      metadata.ts                     ← shared Next.js Metadata factory (OG, Twitter cards)
      changelog.ts                    ← helper to list + sort changelog MDX files at build time
    __tests__/
      unit/                           ← Vitest: metadata helpers, changelog sorting, lib utils
      components/                     ← Vitest + @testing-library/react: component logic
    e2e/                              ← Playwright tests against out/
      smoke.spec.ts                   ← nav, theme toggle, search, 3D canvas load
      docs.spec.ts                    ← all docs routes render; search returns results
      changelog.spec.ts               ← per-version pages render; copy commands present
```

### Structure Rationale

- **`(marketing)/` route group:** Wraps the home page (and any future marketing pages like `/about`, `/security`) in a shared marketing layout (Nav + Footer) without adding a URL segment. Fumadocs `DocsLayout` is completely separate — no shared ancestor layout conflicts.
- **`content/docs/` and `content/changelog/` stay in `web/`:** Content belongs to the web app's build, not the Go module. This avoids fumadocs-mdx having to cross workspace boundaries.
- **`docs/THREAT-MODEL.md` and `docs/harness-support-matrix.md` stay in the Go repo root `docs/`:** They are the canonical source of truth for the shipped product. The web content in `web/content/docs/` is hand-authored prose derived from these files — not symlinked or auto-imported. This is a deliberate copy-with-citation approach (explained under Content Pipeline below).
- **`components/hero/` is isolated:** All R3F imports are confined to `HeroCanvasInner.tsx` and `HiveScene.tsx`. No other component file imports from three, @react-three/fiber, or @react-three/drei. `Hero3D.tsx` is the only file that uses `next/dynamic` with `ssr: false`. This isolation is a hard rule, not a guideline.
- **`components/ui/` is never hand-edited:** shadcn CLI generates and regenerates these. Customization goes into the `@theme` block in `globals.css` or in the consumer component layer.
- **`lib/source.ts`:** Single file; all fumadocs-mdx configuration lives here. The `source` export is the only binding between the docs MDX files and the docs pages.

---

## Architectural Patterns

### Pattern 1: Static-Export Boundary Enforcement

**What:** Every page, component, and API route must be statically renderable at `next build`. No runtime server exists. This constraint is not optional — it governs every architectural decision for the web layer.

**When to use:** Always — this is the foundational constraint for the entire `web/` app.

**Trade-offs:** Eliminates server-side personalization, dynamic rewrites, and middleware entirely. For a docs + marketing site with no auth and static content, these are not real losses. The gain is zero-server hosting (Cloudflare Pages / GitHub Pages) with trivial CDN caching.

**Enforcing it:**
- Route Handlers that must emit content at build time use `export const dynamic = 'force-static'`
- `generateStaticParams` is required on every `[param]` and `[[...slug]]` dynamic route
- Zero use of `cookies()`, `headers()`, `redirect()` (runtime APIs)
- Zero Server Actions (they require a server runtime)
- `images: { unoptimized: true }` in `next.config.ts` — no `next/image` optimization server needed

```typescript
// web/next.config.ts — the complete static-export config
import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  output: 'export',
  trailingSlash: true,
  images: { unoptimized: true },
  transpilePackages: ['three', '@react-three/fiber', '@react-three/drei'],
};

export default nextConfig;
```

### Pattern 2: Client-Only 3D Isolation via dynamic() Boundary

**What:** Three.js, React-Three-Fiber, and drei use browser globals (`WebGLRenderingContext`, `window`, `requestAnimationFrame`) that do not exist in Node.js. During `next build`, Server Components run in Node — any import of R3F in a Server Component crashes the build with "ReferenceError: window is not defined."

The boundary is: a single `dynamic()` call with `ssr: false` in `Hero3D.tsx`. Everything downstream of that call (`HeroCanvasInner.tsx`, `HiveScene.tsx`, `AmbientHex.tsx`) can safely import R3F because Next.js never executes them during SSR/SSG.

**When to use:** Any component that imports from three, @react-three/fiber, or @react-three/drei. No exceptions.

**Trade-offs:** The canvas loads after the initial HTML (deferred to client hydration). The `loading` prop of `dynamic()` renders the static SVG fallback in its place until the canvas is ready — users with slow connections or no WebGL see the SVG, which is intentional and accessible.

```typescript
// web/components/hero/Hero3D.tsx
import dynamic from 'next/dynamic';

const HeroCanvas = dynamic(
  () => import('./HeroCanvasInner'),
  {
    ssr: false,
    loading: () => (
      <img
        src="/hero-fallback.svg"
        alt="Beekeeper agent-mediation diagram"
        className="w-full aspect-video object-contain"
      />
    ),
  }
);

export default function Hero3D() {
  return <HeroCanvas />;
}
```

```typescript
// web/components/hero/HeroCanvasInner.tsx — ALL R3F imports live here
'use client';

import { useReducedMotion } from '../shared/ReducedMotionProvider';
import { Canvas } from '@react-three/fiber';
import { Float, Environment } from '@react-three/drei';
import { HiveScene } from './HiveScene';

export default function HeroCanvasInner() {
  const prefersReduced = useReducedMotion();

  if (prefersReduced) {
    // Return static SVG immediately — no canvas mounted at all
    return (
      <img
        src="/hero-fallback.svg"
        alt="Beekeeper agent-mediation diagram"
        className="w-full aspect-video object-contain"
        aria-label="Hive network diagram showing agent tool calls being intercepted"
      />
    );
  }

  return (
    <Canvas
      camera={{ position: [0, 0, 8], fov: 45 }}
      aria-hidden="true"     // decorative — screen readers use the img fallback
      dpr={[1, 1.5]}          // cap pixel ratio for performance budget
      performance={{ min: 0.5 }} // R3F adaptive performance: drops to 30fps if needed
    >
      <Environment preset="city" />
      <Float speed={0.8} rotationIntensity={0.2} floatIntensity={0.3}>
        <HiveScene />
      </Float>
    </Canvas>
  );
}
```

**Reduced-motion and low-power fallbacks:** `ReducedMotionProvider` is a thin client component wrapping `window.matchMedia('(prefers-reduced-motion: reduce)')` with a React context. It is consumed by `HeroCanvasInner` and `AmbientHex`. When `true`, no canvas is mounted — the static SVG renders in its place. The `performance={{ min: 0.5 }}` prop on the Canvas triggers R3F's adaptive performance mode: if the device cannot sustain 60fps, R3F reduces the DPR and frame rate automatically down to 30fps minimum before giving up.

**Asset strategy:** The hive scene geometry is built entirely from Three.js primitives (BoxGeometry, IcosahedronGeometry, LineSegments) — no external GLTF/OBJ assets to load. This eliminates the need for asset preloading logic and keeps the initial R3F chunk self-contained. Color and material decisions use CSS variables extracted at runtime for dark/light mode coherence.

**Performance budget:** < 300KB compressed for the combined three + r3f + drei chunk (achievable by importing drei helpers individually rather than the full package). `next/dynamic` defers this chunk entirely — it does not block the initial HTML parse or LCP.

### Pattern 3: Fumadocs + shadcn + Tailwind v4 CSS Import Order

**What:** The three design systems share CSS variables via a precise import order in `globals.css`. Getting this wrong produces orphaned color tokens — Fumadocs components render in wrong colors or the dark-mode toggle has no effect on the docs section.

**When to use:** Once, at setup. The order is fixed and must not be changed.

**Trade-offs:** None — this is the documented Fumadocs integration path. The alternative (maintaining separate theme systems) multiplies the token surface area.

```css
/* web/app/globals.css — import order is load-bearing */
@import 'tailwindcss';

/* Step 1: map all --color-fd-* → shadcn CSS variables */
/* (e.g., --color-fd-background → var(--background)) */
@import 'fumadocs-ui/css/shadcn.css';

/* Step 2: Fumadocs component base styles (reads --color-fd-* set above) */
@import 'fumadocs-ui/css/preset.css';

/* Step 3: tell Tailwind v4 to scan Fumadocs dist for utility usage */
@source '../node_modules/fumadocs-ui/dist/**/*.js';

/* Step 4: shadcn @theme block — generated by `pnpm dlx shadcn@latest init` */
/* This is the ONLY place color tokens are defined */
@theme {
  --color-background: oklch(1 0 0);           /* light: white */
  --color-foreground: oklch(0.15 0 0);
  --color-primary: oklch(0.55 0.2 260);       /* Beekeeper brand: amber-adjacent */
  /* ... remainder generated by shadcn CLI ... */
}

/* Dark mode overrides */
@media (prefers-color-scheme: dark) {
  @theme {
    --color-background: oklch(0.1 0 0);
    --color-foreground: oklch(0.95 0 0);
  }
}
```

**Why shadcn.css before preset.css:** `fumadocs-ui/css/shadcn.css` aliases `--color-fd-*` → `--color-*` (shadcn tokens). If preset.css loads first, it tries to read `--color-fd-*` before the aliases exist and falls back to defaults — broken colors in production.

### Pattern 4: Fumadocs Static Search (Orama force-static Route Handler)

**What:** Fumadocs' default search uses a `GET /api/search` Route Handler that the Fumadocs `RootProvider` calls at runtime. Under `output: 'export'` there is no runtime server, so that handler is never called. The solution is `force-static`: Next.js executes the handler at build time and emits the result as a static JSON file in `out/api/search/index.json`. The browser fetches this static file instead of hitting a server.

**When to use:** Always — this is the mandatory search pattern for static export. The Fumadocs dynamic search mode is incompatible with `output: 'export'`.

**Trade-offs:** The search index is rebuilt on every `next build`. For this site (< 60 pages), the index is < 500KB. Build time overhead is negligible.

```typescript
// web/app/api/search/route.ts
import { getDocsSearchIndex } from 'fumadocs-core/search/server';
import { source } from '@/lib/source';

export const dynamic = 'force-static';

export async function GET(): Promise<Response> {
  const index = await getDocsSearchIndex(source.getPages());
  return Response.json(index);
}
```

```typescript
// web/app/layout.tsx — tell Fumadocs to use static mode
import { RootProvider } from 'fumadocs-ui/provider';
import { ThemeProvider } from 'next-themes';

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <RootProvider search={{ type: 'static' }}>
            {children}
          </RootProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
```

### Pattern 5: Changelog as Static MDX Pages with generateStaticParams

**What:** Changelog entries are individual MDX files in `web/content/changelog/`. The `[version]/page.tsx` route uses `generateStaticParams` to enumerate all versions at build time, rendering one static HTML page per version.

**When to use:** For the versioned changelog route (`/changelog/v1.0.0/`, `/changelog/v1.2.0/`, etc.).

**Trade-offs:** Adding a new release requires a new MDX file and a rebuild — this is the correct behavior. The changelog is intentionally hand-authored (not auto-generated from git log) per FEATURES.md anti-features.

```typescript
// web/lib/changelog.ts
import { readdirSync } from 'fs';
import { join } from 'path';

const changelogDir = join(process.cwd(), 'content/changelog');

export function getChangelogVersions(): string[] {
  return readdirSync(changelogDir)
    .filter(f => f.endsWith('.mdx'))
    .map(f => f.replace('.mdx', ''))
    .sort()
    .reverse(); // newest first
}

// web/app/changelog/[version]/page.tsx
import { getChangelogVersions } from '@/lib/changelog';

export function generateStaticParams() {
  return getChangelogVersions().map(version => ({ version }));
}
```

---

## Content Pipeline

### Content Source Strategy: Copy-with-Citation (not symlink/import)

The Go repo's `docs/THREAT-MODEL.md` and `docs/harness-support-matrix.md` are authoritative. The web content in `web/content/docs/` is **hand-authored prose derived from these files, not mechanically copied or symlinked**.

**Why not symlink or auto-import:**
- Fumadocs-mdx processes MDX with frontmatter schemas and link-resolution — it cannot ingest raw Go-project Markdown files unchanged.
- `docs/THREAT-MODEL.md` is a technical reference document; `web/content/docs/security/known-gaps.mdx` is user-facing documentation — different voice, different structure, different level of detail.
- Cross-workspace file reads at build time create fragile paths that break if the Go module layout changes.

**How to keep docs accurate:** Each web MDX file that derives content from a Go doc file should have a comment in its frontmatter stating the source:

```mdx
---
title: Known Gaps and Explicit Non-Defenses
description: What Beekeeper does not claim to defend against
source_doc: docs/THREAT-MODEL.md  # § 8 — keep in sync with this section
last_synced: 2026-06-07
---
```

This is a process control, not an automated one. The convention creates a clear audit trail for the content team (currently: solo developer) without requiring build-time cross-boundary imports.

### Fumadocs MDX Source Configuration

```typescript
// web/lib/source.ts
import { createMDXSource } from 'fumadocs-mdx';
import { loader } from 'fumadocs-core/source';

// createMDXSource reads from web/content/docs/ at build time
export const source = loader({
  baseUrl: '/docs',
  source: createMDXSource(
    // fumadocs-mdx generates this from content/docs/ via the next.config.ts plugin
    // Import path is generated — see fumadocs-mdx docs for exact import
    import('../.source/index'),
    import('../.source/meta'),
  ),
});
```

The `fumadocs-mdx` Next.js plugin (`withFumadocs(nextConfig)` in `next.config.ts`) scans `content/docs/**/*.mdx` at build time and generates a `.source/` directory consumed by `lib/source.ts`. This is entirely build-time — zero runtime overhead.

### Frontmatter Schema

All docs MDX files must include:

```yaml
---
title: string           # required — used in sidebar, page title, OG
description: string     # required — used in meta description, OG
source_doc: string      # optional — path to the Go-side doc being summarized
last_synced: date       # optional — when this was last checked against source_doc
---
```

Changelog MDX files use a distinct schema:

```yaml
---
version: string         # e.g. "v1.2.0"
date: date              # e.g. 2026-06-04
title: string           # e.g. "Runtime Behavioral Hardening"
breaking: boolean       # true if this version has breaking changes
security_changes: boolean  # true if this version has security-relevant changes
---
```

The `breaking: true` flag triggers a red callout block in the changelog page renderer. The `security_changes: true` flag triggers an amber callout. Both are rendered by the `[version]/page.tsx` page component reading frontmatter via `fumadocs-core` APIs.

### Sidebar Organization (meta.json)

`meta.json` files in each `content/docs/` subdirectory control the sidebar order. Top-level `content/docs/meta.json`:

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

Each subdirectory has its own `meta.json` ordering its pages. Fumadocs uses filename-based alphabetical ordering as the fallback — explicit `meta.json` ordering takes priority.

---

## Layout Composition

### Layer 1: Root Layout (`app/layout.tsx`)

The root layout is responsible for the HTML shell, theme provider, and Fumadocs `RootProvider`. It wraps every page — marketing and docs alike.

```
RootLayout
  └── html[lang="en"][suppressHydrationWarning]
        └── body
              └── ThemeProvider (next-themes, attribute="class")
                    └── RootProvider (fumadocs, search: { type: 'static' })
                          └── {children}
```

`suppressHydrationWarning` on `<html>` is required because `next-themes` adds a `class` attribute during hydration that wasn't present in the SSG-rendered HTML — Next.js would otherwise warn about a hydration mismatch.

### Layer 2: Marketing Layout (`app/(marketing)/layout.tsx`)

Applies to the home page (and future marketing pages). Renders the `SiteNav` and `SiteFooter` around the page content.

```
MarketingLayout
  └── SiteNav                    ← sticky, dark-mode aware
  └── main
        └── {children}           ← page.tsx sections
  └── SiteFooter
```

`SiteNav` uses shadcn `NavigationMenu` primitives. It includes the `ThemeToggle` component (next-themes button). The nav is a Server Component at build time — it has no interactivity except the theme toggle, which is a `'use client'` leaf component.

### Layer 3: Fumadocs DocsLayout (`app/docs/layout.tsx`)

Fumadocs supplies its own `DocsLayout` component that renders the sidebar, TOC, breadcrumb, and search dialog. The marketing `SiteNav` and `SiteFooter` do NOT wrap the docs section — Fumadocs has its own nav pattern.

```typescript
// web/app/docs/layout.tsx
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { source } from '@/lib/source';
import type { ReactNode } from 'react';

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <DocsLayout
      tree={source.pageTree}
      nav={{
        title: 'Beekeeper',
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

The `DocsLayout` is styled by Fumadocs' own CSS (loaded via `preset.css`) which reads from shadcn tokens via `shadcn.css`. This is why the import order in `globals.css` is critical.

### Shared Nav + Footer Decision

The marketing `SiteNav` and `SiteFooter` are NOT shared with the docs section. This is intentional:
- Fumadocs `DocsLayout` has its own internal navigation (sidebar, breadcrumb, prev/next links) — adding the marketing nav on top creates conflicting navigation patterns.
- The Fumadocs nav bar (configured via `DocsLayout.nav`) links back to `/` to reach the marketing site.

If a unified nav across marketing + docs becomes a requirement, Fumadocs supports a `HomeLayout` component for the nav — consult Fumadocs docs at that point.

### Theme Provider Architecture

`next-themes` provides `ThemeProvider` as a client component. It sits in the root layout's `<body>`, wrapping everything. Dark/light mode is toggled via `ThemeToggle` (a `'use client'` component using `useTheme()` from `next-themes`).

The shadcn `@theme` block in `globals.css` defines light-mode tokens at root level. Dark-mode overrides are applied via the `.dark` class that `next-themes` adds to `<html>`:

```css
/* In globals.css @theme block — generated by shadcn init */
:root {
  --background: 0 0% 100%;
  /* ... */
}

.dark {
  --background: 0 0% 3.9%;
  /* ... */
}
```

Fumadocs automatically respects this because `shadcn.css` maps its tokens to these same variables.

### Reduced-Motion Provider

```typescript
// web/components/shared/ReducedMotionProvider.tsx
'use client';

import { createContext, useContext, useEffect, useState } from 'react';

const ReducedMotionContext = createContext(false);

export function ReducedMotionProvider({ children }: { children: React.ReactNode }) {
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    setReduced(mq.matches);
    const handler = (e: MediaQueryListEvent) => setReduced(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  return (
    <ReducedMotionContext.Provider value={reduced}>
      {children}
    </ReducedMotionContext.Provider>
  );
}

export const useReducedMotion = () => useContext(ReducedMotionContext);
```

`ReducedMotionProvider` wraps only the marketing home page (inside `(marketing)/layout.tsx`), not the entire app — the docs section has no motion to gate.

---

## The 3D Integration Boundary

### Where the Canvas Mounts

The R3F `Canvas` mounts in `HeroCanvasInner.tsx`, which is dynamically imported from `Hero3D.tsx` with `ssr: false`. `Hero3D.tsx` is used inside `HeroSection.tsx` (a marketing section component). The section is assembled in `(marketing)/page.tsx`.

The mounting hierarchy:

```
(marketing)/page.tsx
  └── HeroSection.tsx                  ← Server Component at build time
        └── Hero3D.tsx                 ← dynamic() boundary — ssr: false
              └── HeroCanvasInner.tsx  ← 'use client'; Canvas mounts here
                    └── HiveScene.tsx  ← three.js geometry
```

**Ambient accents** (other sections with small Three.js canvases) follow the same pattern: a thin wrapper using `dynamic()` with `ssr: false`, and an inner component with `'use client'`. They use `AmbientHex.tsx` as the inner component. The corroboration section and harness matrix section each have one small ambient canvas. These are independent Canvas instances — they do not share a WebGL context with the hero.

### Lazy and Suspense Loading

`next/dynamic` with `ssr: false` is the Suspense boundary for the 3D canvas. The `loading` prop renders immediately (the SVG fallback) while the R3F chunk is downloading. No additional `React.Suspense` wrapper is needed around `Hero3D.tsx` — `dynamic()` is already Suspense-aware.

The `loading` component (SVG placeholder) must be sized identically to the expected canvas to prevent layout shift. Use `aspect-video` + `w-full` matching the canvas container dimensions.

### Perf Budget Approach

| Budget Item | Target | Mechanism |
|-------------|--------|-----------|
| R3F chunk (gzipped) | < 300 KB | `transpilePackages` + Turbopack tree-shaking; import drei per-component |
| Initial LCP | < 2.5s | Hero 3D is deferred; SVG fallback is inlined or tiny |
| FPS target | 60fps on mid-range GPU; 30fps floor | `dpr={[1, 1.5]}` + `performance={{ min: 0.5 }}` on Canvas |
| Frame budget per scene | < 8ms GPU time | Primitives only (no GLTF); minimal light sources |
| Ambient canvas size | < 80 KB each | Simple geometry; reuse material instances |

**Geometry strategy:** The hive scene uses:
- `IcosahedronGeometry` for the central Beekeeper node
- `BoxGeometry` (hexagonal arrangement via rotation) for individual tool-call cells
- `LineSegments` for the routing paths between agents and the central node
- `Float` + `Environment` from drei for ambiance

No textures, no GLTF, no external asset loads. All materials are `MeshStandardMaterial` or `MeshBasicMaterial` with colors sourced from CSS variables read via `getComputedStyle` on mount.

**Reduced-motion path:** When `prefers-reduced-motion: reduce` is detected by `ReducedMotionProvider`, `HeroCanvasInner` returns the static SVG immediately — no Canvas is created, no WebGL context is acquired, no R3F runtime is used. The SVG is a static asset in `public/hero-fallback.svg`.

**Low-power / no-WebGL fallback:** If `Canvas` fails to acquire a WebGL context (old device, no GPU), R3F surfaces an error. The `dynamic()` `loading` component (SVG) is already rendered — in the error case the Canvas simply doesn't appear and the SVG persists. An `ErrorBoundary` around `HeroCanvasInner` catches the WebGL context failure and renders the SVG explicitly:

```typescript
// In HeroSection.tsx
<ErrorBoundary fallback={<img src="/hero-fallback.svg" alt="..." />}>
  <Hero3D />
</ErrorBoundary>
```

---

## Build and Deploy

### pnpm Workspace Wiring

The Go module root gets a single new file:

```yaml
# beekeeper/pnpm-workspace.yaml
packages:
  - 'web'
```

**Isolation rules (non-negotiable):**
- `web/` has its own `package.json` and `pnpm-lock.yaml`
- No Node packages are hoisted to the repo root — Go tooling must not pick up Node artifacts
- `web/package.json` sets `"packageManager": "pnpm@9.x.x"` to pin via Corepack
- The Go module's `go.mod` and `go.sum` are never modified by `pnpm install`
- CI's web job installs Node and pnpm; it does not install Go tools

### CI: Separate Web Job

The web build is a standalone job in GitHub Actions. It does NOT run in the same job as the Go build.

```yaml
# .github/workflows/web.yml (new file)
name: Web

on:
  push:
    branches: [main]
    paths:
      - 'web/**'
      - 'pnpm-workspace.yaml'
  pull_request:
    paths:
      - 'web/**'
      - 'pnpm-workspace.yaml'

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'        # fumadocs-mdx requires Node 22+
      - name: Enable Corepack + install pnpm
        run: corepack enable && corepack prepare pnpm@latest --activate
      - name: Install dependencies
        run: pnpm install --frozen-lockfile
      - name: Biome lint + format check
        run: pnpm biome ci .
      - name: Type check
        run: pnpm tsc --noEmit
      - name: Unit tests (Vitest)
        run: pnpm vitest run
      - name: Build static export
        run: pnpm next build         # emits out/
      - name: Upload out/ artifact
        uses: actions/upload-artifact@v4
        with:
          name: web-static
          path: web/out/
      - name: Install Playwright browsers
        run: pnpm playwright install --with-deps chromium firefox webkit
      - name: E2E tests (Playwright)
        run: pnpm playwright test
        env:
          BASE_URL: 'file:///github/workspace/web/out'
```

**Path filter:** The web CI job triggers only when `web/**` or `pnpm-workspace.yaml` changes. Go CI triggers on `**/*.go`, `go.mod`, `go.sum`. The two jobs never share triggers or artifacts. Go CI does not need Node; web CI does not need Go.

**Playwright against static `out/`:** Playwright's `baseURL` points to the static `out/` directory served locally (via `npx serve out` or `playwright.config.ts` `webServer: { command: 'npx serve out', url: '...' }`). Tests validate: all docs routes return 200, search returns results, theme toggle works, R3F canvas loads (presence of `canvas` element), changelog pages render with correct headings.

### Static `out/` Output

`next build` with `output: 'export'` emits a fully self-contained `out/` directory:

```
web/out/
  index.html
  docs/
    getting-started/
      quickstart/
        index.html
    ...
  changelog/
    v1.0.0/
      index.html
    v1.2.0/
      index.html
  api/
    search/
      index.json           ← Orama static index (from force-static Route Handler)
  _next/
    static/
      chunks/              ← JS bundles including the lazy R3F chunk
      css/                 ← Tailwind CSS
  public assets (copied verbatim)
```

`trailingSlash: true` in `next.config.ts` ensures every page emits as `page-name/index.html` — required for correct serving without a server rewrite. Hosts that serve `index.html` for directory requests (Nginx default, Cloudflare Pages, GitHub Pages) handle this correctly.

### Deployment Options

| Host | Setup | Subdir Consideration | Cost |
|------|-------|----------------------|------|
| **Cloudflare Pages** (recommended) | Connect GitHub repo; build command: `cd web && pnpm install && pnpm next build`; output: `web/out`; Node 22 | Native `_redirects` file for any redirect rules; custom domain trivial; global CDN; 0ms cold start (no serverless) | Free tier sufficient |
| **Vercel** (viable) | Add `web/` as root directory in project settings; auto-detects Next.js; `output: 'export'` works fine | No `next/image` optimization (already `unoptimized: true`); no Server Actions (none used); no analytics by default | Free hobby tier sufficient; paid for bandwidth above 100GB |
| **GitHub Pages** | Action artifact upload → `peaceiris/actions-gh-pages` deploys `out/`; custom domain via CNAME | Serves from `username.github.io/beekeeper/` by default — requires `basePath: '/beekeeper'` in `next.config.ts` if not using a custom domain. Custom domain (`beekeeper.dev` or similar) eliminates the subdir problem | Free |

**Recommendation: Cloudflare Pages.** Reasons:
1. No subdir problem — custom domain maps to repo root, `basePath` not needed
2. `_redirects` file in `out/` handles any redirect rules without server config
3. Global CDN with no cold starts (static files only)
4. Free for open-source projects at this scale
5. Beekeeper is a security tool — Cloudflare's security posture (DDoS, WAF) is appropriate

**Subdir-root consideration:** If deployed to a subpath (e.g., GitHub Pages without a custom domain), add `basePath: '/beekeeper'` and `assetPrefix: '/beekeeper/'` to `next.config.ts`. All `next/link` and `next/image` paths resolve correctly with `basePath` set. The Orama search index at `api/search/index.json` resolves relative to `basePath` automatically.

### SEO for Static Export

Next.js static export supports the `metadata` export from page/layout files — this generates `<meta>` tags in the static HTML. No server is needed.

```typescript
// web/lib/metadata.ts
import type { Metadata } from 'next';

export function buildMetadata(overrides: Partial<Metadata>): Metadata {
  return {
    metadataBase: new URL('https://beekeeper.dev'),   // replace with real domain
    title: {
      default: 'Beekeeper — Real-time safety harness for AI coding agents',
      template: '%s | Beekeeper',
    },
    description: 'Intercepts every tool call, package install, and file access your AI agent makes — before it executes.',
    openGraph: {
      type: 'website',
      images: [{ url: '/og-image.png', width: 1200, height: 630 }],
    },
    twitter: {
      card: 'summary_large_image',
      images: ['/og-image.png'],
    },
    ...overrides,
  };
}
```

**OG images without a server:** There is no `next/og` (ImageResponse) in a static export — it requires an edge/server runtime. OG image strategy: a single static `public/og-image.png` (1200×630) is the project OG image. Per-page OG images are either:
1. All pointing to the same static image (acceptable for v1.3.0)
2. Generated at build time using a script (`node scripts/generate-og.mjs`) that uses `@vercel/og` in Node.js mode — runs as part of `next build` via a custom Next.js plugin or a `prebuild` npm script

For v1.3.0, option 1 (single static OG image) is the correct scope. Per-page OG images are a v1.x enhancement.

**sitemap.xml:** Generate via a script or the `next-sitemap` package:
```
pnpm add -D next-sitemap
```
Configure `next-sitemap.config.js` with `siteUrl` and `outDir: 'out'`. Run as `postbuild` in `package.json`:
```json
{
  "scripts": {
    "build": "next build",
    "postbuild": "next-sitemap"
  }
}
```
This emits `out/sitemap.xml` and `out/robots.txt` as part of the static output.

**robots.txt:** Either a hand-authored static file in `public/robots.txt` or generated by `next-sitemap`. Content:
```
User-agent: *
Allow: /
Sitemap: https://beekeeper.dev/sitemap.xml
```

---

## Data Flow

### Build-Time Content Pipeline

```
web/content/docs/**/*.mdx
         |
         | fumadocs-mdx Next.js plugin (runs during next build)
         v
web/.source/index.ts + meta.ts   (generated; in .gitignore)
         |
         | imported by
         v
web/lib/source.ts → source (page tree, page loader)
         |
         ├─→ app/docs/[[...slug]]/page.tsx → generateStaticParams → static HTML per page
         |
         └─→ app/api/search/route.ts (force-static) → out/api/search/index.json
```

```
web/content/changelog/**/*.mdx
         |
         | lib/changelog.ts (filesystem read at build time)
         v
web/app/changelog/[version]/page.tsx → generateStaticParams → static HTML per version
```

### Runtime Data Flow (browser)

```
Browser fetches /docs/getting-started/quickstart/
         |
         v
index.html (pre-rendered static HTML, full content)
         |
         v
_next/static/chunks/*.js (React hydration — enhances interactivity only)
         |
         ├─→ theme toggle (next-themes: reads localStorage)
         ├─→ search (Fumadocs dialog: fetches /api/search/index.json once, queries in-browser)
         └─→ hero canvas (lazy: fetches R3F chunk → mounts WebGL canvas)
```

There are zero API calls to any Beekeeper backend. All interactivity is client-side. The only network request beyond the initial HTML/CSS/JS is the Orama search index fetch (once per session, cached).

---

## Scaling Considerations

This is a static site — traditional scaling concerns do not apply. The "scaling" dimension is content volume and build time.

| Scale | Approach |
|-------|----------|
| < 60 pages (v1.3.0 launch) | Current architecture; build time < 30s; Orama index < 500KB |
| 60–200 pages | Same architecture; build time 30–90s; Orama index 500KB–2MB; still acceptable |
| 200+ pages | Consider Orama Cloud or Algolia for search (index download becomes noticeable); evaluate ISR via a hosting adapter if content updates need to avoid full rebuilds |
| Multi-language | Locale-prefixed routes + multiple Orama indexes; significant route complexity; defer until warranted |

The site's limiting factor is build time (MDX compilation), not traffic (static files on CDN). At 60 pages, `next build` runs in under 30 seconds on a CI runner. No scaling action is needed for v1.3.0.

---

## Anti-Patterns

### Anti-Pattern 1: Importing R3F in a Server Component

**What people do:** Import `{ Canvas }` from `@react-three/fiber` in a file that doesn't have `'use client'`, or in the same file as the `dynamic()` call.

**Why it's wrong:** Next.js pre-renders Server Components in Node.js at build time. Three.js accesses `window`, `WebGLRenderingContext`, and `requestAnimationFrame` on import — these don't exist in Node. The build crashes with "ReferenceError: window is not defined".

**Do this instead:** All R3F/three/drei imports live in `HeroCanvasInner.tsx` and `HiveScene.tsx` only. Both files have `'use client'` as their first line. `Hero3D.tsx` uses `dynamic()` with `ssr: false` — it never imports R3F itself.

### Anti-Pattern 2: Reversed CSS Import Order (Fumadocs before shadcn mapping)

**What people do:** Follow a tutorial that puts `fumadocs-ui/css/preset.css` before `fumadocs-ui/css/shadcn.css` in `globals.css`.

**Why it's wrong:** `preset.css` reads `--color-fd-*` variables. `shadcn.css` maps those variables to shadcn tokens. If preset loads first, the variables are unresolved — Fumadocs components render in wrong colors (typically near-black or near-white in both dark and light mode).

**Do this instead:** `@import 'fumadocs-ui/css/shadcn.css'` always before `@import 'fumadocs-ui/css/preset.css'`. This is a load-bearing constraint.

### Anti-Pattern 3: Using Server-Side Fumadocs Search

**What people do:** Skip `export const dynamic = 'force-static'` in `app/api/search/route.ts` and configure `RootProvider` with `search={{ type: 'fetch' }}` (or leave it as the default).

**Why it's wrong:** The search Route Handler runs at request time by default. Under `output: 'export'`, Next.js does not generate a static file for this handler — it simply omits it. At runtime (in the browser), the Fumadocs search dialog calls `/api/search` and gets a 404. Search is broken.

**Do this instead:** `export const dynamic = 'force-static'` in the route, and `search={{ type: 'static' }}` in `RootProvider`. Verified pattern from STACK.md §Integration Points.

### Anti-Pattern 4: Aspirational Documentation

**What people do:** Write docs for features that are declared in policy files but not yet enforced in v1 — specifically `release_age` and `lifecycle_script_allowlist`.

**Why it's wrong:** For a security tool, documenting an unenforced feature as if it works creates false confidence. A user who configures `release_age: "24h"` in their policy file based on the docs and believes installs are being age-gated is misconfigured and vulnerable.

**Do this instead:** The policy-as-code guide explicitly notes which fields are enforced and which are planned: "Note: `release_age` and `lifecycle_script_allowlist` are recognized in policy files but are not enforced in v1.3.0. Do not rely on these fields for security enforcement." Sourced from FEATURES.md anti-features and THREAT-MODEL.md §8.

### Anti-Pattern 5: Committing `.source/` (fumadocs-mdx generated directory)

**What people do:** Forget to add `.source/` to `.gitignore` after running `next build`, then commit the generated type files.

**Why it's wrong:** `.source/` is regenerated on every build. Committing it creates noisy diffs (every content change regenerates it), merge conflicts, and can mask content changes in PR reviews.

**Do this instead:** Add `web/.source/` to the repo root `.gitignore`. The file is generated by `fumadocs-mdx` during `next build` and is never needed in version control.

### Anti-Pattern 6: Symlinks from `web/content/` to `docs/`

**What people do:** Symlink `web/content/docs/security/threat-model.mdx` → `../../docs/THREAT-MODEL.md` to avoid maintaining two copies of the content.

**Why it's wrong:** The Go-side `docs/THREAT-MODEL.md` is a technical reference document with Go-centric formatting (inline code refs like `internal/policy/path.go:76-107`). Fumadocs-mdx must process MDX with specific frontmatter schemas — a raw Markdown file without a `title` frontmatter field will fail the build or render incorrectly. Symlinks also break on some Windows CI configurations.

**Do this instead:** Hand-authored prose in `web/content/docs/security/known-gaps.mdx` with a `source_doc: docs/THREAT-MODEL.md` comment convention (see Content Pipeline above). The web docs are a user-facing derivative, not a raw import of the technical reference.

---

## Build Order: Dependency-Ordered

The following order respects hard dependencies between components. Items within the same phase can be parallelized.

### Phase 1: Scaffolding and Config (no content, no components)

These must exist before anything else can be built. No meaningful testing is possible yet.

1. **`pnpm-workspace.yaml`** at repo root — enables `pnpm install` to work from the workspace
2. **`web/` directory scaffold** — `pnpm dlx create-next-app@latest . --typescript --tailwind --app --use-pnpm` inside `web/`
3. **`web/next.config.ts`** — add `output: 'export'`, `trailingSlash`, `images.unoptimized`, `transpilePackages`
4. **Remove default ESLint** — `pnpm remove eslint eslint-config-next`
5. **Install all production + dev dependencies** — Fumadocs, shadcn CLI, three/r3f/drei, motion, next-themes, Biome, Vitest, Playwright (single `pnpm add` pass per STACK.md)
6. **`web/biome.json`** — Biome configuration (extends recommended, custom import order)
7. **`web/tsconfig.json`** — verify path aliases (`@/*` → `./`)
8. **`web/.gitignore`** — add `out/`, `.source/`, `.next/`, `node_modules/`

**Dependency:** Steps 2–8 require step 1. Steps 3–8 can be parallelized after step 2.

### Phase 2: Design System (CSS + shadcn + theme)

These must exist before any component can be built correctly.

9. **Run `pnpm dlx shadcn@latest init`** — generates `globals.css` with `@theme` block and `components/ui/` primitives
10. **Edit `globals.css`** — set the correct import order (tailwindcss → shadcn.css → preset.css → @source → @theme)
11. **Install Fumadocs** — `pnpm add fumadocs-ui fumadocs-core fumadocs-mdx @orama/orama`
12. **`web/app/layout.tsx`** — root layout with `RootProvider` (search: static) + `ThemeProvider`
13. **Add shadcn components** — `pnpm dlx shadcn@latest add button card badge separator navigation-menu` (add as needed)

**Verification gate:** `pnpm next build` must succeed with the empty app (no content yet). If it fails here, the CSS import order or RootProvider config is wrong — fix before proceeding.

**Dependency:** Step 10 requires step 9. Step 11 requires step 10. Steps 12–13 require step 11.

### Phase 3: Content Pipeline (fumadocs-mdx wiring)

These must exist before docs pages can render.

14. **`web/content/docs/`** — create directory structure and stub `meta.json` files for all 8 top-level groups
15. **`web/lib/source.ts`** — fumadocs-mdx `loader()` definition
16. **`web/app/api/search/route.ts`** — force-static Orama index
17. **`web/app/docs/layout.tsx`** — Fumadocs `DocsLayout`
18. **`web/app/docs/[[...slug]]/page.tsx`** — MDX renderer with `generateStaticParams`
19. **Write first docs MDX** — at minimum `getting-started/quickstart.mdx` with correct frontmatter (validates the pipeline end-to-end)

**Verification gate:** `pnpm next build` must succeed and `out/docs/getting-started/quickstart/index.html` must exist. The Orama index `out/api/search/index.json` must be non-empty.

**Dependency:** Step 15 requires step 14. Steps 16–18 require step 15. Step 19 requires step 18.

### Phase 4: Changelog Pipeline

20. **`web/content/changelog/`** — stub `v1.0.0.mdx`, `v1.2.0.mdx`, `v1.3.0.mdx` with correct frontmatter
21. **`web/lib/changelog.ts`** — filesystem enumeration helper
22. **`web/app/changelog/page.tsx`** — index page (list of versions)
23. **`web/app/changelog/[version]/page.tsx`** — per-version renderer with `generateStaticParams`

**Dependency:** Steps 21–23 require step 20. Step 23 requires step 21.

### Phase 5: Marketing Sections (no R3F yet)

Build all marketing sections as Server Components first. The hero uses the static SVG fallback — no R3F yet.

24. **`web/app/(marketing)/layout.tsx`** — marketing layout with `SiteNav` + `SiteFooter`
25. **`web/components/shared/ReducedMotionProvider.tsx`** — needed by hero (even with static fallback)
26. **`web/components/marketing/SiteNav.tsx`** — shadcn NavigationMenu + ThemeToggle
27. **`web/components/marketing/SiteFooter.tsx`**
28. **`web/public/og-image.png`** — 1200×630 static OG image (needed before any page metadata is meaningful)
29. **`web/public/hero-fallback.svg`** — static hero diagram (the fallback AND the reduced-motion path)
30. **`web/components/marketing/HeroSection.tsx`** — with static SVG only (Hero3D not yet wired)
31. **All other marketing section components** — ProblemOriginSection, HowItWorksSection, FeatureCardsSection, HarnessMatrixSection, CorroborationSection, FailClosedCallout, HonestyBlock, SupplyChainCallout
32. **`web/app/(marketing)/page.tsx`** — assembles all sections; `web/lib/metadata.ts` for OG

**Verification gate:** `pnpm next build` succeeds and the marketing home renders correctly with static SVG hero. `out/index.html` is correct.

**Dependency:** Step 24 requires steps 12–13 (layout). Steps 26–32 require step 24. Step 30 requires step 29.

### Phase 6: 3D Layer (R3F)

R3F is added last because it is a pure enhancement. All pages must already build and pass tests without it.

33. **`web/public/hero-fallback.svg`** — already done in Phase 5; verify quality
34. **`web/components/hero/HeroCanvasInner.tsx`** — `'use client'`; Canvas; ReducedMotion gate
35. **`web/components/hero/HiveScene.tsx`** — three.js geometry (primitives only)
36. **`web/components/hero/Hero3D.tsx`** — `dynamic()` boundary; import `HeroCanvasInner`; SVG fallback in `loading`
37. **Update `HeroSection.tsx`** — replace static SVG with `<Hero3D />`
38. **`web/components/hero/AmbientHex.tsx`** — small ambient canvas for non-hero sections
39. **Wire AmbientHex** into CorroborationSection and HarnessMatrixSection

**Verification gate:** `pnpm next build` still succeeds (R3F must not appear in server-side rendered output). Playwright E2E: `<canvas>` element present in rendered home page HTML (after JS hydration). DevTools: R3F chunk is lazy-loaded (not in initial bundle). Lighthouse: LCP is not blocked by the canvas.

**Dependency:** Steps 34–36 require step 33. Step 37 requires step 36. Steps 38–39 require step 36.

### Phase 7: SEO and Static Assets

40. **`next-sitemap` config** — install `next-sitemap`, configure `postbuild` script
41. **`web/public/robots.txt`** — or generated via `next-sitemap`
42. **`web/lib/metadata.ts`** — finalize `buildMetadata` with real domain
43. **Per-page metadata exports** — verify all docs and changelog pages export metadata

### Phase 8: Full Content Authoring

With the pipeline verified end-to-end, write all remaining MDX content. This is parallelizable across all doc sections because the infrastructure exists.

44. **All docs MDX pages** — all sections from the content tree; sourced from `docs/THREAT-MODEL.md`, `docs/harness-support-matrix.md`, real CLI flags
45. **Changelog MDX pages** — v1.0.0, v1.2.0, v1.3.0 prose; security-changes callouts; v1.3.0 exit-code migration note
46. **shadcn components add** — any additional components needed as content is authored (Dialog, Tooltip, etc.)

### Phase 9: Test Suite and CI

47. **Vitest unit tests** — `lib/changelog.ts` sorting, `lib/metadata.ts` output, any utility functions
48. **Playwright E2E tests** — smoke (nav + theme), search, R3F canvas load, changelog routes
49. **`.github/workflows/web.yml`** — CI job (Biome + tsc + vitest + build + playwright)

---

## Integration Points

### shadcn ↔ Fumadocs ↔ Tailwind v4

| Boundary | Integration | Critical Note |
|----------|-------------|---------------|
| Tailwind v4 → shadcn | `shadcn init` auto-detects v4; generates `@theme` block in `globals.css` | Do NOT create `tailwind.config.js` — v4 is CSS-first |
| shadcn → Fumadocs | `fumadocs-ui/css/shadcn.css` maps `--color-fd-*` → shadcn tokens | Must be imported BEFORE `preset.css` |
| Fumadocs → Tailwind v4 | `@source '../node_modules/fumadocs-ui/dist/**/*.js'` scans Fumadocs for utilities | Without this, Fumadocs utility classes are purged by Tailwind |
| three/R3F/drei → Next.js | `transpilePackages` in `next.config.ts` | Without this, ESM-only imports fail at build time |
| next-themes → shadcn | `ThemeProvider attribute="class"` adds `.dark` to `<html>` | shadcn `@theme` uses `.dark` selector for dark-mode overrides |

### Go docs/ ↔ web/content/docs/

| Boundary | Integration | Note |
|----------|-------------|------|
| `docs/THREAT-MODEL.md` → `web/content/docs/security/*.mdx` | Process convention: `source_doc` frontmatter comment | Not automated; manual sync |
| `docs/harness-support-matrix.md` → `web/components/marketing/HarnessMatrixSection.tsx` | Hand-authored component with data from the source doc | Static data, not imported at build time |
| `milestones/v*.md` → `web/content/changelog/*.mdx` | Hand-authored changelog MDX; sourced from milestone archives | Not automated |

### Static Export ↔ Deployment Host

| Boundary | Integration | Note |
|----------|-------------|------|
| `out/` → Cloudflare Pages | `_redirects` file in `out/` for any redirect rules | `trailingSlash: true` handles directory routing |
| `out/api/search/index.json` → browser | Static file served as JSON; Fumadocs fetches it once on first search | No server required; cached at CDN edge |
| `out/sitemap.xml` → search engines | Generated by `next-sitemap` postbuild script | Submit to Google Search Console and Bing Webmaster |

---

## Sources

- [Next.js Static Exports Guide](https://nextjs.org/docs/app/guides/static-exports) — verified unsupported features, `output: 'export'` config; March 2026
- [Fumadocs Static Search (Orama)](https://www.fumadocs.dev/docs/search/orama) — `force-static` Route Handler pattern; `type: 'static'` in RootProvider
- [Fumadocs Theme Docs](https://www.fumadocs.dev/docs/ui/theme) — `shadcn.css` → `preset.css` import order; `@source` requirement
- [shadcn/ui Next.js Installation](https://ui.shadcn.com/docs/installation/next) — `shadcn init` auto-detects Tailwind v4
- [R3F Installation Guide](https://r3f.docs.pmnd.rs/getting-started/installation) — `transpilePackages` requirement; `dynamic({ ssr: false })` pattern
- [Fumadocs DocsLayout API](https://www.fumadocs.dev/docs/ui/layouts/docs) — `pageTree`, `nav`, `sidebar` props
- [next-themes](https://github.com/pacocoursey/next-themes) — `attribute="class"`, `suppressHydrationWarning` requirement
- `.planning/research/STACK.md` — locked stack, integration point details, version matrix (this research)
- `.planning/research/FEATURES.md` — surface IA, feature dependencies, anti-features (this research)
- `docs/THREAT-MODEL.md` — security posture content authority
- `docs/harness-support-matrix.md` — 15-harness tier structure

---

*Architecture research for: Beekeeper v1.3.0 Web Presence & Documentation — Next.js static-export site under `web/`*
*Researched: 2026-06-07*
