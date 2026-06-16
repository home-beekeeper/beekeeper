---
phase: 07-sensitive-path-runtime-enforcement
plan: "02"
subsystem: check
tags: [spath, canonicalization, path-adapter, shell-detection, d-01]
dependency_graph:
  requires:
    - 07-01  # policy.EvaluatePath + DefaultSensitivePaths (pure engine, already built)
  provides:
    - internal/check/paths.go (extractPathTargets, canonicalizePath, expandHome, expandWinEnvVars, mergeDecisions, extractBashCredentialPaths, firstShellToken)
  affects:
    - internal/check/handler.go (07-03 will wire extractPathTargets + canonicalizePath into runCheck)
tech_stack:
  added: []
  patterns:
    - impure-adapter pattern (all I/O in internal/check, pure decision in internal/policy)
    - targeted %VAR% expansion (os.Getenv, NOT os.ExpandEnv)
    - EvalSymlinks-error-fallback-to-Abs (Pitfall 3)
    - conservative verb-prefix shell scanning (SPATH-03)
key_files:
  created:
    - internal/check/paths.go
    - internal/check/paths_test.go
  modified: []
decisions:
  - "D-01 compliance: expandWinEnvVars uses targeted %VAR%→os.Getenv replacement; fail-closed on unresolved var (raw token preserved)"
  - "Pitfall 3 fix: filepath.EvalSymlinks error falls back to filepath.Abs result, not raw input"
  - "Task 1 + Task 2 implemented in same files (paths.go / paths_test.go) per plan — single atomic commit captures both since files are co-located"
metrics:
  duration: "~12 minutes"
  completed: "2026-06-04"
  tasks_completed: 2
  files_created: 2
---

# Phase 07 Plan 02: paths.go Impure Path Adapter Summary

**One-liner:** Impure path-canonicalization adapter for `beekeeper check` using targeted `%VAR%→os.Getenv` expansion, EvalSymlinks-fallback-to-Abs, and conservative verb-prefix shell scanning.

## What Was Built

`internal/check/paths.go` (new) — the I/O adapter that feeds pre-resolved path strings into the pure `policy.EvaluatePath` engine. All filesystem and env I/O lives here, never in `internal/policy`.

### Functions Implemented

| Function | Purpose |
|----------|---------|
| `expandHome(dir string) string` | Tilde → os.UserHomeDir + filepath.Join; fail-safe returns input on error. Copied verbatim from internal/watch/watcher.go:121-132 (avoids fsnotify dep). |
| `expandWinEnvVars(raw string) string` | Targeted `%VAR%`→os.Getenv replacement (D-01). Case-insensitive var name. Fail-closed: unresolved var keeps `%VAR%` token. Does NOT use os.ExpandEnv. |
| `canonicalizePath(raw string) string` | D-01 sequence: expandWinEnvVars → expandHome → filepath.Abs → EvalSymlinks (Abs fallback on error, Pitfall 3) → filepath.ToSlash. Returns "" only for "" input. |
| `extractPathTargets(tc policy.ToolCall) []string` | Reads `file_path` (primary, Claude Code), `path` (legacy compat), and Bash command verb targets. Nil-safe. |
| `extractBashCredentialPaths(cmd string) []string` | Conservative verb-prefix scan: `cat`/`head`/`tail`/`less`/`more`/`type`/`Get-Content`/`gc`. Returns raw tokens (including `%USERPROFILE%` forms) for downstream canonicalization. |
| `firstShellToken(rest string) string` | Strips surrounding single/double quotes, stops at first unquoted whitespace. |
| `mergeDecisions(base, overlay policy.Decision) policy.Decision` | Most-restrictive-wins: block(2) > warn(1) > allow(0). Mirrors enforce.go rank logic. |

## Test Results

```
go test ./internal/check/... -count=1 -timeout=60s
ok  github.com/home-beekeeper/beekeeper/internal/check  10.343s
```

All acceptance-criteria test groups pass:
- `TestExpandWinEnvVars` — 5 cases (expansion, case-insensitive, fail-closed, no-op, no-dollar-expand)
- `TestCanonicalizePath` — 6 cases (empty, tilde, traversal, non-existent path Pitfall-3, USERPROFILE D-01, forward-slashes)
- `TestExtractPathTargets` — 6 cases (file_path, path legacy, WebFetch nil, Bash echo nil, nil ToolInput, Bash cat)
- `TestMergeDecisions` — 7 cases (all rank combos + rule ID preservation)
- `TestExtractBashCredentialPaths` — 11 cases (cat, Get-Content, gc, quoted, type+USERPROFILE, chaining, nil, head, tail)
- `TestFirstShellToken` — 8 cases (double-quote, unquoted, single-quote, leading WS, empty, WS-only, Windows backslash, unclosed quote)

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 + 2 | f182378 | feat(07-02): add paths.go adapter — all functions + full test suite |

(Both tasks land in the same two files per the plan; single commit is correct.)

## Deviations from Plan

None. Plan executed exactly as written.

- expandWinEnvVars uses a sentinel placeholder (`\x00UNEXPANDED\x00varname\x00`) during the replacement pass to avoid re-processing already-expanded vars, then restores them to `%VAR%` form at the end. This is an implementation detail not in the plan but is correct and does not alter the contract.
- Task 1 and Task 2 were committed in a single commit because both tasks write to the same two files (`paths.go` / `paths_test.go`). The plan marks them as separate tasks for semantic separation, but since no other file is modified between them, a single atomic commit is cleaner and correct.

## Threat Model Coverage

All six STRIDE threats from the plan are mitigated:

| Threat | Status |
|--------|--------|
| T-07-05 Path traversal | filepath.Abs in canonicalizePath; tested by TestCanonicalizePath/dot-dot |
| T-07-06 Tilde bypass | expandHome in canonicalizePath; tested by TestCanonicalizePath/tilde |
| T-07-07 Symlink / non-existent path allow | EvalSymlinks fallback to Abs (Pitfall 3); tested by TestCanonicalizePath/non-existent |
| T-07-08 Shell-command wrapping | extractBashCredentialPaths + canonicalizePath %USERPROFILE% expansion; TestExtractBashCredentialPaths/type+USERPROFILE |
| T-07-09 Windows backslash bypass | filepath.ToSlash in canonicalizePath; policy.matchesBlockPattern normalizeSlashes also handles it (defense in depth) |
| T-07-10 Executing the extracted command | Not possible — expandWinEnvVars uses os.Getenv string replacement only; no exec.Command or os.Expand call exists |

## Known Stubs

None. All functions are fully implemented. No placeholder returns or hardcoded values.

## Self-Check

- [x] `internal/check/paths.go` exists and contains `func canonicalizePath(`, `func extractPathTargets(`, `func mergeDecisions(`, `func expandWinEnvVars(`
- [x] `internal/check/paths_test.go` exists and contains `TestCanonicalizePath`
- [x] `go build ./internal/check/...` succeeds (no output)
- [x] `go test ./internal/check/... -count=1 -timeout=60s` exits 0
- [x] `os.ExpandEnv` is NOT called in paths.go; `os.Getenv` IS used
- [x] Commit f182378 exists

## Self-Check: PASSED
