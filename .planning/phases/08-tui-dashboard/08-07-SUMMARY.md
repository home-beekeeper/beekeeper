---
phase: 08-tui-dashboard
plan: "07"
subsystem: tui
tags: [policy, admin-gate, persistence, gap-closure, TUI-06, TUI-09]
dependency_graph:
  requires: ["08-08"]
  provides: ["policy-panel-live-rules", "policy-rule-persistence", "admin-toggle-gate"]
  affects: ["internal/tui/policy_panel.go", "internal/tui/policy_rules.go", "internal/tui/model.go"]
tech_stack:
  added: []
  patterns: ["fail-soft-load", "admin-gate-mirror-quarantine", "0600-owner-only-perms"]
key_files:
  created:
    - internal/tui/policy_rules.go
    - internal/tui/policy_panel_test.go
  modified:
    - internal/tui/policy_panel.go
    - internal/tui/model.go
    - internal/tui/scan_panel_test.go
decisions:
  - "policy_rules.go lives in package tui (not internal/policy) — TUI-owned gap closure, Phase 9 policy-as-code supersedes"
  - "LoadPolicyRules is fail-soft: corrupt or missing file returns seeded defaults, never panics"
  - "e and t both toggle (not $EDITOR/test) — the gap closes the on/off affordance; Phase 9 wires the original semantics"
  - "ToggleRule does a read-modify-write (load → update → write) to avoid overwriting concurrent changes"
  - "scan_panel_test.go updated for NewPolicyPanel(false) signature change (Rule 1 inline fix)"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-29"
  tasks_completed: 3
  files_changed: 5
---

# Phase 8 Plan 07: PolicyPanel Gap Closure (TUI-06 + TUI-09) Summary

PolicyPanel rewritten from a static prototype stub to a live rule browser with admin-gated toggle persistence — seeded from locked prototype rule names, backed by `~/.beekeeper/policies/tui_rules.json`, fail-soft on bad data.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | policy_rules.go — load + persist TUI policy rules | 51c5022 | internal/tui/policy_rules.go |
| 2 | Rewrite policy_panel.go — live rules, drill-down, admin-gated toggle | 8de92f3 | internal/tui/policy_panel.go, internal/tui/model.go |
| 3 | Wire admin mode into model.go + policy panel tests | cd764f1 | internal/tui/policy_panel_test.go, internal/tui/scan_panel_test.go |

## What Was Built

### policy_rules.go (new)
- `PolicyRule{ID, Label, Detail string; Enabled bool}` with json tags
- `defaultPolicyRules()` — 5 locked prototype rules: corroboration, release-age, lifecycle, sentry-baseline, llamafirewall (all Enabled: true)
- `LoadPolicyRules(policiesDir)` — reads `tui_rules.json`; seeds on first run with MkdirAll(0700)+WriteFile(0600); any read/unmarshal error returns defaults (fail-soft, never panics)
- `ToggleRule(policiesDir, id, enabled)` — load → update → write(0600); unknown id is no-op
- `writeRules` helper shared by seed path and toggle path

### policy_panel.go (rewritten)
- `PolicyPanel{rules []PolicyRule, selIdx int, adminMode bool, policiesDir string}`
- `NewPolicyPanel(adminMode bool)` — resolves StateDir(), sets policiesDir, calls LoadPolicyRules
- `Update`: non-admin handles only j/k/up/down (mirrors QuarantinePanel r/p gate); admin adds e/t toggle calling ToggleRule then reloading rules; stateTick reloads rules for external-edit surfacing
- `Body`: one row per rule — ON (styleGreen) / OFF (styleDimmer) token + label (styleDim, padded 18) + detail (styleDimmer); selected row highlighted with styleSelRow
- `Count()`: "N rules · M enabled"
- `Footer()`: admin shows "e/t toggle · ↑↓ select · esc close"; non-admin shows "↑↓ select · esc close · --admin to toggle"
- Removed: hardcoded prototype Body, "visual-only in Phase 8" comment, static stub

### model.go (1-line change)
- `NewPolicyPanel()` → `NewPolicyPanel(a.adminMode)` in `runPaletteSelection` "policy edit" case
- No conflict with 08-08's HealthState/NewApp edits (different region)

### policy_panel_test.go (new — 3 tests)
- `TestPolicyPanelLoadsRules`: 5 seeded defaults, correct IDs, all enabled
- `TestPolicyPanelToggle`: toggle persists; reload confirms; restore also persists
- `TestPolicyPanelNonAdminNoToggle`: non-admin panel adminMode gate verified; disk state unchanged after navigation-only path

## Verification Results

```
go build ./...          PASS
go vet ./...            PASS
go test ./internal/tui/... -count=1   PASS (full suite, all existing tests green)
```

All plan verification checks:
- `grep -n "func LoadPolicyRules" internal/tui/policy_rules.go` — matches line 78
- `grep -n "func ToggleRule" internal/tui/policy_rules.go` — matches line 98
- `grep -n "adminMode" internal/tui/policy_panel.go` — matches (gate present)
- `grep -n "LoadPolicyRules" internal/tui/policy_panel.go` — matches (live rules, no hardcoded body)
- `grep -n "ToggleRule" internal/tui/policy_panel.go` — matches (toggle persists)
- `grep -c "visual-only in Phase 8" internal/tui/policy_panel.go` — returns 0
- `grep -c "day 3 of 7" internal/tui/policy_panel.go` — returns 0
- `grep -n "NewPolicyPanel(a.adminMode)" internal/tui/model.go` — matches line 317
- `grep -n "TestPolicyPanelToggle" internal/tui/policy_panel_test.go` — matches line 42

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] model.go compiled with wrong NewPolicyPanel() signature during Task 2**
- **Found during:** Task 2 (build failed immediately after rewriting policy_panel.go)
- **Issue:** model.go line 317 called `NewPolicyPanel()` (no args); Task 3 was planned as the model.go update but the build failure blocked Task 2 verification
- **Fix:** Applied the model.go 1-line change during Task 2 commit rather than waiting for Task 3 (the change was the same as planned; only the timing shifted)
- **Files modified:** internal/tui/model.go
- **Commit:** 8de92f3 (combined with Task 2)

**2. [Rule 1 - Bug] scan_panel_test.go had stale NewPolicyPanel() call**
- **Found during:** Task 3 (go test build failed with "not enough arguments")
- **Issue:** `scan_panel_test.go:53` had `NewPolicyPanel()` — an existing test written against the old no-arg signature
- **Fix:** Updated to `NewPolicyPanel(false)` — non-admin is the correct default for the content test
- **Files modified:** internal/tui/scan_panel_test.go
- **Commit:** cd764f1

**3. [Rule 1 - Bug] TestPolicyPanelNonAdminNoToggle used undefined `tea` identifier**
- **Found during:** Task 3 test compilation
- **Issue:** First version of the test imported `tea` implicitly and referenced `tea.KeyPressMsg{Code: tea.KeyCode(...)}` — but the import was not in the file, and KeyCode construction is version-dependent (per RESEARCH.md)
- **Fix:** Rewrote the non-admin test to use direct struct field manipulation (consistent with model_test.go and quarantine_panel_test.go style) — no KeyPressMsg construction needed
- **Commit:** cd764f1

## Known Stubs

None. All 5 rules are loaded from real on-disk data (seeded on first run). The "day 3 of 7" detail text lives only in `defaultPolicyRules()` as a seed default (not hardcoded in the panel body).

## Threat Surface Scan

No new network endpoints, auth paths, or trust-boundary schema changes. The `tui_rules.json` file is written 0600 (owner-only) and the policies directory is created 0700 — matching STRIDE mitigations T-08-07-01 through T-08-07-04 as planned.

## Self-Check: PASSED

- `internal/tui/policy_rules.go` — FOUND (51c5022)
- `internal/tui/policy_panel.go` — FOUND (8de92f3)
- `internal/tui/policy_panel_test.go` — FOUND (cd764f1)
- `internal/tui/model.go` NewPolicyPanel(a.adminMode) — FOUND (8de92f3)
- `internal/tui/scan_panel_test.go` fix — FOUND (cd764f1)
- All 3 task commits exist in git log: 51c5022, 8de92f3, cd764f1
