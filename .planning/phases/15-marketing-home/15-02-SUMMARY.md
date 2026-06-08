---
phase: 15-marketing-home
plan: "02"
subsystem: web/marketing-home
tags: [marketing, next.js, react, home-page, content, feature-cards, origin-story, how-it-works]
dependency_graph:
  requires: [phase-15-wave-1, phase-12-design-system, phase-13-docs-pipeline]
  provides: [web/components/home/origin-story, web/components/home/how-it-works, web/components/home/feature-cards]
  affects: [web/out/index.html, wave-3, HOME-02, HOME-03]
tech_stack:
  added: []
  patterns: [server-components, raw-theme-tokens, section-primitive-reuse, biome-lint]
key_files:
  created:
    - web/components/home/origin-story.tsx
    - web/components/home/how-it-works.tsx
    - web/components/home/feature-cards.tsx
  modified:
    - web/app/page.tsx
decisions:
  - "D-03 / D-04 enforced: feature cards carry exactly 6 SHIPPED capabilities; Calm-mode dashboard / Steer toward hardened / Open from day one excluded as card titles"
  - "Fake live-sync badge ('synced 6 minutes ago' + pulse animation) removed from threat list; replaced with static 'Documented 2026 campaigns' label"
  - "Full canonical install path enforced in quickstart (github.com/bantuson/beekeeper/cmd/beekeeper@latest not the truncated mockup form)"
  - "Harness caveat honest: only Claude Code locally live-verified; 15 harnesses supported per docs"
  - "OriginStory links to /docs/security (resolves) not the mockup dead anchor"
  - "Biome auto-format applied (line-length only, no logic changes)"
metrics:
  duration: "~9 minutes"
  completed: "2026-06-08T18:18:38Z"
  tasks_completed: 3
  files_created: 3
  files_modified: 1
---

# Phase 15 Plan 02: Content Body (Origin Story + How It Works + Feature Cards) Summary

**One-liner:** Three server-component marketing sections — Nx Console origin story + documented-2026 threat table, reactive/proactive two-layer explainer + 3-step quickstart, and exactly 6 shipped-capability feature cards (D-03/D-04 reconciled) — wired into page.tsx between Hero and footer; `pnpm build` emits `web/out/index.html` with all 8 grep gates passing.

## Completed Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Origin story + documented-2026 threat list (HOME-02) | 3327c66 | web/components/home/origin-story.tsx |
| 2 | Two-layer explainer + 3-step quickstart (HOME-02) | 07fdf67 | web/components/home/how-it-works.tsx |
| 3 | Six shipped-capability feature cards + page.tsx wiring (HOME-03) | 81f84b9 | web/components/home/feature-cards.tsx, web/app/page.tsx |

## Verification Results

- `cd web && pnpm exec tsc --noEmit` exits 0 after each task — PASS (x3)
- `cd web && pnpm build` exits 0; emits `web/out/index.html` — PASS
- `out/index.html` contains "Corroboration" — PASS
- `out/index.html` contains "Fail-closed" — PASS
- `out/index.html` contains "Editor-extension" — PASS
- `out/index.html` contains "Sentry" — PASS
- `out/index.html` contains "LlamaFirewall" — PASS
- `out/index.html` contains "policy" (case-insensitive) — PASS
- `out/index.html` contains "Nx Console" — PASS
- `out/index.html` contains "beekeeper dashboard" — PASS
- No `--color-bk-*` in any of the three new files — PASS
- `pnpm lint` (Biome) exits 0 after auto-format — PASS

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Formatting] Biome line-length violations in all three new components**
- **Found during:** Task 3 `pnpm lint` run
- **Issue:** Biome reported formatting errors: long `gridTemplateColumns` and `color-mix()` border strings exceeded the configured line length in feature-cards.tsx and how-it-works.tsx; a multi-line JSX `<p>` block in origin-story.tsx needed consolidation.
- **Fix:** `pnpm exec biome check --write` applied — formatting only (line-break placement), no logic changes.
- **Files modified:** web/components/home/feature-cards.tsx, web/components/home/how-it-works.tsx, web/components/home/origin-story.tsx
- **Commit:** 81f84b9 (included in Task 3 commit)

## Design Decisions

1. **React import position in origin-story.tsx**: The `import type React from "react"` needed for `React.CSSProperties` was placed after the data constants (co-located with usage) following Biome's import-order rules. No logic impact.

2. **HowItWorks as a composite export**: The `TwoLayers` and `Quickstart` sub-sections are internal functions; `HowItWorks` wraps them in a React Fragment. This keeps the two HOME-02 sub-sections in one file while giving `page.tsx` a single clean import.

3. **Threat list header wording**: The mockup's "Live threat catalog" header label was kept (accurately describes the catalog's nature) but the fake "synced 6 minutes ago" badge next to it was replaced with the static "Documented 2026 campaigns" chip. The word "Live" appears as a header on the catalog box, not as a real-time sync claim — this is an accurate characterization of the Beekeeper catalog system.

4. **Feature card icons**: Used Unicode glyphs (⬡ ⚡ 🛡 👁 🔥 📋) in amber-tinted bordered boxes, matching the mockup's `.glyph` mono-bullet style. No lucide-react dependency added (zero new npm deps per plan).

## Known Stubs

None. All content is accurate static copy (D-03 compliant):
- Threat rows: documented 2026 campaigns with real dates/impacts
- Feature cards: exactly 6 shipped capabilities with README-sourced descriptions
- Quickstart: canonical install path; honest harness caveat
- Origin story: real Nx Console details (2026-05-18, nrwl.angular-console, 18-min/3800-repos)

## Threat Surface Scan

All threats from the plan's `<threat_model>` are mitigated:

- T-15-05 (content integrity / feature cards): 6 shipped capabilities only; D-03/D-04 compliant; all 8 grep gates pass. MITIGATED.
- T-15-06 (content integrity / threat list): Fake live-sync removed; Nx details match documented campaign. MITIGATED.
- T-15-07 (content integrity / commands): Full canonical install path; honest harness caveat. MITIGATED.
- T-15-08 (XSS): No `dangerouslySetInnerHTML`; all content JSX-escaped. MITIGATED.
- T-15-SC (supply chain): Zero new npm dependencies. ACCEPTED.

## Self-Check: PASSED

- web/components/home/origin-story.tsx: EXISTS
- web/components/home/how-it-works.tsx: EXISTS
- web/components/home/feature-cards.tsx: EXISTS
- web/app/page.tsx: MODIFIED (FeatureCards + OriginStory + HowItWorks wired in)
- web/out/index.html: EXISTS, all 8 grep gates PASSED
- Commits: 3327c66, 07fdf67, 81f84b9 — all verified in git log
