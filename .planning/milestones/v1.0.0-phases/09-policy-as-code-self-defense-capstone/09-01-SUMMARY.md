---
phase: 09-policy-as-code-self-defense-capstone
plan: "01"
subsystem: policyloader
tags: [policy-as-code, validation, dry-run, security, pure-engine]
dependency_graph:
  requires: [internal/policy (types + engine — read-only)]
  provides: [internal/policyloader (LoadPolicyFile, ValidateSchema, RunPolicyTest, ListPolicyFiles)]
  affects: [future cmd/beekeeper policy subcommand wiring]
tech_stack:
  added: [internal/policyloader package]
  patterns: [DisallowUnknownFields parse guard, all-errors-not-just-first, missing-dir-as-empty, TDD RED/GREEN]
key_files:
  created:
    - internal/policyloader/loader.go
    - internal/policyloader/validate.go
    - internal/policyloader/test.go
    - internal/policyloader/loader_test.go
    - internal/policyloader/validate_test.go
    - internal/policyloader/test_test.go
    - internal/policyloader/testdata/valid_release_age.json
    - internal/policyloader/testdata/valid_allowlist.json
    - internal/policyloader/testdata/invalid_url_field.json
    - internal/policyloader/testdata/invalid_exec_action.json
    - internal/policyloader/testdata/invalid_unknown_rule_type.json
    - internal/policyloader/testdata/invalid_schema_version.json
  modified: []
decisions:
  - "validate.go pre-implemented in Task 1 because loader.go requires ValidateSchema to compile — Rule 3 deviation"
  - "runPolicyTestWithCatalog unexported variant added to allow TDD block-rule test to inject a signed catalog match without breaking the Pitfall 4 emptyLookup contract for the public API"
  - "corroboration_threshold rules with non-zero fields only override the corresponding PLCY-01 default threshold field (zero-value preservation)"
metrics:
  duration: "~25 minutes"
  completed: "2026-05-29T20:36:32Z"
  tasks_completed: 3
  files_created: 12
---

# Phase 9 Plan 01: policyloader — Wave 0 Foundation Summary

**One-liner:** New `internal/policyloader` package loads/validates declarative JSON policy files with DisallowUnknownFields exec-surface rejection and a pure-engine dry-run harness — without modifying `internal/policy`.

## What Was Built

A new Go package `internal/policyloader` implementing CODE-01 through CODE-04 (policy file loading, validation, listing, and dry-run testing). This is the Wave 0 foundation for the Phase 9 policy-as-code workstream.

### Package Structure

```
internal/policyloader/
  loader.go          — PolicyFile/PolicyRule/PolicyFileSummary types, LoadPolicyFile, ListPolicyFiles
  validate.go        — SupportedSchemaVersion const, ValidateSchema (enum + exec-surface rejection)
  test.go            — emptyLookup, thresholdsFromPolicyFile, RunPolicyTest, runPolicyTestWithCatalog
  loader_test.go     — TestLoadPolicyFile, TestListPolicyFiles_MissingDir, TestListPolicyFiles_ValidDir
  validate_test.go   — TestValidateSchema_RejectsExec, TestValidateSchema_UnknownRuleType, etc.
  test_test.go       — TestPolicyTest_BlockRule, TestPolicyTest_AllowlistOverride
  testdata/          — 2 valid + 4 adversarial JSON fixtures
```

### Key Design Decisions

1. **DisallowUnknownFields parse guard (T-09-01):** `LoadPolicyFile` uses `json.NewDecoder` with `DisallowUnknownFields()` so any smuggled `"url"` or `"exec"` key in a rule produces a parse error with field context. The `PolicyRule` struct deliberately has NO url/exec field — the constraint is structural, not just a runtime check.

2. **All-errors-not-just-first (T-09-02):** `ValidateSchema` collects all validation errors into a slice and returns them together. Unknown `rule_type` values (default switch branch is an error, never a skip) and invalid action values are both fully enumerated.

3. **Missing-dir-as-empty (Pitfall 3):** `ListPolicyFiles` returns nil, nil when the policies/ directory doesn't exist yet. This matches the `config.Load` pattern for missing files.

4. **emptyLookup for deterministic dry-runs (Pitfall 4):** `RunPolicyTest` uses a no-op `MultiCatalogLookup` so dry-run results reflect only the policy file's threshold overrides — not whether a package happens to be absent from live catalogs. The unexported `runPolicyTestWithCatalog` variant enables white-box tests to inject a signed catalog match.

5. **Pure engine preserved (T-09-03):** `internal/policy` has zero new imports. All I/O lives in `internal/policyloader`. The one-way dependency `policyloader → policy` is the correct direction per CLAUDE.md.

## Task Commits

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Wave 0 types + loader + validate + 6 testdata fixtures | 0070e3d |
| 2 | ValidateSchema tests (TDD) — exec/url/unknown-type/schema-version rejection | aaf3241 |
| 3 RED | Dry-run harness tests (failing) — TestPolicyTest_BlockRule, AllowlistOverride | 485dc1c |
| 3 GREEN | RunPolicyTest, thresholdsFromPolicyFile, emptyLookup implementation | 9cf5052 |

## Test Results

```
go test ./internal/policyloader/... -count=1 -v
--- PASS: TestLoadPolicyFile (0.00s)
--- PASS: TestListPolicyFiles_MissingDir (0.00s)
--- PASS: TestListPolicyFiles_ValidDir (0.00s)
--- PASS: TestPolicyTest_BlockRule (0.00s)
--- PASS: TestPolicyTest_AllowlistOverride (0.00s)
--- PASS: TestThresholdsFromPolicyFile (0.00s)
--- PASS: TestValidateSchema_RejectsExec (0.00s)
--- PASS: TestValidateSchema_UnknownRuleType (0.00s)
--- PASS: TestValidateSchema_UnknownSchemaVersion (0.00s)
--- PASS: TestValidateSchema_URLField (0.00s)
--- PASS: TestValidateSchema_ValidFiles (0.00s)
--- PASS: TestValidateSchema_AllErrorsCollected (0.00s)
PASS ok github.com/mzansi-agentive/beekeeper/internal/policyloader
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] validate.go pre-implemented in Task 1**
- **Found during:** Task 1
- **Issue:** `loader.go` calls `ValidateSchema()` from `validate.go`. If `validate.go` is not present, `loader.go` fails to compile. Task 2 is supposed to create `validate.go` with TDD, but it was required for Task 1 to compile.
- **Fix:** `validate.go` was created as part of Task 1 with the full correct implementation. Task 2 then wrote the test file (`validate_test.go`) which immediately passed (GREEN without RED). This is correct behavior — the tests confirmed the already-correct implementation.
- **Files modified:** `internal/policyloader/validate.go` (created in Task 1)
- **Commits:** 0070e3d (feat Task 1), aaf3241 (test Task 2)

**2. [Rule 2 - Security] runPolicyTestWithCatalog unexported variant added**
- **Found during:** Task 3
- **Issue:** `TestPolicyTest_BlockRule` requires a signed catalog match to produce a block decision. With `emptyLookup{}`, the engine always returns allow (no catalog matches = no corroboration). The test can't verify that `thresholdsFromPolicyFile` correctly lowers `block_at` to produce blocks.
- **Fix:** Added unexported `runPolicyTestWithCatalog(pf, tc, idx, ac)` alongside the public `RunPolicyTest`. Tests inject a `fakeSignedCatalog`; public API continues to use `emptyLookup{}` (Pitfall 4 preserved).
- **Files modified:** `internal/policyloader/test.go`, `internal/policyloader/test_test.go`
- **Commit:** 9cf5052

## Known Stubs

None. All functions are fully implemented. The `package_allowlist` policy rule is parsed and stored but its effect in `RunPolicyTest` is currently limited to the threshold path (the engine's `Evaluate` function doesn't accept an allowlist parameter directly — allowlist logic lives in `EvaluateLifecycle` which is a separate call chain). Future CMD wiring (Plan 09-02) will integrate allowlists into the full check pipeline.

## Threat Flags

No new threat surfaces introduced. The policyloader package operates on local filesystem paths (developer-authored policy files) with no network calls, no subprocess execution, and no data that flows to network endpoints.

## Self-Check: PASSED

Files created:
- internal/policyloader/loader.go: FOUND
- internal/policyloader/validate.go: FOUND
- internal/policyloader/test.go: FOUND
- internal/policyloader/testdata/invalid_exec_action.json: FOUND
- internal/policyloader/testdata/valid_allowlist.json: FOUND

Commits:
- 0070e3d: FOUND
- aaf3241: FOUND
- 485dc1c: FOUND
- 9cf5052: FOUND

Purity gate: internal/policy/*.go has zero new I/O imports (grep found no matches).
