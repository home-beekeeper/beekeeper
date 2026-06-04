---
phase: 8
slug: package-manager-nudge-behavioral-test-suite
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-04
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `08-RESEARCH.md` → "## Validation Architecture". Per-task rows are filled
> by the planner/executor once PLAN.md task IDs exist; infrastructure + requirement map are locked here.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (+ `testing/fuzz`) — go1.25 |
| **Config file** | none (Go convention); fuzz behind `//go:build fuzz`, E2E behind `//go:build e2e` |
| **Quick run command** | `go test ./internal/nudge/... ./internal/pkgparse/...` |
| **Full suite command** | `go test ./...` |
| **Fuzz gate command** | `go test -tags fuzz -run=Fuzz ./internal/nudge/... ./internal/pkgparse/... ./internal/gateway/...` |
| **E2E gate command** | `go test -tags e2e -run=TestE2ELiveBinary ./internal/check/...` |
| **Estimated runtime** | ~30s unit/integration; fuzz seed-corpus + E2E add ~30s |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/nudge/... ./internal/pkgparse/...`
- **After every plan wave:** Run `go test ./...` (full unit + integration)
- **Before `/gsd-verify-work`:** Full suite green **AND** `go test -tags fuzz -run Fuzz ./...` (seed corpus) **AND** `go test -tags e2e -run=TestE2ELiveBinary ./internal/check/...` — ALL must pass before any v1.2.0 tag is cut (SC4 release gate).
- **Max feedback latency:** ~30 seconds (quick run)

---

## Requirement → Test Map (locked from RESEARCH; task IDs assigned during planning)

| Req / SC | Behavior | Test Type | Automated Command | File |
|----------|----------|-----------|-------------------|------|
| NUDGE-01 / SC1 | pnpm/bun/yarn installs parsed + catalog-matched (closes F3) | unit + integration | `go test ./internal/pkgparse/... ./internal/policy/...` | pkgparse_test.go, engine_test.go (EDIT), integration_test.go |
| NUDGE-02 | timeout-bounded detection → PMState; `Evaluate` pure | unit | `go test ./internal/nudge/...` | detect_test.go, evaluate_test.go, `TestNudgeEvaluateImportsArePure` |
| NUDGE-03 / §10-1 | soft Advise + proceed (exit 0); ≤1 advisory/session | table-driven + gateway | `go test ./internal/nudge/...` | evaluate_test.go |
| NUDGE-04 / §10-2,4 | hard Rewrite; `requireHardened` Block | table-driven | `go test ./internal/nudge/...` | evaluate_test.go |
| NUDGE-05 | unpinned (@latest/bare/wide range) flagged | unit | `go test ./internal/pkgparse/...` | pkgparse_test.go |
| NUDGE-06 / §10-14,15 | `record_type:"nudge"` schema; `version_drift` record | unit + audit-shape | `go test ./internal/nudge/... ./internal/audit/...` | evaluate_test.go, version_test.go, types_test.go (EDIT) |
| NUDGE-07 / SC5 | `nudge status\|check\|audit` CLI | CLI unit | `go test ./cmd/beekeeper/...` | nudge_test.go (NEW) |
| NUDGE-08 | wired into check+gateway+shim; layered config; cache gateway-only | integration | `go test ./internal/check/... ./internal/gateway/... ./internal/shim/...` | integration_test.go, gateway_test.go, shim_test.go |
| §10-5 | bun-available-no-scanner reason | table-driven | `go test ./internal/nudge/...` | evaluate_test.go |
| §10-6 | node-incompatible-with-pnpm-11 | table-driven | `go test ./internal/nudge/...` | evaluate_test.go |
| §10-7 | npm ls/run/publish NOT nudged | table-driven | `go test ./internal/pkgparse/... ./internal/nudge/...` | pkgparse_test.go, evaluate_test.go |
| §10-8 | no-arg `npm install` softer reason | table-driven | `go test ./internal/nudge/...` | evaluate_test.go |
| §10-9 | `npx` parsed as install+execute | table-driven | `go test ./internal/pkgparse/...` | pkgparse_test.go |
| §10-10 | sudo parsed, NOT rewritten | table-driven | `go test ./internal/nudge/...` | evaluate_test.go |
| §10-11 | 60s cache (gateway session) | gateway unit (injected clock) | `go test ./internal/nudge/... ./internal/gateway/...` | detect_test.go (Cache), gateway_test.go |
| §10-12 | 2s timeout → graceful fallback (not installed) | unit (injected slow fn) | `go test ./internal/nudge/...` | detect_test.go |
| §10-13 | bunfig.toml parse failure → BunScannerOK=false, no crash | unit + fuzz | `go test ./internal/nudge/...` + fuzz | scanners_test.go, scanners_fuzz_test.go |
| §10-16 | minimumReleaseAge=0 → warn, pnpm_hardened stays true | unit | `go test ./internal/nudge/...` | scanners_test.go / version_test.go |
| §10-17 | config change logged to audit | CLI + audit | `go test ./cmd/beekeeper/...` | nudge_test.go / config audit hook |
| BTEST-01 / SC3 | table-driven pure tests cover §10 1-10,14-17 | table-driven | `go test ./internal/nudge/... ./internal/policy/...` | evaluate_test.go (+ existing path/corroboration tests) |
| BTEST-02 | RunCheck integration: credential read, critical block, pnpm/bun install | integration | `go test -run TestIntegration ./internal/check/...` | integration_test.go (EDIT) |
| BTEST-03 / SC4 | live-binary E2E: SPATH+CORR+NUDGE exit codes + audit records | E2E (release gate) | `go test -tags e2e ./internal/check/...` | e2e_test.go (NEW) |
| BTEST-03 (fuzz) | bunfig.toml + pnpm-workspace.yaml + pkgparse fuzz never panic | fuzz (release gate) | `go test -tags fuzz -run Fuzz ./...` | scanners_fuzz_test.go, pkgparse/fuzz_test.go |

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| _assigned during planning_ | — | — | — | — | — | — | — | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/pkgparse/` package + tests + fuzz — does not exist (Flag 4 extraction is a Wave 0/1 prerequisite)
- [ ] `internal/nudge/` package (detect.go, evaluate.go, version.go, scanners.go + tests) — does not exist
- [ ] `internal/check/e2e_test.go` — no live-binary E2E precedent in repo; net-new harness (gateway fuzz test is the closest template)
- [ ] `internal/nudge/scanners_fuzz_test.go` + `internal/pkgparse/fuzz_test.go` — new fuzz targets
- [ ] `cmd/beekeeper/nudge.go` + `nudge_test.go` — new CLI command (`policy.go` / `policy_test.go` are the template)
- [ ] `audit.AuditRecord` nudge fields (`original_command`, `rewritten_command`, `reason_code`, `pm_state`) — `types.go` EDIT
- [ ] **VERIFY** the compiled binary honors an overridable state/audit dir (env or flag) so the BTEST-03 live-binary E2E is hermetic — check `newCheckCmd` / `platform` before planning the E2E task (RESEARCH Assumption A2)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows corepack-shimmed pnpm `cmd.exe` startup vs 2s detection timeout | NUDGE-02 | Live timing is environment-specific; CI-only on `windows-latest` | Run detection on a Windows runner with corepack-shimmed pnpm; assert detection completes < 2s OR falls back gracefully (never blocks) |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
