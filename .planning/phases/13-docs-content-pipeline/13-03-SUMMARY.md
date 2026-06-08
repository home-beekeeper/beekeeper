---
phase: 13-docs-content-pipeline
plan: "03"
subsystem: web/docs-content
tags: [fumadocs, mdx, static-export, orama, search, sidebar, toc, playwright]
dependency_graph:
  requires:
    - phase: 13-02
      provides: "Fumadocs runtime surface — lib/source.ts, DocsLayout, [[...slug]] page, static-search route, RootProvider, globals.css @source glob, and the 8 seed MDX sections + meta.json files (pre-created as a 13-02 deviation)"
  provides:
    - "Phase 13 phase-complete gate PASSED: SC-1/2/3/4 all verified against static out/ build"
    - "Confirmed 8 navigable docs sections with forced sidebar order"
    - "Confirmed Orama search index (93KB, 80 documents) is non-empty and functional"
    - "Confirmed getting-started TOC renders 4 entries (Prerequisites, Installation, Hook Registration, Verifying the Hook)"
    - "Confirmed search returns 36 results for 'beekeeper' and click navigates to /docs/ path"
  affects:
    - web/content/docs/
    - out/api/search (Orama search index)
tech_stack:
  added: []
  patterns:
    - "Fumadocs search results are <button aria-selected> elements inside [role='dialog'] — not <a> or [role='option'] as documented in research snippets"
    - "TOC anchor links (a[href^='#']) excluding #main-content skip link = correct TOC count selector"
    - "Playwright Ctrl+K triggers search dialog; input[placeholder='Search'] is the visible input; results in [role='dialog'] button[aria-selected]"
key_files:
  created:
    - web/content/docs/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/getting-started/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/getting-started/index.mdx (pre-created in 13-02 Task 3)
    - web/content/docs/installation/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/installation/index.mdx (pre-created in 13-02 Task 3)
    - web/content/docs/configuration/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/configuration/index.mdx (pre-created in 13-02 Task 3)
    - web/content/docs/integration/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/integration/index.mdx (pre-created in 13-02 Task 3)
    - web/content/docs/security/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/security/index.mdx (pre-created in 13-02 Task 3)
    - web/content/docs/cli-reference/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/cli-reference/index.mdx (pre-created in 13-02 Task 3)
    - web/content/docs/audit-log/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/audit-log/index.mdx (pre-created in 13-02 Task 3)
    - web/content/docs/troubleshooting/meta.json (pre-created in 13-02 Task 3)
    - web/content/docs/troubleshooting/index.mdx (pre-created in 13-02 Task 3)
  modified: []
key_decisions:
  - "All 13-03 content (9 meta.json + 8 index.mdx) was pre-created in 13-02 Task 3 as a Rule 2 deviation — empty content/docs/ caused generateStaticParams to return [] which breaks output:export builds. This is a correctness prerequisite, not scope creep."
  - "out/api/search is a flat file (93KB) not out/api/search/index.json — trailingSlash:true causes Next.js static export to emit route handlers as flat files (Pitfall 6 confirmed). Fumadocs client fetches /api/search which resolves correctly on Cloudflare Pages."
  - "Playwright SC-4 search results are <button aria-selected> inside [role='dialog'], not <a> or [role='option'] as the Phase-12-derived research snippets assumed. JS click (element_handle().click()) bypasses the fixed overlay that intercepts pointer events in Playwright."
  - "SC-3 TOC count: 4 entries (Prerequisites, Installation, Hook Registration, Verifying the Hook) confirmed via a[href^='#'] excluding #main-content skip link. Fumadocs renders the 4 h2-level headings from getting-started/index.mdx into the TOC."

patterns-established:
  - "Fumadocs static search: Orama indexes headings + body text from MDX at build time via createFromSource/staticGET. 8 seed pages yielded 80 Orama documents (some pages generate multiple documents from heading segments)."
  - "Playwright on static export: use Python HTTP server (python -m http.server <port> --directory out) for reliable static serving. npx serve had connection reset issues on Windows."

requirements-completed: [DOCS-01]

duration: ~25min
completed: "2026-06-08"
---

# Phase 13 Plan 03: Content Skeleton + Phase-Complete Gate Summary

**Phase 13 phase-complete gate PASSED: all four ROADMAP success criteria (SC-1/2/3/4) verified — 8 docs sections render to static HTML in forced sidebar order, getting-started TOC has 4 entries, Orama search returns 36 results for "beekeeper" and navigates to /docs/ on click.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-06-08
- **Tasks:** 2 (content acceptance criteria confirmed, phase-complete gate run)
- **Files modified:** 0 (all content pre-created in 13-02)

## Accomplishments

This plan's role is the phase-complete validation gate. All 17 content files (1 top-level meta.json, 8 section meta.json, 8 seed index.mdx) were pre-created in 13-02 Task 3 as a Rule 2 deviation (see Deviations section). This plan confirmed they satisfy all 13-03 acceptance criteria and ran the SC-1/2/3/4 validation suite.

### Content Acceptance Criteria: ALL MET

| Section | H2 headings | Total headings | Body words | title | description |
|---------|-------------|----------------|------------|-------|-------------|
| getting-started | 4 | 4 | 67 | Y | Y |
| installation | 1 | 4 (1 h2 + 3 h3) | 53 | Y | Y |
| configuration | 3 | 4 (3 h2 + 1 h3) | 59 | Y | Y |
| integration | 4 | 4 | 60 | Y | Y |
| security | 4 | 4 | 83 | Y | Y |
| cli-reference | 3 | 5 (3 h2 + 2 h3) | 71 | Y | Y |
| audit-log | 4 | 4 | 73 | Y | Y |
| troubleshooting | 4 | 4 | 86 | Y | Y |

All 8 sections: >= 3 total headings, >= 50 body words, title + description frontmatter. getting-started has 4 h2 headings (TOC test requirement met). Scope fence honored: no Phase-18 prose, no `source_doc:` citations.

### Phase-Complete Gate: ALL PASSED

| Criterion | Result | Evidence |
|-----------|--------|----------|
| SC-1 build | PASSED | `pnpm build` exits 0; 13 static pages generated including 8 docs routes |
| SC-1 HTML files | PASSED | All 8 `out/docs/<section>/index.html` exist and non-empty |
| SC-1 search index | PASSED | `out/api/search` flat file 93,388 bytes, 80 Orama documents |
| SC-2 sidebar order | PASSED | Playwright: Getting Started → Installation → Configuration → Integration → Security → CLI Reference → Audit Log → Troubleshooting |
| SC-3 TOC entries | PASSED | Playwright: 4 TOC entries on getting-started (Prerequisites, Installation, Hook Registration, Verifying the Hook) |
| SC-4 search | PASSED | Playwright: 36 results for "beekeeper"; first result click navigated to http://localhost:3099/docs/cli-reference/ |

## Deviations from Plan

### 13-02 Pre-creation Deviation (cross-reference)

All content files (web/content/docs/meta.json + 8 section meta.json + 8 index.mdx) were created in 13-02 Task 3 commit 32e30c4, not in this plan. This was documented in 13-02-SUMMARY.md as:

> [Rule 2 - Missing Critical Functionality] Seed MDX content added in Task 3 (not 13-03) — empty content/docs/ caused generateStaticParams to return [] which Next.js with output:export treats as missing generateStaticParams entirely.

This plan's job was therefore RECONCILE + VALIDATE (per the orchestrator objective), not recreate. No reconcile fixes were needed — all content files passed the 13-03 acceptance criteria as-is.

### SC-1 Path Variant Confirmed

`out/api/search` is a flat file (93,388 bytes), NOT `out/api/search/index.json`. This is correct Next.js behavior with `trailingSlash: true` — route handlers emit as flat files in static export. The Fumadocs static search client fetches `/api/search`, which resolves correctly on Cloudflare Pages. Confirmed empirically in 13-02 (Pitfall 6 from RESEARCH.md).

### Playwright SC-4 Selector Correction

The Phase-12 Playwright snippets in 13-RESEARCH.md specified `[role='option']` as the search result selector. The actual Fumadocs v16 search dialog uses `<button type="button" aria-selected="true/false">` elements (not `<a>` or `[role='option']`). Additionally, the sidebar overlay (`data-state="open"`) intercepts pointer events, so sidebar `a[href*='/docs']` links cannot be directly clicked while the dialog is open. Fix: use `[role='dialog'] button[aria-selected]` selector + `element_handle().click()` to bypass the overlay.

## Known Stubs

The 8 seed MDX pages remain intentional placeholder content (Phase 18 scope). They satisfy all Phase 13 success criteria (build correctness, sidebar order, TOC rendering, search results). Full prose authoring happens in Phase 18 (DOCS-02..DOCS-09).

## Threat Flags

No new security surfaces introduced. This plan is validation-only — no code or content changes.

T-13-06 status: Confirmed `out/api/search` contains only doc-derived titles/headings/body text from seed MDX. No secrets, env vars, internal paths, or unreleased details in the 93KB Orama index.

## Self-Check: PASSED

Files verified:
- web/content/docs/meta.json — FOUND (pages array in correct forced order)
- web/content/docs/getting-started/index.mdx — FOUND (4 h2 headings, 67 body words)
- web/content/docs/troubleshooting/index.mdx — FOUND (4 h2 headings, 86 body words)
- out/docs/getting-started/index.html — FOUND
- out/docs/troubleshooting/index.html — FOUND
- out/api/search — FOUND (93,388 bytes, non-empty Orama index)

Commits verified (pre-created content):
- 32e30c4 — FOUND (feat(13-02): create static-search route + seed MDX content + Wave 1/2 build gate PASSED)

SC validation results:
- SC-1 filesystem: PASSED (pnpm build exits 0; 8/8 HTML files; 93KB search index)
- SC-2 sidebar: PASSED (exact 8-section forced order confirmed via Playwright)
- SC-3 TOC: PASSED (4 entries on getting-started confirmed via Playwright)
- SC-4 search: PASSED (36 results for "beekeeper"; click → /docs/cli-reference/ confirmed via Playwright)
