# Phase 19: Test Suite & CI - Research

**Researched:** 2026-06-10
**Domain:** Vitest unit tests + @playwright/test E2E + GitHub Actions web.yml (pnpm workspace, Next.js 16 static-export)
**Confidence:** HIGH (all key claims verified against official Next.js docs, Playwright docs, npm registry, or the codebase itself)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01** Test toolchain: Full JS-native. Port home_spec.py + gfx_spec.py to @playwright/test (TS) E2E; port seo_spec.py + accuracy_spec.py to Vitest node tests; retire the .py files after JS parity.
- **D-02** Research-first (this document is the D-02 deliverable).
- **D-03** CI is build-verified locally on Windows. The plan must NOT block on a live GitHub run. Verify YAML by inspection/schema, not execution.
- **D-04** Add paths-ignore to Go ci.yml (web/**, pnpm-workspace.yaml, .github/workflows/web.yml). ci.yml currently has on:{pull_request, push:branches:[main]} with NO path filter.
- **D-05** Execution is INLINE on main (subagents lack node/pnpm/playwright-browsers).
- **D-06** New deps via package-legitimacy gate, pinned.
- **D-07** Playwright serves the real static out/ via a managed webServer; chromium-only default; QA-02 critical paths.

### Claude's Discretion

- Exact Vitest config (jsdom vs happy-dom; whether to add React Testing Library).
- Which client components/hooks get unit tests.
- Playwright project/browser matrix (default: chromium-only to mirror the .py specs).
- GitHub Actions pinned action versions (setup-node, pnpm/action-setup, playwright cache).

### Deferred Ideas (OUT OF SCOPE)

- Live GitHub-hosted CI execution (verified at repo push / SITE-03 deploy track).
- Cross-browser E2E (firefox/webkit) beyond chromium.
- Visual-regression / screenshot snapshots.
- Go CI restructuring beyond the `paths-ignore` web isolation.
- The deferred docs-command-card-copy.md backlog item.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| QA-01 | A path-filtered web CI job (separate from Go CI) builds the static site and runs unit (Vitest) + E2E (Playwright against `out/`) tests + lint/format (Biome), gating merges | §Standard Stack (Vitest/Playwright/Actions versions), §Architecture Patterns (job step order), §Code Examples (config shapes) |
| QA-02 | E2E tests verify the critical paths: home renders with hero fallback, docs navigation + search returns results, theme toggle persists across reload, changelog pages build and render headings | §Porting Map (exact assertion equivalence for each .py spec), §Code Examples (Playwright test patterns) |
</phase_requirements>

---

## Summary

Phase 19 delivers the test infrastructure for the `web/` Next.js 16 static-export site: a Vitest unit suite, a `@playwright/test` E2E suite against `out/`, and a path-filtered GitHub Actions job. The four existing Python specs are ported to JS and then retired.

**The core technical discovery is the static-server choice.** The `web/package.json` already has `"start": "pnpm dlx serve out"`, meaning `serve` (by Vercel, v14.x) is the established static server for this project. The Playwright `webServer` config should invoke `pnpm start` (or `npx serve out -p 4199 -l`) as its `command`, serving `out/` with directory/trailing-slash support. This is the direct JS equivalent of the Python specs' `http.server` approach and requires zero new dependencies if `serve` is invoked via `dlx` (no install).

**Vitest works cleanly with Next.js 16/React 19/TypeScript.** The official Next.js docs prescribe `vitest` + `@vitejs/plugin-react` + `jsdom` + `vite-tsconfig-paths` + `@testing-library/react` + `@testing-library/dom`. React 19 is fully supported by `@testing-library/react@16.3.2`. The `vitest.config.mts` must use `vite-tsconfig-paths` to resolve the `@/*` path alias from `tsconfig.json` (`moduleResolution: bundler`). Environment: `jsdom` (not happy-dom — Next.js docs recommend jsdom and it is the more mature option for React 19).

**GitHub Actions:** pnpm caching uses `pnpm/action-setup@v6` + `actions/setup-node@v4` with `cache: 'pnpm'`. Playwright's official guidance says **do NOT cache browser binaries** (restore time = download time). The web job runs Ubuntu-only (not the Go 3-OS matrix — E2E is Linux headless only). Bidirectional isolation: web.yml has a `paths:` include filter; ci.yml gains a `paths-ignore` block. These cannot conflict because they are on different workflow files — the same-file `paths`/`paths-ignore` mutual-exclusion rule does not apply across files.

**Primary recommendation:** Use `vitest@4.1.8` + `@vitejs/plugin-react@6.0.2` + `jsdom@29.1.1` + `vite-tsconfig-paths@6.1.1` + `@testing-library/react@16.3.2` + `@testing-library/dom@10.4.1` for unit; `@playwright/test@1.57.0` for E2E (matches the installed Python playwright 1.57.0 — same Chromium revision, no new browser download); serve `out/` via `pnpm start` in the `webServer` command. Add `vite` as a dev dep (vitest 4.x peer requires vite@^8).

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Unit tests (pure functions, hooks, components) | Developer machine / CI worker | — | Vitest runs in Node+jsdom, no server needed |
| E2E browser tests | CI worker (headless Chromium) | Developer machine | Playwright drives Chromium against the built `out/` via a managed webServer |
| Static file server for E2E | Playwright webServer lifecycle | — | `serve` invoked by PW webServer; PW manages start/stop/readiness |
| CI trigger filtering (web) | GitHub Actions `on.paths` | — | web.yml fires only on `web/**` + `pnpm-workspace.yaml` |
| CI trigger filtering (Go) | GitHub Actions `on.paths-ignore` | — | ci.yml gains `paths-ignore` so Go CI never fires on web-only changes |
| TypeScript path alias resolution | Vite plugin (vite-tsconfig-paths) | — | `@/*` alias from tsconfig.json `moduleResolution: bundler` must be mirrored in Vitest config |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| vitest | 4.1.8 | Unit test runner | Official Next.js recommendation; Vite-native, fastest startup, ESM-first [VERIFIED: npm registry + nextjs.org] |
| @vitejs/plugin-react | 6.0.2 | Vite React JSX transform for Vitest | Official Next.js Vitest setup; peer of vitest 4.x [VERIFIED: npm registry + nextjs.org] |
| jsdom | 29.1.1 | DOM environment for Vitest unit tests | Next.js docs recommend jsdom over happy-dom for React; mature (2011), 50M+/wk downloads [VERIFIED: npm registry + nextjs.org] |
| vite-tsconfig-paths | 6.1.1 | Resolves `@/*` path alias in Vitest | Required when tsconfig has `paths` with `moduleResolution: bundler` [VERIFIED: npm registry + nextjs.org] |
| vite | 8.0.16 | Vitest 4.x peer dep | vitest@4.1.8 peerDependencies requires `^6.0.0 \|\| ^7.0.0 \|\| ^8.0.0`; 8.x is current [VERIFIED: npm registry] |
| @testing-library/react | 16.3.2 | React component rendering + querying | Standard React unit test surface; supports React 19 (`^18 \|\| ^19`) [VERIFIED: npm registry] |
| @testing-library/dom | 10.4.1 | DOM query utilities (RTL dependency) | Peer of @testing-library/react; standalone for non-component assertions [VERIFIED: npm registry] |
| @playwright/test | 1.57.0 | E2E browser tests against out/ | **Pin to 1.57.0 to match the installed Python playwright 1.57.0 — same Chromium r1200, no new browser download needed** [VERIFIED: npm registry, local Python playwright 1.57.0 confirmed] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @testing-library/user-event | 14.6.1 | Realistic DOM user interactions | If testing clipboard, keyboard, form interaction in unit tests |
| vite (already listed) | 8.0.16 | Vitest peer dep | Always needed alongside vitest 4.x |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| jsdom | happy-dom | happy-dom is faster but has known gaps with some Web APIs; jsdom is the Next.js-documented choice for React 19 |
| @playwright/test | cypress | Playwright is lighter, headless-first, and matches the existing Python test patterns exactly |
| vite-tsconfig-paths | manual resolve.alias | Manual alias duplication risks drift from tsconfig.json; vite-tsconfig-paths reads tsconfig directly |
| serve (via dlx) | http-server, @hono/vite-serve | `serve` is already the `pnpm start` script — zero new dep required if invoked via dlx |

**Installation (web/ devDependencies):**
```bash
# Run from repo root (pnpm workspace; installs into web/ devDependencies)
pnpm --filter web add -D \
  vitest@4.1.8 \
  vite@8.0.16 \
  @vitejs/plugin-react@6.0.2 \
  jsdom@29.1.1 \
  vite-tsconfig-paths@6.1.1 \
  @testing-library/react@16.3.2 \
  @testing-library/dom@10.4.1 \
  @playwright/test@1.57.0
```

**Add scripts to `web/package.json`:**
```json
{
  "scripts": {
    "typecheck": "tsc --noEmit",
    "test": "vitest run",
    "test:watch": "vitest",
    "test:e2e": "playwright test"
  }
}
```

**Version verification performed:**

| Package | Registry Latest | Verified Date |
|---------|----------------|---------------|
| vitest | 4.1.8 | 2026-06-10 (published 2026-06-01) |
| @vitejs/plugin-react | 6.0.2 | 2026-06-10 (published 2026-05-14) |
| jsdom | 29.1.1 | 2026-06-10 (published 2026-04-30) |
| vite-tsconfig-paths | 6.1.1 | 2026-06-10 (published 2026-03-29) |
| vite | 8.0.16 | 2026-06-10 (published 2026-06-01) |
| @testing-library/react | 16.3.2 | 2026-06-10 (published 2026-01-19) |
| @testing-library/dom | 10.4.1 | 2026-06-10 (published 2025-12-13) |
| @playwright/test | 1.57.0 | 2026-06-10 (published 2024-12-xx — matches installed Python playwright 1.57.0) |

---

## Package Legitimacy Audit

> slopcheck was run but incorrectly targeted the Go ecosystem (not npm). Manual npm registry verification was performed instead (all packages confirmed legitimate via official GitHub repos, publication dates, and download counts).

| Package | Registry | Age | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-------------|-----------|-------------|
| vitest | npm | 4+ yrs (2021-12-03) | github.com/vitest-dev/vitest | N/A — ecosystem confusion | Approved [VERIFIED: npm registry + nextjs.org] |
| @vitejs/plugin-react | npm | 4+ yrs (2021-09-20) | github.com/vitejs/vite-plugin-react | N/A — ecosystem confusion | Approved [VERIFIED: npm registry + nextjs.org] |
| jsdom | npm | 15 yrs (2011-11-21) | github.com/jsdom/jsdom | N/A — ecosystem confusion | Approved [VERIFIED: npm registry + nextjs.org] |
| vite-tsconfig-paths | npm | 6+ yrs (2020-08-05) | github.com/aleclarson/vite-tsconfig-paths | N/A — ecosystem confusion | Approved [VERIFIED: npm registry + nextjs.org] |
| vite | npm | — (established) | github.com/vitejs/vite | N/A | Approved [VERIFIED: npm registry] |
| @testing-library/react | npm | 7+ yrs (2019-05-30) | github.com/testing-library/react-testing-library | N/A | Approved [VERIFIED: npm registry] |
| @testing-library/dom | npm | 7+ yrs (2019-05-30) | github.com/testing-library/dom-testing-library | N/A | Approved [VERIFIED: npm registry] |
| @playwright/test | npm | 5+ yrs (2020-09-24) | github.com/microsoft/playwright | N/A — ecosystem confusion | Approved [VERIFIED: npm registry, Microsoft] |

**Packages removed due to slopcheck [SLOP] verdict:** none — slopcheck targeted wrong ecosystem (Go). Manual verification confirms all packages are legitimate, well-established npm packages with official GitHub repos and multi-year histories.

**Packages flagged as suspicious [SUS]:** none.

**Postinstall scripts:** none found on any candidate package (`npm view <pkg> scripts.postinstall` returned empty for all).

---

## Architecture Patterns

### System Architecture Diagram

```
Developer Machine / CI Worker
│
├── pnpm install (--filter web, workspace-isolated, never touches go.mod)
│
├── Step 1: biome check --no-errors-on-unmatched
│   └── lint + format gate (biome.json already configured)
│
├── Step 2: tsc --noEmit
│   └── type-check only (noEmit:true already in tsconfig.json)
│
├── Step 3: vitest run --reporter=verbose
│   ├── vitest.config.mts (jsdom + @vitejs/plugin-react + vite-tsconfig-paths)
│   ├── web/tests/unit/*.test.ts(x) ← ported from seo_spec.py + accuracy_spec.py
│   └── reads: web/content/docs/*.mdx, web/lib/metadata.ts, web/out/ (built separately)
│         NOTE: unit tests for file-walk assertions (SEO/accuracy) depend on out/ existing
│               → run after build in CI; locally run after local pnpm build
│
├── Step 4: pnpm build (next build → out/)
│   └── static export to out/ (the E2E input)
│
└── Step 5: playwright test
    ├── playwright.config.ts
    │   ├── webServer: { command: 'pnpm start', url: 'http://127.0.0.1:4199', ... }
    │   │   └── pnpm start = 'pnpm dlx serve out' (already defined)
    │   └── project: chromium only
    └── web/tests/e2e/*.spec.ts ← ported from home_spec.py + gfx_spec.py
        reads: the live served out/ over HTTP (same as Python pattern)
```

> **Important ordering note for Vitest unit tests:** The `seo_spec.py` / `accuracy_spec.py` ports operate on `web/out/` (SEO) and `web/content/docs/` (accuracy). The accuracy tests need only source MDX files (always present). The SEO tests need `out/` (from pnpm build). In the CI pipeline they run **after** pnpm build (Step 4 → Step 5 is E2E; but Vitest runs in Step 3). Resolution: run SEO/accuracy Vitest tests in a separate `test:file` script that runs after build, or restructure so SEO tests are in the E2E playwright suite where `out/` is guaranteed. **Recommended:** Move SEO file-walk to the E2E suite (it's a file check, not a browser check, but having them in a post-build vitest run is also fine — the CI pipeline runs build before E2E anyway). See §Common Pitfalls #2 for full discussion.

### Recommended Project Structure

```
web/
├── vitest.config.mts        # NEW — Vitest config
├── playwright.config.ts     # NEW — Playwright config
├── tests/
│   ├── unit/                # NEW — Vitest unit tests (ported from .py file-walk specs)
│   │   ├── seo.test.ts      # port of seo_spec.py
│   │   └── accuracy.test.ts # port of accuracy_spec.py
│   └── e2e/                 # NEW — Playwright E2E specs (ported from .py browser specs)
│       ├── home.spec.ts     # port of home_spec.py
│       └── gfx.spec.ts      # port of gfx_spec.py
│   (existing .py files remain until JS parity proven, then deleted)
```

### Pattern 1: Vitest Config for Next.js 16 + React 19 + TypeScript

```typescript
// web/vitest.config.mts
// Source: https://nextjs.org/docs/app/guides/testing/vitest [CITED: official Next.js docs]
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tsconfigPaths from 'vite-tsconfig-paths'

export default defineConfig({
  plugins: [tsconfigPaths(), react()],
  test: {
    environment: 'jsdom',
    globals: false,          // explicit imports (import {expect,it,describe} from 'vitest')
    include: ['tests/unit/**/*.test.{ts,tsx}'],
    exclude: ['node_modules', '.next', 'out', 'tests/e2e'],
  },
})
```

**Key constraint:** `tsconfigPaths()` MUST come before `react()` in plugins array. It reads `web/tsconfig.json` which has `"paths": { "@/*": ["./*"], "collections/*": ["./.source/*"] }` — without this, any import using `@/` will fail in Vitest.

**`globals: false` recommendation:** Do not enable globals. The project uses Biome for linting, which won't know about test globals unless configured. Explicit `import { expect, it, describe } from 'vitest'` is cleaner.

**jsdom vs happy-dom:** jsdom is the right choice for this codebase. The `ReducedMotionProvider` uses `window.matchMedia` and `addEventListener` — jsdom supports this, happy-dom has known gaps with media query events. [CITED: nextjs.org/docs/app/guides/testing/vitest]

### Pattern 2: Playwright Config for Static Export

```typescript
// web/playwright.config.ts
// Source: https://playwright.dev/docs/test-configuration [CITED: official Playwright docs]
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: 'tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://127.0.0.1:4199',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'pnpm start',       // pnpm start = 'pnpm dlx serve out' (already in package.json)
    url: 'http://127.0.0.1:4199',
    reuseExistingServer: !process.env.CI,
    timeout: 15000,
  },
  outputDir: 'playwright-results',
})
```

**Key constraint: `pnpm start` vs port conflict.** The existing `start` script is `pnpm dlx serve out`. `serve` picks a random port if 4199 is busy — use `serve -p 4199` explicitly. **Update the start script** to `"start": "pnpm dlx serve out -p 4199 -l tcp://127.0.0.1:4199"` (or keep start as-is and set `webServer.command: 'pnpm dlx serve out --listen 4199'` directly in playwright.config.ts). The `-l` / `--listen` flag for `serve` v14 is the port-lock mechanism.

**trailingSlash routing:** With `trailingSlash: true` in next.config.mjs, all routes emit as `path/index.html`. The `serve` package handles directory requests correctly — `http://127.0.0.1:4199/docs/getting-started/` → `out/docs/getting-started/index.html`. No special config needed.

**`reuseExistingServer: !process.env.CI`:** locally you can pre-start the server; in CI always fresh.

### Pattern 3: GitHub Actions web.yml

```yaml
# .github/workflows/web.yml
name: Web CI

on:
  push:
    branches: [main]
    paths:
      - 'web/**'
      - 'pnpm-workspace.yaml'
      - '.github/workflows/web.yml'
  pull_request:
    paths:
      - 'web/**'
      - 'pnpm-workspace.yaml'
      - '.github/workflows/web.yml'

jobs:
  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Install pnpm
        uses: pnpm/action-setup@v6
        with:
          version: 11.1.3     # pin to match packageManager in web/package.json

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'  # matches local dev (22.17.0)
          cache: 'pnpm'

      - name: Install dependencies
        run: pnpm install --frozen-lockfile

      - name: Install Playwright browsers
        run: pnpm exec playwright install chromium --with-deps

      - name: Lint + format (Biome)
        run: pnpm lint

      - name: Type-check
        run: pnpm typecheck

      - name: Unit tests (Vitest)
        run: pnpm test

      - name: Build (static export)
        run: pnpm build

      - name: E2E tests (Playwright)
        run: pnpm test:e2e

      - name: Upload Playwright report
        uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: playwright-report
          path: web/playwright-results/
          retention-days: 7
```

**Key note on `working-directory: web` via defaults:** This applies to all `run:` steps but NOT to `uses:` steps. The `pnpm install --frozen-lockfile` runs in `web/` because of `defaults.run.working-directory`. The pnpm cache key from `actions/setup-node` reads `pnpm-lock.yaml` from the repo root (pnpm workspace lockfile lives at root) — this is correct: the workspace lockfile covers the `web/` package.

**Key note on Playwright browser caching:** The official Playwright docs explicitly say **do NOT cache browser binaries** — restore time is comparable to download time. Run `playwright install chromium --with-deps` fresh every run. [CITED: playwright.dev/docs/browsers#install-browsers]

**Key note on `--with-deps`:** On Ubuntu-latest, `--with-deps` installs Chromium system dependencies (libgbm, libnss, etc.) in one step. Without it, the browser launch will fail with missing system library errors.

### Pattern 4: Go ci.yml paths-ignore Addition

```yaml
# Addition to existing .github/workflows/ci.yml
# Insert after the 'on:' key, modifying the push and pull_request triggers:

on:
  pull_request:
    paths-ignore:
      - 'web/**'
      - 'pnpm-workspace.yaml'
      - '.github/workflows/web.yml'
  push:
    branches: [main]
    paths-ignore:
      - 'web/**'
      - 'pnpm-workspace.yaml'
      - '.github/workflows/web.yml'
```

**Current ci.yml state (from file inspection):**
```yaml
on:
  pull_request:
  push:
    branches: [main]
```
There is no path filter at all. The `paths-ignore` block must be added to BOTH `pull_request` and `push` triggers to achieve full bidirectional isolation (SC-1).

**`paths` vs `paths-ignore` across files:** The mutual-exclusion rule (cannot use both `paths` and `paths-ignore` on the same event in the same workflow) does NOT apply across different workflow files. `web.yml` uses `paths:` (include); `ci.yml` uses `paths-ignore:` (exclude). These are independent workflow files — no conflict. [CITED: docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions]

### Anti-Patterns to Avoid

- **Using `next start` as the webServer command:** `next start` requires a Node runtime server. With `output: 'export'`, `next start` does not work — use `serve` or any static file server.
- **Running Vitest unit tests with `environment: 'jsdom'` without `vite-tsconfig-paths`:** The `@/*` alias will resolve at the TypeScript level but fail at the Vite/Vitest runtime level, producing `Cannot find module '@/lib/metadata'` errors.
- **Caching Playwright browser binaries in GitHub Actions:** Official docs say it wastes time. Skip the cache.
- **Using `pnpm install` without `--frozen-lockfile` in CI:** Without this flag, pnpm may update the lockfile, making CI non-deterministic. The lockfile lives at repo root (`pnpm-lock.yaml`).
- **Testing async Server Components with Vitest:** Next.js docs warn that Vitest does not support async RSC. The `web/app/` page components that import Fumadocs async source are RSC — do NOT try to render them in Vitest. Unit-test client components only.
- **Serving `out/` with a server that doesn't support directory indexes:** Some static servers return 404 for `/docs/getting-started/` if they don't auto-append `index.html`. `serve` handles this correctly. `python http.server` also handles it — this is why the Python specs worked.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TypeScript path alias resolution in Vitest | Custom `resolve.alias` config | `vite-tsconfig-paths` | Stays in sync with tsconfig.json automatically; manual alias duplication drifts |
| Static file server for E2E | Custom Node.js http.createServer | `serve` via `pnpm dlx serve out` | Already the `pnpm start` script; handles directory indexes, trailing slash, MIME types |
| Playwright browser provisioning | Manual download scripts | `playwright install chromium --with-deps` | Playwright's own tool manages the correct Chromium revision for the installed version |
| React component test assertions | Raw jsdom querySelector | `@testing-library/react` `render` + `screen` | Accessibility-semantics-first queries; less brittle than DOM traversal |

---

## Porting Map: Python Specs → JS Equivalents

### home_spec.py → `web/tests/e2e/home.spec.ts` (@playwright/test)

| Python assertion | JS equivalent | Notes |
|-----------------|---------------|-------|
| `page.locator("text=autonomous coding agents").is_visible()` | `expect(page.locator('text=autonomous coding agents')).toBeVisible()` | Direct port |
| `install_chip.bounding_box()["y"] + height <= 800` | `const box = await installChip.boundingBox(); expect(box.y + box.height).toBeLessThanOrEqual(800)` | Same viewport (1280x800) |
| `page.locator('a', has_text='Read the docs').is_visible()` | `expect(page.getByRole('link', { name: 'Read the docs' })).toBeVisible()` | RTL-style query preferred |
| Dark/light background JS eval + compare | `page.evaluate(...)` same pattern | PW native JS evaluate works identically |
| `page_text = page.content()` then `name in page_text` | `const content = await page.content(); expect(content).toContain(name)` | Direct port; 15 harnesses + 4 gap markers |
| Theme toggle: classList manipulation via JS eval | Same `page.evaluate("document.documentElement.classList.add('dark')")` | Port verbatim |

**QA-02 additions not in Python spec (required by CONTEXT.md D-07):**
- docs navigation: `page.goto('/docs/getting-started/')` → click sidebar link → `expect(page).toHaveURL(/docs/)` 
- docs search: `page.getByRole('button', { name: /search/i }).click()` → type query → `expect(page.locator('[cmdk-item]').first()).toBeVisible()` → click first result → `expect(page).toHaveURL(/docs/)`
- theme toggle persists across reload: click toggle → `page.reload()` → verify class still applied
- All three changelog pages render headings: `page.goto('/changelog/v1.0.0/')` → `expect(page.getByRole('heading').first()).toBeVisible()`

### gfx_spec.py → `web/tests/e2e/gfx.spec.ts` (@playwright/test)

| Python assertion | JS equivalent | Notes |
|-----------------|---------------|-------|
| `open(INDEX_HTML).read()` then `sym in html` | `const html = await readFile('out/index.html', 'utf-8'); expect(html).not.toContain('<canvas')` | File read in Node via `fs/promises` inside `test.beforeAll` — no browser needed |
| `page.evaluate("document.querySelectorAll('canvas').length")` | `page.evaluate(() => document.querySelectorAll('canvas').length)` | Direct port |
| `browser.new_context(reduced_motion='reduce')` | `browser.newContext({ reducedMotion: 'reduce' })` | Same capability |
| `page.locator("img[src='/hero-hive.svg']").is_visible()` | `expect(page.locator("img[src='/hero-hive.svg']")).toBeVisible()` | Direct port |
| LCP/FCP JS eval + timing check | `page.evaluate(() => {...})` same performance API access | Direct port; headless proxy behavior is identical |
| Navigate away + back + canvas count check | `await page.goto(baseURL + 'docs/getting-started/')` → back → count | Direct port |
| `p.sr-only` presence when canvas mounted | `expect(page.locator('p.sr-only')).toHaveCount(1)` | Direct port |

**GFX-01a file check:** In `@playwright/test`, use `test.beforeAll` with `import { readFile } from 'fs/promises'` + `path.join(process.cwd(), 'out', 'index.html')` — no browser required. The `webServer` does not need to be started for this assertion; Playwright still starts it (that's fine). Alternatively, make it a Vitest node test.

### seo_spec.py → `web/tests/unit/seo.test.ts` (Vitest, node environment)

This is a **node environment** test (no jsdom needed — pure file I/O). Add `@vitest/environment-node` or use `// @vitest-environment node` per-file docblock.

| Python assertion | JS equivalent |
|-----------------|---------------|
| `path.read_text()` | `readFileSync(path, 'utf-8')` or `await readFile(path, 'utf-8')` |
| `re.search(r'<title>[^<]+</title>', content)` | `/\<title\>[^\<]+\<\/title\>/.test(content)` |
| `re.search(r'<link rel="canonical"...', content)` | regex or `content.includes(BASE_URL)` |
| `re.findall(r'<loc>', sitemap)` | `(sitemap.match(/\<loc\>/g) \|\| []).length` |
| `glob('out/**/index.html')` | `import { glob } from 'glob'` or manual `readdirSync` recurse |

**Important:** `seo.test.ts` must use `environment: 'node'` — the `@vitest/environment-node` is built-in. Add `// @vitest-environment node` at the top of the file, or set `environmentMatchGlobs: [['tests/unit/seo*', 'node']]` in vitest.config.mts.

**`glob` package:** For recursive file listing, use Node's built-in `fs.readdirSync` with recursion (Node 20+ supports `{ recursive: true }`) rather than adding a `glob` dependency. [ASSUMED]

### accuracy_spec.py → `web/tests/unit/accuracy.test.ts` (Vitest, node environment)

Same node environment pattern. Pure file-walk over `web/content/docs/**/*.mdx`.

| Python assertion | JS equivalent |
|-----------------|---------------|
| `frontmatter_block(text)` | Simple regex: `/^---\s*\n([\s\S]*?)\n---\s*\n/.exec(text)` |
| `re.search(r'^source_doc:\s*(.+?)\s*$', fm, re.MULTILINE)` | `/^source_doc:\s*(.+?)\s*$/m.exec(fm)` |
| `pathlib.Path(REPO_ROOT) / ref` then `.exists()` | `existsSync(path.join(REPO_ROOT, ref))` |
| UNENFORCED_FEATURES substring check | `text.includes('release_age')` etc. |
| PHANTOM_COMMANDS check | `text.includes('beekeeper hooks status')` etc. |

All Python stdlib patterns map directly to Node's `fs` module. The assertions are logically identical.

---

## Unit-Test Surface: What to Test with Vitest

### Genuinely Vitest-unit-testable (client components + pure functions)

| Component/function | Test type | What to assert |
|-------------------|-----------|----------------|
| `cn()` in `lib/utils.ts` | Pure function, node env | Class merging, conflict resolution |
| `useReducedMotion()` hook | React hook, jsdom | Returns `false` default; responds to matchMedia mock |
| `InstallChip` | Client component, jsdom | Renders install command text; copy button accessible label; clipboard mock |
| `BASE_URL` / `SITE_NAME` constants in `lib/metadata.ts` | Pure import, node env | Values match expected strings |
| seo file-walk assertions (seo.test.ts) | Node file I/O | Ported from seo_spec.py |
| accuracy file-walk assertions (accuracy.test.ts) | Node file I/O | Ported from accuracy_spec.py |

### Better left to E2E (not worth unit-testing)

| Component | Why skip unit test |
|-----------|-------------------|
| `Hero`, `HarnessMatrix`, `FeatureCards`, etc. | Static render-only server components; no interactive logic; E2E covers the visible output |
| `Providers` (ThemeProvider + ReducedMotionProvider wrapping) | Provider integration best tested via E2E where localStorage + class application is live |
| `SiteHeader`, `SiteFooter` | Static layout components; no logic |
| Fumadocs `DocsLayout`, `[[...slug]]` page | Async RSC — Vitest cannot render async server components; covered by E2E |
| `useCanMount3D` capability gate | The hero currently uses only the static SVG (R3F removed per commit `9e4a671`); no canvas mounts — this hook is vestigial if still present |

**Important codebase discovery:** The 3D R3F canvas was removed from the hero in commit `9e4a671` ("remove 3D hero canvas, keep the static flat hive"). The `hero.tsx` now renders only the static SVG. This means:
- GFX-01 (no canvas in server HTML) is trivially true  
- GFX-01b (canvas ≤ 1 after hydration) = 0 canvas, which is `<= 1` ✓
- GFX-02 (single WebGL context) = 0 canvas ✓
- GFX-03 (reduced-motion → SVG fallback) = always SVG, reduced-motion check is still valid to port
- GFX-04 (LCP/FCP budget + no context leak) = no canvas to leak; LCP test still valid

The `gfx.spec.ts` port should be faithful to the Python spec's logic (including the 0-canvas cases that pass vacuously) — this preserves the regression gate.

---

## Common Pitfalls

### Pitfall 1: Vitest fails to resolve `@/*` path aliases

**What goes wrong:** `Cannot find module '@/lib/metadata'` or `Cannot find module '@/components/home/install-chip'` at Vitest runtime.

**Why it happens:** The `tsconfig.json` has `"moduleResolution": "bundler"` and `"paths": {"@/*": ["./*"]}`. TypeScript resolves this at compile time but Vitest's Vite runtime does not read tsconfig paths unless `vite-tsconfig-paths` is in the plugins array.

**How to avoid:** `plugins: [tsconfigPaths(), react()]` in `vitest.config.mts`. The `tsconfigPaths()` call reads `web/tsconfig.json` automatically (it looks for `tsconfig.json` in process.cwd(), which is `web/` when running `pnpm test` from `web/`).

**Warning signs:** Import errors on module names starting with `@/` or `collections/`.

### Pitfall 2: SEO Vitest tests run before `pnpm build` in the CI pipeline

**What goes wrong:** `seo.test.ts` asserts over `web/out/**/*.html`. If Vitest runs (Step 3) before `pnpm build` (Step 4), `out/` is missing and all assertions fail.

**Why it happens:** The canonical CI step order (from QA-01) is lint → typecheck → unit → build → E2E. File-walk SEO tests logically need `out/` but sit in the "unit" category.

**How to avoid (two options):**
- Option A: Move `seo.test.ts` into a separate post-build test script `"test:file": "vitest run tests/unit/seo.test.ts"` and run it as a 5.5 step between build and E2E.
- Option B: Move the SEO file-walk into the `gfx.spec.ts` or a dedicated `seo.spec.ts` Playwright test using `import { readFile } from 'fs/promises'` inside `test()` bodies (Playwright tests can read the filesystem without launching a browser if run in the `gfx` project). **This is the recommended approach** — it keeps all post-build assertions in the E2E stage.
- Option C: Only the accuracy test (which reads source MDX, not `out/`) belongs cleanly in the "unit" Vitest stage. Move SEO to E2E stage. [ASSUMED — planner should decide based on QA-01 step-order constraint]

### Pitfall 3: `pnpm dlx serve` not honouring the port, causing Playwright webServer timeout

**What goes wrong:** `webServer` times out waiting for `http://127.0.0.1:4199` if `serve` picks a different port.

**Why it happens:** `pnpm dlx serve out` without a port flag will try port 3000 if 4199 is not the default.

**How to avoid:** In `playwright.config.ts`, set `webServer.command` to `"pnpm dlx serve out --listen 4199"` (or `"pnpm dlx serve out -p 4199"`). Alternatively, update `web/package.json` `start` script to include the port flag. The `-l tcp://127.0.0.1:4199` form of the serve CLI is explicit about both interface and port.

**Warning signs:** Playwright error `Error: browserType.launch: Failed to connect to the server at...` or timeout at webServer URL.

### Pitfall 4: `@playwright/test` version mismatch with installed Playwright browser

**What goes wrong:** `playwright install chromium` downloads a different Chromium revision than expected, or the installed `@playwright/test` version's Chromium revision differs from the Python `playwright 1.57.0` local install (Chromium r1200).

**Why it happens:** Each Playwright release pins a specific browser revision. Mismatches cause cryptic failures.

**How to avoid:** Pin `@playwright/test@1.57.0` — this matches the locally-installed Python `playwright==1.57.0` (both ship Chromium r1200). After `playwright install chromium`, Playwright checks `~/AppData/Local/ms-playwright` (Windows) or `~/.cache/ms-playwright` (Linux). Since r1200 is already installed locally, Windows local runs need no re-download. In CI (Linux), `playwright install chromium --with-deps` downloads it fresh each run (verified: official guidance says skip cache).

### Pitfall 5: Biome linting fails on new test files

**What goes wrong:** `biome check` reports errors in `tests/unit/*.test.ts` or `tests/e2e/*.spec.ts` — e.g., `noUnknownFunction` for `describe`, `it`, `test`, `expect`, or import organization warnings.

**Why it happens:** Biome 2.2.0 is the linter. Test files use testing globals that Biome may flag. The `biome.json` has `domains.react: recommended` and `domains.next: recommended` but no test domain.

**How to avoid:** Add `"domains": {"test": "recommended"}` under `linter` in `biome.json`. Biome 2.x has a `test` domain for Vitest/Jest globals. Alternatively, add `// biome-ignore lint/...` comments only where needed. [ASSUMED — verify Biome 2.2.0 test domain availability before relying on it]

### Pitfall 6: `tsc --noEmit` fails on Vitest config file

**What goes wrong:** `tsc --noEmit` picks up `vitest.config.mts` and fails on the `import { defineConfig } from 'vitest/config'` because `vitest` is not in `tsconfig.json` includes/compilerOptions.

**Why it happens:** `tsconfig.json` `include` field currently has `"**/*.ts"` and `"**/*.tsx"` — this picks up `vitest.config.mts`. If `vitest` types are not configured, tsc errors.

**How to avoid:** The `vitest` package includes its own types. Add a `/// <reference types="vitest/config" />` triple-slash reference at the top of `vitest.config.mts` (the Vitest docs-recommended approach), or ensure `vitest` is installed as a dev dep (which adds its types to the project). The `skipLibCheck: true` in tsconfig.json prevents most deep-type errors in node_modules.

### Pitfall 7: `next/font/google` import fails in Vitest jsdom tests

**What goes wrong:** Any Vitest test that imports from `app/layout.tsx` (which imports `next/font/google`) fails because `next/font/google` is a Next.js server-only module.

**Why it happens:** Vitest runs in Node + jsdom, not the Next.js build pipeline. Next.js server modules cannot be directly imported.

**How to avoid:** Do NOT import from `app/layout.tsx` or other RSC files in Vitest tests. Only import client components (`"use client"`) and pure utility modules. Add `vi.mock('next/font/google', () => ({...}))` if a transitive import pulls it in.

---

## Code Examples

### Example: Vitest test for `cn()` utility

```typescript
// web/tests/unit/utils.test.ts
import { describe, expect, it } from 'vitest'
import { cn } from '@/lib/utils'

describe('cn()', () => {
  it('merges class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar')
  })
  it('resolves Tailwind conflicts (last wins)', () => {
    expect(cn('p-2', 'p-4')).toBe('p-4')
  })
  it('filters falsy values', () => {
    expect(cn('foo', false && 'bar', null, undefined)).toBe('foo')
  })
})
```

### Example: Vitest test for `useReducedMotion` hook

```typescript
// web/tests/unit/reduced-motion.test.tsx
// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { ReducedMotionProvider, useReducedMotion } from '@/lib/reduced-motion'

// Mock window.matchMedia (jsdom does not implement it)
function mockMatchMedia(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

describe('useReducedMotion', () => {
  it('returns false when prefers-reduced-motion: no-preference', () => {
    mockMatchMedia(false)
    const { result } = renderHook(() => useReducedMotion(), {
      wrapper: ReducedMotionProvider,
    })
    expect(result.current).toBe(false)
  })
})
```

Source: official @testing-library/react renderHook pattern [CITED: testing-library.com/docs/react-testing-library/api#renderhook]

### Example: Playwright E2E test (QA-02 home critical path)

```typescript
// web/tests/e2e/home.spec.ts
// Source: playwright.dev/docs/writing-tests [CITED]
import { expect, test } from '@playwright/test'

test.describe('Home page', () => {
  test('hero headline + install chip + CTA are above the fold', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto('/')
    await expect(page.locator('text=autonomous coding agents')).toBeVisible()
    const chip = page.locator('text=go install github.com').first()
    const box = await chip.boundingBox()
    expect(box!.y + box!.height).toBeLessThanOrEqual(800)
    await expect(page.getByRole('link', { name: 'Read the docs' })).toBeVisible()
  })

  test('dark and light theme backgrounds differ', async ({ page }) => {
    await page.goto('/')
    await page.evaluate(() => {
      document.documentElement.classList.remove('light')
      document.documentElement.classList.add('dark')
    })
    await page.waitForTimeout(150)
    const darkBg = await page.evaluate(() =>
      window.getComputedStyle(document.body).backgroundColor
    )
    await page.evaluate(() => {
      document.documentElement.classList.remove('dark')
      document.documentElement.classList.add('light')
    })
    await page.waitForTimeout(150)
    const lightBg = await page.evaluate(() =>
      window.getComputedStyle(document.body).backgroundColor
    )
    expect(darkBg).not.toBe(lightBg)
  })

  test('all 15 harness names are present in the DOM', async ({ page }) => {
    await page.goto('/')
    const content = await page.content()
    for (const name of ['Claude Code', 'Codex', 'Cursor', /* ... */ 'Trae']) {
      expect(content).toContain(name)
    }
  })
})
```

### Example: SEO file-walk as Vitest node test

```typescript
// web/tests/unit/seo.test.ts
// @vitest-environment node
import { readdirSync, readFileSync, existsSync, statSync } from 'fs'
import { join } from 'path'
import { describe, expect, it, beforeAll } from 'vitest'

const WEB_DIR = join(__dirname, '../..')
const OUT_DIR = join(WEB_DIR, 'out')
const BASE_URL = 'https://beekeeper.vercel.app'

function htmlFiles(dir: string): string[] {
  const results: string[] = []
  for (const entry of readdirSync(dir, { recursive: true, withFileTypes: true })) {
    if (entry.isFile() && entry.name === 'index.html') {
      const full = join(entry.parentPath, entry.name)
      if (!full.includes('404') && !full.includes('_not-found')) {
        results.push(full)
      }
    }
  }
  return results
}

beforeAll(() => {
  if (!existsSync(OUT_DIR)) {
    throw new Error('out/ not found — run pnpm build first')
  }
})

describe('SC-1: page metadata', () => {
  it('every page has title, description, canonical', () => {
    for (const file of htmlFiles(OUT_DIR)) {
      const html = readFileSync(file, 'utf-8')
      expect(html).toMatch(/<title>[^<]+<\/title>/)
      expect(html).toMatch(/<meta name="description" content="[^"]+"/)
      expect(html).toContain(`rel="canonical" href="${BASE_URL}/`)
    }
  })
})
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Python playwright + http.server | @playwright/test TS | Phase 19 | Single Node.js toolchain; CI requires no Python in web job |
| Manual Python file-walk tests | Vitest node tests | Phase 19 | Same logic, typed TS, runs in pnpm CI pipeline |
| No CI path filter on ci.yml | `paths-ignore: web/**` | Phase 19 | Go CI no longer spins on every web commit |
| No web CI | web.yml path-filtered job | Phase 19 | Automated quality gate for web/ changes |
| `pnpm dlx serve out` (no port pin) | `pnpm dlx serve out --listen 4199` | Phase 19 | Deterministic port for Playwright webServer |

**Deprecated/outdated:**
- Python specs (`web/tests/home_spec.py`, `gfx_spec.py`, `seo_spec.py`, `accuracy_spec.py`): retained until JS parity proven, then deleted.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Node's `readdirSync` with `{ recursive: true }` is available in Node 20+ and works for glob-style MDX file discovery | §Porting Map (accuracy/seo ports) | If not available on the Node version used, need the `glob` npm package instead — low risk, Node 20+ is well-documented |
| A2 | Biome 2.2.0 has a `test` domain that recognizes Vitest globals and reduces false-positive lint errors | §Common Pitfalls #5 | If no `test` domain, per-file `biome-ignore` comments are needed — cosmetic impact only |
| A3 | SEO Vitest tests should be restructured to run after `pnpm build` (moved to E2E stage or a post-build script) | §Common Pitfalls #2 | If planner keeps them in the Vitest unit stage, CI step order must ensure `pnpm build` runs first — breaks the standard lint→typecheck→unit→build→E2E ordering |
| A4 | `vite@8.0.16` as an explicit devDependency is required (vitest 4.1.8 peerDependency); it won't be auto-resolved by pnpm without explicit install | §Standard Stack | If pnpm auto-resolves vite via vitest's own deps, the explicit install is redundant but harmless |
| A5 | The `serve` CLI flag for port binding is `--listen 4199` or `-p 4199` in serve@14.x | §Common Pitfalls #3 | If the flag changed, the webServer command needs adjustment — serve v14.x changelog confirms `-p`/`--port` flag exists |

---

## Open Questions (RESOLVED)

> All four open questions were resolved by the orchestrator on 2026-06-10 after a codebase
> verification pass. The resolutions below are LOCKED directives for the planner.

- **OQ-1 (SEO test placement) → RESOLVED:** `accuracy.test.ts` reads source MDX only (no `out/`
  dependency) and stays in the **pre-build** Vitest unit stage. The SEO file-walk over `out/` moves
  to a dedicated **post-build** script `"test:postbuild"` (e.g. `vitest run tests/postbuild/seo.test.ts`,
  separate dir or a second Vitest project) wired into `web.yml` as a step that runs AFTER `pnpm build`
  and before/at the E2E stage. Canonical CI order becomes lint → typecheck → unit(accuracy + components)
  → build → postbuild(seo) → e2e. This honors QA-01's ordering without an `out/`-before-build hazard.
- **OQ-2 (`useCanMount3D` / 3D canvas) → RESOLVED:** VERIFIED removed. No `components/home/hero-canvas.tsx`,
  no `useCanMount3D`, and no `@react-three/*` in `web/package.json` (3D dropped in commit `9e4a671`).
  Do NOT write a `useCanMount3D` unit test. The `gfx` coverage ports faithfully as a regression gate, but
  its only QA-02-relevant assertion is "home renders with the `/hero-hive.svg` fallback visible" (GFX-03) —
  the planner MAY fold that single assertion into `home.spec.ts` and either keep a thin `gfx.spec.ts` for the
  (now-vacuous) 0-canvas regression checks or drop the vacuous ones. QA-02's "SVG hero fallback" MUST be covered.
- **OQ-3 (rebuild `out/` before `test:e2e` locally) → RESOLVED:** Do NOT add an auto-build `pretest:e2e`
  hook (it slows local watch). CI enforces order via the explicit build-before-e2e steps; locally the
  executor runs `pnpm build` before `pnpm test:e2e`. Document the order in the plan; no script hook.
- **OQ-4 (Biome `test` domain, A2 [ASSUMED]) → RESOLVED:** Do NOT rely on the unverified Biome 2.2.0
  `test` domain. Set Vitest `globals: false` (explicit `import { describe, it, expect } from 'vitest'`) and
  Playwright explicit `import { test, expect } from '@playwright/test'` so there are NO undeclared test
  globals for Biome to flag. The phase gate is `pnpm lint` exit 0 over the new test files; if Biome flags
  anything, add a point-of-use `biome-ignore`, not a speculative domain. This removes assumption A2 from the
  critical path.

---

## Open Questions (original, for reference)

1. **Where do SEO file-walk assertions live: Vitest unit stage or Playwright E2E stage?**
   - What we know: `seo.test.ts` reads `out/index.html` files; `pnpm build` must run first; the CI step order from QA-01 is lint→typecheck→unit→build→E2E.
   - What's unclear: Does the planner want to reorder (put SEO unit tests after build) or restructure (move SEO to E2E stage)?
   - Recommendation: Move SEO assertions to a post-build Vitest script (`"test:file": "vitest run tests/unit/seo.test.ts tests/unit/accuracy.test.ts"`) that runs between build and E2E. `accuracy.test.ts` reads source MDX (no `out/` dependency) so it can stay in the pre-build unit stage. This keeps the step order clean.

2. **Does `useCanMount3D` hook still exist in the codebase?**
   - What we know: Commit `9e4a671` removed the R3F canvas from `hero.tsx`. The hook was in `components/home/hero-canvas.tsx` which may or may not still exist.
   - What's unclear: If the canvas wrapper components were deleted entirely, there is no `useCanMount3D` to unit-test.
   - Recommendation: Before writing tests for `useCanMount3D`, verify the file exists. From the directory listing, `hero-canvas.tsx` is NOT in `web/components/home/` — it appears to have been removed. Skip the `useCanMount3D` unit test.

3. **Does the `out/` directory need to be rebuilt before running `pnpm test:e2e` locally?**
   - What we know: `out/` exists locally (confirmed by `ls web/out`). Playwright `webServer.reuseExistingServer: !process.env.CI` means the server won't restart if one is already running.
   - What's unclear: Local developer workflow — does the plan need a `pretest:e2e` script that runs `pnpm build`?
   - Recommendation: Add a note to the plan that locally, `pnpm build` must be run before `pnpm test:e2e`. In CI, the build step precedes the E2E step.

4. **Biome `--no-errors-on-unmatched` flag for CI lint step**
   - What we know: `biome check` with no arguments checks the whole project. The `biome.json` `files.includes` already excludes `node_modules`, `.next`, `dist`, `build`, `.source`.
   - What's unclear: Whether any new test files will trigger new Biome rules that cause CI lint failures.
   - Recommendation: The planner should add `"domains": {"test": "recommended"}` to biome.json at the same time as adding test files, so Biome recognizes test globals.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | vitest, playwright | ✓ | 22.17.0 | — |
| pnpm | workspace install | ✓ | 11.1.3 | — |
| Python playwright | existing .py specs | ✓ | 1.57.0 | — (being replaced) |
| Chromium (ms-playwright) | @playwright/test | ✓ | r1200 (at ~/AppData/Local/ms-playwright) | `playwright install chromium` |
| @playwright/test | E2E suite | ✗ (not yet installed) | — | Install per plan |
| vitest | unit suite | ✗ (not yet installed) | — | Install per plan |

**Missing dependencies with no fallback:** none — all dependencies have clear install paths.

**Key local fact:** Python playwright 1.57.0 is installed and Chromium r1200 is cached at `~/AppData/Local/ms-playwright/chromium-1200`. Pinning `@playwright/test@1.57.0` means `playwright install chromium` will confirm the same r1200 revision is present and skip the download on Windows.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Unit framework | vitest 4.1.8 |
| E2E framework | @playwright/test 1.57.0 |
| Unit config file | `web/vitest.config.mts` (new) |
| E2E config file | `web/playwright.config.ts` (new) |
| Unit quick run | `pnpm --filter web test` |
| E2E run (requires out/) | `pnpm --filter web test:e2e` |
| Full suite | `pnpm --filter web lint && pnpm --filter web typecheck && pnpm --filter web test && pnpm --filter web build && pnpm --filter web test:e2e` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| QA-01 | CI job exists and is path-filtered | Schema/lint | Static YAML inspection | ❌ Wave 0 — `web.yml` |
| QA-01 | ci.yml has paths-ignore | Schema/lint | Static YAML inspection | Edit existing `ci.yml` |
| QA-01 | Vitest unit tests pass | unit | `pnpm --filter web test` | ❌ Wave 0 — `vitest.config.mts` + `tests/unit/*.test.ts` |
| QA-01 | `pnpm build` succeeds (regression) | build | `pnpm --filter web build` | ✓ exists, already proven |
| QA-01 | Playwright E2E passes | e2e | `pnpm --filter web test:e2e` | ❌ Wave 0 — `playwright.config.ts` + `tests/e2e/*.spec.ts` |
| QA-02 | Home renders with SVG fallback + above-fold | e2e | `pnpm --filter web test:e2e -- --grep "hero"` | ❌ Wave 0 |
| QA-02 | Docs nav + search returns results | e2e | `pnpm --filter web test:e2e -- --grep "docs"` | ❌ Wave 0 |
| QA-02 | Theme toggle persists across reload | e2e | `pnpm --filter web test:e2e -- --grep "theme"` | ❌ Wave 0 |
| QA-02 | All 3 changelog pages render headings | e2e | `pnpm --filter web test:e2e -- --grep "changelog"` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `pnpm --filter web build && pnpm --filter web test` (unit gate)
- **Per wave merge:** full suite above
- **Phase gate:** full suite green + Python .py specs still green before retiring them

### Wave 0 Gaps

- [ ] `web/vitest.config.mts` — Vitest config
- [ ] `web/playwright.config.ts` — Playwright config
- [ ] `web/tests/unit/utils.test.ts` — `cn()` unit test
- [ ] `web/tests/unit/reduced-motion.test.tsx` — `useReducedMotion` unit test
- [ ] `web/tests/unit/accuracy.test.ts` — port of accuracy_spec.py
- [ ] `web/tests/unit/seo.test.ts` — port of seo_spec.py (or move to E2E stage — see OQ-1)
- [ ] `web/tests/e2e/home.spec.ts` — port of home_spec.py + QA-02 additions
- [ ] `web/tests/e2e/gfx.spec.ts` — port of gfx_spec.py
- [ ] `.github/workflows/web.yml` — new web CI workflow
- [ ] Biome `"domains": {"test": "recommended"}` added to `biome.json`
- [ ] New scripts in `web/package.json`: `typecheck`, `test`, `test:watch`, `test:e2e`
- [ ] `vite`, `vitest`, `@vitejs/plugin-react`, `jsdom`, `vite-tsconfig-paths`, `@testing-library/react`, `@testing-library/dom`, `@playwright/test` added to `web/package.json` devDependencies

---

## Security Domain

> `security_enforcement` is not explicitly set to `false` in config. This phase adds test infrastructure and a CI workflow — low security surface. Applicable ASVS categories are noted for completeness.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | partial | CI YAML is static — no user input; test file patterns are repo-local |
| V6 Cryptography | no | — |

**Security-relevant CI note:** The `web.yml` workflow uses `actions/checkout@v4`, `pnpm/action-setup@v6`, `actions/setup-node@v4`, `actions/upload-artifact@v4` — all established Microsoft/GitHub-maintained actions. No third-party actions with elevated permissions. The job requires no `id-token: write` or `contents: write` permissions (no signing, no release upload). Default `read` permissions are sufficient.

**Supply chain:** `@playwright/test` has no `postinstall` script. `vitest`, `@vitejs/plugin-react`, `jsdom`, `vite-tsconfig-paths`, `@testing-library/react`, `@testing-library/dom` all have no `postinstall` scripts (verified above). Safe to install.

---

## Sources

### Primary (HIGH confidence)

- [nextjs.org/docs/app/guides/testing/vitest](https://nextjs.org/docs/app/guides/testing/vitest) — official Next.js Vitest setup guide (version 16.2.9); prescribed package list, `vitest.config.mts` shape, jsdom recommendation [CITED]
- [playwright.dev/docs/test-configuration](https://playwright.dev/docs/test-configuration) — official Playwright config shape, webServer options, baseURL [CITED]
- [playwright.dev/docs/browsers#install-browsers](https://playwright.dev/docs/browsers#install-browsers) — explicit "do NOT cache browser binaries" guidance [CITED]
- [docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions](https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions#onpushpull_requestpull_request_targetpathspaths-ignore) — `paths` / `paths-ignore` semantics and mutual exclusion rule [CITED]
- npm registry (npm view) — all package versions, ages, repository URLs, postinstall scripts verified 2026-06-10 [VERIFIED: npm registry]
- Codebase: `web/package.json`, `web/tsconfig.json`, `web/biome.json`, `web/next.config.mjs`, `.github/workflows/ci.yml`, `web/tests/*.py` — all read directly from the repo [VERIFIED: codebase]

### Secondary (MEDIUM confidence)

- [pnpm.io/continuous-integration#github-actions](https://pnpm.io/continuous-integration) — pnpm/action-setup@v6 + actions/setup-node@v4 with `cache: 'pnpm'` pattern [CITED]
- [github.com/pnpm/action-setup/releases](https://github.com/pnpm/action-setup/releases) — confirmed latest release v6.0.8 [CITED]
- [github.com/actions/setup-node/releases](https://github.com/actions/setup-node/releases) — confirmed latest release v4 (current tag) [CITED]
- testing-library.com — `@testing-library/react` React 19 support confirmed via peerDependencies (`^18 || ^19`) [VERIFIED: npm registry]

### Tertiary (LOW confidence)

- Biome 2.2.0 `test` domain availability — referenced from general Biome knowledge; not verified against biome.json schema for v2.2.0 [ASSUMED — A2]
- Node 20+ `readdirSync({ recursive: true })` availability — general Node.js knowledge [ASSUMED — A1]

---

## Metadata

**Confidence breakdown:**
- Standard stack (package versions): HIGH — all verified on npm registry 2026-06-10
- Vitest config shape: HIGH — verified against official Next.js docs (version 16.2.9)
- Playwright config shape: HIGH — verified against official Playwright docs
- GitHub Actions YAML syntax: HIGH — verified against official GitHub docs
- Porting map (Python → JS assertions): HIGH — line-by-line comparison of Python specs against Playwright/Vitest APIs
- Biome test domain: LOW — not verified against Biome 2.2.0 schema

**Research date:** 2026-06-10
**Valid until:** 2026-07-10 (stable ecosystem; Vitest/Playwright release frequently but the pinned versions are locked)
