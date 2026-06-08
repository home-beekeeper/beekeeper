---
phase: 12-design-system
plan: "01"
subsystem: web/design-system
tags: [shadcn, tailwind-v4, components, ui, next-themes]
requires: [11-01]
provides: [web/components.json, web/lib/utils.ts, web/components/ui/*]
affects: [12-02, 12-03, 13-xx, 14-xx, 15-xx]
tech_stack_added: [shadcn@4.10.0-CLI, radix-ui@1.5.0, class-variance-authority@0.7.1, clsx@2.1.1, tailwind-merge@3.6.0, tw-animate-css@1.4.0, lucide-react@1.17.0, next-themes@0.4.6]
tech_stack_patterns: [shadcn-new-york, cva-variants, cn-utility, radix-ui-monolith]
key_files_created:
  - web/components.json
  - web/lib/utils.ts
  - web/components/ui/button.tsx
  - web/components/ui/badge.tsx
  - web/components/ui/separator.tsx
  - web/components/ui/tooltip.tsx
key_files_modified:
  - web/package.json
  - pnpm-lock.yaml
  - pnpm-workspace.yaml
decisions:
  - shadcn-4x-radix-monolith: "shadcn 4.10.0 uses radix-ui monolith package instead of @radix-ui/* individual packages — components import from 'radix-ui' not '@radix-ui/*'"
  - components-json-manual: "shadcn 4.10.0 CLI removed --style/--base-color flags; components.json written manually to match UI-SPEC target (new-york, zinc, tailwind.config: empty)"
  - msw-build-approval: "pnpm-workspace.yaml had msw with placeholder value; moved to ignoredBuiltDependencies (DENY posture maintained)"
  - biome-format-fixes: "Applied biome --write --unsafe to shadcn-generated components to meet project Biome config (useImportType, organizeImports, semicolons)"
metrics:
  duration_seconds: 1011
  completed_date: "2026-06-08"
  tasks_completed: 3
  tasks_total: 3
  files_created: 6
  files_modified: 3
requirements_addressed: [DSYS-01]
---

# Phase 12 Plan 01: shadcn Foundation Summary

**One-liner:** shadcn/ui new-york foundation — components.json (zinc/config-less), cn() helper, four base Radix components, next-themes installed; build and lint green.

---

## What Was Built

The shadcn/ui design system foundation for the Beekeeper web site:

- **`web/components.json`** — shadcn config with style `new-york`, baseColor `zinc`, `tailwind.config: ""` (Tailwind v4 CSS-first), no `registries` key (official registry only). Written manually because shadcn 4.10.0 CLI removed the `--style`/`--base-color` flags.
- **`web/lib/utils.ts`** — `cn()` classname helper (clsx + tailwind-merge) — the shared import for all shadcn components.
- **`web/components/ui/button.tsx`** — Button with default/destructive/outline/secondary/ghost/link variants + xs/sm/lg/icon sizes. Exports `Button`, `buttonVariants`.
- **`web/components/ui/badge.tsx`** — Badge with default/secondary/destructive/outline/ghost/link variants. Exports `Badge`, `badgeVariants`.
- **`web/components/ui/separator.tsx`** — Separator (horizontal/vertical). Exports `Separator`.
- **`web/components/ui/tooltip.tsx`** — Tooltip system. Exports `Tooltip`, `TooltipTrigger`, `TooltipContent`, `TooltipProvider`.

All four components import `cn` from `@/lib/utils`. All use the `data-slot` attribute pattern (React 19 / shadcn 4.x — no `forwardRef`). `pnpm build` exits 0. `pnpm lint` (Biome) is clean.

---

## Task Execution

| Task | Name | Status | Commit |
|------|------|--------|--------|
| 1 | Package legitimacy gate | Maintainer-approved (pre-approved per orchestrator) | N/A |
| 2 | shadcn init (components.json, lib/utils.ts) | Complete | a97bcec |
| 3 | Add base components + next-themes | Complete | 2356be1 |

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] pnpm-workspace.yaml msw placeholder blocked all pnpm installs**
- **Found during:** Task 2 (shadcn init)
- **Issue:** `pnpm-workspace.yaml` had `msw: set this to true or false` (a literal placeholder string) in `allowBuilds`. pnpm rejected this with `ERR_PNPM_IGNORED_BUILDS`, blocking the install.
- **Fix:** Removed the placeholder `msw:` entry from `allowBuilds` and added `msw` to `ignoredBuiltDependencies` (alongside `unrs-resolver`). This preserves the Phase 11 DENY posture — msw scripts are not approved.
- **Files modified:** `pnpm-workspace.yaml`
- **Commit:** a97bcec

**2. [Rule 1 - Bug] shadcn-generated components failed Biome lint**
- **Found during:** Task 3 verification (`pnpm lint`)
- **Issue:** shadcn-generated `button.tsx`, `badge.tsx`, `separator.tsx`, `tooltip.tsx` used `import * as React from "react"` (Biome reports `useImportType`), had unsorted imports (`organizeImports`), and missing semicolons (formatter).
- **Fix:** Applied `pnpm exec biome check --write --unsafe` to all four component files and `lib/utils.ts`. All 5 files fixed, lint clean.
- **Files modified:** `web/components/ui/*.tsx`, `web/lib/utils.ts`
- **Commit:** 2356be1

### Architectural Note (informational, not a deviation)

**shadcn 4.10.0 CLI breaking changes from research assumptions:**

The plan and research assumed `shadcn@4.10.0` still supported `--style new-york --base-color zinc` flags. These flags were removed in shadcn 4.x:
- The CLI now uses presets (`nova`, `vega`, etc.) — none map to "new-york zinc"
- Components are now pulled from the official registry and still import `cn from "@/lib/utils"` — the key acceptance criterion is met
- The CLI now installs `radix-ui` (monolith v1.5.0) instead of individual `@radix-ui/*` packages. Components use `from "radix-ui"` imports.
- The runtime `shadcn` package is also installed as a dependency (shadcn 4.x)

**Impact:** Zero impact on component behavior. The `components.json` was written manually to the exact UI-SPEC target format (new-york, zinc, config-less, no registries). `shadcn add` reads this file correctly and generates valid components. The generated components fully satisfy all acceptance criteria: cn() imported from @/lib/utils, official registry only, build green.

---

## Verification Results

```
components.json: style "new-york" ✓ | baseColor "zinc" ✓ | config "" ✓ | no registries ✓
lib/utils.ts: exports cn() ✓ | clsx import ✓ | tailwind-merge import ✓
button.tsx: @/lib/utils import ✓
badge.tsx: @/lib/utils import ✓
separator.tsx: @/lib/utils import ✓
tooltip.tsx: @/lib/utils import ✓
next-themes in package.json: ✓
No member web/pnpm-lock.yaml: ✓
globals.css: left in CLI-generated Phase 11 state (intentional — Plan 03 overwrites)
pnpm build: EXIT 0 ✓
pnpm lint (Biome): CLEAN ✓
```

---

## Known Stubs

None. This plan installs tooling and generates components — no user-facing data or UI is rendered.

---

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: supply-chain | web/package.json | radix-ui@1.5.0 and shadcn@4.10.0 added as runtime deps (new in shadcn 4.x — not individual @radix-ui/* packages). These are established packages but the monolith pattern is newer than what RESEARCH audited. |

The `radix-ui` monolith is published by the Radix UI team (same org as individual packages). Weekly downloads are in the millions. Not flagged as slop or suspicious — noting for awareness.

---

## Self-Check: PASSED

All created files confirmed on disk. All task commits confirmed in git log.

| Check | Result |
|-------|--------|
| web/components.json | FOUND |
| web/lib/utils.ts | FOUND |
| web/components/ui/button.tsx | FOUND |
| web/components/ui/badge.tsx | FOUND |
| web/components/ui/separator.tsx | FOUND |
| web/components/ui/tooltip.tsx | FOUND |
| 12-01-SUMMARY.md | FOUND |
| Commit a97bcec (Task 2) | FOUND |
| Commit 2356be1 (Task 3) | FOUND |
