---
phase: 08-package-manager-nudge-behavioral-test-suite
plan: "01"
subsystem: pkgparse
tags: [pkgparse, nudge, F3, SC1, purity, fuzz, BTEST-03, NUDGE-01, NUDGE-05]
dependency_graph:
  requires: []
  provides:
    - "internal/pkgparse.Parse — single canonical install-command parser"
    - "internal/pkgparse.ParsedCommand — unified struct for all consumers"
  affects:
    - "internal/policy/engine.go (Plan 02 consumer — will swap extractFromCommand)"
    - "internal/policyloader/enforce.go (Plan 02 consumer — will remove duplicate)"
    - "internal/nudge (Plans 03+ consumer — Evaluate receives ParsedCommand)"
tech_stack:
  added: []
  patterns:
    - "Pure library: imports only 'strings' — no os/net/io/time/sync/context"
    - "TestPkgparseImportsArePure: AST purity enforcement (clone of TestReleaseAgeImportsArePure)"
    - "FuzzParse: //go:build fuzz RELEASE GATE never-panic contract (BTEST-03)"
    - "installTable dispatch: prefix-based linear scan, manager/ecosystem split"
key_files:
  created:
    - internal/pkgparse/pkgparse.go
    - internal/pkgparse/pkgparse_test.go
    - internal/pkgparse/fuzz_test.go
  modified: []
decisions:
  - "installTable prefix order: longer/more-specific prefixes listed first so cargo add before cargo install and pnpm add before pnpm install"
  - "npm add included in table (live engine.go lacked it — PRD §6.4 gap)"
  - "pnpm/bun/yarn Ecosystem='npm' so LookupAll('npm', pkg) matches F3/SC1 attacks via non-npm managers"
  - "npx / pnpm dlx / bun x: IsExec=true AND IsInstall=true (both flags, §10-9)"
  - "No-arg install: IsInstall=true, Package='' (§10-8); Unpinned stays false for no-package case"
  - "computeUnpinned: bare name → true; @latest → true; ^/~ → true; exact digits → false (NUDGE-05)"
  - "Sudo stripping: HasPrefix 'sudo ' on lowercased trimmed command before prefix table match"
metrics:
  duration: "~15 minutes"
  completed: "2026-06-04T09:40:00Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 3
  files_modified: 0
---

# Phase 8 Plan 01: Pure pkgparse Package — ParsedCommand + Parse Summary

**One-liner:** Pure `internal/pkgparse` package with canonical install-command parser covering npm/pnpm/bun/yarn (ecosystem "npm"), `npm add` verb, sudo strip, Unpinned flag, and never-panic FuzzParse gate.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create pure pkgparse package with ParsedCommand + Parse | bbf9477 | internal/pkgparse/pkgparse.go |
| 2 | Table-driven parse tests + TestPkgparseImportsArePure + FuzzParse | b650300 | internal/pkgparse/pkgparse_test.go, internal/pkgparse/fuzz_test.go |

## Verification Results

- `go build ./...` — PASS (clean, no errors)
- `go test ./internal/pkgparse/...` — PASS (38 subtests: TestParse 38/38 + TestPkgparseImportsArePure)
- `go test -tags fuzz -run FuzzParse ./internal/pkgparse/...` — PASS (34 seeds, no panics)
- `go vet ./internal/pkgparse/...` — PASS (clean)

## Success Criteria Status

- [x] Single pure `internal/pkgparse.Parse` exists — pnpm/bun/yarn map to Ecosystem "npm" (NUDGE-01 / SC1 foundation)
- [x] `npm add` recognised as an install verb (PRD §6.4) — §10-7/§10-9 silent hole closed
- [x] Unpinned classification present (NUDGE-05): bare name / @latest / ^/~ → true; exact → false
- [x] FuzzParse never-panic target present (BTEST-03)
- [x] TestPkgparseImportsArePure proves package is pure (imports only "strings")

## Requirements Covered

- NUDGE-01: pnpm/bun/yarn install commands parse to ecosystem "npm" — catalog LookupAll will match
- NUDGE-05: Unpinned bool field set on ParsedCommand for @latest, bare name, ^/~ ranges
- BTEST-03: FuzzParse release-gate fuzz target (//go:build fuzz, seed corpus, never-panic)

## Deviations from Plan

None — plan executed exactly as written. All three source behaviors (npm add gap, pnpm/bun/yarn F3 mapping, Unpinned flag) implemented as specified.

## Known Stubs

None — pkgparse.go is a pure parser with no data sources to wire; all exported behaviors are fully implemented and tested. Plan 02 will consume pkgparse.Parse by swapping `extractFromCommand` in engine.go and removing the duplicate in enforce.go.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. `internal/pkgparse` is a pure string-processing library with no I/O surface. T-08-01 (never executed — enforced by TestPkgparseImportsArePure) and T-08-02 (no panic — proven by FuzzParse) are fully mitigated.

## Self-Check

**Files created:**
- `internal/pkgparse/pkgparse.go` — FOUND (committed bbf9477)
- `internal/pkgparse/pkgparse_test.go` — FOUND (committed b650300)
- `internal/pkgparse/fuzz_test.go` — FOUND (committed b650300)

**Commits verified:**
- bbf9477 `feat(08-01): create pure internal/pkgparse package with ParsedCommand + Parse`
- b650300 `test(08-01): add table-driven parse tests, purity test, and FuzzParse gate`

## Self-Check: PASSED
