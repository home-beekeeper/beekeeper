---
phase: 07-sensitive-path-runtime-enforcement
plan: "01"
subsystem: policy
tags: [sensitive-path, allowlist, blocklist, policyloader, purity]
dependency_graph:
  requires: []
  provides: [DefaultSensitivePaths-cursor-windsurf-cargo, isAllowedPath-basename, extractTargetPath-file_path]
  affects: [internal/policy/path.go, internal/policyloader/enforce.go]
tech_stack:
  added: []
  patterns: [TDD-red-green, pure-function-library, basename-matching]
key_files:
  created: []
  modified:
    - internal/policy/path.go
    - internal/policy/path_test.go
    - internal/policyloader/enforce.go
    - internal/policyloader/enforce_test.go
decisions:
  - "isAllowedPath basename branch: patterns without separator match against lastSegment (mirrors matchesBlockPattern — Pitfall 2 closed)"
  - "extractTargetPath: file_path primary + path fallback (Pitfall 4 closed)"
  - "Stale comment 'Plan 08 time' replaced with v1.2.0 Phase 7 reference (D-02)"
metrics:
  duration: "3 minutes"
  completed: "2026-06-03"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 4
requirements: [SPATH-01, SPATH-04]
---

# Phase 7 Plan 01: Sensitive-Path Policy Fixes Summary

**One-liner:** Extended DefaultSensitivePaths with /.cursor/, /.windsurf/, /.cargo/credentials block entries and .env.example/.env.test/.env.schema allowlist; fixed isAllowedPath basename matching (Pitfall 2) and extractTargetPath to read file_path primary (Pitfall 4).

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Failing tests: basename allowlist + MCP block entries | 281fb5f | internal/policy/path_test.go |
| 1 (GREEN) | Extend DefaultSensitivePaths + fix isAllowedPath | f9d9e03 | internal/policy/path.go |
| 2 (RED) | Failing test: TestExtractTargetPathFilePath | a3a5a27 | internal/policyloader/enforce_test.go |
| 2 (GREEN) | Fix extractTargetPath to read file_path | 6a2f48d | internal/policyloader/enforce.go |

## Verification

```
go test ./internal/policy/... ./internal/policyloader/... -count=1 -timeout=60s
ok    github.com/bantuson/beekeeper/internal/policy      1.343s
ok    github.com/bantuson/beekeeper/internal/policyloader  1.252s
```

- `TestPathImportsArePure` — PASS (path.go imports only "strings")
- `TestEvaluatePathBasenameAllowlist` — PASS (.env.example/test/schema allow; production still blocked)
- `TestEvaluatePathCursorMCPBlocked` — PASS (/.cursor/ fragment blocks all three path forms)
- `TestEvaluatePathWindsurfMCPBlocked` — PASS (/.windsurf/ fragment blocks all three path forms)
- `TestEvaluatePathCargoCredentialsBlocked` — PASS (bare + .toml both blocked)
- `TestExtractTargetPathFilePath` — PASS (all 5 behavior cases)
- `TestEvaluatePath` — PASS (no regression in existing table)
- `TestEvaluatePathAllowlistOverride` — PASS (WR-04 boundary guard intact)

## Acceptance Criteria

- [x] `go test ./internal/policy/... -run "TestEvaluatePathBasenameAllowlist|TestEvaluatePathCursorMCPBlocked|TestEvaluatePathWindsurfMCPBlocked" -count=1` exits 0
- [x] `go test ./internal/policy/... -run TestPathImportsArePure -count=1` exits 0 (path.go imports only "strings")
- [x] `go test ./internal/policy/... -run TestEvaluatePath -count=1` exits 0 (no regression)
- [x] `go test ./internal/policyloader/... -run TestExtractTargetPathFilePath -count=1` exits 0
- [x] `go test ./internal/policyloader/... -count=1` exits 0 (no regression)
- [x] DefaultSensitivePaths() BlockPatterns contains "/.cursor/", "/.windsurf/", "/.cargo/credentials"
- [x] DefaultSensitivePaths() AllowPatterns contains ".env.example", ".env.test", ".env.schema"
- [x] isAllowedPath has basename branch calling lastSegment when allow has no "/" or "\"
- [x] path.go no longer contains "Plan 08 time"
- [x] extractTargetPath reads "file_path" before "path"
- [x] enforce_test.go contains func TestExtractTargetPathFilePath

## Deviations from Plan

None — plan executed exactly as written.

## Threat Mitigations Applied

| Threat ID | Mitigation | Status |
|-----------|-----------|--------|
| T-07-01 | Added /.cursor/, /.windsurf/, bare /.cargo/credentials to BlockPatterns | CLOSED |
| T-07-02 | AllowPatterns populated + isAllowedPath basename fix — safe lookalikes no longer over-blocked | CLOSED |
| T-07-03 | Basename branch uses exact lastSegment (not HasPrefix); WR-04 boundary guard intact | CLOSED |
| T-07-04 | extractTargetPath reads file_path primary; tested by TestExtractTargetPathFilePath | CLOSED |

## Known Stubs

None — all additions are wired to the policy engine. No placeholder data flows to the UI.

## Threat Flags

None — no new network endpoints, auth paths, or schema changes introduced.

## TDD Gate Compliance

RED gate: commits 281fb5f (policy tests) and a3a5a27 (enforce test) — failing tests committed before implementation.
GREEN gate: commits f9d9e03 (policy impl) and 6a2f48d (enforce impl) — implementation committed after RED.

## Self-Check: PASSED

- internal/policy/path.go: exists and modified
- internal/policy/path_test.go: exists and contains TestEvaluatePathBasenameAllowlist
- internal/policyloader/enforce.go: exists and modified
- internal/policyloader/enforce_test.go: exists and contains TestExtractTargetPathFilePath
- Commits 281fb5f, f9d9e03, a3a5a27, 6a2f48d: all present in git log
