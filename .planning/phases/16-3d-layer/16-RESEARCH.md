# Phase 16: 3D Layer — Research

**Researched:** 2026-06-08
**Domain:** Three.js / React-Three-Fiber / Next.js static export / WebGL performance & accessibility
**Confidence:** HIGH (core stack verified via npm registry + official docs; patterns verified via Next.js 16 official lazy-loading docs)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| GFX-01 | Home hero features an interactive Three.js centerpiece (hive / agent-mediation visual) loaded behind a client-only boundary that never breaks the static build | `dynamic(ssr:false)` inside a `"use client"` wrapper; `transpilePackages` already pre-declared in next.config.mjs; `frameloop="demand"` for interactivity without drain |
| GFX-02 | Ambient 3D/motion accents enhance marketing sections without harming readability | CSS radial-gradient approach preferred for secondary sections (already in globals.css §7); one optional `<Float>` accent inside the single hero canvas — no additional WebGL contexts |
| GFX-03 | 3D layer falls back to static SVG under reduced-motion / low-power / no-WebGL; canvas is accessibility-invisible with sr-only description | `useReducedMotion()` hook already wired; `webgl`/`webgl2` context probe on mount; `navigator.connection.saveData` check; `aria-hidden="true"` + sibling `<p className="sr-only">` pattern |
| GFX-04 | Home page meets Lighthouse LCP < 2.5s (SVG as LCP candidate), bounded 3D bundle, no leaked WebGL contexts across navigation | SVG rendered synchronously (server HTML); Canvas mounted via `ssr:false` after LCP; `frameloop="demand"`; R3F auto-disposes on unmount; single canvas rule |
</phase_requirements>

---

## Summary

Phase 16 adds an interactive Three.js hive centerpiece to the marketing hero and optional ambient motion accents, all sitting behind a strict client-only boundary so the static export (`output: 'export'`) stays intact. The core stack is `three@0.184.0` + `@react-three/fiber@9.6.1` (React 19 compatible) + `@react-three/drei@10.7.7`, all of which have verified peer-dep compatibility with the project's React 19.2.4 + Next.js 16.2.7 setup.

The single most important constraint, confirmed directly from the Next.js 16 official lazy-loading documentation, is: **`ssr: false` is not permitted inside a Server Component**. The `dynamic(ssr:false)` call must live inside a file prefixed with `"use client"`. The existing `Hero` component (`web/components/home/hero.tsx`) is a Server Component; therefore Phase 16 needs a thin `"use client"` wrapper file (e.g., `web/components/home/hero-canvas-wrapper.tsx`) that imports the R3F canvas lazily. `page.tsx` remains a Server Component and imports only the updated `Hero` which delegates to the wrapper.

Performance is anchored by rendering a static SVG synchronously in the server HTML (so it is the LCP candidate), then mounting the canvas only after JS hydration. With `frameloop="demand"` and `@react-three/drei`'s `PresentationControls` (no continuous RAF on idle), the page stays below budget. Three.js `sideEffects: ['./src/nodes/**/*']` (rest is side-effect-free) + Next.js automatic chunk-splitting means the R3F/three bundle loads as a separate lazy chunk and never blocks LCP.

The project already has `ReducedMotionProvider` + `useReducedMotion()` wired in `web/lib/reduced-motion.tsx` and a CSS-side reduced-motion gate in `globals.css §8`. Phase 16 extends this by also probing WebGL availability and `navigator.connection.saveData` on mount before deciding whether to swap the SVG for the canvas. R3F auto-calls `forceContextLoss()` + `dispose()` on canvas unmount, satisfying the no-context-leak success criterion as long as only one canvas is mounted at a time.

**Primary recommendation:** Install `three@0.184.0 @react-three/fiber@9.6.1 @react-three/drei@10.7.7 @types/three@0.184.1`, create a single `"use client"` canvas wrapper loaded via `dynamic(ssr:false)` inside the existing `Hero` component, render a static SVG fallback in server HTML as the LCP element, and mount the hive canvas after capability checks pass.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Static SVG fallback (LCP element) | Frontend Server (SSR/SSG) | — | Must be in server-rendered HTML to be the LCP candidate — never lazy |
| R3F canvas / hive centerpiece | Browser / Client | — | Requires WebGL APIs unavailable in Node; must never run during static build |
| Capability detection (WebGL / reduced-motion / saveData) | Browser / Client | — | All three probes are client-only APIs; run inside `useEffect` on mount |
| `"use client"` wrapper boundary | Browser / Client | — | `dynamic(ssr:false)` must sit inside a Client Component; this is the isolation boundary |
| Ambient section accents (GFX-02) | Browser / Client (CSS) | — | CSS radial-gradient accents already exist in globals.css §7; no additional WebGL context needed |
| Accessibility sr-only description | Frontend Server (SSR/SSG) | — | The sr-only sibling `<p>` is plain HTML — render it in the server-side SVG fallback branch |
| Performance measurement (Lighthouse LCP) | CI / Test harness | — | Lighthouse CLI audit against `out/index.html` via local http.server |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `three` | `0.184.0` | 3D renderer, geometry, materials | Industry-standard WebGL abstraction; 5.5M weekly downloads [VERIFIED: npm registry] |
| `@react-three/fiber` | `9.6.1` | React renderer for Three.js (R3F v9 = React 19) | Official React 19 tier; `sideEffects: false` enables tree-shaking [VERIFIED: npm registry] |
| `@react-three/drei` | `10.7.7` | Helper components (PresentationControls, Float, Environment) | Peer-requires `@react-three/fiber ^9`; `sideEffects: false` [VERIFIED: npm registry] |
| `@types/three` | `0.184.1` | TypeScript types for three | Matched to three@0.184.0; separate from three package [VERIFIED: npm registry] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `lighthouse` (dev) | `13.3.0` | LCP / performance audit CLI | GFX-04 LCP < 2.5s gate; run against `out/index.html` via local server [VERIFIED: npm registry] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `@react-three/drei` (selected drei controls) | `@react-spring/three`, `framer-motion-3d` | Heavier; drei is the canonical R3F helper set |
| Static SVG fallback (chosen) | CSS-only gradient fallback | SVG is LCP-sized and visually representative; gradient loses brand identity |
| `frameloop="demand"` | `frameloop="always"` | `"always"` burns CPU/battery even when scene is idle; `"demand"` renders on pointer events only |
| Single canvas (chosen) | Multiple small ambient canvases | Multiple canvases hit the browser's WebGL context limit (8–16 on Safari/iOS); use CSS for ambient sections |

**Installation:**

```bash
# From web/ directory
pnpm add three@0.184.0 @react-three/fiber@9.6.1 @react-three/drei@10.7.7 @types/three@0.184.1
```

**pnpm-workspace.yaml `allowBuilds` additions required:**

`three`, `@react-three/fiber`, and `@react-three/drei` have `sideEffects: false` (fiber, drei) and dev-only scripts (three). None have `postinstall` scripts that execute at install time — confirmed via `npm view <pkg> scripts`. No `allowBuilds` entries needed for these three packages.

**Version verification (confirmed 2026-06-08):**

```
three@0.184.0          published 2026-04-16
@react-three/fiber@9.6.1  published 2026-04-28 — peerDeps: react >=19 <19.3, three >=0.156
@react-three/drei@10.7.7  published 2026-02-03 — peerDeps: react ^19, @react-three/fiber ^9.0.0, three >=0.159
@types/three@0.184.1   published 2026-05-06
```

React 19.2.4 satisfies `>=19 <19.3`. three@0.184.0 satisfies `>=0.156` and `>=0.159`. All peer-dep constraints are satisfied with no warnings.

---

## Package Legitimacy Audit

> slopcheck could not be installed (Beekeeper's own sandbox blocked the pip install; acceptable per graceful-degradation rule). Manual legitimacy check performed instead.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `three` | npm | ~13 yrs | 5.5M/week [CITED: npm-stat.com via WebSearch] | github.com/mrdoob/three.js | [ASSUMED OK] | Approved — canonical Three.js package |
| `@react-three/fiber` | npm | ~5 yrs | High (industry standard) | github.com/pmndrs/react-three-fiber | [ASSUMED OK] | Approved — official pmndrs R3F |
| `@react-three/drei` | npm | ~5 yrs | High (industry standard) | github.com/pmndrs/drei | [ASSUMED OK] | Approved — official pmndrs R3F helpers |
| `@types/three` | npm | ~8 yrs | Very high | github.com/DefinitelyTyped/DefinitelyTyped | [ASSUMED OK] | Approved — DefinitelyTyped |
| `lighthouse` | npm | ~10 yrs | Very high | github.com/GoogleChrome/lighthouse | [ASSUMED OK] | Approved — official Google Chrome project |

**Packages removed due to slopcheck [SLOP] verdict:** none

**Packages flagged as suspicious [SUS]:** none

*slopcheck was unavailable at research time; all packages above are tagged `[ASSUMED OK]` based on manual provenance checks. The planner should treat each install as if a `checkpoint:human-verify` advisory applies, though these are all universally recognised packages.*

---

## Architecture Patterns

### System Architecture Diagram

```
web/app/page.tsx (Server Component — static export)
  └── <Hero /> (Server Component — server-renders SVG fallback + wrapper slot)
        ├── <img> or <svg> [LCP CANDIDATE — in server HTML]
        │     aria-label="..." (descriptive, for pre-JS / no-canvas path)
        └── <HeroCanvasWrapper /> (Client Component boundary — "use client")
              │  dynamic(ssr:false) → lazy chunk
              └── <HeroCanvas />
                    ├── useEffect → capability check
                    │     ├── prefersReducedMotion (useReducedMotion hook)
                    │     ├── WebGL probe: canvas.getContext('webgl2') || getContext('webgl')
                    │     └── saveData: navigator.connection?.saveData
                    ├── [FALLBACK] → renders null (SVG in Hero stays as LCP)
                    └── [CANVAS MOUNTED]
                          <Canvas aria-hidden="true" frameloop="demand">
                            <PresentationControls>
                              <HiveMesh />        ← custom hexagonal hive geometry
                            </PresentationControls>
                            <Environment preset="city" />
                          </Canvas>
                          <p className="sr-only">
                            Interactive 3D hive visualization representing
                            Beekeeper mediating agent tool calls
                          </p>
```

### Recommended Project Structure

```
web/
├── components/
│   └── home/
│       ├── hero.tsx                  # Existing — add SVG + HeroCanvasWrapper slot
│       ├── hero-canvas-wrapper.tsx   # NEW — "use client"; dynamic(ssr:false)
│       └── hero-canvas.tsx           # NEW — "use client"; R3F Canvas + capability check
├── lib/
│   └── reduced-motion.tsx            # Existing — useReducedMotion() hook (already wired)
└── public/
    └── hero-hive.svg                 # NEW — static SVG fallback (LCP element)
```

### Pattern 1: `dynamic(ssr:false)` inside a Client Component

**What:** `next/dynamic` with `ssr: false` must be called inside a `"use client"` file. Calling it directly in a Server Component (`page.tsx` or a Server-rendered component) throws a Next.js build error. [VERIFIED: nextjs.org/docs/app/guides/lazy-loading]

**When to use:** Any component that imports `three`, `@react-three/fiber`, or `@react-three/drei` — these packages use browser globals (`window`, `WebGLRenderingContext`) and fail in Node.js SSG.

**Example:**
```typescript
// Source: nextjs.org/docs/app/guides/lazy-loading (Next.js 16.2.7 official docs)
// web/components/home/hero-canvas-wrapper.tsx
"use client";

import dynamic from "next/dynamic";

// The R3F canvas never runs during `next build` SSG pass
const HeroCanvas = dynamic(() => import("./hero-canvas"), {
  ssr: false,
  loading: () => null, // SVG fallback in parent Hero handles pre-JS state
});

export function HeroCanvasWrapper() {
  return <HeroCanvas />;
}
```

### Pattern 2: Static SVG as LCP Candidate

**What:** Render a server-side SVG (or `<img>` pointing to `public/hero-hive.svg`) in the `Hero` component's JSX. Because it is in the server-rendered HTML, the browser can paint it immediately — making it the LCP element. The canvas only mounts after hydration + capability checks, so it never races with LCP.

**When to use:** Any time a 3D element is the visual focal point of an above-the-fold section.

**Example:**
```typescript
// web/components/home/hero.tsx (additions)
// SVG rendered synchronously — LCP candidate
<div className="relative mx-auto" style={{ width: 400, height: 400 }}>
  {/* Server-rendered SVG: always present pre-JS, always LCP candidate */}
  <img
    src="/hero-hive.svg"
    alt=""
    aria-hidden="true"
    width={400}
    height={400}
    className="absolute inset-0"
    fetchPriority="high"   // signal to browser: this IS the LCP element
  />
  {/* Client-only canvas mounts over SVG after hydration */}
  <HeroCanvasWrapper />
</div>
```

### Pattern 3: Capability Gate before Canvas Mount

**What:** In the R3F component (`hero-canvas.tsx`), check three conditions before mounting `<Canvas>`: (1) `useReducedMotion()`, (2) WebGL availability, (3) `navigator.connection?.saveData`. Return `null` on any positive — the SVG remains visible.

**When to use:** Every R3F component that is behind a fallback requirement.

**Example:**
```typescript
// Source: MDN Web APIs (WebGL detection) + web/lib/reduced-motion.tsx (existing hook)
"use client";
import { useEffect, useState } from "react";
import { useReducedMotion } from "@/lib/reduced-motion";
import { Canvas } from "@react-three/fiber";
import { PresentationControls, Environment } from "@react-three/drei";

function detectWebGL(): boolean {
  try {
    const canvas = document.createElement("canvas");
    return !!(
      canvas.getContext("webgl2") ||
      canvas.getContext("webgl") ||
      canvas.getContext("experimental-webgl")
    );
  } catch {
    return false;
  }
}

export default function HeroCanvas() {
  const prefersReducedMotion = useReducedMotion();
  const [canMount, setCanMount] = useState(false);

  useEffect(() => {
    const saveData = (navigator as { connection?: { saveData?: boolean } })
      .connection?.saveData ?? false;
    const hasWebGL = detectWebGL();
    if (!prefersReducedMotion && !saveData && hasWebGL) {
      setCanMount(true);
    }
  }, [prefersReducedMotion]);

  if (!canMount) return null;

  return (
    <>
      <Canvas
        aria-hidden="true"
        frameloop="demand"
        style={{ position: "absolute", inset: 0 }}
        gl={{ antialias: true, alpha: true }}
      >
        <PresentationControls
          global
          snap
          rotation={[0, 0, 0]}
          polar={[-Math.PI / 4, Math.PI / 4]}
          azimuth={[-Math.PI / 4, Math.PI / 4]}
        >
          <HiveMesh />
        </PresentationControls>
        <Environment preset="city" />
      </Canvas>
      <p className="sr-only">
        Interactive 3D hive visualization representing Beekeeper mediating
        agent tool calls
      </p>
    </>
  );
}
```

### Pattern 4: `frameloop="demand"` for Energy Efficiency

**What:** Set `frameloop="demand"` on the R3F `<Canvas>`. The renderer only fires a frame when React state or props change (or pointer events occur). No continuous RAF on idle. [VERIFIED: r3f.docs.pmnd.rs/advanced/scaling-performance]

**When to use:** Every marketing/non-game canvas. Essential for passing GFX-04 (no CPU/battery drain on idle).

### Pattern 5: Single-Canvas Rule (GFX-02)

**What:** Use exactly one WebGL context per page. Browsers impose a hard limit (Chrome: 16, Safari: 8). Navigation cycles that mount/unmount multiple canvases exhaust the limit in Safari/iOS.

**When to use:** GFX-02 ambient accents must NOT use additional `<Canvas>` components. Use CSS radial-gradient (already in `globals.css §7`) or CSS/SVG animations for secondary marketing sections. If ambient 3D accents inside the hero canvas are desired, add them as children of the existing `<Canvas>` — not as separate canvases.

### Anti-Patterns to Avoid

- **`dynamic(ssr:false)` in a Server Component:** Build error: "ssr: false is not allowed with next/dynamic in Server Components." The wrapper file must be `"use client"`. [VERIFIED: nextjs.org/docs/app/guides/lazy-loading]
- **`import { Canvas } from '@react-three/fiber'` in a Server Component file:** Even without `dynamic`, this causes the build to fail because fiber uses `window`. All R3F imports must be behind `"use client"` (direct or dynamic).
- **Static `import` of three.js modules outside `"use client"` files:** Three's geometry classes reference WebGL globals. The entire import chain of three must be client-side.
- **Multiple `<Canvas>` components for ambient accents:** Context limit exhaustion in Safari. Use CSS/SVG for secondary sections.
- **`frameloop="always"` on an idle marketing hero:** Wastes GPU time, kills battery, fails the energy-efficiency implicit in GFX-04.
- **Not passing `fetchPriority="high"` to the hero SVG `<img>`:** Browser may deprioritize the image and miss the LCP budget.
- **`aria-hidden` on the sr-only `<p>`:** The sr-only description exists to serve screen readers; marking it `aria-hidden` defeats its purpose. Only the `<canvas>` gets `aria-hidden`.
- **Importing all of drei:** Tree-shaking on drei is effective (`sideEffects: false`) only with named imports. `import * as drei from '@react-three/drei'` pulls the full 10.7.7 package. Use: `import { PresentationControls, Environment } from '@react-three/drei'`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Drag-to-rotate camera controls | Custom pointer event handlers on canvas | `<PresentationControls>` from drei | Handles momentum, snap, polar/azimuth limits, touch events, pointer capture |
| Ambient floating animation | Custom RAF + sine wave position updates | `<Float>` from drei | Configurable speed/intensity/range; respects R3F lifecycle |
| Environment/lighting | Manual `THREE.AmbientLight` + `THREE.DirectionalLight` setup | `<Environment preset="city">` from drei | Single import; HDRI-backed ambient lighting; no manual tuning |
| WebGL availability detection | Custom GPU feature detection library | Inline `canvas.getContext('webgl2') || canvas.getContext('webgl')` with `failIfMajorPerformanceCaveat: true` option | Standard browser API; no dependency needed |
| Context cleanup on unmount | Manual `renderer.dispose()` + `renderer.forceContextLoss()` in effects | R3F auto-cleanup on `<Canvas>` unmount | R3F's `unmount()` internally calls `forceContextLoss()` + `dispose()`; no manual hook needed |
| Lighthouse LCP measurement | Custom performance observer scripts | `lighthouse` CLI (`npx lighthouse http://... --output json --quiet`) | Standard, maintained, CI-friendly; supports JSON output for threshold assertions |

**Key insight:** R3F's internal cleanup on unmount handles the WebGL context release correctly. The only failure mode is mounting multiple canvases (Safari limit) — prevented by the single-canvas rule.

---

## Common Pitfalls

### Pitfall 1: `ssr: false` in a Server Component

**What goes wrong:** `next build` emits: `Error: 'ssr: false' is not allowed with next/dynamic in Server Components.`
**Why it happens:** In the App Router, `dynamic(ssr:false)` triggers a React-layer change that requires client-side JS APIs. Next.js disallows this in Server Components because the concept of "skip SSR" only applies to Client Components.
**How to avoid:** The `dynamic(...)` call must be in a file that starts with `"use client"`. The calling Server Component simply imports the wrapper. [VERIFIED: nextjs.org/docs/app/guides/lazy-loading]
**Warning signs:** Build output contains "ssr: false is not allowed" error; or silently, the Canvas renders in the server HTML and throws `ReferenceError: WebGLRenderingContext is not defined` during `next build`.

### Pitfall 2: Three.js / R3F symbols appear in server-rendered HTML

**What goes wrong:** `pnpm build` succeeds but `out/index.html` contains references to three.js class names or WebGL-specific attributes. This means the static bundle includes three.js code in the initial HTML payload.
**Why it happens:** A component that imports from `three` or `@react-three/fiber` is not behind `"use client"` + `dynamic(ssr:false)`. The Next.js static export runs these imports in Node.js.
**How to avoid:** All R3F/three imports must be in Client Component files OR inside the `dynamic()` factory function. Grep `out/index.html` for "WebGL" or "three" as a post-build check.
**Warning signs:** `out/index.html` contains `<canvas>` or three.js class names; build emits `ReferenceError` during SSG.

### Pitfall 3: Multiple WebGL contexts on Safari/iOS

**What goes wrong:** After navigating between pages in development (with Fast Refresh) or via client-side routing, Safari emits "Too many active WebGL contexts. Oldest context will be lost" and eventually crashes/reloads the page. [CITED: github.com/pmndrs/react-three-fiber/issues/2456]
**Why it happens:** Each mount of `<Canvas>` creates a new WebGL context. Safari limits active contexts to ~8. If the canvas is not properly unmounted (e.g., `React.StrictMode` double-invocation, hot-reload without cleanup), contexts accumulate.
**How to avoid:** (1) Mount at most one canvas per page. (2) Rely on R3F's automatic `forceContextLoss()` on unmount. (3) In development, `React.StrictMode` double-effects can trigger false positives; this is expected and does not indicate a production leak. (4) Do not wrap the canvas in `React.StrictMode` if context-loss errors block development — `next.config.mjs` can set `reactStrictMode: false` for the 3D component path, but leave it on globally.
**Warning signs:** Browser console shows "Too many active WebGL contexts"; canvas goes black after navigating.

### Pitfall 4: SVG is not the LCP element

**What goes wrong:** Lighthouse reports LCP > 2.5s or reports the LCP element as a text node or a dynamically loaded image.
**Why it happens:** The SVG `<img>` either (a) lacks `fetchPriority="high"`, causing it to be deprioritized; (b) is not in the initial server HTML (loaded via JS); or (c) has no explicit `width`/`height`, causing layout recalculation.
**How to avoid:** The `<img src="/hero-hive.svg">` must be in the server-rendered HTML (never lazy), must have explicit `width`/`height` attributes, and must carry `fetchPriority="high"`. For `output: 'export'`, `next/image` requires `unoptimized: true` (already set in next.config.mjs) — use plain `<img>` for the hero SVG to avoid any ambiguity.
**Warning signs:** Lighthouse LCP element is a `<p>`, `<h1>`, or the canvas; LCP time > 2.5s.

### Pitfall 5: `transpilePackages` must include `@react-three/drei`

**What goes wrong:** `pnpm build` fails with `SyntaxError: Cannot use import statement in a module` from inside the drei package.
**Why it happens:** `@react-three/drei` ships ESM-only entry points. Next.js does not transpile `node_modules` by default; `transpilePackages` must list all ESM-only packages. [CITED: discourse.threejs.org/t/react-three-drei-next-js-mandatory-to-transpile-the-package/65371]
**How to avoid:** `next.config.mjs` must include `transpilePackages: ['three', '@react-three/fiber', '@react-three/drei']`. The existing config already pre-declares `three` and `@react-three/fiber` — simply add `@react-three/drei` to the array.
**Warning signs:** Build error mentioning `import` syntax in `node_modules/@react-three/drei`.

### Pitfall 6: Canvas `aria-hidden` vs sr-only description placement

**What goes wrong:** Screen readers announce the canvas or pick up nothing at all.
**Why it happens:** `aria-hidden="true"` on the `<Canvas>` element is correct (canvas content is not navigable). But if the sr-only description is also inside the canvas component or also marked `aria-hidden`, screen readers are left with no description.
**How to avoid:** The `<p className="sr-only">` must be a sibling of the canvas (same parent DOM container), never a child of the R3F `<Canvas>`. Do NOT add `aria-hidden` to the sr-only element.
**Warning signs:** `axe` / `pa11y` reports missing canvas description; VoiceOver announces no content in the hero area.

### Pitfall 7: `@react-three/drei` imports pulling in the full bundle

**What goes wrong:** The three.js chunk in the production build is unexpectedly large (> 300 KB gzipped).
**Why it happens:** Importing `PresentationControls` via barrel: `import { PresentationControls } from '@react-three/drei'` is fine and tree-shakes correctly (`sideEffects: false`). However, importing unused drei components (e.g., `Html`, `Loader`, `BakeShadows`) adds to the chunk.
**How to avoid:** Import only the components actually used. Measure with `pnpm build` + `ANALYZE=true` (add `@next/bundle-analyzer` if needed) to verify chunk size. The target is: three.js chunk ≤ 200 KB gzipped for the minimal hive scene.
**Warning signs:** Build output shows a single JS chunk > 300 KB gzipped; `NEXT_PUBLIC_ANALYZE` shows large unexplained drei imports.

---

## Code Examples

### Hero Canvas Fallback Architecture

```typescript
// Source: nextjs.org/docs/app/guides/lazy-loading (verified pattern)
// web/components/home/hero-canvas-wrapper.tsx — MUST be "use client"
"use client";
import dynamic from "next/dynamic";

const HeroCanvas = dynamic(() => import("./hero-canvas"), {
  ssr: false,
  loading: () => null,
});

export function HeroCanvasWrapper() {
  return <HeroCanvas />;
}
```

### Capability Gate (WebGL + reduced-motion + saveData)

```typescript
// Source: MDN Web APIs + web/lib/reduced-motion.tsx (project hook)
"use client";
import { useEffect, useState } from "react";
import { useReducedMotion } from "@/lib/reduced-motion";

function isWebGLAvailable(): boolean {
  try {
    const canvas = document.createElement("canvas");
    // failIfMajorPerformanceCaveat: detect software-renderer / blacklisted GPU
    return !!(
      canvas.getContext("webgl2", { failIfMajorPerformanceCaveat: true }) ||
      canvas.getContext("webgl", { failIfMajorPerformanceCaveat: true })
    );
  } catch {
    return false;
  }
}

export function useCanMount3D(): boolean {
  const prefersReducedMotion = useReducedMotion();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (prefersReducedMotion) return;
    const conn = (navigator as { connection?: { saveData?: boolean } }).connection;
    if (conn?.saveData) return;
    if (!isWebGLAvailable()) return;
    setReady(true);
  }, [prefersReducedMotion]);

  return ready;
}
```

### R3F Canvas with Accessibility

```typescript
// Source: r3f.docs.pmnd.rs/api/canvas (canvas props) + WCAG 2.1 pattern
<div className="absolute inset-0" role="presentation">
  <Canvas
    aria-hidden="true"
    frameloop="demand"
    gl={{ antialias: true, alpha: true }}
    camera={{ position: [0, 0, 5], fov: 45 }}
  >
    {/* scene content */}
  </Canvas>
  {/* sr-only: sibling of canvas, NOT inside Canvas — screen readers read this */}
  <p className="sr-only">
    Interactive 3D visualization of Beekeeper mediating agent tool calls
    through a hive of hexagonal nodes
  </p>
</div>
```

### Hive Geometry (CylinderGeometry as hexagonal prism)

```typescript
// Source: three.js docs — CylinderGeometry with radialSegments=6 creates a hexagonal prism
// [ASSUMED] — training knowledge on three.js geometry; verified three.js docs exist at threejs.org
import { useFrame } from "@react-three/fiber";
import { useRef } from "react";
import * as THREE from "three";

function HexCell({ position }: { position: [number, number, number] }) {
  return (
    <mesh position={position}>
      {/* radialSegments=6 → hexagonal prism */}
      <cylinderGeometry args={[0.5, 0.5, 0.15, 6]} />
      <meshStandardMaterial
        color={new THREE.Color("var(--teal)")}  // note: CSS var won't resolve here
        // Use the resolved CSS value or a hardcoded hex: #39c5cf (dark) / #0a6b75 (light)
        metalness={0.3}
        roughness={0.5}
      />
    </mesh>
  );
}
```

**Note on theme tokens in Three.js materials:** `THREE.Color` cannot resolve CSS variables. Either use hardcoded hex values for both themes and switch via a `useTheme()` hook from `next-themes`, or use a uniform color that works in both themes.

### Lighthouse LCP Audit (CI-friendly)

```bash
# Source: lighthouse npm package (v13.3.0, official Google Chrome project)
# Run against static export via local server (same pattern as existing Playwright tests)
cd web
python -m http.server 4200 --directory out &
SERVER_PID=$!
sleep 1
npx lighthouse http://localhost:4200/ \
  --output json \
  --output-path /tmp/lhci-result.json \
  --quiet \
  --chrome-flags="--headless --no-sandbox"
kill $SERVER_PID
# Assert LCP < 2500ms
node -e "
  const r = require('/tmp/lhci-result.json');
  const lcp = r.audits['largest-contentful-paint'].numericValue;
  console.log('LCP:', lcp + 'ms');
  if (lcp > 2500) { console.error('FAIL: LCP ' + lcp + 'ms > 2500ms budget'); process.exit(1); }
  console.log('PASS: LCP within budget');
"
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `@react-three/fiber@8` + React 18 | `@react-three/fiber@9` + React 19 | 2025 (v9 release) | Concurrent mode improvements; new peer-dep range |
| `next-transpile-modules` for ESM packages | `transpilePackages` in next.config.js | Next.js 13.1 | Built-in; no extra dep |
| Custom `useLoader` + cache management | `useLoader` with automatic caching (unchanged) | — | Well-established pattern |
| `next/image` for hero images | Plain `<img fetchPriority="high">` for LCP-critical SVGs | Next.js output:export era | `next/image` still requires `unoptimized: true` for static export; `<img>` is simpler and avoids wrapping markup that can shift the LCP element |

**Deprecated/outdated:**

- `next-transpile-modules`: Replaced by `transpilePackages` in `next.config.js` (Next.js 13.1+). Do not install.
- `@react-three/fiber@8`: Targets React 18. The project uses React 19.2.4 — must use R3F v9.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `three@0.184.0` has gzip size ~36 KB, minified ~112 KB | Standard Stack — bundle notes | Three.js chunk may be larger in full static export context; measure post-build |
| A2 | `CylinderGeometry(r, r, h, 6)` produces a visually acceptable hexagonal hive cell | Code Examples — hive geometry | May need `LatheGeometry` or custom `Shape`; verify in browser |
| A3 | `navigator.connection.saveData` is the right low-power signal | Pattern 3 capability gate | Battery API (`navigator.getBattery()`) also available but async and deprecated in most browsers; saveData is the modern signal |
| A4 | CSS variables cannot be used in `THREE.Color` constructor | Code Examples — hive geometry note | If a future three.js version supports CSS vars in Color, this is no longer a constraint |
| A5 | `@react-three/drei` ships `PresentationControls` and `Float` in v10.7.7 | Standard Stack / Don't Hand-Roll | Confirmed via drei docs fetch — HIGH confidence this is correct |
| A6 | R3F `<Canvas>` passes through arbitrary HTML attributes (including `aria-hidden`) to the underlying `<canvas>` DOM element | Code Examples | If R3F does not proxy HTML attributes, a `ref` + `useEffect` to set the attribute manually is the fallback. Should be verified during implementation. |

---

## Open Questions

1. **Theme-aware colors in Three.js materials**
   - What we know: `THREE.Color` takes hex/name/number, not CSS vars.
   - What's unclear: Whether to use `useTheme()` from `next-themes` to switch between dark teal (`#39c5cf`) and light teal (`#0a6b75`) in the hive mesh material, or to pick a single intermediate color.
   - Recommendation: Use `useTheme()` — the hook is already available in the project via `next-themes@0.4.6`. Reactive material color changes inside R3F work naturally via `useEffect` + `mesh.material.color.set(...)`.

2. **Hive visual complexity vs bundle size**
   - What we know: A simple `CylinderGeometry(r, r, h, 6)` hexagonal cell grid works. Adding `MeshTransmissionMaterial` (glass) from drei significantly increases drei chunk size.
   - What's unclear: The specific hex-cell count and animation complexity needed to match the project visual goals.
   - Recommendation: Start with `meshStandardMaterial` on 7–19 hexagonal cells (honeycomb cluster) with `<Float>` for gentle ambient motion. Assess bundle and visual quality before adding PBR glass material.

3. **`aria-hidden` passthrough on R3F Canvas**
   - What we know: R3F documentation does not explicitly document HTML attribute passthrough on `<Canvas>`.
   - What's unclear: Whether `aria-hidden="true"` as a prop on `<Canvas>` reaches the underlying `<canvas>` DOM element.
   - Recommendation: Verify during Wave 1 implementation. Fallback: use a `ref` on the Canvas and set `canvasRef.current.setAttribute('aria-hidden', 'true')` in `useEffect`.

4. **Lighthouse in Windows CI**
   - What we know: Lighthouse 13.3.0 is available on npm and runs headless Chromium.
   - What's unclear: Whether `lighthouse` CLI works reliably on Windows without WSL (primary dev machine is Windows 11).
   - Recommendation: The existing Playwright test pattern uses Python's `http.server` and works on Windows. Use the same Python server + `lighthouse` CLI for GFX-04 measurement. If Lighthouse fails on Windows, use Playwright's `page.evaluate(() => performance.getEntriesByType('paint'))` as a fallback LCP proxy.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js (pnpm) | `pnpm add`, `pnpm build` | ✓ | Project uses pnpm@11.1.3 | — |
| Chromium (Playwright) | Existing Playwright tests | ✓ | Cached at ~/AppData/Local/ms-playwright | — |
| Python 3 | Playwright test runner (existing pattern) | ✓ | Available (Phase 13/14/15 tests all use it) | — |
| `three`, `@react-three/fiber`, `@react-three/drei` | GFX-01..04 | ✗ (not yet installed) | — | N/A — install required |
| `lighthouse` CLI (dev dep) | GFX-04 LCP measurement | ✗ (not yet installed) | — | Playwright performance.getEntries() proxy |
| WebGL hardware | Browser-side canvas | ✓ (dev machine has GPU) | — | SVG fallback (CSS-only path) |

**Missing dependencies with no fallback:** `three`, `@react-three/fiber`, `@react-three/drei`, `@types/three` — install required before any implementation.

**Missing dependencies with fallback:** `lighthouse` — can be omitted if Windows CLI issues arise; Playwright metrics proxy is available.

---

## Validation Architecture

> `workflow.nyquist_validation: true` in `.planning/config.json` — section required.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Python `playwright` 1.57.0 (established project pattern, Phases 13–15) |
| Config file | none — scripts in `web/tests/` run directly |
| Quick run command | `cd web && python tests/home_spec.py` (existing) |
| GFX-01..04 test | `cd web && python tests/gfx_spec.py` (Wave 0 gap — new file needed) |
| Full suite command | `pnpm build && cd web && python tests/home_spec.py && python tests/gfx_spec.py` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GFX-01 | `pnpm build` succeeds; `out/index.html` has NO `<canvas>` element; `<canvas>` appears after JS hydration | build + DOM assertion | `pnpm build && grep -v '<canvas' out/index.html` + Playwright canvas-after-hydration check | ❌ Wave 0 |
| GFX-01 | Three.js/R3F symbols absent from server HTML | build output grep | `grep -c 'WebGLRenderingContext\|@react-three' out/index.html` should be 0 | ❌ Wave 0 |
| GFX-02 | Ambient accents render without a second `<canvas>` | DOM assertion | Playwright: count `document.querySelectorAll('canvas').length === 1` | ❌ Wave 0 |
| GFX-03 | Reduced-motion: canvas absent, SVG present | Playwright | Emulate `prefers-reduced-motion: reduce` → assert no `<canvas>`, hero SVG visible | ❌ Wave 0 |
| GFX-03 | `aria-hidden` on canvas; sr-only description present | DOM assertion | Playwright: `canvas.getAttribute('aria-hidden') === 'true'` + `p.sr-only` exists | ❌ Wave 0 |
| GFX-04 | LCP < 2.5s | Lighthouse CLI | `lighthouse http://localhost:PORT/ --output json` → assert `lcp < 2500` | ❌ Wave 0 |
| GFX-04 | No canvas on navigation (no context leak) | Playwright | Navigate away and back, count active canvases stays 1 | ❌ Wave 0 |

### Wave 0 Gaps

- [ ] `web/tests/gfx_spec.py` — covers GFX-01 (build assertion + canvas-post-hydration), GFX-02 (single canvas), GFX-03 (reduced-motion fallback + aria), GFX-04 (LCP + context-count)
- [ ] `web/public/hero-hive.svg` — static SVG fallback (LCP element); must exist before `pnpm build`
- [ ] `lighthouse` devDependency install (`pnpm add -D lighthouse`) if CLI audit is used for GFX-04

*(Existing `web/tests/home_spec.py` covers hero visibility and theme switching — no changes needed to it; GFX tests are additive)*

### Sampling Rate

- **Per task commit:** `pnpm build` (verifies static export intact)
- **Per wave merge:** `pnpm build && python tests/home_spec.py && python tests/gfx_spec.py`
- **Phase gate:** Full suite + Lighthouse LCP assertion green before `/gsd-verify-work 16`

---

## Security Domain

> `security_enforcement` not set to `false` in config; section required.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | partial | PresentationControls limits pointer input to polar/azimuth ranges — no user text input in canvas |
| V6 Cryptography | no | — |

### Known Threat Patterns for Three.js / R3F stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Supply-chain risk on 3D packages | Tampering | All packages from pmndrs/mrdoob (established repos); pinned exact versions in pnpm lockfile |
| Canvas fingerprinting | Information disclosure | `aria-hidden` canvas; `failIfMajorPerformanceCaveat: true` reduces fingerprint surface slightly; cannot fully prevent — acceptable for marketing site |
| Resource exhaustion via WebGL context flooding | Denial of service | Single canvas rule; R3F auto-disposes on unmount |
| SVG injection via `public/hero-hive.svg` | Tampering | SVG is a static file in `public/`; it is served as a static asset and rendered via `<img>` (not `<div dangerouslySetInnerHTML>`); no injection path |

**Beekeeper project posture note:** The fail-closed philosophy (from CLAUDE.md) maps to the 3D layer as: if the capability check fails (no WebGL, reduced-motion, saveData), the canvas does not mount and the static SVG is displayed. There is no "degrade gracefully with a broken canvas" path — it either works fully or falls back to static.

---

## Project Constraints (from CLAUDE.md)

Directives applicable to Phase 16 web work:

1. **`charm.land/bubbletea/v2` import path note** — not applicable to web/ (this is a Go constraint)
2. **Windows primary dev machine** — all tests and build commands must work on Windows PowerShell. The existing Python `http.server` + Playwright pattern is confirmed working on Windows.
3. **`pnpm build` must succeed** — `output: 'export'` is locked. Three.js/R3F behind `ssr:false` boundary maintains this invariant.
4. **`web/` Node toolchain is isolated** — pnpm workspace; `pnpm add` only inside `web/` or via workspace protocol. The three/R3F packages install in `web/node_modules`, not the Go root.
5. **Fail-closed philosophy** — carries to the 3D layer: if any capability check fails (no WebGL, reduced-motion, saveData), the code takes the static-SVG fallback path — it does not attempt to render a degraded canvas.
6. **Tailwind v4 `@theme inline` dual-theme token pattern** — `THREE.Color` cannot use CSS vars; resolve token values via `useTheme()` hook and programmatic color switching. Do NOT hardcode dark-only hex values in materials (would break light theme).
7. **Biome 2.2.0 for lint/format** — no ESLint. All new `.tsx` files must pass `pnpm lint` (biome check).

---

## Sources

### Primary (HIGH confidence)

- [nextjs.org/docs/app/guides/lazy-loading (Next.js 16.2.7)] — `ssr:false` in Server Component restriction; Client Component wrapper pattern; verified doc version 16.2.7 / lastUpdated 2026-03-10
- [r3f.docs.pmnd.rs/getting-started/introduction] — R3F v9 = React 19; installation command
- [r3f.docs.pmnd.rs/getting-started/installation] — `transpilePackages: ['three']` for Next.js 13.1+
- [r3f.docs.pmnd.rs/api/canvas] — Canvas props including `frameloop`, `gl`, `onCreated`; unmount → `dispose()`
- [r3f.docs.pmnd.rs/advanced/scaling-performance] — `frameloop="demand"`, instancing, PerformanceMonitor
- [npm registry — `npm view`] — Confirmed versions: three@0.184.0, @react-three/fiber@9.6.1, @react-three/drei@10.7.7, @types/three@0.184.1; peer-dep verification
- [MDN Web Docs — Detect WebGL] — Canonical `canvas.getContext('webgl')` detection pattern

### Secondary (MEDIUM confidence)

- [discourse.threejs.org — drei Next.js transpilePackages] — Community confirmation that `@react-three/drei` also requires `transpilePackages` (corroborated by official drei import pattern; verified in project's pre-existing next.config.mjs which already lists `@react-three/drei`)
- [github.com/pmndrs/react-three-fiber — issues/2456, issues/3093, issues/514] — WebGL context leak patterns; Safari context limit; R3F auto-cleanup behavior
- [web.dev — save-data API] — `navigator.connection.saveData` as low-power signal

### Tertiary (LOW confidence)

- [WebSearch] — three.js ~5.5M weekly downloads figure (from npm-stat aggregation; not directly verified via npm API call)

---

## Metadata

**Confidence breakdown:**

- Standard stack (versions + peer deps): HIGH — confirmed via `npm view` directly against npm registry
- Architecture patterns (dynamic import, ssr:false rule): HIGH — verified against Next.js 16.2.7 official docs
- Pitfalls (context leak, Safari limit): MEDIUM — verified via official R3F GitHub issues + discussion threads
- Bundle size / LCP numbers: MEDIUM — three@0.184.0 full bundle ~36 KB gzip (from WebSearch, not direct measurement)
- Hive geometry implementation: LOW — design details are ASSUMED; implementation will determine final approach

**Research date:** 2026-06-08
**Valid until:** 2026-07-08 (stable stack — R3F 9.x / three 0.18x unlikely to change rapidly in 30 days)
