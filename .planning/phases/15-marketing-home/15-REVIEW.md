---
phase: 15-marketing-home
reviewed: 2026-06-08T12:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - web/app/page.tsx
  - web/components/home/section.tsx
  - web/components/home/site-header.tsx
  - web/components/home/site-footer.tsx
  - web/components/home/install-chip.tsx
  - web/components/home/hero.tsx
  - web/components/home/origin-story.tsx
  - web/components/home/how-it-works.tsx
  - web/components/home/feature-cards.tsx
  - web/components/home/harness-matrix.tsx
  - web/components/home/honesty-callout.tsx
  - web/tests/home_spec.py
findings:
  critical: 2
  warning: 5
  info: 4
  total: 11
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-06-08T12:00:00Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Phase 15 delivers the marketing home page for the Beekeeper static-export site (Next.js 16, Tailwind v4, shadcn). The overall structure is sound: server components are used by default, the single client boundary (`install-chip.tsx`) is appropriate, raw theme tokens are used correctly throughout, and the `.catch()` on the clipboard write is properly implemented.

Two critical issues were found. First, the `site-header.tsx` hardcodes a dark-only `rgba(10,13,18,0.85)` backdrop color that does not switch under the light theme, making the header background invisible-on-invisible in light mode — a direct violation of the Phase-12 dual-theme freeze. Second, the `origin-story.tsx` table header renders the label "Live threat catalog" directly above text that says "Documented 2026 campaigns", creating an internally contradictory content claim on a security product's public page: "Live" implies real-time feed updates that are explicitly not present, which is a content-accuracy defect under D-03.

Five warnings cover: a TypeScript identifier collision in `harness-matrix.tsx` (the `HarnessRow` interface and the `HarnessRow` function component share the exact same name — TypeScript resolves it, but it is a latent readability trap); a misplaced `import type React` at line 86 of `origin-story.tsx` that should appear at the top of the file alongside other imports; the Playwright test's Test 4 reusing `page_text` from Test 3 (the page is never re-fetched after a `.close()` on a different page object, creating a correctness gap in the test harness); a magic hardcoded `time.sleep(0.3)` in the test with no comment explaining what condition is being waited for; and the `HowItWorks` composite export rendering two adjacent `<Section>` elements each with their own `<SectionHead>`, but the Nav link "Docs → /docs/getting-started" exists both in the header and footer with no corresponding active-link state, producing a minor UX inconsistency.

Content accuracy cross-check against `docs/harness-support-matrix.md` and `docs/THREAT-MODEL.md §8` was performed exhaustively:

- All 15 harnesses are present with correct tier assignments.
- Tier-2 Hermes fail-OPEN caveat is faithfully reproduced.
- Tier-3 Kilo/Trae UNGUARDED caveat is faithfully reproduced.
- `honesty-callout.tsx` four gaps match §8 verbatim (Hermes fail-OPEN, Tier-3 unguarded, `release_age`/`lifecycle_script_allowlist` unenforced, `--bind 0.0.0.0` plaintext exposure).
- `feature-cards.tsx` covers exactly the six shipped capabilities with no aspirational claims.
- Corroboration semantics (1 warn / 2 block / 3 quarantine) are stated correctly.
- The `install-chip.tsx` footnote claim "Reproducible builds · Sigstore signed · SLSA L3 provenance" is accurate per `docs/THREAT-MODEL.md §2`.

---

## Critical Issues

### CR-01: Hardcoded Dark-Only RGBA in SiteHeader Backdrop Breaks Light Theme

**File:** `web/components/home/site-header.tsx:23`

**Issue:** The header's `background` is hardcoded to `rgba(10,13,18,0.85)` — the exact dark-theme `--bg` value (`#0a0d12`). Under the `.light` theme, `--bg` resolves to `#f8f9fa` (near-white), but the header continues to render dark charcoal. The result is dark nav text rendered against a dark charcoal backdrop in light mode — near-zero contrast. This directly violates the Phase-12 dual-theme freeze rule that prohibits dark-only hardcoded values and mandates all styling go through raw theme tokens.

The comment block at lines 15–16 of the same file explicitly states "Token rule: style with raw var(--*) tokens only; NEVER --color-bk-* (dark-only)" — the implementation violates its own stated rule.

**Fix:**
```tsx
// Replace:
background: "rgba(10,13,18,0.85)",

// With (uses the theme-switched --bg token at 85% opacity):
background: "color-mix(in srgb, var(--bg) 85%, transparent)",
// OR, for backdrop-blur chips where true transparency is needed over the page:
// background: "oklch(from var(--bg) l c h / 0.85)",
// The simplest correct form that works cross-browser and switches themes:
background: "rgb(from var(--bg) r g b / 0.85)",
```

If the `rgb()` relative-color syntax has insufficient browser coverage targets, use a CSS variable approach:
```css
/* In globals.css, add to :root and .light: */
:root { --header-bg-alpha: rgba(10,13,18,0.85); }
.light { --header-bg-alpha: rgba(248,249,250,0.92); }
```
```tsx
background: "var(--header-bg-alpha)",
```

---

### CR-02: "Live threat catalog" Label Contradicts "Documented 2026 campaigns" — Inaccurate Content Claim

**File:** `web/components/home/origin-story.tsx:153`

**Issue:** The threat table section header reads `"Live threat catalog"` (line 153). The very next element — a badge rendered two lines later at line 163 — reads `"Documented 2026 campaigns"`. These labels directly contradict each other: "Live threat catalog" implies a continuously-updated real-time feed, while "Documented 2026 campaigns" correctly acknowledges these are static historical records. The code comment at line 155 says `{/* Static label — no fake live-sync claim */}`, but the heading itself (`"Live threat catalog"`) is itself the fake live claim that the badge was added to disclaim.

For a security product whose marketing page is under explicit D-03 accuracy constraints, publishing "Live threat catalog" as a section heading — even when immediately qualified — creates a misleading first impression that a visitor skimming the page will take at face value. The spec comment notes the mockup's "synced 6 minutes ago" pulse animation was intentionally dropped to avoid overclaiming a live feed. The heading must be corrected for the same reason.

**Fix:**
```tsx
// Replace:
<span className="text-[13px] font-semibold" style={{ color: "var(--fg-strong)" }}>
  Live threat catalog
</span>

// With an accurate label that matches the badge text and D-03:
<span className="text-[13px] font-semibold" style={{ color: "var(--fg-strong)" }}>
  Documented threat catalog
</span>
// Or: "2026 threat catalog" / "Threat intel catalog"
// Any label that does not imply real-time updates the component cannot deliver.
```

---

## Warnings

### WR-01: `HarnessRow` Interface and Component Share the Same Identifier — Naming Collision

**File:** `web/components/home/harness-matrix.tsx:21` and `:195`

**Issue:** The TypeScript interface `HarnessRow` (line 21) and the React function component `HarnessRow` (line 195) share the exact same identifier name in the same module scope. TypeScript resolves value vs. type namespaces separately so this compiles without error, but it creates an immediate readability hazard: any reader scanning the file sees `HarnessRow` used in three different contexts (as a type annotation, as an array element type, and as a JSX tag) with no disambiguation. The data objects in `TIER1`/`TIER2`/`TIER3` are typed as `HarnessRow[]`, and the same name is the component that renders them — a future editor is likely to conflate the two.

**Fix:**
```tsx
// Rename either the interface or the component. Prefer renaming the component
// since interface names tend to drive prop types across files:

// Option A — rename component:
function HarnessCard({ harness }: { harness: HarnessRow }) { ... }
// Update all three call sites in TierGroup: <HarnessCard key={h.name} harness={h} />

// Option B — rename interface (less invasive if the interface is referenced externally):
interface HarnessEntry { ... }
const TIER1: HarnessEntry[] = [...]
function HarnessRow({ harness }: { harness: HarnessEntry }) { ... }
```

---

### WR-02: `import type React` Is Misplaced (After Runtime Code) in origin-story.tsx

**File:** `web/components/home/origin-story.tsx:86`

**Issue:** The file uses `React.CSSProperties` (in the `sevStyle` type annotation at line 75) before the `import type React from "react"` at line 86. In TypeScript with `isolatedModules` (which Next.js enforces), an `import type` that is referenced before its declaration is a logic error that works today only because TypeScript processes imports before type resolution — but the ordering is confusing and fragile. More importantly, `React.CSSProperties` should be imported directly as `import type { CSSProperties } from "react"` to avoid pulling in the entire React namespace type for a single property type. The import being after the runtime code it annotates also means any linter or future bundler may flag the ordering as a violation.

**Fix:**
```tsx
// At the top of the file, alongside the existing import:
import type { CSSProperties } from "react";

// Replace the late import:
// DELETE line 86: import type React from "react";

// Update the type annotation at line 75:
const sevStyle: Record<"CRIT" | "HIGH", CSSProperties> = { ... };
```

---

### WR-03: Playwright Test 4 Uses Stale `page_text` After `page.close()` in Test 3

**File:** `web/tests/home_spec.py:220-225`

**Issue:** The Test 4 loop at line 220 iterates over `KNOWN_GAP_MARKERS` and checks `if marker in page_text`. The variable `page_text` is assigned at line 209 inside Test 3's `page` context. Test 3's `page.close()` is called at line 227, but Test 4 (lines 219–225) executes before that close and shares the same `page` and `page_text` as Test 3. This is currently functionally correct — Test 4 runs while Test 3's page is still open — but the test structure makes it look like the gap-marker check runs independently, when it is actually co-located inside the same page instance by accident. If someone moves the Test 4 block after the `page.close()` (a reasonable refactor given the `[Test 4]` print label), it will silently use the last value of `page_text` from a closed page, which in Python Playwright returns the page's last DOM snapshot as a string (not an error), so the test will appear to pass even if the page is not navigated. The test should be made explicitly self-contained.

**Fix:**
```python
# Test 4: re-navigate with its own page so it is self-contained
print("\n[Test 4] Four known-gap markers in rendered DOM")
page4 = browser.new_page(viewport={"width": 1280, "height": 800})
page4.goto(base_url, wait_until="networkidle")
page4_text = page4.content()
for marker in KNOWN_GAP_MARKERS:
    if marker in page4_text:
        ok(f"Known-gap marker found: {marker}")
    else:
        fail(f"Known-gap marker MISSING from DOM: {marker}")
page4.close()
```

---

### WR-04: `time.sleep(0.3)` Magic Value With No Stated Condition

**File:** `web/tests/home_spec.py:108`

**Issue:** The server start function is followed immediately by `time.sleep(0.3)` with the comment `# let server bind`. A fixed 300ms sleep is a flaky test pattern: on an overloaded CI runner it may not be enough; on a fast developer machine it is always unnecessary. The `HTTPServer.__init__` call completes synchronously (the socket is bound by the time `__init__` returns), so the sleep guards against nothing that the `threading.Thread.start()` is responsible for.

**Fix:**
```python
def start_server():
    server = http.server.HTTPServer(("127.0.0.1", PORT), SilentHandler)
    server.daemon_threads = True
    t = threading.Thread(target=server.serve_forever, daemon=True)
    t.start()
    # Verify the socket is actually accepting before returning, no arbitrary sleep:
    import socket as _socket
    for _ in range(20):
        try:
            with _socket.create_connection(("127.0.0.1", PORT), timeout=0.1):
                break
        except OSError:
            time.sleep(0.05)
    return server

# Then remove the time.sleep(0.3) at line 108.
```

---

### WR-05: SiteHeader `<nav>` Has No Mobile Fallback — Navigation Invisible on Small Screens

**File:** `web/components/home/site-header.tsx:65-90`

**Issue:** The primary nav is wrapped in `className="ml-10 hidden items-center gap-7 md:flex"` — it is hidden below the `md` breakpoint (768px) and there is no hamburger menu, sheet, or any alternative navigation provided. The marketing page has three nav destinations (Home, Docs, Changelog). Below 768px, users see only the logo with no navigation affordance. The footer navigation remains accessible on mobile (no `hidden` class), but the header navigation is completely missing, which is a usability and accessibility issue: keyboard users tabbing through the header encounter a logo link and then immediately the main content skip link, with no way to reach Docs or Changelog from the header on any mobile viewport. The project spec's plan notes the ThemeToggle was dropped, but does not list mobile nav suppression as an accepted omission.

**Fix (minimal):** Add a visually-hidden but focus-visible fallback, or convert to a disclosure button:
```tsx
{/* Add alongside the desktop nav — visible only on mobile as stacked links: */}
<nav className="ml-auto flex items-center gap-5 md:hidden" aria-label="Mobile navigation">
  <Link href="/docs/getting-started" className="text-sm" style={{ color: "var(--dim)" }}>Docs</Link>
  <Link href="/changelog" className="text-sm" style={{ color: "var(--dim)" }}>Changelog</Link>
</nav>
```
A full hamburger drawer is optional for Phase 15 scope, but the complete absence of navigation below `md` should be documented as a known omission in the component comment if it is intentional.

---

## Info

### IN-01: `Section` Component Uses Hardcoded Pixel Values for Padding Instead of Tokens

**File:** `web/components/home/section.tsx:17-20`

**Issue:** `paddingTop: "80px"` and `paddingBottom: "80px"` are hardcoded numbers. The design system's `globals.css` already defines token-izable spacing, and the mockup uses a consistent 80px section padding throughout. If the spacing value ever changes, it must be updated in every consumer. A CSS variable (e.g. `--section-py: 80px`) would make this a one-place change. This is a mild quality issue, not a bug.

**Fix:**
```css
/* Add to globals.css :root: */
--section-py: 80px;
```
```tsx
paddingTop: "var(--section-py)",
paddingBottom: "var(--section-py)",
```

---

### IN-02: `SectionHead` `maxWidth: "680px"` Is a Magic Number Not Tied to a Token

**File:** `web/components/home/section.tsx:54`

**Issue:** The `maxWidth: "680px"` on the `SectionHead` centering div is a magic value used in one place with no token backing. Similarly `maxWidth: "880px"` in hero.tsx (line 55) and `maxWidth: "620px"` (hero.tsx line 66) are all one-off magic widths. None of these are currently causing bugs, but they diverge from the token discipline applied to `--max-content-width`.

**Fix:** Either document these as intentional fixed widths with an inline comment or add `--lede-max-width`, `--headline-max-width` tokens to the design system for consistency.

---

### IN-03: `origin-story.tsx` Table Footer Uses `<a>` Tag, Not Next.js `<Link>` for Internal Route

**File:** `web/components/home/origin-story.tsx:222`

**Issue:** The footer anchor `<a href="/docs/security">` is a raw HTML anchor, not a Next.js `<Link>`. For a static-export site this functions correctly (both produce `<a>` in the output), but it is inconsistent with every other internal navigation in the same codebase (`site-header.tsx`, `site-footer.tsx`, `hero.tsx` all use `Link`). The same inconsistency exists in `honesty-callout.tsx` line 105 (`<a href="/docs/security">`) and `harness-matrix.tsx` line 325 (`<a href="/docs/integration">`). Raw `<a>` tags for internal routes bypass Next.js's prefetching and client-side routing. On static export this is low-impact, but it sets an inconsistent pattern.

**Fix:**
```tsx
import Link from "next/link";
// Replace each internal <a href="..."> with <Link href="..."> using the same className/style props.
```
Affected lines: `origin-story.tsx:222`, `honesty-callout.tsx:105`, `harness-matrix.tsx:325`.

---

### IN-04: `hero.tsx` Uses the Command Key Symbol (⌘) as a Decorative Icon on a Non-Mac Context

**File:** `web/components/home/hero.tsx:97`

**Issue:** The "Read the docs" CTA button uses `&#8984;` (⌘, the macOS Command key symbol) as a decorative prefix icon. This is a stylistic choice from the mockup, but on Windows and Linux it is a meaningless symbol that has no keyboard equivalent. It is rendered with `aria-hidden="true"` (correct), but the symbol itself implies "keyboard shortcut" to macOS users who may look for a corresponding keyboard binding that does not exist. On non-Mac platforms it simply looks like an unfamiliar character. Since it is `aria-hidden` and purely cosmetic, this is low severity — but worth flagging as a potential confusing affordance on a cross-platform security tool marketed to developers on all OSes.

**Fix:** Replace with a more universally understood icon (e.g., `→` arrow or a book SVG icon), or keep ⌘ and add a tooltip clarifying it is decorative. Removing it entirely also works — the CTA text is self-explanatory.

---

_Reviewed: 2026-06-08T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
