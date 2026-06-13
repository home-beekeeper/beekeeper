---
phase: 08-package-manager-nudge-behavioral-test-suite
plan: "05"
subsystem: config-audit-platform
tags: [config, audit, platform, NudgeConfig, ValidateNudgeConfig, nudge_action, BEEKEEPER_HOME, NUDGE-06, NUDGE-08, BTEST-01, BTEST-03]
dependency_graph:
  requires: []
  provides:
    - "platform.StateDir/AuditDir BEEKEEPER_HOME override — hermetic live-binary E2E (Wave 0 A2)"
    - "audit.AuditRecord nudge fields: OriginalCommand, RewrittenCommand, ReasonCode, PMState, NudgeAction (§9 enum)"
    - "config.NudgeConfig + DefaultNudgeConfig (PRD §5.1 defaults)"
    - "config.ValidateNudgeConfig — EXPORTED fail-closed validator (consumed by cmd/beekeeper in Plan 08)"
  affects:
    - "internal/check, internal/gateway, internal/shim (Plan 06 — read cfg.Nudge, set audit nudge fields)"
    - "cmd/beekeeper (Plan 08 — calls config.ValidateNudgeConfig before Save; config set audit)"
    - "internal/check/e2e_test.go (Plan 07 — BEEKEEPER_HOME enables hermetic E2E)"
tech_stack:
  added: []
  patterns:
    - "Exported validator delegated from Load (config.Load -> ValidateNudgeConfig) so package main can reuse it"
    - "Fail-closed config bounds (mirror validateCorroborationThresholds / CORR-02): unknown mode/preferred/floor rejected at load"
    - "Closed §9 enum nudge_action separate from the existing allow|warn|block Decision field (Blocker-2 resolution)"
    - "TestNudgeRecordConformsToPRDSection9: full §9 schema field-set + closed-enum assertion (§10-14)"
    - "BEEKEEPER_HOME env override checked first in platform dir resolution for test isolation"
key_files:
  created: []
  modified:
    - internal/platform/dirs.go
    - internal/platform/dirs_test.go
    - internal/audit/types.go
    - internal/audit/types_test.go
    - internal/config/config.go
    - internal/config/config_test.go
decisions:
  - "BEEKEEPER_HOME redirects all platform dirs together (state+audit) — single env var for hermetic E2E (RESEARCH A2)"
  - "nudge_action is a new closed §9 enum field (advise|proceed|rewrite|block); existing Decision (allow|warn|block) left untouched — preserves §9 semantics without disturbing the live audit Decision contract"
  - "ValidateNudgeConfig is EXPORTED (not the lowercase form); Load delegates to it so cmd/beekeeper (package main, Plan 08) can fail-closed reject `config set nudge.*` before Save"
  - "DefaultNudgeConfig = PRD §5.1: enabled, mode soft, preferred pnpm, checkSocketScanner true, drift 168h, floors pnpm 11.0.0 / bun 1.3.0 / node 22.0.0 (Flag 5 floors)"
  - "Layered config: project .beekeeper.json nudge.enabled:false disables (NUDGE-08)"
metrics:
  duration: "~30 minutes (interrupted by session limit before SUMMARY; closed out by orchestrator)"
  completed: "2026-06-04T11:00:00Z"
  tasks_completed: 3
  tasks_total: 3
  files_created: 0
  files_modified: 6
---

# Phase 8 Plan 05: Config + Audit + Platform Foundation Summary

**One-liner:** Wave-1 cross-cutting foundation — `BEEKEEPER_HOME` hermetic-E2E override, audit `AuditRecord` nudge fields incl. the `nudge_action` §9 enum + §10-14 conformance test, and `config.NudgeConfig` with an EXPORTED fail-closed `ValidateNudgeConfig`.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | BEEKEEPER_HOME env override in platform dir resolution (Wave 0 A2) | a3a4ee7 | internal/platform/dirs.go, dirs_test.go |
| 2 | Audit nudge fields + NudgeAction §9 enum + §10-14 conformance test | 548a3ec | internal/audit/types.go, types_test.go |
| 3 | config.NudgeConfig + Load defaulting + EXPORTED ValidateNudgeConfig | 0f1e820 | internal/config/config.go, config_test.go |

## Verification Results

- `go build ./...` — PASS (clean)
- `go test ./internal/audit/... ./internal/config/... ./internal/platform/...` — PASS (all three packages green)
- `ValidateNudgeConfig` confirmed EXPORTED at `internal/config/config.go:231`
- `TestNudgeRecordConformsToPRDSection9` (§10-14, BTEST-01) present at `internal/audit/types_test.go:81` and passing

## Success Criteria Status

- [x] `BEEKEEPER_HOME` override present in `platform` dir resolution (Wave 0 A2 — unblocks Plan 07 hermetic E2E)
- [x] Audit record carries `original_command`, `rewritten_command`, `reason_code`, `pm_state`, `nudge_action` (closed §9 enum), with `record_type:"nudge"` / `version_drift` support
- [x] `config.NudgeConfig` + `DefaultNudgeConfig` (PRD §5.1 defaults, Flag 5 floors)
- [x] `ValidateNudgeConfig` EXPORTED + fail-closed; `config.Load` delegates to it
- [x] §10-14 full-§9-schema conformance test green

## Requirements Covered

- NUDGE-06: audit `record_type:"nudge"` schema fields + `version_drift` record support
- NUDGE-08: layered `nudge` config block; project `.beekeeper.json nudge.enabled:false` disables
- BTEST-01: §10-14 audit-conformance table test
- BTEST-03: `BEEKEEPER_HOME` enables the hermetic live-binary E2E release gate (Plan 07)

## Deviations from Plan

The executor agent completed and committed all 3 tasks (a3a4ee7, 548a3ec, 0f1e820) but hit the Claude session limit before writing/committing this SUMMARY.md. Per the execute-phase safe-resume gate, the orchestrator closed the plan out manually: re-verified build + all three package test suites pass, confirmed the exported validator and §10-14 test on disk, then authored this SUMMARY. No code was re-executed (avoids duplicate-work risk). Plan content otherwise executed as written.

## Known Stubs

None. `PMState` audit field is a flattened JSON-string view per §9 — populated by the check/gateway adapters in Plan 06. `MajorDriftCheck` config (interval 168h) is consumed by Plan 06's gateway drift check (§10-15).

## Threat Surface Scan

No new network/auth/file-write surfaces. `ValidateNudgeConfig` is the fail-closed gate (T-08: malformed config rejected at load, not silently degraded). `BEEKEEPER_HOME` is a test-isolation override read from the environment — it redirects beekeeper's own state/audit dirs only; no privilege or path-traversal surface beyond the existing platform dir resolution.

## Self-Check

**Files modified (verified on disk):**
- `internal/platform/dirs.go` + `dirs_test.go` — FOUND (a3a4ee7)
- `internal/audit/types.go` + `types_test.go` — FOUND (548a3ec)
- `internal/config/config.go` + `config_test.go` — FOUND (0f1e820)

**Commits verified:**
- a3a4ee7 `feat(08-05): add BEEKEEPER_HOME env override to platform.StateDir (Wave 0 A2)`
- 548a3ec `feat(08-05): add nudge audit fields + NudgeAction §9 enum + §10-14 conformance test`
- 0f1e820 `feat(08-05): add config.NudgeConfig + Load defaulting + EXPORTED ValidateNudgeConfig`

## Self-Check: PASSED
