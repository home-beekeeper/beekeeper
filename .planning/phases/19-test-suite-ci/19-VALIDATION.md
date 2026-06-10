---
phase: 19
slug: test-suite-ci
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-10
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Phase 19 is meta: its deliverables ARE the test infrastructure (Vitest unit + @playwright/test
> E2E + the `web.yml` CI gate). "Wave 0" here installs the new JS-native framework and config so
> later tasks have a runner. The phase gate is the full local suite green on the production `out/`
> build AND the four Python specs still green at the moment they are retired (no coverage regression).
> CI is build-verified LOCALLY (D-03) — the `web.yml` YAML is verified by inspection/schema, NOT a
> live GitHub run (repo unpushed).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | NEW (Wave 0 installs): Vitest 4.1.8 (unit, jsdom + `@vitejs/plugin-react` + `vite-tsconfig-paths`) + @playwright/test 1.57.0 (E2E, chromium-only, matches local Python playwright r1200). Replaces the 4 Python specs. |
| **Config files** | `web/vitest.config.mts` + `web/playwright.config.ts` (both NEW — Wave 0) |
| **Quick run command** | `cd web && pnpm test` (Vitest unit: accuracy + components; pre-build, fast) |
| **Post-build run** | `cd web && pnpm build && pnpm test:postbuild && pnpm test:e2e` (SEO file-walk over `out/` + Playwright E2E) |
| **Full suite command** | `cd web && pnpm lint && pnpm typecheck && pnpm test && pnpm build && pnpm test:postbuild && pnpm test:e2e` (the exact CI step order) |
| **Estimated runtime** | unit ~2–5s; build ~20–40s; e2e ~10–30s; full suite ~1–2 min |

---

## Sampling Rate

- **After every task commit:** `cd web && pnpm test` (Vitest unit — fast, no build). For tasks that touch
  E2E/SEO/build/CI: also `pnpm build && pnpm test:postbuild && pnpm test:e2e`.
- **After every plan wave:** Full suite command (lint → typecheck → unit → build → postbuild → e2e).
- **Before `/gsd-verify-work`:** Full suite green on the production `out/` build, AND the 4 Python specs
  (`home_spec.py` / `gfx_spec.py` / `seo_spec.py` / `accuracy_spec.py`) re-run green immediately before
  deletion to prove JS parity (no coverage regression), AND `web.yml` + `ci.yml` YAML statically validated.
- **Max feedback latency:** ~5s for the unit walk; ~2 min for the full suite.

---

## Per-Task Verification Map

> Seeded from RESEARCH §Validation Architecture. The planner fills exact task IDs; the requirement→test
> mapping below is locked.

| Task (seed) | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| Install framework + configs + scripts | 1 | QA-01 | N/A (dev tooling, no postinstall scripts — vetted) | build | `cd web && pnpm install --frozen-lockfile && pnpm typecheck` | ❌ W0 — `vitest.config.mts`, `playwright.config.ts` | ⬜ pending |
| Unit: `cn()` + `useReducedMotion` + `InstallChip` + `lib/metadata` consts | 1–2 | QA-01 | N/A | unit | `cd web && pnpm test` | ❌ W0 — `tests/unit/*.test.ts(x)` | ⬜ pending |
| Unit: accuracy port (source-MDX walk, pre-build) | 1–2 | QA-01 | N/A | unit (node) | `cd web && pnpm test` | ❌ W0 — `tests/unit/accuracy.test.ts` | ⬜ pending |
| Postbuild: SEO port (`out/` file-walk) | 2 | QA-01 | N/A | node (post-build) | `cd web && pnpm build && pnpm test:postbuild` | ❌ W0 — `tests/postbuild/seo.test.ts` | ⬜ pending |
| E2E: home renders w/ `/hero-hive.svg` fallback + above-fold + dual-theme | 2 | QA-02 | N/A | e2e | `cd web && pnpm test:e2e -- --grep home` | ❌ W0 — `tests/e2e/home.spec.ts` | ⬜ pending |
| E2E: docs nav + search returns ≥1 result → navigates | 2 | QA-02 | N/A | e2e | `cd web && pnpm test:e2e -- --grep docs` | ❌ W0 — `tests/e2e/home.spec.ts` | ⬜ pending |
| E2E: theme toggle persists across reload (no FOWT) | 2 | QA-02 | N/A | e2e | `cd web && pnpm test:e2e -- --grep theme` | ❌ W0 | ⬜ pending |
| E2E: 3 changelog pages render headings | 2 | QA-02 | N/A | e2e | `cd web && pnpm test:e2e -- --grep changelog` | ❌ W0 | ⬜ pending |
| `web.yml` path-filtered job (5 ordered steps) | 1–3 | QA-01 | least-privilege: default `read` perms; first-party actions only | schema | static YAML inspection (D-03 — no live run) | ❌ W0 — `.github/workflows/web.yml` | ⬜ pending |
| `ci.yml` `paths-ignore` web isolation (bidirectional) | 1–3 | QA-01 | N/A | schema | static YAML inspection | edit existing `.github/workflows/ci.yml` | ⬜ pending |
| Retire 4 Python specs after JS parity proven | 3 | QA-01/02 | N/A | gate | run `.py` specs green, then delete | exists → deleted | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/vitest.config.mts` — Vitest config (jsdom; `tsconfigPaths()` before `react()`; `globals: false`; include `tests/unit`)
- [ ] `web/playwright.config.ts` — chromium project + managed `webServer` (`serve out --listen 4199`, port pinned)
- [ ] `web/package.json` — add `typecheck` / `test` / `test:postbuild` / `test:e2e` scripts + pinned devDependencies
- [ ] `web/tests/unit/` + `web/tests/postbuild/` + `web/tests/e2e/` dirs with at least one stub each (RED until ported)
- [ ] framework install: `vitest`, `vite`, `@vitejs/plugin-react`, `jsdom`, `vite-tsconfig-paths`, `@testing-library/react`, `@testing-library/dom`, `@playwright/test` (pinned, package-legitimacy gate)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `web.yml` actually runs in GitHub Actions (3-OS isolation, cache hit, browser install) | QA-01 | Repo is unpushed (D-03); no live CI run available this phase | Verified at repo-push / SITE-03 deploy track. This phase validates YAML by static inspection only. |
| Social-card / cross-browser nuances | — | Out of scope (chromium-only, no visual regression) | N/A |

*All in-scope Phase-19 behaviors have automated verification (local). The only manual/deferred item is the live GitHub-hosted run.*

---

## Validation Sign-Off

- [ ] All tasks have an automated verify command or a Wave 0 dependency
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (configs + framework + stub dirs)
- [ ] No watch-mode flags in any CI/gate command (`vitest run`, not `vitest`)
- [ ] Feedback latency < 120s (full suite)
- [ ] JS parity proven against the 4 Python specs before they are deleted
- [ ] `nyquist_compliant: true` set in frontmatter (after planner fills task IDs)

**Approval:** pending
