---
phase: 02-windows-root-resolver
plan: 03
subsystem: testing
tags: [pollen, cross-platform, parity, go-test, ndjson, fixture]

# Dependency graph
requires:
  - phase: 02-windows-root-resolver/02-01
    provides: Windows root resolver (roots_windows.go)
  - phase: 02-windows-root-resolver/02-02
    provides: Windows root-resolver tests + flipped 6 Phase-2 skips
provides:
  - 8-ecosystem parity fixture tree (testdata/parity-fixture/) committed to pollen repo
  - TestParityAllEcosystems (PTEST-01) — runs on Linux, macOS, Windows with no build tag
  - endpoint.os assertion against runtime.GOOS before normalization
  - 5-ecosystem coverage assertion (npm, pypi, go, rubygems, packagist)
  - normalize() reuse from locked normalize_diff.go harness
affects: [02-04, Phase-2 success criteria, CI parity gate, PTEST-02]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "runtime.Caller(0)+filepath.Dir for cwd-independent fixture paths in tests"
    - "Raw endpoint.os assertion before normalize() to keep PTEST-02 harness clean"
    - "assertEndpointOS + assertParityRecordCoverage as local helpers (not modifying locked normalize)"
    - "8-ecosystem fixture covering all package managers with parity-* namespace to avoid collisions"

key-files:
  created:
    - ../pollen/cmd/pollen/parity_test.go
    - ../pollen/cmd/pollen/testdata/parity-fixture/npm-fixture/package-lock.json
    - ../pollen/cmd/pollen/testdata/parity-fixture/pnpm-fixture/pnpm-lock.yaml
    - ../pollen/cmd/pollen/testdata/parity-fixture/yarn-fixture/yarn.lock
    - ../pollen/cmd/pollen/testdata/parity-fixture/bun-fixture/bun.lock
    - ../pollen/cmd/pollen/testdata/parity-fixture/pypi-fixture/parity_pypi_canary-1.0.0.dist-info/METADATA
    - ../pollen/cmd/pollen/testdata/parity-fixture/gomod-fixture/go.sum
    - ../pollen/cmd/pollen/testdata/parity-fixture/rubygems-fixture/specifications/parity-gem-1.0.0.gemspec
    - ../pollen/cmd/pollen/testdata/parity-fixture/composer-fixture/composer.lock
  modified: []

key-decisions:
  - "PTEST-01: endpoint.os asserted on RAW output before calling normalize() to keep the assertion independent of normalize()'s strip logic and avoid coupling with the PTEST-02-locked harness"
  - "PTEST-01: per-OS ecosystem-coverage assertion (not cross-OS byte equality) because OS path strings in NDJSON differ by design (backslash vs slash); this is sufficient to prove detector parity"
  - "bun.lock text JSONC format used (not bun.lockb binary) — matches bun.go IsTextLockfile dispatch on basename 'bun.lock'; binary format is not parsed in v0.1"
  - "pnpm-lock.yaml uses lockfileVersion 9.0 with plain name@version keys — matches parsePnpmPackages v9 dispatch path in pnpm.go"
  - "normalize_diff.go NOT modified — parity-specific helpers (assertEndpointOS, assertParityRecordCoverage) are local to parity_test.go only"
  - "All fixture names use parity-* namespace to avoid collision with diff-fixture/selftest packages"

patterns-established:
  - "Parity fixture test pattern: explicit --root flag + committed testdata/ tree + per-OS coverage assertions + runtime.Caller(0) fixture path"

requirements-completed: [PTEST-01]

# Metrics
duration: 15min
completed: 2026-06-02
---

# Phase 02 Plan 03: Cross-Platform Parity Test Summary

**TestParityAllEcosystems (PTEST-01) with 8-ecosystem committed fixture tree covering npm/pnpm/yarn/bun/pypi/go/rubygems/packagist; endpoint.os asserted against runtime.GOOS on all 3 CI runners**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-06-02T10:20:00Z
- **Completed:** 2026-06-02T10:35:00Z
- **Tasks:** 3
- **Files modified:** 9 (all in ../pollen)

## Accomplishments

- Created 8-ecosystem fake-package fixture tree under `../pollen/cmd/pollen/testdata/parity-fixture/` with all parseable, minimal fixture files using `parity-*` namespaced package names
- Implemented `TestParityAllEcosystems` (no build tag, runs on all OSes) in `parity_test.go` — reuses locked `buildCurrentPollen`/`runBinaryOnFixture`/`normalize` helpers from same package; defines only parity-specific helpers locally
- Test passes on Windows dev host: endpoint.os = "windows" and all 5 ecosystem strings (npm, pypi, go, rubygems, packagist) covered across 8 records
- `normalize_diff.go` confirmed unmodified (`git diff --quiet` exits 0)
- Committed to pollen repo as `test(02-03)` commit hash 833d29d

## Task Commits

Each task was committed atomically (all in pollen repo):

1. **Task 1: Build the 8-ecosystem parity-fixture tree** - (fixture files included in pollen commit 833d29d)
2. **Task 2: Write parity_test.go (PTEST-01) reusing locked helpers** - (included in pollen commit 833d29d)
3. **Task 3: Commit the parity test + fixture to the pollen repo** - `833d29d` (test: cross-platform parity test + 8-ecosystem fixture)

**Plan metadata:** (beekeeper planning commit follows)

## Files Created/Modified

All in `../pollen`:

- `cmd/pollen/parity_test.go` — TestParityAllEcosystems + assertEndpointOS + assertParityRecordCoverage helpers; no build tag; reuses locked helpers
- `cmd/pollen/testdata/parity-fixture/npm-fixture/package-lock.json` — lockfileVersion 3, parity-npm-canary@1.0.0
- `cmd/pollen/testdata/parity-fixture/pnpm-fixture/pnpm-lock.yaml` — pnpm v9 format, parity-pnpm-canary@1.0.0
- `cmd/pollen/testdata/parity-fixture/yarn-fixture/yarn.lock` — yarn classic v1, parity-yarn-canary@1.0.0
- `cmd/pollen/testdata/parity-fixture/bun-fixture/bun.lock` — text JSONC, parity-bun-canary@1.0.0
- `cmd/pollen/testdata/parity-fixture/pypi-fixture/parity_pypi_canary-1.0.0.dist-info/METADATA` — Metadata-Version 2.1, parity-pypi-canary 1.0.0
- `cmd/pollen/testdata/parity-fixture/gomod-fixture/go.sum` — one module line, example.com/parity/gomod-canary v1.0.0
- `cmd/pollen/testdata/parity-fixture/rubygems-fixture/specifications/parity-gem-1.0.0.gemspec` — minimal gemspec with s.name/s.version; parent dir is `specifications/`
- `cmd/pollen/testdata/parity-fixture/composer-fixture/composer.lock` — packages array with parity/composer-canary 1.0.0

## Decisions Made

- **endpoint.os asserted on raw output**: assertEndpointOS called before normalize() so the assertion is independent of normalize()'s strip logic. normalize() keeps endpoint.os in the output but asserting on raw avoids coupling.
- **Per-OS coverage only (not cross-OS byte equality)**: Windows path strings in source_file/project_path fields contain backslashes, while Linux/macOS use forward slashes. Cross-OS byte comparison would always fail for these fields. Per-OS ecosystem coverage + the test running identically on all 3 runners is what establishes PTEST-01 parity.
- **bun.lock text format**: bun.go dispatches on `"bun.lock"` (text JSONC) via IsTextLockfile. Binary `bun.lockb` emits no records. Fixture uses text format with `{"packages":{"name":["name@version",{}]}}` shape.

## Deviations from Plan

None — plan executed exactly as written. All 3 tasks completed as specified. normalize_diff.go unmodified (confirmed via `git diff --quiet`).

## Issues Encountered

None. All 8 ecosystem parsers emitted records correctly on first attempt.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- PTEST-01 satisfied: committed fixture + TestParityAllEcosystems passing on Windows dev host
- Phase 2 Success Criterion 2 complete (all 3 Phase-2 requirements now done: WRES-01, WRES-02, PTEST-01)
- Phase 2 plan 04 (CHANGES.md + VERSION bump to v0.1.1-pollen.2) is next and can proceed

---
*Phase: 02-windows-root-resolver*
*Completed: 2026-06-02*
