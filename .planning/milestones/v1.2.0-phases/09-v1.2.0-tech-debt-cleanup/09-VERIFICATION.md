---
phase: 09-v1.2.0-tech-debt-cleanup
verified: 2026-06-04T00:00:00Z
status: passed
score: 9/9 must-haves verified
resolved_gaps:
  - truth: "Full v1.2.0 suite + fuzz (-tags fuzz) + live-binary E2E (-tags e2e) remain green (SC9)"
    status: resolved
    resolution: >
      Closed by orchestrator commit ef4ea97 (2026-06-04): internal/policy/fuzz_test.go:36
      now passes AgentContext{} as the 4th Evaluate() argument. This was a PRE-EXISTING build
      break (AgentContext added in c1051a2 / v1.0.0 Phase 4; fuzz file never updated; reproduced
      at e2a821a~1, before Phase 9) — NOT a Phase 9 regression. Phase 9's SC9 fuzz release gate
      surfaced it. After the fix the full fuzz gate is green:
      `go test -tags fuzz ./internal/{policy,pkgparse,nudge,gateway}/` → all ok (exit 0).
verification_runs:
  - "go build ./... → clean (exit 0)"
  - "go test ./... (full unit suite) → all packages ok (no FAIL)"
  - "go test -tags e2e -run TestE2ELiveBinary ./internal/check/ → ok 15.4s (SPATH+CORR+NUDGE pass; bun SKIP) — RUN by orchestrator (resolves the prior human-verification note)"
  - "go test -tags fuzz ./internal/{policy,pkgparse,nudge,gateway}/ → all ok (after ef4ea97)"
  - "TestPathImportsArePure + TestOverlayAllowCannotDowngradePathBlock → green (purity + CR-02 intact)"
---

# Phase 9: v1.2.0 Tech-Debt Cleanup — Verification Report

**Phase Goal:** Clear the tech debt surfaced by the v1.2.0 milestone audit before closing the milestone — so the release gate is network-independent, layered config is correct at its root, the SPATH credential block resists known evasion edges, and the version-drift path emits live records — without regressing the F1/F2/F3 enforcement proven in Phases 6–8.

**Verified:** 2026-06-04T00:00:00Z
**Status:** passed (9/9 — the lone SC9 gap, a pre-existing `internal/policy/fuzz_test.go` build break, was fixed by orchestrator commit `ef4ea97`; full fuzz gate now green)
**Re-verification:** No — initial verification (gap closed inline post-verification)

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | CLEAN-01: E2E CORR block fires from signed non-wildcard local fixture (network-independent) | VERIFIED | `e2e_test.go` L156–180: `Versions:["1.0.0"]`, `CatalogSignature:"sha256:e2e-corr-test-sig"` (non-empty → Signed:true), `Severity:"critical"`, stdin `npm install ai-figure@1.0.0`. Mirrors hermetic unit test pattern. OSV unreachable cannot change outcome. |
| 2 | CLEAN-02: `config.LoadLayered` merges `Nudge *NudgeConfig` pointer at its root via `mergeNudge`; layered-config tests prove defaulting + project-override + project-disable without any consumer helper | VERIFIED | `layered.go` L257–318: `mergeNudge` function with `srcHasOtherSignal` bool-disambiguation; `LoadLayered` L109–114 adds non-nil + `ValidateNudgeConfig` guarantee. Three `TestLoadLayeredNudge*` tests (A/B/C) assert directly on `LoadLayered` output with no consumer helper. `TestMerge_Nudge*` (4 tests) cover merge mechanics. All green. |
| 3 | CLEAN-03: `06-VALIDATION.md` frontmatter is `nyquist_compliant: true` and `status: approved`, backed by a Reconciliation Note citing `06-VERIFICATION.md` | VERIFIED | `06-VALIDATION.md` L1–9: `status: approved`, `nyquist_compliant: true`, `wave_0_complete: true`, `reconciled: 2026-06-04`. Body L17–28 contains evidence-backed Reconciliation Note. Commit `311d0b7`. |
| 4 | CLEAN-04: `handler.go` decision-merge comment reflects overlay → SPATH → NUDGE order | VERIFIED | `handler.go` L259–271: comment text "1. ApplyPolicyOverlay (FIRST)", "2. SPATH block", "3. NUDGE block (LAST)" with CR-02 rationale. "merged LAST" language removed. |
| 5 | HARDEN-01: `canonicalizePathForms` returns both lexical and EvalSymlinks-resolved forms; wired into `runCheck` and `runCheckWithIndex`; ancestor-symlink credential read blocks end-to-end | VERIFIED | `paths.go` L165–227: `canonicalizePathForms` returns lexical form (pre-EvalSymlinks) and resolved form, de-duplicated. `handler.go` L293: `for _, resolved := range canonicalizePathForms(rawPath)`. `integration_test.go` L84: same loop. `TestIntegrationAncestorSymlinkCredentialBlocks` (L428–473): PASS confirmed by live run. `TestCanonicalizePathForms/ancestor_symlink` PASS. |
| 6 | HARDEN-02: `normalizeBasename` strips NTFS-ADS suffix + trailing dots/spaces; applied in both `matchesBlockPattern` and `isAllowedPath` basename branches; `id_rsa:stream`, `credentials.`, `credentials ` block; `.env.example:stream` stays allowed | VERIFIED | `path.go` L170–177: `normalizeBasename` strips first `:` and `TrimRight(". ")`. L142: applied in `matchesBlockPattern` basename branch. L199: applied in `isAllowedPath` basename branch. `TestNormalizeBasename`, `TestEvaluatePathBasenameADSBlock`, `TestEvaluatePathAllowlistNormalizationAligned` all PASS. `TestPathImportsArePure` PASS (pure — `strings` only). |
| 7 | HARDEN-03: `isShellBoundary` + left-word-boundary guard in `extractBashCredentialPaths`; verb-substrings (`./catalog.sh`, `scatter`) do not false-trigger; real `more ~/.ssh/id_rsa` still flags | VERIFIED | `paths.go` L265–272: `isShellBoundary` returns true for `{' ', '\t', '\n', '\r', ';', '|', '&', '('}`. L337–347: boundary guard `if idx != 0 && !isShellBoundary(cmd[idx-1]) { from = idx+1; continue }`. `TestExtractBashCredentialPaths` subtests `embedded 'cat' in catalog.sh does NOT extract` and `embedded 'cat' in scatter does NOT extract` both PASS. `standalone more still flags` PASS. |
| 8 | DRIFT-01: `realMetadataFetch` performs a real npm dist-tags HTTP query; per-PM fail-open; floors never auto-bumped; `go.mod`/`go.sum` unchanged | VERIFIED | `drift.go` L86–136: real `http.Client{Timeout:5s}`, iterates `{"pnpm","bun"}`, GETs `<base>/-/package/<pm>/dist-tags`, `io.LimitReader` 256KB cap, per-PM `continue` on any error, returns nil error. `TestRealMetadataFetchParsesDistTags`, `TestRealMetadataFetchFailOpenOnError`, `TestCheckDriftEndToEndRealFetch`, `TestCheckDriftFloorsNeverBumped` all PASS. `git diff HEAD~15 go.mod go.sum` empty. |
| 9 | SC9: `go build ./...` clean; full unit suite green; `-tags fuzz` seeds green; `-tags e2e TestE2ELiveBinary` green; `TestPathImportsArePure` green; `TestOverlayAllowCannotDowngradePathBlock` green | PARTIAL — 1 sub-item FAILED | `go build ./...` CLEAN. `go test ./...` all 24 packages PASS. `TestPathImportsArePure` PASS. `TestOverlayAllowCannotDowngradePathBlock` PASS. `-tags fuzz ./internal/policy/` FAILS: `fuzz_test.go:36` calls `Evaluate` with 3 args; signature requires 4 (AgentContext missing). `TestIntegrationAncestorSymlinkCredentialBlocks` PASS (ran, not skipped). See gap note. |

**Score:** 8/9 must-haves verified (1 partial — SC9 fuzz build failure)

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/check/paths.go` | `canonicalizePathForms` + `isShellBoundary` + boundary-anchored `extractBashCredentialPaths` | VERIFIED | All three present, substantive, wired into SPATH loops in `handler.go` and `integration_test.go` |
| `internal/policy/path.go` | `normalizeBasename` applied in `matchesBlockPattern` and `isAllowedPath` basename branches | VERIFIED | Present, pure (strings only), applied in both branches |
| `internal/check/paths_test.go` | HARDEN-01/02/03 regression tests | VERIFIED | `TestCanonicalizePathForms`, `TestCanonicalizeEvaluateADS`, HARDEN-03 subtests all present and green |
| `internal/policy/path_test.go` | ADS/trailing-dot pure tests + `TestPathImportsArePure` | VERIFIED | `TestNormalizeBasename`, `TestEvaluatePathBasenameADSBlock`, `TestEvaluatePathAllowlistNormalizationAligned` present and green |
| `internal/check/e2e_test.go` | Signed non-wildcard `ai-figure` fixture with `CatalogSignature` set | VERIFIED | L158–168: `Versions:["1.0.0"]`, `CatalogSignature:"sha256:e2e-corr-test-sig"`, `Severity:"critical"` |
| `internal/config/layered.go` | `mergeNudge()` function + Nudge non-nil guarantee in `LoadLayered` | VERIFIED | `mergeNudge` L257–318 (with `srcHasOtherSignal` rule), guarantee L109–114 |
| `internal/config/layered_test.go` | `TestLoadLayeredNudge*` (A/B/C) asserting directly on `LoadLayered` output | VERIFIED | All three tests present (L253–320), green |
| `internal/check/handler.go` | Corrected merge-order comment + dual-form SPATH loop using `canonicalizePathForms` | VERIFIED | Comment L259–271 lists overlay→SPATH→NUDGE; SPATH loop L293 uses `canonicalizePathForms` |
| `internal/check/integration_test.go` | `runCheckWithIndex` dual-form SPATH loop + `TestIntegrationAncestorSymlinkCredentialBlocks` | VERIFIED | L84 `canonicalizePathForms` loop; L428–473 regression test; test RAN and PASSED |
| `internal/gateway/drift.go` | `realMetadataFetch` with real HTTP, per-PM fail-open, floors unchanged | VERIFIED | L86–136 full implementation; no new deps; `emitVersionDrift` never mutates floors |
| `internal/gateway/drift_test.go` | 4 new httptest-backed tests for parse/fail-open/end-to-end/floors | VERIFIED | `TestRealMetadataFetchParsesDistTags`, `TestRealMetadataFetchFailOpenOnError`, `TestCheckDriftEndToEndRealFetch`, `TestCheckDriftFloorsNeverBumped` all green |
| `.planning/phases/06-corroboration-severity-hardening/06-VALIDATION.md` | `nyquist_compliant: true`, `status: approved`, Reconciliation Note | VERIFIED | Frontmatter L1–9 confirmed; body note L17–28 cites 06-VERIFICATION.md evidence |
| `internal/policy/fuzz_test.go` | Not in Phase 9 scope (not modified) | PRE-EXISTING FAILURE | `Evaluate` called with 3 args; requires 4. Failure predates Phase 9 (confirmed at e2a821a~1). |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `handler.go` `runCheck` SPATH loop | `policy.EvaluatePath` | `canonicalizePathForms` iteration (both forms) | WIRED | `handler.go` L293: `for _, resolved := range canonicalizePathForms(rawPath)` then `EvaluatePath(resolved, spathCfg)` |
| `integration_test.go` `runCheckWithIndex` SPATH loop | `policy.EvaluatePath` | `canonicalizePathForms` iteration (lockstep with production) | WIRED | `integration_test.go` L84: same loop pattern as handler.go |
| `config.LoadLayered` `merge()` | `NudgeConfig` pointer | `mergeNudge(dst.Nudge, src.Nudge)` | WIRED | `layered.go` L215: `dst.Nudge = mergeNudge(dst.Nudge, src.Nudge)` |
| `realMetadataFetch` | npm registry dist-tags | `http.Client` GET `/-/package/<pm>/dist-tags` | WIRED | `drift.go` L87–129; `metadataFetchFn = realMetadataFetch` L58 |
| `06-VALIDATION.md` frontmatter | `06-VERIFICATION.md` evidence | Reconciliation Note body text | WIRED | `06-VALIDATION.md` L17–28 Reconciliation Note cites passed 5/5 |
| `matchesBlockPattern` basename branch | `normalizeBasename` | Applied to `lastSegment(resolvedPath)` before comparison | WIRED | `path.go` L142: `seg := normalizeBasename(lastSegment(resolvedPath))` |
| `isAllowedPath` basename branch | `normalizeBasename` | Applied to `lastSegment(resolvedPath)` before comparison | WIRED | `path.go` L199: `seg := normalizeBasename(lastSegment(resolvedPath))` |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `drift.go realMetadataFetch` | `result map[string]string` | npm registry `/-/package/<pm>/dist-tags` via `http.Client` | Yes — real HTTP GET; per-PM fail-open on non-200/error | FLOWING |
| `drift.go emitVersionDrift` | `version_drift` audit record | `h.cfg.Nudge.VersionFloors` (read-only; never mutated) | Yes — reads real floor values; never writes back | FLOWING |
| `e2e_test.go CORR_aifigure_critical_block` | `exitCode`, `rec.Decision` | `catalog.BuildIndex` with explicit `CatalogSignature`; `runCase` drives real binary | Yes — block comes from signed non-wildcard local fixture | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` clean | `go build ./...` | No output (exit 0) | PASS |
| Full unit suite | `go test ./...` | 24 packages PASS | PASS |
| `TestPathImportsArePure` | `go test ./internal/policy/ -run TestPathImportsArePure` | PASS | PASS |
| `TestOverlayAllowCannotDowngradePathBlock` (CR-02) | `go test ./internal/check/ -run TestOverlayAllowCannotDowngradePathBlock` | PASS (3.44s) | PASS |
| `TestIntegrationAncestorSymlinkCredentialBlocks` | `go test ./internal/check/ -run TestIntegrationAncestorSymlinkCredentialBlocks` | PASS (0.04s) — ran, NOT skipped | PASS |
| `TestMerge_Nudge*` + `TestLoadLayeredNudge*` | `go test ./internal/config/ -run "TestMerge_Nudge|TestLoadLayeredNudge"` | 7 tests PASS | PASS |
| `TestCheckDriftFloorsNeverBumped` | `go test ./internal/gateway/ -run TestCheckDriftFloorsNeverBumped` | PASS | PASS |
| `-tags fuzz ./internal/policy/` | `go test -tags fuzz ./internal/policy/` | BUILD FAILED: `fuzz_test.go:36` missing AgentContext arg | FAIL |
| `-tags fuzz` all other packages | `go test -tags fuzz ./...` (excl. policy) | All other 23 packages PASS | PASS |

---

### Requirements Coverage

| Requirement | Phase | Description | Status | Evidence |
|-------------|-------|-------------|--------|----------|
| CLEAN-01 | Phase 9 | Hermetic CORR E2E — signed non-wildcard fixture | SATISFIED | `e2e_test.go` CORR case; `Versions:["1.0.0"]`, `CatalogSignature` set; commit `e2a821a` |
| CLEAN-02 | Phase 9 | `LoadLayered` merges `Nudge` pointer at root | SATISFIED | `layered.go` `mergeNudge` + LoadLayered guarantee; commits `564104b`, `350b44f` |
| CLEAN-03 | Phase 9 | `06-VALIDATION.md` reconciled to COMPLIANT | SATISFIED | `06-VALIDATION.md` `nyquist_compliant: true`, `status: approved`; commit `311d0b7` |
| CLEAN-04 | Phase 9 | `handler.go` merge-order comment corrected | SATISFIED | `handler.go` L259–271 overlay→SPATH→NUDGE; commit `a06524a` |
| HARDEN-01 | Phase 9 | Ancestor-symlink dual-form credential block | SATISFIED | `canonicalizePathForms` in `paths.go`; wired at both SPATH call sites; `TestIntegrationAncestorSymlinkCredentialBlocks` PASS |
| HARDEN-02 | Phase 9 | Windows ADS + trailing-dot basename normalization | SATISFIED | `normalizeBasename` in `path.go`; OS-agnostic pure tests + Windows-gated adapter tests; `TestPathImportsArePure` green |
| HARDEN-03 | Phase 9 | Left word-boundary read-verb matching | SATISFIED | `isShellBoundary` + boundary guard in `extractBashCredentialPaths`; boundary subtests PASS |
| DRIFT-01 | Phase 9 | Real npm dist-tags fetch; fail-open; floors unchanged | SATISFIED | `realMetadataFetch` in `drift.go`; 4 httptest tests; `go.mod`/`go.sum` unchanged |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/policy/fuzz_test.go` | 36 | Stale 3-arg `Evaluate()` call (AgentContext param missing) | BLOCKER | `-tags fuzz` build fails for `internal/policy`; this is a pre-existing bug (file last modified Phase 2, AgentContext added Phase 4); SC9 requires fuzz seeds green |

**Debt-marker scan:** No unreferenced TBD/FIXME/XXX markers found in any file modified by Phase 9.

---

### Human Verification Required

#### 1. Live-Binary E2E (`-tags e2e TestE2ELiveBinary`)

**Test:** `go test -tags e2e -run TestE2ELiveBinary ./internal/check/ -v`
**Expected:** All sub-cases PASS — `SPATH_credential_block` (exit 1/block), `CORR_aifigure_critical_block` (exit 1/block), `NUDGE_pnpm_add_chalk` (exit 0/nudge record), `NUDGE_bun_add_chalk` (skip or pass)
**Why human:** Requires compiling the beekeeper binary (~2min build) and exercising nudge detection against the real pnpm binary on PATH. The CORR fixture wiring and all behavioral preconditions are confirmed at the code level. The SUMMARY records this ran PASS (commit `e2a821a`), but the verifier cannot confirm binary execution outcomes programmatically.

---

### Gaps Summary

**1 BLOCKER gap (pre-existing):** `internal/policy/fuzz_test.go:36` calls `Evaluate()` with 3 arguments; the function requires 4 (`AgentContext` was added at v1.0.0 Phase 4, commit `c1051a2`; `fuzz_test.go` was last touched at Phase 2 `6bf6f05` and never updated). This causes `-tags fuzz ./internal/policy/` to fail with a build error, breaking SC9's "fuzz seeds green" criterion.

**Pre-existing status confirmed:** Running `go test -tags fuzz ./internal/policy/` on the commit immediately before Phase 9 (`e2a821a~1`) produces the identical error, proving this was not introduced by Phase 9. Phase 9 plans (09-01 through 09-05) do not list `fuzz_test.go` in any `files_modified` set, and `git diff e2a821a^..HEAD -- internal/policy/fuzz_test.go` is empty.

**Fix is trivial:** Add `policy.AgentContext{}` as the 4th argument on line 36 of `internal/policy/fuzz_test.go`.

**All 8 Phase 9 requirements (CLEAN-01/02/03/04, HARDEN-01/02/03, DRIFT-01) are SATISFIED with code-level evidence and live test runs.**

---

*Verified: 2026-06-04T00:00:00Z*
*Verifier: Claude (gsd-verifier)*
