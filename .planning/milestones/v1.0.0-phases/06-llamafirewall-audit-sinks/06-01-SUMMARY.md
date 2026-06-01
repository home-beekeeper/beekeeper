---
plan: 06-01
status: complete
commit: 07b2c66
---
# 06-01 Summary: Audit Rotation, Query, Export

## Completed

- `internal/audit/rotate.go`: `Rotate()` with numbered archive shift, retention-based deletion, and fresh-log creation (AUDT-02)
- `internal/audit/query.go`: Streaming NDJSON filter with `QueryOpts` (since/agent/tool/decision/limit), 1MB scanner buffer, context-check every 100 lines, malformed-line skipping with count summary (AUDT-06)
- `internal/audit/export.go`: Multi-format export (`ndjson`/`csv`/`otlp`) via `ExportOpts`; CSV with fixed header; OTLP envelope with `resourceLogs` structure; shared `filterRecord` helper (AUDT-07)
- `cmd/beekeeper/main.go`: `audit query` and `audit export` subcommands with filter flags; `--no-follow` flag for `audit tail` via `tailAuditLogOnce` (AUDT-05)

## Tests

```
=== RUN   TestExportNDJSON    --- PASS
=== RUN   TestExportCSV       --- PASS
=== RUN   TestExportOTLP      --- PASS
=== RUN   TestQueryFilterDecision --- PASS
=== RUN   TestQueryFilterSince    --- PASS
=== RUN   TestQueryLimit          --- PASS
=== RUN   TestQuerySkipsMalformed --- PASS
=== RUN   TestRotateCreatesNumberedArchive  --- PASS
=== RUN   TestRotateShiftsExistingArchives  --- PASS
=== RUN   TestRotateDeletesOldArchives      --- PASS
=== RUN   TestRotateNoOpWhenSmall           --- PASS
PASS  ok  github.com/mzansi-agentive/beekeeper/internal/audit  1.177s
```

`go build ./...` and `go vet ./internal/audit/...` pass on Windows (dev machine).

## Deviations from Plan

**1. [Rule 1 - Bug] Fixed test assertion in TestRotateDeletesOldArchives**
- **Found during:** Task 1 test run
- **Issue:** The original test checked `os.IsNotExist` on the same path (`auditPath+".1"`) that Rotate would create for the rotated current log — an inherent contradiction. After deletion of the old `.1` and rename of the current log to `.1`, the file exists again (new content), so the `os.IsNotExist` assertion always fails.
- **Fix:** Replaced the contradiction with: (a) assert no `.2` exists (old `.1` was deleted, not shifted), (b) assert `.1` exists with 100-byte content (the rotated current log), (c) assert current log is empty.
- **Files modified:** `internal/audit/rotate_test.go`
- **Commit:** 07b2c66

## Requirements closed

AUDT-02, AUDT-05, AUDT-06, AUDT-07
