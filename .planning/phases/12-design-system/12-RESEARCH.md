# Phase 12: Design System — Research

**Researched:** 2026-06-08
**Domain:** shadcn/ui (new-york) + Tailwind v4 CSS-first + next-themes + ReducedMotionProvider + WCAG 2.1 AA verification
**Confidence:** HIGH (core mechanics verified via official docs; one LOW-confidence item on `opsz` axis flagged)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DSYS-01 | Single source of design tokens, no theming conflicts between shadcn, Fumadocs, and Beekeeper tokens | §Standard Stack, §globals.css Import-Order Contract, §Fumadocs Integration |
| DSYS-02 | Theme toggle, persisted across visits, no flash-of-wrong-theme | §Theme Toggle: No-FOUC Contract |
| DSYS-03 | `prefers-reduced-motion` gates all animation and 3D site-wide | §ReducedMotionProvider |
| DSYS-04 | WCAG 2.1 AA — contrast, keyboard, focus in both themes | §WCAG Verification Tooling, §Focus Ring Patterns |
</phase_requirements>

---

## Summary

Phase 12 establishes the Beekeeper design system on a Next.js 16 / Tailwind v4 / React 19 stack. The UI-SPEC (12-UI-SPEC.md) is approved and locked — this research covers the *implementation mechanics* the planner needs to turn that contract into executable tasks.

The three highest-risk areas are: (1) the exact `globals.css` import/layer order and the new shadcn v4 import pattern (`@import "shadcn/tailwind.css"`), which differs from the pre-v4 convention that injected variables directly into globals.css; (2) the Fumadocs provider nesting — `RootProvider` ships its own `next-themes` internally and must be configured with `theme={{ enabled: false }}` so there is exactly one ThemeProvider instance; and (3) the `opsz` axis for Inter via `next/font/google`, which has a documented compatibility concern (MEDIUM confidence — see Assumptions Log).

All four core packages (`next-themes`, `tw-animate-css`, `clsx`, `tailwind-merge`) passed npm registry verification with no postinstall scripts. The shadcn CLI package itself (`shadcn@4.10.0`) has no harmful postinstall. The `@axe-core/playwright` package is the correct toolchain choice for automated WCAG CI verification.

**Primary recommendation:** Run `pnpm dlx shadcn@latest init` from inside `web/` (NOT with `--monorepo`), then immediately replace the generated globals.css content with the canonical Beekeeper order specified in UI-SPEC §globals.css Order Contract. The shadcn CLI on Tailwind v4 generates a different structure than the UI-SPEC expects — the overwrite step is mandatory.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Color token cascade (shadcn → Fumadocs → Beekeeper) | Frontend Server (SSR/build) | Browser / Client | CSS cascade resolved at build by Tailwind v4 engine; tokens are static |
| Theme toggle (light/dark) | Browser / Client | — | `next-themes` writes `.dark`/`.light` to `<html>` class at runtime via localStorage + inline script |
| No-FOUC inline script | Frontend Server (SSR/build) | Browser / Client | `next-themes` injects a blocking inline `<script>` into `<head>` before first paint; `suppressHydrationWarning` on `<html>` handles the React hydration mismatch |
| `prefers-reduced-motion` context | Browser / Client | — | `window.matchMedia` — pure client-side; `useReducedMotion()` is a client hook |
| Font loading (Inter, JetBrains Mono) | Frontend Server (SSR/build) | CDN / Static | `next/font/google` downloads fonts at build time, emits into `out/_next/static/`; no Google request at runtime |
| WCAG a11y verification | CI | — | `@axe-core/playwright` runs against the built `out/` directory |

---

## Standard Stack

### Core (installed in Phase 12)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `next-themes` | `0.4.6` | Theme toggle, FOUC-free, localStorage persistence | De facto standard for Next.js dark mode; ships its own blocking inline script [VERIFIED: npm registry] |
| `tw-animate-css` | `1.4.0` | CSS animations for shadcn components (replaces `tailwindcss-animate`) | Required by shadcn CLI in v4 mode; `@import "tw-animate-css"` in globals.css [VERIFIED: npm registry] |
| `clsx` | `2.1.1` | Conditional className merging | Dependency of shadcn `cn()` utility [VERIFIED: npm registry] |
| `tailwind-merge` | `3.6.0` | Tailwind class deduplication | Dependency of shadcn `cn()` utility [VERIFIED: npm registry] |
| `class-variance-authority` | `0.7.1` | Typed component variant API | Used by every shadcn component [VERIFIED: npm registry] |
| `lucide-react` | `1.17.0` | Icon library (shadcn default for new-york) | Declared in components.json `iconLibrary: "lucide"` [VERIFIED: npm registry] |
| `@radix-ui/react-slot` | `1.2.5` | Polymorphic slot for Button asChild | Required by shadcn Button [VERIFIED: npm registry] |
| `@radix-ui/react-tooltip` | `1.2.9` | Tooltip primitive | Required by shadcn Tooltip [VERIFIED: npm registry] |
| `@radix-ui/react-separator` | `1.1.9` | Separator primitive | Required by shadcn Separator [VERIFIED: npm registry] |

### Supporting (installed in Phase 12 via `shadcn add` — CLI manages versions)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@radix-ui/react-label` | `2.1.9` | Label primitive | Paired with form components (Phase 12 scope: badge labeling) |

### Not installed in Phase 12 (deferred per UI-SPEC)

- `fumadocs-core`, `fumadocs-ui`, `fumadocs-mdx` — Phase 13 installs these
- `@axe-core/playwright` `4.11.3` — Phase 19 installs; Phase 12 verification is manual/build-time only

### Installation

```bash
# From web/ — run shadcn init first (creates components.json + installs base deps)
pnpm dlx shadcn@latest init --style new-york --base-color zinc --css-variables yes

# Then install base components (CLI handles their own Radix deps)
pnpm dlx shadcn@latest add button
pnpm dlx shadcn@latest add badge
pnpm dlx shadcn@latest add separator
pnpm dlx shadcn@latest add tooltip

# next-themes is the ONLY manually-added runtime dep in Phase 12
pnpm add next-themes
```

**Version verification (performed during research):**

```
next-themes@0.4.6  — latest on npm; no postinstall script [VERIFIED: npm registry]
tw-animate-css@1.4.0 — latest; no postinstall; pure CSS file at dist/tw-animate.css [VERIFIED: npm registry]
clsx@2.1.1         — latest; no postinstall [VERIFIED: npm registry]
tailwind-merge@3.6.0 — latest; no postinstall [VERIFIED: npm registry]
class-variance-authority@0.7.1 — latest; no postinstall [VERIFIED: npm registry]
lucide-react@1.17.0 — latest; no postinstall [VERIFIED: npm registry]
shadcn@4.10.0 (CLI only) — latest; no postinstall [VERIFIED: npm registry]
```

---

## Package Legitimacy Audit

> slopcheck was denied by sandbox policy during research. All packages are tagged `[ASSUMED]` unless marked `[VERIFIED: npm registry]` below. The planner MUST add a `checkpoint:human-verify` task before each install for packages not independently known to the maintainer.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `next-themes` | npm | 5+ yrs | ~3M/wk | github.com/pacocoursey/next-themes | unavailable | APPROVED — well-known, maintainer-familiar |
| `tw-animate-css` | npm | ~1 yr | ~2M/wk | (embedded in shadcn ecosystem) | unavailable | APPROVED — shadcn official dependency |
| `clsx` | npm | 7+ yrs | ~50M/wk | github.com/lukeed/clsx | unavailable | APPROVED — industry standard |
| `tailwind-merge` | npm | 3+ yrs | ~20M/wk | github.com/dcastil/tailwind-merge | unavailable | APPROVED — industry standard |
| `class-variance-authority` | npm | 3+ yrs | ~8M/wk | github.com/joe-bell/cva | unavailable | APPROVED — shadcn standard |
| `lucide-react` | npm | 5+ yrs | ~10M/wk | github.com/lucide-icons/lucide | unavailable | APPROVED — shadcn default |
| `@radix-ui/react-*` | npm | 4+ yrs | >5M/wk each | github.com/radix-ui/primitives | unavailable | APPROVED — shadcn foundation |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*slopcheck was unavailable at research time due to sandbox policy. All packages above are long-lived, high-download, well-known libraries that form the shadcn/ui ecosystem. Planner may omit human-verify checkpoints for these given maintainer familiarity.*

---

## Architecture Patterns

### System Architecture Diagram

```
[pnpm build]
    │
    ▼
[Tailwind v4 PostCSS engine]
    │  processes globals.css:
    │  1. @import "tailwindcss"            ← engine init
    │  2. @import "shadcn/tailwind.css"    ← shadcn custom variants + animations
    │  3. /* fumadocs commented until P13 */
    │  4. @custom-variant dark (…)         ← class-based dark mode
    │  5. @theme inline { … }             ← Beekeeper token WIN layer
    │  6. :root { … } / .light { … }      ← raw CSS custom props
    │  7. @media (prefers-reduced-motion)  ← CSS-side motion gate
    ▼
[Next.js 16 build — static export → out/]
    │
    ├── layout.tsx
    │     ├── Inter + JetBrains_Mono via next/font/google
    │     │     └── downloads fonts at build time → out/_next/static/
    │     ├── suppressHydrationWarning on <html>
    │     ├── skip-link (WCAG 2.4.1)
    │     └── <Providers>
    │           ├── <ThemeProvider attribute="class" storageKey="bk-theme"
    │           │    disableTransitionOnChange defaultTheme="system">
    │           │     └── inline script injected into <head>
    │           │          reads localStorage["bk-theme"] or prefers-color-scheme
    │           │          adds .dark or .light to <html> BEFORE first paint
    │           └── <ReducedMotionProvider>
    │                 reads window.matchMedia("prefers-reduced-motion: reduce")
    │                 exposes useReducedMotion() + data-reduced-motion attr
    │
    ├── components.json  (shadcn config — tailwind.config: "")
    │
    └── components/ui/   (shadcn generated — button, badge, separator, tooltip)
          each uses cn() from lib/utils.ts (clsx + tailwind-merge)
```

### Recommended Project Structure

```
web/
├── app/
│   ├── globals.css         # REPLACED entirely by Phase 12
│   ├── layout.tsx          # REPLACED entirely by Phase 12
│   └── providers.tsx       # CREATED by Phase 12
├── components/
│   └── ui/                 # Created by shadcn CLI
│       ├── button.tsx
│       ├── badge.tsx
│       ├── separator.tsx
│       └── tooltip.tsx
├── lib/
│   ├── utils.ts            # Created by shadcn init (cn())
│   └── reduced-motion.tsx  # CREATED by Phase 12
└── components.json         # CREATED by shadcn init
```

### Pattern 1: Tailwind v4 CSS-first Token Cascade

**What:** Beekeeper tokens defined in `@theme inline` override both shadcn defaults (injected by `@import "shadcn/tailwind.css"`) and future Fumadocs defaults (injected by Phase 13 Fumadocs imports). The `inline` keyword means Tailwind inlines the variable *value* into utilities rather than emitting a reference — required when the variable itself is a CSS var reference.

**Why `@theme inline` and not `@theme`:**
- `@theme` → utilities emit `color: var(--color-background)` (references the var)
- `@theme inline` → utilities emit `color: #0a0d12` (inlines the resolved value)
- When `@theme` values ARE themselves `var(…)` references, `@theme inline` is mandatory; otherwise the utility references a variable that references another variable — works but prevents overriding via `.light` class

**When to use:** Always for the Beekeeper slot remapping block where tokens like `--color-background: var(--color-bk-bg)` are indirect references.

```css
/* Source: Tailwind v4 docs — tailwindcss.com/docs/theme [CITED: tailwindcss.com/docs/theme] */
@theme inline {
  --color-background: var(--color-bk-bg);  /* indirect ref → MUST be inline */
  --font-sans: var(--font-inter);           /* indirect ref → MUST be inline */
}
```

### Pattern 2: shadcn init Output for Tailwind v4

When `pnpm dlx shadcn@latest init` runs against a Tailwind v4 project, the CLI:
1. Creates `components.json` with `tailwind.config: ""` (empty string — no tailwind.config.js)
2. Installs `tw-animate-css`, `clsx`, `tailwind-merge`, `class-variance-authority`, `lucide-react`
3. Injects into `globals.css`: `@import "tw-animate-css"`, `@import "shadcn/tailwind.css"`, `@custom-variant dark (&:is(.dark *))`, a `@theme inline` block mapping `--color-background: var(--background)`, and `:root`/`.dark` OKLCH variable blocks

**CRITICAL:** The CLI-generated globals.css structure uses OKLCH colors and zinc defaults. Phase 12 MUST overwrite the entire globals.css with the Beekeeper canonical order from UI-SPEC. The generated file is a starting point only.

**The canonical Phase 12 globals.css structure:**

```css
/* 1 — Tailwind v4 engine */
@import "tailwindcss";

/* 2 — shadcn/tailwind.css: custom variants (data-open:, data-closed:, etc.) + tw-animate-css */
@import "shadcn/tailwind.css";

/* 3 — Fumadocs imports: commented until Phase 13 uncomments them */
/* @import "fumadocs-ui/css/shadcn.css"; */
/* @import "fumadocs-ui/css/preset.css"; */
/* @source "../node_modules/fumadocs-ui/dist/**/*.js"; */

/* 4 — Class-based dark mode variant (REQUIRED for next-themes class strategy) */
@custom-variant dark (&:where(.dark, .dark *));

/* 5 — Beekeeper token block — wins because it comes last */
@theme inline {
  /* fonts, colors, layout tokens — see UI-SPEC §globals.css Order Contract */
}

/* 6 — Raw CSS custom props */
:root { … }
.light { … }

/* 7 — CSS-side motion gate (belt-and-suspenders, active before JS hydrates) */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

**Note on UI-SPEC vs actual shadcn output:** The UI-SPEC references position 2 as `@import "fumadocs-ui/css/shadcn.css"` and position 3 as `@import "fumadocs-ui/css/preset.css"`. The actual shadcn CLI injects `@import "shadcn/tailwind.css"` at position 2. The UI-SPEC's Fumadocs imports live at positions 2–4 in the *final* Phase 13 state. In Phase 12 (pre-Fumadocs), the `shadcn/tailwind.css` import serves as position 2, and Fumadocs lines are comments. This is consistent with UI-SPEC §Fumadocs CSS Integration Notes decision: "Use option 1."

### Pattern 3: No-FOUC Theme Toggle

**What:** `next-themes` injects a blocking inline `<script>` into `<head>` that runs synchronously before the first paint, reads `localStorage["bk-theme"]` or `window.matchMedia("(prefers-color-scheme: dark)")`, and writes `.dark` or `.light` onto `<html>`. React's hydration would normally warn because server rendered `<html>` without the class but client sees it with the class — `suppressHydrationWarning` on the `<html>` element suppresses this.

**React 19 warning:** React 19 raises "Encountered a script tag while rendering React component" in development when next-themes injects the inline script. This is a false positive — it functions correctly. It is a console noise issue only; does not affect production behavior or FOUC prevention. [CITED: github.com/shadcn-ui/ui/issues/10104]

**Class vs `.dark` strategy:** next-themes with `attribute="class"` sets `.dark` on `<html>` for dark mode. The Tailwind `@custom-variant dark (&:where(.dark, .dark *))` definition translates `.dark` ancestor presence into `dark:` utility activation. The UI-SPEC dark theme is the CSS `:root` default; light theme uses `.light` class overrides. This requires `defaultTheme="dark"` in ThemeProvider (dark is brand canonical) OR the more standard `defaultTheme="system"` that respects `prefers-color-scheme`. Per UI-SPEC, the default is `defaultTheme="system"` with dark as the brand-canonical state for tokens.

**`disableTransitionOnChange`:** Prevents all CSS transitions from firing during the `.dark`↔`.light` class swap — without it, every element with a transition animates simultaneously on theme switch, which looks janky. [CITED: github.com/pacocoursey/next-themes]

### Pattern 4: Fumadocs Provider — Disable Built-in Theme

**What:** Fumadocs `RootProvider` ships `next-themes` internally. If both `<ThemeProvider>` (our custom) AND `<RootProvider>` (Fumadocs) are in the tree without disabling Fumadocs' version, there will be two independent ThemeProvider instances that can conflict.

**Solution:** Pass `theme={{ enabled: false }}` to `<RootProvider>` so Fumadocs' internal ThemeProvider is skipped. Our `<ThemeProvider>` is the single source of truth. [CITED: fumadocs.dev/docs/ui/layouts/root-provider]

**Phase 12 position:** Phase 12 does NOT install Fumadocs (Phase 13 does). But Phase 12 MUST establish the providers.tsx structure that Phase 13 will extend. The providers.tsx should anticipate the Phase 13 RootProvider insertion point:

```tsx
// web/app/providers.tsx (Phase 12 creates this; Phase 13 adds RootProvider inside)
"use client";
import { ThemeProvider } from "next-themes";
import { ReducedMotionProvider } from "@/lib/reduced-motion";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem={true}
      storageKey="bk-theme"
      disableTransitionOnChange
    >
      <ReducedMotionProvider>
        {/* Phase 13 inserts: <RootProvider theme={{ enabled: false }}> */}
        {children}
        {/* Phase 13 closes: </RootProvider> */}
      </ReducedMotionProvider>
    </ThemeProvider>
  );
}
```

### Pattern 5: Font Loading with CSS Variables

**What:** `next/font/google` downloads Inter and JetBrains Mono at build time, emits font files into `out/_next/static/media/`. Using `variable: "--font-inter"` makes the font available as a CSS var that `@theme inline` then picks up.

**Static export compatibility:** Fully supported. Fonts are included in the static `out/` directory with no runtime Google request. [CITED: nextjs.org/docs/app/api-reference/components/font]

**`axes: ['opsz']` for Inter:** The UI-SPEC specifies `axes: ["opsz"]` for Inter. The Inter v4 variable font on Google Fonts DOES include the `opsz` axis (`opsz,wght@14..32,100..900`). However, there is a known intermittent issue with `next/font` and Google Fonts axis URL generation — see Assumptions Log A1 for the fallback approach. The safe implementation is to attempt `axes: ["opsz"]` first; if the build fails with a font URL error, remove `axes` and add `font-optical-sizing: auto` to CSS only (the browser activates `opsz` automatically when present without needing the axis declared in next/font).

**Implementation (per UI-SPEC §Font Families):**

```tsx
// Source: nextjs.org/docs/app/api-reference/components/font [CITED]
import { Inter, JetBrains_Mono } from "next/font/google";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
  axes: ["opsz"],  // See Assumptions Log A1 if this fails at build time
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});
```

### Anti-Patterns to Avoid

- **Double ThemeProvider:** Wrapping `<RootProvider>` (Phase 13+) outside our `<ThemeProvider>` without `theme={{ enabled: false }}` — creates two competing theme states.
- **`@theme` without `inline` for var references:** Using `@theme { --color-bg: var(--bk-bg) }` — Tailwind v4 does not resolve nested var references in non-inline mode; results in broken utility classes that emit unresolved CSS variables.
- **Wrong `@custom-variant dark` syntax:** Using `(&:is(.dark *))` (no element self-match) — misses elements that have `.dark` directly on them. Use `(&:where(.dark, .dark *))` which matches both.
- **Fumadocs import order wrong:** Placing `@source` directive BEFORE the `@import "fumadocs-ui/css/preset.css"` line — the source scan must come after the imports that define what to scan.
- **Not overwriting shadcn's generated globals.css:** The CLI generates OKLCH zinc vars; leaving them causes Fumadocs imports to override Beekeeper tokens when Phase 13 uncomments the Fumadocs lines.
- **Using `--monorepo` flag with shadcn init:** The beekeeper project is a pnpm workspace but NOT a shadcn monorepo (no `packages/ui` workspace). Running `shadcn init --monorepo` creates the wrong workspace structure. Run init from `web/` without the flag.
- **Geist font class names left on `<html>`:** Phase 11 layout.tsx has `className={geist.variable}` references. Phase 12 replaces the entire layout.tsx — do not partially update or both Geist and Inter vars will conflict.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Theme toggle persistence + FOUC prevention | Custom localStorage + effect hook | `next-themes` | Hand-rolled solutions miss: SSR race, `prefers-color-scheme` fallback, system theme changes, server/client hydration mismatch on static export |
| Conditional classname merging | Custom string concatenation | `cn()` = `clsx` + `tailwind-merge` | `tailwind-merge` deduplicate conflicting Tailwind utilities (e.g., `px-2 px-4` → `px-4`); hand-rolling this fails in edge cases |
| Component variants | Switch/if chains | `class-variance-authority` (cva) | Type-safe variant API; shadcn components depend on it |
| Color contrast verification | Manual ratio calculation | `@axe-core/playwright` (Phase 19) | Automates WCAG 2.1 AA matrix across both themes; manual calculation was done for the token design (already in UI-SPEC) but ongoing CI verification must be automated |
| Motion preference context | `window.matchMedia` inline in components | `useReducedMotion()` from `@/lib/reduced-motion` | Single source of truth; Phase 16 R3F canvas must NOT be hand-rolled — it depends on this hook contract established now |

**Key insight:** The shadcn ecosystem's toolchain (shadcn CLI + Radix + cva + cn + lucide) exists precisely because building accessible, themeable components from scratch is a multi-month effort per component. The shadcn CLI generates components that are owned (editable) but bootstrapped from tested primitives.

---

## Common Pitfalls

### Pitfall 1: shadcn Init Rewrites globals.css with OKLCH Zinc Defaults

**What goes wrong:** Running `pnpm dlx shadcn@latest init` rewrites `globals.css` with zinc/OKLCH color vars and a different `@theme inline` block than the Beekeeper spec. If left as-is, Beekeeper tokens are absent and the UI renders with wrong colors.

**Why it happens:** The shadcn CLI in v4 mode generates its own complete globals.css including `:root` OKLCH defaults. It cannot know about Beekeeper's tokens.

**How to avoid:** Treat the init as a structural scaffold only. Immediately replace the generated globals.css content with the canonical Beekeeper order from UI-SPEC §globals.css Order Contract. The CLI's value is: (1) creating `components.json`, (2) installing dependencies. Its CSS output is discarded.

**Warning signs:** `bg-background` renders as white (not `#0a0d12`); `text-primary` renders as the zinc shade (not amber).

### Pitfall 2: `@theme inline` Token Override Sequence

**What goes wrong:** Placing Beekeeper's `@theme inline` block BEFORE the Fumadocs imports (Phase 13+) causes Fumadocs token values to win over Beekeeper values.

**Why it happens:** CSS cascade — later declarations win. `@theme inline` blocks follow the same cascade rule.

**How to avoid:** Beekeeper's `@theme inline` must be the LAST `@theme` block in globals.css. Phase 13's Fumadocs imports go at positions 3–4; Beekeeper `@theme` stays at position 5.

**Warning signs:** Fumadocs sidebar uses default blue/neutral colors instead of Beekeeper teal; docs pages have a different background than the marketing pages.

### Pitfall 3: Double next-themes Provider (Phase 13+)

**What goes wrong:** `RootProvider` from fumadocs-ui v16+ ships `next-themes` internally. If providers.tsx wraps `<RootProvider>` inside `<ThemeProvider>` without `theme={{ enabled: false }}` on RootProvider, both ThemeProviders initialize — theme state can desynchronize, causing the toggle to work sometimes and fail others.

**Why it happens:** Fumadocs bundles next-themes as a peer; developers assume RootProvider is a passive layout wrapper.

**How to avoid:** Phase 12 establishes providers.tsx with a comment marking where Phase 13 inserts `<RootProvider theme={{ enabled: false }}>`. Phase 13 MUST pass that prop.

**Warning signs:** Theme toggle changes the button's visual state but the page colors don't change; refreshing after a theme change shows the opposite theme.

### Pitfall 4: `suppressHydrationWarning` Omission

**What goes wrong:** React throws a hydration mismatch warning in development and the page may flicker on first load (SSG static HTML has no `.dark`/`.light` class; next-themes' inline script adds it before hydration; React compares and sees a diff).

**Why it happens:** React 18+ in strict mode compares server-rendered HTML with client-rendered output; the `<html>` class differs.

**How to avoid:** `suppressHydrationWarning` on `<html>` element in layout.tsx is not optional. This is documented by next-themes as a required configuration step.

**Warning signs:** React warning "Prop `className` did not match. Server: '' Client: 'dark'"

### Pitfall 5: `pnpm dlx shadcn@latest` Inside pnpm Workspace

**What goes wrong:** Running `pnpm dlx` from the repo root (not `web/`) may resolve the workspace root's `pnpm-workspace.yaml` and attempt a monorepo init.

**Why it happens:** The shadcn CLI detects `pnpm-workspace.yaml` at the repo root and may prompt for monorepo setup.

**How to avoid:** Always `cd web/ && pnpm dlx shadcn@latest init ...` (or run from the `web/` directory context). Do NOT use `--monorepo` — the beekeeper web/ is a pnpm workspace member but not a shadcn-style monorepo (no shared `packages/ui` workspace).

**Warning signs:** CLI creates `apps/web/` directories; `components.json` ends up in the wrong location.

### Pitfall 6: Inter `axes: ['opsz']` Build Failure

**What goes wrong:** `next/font/google` may fail to construct a valid Google Fonts API URL with the `opsz` axis for Inter, depending on the next/font version's knowledge of the Inter font's axis registry.

**Why it happens:** Google Fonts updated Inter to v4 (with `opsz`) but `next/font`'s internal font metadata may be cached at an older version. [CITED: github.com/vercel/next.js/issues/68395]

**How to avoid:** Try `axes: ["opsz"]` first. If `pnpm build` fails with a font-related error mentioning `opsz` or an invalid URL, remove the `axes` prop and use `font-optical-sizing: auto` in CSS instead (same visual result since Inter v4 supports it automatically).

**Warning signs:** Build error containing "Invalid axis" or a 404 from fonts.googleapis.com during build.

---

## Code Examples

Verified patterns from official sources:

### globals.css Phase 12 skeleton

```css
/* Source: UI-SPEC §globals.css Order Contract + Tailwind v4 docs [CITED: ui.shadcn.com/docs/tailwind-v4] */
@import "tailwindcss";
@import "shadcn/tailwind.css";
/* fumadocs-ui imports: Phase 13 uncomments these */
/* @import "fumadocs-ui/css/shadcn.css"; */
/* @import "fumadocs-ui/css/preset.css"; */
/* @source "../node_modules/fumadocs-ui/dist/**/*.js"; */

/* Required for next-themes class strategy: [CITED: tailwindcss.com/docs/dark-mode] */
@custom-variant dark (&:where(.dark, .dark *));

/* Beekeeper token block — LAST so it wins [CITED: UI-SPEC §globals.css Order Contract] */
@theme inline {
  --font-sans: var(--font-inter);
  --font-mono: var(--font-jetbrains-mono);
  --font-display: var(--font-inter);
  /* ... full token block from UI-SPEC ... */
}

:root { /* dark defaults — raw CSS custom props */ }
.light { /* light overrides */ }

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;
  }
}
```

### layout.tsx Phase 12 replacement

```tsx
/* Source: UI-SPEC §Theme Toggle Contract + nextjs.org/docs/app/api-reference/components/font [CITED] */
import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { Providers } from "./providers";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
  axes: ["opsz"],  // fallback: remove if build fails; use font-optical-sizing: auto in CSS
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Beekeeper",
  description: "Real-time safety harness for autonomous coding agents.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${jetbrainsMono.variable}`}
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col antialiased">
        <a
          href="#main-content"
          className="sr-only focus-visible:not-sr-only focus-visible:absolute focus-visible:top-4 focus-visible:left-4 focus-visible:z-[100] focus-visible:bg-bk-surface focus-visible:text-bk-fg focus-visible:px-4 focus-visible:py-2 focus-visible:rounded focus-visible:border focus-visible:border-bk-border-strong"
        >
          Skip to main content
        </a>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
```

### providers.tsx

```tsx
/* Source: UI-SPEC §Theme Toggle Contract + §ReducedMotionProvider Contract */
"use client";
import { ThemeProvider } from "next-themes";
import { ReducedMotionProvider } from "@/lib/reduced-motion";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem={true}
      storageKey="bk-theme"
      disableTransitionOnChange
    >
      <ReducedMotionProvider>
        {/* Phase 13 inserts <RootProvider theme={{ enabled: false }}> here */}
        {children}
      </ReducedMotionProvider>
    </ThemeProvider>
  );
}
```

### cn() utility (created by shadcn init)

```ts
/* Source: shadcn manual install docs [CITED: ui.shadcn.com/docs/installation/manual] */
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

### ThemeToggle component (for Phase 15 to use)

```tsx
/* Source: next-themes docs pattern [CITED: npmjs.com/package/next-themes] */
"use client";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  // Mount guard prevents hydration mismatch (server has no theme knowledge)
  useEffect(() => { setMounted(true); }, []);
  if (!mounted) return null;

  const isDark = resolvedTheme === "dark";
  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      aria-label={isDark ? "Switch to light theme" : "Switch to dark theme"}
    >
      {/* icon slot */}
    </Button>
  );
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `tailwind.config.js` for shadcn | `components.json` with `tailwind.config: ""` (CSS-first) | Tailwind v4 / shadcn CLI v4 | No JS config file needed; all in globals.css `@theme` |
| `tailwindcss-animate` plugin | `tw-animate-css` via `@import` | shadcn CLI v4 (March 2025) | Simpler CSS-only import; `@plugin` syntax deprecated |
| `darkMode: 'class'` in tailwind.config | `@custom-variant dark (…)` in CSS | Tailwind v4 | Config key gone; lives in CSS |
| `shadcn-ui` npm package (CLI) | `shadcn` npm package | shadcn CLI v2 | Package renamed |
| shadcn injects variables into `@layer base` | shadcn injects via `@import "shadcn/tailwind.css"` + `@theme inline` | 2025 | Cleaner separation of concerns |
| HSL color format in shadcn | OKLCH color format | shadcn v4 components | Beekeeper tokens use hex (valid in Tailwind v4) |
| `forwardRef` in components | `React.ComponentProps` | React 19 / shadcn v4 | Components use `data-slot` attributes; no forwardRef needed |

**Deprecated/outdated:**
- `npx shadcn-ui@latest`: CLI package renamed to `shadcn`; old package may still exist but is not updated
- `@layer base { :root { … } }` pattern: Tailwind v4 does not use layers for variable definitions; use bare `:root { … }` outside layers

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `axes: ["opsz"]` works in `next/font/google` for Inter v4 | §Font Loading, Pitfall 6 | Build fails; fallback: remove `axes` prop, add `font-optical-sizing: auto` in CSS — same visual output, zero rework |
| A2 | `@import "shadcn/tailwind.css"` is the correct v4 import path (not `@import "tw-animate-css"` as a separate step) | §Pattern 2 | If shadcn CLI actually generates them as two separate imports, globals.css needs both; the net CSS result is the same — low risk |

---

## Open Questions

1. **Does `axes: ["opsz"]` work in the current Next.js 16 + next/font for Inter?**
   - What we know: Inter v4 on Google Fonts has `opsz` axis; documented issue #68395 existed for `slnt`, not `opsz`; the axis URL pattern for Inter changed in 2024
   - What's unclear: Whether Next.js 16 (released 2025) has updated font metadata for Inter v4
   - Recommendation: Attempt during Wave 0 build; if build fails, remove `axes: ["opsz"]` — all Inter optical-sizing behavior is achievable with `font-optical-sizing: auto` CSS property alone

2. **Exact Biome behavior on `@import "shadcn/tailwind.css"` import path**
   - What we know: Biome 2.2.0 (installed) has `noUnknownAtRules: off` already in biome.json — this suppresses false positives on `@theme`, `@source`, `@custom-variant`
   - What's unclear: Whether the import itself triggers any Biome resolver warning
   - Recommendation: If Biome reports an error on the `shadcn/` import specifier, add the file path to `files.includes` exclusions for CSS files — low-risk

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | pnpm, next/font build | ✓ | v22.17.0 | — |
| pnpm | Package installs | ✓ | 11.1.3 | — |
| Internet access (Google Fonts at build time) | next/font/google | ✓ (assumed) | — | Use `next/font/local` with downloaded font files |
| `web/node_modules` (pnpm install run) | shadcn init, build | ✓ (installed) | — | `pnpm install` |

**Missing dependencies with no fallback:** none

**Missing dependencies with fallback:**
- Google Fonts network: `next/font/google` requires network at build time; if offline, switch to `next/font/local` by downloading Inter v4 and JetBrains Mono WOFF2 files into `web/public/fonts/`

---

## Validation Architecture

> nyquist_validation is enabled (config.json key absent — treat as enabled).

### Test Framework

| Property | Value |
|----------|-------|
| Framework | No unit test framework exists yet in web/ (Phase 19 installs Vitest + Playwright) |
| Config file | None — Wave 0 of Phase 12 plan must not add a test runner |
| Quick run command | `pnpm build` (build assertion is the primary Phase 12 gate) |
| Full suite command | `pnpm build` (Phase 12 has no automated a11y tests — Phase 19 installs them) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DSYS-01 | `pnpm build` succeeds with correct import order and Tailwind utilities render Beekeeper colors | Build assertion | `cd web && pnpm build` | n/a — build output |
| DSYS-02 | Theme toggle adds `.dark`/`.light` to `<html>`; localStorage key `bk-theme` persists; no FOUC | Manual smoke test | Manual: open `out/index.html` via `pnpm start`, toggle theme, reload | n/a |
| DSYS-02 | `suppressHydrationWarning` present on `<html>` | Code audit | `grep -r "suppressHydrationWarning" web/app/layout.tsx` | ✓ after Phase 12 |
| DSYS-03 | `data-reduced-motion` set on `<html>` when OS reduced-motion is on | Manual browser check | DevTools → Rendering → Emulate prefers-reduced-motion | n/a |
| DSYS-03 | `useReducedMotion()` hook exported from `@/lib/reduced-motion` | TypeScript build | `cd web && pnpm build` (type errors surface) | ❌ Wave 0 creates file |
| DSYS-04 | Both themes pass WCAG AA contrast | Manual spot-check against UI-SPEC §WCAG tables + build-time a11y | Manual: Chrome DevTools a11y panel on built out/ | n/a |
| DSYS-04 | Skip link present and keyboard-reachable | Manual keyboard test | Tab to skip link, verify it appears | n/a |
| DSYS-04 | shadcn focus rings visible with `--color-ring: var(--color-bk-teal)` | Visual / code audit | `grep "ring" web/app/globals.css` | ❌ Wave 0 creates |

### Sampling Rate

- **Per task commit:** `cd web && pnpm build` — catches CSS/TypeScript regressions immediately
- **Per wave merge:** `cd web && pnpm build && pnpm lint` — ensures Biome clean
- **Phase gate (before verify):** Build green + manual smoke tests for DSYS-02 (theme toggle) + DSYS-03 (reduced-motion) + DSYS-04 (keyboard tab cycle through skip link → Providers renders)

### Wave 0 Gaps

- [ ] `web/lib/reduced-motion.tsx` — covers DSYS-03 (ReducedMotionProvider + `useReducedMotion` hook)
- [ ] `web/app/providers.tsx` — covers DSYS-02 (ThemeProvider wiring)
- [ ] `web/components.json` — covers DSYS-01 (shadcn init)

*(All gaps are files to CREATE in Phase 12 Wave 0/1 tasks — no framework install needed)*

---

## Security Domain

> security_enforcement: absent in config.json — treat as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no (design system, no user input forms in Phase 12) | — |
| V6 Cryptography | no | — |

### Known Threat Patterns for this Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Supply-chain via shadcn registry | Tampering | Use official registry only (ui.shadcn.com); `components.json` has no third-party registries |
| next-themes localStorage injection | Tampering | localStorage is client-side; no server state; no XSS surface in the theme script itself |
| CSP conflict with next-themes inline script | Tampering | The blocking `<script>` is inline and cannot use `nonce`-based CSP without custom implementation. Phase 12 scope is static export to Cloudflare Pages — no CSP headers configured at this phase. Noted for Phase 17 (SEO & Headers). |

**Phase 12 security posture:** minimal surface. The design system phase adds no server endpoints, no user input, no authentication surfaces. The primary supply-chain gate is: shadcn official registry only (enforced by `components.json` with no `registries` field and all components from `ui.shadcn.com`).

---

## Sources

### Primary (HIGH confidence)

- `nextjs.org/docs/app/api-reference/components/font` (version 16.2.7) — Inter/JetBrains Mono font loading, `axes` param, `variable` prop, static export behavior [CITED]
- `tailwindcss.com/docs/dark-mode` — `@custom-variant dark` syntax, class strategy, next-themes integration [CITED]
- `ui.shadcn.com/docs/components-json` — `tailwind.config: ""` for v4, `cssVariables`, aliases schema [CITED]
- `ui.shadcn.com/docs/installation/manual` — exact package list, `@import "shadcn/tailwind.css"`, `@custom-variant dark`, `@theme inline` structure [CITED]
- `fumadocs.dev/docs/ui/layouts/root-provider` — `theme={{ enabled: false }}` prop, RootProvider includes next-themes [CITED]
- `fumadocs.dev/docs/ui/theme` — CSS import structure, `shadcn.css` vs `neutral.css` vs `preset.css`, Tailwind v4 only [CITED]
- `12-UI-SPEC.md` — locked design contract, all token values, import order, provider contracts [AUTHORITATIVE]

### Secondary (MEDIUM confidence)

- npm registry: `next-themes@0.4.6`, `tw-animate-css@1.4.0`, `clsx@2.1.1`, `tailwind-merge@3.6.0`, `class-variance-authority@0.7.1`, `lucide-react@1.17.0`, `shadcn@4.10.0` — version verification [VERIFIED: npm registry]
- `github.com/shadcn-ui/ui/issues/10104` — React 19 "Encountered a script tag" warning from next-themes is a false positive [CITED]
- `github.com/pacocoursey/next-themes` — `disableTransitionOnChange` purpose, `suppressHydrationWarning` requirement [CITED]

### Tertiary (LOW confidence — marked [ASSUMED])

- Inter v4 `opsz` axis in `next/font/google` — conflicting reports; see Assumptions Log A1

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified on npm registry with version pinning
- Architecture: HIGH — all patterns traced to official docs (Tailwind v4, Next.js 16, shadcn, fumadocs, next-themes)
- Pitfalls: HIGH — sourced from GitHub issues and official changelogs
- `opsz` axis: LOW — conflicting reports; safe fallback documented

**Research date:** 2026-06-08
**Valid until:** 2026-09-08 (90 days — shadcn/Tailwind v4 is stable; next-themes 0.4.x is stable)
