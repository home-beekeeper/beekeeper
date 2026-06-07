# Stack Research

**Domain:** Go security daemon / agent runtime safety harness (v1.0.0 + v1.2.0 — archived research below)
**Domain (this addendum):** Next.js marketing + documentation website (v1.3.0 Web Presence)
**Researched:** 2026-06-07 (v1.3.0 addendum); 2026-05-26 / 2026-06-03 (prior Go research)
**Confidence:** HIGH (core stack verified against official docs and npm; version numbers cross-checked via WebSearch against live npm data as of 2026-06-07)

---

# v1.3.0 Web Presence — Next.js Stack

**Scope:** Greenfield `web/` subdirectory only. Do NOT re-litigate the Go binary stack. The `web/` app is isolated from the Go module under its own `package.json` and `pnpm-lock.yaml`.

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Next.js | 16.2.7 (current stable LTS) | App Router SSG framework, `output: 'export'` | Greenfield baseline as of June 2026. Turbopack is default (2–5x faster builds). React 19.2 built-in. Static export fully supported and actively maintained (docs updated March 2026). Node 20.9+ minimum. |
| React | 19.2 (via `react@latest`) | UI rendering; Server Components run at build time under static export | Next.js 16 hard minimum. Pairs with R3F v9. View Transitions API and Activity components available. |
| TypeScript | 5.x (Next.js 16 minimum: 5.1) | Type safety throughout | Mandatory for shadcn/ui and Fumadocs typings; Next.js 16 is TypeScript-first by default. `strict: true`. |
| Tailwind CSS | 4.3.0 (latest stable, released May 8 2026) | Utility-first styling, CSS-variable-driven theming | Fumadocs v16 requires Tailwind v4 (dropped v3 support in Fumadocs v15). CSS-first config (`@theme` directive, no `tailwind.config.js`). Incremental builds 100x faster than v3. |
| shadcn/ui | CLI-managed (no lockable package version) | Design-system component primitives | Copy-owned components — no runtime version dependency. Full Tailwind v4 + React 19 support shipped February 2025. Canonical component set matching the Beekeeper UI-SPEC. |

### Documentation Layer

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| fumadocs-ui | 16.9.3 | Docs layout shell, sidebar, TOC, built-in search dialog | Always — renders the docs section. |
| fumadocs-core | 16.9.3 | Headless search engine, content-source adapters, link utilities | Always — fumadocs-ui depends on it. |
| fumadocs-mdx | 14.2.11 | Build-time MDX compilation + file-system source provider | Always for local `.mdx` content files. Zero runtime overhead — runs only during `next build`. |
| @orama/orama | latest (^3.x) | Client-side full-text search, static JSON index mode | Use **static mode** for `output: 'export'`. Index JSON emitted at build time; downloaded and queried entirely in the browser. No server required. Fits this site scale (< 50 pages). |

### 3D and Animation

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| three | 0.184.0 (latest; R3F peer dep `>=0.156`) | 3D scene graph, WebGL renderer | Always — the underlying engine. Pin `^0.184.0`. |
| @react-three/fiber | 9.6.1 | React declarative renderer for Three.js | Always for hero canvas and ambient 3D accents. React peer dep `>=19 <19.3`. Must load client-side only via `next/dynamic` with `{ ssr: false }`. |
| @react-three/drei | 10.7.7 | R3F helper components: OrbitControls, Environment, Float, Text, etc. | Use for standard 3D elements. Import per-component, not as wildcard, to keep client bundle lean. |
| motion | 12.40.0 (ex–Framer Motion) | Ambient CSS/JS animation: scroll-triggered, hover, entrance effects | Use for non-3D ambient motion only. Tree-shakable; use `m` component + `LazyMotion` to keep contribution < 5 KB for simple animations. Do NOT use for anything Three.js handles (camera movement, particles). |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| lucide-react | 1.17.0 | Icon set (1,655 SVG icons) | Always — shadcn/ui's default icon dependency. Import per-icon for tree-shaking. |
| next-themes | ^0.4.x | Dark/light mode theme toggle | Needed for shadcn color-scheme switching. Wraps the app in a thin `ThemeProvider` client component. |
| clsx | ^2.x | Conditional class merging | Always — used in shadcn's `cn()` utility. |
| tailwind-merge | ^3.x | Tailwind class conflict resolution | Always — the other half of the `cn()` utility. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| pnpm 9.x | Package manager | Beekeeper nudges pnpm/bun. pnpm has strongest monorepo isolation. Use Corepack: `corepack enable && corepack prepare pnpm@latest --activate`. `pnpm-workspace.yaml` at repo root isolates `web/` from the Go module. |
| Biome | Linting + formatting (replaces ESLint + Prettier) | Single Rust binary, 10–25x faster than ESLint+Prettier. 200+ lint rules + 97%-Prettier-compatible formatter. Next.js 16 removed `next lint` — Biome fills the gap cleanly on a new project. |
| Vitest + @testing-library/react | Unit and component tests | Vite-native, native ESM, near-instant cold starts. Use for utility functions, MDX processing helpers, component logic. |
| Playwright | E2E / smoke tests | Validate static `out/` directory: search, nav, theme toggle, R3F canvas load across Chrome/Firefox/Safari. |

---

## Static Export Critical Config

`output: 'export'` removes the Next.js server runtime entirely. Every page must be statically renderable at build time.

```typescript
// web/next.config.ts
const nextConfig = {
  output: 'export',           // emit static HTML/CSS/JS into out/
  trailingSlash: true,        // required for most static hosts (Nginx, CF Pages, GitHub Pages)
  images: {
    unoptimized: true,        // skip built-in optimizer (needs server); serve images as-is
    // Alternative: custom loader pointing to Cloudinary/imgix for responsive images
  },
  transpilePackages: [
    'three',
    '@react-three/fiber',
    '@react-three/drei',
  ],
  // Turbopack is the default bundler in Next.js 16 — no extra config needed
};

export default nextConfig;
```

`transpilePackages` is required. Three.js, R3F, and drei ship ESM-only modules that Next.js/Turbopack must transform during the Node.js build. Without this, you get import errors at build time.

`images.unoptimized: true` is the zero-friction path for a static export. A marketing/docs site with a handful of images is fine with raw serving. If responsive images or CDN optimization are later needed, swap to a `loaderFile` pointing at Cloudinary/imgix.

### What Static Export Cannot Do — Hard Constraints

| Unsupported Feature | Impact on this Site | Mitigation |
|---------------------|--------------------|----|
| Server Actions | None needed (no forms with server mutations) | Client-side logic only |
| Dynamic routes without `generateStaticParams` | Changelog `[version]` pages need it | Enumerate versions explicitly at build time — straightforward |
| Cookies / Headers APIs at runtime | None needed (no auth, no personalization) | Not applicable |
| Rewrites / Redirects in `next.config` | Cannot use config-level redirects | Handle at host: Nginx `rewrite`, CF Pages `_redirects` |
| ISR / `revalidate` | None needed (content is MDX, rebuild on change) | CI triggers rebuild on content push |
| Server-side search API route | Fumadocs default uses a `GET /api/search` API route | Use Orama **static mode** — index pre-built at build time |
| Default `next/image` optimization | Requires a running server | Use `unoptimized: true` or a CDN custom loader |
| `middleware.ts` (deprecated) / `proxy.ts` | Not needed for a docs/marketing site | Not applicable |

---

## Integration Points

### shadcn + Fumadocs + Tailwind v4 Coexistence

This is the most error-prone part of the setup. Follow this exact sequence.

**Step 1 — Initialize Next.js and Tailwind v4:**

```bash
pnpm dlx create-next-app@latest . --typescript --tailwind --app --use-pnpm
```

`create-next-app` in Next.js 16 generates a Tailwind v4 config by default (CSS-first, no `tailwind.config.js`).

**Step 2 — Run `shadcn init`:**

```bash
pnpm dlx shadcn@latest init
```

The CLI auto-detects Tailwind v4 and writes CSS variables using the `@theme` syntax into `globals.css`.

**Step 3 — Install Fumadocs:**

```bash
pnpm add fumadocs-ui fumadocs-core fumadocs-mdx
```

**Step 4 — Set the CSS import order in `app/globals.css`:**

```css
@import 'tailwindcss';
@import 'fumadocs-ui/css/shadcn.css';    /* maps all --color-fd-* → shadcn color tokens */
@import 'fumadocs-ui/css/preset.css';    /* Fumadocs component base styles */

/* Tailwind v4 must scan Fumadocs source for its own utility usage */
@source '../node_modules/fumadocs-ui/dist/**/*.js';

/* Your shadcn @theme block goes here (generated by shadcn init) */
@theme { ... }
```

**Why this order matters:** `fumadocs-ui/css/shadcn.css` remaps all `--color-fd-*` variables to read from shadcn's existing CSS variables (`--background`, `--foreground`, `--primary`, etc.). One source of truth: the shadcn theme. Fumadocs styles automatically adopt your colors. If you reverse the order (Fumadocs preset before shadcn mapping), you get orphaned variables and broken Fumadocs colors.

**CSS variable conflict guard (optional):** If Fumadocs component colors bleed into the marketing pages (or vice versa), add a prefix to Fumadocs variables. This is rarely needed for a unified single-domain site because `shadcn.css` is explicitly designed for coexistence, but it is the escape hatch if visual conflicts appear.

### R3F Under `output: 'export'`

Three.js and R3F use browser APIs (`WebGLRenderingContext`, `window`, `requestAnimationFrame`) that crash in Node.js during `next build`. The mandatory pattern:

```typescript
// components/hero/Hero3D.tsx — never import R3F at module scope
import dynamic from 'next/dynamic';

const HeroCanvas = dynamic(
  () => import('./HeroCanvasInner'),
  {
    ssr: false,         // CRITICAL — prevents Node-side execution during static build
    loading: () => (
      <div className="hero-placeholder aspect-video w-full" aria-hidden="true" />
    ),
  }
);

export default function Hero3D() {
  return <HeroCanvas />;
}
```

```typescript
// components/hero/HeroCanvasInner.tsx — R3F lives entirely in this file
'use client';

import { Canvas } from '@react-three/fiber';
import { Environment, Float } from '@react-three/drei';

export default function HeroCanvasInner() {
  return (
    <Canvas camera={{ position: [0, 0, 5], fov: 50 }}>
      <Environment preset="city" />
      <Float speed={1.5} rotationIntensity={0.4}>
        {/* your hive / agent-mediation scene geometry */}
      </Float>
    </Canvas>
  );
}
```

Do NOT import R3F in Server Components. Do NOT use it in any file without the `'use client'` directive. The `dynamic(..., { ssr: false })` boundary is the only safe integration point.

### Fumadocs Static Search (Orama Static Mode)

Fumadocs' default search implementation uses a `GET /api/search` Route Handler that requires a running server — incompatible with `output: 'export'`. Switch to static mode:

**Route Handler (generates a static JSON index at build time):**

```typescript
// app/api/search/route.ts
import { getDocsSearchIndex } from 'fumadocs-core/search/server';
import { source } from '@/lib/source';

export const dynamic = 'force-static';   // emit as a static file in out/

export async function GET() {
  const index = await getDocsSearchIndex(source.getPages());
  return Response.json(index);
}
```

**RootProvider configuration (tells the search dialog to use the static JSON):**

```typescript
// app/layout.tsx
import { RootProvider } from 'fumadocs-ui/provider';

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <RootProvider search={{ type: 'static' }}>
          {children}
        </RootProvider>
      </body>
    </html>
  );
}
```

**Size consideration:** For this site (< 50 pages: getting started, install, config, security posture, CLI reference, changelog pages), the Orama static index will be well under 500 KB. Entirely acceptable. If the docs grow past ~200 pages, consider Orama Cloud or Algolia.

---

## Installation

```bash
# In the web/ subdirectory — isolated from Go module root
cd web

# 1. Bootstrap Next.js 16 with App Router + TypeScript + Tailwind v4
pnpm dlx create-next-app@latest . --typescript --tailwind --app --use-pnpm

# 2. Remove default ESLint (Biome replaces it; Next.js 16 removed next lint)
pnpm remove eslint eslint-config-next

# 3. Add Biome for linting + formatting
pnpm add -D @biomejs/biome
pnpm biome init

# 4. shadcn/ui — auto-detects Tailwind v4
pnpm dlx shadcn@latest init

# 5. Fumadocs
pnpm add fumadocs-ui fumadocs-core fumadocs-mdx

# 6. Fumadocs Orama static search
pnpm add @orama/orama

# 7. Three.js + R3F ecosystem
pnpm add three @react-three/fiber @react-three/drei

# 8. Animation
pnpm add motion

# 9. Supporting utilities (may already be pulled in by shadcn)
pnpm add next-themes clsx tailwind-merge lucide-react

# 10. Testing
pnpm add -D vitest @vitejs/plugin-react @testing-library/react @testing-library/dom
pnpm add -D @playwright/test
```

---

## Repository Layout

```
beekeeper/                   ← Go module root — untouched
  go.mod
  pnpm-workspace.yaml        ← add 'web' entry: packages: ['web']
  web/                       ← isolated Node app
    package.json             ← "packageManager": "pnpm@9.x.x"
    pnpm-lock.yaml
    next.config.ts
    biome.json
    tsconfig.json
    public/                  ← favicon.ico, og-image.png, etc.
    app/
      globals.css            ← @import order matters (see Integration Points)
      layout.tsx             ← RootProvider (Fumadocs) + ThemeProvider (next-themes)
      page.tsx               ← marketing home (Three.js hero via dynamic import)
      docs/
        layout.tsx           ← Fumadocs DocsLayout + Sidebar
        [[...slug]]/
          page.tsx           ← dynamic MDX page rendering
      changelog/
        [version]/
          page.tsx           ← generateStaticParams enumerates versions
      api/
        search/
          route.ts           ← force-static Orama index (emitted as JSON)
    components/
      hero/
        Hero3D.tsx           ← dynamic(..., { ssr: false }) boundary
        HeroCanvasInner.tsx  ← 'use client' + Canvas + R3F scene
    content/
      docs/                  ← *.mdx (getting-started, install, config, security, CLI)
      changelog/             ← *.mdx per version (v1.0.0.mdx, v1.2.0.mdx, ...)
    lib/
      source.ts              ← fumadocs-mdx source definition
```

**Isolation rule:** `web/` has its own `package.json` and `pnpm-lock.yaml`. The Go module at repo root (`go.mod`) is independent. Never hoist Node packages to the repo root — Go tooling must not pick up Node artifacts.

---

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| Next.js 16 `output: 'export'` | Astro | Excellent for static docs sites but milestone spec locks Next.js. Astro would be a valid greenfield alternative. |
| Fumadocs | Nextra | Nextra 3 supports static export and shadcn. Fumadocs has better Tailwind v4 native integration and more active maintenance cadence in 2025–2026. Nextra 2 is effectively unmaintained. |
| Orama static mode | Algolia DocSearch | Algolia requires a server-side crawler + API key even for static sites. Free tier requires open-source approval process. Orama is self-hosted, zero API key, zero rate limits. |
| Orama static mode | FlexSearch (older Fumadocs option) | FlexSearch is the older Fumadocs search integration; Orama is the current default and better maintained. |
| motion (ex–Framer Motion) | GSAP | GSAP's core is MIT but many plugins (ScrollTrigger, etc.) require a paid license for commercial use. Motion is Apache-2.0, React-native, tree-shakable. Sufficient for ambient accent animations. |
| @react-three/fiber + drei | Vanilla Three.js | R3F's declarative JSX model integrates cleanly with React lifecycle. Vanilla Three.js in React leads to manual `useRef` juggling and imperative teardown bugs. |
| pnpm | npm / yarn / bun | Beekeeper nudges pnpm/bun. pnpm has strongest monorepo isolation story and fastest install for mixed-language repos. |
| Biome | ESLint + Prettier | Next.js 16 removed `next lint`. Biome is a clean single-tool default for a new greenfield web-only directory with no legacy ESLint config. |
| Vitest | Jest | Jest requires extra Babel config for ESM/TypeScript in Next.js 16. Vitest is native ESM with near-instant cold starts. |

---

## What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `next-mdx-remote` | Designed for runtime / remote MDX compilation; adds server dependency. Not needed when all MDX is local. | `fumadocs-mdx` (build-time, zero runtime overhead) |
| `@mdx-js/react` (direct) | fumadocs-mdx owns and configures the MDX pipeline. Adding `@mdx-js/react` separately creates version conflicts and duplicate processing. | Let fumadocs-mdx manage MDX entirely |
| `framer-motion` (legacy package name) | Rebranded to `motion` in late 2024. The `framer-motion` package still resolves but is the deprecated alias. | `motion` |
| `react-spring` | More complex physics API for needs that ambient motion handles simply. Only justified for physics-simulation animations, which this site does not need. | `motion` |
| `gsap` | Commercial license required for ScrollTrigger and other essential plugins. Overkill for ambient accent animations. | `motion` for CSS-layer accents; R3F for 3D motion |
| `@vercel/analytics` / `@vercel/speed-insights` | Vendor-locked to Vercel deployment. Beekeeper is a privacy-first security tool — shipping tracking scripts would undermine trust. | No analytics by default; self-hosted Plausible if ever needed |
| `react-query` / `@tanstack/react-query` | No dynamic data fetching at runtime. All content is static MDX built at compile time. | Static fetch in Server Components at build time |
| `axios` | No HTTP calls at runtime for this site. | Native `fetch` if any client-side data fetch becomes necessary |
| `i18n` libraries (next-intl, etc.) | Docs are English-only for v1.3.0. i18n in static export adds significant route complexity (locale-prefixed paths, multiple Orama indexes). | English only; revisit if multi-language becomes a goal |
| Storybook | No shared design system to document across teams. shadcn components are copy-owned per-project. Adds substantial dev overhead for marginal gain. | Playwright visual tests cover regressions |
| `redux` / `zustand` / `jotai` | No shared client state needed. Theme toggle and search dialog are the only interactive states, handled by `next-themes` and Fumadocs internals respectively. | React built-ins or Fumadocs state |
| `@react-three/fiber@^10` + `@react-three/drei@^11` | Both are alpha as of June 2026. R3F v10 adds WebGPU/TSL support that is not needed for the hero. Alpha APIs have breaking changes between minor versions. | R3F 9.6.1 + Drei 10.7.7 (stable) |
| `react-icons` | Large bundle; includes hundreds of icon packs. shadcn/ui uses Lucide — stay consistent. | `lucide-react` |
| `styled-components` or `emotion` | CSS-in-JS at runtime conflicts with Tailwind v4's CSS-variable model. Performance overhead on a static site. | Tailwind CSS utility classes + CSS variables |
| Server-only Orama (`fumadocs-core/search/server` without `force-static`) | Requires a running Node.js server. Static export has no server. | Orama static mode with `force-static` Route Handler |

---

## Version Compatibility Matrix

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `next@^16.2.7` | `react@^19.2`, `react-dom@^19.2` | Next.js 16 requires React 19 minimum. Node 20.9+ minimum. |
| `next@^16.2.7` | `typescript@>=5.1` | Hard minimum enforced at build time. |
| `@react-three/fiber@^9.6.1` | `three@>=0.156`, `react@>=19 <19.3` | Use `three@^0.184.0`. R3F is loose on three.js minor versions; pin a specific minor to avoid surprise API breaks. |
| `@react-three/drei@^10.7.7` | `@react-three/fiber@^9.x` | Drei 10 aligns with R3F 9. Drei 11 (alpha) aligns with R3F 10 (alpha) — do NOT mix. |
| `fumadocs-ui@^16.9.3` | `tailwindcss@^4.x` only | Fumadocs v16 dropped Tailwind v3. Cannot mix. Node 22+ required. |
| `fumadocs-mdx@^14.2.11` | `node@>=22` | Fumadocs requires Node 22 minimum. |
| `motion@^12.x` | `react@^19.x` | Motion 12.x tested against React 19 / Next.js 16. |
| `shadcn@latest CLI` | `tailwindcss@^4.x` | CLI auto-detects v4, generates `@theme` CSS-variable syntax. Do NOT create a `tailwind.config.js` — v4 is CSS-first. |
| `lucide-react@^1.17.0` | `react@^19.x` | No known conflicts. |
| `next-themes@^0.4.x` | `next@^16.x` | Uses a client component wrapper; compatible with App Router. |

---

## Sources

- [Next.js Static Exports Guide](https://nextjs.org/docs/app/guides/static-exports) — verified unsupported features list, config options; updated March 2026. **HIGH confidence.**
- [Next.js 16 Release Blog](https://nextjs.org/blog/next-16) — stable release October 2025, Turbopack default, React 19.2, Node 20.9+ minimum. **HIGH confidence.**
- [Next.js 16.2 Blog](https://nextjs.org/blog/next-16-2) — 16.2 stable adapters API, current version 16.2.7. **HIGH confidence.**
- [Fumadocs npm (fumadocs-ui)](https://www.npmjs.com/package/fumadocs-ui) — v16.9.3 published 9 days ago (June 2026). **HIGH confidence.**
- [Fumadocs npm (fumadocs-core)](https://www.npmjs.com/package/fumadocs-core) — v16.9.3 confirmed. **HIGH confidence.**
- [Fumadocs npm (fumadocs-mdx)](https://www.npmjs.com/package/fumadocs-mdx) — v14.2.11 confirmed. **HIGH confidence.**
- [Fumadocs Orama Search Docs](https://www.fumadocs.dev/docs/search/orama) — static mode for `output: 'export'` confirmed. **HIGH confidence.**
- [Fumadocs Theme Docs](https://www.fumadocs.dev/docs/ui/theme) — `shadcn.css` + `preset.css` import order verified; Tailwind v4 `@source` scan requirement confirmed. **HIGH confidence.**
- [shadcn/ui Tailwind v4 Docs](https://ui.shadcn.com/docs/tailwind-v4) — Tailwind v4 + React 19 full support confirmed February 2025; OKLCH colors. **HIGH confidence.**
- [shadcn/ui Next.js Docs](https://ui.shadcn.com/docs/installation/next) — `pnpm dlx shadcn@latest init` confirmed. **HIGH confidence.**
- [R3F GitHub package.json](https://github.com/pmndrs/react-three-fiber) — peer deps `three@>=0.156`, `react@>=19 <19.3`. **HIGH confidence.**
- [R3F Installation Docs](https://r3f.docs.pmnd.rs/getting-started/installation) — v9.6.1 latest, `transpilePackages` requirement. **HIGH confidence.**
- [Drei npm — version 10.7.7](https://www.npmjs.com/package/@react-three/drei) — confirmed latest stable via WebSearch (npm returned 403 on direct fetch). **MEDIUM confidence.**
- [three.js npm — 0.184.0](https://www.npmjs.com/package/three) — confirmed latest via WebSearch cross-check. **MEDIUM confidence.**
- [motion npm — 12.40.0](https://www.npmjs.com/package/motion) — Framer Motion renamed to `motion` late 2024; v12.40.0 latest. **MEDIUM confidence.**
- [Tailwind CSS v4.0 Blog](https://tailwindcss.com/blog/tailwindcss-v4) — stable release January 22, 2025. **HIGH confidence.**
- [Tailwind CSS releases](https://github.com/tailwindlabs/tailwindcss/releases) — v4.3.0 released May 8, 2026. **HIGH confidence.**
- [lucide-react npm — 1.17.0](https://www.npmjs.com/package/lucide-react) — confirmed via WebSearch. **MEDIUM confidence.**
- [Fumadocs Tailwind v4 Discussion](https://github.com/fuma-nama/fumadocs/discussions/1338) — Tailwind v4 + shadcn monorepo coexistence patterns. **MEDIUM confidence.**

---

*v1.3.0 web stack research: 2026-06-07*

---
---

# Prior Research: Go Binary Stack (v1.0.0 and v1.2.0 — archived)

**Domain:** Go security daemon / agent runtime safety harness
**Researched:** 2026-05-26 (v1.0.0), 2026-06-03 (v1.2.0 addendum)
**Confidence:** MEDIUM-HIGH (versions verified via web; some API details LOW due to doc access limits)

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.25 (released Aug 2025) | Primary language for all binary components | Single static binary, no CGO for core, memory safety eliminates C-class bugs, `go.sum` integrity verification for all deps; 1.25 adds container-aware GOMAXPROCS and experimental GreenTeaGC |
| `github.com/fsnotify/fsnotify` | v1.10.1 (May 2026) | Cross-platform filesystem notifications for extension watcher | Wraps inotify/FSEvents/ReadDirectoryChangesW behind one API; only justified non-stdlib dep for OS-native file watching |
| `github.com/cilium/ebpf` | v0.21.0 (Mar 2026) | eBPF program loading/attachment for Sentry on Linux | Pure-Go, no CGO, vetted by the Cilium project, production-proven at scale; alternatives require CGO or are kernel-version-specific wrappers |
| `charm.land/bubbletea/v2` | v2.0.6 (Apr 2026) | TUI dashboard | Mature, no CGO, event-driven Elm-architecture, single-binary-friendly; v2 is the current stable release |
| Python | 3.11+ | LlamaFirewall sidecar only | PyTorch/PromptGuard 2 ecosystem is Python-native; keeping it as a sidecar preserves the Go binary boundary |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `charm.land/lipgloss/v2` | latest v2 | TUI styling (colors, layout, borders) | Always pair with bubbletea v2 — moved to same vanity domain |
| `github.com/charmbracelet/bubbles` | v0.x (verify v2 compat) | Reusable TUI components (list, viewport, spinner) | Use for live activity feed, catalog freshness panel — reduces boilerplate |
| `github.com/charmbracelet/x/exp/teatest` | exp | Snapshot testing for bubbletea programs | v2 teatest compatibility — API is experimental, wrap it in internal helpers |
| `github.com/google/osv-scanner/v2` | v2.3.8 (May 2026) | OSV vulnerability database integration | Use as Go library (`github.com/google/osv-scanner/v2`) rather than shelling out — avoids process spawning on the hot path |
| `github.com/sigstore/cosign/v2` | latest v2.x | Release signing in CI (not a runtime dep) | Build/release tooling only; use keyless signing via GitHub Actions OIDC |
| `github.com/slsa-framework/slsa-github-generator` | v2.1.0+ | SLSA Level 3 provenance (CI only) | Reference via `builder_go_slsa3.yml@v2.1.0` in release workflow |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| GoReleaser v2.13.0+ | Cross-platform binary release with cosign signing | Required for cosign v3 bundle format (`.sigstore.json`); earlier versions use incompatible v2 sig format |
| `cosign` v3.x | Keyless artifact signing | Use `--bundle` flag (not `--output-signature` + `--output-certificate`); requires GoReleaser v2.13+ |
| `osv-scanner` CLI v2.3.8 | Supply chain scanning (offline DB sync) | Invoke via `--offline --download-offline-databases` for local DB; use as library for programmatic use |
| `govulncheck` | Go stdlib vuln scanning in CI | Separate from OSV; covers Go module graph specifically |
| `golangci-lint` | Static analysis | Pin version in CI; run on all three OS targets |
| Renovate | Automated dependency updates | Pin `go.mod` and `go.sum`; Renovate PRs get second-account approval before merge |

---

## Go 1.25 — What Changed for This Use Case

**Release date:** August 2025. **Confidence: HIGH** (verified against go.dev/doc/go1.25)

### `encoding/json`
`encoding/json/v2` is now available as `GOEXPERIMENT=jsonv2`. It exposes `encoding/json/jsontext` for lower-level streaming JSON. **Do NOT enable for the policy engine hot path in v0.1.0** — it is still experimental and not subject to Go 1 compatibility guarantees. The working group targets Go 1.26 for stable adoption. Use stdlib `encoding/json` for the policy engine today; the performance profile is sufficient for sub-100ms targets at catalog-matching scale. Revisit when v2 stabilizes in ~Go 1.26 (Q1 2027 estimate).

**What this means for the NDJSON audit log hot path:** `encoding/json` in Go 1.25 with the new experimental GC (`GOEXPERIMENT=greenteagc`) shows 10-40% GC overhead reduction. Keep the audit log writer as a simple `json.NewEncoder(f).Encode(record)` call — this is already the idiomatic pattern and benefits from GC improvements automatically.

### `net/http`
New `http.CrossOriginProtection` middleware and SHA-1 TLS handshake rejection (RFC 9155). The MCP gateway daemon uses `net/http` as its transport. **Enable CORS protection on the gateway** — even on localhost binding, defense in depth. SHA-1 rejection is a net positive.

### `os/exec`
No breaking changes in 1.25 for `os/exec`. The shim layer and Bumblebee invocation patterns from 1.24 carry forward unchanged.

### Crypto
4x signing speedup for ECDSA/Ed25519 in FIPS mode, 2x SHA-1 hashing via SHA-NI. Relevant if Beekeeper ever runs in a FIPS-140-3 environment (enterprise deployments). Not a blocker for v0.1.0.

### Container-aware GOMAXPROCS
Auto-adjusts for cgroup CPU limits on Linux. The Sentry daemon running inside a Docker container in CI gets correct parallelism without manual `runtime.GOMAXPROCS` calls.

---

## `fsnotify` — v1.10.1 Windows Gotchas

**Version:** v1.10.1, released May 4, 2026. **Confidence: HIGH** (verified pkg.go.dev)

**Requires Go 1.23+.** This is fine for Beekeeper's Go 1.25+ requirement.

### Windows ReadDirectoryChangesW — What to Know

**Recursive watching is NOT in the public API.** `fsnotify` does not expose `Watch("/path", Recursive)`. The recursive code path exists internally for test purposes only. This is a critical design constraint for the extension watcher watching `~/.vscode/extensions/` (flat directory with many subdirectories).

**Implication for Beekeeper:** You must enumerate watched directories explicitly or watch the parent and filter events by path prefix. Watching `~/.vscode/extensions/` catches new directory creation (each extension is a directory); you do not need recursive watching for the extension install detection use case. For the general file watcher, call `watcher.Add()` per directory.

**Buffer size.** Default is 64KB (`WithBufferSize` default). During `npm install` or extension installs, event bursts can overflow. Use `watcher.AddWith(path, fsnotify.WithBufferSize(262144))` — 256KB — for directories that see heavy churn. This applies specifically to `node_modules/` watching if ever needed; for extension dirs the 64KB default is sufficient.

**Windows Write events on parent dirs.** When a child entry is created inside a watched directory, the parent directory itself receives a `Write` event (NTFS last-write-time update). Filter by `event.Op == fsnotify.Create` for the extension directory watcher to avoid acting on spurious `Write` events.

**Chmod events never fire on Windows.** Do not write code that depends on `fsnotify.Chmod` to detect permission changes on Windows — it is silently dropped.

**Path formats.** Accept both `C:\path` and `C:/path` in config; fsnotify handles both.

**What NOT to do:** Do not attempt to enable recursive watching by calling `watcher.Add()` on every subdirectory dynamically — this creates a race condition during rapid directory creation (extension install). Watch the parent dir, filter `Create` events by directory type.

---

## `cilium/ebpf` — v0.21.0 Kernel Requirements

**Version:** v0.21.0, released March 5, 2026. **Confidence: MEDIUM** (releases page verified; feature/kernel mapping from kernel docs and community sources)

### Kernel Version Matrix for Sentry's Three Event Streams

| Feature | Minimum Kernel | Notes |
|---------|---------------|-------|
| Basic eBPF program loading | 4.4+ | EOL'd; CI tests against LTS kernels |
| kprobes/tracepoints | 4.4+ | Core process event capture |
| `CAP_BPF` capability | 5.8 | Before 5.8, need `CAP_SYS_ADMIN`; use `rlimit` shim for older kernels |
| `BPF_MAP_TYPE_RINGBUF` | 5.8 | Preferred over perf event arrays — lower overhead, no per-CPU allocation |
| `fentry`/`fexit` probes (BTF-based) | 5.5 | Better than kprobes for stable kernel ABI |
| CO-RE (Compile Once, Run Everywhere) | 5.8 | Required for shipping pre-compiled eBPF; alternatives need per-kernel compilation |
| `bpf_link` (stable attachment) | 5.7 | Without this, probe detaches on process exit |
| Network socket events via `sock_ops` | 5.4+ | TCP connection tracing |
| fanotify `FAN_REPORT_FID` | 5.1 | File identity in events |
| fanotify `FAN_REPORT_PIDFD` | 5.15 (also 5.10.220 LTS) | Process identity in file events |

**Recommended minimum for Sentry (Linux):** **Kernel 5.15** — gets you `bpf_link`, `BPF_MAP_TYPE_RINGBUF`, CO-RE, and `FAN_REPORT_PIDFD` in fanotify. This aligns with Ubuntu 22.04 LTS (kernel 5.15) and RHEL 9 (kernel 5.14). Kernel 5.10 LTS is acceptable if `FAN_REPORT_PIDFD` is backported (5.10.220+).

**Practical target:** Ubuntu 22.04+ (`ubuntu-latest` on GitHub Actions uses 22.04, kernel 5.15).

**eBPF for Windows:** `cilium/ebpf` lists Windows Server 2022 as tested. Beekeeper's Windows Sentry uses ETW, not eBPF — do not use `cilium/ebpf` on Windows.

**fanotify is separate from eBPF.** fanotify is a standard Linux syscall API (`CAP_SYS_ADMIN` required). Use it directly via `golang.org/x/sys/unix` — no eBPF library needed for file events. Use `cilium/ebpf` only for the process-creation and network-connection event streams via kprobe/tracepoint attachment.

**Key gotcha:** `cilium/ebpf` requires `CGO=0` is NOT set — it is pure Go, no CGO, which is correct. But you DO need the kernel headers or BTF info at compile time for CO-RE programs. The standard pattern is to embed pre-compiled eBPF bytecode using `go:generate` + `bpf2go`, then ship the bytecode in the binary. Do this from the start; retrofitting is painful.

---

## Bubble Tea v2 — Current State and Gotchas

**Version:** v2.0.6, released April 16, 2026. **Import path: `charm.land/bubbletea/v2`** (changed from `github.com/charmbracelet/bubbletea`). **Confidence: HIGH** (verified GitHub releases)

### v1 vs v2 — Use v2

v2 is the current stable release. v1 (last: v1.3.10) is in maintenance-only mode. Start on v2.

**Breaking changes that matter for Beekeeper:**
- `View()` returns `tea.View` (a struct), not `string`. Use `tea.NewView("content")`.
- `tea.KeyMsg` is now an interface; use `tea.KeyPressMsg` for key press handling.
- Import `charm.land/lipgloss/v2` not the old path.

### Windows Terminal Known Issues (Confirmed Bugs as of May 2026)

**CRITICAL — Window resize events not detected (Issue #1601).** Beekeeper's TUI dashboard runs `beekeeper dashboard` on Windows. Terminal resize events (`WindowSizeMsg`) are never fired after the initial startup on Windows. This is a regression from v1 introduced by switching to VT input mode. The dashboard layout will not reflow when the user resizes their terminal window on Windows.

**Mitigation:** Implement a resize polling fallback — use a goroutine that polls `os.Stdout` console size via `golang.org/x/term` every 500ms and sends a synthetic `WindowSizeMsg` when dimensions change. This is the workaround until the upstream issue is resolved.

**Escape sequence leak in short-lived programs (Issue #1627).** v2 queries terminal capability on init (Synchronized Output mode 2026 / Unicode Core mode 2027). If the program exits too quickly, raw escape sequences leak to the shell. The `beekeeper check` hook handler is a short-lived process — **do not use Bubble Tea for the hook handler output**. Use plain `fmt.Println` / `os.Stderr.WriteString`. Bubble Tea is only for `beekeeper dashboard` which is long-lived.

**Window title not reset on panic (Issue #1474).** If Beekeeper panics during dashboard mode, the terminal title stays set. Add a `defer` that resets the title via ANSI escape before the panic propagates.

### Snapshot Testing

`teatest` lives in `github.com/charmbracelet/x/exp/teatest` — note `exp` namespace, API unstable. A new `charm-test` framework proposal opened April 1, 2026 (Issue #1654) but is not available yet.

**Recommendation:** Use `teatest` with `x/exp/golden` for TUI snapshot tests but wrap it in an internal `beekeepertest` package so you isolate the unstable API. When `charm-test` stabilizes, migration is a one-file change. Do not reference `teatest` directly from test files outside the wrapper.

**v2 compatibility of teatest:** As of May 2026, teatest is being updated for v2 but verify `charm.land/x/exp/teatest` (v2 namespace) availability before building TUI tests.

---

## MCP Protocol — 2026 Spec Changes Affecting the Gateway

**Confidence: HIGH** (verified against the 2026-07-28 release candidate blog post)

The final MCP 2026 spec ships **July 28, 2026**. The 10-week RC window started in May 2026, meaning the spec changes are finalized but SDKs may lag.

### What Changes for a Proxy/Gateway Implementation

**`Mcp-Method` and `Mcp-Name` required headers (SEP-2243).** Every Streamable HTTP request now carries these headers so load balancers and proxies can route by operation without body inspection. Beekeeper's gateway MUST:
1. Read and forward these headers on inbound requests.
2. Reject or flag requests where header and body disagree (servers do; gateway should too for defense in depth).
3. Use `Mcp-Method` for rate limiting specific operations without JSON parsing.

**Session model eliminated.** `Mcp-Session-Id` is gone. The `initialize`/`initialized` handshake is removed. Client metadata travels in `_meta` on every request. **This is the most impactful change for the gateway.** Beekeeper's v0.6.0 gateway was designed when sessions were stateful; the new model makes it stateless. Every request now carries full context — simpler per-request policy evaluation, no session state to maintain.

**`ttlMs` and `cacheScope` on list responses.** The gateway can now cache `tools/list` responses per the server's declared TTL without a long-lived SSE stream. Cache at the gateway layer for frequently-polled MCP clients.

**W3C Trace Context in `_meta`.** `traceparent`, `tracestate`, `baggage` keys are now standardized. Beekeeper's OTLP audit sink can correlate with distributed traces from MCP clients/servers by forwarding these headers.

**Authorization.** Clients must validate the `iss` parameter per RFC 9207. The gateway should validate this before forwarding to upstream MCP servers.

### Recommended Gateway Design for July 2026 Spec

Design the gateway as a **stateless HTTP proxy** from the start (not session-based). Each request carries full context in `_meta`. Per-request policy evaluation with no session state simplifies v0.6.0 implementation considerably. The old spec required session affinity; the new spec does not. This is a good thing — simpler implementation, easier horizontal scaling if ever needed.

**What NOT to do:** Do not implement session tracking for MCP routing. The spec explicitly removed it. Any code that tracks `Mcp-Session-Id` is dead code against the July 2026 spec.

---

## Sigstore / Cosign — Keyless Signing Toolchain

**Confidence: MEDIUM-HIGH** (cosign v3 bundle format verified; OIDC flow verified against official docs and GoReleaser blog)

### Current Toolchain (2026)

Use **cosign v3.x** with the `--bundle` flag. cosign v3 replaced the two-file output (`--output-signature` + `--output-certificate`) with a single `.sigstore.json` bundle. GoReleaser v2.13.0+ supports cosign v3 natively.

**Old pattern (v2, DO NOT USE):**
```
cosign sign-blob --output-signature sig.txt --output-certificate cert.pem artifact
```

**Current pattern (v3):**
```
cosign sign-blob --bundle artifact.sigstore.json --yes artifact
```

### GitHub Actions OIDC Flow

Required permissions on the release job:
```yaml
permissions:
  id-token: write    # OIDC token for keyless signing
  contents: write    # Upload release artifacts
  actions: read      # Read workflow path for SLSA provenance
```

No long-lived signing keys. Fulcio issues ephemeral certificates bound to the GitHub Actions OIDC token. The certificate identity is the workflow URL; anyone can verify with:
```
cosign verify-blob \
  --bundle artifact.sigstore.json \
  --certificate-identity "https://github.com/org/beekeeper/.github/workflows/release.yml@refs/tags/v*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  artifact
```

### GoReleaser Integration

`.goreleaser.yaml` signing stanza:
```yaml
signs:
  - cmd: cosign
    signature: "${artifact}.sigstore.json"
    args:
      - sign-blob
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"
    artifacts: all
```

**GoReleaser reference:** `goreleaser/example-supply-chain` repo demonstrates the full pattern including SBOM generation.

### SBOM

Use `syft` to generate CycloneDX SBOM as part of the GoReleaser pipeline. GoReleaser has native `sboms:` config with `syft` integration.

---

## SLSA Level 3 — GitHub Actions Setup

**Confidence: HIGH** (slsa-github-generator README verified; v2.1.0 is current)

### Current Recommended Setup

Use `slsa-framework/slsa-github-generator` v2.1.0 Go builder. Reference by full semantic version tag — not `@main` or `@v2`.

**Critical:** All versions through v1.9.0 have a TUF mirror error. Minimum viable version is **v1.10.0**; use v2.1.0 (current).

**Release workflow skeleton:**
```yaml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    permissions:
      id-token: write
      contents: write
      actions: read
    uses: slsa-framework/slsa-github-generator/.github/workflows/builder_go_slsa3.yml@v2.1.0
    with:
      go-version: "1.25"
      config-file: .github/workflows/slsa-goreleaser.yml
      upload-assets: true
```

**Outputs:** `go-binary-name` (binary filename) and `go-provenance-name` (`.intoto.jsonl` provenance file).

**Artifact download:** Must use `actions/download-artifact@v3` (not v4) due to an incompatibility with the provenance artifact format.

**Private repos:** All builds post to the public Rekor transparency log by default. Set `private-repository: true` only if acceptable that repo name appears in public logs — for a public Apache 2.0 project this is not a concern.

### SLSA Phase Targeting

- **v0.1.0:** Sigstore keyless signing only (no SLSA provenance yet — acceptable for early releases).
- **v0.9.0:** Add SLSA Level 3 provenance via `builder_go_slsa3.yml` (as planned in the PRD).
- **v1.0.0:** SLSA + SBOM + reproducible build verification script (`make verify-release`).

### Verification for Users

```bash
slsa-verifier verify-artifact beekeeper-linux-amd64 \
  --provenance-path beekeeper-linux-amd64.intoto.jsonl \
  --source-uri github.com/your-org/beekeeper \
  --source-tag v1.0.0
```

---

## Bumblebee NDJSON Schema

**Confidence: HIGH** (verified against perplexityai/bumblebee GitHub repo, v0.1.1)

**Current Bumblebee version:** v0.1.1 (`go install github.com/perplexityai/bumblebee/cmd/bumblebee@v0.1.1`)

### Record Types

Bumblebee emits three record types: `package`, `finding`, `scan_summary`.

### Package Record (canonical fields)

```json
{
  "record_type": "package",
  "record_id": "<uuid>",
  "schema_version": "0.1.0",
  "scanner_name": "bumblebee",
  "scanner_version": "v0.1.1",
  "run_id": "<uuid>",
  "scan_time": "<RFC3339>",
  "endpoint": {
    "hostname": "...",
    "os": "darwin|linux|windows",
    "arch": "amd64|arm64",
    "username": "...",
    "uid": "...",
    "device_id": "..."
  },
  "profile": "...",
  "ecosystem": "npm|pypi|go|rubygems|packagist|cargo|editor-extension|browser-extension|mcp",
  "package_name": "...",
  "normalized_name": "...",
  "version": "...",
  "project_path": "...",
  "root_kind": "...",
  "package_manager": "...",
  "source_type": "...",
  "source_file": "...",
  "has_lifecycle_scripts": false,
  "confidence": "high|medium|low"
}
```

### Finding Record (exposure match)

```json
{
  "record_type": "finding",
  "finding_type": "package_exposure",
  "severity": "critical|high|medium|low",
  "catalog_id": "...",
  "catalog_name": "...",
  "evidence": "...",
  "...": "plus all package base fields"
}
```

### Exposure Catalog Format (threat_intel/)

```json
{
  "schema_version": "0.1.0",
  "entries": [
    {
      "id": "advisory-2026-XXXX",
      "name": "...",
      "ecosystem": "npm",
      "package": "nx-console-vscode",
      "versions": ["1.2.3"],
      "severity": "critical"
    }
  ]
}
```

**Key constraint:** The schema requires top-level `schema_version` and `entries` keys. Bare arrays are rejected. Beekeeper's extended catalog schema (with `source_url`, `catalog_signature`, `catalog_source`) must remain an extension — compatible with Bumblebee's schema, not a replacement.

**Beekeeper `scanner_name`:** Set `"scanner_name": "beekeeper"` in Beekeeper-generated records; `"scanner_name": "bumblebee"` in records that pass through from Bumblebee invocations.

---

## OSV Database — Offline Sync

**Confidence: HIGH** (osv.dev docs and osv-scanner v2 docs verified)

**OSV-Scanner version:** v2.3.8 (May 8, 2026)

### Offline DB Structure

```
{OSV_SCANNER_LOCAL_DB_CACHE_DIRECTORY}/osv-scanner/{ECOSYSTEM}/all.zip
```

`OSV_SCANNER_LOCAL_DB_CACHE_DIRECTORY` defaults to the OS cache dir. Beekeeper should set this explicitly to `~/.beekeeper/catalogs/osv/`.

### Download URLs (direct GCS, no SDK needed)

```
https://osv-vulnerabilities.storage.googleapis.com/{ECOSYSTEM}/all.zip
```

Ecosystem list: `https://osv-vulnerabilities.storage.googleapis.com/ecosystems.txt`

Example:
```
https://osv-vulnerabilities.storage.googleapis.com/npm/all.zip
https://osv-vulnerabilities.storage.googleapis.com/PyPI/all.zip
https://osv-vulnerabilities.storage.googleapis.com/Go/all.zip
```

Beekeeper can download these directly in the catalog sync daemon without invoking the `osv-scanner` CLI — just `http.Get` + write to the expected path. This is simpler, faster, and avoids a subprocess for the hourly sync.

### Programmatic Use

`github.com/google/osv-scanner/v2` is importable as a Go library. For Beekeeper's policy engine hot path, import the library rather than shelling out to the CLI. The library exposes the database query logic. Shelling out to `osv-scanner` is acceptable for the `beekeeper scan` command where latency is not critical; avoid it for per-tool-call evaluation.

---

## Socket Public API

**Confidence: MEDIUM** (endpoint URL verified; rate limits and full ecosystem support partially verified; score endpoint marked deprecated)

### Known Endpoints

| Endpoint | Method | Notes |
|----------|--------|-------|
| `https://api.socket.dev/v0/npm/{package}/{version}/score` | GET | **Deprecated** — use successor |
| `https://api.socket.dev/v0/purl` | POST | PURL-based multi-ecosystem lookup; preferred |
| SBOM export | GET | CycloneDX, beta, requires `report:read` scope |

**Authentication:** Bearer token. API tokens available from Socket dashboard.

**Free tier:** Socket is free for open-source use. The public website (socket.dev) shows package scores without auth. The REST API requires a token but open-source projects get free quota. Each endpoint call consumes 1 quota unit. The public MCP server at `https://mcp.socket.dev/` requires no API key at all — explore this as a zero-auth catalog source.

---

## Alternatives Considered (Go stack)

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `charm.land/bubbletea/v2` | `tview` / `tcell` | tview has richer widget set (tables, forms) but is less idiomatic for event-driven agent monitoring; use tview if you need a full-featured data grid |
| `github.com/cilium/ebpf` | `libbpfgo` (CGO wrapper) | libbpfgo if you need features newer than cilium/ebpf supports; CGO cost is real for a pure-Go binary |
| `github.com/fsnotify/fsnotify` | `golang.org/x/sys/windows` + manual ReadDirectoryChangesW | Direct syscall if you need USN journal-based watching (better for high-churn dirs); more code, same reliability |
| Direct GCS download for OSV | `osv-scanner` CLI subprocess | CLI subprocess is fine for the daily `beekeeper scan` command; use direct download + library for the hourly catalog sync daemon |
| Keyless cosign + SLSA | Long-lived signing keys | Long-lived keys only if deploying in an air-gapped environment without GitHub Actions OIDC access |
| `encoding/json` stdlib | `encoding/json/v2` (jsonv2) | Adopt jsonv2 when it stabilizes in Go 1.26; the performance gains (substantially faster decoding) are real but not worth an experimental dep for a security tool |

---

## What NOT to Use (Go stack)

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| CGO in the core binary | Breaks cross-compilation, adds C-class memory bugs, complicates static binary | Pure-Go throughout; sidecars for Python-ecosystem code |
| `encoding/json/v2` (`GOEXPERIMENT=jsonv2`) in production | Experimental, not covered by Go 1 compat promise, API may change in Go 1.26 | `encoding/json` stdlib; revisit for Go 1.26 |
| `charm.land/bubbletea/v2` for short-lived processes | Escape sequence leak on quick exit (Issue #1627) | Plain `fmt.Fprintf` / `os.Stderr` for `beekeeper check`, hook handler, and any sub-100ms process |
| `fsnotify` recursive watching | Not in public API; internal implementation only | Watch each extension directory explicitly with `watcher.Add()` per path |
| `Mcp-Session-Id` tracking in the MCP gateway | Removed from July 2026 spec; dead code | Stateless per-request proxy with `_meta` context |
| `cosign sign-blob --output-signature --output-certificate` | cosign v2 format; GoReleaser v2.13+ uses v3 bundle | `cosign sign-blob --bundle artifact.sigstore.json --yes` |
| `slsa-github-generator` before v1.10.0 | TUF mirror error affects all versions <= v1.9.0 | v2.1.0+ |
| Socket score endpoint (`/v0/npm/{pkg}/{ver}/score`) | Marked deprecated | PURL endpoint (`/v0/purl`) |
| `github.com/charmbracelet/bubbletea` (old import) | v1, maintenance-only; v2 moved to vanity domain | `charm.land/bubbletea/v2` |
| Bare array in Bumblebee catalog files | Schema validation rejects arrays at top level | Wrap in `{"schema_version": "0.1.0", "entries": [...]}` |

---

## Version Compatibility (Go stack)

| Package | Compatible With | Notes |
|---------|----------------|-------|
| `charm.land/bubbletea/v2` v2.0.6 | `charm.land/lipgloss/v2` | Both must be v2; mixing old lipgloss with new bubbletea breaks |
| `github.com/cilium/ebpf` v0.21.0 | Go 1.23+ | v0.21.0 requires Go 1.23; Beekeeper's Go 1.25 requirement is compatible |
| `github.com/fsnotify/fsnotify` v1.10.1 | Go 1.23+ | Same minimum; compatible |
| `github.com/google/osv-scanner/v2` v2.3.8 | Go 1.21+ | Compatible with Go 1.25 |
| GoReleaser v2.13.0+ | cosign v3.x | Earlier GoReleaser versions ship `.sig` files (cosign v2 format) incompatible with v3 verification |
| `slsa-github-generator` v2.1.0 | `actions/download-artifact@v3` | NOT v4 — provenance artifact format incompatibility |

---

## Go Stack Sources

- [go.dev/doc/go1.25](https://go.dev/doc/go1.25) — Go 1.25 release notes (HIGH confidence)
- [pkg.go.dev/github.com/fsnotify/fsnotify](https://pkg.go.dev/github.com/fsnotify/fsnotify) — v1.10.1 docs, Windows caveats (HIGH confidence)
- [github.com/cilium/ebpf/releases](https://github.com/cilium/ebpf/releases) — v0.21.0 release (HIGH confidence for version; MEDIUM for kernel-feature mapping)
- [github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) — v2.0.6 release, Windows issues #1601 #1627 (HIGH confidence)
- [blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/](https://blog.modelcontextprotocol.io/posts/2026-07-28-release-candidate/) — July 2026 MCP spec RC (HIGH confidence)
- [github.com/perplexityai/bumblebee](https://github.com/perplexityai/bumblebee) — NDJSON schema v0.1.0 (HIGH confidence)
- [google.github.io/osv-scanner/](https://google.github.io/osv-scanner/) — v2.3.8, offline mode (HIGH confidence)
- [docs.socket.dev/reference/getscorebynpmpackage](https://docs.socket.dev/reference/getscorebynpmpackage) — endpoint docs, deprecated status (MEDIUM confidence; full rate limit docs not publicly accessible)
- [github.com/slsa-framework/slsa-github-generator](https://github.com/slsa-framework/slsa-github-generator) — v2.1.0 Go builder README (HIGH confidence)
- [goreleaser.com/blog/cosign-v3/](https://goreleaser.com/blog/cosign-v3/) — cosign v3 bundle migration (HIGH confidence)
- [man7.org/linux/man-pages/man7/fanotify.7.html](https://www.man7.org/linux/man-pages/man7/fanotify.7.html) — fanotify kernel version matrix (HIGH confidence)

---

# v1.2.0 Runtime Behavioral Hardening — Stack Addendum

**Researched:** 2026-06-03
**Confidence:** HIGH (all version claims verified against official sources and GitHub releases as of 2026-06-03)
**Scope:** ONLY the three new features: PLCY-05 (sensitive-path wiring), NUDGE (package manager nudge), PLCY-07 (corroboration hardening). Does not re-research v1.0.0 stack.

---

## New Dependency Required

Exactly one new Go module dependency is needed for this entire milestone:

```bash
go get golang.org/x/mod@latest
```

Everything else uses Go stdlib or already-present transitive packages.

---

## Feature Stack Breakdown

### PLCY-05: Sensitive-Path Wiring

**Stack delta: zero.** This feature wires `internal/policy.EvaluatePath` and `DefaultSensitivePaths` into `internal/check/handler.go`. All types exist. No new dependencies.

### PLCY-07: Corroboration Hardening

**Stack delta: zero.** Per-severity escalation logic is a pure change to threshold constants/rules in the existing corroboration package. No new types, no new dependencies.

### NUDGE: `internal/nudge/` — Full Stack Analysis

#### Subprocess Detection (`detect.go`)

Use `os/exec.CommandContext` + `context.WithTimeout(ctx, 2*time.Second)` for `pnpm --version`, `bun --version`, `node --version`. Use `exec.LookPath` before exec to distinguish "not installed" from "installed but broken."

Pattern for `cmd.Output()` is safe for `--version` output (single line, < 1KB): no pipe deadlock risk at this size.

Detection cache: `sync.Mutex`-guarded struct with last `PMState` + `time.Time`. TTL 60 seconds per PRD §4. Pure stdlib — no external caching library.

#### Semver Comparison (`version.go`)

**Use `golang.org/x/mod/semver`.** This is the only new dependency needed.

Critical implementation note — the `"v"` prefix requirement:

| Binary | `--version` output | Needs normalization? |
|--------|-------------------|---------------------|
| `pnpm --version` | `11.5.1` (bare) | YES — prepend `"v"` |
| `bun --version` | `1.3.14` (bare) | YES — prepend `"v"` |
| `node --version` | `v22.x.y` (already prefixed) | NO |

Required normalizer:

```go
// normalize prepends "v" if absent. golang.org/x/mod/semver requires it.
func normalize(v string) string {
    v = strings.TrimSpace(v)
    if strings.HasPrefix(v, "v") {
        return v
    }
    return "v" + v
}
```

Why `golang.org/x/mod/semver` over a hand-written comparator: The drift-detection path must handle pre-release versions like `pnpm 12.0.0-rc.1`. A hand-written integer-tuple comparator handles `>=` correctly for stable versions but misorders pre-releases. `golang.org/x/mod/semver` handles pre-release ordering per spec. Zero transitive deps.

#### TOML / YAML Parsing for Nudge Detection

**Do not add a TOML or YAML library.** The nudge module needs exactly one scalar from `bunfig.toml` and two scalars from `pnpm-workspace.yaml`. Hand-written line scanners (20 lines each) are simpler, have zero audit surface, and are easier to fuzz than a full parser dependency.

---

## PRD Version Claims — Verification Results (v1.2.0)

### pnpm >= 11.0 requires Node >= 22

**CONFIRMED.** pnpm 11.0.0 released 2026-04-28. Drops Node 18/19/20/21. Latest: **pnpm 11.5.1** (2026-06-02). `versionFloors.pnpm: "11.0.0"` is correct.

**PRD inaccuracy to fix:** PRD §6.3 says threshold "less than 60 minutes" is a configuration weakness. The pnpm default for `minimumReleaseAge` is **1440 minutes** (24 hours). Fix the threshold constant or document the deliberate conservatism explicitly.

### bun >= 1.3, Security Scanner API stable

**CONFIRMED.** Bun 1.3.0 released 2025-10-10. Security Scanner API in 1.3.0. Latest: **bun 1.3.14** (May 2026). `versionFloors.bun: "1.3.0"` is correct.

### `@socketsecurity/bun-security-scanner` package name

**CONFIRMED.** Exact npm package name `@socketsecurity/bun-security-scanner`, publisher SocketDev. `bunfig.toml` section `[install.security]`, key `scanner = "@socketsecurity/bun-security-scanner"`.

### Node.js 22 "LTS" status

**PARTIALLY ACCURATE.** Node 22 is in **Maintenance LTS** as of 2025-10-21. Node 24 ("Krypton") is current **Active LTS**. Messages should say "Node 22 or later (Node 24 is the current Active LTS)."

---

## What NOT to Add (v1.2.0 specific)

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `BurntSushi/toml` | One key from `bunfig.toml` does not justify a full parser | Hand-written 20-line line scanner |
| `gopkg.in/yaml.v3` | Two scalar keys from `pnpm-workspace.yaml` do not justify a YAML parser | Hand-written `strings.HasPrefix` line scan |
| `Masterminds/semver/v3` | Heavier than needed; `x/mod/semver` is Go-team maintained with zero transitive deps | `golang.org/x/mod/semver` |
| External cache library | 60s TTL single-value cache is 5 lines of `sync.Mutex` + `time.Time` | stdlib `sync` + `time` |

---

## v1.2.0 Sources

- [pnpm 11.0 blog](https://pnpm.io/blog/releases/11.0) — Node >=22 requirement, `minimumReleaseAge` default 1440. HIGH confidence.
- [pnpm settings docs](https://pnpm.io/settings) — Both settings in `pnpm-workspace.yaml` only. HIGH confidence.
- [bun 1.3 blog](https://bun.com/blog/bun-v1.3) — Bun 1.3.0 released 2025-10-10; Security Scanner API in 1.3.0. HIGH confidence.
- [SocketDev/bun-security-scanner GitHub](https://github.com/SocketDev/bun-security-scanner) — Package name, publisher, bunfig.toml key. HIGH confidence.
- [Node.js releases page](https://nodejs.org/en/about/previous-releases) — Node 22 Maintenance LTS, Node 24 Active LTS. HIGH confidence.
- [pkg.go.dev/golang.org/x/mod/semver](https://pkg.go.dev/golang.org/x/mod/semver) — Requires "v" prefix; `Compare(v, w)`. HIGH confidence.

---

*v1.0.0 stack researched: 2026-05-26*
*v1.2.0 addendum researched: 2026-06-03*
*v1.3.0 web addendum researched: 2026-06-07*
