# Phase 19: Test Suite & CI - Context

**Gathered:** 2026-06-10
**Status:** Ready for planning
**Source:** Captured inline (discuss-phase skipped per the v1.3.0 inline precedent, Phases 13-18)

<domain>
## Phase Boundary

Phase 19 is the LAST v1.3.0 ("Web Presence & Documentation") phase. It delivers the
automated quality gate for the `web/` Next.js static-export site:

1. A path-filtered GitHub Actions job `.github/workflows/web.yml` that runs, in order:
   Biome lint/format check -> `tsc --noEmit` type-check -> Vitest unit tests ->
   `pnpm build` (static export to `out/`) -> Playwright (TS) E2E against `out/`.
2. The test files themselves: Vitest unit tests + `@playwright/test` E2E specs.
3. A path-ignore edit on the existing Go `.github/workflows/ci.yml` so Go CI never
   fires on web-only changes (SC-1 is bidirectional isolation).

**Requirements:** QA-01 (path-filtered web CI: build + Vitest unit + Playwright E2E +
Biome, gating merges) and QA-02 (E2E verifies the critical paths: home renders with
hero fallback, docs nav + search returns results, theme toggle persists, all three
changelog pages render).

**Out of scope:** any change to Go CI beyond the path-ignore; live GitHub-hosted CI
execution (the repo has never been pushed — see D-03); deploy/push (SITE-03, deferred).
</domain>

<decisions>
## Implementation Decisions

### D-01 - Test toolchain: Full JS-native (maintainer decision, 2026-06-10)
Adopt Vitest (unit) + `@playwright/test` (E2E in TS) as the single, Node-only test
toolchain that QA-01 names. The four existing pure-Python specs are PORTED and then
RETIRED once parity is proven:
- **Browser specs** `web/tests/home_spec.py` + `web/tests/gfx_spec.py`
  (serve `out/` + drive chromium) -> `@playwright/test` E2E specs.
- **File-walk specs** `web/tests/seo_spec.py` + `web/tests/accuracy_spec.py`
  (no browser; assert over `out/` and source MDX) -> Vitest node tests.
CI runs Node-only; no Python toolchain in the web job. Retire the `.py` files only
after the JS equivalents pass on the same `out/` build (no coverage regression).

### D-02 - Research-first (maintainer decision, 2026-06-10)
Run gsd-phase-researcher before planning to de-risk the CI/test config: pnpm caching in
GitHub Actions, `paths`/`paths-ignore` filter semantics, `@playwright/test` `webServer`
serving the static `out/`, and a Vitest setup compatible with Next 16 / React 19 /
Tailwind v4. Matches the Phase 13-18 precedent.

### D-03 - CI is build-verified locally; live execution deferred
The repo has never been pushed to GitHub (every prior v1.3.0 phase deferred deploy/push).
Phase 19's deliverable is the workflow file + tests that pass LOCALLY on Windows:
`pnpm build` + `vitest run` + `playwright test` against `out/` all green. The workflow's
actual GitHub-hosted run is verified when the repo is pushed (deploy/SITE-03 track).
**The plan must NOT block on a live CI run.** YAML correctness is verified by static
inspection / schema, not by a GitHub run.

### D-04 - Go CI isolation is bidirectional
`web.yml` triggers ONLY on `web/**` + `pnpm-workspace.yaml` (+ itself). The existing
Go `ci.yml` gains a `paths-ignore` for `web/**`, `pnpm-workspace.yaml`, and
`.github/workflows/web.yml` so a web-only change never spins the 3-OS Go matrix
(currently `ci.yml` triggers on every push/PR with no path filter). SC-1 requires BOTH
directions.

### D-05 - Execution runs INLINE on main
Subagents lack node/pnpm (and now Playwright browser binaries), so plans execute inline
on `main` with per-task atomic commits, NOT via gsd-executor. Established precedent
across Phases 15-18. Tracking (STATE/ROADMAP/REQUIREMENTS) is hand-managed (the GSD
state.* SDK verbs corrupt STATE frontmatter for this project).

### D-06 - New dev deps pass the package-legitimacy gate
Vitest, `@playwright/test`, and any adapters (e.g. jsdom/happy-dom, @vitejs/plugin-react,
React Testing Library if used) are added as workspace-isolated `devDependencies` with
pinned versions, vetted through the same package-legitimacy gate used in Phases 12/16
before install. Add the missing `typecheck` (`tsc --noEmit`), `test` (vitest run), and
`test:e2e` (playwright test) scripts to `web/package.json`.

### D-07 - Playwright serves the real static `out/`
E2E runs against the actual static-export output (`webServer` launches a static file
server over `out/`, mirroring the Python specs' `http.server` approach), respecting the
project's `trailingSlash` + flat-file reality. E2E must cover the QA-02 critical paths
exactly: (a) home renders with the SVG hero fallback present, (b) docs navigation works
AND search returns >=1 result then navigates, (c) theme toggle persists across reload
(no flash-of-wrong-theme), (d) all three changelog version pages (v1.0.0 / v1.2.0 /
v1.3.0) render their headings.

### Claude's Discretion
- Exact Vitest config (jsdom vs happy-dom; whether to add React Testing Library).
- Which client components/hooks get unit tests (thin surface: InstallChip copy,
  theme toggle, `useReducedMotion`, the `useCanMount3D` capability gate, `cn()` util,
  source/changelog loaders).
- Playwright project/browser matrix (default: chromium-only to mirror the .py specs).
- GitHub Actions pinned action versions (setup-node, pnpm/action-setup, playwright cache).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap / requirements
- `.planning/ROADMAP.md` (Phase 19 section) - goal, SC-1..3, QA-01/QA-02
- `.planning/REQUIREMENTS.md` (QA-01, QA-02 rows)

### Port source (existing tests to replace)
- `web/tests/home_spec.py` - browser E2E coverage to port (home, both themes, above-fold, harness matrix, honesty callout)
- `web/tests/gfx_spec.py` - browser E2E coverage to port (GFX-01..04: server-clean, canvas-post-hydration, reduced-motion fallback, FCP/LCP proxy, no context leak)
- `web/tests/seo_spec.py` - file-walk assertions to port (title/desc/canonical, OG image, sitemap/robots over `out/`)
- `web/tests/accuracy_spec.py` - source-MDX file-walk to port (source_doc frontmatter, unenforced labels, no phantom commands)

### CI / build
- `.github/workflows/ci.yml` - Go CI to path-filter (D-04); pnpm/setup-node patterns to mirror
- `.github/workflows/release.yml` - existing Actions style reference
- `web/package.json` - scripts + devDependencies surface (no test infra yet)
- `web/next.config.mjs` (or `.ts`) - static-export config (`output:'export'`, `trailingSlash`, `images.unoptimized`)
- `web/biome.json` (or biome config) - the `biome check` gate already wired as `lint`

### Precedent
- Phase 12/16 package-legitimacy gate (adding pinned web deps safely)
- Phase 11 pnpm-workspace isolation (web install must not touch go.mod/go.sum)
</canonical_refs>

<specifics>
## Specific Ideas

- The web job step order is fixed by QA-01: lint/format -> typecheck -> unit -> build -> E2E.
  E2E depends on a fresh `out/` from the build step (same job, or an artifact handoff).
- Mirror the Python specs' distinct-port convention (home 4199, gfx 4200) only if not
  using `@playwright/test`'s managed `webServer` (preferred: one managed webServer).
- `pnpm build` already proven green on Windows; the JS specs must pass on the same `out/`.
- Keep the whole web CI job isolated: `working-directory: web` (or `cwd`), pnpm workspace
  install must not modify any Go-module file (Phase 11 invariant).
</specifics>

<deferred>
## Deferred Ideas

- Live GitHub-hosted CI execution (verified at repo push / SITE-03 deploy track).
- Cross-browser E2E (firefox/webkit) beyond chromium - default chromium-only, parity with .py.
- Visual-regression / screenshot snapshots - not in QA-01/02 scope.
- Go CI restructuring beyond the `paths-ignore` web isolation.
- The deferred `.planning/todos/pending/docs-command-card-copy.md` backlog item (separate).
</deferred>

---

*Phase: 19-test-suite-ci*
*Context gathered: 2026-06-10 (captured inline)*
