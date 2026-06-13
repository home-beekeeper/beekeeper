---
phase: 14-changelog-pipeline
plan: "01"
subsystem: web/changelog-pipeline
tags: [fumadocs, next.js, mdx, static-export, changelog, security-verification]
dependency_graph:
  requires: [13-02-SUMMARY.md]
  provides: [changelog collection, changelog loader, changelog catch-all route, changelog layout, VerifyCommands, ReleaseLinks, BreakingChangeCallout, v1.0.0 notes, v1.2.0 notes, docs nav link]
  affects:
    - web/source.config.ts
    - web/lib/changelog-source.ts
    - web/app/changelog/layout.tsx
    - web/app/changelog/[[...slug]]/page.tsx
    - web/components/changelog/
    - web/mdx-components.tsx
    - web/app/docs/layout.tsx
    - web/content/changelog/
tech_stack:
  added: []
  patterns:
    - second fumadocs-mdx defineDocs collection (content/changelog/) alongside existing docs
    - changelog loader() with baseUrl /changelog + changelog.toFumadocsSource()
    - DocsLayout with changelog source.pageTree
    - generateStaticParams + async params (Next 16) for changelog catch-all route
    - useMDXComponents passed explicitly to <MDX components={...}> for custom component resolution
    - raw theme tokens (var(--red)/var(--teal)/var(--fg)/var(--border)) — no --color-bk-* (dual-theme correct)
    - color-mix(in srgb, var(--red) 10%, transparent) for tinted callout background
key_files:
  created:
    - web/source.config.ts (modified — added changelog export)
    - web/lib/changelog-source.ts
    - web/app/changelog/layout.tsx
    - web/app/changelog/[[...slug]]/page.tsx
    - web/components/changelog/verify-commands.tsx
    - web/components/changelog/release-links.tsx
    - web/components/changelog/breaking-change-callout.tsx
    - web/mdx-components.tsx
    - web/content/changelog/meta.json
    - web/content/changelog/v1.0.0/meta.json
    - web/content/changelog/v1.0.0/index.mdx
    - web/content/changelog/v1.2.0/meta.json
    - web/content/changelog/v1.2.0/index.mdx
  modified:
    - web/app/docs/layout.tsx (Changelog nav link added)
    - web/biome.json (added !.source to excludes)
    - web/app/api/search/route.ts (import ordering fix, Biome)
    - web/app/providers.tsx (import ordering fix, Biome)
    - web/next.config.mjs (single-quote → double-quote, Biome format)
    - web/app/docs/layout.tsx (import ordering fix, Biome)
    - web/app/docs/[[...slug]]/page.tsx (import ordering fix, Biome)
decisions:
  - "Explicit useMDXComponents({}) passed to <MDX components={...} /> in changelog page — fumadocs-mdx compiles MDX outside @next/mdx so the Next.js mdx-components.tsx convention does NOT auto-inject components; must pass explicitly to the body component at render time"
  - "changelog meta.json lists only v1.2.0 and v1.0.0 (v1.3.0 added in 14-02); fumadocs tolerates missing-page entries in meta.json at build time but the plan notes this as a risk — confirmed no error with 2-entry meta"
  - "biome.json !.source added to file includes to exclude generated fumadocs-mdx codegen from Biome linting; without this exclusion pnpm lint exits 1 on noBannedTypes and noUnusedVariables in generated .source/ files"
  - "Pre-existing Biome import-ordering and quote-style issues fixed across app/providers.tsx, app/api/search/route.ts, app/docs/layout.tsx, app/docs/[[...slug]]/page.tsx, next.config.mjs, source.config.ts as a side effect of making pnpm lint clean (required by Task 2 acceptance criteria)"
metrics:
  duration: "~45 minutes"
  completed: "2026-06-08"
  tasks_completed: 3
  files_created: 13
  files_modified: 7
---

# Phase 14 Plan 01: Changelog Pipeline Summary

**One-liner:** Second fumadocs-mdx `changelog` collection wired — loader (baseUrl /changelog), catch-all route, DocsLayout — plus VerifyCommands/ReleaseLinks/BreakingChangeCallout MDX components and accurate v1.0.0/v1.2.0 release notes with cosign/SLSA verification commands (capital-B Bantuson); pnpm build exits 0, out/changelog/v1.0.0 and out/changelog/v1.2.0 emitted, changelog reachable from docs nav.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Wire the changelog collection, loader, route, and layout | 092a09d | web/source.config.ts, web/lib/changelog-source.ts, web/app/changelog/layout.tsx, web/app/changelog/[[...slug]]/page.tsx |
| 2 | Build VerifyCommands, ReleaseLinks, BreakingChangeCallout + MDX map | 27c0775 | web/components/changelog/verify-commands.tsx, release-links.tsx, breaking-change-callout.tsx, web/mdx-components.tsx, web/biome.json + 6 pre-existing format fixes |
| 3 | Author v1.0.0 and v1.2.0 release notes + docs nav link + build gate | b2de991 | web/content/changelog/{meta.json, v1.0.0/*, v1.2.0/*}, web/app/docs/layout.tsx, web/app/changelog/[[...slug]]/page.tsx (components prop) |

## Verification

- `pnpm exec tsc --noEmit` — PASSED (all tasks; fumadocs-mdx codegen `changelog` export resolves via collections/server alias)
- `pnpm lint` (Biome) — PASSED (Task 2: .source excluded via biome.json !.source; import ordering fixed in pre-existing files)
- `pnpm build` exits 0 — PASSED (Task 3)
- `out/changelog/v1.0.0/index.html` — FOUND (non-empty)
- `out/changelog/v1.2.0/index.html` — FOUND (non-empty)
- Rendered HTML contains `Bantuson/beekeeper` (capital-B cosign identity) — PASSED (grep confirmed × 2 per version)
- Rendered HTML contains `bantuson/beekeeper/releases/tag/vX.Y.Z` canonical release links — PASSED
- No `--color-bk-*` tokens in new changelog components — PASSED (grep confirms: raw theme tokens only)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Explicit components prop required on <MDX /> for custom component resolution**

- **Found during:** Task 3 — first `pnpm build` attempt
- **Issue:** `pnpm build` failed with `Error: Expected component 'ReleaseLinks' to be defined`. The Next.js `mdx-components.tsx` convention (exporting `useMDXComponents`) only auto-injects components when MDX is compiled via `@next/mdx`. fumadocs-mdx uses its own Turbopack loader and the body component does NOT pick up the global `mdx-components.tsx` automatically.
- **Fix:** Import `useMDXComponents` in the changelog page and pass `components={useMDXComponents({})}` explicitly to `<MDX />`. Also updated the `changelog/[[...slug]]/page.tsx` import committed in Task 1 to add this wiring.
- **Files modified:** `web/app/changelog/[[...slug]]/page.tsx`
- **Commit:** b2de991

**2. [Rule 1 - Bug] DocsLayout nav `links` is a top-level prop, not under `nav`**

- **Found during:** Task 1 TypeScript check
- **Issue:** `tsc --noEmit` failed: `Property 'links' does not exist in type 'NavOptions'`. The plan specified a `links` array inside `nav: {}`, but the Fumadocs `DocsLayoutProps` (extending `BaseLayoutProps`) has `links` as a top-level prop, not inside `NavOptions`.
- **Fix:** Moved `links` to top-level on `DocsLayout` component.
- **Files modified:** `web/app/changelog/layout.tsx`
- **Commit:** 092a09d

**3. [Rule 2 - Missing Critical Functionality] Biome excludes generated .source/ files**

- **Found during:** Task 2 `pnpm lint` run
- **Issue:** `pnpm lint` exited 1 with `noBannedTypes` and `noUnusedVariables` errors in `.source/browser.ts`, `.source/dynamic.ts`, and `.source/server.ts` — generated fumadocs-mdx codegen files. These are in `.gitignore` but `biome.json` `includes` did not explicitly exclude them; `useIgnoreFile: true` did not suppress them in this biome version.
- **Fix:** Added `"!.source"` to `biome.json` `files.includes` array.
- **Files modified:** `web/biome.json`
- **Commit:** 27c0775

**4. [Rule 1 - Bug] Pre-existing Biome import-ordering / quote-style issues in existing files**

- **Found during:** Task 2 `pnpm lint` — after fixing `.source` exclusion, 6 pre-existing files still had import-ordering and single-quote formatting issues.
- **Fix:** `pnpm exec biome check --write` applied safe auto-fixes (import reordering, single → double quotes) to `app/providers.tsx`, `app/api/search/route.ts`, `app/docs/layout.tsx`, `app/docs/[[...slug]]/page.tsx`, `next.config.mjs`, `source.config.ts`. These are correctness issues under the Task 2 acceptance criterion (`pnpm lint` clean).
- **Files modified:** 6 pre-existing web/ files
- **Commit:** 27c0775

## Known Stubs

**v1.3.0 page**: Not authored in 14-01 (14-02 scope). The `meta.json` lists only v1.2.0 and v1.0.0 in this plan; 14-02 adds the v1.3.0 entry and content.

No other stubs — all content is accurate human-written release notes sourced from MILESTONES.md, not lorem/placeholder.

## Threat Flags

T-14-01 mitigation confirmed: `VerifyCommands` source contains literal `Bantuson/beekeeper` (capital-B) in the `--certificate-identity-regexp` string. Rendered HTML confirmed to contain `Bantuson/beekeeper` × 2 in both `/changelog/v1.0.0/index.html` and `/changelog/v1.2.0/index.html`.

T-14-02 mitigation confirmed: Release-note MDX content sourced from MILESTONES.md (public-class summaries). No secrets, env vars, private hostnames, or unreleased internals observed in authored content.

T-14-03 mitigation confirmed: `ReleaseLinks` uses `github.com/bantuson/beekeeper/releases/tag/${version}` (lowercase canonical repo path) with `rel="noopener noreferrer"` and honest "resolves once published" microcopy.

T-14-04 mitigation confirmed: `pnpm install --frozen-lockfile` not run (no new packages added). `web/package.json` and `pnpm-lock.yaml` unchanged.

## Self-Check: PASSED

Files verified:
- web/source.config.ts — FOUND (contains `export const changelog` and `export const docs`)
- web/lib/changelog-source.ts — FOUND (contains `baseUrl: "/changelog"` and `.toFumadocsSource()`)
- web/app/changelog/layout.tsx — FOUND (contains `DocsLayout` and `tree={source.pageTree}`)
- web/app/changelog/[[...slug]]/page.tsx — FOUND (contains `generateStaticParams` and `await props.params`)
- web/components/changelog/verify-commands.tsx — FOUND (contains `Bantuson/beekeeper`, `cosign verify-blob`, `slsa-verifier verify-artifact`)
- web/components/changelog/release-links.tsx — FOUND (contains `github.com/bantuson/beekeeper/releases/tag/`)
- web/components/changelog/breaking-change-callout.tsx — FOUND (contains `var(--red)`, `TriangleAlert`)
- web/mdx-components.tsx — FOUND (maps `VerifyCommands`, `ReleaseLinks`, `BreakingChangeCallout`)
- web/content/changelog/meta.json — FOUND
- web/content/changelog/v1.0.0/index.mdx — FOUND (contains `<VerifyCommands`, `<ReleaseLinks`, `2026-06-01`)
- web/content/changelog/v1.2.0/index.mdx — FOUND (contains `<VerifyCommands`, `<ReleaseLinks`, `2026-06-04`)
- web/out/changelog/v1.0.0/index.html — FOUND
- web/out/changelog/v1.2.0/index.html — FOUND

Commits verified:
- 092a09d — FOUND (feat(14-01): wire changelog collection, loader, route, and layout)
- 27c0775 — FOUND (feat(14-01): add VerifyCommands, ReleaseLinks, BreakingChangeCallout + MDX map)
- b2de991 — FOUND (feat(14-01): author v1.0.0 and v1.2.0 changelog notes + docs nav link + build gate PASSED)

Build gate: PASSED (pnpm build exits 0; out/changelog/v1.0.0/index.html + out/changelog/v1.2.0/index.html emitted and verified non-empty with correct content)
