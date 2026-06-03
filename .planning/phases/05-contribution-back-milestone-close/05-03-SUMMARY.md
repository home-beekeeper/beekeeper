---
phase: 05-contribution-back-milestone-close
plan: "03"
subsystem: pollen-upstream-sync
tags: [upstream-sync, contribution-back, release-prep, documentation]
dependency_graph:
  requires: []
  provides:
    - pollen UPSTREAM.md sync workflow with runnable commands (SYNC-01)
    - pollen version-history table rows for pollen.2/3/4/5
    - contribution-back-deferred rationale and patch set commit range (SYNC-02 D-2)
    - pollen VERSION=0.1.1-pollen.5 and CHANGES.md milestone-close section (pollen.5 release prep)
  affects:
    - ../pollen/UPSTREAM.md
    - ../pollen/VERSION
    - ../pollen/CHANGES.md
tech_stack:
  added: []
  patterns:
    - "Newest-first CHANGES.md with status blockquote (prepared/not yet tagged)"
    - "8-step sync workflow with bash command blocks per step"
    - "Contribution-back-by-documented-deferral pattern (SYNC-02 satisfied without upstream PR)"
key_files:
  created: []
  modified:
    - "../pollen/UPSTREAM.md"
    - "../pollen/VERSION"
    - "../pollen/CHANGES.md"
decisions:
  - "D-2 honored: no upstream PRs opened against perplexityai/bumblebee; UPSTREAM.md §Contribution-back status documents deferral"
  - "Sync workflow steps upgraded from prose to runnable bash commands with worked example invocations per SC1"
  - "Version-history note added explaining pollen.2-5 uniform pinned commit (Windows-addition, not upstream-absorption)"
metrics:
  duration: "15min"
  completed: "2026-06-03"
  tasks_completed: 2
  files_modified: 3
---

# Phase 05 Plan 03: UPSTREAM.md sync workflow + pollen.5 release prep Summary

Extended `../pollen/UPSTREAM.md` with the 8-step sync workflow upgraded to runnable commands, version-history rows for pollen.2/3/4/5, and the contribution-back-deferred section satisfying SYNC-02 by documented deferral (D-2). Bumped `../pollen/VERSION` to `0.1.1-pollen.5` and prepended the milestone-close `CHANGES.md` section. All work committed in the pollen repo; no tag, no push, no upstream PR.

## Tasks Completed

| Task | Description | Commit (pollen repo) | Files |
|------|-------------|----------------------|-------|
| 1 | UPSTREAM.md: version history rows + contribution-back-deferred section | `cea2dd0` | `UPSTREAM.md` |
| 2 | VERSION bump to 0.1.1-pollen.5 + CHANGES.md milestone-close section | `3b6d8f2` | `VERSION`, `CHANGES.md` |

## What Was Built

### Task 1 — UPSTREAM.md (cea2dd0)

Three changes to `../pollen/UPSTREAM.md`:

1. **Sync workflow upgraded to runnable commands.** Each of the 8 steps now has a concrete
   bash block (e.g. step 1: `git remote update upstream`, step 2: `git diff $PINNED $NEW_COMMIT --stat`,
   step 3: `git worktree add /tmp/bumblebee-check $NEW_COMMIT && go test ./...`, step 8:
   `git tag -a $NEXT_TAG HEAD && cosign verify-blob ...`). The "Status" preamble was updated
   to note no upstream absorption occurred in Milestone 2.

2. **Version-history table.** Four rows appended below the pollen.1 row — pollen.2/3/4/5 —
   all pinned to `c24089804ee66ece4bec6f14638cb98985389cdb` (no upstream sync; Windows-addition
   releases). A note explains why uniform pinned commit is correct and expected.

3. **`## Contribution-back status` section.** Documents:
   - The prepared Windows patch set across commits `2c202ef..b906404` (WRES-01/02, WPATH-01/02,
     WEXT-01/02/03) with per-requirement-group commit ranges.
   - Upstream PR #3/#4 context and why contribution-back is not viable now.
   - Explicit DEFERRED disposition per maintainer decision D-2.
   - Re-submission guide: three upstream-shaped PRs (root resolver, path representation,
     extension/MCP coverage), each linking the equivalent Pollen tag + `TestDifferential` result.

### Task 2 — VERSION + CHANGES.md (3b6d8f2)

- `VERSION`: single-line bump from `0.1.1-pollen.4` to `0.1.1-pollen.5`.
- `CHANGES.md`: new `## v0.1.1-pollen.5 (2026-06-03) — Milestone close` section prepended
  above the pollen.4 section (newest-first). Includes:
  - Status blockquote: prepared/not yet tagged; all four deferred tags batched at M2 close.
  - Metadata-only release note: no Pollen source-code change; `schema_version` stays `0.1.0`;
    `TestDifferential` unaffected.
  - Cross-repo beekeeper deliverables that close the milestone (BKINT-02, PTEST-05, SDEF-01).
  - SYNC-02 deferred contribution-back reference.

## Verification

- `../pollen/UPSTREAM.md` contains `## Contribution-back status` (verified: `git grep`)
- `../pollen/UPSTREAM.md` version-history table has pollen.2/3/4/5 rows (verified: `git grep pollen.5`)
- `../pollen/VERSION` = `0.1.1-pollen.5` (verified: `cat VERSION`)
- `../pollen/CHANGES.md` has `## v0.1.1-pollen.5` section above pollen.4 (verified: `grep`)
- `cd ../pollen && go build ./...` exits 0 (verified)
- `git -C ../pollen tag --list v0.1.1-pollen.5` is empty — no tag created (verified)
- Beekeeper repo `git status` shows no `../pollen` paths staged (verified)

## Deviations from Plan

None — plan executed exactly as written.

The 8-step sync workflow was already present in UPSTREAM.md with runnable commands in most
steps. The plan asked to verify and tighten any prose-only steps; the upgrade to full bash
blocks per step (with a worked example invocation pattern per step) satisfies SC1 without
restructuring the existing document.

## Known Stubs

None. This plan is documentation and metadata only; no source-code stubs exist.

## Threat Flags

None. All content references public upstream PR numbers and public commit hashes only; no
new network endpoints, auth paths, or trust-boundary changes were introduced (per T-05-09:
accepted low-risk).

## Self-Check: PASSED

- `../pollen/UPSTREAM.md` modified: confirmed (git diff --stat shows 177 insertions)
- `../pollen/VERSION` reads `0.1.1-pollen.5`: confirmed
- `../pollen/CHANGES.md` has v0.1.1-pollen.5 section: confirmed
- Pollen commits `cea2dd0` and `3b6d8f2` exist: confirmed (git log --oneline -5)
- No beekeeper-repo changes staged: confirmed (git status in beekeeper shows only .planning/config.json M)
- `go build ./...` exits 0: confirmed
- No `v0.1.1-pollen.5` tag: confirmed (git tag --list empty)
