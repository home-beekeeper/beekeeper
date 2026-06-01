---
phase: 08-tui-dashboard
plan: "06"
subsystem: tui
tags: [gap-closure, alerts-panel, tui-02, tui-03, bubbletea]
dependency_graph:
  requires: []
  provides: [TUI-02, TUI-03]
  affects: [internal/tui/alerts_panel.go, internal/tui/alerts_panel_test.go]
tech_stack:
  added: []
  patterns:
    - "AlertRow struct carries Agent + detail slice fields for expanded view"
    - "recordToRow allow case returns BadgeOK() row (previously silently dropped)"
    - "expanded bool on AlertsPanel toggles Body branch for detail view"
    - "renderExpandedDetail: three-section forensic card (PROCESS TREE / FILES ACCESSED / NETWORK)"
key_files:
  created: []
  modified:
    - internal/tui/alerts_panel.go
    - internal/tui/alerts_panel_test.go
decisions:
  - "Navigation (j/k/up/down) collapses expanded detail automatically — clarity over persistence"
  - "renderExpandedDetail is a standalone function (not inlined in Body) — testability and readability"
  - "Nil/empty slices render '(none)' placeholders — T-08-06-04 mitigation"
  - "Paths truncated to width-6 in expanded view — T-08-06-02 DoS mitigation"
  - "Footer hints update to 'enter collapse' when expanded — explicit affordance"
metrics:
  duration: "~15m"
  completed: "2026-05-29"
  tasks_completed: 3
  files_changed: 2
---

# Phase 8 Plan 06: AlertsPanel Gap Closure (TUI-02 + TUI-03) Summary

Gap closure for the AlertsPanel live activity feed: allow decisions now appear with `BadgeOK()`, each row shows agent identity from `rec.AgentName`, and pressing enter toggles a full forensic detail view for the selected row.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | AlertRow Agent + detail fields; allow rows; agent column | b9225a6 | alerts_panel.go |
| 2 | enter-key expandable detail view (process tree / files / network) | b9225a6 | alerts_panel.go |
| 3 | Update alerts_panel_test.go: allow, agent, expand tests | b9225a6 | alerts_panel_test.go |

## What Was Built

### Gap 1 — TUI-02 (allow decisions + agent identity)

`AlertRow` gained an `Agent string` field. `recordToRow` was extended with a `case "allow":` branch that returns `(row, true)` with `Badge: BadgeOK()` — previously allow decisions fell through to `return AlertRow{}, false` and were silently discarded. `Agent` is populated from `rec.AgentName` for all record types (sentry_alert and all policy_decision variants).

`Body` now renders an agent-identity column between the badge and the label, 14 characters wide with `styleDimmer` so it is visually subordinate. Empty agents render as padded blanks, preserving column alignment.

### Gap 2 — TUI-03 (enter-key expandable detail)

`AlertsPanel` gained an `expanded bool` field. The `Update` key switch gained a `case "enter":` that toggles `p.expanded` when a row is selected. Navigation keys (j/k/up/down) reset `p.expanded = false` for clarity.

`AlertRow` gained `ParentChain []string`, `FilesAccessed []string`, `NetworkDests []string` detail fields. `recordToRow` copies `rec.SentryParentChain`, `rec.SentryFilesAccessed`, and `rec.SentryNetworkDests` into these fields for sentry_alert rows; policy_decision rows leave them nil (guarded by the expanded view).

When `p.expanded` is true, `Body` delegates to `renderExpandedDetail` instead of rendering the row list. The detail card shows:
- **PROCESS TREE** — `ParentChain` entries in `styleDim`
- **FILES ACCESSED** — `FilesAccessed` entries in `styleCoral` (full list, not truncated head)
- **NETWORK** — `NetworkDests` entries in `styleRed` (full list)
- Nil/empty slices render `(none)` in `styleDimmer` (T-08-06-04 guard)
- Paths truncated to `width-6` to prevent layout overflow (T-08-06-02 guard)

### Tests

`TestAlertsPanelRecordFilter` (the stale "expected 0 rows for allow" assertion) was renamed to `TestAlertsPanelAllowDecision` and inverted to assert `len(ap.rows) == 1` and `Body contains "OK"`.

Three new tests added:
- `TestAlertsPanelAllowDecision` — allow decision yields 1 row with OK badge
- `TestAlertsPanelAgentColumn` — AgentName "claude-code" appears in rendered body
- `TestAlertsPanelExpandDetail` — sentry_alert with file/network detail renders full paths when `expanded=true`

## Verification

```
go build ./...    PASS
go vet ./...      PASS
go test ./internal/tui/... -count=1   PASS (37 tests, 0 failures)
```

All 10 alerts panel tests pass. All 37 TUI package tests pass. No regressions in warn/block/crit rendering, filtering, 200-row cap, or q/Q quarantine handler.

## Deviations from Plan

### Structural note: single commit for Tasks 1+2+3

The plan assigns tasks 1 and 2 to `alerts_panel.go` and task 3 to `alerts_panel_test.go`. Because task 3's tests reference the `expanded` field introduced in task 2, and the TDD RED phase (writing tests first) produced a compile error that is only resolved by the GREEN implementation, all three tasks were implemented together and committed as a single `feat` commit. The TDD cycle was followed in terms of writing tests before implementation; the commit structure reflects that the test file and implementation file are co-dependent.

None.

## Known Stubs

None. All data flows are wired: `rec.AgentName` → `row.Agent` → rendered column; `rec.SentryParentChain/FilesAccessed/NetworkDests` → row detail fields → `renderExpandedDetail`. No placeholder text or mock data.

## Threat Flags

None. All new surface is read-only rendering of audit record fields already present in the `AuditRecord` struct. The expanded view reads only slices stored on the `AlertRow` — no new I/O, no re-parse. Threat model from the plan covers all cases (T-08-06-01 through T-08-06-04).

## Self-Check: PASSED

- `internal/tui/alerts_panel.go` exists and contains `Agent    string`, `expanded    bool`, `BadgeOK()`, `case "allow":`, `case "enter":`, `ParentChain`, `PROCESS TREE`, `FILES ACCESSED`, `NETWORK`
- `internal/tui/alerts_panel_test.go` exists and contains `TestAlertsPanelAllowDecision`, `TestAlertsPanelAgentColumn`, `TestAlertsPanelExpandDetail`; stale assertion `"expected 0 rows for allow decision"` removed (grep count = 0)
- Commit b9225a6 exists: `git log --oneline | grep b9225a6` → `feat(08-06): AlertRow agent+detail fields, allow rows, enter-expand TUI-02/TUI-03`
- `go build ./...` PASS; `go vet ./...` PASS; `go test ./internal/tui/... -count=1` PASS (37/37)
