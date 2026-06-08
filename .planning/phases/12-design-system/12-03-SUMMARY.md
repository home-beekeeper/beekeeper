---
phase: 12-design-system
plan: "03"
subsystem: web/design-system-integration
tags: [tailwind-v4, theme-tokens, globals-css, next-font, layout, wcag, playwright]
requirements: [DSYS-01, DSYS-02, DSYS-03, DSYS-04]

dependency_graph:
  requires: ["12-01", "12-02"]
  provides: ["web/app/globals.css (canonical Beekeeper token cascade)", "web/app/layout.tsx (fonts + skip link + Providers)"]
  affects: ["Phase 13 Fumadocs (uncomment imports + restore @source glob)", "Phase 15 marketing home (#main-content target, shadcn component usage)", "Phase 16 R3F (reduced-motion gate)"]

tech_stack:
  added: []
  patterns:
    - "Tailwind v4 @theme inline slot remaps MUST reference theme-switched raw tokens (--bg/--fg/--amber/--teal), NOT dark-only brand constants (--color-bk-*)"
    - "@layer base { body { background-color: var(--bg); color: var(--fg) } } to paint the document surface per-theme"
    - "next/font/google Inter (axes:[opsz]) + JetBrains_Mono with CSS variables"
    - "next-themes class strategy + suppressHydrationWarning (no-FOUC) verified in-browser"
    - "Playwright (python, chromium) smoke verification of a static export"

key_files:
  created: []
  modified:
    - web/app/globals.css
    - web/app/layout.tsx

decisions:
  - "globals.css authored as the 8-section canonical order; Fumadocs imports kept COMMENTED until Phase 13"
  - "Inter axes:[opsz] accepted (no opsz build failure on Next 16.2.7) — fallback not needed"
  - "Restored !important on the reduced-motion gate via biome-ignore-start/end (UI-SPEC belt-and-suspenders)"
  - "Theme-switch architecture fix: slot tokens repointed from --color-bk-* to raw --bg/--fg/--amber/--teal so light/dark actually switch"
  - "Added @layer base body rule — nothing was painting <body>"
  - "Task 3 human-verify performed via Playwright instead of manual browser (maintainer directive)"

metrics:
  completed_date: "2026-06-08"
  tasks_total: 3
  tasks_completed: 3
  files_created: 0
  files_modified: 2
---

# Phase 12 Plan 03: Design System Integration Summary

**One-liner:** Canonical Beekeeper `globals.css` token cascade + `layout.tsx` (Inter/JetBrains fonts, skip link, Providers) replacing the Phase 11 scaffold — with two correctness fixes (theme-switch tokens, body paint) caught by Playwright smoke verification, leaving all four DSYS criteria verified in both themes.

---

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Rewrite globals.css — canonical 8-section Beekeeper token order | cdc5a73 | web/app/globals.css |
| 2 | Rewrite layout.tsx — Inter/JetBrains fonts, suppressHydrationWarning, skip link, Providers | e0cdf45 | web/app/layout.tsx, web/app/globals.css (Biome reformat) |
| 2b | (post-checkpoint) Restore !important on reduced-motion gate | 205ff71 | web/app/globals.css |
| 2c | (post-smoke fix) Theme-switch slot tokens + paint body | 2cfacd1 | web/app/globals.css |
| 3 | Manual smoke verification — DSYS-02/03/04 (via Playwright) | (verification, no code) | — |

---

## Task 3 — Smoke Verification (Playwright / chromium against static `out/`)

The four behavioral DSYS criteria have no automated runner this phase (Vitest/Playwright land in Phase 19). Per maintainer directive, Task 3 was verified with the installed Playwright (python 1.57.0, chromium) driving the built static export rather than a hand smoke test. Results:

| Criterion | Check | Result |
|-----------|-------|--------|
| DSYS-02 | Theme class applies + persists across 2 reloads (light + dark) | PASS |
| DSYS-02 | No-FOUC: blocking `bk-theme` script present, class set before load | PASS |
| DSYS-03 | `data-reduced-motion="true"` under `reduced_motion=reduce`; `"false"` otherwise | PASS |
| DSYS-04 | Contrast (real utilities): dark text/bg 13.84, amber/bg 10.0; light text/bg 8.94, amber/bg 5.05, teal/bg 5.91; dark teal/bg ≈9.3 (computed) | PASS (all ≥ AA) |
| DSYS-04 | First Tab focuses "Skip to main content"; grows 1×1 → 187×42 on focus; visible outline | PASS |
| DSYS-01 | `bg-background`/`bg-primary` switch per theme; `<body>` painted (#0a0d12 dark / #f8f9fa light) | PASS (after 2cfacd1) |

The smoke scripts were throwaway (under `.smoke-tmp/`, not committed). Phase 19 should formalize equivalent Playwright assertions.

---

## Deviations from Plan

### 1. [Blocker → auto-fixed] `@source` glob broke PostCSS in a CSS comment
- **Found during:** Task 1 (`pnpm build`).
- **Issue:** `/* @source "../node_modules/fumadocs-ui/dist/**/*.js"; */` raised `CssSyntaxError: Unclosed string` — the `**/*` glob inside a CSS comment was mis-parsed.
- **Fix:** Rewrote the commented stub as a placeholder: `/* @source "../node_modules/fumadocs-ui/dist/[double-star]/*.js"; (uncomment in Phase 13 with correct glob) */`.
- **⚠ Phase 13 action required:** when uncommenting Fumadocs, restore the real glob (`**/*.js`) — the placeholder is NOT a valid `@source`.
- **Commit:** cdc5a73.

### 2. [Biome → restored with suppression] `!important` stripped from reduced-motion gate
- **Found during:** Task 2 (`pnpm lint`, `--unsafe`).
- **Issue:** Biome `lint/complexity/noImportantStyles` removed the `!important` the UI-SPEC required as a belt-and-suspenders override.
- **Fix:** Restored `!important` on all four reduced-motion properties, wrapped in `/* biome-ignore-start lint/complexity/noImportantStyles … */ … /* biome-ignore-end … */`. Lint clean, semantics preserved for future (Phase 16) animation rules.
- **Commit:** 205ff71.

### 3. [Theme-switch bug → fixed] light/dark utilities rendered identical values
- **Found during:** Task 3 Playwright smoke (utility-class measurement).
- **Issue:** `@theme inline` mapped the shadcn slots (`--color-background/foreground/primary/ring/...`) to the **dark-only** `--color-bk-*` brand constants. Tailwind inlined e.g. `.bg-background{background-color:var(--color-bk-bg)}`, and since `.light` never overrides `--color-bk-*`, every utility was frozen to its dark value in BOTH themes. The `.light` block's direct `--color-*` overrides were dead (no utility referenced them under `inline`).
- **Fix:** Repointed the slot remaps at the theme-switched raw tokens (`var(--bg)`, `var(--fg)`, `var(--amber)`, `var(--teal)`, `var(--surface*)`, `var(--dim)`, `var(--fg-strong)`, `var(--red)`, `var(--border)`) that `:root` defines and `.light` overrides. Utilities now compile to `var(--bg)` etc. and switch correctly.
- **Commit:** 2cfacd1.

### 4. [Missing base style → added] `<body>` was never painted
- **Found during:** Task 3 Playwright smoke (body background measured transparent in both themes).
- **Issue:** No rule applied `bg-background`/`text-foreground` to `body` (the standard shadcn `@layer base { body { … } }` was absent), so the page rendered on browser-white regardless of theme.
- **Fix:** Added `@layer base { body { background-color: var(--bg); color: var(--fg) } }` (raw theme tokens — theme-correct regardless of utility generation).
- **Commit:** 2cfacd1.

### Notes (no fallback taken)
- Inter `axes: ["opsz"]` compiled cleanly on Next 16.2.7 — the documented `font-optical-sizing: auto` fallback was NOT needed.

---

## Known Stubs / Forward Dependencies

- **Fumadocs imports (Section 3 of globals.css)** are intentionally COMMENTED — Phase 13 installs `fumadocs-ui` and uncomments them (and must restore the real `@source` glob, see Deviation 1).
- **`#main-content` skip-link target** does not yet exist — Phase 15's marketing home adds it (UI-SPEC scope fence). The skip link itself is present, first-focusable, and visible on focus.
- **shadcn focus ring on components** — `--color-ring` (teal) is wired, but no shadcn interactive component is on the placeholder page yet; the ring renders when Phase 15 uses components. The skip link shows a visible browser-default focus outline.
- **`.light` block still carries dead `--color-*` direct overrides** — harmless after the fix (utilities use raw tokens). If Phase 13/components ever read `var(--color-*)` directly, emit dark values too or remove the dead overrides.

---

## Threat Flags

None new. The plan's threat model (T-12-05 build-time font fetch self-hosted into `out/` — accepted; T-12-06 token cascade order — mitigated by `@theme inline` authored last + Fumadocs commented; T-12-04 next-themes inline script vs CSP — deferred to Phase 17) holds. No endpoints, auth, or schema introduced.

---

## Self-Check: PASSED

- `web/app/globals.css` — canonical order, `@custom-variant dark (&:where(.dark, .dark *))`, `@theme inline` last, slots → theme-switched tokens, `@layer base` body paint, reduced-motion gate with `!important` + biome-ignore, Fumadocs commented, no Geist — FOUND
- `web/app/layout.tsx` — Inter + JetBrains_Mono (next/font), `suppressHydrationWarning`, skip link, `<Providers>` wrap, no Geist — FOUND
- Commits cdc5a73, e0cdf45, 205ff71, 2cfacd1 — FOUND (git log)
- `pnpm build` — exits 0
- `pnpm lint` (Biome) — clean
- Playwright smoke (DSYS-01/02/03/04) — all PASS in both themes
- STATE.md / ROADMAP.md — NOT modified (orchestrator owns tracking)
