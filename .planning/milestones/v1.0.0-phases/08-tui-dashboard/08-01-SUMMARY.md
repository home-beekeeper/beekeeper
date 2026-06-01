---
phase: 08-tui-dashboard
plan: "01"
subsystem: tui
tags: [bubbletea-v2, lipgloss-v2, tui, dashboard, state-machine, fsnotify, windows-resize]
dependency_graph:
  requires:
    - internal/audit/types.go (AuditRecord type)
    - internal/platform (AuditDir)
  provides:
    - internal/tui (App state machine, Run function)
    - cmd/beekeeper dashboard command
  affects:
    - go.mod / go.sum (Charm v2 deps added)
tech_stack:
  added:
    - charm.land/bubbletea/v2 v2.0.6
    - charm.land/lipgloss/v2 v2.0.3
    - golang.org/x/term v0.43.0
  patterns:
    - Bubble Tea v2 Model interface (View() returns tea.View, not string)
    - lipgloss v2 Color() returns color.Color (not type alias)
    - fsnotify parent-directory watch with event.Name filter
    - Windows SIGWINCH workaround via 500ms polling goroutine
key_files:
  created:
    - internal/tui/styles.go
    - internal/tui/watcher.go
    - internal/tui/resize_windows.go
    - internal/tui/resize_other.go
    - internal/tui/toast.go
    - internal/tui/incidents.go
    - internal/tui/panel.go
    - internal/tui/palette.go
    - internal/tui/base.go
    - internal/tui/model.go
    - internal/tui/model_test.go
    - internal/tui/scan_stub.go
  modified:
    - go.mod
    - go.sum
    - cmd/beekeeper/main.go
decisions:
  - "lipgloss v2 uses func Color(string) color.Color — not a type alias; renderBadge parameter is image/color.Color"
  - "lipgloss v2 WithWhitespaceBackground removed — replaced with WithWhitespaceStyle(lipgloss.NewStyle().Background(c))"
  - "scan_stub.go provides stepTickCmd() no-op to satisfy model.go reference until 08-02 adds scan_panel.go"
  - "bubbles/v2 removed by go mod tidy since unused; panel plans (08-02+) will re-add when they use it"
metrics:
  duration_seconds: 807
  completed_date: "2026-05-29"
  tasks_completed: 11
  files_created: 13
  files_modified: 3
---

# Phase 8 Plan 01: TUI Dashboard Scaffold Summary

**One-liner:** Bubble Tea v2 App state machine (calm/palette/panel/critical modes) with fsnotify audit watcher, Windows resize poller, and `beekeeper dashboard` CLI command.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add Charm v2 deps | fcde895 | go.mod, go.sum |
| 2 | styles.go — color palette + badge functions | fcde895 | internal/tui/styles.go |
| 3 | watcher.go + resize files | fcde895 | internal/tui/watcher.go, resize_windows.go, resize_other.go |
| 4 | toast.go | fcde895 | internal/tui/toast.go |
| 5 | incidents.go — IncidentModel | fcde895 | internal/tui/incidents.go |
| 6 | panel.go — PanelContent + PanelModel | fcde895 | internal/tui/panel.go |
| 7 | palette.go — command palette | fcde895 | internal/tui/palette.go |
| 8 | base.go — calm base screen renderer | fcde895 | internal/tui/base.go |
| 9 | model.go — App state machine + Run | fcde895 | internal/tui/model.go |
| 10 | model_test.go — 8 unit tests | fcde895 | internal/tui/model_test.go |
| 11 | Add newDashboardCmd to main.go | fcde895 | cmd/beekeeper/main.go |

## Verification Results

- `go test ./internal/tui/... -run "TestApp|TestNewRecords"`: 8/8 PASS
- `go build ./...`: exit 0
- `go vet ./internal/tui/...`: clean
- `beekeeper dashboard --help`: shows --admin flag
- All 17 verification grep checks: PASS

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] lipgloss v2 API differences from plan interfaces**

- **Found during:** Task 2 (styles.go) and Task 9 (model.go)
- **Issue 1:** `lipgloss.Color` is a function returning `color.Color`, not a type. Plan interfaces showed it as a type parameter.
- **Fix 1:** Changed `renderBadge(text string, bg lipgloss.Color)` to `renderBadge(text string, bg color.Color)` with `image/color` import.
- **Issue 2:** `lipgloss.WithWhitespaceBackground` does not exist in v2; replaced by `lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(c))`.
- **Fix 2:** Updated both `Place()` calls in model.go to use `WithWhitespaceStyle`.
- **Files modified:** internal/tui/styles.go, internal/tui/model.go

**2. [Rule 2 - Missing] stepTickCmd stub for scan panel reference**

- **Found during:** Task 9 (model.go) — `openPanel(panelScan, nil)` calls `stepTickCmd()`
- **Issue:** model.go references `stepTickCmd()` which is defined in scan_panel.go (plan 08-02, not yet written). Would cause compile error.
- **Fix:** Added `internal/tui/scan_stub.go` with a no-op `stepTickCmd() tea.Cmd` stub. This will be replaced when 08-02 writes scan_panel.go.
- **Files modified:** internal/tui/scan_stub.go (created)

**3. [Rule 1 - Bug] bubbles/v2 removed by go mod tidy**

- **Found during:** Task 1 verification
- **Issue:** `charm.land/bubbles/v2` was not used in any source file, so `go mod tidy` removed it. The plan specified it should be added.
- **Fix:** Accepted as correct behavior — tidy removes unused deps. The bubbles package will be added by plan 08-02 when panel implementations use it. The 3 actually-used deps (bubbletea/v2, lipgloss/v2, term) are present at the correct versions.

## Known Stubs

| Stub | File | Reason |
|------|------|--------|
| `stepTickCmd() tea.Cmd { return nil }` | internal/tui/scan_stub.go | Scan panel animation tick; full impl in 08-02 scan_panel.go |
| `PanelModel.content == nil` path in panel.go | internal/tui/panel.go | Panel content implementations come in plans 08-02 through 08-05 |

## Threat Surface Scan

No new network endpoints, auth paths, or trust boundary crossings introduced. The TUI:
- Reads only the local NDJSON audit log via fsnotify (T-08-01-01 mitigated: malformed lines silently skipped)
- Handles WindowSizeMsg dedup guard (T-08-01-02 mitigated)
- Filters fsnotify events by `event.Name == auditPath` (T-08-01-03 mitigated)
- Renders audit content as display strings only — no eval, no shell (T-08-01-04 accepted)

## Self-Check: PASSED

- internal/tui/model.go: EXISTS
- internal/tui/styles.go: EXISTS
- internal/tui/watcher.go: EXISTS
- internal/tui/resize_windows.go: EXISTS
- internal/tui/resize_other.go: EXISTS
- internal/tui/toast.go: EXISTS
- internal/tui/incidents.go: EXISTS
- internal/tui/panel.go: EXISTS
- internal/tui/palette.go: EXISTS
- internal/tui/base.go: EXISTS
- internal/tui/model_test.go: EXISTS
- commit fcde895: EXISTS (git log confirms)
