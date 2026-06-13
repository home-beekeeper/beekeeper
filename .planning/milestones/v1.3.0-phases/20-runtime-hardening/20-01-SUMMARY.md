---
phase: 20-runtime-hardening
plan: 01
subsystem: infra
tags: [catalog-sync, config, etag, systemd, launchd, schtasks, tui, bubbletea]

requires:
  - phase: 09-tech-debt-cleanup
    provides: layered config merge (mergeNudge/mergeNudgeUntrusted), SourceState/WatchState, catalog.Sync
provides:
  - config.CatalogSyncConfig + DefaultCatalogSyncConfig + ValidateCatalogSyncConfig + CatalogSyncInterval/CatalogSyncEnabled accessors
  - mergeCatalogSync (trusted) + mergeCatalogSyncUntrusted (CSYNC-04 self-defense — refuses disable + interval-loosening)
  - SourceState LastSuccess/LastAttempt/LastError/ETag (omitempty, back-compat)
  - catalog.SyncConditional (If-None-Match list call, 304 short-circuit, last-good-safe) + SyncResult
  - interval-gated `catalogs sync --force` (catalogSyncDue clock seam) recording freshness in state.json
  - `catalogs daemon install|uninstall|status` — unprivileged per-OS (systemd --user / LaunchAgent / current-user schtasks)
  - hooks install first-run sync + daemon offer (CSYNC-06)
  - TUI real sync (runSyncCmd/syncDoneMsg), honest pipColor, cadence selector (validate-before-write)
affects: [20-02, 20-05, catalog freshness, TUI]

tech-stack:
  added: []
  patterns:
    - "Pointer config block + Untrusted merge variant refusing security-relaxing levers (mirrors Nudge)"
    - "Unprivileged user-level OS job (no elevation) modeled on protect_*.go but user-scoped"
    - "OS scheduler is the scheduler; interval gate (catalogSyncDue) keeps the OS schedule static across config changes"

key-files:
  created:
    - cmd/beekeeper/catalogs_daemon.go
    - cmd/beekeeper/catalogs_daemon_linux.go
    - cmd/beekeeper/catalogs_daemon_darwin.go
    - cmd/beekeeper/catalogs_daemon_windows.go
    - cmd/beekeeper/catalogs_daemon_other.go
    - cmd/beekeeper/catalogs_daemon_test.go
    - internal/catalog/sync_test.go
  modified:
    - internal/config/config.go
    - internal/config/layered.go
    - internal/catalog/state.go
    - internal/catalog/sync.go
    - cmd/beekeeper/main.go
    - internal/tui/catalogs_panel.go
    - internal/tui/model.go

key-decisions:
  - "Sync() kept as a thin unconditional wrapper over SyncConditional so the watch-delta caller is unchanged; only the manual/daemon path threads the ETag."
  - "bumblebeeContentsURL changed const→var for httptest substitution (PipePath convention)."
  - "The catalogs sync handler (not catalog.Sync) owns state.json freshness writes — catalog package stays responsible only for the index, matching the Watch-owns-state architecture."
  - "CSYNC-06 hooks-install offer is non-interactive: best-effort first-run sync + printed daemon-registration guidance (daemon stays opt-in via `catalogs daemon install`) — avoids a blocking stdin prompt in a scriptable command."
  - "TUI selector 'admin-gated' interpreted as validate-before-write config mutation (not an OS-admin check) — an OS-admin gate would contradict the unprivileged design."

patterns-established:
  - "catalogSyncDue(lastSuccess, interval, now, force) pure gate — injected-clock testable seam for the OS hourly heartbeat"
  - "mergeCatalogSyncUntrusted refuses enabled:false AND interval-loosening (longer = less frequent = less secure); honors enables + tightenings"

requirements-completed: [CSYNC-01, CSYNC-02, CSYNC-03, CSYNC-04, CSYNC-05, CSYNC-06]

duration: ~95 min
completed: 2026-06-10
---

# Phase 20 Plan 01: Background Catalog Sync + TUI Scheduler (CSYNC) Summary

**ETag-conditional, interval-gated background catalog sync via an unprivileged per-OS scheduler daemon, with a project-layer-can't-disable config lever, freshness timestamps in state.json, and a live TUI sync + cadence selector.**

## Performance

- **Duration:** ~95 min
- **Tasks:** 4
- **Files modified:** 7 modified + 7 created

## Accomplishments
- `CatalogSyncConfig` pointer block with fail-closed validator, defensive `[5h,24h]` clamp accessor, and a low-trust merge that refuses both `enabled:false` and interval-loosening from project/env layers (CSYNC-04 self-defense, proven end-to-end via `LoadLayered`).
- `SourceState` gained `LastSuccess/LastAttempt/LastError/ETag` (omitempty → legacy `state.json` still loads); `catalog.SyncConditional` sends `If-None-Match` on the list call, short-circuits on 304 (no fetch, no rebuild), and preserves the last-good index on any error.
- Interval-gated `catalogs sync` (`--force` bypass) that records freshness in `state.json`, plus an unprivileged `catalogs daemon install|uninstall|status` on all three OSes (systemd `--user` timer / LaunchAgent `StartInterval` / current-user `schtasks` with no SYSTEM run-as and no highest run-level).
- TUI: `s` now runs a real sync (async `runSyncCmd` → `syncDoneMsg` toast); `pipColor` keys off `SourceState` and renders amber (never "fresh") when the last attempt is newer than the last success; `i` cycles the 5h/10h/24h/off cadence persisted via validate-before-write.

## Task Commits

1. **Task 1: CatalogSyncConfig schema + validator + project-can't-disable merge** - `0540e52` (feat)
2. **Task 2: SourceState timestamps+ETag; ETag-conditional last-good-safe Sync** - `482e8b9` (feat)
3. **Task 3: catalogs daemon (unprivileged per-OS) + interval-gated sync + first-run** - `6280a5f` (feat)
4. **Task 4: TUI real sync + schedule selector + honest pip color** - `8415c73` (feat)

## Decisions Made
See `key-decisions` frontmatter. Notably: `Sync()` is preserved as a wrapper (watch caller untouched); the sync command — not `catalog.Sync` — owns the `state.json` freshness writes; the CSYNC-06 offer is non-interactive; the TUI cadence selector uses validate-before-write rather than an OS-admin gate.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Existing pipColor callers broken by signature change**
- **Found during:** Task 4 (honest pip color)
- **Issue:** `pipColor(mtime, degraded)` → `pipColor(ss catalog.SourceState, mtime)` broke four existing assertions in `catalogs_panel_test.go`.
- **Fix:** Updated all four callers to the new signature (`catalog.SourceState{}` / `{Degraded:true}` + mtime).
- **Verification:** `go test ./internal/tui/ -run Catalogs -count=1` green.
- **Committed in:** `8415c73` (Task 4 commit)

**2. [Rule 2 - Missing critical] `catalogs sync` should not silently sync when disabled**
- **Found during:** Task 3
- **Issue:** Plan specified the interval gate but not the `enabled:false` case for a manual `catalogs sync`.
- **Fix:** Added an explicit "disabled (use --force)" no-op branch alongside the interval gate.
- **Verification:** Covered by the gate's `--force` semantics; build + tests green.
- **Committed in:** `6280a5f` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing-critical)
**Impact on plan:** Both necessary for correctness; no scope creep.

## Issues Encountered
- The Task 3 acceptance grep required the literal strings `/ru SYSTEM` and `/rl HIGHEST` to be ABSENT from the Windows daemon file — explanatory comments initially contained them. Reworded the comments (the schtasks args correctly omit both flags) so the grep returns 0.

## User Setup Required
None - no external service configuration required. (Registering the background daemon is opt-in: `beekeeper catalogs daemon install`.)

## Next Phase Readiness
- Wave 1's other plan is 20-03 (Tier 3 Sentry rules), independent of this plan.
- Wave 2 plan 20-02 (LlamaFirewall) depends on 20-01 only for shared-file sequencing (config.go / layered.go / main.go) — no symbol coupling.
- Cross-OS builds (linux/darwin/windows) all green; full package suites for config/catalog/tui/cmd green; `go vet` clean.

---
*Phase: 20-runtime-hardening*
*Completed: 2026-06-10*
