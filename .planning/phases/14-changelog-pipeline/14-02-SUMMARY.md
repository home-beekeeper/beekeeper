---
phase: 14-changelog-pipeline
plan: "02"
subsystem: web/changelog-pipeline
tags: [fumadocs, next.js, mdx, static-export, changelog, breaking-change, playwright]
dependency_graph:
  requires: [14-01-SUMMARY.md]
  provides: [v1.3.0 changelog page, phase-complete gate SC-1/2/3]
  affects:
    - web/content/changelog/v1.3.0/index.mdx
    - web/content/changelog/v1.3.0/meta.json
    - web/content/changelog/meta.json
tech_stack:
  added: []
  patterns:
    - MDX HTML entity escaping for literal ${ sequences (&#123; for { prevents JS template evaluation)
    - BreakingChangeCallout MDX component — var(--red) dual-theme correct (dark rgb(248,81,73) / light rgb(192,57,43))
    - Playwright-on-static-export dual-theme visual check (python -m http.server + chromium)
key_files:
  created:
    - web/content/changelog/v1.3.0/meta.json
    - web/content/changelog/v1.3.0/index.mdx
  modified:
    - web/content/changelog/meta.json (v1.3.0 added first — newest-first order)
decisions:
  - "MDX evaluates \${...} as a JavaScript expression — literal dollar-brace sequences in prose must use HTML entity &#123; for { or avoid the brace form entirely (e.g. $USERPROFILE not ${USERPROFILE})"
  - "v1.3.0 callout states only Claude Code is live-verified; other 14 harnesses are contract-shape tested — no overclaim"
  - "Playwright localStorage approach: navigate to the page first to establish origin, then set bk-theme + reload — avoids SecurityError from about:blank localStorage access"
metrics:
  duration: "~20 minutes"
  completed: "2026-06-08"
  tasks_completed: 2
  files_created: 2
  files_modified: 1
---

# Phase 14 Plan 02: Changelog Pipeline — v1.3.0 Notes + Phase-Complete Gate

**One-liner:** v1.3.0 changelog page authored with accurate exit-1→exit-2 breaking-change callout (BreakingChangeCallout, red in both themes via Playwright), VerifyCommands + ReleaseLinks wired; pnpm build emits all three changelog static pages; SC-1/2/3 phase-complete gate PASSED.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Author v1.3.0 release notes with red breaking-change callout | 9a2ae0b | web/content/changelog/v1.3.0/meta.json, web/content/changelog/v1.3.0/index.mdx, web/content/changelog/meta.json |
| 2 | Phase-complete validation gate — SC-1/2/3 confirmed | cb7ee3f | web/content/changelog/v1.3.0/index.mdx (MDX escape fix) |

## Verification

### SC-1: All three changelog pages emitted as separate static files

| File | Status | Size (non-empty) |
|------|--------|-----------------|
| `out/changelog/v1.0.0/index.html` | FOUND | yes |
| `out/changelog/v1.2.0/index.html` | FOUND | yes |
| `out/changelog/v1.3.0/index.html` | FOUND | yes |

`pnpm build` exit code: **0**. Next.js build output confirmed all three `/changelog/[[...slug]]` routes statically generated.

### SC-2: Verification commands + release links on all three pages

| Page | `Bantuson/beekeeper` matches | `slsa-verifier verify-artifact` matches | release link matches |
|------|-------|------|------|
| v1.0.0 | 2 | 1 | 2 (`/releases/tag/v1.0.0`) |
| v1.2.0 | 2 | 1 | 2 (`/releases/tag/v1.2.0`) |
| v1.3.0 | 2 | 1 | 2 (`/releases/tag/v1.3.0`) |

All confirmed by `grep -c` on rendered HTML.

### SC-3: Red breaking-change callout on v1.3.0 only, theme-correct in both themes

**Callout keyword presence in `out/changelog/v1.3.0/index.html`:**

| Keyword | Count |
|---------|-------|
| `exit code 1` | 2 |
| `--hook` | 8 |
| `hooks install` | 2 |
| `restart` | 2 |

**Playwright-on-static-export dual-theme visual check (`python -m http.server 3099 --directory out` + chromium):**

| Theme | `border-left-color` (computed) | `background-color` (computed) | Result |
|-------|-------------------------------|-------------------------------|--------|
| dark | `rgb(248, 81, 73)` | `color(srgb 0.97 0.32 0.29 / 0.1)` | **PASS** (red) |
| light | `rgb(192, 57, 43)` | `color(srgb 0.75 0.22 0.17 / 0.1)` | **PASS** (red) |

Both themes resolve `var(--red)` to a distinctly red RGB color — proving the callout is prominently colored and not transparent or theme-broken.

**Callout absent from v1.0.0 and v1.2.0:**

| File | `exit code 1` count |
|------|---------------------|
| `out/changelog/v1.0.0/index.html` | 0 |
| `out/changelog/v1.2.0/index.html` | 0 |

Phase-complete gate: **PHASE_GATE_PASS**

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] MDX `${VAR}` template expression interpolation at build time**

- **Found during:** Task 2 — first `pnpm build` attempt
- **Issue:** `pnpm build` failed with `ReferenceError: VAR is not defined` during `/api/search` page data collection. The v1.3.0 index.mdx had `${USERPROFILE}` in prose (outside a code span), which MDX/Turbopack evaluated as a JavaScript template expression `${ USERPROFILE }` — looking up the variable `USERPROFILE` in JS scope, which does not exist.
- **Fix:** Escaped the heading to `$VAR/$&#123;VAR&#125;` (HTML entity for `{`) and rewrote the prose example from `${USERPROFILE}/.aws/credentials` to `$USERPROFILE/.aws/credentials` (no braces). The `${VAR}` remaining in a backtick code span on the same section is safe — MDX treats backtick spans as raw text.
- **Files modified:** `web/content/changelog/v1.3.0/index.mdx`
- **Commit:** cb7ee3f

## Known Stubs

None. All content is accurate human-written release notes sourced from Phase-10 seed history and MILESTONES.md.

## Threat Flags

T-14-06 mitigation confirmed: The breaking-change callout accurately states BEFORE (exit 1 + silent allow), AFTER (exit 2 + per-harness deny contract), and MIGRATION steps (upgrade → `beekeeper hooks install --hook <harness>` → restart session). The callout explicitly states only Claude Code is live-verified; other harnesses are contract-shape tested. No overclaim.

T-14-07 mitigation confirmed: Notes describe only shipped behavior; no secrets, no unreleased internals.

T-14-08 mitigation confirmed: `VerifyCommands version="v1.3.0"` reuses the capital-B Bantuson component — no per-page command drift possible. Confirmed 2 matches of `Bantuson/beekeeper` in rendered HTML.

T-14-09 mitigation confirmed: No new npm dependencies. `web/package.json` and `pnpm-lock.yaml` unchanged.

## Self-Check: PASSED

Files verified:

- `web/content/changelog/v1.3.0/meta.json` — FOUND (`{ "title": "v1.3.0", "pages": ["index"] }`)
- `web/content/changelog/v1.3.0/index.mdx` — FOUND (contains `<BreakingChangeCallout`, `<VerifyCommands version="v1.3.0"`, `<ReleaseLinks version="v1.3.0"`)
- `web/content/changelog/meta.json` — FOUND (`"pages": ["v1.3.0", "v1.2.0", "v1.0.0"]` — newest-first)
- `out/changelog/v1.0.0/index.html` — FOUND
- `out/changelog/v1.2.0/index.html` — FOUND
- `out/changelog/v1.3.0/index.html` — FOUND

Commits verified:

- `9a2ae0b` — FOUND (feat(14-02): author v1.3.0 changelog with red breaking-change callout)
- `cb7ee3f` — FOUND (fix(14-02): escape \${VAR} in MDX to prevent JS template interpolation)

Phase-complete gate: PASSED (pnpm build exits 0; all three out/changelog/vX.Y.Z/index.html emitted; SC-1/2/3 verified)
