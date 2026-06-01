---
phase: 08-tui-dashboard
plan: "04"
subsystem: tui
tags: [tui, bubbletea-v2, scan-panel, policy-panel, audit-panel, help-panel, tdd, animation]
dependency_graph:
  requires:
    - internal/tui/panel.go (PanelContent interface — 08-01)
    - internal/tui/styles.go (styleGreen, styleDim, styleDimmer, styleTeal, styleWhite, styleRed — 08-01)
    - internal/tui/watcher.go (newRecordsMsg type — 08-01)
    - internal/audit/types.go (AuditRecord struct)
  provides:
    - internal/tui/scan_panel.go (ScanPanel, stepTickMsg, stepTickCmd, scanSteps)
    - internal/tui/policy_panel.go (PolicyPanel — static 5-row policy display)
    - internal/tui/audit_panel.go (AuditPanel — NDJSON tail, 20-record cap)
    - internal/tui/help_panel.go (HelpPanel — static NAVIGATION+CONCEPT)
    - internal/tui/scan_panel_test.go (6 tests for all 4 panels)
  affects:
    - internal/tui/scan_stub.go (deleted — replaced by scan_panel.go)
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN pattern — test file committed before implementation
    - stepTickMsg + tea.Tick(480ms) one-shot timer chain for animation
    - maxAuditLines cap prevents unbounded record growth (T-08-04-02)
    - Prototype content locked via const/var (scanSteps, scanCompleteText, etc.)
key_files:
  created:
    - internal/tui/scan_panel.go
    - internal/tui/policy_panel.go
    - internal/tui/audit_panel.go
    - internal/tui/help_panel.go
    - internal/tui/scan_panel_test.go
  modified:
    - internal/tui/scan_stub.go (deleted — no longer needed)
decisions:
  - "scan_stub.go deleted and replaced by scan_panel.go which provides real stepTickCmd implementation"
  - "stepTickCmd disarms naturally after done=true — 5 ticks max (4 steps + 1 done trigger)"
  - "AuditPanel isAlertRecord() checks both record_type==sentry_alert AND decision==alert for maximum coverage"
  - "PolicyPanel imports fmt for fmt.Sprintf in row padding helper — not lipgloss (no layout needed)"
  - "HelpPanel Count() returns 'beekeeper v0.6.0' per spec TUI-05/06"
  - "scan_panel_test.go contains tests for all 4 panels per wave cohesion design (plan note)"
metrics:
  duration_seconds: 0
  completed_date: "2026-05-29"
  tasks_completed: 2
  files_created: 5
  files_modified: 0
---

# Phase 8 Plan 04: ScanPanel, PolicyPanel, AuditPanel, HelpPanel Summary

**One-liner:** Four PanelContent implementations — ScanPanel with 480ms step animation (TUI-05), PolicyPanel with locked prototype policy rows (TUI-06), AuditPanel with 20-record NDJSON tail, and HelpPanel with static NAVIGATION+CONCEPT — all TDD with 6 green tests.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 RED | scan_panel_test.go — 6 failing tests | 12fb509 | internal/tui/scan_panel_test.go |
| 1+2 GREEN | ScanPanel + PolicyPanel + AuditPanel + HelpPanel | a50c36c | scan_panel.go, policy_panel.go, audit_panel.go, help_panel.go, -scan_stub.go |

## Verification Results

- `go test ./internal/tui/... -run "TestScanPanel|TestPolicyPanel|TestAuditPanel|TestHelpPanel" -v -count=1`: 6/6 PASS
- `go test ./internal/tui/... -count=1`: all tests PASS
- `go build ./...`: exit 0
- `go vet ./internal/tui/...`: clean
- All 10 plan verification grep checks: PASS

## Deviations from Plan

### Auto-applied Rule 2 — Replacement of scan_stub.go

**1. [Rule 2 - Missing] Deleted scan_stub.go when scan_panel.go was created**
- **Found during:** Task 1+2 GREEN phase
- **Issue:** The 08-01 SUMMARY documented `scan_stub.go` as a stub specifically intended to be replaced by scan_panel.go. Leaving both files would cause a redeclared `stepTickCmd` compile error.
- **Fix:** Removed `scan_stub.go` when creating `scan_panel.go` (the real implementation).
- **Files modified:** internal/tui/scan_stub.go (deleted)
- **Commit:** a50c36c

No other deviations — plan executed exactly as written with the expected stub replacement.

## TDD Gate Compliance

- RED gate: `test(08-04)` commit 12fb509 — all 6 tests fail (undefined types confirmed)
- GREEN gate: `feat(08-04)` commit a50c36c — all 6 tests pass, build clean

## Known Stubs

None. All 4 panels are fully implemented per spec:
- ScanPanel: animation complete, history mode complete
- PolicyPanel: static content complete (Phase 9 will add live config reading)
- AuditPanel: live NDJSON tail complete with 20-record cap
- HelpPanel: static NAVIGATION + CONCEPT complete

PolicyPanel and HelpPanel are intentionally static for Phase 8. The plan explicitly notes "Phase 9 will add live config reading" for policy and the help content is LOCKED from the prototype.

## Threat Surface Scan

No new network endpoints, auth paths, or trust boundary crossings introduced.

- AuditPanel re-serializes `audit.AuditRecord` via `json.Marshal` to display strings — no eval or shell execution (T-08-04-01: accepted)
- AuditPanel `maxAuditLines=20` cap enforced (T-08-04-02: mitigated)
- ScanPanel `stepTickCmd` fires at most 5 times total then disarms (T-08-04-03: accepted)
- PolicyPanel shows hardcoded config values — no live credentials (T-08-04-04: accepted)

## Self-Check: PASSED

- internal/tui/scan_panel.go: EXISTS
- internal/tui/policy_panel.go: EXISTS
- internal/tui/audit_panel.go: EXISTS
- internal/tui/help_panel.go: EXISTS
- internal/tui/scan_panel_test.go: EXISTS
- commit 12fb509: EXISTS (git log confirms)
- commit a50c36c: EXISTS (git log confirms)
- scan_stub.go: DELETED (intentional — replaced by scan_panel.go)
