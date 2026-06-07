---
phase: 11-scaffold-toolchain-isolation
plan: 01
subsystem: infra
tags: [nextjs, pnpm, tailwindcss, biome, typescript, static-export, workspace, web]

# Dependency graph
requires:
  - phase: 10-hook-block-protocol-compliance
    provides: v1.3.0 seed shipped (hook-block, self-protection) — the web milestone foundation
provides:
  - "web/ Next.js 16 static-export app (App Router at web/app/, Tailwind v4 CSS-first, Biome, TS @/* alias)"
  - "Static-export next.config.ts (output:'export', images.unoptimized, trailingSlash, transpilePackages stub for three/r3f/drei)"
  - "Repo-root pnpm-workspace.yaml isolating the Node toolchain from the Go module (single member: web)"
  - "Root pnpm-lock.yaml (authoritative workspace lockfile) + root .gitignore web entries + web/.gitattributes"
  - "Buildable foundation: pnpm dev serves localhost, pnpm build emits non-empty web/out/index.html"
affects: [phase-12-design-system, phase-13-docs, phase-15-marketing-home, phase-16-3d-layer, phase-19-ci]

# Tech tracking
tech-stack:
  added: [next@16.2.7, react@19.2.4, react-dom@19.2.4, typescript@5.9.3, tailwindcss@4.3.0, "@biomejs/biome@2.2.0", "@tailwindcss/postcss@4"]
  patterns: ["App Router at web/app/ (no src/)", "@/* import alias -> web/*", "Tailwind v4 CSS-first (no tailwind.config.js)", "pnpm single-member workspace at repo root", "static export (no server runtime)", "native build scripts denied on install"]

key-files:
  created: [web/package.json, web/next.config.ts, web/tsconfig.json, web/biome.json, web/app/layout.tsx, web/app/page.tsx, web/app/globals.css, pnpm-workspace.yaml, pnpm-lock.yaml, web/.gitattributes, web/public/.gitkeep]
  modified: [.gitignore]

key-decisions:
  - "packageManager pinned to pnpm@11.1.3 (corepack activation of 11.5.2 blocked by EPERM on C:\\Program Files\\nodejs; RESEARCH A3 fallback) — create-next-app@16 no longer writes the field with --use-pnpm, so it was added by hand"
  - "Authoritative lockfile lives at the REPO ROOT (pnpm workspace mode), not web/ — 11-RESEARCH expected web/pnpm-lock.yaml; the stale member lockfile was deleted"
  - "create-next-app@16 generated a web/pnpm-workspace.yaml (build-approval stub) — removed; the workspace is declared once at the repo root"
  - "Native postinstall build scripts DENIED on install (allowBuilds sharp:false + unrs-resolver ignored): unneeded for a static export and avoids running untrusted lifecycle scripts (aligned with Beekeeper's own posture)"
  - "web/.source gitignore entry has no trailing slash so git check-ignore matches it before Phase 13 creates the directory"

patterns-established:
  - "web/ is a self-contained Node workspace; go.mod/go.sum are never touched by pnpm (asserted in Task 3)"
  - "Geist scaffold fonts retained in layout.tsx because globals.css (untouched) references their CSS vars; design-system fonts arrive in Phase 12"

requirements-completed: [SITE-01, SITE-02]

# Metrics
duration: ~22min
completed: 2026-06-07
---

# Phase 11: Scaffold & Toolchain Isolation Summary

**A minimal, buildable, fully Go-isolated Next.js 16 static-export app under `web/` — `pnpm dev` serves localhost, `pnpm build` emits a non-empty static `web/out/`, and `pnpm install` provably never touches the Go module.**

## Performance

- **Duration:** ~22 min (incl. human-verify checkpoint)
- **Started:** 2026-06-07T22:29 (+0200)
- **Completed:** 2026-06-07T22:46 (+0200) + SC1 human-verify
- **Tasks:** 4 (3 auto + 1 human-verify checkpoint)
- **Files modified:** 21 tracked (17 scaffold + 4 workspace/isolation)

## Accomplishments
- Scaffolded `web/` with create-next-app@16.2.7: App Router at `web/app/` (no `src/`), Tailwind v4 CSS-first, Biome (no ESLint), TypeScript with the `@/*` → `./*` alias, pnpm pinned in `packageManager`.
- Configured static export: `output: 'export'` + `images.unoptimized` + `trailingSlash` + a `transpilePackages` stub for three/@react-three/fiber/@react-three/drei (so the Phase 16 3D layer can't break the build).
- Wired a repo-root pnpm workspace isolating the Node toolchain from the Go module; native build scripts denied on install.
- Proved all four success criteria (SC1 human-verified; SC2/SC3/SC4 automated).

## Task Commits

1. **Task 1: create-next-app scaffold** — `b904e21` (feat)
2. **Task 2: static export config + minimal placeholder page/layout** — `3546503` (feat)
3. **Task 3: pnpm workspace isolation + root .gitignore/.gitattributes + Go-isolation verify** — `9b86459` (feat)
4. **Task 4: human-verify `pnpm dev` (SC1)** — no commit (verification checkpoint; maintainer approved)

## Files Created/Modified
- `web/package.json` — Next.js app manifest; `packageManager: pnpm@11.1.3`; scripts dev/build/start/lint/format
- `web/next.config.ts` — static-export config (output:'export', images.unoptimized, trailingSlash, transpilePackages stub)
- `web/tsconfig.json`, `web/biome.json` — TS (`@/*`→`./*`) + Biome (generated)
- `web/app/layout.tsx` — minimal root shell (Geist fonts kept; metadata title "Beekeeper"; no providers)
- `web/app/page.tsx` — minimal "Beekeeper" placeholder (replaced in Phase 15)
- `web/app/globals.css` — Tailwind v4 CSS-first entry (untouched from scaffold; Phase 12 expands)
- `web/public/.gitkeep` + scaffold SVGs — static assets dir
- `pnpm-workspace.yaml` (root) — registers `web` as the single member; denies sharp/unrs-resolver builds
- `pnpm-lock.yaml` (root) — authoritative workspace lockfile
- `web/.gitattributes` — `* text=auto eol=lf` line-ending normalization
- `.gitignore` (root) — node_modules/, web/.next/, web/out/, web/.source

## Decisions Made
See `key-decisions` frontmatter. Summary: pinned pnpm@11.1.3 (corepack 11.5.2 EPERM), root-level lockfile (workspace reality vs RESEARCH), removed create-next-app's misplaced web/pnpm-workspace.yaml, denied native build scripts, no-trailing-slash web/.source ignore.

## Deviations from Plan

create-next-app@16 had moved past the version the RESEARCH `--help` check was run against, producing four reality-driven deviations (all documented in the relevant task commit):

1. **packageManager field added by hand** — `--use-pnpm` no longer writes it. Pinned to the active `pnpm@11.1.3` because `corepack prepare pnpm@11.5.2 --activate` failed with EPERM on `C:\Program Files\nodejs` (RESEARCH Assumption A3 fallback). Committed in `b904e21`.
2. **Build-script approval gate** — create-next-app generated a `web/pnpm-workspace.yaml` with a `sharp: set this to true or false` stub, and pnpm 11.1.3 aborts (exit 1) a non-interactive install until decided. Resolved by deleting the misplaced web file and putting an explicit DENY decision (`allowBuilds: sharp: false`, `ignoredBuiltDependencies: unrs-resolver`) in the root `pnpm-workspace.yaml`. Committed in `9b86459`.
3. **Lockfile location** — pnpm places the lockfile at the workspace ROOT (`pnpm-lock.yaml`), not `web/pnpm-lock.yaml` as RESEARCH assumed. The stale member lockfile was removed; the root lockfile is committed and tracked. Acceptance-criterion "web/pnpm-lock.yaml tracked" is satisfied in spirit by the root lockfile.
4. **web/.source ignore has no trailing slash** — so `git check-ignore web/.source` (SC4) passes before Phase 13's fumadocs-mdx creates the directory.

**Impact on plan:** None negative — all four success criteria met. No scope creep; nothing from Phases 12–19 introduced.

## Issues Encountered
- pnpm 11.1.3 build-approval gate aborted installs on `sharp` (see Deviation 2). Resolved by an explicit deny decision at the workspace root — installs now exit 0 and no untrusted lifecycle scripts run. ESLint did NOT need removing (Biome-only scaffold, Assumption A2 held).

## User Setup Required
None — no external service configuration required. (`pnpm dev` / `pnpm build` run locally; Cloudflare Pages deploy is SITE-03 / Phase 15.)

## Next Phase Readiness
- Buildable, isolated `web/` is in place — Phase 12 (Design System: shadcn/ui + Tailwind v4 + Fumadocs CSS + theme toggle) can begin.
- `transpilePackages` already lists three/r3f/drei for Phase 16.
- `@/*` resolves to `web/*` and the App Router lives at `web/app/` (no `src/`) — Phase 12+ component paths depend on this.
- For Phase 13: fumadocs-mdx current latest is `15.0.11` (not `14.2.11`); `web/.source` is already gitignored.

---
*Phase: 11-scaffold-toolchain-isolation*
*Completed: 2026-06-07*
