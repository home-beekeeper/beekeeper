---
quick_id: 260611-mpe
slug: pollen-windows-test-coverage
description: Pollen Windows test coverage
date: 2026-06-11
status: in_progress
target_repo: ../pollen (github.com/home-beekeeper/pollen)
---

# Quick Task 260611-mpe: Pollen Windows Test Coverage

## Goal

Raise test coverage of the **Pollen** fork (`../pollen`, `github.com/home-beekeeper/pollen`)
to its release bar, **Windows-scoped**, closing the genuinely-closeable gaps and
formalizing a reason-coded no-test allowlist for code that is out of Pollen's
Windows scope.

## Context (locked)

- **Pollen is Windows-only** — a stopgap support fork of `perplexityai/bumblebee`
  (Apache-2.0, license-cleared this session) until upstream ships Windows natively.
  Bumblebee remains the recommended macOS/Linux tool.
- **Do NOT build macOS/Linux CI complexity.** Coverage is measured/targeted on
  **Windows**, Pollen's platform.
- Unix-only source (`dirkey_unix.go`, `roots_notwindows.go`) and Unix-semantics
  tests are **out of Pollen's Windows scope** (upstream's domain) — they belong in a
  reason-coded allowlist, not the coverage target.
- Mirror beekeeper's **VAL-01 `internal/coveragegate`** philosophy: presence +
  reason-coded fail-closed allowlist, NOT a single-OS %-threshold.
- Tracking artifact lives in beekeeper `.planning/quick/`; **code + atomic commits
  land in the `../pollen` repo**.

## Baseline (Windows `go test ./... -cover`, all passing)

| Package | Coverage |
|---|---|
| internal/walk | 0.0% (Walk() test skips on Windows) |
| internal/model | 35.3% |
| cmd/pollen | 62.8% |
| internal/ecosystem/editorext | 64.6% |
| internal/endpoint | 70.0% |
| internal/ecosystem/scanner | 72.6% |
| internal/ecosystem/rubygems | 73.7% |
| (others) | 77–92% |
| internal/normalize | 100% |

## Tasks

### Task 1 — `internal/model` pure-function tests (35% → ~100%)
- **Files:** `../pollen/internal/model/model_test.go`
- **Action:** Table tests for `DedupKey`, `StableID`, `SupportedEcosystems`,
  `IsSupportedEcosystem`, `canonicalCounts`, and the `joinSorted` 40% branch.
- **Verify:** `go test ./internal/model/ -cover` ≈ 100%.
- **Done:** model uncovered pure functions covered; package green.
- **Commit (pollen):** `test(model): cover DedupKey/StableID/ecosystem helpers`

### Task 2 — `internal/walk` Windows coverage (0% → covered)
- **Files:** `../pollen/internal/walk/walk_test.go`
- **Action:** Add a Windows-compatible Walk() test using `filepath.Join` /
  OS-native separators (the existing `TestWalkSkipsExcludedLibrarySubtrees`
  `t.Skip`s on Windows). Exercise `Walk`, `walkOne`, `isExcluded`,
  `normalizeExcludes`, and `dirkey_other.go:dirKey` (the non-unix path).
- **Verify:** `go test ./internal/walk/ -cover` exercises Walk on Windows (>0%).
- **Done:** walk covered on Windows; existing macOS-suffix test untouched.
- **Commit (pollen):** `test(walk): Windows-compatible Walk() coverage`

### Task 3 — Raise low platform-neutral packages
- **Files:** `_test.go` in `cmd/pollen`, `internal/ecosystem/editorext`,
  `internal/endpoint`, `internal/ecosystem/rubygems`, `internal/ecosystem/scanner`.
- **Action:** Per package, enumerate uncovered functions (`go tool cover -func`),
  add targeted tests for the Windows-reachable ones. Skip Unix-only branches
  (those go to the allowlist in Task 4).
- **Verify:** each package coverage materially up; `go test ./...` green.
- **Done:** low packages raised toward 100% on the Windows-testable surface.
- **Commit (pollen):** one atomic commit per package (`test(<pkg>): …`).

### Task 4 — Reason-coded no-test allowlist (VAL-01 style)
- **Files:** `../pollen/internal/coveragegate/` (new) + its test, or an equivalent
  package-walking test that fails closed.
- **Action:** Port the beekeeper `internal/coveragegate` idea: walk production
  `.go` files, assert each has a sibling/linked test OR a reason-coded allowlist
  entry. Allowlist the Unix-only files (`dirkey_unix.go`, `roots_notwindows.go`,
  any `_unix.go`/`_notwindows.go`) with reason `unix-only-out-of-windows-scope`,
  and pure type/const files with `type-only`. Fail closed on bare/unknown codes.
- **Verify:** `go test ./internal/coveragegate/` green; allowlist enforced.
- **Done:** coverage gate present + green; Unix-only files reason-coded.
- **Commit (pollen):** `test(coveragegate): reason-coded no-test allowlist (Windows scope)`

### Task 5 — Final verification
- **Action:** `go test ./... -cover` on Windows (all green) + `go vet ./...`.
- **Done:** full suite green; coverage report captured in SUMMARY.

## Out of scope (this task)
- macOS/Linux CI matrix for Pollen (explicitly excluded — Bumblebee's domain).
- The `differential`/`parity` e2e vs upstream (Linux/macOS-gated; lives in CI).
- The public GitHub push + signed tags (next step, after coverage green).
- beekeeper web-docs Pollen/Bumblebee positioning (separate follow-up).

## Notes
- Executed **inline** (orchestrator-as-executor, sequential) per the project's
  VAL-01 precedent — worktree-isolated subagents don't fit cross-repo work
  (a beekeeper worktree can't contain the sibling `../pollen` repo).
