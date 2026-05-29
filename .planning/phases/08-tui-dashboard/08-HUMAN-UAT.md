---
status: partial
phase: 08-tui-dashboard
source: [08-VERIFICATION.md]
started: "2026-05-29T15:10:00.000Z"
updated: "2026-05-29T15:10:00.000Z"
---

## Current Test

[awaiting human testing]

## Tests

### 1. Windows Terminal resize behavior
expected: Run `beekeeper dashboard` in Windows Terminal, then resize the window — dashboard redraws correctly to new dimensions within 500ms, no layout corruption (StartResizePoller).
result: [pending]

### 2. AltScreen entry and exit
expected: Launch `beekeeper dashboard` (enters alt-screen); press `q` — terminal restored cleanly, no leftover ANSI artifacts.
result: [pending]

### 3. fsnotify real-time feed (CR-01 regression)
expected: With `beekeeper dashboard` running, append an NDJSON record to the audit log mid-write (no trailing newline, then add the newline a moment later); open alerts panel via `!` — the record appears after the newline lands, not dropped.
result: [pending]

### 4. Filter mode interactivity
expected: In the alerts panel, press `/`, type "exfil" — only matching rows show in real time; press Esc — full list restored.
result: [pending]

### 5. PolicyPanel admin toggle round-trip
expected: Launch `beekeeper dashboard --admin`, open policy panel (`:` → policy edit), select a rule, press `e` to toggle; close and reopen the panel — rule shows the opposite ON/OFF state (persisted to `~/.beekeeper/policies/tui_rules.json`).
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
