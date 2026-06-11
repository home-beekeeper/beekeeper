---
quick_id: 260611-mpe
slug: pollen-windows-test-coverage
description: Pollen Windows test coverage
date: 2026-06-11
status: complete
target_repo: ../pollen (github.com/bantuson/pollen)
pollen_commits: [da58af7, eeaf529, f7e5bad, c8ff8de]
---

# Quick Task 260611-mpe — Pollen Windows Test Coverage — SUMMARY

Raised Pollen test coverage (Windows-scoped) and added a VAL-01-style coverage
gate. **All work + atomic commits are in the `../pollen` repo**; this tracking
artifact lives in beekeeper `.planning/quick/`. Executed inline (orchestrator-
as-executor, sequential) — worktree-isolated subagents don't fit cross-repo
work. Full suite green, `go vet ./...` clean.

## Pollen commits

| Commit | What |
|--------|------|
| `da58af7` | `test(model)`: DedupKey/ecosystem helpers/ScanSummary.StableID — **35.3% → 100%** |
| `eeaf529` | `test(walk)`: Windows-compatible Walk() test — **0% → 84.8%** |
| `f7e5bad` | `test`: scanner error-classifiers (**72.6% → 79.6%**) + rubygems predicates + endpoint DeviceID/WPATH-02 |
| `c8ff8de` | `test(coveragegate)`: reason-coded fail-closed no-test allowlist (Windows scope) |

## Coverage delta (Windows `go test ./... -cover`, all green)

| Package | Before | After |
|---|---|---|
| internal/model | 35.3% | **100%** |
| internal/walk | 0% | **84.8%** |
| internal/scanner | 72.6% | **79.6%** |
| internal/ecosystem/rubygems | 73.7% | 74.3% |
| internal/endpoint | 70.0% | 70.0% (remainder = non-Windows UID branches) |
| internal/coveragegate | — | NEW (gate; no executable statements) |
| internal/ecosystem/editorext | 64.6% | 64.6% (**not raised** — see below) |
| cmd/pollen | 62.8% | 62.8% (**not raised** — see below) |
| (others) | 77–92% | unchanged |

## The coverage gate (`internal/coveragegate`) — the durable deliverable

Mirrors beekeeper's VAL-01 philosophy: **presence + fail-closed reason-coded
allowlist**, not a single-OS %-threshold (coverage on a Windows-only fork can't
be one number — non-Windows branches are uncoverable here by construction).

- `TestProductionFilesLinkedOrAllowlisted`: every Windows-compiled production
  `.go` file's package must have a test OR be allowlisted; a non-Windows file
  (`//go:build unix/!windows/linux/darwin` or platform suffix) MUST be
  allowlisted so a new platform file can't slip in untracked. GOOS=windows
  compilation decided via stdlib `go/build/constraint` + filename-suffix
  convention.
- `TestAllowlistIsReasonCoded`: fail-closed — every entry needs a known reason
  code (closed taxonomy `non-windows-platform`, `type-only`) and an existing file.
- Allowlisted (`non-windows-platform`): `internal/walk/dirkey_unix.go`,
  `cmd/pollen/roots_notwindows.go`.

All 19 packages already have tests, so the presence contract is satisfied
repo-wide.

## Honest scope notes (NOT done this pass)

- **`internal/ecosystem/editorext` (64.6%)** and **`cmd/pollen` (62.8%)** were
  NOT raised. Their uncovered surface is fixture-heavy (editor-extension
  manifest parsing; `readBounded`/`hostFromExtRoot`) and CLI-flag/`Run`-heavy.
  Both **have tests** so they pass the presence gate; pushing them toward 100%
  is a larger fixture-writing effort — a candidate follow-up if the maintainer
  wants the numbers higher.
- **`internal/endpoint` (70%)**: the uncovered lines are the non-Windows numeric
  UID branches — uncoverable on Windows by design; acknowledged via the gate's
  philosophy (the file is tested; the branches are platform-exclusive).
- macOS/Linux CI complexity for Pollen was deliberately NOT built (maintainer
  directive: Pollen is Windows-only; Bumblebee is the macOS/Linux tool).

## Locked context

Pollen is a **Windows-only** support fork of `perplexityai/bumblebee`
(Apache-2.0, license-cleared this session — safe to release). Bumblebee remains
the recommended macOS/Linux tool.

## Next steps (per the chosen release path: coverage → push)

1. (Optional) Raise `editorext` + `cmd/pollen` with fixtures if literal-high
   coverage is wanted there.
2. Public push: `bantuson/pollen` GitHub repo + signed tags `pollen.2–.5`
   (`docs/release-runbook.md` in the pollen repo), cosign-verify.
3. beekeeper web-docs positioning (Bumblebee = recommended macOS/Linux;
   Pollen = Windows-only stopgap; Beekeeper inspired by Bumblebee).
4. The §4(b) per-file change-notice tightening (Apache-2.0 hardening) +
   beekeeper's own missing LICENSE.
