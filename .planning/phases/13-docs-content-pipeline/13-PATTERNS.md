# Phase 13: Docs Content Pipeline — Pattern Map

**Mapped:** 2026-06-08
**Files analyzed:** 14 (new/modified)
**Analogs found:** 7 / 14 (7 net-new with no in-repo analog — use RESEARCH.md patterns)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/next.config.mjs` (rename from `.ts`) | config | transform | `web/next.config.ts` (same file) | exact — rename + augment |
| `web/source.config.ts` | config | transform | none | no analog — use RESEARCH.md Pattern 2 |
| `web/tsconfig.json` | config | — | `web/tsconfig.json` (same file) | exact — single `paths` key addition |
| `web/package.json` | config | — | `web/package.json` (same file) | exact — add `postinstall` script |
| `web/lib/source.ts` | utility / data-binding | transform | `web/lib/utils.ts` | role-match (lib utility, single export) |
| `web/app/providers.tsx` | provider | request-response | `web/app/providers.tsx` (same file) | exact — insert at marked location |
| `web/app/globals.css` | config | — | `web/app/globals.css` (same file) | exact — uncomment 3 lines |
| `web/app/docs/layout.tsx` | layout | request-response | `web/app/layout.tsx` | partial — RSC layout wrapper pattern |
| `web/app/docs/[[...slug]]/page.tsx` | page (RSC) | request-response | `web/app/page.tsx` | partial — RSC default export shape |
| `web/app/api/search/route.ts` | route handler | request-response | none | no analog — use RESEARCH.md Pattern 7 |
| `web/content/docs/meta.json` | data | — | none | no analog — use RESEARCH.md Sidebar Ordering |
| `web/content/docs/*/meta.json` (×8) | data | — | none | no analog — use RESEARCH.md Sidebar Ordering |
| `web/content/docs/*/index.mdx` (×8) | content | — | none | no analog — use RESEARCH.md Seed Content |

---

## Pattern Assignments

### `web/next.config.mjs` — rename from `web/next.config.ts`

**Analog:** `web/next.config.ts` (lines 1–29 — the file being renamed/modified)

**Existing content to preserve** (lines 1–29 of `web/next.config.ts`):
```typescript
// These four config properties must be kept verbatim:
output: "export",
trailingSlash: true,
images: { unoptimized: true },
transpilePackages: ["three", "@react-three/fiber", "@react-three/drei"],
```

**Transformation:** Replace `import type { NextConfig } from "next"` style with JSDoc comment; wrap config with `createMDX()`.

**Target file content after rename:**
```javascript
// web/next.config.mjs
// Source: fumadocs.dev/docs/mdx/next [CITED: RESEARCH.md Pattern 1]
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

**Biome note:** Biome covers `**/*.mts` (in `tsconfig.json` include) but `.mjs` is covered by `files.includes: ["**"]` in `web/biome.json` line 10. No Biome config change needed.

---

### `web/source.config.ts` — NEW

**No in-repo analog.** This is a fumadocs-mdx config file; no existing file in `web/` plays this role.

**Use RESEARCH.md Pattern 2 verbatim:**
```typescript
// web/source.config.ts
import { defineDocs, defineConfig } from 'fumadocs-mdx/config';

export const docs = defineDocs({
  dir: 'content/docs',
});

export default defineConfig();
```

**Formatting convention** (from `web/biome.json` line 13): 2-space indent. The snippet above already matches.

---

### `web/tsconfig.json` — MODIFY

**Analog:** `web/tsconfig.json` itself (lines 1–34).

**Current `paths` block** (lines 21–23):
```json
"paths": {
  "@/*": ["./*"]
}
```

**Target `paths` block** — add `collections/*` alias immediately after `@/*`:
```json
"paths": {
  "@/*": ["./*"],
  "collections/*": ["./.source/*"]
}
```

**No other changes.** All other `compilerOptions`, `include`, and `exclude` fields remain unchanged. The `include` array already has `"**/*.mts"` (line 32) so `next.config.mjs` / `.mts` variants are covered.

---

### `web/package.json` — MODIFY

**Analog:** `web/package.json` itself (lines 1–35).

**Current `scripts` block** (lines 7–12):
```json
"scripts": {
  "dev": "next dev",
  "build": "next build",
  "start": "pnpm dlx serve out",
  "lint": "biome check",
  "format": "biome format --write"
}
```

**Target `scripts` block** — add `postinstall` as first entry:
```json
"scripts": {
  "postinstall": "fumadocs-mdx",
  "dev": "next dev",
  "build": "next build",
  "start": "pnpm dlx serve out",
  "lint": "biome check",
  "format": "biome format --write"
}
```

**Also add to `dependencies`** (install command: `pnpm --filter web add fumadocs-ui fumadocs-core fumadocs-mdx @types/mdx`):
```json
"fumadocs-ui": "16.9.3",
"fumadocs-core": "16.9.3",
"fumadocs-mdx": "15.0.11",
"@types/mdx": "2.0.14"
```

---

### `web/lib/source.ts` — NEW

**Analog:** `web/lib/utils.ts` (lines 1–6) — same directory, same role (single-export utility), same formatting.

**Import convention from analog** (`web/lib/utils.ts` lines 1–2):
```typescript
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";
```
Conventions: double-quotes, no semicolon omissions, named imports. Biome `organizeImports: on` will sort imports automatically.

**Target file:**
```typescript
// web/lib/source.ts
import { docs } from 'collections/server';
import { loader } from 'fumadocs-core/source';

export const source = loader({
  baseUrl: '/docs',
  source: docs.toFumadocsSource(),
});
```

**Note:** `collections/server` resolves via the `tsconfig.json` `paths` alias to `.source/server` (generated by fumadocs-mdx at build). This file has NO `"use client"` directive — it is build-time only (RSC / server boundary).

---

### `web/app/providers.tsx` — MODIFY

**Analog:** `web/app/providers.tsx` itself (lines 1–21).

**Current file** (full, lines 1–21):
```typescript
"use client";
import { ThemeProvider } from "next-themes";
import { ReducedMotionProvider } from "@/lib/reduced-motion";

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
        {/* Phase 13 inserts: <RootProvider theme={{ enabled: false }}> here */}
        {children}
        {/* Phase 13 closes: </RootProvider> */}
      </ReducedMotionProvider>
    </ThemeProvider>
  );
}
```

**Target file after edit** — replace the two marker comments + `{children}` with `<RootProvider>` wrapper; add import:
```typescript
"use client";
import { ThemeProvider } from "next-themes";
import { ReducedMotionProvider } from "@/lib/reduced-motion";
import { RootProvider } from "fumadocs-ui/provider/next";

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
        <RootProvider
          theme={{ enabled: false }}
          search={{ options: { type: 'static' } }}
        >
          {children}
        </RootProvider>
      </ReducedMotionProvider>
    </ThemeProvider>
  );
}
```

**Critical:** Import path is `'fumadocs-ui/provider/next'` — NOT `'fumadocs-ui/provider'` (does not exist in v16). Theme prop `theme={{ enabled: false }}` is mandatory to prevent double next-themes instance.

---

### `web/app/globals.css` — MODIFY

**Analog:** `web/app/globals.css` itself (lines 1–167).

**Current Section 3** (lines 7–10):
```css
/* Section 3 — Fumadocs imports: uncomment in Phase 13 after installing packages */
/* @import "fumadocs-ui/css/shadcn.css"; */
/* @import "fumadocs-ui/css/preset.css"; */
/* @source "../node_modules/fumadocs-ui/dist/[double-star]/*.js"; (uncomment in Phase 13 with correct glob) */
```

**Target Section 3** — replace all four lines with:
```css
/* Section 3 — Fumadocs imports: Phase 13 uncomments these */
@import "fumadocs-ui/css/shadcn.css";
@import "fumadocs-ui/css/preset.css";
@source "../node_modules/fumadocs-ui/dist/**/*.js";
```

**Critical:** Line 10 placeholder `[double-star]` must be replaced — it is NOT valid PostCSS when uncommented. The real glob is `**/*.js` (double-star, unescaped). All lines in Sections 4–8 (lines 12–167) are UNCHANGED.

**Import order is load-bearing** (confirmed by Phase 12 Tailwind `@theme inline` dual-theme lesson in MEMORY.md):
- `shadcn.css` before `preset.css` — maps `--color-fd-*` to shadcn tokens first
- Both before Section 4 `@custom-variant dark`
- Beekeeper `@theme inline` (Section 5, lines 16–76) stays LAST — wins over Fumadocs tokens

---

### `web/app/docs/layout.tsx` — NEW

**Analog:** `web/app/layout.tsx` (lines 1–46) — same role (RSC layout wrapper), same file location pattern, same `ReactNode` children typing.

**Import convention from analog** (`web/app/layout.tsx` lines 1–4):
```typescript
import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { Providers } from "./providers";
import "./globals.css";
```
Conventions: `import type` for type-only imports, double-quotes, Biome import organization.

**Children prop pattern from analog** (`web/app/layout.tsx` lines 24–26):
```typescript
export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
```

**Target file** (does NOT share marketing `SiteNav`/`SiteFooter` — docs layout is self-contained in DocsLayout):
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

**Key points:**
- `source.pageTree` (not `source.getPageTree()`) — direct property, no i18n needed
- `DocsLayout` import from `'fumadocs-ui/layouts/docs'` — NOT `'fumadocs-ui/layout'`
- No `"use client"` — this is a Server Component (RSC)
- `@/lib/source` uses the project's `@/*` alias (from `tsconfig.json` line 22)

---

### `web/app/docs/[[...slug]]/page.tsx` — NEW

**Analog:** `web/app/page.tsx` (lines 1–11) — same role (RSC page default export), same location tier (`app/`), same async RSC pattern.

**RSC default export convention from analog** (`web/app/page.tsx` lines 2–11):
```typescript
export default function Home() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
      ...
    </main>
  );
}
```
Conventions: no `"use client"`, default export function, JSX return.

**Target file** (async RSC with Next.js 16 async params pattern):
```typescript
// web/app/docs/[[...slug]]/page.tsx
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

**Critical:** `params` is a `Promise` in Next.js 16 App Router — must `await props.params`. Page components from `'fumadocs-ui/layouts/docs/page'` (NOT `'fumadocs-ui/layouts/docs'`). `generateStaticParams` is required for `output: 'export'`.

---

### `web/app/api/search/route.ts` — NEW

**No in-repo analog.** No route handlers exist yet in `web/app/api/`.

**Use RESEARCH.md Pattern 7 verbatim:**
```typescript
// web/app/api/search/route.ts
import { source } from '@/lib/source';
import { createFromSource } from 'fumadocs-core/search/server';

export const revalidate = false;

export const { staticGET: GET } = createFromSource(source);
```

**Critical:** `revalidate = false` (not `dynamic = 'force-static'`) — this is what fumadocs docs show and ensures the route handler executes at `next build` time, emitting `out/api/search/index.json`. Import `createFromSource` from `'fumadocs-core/search/server'` — NOT the old `getDocsSearchIndex`.

---

### `web/content/docs/meta.json` — NEW (top-level)

**No in-repo analog.** Data file, no code pattern needed.

**Use RESEARCH.md Sidebar Ordering verbatim:**
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

---

### `web/content/docs/*/meta.json` — NEW (×8 section-level)

**No in-repo analog.** Same pattern for all 8 sections — title varies.

**Template:**
```json
{
  "title": "<Section Title>",
  "pages": ["index"]
}
```

**All 8 titles** (match sidebar order above):
- `getting-started/meta.json` → `"Getting Started"`
- `installation/meta.json` → `"Installation"`
- `configuration/meta.json` → `"Configuration"`
- `integration/meta.json` → `"Integration"`
- `security/meta.json` → `"Security"`
- `cli-reference/meta.json` → `"CLI Reference"`
- `audit-log/meta.json` → `"Audit Log"`
- `troubleshooting/meta.json` → `"Troubleshooting"`

---

### `web/content/docs/*/index.mdx` — NEW (×8 seed files)

**No in-repo analog.** MDX content files; no existing MDX in the project.

**Required frontmatter schema** (fumadocs-mdx enforces `title` and `description`):
```mdx
---
title: <Section Title>
description: <One sentence describing this section.>
---
```

**TOC test page** (`getting-started/index.mdx`) must have ≥3 headings (SC-3 requirement). Use RESEARCH.md Seed Content section for the full content of this page.

**Other 7 files** — each needs ≥3 headings and ≥50 words so Orama can index ≥1 result per query. Follow the stub pattern from RESEARCH.md Seed Content section.

---

## Shared Patterns

### Import alias convention
**Source:** `web/tsconfig.json` lines 21–23, `web/app/layout.tsx` line 3, `web/lib/utils.ts` line 1
**Apply to:** All new `.ts`/`.tsx` files under `web/`
```typescript
// Internal web/ modules: use @/* alias
import { source } from '@/lib/source';
import { ReducedMotionProvider } from '@/lib/reduced-motion';
// NOT relative: ../../lib/source
```

### RSC vs Client boundary
**Source:** `web/app/providers.tsx` line 1, `web/lib/reduced-motion.tsx` line 1, `web/app/layout.tsx` (no directive)
**Apply to:** All new files
```typescript
// Client components MUST declare at the top of the file:
"use client";
// Server Components (RSC) have NO directive — absence = server by default
// New Phase 13 files that are RSC: layout.tsx, [[...slug]]/page.tsx, route.ts, lib/source.ts
// Modified client file: providers.tsx (already has "use client")
```

### Formatting (Biome)
**Source:** `web/biome.json` lines 12–15
**Apply to:** All new `.ts`/`.tsx`/`.mjs` files
- 2-space indent
- `organizeImports: on` (Biome auto-sorts on save/lint)
- Double-quotes for string literals (follow existing `web/app/layout.tsx` and `web/lib/*.ts` convention)

### `import type` for type-only imports
**Source:** `web/app/layout.tsx` line 1, `web/app/docs/layout.tsx` (ReactNode)
**Apply to:** All new TypeScript files
```typescript
import type { Metadata } from 'next';
import type { ReactNode } from 'react';
```

---

## No Analog Found

Files with no close match in the codebase (use RESEARCH.md patterns directly):

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/source.config.ts` | config | transform | fumadocs-mdx collection config; no equivalent config pattern exists |
| `web/app/api/search/route.ts` | route handler | request-response | First route handler in the project; no existing API routes |
| `web/content/docs/meta.json` (all) | data | — | First JSON content files; no MDX/content infrastructure exists yet |
| `web/content/docs/*/index.mdx` (all) | content | — | First MDX files in the project |

---

## Metadata

**Analog search scope:** `web/app/`, `web/lib/`, `web/next.config.ts`, `web/tsconfig.json`, `web/package.json`, `web/biome.json`
**Files scanned:** 9
**Pattern extraction date:** 2026-06-08
