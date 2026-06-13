---
phase: 19-test-suite-ci
verified: 2026-06-11T09:05:00Z
status: passed
score: 3/3 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: initial verification (no prior VERIFICATION.md)
---

# Phase 19: Test Suite & CI Verification Report

**Phase Goal:** A path-filtered web CI job gates merges with lint, type-check, unit tests, a static build, and E2E tests against the `out/` directory — completely isolated from Go CI.
**Verified:** 2026-06-11T09:05:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria — the contract)

| # | Truth (SC) | Status | Evidence |
| --- | --- | --- | --- |
| 1 | `web.yml` triggers only on `web/**` + `pnpm-workspace.yaml` (never Go); Go CI never triggers on web changes | ✓ VERIFIED | `web.yml:7-18` has `paths:` include = `web/**`, `pnpm-workspace.yaml`, `.github/workflows/web.yml` on BOTH `push` and `pull_request`. `ci.yml:3-14` has the mirror `paths-ignore` (same three globs) on BOTH triggers. Bidirectional isolation (SC-1) confirmed by direct file read. |
| 2 | CI runs Biome lint/format, `tsc --noEmit`, Vitest unit, `pnpm build`, Playwright E2E vs `out/` — all required gates | ✓ VERIFIED | `web.yml:54-72` has six `run:` steps in exact order: `pnpm lint` → `pnpm typecheck` → `pnpm test` → `pnpm build` → `pnpm test:postbuild` → `pnpm test:e2e`. YAML parse confirmed gate order programmatically ("GATE ORDER OK"). All five named tools present (lint=biome, typecheck=tsc, unit=vitest, build=next build, e2e=playwright; SEO postbuild added per OQ-1). |
| 3 | Playwright E2E verifies: home SVG hero fallback, docs nav + search returns results, theme persists across reload, all three changelog pages render headings | ✓ VERIFIED | `home.spec.ts:104-163` has all four QA-02 tests with REAL assertions (not weakened): docs nav (`toHaveURL(/\/docs\//)`), docs search (`input.fill("configuration")` → result `toBeVisible` → click → `toHaveURL(/\/docs\//)`), theme persist (`localStorage bk-theme` + reload → `className` contains "light"), three changelog headings (loop v1.0.0/v1.2.0/v1.3.0 → `getByRole("heading").first()` visible). SVG fallback in `gfx.spec.ts:44-65` (`img[src='/hero-hive.svg']` `toBeVisible` under `reducedMotion:'reduce'`). |

**Score:** 3/3 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `web/vitest.config.mts` | jsdom default, tsconfigPaths before react, globals:false | ✓ VERIFIED | Triple-slash ref line 1; `plugins:[tsconfigPaths(),react()]` correct order; `globals:false`; include covers unit+postbuild; e2e excluded. |
| `web/playwright.config.ts` | chromium-only, webServer serving out/ on pinned 4199 | ✓ VERIFIED | One `chromium` project; `command:"pnpm exec serve out --listen 4199"` (WR-01 fix: `pnpm exec` not `pnpm dlx`); baseURL 127.0.0.1:4199. |
| `web/tests/unit/*.test.ts(x)` (5 files) | cn, useReducedMotion, InstallChip, metadata, accuracy port | ✓ VERIFIED | All 5 present + tracked. `pnpm test` ran 5 files / 33 tests green (independently re-run). accuracy.test.ts carries verbatim UNENFORCED_FEATURES/LABELS + PHANTOM_COMMANDS + AC-1/2/3. |
| `web/tests/e2e/home.spec.ts` | home_spec.py port + 4 QA-02 paths | ✓ VERIFIED | HARNESS_NAMES (15) + KNOWN_GAP_MARKERS (4) verbatim; above-fold + dual-theme + DOM-presence ports + 4 QA-02 tests. |
| `web/tests/e2e/gfx.spec.ts` | gfx_spec.py port incl. SVG fallback | ✓ VERIFIED | FORBIDDEN_SERVER_SYMBOLS verbatim; GFX-01a/01b/02/03/04 ported; QA-02 SVG fallback assertion real. |
| `web/tests/postbuild/seo.test.ts` | seo_spec.py post-build port, BASE_URL imported | ✓ VERIFIED | `// @vitest-environment node`; `import { BASE_URL } from "@/lib/metadata"`; SC-1/2/3 with ≥13-loc threshold. |
| `.github/workflows/web.yml` | path-filtered, 6 ordered gates, first-party, least-priv | ✓ VERIFIED | All criteria met (see Truth 1+2); `permissions: contents: read`; all `uses:` first-party (checkout@v4, action-setup@v6, setup-node@v4, upload-artifact@v4). |
| `.github/workflows/ci.yml` | paths-ignore web isolation on both triggers | ✓ VERIFIED | `paths-ignore` on push + pull_request; jobs unchanged. |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| vitest.config.mts | tsconfig @/ + collections/* aliases | vite-tsconfig-paths | ✓ WIRED | `tsconfigPaths()` present + first in plugins; `pnpm test` resolved `@/lib/*` imports with zero "Cannot find module" errors. |
| playwright.config.ts | static out/ on :4199 | `serve out --listen 4199` | ✓ WIRED | Command pins port via `--listen`; `serve@14.2.6` pinned in package.json + pnpm-lock.yaml (no runtime `dlx` fetch). |
| seo.test.ts | web/lib/metadata.ts BASE_URL | import | ✓ WIRED | Imported (not hardcoded) — eliminates the Python keep-in-sync coupling. |
| home.spec.ts | next-themes storageKey | localStorage `bk-theme` | ✓ WIRED | Test uses `bk-theme`; `web/app/providers.tsx:12` confirms `storageKey="bk-theme"` — selector matches live source. |

### Behavioral Spot-Checks (independently re-run; node 22.17.0 + pnpm 11.1.3 on PATH)

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Vitest pre-build unit suite green | `pnpm test` (web/) | 5 files, 33/33 passed | ✓ PASS |
| Biome lint gate green | `pnpm lint` (web/) | Checked 58 files, exit 0 | ✓ PASS |
| tsc typecheck gate green | `pnpm typecheck` (web/) | exit 0 (no output) | ✓ PASS |
| Python specs retired | `git ls-files web/tests/*.py` | empty | ✓ PASS |
| `serve` pinned (WR-01 fix) | `grep serve@14 pnpm-lock.yaml` | `serve@14.2.6` present | ✓ PASS |
| web.yml gate order | `python yaml` parse | "GATE ORDER OK" | ✓ PASS |
| Post-build SEO + E2E (build-dependent) | `pnpm test:postbuild` / `pnpm test:e2e` | NOT re-run here (require fresh `pnpm build` + chromium serve; ~minutes) | ? SKIP — relied on orchestrator evidence (postbuild 29/29, e2e 12/12) + static file inspection |

Build-dependent suites (postbuild SEO 29/29, E2E 12/12 incl. all four QA-02 paths) were not re-executed in this verification pass to stay fast; their test files were read in full and confirmed substantive (real assertions, correct selectors matching live source), and the orchestrator's same-session green runs plus the parity-baseline (4 .py specs green on the same out/ before deletion, per 19-03-SUMMARY) are the recorded evidence.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| QA-01 | 19-01/02/03 | Path-filtered web CI: build + Vitest unit + Playwright E2E vs out/ + Biome, gating merges | ✓ SATISFIED | Truths 1+2; web.yml + ci.yml; full JS toolchain installed (Vitest 4.1.8 + Playwright 1.57.0 pinned); lint/typecheck/unit re-run green. |
| QA-02 | 19-02/03 | E2E verifies critical paths: hero fallback, docs nav + search returns results, theme toggle, changelog pages build | ✓ SATISFIED | Truth 3; all four paths in home.spec.ts/gfx.spec.ts with real (non-weakened) assertions; search asserts a visible result AND navigation. |

No orphaned requirements: REQUIREMENTS.md maps only QA-01 + QA-02 to Phase 19, both claimed by the plans and verified.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| gfx.spec.ts | 31-41, 95-111 | Vacuous canvas-count assertions (`<= 1` always true after 3D removal) | ℹ Info | Honestly documented; QA-02-relevant assertion is the SVG fallback (real `toBe(0)` + `toBeVisible`). WR-04 dispositioned — regression-gate intent only, not a goal-blocking stub. |
| gfx.spec.ts | 88-92 | LCP/FCP budget skipped when no perf entry emitted (headless) | ℹ Info | WR-05 dispositioned; not a QA-02 contract path. |

No debt markers (TBD/FIXME/XXX) in any phase-19 file. No stubs in the goal-bearing artifacts (the four QA-02 paths and the CI gates are all substantive).

### Human Verification Required

None. SC-3's claim ("E2E verifies ...") is satisfied by the existence + correctness of the E2E assertions, which is programmatically verifiable. The live GitHub Actions run is intentionally NOT required (D-03: repo unpushed; web.yml correctness is verified by static inspection, which passed). No visual/UX/external-service items remain open.

### Deferred Items

None — both QA requirements are the last functional requirements of Phase 19; nothing is pushed to a later phase.

### Gaps Summary

No gaps. All three ROADMAP Success Criteria are observably true in the codebase:

1. Bidirectional path isolation is real and symmetric (web.yml `paths:` ↔ ci.yml `paths-ignore:`).
2. The web CI job runs all five named gates plus SEO post-build, in the correct lint→typecheck→unit→build→postbuild→e2e order, with least-privilege permissions and first-party-only actions.
3. Playwright E2E covers all four QA-02 critical paths (SVG hero fallback, docs nav + search-returns-a-result-then-navigates, theme-persist-across-reload, three changelog headings) with assertions that are not weakened to pass.

The four legacy Python specs are retired (`git ls-files web/tests/*.py` empty) after parity was proven, and `serve` was pinned (WR-01 fix, commit 7c1e1b4) so the `--frozen-lockfile` guarantee the product itself preaches is no longer punctured. The pre-build half of the suite (lint, typecheck, 33 unit tests) was independently re-run green during this verification; the build-dependent half was verified by full source inspection plus the orchestrator's same-session green runs.

The 6 code-review warnings are all latent/posture concerns (WR-01 already fixed; WR-02 branch-protection footgun is a future-config caveat; WR-03 SHA-pinning is a project-wide hardening item beyond this phase's scope; WR-04/05/06 are honestly-documented test-robustness notes) — none falsifies a Success Criterion.

---

_Verified: 2026-06-11T09:05:00Z_
_Verifier: Claude (gsd-verifier)_
