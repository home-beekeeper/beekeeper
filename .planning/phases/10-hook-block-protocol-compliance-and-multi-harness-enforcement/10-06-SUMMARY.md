---
phase: 10-hook-block-protocol-compliance-and-multi-harness-enforcement
plan: "06"
subsystem: hooks
tags: [gateway-routing, kilo, trae, harness-support, docs, honesty]
dependency_graph:
  requires: ["10-01", "10-05"]
  provides: ["kilo_trae_gateway_guides", "harness_support_matrix", "readme"]
  affects: ["internal/hooks", "docs", "README.md"]
tech_stack:
  added: []
  patterns: ["gateway-guide (printGuide)", "dedicated per-harness file in internal/hooks/"]
key_files:
  created:
    - internal/hooks/kilo_trae.go
    - docs/harness-support-matrix.md
    - README.md
  modified:
    - internal/hooks/gateway_targets.go
    - internal/hooks/hooks_test.go
decisions:
  - "kilo_trae.go is a dedicated file (not gateway_targets.go) to cleanly separate Tier-3 no-hook harnesses from the generic Continue/OpenClaw gateway dispatch"
  - "Both kilo/trae guides use 'UNGUARDED' in the coverage summary section (T-10-22 mitigation); enforced by TestInstallGatewayTargetKiloTraeUNGUARDED"
  - "docs/harness-support-matrix.md is the authoritative honesty document sourced from 10-RESEARCH.md §2-3; README links it and states the headline honestly"
  - "README.md created from scratch (no prior README.md existed in the project)"
  - "Deviation: plan 10-05 had already added printKiloGuide + printTraeGuide to gateway_targets.go; this plan moved them to kilo_trae.go and upgraded them with UNGUARDED text"
metrics:
  duration: "6 minutes"
  completed_date: "2026-06-05"
  tasks_completed: 2
  files_created: 3
  files_modified: 2
---

# Phase 10 Plan 06: Kilo/Trae Gateway Guides + Honest Support Matrix Summary

Kilo and Trae route to printed MCP-gateway guides (no file write) that
explicitly state native tools are UNGUARDED; the honest Tier 1/2/3 support
matrix is published in docs/harness-support-matrix.md and linked from README.

## What Was Built

### Task 1: Kilo + Trae MCP-gateway routing guides + dispatch

- Created `internal/hooks/kilo_trae.go` with `printKiloGuide(out)` and
  `printTraeGuide(out)`. Both functions:
  - Include the gateway URL `http://127.0.0.1:7837/mcp`
  - Include `beekeeper gateway token` instructions
  - State the native-tools-UNGUARDED caveat prominently and in the coverage
    summary block (required for T-10-22 threat mitigation)
  - Write no files (guide-only, consistent with Tier-3 status)

- Updated `internal/hooks/gateway_targets.go`: removed the duplicate
  `printKiloGuide` and `printTraeGuide` implementations that plan 10-05 had
  placed there. Added a reference comment pointing to `kilo_trae.go`. The
  `printGatewayGuide` dispatch function already called these functions, so
  no dispatch changes were needed.

- Added `TestInstallGatewayTargetKiloTraeUNGUARDED` to `hooks_test.go`:
  - Asserts kilo/trae guides mention gateway URL (MCP interception available)
  - Asserts both guides contain the string "UNGUARDED" (T-10-22 honesty gate)
  - Asserts no files are written for either target

- Existing `TestInstallGatewayTarget` already covered kilo/trae for gateway URL
  and token command; the new test adds the UNGUARDED honesty assertion.

### Task 2: Honest Tier 1/2/3 harness support matrix (docs + README)

- Created `docs/harness-support-matrix.md` (208 lines):
  - Full 15-harness table with columns: Harness, Config Dir, Interception,
    Deny Mechanism, Tier, Caveats, Verification
  - Tier 1 testable = Claude Code (live-verified HPC-04)
  - Tier 1 documented = Codex, Cursor, Augment, CodeBuddy, Qwen Code, Gemini
    CLI, Copilot, Antigravity, Windsurf
  - Tier 2 caveated = Hermes (fail-open), Cline (no Windows), OpenCode
    (plugin + subagent bypass)
  - Tier 3 MCP-gateway-only = Kilo, Trae (UNGUARDED native tools)
  - Honesty Notes section (5 notes): only Claude Code live-verified; CI cannot
    test harness contract honor; Cline no-Windows; Tier-3 native-unguarded gap;
    documented-contract caveat for all 14 non-Claude-Code harnesses

- Created `README.md` (project had no README previously):
  - Project overview, architecture summary, quick-start commands
  - "Agent harness support" subsection with tier summary table
  - Links to `docs/harness-support-matrix.md`
  - Tier-2 caveats stated explicitly (Hermes fail-open, Cline no-Windows,
    OpenCode subagent bypass)
  - Tier-3 (Kilo/Trae) UNGUARDED gap stated without overclaiming
  - Verification scope: "Only Claude Code is locally live-verified"

## Deviations from Plan

### Deviation (acknowledged, no rule violation): plan 10-05 pre-empted kilo_trae.go

Plan 10-05 ran ahead and already placed `printKiloGuide` and `printTraeGuide`
in `gateway_targets.go`. This plan:
1. Created the dedicated `kilo_trae.go` file as planned
2. Upgraded both guide functions to add the "UNGUARDED" language (gateway_targets.go
   versions lacked the UNGUARDED text)
3. Removed the duplicate implementations from `gateway_targets.go` and replaced
   them with a reference comment
4. Added the `TestInstallGatewayTargetKiloTraeUNGUARDED` test to enforce the
   UNGUARDED honesty gate going forward

Build was green throughout — no duplicate-declaration build break.

## Verification Results

- `go build ./...` — PASSED (0 errors)
- `go test ./internal/hooks/ -run 'TestInstallGatewayTarget|TestInstallDispatch' -count=1` — PASSED
- `TestInstallGatewayTargetKiloTraeUNGUARDED/kilo` — PASSED (UNGUARDED in guide)
- `TestInstallGatewayTargetKiloTraeUNGUARDED/trae` — PASSED (UNGUARDED in guide)
- `test -f docs/harness-support-matrix.md && grep -q "Tier 3" ... && grep -q "harness-support-matrix" README.md` — PASSED
- Line count: docs/harness-support-matrix.md = 208 lines (min requirement: 40)
- All 15 harnesses present in the table
- UNGUARDED appears 5 times in harness-support-matrix.md

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced by
this plan (docs and print-only Go functions). Threat T-10-22 mitigated by
the UNGUARDED text in guides and the UNGUARDED honesty gate test.

## Known Stubs

None. All content is fully implemented.

## Self-Check: PASSED

- `internal/hooks/kilo_trae.go` — FOUND
- `docs/harness-support-matrix.md` — FOUND
- `README.md` — FOUND
- Task 1 commit c7018e0 — verified in git log
- Task 2 commit 8e1b50d — verified in git log
