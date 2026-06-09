---
phase: 16-3d-layer
plan: 02
subsystem: ui
tags: [three, react-three-fiber, drei, webgl, next-dynamic, ssr-false, accessibility, next-themes, playwright]

requires:
  - phase: 16-3d-layer (plan 01)
    provides: pinned 3D deps, hero-hive.svg LCP fallback, gfx_spec.py harness
  - phase: 12-design-system
    provides: useReducedMotion() hook, next-themes resolvedTheme, dual-theme token hex pairs
provides:
  - Client-only R3F hive centerpiece behind dynamic(ssr:false) — GFX-01/02/03 satisfied
  - Fail-closed capability gate (reduced-motion + saveData + WebGL probe)
  - Single aria-hidden Canvas (frameloop=demand) + sr-only sibling + theme-aware honeycomb
  - hero.tsx server-rendered SVG <img> as the LCP element
affects: [16-03 (perf gate + UAT extends gfx_spec.py), 19-test-ci]

tech-stack:
  added: []
  patterns:
    - "dynamic(ssr:false) MUST live in a use-client wrapper; the Server Component imports the wrapper"
    - "aria-hidden set on the real <canvas> via R3F onCreated gl.domElement (props go to the wrapper div)"
    - "theme-aware THREE material colors via next-themes resolvedTheme + pinned hex pairs (never CSS vars / dark-only)"
    - "explicit three.js lights instead of drei <Environment> to avoid a runtime HDRI CDN fetch (bounded/offline bundle)"

key-files:
  created:
    - web/components/home/hero-canvas-wrapper.tsx
    - web/components/home/hero-canvas.tsx
  modified:
    - web/components/home/hero.tsx
    - web/tests/gfx_spec.py
    - web/mdx-components.tsx

key-decisions:
  - "Used explicit three.js lights (ambient/directional/point) instead of drei <Environment preset='city'> — avoids a third-party HDRI CDN fetch at runtime (bounded, offline, supply-chain-clean)"
  - "Dropped drei <Float> — continuous RAF is suspended under frameloop='demand', so it would be dead bundle weight (Pitfall 7)"
  - "Scoped cast in mdx-components.tsx to absorb R3F v9's global JSX.IntrinsicElements augmentation"

patterns-established:
  - "Pattern: R3F-on-static-export — all three/R3F imports behind a use-client dynamic(ssr:false) chunk so out/index.html stays free of 3D symbols"

requirements-completed: [GFX-01, GFX-02, GFX-03]

duration: ~40min
completed: 2026-06-09
---

# Phase 16 Plan 02: 3D Layer Summary

**Interactive client-only R3F hive centerpiece behind dynamic(ssr:false) with a fail-closed capability gate, a single aria-hidden Canvas + sr-only sibling, theme-aware honeycomb materials, and a server-rendered SVG LCP element — GFX-01/02/03 satisfied and asserted.**

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-06-09
- **Tasks:** 4 (all auto)
- **Files modified:** 5 (3 component + test + 1 type-fix)

## Accomplishments
- `hero-canvas-wrapper.tsx` — the `"use client"` `dynamic(ssr:false)` isolation boundary keeping three/R3F/drei out of the static-export SSG pass.
- `hero-canvas.tsx` — fail-closed `useCanMount3D()` gate (reduced-motion / `saveData` / WebGL probe with `failIfMajorPerformanceCaveat`), a single `aria-hidden` `<Canvas frameloop="demand">`, a 7-cell theme-aware honeycomb (`HiveMesh`/`HexCell`), `PresentationControls` drag, and an sr-only `<p>` sibling. `aria-hidden` is forced onto the real `<canvas>` via `onCreated`.
- `hero.tsx` — server-rendered `<img src="/hero-hive.svg" fetchPriority="high">` LCP element + `<HeroCanvasWrapper />` slot; stays a Server Component; all prior hero content preserved.
- `gfx_spec.py` — GFX-02 (single canvas) + GFX-03 (reduced-motion fallback + aria/sr-only) implemented and green; GFX-04 still staged.
- Build clean (`out/index.html` free of `<canvas`/`@react-three`/`WebGLRenderingContext`, SVG + `fetchpriority` present); `tsc`, Biome, `gfx_spec.py`, and `home_spec.py` all green.

## Task Commits

1. **Tasks 1–2: wrapper + capability-gated canvas (+ mdx type-fix)** — `ec0875b` (feat)
2. **Task 3: hero.tsx SVG-LCP element + slot** — `ab83b36` (feat)
3. **Task 4: GFX-02/GFX-03 assertions** — `130744e` (test)

(Tasks 1–2 share a commit: the wrapper imports the canvas, and the canvas's R3F import requires the mdx-components type-fix — minimal buildable unit.)

## Files Created/Modified
- `web/components/home/hero-canvas-wrapper.tsx` — use-client dynamic(ssr:false) boundary
- `web/components/home/hero-canvas.tsx` — capability gate + Canvas + hive + sr-only
- `web/components/home/hero.tsx` — SVG LCP `<img>` + wrapper slot (Server Component)
- `web/tests/gfx_spec.py` — GFX-02/03 assertions
- `web/mdx-components.tsx` — scoped cast for R3F global-JSX augmentation

## Decisions Made
- **Explicit lights over drei `<Environment>`** — the preset fetches a ~1 MB HDRI from a third-party CDN at runtime; explicit `ambient`/`directional`/`point` lights keep the bundle self-contained, offline-safe, and bounded (GFX-04 spirit + supply-chain hygiene for a security product).
- **Dropped `<Float>`** — `frameloop="demand"` suspends continuous RAF, so Float would render no motion while adding bundle weight (Pitfall 7). drei imports remain named (`PresentationControls`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] R3F v9 global JSX augmentation broke `mdx-components.tsx` type-check**
- **Found during:** Task 2 (first import of `@react-three/fiber`)
- **Issue:** Importing R3F activates its global `React.JSX.IntrinsicElements` augmentation (adds three.js elements, one typed `Component<never>`), which pollutes `mdx/types`' `MDXComponents` index signature (`Component<any>`) and failed `tsc`/`pnpm build`. Wave 1 built fine because the package was installed but not yet imported.
- **Fix:** Scoped `as MDXComponents` cast on the `useMDXComponents` return, with an explanatory comment. Returned shape unchanged.
- **Files modified:** web/mdx-components.tsx
- **Verification:** `pnpm exec tsc --noEmit` exits 0; `pnpm build` green; docs/changelog pages unaffected.
- **Committed in:** `ec0875b` (Tasks 1–2 commit)

**2. [Design — bundle/robustness] Explicit lights instead of drei `<Environment>`; dropped `<Float>`**
- See Decisions Made. Aligned with GFX-04 bounded-bundle + offline/supply-chain ethos; drei imports stay named.

---

**Total deviations:** 2 (1 blocking type-fix, 1 design substitution)
**Impact on plan:** The type-fix was required to compile. The lighting/Float substitution improves bundle boundedness and offline safety with no loss of the required behaviors (single canvas, theme-aware, drag, fail-closed). No scope creep.

## Issues Encountered
- Headless Chromium rejects WebGL under `failIfMajorPerformanceCaveat:true` (SwiftShader), so the canvas never mounts in the automated suite → GFX-03 aria-hidden/sr-only takes its labeled headless-fallback path. The real-browser canvas path (mount, aria, drag, theme recolor) is verified by the maintainer UAT in Plan 16-03.

## User Setup Required
None.

## Next Phase Readiness
- GFX-01/02/03 complete and green. Plan 16-03 implements GFX-04 (Playwright LCP proxy < 2500ms + no-context-leak across nav), optionally adds `lighthouse`, and runs the maintainer visual/interaction UAT (the real-browser aria/drag/theme/reduced-motion checks).

---
*Phase: 16-3d-layer*
*Completed: 2026-06-09*
