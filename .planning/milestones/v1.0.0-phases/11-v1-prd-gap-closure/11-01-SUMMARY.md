---
phase: 11
plan: 11-01
subsystem: gateway, config, catalog, policy, sentry
tags: [security, config-layering, ebpf, catalog-diff, signature-verification, dissent-tracking]
dependency_graph:
  requires: []
  provides: [llmf-gateway-scan-fix, layered-config-enforcement, ebpf-ci-pipeline, catalog-delta-scan, catalogs-diff-cmd, ed25519-catalog-verify]
  affects: [internal/gateway, cmd/beekeeper, internal/catalog, internal/policy, Makefile, .goreleaser.yaml, .github/workflows/ci.yml]
tech_stack:
  added: []
  patterns: [injectable-package-var-test-seam, dissent-sentinel-pattern, ed25519-canonical-payload-verify]
key_files:
  created:
    - cmd/beekeeper/config_resolve.go
    - cmd/beekeeper/main_test.go
    - internal/catalog/diff.go
    - internal/catalog/diff_test.go
  modified:
    - internal/gateway/proxy.go
    - internal/gateway/proxy_test.go
    - cmd/beekeeper/main.go
    - cmd/beekeeper/diag.go
    - internal/sentry/linux/gen.go (unchanged — pipeline wired around it)
    - Makefile
    - .github/workflows/ci.yml
    - .goreleaser.yaml
    - internal/catalog/verify.go
    - internal/catalog/verify_test.go
    - internal/catalog/multi.go
    - internal/catalog/multi_test.go
    - internal/policy/corroboration.go
    - internal/policy/corroboration_test.go
    - CLAUDE.md
decisions:
  - "VerifySignatureWithKey(entry, pubKey) added alongside VerifySignature — presence-only path unchanged for backward compat"
  - "Dissent sentinels (CatalogMatch{Dissented:true}) emitted by MultiIndex.LookupAll for configured-but-no-match sources; corroborate() filters them into SourcesDissented"
  - "scanOnDeltaFn injectable var follows runBumblebeeFn pattern for test-time mock without real scan binary"
  - "GoReleaser before.hooks uses sh -c guard so non-Linux environments skip eBPF generate gracefully"
  - "-buildvcs=false added to goreleaser build flags (reproducibility gap closure)"
metrics:
  duration: 27 minutes
  completed: 2026-06-01
  tasks_completed: 6
  files_changed: 15
---

# Phase 11 Plan 01: v1.0.0 PRD-Gap Closure Summary

Closed 6 PRD-audit gaps (3 serious + 3 moderate) to ensure beekeeper-prd.md is honestly met before the v1.0.0 tag is pushed. No new features — corrections and wiring only.

## One-liner

Six PRD audit gaps closed: gateway PromptGuard now runs for eligible tools, all enforcement commands use layered config, eBPF build pipeline wired into CI+goreleaser, catalog deltas trigger real re-scans, `catalogs diff` command added, and real Ed25519 catalog signature verification with SourcesDissented forensic provenance.

## Tasks Completed

| # | Task | Commit | Status |
|---|------|--------|--------|
| 01 | SERIOUS: gateway PromptGuard scan with real tool name (LLMF-02) | 3b79c90 | Done |
| 02 | SERIOUS: enforcement commands through layered config (CODE-05 SC2) | 1f3682b | Done |
| 03 | SERIOUS: eBPF bytecode generation into build pipeline (SLNX-02/04) | 86686d5 | Done |
| 04 | MODERATE: catalog-delta-triggered scan (CTLG-06) | c42c681 | Done |
| 05 | MODERATE: `beekeeper catalogs diff` (PRD §10) | 0b7f64f | Done |
| 06 | MODERATE: real Ed25519 catalog signature verification + dissent (CTLG-07/09) | deb8783 | Done |

## Task 01 — Gateway PromptGuard real tool name (LLMF-02)

**Root cause:** `forwardWithWarningInjection` and `forwardAllowWithScan` both called `ScanProxiedResponse(ctx, "", ...)` with an empty string. `ShouldScanPrompt("")` returns `false`, so PromptGuard 2 never ran on the gateway path for any tool.

**Fix:** Extract `toolName` from the `policy.ToolCall` already parsed in `handleToolCall` and thread it into both call sites.

**Tests added (proxy_test.go):**
- `TestGatewayScannerInvokedForEligibleToolOnWarnPath` — scanner called for `read_file` on warn path
- `TestGatewayScannerInvokedForEligibleToolOnAllowPath` — scanner called for `web_search` on allow path
- `TestGatewayScannerSkippedForNonEligibleTool` — `Bash` skips scanner (no behavior change)
- `TestScanProxiedResponseRealToolNamePassedNotEmpty` — unit regression guard on empty vs real toolName

## Task 02 — Layered config for enforcement commands (CODE-05 SC2)

**Root cause:** `resolveConfig`/`LoadLayered` was only called by `diag`. `newCheckCmd`, `newGatewayCmd`, `newWatchCmd`, `newScanCmd` called single-file `config.Load(configPath)` — project config overrides had no effect.

**Fix:** Moved `resolveConfig`, `systemConfigPath`, `discoverProjectConfig` from `diag.go` into shared `config_resolve.go`. Added `resolveConfigWithPaths` test variant. All 4 enforcement commands now call `resolveConfig(cmd)`.

**Tests added (main_test.go):**
- `TestLayeredConfigProjectOverridesUser` — project `fail_mode=closed` overrides user `fail_mode=open` without env vars
- `TestLayeredConfigUserAppliedWhenNoProject` — user config applies when no project config
- `TestLayeredConfigCorruptUserFails` — corrupt user config returns error (fail-closed)
- `TestLayeredConfigEnvOverridesProject` — `BEEKEEPER_FAIL_MODE` env overrides project config
- `TestDiscoverProjectConfig` — project config discovery walks directory tree

## Task 03 — eBPF build pipeline wiring (SLNX-02/04)

**Constraint enforced:** Do NOT generate/run eBPF bytecode locally on Windows. Only wired into the build pipeline.

**Changes:**
- `Makefile`: Added `generate` target (Linux-only guard via `uname -s` check)
- `.github/workflows/ci.yml`: Install clang/llvm/libbpf + run `go generate ./internal/sentry/linux/...` before build on Linux only
- `.goreleaser.yaml`: Added `before.hooks` entry (sh -c guard for non-Linux), added `-buildvcs=false` to flags
- `CLAUDE.md`: eBPF section updated: CI-generated at build time, never committed, never runtime-compiled

**Loader behavior unchanged:** stubs still return `errors.New("bpf2go: run go generate...")` when bytecode absent (fail-closed).

## Task 04 — Catalog delta triggers scan (CTLG-06)

**Root cause:** `catalogs watch` `onDelta` callback only logged. No `catalog.Sync` or `scan.Scan` call.

**Fix:** Added `scanOnDeltaFn` injectable var (production = `scan.Scan`). On real delta or alert, callback calls `catalog.Sync` to refresh the index then `scanOnDeltaFn` to scan extensions. Hard-block sanity breach returns early (poisoned catalog — do not scan).

**Tests added (main_test.go):**
- `TestCatalogWatchDeltaTriggersScan` — simulated delta with `HasChanges()=true` invokes `scanOnDeltaFn`
- `TestCatalogWatchDeltaNoScanOnHardBlock` — hard-block sanity breach does NOT invoke scan

## Task 05 — `beekeeper catalogs diff` (PRD §10)

**New files:**
- `internal/catalog/diff.go`: `Diff(ctx, stateFile, catalogDir, client) ([]DiffResult, error)` — computes per-source delta between persisted state and current on-disk snapshot; `DiffResult` has `Added/Removed/Changed/Degraded/PrevHash/CurrentHash`
- `internal/catalog/diff_test.go`: 5 tests covering empty state, persisted→current, count math, degraded source surfacing, `HasChanges()` method

**Command:** `beekeeper catalogs diff` registered in `newCatalogsCmd`. Read-only, no enforcement side effects. Prints status+delta+hash per source.

## Task 06 — Real Ed25519 signature verification + SourcesDissented (CTLG-07/09)

**Changes:**

1. **`verify.go`**: Added `VerifySignatureWithKey(entry, pubKey ed25519.PublicKey) bool`:
   - Real `ed25519.Verify` over canonical entry payload (excluding the signature field itself)
   - Unsigned entry (empty sig) → `false`, not an error (warn-only preserved)
   - Nil key → falls back to presence-only (backward compat)
   - Invalid base64 → `false` (not a panic)

2. **`multi.go`**: When a configured (non-nil) source finds no match, appends `CatalogMatch{CatalogSource: "...", Dissented: true}` dissent sentinel for CTLG-09 forensic provenance.

3. **`corroboration.go`**: Filters dissent sentinels from real matches; populates `SourcesDissented` output. Only `fmt` + `sort` imports — purity preserved.

**Tests:**
- `verify_test.go`: valid sig→true, tampered entry→false, invalid base64→false, unsigned stays false, nil key fallback
- `multi_test.go`: `TestMultiIndexMissReturnsDiffentSentinel` — miss returns dissent sentinel
- `corroboration_test.go`: `TestCorroborationSourcesDissented` — OSV dissent appears in `SourcesDissented`; `TestCorroborationDissentDoesNotAffectDecision` — dissent is forensic only (doesn't change level)

## Deviations from Plan

### Auto-fixes (Rule 3)

None required — all tasks executed as specified.

### Design notes

**Task 04 (CTLG-06):** Added `scanOnDeltaFn` injectable package var rather than modifying the signature of `catalog.Watch`. This follows the existing `runBumblebeeFn` pattern in `scan/scanner.go` and avoids making `catalog.Watch` depend on the `scan` package (which would create an import cycle since `scan` imports `catalog`).

**Task 06 (dissent tracking):** Implemented dissent as sentinels in `MultiIndex.LookupAll` rather than a separate "queried sources" slice. This keeps the `policy.MultiCatalogLookup` interface unchanged and lets `corroborate()` receive all information it needs from the single `matches` slice — preserving the pure-function contract.

## Known Stubs

None. All acceptance criteria are substantively met.

## Threat Flags

None. No new network endpoints, auth paths, or trust boundaries introduced.

## Self-Check: PASSED

All 6 task commits exist, all tests pass (22/22 packages green), both native and Windows builds clean.
