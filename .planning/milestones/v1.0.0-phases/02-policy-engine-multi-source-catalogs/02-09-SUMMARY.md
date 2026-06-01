---
phase: 02-policy-engine-multi-source-catalogs
plan: "09"
subsystem: fuzz-testing-selftest-corpus
tags: [go, fuzz, self-defense, ctlg-09, plcy-01, release-gate]

# Dependency graph
requires:
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 01
    provides: Evaluate signature + fakeMultiCatalog in engine_test.go
  - phase: 02-policy-engine-multi-source-catalogs
    plan: 08
    provides: full pipeline + MultiIndex + runCheck wiring
provides:
  - "FuzzEvaluate (//go:build fuzz): policy engine fuzz target, seed corpus, Level assertion"
  - "FuzzParseCatalogFile (//go:build fuzz): catalog parser fuzz target, no-panic contract"
  - "CI fuzz job: FuzzEvaluate + FuzzParseCatalogFile with -fuzztime 30s as Phase 2 release gate"
  - "fixture.CheckProvenance / ExpectCorroborationCount / ExpectSourcesAgreed: selftest provenance assertions"
  - "corpus fixtures: two provenance-asserting cases (unsigned single-source warn + allow)"
  - "TestIntegrationTwoSourceBlock: two-signed-source block path with audit-record verification"
  - "TestIntegrationSingleSourceWarn: single-signed-source warn path with audit-record verification"

key-files:
  created:
    - internal/policy/fuzz_test.go
    - internal/catalog/fuzz_test.go
    - internal/check/integration_test.go
  modified:
    - internal/check/selftest.go
    - internal/check/corpus/fixtures.json
    - .github/workflows/ci.yml

key-decisions:
  - "fuzz_test.go files both have //go:build fuzz as the first line — excluded from go test ./... and go build ./... by default"
  - "FuzzEvaluate reuses fakeMultiCatalog{} from engine_test.go (same package, compiled together under -tags fuzz)"
  - "runCheckWithIndex defined in integration_test.go (package check) — uses unexported constants/functions from handler.go without modifying production code"
  - "mapMultiIndex name chosen to avoid conflict with existing fakeMultiIndex in handler_test.go"
  - "CheckProvenance bool sentinel in fixture struct: zero value (false) skips provenance checks, keeping all existing fixtures unaffected"
  - "Selftest bumblebee entries are unsigned → CorroborationCount=0, SourcesAgreed=['bumblebee'] for warn fixtures"

requirements-completed: [PLCY-01, CTLG-02]

# Metrics
duration: ~30min
completed: 2026-05-26
---

# Phase 2 Plan 09: Fuzz Release Gate + Corroboration Integration Tests

**Both fuzz targets wired as a CI release gate; selftest corpus extended with corroboration provenance assertions; hermetic end-to-end integration tests prove the two-source block and single-source warn paths.**

## Performance

- **Duration:** ~30 min
- **Completed:** 2026-05-26
- **Tasks:** 3
- **Files created:** 3
- **Files modified:** 3

## Accomplishments

### Task 1: Fuzz targets for policy engine + catalog parser

- `internal/policy/fuzz_test.go` (`//go:build fuzz`): `FuzzEvaluate` seeds 5 tool-call JSON shapes (Bash command, direct ecosystem shape, scoped npm package, empty object, deeply nested). Fuzz harness: `json.Unmarshal` (ignoring errors) → `Evaluate(tc, fakeMultiCatalog{}, DefaultCorroborationThresholds())` → assert `Level ∈ {allow,warn,block}`. Reuses `fakeMultiCatalog` from `engine_test.go` (same package, compiled together under `-tags fuzz`).

- `internal/catalog/fuzz_test.go` (`//go:build fuzz`): `FuzzParseCatalogFile` seeds 5 inputs (valid catalog, bare top-level array, unknown schema_version, truncated JSON, deeply nested extra fields). Fuzz harness: `ParseCatalogFile(data)` → if no error, assert `cf.SchemaVersion == SupportedSchemaVersion`.

- Verification: `go test -tags fuzz ./internal/policy/... ./internal/catalog/... -run 'FuzzEvaluate|FuzzParseCatalogFile' -count=1` → PASS; `go build ./...` unaffected; `go vet -tags fuzz` → PASS.

- Local 10s fuzz run: `FuzzEvaluate` executed 43,106 inputs, found 0 crashes.

### Task 2: Selftest corpus extension + integration tests

- `internal/check/selftest.go`: `fixture` struct gains `CheckProvenance bool`, `ExpectCorroborationCount int`, `ExpectSourcesAgreed []string`. `fixtureMatches` enforces these when `CheckProvenance=true` (zero value skips, keeping existing fixtures unaffected). Failure log extended with corroboration fields.

- `internal/check/corpus/fixtures.json`: Two new provenance fixtures:
  - "corroboration provenance: single unsigned bumblebee source yields corr_count=0 and agreed=[bumblebee]" — asserts `CorroborationCount=0`, `SourcesAgreed=["bumblebee"]` for the Nx Console case (unsigned selftest entry → signed count 0, but bumblebee appears in agreedList)
  - "corroboration provenance: allow decision yields empty provenance" — asserts `CorroborationCount=0`, `SourcesAgreed=[]` for a clean package

- `internal/check/integration_test.go`: In-package helpers `mapMultiIndex` (pre-canned matches, no disk/network) and `runCheckWithIndex` (reuses handler constants/functions, skips disk opener + adapter construction). Two tests:
  - `TestIntegrationTwoSourceBlock`: two signed sources (bumblebee + osv) → ExitCode non-zero, Level "block", CorroborationCount≥2; audit NDJSON record has `corroboration_count≥2` and both source names in `sources_agreed`
  - `TestIntegrationSingleSourceWarn`: one signed source (bumblebee) → ExitCode 0, Level "warn", CorroborationCount=1; audit record verified

- Verification: `go test ./internal/check/... -count=1` → 16/16 PASS (including both new integration tests and `TestSelftestAllFixturesPass` covering the new corpus fixtures).

### Task 3: CI release-gate job

- `.github/workflows/ci.yml`: Added `fuzz` job (ubuntu-latest, same Go version via `go-version-file: go.mod`):
  - `go test -tags fuzz -run FuzzEvaluate -fuzz FuzzEvaluate -fuzztime 30s ./internal/policy/`
  - `go test -tags fuzz -run FuzzParseCatalogFile -fuzz FuzzParseCatalogFile -fuzztime 30s ./internal/catalog/`
  - Comment identifies this as the Phase 2 release gate per CLAUDE.md, noting Phase 4 adds the MCP message parser gate
  - Existing jobs (build, vet, test, race, go mod verify) unchanged

## Threat Mitigations Implemented

| Threat ID | Mitigation |
|-----------|-----------|
| T-02-09-01 | FuzzEvaluate + FuzzParseCatalogFile assert no-panic over fuzz-generated input; CI fails on any discovered crash |
| T-02-09-02 | FuzzEvaluate asserts `Level ∈ {allow,warn,block}`; a malformed level is a test failure |
| T-02-09-03 | Bounded `-fuzztime 30s` per target in CI keeps the gate fast and deterministic |
| T-02-09-04 | Selftest corpus stays hermetic (embedded, no network); multi-source block path proven by integration test with in-process fakes |

## Self-Check

Files exist:
- `internal/policy/fuzz_test.go` — CREATED
- `internal/catalog/fuzz_test.go` — CREATED
- `internal/check/integration_test.go` — CREATED
- `internal/check/selftest.go` (modified) — MODIFIED
- `internal/check/corpus/fixtures.json` (modified) — MODIFIED
- `.github/workflows/ci.yml` (modified) — MODIFIED

Commit: `6bf6f05`

Verification results:
- `go test -tags fuzz ./internal/policy/... ./internal/catalog/... -run Fuzz*`: PASS
- `go test ./internal/check/... -count=1`: 16/16 PASS
- `go test -tags fuzz -fuzz FuzzEvaluate -fuzztime 10s ./internal/policy/`: 43k execs, no crash
- `go build ./...`: PASS (unaffected by fuzz tag)
- `go vet -tags fuzz ./internal/policy/... ./internal/catalog/...`: PASS
- Full suite `go test ./...`: all packages PASS

## Self-Check: PASSED

---
*Phase: 02-policy-engine-multi-source-catalogs*
*Completed: 2026-05-26*
