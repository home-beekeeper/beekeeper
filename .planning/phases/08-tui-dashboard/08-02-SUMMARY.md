---
phase: 08-tui-dashboard
plan: "02"
subsystem: tui
tags: [tui, alerts, bubbletea, lipgloss, panel]
dependency_graph:
  requires: [internal/audit/types.go, internal/tui/panel.go (08-01), internal/tui/styles.go (08-01), internal/tui/watcher.go (08-01)]
  provides: [internal/tui/alerts_panel.go AlertsPanel, internal/tui/alerts_panel_test.go]
  affects: [internal/tui/model.go (08-01 consumer)]
tech_stack:
  added: []
  patterns: [PanelContent interface implementation, tea.KeyPressMsg navigation, newRecordsMsg type switch, lipgloss badge rendering]
key_files:
  created:
    - internal/tui/alerts_panel.go
    - internal/tui/alerts_panel_test.go
  modified: []
decisions:
  - "minInt helper renamed from min to avoid conflict with Go 1.21+ builtin min"
  - "max helper added for filter bar separator width calculation"
  - "recordToRow maps sentry_alert high/medium/low all to BadgeWarn() (only critical gets BadgeCrit)"
  - "allow policy_decision records are silently dropped (not added to rows) — correct per spec"
  - "filteredRows() returns p.rows directly (not a copy) when filterQuery is empty — avoids allocation"
metrics:
  duration: "~10 min"
  completed: "2026-05-29"
  tasks_completed: 1
  tasks_total: 1
  files_created: 2
  files_modified: 0
---

# Phase 8 Plan 02: AlertsPanel Summary

**One-liner:** AlertsPanel implementing PanelContent — sentry_alert + policy_decision consolidation with CRIT/WARN/BLOCK badges, 200-row cap, case-insensitive filter, and Critical() red border flag.

## What Was Built

`internal/tui/alerts_panel.go` — AlertsPanel that implements the PanelContent interface (defined in 08-01's `panel.go`). The panel is the primary operational surface combining sentry_alert records (CRIT/WARN badges) and policy_decision block/warn records (BLOCK/WARN badges) in a single chronological view.

Key types and functions:
- `AlertRow` struct: Time (HH:MM:SS), Badge (rendered lipgloss string), Label, Meta, RecordID
- `AlertsPanel` struct: rows, selIdx, critical bool, filterMode bool, filterQuery string
- `NewAlertsPanel(critical bool) *AlertsPanel`
- `Update(tea.Msg) (PanelContent, tea.Cmd)` — handles `newRecordsMsg` and `tea.KeyPressMsg`
- `recordToRow(audit.AuditRecord) (AlertRow, bool)` — sentry_alert critical→CRIT, high/medium/low→WARN; policy_decision block→BLOCK, warn→WARN; allow→excluded
- `filteredRows() []AlertRow` — case-insensitive substring on label+meta; all rows when empty
- `Body(width, height int) string` — row rendering with selection highlight; filter bar when active
- `Footer() string` — "↑↓ select · enter inspect · q quarantine · esc close" (or filter hint)
- `Title() string` = "Sentry alert log"
- `Critical() bool` — returns the critical flag set at construction time

`internal/tui/alerts_panel_test.go` — 8 tests covering all specified behaviors.

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | AlertsPanel model + tests | b39c884 | internal/tui/alerts_panel.go, internal/tui/alerts_panel_test.go |

## Verification Status

Build `go build ./internal/tui/...` fails with "no required module provides package charm.land/bubbletea/v2" — this is EXPECTED for Wave 1 parallel execution. The charm.land dependencies (bubbletea/v2, lipgloss/v2) are added to go.mod by plan 08-01 running concurrently. The package will compile cleanly once both worktrees are merged by the orchestrator.

Tests (`go test ./internal/tui/... -run TestAlertsPanel`) cannot run in isolation for the same reason. Full test verification occurs after wave merge.

## Deviations from Plan

### Auto-applied Rule 1 fixes

**1. [Rule 1 - Bug] Renamed `min` helper to `minInt`**
- **Found during:** Task 1 implementation
- **Issue:** Go 1.21+ defines a builtin `min` function; defining a package-level `min` shadows it and causes a compile warning/error in Go 1.25
- **Fix:** Named the helper `minInt(a, b int) int`
- **Files modified:** internal/tui/alerts_panel.go
- **Commit:** b39c884

**2. [Rule 2 - Missing functionality] Added `max` helper**
- **Found during:** Task 1 implementation
- **Issue:** `Body()` uses `max(0, width-6)` for filter bar separator width to prevent negative repeat counts; plan snippet did not define this helper
- **Fix:** Added `max(a, b int) int` helper in alerts_panel.go
- **Files modified:** internal/tui/alerts_panel.go
- **Commit:** b39c884

## Known Stubs

None. AlertsPanel is fully implemented per spec. All badge text, row format, footer, filter logic, and 200-row cap are wired. The panel is a pure display consumer — no data wiring stubs.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. AlertsPanel renders only from `audit.AuditRecord` structs (already in the audit log trust boundary). All string rendering is via lipgloss (display-only, no eval). No new threat surface beyond what was declared in the plan's threat model.

## Self-Check

- [x] internal/tui/alerts_panel.go created — FOUND
- [x] internal/tui/alerts_panel_test.go created — FOUND
- [x] Commit b39c884 exists
- [x] type AlertsPanel present in alerts_panel.go
- [x] maxAlertRows constant = 200 present
- [x] PanelContent interface reference in file (Update returns PanelContent)
- [x] All 8 TestAlertsPanel* test functions present in test file

## Self-Check: PASSED
