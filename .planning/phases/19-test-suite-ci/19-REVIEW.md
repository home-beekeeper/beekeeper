---
phase: 19-test-suite-ci
reviewed: 2026-06-11T00:00:00Z
depth: standard
files_reviewed: 16
files_reviewed_list:
  - web/vitest.config.mts
  - web/playwright.config.ts
  - web/package.json
  - web/.gitignore
  - web/tests/unit/utils.test.ts
  - web/tests/unit/reduced-motion.test.tsx
  - web/tests/unit/install-chip.test.tsx
  - web/tests/unit/metadata.test.ts
  - web/tests/unit/accuracy.test.ts
  - web/tests/e2e/home.spec.ts
  - web/tests/e2e/gfx.spec.ts
  - web/tests/postbuild/seo.test.ts
  - .github/workflows/web.yml
  - .github/workflows/ci.yml
  - web/app/sitemap.ts
  - web/components/docs/unenforced-callout.tsx
findings:
  critical: 0
  warning: 6
  info: 5
  total: 11
status: issues_found
---

# Phase 19: Code Review Report

**Reviewed:** 2026-06-11
**Depth:** standard
**Files Reviewed:** 16
**Status:** issues_found

## Summary

Phase 19 establishes the JS-native web test toolchain (Vitest unit, Playwright E2E, post-build SEO file-walk) plus path-filtered CI. The code is generally careful — the authors clearly anticipated several pitfalls (globals:false explicit cleanup, force-static sitemap, dual-theme tokens, pinned port). No security vulnerabilities or correctness BLOCKERS were found.

The concerns that remain cluster around **CI robustness and supply-chain posture** rather than functional bugs:

1. The E2E gate downloads `serve` at runtime via `pnpm dlx`, which is neither in the lockfile nor pinned — this punctures the `--frozen-lockfile` guarantee and introduces a runtime network/supply-chain dependency the rest of the pipeline avoids (WR-01).
2. The bidirectional `paths-ignore` / `paths` split between `ci.yml` and `web.yml` will deadlock merges if either workflow's job is a *required* branch-protection check (WR-02).
3. Several CI actions use mutable major-version tags rather than pinned SHAs, inconsistent with the project's stated pinned-deps / self-defense posture (WR-03).
4. Multiple E2E assertions are structurally allowed to pass without proving anything (vacuous canvas counts, conditionally-skipped LCP) (WR-04, WR-05).

This is a notable risk profile for a project whose entire reason for existing is supply-chain / agent-runtime safety: the test harness for the public site should not relax the very guarantees the product enforces.

## Warnings

### WR-01: E2E webServer downloads `serve` at runtime via `pnpm dlx`, defeating `--frozen-lockfile`

**File:** `web/playwright.config.ts:29` (and `web/package.json:10` `start` script)
**Issue:** The Playwright `webServer.command` is `pnpm dlx serve out --listen 4199`. `serve` is **not** a dependency in `web/package.json` and is **not present in the root `pnpm-lock.yaml`** (verified — only `serve-static`/`@hono/node-server` transitive deps exist, not the `serve` CLI). `pnpm dlx` resolves and downloads the latest `serve` from the registry at test time. Consequences:
- The CI pipeline runs `pnpm install --frozen-lockfile` (web.yml:47) to guarantee a reproducible, audited dependency set — but the E2E gate then pulls an **unpinned, unaudited** package from the network at runtime, silently bypassing that guarantee. For a supply-chain security product this is a posture contradiction.
- It is a flakiness source: a registry hiccup or a breaking `serve` release fails the gate for reasons unrelated to the site. The config comment even acknowledges the 60s timeout exists *because* of this cold download.
- The latest `serve` could change its default behavior (port, SPA fallback, trailing-slash handling) and break the static-export contract without any lockfile diff to review.

**Fix:** Add `serve` (pinned) to `web/devDependencies` so it is in the lockfile and installed by `--frozen-lockfile`, then invoke it deterministically:
```jsonc
// package.json devDependencies
"serve": "14.2.4"
```
```ts
// playwright.config.ts
webServer: {
  command: "pnpm exec serve out --listen 4199 --no-port-switching",
  // ...
  timeout: 30000, // no runtime download, can tighten
}
```
(`--no-port-switching` also hard-fails instead of silently picking another port if 4199 is taken — see WR-06.)

### WR-02: Bidirectional `paths`/`paths-ignore` split deadlocks merges when either job is a required check

**File:** `.github/workflows/ci.yml:5-14` and `.github/workflows/web.yml:10-18`
**Issue:** `ci.yml` uses `paths-ignore: ['web/**', ...]` and `web.yml` uses `paths: ['web/**', ...]`. This is the documented GitHub footgun: if branch protection marks the Go `test` job (or `web` job) as a **required status check**, a PR that touches only the *other* path set will never trigger that workflow, so the required check never reports and stays "Expected — Waiting for status" **forever**, blocking merge. A web-only PR can never satisfy a required Go `test`, and vice-versa.

This is latent — it only bites once branch protection is configured with these as required checks (likely, given the release-gate emphasis in CLAUDE.md). The Phase context claims "bidirectional isolation" as a feature; the merge-blocking interaction is the cost.

**Fix:** Either (a) do not mark the path-filtered jobs as *required* checks, or (b) replace `paths-ignore`/`paths` skipping with non-skipping conditional jobs that always run but no-op cheaply (the "dummy job that always succeeds" pattern from GitHub docs), so the required-check name always reports a conclusion. Document the chosen approach next to the path filters so a future maintainer enabling branch protection does not hit the deadlock.

### WR-03: CI actions pinned to mutable major-version tags, not SHAs — inconsistent with project pinned-deps posture

**File:** `.github/workflows/web.yml:33,36,42,76` (`actions/checkout@v4`, `pnpm/action-setup@v6`, `actions/setup-node@v4`, `actions/upload-artifact@v4`); also `.github/workflows/ci.yml` throughout (`actions/checkout@v4`, `actions/setup-go@v5`, `cilium/little-vm-helper@v0.0.21`)
**Issue:** CLAUDE.md's self-defense non-negotiables call for "pinned deps" and the project uses full-semver action pins elsewhere (e.g. `slsa-github-generator@v2.1.0`, the explicit warning against `@v2`). These workflows pin only to mutable major tags. A compromised or retagged third-party action (notably `cilium/little-vm-helper@v0.0.21` — already a version tag, lower risk, but the first-party `@vN` tags are mutable) can execute in CI. For a security tool this is exactly the supply-chain class the product guards against. The `web.yml` `permissions: contents: read` limits blast radius (good), but the Go `ci.yml` jobs run with default token permissions and `go install` from a network source.

**Fix:** Pin actions to immutable commit SHAs with a trailing version comment, e.g.:
```yaml
- uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
- uses: pnpm/action-setup@... # v6.x
```
At minimum pin the third-party `cilium/little-vm-helper` to a SHA. Note `ci.yml` lacks an explicit top-level `permissions:` block — add `permissions: contents: read` there too (least privilege; the Go jobs need no write scope).

### WR-04: GFX canvas assertions pass vacuously after 3D removal — regression value is near zero

**File:** `web/tests/e2e/gfx.spec.ts:31-41, 95-111`
**Issue:** The R3F canvas was removed (OQ-2), so `canvas count <= 1` and `canvasAfter <= Math.max(canvasBefore, 1)` are satisfied by `0 <= 1` regardless of what the page does. The comment is honest that these "pass vacuously," but as written they would **not catch** the failure mode they claim to guard (a reintroduced stray WebGL context that mounts exactly one canvas still passes `<= 1`; a leak that mounts on every nav would need `> 1` to trip, which a single reintroduced canvas never does). The "no context leak across navigate" test compares `canvasAfter <= max(canvasBefore, 1)` — with both 0, it can never detect the realistic single-canvas reintroduction.

**Fix:** If the contract is "3D is gone, keep it gone," assert the *absence* directly and exactly: `expect(count).toBe(0)` in the post-hydration and post-navigation cases (the reduced-motion test at line 58 already correctly uses `toBe(0)`). That turns a reintroduced canvas into a hard failure instead of a silent pass.

### WR-05: LCP/FCP budget assertion is skipped entirely when no perf entry is emitted

**File:** `web/tests/e2e/gfx.spec.ts:88-92`
**Issue:** The GFX-04 budget check only runs `expect(...).toBeLessThan(2500)` when `perf.src !== "none" && perf.t > 0`. In headless Chromium the LCP/paint entries are frequently absent (the comment acknowledges this), so on CI this test routinely asserts **nothing** and reports green. A real LCP regression on a runner that happens not to emit the entry would pass silently. This is a test that can never fail on the path it most often takes.

**Fix:** Make headless emit paint timings deterministically, or fail loudly when no entry is available rather than passing. Options: use `PerformanceObserver` with `await page.evaluate(() => new Promise(...))` to wait for the entry, or assert that an entry *was* captured (`expect(perf.src).not.toBe("none")`) so a missing-entry environment is a visible signal, not a silent skip. At minimum, log when the assertion is skipped so green doesn't imply "budget met."

### WR-06: E2E webServer port collision is silently tolerated locally via `reuseExistingServer`

**File:** `web/playwright.config.ts:29,31`
**Issue:** `reuseExistingServer: !process.env.CI` means locally Playwright reuses whatever is already listening on `http://127.0.0.1:4199`. Combined with bare `serve out` (which historically falls back to another port if the requested one is busy — see WR-01), a stale or wrong server on 4199 (e.g. a previous build's `out/`, or an unrelated process) is silently reused, and the suite tests **stale content**, producing false greens or confusing failures. There is no health/identity check that the reused server is actually serving the current `out/`.

**Fix:** Pass `--no-port-switching` (or the equivalent) so the server hard-fails on a busy port, and consider gating reuse behind an explicit opt-in env var rather than "any non-CI run." This prevents testing against a stale server.

## Info

### IN-01: `start` script and Playwright webServer both rely on `pnpm dlx serve` — duplicate untracked dependency

**File:** `web/package.json:10`
**Issue:** `"start": "pnpm dlx serve out"` has the same untracked-`serve` problem as WR-01 and additionally omits `--listen`, so `pnpm start` and the E2E server bind different ports — a small inconsistency that can confuse local debugging.
**Fix:** Once `serve` is a pinned devDependency (WR-01), point both at it with explicit, matching `--listen` flags.

### IN-02: `setup-node` Node version is a floating major (`'22'`) while pnpm is pinned exactly

**File:** `.github/workflows/web.yml:43`
**Issue:** `node-version: '22'` floats across all 22.x minors, while pnpm is pinned to `11.1.3` and `packageManager` is exact. A Node minor bump could shift behavior (e.g. test runner, fetch, fs) without a config change. Minor reproducibility gap.
**Fix:** Pin to a full version (`node-version: '22.x.y'`) or add a committed `.nvmrc`/`.node-version` and use `node-version-file`.

### IN-03: Post-build content-file filter uses fragile substring match

**File:** `web/tests/postbuild/seo.test.ts:31`
**Issue:** `.filter((f) => !f.includes("404") && !f.includes("_not-found"))` will also exclude any future legitimate content route whose *path* contains the substring "404" (e.g. a docs page about HTTP 404s, `out/docs/http-404/index.html`). Low likelihood, but it would silently drop a real page from SEO coverage rather than testing it.
**Fix:** Match on path segments / basename precisely, e.g. exclude only `404.html`, `404/index.html`, and `_not-found` directory segments, instead of any-substring `includes`.

### IN-04: `accuracy.test.ts` AC-1 path-existence check is platform-sensitive on case and separators

**File:** `web/tests/unit/accuracy.test.ts:60-68`
**Issue:** `source_doc` values are split on `,` and joined to `REPO_ROOT` with `join`, then `existsSync`. On case-insensitive filesystems (Windows/macOS dev) a wrong-case `source_doc` path passes locally but would fail on Linux CI (or vice-versa). The test will catch a *missing* file but can mask a *wrong-case* reference depending on the runner OS. Since the gate exists to keep docs honest, a path that resolves only on the dev OS is a real (if narrow) gap.
**Fix:** Acceptable as-is for the missing-file contract, but consider a case-sensitive existence check (read the parent dir and assert the exact entry name) so doc references are verified the way Linux CI / production will resolve them.

### IN-05: E2E QA-02 docs nav/search tests are coupled to link label text

**File:** `web/tests/e2e/home.spec.ts:112-115, 132-138`
**Issue:** Tests locate sidebar/search results by `name: /configuration/i`. If the "Configuration" doc is renamed or reordered, these break for a reason unrelated to the behavior under test (navigation works / search returns a result). The test name promises a generic contract but is bound to one specific page label.
**Fix:** Lower-risk: pick a label guaranteed stable, or assert on a structural result (any result exists and clicking any result lands under `/docs/`) rather than a specific title. Minor maintainability note, not a correctness bug.

---

_Reviewed: 2026-06-11_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
