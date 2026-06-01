---
phase: 08-tui-dashboard
plan: "09"
subsystem: tui
tags: [bugfix, watcher, audit-tail, cr-01, gap-closure]
dependency_graph:
  requires: []
  provides: [watcher/tailFrom-correct-offset]
  affects: [internal/tui/watcher.go, internal/tui/watcher_test.go]
tech_stack:
  added: []
  patterns: [bufio.Reader.ReadString, per-line offset tracking]
key_files:
  created:
    - internal/tui/watcher_test.go
  modified:
    - internal/tui/watcher.go
decisions:
  - "Use bufio.Reader.ReadString('\\n') instead of bufio.Scanner to get per-line control over offset advancement"
  - "Partial trailing lines break the loop without advancing offset — re-read on next tick once newline lands"
  - "Malformed complete lines advance offset anyway (they are complete; do not re-read forever)"
metrics:
  duration: "15m"
  completed: "2026-05-29"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 2
---

# Phase 8 Plan 09: Gap Closure CR-01 — tailFrom Partial-Line Fix Summary

**One-liner:** Replace bufio.Scanner + Seek/Stat fallback in tailFrom with a bufio.Reader ReadString loop that advances offset only past complete newline-terminated NDJSON lines, closing the CR-01 data-loss bug in the dashboard live feed.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Rewrite tailFrom — offset only past complete lines | 0db9d39 | internal/tui/watcher.go |
| 2 | Regression tests proving partial trailing records are not dropped | 0f38cbb | internal/tui/watcher_test.go |

## What Was Built

### Task 1 — tailFrom rewrite (0db9d39)

Replaced the buggy `tailFrom` body in `internal/tui/watcher.go`:

**Before (buggy):**
- `bufio.Scanner` with a 1 MB buffer drains the file including any partial trailing line
- `f.Seek(0, 1)` returns the position of the last byte the reader *buffered* — past the partial line
- A `newOffset == 0` fallback reset the offset to `info.Size()`, potentially skipping the entire tail
- Result: records written mid-tick (no trailing `\n` yet — the common NDJSON case) were silently dropped

**After (correct):**
- `bufio.NewReader(f)` with `ReadString('\n')` loop
- Offset advances by `int64(len(line))` **only** for lines that returned no error (i.e., newline-terminated)
- On `err != nil` (partial trailing line or EOF with no data), the loop breaks WITHOUT advancing offset
- Next `tailFrom` call with the persisted offset re-reads the fragment; once its newline lands it is emitted exactly once
- Malformed-but-complete lines still advance offset (no infinite re-read)
- The `newOffset == 0` / `info.Size()` fallback block is deleted entirely

### Task 2 — Regression tests (0f38cbb)

Created `internal/tui/watcher_test.go` (package `tui`) with four deterministic tests:

| Test | What it proves |
|------|---------------|
| `TestTailFromPartialLine` | Partial trailing record is held (not emitted), then emitted exactly once after its newline lands — the core CR-01 scenario |
| `TestTailFromCompleteLines` | Two complete lines returned once on first call; second call returns zero (no double-emit) |
| `TestTailFromMalformedSkipped` | Malformed complete line skipped silently; offset advances past it (no infinite re-read on second call) |
| `TestTailFromMissingFile` | Non-existent path returns `(nil, offset)` without panic |

## Deviations from Plan

None — plan executed exactly as written. The reference fix in `08-REVIEW.md` CR-01 was followed verbatim. The `strings` import was added as specified.

## Verification Results

```
grep -n "ReadString" internal/tui/watcher.go     → line 64: r.ReadString('\n')    PASS
grep -c "bufio.NewScanner" internal/tui/watcher.go → 0                             PASS
grep -c "info.Size()" internal/tui/watcher.go      → 0                             PASS
grep -n "offset += int64(len(line))" watcher.go    → line 69                       PASS
go build ./...                                      → exit 0                        PASS
go vet ./...                                        → exit 0                        PASS
go test ./internal/tui/... -count=1                → ok (6.087s)                   PASS
go test ./internal/tui/... -run TestTailFrom -v    → 4/4 PASS                      PASS
```

## Known Stubs

None introduced by this plan. The fix is purely correctness — no UI data sources added.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced. The fix only changes the read strategy for an existing file path (`auditPath`). The threat register in the plan (T-08-09-01 through T-08-09-03) is fully accounted for:

- T-08-09-01 (malformed/partial stalls feed): mitigated — malformed complete lines skip but advance offset; partial lines re-read once complete
- T-08-09-02 (never-terminated line): accepted per plan
- T-08-09-03 (extremely long line memory): accepted per plan (ReadString is strictly less lossy than the prior 1 MB scanner cap)

## Self-Check: PASSED

- [x] `internal/tui/watcher.go` exists and modified
- [x] `internal/tui/watcher_test.go` exists and created
- [x] Commit `0db9d39` exists (Task 1)
- [x] Commit `0f38cbb` exists (Task 2)
- [x] All four TestTailFrom* tests pass
- [x] No STATE.md or ROADMAP.md modified
