---
phase: 15-marketing-home
plan: "03"
subsystem: web/marketing-home
tags: [marketing, next.js, react, home-page, harness-matrix, honesty-callout, playwright, HOME-04, HOME-05]
dependency_graph:
  requires: [phase-15-wave-1, phase-15-wave-2, phase-12-design-system, phase-13-docs-pipeline]
  provides:
    - web/components/home/harness-matrix.tsx (HOME-04 — 15 harness support matrix with honest tier labels)
    - web/components/home/honesty-callout.tsx (HOME-05 — four documented known gaps, no overclaiming)
    - web/tests/home_spec.py (both-theme + above-the-fold Playwright proof)
    - web/app/page.tsx (final assembly: Hero > FeatureCards > OriginStory > HowItWorks > HarnessMatrix > HonestyCallout)
  affects: [web/out/index.html, HOME-01, HOME-02, HOME-03, HOME-04, HOME-05]
tech_stack:
  added: []
  patterns:
    - server-components
    - raw-theme-tokens
    - color-mix-dual-theme-safe-callout
    - python-playwright-on-static-export
    - biome-lint
key_files:
  created:
    - web/components/home/harness-matrix.tsx
    - web/components/home/honesty-callout.tsx
    - web/tests/home_spec.py
  modified:
    - web/app/page.tsx
decisions:
  - "HarnessMatrix groups all 15 harnesses verbatim from docs/harness-support-matrix.md into three tier blocks; Claude Code the only Live-verified entry; 14 others labeled documented contract"
  - "HonestyCallout follows BreakingChangeCallout pattern (coral border-left + color-mix) per D-03 framing — coral reads honest-disclosure, not error"
  - "Python Playwright home_spec.py follows Phase 13/14 pattern (http.server + sync_playwright chromium); no new npm deps added"
  - "Playwright theme toggle uses explicit .dark/.light class injection rather than relying on next-themes system default — needed because static HTML has no class until JS hydrates (Theme-Toggle-Fix deviation)"
  - "page.tsx final section order: Hero > FeatureCards > OriginStory > HowItWorks > HarnessMatrix > HonestyCallout; SITE-03 live deploy deferred per D-02"
metrics:
  duration: "~30 minutes"
  completed: "2026-06-08"
  tasks_completed: 3
  files_created: 3
  files_modified: 1
---

# Phase 15 Plan 03: Harness Matrix + Honesty Callout + Final Assembly Summary

**One-liner:** 15-harness support matrix (HOME-04, sourced verbatim from docs/harness-support-matrix.md with honest tier caveats) + four-gap honesty callout (HOME-05, from THREAT-MODEL.md §8) assembled into the final page; `pnpm build` exits 0 with all grep gates passing and Python Playwright proves both themes + above-the-fold layout at 1280x800.

## Completed Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | 15-harness support matrix (HOME-04) | 21b3fe2 | web/components/home/harness-matrix.tsx |
| 2 | Honesty / known-gaps callout (HOME-05) | 98d6ab3 | web/components/home/honesty-callout.tsx |
| 3 | Final page assembly + build gate + Playwright both-theme proof | de8b252 | web/app/page.tsx, web/tests/home_spec.py |

## Verification Results

### Build Gate

- `cd web && pnpm exec tsc --noEmit` exits 0 after each task — PASS (x3)
- `cd web && pnpm build` exits 0; emits `web/out/index.html` — PASS
- `pnpm lint` (Biome) — 51 files checked, no fixes applied — PASS

### Grep Gates on out/index.html (all 21 pass)

**All 15 harness names:**
- `out/index.html` contains "Claude Code" — PASS
- `out/index.html` contains "Codex" — PASS
- `out/index.html` contains "Cursor" — PASS
- `out/index.html` contains "Augment" — PASS
- `out/index.html` contains "CodeBuddy" — PASS
- `out/index.html` contains "Qwen Code" — PASS
- `out/index.html` contains "Gemini CLI" — PASS
- `out/index.html` contains "Copilot" — PASS
- `out/index.html` contains "Antigravity" — PASS
- `out/index.html` contains "Windsurf" — PASS
- `out/index.html` contains "Hermes" — PASS
- `out/index.html` contains "Cline" — PASS
- `out/index.html` contains "OpenCode" — PASS
- `out/index.html` contains "Kilo" — PASS
- `out/index.html` contains "Trae" — PASS

**Known-gap markers:**
- `out/index.html` contains "release_age" — PASS
- `out/index.html` contains "0.0.0.0" — PASS
- `out/index.html` contains "fail-OPEN" — PASS
- `out/index.html` contains "UNGUARDED" — PASS

**Doc links:**
- `out/index.html` contains "/docs/integration" — PASS
- `out/index.html` contains "/docs/security" — PASS

### Playwright Both-Theme + Above-the-Fold Proof

Script: `web/tests/home_spec.py` (Python Playwright 1.57.0, cached chromium)
Pattern: Python http.server serving `out/` on port 4199, sync_playwright chromium

| Test | Assertion | Result |
|------|-----------|--------|
| Test 1 | Hero headline "autonomous coding agents" visible at 1280x800 | PASS |
| Test 1 | Install chip above fold (y=535, h=42; y+h=577 ≤ 800) | PASS |
| Test 1 | "Read the docs" link above fold (y=673, h=42; y+h=715 ≤ 800) | PASS |
| Test 2 | Dark body background = rgb(10, 13, 18) (#0a0d12) | PASS |
| Test 2 | Light body background = rgb(248, 249, 250) (#f8f9fa) | PASS |
| Test 2 | Backgrounds differ between themes | PASS |
| Test 3 | All 15 harness names in rendered DOM | PASS (15/15) |
| Test 4 | All 4 known-gap markers in rendered DOM | PASS (4/4) |

**Overall Playwright result: ALL ASSERTIONS PASSED**

## HOME Requirements Status

| Requirement | Status | Evidence |
|-------------|--------|----------|
| HOME-01 | SATISFIED (Wave 1) | Hero above fold @1280x800 confirmed by Playwright Test 1 |
| HOME-02 | SATISFIED (Wave 2) | OriginStory + HowItWorks wired into page |
| HOME-03 | SATISFIED (Wave 2) | 6 shipped-capability feature cards |
| HOME-04 | SATISFIED (Wave 3) | 15-harness matrix with honest tier labels, Claude Code live-verified marker, Tier-3 UNGUARDED and Hermes fail-OPEN caveats, /docs/integration link |
| HOME-05 | SATISFIED (Wave 3) | 4 documented known gaps (Hermes fail-open, Tier-3 unguarded, release_age unenforced, --bind 0.0.0.0), /docs/security link, no overclaiming |

SITE-03 (live Vercel deploy) remains deferred per D-02 — pending repo push + Vercel account setup.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Playwright theme detection required explicit class injection**
- **Found during:** Task 3 Playwright run (first attempt)
- **Issue:** Test 2 initially compared the system-default rendered state (next-themes defaults to `system` → Chromium reported system theme as "light" in headless mode → `rgb(248,249,250)`) with the explicitly set `.light` state. Both were the same light background, causing the diff assertion to fail: "Body background-color did NOT change between themes (both: rgb(248, 249, 250))".
- **Fix:** Changed Test 2 to explicitly inject `.dark` class on the html element first (removing `.light`) to get a true dark baseline `rgb(10,13,18)`, then inject `.light` (removing `.dark`) for the light baseline. Both explicit states are unambiguous and directly test the `--bg` raw token switching.
- **Files modified:** web/tests/home_spec.py
- **Commit:** de8b252

**2. [Rule 3 - Formatting] Biome formatting on harness-matrix.tsx and honesty-callout.tsx**
- **Found during:** Task 1 and Task 2 after biome check
- **Issue:** Long string literals in data arrays exceeded configured line length; biome auto-wrapped them.
- **Fix:** `pnpm exec biome check --write` applied — formatting only, no logic changes.
- **Files modified:** web/components/home/harness-matrix.tsx, web/components/home/honesty-callout.tsx
- **Commits:** 21b3fe2, 98d6ab3

## Design Decisions

1. **Playwright pattern matches Phase 13 (Python script, not .spec.ts):** No JS Playwright infrastructure exists in the project; Phase 13/14 established `python -m http.server` + `sync_playwright` as the standard. The test file is `web/tests/home_spec.py` (not `home.spec.ts`). The plan explicitly allowed this — "If the established pattern is a Python script rather than a .spec.ts, create the equivalent script."

2. **HarnessMatrix uses a 3-block TierGroup layout** instead of a single flat table: the grouped layout makes tier-level caveats (Tier-3 UNGUARDED, Tier-2 Hermes fail-open) prominent alongside the tier definition text, matching the mockup's card visual language used in FeatureCards.

3. **HonestyCallout uses coral accent, not red:** Per the plan spec — "coral or amber accent, NOT the red breaking-change red so it reads as 'honest disclosure' not 'error'". The coral `color-mix(in srgb, var(--coral) 8%, transparent)` background is visually distinct from the changelog red callout while using the identical structural pattern.

4. **Tier badge color tokens:** Tier 1 = amber, Tier 2 = coral, Tier 3 = red — matching the mockup's semantic color roles (amber = brand/positive, coral = caution, red = threat/gap).

## Known Stubs

None. All content is accurate:
- Harness matrix: verbatim from docs/harness-support-matrix.md; Claude Code the only live-verified; 14 others labeled "Documented contract"; Tier-3 UNGUARDED caveats sourced exactly.
- Honesty callout: four gaps sourced verbatim from THREAT-MODEL.md §8; no softening, no invented gaps.
- All doc links (/docs/integration, /docs/security) resolve under the Phase-13 Fumadocs docs skeleton.

## Threat Surface Scan

All threats from the plan's `<threat_model>` mitigated:

- T-15-09 (content integrity / harness matrix): Claude Code the only live-verified marker; Tier-3 UNGUARDED and Hermes fail-OPEN required caveats present; verified by grep gate + Playwright DOM assertion. MITIGATED.
- T-15-10 (content integrity / honesty callout): All four gaps (Hermes fail-open, Tier-3 unguarded, release_age unenforced, 0.0.0.0) asserted present by grep gate + Playwright. No gaps softened or omitted. MITIGATED.
- T-15-11 (link safety): All links are internal (/docs/integration, /docs/security). No target=_blank introduced. MITIGATED.
- T-15-12 (XSS): No dangerouslySetInnerHTML; all content JSX-escaped React. MITIGATED.
- T-15-13 (repudiation / false confidence): Playwright + grep gates fail the build if any of the four documented gaps is absent — the page cannot silently drop a limitation. MITIGATED.
- T-15-SC (supply chain): Zero new npm/runtime dependencies. Python Playwright already installed from Phase 13/14. ACCEPTED.

## Self-Check: PASSED

- web/components/home/harness-matrix.tsx: EXISTS
- web/components/home/honesty-callout.tsx: EXISTS
- web/tests/home_spec.py: EXISTS
- web/app/page.tsx: MODIFIED (HarnessMatrix + HonestyCallout imports + renders)
- web/out/index.html: EXISTS (pnpm build exit 0)
- Commits: 21b3fe2, 98d6ab3, de8b252 — all verified in git log

Grep gates on out/index.html:
- All 15 harness names: PASS
- release_age, 0.0.0.0, fail-OPEN, UNGUARDED: PASS
- /docs/integration, /docs/security: PASS

Playwright proof: ALL ASSERTIONS PASSED (Test 1: above-fold; Test 2: both-theme; Test 3: 15 harnesses; Test 4: 4 gaps)
