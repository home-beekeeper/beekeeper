---
phase: 13
slug: docs-content-pipeline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-08
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `13-RESEARCH.md` § Validation Architecture. Phase 13 has **no JS test
> runner yet** (Vitest/Playwright land in Phase 19) — validation here is `pnpm build`
> (exit 0) + filesystem asserts on `out/`, plus the **Playwright-Python** smoke harness
> already available from the Phase 12 smoke tests for the interactive criteria (SC-3, SC-4).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | None installed (Phase 19 adds Vitest + Playwright-TS). Phase 13 uses `pnpm build` + filesystem asserts + the **Playwright-Python 1.57.0 + cached chromium** harness used for Phase 12 smoke. |
| **Config file** | none — `pnpm build` is the gate; Playwright-Python invoked ad hoc against served `out/` |
| **Quick run command** | `cd web && pnpm build` |
| **Full suite command** | `cd web && pnpm build` then serve `out/` (`pnpm start`) + run the SC-2/3/4 Playwright-Python checks |
| **Estimated runtime** | ~30–90s for `pnpm build`; +~15s for the served Playwright pass |

---

## Sampling Rate

- **After every task commit:** Run `cd web && pnpm build` — must exit 0 (codegen + static export must not break).
- **After every plan wave:** Run `pnpm build` + the wave-gate filesystem asserts (below).
- **Before `/gsd-verify-work`:** Full build green + all 4 success-criteria checks pass against served `out/`.
- **Max feedback latency:** ~90 seconds (one `pnpm build`).

---

## Per-Task Verification Map

> Task IDs are assigned by the planner; this map is keyed by the wave gates from
> `13-RESEARCH.md`. Every task's minimum gate is `pnpm build` exits 0. The plans
> must additionally attach the criterion-specific asserts below to the task that
> completes each wave.

| Wave | Gate | Requirement | Secure Behavior | Test Type | Automated Command | Status |
|------|------|-------------|-----------------|-----------|-------------------|--------|
| 0 | Toolchain wiring (`source.config.ts`, `next.config.mjs` rename, `tsconfig` alias, deps installed) | DOCS-01 | Build-script approval gate handled non-interactively; no untrusted lifecycle scripts run | build | `cd web && pnpm build` exits 0; `.source/` generated | ⬜ pending |
| 1/2 | Docs route + source loader + DocsLayout + RootProvider + static-search route + `@source` glob restored | DOCS-01 | Static export only — no server runtime; search index is a build artifact, not a live endpoint | build + fs | `pnpm build`; `test -s out/api/search/index.json` | ⬜ pending |
| 3 | 8 seed MDX sections + `meta.json` ordering (phase-complete gate) | DOCS-01 | Static content only; no remote/runtime fetch | build + fs + e2e | `pnpm build`; all 8 `out/docs/<section>/index.html` exist; SC-2/3/4 Playwright pass | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Success-Criteria Asserts (phase-complete gate)

- **SC-1** — `pnpm build` exits 0; `out/api/search/index.json` exists and is >100 bytes; all 8 `out/docs/<section>/index.html` exist.
- **SC-2** — Served `out/`: sidebar nav lists getting-started → installation → configuration → integration → security → cli-reference → audit-log → troubleshooting **in that order**.
- **SC-3** — `out/docs/getting-started/` renders a TOC panel with ≥3 entries (seed page has multiple headings).
- **SC-4** — Served `out/`: open search dialog, query `beekeeper`, assert ≥1 result, click → URL navigates to a `/docs/` path.

---

## Wave 0 Requirements

- [ ] `web/source.config.ts` — fumadocs-mdx collection config; required before `pnpm build` can generate `.source/`
- [ ] `web/next.config.mjs` (rename from `next.config.ts`) — required for the ESM-only `createMDX` from fumadocs-mdx; **must preserve** `output: "export"`, `trailingSlash: true`, `images.unoptimized`, `transpilePackages`
- [ ] `web/tsconfig.json` — add `"collections/*": ["./.source/*"]` path alias so `import { docs } from 'collections/server'` resolves
- [ ] Fumadocs deps installed (`fumadocs-ui`, `fumadocs-core`, `fumadocs-mdx` @ research-pinned versions) via root pnpm workspace; build-script approval gate handled non-interactively (`CI=true`)
- [ ] 8 seed MDX files + `meta.json` — required for `source.generateParams()` to emit routes and for Orama to have content to index

*Existing infrastructure does NOT cover these — they are net-new wiring this phase introduces.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| FOWT / theme-bridge non-regression (RootProvider `theme={{ enabled: false }}` defers to next-themes) | DSYS-02 / DOCS-01 | No automated theme-flash detector in this phase | Toggle theme on a `/docs/` page; confirm no flash-of-wrong-theme and a single theme owner (no double toggle) |
| Search relevance feel | DOCS-01 | Orama ranking is subjective | Query a few real terms; confirm results point at the right sections |

*SC-2/3/4 are automatable via Playwright-Python against served `out/`; the above are the residual manual checks.*

---

## Validation Sign-Off

- [ ] Every task has a `pnpm build` gate (minimum) + wave-gate asserts attached to the wave-completing task
- [ ] Sampling continuity: no 3 consecutive tasks without an automated `pnpm build` verify
- [ ] Wave 0 covers all net-new wiring (source.config, next.config.mjs, tsconfig alias, deps, seed content)
- [ ] No watch-mode flags in any verify command (`next dev` / `--watch` forbidden in gates)
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter once the plan attaches these gates

**Approval:** pending
