---
phase: 08-tui-dashboard
plan: "03"
subsystem: tui
tags: [tui, catalogs, quarantine, bubbletea-v2, lipgloss-v2, panel-content, freshness-pips]
dependency_graph:
  requires:
    - internal/tui/panel.go (PanelContent interface — 08-01)
    - internal/tui/styles.go (color palette, badge functions — 08-01)
    - internal/tui/watcher.go (stateTick type — 08-01)
    - internal/catalog/state.go (WatchState, SourceState, LoadState)
    - internal/quarantine/quarantine.go (Manifest, List, Restore, Purge)
    - internal/platform/dirs.go (StateDir, CatalogDir)
  provides:
    - internal/tui/catalogs_panel.go (CatalogsPanel implementing PanelContent)
    - internal/tui/quarantine_panel.go (QuarantinePanel implementing PanelContent)
  affects:
    - internal/tui/model.go (08-01 consumer — openPanel can now use these panel types)
tech_stack:
  added: []
  patterns:
    - PanelContent interface implementation (Update/Title/Count/Padded/Body/Footer/Critical)
    - lipgloss.Color returns color.Color (not a type alias) — pipColor returns color.Color
    - stateTick message dispatch to panel refresh
    - tea.KeyPressMsg key handling in panel Update
    - confirmPurge two-step confirmation gate for destructive action
key_files:
  created:
    - internal/tui/catalogs_panel.go
    - internal/tui/quarantine_panel.go
    - internal/tui/catalogs_panel_test.go
    - internal/tui/quarantine_panel_test.go
  modified: []
decisions:
  - "pipColor returns color.Color (not lipgloss.Color) — lipgloss.Color is a function in v2, not a type"
  - "colorDim used as pipColor return for unknown/zero mtime (not a separate colorUnknown)"
  - "QuarantinePanel.loadItems() no-ops on error (items stays empty slice, not nil)"
  - "TestCatalogsPanelFresh tests pipColor directly rather than body string for color assertion"
metrics:
  duration_seconds: 660
  completed_date: "2026-05-29"
  tasks_completed: 2
  files_created: 4
  files_modified: 0
---

# Phase 8 Plan 03: CatalogsPanel + QuarantinePanel Summary

**One-liner:** CatalogsPanel with 4-source freshness pips (green/amber/red) and QuarantinePanel with HELD badge rows and admin-gated purge confirmation — both implementing PanelContent.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | CatalogsPanel | d316284 | internal/tui/catalogs_panel.go, internal/tui/catalogs_panel_test.go |
| 2 | QuarantinePanel | 83e8ef4 | internal/tui/quarantine_panel.go, internal/tui/quarantine_panel_test.go |

## Verification Results

- `go test ./internal/tui/... -run "TestCatalogsPanel|TestQuarantinePanel" -count=1`: 8/8 PASS
- `go build ./...`: exit 0
- `go vet ./internal/tui/...`: clean
- `grep "type CatalogsPanel" internal/tui/catalogs_panel.go`: FOUND
- `grep "type QuarantinePanel" internal/tui/quarantine_panel.go`: FOUND
- `grep "knownSources" internal/tui/catalogs_panel.go`: FOUND
- `grep "confirmPurge" internal/tui/quarantine_panel.go`: FOUND
- `grep "BadgeHeld" internal/tui/quarantine_panel.go`: FOUND

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] lipgloss.Color is a function in v2, not a type**

- **Found during:** Task 1 implementation
- **Issue:** Plan specified `func pipColor(...) lipgloss.Color` but in lipgloss v2, `lipgloss.Color` is a function `func(s string) color.Color`, not a type. Using it as a return type caused compile error: `lipgloss.Color (value of type func(s string) color.Color) is not a type`.
- **Fix:** Changed `pipColor` return type to `color.Color` (from `image/color`). Added `image/color` import to catalogs_panel.go.
- **Files modified:** internal/tui/catalogs_panel.go
- **Commit:** d316284

**2. [Rule 1 - Bug] TestCatalogsPanelFresh test logic too narrow**

- **Found during:** Task 1 test run (GREEN phase)
- **Issue:** Original test checked `!strings.Contains(body, "never synced")` but the body always contains "never synced" for osv/bumblebee sources that have no mtime set. Since only bumblebee was given a fresh mtime, osv would still show "never synced" making the assertion always fail.
- **Fix:** Changed test to assert `pipColor(...)` returns `colorGreen` directly, and check body contains "synced" (which bumblebee's entry does). This more accurately tests the freshness color behavior.
- **Files modified:** internal/tui/catalogs_panel_test.go
- **Commit:** d316284

## Known Stubs

None. Both panels are fully implemented per spec. All badge rendering, row format, footer keys, pip colors, and admin guards are wired. No placeholder data — panels read from real catalog.WatchState and quarantine.Manifest sources (with graceful fallback on missing files).

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries.

### STRIDE Mitigations Applied (from plan threat model)

| Threat ID | Mitigation | Status |
|-----------|-----------|--------|
| T-08-03-03 | QuarantinePanel 'r'/'p' keys guarded by adminMode bool; non-admin paths return early without dispatching Restore/Purge | Implemented |
| T-08-03-04 | confirmPurge=true gate requires 'y' response before quarantine.Purge is called; any other key cancels | Implemented |
| T-08-03-01 | Freshness display from mtime/state.json — display-only, no policy decisions made from this value | Accepted (display only) |
| T-08-03-02 | Manifest fields rendered as lipgloss display strings — no eval, no shell execution | Accepted (display only) |

## TDD Gate Compliance

- RED gate: Tests written first, confirmed failing (compile errors for missing types)
- GREEN gate: Implementation written to make tests pass
- All 8 tests pass after implementation

## Self-Check: PASSED

- internal/tui/catalogs_panel.go: EXISTS
- internal/tui/quarantine_panel.go: EXISTS
- internal/tui/catalogs_panel_test.go: EXISTS
- internal/tui/quarantine_panel_test.go: EXISTS
- commit d316284: EXISTS (git log confirms)
- commit 83e8ef4: EXISTS (git log confirms)
