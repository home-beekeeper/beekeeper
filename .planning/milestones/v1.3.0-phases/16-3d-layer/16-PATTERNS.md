# Phase 16: 3D Layer — Pattern Map

**Mapped:** 2026-06-09
**Files analyzed:** 6 (2 new client components, 1 server component modify, 1 static asset, 1 config modify, 1 test)
**Analogs found:** 5 / 6 (hero-hive.svg has no existing analog — public/ contains only .gitkeep)

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `web/components/home/hero-canvas-wrapper.tsx` | client component (boundary) | request-response (dynamic import gate) | `web/components/home/install-chip.tsx` (implied "use client" home component) / pattern from RESEARCH.md | role-match |
| `web/components/home/hero-canvas.tsx` | client component (R3F canvas) | event-driven (pointer + useEffect capability gate) | `web/lib/reduced-motion.tsx` — "use client" + useEffect + useState | role-match |
| `web/components/home/hero.tsx` | server component (modify) | request-response (static render) | itself — read in full above | self |
| `web/public/hero-hive.svg` | static asset | — | none (public/ is empty) | no analog |
| `web/next.config.mjs` | config (modify) | — | itself — already contains the target array | self |
| `web/tests/gfx_spec.py` | test | request-response (Playwright + http.server) | `web/tests/home_spec.py` | exact |

---

## Pattern Assignments

### `web/components/home/hero-canvas-wrapper.tsx` (client component, dynamic import boundary)

**Analog:** `web/lib/reduced-motion.tsx` (lines 1–3) for `"use client"` placement convention; RESEARCH.md Pattern 1 for `dynamic(ssr:false)`.

**Rule:** `"use client"` must be the very first line, before all imports. This is how every existing client component in the project is written (`reduced-motion.tsx` line 1, `providers.tsx` line 1).

**Imports pattern** — mirror `reduced-motion.tsx` lines 1–2 for directive + import style:
```typescript
"use client";
import dynamic from "next/dynamic";
```

**Core dynamic-import pattern** (from RESEARCH.md Pattern 1, verified against Next.js 16.2.7 docs):
```typescript
// The R3F canvas never runs during `next build` SSG pass.
// loading: () => null — SVG fallback in parent Hero stays visible pre-JS.
const HeroCanvas = dynamic(() => import("./hero-canvas"), {
  ssr: false,
  loading: () => null,
});

export function HeroCanvasWrapper() {
  return <HeroCanvas />;
}
```

**Token rule:** This wrapper renders no visible DOM of its own — no style props needed. If a wrapper `<div>` is added for positioning, use `style={{ position: "absolute", inset: 0 }}` (raw CSS, not a Tailwind token) to avoid any theme dependency.

---

### `web/components/home/hero-canvas.tsx` (client component, R3F Canvas + capability gate)

**Analog:** `web/lib/reduced-motion.tsx` (full file, 48 lines) — the project's established `"use client"` + `useEffect` + `useState` pattern. The capability gate in `hero-canvas.tsx` is structurally identical: mount `useEffect`, probe a browser API, update boolean state, render nothing until ready.

**Imports pattern** — copy `reduced-motion.tsx` lines 1–2 for directive + react imports, then add R3F:
```typescript
"use client";
import { useEffect, useState } from "react";
import { useReducedMotion } from "@/lib/reduced-motion";
import { Canvas } from "@react-three/fiber";
import { PresentationControls, Environment, Float } from "@react-three/drei";
```

Note: import only named exports from `@react-three/drei` (tree-shaking requires named imports; `sideEffects: false` is only effective this way — RESEARCH.md Pitfall 7).

**`useReducedMotion` hook consumption** — copy call site pattern from `reduced-motion.tsx` line 46–48 (the exported hook) and mirror the consumer pattern:
```typescript
// web/lib/reduced-motion.tsx lines 46-48 (the hook definition being consumed):
export function useReducedMotion(): boolean {
  return useContext(ReducedMotionContext).prefersReducedMotion;
}
// Consumer pattern in hero-canvas.tsx:
const prefersReducedMotion = useReducedMotion();
```

**Capability gate pattern** — mirror `reduced-motion.tsx` lines 18–35 (useEffect + state update + event listener cleanup) but for three conditions instead of one:
```typescript
// Pattern: reduced-motion.tsx lines 18-35 — useEffect + setState + cleanup
const [canMount, setCanMount] = useState(false);

useEffect(() => {
  if (prefersReducedMotion) return;
  const conn = (navigator as { connection?: { saveData?: boolean } }).connection;
  if (conn?.saveData) return;
  // WebGL probe — inline, no dependency needed (MDN standard API)
  try {
    const probe = document.createElement("canvas");
    const hasWebGL = !!(
      probe.getContext("webgl2", { failIfMajorPerformanceCaveat: true }) ||
      probe.getContext("webgl", { failIfMajorPerformanceCaveat: true })
    );
    if (!hasWebGL) return;
  } catch {
    return;
  }
  setCanMount(true);
}, [prefersReducedMotion]);

if (!canMount) return null;
```

**Canvas + accessibility pattern** (from RESEARCH.md Code Examples — "R3F Canvas with Accessibility"):
```typescript
// Wrapper div is role="presentation"; Canvas is aria-hidden; sr-only is a
// sibling of <Canvas>, NOT a child of it (RESEARCH.md Pitfall 6).
return (
  <div style={{ position: "absolute", inset: 0 }} role="presentation">
    <Canvas
      aria-hidden="true"
      frameloop="demand"
      style={{ position: "absolute", inset: 0 }}
      gl={{ antialias: true, alpha: true }}
      camera={{ position: [0, 0, 5], fov: 45 }}
    >
      <PresentationControls
        global
        snap
        polar={[-Math.PI / 4, Math.PI / 4]}
        azimuth={[-Math.PI / 4, Math.PI / 4]}
      >
        <HiveMesh />
      </PresentationControls>
      <Environment preset="city" />
    </Canvas>
    {/* sr-only: sibling of canvas — screen readers read this, canvas is aria-hidden */}
    <p className="sr-only">
      Interactive 3D hive visualization representing Beekeeper mediating
      agent tool calls
    </p>
  </div>
);
```

**Theme-aware material colors** — `THREE.Color` cannot resolve CSS vars (globals.css Section 6 tokens are CSS custom properties, not accessible to WebGL shaders). Use `useTheme()` from `next-themes` (already installed in `web/node_modules`; wired in `web/app/providers.tsx` line 3) and switch programmatically:
```typescript
import { useTheme } from "next-themes";
// Inside HiveMesh component:
const { resolvedTheme } = useTheme();
const tealHex = resolvedTheme === "light" ? "#0a6b75" : "#39c5cf";
// globals.css Section 6a: --teal: #39c5cf (dark default)
// globals.css Section 6b: --teal: #0a6b75 (light override)
// Use tealHex in meshStandardMaterial color prop or via ref.material.color.set()
```

Do NOT hardcode only the dark value `#39c5cf` — this would break light theme, which is the exact mistake caught in Phase 15 code review (memory: "dual-theme trap... HARDCODED rgba on the header, NOT --color-bk-*, that a body-bg-only Playwright proof missed").

---

### `web/components/home/hero.tsx` (server component, MODIFY)

**Analog:** itself — the current file is the target. Read in full above (104 lines).

**Current structure to preserve** (hero.tsx lines 17–103):
- Outer `<div>` with `style={{ maxWidth: "var(--max-content-width)" }}` — keep, wrap canvas slot inside a sibling container
- All existing children (version pill, h1, subhead, InstallChip, CTA row) — preserve without modification
- Token pattern: all inline styles use raw `var(--*)` tokens (e.g. `var(--surface-2)`, `var(--border)`, `var(--dim)`, `var(--fg-strong)`) — never `--color-bk-*`

**Addition pattern** — insert SVG + canvas wrapper slot as a new child block. Mirror the existing container pattern from hero.tsx lines 19–24 (outer div with style maxWidth):
```typescript
// ADD after the closing </div> of the CTA row (after line 101),
// inside the outer wrapper div, before the final closing </div>:

{/* Hero visual — SVG is server-rendered LCP candidate; canvas mounts after hydration */}
<div className="relative mx-auto mt-10" style={{ width: 400, height: 400 }}>
  {/* Server-rendered SVG: always present, never lazy — LCP element */}
  {/* fetchPriority="high" signals to browser: this is the LCP element (RESEARCH.md Pitfall 4) */}
  {/* Plain <img> not next/image — avoids static-export wrapper markup (RESEARCH.md Pattern 2) */}
  <img
    src="/hero-hive.svg"
    alt=""
    aria-hidden="true"
    width={400}
    height={400}
    className="absolute inset-0"
    // biome-ignore lint/a11y/noRedundantAlt: intentionally empty alt + aria-hidden; canvas has sr-only
    fetchPriority="high"
  />
  {/* Client boundary — dynamic(ssr:false) lives inside HeroCanvasWrapper ("use client") */}
  <HeroCanvasWrapper />
</div>
```

Import to add at top of hero.tsx (after existing `import Link`):
```typescript
import { HeroCanvasWrapper } from "./hero-canvas-wrapper";
```

hero.tsx remains a Server Component — no `"use client"` directive added. The import of `HeroCanvasWrapper` is safe because Next.js allows Server Components to import Client Components.

---

### `web/public/hero-hive.svg` (static asset)

**Analog:** None — `web/public/` contains only `.gitkeep`. No existing SVG convention in the project.

**Constraints from RESEARCH.md + project context:**
- Must be a valid SVG served as a static file (referenced via `<img src="/hero-hive.svg">` — not `dangerouslySetInnerHTML`)
- Must have explicit `viewBox` and `width`/`height` attributes matching the 400×400 container in hero.tsx (prevents layout recalculation that would break LCP budget — RESEARCH.md Pitfall 4)
- Visual: hexagonal hive cluster representing Beekeeper mediating agent tool calls
- Color: use teal and amber values from the dark theme as the SVG's default colors (dark is the brand canonical; the SVG is the fallback for reduced-motion/no-WebGL, both of which can appear in dark or light theme — use a neutral brand color or a color that works acceptably in both)
  - Recommended: `#39c5cf` (teal dark, `--teal` from globals.css Section 6a) for hex cell fills; `#e3b341` (amber dark, `--amber`) for accent nodes
  - The SVG is behind `aria-hidden="true"` on the `<img>` — no accessibility description needed in the SVG itself
- Minimum viable SVG: `CylinderGeometry(r, r, h, 6)` is the 3D equivalent of a hexagon; in SVG that is a `<polygon>` with 6 points. A cluster of 7 hexagons (1 center + 6 surrounding) is the honeycomb minimum.

**No pattern excerpt to copy — this is net-new.** Planner should use RESEARCH.md Code Examples — "Hive Geometry" as the visual reference and generate a static SVG that represents the same hexagonal cluster.

---

### `web/next.config.mjs` (config, MODIFY)

**Analog:** itself — the current file is the target. Read in full above (31 lines).

**Current state** (next.config.mjs line 27 — already correct per RESEARCH.md Pitfall 5):
```javascript
transpilePackages: ["three", "@react-three/fiber", "@react-three/drei"],
```

**Action required:** No change needed. The `transpilePackages` array was pre-declared in Phase 15 as a zero-cost pre-declaration (comment on line 23–27 confirms this). Verify after `pnpm add` that the installed packages match these exact names. The config is already correct.

---

### `web/tests/gfx_spec.py` (test, NEW)

**Analog:** `web/tests/home_spec.py` — exact match. Mirror the entire server-launch + assertion + exit-code structure.

**Server-launch boilerplate** (home_spec.py lines 24–64) — copy verbatim, change only PORT:
```python
import http.server
import os
import sys
import threading
import time
from playwright.sync_api import sync_playwright

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
WEB_DIR = os.path.dirname(SCRIPT_DIR)
OUT_DIR = os.path.join(WEB_DIR, "out")

if not os.path.isdir(OUT_DIR):
    print(f"FAIL: out/ directory not found at {OUT_DIR} — run `pnpm build` first")
    sys.exit(1)

PORT = 4200  # home_spec.py uses 4199; use 4200 to avoid port conflict

class SilentHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=OUT_DIR, **kwargs)
    def log_message(self, format, *args):
        pass

def start_server():
    server = http.server.HTTPServer(("127.0.0.1", PORT), SilentHandler)
    server.daemon_threads = True
    t = threading.Thread(target=server.serve_forever, daemon=True)
    t.start()
    return server
```

**Test runner scaffold** (home_spec.py lines 94–250) — copy the `failures`, `fail()`, `ok()`, `run_tests()`, `__main__` pattern exactly:
```python
failures = []

def fail(msg: str) -> None:
    failures.append(msg)
    print(f"  FAIL: {msg}")

def ok(msg: str) -> None:
    print(f"  PASS: {msg}")

def run_tests():
    server = start_server()
    time.sleep(0.3)  # let server bind
    base_url = f"http://127.0.0.1:{PORT}/"

    with sync_playwright() as pw:
        browser = pw.chromium.launch(headless=True)
        # ... test blocks here (see GFX assertions below) ...
        browser.close()
    server.shutdown()

if __name__ == "__main__":
    print("=== Phase 16 GFX — 3D Layer Playwright Proof ===")
    run_tests()
    print()
    if failures:
        print(f"RESULT: FAILED — {len(failures)} assertion(s) failed:")
        for f in failures:
            print(f"  - {f}")
        sys.exit(1)
    else:
        print("RESULT: ALL ASSERTIONS PASSED")
        sys.exit(0)
```

**Reduced-motion emulation pattern** (new, not in home_spec.py) — use Playwright's `browser.new_context` with `reduced_motion`:
```python
# GFX-03: reduced-motion → canvas absent, SVG present
context = browser.new_context(
    viewport={"width": 1280, "height": 800},
    reduced_motion="reduce",
)
page = context.new_page()
page.goto(base_url, wait_until="networkidle")
# Assert no <canvas> element
canvas_count = page.evaluate("document.querySelectorAll('canvas').length")
if canvas_count != 0:
    fail(f"Canvas present under reduced-motion (count={canvas_count}), expected 0")
else:
    ok("No canvas under prefers-reduced-motion: reduce")
# Assert SVG img present (LCP fallback)
svg_img = page.locator("img[src='/hero-hive.svg']").first
if svg_img.is_visible():
    ok("hero-hive.svg fallback visible under reduced-motion")
else:
    fail("hero-hive.svg fallback NOT visible under reduced-motion")
page.close()
context.close()
```

**Theme-switching eval pattern** — copy home_spec.py lines 168–199 exactly for any theme-assertion tests needed in GFX.

**GFX assertions to implement** (from RESEARCH.md Validation Architecture test map):

| GFX req | Playwright pattern |
|---|---|
| GFX-01: no `<canvas>` in server HTML | Check `out/index.html` file content via `open(OUT_DIR + "/index.html").read()` — assert `"<canvas"` not in string |
| GFX-01: canvas appears after hydration | `page.goto(base_url, wait_until="networkidle")` → `page.evaluate("document.querySelectorAll('canvas').length")` → assert `>= 0` (1 if WebGL available in headless Chromium; accept 0 if headless lacks WebGL) |
| GFX-02: single canvas at most | Same page after networkidle: `canvas_count <= 1` |
| GFX-03: reduced-motion → no canvas | `browser.new_context(reduced_motion="reduce")` pattern above |
| GFX-03: aria-hidden on canvas + sr-only present | `page.evaluate("document.querySelector('canvas')?.getAttribute('aria-hidden')")` == `"true"`; `page.locator(".sr-only").count() > 0` |
| GFX-04: LCP < 2500ms | Use Playwright `page.evaluate("performance.getEntriesByType('largest-contentful-paint').slice(-1)[0]?.startTime")` as Windows-safe LCP proxy (avoids Lighthouse CLI Windows issues per RESEARCH.md Open Question 4) |

---

## Shared Patterns

### Token Rule: Raw `var(--*)` in inline styles, never `--color-bk-*`

**Source:** `web/components/home/hero.tsx` lines 14–15 (comment), `web/components/home/section.tsx` lines 29–31 (comment), `web/app/globals.css` lines 43–50 (explanatory comment).
**Apply to:** All new `.tsx` files with any inline `style={}` props.

The `--color-bk-*` tokens are dark-only brand constants defined in `@theme inline` (globals.css lines 22–41) and are NEVER overridden in the `.light` block. Using them in inline styles freezes the component to dark values in both themes.

Correct:
```typescript
style={{ color: "var(--teal)" }}       // switches: #39c5cf dark, #0a6b75 light
style={{ background: "var(--surface-2)" }} // switches per theme
```
Wrong:
```typescript
style={{ color: "var(--color-bk-teal)" }}  // NEVER — dark-only, breaks light theme
```

### `"use client"` Placement

**Source:** `web/lib/reduced-motion.tsx` line 1, `web/app/providers.tsx` line 1.
**Apply to:** `hero-canvas-wrapper.tsx`, `hero-canvas.tsx`.

`"use client"` must be the absolute first line of the file — before any blank lines, before any imports. Any content before it causes Next.js to treat the file as a Server Component.

### `useReducedMotion()` Hook Consumption

**Source:** `web/lib/reduced-motion.tsx` lines 46–48 (hook definition) + `web/app/providers.tsx` lines 4, 15 (provider wiring showing it is always available).
**Apply to:** `hero-canvas.tsx`.

The provider is already wired at the app level (`ReducedMotionProvider` wraps all children in `providers.tsx` line 15). The hook is always safe to call — no conditional hook usage needed. Import path: `@/lib/reduced-motion`.

### `useTheme()` for Programmatic Theme Switching

**Source:** `web/app/providers.tsx` lines 3, 6–14 (`ThemeProvider` with `attribute="class"`, `storageKey="bk-theme"`).
**Apply to:** `hero-canvas.tsx` → `HiveMesh` sub-component for material colors.

`resolvedTheme` from `useTheme()` is `"light"` or `"dark"`. Map directly to the hex values from globals.css Section 6:
- `--teal` dark → `#39c5cf`; light → `#0a6b75`
- `--amber` dark → `#e3b341`; light → `#8a6500`

### Positioning Container Convention

**Source:** `web/components/home/hero.tsx` lines 19–21 (outer wrapper with `style={{ maxWidth: "var(--max-content-width)" }}`).
**Apply to:** The hero visual container added in `hero.tsx`.

Use `className="relative mx-auto"` with explicit `style={{ width: N, height: N }}` when content must overlap (SVG under canvas). The `relative` class establishes the positioning context for `absolute inset-0` children.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `web/public/hero-hive.svg` | static asset | — | `web/public/` contains only `.gitkeep`; no existing SVG convention. Design from scratch using hexagonal polygon geometry (`<polygon points="...">`) with teal/amber fill from globals.css Section 6 dark defaults. |

---

## Metadata

**Analog search scope:** `web/components/home/`, `web/lib/`, `web/app/`, `web/tests/`, `web/public/`, `web/next.config.mjs`
**Files scanned:** 9
**Pattern extraction date:** 2026-06-09
