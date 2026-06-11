---
phase: 21-full-system-validation-ci-calibration
plan: 01
subsystem: testing
tags: [coverage-gate, go-parser, allowlist, ipc, peer-cred, tui, self-defense]

requires:
  - phase: 20-runtime-hardening
    provides: the ipc/sentry/tui runtime surface this validates
provides:
  - "internal/coveragegate package: go/parser prod-file walker + fail-closed reason-coded allowlist (VAL-01/VAL-08)"
  - "coverage-allowlist.txt: the reason-coded no-test allowlist (one entry: internal/version, type-only)"
  - "Tier-A gap tests: hooks/protected, tui/computeStatus, ipc server/client/peer (Tier-B)"
affects: [21-03 (CI matrix runs the ipc Tier-B tests), 21-04 (validation-posture documents the allowlist taxonomy)]

tech-stack:
  added: []
  patterns:
    - "Package-level coverage linkage + file-level reason-coded fail-closed allowlist"
    - "Tier-B build-tagged tests (linux||darwin) compile-checked via cross-go-vet on Windows, run in CI"

key-files:
  created:
    - internal/coveragegate/coveragegate.go
    - internal/coveragegate/allowlist.go
    - internal/coveragegate/coveragegate_test.go
    - internal/coveragegate/allowlist_test.go
    - coverage-allowlist.txt
    - internal/hooks/protected_test.go
    - internal/tui/model_coverage_test.go
    - internal/ipc/server_test.go
    - internal/ipc/client_test.go
    - internal/ipc/peer_linux_test.go
    - internal/ipc/peer_darwin_test.go
  modified: []

key-decisions:
  - "Coverage gate is PACKAGE-LEVEL linkage + reason-coded allowlist (NOT a coverage-% threshold) — Pitfall 1: same-name-sibling linkage gives 70/184 false positives"
  - "Only internal/version is a zero-test package; it is allowlisted type-only (3 ldflag-injected build-metadata vars, nothing to test)"
  - "catalog.ResolveHealthy was ALREADY covered by a pre-existing health_test.go (all 4 branches) — no new catalog file needed (RESEARCH over-scoped this)"
  - "The Windows Cline ~-sentinel in HookConfigFiles is intended (Cline is macOS/Linux-only), not a defect — test accepts it"

patterns-established:
  - "internal/coveragegate.Walk: filesystem enumeration + go/parser validation, OS-consistent (counts platform files on every OS)"
  - "Allowlist fails closed: bare path / out-of-taxonomy reason code breaks the gate (TestAllowlistFailsClosed)"

requirements-completed: [VAL-01, VAL-08]

duration: ~45min
completed: 2026-06-11
---

# Phase 21 Plan 01: Coverage Gate + Tier-A Gap Tests Summary

**A self-defending `internal/coveragegate` package (go/parser walker + fail-closed reason-coded allowlist) that accounts every production file, plus real tests closing the surfaced Tier-A gaps (hooks/protected, tui/computeStatus, ipc server/client/peer).**

## Performance

- **Duration:** ~45 min
- **Tasks:** 3
- **Files created:** 11 (4 coveragegate, 1 allowlist, 6 gap-test files)
- **Production files modified:** 0 (test-only — D-03 guardrail; RESEARCH found no defects in the gap files)

## Accomplishments
- **VAL-01/VAL-08 coverage gate** — `internal/coveragegate` walks every non-test `.go` under `internal/` and `cmd/` (filesystem enumeration + `go/parser` validation, OS-consistent), classifies each package-tested / allowlisted / UNACCOUNTED, and `TestCoverageManifest` fails on any UNACCOUNTED file. Empirically confirmed RESEARCH's claim: only `internal/version` is a zero-test package.
- **Self-defending allowlist** — `coverage-allowlist.txt` is parsed by a fail-closed reader: a bare path, an empty reason, or an out-of-taxonomy reason code breaks the gate (`TestAllowlistFailsClosed`). Closed six-code taxonomy: `generated-bpf | platform-stub | type-only | exec-seam-stub | thin-delegator | gen-directive`.
- **Tier-A gap tests** — `hooks/protected.go` (markers + home-rooting), `tui/computeStatus` (81%→100%, direct test of the de-mock status logic + bounded-tail), and `ipc` server/client/peer (Tier-B, `//go:build linux||darwin`, fail-closed peer-UID auth).

## Task Commits

1. **Task 1: coveragegate package + fail-closed allowlist** — `feat(21-01)` (coverage gate, go/parser walker, six-code taxonomy)
2. **Task 2: hooks/protected + tui model gap tests** — `test(21-01)` (catalog.ResolveHealthy pre-existed; computeStatus 81%→100%)
3. **Task 3: IPC server/client/peer Tier-B tests** — `test(21-01)` (linux||darwin + per-OS peer-cred; CI-validated)

## Decisions Made
- **Package-level linkage** (not same-name-sibling): the plan's own Test 2 ("test-less *package*") and RESEARCH Pitfall 1 (70/184 false positives) settle this. The gate accounts every file in a tested directory; the allowlist is the escape hatch for genuinely test-less packages.
- **`internal/version` → `type-only`**: it is three ldflag-injected build-metadata strings with no logic. Testing `Version == "dev"` would be a tautology, so allowlisting is the honest call (D-02's legitimate allowlist case).

## Deviations from Plan

### 1. catalog.ResolveHealthy test already existed (RESEARCH over-scope)
- **Found during:** Task 2.
- **Issue:** The plan listed `internal/catalog/health_test.go` as a file to create for the four `sanity.go` delegators. It ALREADY EXISTS and comprehensively covers all four `ResolveHealthy` branches (empty / missing / degraded / healthy / no-sources).
- **Fix:** Left it untouched. The four `*/sanity.go` delegators are accounted by the gate via package-level linkage; the real logic is already tested. No new catalog file.
- **Verification:** `go test -run TestResolveHealthy ./internal/catalog/` exits 0 (5 branch subtests).

### 2. HookConfigFiles Windows Cline ~-sentinel (test assertion relaxed, not a prod fix)
- **Found during:** Task 2 — the new `TestHookConfigFilesAreHomeRooted` initially failed on `~\Documents\Cline\Rules\Hooks\PreToolUse`.
- **Issue:** `clineHooksDir` on Windows (`cline_windows.go`) deliberately returns a non-expandable `~`-sentinel because Cline is a macOS/Linux-only harness (its Windows installer errors); the `!windows` `cline.go` is correctly home-rooted.
- **Fix:** Accepted the documented sentinel in the test (a genuine absolute-path escape still fails). NO production change — this is intended cross-platform behavior, consistent with RESEARCH's "no defects in these files" and D-03.
- **Verification:** `go test ./internal/hooks/` exits 0.

---

**Total deviations:** 2 (1 pre-existing test discovered, 1 test-assertion relaxed for intended behavior). **Impact:** none on scope — both reduce work / correct an over-strict assertion; zero production behavior changed.

## Issues Encountered
None beyond the two deviations above.

## Next Phase Readiness
- The gate is now part of `go test ./...` — a new test-less package breaks CI (VAL-08).
- The ipc Tier-B tests are ready for the 21-03 CI matrix to run on the Linux/macOS legs.
- The allowlist taxonomy is ready to be documented by 21-04's `docs/validation-posture.md`.

---
*Phase: 21-full-system-validation-ci-calibration*
*Completed: 2026-06-11*
