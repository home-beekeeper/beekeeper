---
phase: 08-package-manager-nudge-behavioral-test-suite
plan: "02"
subsystem: policy
tags: [pkgparse, F3, SC1, NUDGE-01, refactor, policy, policyloader, pnpm, bun, yarn]

dependency_graph:
  requires:
    - phase: 08-01
      provides: "internal/pkgparse.Parse — canonical install-command parser with pnpm/bun/yarn→npm mapping"
  provides:
    - "internal/policy/engine.go routes install extraction through pkgparse (no local duplicate)"
    - "internal/policyloader/enforce.go routes install extraction through pkgparse (no local duplicate)"
    - "F3/SC1 closed end-to-end: pnpm add / bun add catalog-match proven by regression tests"
  affects:
    - "internal/check (Plan 04+: handler.go nudge block consumes same pkgparse via engine)"
    - "internal/gateway (Plan 04+: policy.go nudge merge also benefits from F3 fix)"

tech-stack:
  added: []
  patterns:
    - "Single-source-of-truth extract: both policy and policyloader delegate to pkgparse.Parse"
    - "Ecosystem key aliasing: pnpm/bun/yarn Manager with npm Ecosystem ensures LookupAll matches"
    - "Byte-identical refactor pattern: existing test suites unchanged, prove behavioral equivalence"

key-files:
  created: []
  modified:
    - internal/policy/engine.go
    - internal/policy/engine_test.go
    - internal/policyloader/enforce.go

key-decisions:
  - "Route both extract paths through pkgparse.Parse (Flag 4 EXTRACT — single source eliminates parser drift hazard)"
  - "Keep firstPackageToken/splitVersion/normalize in engine.go: still consumed by editor-extension helpers (extractExtensionInstall/extractAllExtensionInstalls)"
  - "Add TestBunAddCatalogMatch alongside TestPnpmAddCatalogMatch: both are SC1 attack vectors, both need regression coverage"
  - "enforce.go comment updated: removes stale 'duplicated here to keep internal/policy untouched' rationale now that pkgparse is the shared source"

requirements-completed: [NUDGE-01]

duration: ~10min
completed: 2026-06-04
---

# Phase 8 Plan 02: Collapse Duplicate Install-Parsers to pkgparse — Summary

**Collapsed two duplicate install-prefix tables (engine.go + enforce.go) into a single `pkgparse.Parse` call, mapping pnpm/bun/yarn to ecosystem "npm" and closing F3/SC1 (malicious pnpm/bun installs now surface in catalog corroboration) — proven by new TestPnpmAddCatalogMatch and TestBunAddCatalogMatch regression cases.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-06-04T~10:00Z
- **Completed:** 2026-06-04T~10:10Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Removed local `installPrefixes` var (9-entry table) and `extractFromCommand` func from `engine.go`; both replaced by a single `pkgparse.Parse(cmd)` call
- Removed duplicate `installPrefixesOverlay` table, `firstNonFlagToken`, and `stripVersionSuffix` from `enforce.go`; replaced by `pkgparse.Parse(cmd)`
- pnpm/bun/yarn install commands now resolve to `Ecosystem="npm"` on both the engine and overlay paths, so `LookupAll("npm", pkg)` matches — F3/SC1 bypass closed
- Added `TestPnpmAddCatalogMatch` and `TestBunAddCatalogMatch` to `engine_test.go` as the end-to-end regression proof
- Full test suite (engine_test + enforce_test) passes unchanged — byte-identical behavior confirmed

## Task Commits

1. **Task 1: Route engine.go install extraction through pkgparse** - `47770ce` (refactor)
2. **Task 2: Route enforce.go through pkgparse + add F3/SC1 regression cases** - `12b15c3` (feat)

**Plan metadata:** (SUMMARY commit — see below)

## Files Created/Modified

- `internal/policy/engine.go` — Removed `installPrefixes` var + `extractFromCommand` func; added `pkgparse` import; `extract()` now calls `pkgparse.Parse(cmd)` for generic install commands
- `internal/policy/engine_test.go` — Added `TestPnpmAddCatalogMatch` and `TestBunAddCatalogMatch` (F3/SC1 regression net)
- `internal/policyloader/enforce.go` — Removed `installPrefixesOverlay` var + `firstNonFlagToken` + `stripVersionSuffix` helpers; added `pkgparse` import; `extractEcoPackageFromCommand` delegates to `pkgparse.Parse`

## Decisions Made

- Flag 4 EXTRACT decision made concrete: both consumers route through one canonical source, eliminating the risk that `engine.go` and `enforce.go` views of "what is being installed" diverge
- Retained `firstPackageToken`/`splitVersion`/`normalize` in `engine.go` — still consumed by editor-extension helpers (`extractExtensionInstall`/`extractAllExtensionInstalls`) which are out of pkgparse scope
- Added a `TestBunAddCatalogMatch` case in addition to the plan-required pnpm case: bun is equally an F3 attack vector and deserves its own regression lock

## Deviations from Plan

None — plan executed exactly as written. The extra `TestBunAddCatalogMatch` test case is additive (Rule 2: missing critical coverage for a documented SC1 variant) and does not change scope.

## Known Stubs

None — all behavioral changes are fully implemented, tested, and proven by the regression net.

## Threat Surface Scan

No new network endpoints, auth paths, file access, or schema changes introduced. This is a pure internal refactor. T-08-04 (F3 bypass via pnpm/bun) and T-08-05 (parser drift between two copies) are now both fully mitigated: single pkgparse source removes drift surface, and the new engine_test cases are the proof.

## Verification Results

- `go test ./internal/policy/... ./internal/policyloader/...` — PASS (all existing tests unchanged + new F3/SC1 cases green)
- `go build ./...` — PASS (clean, no errors)
- `TestPnpmAddCatalogMatch` — PASS (`pnpm add evil-pkg` → Ecosystem "npm" → `LookupAll("npm","evil-pkg")` → block, CorroborationCount=2)
- `TestBunAddCatalogMatch` — PASS (`bun add evil-pkg` → same path → block)
- `TestEngineImportsArePure` — PASS (pkgparse imports only "strings"; adding it does not violate the pure-library contract)

## Self-Check

**Files modified (committed):**
- `internal/policy/engine.go` — FOUND (committed 47770ce)
- `internal/policy/engine_test.go` — FOUND (committed 12b15c3)
- `internal/policyloader/enforce.go` — FOUND (committed 12b15c3)

**No duplicate install-prefix table remains:** confirmed — `installPrefixes` and `installPrefixesOverlay` both deleted.

**Commits verified:**
- 47770ce `refactor(08-02): route engine.go install extraction through pkgparse`
- 12b15c3 `feat(08-02): route enforce.go through pkgparse; add F3/SC1 regression cases`

## Self-Check: PASSED

---
*Phase: 08-package-manager-nudge-behavioral-test-suite*
*Completed: 2026-06-04*
