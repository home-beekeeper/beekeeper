---
phase: 08-package-manager-nudge-behavioral-test-suite
plan: "07"
subsystem: testing
tags: [testing, nudge, behavioral, integration, e2e, BTEST-02, BTEST-03, release-gate, DetectStateFn, BEEKEEPER_HOME]

# Dependency graph
requires:
  - phase: 08-04
    provides: "nudge.DetectStateFn (exported cross-package seam for TestIntegration injection)"
  - phase: 08-05
    provides: "BEEKEEPER_HOME env override — hermetic E2E state/audit/catalog dir"
  - phase: 08-06
    provides: "evaluateNudge in nudge_adapter.go, writeNudgeAuditRecord, runCheck nudge merge"
provides:
  - "BTEST-02: TestIntegrationNudgePnpmAddEvilPkg (F3 end-to-end, pnpm → npm::evil-pkg catalog block)"
  - "BTEST-02: TestIntegrationNudgeSoftAdvisory (DetectStateFn swap, exit 0, record_type nudge)"
  - "BTEST-02: TestIntegrationNudgeNonInstallSkipped (npm ls, no nudge record §10-7)"
  - "BTEST-02: readAuditRecordByType helper for record_type-filtered NDJSON scanning"
  - "BTEST-02: nudge merge added to runCheckWithIndex (mirrors production CR-02 ordering)"
  - "BTEST-03: TestE2ELiveBinary release gate — compiled binary, real catalog, hermetic BEEKEEPER_HOME"
  - "BTEST-03: SPATH (credential read exit 1), CORR (ai-figure critical exit 1), NUDGE (pnpm add chalk audit record)"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "DetectStateFn seam swap pattern: orig := nudge.DetectStateFn; defer func(){nudge.DetectStateFn = orig}() for cross-package PMState injection in package check tests"
    - "E2E binary harness: go build → exec.Command(binPath, check) with BEEKEEPER_HOME=t.TempDir() + catalog.BuildIndex seed"
    - "readAuditRecordByType: record_type-filtered NDJSON scanner for multi-record audit files"

key-files:
  created:
    - internal/check/e2e_test.go
  modified:
    - internal/check/integration_test.go

key-decisions:
  - "runCheckWithIndex extended with nudge merge (not a new testable variant) so integration tests exercise the live nudge wiring path rather than bypassing it"
  - "CORR E2E uses a single critical bumblebee entry (DefaultCorroborationThresholds SeverityOverrides.critical.BlockAt=1) — no OSV/Socket network required for hermetic block assertion"
  - "NUDGE E2E uses REAL pnpm on PATH (no DetectStateFn swap in child process — proves shipped detection path); pnpm 11.1.3 present on dev box → advisory fires"
  - "readAuditRecordByType added as helper alongside readLastAuditRecord — returns last record of a specific type from multi-record NDJSON audit file"

requirements-completed: [BTEST-01, BTEST-02, BTEST-03, NUDGE-01, NUDGE-08]

# Metrics
duration: ~20min
completed: "2026-06-04"
tasks_completed: 2
tasks_total: 2
files_created: 1
files_modified: 1
---

# Phase 8 Plan 07: Behavioral Test Battery Summary

**v1.2.0 release-gate test battery: BTEST-02 RunCheck integration cases (DetectStateFn seam swap, F3 end-to-end) + BTEST-03 live-binary E2E (SPATH/CORR/NUDGE hermetic via BEEKEEPER_HOME)**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-06-04T14:00:00Z
- **Completed:** 2026-06-04T14:20:00Z
- **Tasks:** 2
- **Files created:** 1 (e2e_test.go)
- **Files modified:** 1 (integration_test.go)

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | BTEST-02 — pnpm/bun nudge integration cases via DetectStateFn seam | 78abdd7 | internal/check/integration_test.go |
| 2 | BTEST-03 — live-binary E2E release gate (SPATH + CORR + NUDGE) | cf36cf7 | internal/check/e2e_test.go |

## Accomplishments

### Task 1 — BTEST-02 Integration Cases

- Extended `runCheckWithIndex` with the nudge merge block (mirrors production `runCheck` nudge block, CR-02 ordering: AFTER overlay + SPATH) so all integration tests exercise the live nudge wiring path
- Added `readAuditRecordByType` helper — returns the last NDJSON record of a given record_type from a multi-record audit file
- **Case (a) F3 end-to-end:** `TestIntegrationNudgePnpmAddEvilPkg` — `pnpm add evil-pkg` with a 2-source catalog match at key `npm::evil-pkg` → exit 1 / decision "block"; confirms pnpm→npm ecosystem mapping (F3/SC1)
- **Case (b) soft advisory:** `TestIntegrationNudgeSoftAdvisory` — swaps `nudge.DetectStateFn` with a stub returning `PMState{PnpmInstalled:true, PnpmVersion:"11.5.1", NodeVersion:"22.5.0", PnpmHardened:true}` (EXPORTED seam + `defer` restore — NOT the unexported `pnpmVersionFn`/`nodeVersionFn`); asserts exit 0, `record_type:"nudge"`, `nudge_action:"advise"`, `reason_code:"pnpm-available-soft"` (NUDGE-03 live)
- **Case (c) non-install:** `TestIntegrationNudgeNonInstallSkipped` — `npm ls` → exit 0 / no nudge record (§10-7 Pitfall 2 confirmed)
- All 5 integration tests pass: `go test -run TestIntegration ./internal/check/...` green

### Task 2 — BTEST-03 Live-Binary E2E Release Gate

- Created `internal/check/e2e_test.go` with `//go:build e2e` and RELEASE-GATE header (mirrors `parser_fuzz_test.go` convention)
- `TestE2ELiveBinary` builds the compiled binary via `go build -o <tmp>/beekeeper github.com/home-beekeeper/beekeeper/cmd/beekeeper` and drives 4 sub-cases with `cmd.Env = append(os.Environ(), "BEEKEEPER_HOME=<tmpdir>")`:
  - **SPATH:** `~/.aws/credentials` Read → exit 1 / audit `decision:"block"` — credential path blocked by real `EvaluatePath`
  - **CORR:** `npm install ai-figure` with single critical bumblebee entry → exit 1 / audit `decision:"block"` (DefaultCorroborationThresholds `SeverityOverrides["critical"].BlockAt=1`)
  - **NUDGE:** `pnpm add chalk` → exit 0 / `record_type:"nudge"` present / decision non-block (real pnpm 11.1.3 on dev box, real `DetectState` in child process, no seam swap)
  - **BUN:** `t.Skip("bun not installed")` — bun absent on dev box, skips gracefully
- Hermetic: each sub-case uses its own `t.TempDir()` as BEEKEEPER_HOME; never touches the developer's real `~/.beekeeper`
- `go test -tags e2e -run=TestE2ELiveBinary ./internal/check/...` passes in ~20s (build ~5s + 3 live cases)

## Deviations from Plan

None — plan executed exactly as written. The nudge merge in `runCheckWithIndex` was the explicit design intent per the plan note: "drive the production RunCheck or a runCheckWithIndex that now includes the nudge merge added in Plan 06".

## Known Stubs

None. All assertions are concrete and wired to real behavior.

## Threat Surface Scan

All STRIDE threats from the plan's `<threat_model>` are addressed:

| Threat | Mitigation | Status |
|--------|------------|--------|
| T-08-21 (regression in live wiring) | BTEST-02 drives raw stdin through the LIVE nudge merge path; future refactors that disconnect nudge from runCheck fail the integration gate | Applied |
| T-08-22 (shipped binary diverges from unit behavior) | BTEST-03 exercises the actual compiled binary against the real catalog for SPATH+CORR+NUDGE — must pass before v1.2.0 tag | Applied |
| T-08-23 (non-hermetic test pollutes real state) | BEEKEEPER_HOME per-case t.TempDir() isolation; E2E never touches ~/.beekeeper | Applied |

No new network endpoints, auth paths, file-write surfaces, or security-relevant surfaces introduced.

## Self-Check: PASSED

**Files created:**
- `internal/check/e2e_test.go` — FOUND

**Files modified:**
- `internal/check/integration_test.go` — FOUND

**Commits verified:**
- `78abdd7` `test(08-07): BTEST-02 — pnpm/bun nudge integration cases via DetectStateFn seam` — FOUND
- `cf36cf7` `test(08-07): BTEST-03 — live-binary E2E release gate (SPATH + CORR + NUDGE)` — FOUND

**Build + test verification:**
- `go build ./...` — CLEAN
- `go test -run TestIntegration ./internal/check/...` — ALL PASS (5 integration tests)
- `go test -tags e2e -run TestE2ELiveBinary ./internal/check/...` — ALL PASS (3 pass + 1 skip)

---
*Phase: 08-package-manager-nudge-behavioral-test-suite*
*Completed: 2026-06-04*
