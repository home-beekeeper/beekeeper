# Pitfalls Research

**Domain:** Next.js static-export marketing + docs site (R3F/Three.js + shadcn + Fumadocs + Tailwind v4) added to a Go security CLI monorepo
**Researched:** 2026-06-07
**Confidence:** HIGH (grounded in official Next.js, Fumadocs, R3F, shadcn, and WCAG documentation; verified against GitHub issues and community discussions; specific to this stack combination)

---

## Critical Pitfalls

### Pitfall 1: R3F / three / drei imported outside the ssr:false dynamic() boundary

**What goes wrong:**
`next build` crashes with `ReferenceError: window is not defined` or `ReferenceError: WebGLRenderingContext is not defined`. The build aborts and emits no `out/`. In development mode the error appears as a hydration mismatch — the canvas renders fine in `next dev` (which is client-only in the browser) but explodes during build because Server Components execute in Node.js where browser globals do not exist.

**Why it happens:**
Three.js, R3F, and drei access `window`, `document`, `WebGLRenderingContext`, and `requestAnimationFrame` at module-import time (not just at call time). Any Server Component file that directly or transitively imports from these packages triggers Node.js execution of browser-only code.

The error is invisible in `next dev` if you only test in a browser session — the dev server never runs those components server-side in the same way `next build` does.

**How to avoid:**
- All three/r3f/drei imports must live exclusively in `HeroCanvasInner.tsx`, `HiveScene.tsx`, and `AmbientHex.tsx`.
- Every one of those files must have `'use client'` as its literal first line.
- `Hero3D.tsx` uses `next/dynamic` with `{ ssr: false }` to import `HeroCanvasInner`. It must NOT itself import from three/r3f/drei.
- Never let a Server Component or any file without `'use client'` import (even transitively) from these packages.
- Add a lint rule or `eslint-plugin-import` check to prevent three/r3f/drei imports outside the `components/hero/` directory.

**Warning signs:**
- `next build` passes in CI but the team says "it works locally" — they are testing in `next dev` only.
- `ReferenceError: window is not defined` in build logs.
- Hydration mismatch warning in the browser involving canvas or Three.js components.
- Any `import { Canvas }` appearing in a file that lacks `'use client'`.

**Phase to address:**
Phase 6 (3D Layer) — the R3F layer is built last deliberately. All prior phases (scaffolding through marketing sections) must build and pass without R3F. When R3F is added, `next build` must be run immediately and used as the gate before any scene work proceeds.

---

### Pitfall 2: Fumadocs search silently broken under output: 'export' (missing force-static)

**What goes wrong:**
The docs site builds successfully. All pages render. But when a user opens the search dialog and types, no results appear. The browser console shows `GET /api/search 404`. The Orama search index was never emitted to `out/` because the Route Handler was not configured as force-static.

**Why it happens:**
Fumadocs' default `RootProvider` search mode is `type: 'fetch'` — it calls `/api/search` at runtime as a live API route. Under `output: 'export'`, Next.js does not execute API route handlers at request time (there is no server). The route handler is simply skipped during build unless it is explicitly marked `export const dynamic = 'force-static'`. The missing file is silent — Next.js does not error; it just omits the file from `out/`.

A second failure mode: the Route Handler has `force-static` but `RootProvider` is not configured with `search={{ type: 'static' }}`. In this case the browser fetches the static JSON correctly but Fumadocs does not know to use it — the dialog still shows no results.

**How to avoid:**
Both sides of the integration must be set:
1. `app/api/search/route.ts`: `export const dynamic = 'force-static'`
2. `app/layout.tsx` RootProvider: `search={{ type: 'static' }}`

Verify by inspecting `out/api/search/index.json` after `next build`. If the file is absent or empty, one side is misconfigured. Add a Playwright E2E test: open the search dialog, type a term that appears in a doc, assert that at least one result is returned.

**Warning signs:**
- `out/api/search/` directory is absent or contains an empty file after build.
- Browser console shows `404 /api/search` on the deployed site.
- Search dialog renders but shows "No results" for any query.
- `RootProvider` has `search={{ type: 'fetch' }}` (the default) anywhere in the codebase.

**Phase to address:**
Phase 3 (Content Pipeline). The first verification gate for Phase 3 is `pnpm next build` succeeding AND `out/api/search/index.json` being non-empty. Do not proceed to content authoring until search is confirmed working.

---

### Pitfall 3: CSS import order silently breaks Fumadocs colors (invisible or wrong-color text)

**What goes wrong:**
Fumadocs components (sidebar, TOC, search dialog, code blocks) render in wrong colors — typically near-black-on-black in dark mode, or near-white-on-white in light mode. The home page looks fine. Only the docs section is broken. The bug only manifests visually — there are no build errors and no console warnings.

**Why it happens:**
`fumadocs-ui/css/shadcn.css` maps all `--color-fd-*` Fumadocs variables to shadcn CSS variable names (`--background`, `--foreground`, `--primary`, etc.). `fumadocs-ui/css/preset.css` reads `--color-fd-*` to style its components. If `preset.css` loads before `shadcn.css`, it reads the `--color-fd-*` variables before they are defined — they fall back to initial values (browser defaults), producing invisible or incorrectly colored elements.

The bug is insidious because `next dev` with hot reload can mask it — the browser receives the CSS files piecemeal and they may load in a different order than the emitted bundle.

Additionally, Tailwind v4's content scanning must be told to include Fumadocs distribution files. Without `@source '../node_modules/fumadocs-ui/dist/**/*.js'` in `globals.css`, Tailwind purges utility classes used inside Fumadocs components and docs pages render without their utility-class styling.

**How to avoid:**
Lock the import order in `globals.css` and never change it:
```css
@import 'tailwindcss';
@import 'fumadocs-ui/css/shadcn.css';   /* MUST be before preset.css */
@import 'fumadocs-ui/css/preset.css';
@source '../node_modules/fumadocs-ui/dist/**/*.js';
@theme { /* shadcn-generated token block */ }
```

Add a Playwright screenshot test against a docs page in both light and dark mode. Compare against a baseline to detect color regressions. This is the only reliable automated check for this class of bug.

**Warning signs:**
- Fumadocs sidebar text is invisible or nearly invisible in dark mode but fine in light mode (or vice versa).
- Code blocks in docs pages have wrong background colors.
- The search dialog has unreadable contrast.
- Any edit to `globals.css` that changes import order without reviewing this constraint.
- A developer follows a Fumadocs tutorial that does not mention the shadcn.css / preset.css ordering (many don't).

**Phase to address:**
Phase 2 (Design System). The CSS import order must be set correctly during initial setup and verified before any Fumadocs component is rendered. The Phase 2 verification gate is `pnpm next build` succeeding with the docs section rendering at correct contrast on both light and dark themes.

---

### Pitfall 4: Content accuracy failure — docs claim protection that the shipped binary does not enforce

**What goes wrong:**
The documentation states or implies that `release_age`, `lifecycle_script_allowlist`, or other policy-file fields actively block installations. A developer reads the docs, configures their policy file, and believes they are protected. They are not — those fields are parsed but not enforced in v1.3.0. When an unvetted package slips through, the user loses trust not just in Beekeeper but in the project's honesty.

For a security tool, overclaiming is not a UX issue — it is a credibility and trust failure with security consequences.

**Why it happens:**
Docs are written while referencing the policy schema (which is complete) rather than the enforcement layer (which has gaps). The schema shows `release_age` as a valid field. A writer assumes "if it's in the schema, it works." They write the configuration guide as if it is enforced.

The same failure occurs with the Hermes harness (structurally fail-OPEN — tool calls succeed even when Beekeeper is running unless the MCP gateway is also configured), Kilo and Trae (native tools are unguarded), and the `--bind 0.0.0.0` gateway option (the `allow_remote_gateway` config gate described in help text does not exist in v1.3.0).

**How to avoid:**
Every doc page that touches a configuration field or integration must be sourced directly from a shipped code reference or `docs/THREAT-MODEL.md` — not from the PRD, requirements, or the policy schema file alone. Unenforced features must carry an explicit callout:

```
:::warning[Not enforced in v1.3.0]
`release_age` appears in policy files and passes validation but is not evaluated
during `beekeeper check`. Do not rely on it for security enforcement.
:::
```

Tier-2 and Tier-3 harness pages must lead with the caveat, not bury it after the installation instructions. The Hermes page first sentence should state it is structurally fail-OPEN.

Process control: use the `source_doc:` frontmatter field on every MDX file that makes security claims. Before shipping, do a review pass comparing every such file against the referenced source document.

**Warning signs:**
- Any doc page that says a feature "blocks" or "prevents" without citing a specific code path or verified behavior.
- Harness integration pages that present installation steps before caveats.
- Policy-as-code guide that does not distinguish enforced from unenforced fields.
- Any content written from the PRD or REQUIREMENTS.md without cross-checking against the actual implementation.

**Phase to address:**
Phase 8 (Content Authoring) — the content review gate. Every MDX file that makes a security claim must be explicitly reviewed against `docs/THREAT-MODEL.md` and the real CLI flags before the phase is closed. This review is not optional and cannot be deferred to a "polish" pass.

---

### Pitfall 5: Dynamic routes without generateStaticParams silently produce 404s

**What goes wrong:**
`/changelog/v1.2.0/` returns 404 on the deployed site. The page renders in `next dev` because the dev server resolves dynamic routes at request time. But `next build` with `output: 'export'` requires every dynamic route to enumerate its parameters at build time. If `generateStaticParams` is missing from `app/changelog/[version]/page.tsx`, Next.js cannot know which version slugs to render — it emits no HTML for those routes.

**Why it happens:**
Developers test in `next dev` where dynamic routes resolve normally. The `output: 'export'` constraint is only enforced at build time. The missing function causes no build error — Next.js simply skips those routes.

**How to avoid:**
Any route with a `[param]` or `[[...slug]]` segment must export `generateStaticParams`. For the docs `[[...slug]]` route, fumadocs-core provides `source.getPages()` to enumerate all slugs. For `changelog/[version]`, `lib/changelog.ts` enumerates MDX files at build time.

After `next build`, manually check `out/changelog/` — every expected version directory with `index.html` must be present. Add this as a Playwright test: navigate to `/changelog/v1.2.0/` and assert the heading is visible.

**Warning signs:**
- A route with a `[param]` segment that lacks a `generateStaticParams` export.
- `out/` directory missing subdirectories for expected dynamic routes after build.
- Pages that render in `next dev` but return 404 on the static host.
- Build succeeds with no errors but the deployed changelog or docs pages are missing.

**Phase to address:**
Phase 4 (Changelog Pipeline) and Phase 3 (Docs Pipeline). Both must verify `generateStaticParams` is wired before content authoring proceeds. The verification gate is inspecting `out/` after build, not just checking that build succeeds.

---

### Pitfall 6: Theme flash (FOUC) from next-themes + suppressHydrationWarning omission

**What goes wrong:**
On first page load, users see a brief flash from the wrong theme — white background flashing to dark, or vice versa. This is especially prominent on the marketing home page where the Three.js hero background color must match the page background. A flash produces a jarring visual pop. The `<html>` element also throws a React hydration mismatch warning in the console (`Warning: Prop 'class' did not match`).

**Why it happens:**
`next-themes` reads the user's theme preference from `localStorage` on the client. The static HTML is generated without knowing the theme. When React hydrates, `next-themes` injects the correct class onto `<html>` — but React sees a mismatch between the SSG-generated HTML (no class) and the client-hydrated version (with the dark class). React logs a hydration warning unless `suppressHydrationWarning` is on `<html>`.

The flash itself is prevented by next-themes injecting a blocking inline script in `<head>` that reads localStorage and applies the class before paint. This works correctly in production builds — but only if the ThemeProvider is configured correctly.

**How to avoid:**
```typescript
// app/layout.tsx
<html lang="en" suppressHydrationWarning>
```

```typescript
// ThemeProvider configuration
<ThemeProvider
  attribute="class"
  defaultTheme="system"
  enableSystem
  disableTransitionOnChange
>
```

`suppressHydrationWarning` on `<html>` suppresses the expected class attribute mismatch. `disableTransitionOnChange` prevents a CSS transition flash when the theme switches on load.

Test in production mode (`pnpm next build` and serve `out/` locally) — the flash is invisible in `next dev` but can appear in production. Check with "System preference: Dark" in DevTools emulation.

**Warning signs:**
- React hydration mismatch warning referencing the `class` attribute on `<html>`.
- Visible flash of wrong background color on first load (especially noticeable when the system theme is dark and the page defaults to light).
- `suppressHydrationWarning` absent from the `<html>` element.
- `disableTransitionOnChange` absent from ThemeProvider (causes a CSS transition animation on page load).

**Phase to address:**
Phase 2 (Design System). The ThemeProvider setup is part of `app/layout.tsx` wiring. Verify by building and serving the static `out/` locally with DevTools set to dark/light system preference, checking for flash.

---

### Pitfall 7: Three.js WebGL context loss and memory leak on unmount / page navigation

**What goes wrong:**
After navigating between pages a few times, the Three.js hero slows down, drops to single-digit FPS, or the canvas goes black. In Safari and mobile browsers this is more pronounced. Memory usage in DevTools climbs with each navigation. In development, React Strict Mode double-invokes effects — this can trigger premature context loss during development but mask disposal bugs that appear only in production.

**Why it happens:**
Three.js creates GPU-side resources (geometries, materials, textures, render targets) that must be explicitly disposed when the component unmounts. R3F does not automatically dispose scene objects — only the `WebGLRenderer` itself is cleaned up on Canvas unmount. If `HiveScene.tsx` creates geometries or materials without disposing them in a `useEffect` cleanup, the GPU memory leaks across navigations.

Additionally, if multiple Canvas instances (hero + ambient accents) share a WebGL context on a device that limits context count (Safari caps at 8; Chrome at 16), exceeding the limit causes the oldest contexts to be lost — producing a black canvas.

**How to avoid:**
Every geometry, material, and texture created in scene components must be disposed in the `useEffect` cleanup or by using the `dispose` prop on R3F's `<mesh>` and `<primitive>` components.

```typescript
// In HiveScene.tsx
useEffect(() => {
  return () => {
    geometry.dispose();
    material.dispose();
  };
}, [geometry, material]);
```

Limit ambient canvas instances. The architecture calls for one hero canvas and two ambient canvases — three total. This is safe on all browsers. Do not add more without measuring context budget.

Verify in Chrome DevTools Memory tab: navigate away from the home page and back three times. Heap should not grow unboundedly.

**Warning signs:**
- Canvas goes black after several navigations.
- Performance degrades over time (requestAnimationFrame callback takes longer on each visit).
- Browser console: `THREE.WebGLRenderer: Context Lost.`
- Chrome DevTools Memory heap snapshot growing across navigation cycles.
- More than three simultaneous Canvas elements on any single page.

**Phase to address:**
Phase 6 (3D Layer). Disposal must be implemented from the first commit that adds scene objects — retrofitting it is error-prone because it requires auditing every resource creation call.

---

### Pitfall 8: Three.js hero blocks LCP — canvas is the largest contentful element

**What goes wrong:**
Lighthouse and PageSpeed Insights report LCP > 2.5s. The culprit is the Three.js canvas — it is sized to take up most of the viewport and does not paint until the R3F JavaScript chunk downloads and executes. The initial HTML has only the SVG fallback (or a loading spinner) in the hero slot, which is smaller and lower-contrast than the canvas. When the canvas eventually loads, it becomes the new LCP element — but the clock started from navigation, including the JS download time.

**Why it happens:**
`next/dynamic` with `ssr: false` defers the canvas entirely to client-side. The R3F chunk (~300KB compressed) must download, parse, and execute before the canvas paints. On 3G or in resource-constrained environments, this takes 2–6 seconds. If the canvas is the visually largest element on the page, it is the LCP candidate — and its late paint time becomes the LCP time.

**How to avoid:**
The SVG fallback in the `loading` prop of `dynamic()` must be as visually large and prominent as the canvas — styled with `w-full aspect-video` to fill the same layout space. This makes the SVG (not the canvas) the LCP element. The SVG is inlined or served as a tiny static file, so it loads instantly from the pre-rendered HTML.

Ensure the hero text (`<h1>`) above or beside the canvas is not gated behind the canvas load. The headline must be in the Server Component output (not inside `HeroCanvasInner`) so it is present in the initial HTML.

Run Lighthouse against the static `out/` (not `next dev`) before Phase 6 is closed. Target: LCP < 2.5s on simulated 4G, LCP candidate is the SVG or the `<h1>`, not the canvas.

**Warning signs:**
- Lighthouse LCP > 2.5s with the Three.js canvas identified as the LCP element.
- The hero SVG fallback is tiny or absent — the hero space is empty until JS loads.
- The `<h1>` headline is rendered inside `HeroCanvasInner.tsx` (inside the client component boundary) rather than in the Server Component layer.
- The R3F JS chunk appears in the critical render path (not deferred).

**Phase to address:**
Phase 6 (3D Layer). Run Lighthouse immediately after wiring the canvas. Do not proceed to ambient accent canvases until hero LCP passes.

---

### Pitfall 9: prefers-reduced-motion not respected — 3D canvas mounts for all users

**What goes wrong:**
Users with vestibular disorders or motion sensitivity who have enabled "Reduce motion" in their OS settings still see the rotating Three.js hero. The animation can cause dizziness, nausea, or (for photosensitive users) seizure risk if the scene includes rapid flickering. WCAG 2.3.3 (AAA) requires that non-essential animation can be disabled. WCAG 2.3.2 (AA) applies when animation is triggered by user interaction. A continuously animating hero fails these criteria if reduced-motion is ignored.

**Why it happens:**
Developers test on their own machines where system reduced-motion is not set. The `prefers-reduced-motion` media query is not automatically honored by Three.js or R3F — they animate regardless of system preference. Without an explicit check, the canvas renders and animates for all users.

**How to avoid:**
`ReducedMotionProvider` detects `window.matchMedia('(prefers-reduced-motion: reduce)')` and exposes a context value. `HeroCanvasInner.tsx` reads this context: if `true`, it returns the static SVG immediately without mounting a Canvas at all. No RAF loop runs, no WebGL context is created.

The `motion` library (ex-Framer Motion) respects `prefers-reduced-motion` automatically when using the `m` component with `LazyMotion` — verify this is the configuration used, not the full `motion` component without the reduced-motion guard.

Test by enabling "Reduce motion" in OS settings (macOS: Accessibility > Display > Reduce Motion; Windows: Ease of Access > Display > Show animations — disabled) and verifying the canvas does not mount.

**Warning signs:**
- `ReducedMotionProvider` exists but `HeroCanvasInner.tsx` does not import or check it.
- The Canvas mounts unconditionally regardless of `useReducedMotion()` return value.
- Ambient accent canvases (`AmbientHex.tsx`) do not check the reduced-motion context.
- Any Three.js scene with particles, rapid rotation, or bright flashes — these require a PEAT tool assessment even with reduced-motion gating.

**Phase to address:**
Phase 5 (Marketing Sections — no R3F yet) establishes `ReducedMotionProvider`. Phase 6 (3D Layer) integrates the check into `HeroCanvasInner.tsx`. Both checks must be in place before the canvas is considered releasable.

---

### Pitfall 10: pnpm workspace hoisting Node packages into Go module root

**What goes wrong:**
Running `pnpm install` from the repo root creates a `node_modules/` directory at the repo root. Go tooling (`go build`, `go test`, `go mod tidy`) scans the directory tree for `.go` files and occasionally trips over non-Go content in `node_modules/`. More critically, `go mod tidy` may misinterpret `package.json` files or `.ts` files nested in `node_modules/` as signals to modify `go.sum`. In practice this is rare but the pollution of the Go module root with Node artifacts can cause surprising CI failures on the Go build jobs.

A related failure: if `pnpm-workspace.yaml` is not at the repo root (or is incorrectly configured), `pnpm install` run from `web/` creates a root-level `node_modules/` anyway because pnpm resolves workspaces from the nearest `pnpm-workspace.yaml`. Misconfigured workspace boundaries leave hoisted packages in the wrong location.

**Why it happens:**
pnpm by default does not hoist to the root in workspace mode, but some older pnpm versions or certain `.npmrc` configurations (`hoist=true` or `shamefully-hoist=true`) do. If `web/package.json` is not marked with `"packageManager": "pnpm@9.x.x"` and Corepack is not enabled, a developer might accidentally run npm or yarn from `web/`, creating root-level artifacts.

**How to avoid:**
- `pnpm-workspace.yaml` at repo root, listing only `'web'`.
- `web/package.json` includes `"packageManager": "pnpm@9.x.x"` (Corepack enforces this).
- `.npmrc` in `web/` (not repo root): do not add `shamefully-hoist=true`.
- Add `node_modules/`, `web/.next/`, `web/out/`, `web/.source/` to repo root `.gitignore`.
- CI Go job and web job run in separate GitHub Actions jobs with no shared working directory steps.
- Never run `go` tooling from `web/` directory. Never run `pnpm` tooling from the Go module root without first checking that `pnpm-workspace.yaml` is configured to prevent root-level hoisting.

**Warning signs:**
- `node_modules/` appears at the repo root after `pnpm install`.
- `go mod tidy` reports unexpected changes after a web dependency update.
- `go build ./...` takes significantly longer (scanning node_modules).
- `web/pnpm-lock.yaml` is absent (packages installed into root instead of `web/`).

**Phase to address:**
Phase 1 (Scaffolding). The workspace isolation must be verified before any package installation. Run `pnpm install` from repo root and confirm that `node_modules/` appears only under `web/`, not at repo root.

---

### Pitfall 11: GitHub Pages subpath deployment breaks all asset URLs (missing basePath / assetPrefix)

**What goes wrong:**
The site is deployed to `username.github.io/beekeeper/` (without a custom domain). All JavaScript, CSS, and image assets return 404. Navigation to any page other than the root returns 404. The `next/link` hrefs point to `/docs/...` instead of `/beekeeper/docs/...`. The site is completely broken.

**Why it happens:**
Next.js generates all asset URLs relative to the domain root (`/_next/static/...`) unless `basePath` and `assetPrefix` are configured. When deployed to a subpath, the browser requests `username.github.io/_next/static/...` which does not exist — assets are at `username.github.io/beekeeper/_next/static/...`.

Additionally, GitHub Pages by default applies Jekyll processing which skips any directory starting with `_`. This means `_next/` is entirely excluded from the published site unless a `.nojekyll` file is present in the output root.

**How to avoid:**
If deploying to GitHub Pages without a custom domain:
```typescript
// next.config.ts
const nextConfig: NextConfig = {
  output: 'export',
  trailingSlash: true,
  basePath: '/beekeeper',
  assetPrefix: '/beekeeper/',
  images: { unoptimized: true },
  transpilePackages: ['three', '@react-three/fiber', '@react-three/drei'],
};
```

Add a `.nojekyll` file to `web/public/` — Next.js copies `public/` contents to `out/`, so `.nojekyll` ends up at the root of the deployed output, disabling Jekyll processing.

Recommendation: use Cloudflare Pages with a custom domain instead — this eliminates the subpath problem entirely. `basePath` is not needed, `.nojekyll` is not needed, and the `_next/` directory is served correctly.

**Warning signs:**
- Deployed site shows broken asset loading (DevTools: 404 for `/_next/static/...`).
- All navigation links lead to 404.
- `basePath` and `assetPrefix` absent from `next.config.ts` when using GitHub Pages without a custom domain.
- `.nojekyll` absent from `web/public/` for GitHub Pages deployments.

**Phase to address:**
Phase 1 (Scaffolding) — deployment target and `next.config.ts` must be decided before the first build. Phase 9 (CI) — CI deployment step must include `.nojekyll` for GitHub Pages or verify Cloudflare Pages is the target.

---

## Moderate Pitfalls

### Pitfall 12: next/og (ImageResponse) silently unavailable in static export

**What goes wrong:**
A developer adds per-page OG images using Next.js's `opengraph-image.tsx` file convention with `ImageResponse` from `next/og`. In `next dev` it works. In `next build` with `output: 'export'`, the OG image generation silently fails or the build errors — because `ImageResponse` uses an Edge runtime that is not available in the static export pipeline without careful `generateStaticParams` configuration.

**Why it happens:**
`next/og` with `ImageResponse` was designed for Edge runtime execution. When used without `generateStaticParams`, Next.js cannot render it at build time. The default `opengraph-image.tsx` convention works with static export only when combined with `generateStaticParams` that enumerates all slugs.

**How to avoid:**
For v1.3.0, use a single static `public/og-image.png` (1200x630) referenced in `lib/metadata.ts`. This is zero-complexity and works unconditionally with static export.

If per-page OG images are needed in the future: use `opengraph-image.tsx` with `generateStaticParams` (confirmed to work with static export as of Next.js 15+), or run a `prebuild` Node.js script using `@vercel/og` in Node.js mode to generate PNGs into `public/og/`.

Do not use `ImageResponse` in a Route Handler without `force-static` and `generateStaticParams`.

**Warning signs:**
- `app/[route]/opengraph-image.tsx` exists but does not export `generateStaticParams`.
- `next build` error referencing Edge runtime modules in a static export context.
- Social card preview shows no image when sharing docs or changelog URLs.

**Phase to address:**
Phase 7 (SEO and Static Assets). Decide the OG strategy (single static image vs. per-page) before implementing. Single static image is the correct v1.3.0 scope.

---

### Pitfall 13: Tailwind v4 @theme inline syntax required — shadcn form components unstyled in production

**What goes wrong:**
shadcn/ui form components (Input, FormLabel, Button) render with browser-default styles — no borders, wrong background, no focus ring. The bug is intermittent: some components look correct, others don't. It appears only in production builds, not in `next dev`.

**Why it happens:**
Tailwind v4 uses CSS-first configuration via `@theme`. When shadcn components use CSS custom properties (`hsl(var(--input))`) as color values inside Tailwind utility classes, Tailwind v4 must be told to resolve these via `@theme inline` syntax. Without it, Tailwind v4 generates the CSS with the variable reference but the utility mapping is incomplete — some utilities are emitted, some are purged or unresolved.

The shadcn CLI generates the correct `@theme` block if auto-detection of Tailwind v4 works correctly. If the setup sequence is wrong (e.g., Tailwind v4 installed after shadcn init) the generated block may use the wrong syntax.

**How to avoid:**
Follow the setup sequence from STACK.md exactly: `create-next-app` first (generates Tailwind v4 config), then `shadcn@latest init` (auto-detects v4 and generates `@theme inline` syntax). Never create `tailwind.config.js` — v4 is CSS-first.

If the `@theme` block generated by shadcn uses `@layer base` with `:root` and `.dark` without the `@theme` wrapper, shadcn detected Tailwind v3 — reinstall in the correct order.

**Warning signs:**
- shadcn component primitives appear unstyled in production but styled in dev.
- `Input` component has no visible border.
- `tailwind.config.js` file exists in `web/` — Tailwind v4 should use CSS-only config.

**Phase to address:**
Phase 2 (Design System). Verify by adding a shadcn `Button` and `Input` to a page and running `pnpm next build` — inspect the built HTML to confirm the components have Tailwind utility classes applied.

---

### Pitfall 14: Fumadocs @source scan missing — Tailwind purges docs utility classes

**What goes wrong:**
The docs section renders with Fumadocs layout structure (sidebar, TOC) but many utility classes are missing: wrong spacing, incorrect font sizes in code blocks, broken prose layout. The issue only manifests in production builds.

**Why it happens:**
Tailwind v4 uses content scanning to determine which utility classes to include in the output CSS. By default it scans `web/**/*.{ts,tsx}` — but Fumadocs components live in `node_modules/fumadocs-ui/dist/`, which is not scanned by default. Without `@source '../node_modules/fumadocs-ui/dist/**/*.js'`, Tailwind purges utility classes that appear only in Fumadocs components.

**How to avoid:**
Add to `globals.css` after the Fumadocs CSS imports:
```css
@source '../node_modules/fumadocs-ui/dist/**/*.js';
```

**Warning signs:**
- Docs pages have correct layout structure but wrong spacing, font sizes, or code block styling in production.
- `pnpm next build` completes but the deployed docs look different from `next dev`.
- `globals.css` does not contain `@source` pointing to `fumadocs-ui/dist`.

**Phase to address:**
Phase 2 (Design System). Part of the CSS import order setup.

---

### Pitfall 15: drei imported as a wildcard blows the client bundle past the 300KB budget

**What goes wrong:**
The Three.js + R3F + drei bundle exceeds 500KB compressed, causing noticeable load delay on the home page and failing the performance budget. Lighthouse shows TBT > 200ms because the large JS chunk blocks the main thread during parse.

**Why it happens:**
`@react-three/drei` exports hundreds of helpers. A wildcard import (`import * as drei from '@react-three/drei'`) or importing many helpers from the package root can cause Turbopack to include the entire drei package in the client bundle if tree-shaking is not effective.

Three.js itself does not tree-shake cleanly. But drei helpers that import optional peer dependencies (physics engines, postprocessing, etc.) add significant weight if not tree-shaken.

**How to avoid:**
Use named imports from the package root and let Turbopack tree-shake:
```typescript
import { Float, Environment } from '@react-three/drei';
```

Avoid importing helpers that have large optional peer dependencies unless those deps are installed and needed.

After Phase 6, run `pnpm next build` and inspect `out/_next/static/chunks/` — identify any chunk above 250KB. Use `@next/bundle-analyzer` if bundle composition is unclear.

**Warning signs:**
- Lighthouse TBT > 200ms on the home page.
- A single JS chunk in `out/_next/static/chunks/` larger than 300KB (compressed).
- `import * as drei` anywhere in the codebase.

**Phase to address:**
Phase 6 (3D Layer). Run bundle analysis as the final step of Phase 6 before proceeding to Phase 7.

---

### Pitfall 16: Go CI and web CI triggering on each other's changes (missing path filters)

**What goes wrong:**
Every commit to a Go file triggers the full web CI pipeline. Every commit to a web MDX file triggers the full Go CI pipeline. Both pipelines take 5–10 minutes. On a solo project with fast iteration, this doubles CI wait time and wastes compute.

The more dangerous failure: the web CI job installs Node.js on the same runner that runs Go tests, and a mis-scoped `node_modules/` or path pollution causes the Go build to fail with "package not found" errors because `$PATH` or `$GOPATH` is contaminated.

**Why it happens:**
A single workflow file (`ci.yml`) with no `paths:` filter on the trigger. Or separate workflow files with an overly broad trigger (`on: push: branches: [main]` with no path filter).

**How to avoid:**
Two separate workflow files with mutually exclusive path filters:

```yaml
# .github/workflows/go.yml
on:
  push:
    paths:
      - '**/*.go'
      - 'go.mod'
      - 'go.sum'
      - '.github/workflows/go.yml'

# .github/workflows/web.yml
on:
  push:
    paths:
      - 'web/**'
      - 'pnpm-workspace.yaml'
      - '.github/workflows/web.yml'
```

Go jobs must never install Node or pnpm. Web jobs must never install Go or run `go` commands.

**Warning signs:**
- A single `ci.yml` with no `paths:` filter.
- Go CI job steps that include `actions/setup-node`.
- Web CI job steps that include `actions/setup-go`.
- CI runtime doubling with each commit regardless of what changed.

**Phase to address:**
Phase 9 (CI). Path filters must be the first thing configured in the workflow files.

---

### Pitfall 17: Canvas has no accessible text — screen reader users see a blank element

**What goes wrong:**
Screen reader users navigating the marketing home page encounter the hero canvas element and hear nothing (or the generic fallback "canvas"). The Three.js scene that communicates Beekeeper's core value proposition is invisible to assistive technology. WCAG 1.1.1 (Level A) requires non-text content to have a text alternative.

**Why it happens:**
The HTML `<canvas>` element has no built-in accessibility semantics. R3F's `Canvas` component renders a `<canvas>` with no `aria-label`, `role`, or descriptive text content. Developers focus on the visual experience and overlook the AT experience.

**How to avoid:**
Two-pronged approach:
1. `aria-hidden="true"` on the `<canvas>` — marks it as decorative for screen readers.
2. A visually hidden but AT-visible description adjacent to the canvas:
```typescript
<canvas aria-hidden="true" />
<p className="sr-only">
  Diagram showing an AI agent's tool call being intercepted by Beekeeper,
  evaluated against threat intelligence, and either allowed or blocked.
</p>
```

The `loading` prop of `dynamic()` already renders the SVG fallback with an `alt` attribute — ensure that alt text is descriptive, not empty or generic.

For `motion` library animations on section headings, ensure they do not interfere with focus order. Animated elements must not trap keyboard focus.

**Warning signs:**
- `<canvas>` element without `aria-hidden="true"` in the rendered DOM.
- Screen reader announces "canvas" with no additional context.
- `alt=""` on the SVG fallback image when it is the primary means of conveying the hero concept.
- Tab order on the marketing page skips interactive elements or lands on non-interactive animated elements.

**Phase to address:**
Phase 5 (Marketing Sections) sets up the static SVG with correct alt text. Phase 6 (3D Layer) adds `aria-hidden` to the canvas and the `.sr-only` description.

---

### Pitfall 18: .source/ committed to git — noisy diffs and masked content changes

**What goes wrong:**
After running `next build`, the generated `.source/` directory (fumadocs-mdx output) appears as untracked files. A developer adds them with `git add .` and commits. Every subsequent content change regenerates `.source/` — PRs become unreviable with hundreds of lines of generated type changes mixed into MDX prose changes.

**Why it happens:**
fumadocs-mdx generates `.source/index.ts` and `.source/meta.ts` from the MDX content at build time. These are analogous to generated protobuf files — they should never be committed. Without a `.gitignore` entry, they appear as untracked and any `git add -A` operation includes them.

**How to avoid:**
Add to repo root `.gitignore`:
```
web/.source/
web/.next/
web/out/
```

The `pnpm dlx create-next-app` scaffold adds `.next/` automatically. `.source/` and `out/` must be added manually.

**Warning signs:**
- `.source/` directory appears in `git status` as untracked.
- PRs include changes to `.source/index.ts` or `.source/meta.ts`.
- `git diff` on a content-only MDX change shows hundreds of lines of generated type changes.

**Phase to address:**
Phase 1 (Scaffolding). `.gitignore` must include `.source/` before the first `next build` is run.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Single static `og-image.png` for all pages | Zero implementation cost; works with static export | Social shares for docs pages show the same generic image; reduced click-through from social | Acceptable for v1.3.0; revisit when social traffic is measurable |
| Hand-sync between `docs/THREAT-MODEL.md` and web MDX (no automation) | No build complexity | Content drift is a real risk — the source doc updates but the web doc does not | Acceptable with the `source_doc:` frontmatter convention + sync review at each release |
| No per-page Orama index tuning | Default tokenizer and relevance weights | Search result ranking may surface less relevant pages for Beekeeper-specific terms (e.g., "corroboration") | Acceptable at < 60 pages; tune when search quality complaints arise |
| Single R3F chunk not split by scene | Simpler imports | Ambient accents force-load the full drei package even if only Float and Environment are used | Acceptable if drei imports are per-component; revisit if bundle analysis shows disproportionate size |
| `images.unoptimized: true` (no CDN) | No CDN config required | Images served at full resolution regardless of viewport — mobile users download unnecessarily large images | Acceptable for v1.3.0 (site has few images: OG, logo, hero SVG); revisit if image-heavy content is added |
| No `@next/bundle-analyzer` in CI | Simpler CI | Bundle regressions not caught automatically — a bad drei import could double the bundle silently | Add as a manual verification step for Phase 6; automate if budget violations occur |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| shadcn + Tailwind v4 | Running `shadcn init` before `create-next-app` generates Tailwind v3-style `tailwind.config.js` | Run `create-next-app` first (generates v4 CSS-first config), then `shadcn init` (auto-detects v4) |
| Fumadocs + shadcn CSS | `fumadocs-ui/css/preset.css` imported before `fumadocs-ui/css/shadcn.css` in globals.css | Always `shadcn.css` before `preset.css`. The order is load-bearing. |
| Fumadocs + Tailwind v4 | Missing `@source` directive for fumadocs-ui dist — utility classes purged in production | Add `@source '../node_modules/fumadocs-ui/dist/**/*.js'` to globals.css |
| R3F + static export | Importing three/r3f/drei in any file without `'use client'` | All R3F imports isolated in `HeroCanvasInner.tsx` and `HiveScene.tsx`; dynamic() with ssr:false is the only boundary |
| R3F + next-themes | Canvas background color does not match page background on theme switch | Read CSS variables with `getComputedStyle(document.documentElement)` on theme change inside a `useThree` callback |
| next-themes + RSC | `useTheme()` called in a Server Component — throws "hooks can only be called in a client component" | `ThemeToggle.tsx` must be `'use client'`; never call `useTheme` in a Server Component |
| Fumadocs search + static export | `RootProvider` without `search={{ type: 'static' }}` — fetch mode calls /api/search at runtime | Always `type: 'static'` in RootProvider for output: 'export' |
| pnpm workspace + Go module | Running `pnpm install` from repo root without correct pnpm-workspace.yaml — hoists to root | pnpm-workspace.yaml must be at repo root; verify node_modules location after first install |
| next-sitemap + static deploy | sitemap.xml not in `out/` because postbuild script not configured | Add `"postbuild": "next-sitemap"` to package.json scripts |
| Changelog [version] route | `generateStaticParams` absent — dynamic route renders in dev but 404s in static export | Every [param] route must export generateStaticParams; verify by inspecting out/ after build |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Canvas as LCP element | LCP > 2.5s; canvas identified as LCP candidate | SVG fallback fills same layout space as canvas so LCP candidate is SVG (instant); `<h1>` is in server output | Breaks on 3G and slower connections; also breaks on mid-range laptops with slow JS parse |
| drei wildcard import | Initial JS bundle > 500KB compressed; TBT > 200ms | Named imports only; run bundle analyzer after Phase 6 | Breaks the performance budget immediately; masked in dev by hot-reload chunking |
| Multiple Canvas instances over context limit | GPU memory growth; context loss on Safari after 8 contexts | Cap at 3 Canvas instances total; dispose scene objects on unmount | Breaks on Safari (8 context limit) and on mobile GPUs (memory limited) |
| motion animations causing CLS | Layout shift metric > 0.1 in Lighthouse | Use `initial={{ opacity: 0 }}` with `animate={{ opacity: 1 }}` — opacity changes don't cause CLS; avoid initial layout-affecting animations | Breaks CLS score on any page with entrance animations |
| MDX compilation not cached between CI builds | CI build time grows linearly with content additions | Turbopack caches MDX compilation; ensure `.next/` is preserved in CI cache | Acceptable at < 60 pages; measure at Phase 8 when all content is authored |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Documenting unenforced features as working | Users configure policy fields that have no effect; false sense of protection | `source_doc:` frontmatter + manual review against shipped code at Phase 8 content gate |
| Hermes fail-OPEN caveat buried in a "known gaps" section but absent from the Hermes integration page | Hermes users believe their setup is fully protected | Caveat must appear at the top of the Hermes page AND in the known-gaps section — duplication is intentional |
| `--bind 0.0.0.0` gateway option documented without the warning that the config gate is absent | Operators expose the MCP gateway to the network believing a config guard exists | Document verbatim: "The `allow_remote_gateway` config gate described in the help text is not implemented in v1.3.0. Remote binding is unrestricted once `--bind` is set." |
| Using `@vercel/analytics` or any third-party analytics script | Tracking scripts on a security tool undermine trust; potential data exfiltration surface | No analytics by default; if ever needed, use self-hosted Plausible via a same-origin script |
| MDX components that render arbitrary user content | XSS risk if changelog MDX ever includes external content | All MDX is hand-authored in-repo; no external MDX sources; no dynamic remote MDX |
| Social OG image metadata pointing to an http:// URL | Mixed content warning; image may not load in some browsers | `metadataBase` must use `https://`; verify the domain is correct before launch |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Docs pages lack breadcrumbs | Users lose orientation in deep doc sections | Fumadocs DocsLayout includes breadcrumbs automatically — ensure DocsLayout is used, not a custom layout |
| Install CTA requires scrolling on mobile | Developer bounces without installing | Hero section must be entirely visible on mobile (375px wide) without scrolling; two CTAs stacked vertically on mobile |
| No copy-to-clipboard on `go install` command | Developers must manually select and copy; error-prone on mobile | `CopyableCodeChip` component wraps the install command with a copy button |
| Dark mode toggle not present in docs nav | Users cannot toggle theme while reading docs | Configure `nav.links` in DocsLayout to include a ThemeToggle component; test both sections |
| Keyboard users cannot dismiss the search dialog | Traps focus in the search dialog | Fumadocs search dialog handles Escape to close — verify in Playwright; do not override the Fumadocs dialog with a custom implementation |

---

## "Looks Done But Isn't" Checklist

- [ ] **Static export**: Run `pnpm next build` in CI (not just `next dev`) — build must succeed; inspect `out/` structure manually.
- [ ] **Search**: `out/api/search/index.json` must be non-empty; open search dialog in Playwright and verify results are returned.
- [ ] **Dynamic routes**: `out/changelog/v1.0.0/index.html`, `out/changelog/v1.2.0/index.html`, `out/docs/getting-started/quickstart/index.html` must all exist after build.
- [ ] **R3F isolation**: Grep for `from '@react-three/fiber'` in `web/components/` — only `HeroCanvasInner.tsx` and its direct imports should appear.
- [ ] **Reduced motion**: Enable OS reduced-motion setting; reload home page; canvas must not mount (verify with DevTools: no `<canvas>` element in DOM).
- [ ] **Canvas a11y**: `<canvas>` in DOM has `aria-hidden="true"`; adjacent `.sr-only` text is present and descriptive.
- [ ] **Fumadocs dark mode**: Docs section tested with both light and dark theme; sidebar, TOC, and code blocks have correct contrast.
- [ ] **Content accuracy**: Every MDX file with `source_doc:` frontmatter has been reviewed against the referenced source document before Phase 8 closes.
- [ ] **Hermes caveat**: Open `web/content/docs/integration/hermes.mdx`; the fail-OPEN warning must appear before the first configuration step.
- [ ] **Unenforced features**: Policy-as-code guide explicitly states which fields are enforced and which are recognized but unenforced in v1.3.0.
- [ ] **Node isolation**: After `pnpm install` from repo root, `node_modules/` must not exist at repo root — only under `web/`.
- [ ] **OG image**: Share a docs page URL in a social preview tool; the correct OG image must appear.
- [ ] **trailingSlash**: Navigate to `/docs` (no trailing slash) on the deployed site — must redirect to `/docs/` (with trailing slash), not 404.
- [ ] **Suppressed hydration warning**: No React hydration warnings in browser console on the home page with system dark mode enabled.
- [ ] **.source/ not in git**: `git status` shows no `.source/` files after `next build`.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| R3F in server component (build crash) | LOW | Move all R3F imports to `HeroCanvasInner.tsx`; add `'use client'`; verify build |
| CSS import order wrong (docs colors broken) | LOW | Reorder `globals.css`; `pnpm next build`; visual check |
| Search returning 404 | LOW | Add `export const dynamic = 'force-static'`; add `search={{ type: 'static' }}` to RootProvider; rebuild |
| Content accuracy failure discovered post-launch | HIGH | Emergency MDX update + redeploy; add public errata notice on the page; implement content review gate retroactively for all remaining pages |
| Bundle budget exceeded | MEDIUM | Run `@next/bundle-analyzer`; convert wildcard drei imports to named; remove unused drei helpers |
| Theme flash in production | LOW | Add `suppressHydrationWarning` to `<html>`; add `disableTransitionOnChange` to ThemeProvider |
| WebGL context loss | MEDIUM | Add `useEffect` cleanup with `.dispose()` for all scene objects; reduce Canvas instance count |
| pnpm hoisting to Go root | LOW | Add / fix `pnpm-workspace.yaml`; remove stray root `node_modules/`; re-run `pnpm install` from workspace root |
| GitHub Pages 404s (missing basePath) | LOW | Add `basePath` and `assetPrefix` to `next.config.ts`; add `.nojekyll` to `public/`; redeploy |
| generateStaticParams missing (404 on dynamic routes) | LOW | Add `generateStaticParams` to route; inspect `out/`; redeploy |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| R3F SSR crash | Phase 6 (3D Layer) | `pnpm next build` succeeds; grep confirms R3F imports isolated |
| Fumadocs search broken | Phase 3 (Content Pipeline) | `out/api/search/index.json` non-empty; Playwright search test passes |
| CSS import order (invisible text) | Phase 2 (Design System) | Visual check: docs in light + dark mode at correct contrast |
| Content accuracy / overclaiming | Phase 8 (Content Authoring) | source_doc review gate before phase closes |
| generateStaticParams missing | Phase 3 + 4 | Inspect `out/` for all expected route directories |
| Theme flash / FOUC | Phase 2 (Design System) | Build + serve `out/` with system dark mode; no visible flash |
| WebGL context loss | Phase 6 (3D Layer) | Navigate home then docs then home three times; no black canvas or perf degradation |
| LCP blocked by canvas | Phase 6 (3D Layer) | Lighthouse LCP < 2.5s; LCP candidate is SVG or `<h1>` |
| prefers-reduced-motion ignored | Phase 5 + 6 | Enable OS reduced-motion; verify no canvas in DOM |
| pnpm hoisting to Go root | Phase 1 (Scaffolding) | Verify `node_modules/` location after install |
| GitHub Pages subpath / missing basePath | Phase 1 (Scaffolding) + Phase 9 (CI) | Deploy to target host; verify all assets load |
| next/og not supported in static export | Phase 7 (SEO) | OG image renders in social preview tool |
| Tailwind v4 @theme syntax wrong | Phase 2 (Design System) | shadcn Input/Button styled correctly in production build |
| @source scan missing (Fumadocs purge) | Phase 2 (Design System) | Docs pages in production have correct spacing and code block styling |
| drei wildcard bundle blowup | Phase 6 (3D Layer) | Bundle analyzer shows R3F chunk < 300KB compressed |
| Go/web CI cross-triggering | Phase 9 (CI) | Commit a `.go` file; verify web CI does not trigger |
| Canvas a11y (screen reader blank) | Phase 5 + 6 | Screen reader test; `aria-hidden` + `.sr-only` text present |
| .source/ committed to git | Phase 1 (Scaffolding) | `git status` clean after `next build` |

---

## Sources

- [Next.js Static Exports — Unsupported Features](https://nextjs.org/docs/app/guides/static-exports) — confirmed ISR, middleware, next/og without generateStaticParams, Route Handlers without force-static are all unavailable. HIGH confidence.
- [Next.js API Routes in Static Export Warning](https://nextjs.org/docs/messages/api-routes-static-export) — Route Handler must use force-static. HIGH confidence.
- [OG image generation doesn't work with Static Export — Next.js Discussion #55890](https://github.com/vercel/next.js/discussions/55890) — confirmed ImageResponse requires generateStaticParams for static export. HIGH confidence.
- [Fumadocs Static Build Guide](https://www.fumadocs.dev/docs/deploying/static) — force-static Route Handler + `type: 'static'` RootProvider. HIGH confidence.
- [Fumadocs Orama Search Docs](https://www.fumadocs.dev/docs/ui/search/orama) — static mode configuration. HIGH confidence.
- [Fumadocs Theme Docs](https://www.fumadocs.dev/docs/ui/theme) — shadcn.css then preset.css import order; @source scan requirement. HIGH confidence.
- [Fumadocs Tailwind v4 Discussion #1338](https://github.com/fuma-nama/fumadocs/discussions/1338) — monorepo coexistence, variable mapping. MEDIUM confidence.
- [shadcn/ui Tailwind v4 Docs](https://ui.shadcn.com/docs/tailwind-v4) — @theme inline syntax, setup order. HIGH confidence.
- [shadcn/ui Troubleshooting — CSS Variable Conflicts](https://eastondev.com/blog/en/posts/dev/20260402-shadcn-ui-troubleshooting/) — form components unstyled in production with Tailwind v4. MEDIUM confidence.
- [R3F — Leaking WebGLRenderer Issue #3093](https://github.com/pmndrs/react-three-fiber/issues/3093) — disposal requirements on unmount. HIGH confidence.
- [R3F — WebGL Context Loss Discussion #723](https://github.com/pmndrs/react-three-fiber/discussions/723) — context loss on unmount; browser context limits. HIGH confidence.
- [R3F — Too many active WebGL contexts on Safari Discussion #2457](https://github.com/pmndrs/react-three-fiber/discussions/2457) — Safari 8-context limit. HIGH confidence.
- [next-themes GitHub README](https://github.com/pacocoursey/next-themes) — suppressHydrationWarning requirement; flash prevention via blocking script. HIGH confidence.
- [Fixing Dark Mode Flickering (FOUC) in Next.js](https://notanumber.in/blog/fixing-react-dark-mode-flickering) — disableTransitionOnChange recommendation. MEDIUM confidence.
- [WCAG C39 — prefers-reduced-motion](https://www.w3.org/WAI/WCAG21/Techniques/css/C39) — CSS media query technique for motion reduction. HIGH confidence.
- [web.dev — Animation and Motion Accessibility](https://web.dev/learn/accessibility/motion) — WCAG 2.3.3, seizure risk, vestibular disorder impact. HIGH confidence.
- [Next.js basePath and assetPrefix for GitHub Pages](https://wallis.dev/blog/next-js-basepath-and-assetprefix) — basePath + assetPrefix + .nojekyll requirement. HIGH confidence.
- [Three.js tree-shaking state — three.js forum](https://discourse.threejs.org/t/what-is-the-state-of-tree-shaking/33168) — three.js does not tree-shake cleanly; drei named imports help. MEDIUM confidence.
- [GitHub Actions monorepo path filtering](https://oneuptime.com/blog/post/2025-12-20-monorepo-path-filters-github-actions/view) — separate workflow files with paths: filters. HIGH confidence.
- `.planning/research/STACK.md` — locked stack, integration sequences, force-static pattern, transpilePackages requirement.
- `.planning/research/ARCHITECTURE.md` — component boundaries, R3F isolation, CSS layer composition.
- `.planning/research/FEATURES.md` — content accuracy requirements, known gaps, anti-features.
- `docs/THREAT-MODEL.md` — security posture facts, known gaps, fail-OPEN/fail-closed status per harness tier (authoritative).

---

*Pitfalls research for: Beekeeper v1.3.0 Web Presence & Documentation — Next.js static-export site (R3F + shadcn + Fumadocs + Tailwind v4) in a Go security CLI monorepo*
*Researched: 2026-06-07*
