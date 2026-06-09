---
phase: 16-3d-layer
plan: 01
subsystem: ui
tags: [three, react-three-fiber, drei, webgl, svg, playwright, static-export, pnpm]

requires:
  - phase: 15-marketing-home
    provides: web/app/page.tsx + web/components/home/* (the hero the 3D layer mounts into)
  - phase: 12-design-system
    provides: useReducedMotion() hook, dual-theme tokens (teal/amber), globals.css sr-only
provides:
  - Pinned 3D toolchain installed (three/@react-three/fiber/@react-three/drei/@types/three), workspace-isolated
  - web/public/hero-hive.svg — static honeycomb SVG, the server-rendered LCP fallback element
  - web/tests/gfx_spec.py — GFX-01..04 Playwright harness (GFX-01 live; 02/03/04 staged)
affects: [16-02 (consumes deps + svg + harness), 16-03 (extends harness), 19-test-ci]

tech-stack:
  added: [three@0.184.0, "@react-three/fiber@9.6.1", "@react-three/drei@10.7.7", "@types/three@0.184.1"]
  patterns:
    - "3D deps pinned exact (no caret) into root pnpm-lock.yaml for supply-chain reproducibility"
    - "Playwright GFX harness mirrors home_spec.py on a distinct port (4200) with visible PENDING placeholders for staged waves"

key-files:
  created:
    - web/public/hero-hive.svg
    - web/tests/gfx_spec.py
  modified:
    - web/package.json
    - pnpm-lock.yaml

key-decisions:
  - "Package-legitimacy gate verified live against npm registry (versions exist, no install-time hooks) before maintainer approval"
  - "Exact-pinned versions (no caret) to lock supply-chain resolutions (T-16-SC)"
  - "next.config.mjs transpilePackages already declared all three (verify-only, no edit)"

patterns-established:
  - "Pattern 1: GFX harness on port 4200 with pending()/ok()/fail() — staged blocks fill in across waves without ever silently passing"

requirements-completed: [GFX-01, GFX-03]

duration: ~25min
completed: 2026-06-09
---

# Phase 16 Plan 01: 3D Foundation Summary

**Pinned Three.js/R3F/drei stack installed workspace-isolated, static honeycomb `hero-hive.svg` LCP fallback authored, and the GFX-01..04 Playwright harness created (GFX-01 enforced live; 02/03/04 staged for later waves).**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-06-09
- **Tasks:** 4 (1 blocking checkpoint + 3 auto)
- **Files modified:** 4

## Accomplishments
- Maintainer-approved package-legitimacy gate, then installed `three@0.184.0 / @react-three/fiber@9.6.1 / @react-three/drei@10.7.7 / @types/three@0.184.1` inside `web/` only (Go module untouched), exact-pinned into the root `pnpm-lock.yaml`.
- Authored `web/public/hero-hive.svg` — a self-contained 400×400 honeycomb (7 flat-top hexagons, teal `#39c5cf` cells + amber `#e3b341` mediation accents), the future server-rendered LCP element.
- Created `web/tests/gfx_spec.py` mirroring `home_spec.py` on port 4200: GFX-01 (server-HTML clean + canvas-count ≤1 post-hydration) enforced now; GFX-02/03/04 are visible labeled PENDING blocks for Plans 16-02/16-03.
- `pnpm build` green throughout (static export intact, `out/index.html` 119 KB); `home_spec.py` unaffected.

## Task Commits

1. **Task 1: Package-legitimacy gate** — checkpoint (maintainer "approved"; no commit)
2. **Task 2: Install pinned 3D stack** — `72ca292` (feat)
3. **Task 3: Author hero-hive.svg** — `c8f5a98` (feat)
4. **Task 4: gfx_spec.py harness** — `acf72b9` (feat)

## Files Created/Modified
- `web/package.json` / `pnpm-lock.yaml` — 4 pinned 3D deps (exact, no caret)
- `web/public/hero-hive.svg` — static honeycomb LCP fallback
- `web/tests/gfx_spec.py` — GFX-01..04 harness (port 4200)

## Decisions Made
- Verified package legitimacy live against the npm registry (versions present, no `postinstall`/`preinstall`/`install` hooks) — strengthened the RESEARCH.md `[ASSUMED OK]` audit before maintainer sign-off.
- Kept versions exact-pinned (no caret) for supply-chain reproducibility.

## Deviations from Plan
None — plan executed exactly as written. (`transpilePackages` confirmed already complete → verify-only as the plan anticipated.)

## Issues Encountered
- Only peer-dependency warning is a **pre-existing** fumadocs `zod` 3.x/4.x mismatch (from Phase 13) — unrelated to the 3D stack, which introduced zero new peer conflicts (as RESEARCH predicted).
- GSD tooling note (not blocking): `phase-plan-index`/`init.execute-phase` can't resolve this phase because `extractPhaseToken("16-3d-layer")` greedily reads the digit-led `3d` token → `"16-3d"`. Execution proceeds inline; STATE/ROADMAP hand-managed.

## User Setup Required
None — no external service configuration required.

## Next Phase Readiness
- Deps + SVG + harness all in place — Plan 16-02 can implement the R3F components against an assertion target that already exists.
- Plan 16-02 will: create `hero-canvas-wrapper.tsx` (use-client `dynamic(ssr:false)`), `hero-canvas.tsx` (capability gate + single Canvas + theme-aware hive + sr-only sibling), modify `hero.tsx` (server-rendered SVG `<img>` LCP + wrapper slot), and fill the GFX-02/GFX-03 harness blocks.

---
*Phase: 16-3d-layer*
*Completed: 2026-06-09*
