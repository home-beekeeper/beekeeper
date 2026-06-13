---
phase: 21-full-system-validation-ci-calibration
plan: 02
subsystem: testing
tags: [golden-file, deny-contract, harness-conformance, hooks, gitattributes]

requires:
  - phase: 10-hook-block-protocol
    provides: RenderDeny deny contract + the 17-harness installer roster this asserts
provides:
  - "Byte-exact golden deny-contract test for all 15 HarnessIDs + unknown fail-closed (testdata/deny/*.golden + -update)"
  - "TestHarnessConformance: uniform installer-config sweep over all 17 allTargets (no silent skips)"
  - ".gitattributes pinning golden fixtures to LF (cross-platform byte-exact)"
affects: [21-04 (validation-register sources deny families from these contracts)]

tech-stack:
  added: []
  patterns:
    - "Golden-file conformance with a -update flag (canonical exit/stdout/stderr layout)"
    - "Uniform multi-target conformance sweep via InstallTo + t.Setenv home redirect"

key-files:
  created:
    - internal/check/deny_render_golden_test.go
    - internal/check/testdata/deny/*.golden (16 files)
    - internal/hooks/conformance_test.go
    - .gitattributes
  modified: []

key-decisions:
  - "Golden canonical form: exit=N\\nstdout=<bytes>\\nstderr=<bytes>\\n with a FIXED reason → deterministic"
  - "Idempotency = hook-marker count STABLE across re-install (NOT ==1) — multi-event harnesses (Cursor=3) legitimately install several entries"
  - "Conformance uses InstallTo + t.Setenv(HOME/USERPROFILE) for a uniform 17-target sweep; deep per-format preserve stays in the existing per-harness tests (shared PatchSettings helper)"
  - "Backup-on-overwrite asserted only for the 10 JSON fileTargets (Hermes/Cline/OpenCode write custom formats without the backup helper)"

patterns-established:
  - "Honesty seams as explicit named cases: Hermes exit-0+action:block, Kilo/Trae exit-2+empty-stdout, unknown exit-2 fail-closed"
  - "LF-pinned golden fixtures via .gitattributes (core.autocrlf=true safe)"

requirements-completed: [VAL-02]

duration: ~30min
completed: 2026-06-11
---

# Phase 21 Plan 02: 17-Harness Conformance Suite Summary

**A local, deterministic VAL-02 conformance gate: byte-exact golden deny contracts for all 15 HarnessIDs + the unknown fail-closed default, plus a uniform installer-config sweep over all 17 `allTargets` — with the Hermes fail-open and Kilo/Trae UNGUARDED honesty seams as explicit regression-protected cases.**

## Performance

- **Duration:** ~30 min
- **Tasks:** 2
- **Files created:** 19 (1 golden test, 16 goldens, 1 conformance test, .gitattributes)
- **Production files modified:** 0 (test-only — D-03)

## Accomplishments
- **Golden deny contract (VAL-02 deny half)** — `TestRenderDenyGolden` compares `RenderDeny`'s canonical output to a committed `testdata/deny/<harness>.golden` for all 15 HarnessIDs plus the unknown/empty fail-closed default (16 goldens), with a `-update` regeneration flag. The existing substring `TestRenderDeny` is kept as the fast smoke.
- **Honesty seams** — `TestRenderDenyHonestySeams` asserts the regression-critical contracts explicitly: Hermes = **exit 0 + `"action":"block"`** (fail-open), Kilo/Trae = **exit 2 + empty stdout** (UNGUARDED), unknown = exit 2 (fail closed).
- **Installer conformance (VAL-02 installer half)** — `TestHarnessConformance` sweeps all 17 `allTargets` with no silent skips: file-writers create config with the hook marker + are idempotent (marker count stable across re-install) + JSON targets back up on overwrite; the 4 gateway targets print a guide and write no file (Kilo/Trae assert UNGUARDED); Cline errors on Windows.
- **Cross-platform fixture safety** — `.gitattributes` pins the goldens to LF so `core.autocrlf=true` cannot break the byte-exact comparison on a Windows checkout.

## Task Commits

1. **Task 1: golden deny conformance + 16 goldens** — `test(21-02)` (TestRenderDenyGolden + honesty seams + -update)
2. **Task 2: 17-target installer conformance** — `test(21-02)` (TestHarnessConformance over allTargets)
3. **Fixture EOL fix** — `build(21-02)` (.gitattributes LF pin)

## Decisions Made
- **Idempotency contract = stable marker count, not ==1.** Cursor installs 3 hook entries (beforeShellExecution/beforeMCPExecution/beforeReadFile), Windsurf and OpenCode similarly. The real contract is that re-install does not DUPLICATE entries, so the test records count1 then asserts count2 == count1.
- **Uniform sweep via InstallTo + Setenv** rather than per-harness lower-level calls, so every one of the 17 targets is asserted exactly once with no silent skips (the core VAL-02 goal). The deny half is already 95% built (RenderDeny + the substring TestRenderDeny); this plan upgraded it to byte-exact golden.

## Deviations from Plan

### 1. Idempotency assertion corrected from "==1" to "stable count"
- **Found during:** Task 2 — cursor/windsurf/opencode reported the marker 3× after re-install.
- **Issue:** The plan's "idempotent (1 entry)" wording assumed one hook entry per harness; multi-event harnesses legitimately install several (each carrying `beekeeper check`).
- **Fix:** Assert the count is unchanged across re-installs (no duplication), which is the true idempotency contract. No production change.

### 2. Backup-on-overwrite scoped to JSON fileTargets
- **Issue:** Hermes/Cline/OpenCode write custom (YAML/exec/JS) formats and are documented to NOT use the `backupSettings` helper.
- **Fix:** The backup assertion runs only for the 10 `fileTargets`; the custom-format writers assert key + idempotency. Honest to actual installer behavior.

### 3. .gitattributes added (cross-platform fixture EOL)
- **Found during:** committing the goldens — `core.autocrlf=true` warned it would rewrite them to CRLF.
- **Fix:** Pin `testdata/deny/*.golden` to LF so the byte-exact gate holds on Windows and Linux alike.

---

**Total deviations:** 3 (1 assertion correction, 1 honest scoping, 1 cross-platform fixture fix). **Impact:** none on scope; all three make the suite correct and portable. Zero production behavior changed.

## Issues Encountered
- `go test -update <pkg>` flag-ordering: a custom test-binary flag must follow the package path (`go test ./internal/check/ -run X -update`), else `go test` mis-parses and defaults to `.`.

## Next Phase Readiness
- The deny families are golden-pinned — 21-04's `docs/validation-register.md` can source each harness's Expected contract from them.
- Conformance + golden tests are in the default `go test ./...`, so they gate every CI run.

---
*Phase: 21-full-system-validation-ci-calibration*
*Completed: 2026-06-11*
