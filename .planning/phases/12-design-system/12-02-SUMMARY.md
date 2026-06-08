---
phase: 12-design-system
plan: "02"
subsystem: web/providers
tags: [react-context, next-themes, reduced-motion, providers, design-system]
requirements: [DSYS-02, DSYS-03]

dependency_graph:
  requires: ["12-01"]
  provides: ["web/app/providers.tsx", "web/lib/reduced-motion.tsx"]
  affects: ["web/app/layout.tsx (Plan 03 wires Providers)", "Phase 16 Hero3D (useReducedMotion contract)"]

tech_stack:
  added: []
  patterns:
    - "next-themes ThemeProvider with class strategy, storageKey=bk-theme, disableTransitionOnChange"
    - "ReducedMotionProvider: matchMedia + data attribute + React context"
    - "useReducedMotion(): boolean — locked hook signature for Phase 16 R3F"
    - "Phase 13 RootProvider insertion-point comment pattern"

key_files:
  created:
    - web/app/providers.tsx
    - web/lib/reduced-motion.tsx
  modified: []

decisions:
  - "ThemeProvider is outermost context so Phase 13 RootProvider can consume it from inside"
  - "ReducedMotionProvider nested inside ThemeProvider (DSYS-03 nesting contract from UI-SPEC)"
  - "Phase 13 insertion marker as literal JSX comment around children — not an import stub"
  - "Biome auto-format applied to ternary expressions in reduced-motion.tsx (no logic change)"

metrics:
  duration_seconds: 226
  completed_date: "2026-06-08"
  tasks_total: 2
  tasks_completed: 2
  files_created: 2
  files_modified: 1
---

# Phase 12 Plan 02: Providers (ThemeProvider + ReducedMotionProvider) Summary

**One-liner:** next-themes ThemeProvider (class strategy, bk-theme persistence) wrapping ReducedMotionProvider with matchMedia-backed useReducedMotion() hook and data-reduced-motion attribute.

---

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | ReducedMotionProvider + useReducedMotion hook | 646df2c | web/lib/reduced-motion.tsx (created) |
| 2 | Providers wrapper (ThemeProvider + ReducedMotionProvider) | 78bce3b | web/app/providers.tsx (created), web/lib/reduced-motion.tsx (Biome format) |

---

## Verification Results

- `web/lib/reduced-motion.tsx` exists; first line is `"use client";`
- `export function ReducedMotionProvider` present
- `export function useReducedMotion(): boolean` present (locked Phase 16 signature)
- `prefers-reduced-motion: reduce` matchMedia query present
- `document.documentElement.dataset.reducedMotion` write present (data-reduced-motion attribute)
- `web/app/providers.tsx` exists; first line is `"use client";`
- `export function Providers` present
- `storageKey="bk-theme"` present (exact localStorage key)
- `attribute="class"` present
- `disableTransitionOnChange` present
- `@/lib/reduced-motion` import present (ReducedMotionProvider wired)
- `RootProvider` present as comment (Phase 13 insertion marker)
- `pnpm build` exits 0 (both pre- and post-Task 2)
- `pnpm lint` clean

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Format] Biome formatting applied to reduced-motion.tsx**
- **Found during:** Task 2 (`pnpm lint` check)
- **Issue:** Biome formatter required ternary operators to be split across lines and the function parameter destructuring to use multi-line form. Single-line form in the verbatim UI-SPEC code did not match Biome's configured line-length rules.
- **Fix:** Ran `pnpm exec biome check --write --unsafe lib/reduced-motion.tsx app/providers.tsx`. Biome rewrote one file (`reduced-motion.tsx`); `providers.tsx` required no changes. Zero logic changes — acceptance criteria (grep patterns) all still match.
- **Files modified:** web/lib/reduced-motion.tsx
- **Commit:** 78bce3b (included with Task 2 changes)

---

## Known Stubs

None — both files are complete implementations. `providers.tsx` contains a deliberate comment marker for Phase 13 (`{/* Phase 13 inserts: <RootProvider theme={{ enabled: false }}> here */}`), which is intentional scaffolding, not a stub.

---

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced. The two trust boundaries from the plan's threat model (localStorage ↔ client for theme persistence, OS media query → DOM for reduced-motion attribute) are explicitly accepted per T-12-03 and T-12-04.

---

## Decisions Made

1. **ThemeProvider is outermost context** — ensures Phase 13's `RootProvider theme={{ enabled: false }}` can be inserted inside ThemeProvider without creating a second, conflicting ThemeProvider instance (Pitfall 3 from RESEARCH.md).
2. **ReducedMotionProvider nested inside ThemeProvider** — matches the nesting contract in UI-SPEC §Nesting in Providers (lines 779–790).
3. **Phase 13 marker as JSX comment** — literal `{/* Phase 13 inserts: … */}` / `{/* Phase 13 closes: … */}` comments around `{children}` inside ReducedMotionProvider, following PATTERNS.md §web/app/providers.tsx verbatim.
4. **Biome format auto-applied** — same approach as Plan 01; no manual bypass of linter. Logic is unchanged.

---

## Self-Check: PASSED

- `web/lib/reduced-motion.tsx` — FOUND
- `web/app/providers.tsx` — FOUND
- Commit `646df2c` — FOUND (git log confirms)
- Commit `78bce3b` — FOUND (git log confirms)
- `pnpm build` — exits 0
- `pnpm lint` — clean
- globals.css — NOT modified (confirmed)
- layout.tsx — NOT modified (confirmed)
- STATE.md — NOT modified (confirmed)
- ROADMAP.md — NOT modified (confirmed)
