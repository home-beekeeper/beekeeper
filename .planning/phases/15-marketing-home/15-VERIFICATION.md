---
phase: 15-marketing-home
verified: 2026-06-08T00:00:00Z
status: passed
score: 5/5 must-haves verified
overrides_applied: 0
re_verification: false
---

# Phase 15: Marketing Home Verification Report

**Phase Goal:** A visitor sees a complete marketing home page — hero with dual CTA, origin story, how-it-works, feature highlights for shipped capabilities, the 15-harness support matrix, and an honesty callout — server-rendered static content with a static SVG hero (no 3D yet).
**Verified:** 2026-06-08
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC-1 | Hero displays headline, subhead, copyable `go install` chip, and "Read the docs" link — all above the fold @1280px | VERIFIED | `hero.tsx` renders h1 "autonomous coding agents" with amber accent, subhead constrained to 620px, `<InstallChip />`, and `<Link href="/docs/getting-started">Read the docs</Link>`. install-chip.tsx has canonical INSTALL_COMMAND const, navigator.clipboard.writeText with `.catch()` no-false-success. out/index.html contains both strings. Playwright Test 1 confirmed above-fold (install chip y+h=577, docs link y+h=715, both ≤800px). |
| SC-2 | The Nx Console compromise origin story + a 3-step how-it-works flow | VERIFIED | `origin-story.tsx` contains "Nx Console", "nrwl.angular-console 18.95.0", "2026-05-18", "18-minute exposure", "~3,800 GitHub repos". `how-it-works.tsx` exports `HowItWorks` composing `TwoLayers` (Reactive/Proactive cards) and `Quickstart` (3 numbered steps with full canonical commands). No truncated install path. |
| SC-3 | Feature cards cover ONLY the six SHIPPED capabilities — no aspirational claims | VERIFIED | `feature-cards.tsx` CARDS array has exactly 6 entries by name: "Corroboration engine", "Fail-closed hooks", "Editor-extension defense", "Sentry", "LlamaFirewall", "Policy-as-code". The mockup's aspirational cards ("Calm-mode dashboard", "Steer toward hardened tooling", "Open from day one") are absent from the card titles array. out/index.html confirmed to contain all 6 names and have no "90-second demo" or "2.4k" strings. |
| SC-4 | 15-harness support matrix with honest tier labels + live-verification caveats, linking to /docs/integration | VERIFIED | `harness-matrix.tsx` lists all 15 harnesses in 3 tier blocks sourced verbatim from `docs/harness-support-matrix.md`. Claude Code is the only entry with `liveVerified: true` (renders `<LiveVerifiedBadge />`). Tier-3 caveat strings contain "UNGUARDED". Hermes caveat contains "fail-OPEN". Verification note callout explicitly states "All other 14 harnesses are implemented against documented contracts… documented contract, unverified locally." Links to `/docs/integration`. Content cross-checked against `docs/harness-support-matrix.md` — tiers and caveats match exactly. |
| SC-5 | Honesty / known-gaps callout sourced from docs/THREAT-MODEL.md linking to /docs/security | VERIFIED | `honesty-callout.tsx` KNOWN_GAPS array contains 4 items: (1) "Hermes is structurally fail-OPEN" with exit-codes-ignored explanation; (2) "Tier-3 harnesses: native tools UNGUARDED (Kilo, Trae)" covering MCP-only interception; (3) "release_age and lifecycle-script policy rules not enforced (v1.3.0)"; (4) "Gateway --bind 0.0.0.0 exposes the proxy over plaintext HTTP". All 4 sourced from THREAT-MODEL.md §8. Links to `/docs/security`. No softening of any gap. out/index.html confirmed to contain: "release_age", "0.0.0.0", "fail-OPEN", "UNGUARDED". |

**Score:** 5/5 truths verified

---

### Required Artifacts

| Artifact | Min Lines | Actual | Status | Notes |
|----------|-----------|--------|--------|-------|
| `web/components/home/section.tsx` | 30 | 87 | VERIFIED | Exports both `Section` and `SectionHead`; raw theme tokens throughout |
| `web/components/home/site-header.tsx` | 25 | 95 | VERIFIED | Contains `/docs/getting-started` and `/changelog`; no "2.4k"; CR-01 fix confirmed (color-mix bg) |
| `web/components/home/site-footer.tsx` | 15 | 69 | VERIFIED | Links to /docs/getting-started, /changelog, /docs/security |
| `web/components/home/install-chip.tsx` | 25 | 124 | VERIFIED | "use client"; canonical INSTALL_COMMAND const; navigator.clipboard with .catch(); no brew/scoop/docker |
| `web/components/home/hero.tsx` | 30 | 104 | VERIFIED | Headline, subhead, InstallChip, Read-the-docs CTA; no demo button |
| `web/components/home/origin-story.tsx` | 40 | 231 | VERIFIED | Nx Console narrative; CR-02 fix confirmed ("Documented threat catalog" not "Live threat catalog"); no pulse animation |
| `web/components/home/how-it-works.tsx` | 50 | 286 | VERIFIED | TwoLayers + Quickstart; full canonical install path in Step 1; honest harness caveat |
| `web/components/home/feature-cards.tsx` | 50 | 142 | VERIFIED | Exactly 6 SHIPPED capability cards by canonical names |
| `web/components/home/harness-matrix.tsx` | 50 | 334 | VERIFIED | All 15 harnesses, 3 tier groups, live-verified marker on Claude Code only, UNGUARDED Tier-3, fail-OPEN Hermes |
| `web/components/home/honesty-callout.tsx` | 30 | 116 | VERIFIED | 4 known gaps from THREAT-MODEL.md §8; coral accent (not red); /docs/security link |
| `web/tests/home_spec.py` | 30 | 250 | VERIFIED | Python Playwright; Test 1 above-fold, Test 2 both-theme, Test 3 15 harnesses, Test 4 4 gap markers |
| `web/app/page.tsx` | — | 32 | VERIFIED | Imports and renders all 8 section components in correct order |

---

### Key Link Verification

| From | To | Via | Status | Evidence |
|------|----|-----|--------|----------|
| `web/app/page.tsx` | `web/components/home/hero.tsx` | `import { Hero }` | WIRED | Line 10: `import { Hero } from "@/components/home/hero"` + renders `<Hero />` in main |
| `web/app/page.tsx` | `web/components/home/feature-cards.tsx` | `import { FeatureCards }` + render | WIRED | Line 8 import; `<FeatureCards />` in JSX |
| `web/app/page.tsx` | `web/components/home/origin-story.tsx` | `import { OriginStory }` + render | WIRED | Line 13 import; `<OriginStory />` in JSX |
| `web/app/page.tsx` | `web/components/home/harness-matrix.tsx` | `import { HarnessMatrix }` + render | WIRED | Line 9 import; `<HarnessMatrix />` in JSX |
| `web/app/page.tsx` | `web/components/home/honesty-callout.tsx` | `import { HonestyCallout }` + render | WIRED | Line 11 import; `<HonestyCallout />` in JSX |
| `web/components/home/install-chip.tsx` | `navigator.clipboard` | copy handler | WIRED | `navigator.clipboard.writeText(INSTALL_COMMAND)` with `.catch(() => { ... })` |
| `web/components/home/site-header.tsx` | `/docs/getting-started` | nav link href | WIRED | Line 79: `href="/docs/getting-started"` |
| `web/components/home/harness-matrix.tsx` | `/docs/integration` | anchor link | WIRED | `href="/docs/integration"` in integration link element |
| `web/components/home/honesty-callout.tsx` | `/docs/security` | anchor link | WIRED | `href="/docs/security"` in security-posture docs link |
| `web/components/home/feature-cards.tsx` | `web/components/home/section.tsx` | `import { Section, SectionHead }` | WIRED | `from "@/components/home/section"` |

---

### Data-Flow Trace (Level 4)

Not applicable. All content is static build-time data (server components with in-file data arrays). No dynamic data sources, no API calls, no state that could be hollow. The only client component (install-chip.tsx) writes a fixed const to clipboard on user interaction — no data source to trace.

---

### Behavioral Spot-Checks

| Behavior | Evidence | Status |
|----------|----------|--------|
| `out/index.html` contains headline | Grep confirmed "autonomous coding agents" present | PASS |
| `out/index.html` contains canonical install command | Grep confirmed full `go install github.com/bantuson/beekeeper/cmd/beekeeper@latest` present | PASS |
| `out/index.html` contains all 4 known-gap markers | Grep confirmed "release_age", "0.0.0.0", "fail-OPEN", "UNGUARDED" each present | PASS |
| `out/index.html` contains all harness matrix identifiers | Grep confirmed "Claude Code", "Kilo", "Trae", "Hermes" present | PASS |
| `out/index.html` contains doc links | Grep confirmed "docs/integration", "docs/security", "docs/getting-started" present | PASS |
| `out/index.html` absent of aspirational/inaccurate strings | Grep confirmed "Live threat catalog", "90-second demo", "2.4k" are absent | PASS |
| Playwright Test 1: above-fold @1280x800 | SUMMARY documents: install chip y+h=577, docs link y+h=715 — both ≤ 800px | PASS |
| Playwright Test 2: both-theme body background differs | SUMMARY documents: dark=rgb(10,13,18), light=rgb(248,249,250) — differ | PASS |
| Playwright Test 3: all 15 harnesses in DOM | SUMMARY documents: 15/15 PASS | PASS |
| Playwright Test 4: 4 known-gap markers in DOM | SUMMARY documents: 4/4 PASS | PASS |

---

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes defined for this phase. The phase-complete gate is `pnpm build` (emits `out/index.html`) and the Python Playwright spec (`web/tests/home_spec.py`). Both are documented as passed in 15-03-SUMMARY.md with specific output values. Step 7b behavioral spot-checks above independently verify the built output via grep.

---

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| HOME-01 | Hero with dual CTA (go install chip + Read the docs) | SATISFIED | hero.tsx + install-chip.tsx; above-fold confirmed by Playwright |
| HOME-02 | Origin/problem (Nx Console) + 3-step how-it-works | SATISFIED | origin-story.tsx + how-it-works.tsx; all narrative elements and commands verified |
| HOME-03 | Feature highlights — ONLY shipped capabilities | SATISFIED | feature-cards.tsx: exactly 6 canonical shipped capabilities; no aspirational titles |
| HOME-04 | 15-harness support matrix with honest tier/verification caveats | SATISFIED | harness-matrix.tsx: all 15, live-verified marker, UNGUARDED Tier-3, fail-OPEN Hermes; content matches source doc |
| HOME-05 | Honesty / known-gaps callout (no overclaiming) | SATISFIED | honesty-callout.tsx: 4 gaps from THREAT-MODEL.md §8; coral accent (not red); /docs/security link |
| SITE-03 | Live deploy (deferred) | DEFERRED | Explicitly scoped out of Phase 15 per D-02 (REQUIREMENTS.md): "DEFERRED out of Phase 15 (page build-verified locally; live deploy pending repo push / Vercel setup)" |

---

### Code Review Status (15-REVIEW.md)

Two criticals were confirmed remediated inline before this verification (commit 0c7a7b4):

- **CR-01 FIXED:** `site-header.tsx` background was `rgba(10,13,18,0.85)` (dark-only). Now `color-mix(in srgb, var(--bg) 85%, transparent)` — theme-switched, confirmed by grep.
- **CR-02 FIXED:** `origin-story.tsx` table header was "Live threat catalog". Now "Documented threat catalog" — confirmed by grep; no match for "Live threat catalog" in source or built output.

Remaining findings are advisory:
- WR-01 (HarnessRow name collision — TypeScript resolves it; readability only): non-blocking
- WR-02 (misplaced `import type React` in origin-story.tsx): non-blocking
- WR-03 (Playwright Test 4 stale page_text): non-blocking; test currently passes and pages are not actually closed before Test 4 runs
- WR-04 (magic `time.sleep(0.3)`): advisory
- WR-05 (header nav hidden on <768px — MITIGATED by SiteFooter providing Docs/Changelog/Security links): advisory
- IN-01..04: informational

---

### Token Discipline Verification (D-01)

All `--color-bk-*` occurrences in `web/components/home/` are guard comments (e.g. `// NEVER --color-bk-* (dark-only; breaks light theme)`). Zero color-value usages of `--color-bk-*` in any home component. All color styling uses raw theme tokens: `var(--bg)`, `var(--fg)`, `var(--amber)`, `var(--teal)`, `var(--coral)`, `var(--red)`, `var(--green)`, `var(--dim)`, `var(--dimmer)`, `var(--surface)`, `var(--surface-2)`, `var(--border)`, `var(--border-strong)`.

---

### Anti-Patterns Found

None. No TBD/FIXME/XXX markers in any file modified by this phase. No stub implementations, no placeholder text, no hardcoded empty returns. No aspirational claims in rendered content.

---

### Human Verification Required

None. The both-theme and above-the-fold assertions were verified by the Python Playwright spec (`web/tests/home_spec.py`) which serves the built `out/` directory locally and drives headless Chromium. All 8 assertions documented in 15-03-SUMMARY.md passed with specific measured values (pixel coordinates, RGB color values). The built `web/out/index.html` was independently spot-checked above.

---

### Content Accuracy Cross-Check (D-03)

All content claims verified against source documents:

| Claim | Source | Verdict |
|-------|--------|---------|
| Nx Console details (nrwl.angular-console, 18-min, ~3800 repos, 2026-05-18) | docs/harness-support-matrix.md; THREAT-MODEL references | Accurate |
| 6 feature card capabilities | README.md + CLAUDE.md | Accurate; exactly the 6 shipped capabilities listed |
| Corroboration semantics (1 warn / 2 block / 3 quarantine) | CLAUDE.md "Corroboration-based" decision | Accurate |
| install-chip footnote ("Reproducible builds · Sigstore signed · SLSA L3 provenance") | THREAT-MODEL.md §2; CLAUDE.md Phase 1 | Accurate |
| All 15 harness names, tiers, caveats | docs/harness-support-matrix.md (verbatim) | Accurate — code review confirmed exhaustive cross-check |
| 4 known gaps in honesty callout | docs/THREAT-MODEL.md §8 | Accurate — each gap traced to named section |
| Claude Code the only live-verified harness | docs/harness-support-matrix.md Honesty Note §1 | Accurate |
| Hermes fail-OPEN (exit codes ignored) | docs/harness-support-matrix.md Tier 2; THREAT-MODEL §8 | Accurate |
| Kilo/Trae UNGUARDED native tools | docs/harness-support-matrix.md Tier 3 | Accurate |

---

## Gaps Summary

No gaps. All 5 roadmap success criteria are satisfied. SITE-03 (live deploy) is explicitly deferred per maintainer decision D-02 and is not a blocking gap for this phase.

---

_Verified: 2026-06-08_
_Verifier: Claude (gsd-verifier)_
