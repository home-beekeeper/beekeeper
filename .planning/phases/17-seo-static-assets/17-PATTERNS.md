# Phase 17: SEO & Static Assets — Pattern Map

**Mapped:** 2026-06-09
**Files analyzed:** 9 new/modified files
**Analogs found:** 8 / 9 (1 purely-new with no analog: `web/lib/metadata.ts`)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/app/layout.tsx` (MODIFY) | config / root layout | request-response | `web/app/layout.tsx` (self — current state) | self (extend) |
| `web/app/page.tsx` (MODIFY) | route / page | request-response | `web/app/docs/[[...slug]]/page.tsx` | role-match (static metadata export) |
| `web/lib/metadata.ts` (CREATE) | utility / constant | — | none in codebase | no analog |
| `web/app/opengraph-image.png` (CREATE) | static asset | — | `web/public/` assets (Phase 12) | asset-match |
| `web/app/opengraph-image.alt.txt` (CREATE) | static asset | — | file convention, no codebase analog | no analog |
| `web/app/sitemap.ts` (CREATE) | route handler | batch / transform | `web/app/docs/[[...slug]]/page.tsx` generateStaticParams | role-match (source enumeration) |
| `web/app/robots.ts` (CREATE) | route handler | request-response | `web/app/sitemap.ts` (sibling, same pattern) | exact (once sitemap.ts exists) |
| `web/app/docs/[[...slug]]/page.tsx` (MODIFY) | route / catch-all | request-response | self (extend generateMetadata) | self (extend) |
| `web/app/changelog/[[...slug]]/page.tsx` (MODIFY) | route / catch-all | request-response | `web/app/docs/[[...slug]]/page.tsx` | exact |
| `web/tests/seo_spec.py` (CREATE) | test / harness | file-I/O | `web/tests/gfx_spec.py` | exact |

---

## Pattern Assignments

### `web/app/layout.tsx` (MODIFY — add metadataBase + title template + shared OG/Twitter)

**Analog:** `web/app/layout.tsx` (current file — extend in place)

**Current metadata export** (lines 19–22) — this is what exists today and must be replaced:
```typescript
export const metadata: Metadata = {
  title: "Beekeeper",
  description: "Real-time safety harness for autonomous coding agents.",
};
```

**Current imports block** (lines 1–4) — add `BASE_URL` import here:
```typescript
import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { Providers } from "./providers";
import "./globals.css";
```

**Pattern to write (replace lines 19–22):**
```typescript
// Add to imports at top:
import { BASE_URL } from "@/lib/metadata";

// Replace the metadata export:
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
    // NOTE: do NOT set openGraph.images — the app/opengraph-image.png file
    // convention has higher priority and adding both causes duplicate og:image tags.
  },
  twitter: {
    card: "summary_large_image",
    // NOTE: same — do NOT set twitter.images; file convention handles it.
  },
};
```

**Do NOT touch** (lines 6–17, 24–46): font setup, `html`/`body` structure, `suppressHydrationWarning`, skip-link, `<Providers>` — these are Phase-12 locked.

---

### `web/app/page.tsx` (MODIFY — add static metadata export)

**Analog:** `web/app/docs/[[...slug]]/page.tsx` (lines 7, 16–26 — static metadata pattern)

**Current file** (lines 1–32): no `metadata` export at all — it exports only the default `Home` component.

**Pattern to add** (insert before the `export default function Home()`):
```typescript
// Add to imports at top:
import type { Metadata } from "next";

// Add this export before the default function:
export const metadata: Metadata = {
  // Do NOT set title here — the root layout's title.default "Beekeeper" applies
  // without the template. Setting title: "Beekeeper" here would produce
  // "Beekeeper | Beekeeper" via the template (Pitfall 3).
  description:
    "Real-time safety harness for autonomous coding agents. Intercepts tool calls before execution and evaluates them against corroboration-based threat intelligence.",
  alternates: {
    canonical: "/",   // resolves to https://beekeeper.vercel.app/ via metadataBase
  },
  openGraph: {
    url: "/",
  },
};
```

---

### `web/lib/metadata.ts` (CREATE — BASE_URL constant)

**Analog:** none in codebase. This is a new utility constant file.

**Pattern (full file):**
```typescript
// web/lib/metadata.ts
// Single source of truth for the canonical base URL.
// Used by: app/layout.tsx (metadataBase), app/sitemap.ts, app/robots.ts,
//          and any generateMetadata that builds absolute canonical paths.
// Change this ONE place when a custom domain replaces the Vercel project URL.
export const BASE_URL = "https://beekeeper.vercel.app";
```

**Import convention used in this codebase** (from `web/lib/source.ts` and `web/lib/changelog-source.ts`):
- Files under `web/lib/` are imported via the `@/lib/` alias (e.g. `import { source } from "@/lib/source"`)
- Keep the file minimal — constants only, no side effects

---

### `web/app/opengraph-image.png` (CREATE — static 1200×630 PNG)

**Analog:** static assets in `web/public/` — same convention of committed binary files.

**No code pattern.** This is a design artifact authored by the maintainer (or via a one-off script). Constraints:
- Exactly 1200×630 pixels
- PNG format
- Location: `web/app/opengraph-image.png` (NOT `web/public/`)
- Brand tokens: bg `#0a0d12`, amber `#e3b341`, teal `#39c5cf`, fg `#d4dae3`, white `#ffffff`; fonts Inter + JetBrains Mono

**Plan note:** Stub with a placeholder 1200×630 solid-color PNG for SC-2 file-presence gate; add `checkpoint:human-verify` for the maintainer to replace with the branded design.

---

### `web/app/opengraph-image.alt.txt` (CREATE — OG image alt text)

**Analog:** Next.js file convention, no codebase analog.

**Full file content:**
```
Beekeeper — real-time safety harness for autonomous coding agents
```
(Single line, no trailing newline required. Next.js injects this as `<meta property="og:image:alt" content="...">` automatically.)

---

### `web/app/sitemap.ts` (CREATE — MetadataRoute.Sitemap + force-static)

**Analog:** `web/app/docs/[[...slug]]/page.tsx` lines 12–14 — `source.generateParams()` enumeration pattern is directly reusable.

**Key import shape from analog** (`web/app/docs/[[...slug]]/page.tsx` lines 1–9):
```typescript
import { source } from "@/lib/source";
// (changelog page uses:)
import { source } from "@/lib/changelog-source";
```

**`generateStaticParams` in analog** (lines 12–14) — the proven enumeration call:
```typescript
export async function generateStaticParams() {
  return source.generateParams();
}
```
`source.generateParams()` returns `{ slug: string[] }[]`. Use this same call in `sitemap.ts` — it is already proven to enumerate all pages correctly.

**Full pattern for `web/app/sitemap.ts`:**
```typescript
import type { MetadataRoute } from "next";
import { BASE_URL } from "@/lib/metadata";
import { source as docsSource } from "@/lib/source";
import { source as changelogSource } from "@/lib/changelog-source";

// REQUIRED under output:'export' — without this the build errors:
// "Page '/sitemap.xml/[[...__metadata_id__]]/route' is missing generateStaticParams()"
export const dynamic = "force-static";

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();

  // Home
  const staticPages: MetadataRoute.Sitemap = [
    { url: `${BASE_URL}/`, lastModified: now, changeFrequency: "monthly", priority: 1 },
  ];

  // Docs — reuse same generateParams() the catch-all route uses
  const docsParams = docsSource.generateParams(); // [{ slug: ["getting-started"] }, ...]
  const docPages: MetadataRoute.Sitemap = docsParams.map(({ slug }) => ({
    url: `${BASE_URL}/docs/${slug.join("/")}/ `,
    lastModified: now,
    changeFrequency: "weekly" as const,
    priority: 0.8,
  }));

  // Changelog landing (hardcoded — safer than relying on slug=[])
  // + version pages from source
  const changelogParams = changelogSource.generateParams();
  const changelogVersionPages: MetadataRoute.Sitemap = changelogParams
    .filter(({ slug }) => slug.length > 0)
    .map(({ slug }) => ({
      url: `${BASE_URL}/changelog/${slug.join("/")}/ `,
      lastModified: now,
      changeFrequency: "monthly" as const,
      priority: 0.6,
    }));

  return [
    ...staticPages,
    ...docPages,
    { url: `${BASE_URL}/changelog/`, lastModified: now, changeFrequency: "monthly", priority: 0.7 },
    ...changelogVersionPages,
  ];
}
```

**Note on trailing space in URL strings above:** The research examples include a trailing space in the template literal (e.g. `` `${BASE_URL}/docs/${slug.join("/")}/ ` ``). Remove the space — that is a research transcription artefact. The URL must end with `/` (no space).

---

### `web/app/robots.ts` (CREATE — MetadataRoute.Robots + force-static)

**Analog:** `web/app/sitemap.ts` (sibling, same pattern). Also mirrors `force-static` route handler shape.

**Full pattern for `web/app/robots.ts`:**
```typescript
import type { MetadataRoute } from "next";
import { BASE_URL } from "@/lib/metadata";

// Same force-static requirement as sitemap.ts under output:'export'
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

**Fallback:** If `app/robots.ts` causes a build error (Assumption A1 — output path may be `robots.txt/index.html` under `trailingSlash`), fall back to a static `web/public/robots.txt` file:
```
User-agent: *
Allow: /
Sitemap: https://beekeeper.vercel.app/sitemap.xml
```

---

### `web/app/docs/[[...slug]]/page.tsx` (MODIFY — extend generateMetadata with canonical)

**Analog:** self (current file)

**Current generateMetadata** (lines 16–26) — copy this shape and extend it:
```typescript
export async function generateMetadata(props: {
  params: Promise<{ slug?: string[] }>;
}): Promise<Metadata> {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();
  return {
    title: page.data.title,
    description: page.data.description,
  };
}
```

**Pattern to write (replace the return object only):**
```typescript
  // Build trailing-slash canonical — must match trailingSlash:true emitted shape
  const slugSuffix = params.slug ? params.slug.join("/") + "/" : "";
  const canonicalPath = `/docs/${slugSuffix}`;

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
```

**Leave untouched:** imports (lines 1–10), `generateStaticParams` (lines 12–14), the `Page` component (lines 28–47).

---

### `web/app/changelog/[[...slug]]/page.tsx` (MODIFY — extend generateMetadata with canonical)

**Analog:** `web/app/docs/[[...slug]]/page.tsx` — identical shape; only difference is the `/changelog/` prefix and the source import.

**Current generateMetadata** (lines 16–26 — identical structure to docs):
```typescript
export async function generateMetadata(props: {
  params: Promise<{ slug?: string[] }>;
}): Promise<Metadata> {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) notFound();
  return {
    title: page.data.title,
    description: page.data.description,
  };
}
```

**Pattern to write (replace the return object only):**
```typescript
  const slugSuffix = params.slug ? params.slug.join("/") + "/" : "";
  const canonicalPath = `/changelog/${slugSuffix}`;

  return {
    title: page.data.title,
    description: page.data.description,
    alternates: {
      canonical: canonicalPath,   // e.g. "/changelog/v1.2.0/"
    },
    openGraph: {
      url: canonicalPath,
    },
  };
```

---

### `web/tests/seo_spec.py` (CREATE — Python file-walk spec)

**Analog:** `web/tests/gfx_spec.py` — exact scaffold to copy. The SEO spec differs in that it needs NO Playwright (pure file-walk against `out/`) so the HTTP server + browser boilerplate is dropped entirely.

**Reusable scaffold from `web/tests/gfx_spec.py`:**

Path resolution pattern (lines 45–53):
```python
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
WEB_DIR = os.path.dirname(SCRIPT_DIR)
OUT_DIR = os.path.join(WEB_DIR, "out")

if not os.path.isdir(OUT_DIR):
    print(f"FAIL: out/ directory not found at {OUT_DIR} — run `pnpm build` first")
    sys.exit(1)
```

Test runner helpers (lines 81–90):
```python
failures = []

def fail(msg: str) -> None:
    failures.append(msg)
    print(f"  FAIL: {msg}")

def ok(msg: str) -> None:
    print(f"  PASS: {msg}")
```

Main exit contract (lines 251–260):
```python
if __name__ == "__main__":
    print("=== Phase XX ... ===")
    run_tests()
    print()
    if failures:
        print(f"RESULT: FAILED — {len(failures)} assertion(s) failed:")
        for f in failures:
            print(f"  - {f}")
        sys.exit(1)
    else:
        print("RESULT: ALL ASSERTIONS PASSED")
        sys.exit(0)
```

**Full pattern for `web/tests/seo_spec.py`** (no Playwright — pure pathlib + re):
```python
"""
web/tests/seo_spec.py — Phase-17 SEO & Static Assets (SEO-01 SC-1..3) file-walk harness.

No Playwright required — pure file I/O against web/out/ after `pnpm build`.

REQUIREMENT COVERAGE:
  SC-1  Every out/**/*.html (excluding 404/_not-found) has <title>, <meta name="description">,
        <link rel="canonical" href="https://beekeeper.vercel.app/...">.
  SC-2  Every out/**/*.html has <meta property="og:image"> + <meta name="twitter:card">;
        out/opengraph-image.png exists.
  SC-3  out/sitemap.xml exists with >= 13 <loc> entries all matching BASE_URL;
        out/robots.txt contains "Allow: /" and references "sitemap.xml".

Usage:
  cd web && python tests/seo_spec.py
  Exit 0 = all assertions PASSED; exit 1 = one or more FAILED.

Prerequisites:
  - web/out/ must exist (run `pnpm build` first)
  - Python stdlib only (pathlib, re, os, sys) — no pip install required
"""

import os
import pathlib
import re
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
WEB_DIR = os.path.dirname(SCRIPT_DIR)
OUT_DIR = pathlib.Path(WEB_DIR) / "out"
BASE_URL = "https://beekeeper.vercel.app"

if not OUT_DIR.is_dir():
    print(f"FAIL: out/ directory not found at {OUT_DIR} — run `pnpm build` first")
    sys.exit(1)

failures = []

def fail(msg: str) -> None:
    failures.append(msg)
    print(f"  FAIL: {msg}")

def ok(msg: str) -> None:
    print(f"  PASS: {msg}")

def content_html_files():
    """Return all index.html files excluding error pages."""
    return [
        p for p in OUT_DIR.rglob("index.html")
        if "404" not in str(p) and "_not-found" not in str(p)
    ]

def sc1_html_metadata():
    print("\n[SC-1] Every page has <title>, <meta name='description'>, <link rel='canonical'>")
    for path in content_html_files():
        content = path.read_text(encoding="utf-8")
        rel = path.relative_to(OUT_DIR)
        if not re.search(r"<title>[^<]+</title>", content):
            fail(f"MISSING <title> in {rel}")
        else:
            ok(f"<title> present in {rel}")
        if not re.search(r'<meta name="description" content="[^"]+"', content):
            fail(f"MISSING <meta name=\"description\"> in {rel}")
        else:
            ok(f"<meta description> present in {rel}")
        if not re.search(
            rf'<link rel="canonical" href="{re.escape(BASE_URL)}/', content
        ):
            fail(f"MISSING canonical href={BASE_URL}/... in {rel}")
        else:
            ok(f"<link rel=canonical> present in {rel}")

def sc2_og_twitter():
    print("\n[SC-2] OG/Twitter tags + opengraph-image.png file present")
    og_file = OUT_DIR / "opengraph-image.png"
    if og_file.exists():
        ok(f"out/opengraph-image.png exists ({og_file.stat().st_size} bytes)")
    else:
        fail("MISSING out/opengraph-image.png")
    for path in content_html_files():
        content = path.read_text(encoding="utf-8")
        rel = path.relative_to(OUT_DIR)
        if not re.search(r'<meta property="og:image"', content):
            fail(f"MISSING og:image in {rel}")
        else:
            ok(f"og:image present in {rel}")
        if not re.search(r'<meta name="twitter:card"', content):
            fail(f"MISSING twitter:card in {rel}")
        else:
            ok(f"twitter:card present in {rel}")

def sc3_sitemap_robots():
    print("\n[SC-3] sitemap.xml >= 13 URLs + robots.txt with Allow + sitemap ref")
    sitemap_path = OUT_DIR / "sitemap.xml"
    if not sitemap_path.exists():
        fail("MISSING out/sitemap.xml")
    else:
        sitemap = sitemap_path.read_text(encoding="utf-8")
        if BASE_URL + "/" in sitemap:
            ok(f"sitemap.xml contains {BASE_URL}/")
        else:
            fail(f"sitemap.xml missing {BASE_URL}/")
        url_count = len(re.findall(r"<loc>", sitemap))
        if url_count >= 13:
            ok(f"sitemap.xml has {url_count} <loc> entries (>= 13)")
        else:
            fail(f"sitemap.xml has only {url_count} <loc> entries (expected >= 13)")
        bad_urls = [
            u for u in re.findall(r"<loc>([^<]+)</loc>", sitemap)
            if not u.startswith(BASE_URL)
        ]
        if bad_urls:
            fail(f"sitemap.xml has {len(bad_urls)} URL(s) not starting with {BASE_URL}: {bad_urls[:3]}")
        else:
            ok(f"All sitemap URLs start with {BASE_URL}")

    robots_path = OUT_DIR / "robots.txt"
    if not robots_path.exists():
        fail("MISSING out/robots.txt")
    else:
        robots = robots_path.read_text(encoding="utf-8")
        if "Allow: /" in robots:
            ok("robots.txt contains 'Allow: /'")
        else:
            fail("MISSING 'Allow: /' in robots.txt")
        if "sitemap.xml" in robots:
            ok("robots.txt references sitemap.xml")
        else:
            fail("MISSING sitemap.xml reference in robots.txt")

def run_tests():
    sc1_html_metadata()
    sc2_og_twitter()
    sc3_sitemap_robots()

if __name__ == "__main__":
    print("=== Phase 17 SEO & Static Assets — SC-1..3 File-Walk Harness ===")
    run_tests()
    print()
    if failures:
        print(f"RESULT: FAILED — {len(failures)} assertion(s) failed:")
        for f in failures:
            print(f"  - {f}")
        sys.exit(1)
    else:
        print("RESULT: ALL ASSERTIONS PASSED")
        sys.exit(0)
```

---

## Shared Patterns

### Source Enumeration (sitemap.ts)
**Source:** `web/app/docs/[[...slug]]/page.tsx` lines 12–14 and `web/app/changelog/[[...slug]]/page.tsx` lines 12–14
**Apply to:** `web/app/sitemap.ts`
```typescript
// Proven enumeration call — same API used by both catch-all routes
export async function generateStaticParams() {
  return source.generateParams();  // returns { slug: string[] }[]
}
// In sitemap.ts, call the same method:
const params = docsSource.generateParams(); // [{ slug: ["getting-started"] }, ...]
```

### `@/lib/` Import Convention
**Source:** `web/lib/source.ts` (line 1: `import { docs } from "collections/server"`) and `web/lib/changelog-source.ts`
**Apply to:** `web/lib/metadata.ts`, `web/app/sitemap.ts`, `web/app/robots.ts`
- All `web/lib/` files are imported via `@/lib/<name>` alias
- Keep lib files minimal — one concern per file

### Python Spec Harness Scaffold
**Source:** `web/tests/gfx_spec.py` lines 45–53 (path resolution), 81–90 (fail/ok helpers), 251–260 (exit contract)
**Apply to:** `web/tests/seo_spec.py`
- `SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))` — always use for platform-safe absolute paths
- `WEB_DIR = os.path.dirname(SCRIPT_DIR)` — derive web root from script location
- `OUT_DIR` guard with `sys.exit(1)` if missing
- `failures = []` list; `fail()` appends + prints; `ok()` prints only
- `sys.exit(1)` on any failure, `sys.exit(0)` on all-pass

### `next.config.mjs` constraints (do NOT regress)
**Source:** `web/next.config.mjs` lines 11–28
```javascript
output: "export",        // must stay — all SEO work must work under this constraint
trailingSlash: true,     // must stay — canonical URLs MUST end with /
images: { unoptimized: true },  // must stay
transpilePackages: ["three", "@react-three/fiber", "@react-three/drei"],  // must stay
```
None of these lines are touched in Phase 17.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `web/lib/metadata.ts` | utility / constant | — | No existing constant/config utility files in `web/lib/` (source files are loader wrappers, not constant exports) |
| `web/app/opengraph-image.alt.txt` | static asset | — | Next.js file convention; no similar `.alt.txt` files exist in the repo |

---

## Key Anti-Patterns (from RESEARCH.md — planner must enforce)

| Anti-Pattern | Consequence | What to Do Instead |
|---|---|---|
| `app/opengraph-image.tsx` with `ImageResponse` | Edge runtime — fails under `output:'export'` | Use static `app/opengraph-image.png` file |
| `openGraph.images` in `layout.tsx` + `app/opengraph-image.png` | Duplicate/conflicting `og:image` tags | Use file convention ONLY; omit `openGraph.images` from layout |
| `export const dynamic` omitted from `sitemap.ts`/`robots.ts` | `pnpm build` fails with cryptic `generateStaticParams` error | Always add `export const dynamic = "force-static"` as first export |
| Canonical without trailing slash | Canonical/URL mismatch on `trailingSlash:true` site | Always end canonical paths with `/` e.g. `/docs/getting-started/` |
| `title: "Beekeeper"` in `app/page.tsx` | `<title>Beekeeper \| Beekeeper</title>` (doubled) | Do NOT set `title` in `page.tsx`; let root layout `title.default` apply |

---

## Metadata

**Analog search scope:** `web/app/`, `web/lib/`, `web/tests/`, `web/next.config.mjs`
**Files read:** 9 source files
**Pattern extraction date:** 2026-06-09
