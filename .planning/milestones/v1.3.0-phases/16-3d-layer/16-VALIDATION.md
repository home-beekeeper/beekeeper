---
phase: 16
slug: 3d-layer
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-09
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `16-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Python `playwright` 1.57.0 (established project pattern, Phases 13–15) |
| **Config file** | none — scripts live in `web/tests/` and run directly |
| **Quick run command** | `cd web && pnpm build` (verifies static export intact) |
| **Full suite command** | `cd web && pnpm build && python tests/home_spec.py && python tests/gfx_spec.py` |
| **Estimated runtime** | ~60–120 seconds (build dominates; Lighthouse adds ~20s) |

---

## Sampling Rate

- **After every task commit:** Run `cd web && pnpm build` (static export must stay intact — GFX-01 invariant)
- **After every plan wave:** Run `cd web && pnpm build && python tests/home_spec.py && python tests/gfx_spec.py`
- **Before `/gsd-verify-work`:** Full suite + Lighthouse LCP assertion green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| W0 stub | 00 | 0 | GFX-01..04 | — | N/A | harness | `cd web && python tests/gfx_spec.py` (new) | ❌ W0 | ⬜ pending |
| build-clean | — | 1 | GFX-01 | — | No R3F/three symbols in server HTML | build grep | `pnpm build && grep -c 'WebGLRenderingContext\|@react-three' out/index.html` → 0 | ❌ W0 | ⬜ pending |
| canvas-lazy | — | 1 | GFX-01 | — | `<canvas>` absent in `out/index.html`, appears post-hydration | DOM | Playwright: no `<canvas>` in raw HTML; present after load | ❌ W0 | ⬜ pending |
| single-canvas | — | 2 | GFX-02 | T-DoS (context flood) | Exactly one WebGL context page-wide | DOM | Playwright: `document.querySelectorAll('canvas').length === 1` | ❌ W0 | ⬜ pending |
| reduced-motion | — | 2 | GFX-03 | — | Canvas absent, SVG present under reduce | Playwright | Emulate `prefers-reduced-motion: reduce` → no `<canvas>`, SVG visible | ❌ W0 | ⬜ pending |
| a11y-aria | — | 2 | GFX-03 | — | `aria-hidden` canvas + sr-only sibling | DOM | Playwright: `canvas[aria-hidden=true]` + `p.sr-only` present | ❌ W0 | ⬜ pending |
| lcp-budget | — | 3 | GFX-04 | — | LCP < 2.5s, SVG is LCP element | Lighthouse | `lighthouse http://localhost:PORT/ --output json` → `lcp < 2500` | ❌ W0 | ⬜ pending |
| no-context-leak | — | 3 | GFX-04 | T-DoS | Canvas count stays 1 across nav cycles | Playwright | Navigate away/back; assert active canvases ≤ 1 | ❌ W0 | ⬜ pending |

*Plan/Task IDs finalize when PLAN.md files are written; rows above map every success criterion to an observable signal. Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/tests/gfx_spec.py` — covers GFX-01 (build assertion + canvas-post-hydration), GFX-02 (single canvas), GFX-03 (reduced-motion fallback + aria), GFX-04 (LCP + context-count)
- [ ] `web/public/hero-hive.svg` — static SVG fallback (LCP element); must exist before `pnpm build`
- [ ] `lighthouse` devDependency install (`pnpm add -D lighthouse@13.3.0`) if CLI audit is used for GFX-04 — Playwright `performance` proxy is the documented fallback (Windows-CLI risk)

*Existing `web/tests/home_spec.py` covers hero visibility + theme switching — no changes needed; GFX tests are additive.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Hive visual quality / brand fit | GFX-01 | Subjective visual acceptance (geometry, color, motion feel) not assertable | Maintainer views served `out/` build in both themes; confirms hive reads as a hive and matches brand teal/amber |
| Drag-to-rotate interaction feel | GFX-01 | Momentum/snap "feel" is qualitative | Maintainer drags the hero centerpiece; confirms PresentationControls snap/limits feel right |

*All correctness-critical behaviors (build cleanliness, fallback, a11y, LCP, context count) have automated verification above.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (`gfx_spec.py`, `hero-hive.svg`, optional `lighthouse`)
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
