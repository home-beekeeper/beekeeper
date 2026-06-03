---
phase: "05"
plan: "02"
subsystem: catalog/check
tags: [sdef-01, pollen-self, self-catalog, selftest, supply-chain]
dependency_graph:
  requires: []
  provides: [SDEF-01]
  affects: [internal/catalog/selfcatalog_test.go, internal/check/selftest.go, internal/check/corpus/fixtures.json, internal/check/selftest_test.go]
tech_stack:
  added: []
  patterns: [selfCatalogAdapter pollen discriminator, non-production version string safety, catalog.Entry mmap schema for selftest]
key_files:
  created: []
  modified:
    - internal/catalog/selfcatalog_test.go
    - internal/check/selftest.go
    - internal/check/corpus/fixtures.json
    - internal/check/selftest_test.go
decisions:
  - "SDEF-01: pollen-self entries use Ecosystem:beekeeper, Package:pollen in catalog.Entry (mmap schema) — NOT selfCatalogEntry (feed schema). Pitfall 5 respected."
  - "Non-production version string pollen-test-v0.0.1 chosen for all selftest/adapter fixtures to prevent false quarantine of real v0.1.1-pollen.N releases (T-05-06)."
  - "TestRunSelftest added as a second test function alongside TestSelftestAllFixturesPass — plan verification command references canonical name TestRunSelftest."
  - "Static feed fixture selfcatalog_match_pollen.json not created — test builds the feed discriminator inline (TestSelfCatalogAdapter_PollenEntries constructs the adapter directly without a signed feed, which is correct because the adapter test does not exercise parseAndVerifySelfFeed)."
metrics:
  duration: "~5min"
  completed: "2026-06-03"
  tasks: 2
  files: 4
---

# Phase 05 Plan 02: Pollen-Self Catalog Extension Summary

**One-liner:** Adds `pollen-self` entries (Ecosystem:"beekeeper", Package:"pollen", non-production version "pollen-test-v0.0.1") to the unified beekeeper-self catalog adapter test and selftest corpus so a hypothetical compromised Pollen release is detectable via `beekeeper selftest` (SDEF-01, T-05-04).

## What Was Built

### Task 1: pollen-self adapter test (commit 225cd66)

Added `TestSelfCatalogAdapter_PollenEntries` to `internal/catalog/selfcatalog_test.go`. The test:

- Constructs a `selfCatalogAdapter` with a single `selfCatalogEntry{ID:"pollen-self-2026-001", Ecosystem:"beekeeper", Package:"pollen", Versions:["pollen-test-v0.0.1"], Severity:"critical"}`.
- Asserts `LookupAll("beekeeper","pollen")` returns exactly one match with `CatalogSource:"beekeeper-self"`, `Package:"pollen"`, `Signed:true`, `Severity:"critical"`, `Version:"pollen-test-v0.0.1"`.
- Asserts package discriminator: `LookupAll("beekeeper","beekeeper")` on a pollen-only adapter returns nil.
- Asserts ecosystem filter: `LookupAll("npm","pollen")` returns nil.

No static feed fixture created — the test constructs the adapter directly (no `parseAndVerifySelfFeed` involvement). This is the correct approach per the plan guardrails: a static fixture would require a valid Ed25519 signature; the adapter-level test doesn't need it.

### Task 2: selftestEntries + corpus extension (commit 07adf5c)

- Added `catalog.Entry{ID:"pollen-self-2026-001", Ecosystem:"beekeeper", Package:"pollen", Versions:["pollen-test-v0.0.1"], Severity:"critical", CatalogSource:"beekeeper-self"}` to `selftestEntries` in `internal/check/selftest.go`.
- Added corpus fixture to `internal/check/corpus/fixtures.json`: `ecosystem=beekeeper, package=pollen, version=pollen-test-v0.0.1`, `expect_level:"warn"`, `expect_catalog_match:true`.
- Added `TestRunSelftest` to `internal/check/selftest_test.go` as the canonical verification function name.
- `beekeeper selftest` now passes 11 fixtures (was 10), FAIL=0.

## Verification Results

| Command | Result |
|---------|--------|
| `go test ./internal/catalog/ -run TestSelfCatalogAdapter_PollenEntries -count=1` | PASS |
| `go test ./internal/check/ -run TestRunSelftest -count=1` | PASS (11 passed, 0 failed) |
| `go test ./internal/catalog/ ./internal/check/ -count=1` | PASS |
| `go vet ./internal/catalog/ ./internal/check/` | clean |
| `go build ./...` | clean |

## Commits

| Task | Commit | Type | Description |
|------|--------|------|-------------|
| 1 | 225cd66 | test | TestSelfCatalogAdapter_PollenEntries for SDEF-01 pollen discriminator |
| 2 | 07adf5c | feat | extend selftestEntries and corpus with pollen-self fixture (SDEF-01) |

## Deviations from Plan

### Auto-added: TestRunSelftest test function

**Found during:** Task 2 verification
**Issue:** The plan's `<verify>` command `go test ./internal/check/ -run TestRunSelftest -count=1` had no matching test function — the existing function was `TestSelftestAllFixturesPass`. The plan required a `TestRunSelftest` function to exist.
**Fix:** Added `TestRunSelftest` to `selftest_test.go` as a second test function that calls `RunSelftest()` with the same assertions, logging the pass/fail count.
**Files modified:** `internal/check/selftest_test.go`
**Commit:** 07adf5c

**No static feed fixture created:** The plan marks `internal/catalog/testdata/selfcatalog_match_pollen.json` as optional and recommends building the feed in-test instead. The `TestSelfCatalogAdapter_PollenEntries` test directly constructs a `selfCatalogAdapter` with inline entries — no signed feed needed. No fixture file was created, which is correct per the SDEF_01_GUARDRAILS.

## Known Stubs

None — all new code exercises real adapter/selftest logic paths.

## Threat Flags

No new network endpoints, auth paths, or schema changes at trust boundaries introduced by this plan. The pollen-self entry uses the same `selfCatalogAdapter` path that already exists for beekeeper-self entries (T-05-04 mitigated).

## Self-Check: PASSED
