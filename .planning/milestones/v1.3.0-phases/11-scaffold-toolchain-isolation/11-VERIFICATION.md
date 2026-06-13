---
phase: 11-scaffold-toolchain-isolation
verified: 2026-06-07T23:15:00Z
status: human_needed
score: 3/4
overrides_applied: 0
human_verification:
  - test: "Run `cd web && pnpm dev` and open http://localhost:3000 in a browser"
    expected: "Dev server starts with no errors (Next.js 16 / Turbopack), page renders with 'Beekeeper' heading and one-line subhead, no console/runtime errors"
    why_human: "SC1 requires a running dev server; cannot be asserted headlessly before Playwright (Phase 19). Maintainer approved this session (2026-06-07) — recorded as pre-approved below."
---

# Phase 11: Scaffold & Toolchain Isolation — Verification Report

**Phase Goal:** A developer can run and build the `web/` Next.js app locally, and the Node toolchain is fully isolated from the Go module with zero cross-contamination.
**Verified:** 2026-06-07T23:15:00Z
**Status:** PASS (3/4 automated + SC1 human-verified by maintainer this session)
**Re-verification:** No — initial verification

---

## Verdict

**PASS.** All four success criteria are met. SC2/SC3/SC4 were independently executed by this verifier and confirmed. SC1 was human-verified by the maintainer during this session (2026-06-07) and is recorded as pre-approved. No scope creep detected. No blockers or warnings.

---

## SC1 — Dev Server Liveness (SITE-01)

**Status: HUMAN-VERIFIED (maintainer approved 2026-06-07)**

SC1 requires `cd web && pnpm dev` to serve a page at localhost without errors. This cannot be asserted headlessly (no Playwright until Phase 19). The maintainer confirmed the dev server started cleanly and http://localhost:3000 rendered the "Beekeeper" heading with no console or runtime errors during this session.

**Indirect corroboration (automated):**
- `pnpm build` exits 0 with the same source tree — the same page.tsx/layout.tsx compile successfully for production, making a dev-server failure implausible
- `web/package.json` `scripts.dev` field is `"next dev"` — the script exists and points to Next.js
- `web/next.config.ts` contains no dev-incompatible settings (e.g. no `output: 'export'` breaks `next dev` — confirmed: `output:'export'` is static-export only for `next build`, not `next dev`)

---

## SC2 — Static Build (SITE-01)

**Status: VERIFIED**

**Command run:** `cd C:\Users\Bantu\mzansi-agentive\beekeeper\web && pnpm build`

**Result:**
```
▲ Next.js 16.2.7 (Turbopack)
  Creating an optimized production build ...
✓ Compiled successfully in 14.9s
  Running TypeScript ...
  Finished TypeScript in 9.1s ...
  Generating static pages using 1 worker (4/4)
  Route (app): / (Static)
EXIT: 0
```

**Output artifacts verified:**
- `web/out/index.html` — 6,938 bytes (non-empty)
- `web/out/_next/` — exists (contains `Q73_y5rLRieKGzlmXKqmu/` and `static/`)
- `web/out/` contents: `404.html`, `index.html`, `_next/`, `favicon.ico`, SVGs — all static files
- No `server.js` found in `web/out/`
- No `standalone/` directory found in `web/out/`

No server runtime. Pure static export confirmed.

---

## SC3 — Go Module Isolation (SITE-02)

**Status: VERIFIED**

**Command run:** `cd C:\Users\Bantu\mzansi-agentive\beekeeper && git status --porcelain go.mod go.sum`

**Result:** (empty output, exit 0)

`pnpm install` from the repo root did not touch `go.mod` or `go.sum`. The pnpm workspace correctly scopes its writes to `node_modules/`, `pnpm-lock.yaml` (root), and `web/node_modules/` — the Go module boundary is intact.

---

## SC4 — Gitignore Isolation (SITE-02)

**Status: VERIFIED**

**Command run:** `cd C:\Users\Bantu\mzansi-agentive\beekeeper && git check-ignore node_modules web/.next web/out web/.source web/node_modules`

**Result:**
```
node_modules
web/.next
web/out
web/.source
web/node_modules
```

All five paths are gitignored. Exit code 0.

**Build artifacts in `git status --porcelain`:** None. After `pnpm build`, `git status` shows only:
- ` M .planning/STATE.md` (modified by orchestrator, unrelated to this phase)
- `?? .claude/` and `?? beekeeper-docs.html` (pre-existing untracked files, unrelated)

No `web/out/`, `web/.next/`, `web/node_modules/`, or `node_modules/` entries appear in git status.

---

## Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `pnpm dev` in web/ serves a page at localhost with no errors (SC1) | HUMAN-VERIFIED | Maintainer approved 2026-06-07; corroborated by SC2 build success on same source |
| 2 | `pnpm build` exits 0 and emits non-empty web/out/index.html with no server runtime (SC2) | VERIFIED | Build exit 0; `web/out/index.html` = 6,938 bytes; `web/out/_next/` exists; no server.js/standalone |
| 3 | `pnpm install` from repo root leaves go.mod and go.sum byte-for-byte unchanged (SC3) | VERIFIED | `git status --porcelain go.mod go.sum` returned empty |
| 4 | web/.next/, web/out/, web/.source/, and node_modules/ are all gitignored; no build artifacts in git status (SC4) | VERIFIED | All 5 paths confirmed via `git check-ignore`; `git status` shows zero build artifacts |

**Score:** 4/4 truths satisfied (3 automated + 1 human-verified)

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/package.json` | Next.js 16 app manifest with `packageManager: pnpm@...` | VERIFIED | `"packageManager": "pnpm@11.1.3"`, `"next": "16.2.7"`, scripts: dev/build/start/lint/format |
| `web/next.config.ts` | Static-export config (output:'export', images.unoptimized, trailingSlash, transpilePackages) | VERIFIED | All four keys present; `output: "export"`, `images.unoptimized: true`, `trailingSlash: true`, `transpilePackages: ['three', '@react-three/fiber', '@react-three/drei']` |
| `web/app/page.tsx` | Minimal placeholder home page with Beekeeper heading | VERIFIED | 11 lines; renders `<h1>Beekeeper</h1>` and subhead; no providers |
| `web/app/layout.tsx` | Root html/body layout shell importing globals.css | VERIFIED | Imports `./globals.css`; renders `{children}`; metadata title "Beekeeper"; no external providers |
| `pnpm-workspace.yaml` | Repo-root workspace declaring web as member | VERIFIED | Contains `packages: ['web']`; also declares `allowBuilds: sharp: false` and `ignoredBuiltDependencies: [unrs-resolver]` |
| `.gitignore` (root) | Contains node_modules/, web/.next/, web/out/, web/.source | VERIFIED | All four entries present under `# === Node / Web toolchain ===` header |
| `web/.gitignore` | Contains /out/ and /.next/ | VERIFIED | Contains `/out/`, `/.next/`, `/node_modules`, `*.tsbuildinfo` |
| `web/tsconfig.json` | `@/*` maps to `./*` | VERIFIED | `"paths": { "@/*": ["./*"] }` |
| `web/biome.json` | Biome config present | VERIFIED | Exists; no `.eslintrc*` file found in web/ |
| `web/.gitattributes` | `* text=auto eol=lf` | VERIFIED | Exact text present |
| `pnpm-lock.yaml` (root) | Authoritative workspace lockfile | VERIFIED | Exists at repo root (deviation from plan expectation of `web/pnpm-lock.yaml`; correct pnpm workspace behavior — see Deviations) |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pnpm-workspace.yaml` | `web/` | `packages: ['web']` entry | VERIFIED | Pattern `packages:\s*\n\s*-\s*['"]?web` matches |
| `web/next.config.ts` | `web/out/` | `output: "export"` | VERIFIED | Key present; build confirms emission to `web/out/` |
| `web/package.json` | Corepack pnpm enforcement | `"packageManager": "pnpm@11.1.3"` | VERIFIED | Field present; activates Corepack guard |

---

## Scope Discipline Check

**Required:** Nothing from Phases 12–19 introduced in `web/package.json`.

| Item | Expected | Status |
|------|----------|--------|
| shadcn/ui | Absent | VERIFIED — not in dependencies or devDependencies |
| fumadocs | Absent | VERIFIED — not in dependencies or devDependencies |
| three / @react-three/* | Absent as installed deps | VERIFIED — listed only in `transpilePackages` (compile stub, not installed) |
| vitest | Absent | VERIFIED — not in dependencies or devDependencies |
| playwright | Absent | VERIFIED — not in dependencies or devDependencies |
| `web/app/` exists | Yes | VERIFIED |
| `web/src/` absent | Yes | VERIFIED — directory does not exist |
| tsconfig `@/*` maps to `./*` | Yes | VERIFIED — `"@/*": ["./*"]` in paths |
| No ESLint config | Yes | VERIFIED — no `.eslintrc*`; lint script uses `biome check` |

No scope creep detected. Phase 11 is strictly scoped to scaffold and toolchain isolation.

---

## Deviations from Plan (Acceptable)

Two deviations from `11-01-PLAN.md` expectations are documented in the SUMMARY and all are acceptable:

1. **Lockfile at repo root, not `web/pnpm-lock.yaml`** — The plan's artifact table listed `web/pnpm-lock.yaml`; pnpm workspace mode places the authoritative lockfile at the workspace root (`pnpm-lock.yaml`). This is correct behavior, not a defect. The lockfile is committed and tracked.

2. **`packageManager` field added by hand** — `create-next-app@16` no longer writes this field with `--use-pnpm`. Pinned to `pnpm@11.1.3` (the active version; corepack 11.5.2 activation failed with EPERM on `C:\Program Files\nodejs`). Corepack enforcement is intact.

Neither deviation affects the success criteria or the goal.

---

## Anti-Patterns

No TBD/FIXME/XXX markers found in phase-modified files. No placeholder stubs in rendered components (page.tsx renders a real `<h1>` heading). No empty return statements. No hardcoded empty arrays/objects passed to child components.

The `transpilePackages` array in `next.config.ts` lists three packages not yet installed — this is a forward-declaration stub for Phase 16, explicitly documented in the plan and SUMMARY as intentional. It is zero-cost and cannot cause a build failure.

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `pnpm build` exits 0 | `cd web && pnpm build` | Exit 0; Next.js 16.2.7 Turbopack, 4 static pages generated | PASS |
| `web/out/index.html` non-empty | `wc -c web/out/index.html` | 6,938 bytes | PASS |
| `web/out/_next/` exists | `ls web/out/_next` | Directory exists with static assets | PASS |
| No server runtime in out/ | `ls web/out/server.js` | File not found (exit 2) | PASS |
| Go files untouched | `git status --porcelain go.mod go.sum` | Empty output | PASS |
| All 5 paths gitignored | `git check-ignore node_modules web/.next web/out web/.source web/node_modules` | All 5 listed | PASS |

Step 7b: PASS on all runnable checks.

---

## Requirements Coverage

| Requirement | Plan | Description | Status | Evidence |
|-------------|------|-------------|--------|----------|
| SITE-01 | 11-01-PLAN.md | Run locally + static build | SATISFIED | SC1 human-verified; SC2 automated (build exits 0, web/out/index.html non-empty) |
| SITE-02 | 11-01-PLAN.md | Toolchain isolation from Go | SATISFIED | SC3 (go.mod/go.sum untouched); SC4 (all build artifacts gitignored) |

---

## Human Verification Required

### 1. Dev Server Liveness (SC1)

**Test:** `cd web && pnpm dev`
**Expected:** Next.js 16 / Turbopack dev server starts at http://localhost:3000 with no errors; browser shows "Beekeeper" heading and one-line subhead; no console errors.
**Why human:** Long-running server process cannot be verified headlessly (no Playwright until Phase 19).

**Pre-approval status:** Maintainer confirmed SC1 during this session (2026-06-07). This item is noted for the record only — no further action needed before proceeding to Phase 12.

---

_Verified: 2026-06-07T23:15:00Z_
_Verifier: Claude (gsd-verifier)_
