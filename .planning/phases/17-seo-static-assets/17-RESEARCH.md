# Phase 17: SEO & Static Assets - Research

**Researched:** 2026-06-09
**Domain:** Next.js 16 App Router metadata API, static export SEO, OG image conventions
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 Canonical base URL = `https://beekeeper.vercel.app`** — bake this in as a single source-of-truth constant used by `metadataBase`, every canonical link, absolute OG/Twitter image + `og:url`, and every `sitemap.xml` URL.
- **D-02 `trailingSlash: true` governs URL shape** — canonical links and sitemap URLs MUST match the emitted route shape (e.g. `https://beekeeper.vercel.app/docs/getting-started/`, home = `https://beekeeper.vercel.app/`).
- **D-03 Keep the static export; everything must work at build time** — `output: 'export'` stays. SEO assets must be produced during `pnpm build` with NO server/Edge runtime. `next/og` `ImageResponse` (`opengraph-image.tsx`) does NOT execute during `output: 'export'`. The OG image is a pre-rendered static 1200×630 PNG committed to the repo.
- **D-04 Honesty obligation still applies** — metadata must not claim the product is live/installable in ways that overclaim.
- **D-05 Research-first, plan inline** — discuss-phase skipped; research-first chosen.

### Claude's Discretion

- OG image production method (static PNG asset vs any build-time generation that actually works under `output: 'export'`) and its visual design (reuse Phase-12 brand: amber #e3b341, teal #39c5cf, dark `--bg #0a0d12`, Inter/JetBrains; honeycomb/hive motif).
- `sitemap.xml` / `robots.txt` mechanism — Next `app/sitemap.ts` + `app/robots.ts` (MetadataRoute) convention vs static `public/sitemap.xml` + `public/robots.txt`.
- Per-page `title`/`description`/canonical via the Next Metadata API in `layout.tsx` + per-route.
- Verification approach (extend existing Python+Playwright harness or static `out/` grep spec).

### Deferred Ideas (OUT OF SCOPE)

- SITE-03 live Vercel deploy + public-URL verification.
- Per-page bespoke OG images, JSON-LD/structured data beyond basic OG, analytics.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SEO-01 | Each page emits correct static metadata (title / description / canonical), an OG / social card image, `sitemap.xml`, and `robots.txt` | Full coverage: metadataBase API, static PNG file convention, app/sitemap.ts + app/robots.ts with force-static, generateMetadata for Fumadocs routes |
</phase_requirements>

---

## Summary

Phase 17 adds SEO completeness to the static-export Beekeeper site. The existing codebase already emits per-page `<title>` and `<meta name="description">` for all Fumadocs routes (via frontmatter + `generateMetadata`) and for the home page (via `layout.tsx`). What is missing is: `metadataBase` (so relative canonical/OG URLs resolve), `alternates.canonical` on every page, `openGraph.*` and `twitter.*` tags site-wide, the OG card PNG itself, `sitemap.xml`, and `robots.txt`.

The critical research finding is that Next.js 16's `output: 'export'` supports the static-file OG convention (`app/opengraph-image.png`), `app/sitemap.ts`, and `app/robots.ts` — BUT `sitemap.ts` and `robots.ts` REQUIRE `export const dynamic = 'force-static'` to actually emit during a static build (without it the build errors with a generateStaticParams complaint). The static `public/robots.txt` + `public/sitemap.xml` approach is a simpler, zero-risk fallback. Both approaches are evaluated below; the recommendation is `app/sitemap.ts` + `app/robots.ts` with `force-static` for the canonical URLs to be correct, plus the static PNG file convention for the OG card.

The verification strategy is a new Python file-walk spec (`tests/seo_spec.py`) using no Playwright — pure file I/O against `out/` — which is simpler and faster than browser-driving, and sufficient for SC-1 (HTML tag assertions) and SC-3 (sitemap/robots file content).

**Primary recommendation:** `metadataBase` in root `layout.tsx` + shared OG config via `lib/metadata.ts` constant; `app/opengraph-image.png` static file convention; `app/sitemap.ts` + `app/robots.ts` each with `export const dynamic = 'force-static'`; verification via `web/tests/seo_spec.py` Python file-walk.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| metadataBase + title template | Frontend Server (SSR/static) | — | Lives in `app/layout.tsx`; executed at build time during prerendering |
| Per-page canonical + description | Frontend Server (SSR/static) | — | `generateMetadata` in page.tsx / catch-all; resolved at build time per static param |
| OG/Twitter tags (shared) | Frontend Server (SSR/static) | — | Inherited from root layout `metadata`; all pages share one card |
| OG card PNG (1200×630) | Static / CDN | — | Pre-authored PNG committed to repo; served directly from `out/` |
| sitemap.xml | Build-time Route Handler | — | `app/sitemap.ts` with `force-static`; emits `out/sitemap.xml` |
| robots.txt | Build-time Route Handler | — | `app/robots.ts` with `force-static`; emits `out/robots.txt` |
| SEO verification | CI / test harness | — | Python file-walk `seo_spec.py` asserts on `out/` after `pnpm build` |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `next` (Metadata API) | 16.2.7 (already installed) | `metadataBase`, `metadata` export, `generateMetadata`, `alternates.canonical`, `openGraph`, `twitter` | Native App Router API; zero new deps |
| `next` (file conventions) | 16.2.7 | `app/opengraph-image.png`, `app/sitemap.ts`, `app/robots.ts` | Native; no extra packages needed |

### Supporting

No new npm packages required. All capability is in the already-installed `next@16.2.7`.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `app/sitemap.ts` with `force-static` | Static `public/sitemap.xml` | Public/ file is zero-risk but requires manual enumeration and hand-editing when new routes are added. `app/sitemap.ts` lets you enumerate Fumadocs routes programmatically — but the `force-static` requirement is a known gotcha. Either works. |
| `app/robots.ts` with `force-static` | Static `public/robots.txt` | Robots.txt content is completely static (3 lines). A `public/robots.txt` file is the simpler, lower-risk choice for this specific file. |
| `app/opengraph-image.png` file convention | `public/og.png` referenced via `openGraph.images` absolute URL | Both work under static export. File convention auto-injects correct tags (including width/height) and is scoped to the `app/` segment. `public/og.png` requires manual metadata wiring. File convention is recommended. |
| Authored static PNG | Build-time PNG generation (canvas/sharp/puppeteer) | Build-time generation adds complexity and cross-platform tooling deps. A committed static PNG is simpler and the OG design is stable (one card, no dynamic data). |

**Installation:** No new packages needed.

---

## Package Legitimacy Audit

No new external packages are installed in this phase. All work uses `next@16.2.7` (already installed, already verified in Phase 11). The Python `seo_spec.py` test uses only stdlib (`pathlib`, `re`, `os`, `sys`).

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| next@16.2.7 | npm | Already installed Phase 11 | 100M+/wk | github.com/vercel/next.js | N/A (already verified) | Approved (pre-existing) |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

---

## Architecture Patterns

### System Architecture Diagram

```
pnpm build
    │
    ├── app/layout.tsx              metadataBase + title template + shared OG/Twitter
    │       ↓ inherits to all pages
    ├── app/page.tsx                static `metadata` export (home title/desc/canonical)
    ├── app/docs/[[...slug]]/page   generateMetadata ← source.getPage(slug).data
    ├── app/changelog/[[...slug]]   generateMetadata ← changelogSource.getPage(slug).data
    │
    ├── app/opengraph-image.png     1200×630 PNG → out/opengraph-image.png
    │       ↓ auto-injects          <meta og:image>, og:image:type/width/height
    │
    ├── app/sitemap.ts  (force-static)
    │       ↓ enumerates            source.getPages() + changelogSource.getPages() + home
    │       ↓ emits                 out/sitemap.xml
    │
    └── app/robots.ts  (force-static)
            ↓ emits                 out/robots.txt
                                    User-agent: * Allow: / Sitemap: …/sitemap.xml
```

Static output:
```
out/
  index.html              ← <title>, <meta description>, canonical, og:*, twitter:*
  docs/getting-started/
    index.html            ← per-page title/desc from frontmatter + canonical
  ...
  opengraph-image.png     ← served at /opengraph-image.png (absolute URL in tags)
  sitemap.xml             ← plain XML file (NOT sitemap.xml/index.html)
  robots.txt              ← plain text file (NOT robots.txt/index.html)
```

### Recommended Project Structure

```
web/
├── app/
│   ├── layout.tsx              # MODIFY: add metadataBase + title template + shared OG
│   ├── page.tsx                # MODIFY: add static metadata export
│   ├── opengraph-image.png     # NEW: 1200×630 PNG committed to repo
│   ├── opengraph-image.alt.txt # NEW: alt text for OG image
│   ├── sitemap.ts              # NEW: MetadataRoute.Sitemap + force-static
│   ├── robots.ts               # NEW: MetadataRoute.Robots + force-static
│   ├── docs/[[...slug]]/
│   │   └── page.tsx            # MODIFY: add alternates.canonical to generateMetadata
│   └── changelog/[[...slug]]/
│       └── page.tsx            # MODIFY: add alternates.canonical to generateMetadata
├── lib/
│   └── metadata.ts             # NEW: BASE_URL constant + shared OG config
└── tests/
    └── seo_spec.py             # NEW: Python file-walk spec (no Playwright needed)
```

### Pattern 1: Root Layout metadataBase + Shared OG

**What:** Set `metadataBase` once in root `layout.tsx` so all relative URLs in metadata fields resolve to the production host. Export shared OG/Twitter config at root so every page inherits it.

**When to use:** Always — metadataBase is required for all canonical/OG/Twitter URL resolution.

```typescript
// Source: nextjs.org/docs/app/api-reference/functions/generate-metadata#metadatabase [VERIFIED]
// web/app/layout.tsx
import type { Metadata } from "next";
import { BASE_URL } from "@/lib/metadata";

export const metadata: Metadata = {
  metadataBase: new URL(BASE_URL),
  title: {
    default: "Beekeeper",
    template: "%s | Beekeeper",
  },
  description: "Real-time safety harness for autonomous coding agents.",
  openGraph: {
    siteName: "Beekeeper",
    type: "website",
    images: [
      {
        url: "/opengraph-image.png",   // resolved to absolute via metadataBase
        width: 1200,
        height: 630,
        alt: "Beekeeper — real-time safety harness for autonomous coding agents",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    images: ["/opengraph-image.png"],
  },
};
```

```typescript
// web/lib/metadata.ts — single source of truth
export const BASE_URL = "https://beekeeper.vercel.app";
```

**Key behavior:** `metadataBase: new URL('https://beekeeper.vercel.app')` causes Next.js to resolve any relative URL in a metadata field against it. So `/opengraph-image.png` becomes `https://beekeeper.vercel.app/opengraph-image.png` in the emitted tags. [VERIFIED: nextjs.org/docs/app/api-reference/functions/generate-metadata#metadatabase]

### Pattern 2: Per-Page Static Metadata (Home)

**What:** Export static `metadata` from `app/page.tsx` with title (template-applied), description, and canonical.

```typescript
// web/app/page.tsx
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Beekeeper",   // becomes "Beekeeper | Beekeeper" via template — use title.absolute
  // Actually: use title.absolute to suppress template on the home page
  // OR rely on the root layout's title.default (which doesn't get template applied)
  // RECOMMENDED: do NOT set title in page.tsx; let layout default "Beekeeper" apply.
  description: "Real-time safety harness for autonomous coding agents. Intercepts tool calls before execution and evaluates them against unified threat intelligence.",
  alternates: {
    canonical: "/",     // resolved to https://beekeeper.vercel.app/ via metadataBase
  },
  openGraph: {
    url: "/",
  },
};
```

**Important nuance on title template:** `title.template` in `layout.tsx` applies to `title` (string) in child segments. If the home page sets `title: "Beekeeper"`, the output is `"Beekeeper | Beekeeper"` (doubled). Solutions:
1. Do NOT set `title` in `app/page.tsx` — the root layout `title.default: "Beekeeper"` applies.
2. Use `title: { absolute: "Beekeeper" }` in `app/page.tsx` to bypass the template.
Option 2 is more explicit. [VERIFIED: nextjs.org/docs/app/api-reference/functions/generate-metadata#template]

### Pattern 3: Fumadocs generateMetadata with Canonical

**What:** Extend existing `generateMetadata` in `app/docs/[[...slug]]/page.tsx` and `app/changelog/[[...slug]]/page.tsx` to add `alternates.canonical` and openGraph URL.

**The existing code already sets `title` and `description` from frontmatter.** This pattern ADDS canonical:

```typescript
// Source: current codebase + nextjs.org metadata docs [VERIFIED]
// web/app/docs/[[...slug]]/page.tsx
export async function generateMetadata(props: {
  params: Promise<{ slug?: string[] }>;
}): Promise<Metadata> {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();

  // Build the canonical URL matching trailingSlash: true shape
  const slugPath = params.slug ? params.slug.join("/") + "/" : "";
  const canonicalPath = `/docs/${slugPath}`;

  return {
    title: page.data.title,
    description: page.data.description,
    alternates: {
      canonical: canonicalPath,   // e.g. "/docs/getting-started/"
    },
    openGraph: {
      url: canonicalPath,
    },
  };
}
```

Same pattern applies to `app/changelog/[[...slug]]/page.tsx` with `/changelog/` prefix.

**trailingSlash and canonical:** The canonical path must end with `/` to match the route shape emitted by `trailingSlash: true`. For example, `/docs/getting-started/` not `/docs/getting-started`. [VERIFIED: D-02 + empirical out/ inspection]

### Pattern 4: Static OG Image File Convention

**What:** Place `app/opengraph-image.png` (1200×630) in the `app/` directory. Next.js picks it up and injects `og:image`, `og:image:type`, `og:image:width`, `og:image:height` into every page's `<head>` automatically.

**This works under `output: 'export'`** — the static file convention is not a Route Handler; it's a static asset copy that Next.js processes during build. [VERIFIED: nextjs.org/docs/app/api-reference/file-conventions/metadata/opengraph-image#image-files-jpg-png-gif]

**Output in `out/`:**
- `out/opengraph-image.png` — the file (served at `https://beekeeper.vercel.app/opengraph-image.png`)
- Every `index.html` gets: `<meta property="og:image" content=".../_next/static/media/opengraph-image.HASH.png?... />` (hashed URL in production for cache busting — this is the expected behavior)

**alt text:** Create `app/opengraph-image.alt.txt` containing one line: `Beekeeper — real-time safety harness for autonomous coding agents`. This injects `<meta property="og:image:alt" content="...">`. [VERIFIED: nextjs.org/docs/app/api-reference/file-conventions/metadata/opengraph-image#opengraph-imagealtttxt]

**Dimensions:** The file MUST be exactly 1200×630px. Next.js reads image dimensions and emits the correct `og:image:width` / `og:image:height`. If dimensions differ, the emitted values will differ. Author the PNG at 1200×630.

**IMPORTANT: The root layout's `openGraph.images` array and the file convention interact.** When both are present, the file convention has higher priority (per Next.js docs: "File-based metadata has the higher priority and will override the metadata object"). To avoid duplicate or conflicting `og:image` tags, **choose one approach**: either the file convention (`app/opengraph-image.png`) OR the metadata object (`openGraph.images: [...]` in layout.tsx). The recommended approach is the file convention ONLY — remove the `openGraph.images` from `layout.tsx` when using the file convention.

**OG card design (Claude's discretion):** Per CONTEXT.md, reuse Phase-12 brand tokens. Suggested composition:
- Background: `#0a0d12` (bk-bg dark)
- Wordmark "Beekeeper" in Inter SemiBold, white `#ffffff`
- Tagline "Real-time safety harness for autonomous coding agents" in Inter Regular, `#d4dae3` (bk-fg)
- Amber honeycomb/hive motif from `hero-hive.svg` as right-side decoration, tinted `#e3b341` (bk-amber)
- Teal accent stripe or line, `#39c5cf` (bk-teal)
- Dimensions exactly 1200×630px, PNG format

**Tool for authoring (Windows):** The PNG must be created by the human maintainer or via a script — it cannot be auto-generated by the Next.js build under `output: 'export'`. Options: Figma, Canva, GIMP, `sharp` CLI, or a one-off Node.js script using `canvas` / `puppeteer` run once and committed. The PNG is committed to the repo at `web/app/opengraph-image.png`.

### Pattern 5: sitemap.ts with force-static

**What:** `app/sitemap.ts` using `MetadataRoute.Sitemap`, with `export const dynamic = 'force-static'`, enumerating all public pages from the Fumadocs sources.

**Confirmed behavior with `output: 'export'`:** Requires `export const dynamic = 'force-static'`. Without it, the build errors: `"Page '/sitemap.xml/[[...__metadata_id__]]/route' is missing exported function 'generateStaticParams()'"`. [VERIFIED: github.com/vercel/next.js/discussions/59019 + github.com/vercel/next.js/discussions/73022]

**Output path:** `out/sitemap.xml` — a plain XML file. Because `sitemap.ts` is a Route Handler (not an HTML page route), `trailingSlash: true` does NOT convert it to `sitemap.xml/index.html`. The analogous existing proof: `out/api/search` is an extensionless file (not `out/api/search/index.html`). [VERIFIED: empirical inspection of out/ in this codebase]

**Enumerating Fumadocs routes:** Use `source.getPages()` (from `@/lib/source`) and the changelog source's `source.getPages()` (from `@/lib/changelog-source`). This is the same source that `generateStaticParams()` uses. [CITED: existing codebase pattern in `app/docs/[[...slug]]/page.tsx`]

```typescript
// Source: nextjs.org/docs/app/api-reference/file-conventions/metadata/sitemap [VERIFIED]
// web/app/sitemap.ts
import type { MetadataRoute } from "next";
import { BASE_URL } from "@/lib/metadata";
import { source as docsSource } from "@/lib/source";
import { source as changelogSource } from "@/lib/changelog-source";

export const dynamic = "force-static";

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();

  // Static pages
  const staticPages: MetadataRoute.Sitemap = [
    { url: `${BASE_URL}/`, lastModified: now, changeFrequency: "monthly", priority: 1 },
  ];

  // Docs pages — each slug array → /docs/section/ or /docs/section/sub/
  const docPages: MetadataRoute.Sitemap = docsSource.getPages().map((page) => {
    const slugPath = page.slugs.join("/");
    return {
      url: `${BASE_URL}/docs/${slugPath}/`,
      lastModified: now,
      changeFrequency: "weekly",
      priority: 0.8,
    };
  });

  // Changelog landing + version pages
  const changelogPages: MetadataRoute.Sitemap = [
    { url: `${BASE_URL}/changelog/`, lastModified: now, changeFrequency: "monthly", priority: 0.7 },
    ...changelogSource.getPages().map((page) => {
      const slugPath = page.slugs.join("/");
      return {
        url: `${BASE_URL}/changelog/${slugPath}/`,
        lastModified: now,
        changeFrequency: "monthly",
        priority: 0.6,
      };
    }),
  ];

  return [...staticPages, ...docPages, ...changelogPages];
}
```

**Note on `source.getPages()` vs `source.generateParams()`:** `generateParams()` returns `{ slug: string[] }[]` (suitable for `generateStaticParams`). `getPages()` returns `Page[]` objects with `.slugs` array. Both patterns work. The slug array maps directly to the URL path segments. [CITED: existing codebase — `source.generateParams()` used in `generateStaticParams()`]

**Known sitemap URL count (from out/ inspection):**
- `/` (home)
- `/docs/getting-started/`, `/docs/installation/`, `/docs/configuration/`, `/docs/integration/`, `/docs/security/`, `/docs/cli-reference/`, `/docs/audit-log/`, `/docs/troubleshooting/` (8 docs)
- `/changelog/`, `/changelog/v1.0.0/`, `/changelog/v1.2.0/`, `/changelog/v1.3.0/` (4 changelog)
- Total: 13 URLs (404 and _not-found are NOT included — they are error pages, not public content)

### Pattern 6: robots.ts with force-static

**What:** `app/robots.ts` with `export const dynamic = 'force-static'`.

**Confirmed behavior:** Same `force-static` requirement as sitemap.ts under `output: 'export'`. [VERIFIED: github.com/vercel/next.js/issues/68667 — the reported bug was with canary; in stable Next.js 16, `force-static` resolves it]

**Output path:** `out/robots.txt` — a plain text file. Not affected by `trailingSlash`. [ASSUMED based on Route Handler behavior parity with `out/api/search`]

```typescript
// Source: nextjs.org/docs/app/api-reference/file-conventions/metadata/robots [VERIFIED]
// web/app/robots.ts
import type { MetadataRoute } from "next";
import { BASE_URL } from "@/lib/metadata";

export const dynamic = "force-static";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
    },
    sitemap: `${BASE_URL}/sitemap.xml`,
  };
}
```

**Alternative (lower risk):** Place a static `public/robots.txt` instead. Since robots.txt content is static and trivial (3 lines), this eliminates the `force-static` risk entirely. If `app/robots.ts` causes a build error in CI, fall back to `public/robots.txt`.

### Anti-Patterns to Avoid

- **`app/opengraph-image.tsx` with `ImageResponse`:** This is an Edge runtime Route Handler. Under `output: 'export'`, Edge runtime routes are NOT supported. The build will error or silently skip the OG image. DO NOT use this pattern. [VERIFIED: CONTEXT.md D-03 + Next.js static-exports unsupported features]
- **Setting `openGraph.images` in layout.tsx AND having `app/opengraph-image.png`:** The file convention wins (higher priority per Next.js docs) but having both creates confusion and potentially duplicate/conflicting tags. Use only one.
- **Relative canonical without metadataBase:** Next.js 16 will emit a build error if you use a relative path in `alternates.canonical` without setting `metadataBase`. Always set `metadataBase` in root layout first.
- **Hardcoding `https://beekeeper.vercel.app` in multiple places:** Use the `BASE_URL` constant from `lib/metadata.ts` everywhere — sitemap, robots, OG URL, canonical.
- **Omitting `force-static` from sitemap.ts/robots.ts:** Without it, `pnpm build` fails under `output: 'export'`. The error message is cryptic (mentions `generateStaticParams`) and can waste significant debugging time.
- **Sitemap including 404/ and _not-found/:** These are error pages, not public content. Do not enumerate them. `source.getPages()` only returns actual content pages, not error routes.
- **canonical URL without trailing slash on a trailingSlash: true site:** Canonicals MUST match the emitted URL shape. `/docs/getting-started` (no slash) is wrong; `/docs/getting-started/` (with slash) is correct.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OG/Twitter meta tag injection | Custom `<head>` JSX with `<meta>` tags | `metadata` / `generateMetadata` + `metadataBase` | Next.js handles all tag names, deduplication, inheritance, and streaming vs blocking correctly |
| Sitemap XML generation | Manual XML string concatenation | `app/sitemap.ts` with `MetadataRoute.Sitemap` | Type-safe, correct XML escaping, correct `xmlns` attribute |
| robots.txt content | Manual string in a route.ts | `app/robots.ts` with `MetadataRoute.Robots` | Correct robots.txt syntax guaranteed |
| URL enumeration for sitemap | Walking `out/` filesystem after build | `source.getPages()` from Fumadocs source | Build-time source truth; `out/` doesn't exist when `sitemap.ts` runs |
| Canonical URL generation | Reconstructing URL from window.location | `alternates.canonical` via Next Metadata API | Server-side, static, correct |

**Key insight:** Every metadata concern in this phase is served by `next`'s built-in Metadata API. There is nothing custom to build except the OG card PNG itself (which is a design artifact, not code).

---

## Common Pitfalls

### Pitfall 1: `opengraph-image.tsx` (ImageResponse) fails silently or errors under static export

**What goes wrong:** Developer adds `app/opengraph-image.tsx` using `ImageResponse` from `next/og`. Under `output: 'export'`, this either errors at build time or silently produces no OG image. The OG tags may be missing from all pages.

**Why it happens:** `ImageResponse` runs on the Edge runtime. Static export does not execute Edge or Node.js server routes at runtime — only at prerender time. However, the prerender path also fails because `next/og` depends on Satori/wasm binaries that are not available in all execution contexts.

**How to avoid:** Use the static file convention (`app/opengraph-image.png`) — a literal PNG file committed to the repo.

**Warning signs:** Build completes but no `<meta property="og:image">` in `out/index.html`.

### Pitfall 2: sitemap.ts / robots.ts build error without `force-static`

**What goes wrong:** `pnpm build` fails with: `"Page '/sitemap.xml/[[...__metadata_id__]]/route' is missing exported function 'generateStaticParams()'"` or `"export const dynamic = 'force-static'/export const revalidate not configured on route '/sitemap.xml'"`.

**Why it happens:** Under `output: 'export'`, Next.js requires that every dynamically-rendered route either provides `generateStaticParams` or is marked `force-static`. The `sitemap.ts` and `robots.ts` Route Handlers are not static by default.

**How to avoid:** Add `export const dynamic = 'force-static';` as the first export in both files.

**Warning signs:** Build errors mentioning `/sitemap.xml` or `/robots.txt` route segments.

### Pitfall 3: Title template double-applying on the home page

**What goes wrong:** Root layout sets `title: { template: '%s | Beekeeper', default: 'Beekeeper' }`. Home page also sets `title: 'Beekeeper'`. Result: `<title>Beekeeper | Beekeeper</title>`.

**Why it happens:** The template applies to child segments' `title` string values. Home page IS a child segment of the root layout.

**How to avoid:** Either (a) don't set `title` in `app/page.tsx` (the `default: 'Beekeeper'` from root layout applies without template), or (b) set `title: { absolute: 'Beekeeper' }` in `app/page.tsx`.

**Warning signs:** Home page `<title>` contains "Beekeeper | Beekeeper".

### Pitfall 4: Canonical URL shape mismatching `trailingSlash: true`

**What goes wrong:** Setting `alternates: { canonical: '/docs/getting-started' }` (no trailing slash) on a site with `trailingSlash: true`. The emitted canonical `href` is `/docs/getting-started` but the actual URL served is `/docs/getting-started/`. Search engines see a canonical/URL mismatch.

**Why it happens:** Next.js uses the `alternates.canonical` value verbatim (after resolving against `metadataBase`). It does not automatically append a trailing slash.

**How to avoid:** Always include the trailing slash in canonical paths: `'/docs/getting-started/'`. The Fumadocs slug pattern builds this as: `const slugPath = params.slug.join('/') + '/'; return '/docs/' + slugPath;`.

**Warning signs:** Canonical links in `out/` HTML don't end with `/`.

### Pitfall 5: OG image tags conflict (file convention + metadata object both set)

**What goes wrong:** Root layout sets `openGraph: { images: ['/og.png'] }` AND `app/opengraph-image.png` exists. The file convention overrides the metadata object, but tags from both may appear, causing duplicate `og:image` entries.

**Why it happens:** File-based metadata has higher priority per Next.js docs but both may emit tags in some versions.

**How to avoid:** Choose ONE approach. Recommended: file convention only (`app/opengraph-image.png`). Do NOT set `openGraph.images` in `layout.tsx` if the file convention is used. DO set `openGraph.url`, `openGraph.siteName`, `openGraph.type` — these do not conflict.

**Warning signs:** Multiple `<meta property="og:image">` tags in the emitted HTML.

### Pitfall 6: `source.getPages()` vs `source.generateParams()` in sitemap

**What goes wrong:** Using `source.generateParams()` in `sitemap.ts` returns `[{ slug: string[] }]` objects, not page objects. Treating them as page objects causes runtime errors.

**How to avoid:** Use `source.getPages()` which returns `Page[]` objects with `.slugs` array (and also `.data.title` etc. if needed). Cross-reference with the existing `generateStaticParams()` in `app/docs/[[...slug]]/page.tsx` which correctly calls `source.generateParams()` — but that API returns different shape.

---

## Code Examples

### Verified layout.tsx metadata expansion

```typescript
// Source: nextjs.org/docs/app/api-reference/functions/generate-metadata [VERIFIED]
// web/app/layout.tsx — full metadata block
import type { Metadata } from "next";
import { BASE_URL } from "@/lib/metadata";

export const metadata: Metadata = {
  metadataBase: new URL(BASE_URL),
  title: {
    default: "Beekeeper",
    template: "%s | Beekeeper",
  },
  description: "Real-time safety harness for autonomous coding agents.",
  openGraph: {
    siteName: "Beekeeper",
    type: "website",
    locale: "en_US",
    // NOTE: do NOT set images here if using app/opengraph-image.png file convention
  },
  twitter: {
    card: "summary_large_image",
    // NOTE: do NOT set images here if using app/opengraph-image.png file convention
  },
};
```

### Home page metadata (no title override, just desc + canonical)

```typescript
// web/app/page.tsx addition (does NOT need title — default "Beekeeper" from layout applies)
import type { Metadata } from "next";

export const metadata: Metadata = {
  description: "Real-time safety harness for autonomous coding agents. Intercepts tool calls before execution and evaluates them against corroboration-based threat intelligence.",
  alternates: {
    canonical: "/",   // → https://beekeeper.vercel.app/
  },
  openGraph: {
    url: "/",
  },
};
```

### Docs catch-all generateMetadata with canonical

```typescript
// web/app/docs/[[...slug]]/page.tsx — existing generateMetadata extended
export async function generateMetadata(props: {
  params: Promise<{ slug?: string[] }>;
}): Promise<Metadata> {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();
  // Build trailing-slash canonical: /docs/ (no slug) or /docs/getting-started/
  const slugSuffix = params.slug ? params.slug.join("/") + "/" : "";
  const canonicalPath = `/docs/${slugSuffix}`;
  return {
    title: page.data.title,
    description: page.data.description,
    alternates: { canonical: canonicalPath },
    openGraph: { url: canonicalPath },
  };
}
```

### sitemap.ts skeleton

```typescript
// Source: nextjs.org/docs/app/api-reference/file-conventions/metadata/sitemap [VERIFIED]
// web/app/sitemap.ts
import type { MetadataRoute } from "next";
import { BASE_URL } from "@/lib/metadata";
import { source as docsSource } from "@/lib/source";
import { source as changelogSource } from "@/lib/changelog-source";

export const dynamic = "force-static";  // REQUIRED under output:'export'

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();
  return [
    { url: `${BASE_URL}/`, lastModified: now, changeFrequency: "monthly", priority: 1 },
    ...docsSource.getPages().map((page) => ({
      url: `${BASE_URL}/docs/${page.slugs.join("/")}/${page.slugs.length ? "" : ""}`,
      // Simplify: page.slugs is always non-empty for section pages; index is slugs=[]
      // Handle: if slugs=[] → /docs/  (shouldn't happen; root is handled separately)
      lastModified: now,
      changeFrequency: "weekly" as const,
      priority: 0.8,
    })),
    { url: `${BASE_URL}/changelog/`, lastModified: now, changeFrequency: "monthly", priority: 0.7 },
    ...changelogSource.getPages().filter(p => p.slugs.length > 0).map((page) => ({
      url: `${BASE_URL}/changelog/${page.slugs.join("/")}/ `,
      lastModified: now,
      changeFrequency: "monthly" as const,
      priority: 0.6,
    })),
  ];
}
```

**Note for planner:** The exact `getPages()` return shape needs verification against the actual Fumadocs v16 API. The existing `generateStaticParams()` uses `source.generateParams()` which returns `{slug: string[]}[]`. Use `source.generateParams()` as the canonical enumeration method in `sitemap.ts` for consistency:

```typescript
// safer: reuse the same enumeration as generateStaticParams
const docParams = docsSource.generateParams(); // [{slug:['getting-started']}, ...]
docParams.map(({ slug }) => ({
  url: `${BASE_URL}/docs/${slug.join("/")}/ `,
  ...
}))
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `next export` CLI command | `output: 'export'` in next.config | Next.js 14.0 | `next export` removed; config key is the only way |
| `opengraph-image.tsx` with Edge runtime | Static `.png` file convention for static exports | Next.js 13.3+ (file convention); Edge limitation is inherent | Edge runtime not supported in static export |
| `next-sitemap` npm package | Built-in `app/sitemap.ts` | Next.js 13.3+ | No extra dep needed; built-in handles XML format |
| Manual robots.txt in public/ | Built-in `app/robots.ts` | Next.js 13.3+ | Code-generated, type-safe |
| `title` string in every page | `title.template` in root layout | Next.js 13.2+ (App Router) | DRY title pattern — "Page | Site" |
| `head.tsx` component for OG tags | `metadata` object / `generateMetadata` | Next.js 13.2+ | `<Head>` component is pages-router; App Router uses metadata export |

**Deprecated/outdated patterns to avoid:**
- `<Head>` component from `next/head` — Pages Router only; does not work in App Router
- `next export` CLI — removed in Next.js 14; use `output: 'export'` config
- `themeColor` in metadata object — deprecated Next.js 14+; use `generateViewport` instead

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `app/robots.ts` with `force-static` emits `out/robots.txt` as a file (not `out/robots.txt/index.html`) under `trailingSlash: true` | Pattern 6 | If wrong, robots.txt is served as a directory; fallback: use `public/robots.txt` |
| A2 | `docsSource.getPages()` returns objects with a `.slugs` array matching the route slug segments | Pattern 5 + Code Examples | If `.slugs` doesn't exist, sitemap generation will error; use `source.generateParams()` instead as the safer alternative |
| A3 | The `app/opengraph-image.png` file convention with `output: 'export'` injects absolute `og:image` URL (hashed) into all pages' `<head>`, not just the root page | Pattern 4 | If only root page gets the tag, all other pages need explicit `openGraph.images` in their metadata |

**If this table is empty:** N/A — three assumptions recorded above.

---

## Open Questions (RESOLVED)

> All three were resolved at planning time and the resolutions are implemented by the Phase 17 plans (17-01/02/03). Retained here for traceability.

1. **`source.getPages()` vs `source.generateParams()` API shape**
   - What we know: Both exist on the Fumadocs `loader()` return; `generateParams()` is used in `generateStaticParams()` already
   - What's unclear: The exact property name for the slug array on objects returned by `getPages()` — is it `.slugs`, `.slug`, or another property?
   - Recommendation: In `sitemap.ts`, use `source.generateParams()` (returns `{slug: string[]}[]`) for consistency with the existing codebase — this is verified to work since `app/docs/[[...slug]]/page.tsx` already uses it.
   - **RESOLVED: use `source.generateParams()`** (the proven path) — implemented in 17-03 (sitemap.ts enumerates docs/changelog via `generateParams()`, not `getPages()`).

2. **OG image PNG authoring tool**
   - What we know: Must be a 1200×630 PNG committed to `web/app/opengraph-image.png`; cannot be auto-generated at build time under `output: 'export'`
   - What's unclear: Whether the maintainer prefers to design it in Figma, or whether a one-off Node.js `canvas`/`sharp` generation script is acceptable
   - Recommendation: Planner should include a `checkpoint:human-verify` task for maintainer to author and commit the PNG. The plan can stub `web/app/opengraph-image.png` with a placeholder and the maintainer replaces it. SC-2 should pass once any 1200×630 PNG is present.
   - **RESOLVED: `checkpoint:human-verify` + placeholder stub** — implemented in 17-02 Task 3 (a placeholder 1200×630 PNG lands first; the maintainer authors/approves the final card at a blocking human checkpoint).

3. **`changelog/index` page canonical path**
   - What we know: The changelog landing page is at `out/changelog/index.html` (URL: `/changelog/`). The source has a `content/changelog/index.mdx` with `generateStaticParams()` producing an empty slug.
   - What's unclear: Whether `source.generateParams()` for the changelog source returns `{slug: []}` (empty array, representing the landing) or omits it (since the landing may be the `index.mdx`)
   - Recommendation: Test in the plan by inspecting what `changelogSource.generateParams()` returns. For the sitemap, add the `/changelog/` entry as a hardcoded static entry (safe regardless).
   - **RESOLVED: hardcoded `/changelog/` static entry in sitemap.ts** — implemented in 17-03 (the landing URL is enumerated explicitly so it is present regardless of how `generateParams()` represents the empty slug).

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `next@16.2.7` | All metadata API, sitemap.ts, robots.ts | ✓ | 16.2.7 | — |
| `fumadocs-core@16.9.3` | `source.getPages()` / `generateParams()` in sitemap.ts | ✓ | 16.9.3 | — |
| Image editor / PNG authoring | `app/opengraph-image.png` | Manual (human) | — | Placeholder PNG (1200×630 solid color) as stub |
| Python stdlib (`pathlib`, `re`, `os`) | `seo_spec.py` test harness | ✓ | Built-in | — |
| Python `playwright` | NOT required for seo_spec.py | N/A | — | File-walk is used instead |

**Missing dependencies with no fallback:** None — all automated tasks use existing installed packages.

**Missing dependencies with fallback:**
- OG PNG image: human author step (checkpoint:human-verify in plan); placeholder PNG can be committed as a stub to pass SC-2 at the file-presence level.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Python stdlib (pathlib, re) — no new framework needed |
| Config file | none |
| Quick run command | `cd web && python tests/seo_spec.py` |
| Full suite command | `cd web && python tests/seo_spec.py && python tests/home_spec.py && python tests/gfx_spec.py` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEO-01 (SC-1) | Every `out/**/*.html` has `<title>`, `<meta name="description">`, `<link rel="canonical">` | file-walk (Python) | `cd web && python tests/seo_spec.py` | ❌ Wave 0 — create `web/tests/seo_spec.py` |
| SEO-01 (SC-2) | Every `out/**/*.html` has `<meta property="og:image">`, `<meta name="twitter:card">`, and `out/opengraph-image.png` exists at ≥ file level; social render = manual UAT | file-walk + manual | `cd web && python tests/seo_spec.py` (automated part); Twitter/LinkedIn card preview = maintainer UAT | ❌ Wave 0 |
| SEO-01 (SC-3) | `out/sitemap.xml` lists ≥ 13 URLs matching `https://beekeeper.vercel.app/`; `out/robots.txt` contains `Allow: /` and references `sitemap.xml` | file-walk (Python) | `cd web && python tests/seo_spec.py` | ❌ Wave 0 |

### seo_spec.py Design (no Playwright required)

```python
# Approach: walk out/ after pnpm build; assert on file contents
import pathlib, re, sys, os

OUT = pathlib.Path("web/out")  # or discover from __file__

# SC-1: every HTML page has title, description, canonical
def check_html_metadata():
    html_files = [p for p in OUT.rglob("index.html")
                  if "404" not in str(p) and "_not-found" not in str(p)]
    for path in html_files:
        content = path.read_text(encoding="utf-8")
        # title
        assert re.search(r'<title>[^<]+</title>', content), f"MISSING title in {path}"
        # description
        assert re.search(r'<meta name="description" content="[^"]+"', content), f"MISSING description in {path}"
        # canonical
        assert re.search(r'<link rel="canonical" href="https://beekeeper\.vercel\.app/', content), f"MISSING canonical in {path}"

# SC-2: OG/Twitter tags + OG file exists
def check_og():
    assert (OUT / "opengraph-image.png").exists(), "MISSING out/opengraph-image.png"
    html_files = [p for p in OUT.rglob("index.html")
                  if "404" not in str(p) and "_not-found" not in str(p)]
    for path in html_files:
        content = path.read_text(encoding="utf-8")
        assert re.search(r'<meta property="og:image"', content), f"MISSING og:image in {path}"
        assert re.search(r'<meta name="twitter:card"', content), f"MISSING twitter:card in {path}"

# SC-3: sitemap.xml + robots.txt
def check_sitemap_robots():
    sitemap = (OUT / "sitemap.xml").read_text(encoding="utf-8")
    assert "https://beekeeper.vercel.app/" in sitemap, "MISSING home URL in sitemap.xml"
    assert sitemap.count("<loc>") >= 13, f"Sitemap has fewer than 13 URLs"
    robots = (OUT / "robots.txt").read_text(encoding="utf-8")
    assert "Allow: /" in robots, "MISSING 'Allow: /' in robots.txt"
    assert "sitemap.xml" in robots, "MISSING sitemap reference in robots.txt"
```

### Sampling Rate

- **Per task commit:** `cd web && python tests/seo_spec.py`
- **Per wave merge:** Full seo_spec.py + home_spec.py + gfx_spec.py
- **Phase gate:** Full suite green before `/gsd-verify-work 17`

### Wave 0 Gaps

- [ ] `web/tests/seo_spec.py` — covers SC-1, SC-2, SC-3
- [ ] `web/lib/metadata.ts` — `BASE_URL` constant
- [ ] `web/app/opengraph-image.png` — 1200×630 PNG (human-authored; checkpoint:human-verify)
- [ ] `web/app/opengraph-image.alt.txt` — one line of alt text
- [ ] `web/app/sitemap.ts` — MetadataRoute.Sitemap with force-static
- [ ] `web/app/robots.ts` — MetadataRoute.Robots with force-static

*(If no gaps: N/A — gaps listed above)*

---

## Manual UAT Items

The following cannot be automated headlessly and require a maintainer check:

1. **Social card preview render (SC-2 partial):** Paste `https://beekeeper.vercel.app` into the Twitter Card Validator (`cards-dev.twitter.com/validator`) or LinkedIn Post Inspector (`linkedin.com/post-inspector`) after SITE-03 (live deploy) is complete. Verify the card renders at 1200×630 with correct image. **This is blocked on SITE-03 (deferred)** — note as a post-deploy UAT item, not a Phase 17 gate.

2. **OG image visual quality review:** Maintainer authors the PNG and confirms it renders correctly at 1200×630 with brand colors and readable text before committing.

---

## Security Domain

This phase adds metadata and static files only — no authentication, no user input, no cryptography, no server-side data handling. The security surface is minimal:

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no (no user input) | — |
| V6 Cryptography | no | — |

**Honesty constraint (D-04):** Meta descriptions must not claim the product is live/downloadable at `https://beekeeper.vercel.app` (repo unpushed). Reuse the honest framing from Phases 14/15. Specifically: the `go install` path in docs uses `github.com/bantuson/beekeeper` but the repo is currently unpushed. Meta descriptions should describe the product's purpose and capabilities, not claim installation instructions are live today.

---

## Sources

### Primary (HIGH confidence)

- [nextjs.org/docs/app/api-reference/functions/generate-metadata](https://nextjs.org/docs/app/api-reference/functions/generate-metadata) — metadataBase, title template, alternates.canonical, openGraph, twitter — fetched directly, version 16.2.7
- [nextjs.org/docs/app/api-reference/file-conventions/metadata/opengraph-image](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/opengraph-image) — static file convention (opengraph-image.png) vs dynamic (opengraph-image.tsx) — fetched directly, version 16.2.7
- [nextjs.org/docs/app/api-reference/file-conventions/metadata/sitemap](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/sitemap) — MetadataRoute.Sitemap, force-static — fetched directly, version 16.2.7
- [nextjs.org/docs/app/api-reference/file-conventions/metadata/robots](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/robots) — MetadataRoute.Robots — fetched directly, version 16.2.7
- [nextjs.org/docs/app/guides/static-exports](https://nextjs.org/docs/app/guides/static-exports) — supported/unsupported features list — fetched directly, version 16.2.7
- [nextjs.org/docs/app/getting-started/metadata-and-og-images](https://nextjs.org/docs/app/getting-started/metadata-and-og-images) — static OG file convention placement — fetched directly, version 16.2.7

### Secondary (MEDIUM confidence)

- [github.com/vercel/next.js/discussions/59019](https://github.com/vercel/next.js/discussions/59019) — confirmed that `sitemap.ts` needs `force-static` under `output: 'export'`; solution posted February 2026
- [github.com/vercel/next.js/discussions/73022](https://github.com/vercel/next.js/discussions/73022) — collaborator verified `force-static` fix for sitemap; discussion thread 2024-2025
- [github.com/vercel/next.js/issues/68667](https://github.com/vercel/next.js/issues/68667) — robots.ts build failure under static export in Next.js 15 canary; confirmed `force-static` as resolution in stable releases

### Tertiary (LOW confidence — informational only)

- Empirical codebase inspection: `out/api/search` is an extensionless file (not a directory), confirming Route Handlers emit extensionless outputs even with `trailingSlash: true` — this is the basis for A1 claim that `sitemap.xml` and `robots.txt` will similarly be extensionless files

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all capabilities are in already-installed Next.js 16.2.7; verified from official docs
- Architecture: HIGH — patterns verified from official Next.js 16 docs + existing codebase inspection
- Pitfalls: HIGH — `force-static` requirement is a confirmed, widely-reported issue with documented solutions; other pitfalls derived from official docs behavior
- `source.getPages()` API shape: MEDIUM — behavior verified conceptually from Fumadocs source patterns but exact property names not independently confirmed (A2 assumption)

**Research date:** 2026-06-09
**Valid until:** 2026-09-09 (stable APIs; only risk is Fumadocs or Next.js major version bump)
