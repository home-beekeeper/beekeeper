---
phase: 13-docs-content-pipeline
reviewed: 2026-06-08T15:30:00Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - web/next.config.mjs
  - web/source.config.ts
  - web/tsconfig.json
  - web/package.json
  - pnpm-workspace.yaml
  - web/lib/source.ts
  - web/app/docs/layout.tsx
  - web/app/docs/[[...slug]]/page.tsx
  - web/app/api/search/route.ts
  - web/app/providers.tsx
  - web/app/globals.css
  - web/content/docs/meta.json
  - web/content/docs/getting-started/index.mdx
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 13: Code Review Report

**Reviewed:** 2026-06-08T15:30:00Z
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

Phase 13 wires the Fumadocs docs pipeline into the existing Next.js 16 static-export site: MDX collection
config, source loader, DocsLayout, `[[...slug]]` catch-all with `generateStaticParams`, static Orama search
route, RootProvider injection, globals.css Fumadocs imports, and eight seed MDX sections. The implementation
is structurally sound — the build gate passed, all four success criteria were verified via Playwright, and the
critical API usages (import paths, `toFumadocsSource`, `staticGET`, `theme={{ enabled: false }}`, async
params, `@source` glob) are correct.

Three substantive issues were found:

1. **Supply-chain posture regression** (`pnpm-workspace.yaml`): `esbuild: true` was added to `allowBuilds`
   to unblock a transitive dep. The fix works, but the comment block it sits in says "build scripts are
   DENIED" — creating a documented contradiction. More importantly, the decision rationale given ("identical
   pattern to unrs-resolver") is misleading: `unrs-resolver` is in `ignoredBuiltDependencies` (its script is
   ignored entirely), while `esbuild` is in `allowBuilds` (its script runs). These are different gates with
   different security semantics. For a security product that dogfoods its own posture, the distinction matters
   and should be corrected.

2. **Search index path under `trailingSlash: true`** (`app/api/search/route.ts`): The build emits
   `out/api/search` as a flat JSON file, not `out/api/search/index.json`. The Fumadocs search client
   requests `/api/search` (without trailing slash). On Cloudflare Pages, `trailingSlash: true` in
   `next.config.mjs` rewrites browser navigation links, but it does not add a trailing slash to fetch
   requests made by the Orama client. However, Cloudflare Pages may redirect `/api/search` → `/api/search/`
   before serving, then fail because there is no `index.json` inside a search directory. No redirect rule
   is in place to guarantee the flat file is served. This is a latent deployment breakage.

3. **`generateMetadata` calls `notFound()` with unreachable code after it** (`[[...slug]]/page.tsx`):
   TypeScript correctly types `notFound()` as `never`, so the compiler accepts the code. However, the
   duplicate `source.getPage(params.slug)` call between `generateMetadata` and the `Page` component means
   that on a 404 path the source lookup runs twice per render, and the `return` statement after `notFound()`
   is unreachable dead code from TypeScript's view — the function exits via `notFound()` but the return
   type annotation implies `Metadata` is always returned. This is a minor quality defect, not a crash.

---

## Warnings

### WR-01: `pnpm-workspace.yaml` comment contradicts `allowBuilds: esbuild: true` and misrepresents the security gate

**File:** `pnpm-workspace.yaml:12-13`

**Issue:** The file's own header comment reads "Native postinstall build scripts are DENIED." Line 13
immediately contradicts this by setting `esbuild: true` under `allowBuilds`. The 13-01-SUMMARY.md
rationale ("identical pattern to unrs-resolver") is incorrect: `unrs-resolver` is in
`ignoredBuiltDependencies` (pnpm silently ignores its script — it never runs), while `esbuild: true` in
`allowBuilds` causes esbuild's `postinstall` to execute `node install.js`, which downloads and installs
a platform-specific native binary. These are semantically different gates:

- `ignoredBuiltDependencies` = script is suppressed, binary still resolves from a bundled optional
  dependency package.
- `allowBuilds: true` = script is explicitly allowed to run at install time.

For a security product that documents "deny by default" as a core posture and enforces the same for
the packages it monitors (npm/yarn), allowing a native build script to run without accurate documentation
is a posture gap. The esbuild package is well-known and the binary it installs is harmless, but the
comment/code mismatch means future reviewers or CI auditors reading the file will have incorrect
expectations about what runs on `pnpm install`.

**Fix:** Either update the comment to accurately describe the policy (three tiers: approved + run,
approved + ignored, denied), or move esbuild from `allowBuilds` to `ignoredBuiltDependencies` if
esbuild's prebuilt binary is already present as an optional dep (check `@esbuild/win32-x64` etc.
in the lockfile — if the platform-specific packages are bundled, the postinstall is a no-op anyway
and can be ignored). The accurate comment should read:

```yaml
# Build-script policy (three tiers):
#   allowBuilds: true  — script is permitted to run (esbuild: downloads platform binary via install.js)
#   allowBuilds: false — script is explicitly denied (sharp: never needed for static export)
#   ignoredBuiltDependencies — script is silently suppressed (unrs-resolver, msw: prebuilt binaries
#                               are bundled as optional deps; the postinstall is redundant)
allowBuilds:
  esbuild: true
  sharp: false

ignoredBuiltDependencies:
  - unrs-resolver
  - msw
```

---

### WR-02: Static search index path may 404 on Cloudflare Pages under `trailingSlash: true`

**File:** `web/app/api/search/route.ts:4-6`

**Issue:** With `trailingSlash: true` in `next.config.mjs`, Next.js static export emits the search
route handler as `out/api/search` (a flat JSON file), not `out/api/search/index.json`. The 13-02
SUMMARY confirms this empirically.

The Fumadocs search client fetches `/api/search` (no trailing slash — it is a fetch, not a navigation).
Cloudflare Pages has the following behavior for static assets:
- HTML files: `trailingSlash` redirects apply.
- Non-HTML files (JSON, JS, etc.): served at their exact path with no redirect.

Therefore: a browser fetch to `https://beekeeper.example/api/search` will succeed if Cloudflare maps
the path to the flat file `out/api/search`. **However**, if Cloudflare's static asset serving treats
`api/search` as a directory (because the build output has no `api/search` extension), it may serve a
directory listing or 404. The actual behavior depends on whether Cloudflare Pages sees the file as an
extensionless asset or a directory indicator.

No `_redirects` rule exists in the `out/` root or `public/` directory to guarantee this path resolves.
This creates a latent deployment risk that will only manifest when the site is deployed to Cloudflare
Pages — not caught by local `pnpm start` (which uses `serve`, which handles extensionless files
differently from Cloudflare's CDN).

**Fix:** Add a `public/_redirects` file (or `public/_headers` with content-type) to ensure Cloudflare
serves the flat file correctly:

```
# public/_redirects
/api/search    /api/search    200
```

Or, alternatively, verify in CI that the deployed Cloudflare Pages URL returns HTTP 200 for
`/api/search` with `Content-Type: application/json` before marking the phase as released. Add a
`next.config.mjs` note documenting the known path variant. The risk is low if Cloudflare treats
extensionless files as assets, but it has not been empirically verified at deploy time.

---

### WR-03: `generateMetadata` duplicates the page lookup already done in `Page`

**File:** `web/app/docs/[[...slug]]/page.tsx:15-25`

**Issue:** `source.getPage(params.slug)` is called in `generateMetadata` (line 19) and again in the
`Page` component (line 30). If the page is found in `generateMetadata`, Next.js will call `Page` for
the same route and perform the lookup a second time. There is no caching between the two calls within
the same render pass. In a static-export build this doubles the number of lookups per page route during
`generateStaticParams` pre-rendering.

Additionally, the pattern:
```typescript
if (!page) notFound();
return {
  title: page.data.title,  // page could be null here from TypeScript's perspective without the notFound() never type
```
works correctly because `notFound()` is typed as `never`, but it creates a code pattern where the
nullability guard and the usage are separated in a way that surprises readers unfamiliar with how
`notFound()` works in Next.js — and it will break if this pattern is ever copied to a context where
`notFound()` is not available or not typed as `never`.

**Fix:** Extract the lookup into a shared helper or use the Fumadocs-recommended pattern where
`generateMetadata` and `Page` both independently look up the page (which is idiomatic for Next.js
App Router). If the double lookup is a concern under heavy static export, cache it with a per-request
cache:

```typescript
import { cache } from "react";

const getPage = cache((slug?: string[]) => source.getPage(slug));

export async function generateMetadata(props: {
  params: Promise<{ slug?: string[] }>;
}): Promise<Metadata> {
  const params = await props.params;
  const page = getPage(params.slug);
  if (!page) notFound();
  return { title: page.data.title, description: page.data.description };
}

export default async function Page(props: {
  params: Promise<{ slug?: string[] }>;
}) {
  const params = await props.params;
  const page = getPage(params.slug);
  if (!page) notFound();
  const MDX = page.data.body;
  return (
    <DocsPage toc={page.data.toc} full={page.data.full}>
      <DocsTitle>{page.data.title}</DocsTitle>
      <DocsDescription>{page.data.description}</DocsDescription>
      <DocsBody><MDX /></DocsBody>
    </DocsPage>
  );
}
```

`React.cache` deduplicates the call within the same render pass.

---

## Info

### IN-01: `@types/mdx` placed in `dependencies` instead of `devDependencies`

**File:** `web/package.json:15`

**Issue:** `"@types/mdx": "2.0.14"` is listed under `dependencies`, not `devDependencies`. Type
packages are pure compile-time artifacts — they produce no runtime output and should not be shipped
in a production bundle. While this has no runtime effect for a Next.js site (the build output is
static HTML/JS with no `node_modules` shipped), it is inconsistent with how the other `@types/*`
packages (`@types/node`, `@types/react`, `@types/react-dom`) are placed in `devDependencies`.

**Fix:** Move `@types/mdx` to `devDependencies`:

```json
"devDependencies": {
  "@biomejs/biome": "2.2.0",
  "@tailwindcss/postcss": "^4",
  "@types/mdx": "2.0.14",
  "@types/node": "^20",
  "@types/react": "^19",
  "@types/react-dom": "^19",
  "shadcn": "^4.10.0",
  "tailwindcss": "^4",
  "typescript": "^5"
}
```

---

### IN-02: `web/app/api/search/route.ts` imports `source` without `export const dynamic`

**File:** `web/app/api/search/route.ts:1-6`

**Issue:** The route exports `revalidate = false`, which is the fumadocs-recommended approach and
works correctly under `output: 'export'`. However, the fumadocs static search guide also shows this
pattern used alongside `export const dynamic = 'force-static'` in some versions. With Next.js 16 and
`output: 'export'`, all routes are statically rendered at build time regardless — so `revalidate = false`
alone is sufficient. This is a documentation clarity note, not a bug.

What is worth noting: `source` is imported at module level. If the `.source/server.ts` generated file
is absent (e.g., on a fresh clone before `postinstall` or `next build` has run), TypeScript will error
at this import. The `postinstall: "fumadocs-mdx"` in `package.json` mitigates this for `pnpm install`
flows, but not for direct `tsc` invocations without a prior build.

**Fix:** No code change needed. Ensure CI runs `pnpm install` (which triggers `postinstall: fumadocs-mdx`
to generate `.source/`) before any `tsc --noEmit` step. Document this ordering constraint in the CI
workflow when Phase 19 adds the CI pipeline.

---

_Reviewed: 2026-06-08T15:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
