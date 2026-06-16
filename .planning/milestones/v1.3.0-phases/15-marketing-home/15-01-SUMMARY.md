---
phase: 15-marketing-home
plan: "01"
subsystem: web/marketing-home
tags: [marketing, next.js, react, home-page, hero, design-tokens]
dependency_graph:
  requires: [phase-12-design-system, phase-13-docs-pipeline]
  provides: [web/components/home, web/app/page.tsx marketing root]
  affects: [web/out/index.html, wave-2, wave-3]
tech_stack:
  added: []
  patterns: [server-components, client-component-clipboard, raw-theme-tokens, biome-lint]
key_files:
  created:
    - web/components/home/section.tsx
    - web/components/home/site-header.tsx
    - web/components/home/site-footer.tsx
    - web/components/home/install-chip.tsx
    - web/components/home/hero.tsx
  modified:
    - web/app/page.tsx
decisions:
  - "INSTALL_COMMAND rendered as single <code> text node (not split across spans) to ensure full canonical string appears in static HTML for grep gate and accessibility"
  - "Biome auto-format applied (pnpm exec biome check --write) — formatting-only, no logic changes"
  - "Omit module path color emphasis span to avoid React SSR <!-- --> comment breaking the continuous string; visual simplicity is acceptable"
metrics:
  duration: "~45 minutes"
  completed: "2026-06-08T18:01:57Z"
  tasks_completed: 3
  files_created: 5
  files_modified: 1
---

# Phase 15 Plan 01: Marketing Chrome + Hero Summary

**One-liner:** Sticky SiteHeader (v1.3.0, Docs/Changelog nav), go-install InstallChip (copy + honest D-03 framing), Hero section (headline + subhead + chip + Read-the-docs CTA), and SiteFooter — replacing the Phase-11 scaffold; `pnpm build` emits `web/out/index.html` with all grep gates passing.

## Completed Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Shared marketing primitives — Section, SiteHeader, SiteFooter | 36733d1 | web/components/home/section.tsx, site-header.tsx, site-footer.tsx |
| 2 | InstallChip client copy component (HOME-01) | 83e3a59 | web/components/home/install-chip.tsx |
| 3 | Hero section + page.tsx assembly + build gate (HOME-01) | 6faf09b | web/components/home/hero.tsx, web/app/page.tsx |

## Verification Results

- `cd web && pnpm exec tsc --noEmit` exits 0 after each task
- `cd web && pnpm build` exits 0; emits `web/out/index.html`
- `out/index.html` contains "autonomous coding agents" — PASS
- `out/index.html` contains "go install github.com/home-beekeeper/beekeeper/cmd/beekeeper@latest" — PASS
- `out/index.html` contains "/docs/getting-started" — PASS
- `out/index.html` does NOT contain "90-second demo" — PASS
- `out/index.html` does NOT contain "2.4k" — PASS
- No `--color-bk-*` color values in any new home component — PASS
- `pnpm lint` (Biome) exits 0 after auto-format — PASS

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] InstallChip command string split in static HTML output**
- **Found during:** Task 3 build verification
- **Issue:** The command display in `InstallChip` split the string across multiple React elements (`go install{" "}<span>github.com/home-beekeeper/beekeeper</span>/cmd/beekeeper@latest`) with React SSR `<!-- -->` comment boundaries, causing the acceptance criteria grep for the continuous string `go install github.com/home-beekeeper/beekeeper/cmd/beekeeper@latest` to fail on `out/index.html`.
- **Fix:** Replaced the split display with a single `<code>` element containing `{INSTALL_COMMAND}` (the const) as its sole text child. This renders as a single uninterrupted string in the HTML, is better for accessibility (screen readers get the full command), and supports `select-all` behavior.
- **Files modified:** web/components/home/install-chip.tsx
- **Commit:** 6faf09b (included in Task 3 commit)

**2. [Rule 3 - Formatting] Biome formatter issues on all 5 new files**
- **Found during:** Task 3 `pnpm lint` run
- **Issue:** Biome reported formatting errors: import order in page.tsx, line length violations in hero.tsx, install-chip.tsx, section.tsx, site-footer.tsx.
- **Fix:** `pnpm exec biome check --write` applied — formatting only, no logic changes.
- **Files modified:** web/app/page.tsx, web/components/home/hero.tsx, web/components/home/install-chip.tsx, web/components/home/section.tsx, web/components/home/site-footer.tsx
- **Commit:** 6faf09b

## Design Decisions

1. **ThemeToggle omitted from SiteHeader**: No `ThemeToggle` export exists in `web/components`. Per plan spec, this is acceptable — theme switching is provided by the root `ThemeProvider` in `Providers`. The toggle may be added in a later wave if a toggle component is built.

2. **Module path emphasis removed**: The mockup's `.cmd .em { color: var(--fg-strong) }` emphasizing `github.com/home-beekeeper/beekeeper` was removed in favor of a single uniform `<code>` text node. This avoids the static HTML string continuity issue while keeping the command readable. Minor visual delta from the mockup.

3. **"Beekeeper 1.3.0 shipped" pill copy**: Updated from the mockup's `v1.0` with an accurate description of the current shipped milestone ("self-protection + TUI policy editor") per D-03 content accuracy.

4. **Docs link → /docs/getting-started (not bare /docs)**: Confirmed per changelog/layout.tsx comment and plan spec. All three new nav components (SiteHeader, SiteFooter, Hero CTA) consistently link to `/docs/getting-started`.

## Known Stubs

None. All content is either static accurate copy (D-03 compliant) or properly deferred with honest framing (install command: "Canonical install path — published once the repo is public."). No placeholder text or TODO stubs in any rendered content.

## Threat Surface Scan

No new security-relevant surface introduced beyond the plan's `<threat_model>`:

- T-15-01 (content integrity): All grep gates enforce no aspirational claims. No "2.4k", no "90-second demo", no brew/scoop/docker. Install command framed as canonical-but-unpublished. MITIGATED.
- T-15-02 (clipboard write): Fixed const string only; no secrets or user data. ACCEPTED.
- T-15-03 (link safety): All links are internal (/docs/*, /changelog); no `target="_blank"` introduced. MITIGATED.
- T-15-04 (XSS): No `dangerouslySetInnerHTML`; all text is JSX-escaped React content. MITIGATED.
- T-15-SC (supply chain): Zero new npm dependencies added. ACCEPTED.

## Self-Check: PASSED

- web/components/home/section.tsx: EXISTS
- web/components/home/site-header.tsx: EXISTS
- web/components/home/site-footer.tsx: EXISTS
- web/components/home/install-chip.tsx: EXISTS
- web/components/home/hero.tsx: EXISTS
- web/app/page.tsx: MODIFIED (Phase-11 scaffold replaced)
- Commits: 36733d1, 83e3a59, 6faf09b — all verified in git log
- web/out/index.html: EXISTS, all grep gates PASSED
