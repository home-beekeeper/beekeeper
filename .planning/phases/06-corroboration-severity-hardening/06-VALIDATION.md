---
phase: 6
slug: corroboration-severity-hardening
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-03
reconciled: 2026-06-04
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Derived from `06-RESEARCH.md` § Validation Architecture.

---

## Reconciliation Note

This frontmatter was drafted pre-execution and was never flipped from its initial `draft` / `nyquist_compliant: false` / `wave_0_complete: false` state. It is now reconciled to **COMPLIANT** (2026-06-04) consistent with `06-VERIFICATION.md`:

- **Verification status:** `passed` — verified 2026-06-03T00:00:00Z
- **Score:** 5/5 must-haves verified (all five Observable Truths in `06-VERIFICATION.md` confirmed with evidence)
- **All Wave-0 tests delivered and passing:** `TestCorroborationShaiHuludCriticalBlock`, `TestCorroborationDegradedCatalogNoEscalation`, `TestCorroborationAllVersionsCriticalWildcardStaysWarn`, `TestValidateCorroborationThresholdsRejectsBlockAtZero`, `TestValidateCorroborationThresholdsRejectsLooserOverride`, `TestDefaultThresholdsIncludeSeverityOverrides`, `TestThresholdsFromPolicyFilesCriticalBlockAt`, `TestRunCheckAiFigureBlocks`, `TestRunCheckCriticalDegradedCatalogWarn`, `TestRunCheckCriticalBlockWithHealthyCatalog`
- **Milestone audit finding:** `v1.2.0-MILESTONE-AUDIT.md` noted this as a stale-frontmatter artifact ("drafted pre-execution, never flipped to compliant though the phase verified passed") — explicitly flagged as a discovery, not a coverage gap.
- **Requirement:** CLEAN-03 (v1.2.0 tech-debt cleanup, plan 09-05)

No validation content, test map, or source/test files were altered — only the frontmatter fields and this note.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (`testing` package) — existing, no config needed |
| **Config file** | None (Go test files co-located with source) |
| **Quick run command** | `go test ./internal/policy/... ./internal/policyloader/... -run TestCorroboration -v` |
| **Full suite command** | `go test ./internal/policy/... ./internal/policyloader/... ./internal/check/... ./internal/gateway/... ./internal/watch/... ./internal/scan/... -race` |
| **Estimated runtime** | ~20–40 s (full); <5 s (quick). `-race` requires CGO — CI-only on Windows dev box (see STATE Deferred Items). |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/policy/... -run TestCorroboration -v`
- **After every plan wave:** Run `go test ./internal/policy/... ./internal/policyloader/...`
- **Before `/gsd-verify-work 6`:** Full suite must be green (`-race` in CI)
- **Max feedback latency:** ~40 seconds

---

## Requirement → Test Map

| Req / SC | Behavior | Test Type | Automated Command | File |
|----------|----------|-----------|-------------------|------|
| CORR-01 | Critical single-signed-source → block (Shai-Hulud `ai-figure` fixture) | unit | `go test ./internal/policy/... -run TestCorroborationShaiHuludCriticalBlock` | ❌ W0 |
| CORR-01 | `DefaultCorroborationThresholds()` includes `SeverityOverrides["critical"]` | unit | `go test ./internal/policy/... -run TestDefaultThresholdsIncludeSeverityOverrides` | ❌ W0 |
| CORR-01 | Non-critical single-source still warns (no regression) | unit | `go test ./internal/policy/... -run TestCorroborationOneSignedSource` | ✅ existing |
| CORR-01 | `critical_block_at` in policy file lowers effective threshold | unit | `go test ./internal/policyloader/... -run TestThresholdsFromPolicyFilesCriticalBlockAt` | ❌ W0 |
| CORR-02 | Degraded catalog (`CatalogHealthy=false`) → escalation suppressed | unit | `go test ./internal/policy/... -run TestCorroborationDegradedCatalogNoEscalation` | ❌ W0 |
| CORR-02 | `validateCorroborationThresholds` rejects `BlockAt < 1` | unit | `go test ./internal/policy/... -run TestValidateCorroborationThresholdsRejectsBlockAtZero` | ❌ W0 |
| CORR-02 | `validateCorroborationThresholds` rejects override `BlockAt > global BlockAt` | unit | `go test ./internal/policy/... -run TestValidateCorroborationThresholdsRejectsLooserOverride` | ❌ W0 |
| CORR-02 | All-versions wildcard critical entry → warn at single source | unit | `go test ./internal/policy/... -run TestCorroborationAllVersionsCriticalWildcardStaysWarn` | ❌ W0 |
| CORR-02 | `CatalogHealthy` threaded at handler.go call site | integration | `go test ./internal/check/... -run TestRunCheckCriticalBlockWithHealthyCatalog` | ❌ W0 |
| SC1 | `beekeeper check` with `ai-figure` → exit 1, `decision:"block"` | integration | `go test ./internal/check/... -run TestRunCheckAiFigureBlocks` | ❌ W0 |
| SC2 | Degraded catalog (1001 entries) → `ai-figure` still warns | integration | `go test ./internal/check/... -run TestRunCheckCriticalDegradedCatalogWarn` | ❌ W0 |
| SC5 | Table-driven tests in `internal/policy/` | unit | `go test ./internal/policy/... -race` | ❌ W0 |
| Purity | `corroboration.go` imports no I/O packages | static | `go test ./internal/policy/... -run TestCorroborationImportsArePure` | ✅ existing |

---

## Wave 0 Requirements

New test functions to stub before/with implementation (no new framework — `go test` already configured):

- [ ] `internal/policy/corroboration_test.go` — `TestCorroborationShaiHuludCriticalBlock`, `TestCorroborationDegradedCatalogNoEscalation`, `TestCorroborationAllVersionsCriticalWildcardStaysWarn`, `TestValidateCorroborationThresholdsRejectsBlockAtZero`, `TestValidateCorroborationThresholdsRejectsLooserOverride`, `TestDefaultThresholdsIncludeSeverityOverrides`
- [ ] `internal/policyloader/test_test.go` — `TestThresholdsFromPolicyFilesCriticalBlockAt`
- [ ] `internal/check/handler_test.go` — `TestRunCheckAiFigureBlocks`, `TestRunCheckCriticalDegradedCatalogWarn`, `TestRunCheckCriticalBlockWithHealthyCatalog`
- [x] `catalog.LoadState` signature verified — `SourceState.Degraded bool`; `resolveCatalogHealthy` uses `filepath.Dir(cacheDir)+"/state.json"`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live `beekeeper check npm install ai-figure` → exit 1 | CORR-01 / SC1 | Requires the real `%APPDATA%\beekeeper` catalog (live OSV + bundled bumblebee) | `printf '{"tool_name":"Bash","tool_input":{"command":"npm install ai-figure"}}' \| beekeeper check; echo $?` → expect `1`. (Codified in the Phase-8 live-binary E2E battery, BTEST-03.) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 40s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-03 (reconciled 2026-06-04 via CLEAN-03)
