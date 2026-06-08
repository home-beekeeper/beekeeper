---
phase: 13-docs-content-pipeline
plan: "01"
subsystem: web/toolchain
tags: [fumadocs, next.js, mdx, toolchain, static-export]
dependency_graph:
  requires: [12-03-SUMMARY.md]
  provides: [fumadocs-mdx toolchain, .source/ codegen, Wave 0 build gate]
  affects: [web/next.config.mjs, web/source.config.ts, web/tsconfig.json, web/package.json]
tech_stack:
  added: [fumadocs-ui@16.9.3, fumadocs-core@16.9.3, fumadocs-mdx@15.0.11, "@types/mdx@2.0.14"]
  patterns: [createMDX wrapper, defineDocs collection config, tsconfig path alias]
key_files:
  created: [web/next.config.mjs, web/source.config.ts, web/content/docs/.gitkeep]
  modified: [web/package.json, web/tsconfig.json, pnpm-lock.yaml, pnpm-workspace.yaml]
decisions:
  - "esbuild@0.28.0 approved as transitive dep of fumadocs-mdx (allowBuilds:true in pnpm-workspace.yaml — installs platform-specific binary; well-known package)"
  - "next.config.mjs uses JSDoc @type annotation (not import type) — .mjs extension resolves fumadocs-mdx ESM import on Node 22.17.0"
  - "content/docs/.gitkeep added — git does not track empty dirs; fumadocs-mdx codegen handles empty/absent content gracefully"
metrics:
  duration: "~15 minutes"
  completed: "2026-06-08"
  tasks_completed: 3
  files_created: 3
  files_modified: 4
---

# Phase 13 Plan 01: Fumadocs Toolchain Wiring Summary

**One-liner:** fumadocs-mdx pipeline wired into Next.js 16 static-export via createMDX wrapper, source.config.ts collection config, and tsconfig collections/* alias — Wave 0 build gate PASSED.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Install pinned fumadocs deps + postinstall script | 39aa5c8 | web/package.json, pnpm-lock.yaml, pnpm-workspace.yaml |
| 2 | Rename next.config.ts → next.config.mjs with createMDX | 647937b | web/next.config.mjs (web/next.config.ts deleted) |
| 3 | Create source.config.ts + tsconfig alias + build-verify Wave 0 | 5d99677 | web/source.config.ts, web/tsconfig.json, web/content/docs/.gitkeep |

## Verification

Wave 0 gate (per 13-VALIDATION.md):

- `pnpm install --frozen-lockfile` — PASSED (exits 0, non-interactive)
- `pnpm build` — PASSED (exits 0; Turbopack compilation + TypeScript check green)
- `web/.source/` generated — PASSED (browser.ts, server.ts, dynamic.ts, source.config.mjs present)
- `web/next.config.mjs` config keys preserved — PASSED (verified via Node ESM import: output=export, trailingSlash=true, images.unoptimized=true, transpilePackages=["three","@react-three/fiber","@react-three/drei"])
- `web/next.config.ts` deleted — PASSED
- `web/tsconfig.json` collections/* alias present — PASSED

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] esbuild build-script approval gate**
- **Found during:** Task 1 (first `pnpm --filter web add ...` invocation)
- **Issue:** pnpm's build-approval gate fired for `esbuild@0.28.0` (transitive dep of fumadocs-mdx). The install exited 1 with `ERR_PNPM_IGNORED_BUILDS` and pnpm partially wrote `pnpm-workspace.yaml` with a placeholder `esbuild: set this to true or false`.
- **Fix:** Set `allowBuilds: esbuild: true` in `pnpm-workspace.yaml`. esbuild is a well-known bundler (>20M wk downloads, github.com/evanw/esbuild) whose postinstall script installs a platform-specific prebuilt binary — identical pattern to the already-approved `unrs-resolver`. Retry install succeeded on second run.
- **Files modified:** `pnpm-workspace.yaml` (esbuild allowBuilds entry added)
- **Commit:** 39aa5c8 (included in Task 1 commit)
- **Note:** Research (§Pattern 10, §pnpm-workspace.yaml build-approval gate) stated "none of the three fumadocs packages have postinstall scripts" — this is accurate for the direct deps. The deviation was a transitive dep (esbuild, pulled by fumadocs-mdx). The research note about no changes to pnpm-workspace.yaml was therefore not fully accurate for the transitive graph.

**2. [Rule 2 - Missing] content/docs/ empty directory not tracked**
- **Found during:** Task 3 (staging)
- **Issue:** `web/content/docs/` was created as an empty directory to satisfy fumadocs-mdx codegen, but git does not track empty directories. Without a tracked file, fresh clones would be missing the directory and fumadocs-mdx might fail on `dir: 'content/docs'` at build time.
- **Fix:** Added `web/content/docs/.gitkeep` so the directory is preserved in git history for clones.
- **Files modified:** `web/content/docs/.gitkeep` (new)
- **Commit:** 5d99677 (included in Task 3 commit)

### Lockfile Deviation (expected)

- **Plan listed:** `web/pnpm-lock.yaml`
- **Actual:** `pnpm-lock.yaml` (repo root) — the authoritative lockfile in pnpm workspace mode (matches Phase 11 decision, documented in STATE.md and plan's `read_first` note)

## Known Stubs

None — this plan creates toolchain config only. No UI rendering, no data stubs.

## Threat Flags

None — supply-chain surface bounded to four well-known fumadocs packages + esbuild (transitive). All installed under frozen lockfile. T-13-01 and T-13-02 mitigations applied as planned.

## Self-Check: PASSED

Files verified:
- web/next.config.mjs — FOUND
- web/source.config.ts — FOUND
- web/tsconfig.json (collections/* alias) — FOUND
- web/package.json (postinstall + 4 deps) — FOUND
- web/content/docs/.gitkeep — FOUND
- web/next.config.ts — ABSENT (correctly deleted)

Commits verified:
- 39aa5c8 — FOUND (chore: install fumadocs deps)
- 647937b — FOUND (feat: next.config.mjs rename)
- 5d99677 — FOUND (feat: source.config.ts + alias + build verify)

Wave 0 build gate: PASSED (pnpm build exits 0; .source/ generated)
