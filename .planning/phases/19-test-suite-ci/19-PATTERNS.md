# Phase 19: Test Suite & CI - Pattern Map

**Mapped:** 2026-06-10
**Files analyzed:** 13 new/modified files
**Analogs found:** 13 / 13

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/vitest.config.mts` | config | transform | `web/tsconfig.json` (path alias source) | config-match |
| `web/playwright.config.ts` | config | request-response | `web/tests/home_spec.py` (server+browser pattern) | role-match |
| `web/tests/unit/utils.test.ts` | test | transform | `web/lib/utils.ts` (unit under test) | exact |
| `web/tests/unit/reduced-motion.test.tsx` | test | event-driven | `web/lib/reduced-motion.tsx` (hook under test) | exact |
| `web/tests/unit/install-chip.test.tsx` | test | request-response | `web/components/home/install-chip.tsx` (component under test) | exact |
| `web/tests/unit/metadata.test.ts` | test | transform | `web/lib/metadata.ts` (unit under test) | exact |
| `web/tests/unit/accuracy.test.ts` | test | file-I/O | `web/tests/accuracy_spec.py` | port-exact |
| `web/tests/postbuild/seo.test.ts` | test | file-I/O | `web/tests/seo_spec.py` | port-exact |
| `web/tests/e2e/home.spec.ts` | test | request-response | `web/tests/home_spec.py` | port-exact |
| `web/tests/e2e/gfx.spec.ts` | test | request-response | `web/tests/gfx_spec.py` | port-exact |
| `web/package.json` | config | — | `web/package.json` (current scripts block) | modify-existing |
| `web/biome.json` | config | — | `web/biome.json` (current linter.domains block) | modify-existing |
| `.github/workflows/web.yml` | config | event-driven | `.github/workflows/ci.yml` (Actions style) | role-match |
| `.github/workflows/ci.yml` | config | event-driven | `.github/workflows/ci.yml` (current `on:` block) | modify-existing |

---

## Pattern Assignments

### `web/vitest.config.mts` (config, transform)

**Analogs:** `web/tsconfig.json` (path aliases to mirror), RESEARCH §Pattern 1

**Path aliases to mirror** (from `web/tsconfig.json` lines 22-25):
```json
"paths": {
  "@/*": ["./*"],
  "collections/*": ["./.source/*"]
}
```
`vite-tsconfig-paths` reads this automatically when run from `web/` — no manual alias duplication needed.

**Core config pattern** (from RESEARCH §Pattern 1):
```typescript
// web/vitest.config.mts
/// <reference types="vitest/config" />
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tsconfigPaths from 'vite-tsconfig-paths'

export default defineConfig({
  plugins: [tsconfigPaths(), react()],   // tsconfigPaths MUST come first
  test: {
    environment: 'jsdom',
    globals: false,                        // explicit imports per OQ-4 resolution
    include: ['tests/unit/**/*.test.{ts,tsx}'],
    exclude: ['node_modules', '.next', 'out', 'tests/e2e', 'tests/postbuild'],
    environmentMatchGlobs: [
      ['tests/unit/accuracy*', 'node'],    // pure file-walk, no DOM needed
    ],
  },
})
```

**Key constraints:**
- `globals: false` (OQ-4 RESOLVED): no Biome test domain available; use explicit `import { describe, it, expect } from 'vitest'`
- `/// <reference types="vitest/config" />` prevents `tsc --noEmit` failures when tsconfig picks up this `.mts` file
- `environmentMatchGlobs` for `accuracy.test.ts` overrides the top-level `jsdom` env so pure Node file I/O works without jsdom overhead

---

### `web/playwright.config.ts` (config, request-response)

**Analog:** `web/tests/home_spec.py` lines 46-64 (server start + PORT 4199 pattern)

**Server pattern from analog** (`home_spec.py` lines 59-64, 110):
```python
PORT = 4199
server = http.server.HTTPServer(("127.0.0.1", PORT), SilentHandler)
base_url = f"http://127.0.0.1:{PORT}/"
```

**JS equivalent** (from RESEARCH §Pattern 2 + Pitfall 3 fix):
```typescript
// web/playwright.config.ts
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
    // Pin port explicitly — `pnpm dlx serve out` without port picks 3000 (Pitfall 3)
    command: 'pnpm dlx serve out --listen 4199',
    url: 'http://127.0.0.1:4199',
    reuseExistingServer: !process.env.CI,
    timeout: 15000,
  },
  outputDir: 'playwright-results',
})
```

**Import pattern:** `import { test, expect } from '@playwright/test'` in every spec (not globals — OQ-4 resolution).

---

### `web/tests/unit/utils.test.ts` (test, transform)

**Analog:** `web/lib/utils.ts` (full file, 6 lines)

**Real signature** (`web/lib/utils.ts` lines 1-6):
```typescript
import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

**Test pattern** (from RESEARCH §Code Examples):
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

**No `@vitest-environment` directive needed** — runs in default jsdom env (pure function, no DOM use).

---

### `web/tests/unit/reduced-motion.test.tsx` (test, event-driven)

**Analog:** `web/lib/reduced-motion.tsx` (full file, 48 lines)

**Real exports** (`web/lib/reduced-motion.tsx` lines 13, 46):
```typescript
export function ReducedMotionProvider({ children }: { children: React.ReactNode })
export function useReducedMotion(): boolean
```

**Context default** (`web/lib/reduced-motion.tsx` line 9-11):
```typescript
const ReducedMotionContext = createContext<ReducedMotionContextValue>({
  prefersReducedMotion: false,
});
```

**matchMedia dependency** (`web/lib/reduced-motion.tsx` lines 20-22):
```typescript
const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
setPrefersReducedMotion(mq.matches);
```
jsdom does not implement `window.matchMedia` — test must mock it before render.

**Test pattern** (from RESEARCH §Code Examples):
```typescript
// web/tests/unit/reduced-motion.test.tsx
// No @vitest-environment directive — jsdom is the top-level default
import { describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { ReducedMotionProvider, useReducedMotion } from '@/lib/reduced-motion'

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
  it('returns true when prefers-reduced-motion: reduce', () => {
    mockMatchMedia(true)
    const { result } = renderHook(() => useReducedMotion(), {
      wrapper: ReducedMotionProvider,
    })
    expect(result.current).toBe(true)
  })
})
```

---

### `web/tests/unit/install-chip.test.tsx` (test, request-response)

**Analog:** `web/components/home/install-chip.tsx` (full file, 134 lines)

**Real component signature** (`install-chip.tsx` line 22):
```typescript
export function InstallChip()    // no required props
```

**Install command constant** (`install-chip.tsx` lines 11-12):
```typescript
const INSTALL_COMMAND =
  "go install github.com/bantuson/beekeeper/cmd/beekeeper@latest";
```

**Copy button aria-label** (`install-chip.tsx` lines 93-97):
```typescript
aria-label={
  copied
    ? "Install command copied"
    : "Copy install command to clipboard"
}
```

**Test pattern:**
```typescript
// web/tests/unit/install-chip.test.tsx
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { InstallChip } from '@/components/home/install-chip'

describe('InstallChip', () => {
  it('renders the install command text', () => {
    render(<InstallChip />)
    expect(screen.getByText(/go install github\.com\/bantuson\/beekeeper/)).toBeTruthy()
  })
  it('copy button has accessible label', () => {
    render(<InstallChip />)
    expect(screen.getByRole('button', { name: 'Copy install command to clipboard' })).toBeTruthy()
  })
})
```

**Note:** `navigator.clipboard` is not available in jsdom by default. The component's `.catch()` guard means a clipboard mock is NOT required for the above assertions (they don't trigger copy). If testing the copy interaction, use `vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })`.

---

### `web/tests/unit/metadata.test.ts` (test, transform)

**Analog:** `web/lib/metadata.ts` (full file, 15 lines)

**Real exports** (`web/lib/metadata.ts` lines 14-15):
```typescript
export const BASE_URL = "https://beekeeper.vercel.app";
export const SITE_NAME = "Beekeeper";
```

**Test pattern:**
```typescript
// web/tests/unit/metadata.test.ts
// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { BASE_URL, SITE_NAME } from '@/lib/metadata'

describe('metadata constants', () => {
  it('BASE_URL is the locked Vercel URL', () => {
    expect(BASE_URL).toBe('https://beekeeper.vercel.app')
  })
  it('SITE_NAME is Beekeeper', () => {
    expect(SITE_NAME).toBe('Beekeeper')
  })
})
```

---

### `web/tests/unit/accuracy.test.ts` (test, file-I/O)

**Analog:** `web/tests/accuracy_spec.py` (full file, 155 lines) — direct port

**Python constants to port** (`accuracy_spec.py` lines 42-55):
```python
UNENFORCED_FEATURES = ["release_age", "minimumReleaseAge", "lifecycle_script_allowlist"]
UNENFORCED_LABELS = ["unenforced", "not enforced", "<UnenforcedCallout"]
PHANTOM_COMMANDS = [
    "beekeeper hooks status",
    "beekeeper catalogs rebuild",
    "beekeeper check --input",
    "beekeeper status",
]
```

**Path resolution** (`accuracy_spec.py` lines 35-38):
```python
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
WEB_DIR = os.path.dirname(SCRIPT_DIR)
REPO_ROOT = os.path.dirname(WEB_DIR)
CONTENT_DIR = pathlib.Path(WEB_DIR) / "content" / "docs"
```

**Frontmatter extraction** (`accuracy_spec.py` lines 78-81):
```python
def frontmatter_block(text: str) -> str:
    m = re.match(r"^---\s*\n(.*?)\n---\s*\n", text, re.DOTALL)
    return m.group(1) if m else ""
```

**JS port pattern:**
```typescript
// web/tests/unit/accuracy.test.ts
// @vitest-environment node
import { readdirSync, readFileSync, existsSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'
import { describe, expect, it } from 'vitest'

const __dirname = dirname(fileURLToPath(import.meta.url))
const WEB_DIR = join(__dirname, '../..')
const REPO_ROOT = join(WEB_DIR, '..')
const CONTENT_DIR = join(WEB_DIR, 'content', 'docs')

const UNENFORCED_FEATURES = ['release_age', 'minimumReleaseAge', 'lifecycle_script_allowlist']
const UNENFORCED_LABELS = ['unenforced', 'not enforced', '<UnenforcedCallout']
const PHANTOM_COMMANDS = [
  'beekeeper hooks status',
  'beekeeper catalogs rebuild',
  'beekeeper check --input',
  'beekeeper status',
]

function mdxFiles(): string[] {
  return readdirSync(CONTENT_DIR, { recursive: true, withFileTypes: true })
    .filter(e => e.isFile() && e.name.endsWith('.mdx'))
    .map(e => join(e.parentPath, e.name))
}

function frontmatterBlock(text: string): string {
  const m = /^---\s*\n([\s\S]*?)\n---\s*\n/.exec(text)
  return m ? m[1] : ''
}

describe('AC-1: source_doc frontmatter', () => {
  for (const file of mdxFiles()) {
    it(`${file} has source_doc pointing at real paths`, () => {
      const text = readFileSync(file, 'utf-8')
      const fm = frontmatterBlock(text)
      const m = /^source_doc:\s*(.+?)\s*$/m.exec(fm)
      expect(m, `MISSING source_doc: in ${file}`).toBeTruthy()
      const raw = m![1].trim().replace(/^["']|["']$/g, '')
      expect(raw).not.toBe('')
      for (const ref of raw.split(',').map(s => s.trim()).filter(Boolean)) {
        expect(existsSync(join(REPO_ROOT, ref)), `source_doc path missing: ${ref}`).toBe(true)
      }
    })
  }
})

describe('AC-2: unenforced features are labeled', () => { /* ... */ })
describe('AC-3: no phantom commands', () => { /* ... */ })
```

---

### `web/tests/postbuild/seo.test.ts` (test, file-I/O, post-build)

**Analog:** `web/tests/seo_spec.py` (full file, 164 lines) — direct port

**Key difference from analog:** placed in `tests/postbuild/` (separate from `tests/unit/`) to enforce the OQ-1 resolution — CI runs this AFTER `pnpm build`, not in the pre-build unit stage. Configure a separate `"test:postbuild": "vitest run tests/postbuild"` script in package.json.

**Python path/constant pattern** (`seo_spec.py` lines 33-36):
```python
WEB_DIR = os.path.dirname(SCRIPT_DIR)
OUT_DIR = pathlib.Path(WEB_DIR) / "out"
BASE_URL = "https://beekeeper.vercel.app"  # keep in sync with web/lib/metadata.ts
```

**Python SC-1 assertion** (`seo_spec.py` lines 63-81):
```python
for path in content_html_files():
    content = path.read_text(encoding="utf-8")
    if not re.search(r"<title>[^<]+</title>", content): fail(...)
    if not re.search(r'<meta name="description" content="[^"]+"', content): fail(...)
    if not re.search(rf'<link rel="canonical" href="{re.escape(BASE_URL)}/', content): fail(...)
```

**Python SC-3 sitemap assertion** (`seo_spec.py` lines 104-131):
```python
url_count = len(re.findall(r"<loc>", sitemap))
if url_count >= 13: ok(...)
bad_urls = [u for u in re.findall(r"<loc>([^<]+)</loc>", sitemap) if not u.startswith(BASE_URL)]
```

**JS port pattern:**
```typescript
// web/tests/postbuild/seo.test.ts
// @vitest-environment node
import { readdirSync, readFileSync, existsSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'
import { describe, expect, it, beforeAll } from 'vitest'

const __dirname = dirname(fileURLToPath(import.meta.url))
const WEB_DIR = join(__dirname, '../..')
const OUT_DIR = join(WEB_DIR, 'out')
// Import BASE_URL from source rather than duplicating the Python mirror coupling:
import { BASE_URL } from '../../lib/metadata'

beforeAll(() => {
  if (!existsSync(OUT_DIR)) {
    throw new Error('out/ not found — run pnpm build first (test:postbuild requires built out/)')
  }
})

function htmlFiles(): string[] {
  return readdirSync(OUT_DIR, { recursive: true, withFileTypes: true })
    .filter(e => e.isFile() && e.name === 'index.html')
    .map(e => join(e.parentPath, e.name))
    .filter(f => !f.includes('404') && !f.includes('_not-found'))
}
```

**Key improvement over Python analog:** import `BASE_URL` directly from `@/lib/metadata` (or relative `../../lib/metadata`) instead of duplicating the string constant — eliminates the Python spec's "keep in sync" coupling point noted in `seo_spec.py` line 36.

---

### `web/tests/e2e/home.spec.ts` (test, request-response)

**Analog:** `web/tests/home_spec.py` (full file, 250 lines) — direct port

**Harness names list** (`home_spec.py` lines 70-86) — copy verbatim:
```python
HARNESS_NAMES = [
    "Claude Code", "Codex", "Cursor", "Augment", "CodeBuddy",
    "Qwen Code", "Gemini CLI", "Copilot", "Antigravity", "Windsurf",
    "Hermes", "Cline", "OpenCode", "Kilo", "Trae",
]
KNOWN_GAP_MARKERS = ["release_age", "0.0.0.0", "fail-OPEN", "UNGUARDED"]
```

**Python Test 1 — above-fold assertions** (`home_spec.py` lines 122-156):
```python
page = browser.new_page(viewport={"width": 1280, "height": 800})
page.goto(base_url, wait_until="networkidle")
headline = page.locator("text=autonomous coding agents").first
install_chip = page.locator("text=go install github.com").first
bb = install_chip.bounding_box()
if bb and (bb["y"] + bb["height"]) <= 800: ok(...)
read_docs = page.locator("a", has_text="Read the docs").first
```

**Python Test 2 — dual-theme proof** (`home_spec.py` lines 168-199):
```python
page.evaluate("() => { document.documentElement.classList.remove('light'); document.documentElement.classList.add('dark'); }")
page.wait_for_timeout(150)
dark_bg = page.evaluate("() => window.getComputedStyle(document.body).backgroundColor")
```

**JS port pattern** (from RESEARCH §Porting Map + §Code Examples):
```typescript
// web/tests/e2e/home.spec.ts
import { expect, test } from '@playwright/test'

const HARNESS_NAMES = [
  'Claude Code', 'Codex', 'Cursor', 'Augment', 'CodeBuddy',
  'Qwen Code', 'Gemini CLI', 'Copilot', 'Antigravity', 'Windsurf',
  'Hermes', 'Cline', 'OpenCode', 'Kilo', 'Trae',
]
const KNOWN_GAP_MARKERS = ['release_age', '0.0.0.0', 'fail-OPEN', 'UNGUARDED']

test.describe('Home page', () => {
  test('hero headline + install chip + CTA are above the fold', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto('/')
    await expect(page.locator('text=autonomous coding agents').first()).toBeVisible()
    const chip = page.locator('text=go install github.com').first()
    const box = await chip.boundingBox()
    expect(box!.y + box!.height).toBeLessThanOrEqual(800)
    await expect(page.getByRole('link', { name: 'Read the docs' })).toBeVisible()
  })

  test('dark and light theme body backgrounds differ', async ({ page }) => {
    await page.goto('/')
    await page.evaluate(() => {
      document.documentElement.classList.remove('light')
      document.documentElement.classList.add('dark')
    })
    await page.waitForTimeout(150)
    const darkBg = await page.evaluate(() => window.getComputedStyle(document.body).backgroundColor)
    await page.evaluate(() => {
      document.documentElement.classList.remove('dark')
      document.documentElement.classList.add('light')
    })
    await page.waitForTimeout(150)
    const lightBg = await page.evaluate(() => window.getComputedStyle(document.body).backgroundColor)
    expect(darkBg).not.toBe(lightBg)
  })

  test('all 15 harness names in DOM', async ({ page }) => {
    await page.goto('/')
    const content = await page.content()
    for (const name of HARNESS_NAMES) {
      expect(content, `missing harness: ${name}`).toContain(name)
    }
    for (const marker of KNOWN_GAP_MARKERS) {
      expect(content, `missing gap marker: ${marker}`).toContain(marker)
    }
  })

  // QA-02 additions (not in Python spec) — D-07 critical paths:
  test('docs nav: can navigate to a docs page', async ({ page }) => {
    await page.goto('/docs/getting-started/')
    await expect(page).toHaveURL(/docs/)
  })

  test('theme toggle persists across reload', async ({ page }) => {
    await page.goto('/')
    await page.evaluate(() => {
      document.documentElement.classList.add('light')
      localStorage.setItem('theme', 'light')
    })
    await page.reload()
    const cls = await page.evaluate(() => document.documentElement.className)
    expect(cls).toContain('light')
  })

  test('all three changelog pages render headings', async ({ page }) => {
    for (const slug of ['v1.0.0', 'v1.2.0', 'v1.3.0']) {
      await page.goto(`/changelog/${slug}/`)
      await expect(page.getByRole('heading').first()).toBeVisible()
    }
  })
})
```

---

### `web/tests/e2e/gfx.spec.ts` (test, request-response)

**Analog:** `web/tests/gfx_spec.py` (full file, 262 lines) — direct port

**CRITICAL codebase fact (OQ-2 RESOLVED):** 3D canvas removed in commit `9e4a671`. GFX-01/02/04 canvas-count assertions will return 0 (which is `<= 1` — passes vacuously). The only QA-02-relevant assertion is **GFX-03: SVG fallback visible** (`img[src='/hero-hive.svg']`).

**Python GFX-01a file check** (`gfx_spec.py` lines 96-105):
```python
FORBIDDEN_SERVER_SYMBOLS = ["<canvas", "WebGLRenderingContext", "@react-three"]
with open(INDEX_HTML, "r", encoding="utf-8") as fh:
    html = fh.read()
for sym in FORBIDDEN_SERVER_SYMBOLS:
    if sym in html: fail(...)
```

**Python GFX-03 reduced-motion SVG check** (`gfx_spec.py` lines 135-154):
```python
ctx = browser.new_context(viewport={"width": 1280, "height": 800}, reduced_motion="reduce")
page = ctx.new_page()
page.goto(base_url, wait_until="networkidle")
page.wait_for_timeout(250)
rm_count = page.evaluate("document.querySelectorAll('canvas').length")
# 0 canvas expected
if page.locator("img[src='/hero-hive.svg']").is_visible(): ok(...)
```

**Python GFX-04 LCP/FCP perf eval** (`gfx_spec.py` lines 195-212):
```python
perf = page.evaluate(
    "() => {"
    "  const lcp = performance.getEntriesByType('largest-contentful-paint');"
    "  if (lcp.length) return { t: lcp[lcp.length - 1].startTime, src: 'lcp' };"
    "  const fcp = performance.getEntriesByType('paint')"
    "    .find(e => e.name === 'first-contentful-paint');"
    "  if (fcp) return { t: fcp.startTime, src: 'fcp-fallback' };"
    "  return { t: 0, src: 'none' };"
    "}"
)
```

**JS port pattern:**
```typescript
// web/tests/e2e/gfx.spec.ts
import { readFile } from 'fs/promises'
import { join } from 'path'
import { expect, test } from '@playwright/test'

const OUT_INDEX = join(process.cwd(), 'out', 'index.html')
const FORBIDDEN_SYMBOLS = ['<canvas', 'WebGLRenderingContext', '@react-three']

test.describe('GFX-01a: server HTML clean', () => {
  test('out/index.html contains no 3D symbols', async () => {
    const html = await readFile(OUT_INDEX, 'utf-8')
    for (const sym of FORBIDDEN_SYMBOLS) {
      expect(html, `forbidden symbol in server HTML: ${sym}`).not.toContain(sym)
    }
  })
})

test.describe('GFX-01b/02: canvas count after hydration', () => {
  test('canvas count <= 1 (0 expected, 3D removed)', async ({ page }) => {
    await page.goto('/')
    await page.waitForTimeout(250)
    const count = await page.evaluate(() => document.querySelectorAll('canvas').length)
    expect(count).toBeLessThanOrEqual(1)
  })
})

test.describe('GFX-03: reduced-motion SVG fallback', () => {
  test('SVG hero visible under prefers-reduced-motion: reduce', async ({ browser }) => {
    const ctx = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      reducedMotion: 'reduce',
    })
    const page = await ctx.newPage()
    await page.goto('/')
    await page.waitForTimeout(250)
    const canvasCount = await page.evaluate(() => document.querySelectorAll('canvas').length)
    expect(canvasCount).toBe(0)
    await expect(page.locator("img[src='/hero-hive.svg']")).toBeVisible()
    await ctx.close()
  })
})

test.describe('GFX-04: LCP budget + no context leak', () => {
  test('LCP/FCP < 2500ms (headless proxy)', async ({ page }) => {
    await page.goto('/')
    await page.waitForTimeout(300)
    const perf = await page.evaluate(() => {
      const lcp = performance.getEntriesByType('largest-contentful-paint') as PerformanceEntry[]
      if (lcp.length) return { t: (lcp[lcp.length - 1] as any).startTime, src: 'lcp' }
      const fcp = performance.getEntriesByType('paint').find(e => e.name === 'first-contentful-paint')
      if (fcp) return { t: fcp.startTime, src: 'fcp-fallback' }
      return { t: 0, src: 'none' }
    })
    if (perf.src !== 'none' && perf.t > 0) {
      expect(perf.t).toBeLessThan(2500)
    }
  })

  test('no context leak across navigate-away-and-back', async ({ page }) => {
    await page.goto('/')
    const canvasBefore = await page.evaluate(() => document.querySelectorAll('canvas').length)
    await page.goto('/docs/getting-started/')
    await page.goto('/')
    await page.waitForTimeout(300)
    const canvasAfter = await page.evaluate(() => document.querySelectorAll('canvas').length)
    expect(canvasAfter).toBeLessThanOrEqual(1)
    expect(canvasAfter).toBeLessThanOrEqual(Math.max(canvasBefore, 1))
  })
})
```

---

### `web/package.json` (MODIFY — scripts + devDependencies)

**Current scripts block** (lines 7-12):
```json
"scripts": {
  "postinstall": "fumadocs-mdx",
  "dev": "next dev",
  "build": "next build",
  "start": "pnpm dlx serve out",
  "lint": "biome check",
  "format": "biome format --write"
}
```

**Add these scripts** (per D-06 + OQ-1 resolution):
```json
"typecheck": "tsc --noEmit",
"test": "vitest run",
"test:watch": "vitest",
"test:postbuild": "vitest run tests/postbuild",
"test:e2e": "playwright test"
```

**Note on `start` script:** Keep `"start": "pnpm dlx serve out"` unchanged. The Playwright `webServer.command` in `playwright.config.ts` explicitly calls `pnpm dlx serve out --listen 4199` (with port pin) instead of `pnpm start` — this avoids a race if `start` is also used for other things.

**Current devDependencies block** (lines 30-38):
```json
"devDependencies": {
  "@biomejs/biome": "2.2.0",
  "@tailwindcss/postcss": "^4",
  "@types/node": "^20",
  "@types/react": "^19",
  "@types/react-dom": "^19",
  "shadcn": "^4.10.0",
  "tailwindcss": "^4",
  "typescript": "^5"
}
```

**Add pinned devDependencies** (per D-06 + RESEARCH §Standard Stack):
```json
"@playwright/test": "1.57.0",
"@testing-library/dom": "10.4.1",
"@testing-library/react": "16.3.2",
"@vitejs/plugin-react": "6.0.2",
"jsdom": "29.1.1",
"vite": "8.0.16",
"vite-tsconfig-paths": "6.1.1",
"vitest": "4.1.8"
```

---

### `web/biome.json` (MODIFY — only if lint fails on test files)

**Current `linter.domains` block** (lines 25-28):
```json
"domains": {
  "next": "recommended",
  "react": "recommended"
}
```

**OQ-4 RESOLVED:** Do NOT add a `"test"` domain (unverified for Biome 2.2.0). Because `globals: false` is set in vitest.config.mts and specs use explicit `import { test, expect } from '@playwright/test'` and `import { describe, it, expect } from 'vitest'`, there are no undeclared globals for Biome to flag. The `biome.json` should NOT be modified unless `pnpm lint` actually fails on a specific test file. If it does, add a point-of-use `// biome-ignore lint/...` comment at the specific line.

---

### `.github/workflows/web.yml` (NEW)

**Analog:** `.github/workflows/ci.yml` (Actions style, step structure, pinned action versions)

**ci.yml Actions versions to mirror** (lines 16, 19, 21):
```yaml
uses: actions/checkout@v4
uses: actions/setup-go@v5        # → use actions/setup-node@v4 equivalent
```

**ci.yml job structure to mirror** (lines 9-12):
```yaml
jobs:
  test:
    strategy:
      fail-fast: false
    runs-on: ${{ matrix.os }}   # → single ubuntu-latest for web job
```

**Full web.yml pattern** (from RESEARCH §Pattern 3):
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
          version: 11.1.3        # matches web/package.json packageManager field

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'     # matches local Node 22.17.0
          cache: 'pnpm'          # reads repo-root pnpm-lock.yaml (workspace lockfile)

      - name: Install dependencies
        run: pnpm install --frozen-lockfile

      - name: Install Playwright browsers
        run: pnpm exec playwright install chromium --with-deps
        # Do NOT cache — restore time equals download time (official PW docs)
        # --with-deps installs libgbm/libnss system dependencies on ubuntu-latest

      - name: Lint + format (Biome)
        run: pnpm lint

      - name: Type-check
        run: pnpm typecheck

      - name: Unit tests (Vitest)
        run: pnpm test

      - name: Build (static export)
        run: pnpm build

      - name: Post-build tests (SEO file-walk)
        run: pnpm test:postbuild

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

**Step order enforces QA-01:** lint → typecheck → unit → build → postbuild → e2e.

---

### `.github/workflows/ci.yml` (MODIFY — add paths-ignore)

**Current `on:` block** (`ci.yml` lines 3-7):
```yaml
on:
  pull_request:
  push:
    branches: [main]
```

**Replace with** (D-04 bidirectional isolation):
```yaml
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

**This is the ONLY change to `ci.yml`.** All jobs (`test`, `fuzz`, `fuzz-ipc`, `fuzz-llamafirewall`, `test-sentry-kernel-5-4`, `test-sentry-kernel-5-15`, `test-eslogger-fields`, `release-gate`) remain unchanged.

---

## Shared Patterns

### Vitest explicit imports (OQ-4 resolution)
**Apply to:** All `tests/unit/**/*.test.{ts,tsx}` and `tests/postbuild/**/*.test.ts`
```typescript
import { describe, it, expect, beforeAll, vi } from 'vitest'
```
Never rely on globals — `globals: false` in vitest.config.mts means Biome won't see undeclared test symbols.

### Playwright explicit imports (OQ-4 resolution)
**Apply to:** All `tests/e2e/**/*.spec.ts`
```typescript
import { test, expect } from '@playwright/test'
```

### Node environment directive for file-walk tests
**Apply to:** `tests/unit/accuracy.test.ts`, `tests/unit/metadata.test.ts`, `tests/postbuild/seo.test.ts`
```typescript
// @vitest-environment node
```
Place at the top of the file (first line, before imports). Overrides the top-level `jsdom` environment set in vitest.config.mts.

### `__dirname` in ESM test files
**Apply to:** All Vitest node-environment tests that use `__dirname` or `__filename`
```typescript
import { dirname } from 'path'
import { fileURLToPath } from 'url'
const __dirname = dirname(fileURLToPath(import.meta.url))
```
Required because `web/tsconfig.json` targets `"module": "esnext"` (ESM) — `__dirname` is not automatically defined.

### Path alias usage
**Apply to:** All unit test files importing project source
```typescript
// Use @/ alias for project modules (resolved by vite-tsconfig-paths):
import { cn } from '@/lib/utils'
import { InstallChip } from '@/components/home/install-chip'
import { BASE_URL } from '@/lib/metadata'
// For postbuild tests that need relative paths to out/:
import { join, dirname } from 'path'
const OUT_DIR = join(__dirname, '../../out')
```

---

## No Analog Found

All files have close analogs. No entries in this section.

---

## Metadata

**Analog search scope:** `web/tests/`, `web/lib/`, `web/components/home/`, `web/package.json`, `web/tsconfig.json`, `web/biome.json`, `.github/workflows/`
**Files scanned:** 14
**Pattern extraction date:** 2026-06-10
