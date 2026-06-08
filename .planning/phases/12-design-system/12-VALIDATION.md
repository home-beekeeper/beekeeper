---
phase: 12
slug: design-system
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-08
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `12-RESEARCH.md` § Validation Architecture. Phase 12 ships **no automated
> test runner** — Vitest + Playwright land in Phase 19. The primary executable gate is
> `pnpm build`; behavioral criteria (theme toggle, reduced-motion, AA/keyboard) are
> manual smoke checks this phase.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | None yet in `web/` (Phase 19 installs Vitest + Playwright). Wave 0 of this phase MUST NOT add a test runner. |
| **Config file** | none — N/A this phase |
| **Quick run command** | `cd web && pnpm build` |
| **Full suite command** | `cd web && pnpm build && pnpm lint` (Biome) |
| **Estimated runtime** | ~30–60 seconds (Next.js 16 static export build) |

---

## Sampling Rate

- **After every task commit:** Run `cd web && pnpm build` — catches CSS/TypeScript regressions immediately
- **After every plan wave:** Run `cd web && pnpm build && pnpm lint` — ensures Biome clean
- **Before `/gsd-verify-work`:** Build green + manual smoke tests for DSYS-02 (theme toggle), DSYS-03 (reduced-motion), DSYS-04 (keyboard tab cycle)
- **Max feedback latency:** ~60 seconds (build wall time)

---

## Per-Task Verification Map

> Task IDs are assigned by the planner. This map is seeded from the requirement→test map in
> RESEARCH.md and should be reconciled to concrete `{12}-{plan}-{task}` IDs after planning.

| Req | Behavior | Test Type | Automated Command | File Exists | Status |
|-----|----------|-----------|-------------------|-------------|--------|
| DSYS-01 | `pnpm build` succeeds with correct globals.css import order; Tailwind utilities render Beekeeper tokens | Build assertion | `cd web && pnpm build` | output | ⬜ pending |
| DSYS-01 | `web/components.json` present, official shadcn registry only (no third-party `registries`) | Code audit | `grep -L "registries" web/components.json` | ❌ W0 creates | ⬜ pending |
| DSYS-02 | Theme toggle adds `.dark`/light class to `<html>`; `bk-theme` persists in localStorage; no FOUC | Manual smoke | open built `out/` via `pnpm start`, toggle, reload | n/a | ⬜ pending |
| DSYS-02 | `suppressHydrationWarning` on `<html>` | Code audit | `grep suppressHydrationWarning web/app/layout.tsx` | ❌ W0/W1 | ⬜ pending |
| DSYS-03 | `data-reduced-motion` set on `<html>` when OS reduced-motion on | Manual browser | DevTools → Rendering → Emulate prefers-reduced-motion | n/a | ⬜ pending |
| DSYS-03 | `useReducedMotion()` hook exported from `@/lib/reduced-motion` (compiles) | TS build | `cd web && pnpm build` | ❌ W0 creates | ⬜ pending |
| DSYS-04 | Both themes pass WCAG 2.1 AA contrast | Manual spot-check vs UI-SPEC §WCAG + DevTools a11y | Chrome DevTools a11y panel on built `out/` | n/a | ⬜ pending |
| DSYS-04 | Skip link present + keyboard-reachable | Manual keyboard | Tab to skip link, verify it appears | n/a | ⬜ pending |
| DSYS-04 | shadcn focus rings visible (`--color-ring: var(--color-bk-teal)`) | Visual / code audit | `grep ring web/app/globals.css` | ❌ W0/W1 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/components.json` — covers DSYS-01 (shadcn init for Tailwind v4, `tailwind.config: ""`)
- [ ] `web/lib/reduced-motion.tsx` — covers DSYS-03 (ReducedMotionProvider + `useReducedMotion` hook)
- [ ] `web/app/providers.tsx` — covers DSYS-02 (next-themes ThemeProvider wiring; marks Phase 13 RootProvider insertion point)

*No test framework install in Phase 12 — all Wave 0 items are source files to CREATE. (Vitest/Playwright deferred to Phase 19.)*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Theme toggle persistence + no flash-of-wrong-theme | DSYS-02 | No automated a11y/visual runner until Phase 19; FOUC is a paint-timing behavior | Build (`pnpm build`), serve `out/` (`pnpm start` or static server), toggle theme, hard-reload — confirm no flash and choice persists |
| Reduced-motion gate disables animation/3D | DSYS-03 | Requires OS/DevTools emulation of `prefers-reduced-motion` | DevTools → Rendering → "Emulate prefers-reduced-motion: reduce"; confirm `data-reduced-motion` on `<html>` and animations are off |
| WCAG 2.1 AA contrast in both themes | DSYS-04 | Automated axe/Lighthouse gate deferred to Phase 19; this phase spot-checks against UI-SPEC §WCAG tables | Chrome DevTools a11y / contrast checker on built `out/` in both themes; compare to UI-SPEC contrast tables |
| Keyboard reachability + visible focus | DSYS-04 | Behavioral; no automated runner this phase | Tab through page: skip link appears first, all interactive elements reachable, focus ring visible in both themes |

---

## Validation Sign-Off

- [ ] All tasks have an automated `pnpm build` verify or a documented manual check above
- [ ] Sampling continuity: build runs after each task commit (no 3 consecutive tasks without a build gate)
- [ ] Wave 0 covers all MISSING file references (components.json, reduced-motion.tsx, providers.tsx)
- [ ] No watch-mode flags in any verify command
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter (after plan reconciliation)

**Approval:** pending
