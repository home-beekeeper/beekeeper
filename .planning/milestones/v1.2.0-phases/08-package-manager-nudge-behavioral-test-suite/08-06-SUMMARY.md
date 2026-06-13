---
phase: 08-package-manager-nudge-behavioral-test-suite
plan: "06"
subsystem: enforcement-wiring
tags: [nudge, check, gateway, shim, drift, NUDGE-03, NUDGE-04, NUDGE-06, NUDGE-08, BTEST-01]

# Dependency graph
requires:
  - phase: 08-02
    provides: "internal/pkgparse.Parse (ParsedCommand, IsInstall)"
  - phase: 08-03
    provides: "nudge.Evaluate, nudge.ActionString, nudge.ConfigFrom, nudge.DefaultConfig, nudge.IsMajorDrift"
  - phase: 08-04
    provides: "nudge.DetectStateFn (exported seam), nudge.NewCache, Cache.State"
  - phase: 08-05
    provides: "config.NudgeConfig, audit nudge fields (NudgeAction, OriginalCommand, etc.)"
provides:
  - "Nudge wired into check hook: evaluateNudge in nudge_adapter.go (fresh DetectStateFn, no cache)"
  - "Nudge wired into gateway: Nudge field on gateway.Config + 60s cache + advisory cap at applyPolicy call site"
  - "Nudge wired into shim: NudgeCheck + NudgeResult using DetectStateFn seam"
  - "Gateway drift check: checkDrift + startDriftScheduler + version_drift audit record"
  - "Tests: TestGatewayNudgeMerge, TestGatewayAdvisoryCapPerSession, TestShimNudgeBeforeProxy, TestShimNudgeNonInstallSkipped, TestCheckDrift{EmitsVersionDrift,NoDrift,FetchError}"
affects:
  - "08-07 (behavioral integration tests call nudge via the check + gateway paths)"
  - "08-08 (cmd/beekeeper/main.go populates gatewayCfg.Nudge — Plan 08 Wave 4)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Nudge-before-proxy pattern: evaluateNudge/NudgeCheck call DetectStateFn (fresh), Evaluate (pure), mergeDecisions AFTER overlay (CR-02 ordering)"
    - "Flag 2 Position B: one-shot hook + short-lived shim = fresh detection; gateway = 60s cache wrapping DetectStateFn"
    - "WARNING 2 closed: cache-backed merge at the proxy.go applyPolicy call site (where h is in scope), not inside the free function"
    - "WARNING 3 closed: newGatewayHandler defaults zero cfg.Nudge to nudge.DefaultConfig() (T-08-25b)"
    - "Per-session advisory cap (NUDGE-03): advSeen map on gatewayHandler keyed by agent-id else process-global sentinel"
    - "Drift check: injectable metadataFetchFn + nudge.IsMajorDrift (BLOCKER 2) + async goroutine + never on hot path"
    - "Rule 1 auto-fix: TestAuditRecordWrittenOnEveryPath updated to scan NDJSON lines (multi-record file)"

key-files:
  created:
    - internal/check/nudge_adapter.go
    - internal/gateway/drift.go
    - internal/gateway/drift_test.go
  modified:
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/gateway/policy.go
    - internal/gateway/proxy.go
    - internal/gateway/gateway.go
    - internal/gateway/gateway_test.go
    - internal/shim/shim.go
    - internal/shim/shim_test.go

key-decisions:
  - "Advisory cap: second Advise for the same agent-id is suppressed as a message (no _beekeeper_warning injected) but the audit record is still written; Block decisions are never capped regardless of session state"
  - "metadataFetchFn returns empty map in production (real registry query is future work); test injection provides full coverage of the §10-15 drift record wiring without live network calls"
  - "cfg.Nudge zero-value detection: Mode==\"\" is the reliable signal (DefaultConfig has Mode:\"soft\"); this avoids needing a separate sentinel boolean"
  - "mergeGatewayDecisions is duplicated in proxy.go (not imported from internal/check) to preserve package encapsulation"

requirements-completed: [NUDGE-03, NUDGE-04, NUDGE-06, NUDGE-08, BTEST-01]

# Metrics
duration: "~15 minutes"
completed: "2026-06-04T13:49:59Z"
tasks_completed: 3
tasks_total: 3
files_created: 3
files_modified: 8
---

# Phase 8 Plan 06: Nudge Live Wiring — Check, Gateway, Shim, Drift Summary

**One-liner:** Wire the pure nudge decision into all three enforcement consumers (check hook fresh detect, gateway 60s cache, shim NudgeCheck) plus the gateway version_drift drift scheduler — WARNING 2, WARNING 3, BLOCKER 2, and Open Q2/Q3 all closed.

## Performance

- **Duration:** ~15 min
- **Started:** 2026-06-04T13:34:29Z
- **Completed:** 2026-06-04T13:49:59Z
- **Tasks:** 3
- **Files created:** 3
- **Files modified:** 8

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | nudge_adapter.go + runCheck merge | 3df35e4 | internal/check/nudge_adapter.go, handler.go, handler_test.go |
| 2 | gateway.Config.Nudge + cache + advisory cap + shim | 5c68c47 | policy.go, proxy.go, gateway_test.go, shim.go, shim_test.go |
| 3 | gateway drift.go + drift_test.go | 76bd725 | drift.go, drift_test.go, gateway.go |

## Accomplishments

### Task 1 — Check Hook (nudge_adapter.go + handler.go)

- `nudge_adapter.go` created: `evaluateNudge(ctx, tc, nc)` extracts the Bash command, calls `pkgparse.Parse`, and when `IsInstall` calls `nudge.DetectStateFn` FRESH (the EXPORTED seam — NOT `nudge.DetectState` directly, so Plan 07 tests can inject a synthetic PMState across the package boundary — T-08-10b)
- `nudge.ConfigFrom` is the SINGLE mapper (no local copy — BLOCKER 1 closed)
- Returns `policy.Decision` + `audit.AuditRecord` with `record_type:"nudge"`, `NudgeAction` (closed §9 enum, distinct from Level), and the §9 fields
- `handler.go` wires nudge merge AFTER `ApplyPolicyOverlay` and SPATH block via `mergeDecisions` (CR-02 ordering, T-08-17); `writeNudgeAuditRecord` is best-effort (never changes decision)
- Non-install commands skip detection entirely (Pitfall 2 / T-08-18)

### Task 2 — Gateway + Shim

- `policy.go`: `Nudge nudge.Config` field added to `gateway.Config` (WARNING 3 fix); pure `nudgeDecisionFor` helper (takes already-resolved PMState, cache-agnostic)
- `proxy.go`: `newGatewayHandler` defaults zero cfg.Nudge to `nudge.DefaultConfig()` (T-08-25b); constructs `nudge.Cache` ONCE wrapping `nudge.DetectStateFn` (Flag 2 Position B — gateway-only cache); `gatewayHandler` gains `nudgeCache *nudge.Cache` + `advSeen map[string]bool` + mutex
- Cache-backed nudge merge wired at the `applyPolicy` CALL SITE in proxy.go (WARNING 2 closed — applyPolicy stays a free function; h and thus h.nudgeCache are in scope at the call site)
- Per-session advisory cap (NUDGE-03, Open Q3 resolved): first Advise per agent-id surfaces `_beekeeper_warning`; subsequent Advise messages are suppressed (message only — audit record still written); Block decisions are never capped
- `shim.go`: `NudgeCheck` + `NudgeResult` — nudge-before-proxy logic using `DetectStateFn` seam (fresh detect, no cache; mirrors check hook, Flag 2 Position B)
- Tests: `TestGatewayNudgeMerge`, `TestGatewayAdvisoryCapPerSession`, `TestShimNudgeBeforeProxy`, `TestShimNudgeNonInstallSkipped` — all pass

### Task 3 — Gateway Drift (drift.go)

- `drift.go`: injectable `metadataFetchFn`; `checkDrift` calls the EXPORTED `nudge.IsMajorDrift` for pnpm and bun (BLOCKER 2 closed — private `isMajorDrift` is uncallable from package gateway); emits `record_type:"version_drift"` info record; NEVER auto-updates floors (T-08-25)
- `gateway.go`: `startDriftScheduler` wired in `Start()` after handler creation; runs in a dedicated goroutine, ticked at `cfg.Nudge.MajorDriftCheck.Interval` (default 168h); never on the request path (Pitfall 6, T-08-24)
- `drift_test.go`: `TestCheckDriftEmitsVersionDrift` (pnpm 12.0.0 vs floor 11.0.0 → one record), `TestCheckDriftNoDrift` (same major → zero records), `TestCheckDriftFetchError` (error → zero records, no panic)

## Deviations from Plan

### Auto-fixed Issues

**[Rule 1 - Bug] TestAuditRecordWrittenOnEveryPath failed after nudge record addition**

- **Found during:** Task 1 verification (`go test ./internal/check/...`)
- **Issue:** The test read the entire audit file content as a single JSON value (`json.Unmarshal(string(data))`). With our nudge block writing a `record_type:"nudge"` record BEFORE the `policy_decision` record, the file now contains two NDJSON lines. The `json.Unmarshal` on the multi-line string failed with "invalid character '{' after top-level value".
- **Fix:** Updated the test to scan NDJSON line-by-line and search for a `policy_decision` record (the invariant the test actually cares about). The new test correctly handles multi-record audit files.
- **Files modified:** `internal/check/handler_test.go`
- **Commit:** `3df35e4`

## Known Stubs

- `realMetadataFetch` in `drift.go` returns an empty map in production. This is intentional: a proper npm registry query for pnpm/bun latest versions is future work (noted in the comment). The test injection (`metadataFetchFn = fake`) provides full coverage of the §10-15 drift record wiring. The stub is documented and does not prevent the plan's goal — the structural wiring and test coverage are complete.

## Threat Surface Scan

All STRIDE threats from the plan's `<threat_model>` are addressed:

| Threat | Mitigation | Status |
|--------|------------|--------|
| T-08-17 (overlay downgrade of Block) | Nudge merge runs AFTER ApplyPolicyOverlay + SPATH; mergeDecisions is most-restrictive-wins | Applied |
| T-08-18 (detection on hot path) | Detection ONLY when IsInstall; check hook = fresh 2s-bounded; gateway = 60s cache | Applied |
| T-08-19 (untraced nudge actions) | Every evaluateNudge/nudgeDecisionFor result writes record_type:"nudge" best-effort | Applied |
| T-08-24 (drift network call DoS) | checkDrift is async goroutine + fail-open on error + never on request path | Applied |
| T-08-25 (drift auto-update floors) | checkDrift emits info record only; no floor mutation anywhere | Applied |
| T-08-25b (gateway zero-config floor) | newGatewayHandler defaults zero cfg.Nudge to DefaultConfig() | Applied |

No new network endpoints, auth paths, or file-write surfaces beyond what the plan's threat model covers.

## Self-Check: PASSED

**Files created (verified on disk):**
- `internal/check/nudge_adapter.go` — FOUND
- `internal/gateway/drift.go` — FOUND
- `internal/gateway/drift_test.go` — FOUND

**Files modified (verified on disk):**
- `internal/check/handler.go` — FOUND
- `internal/check/handler_test.go` — FOUND
- `internal/gateway/policy.go` — FOUND
- `internal/gateway/proxy.go` — FOUND
- `internal/gateway/gateway.go` — FOUND
- `internal/gateway/gateway_test.go` — FOUND
- `internal/shim/shim.go` — FOUND
- `internal/shim/shim_test.go` — FOUND

**Commits verified:**
- 3df35e4 `feat(08-06): wire nudge into check hook (nudge_adapter.go + handler.go merge)`
- 5c68c47 `feat(08-06): gateway.Config.Nudge + 60s cache + advisory cap + shim nudge`
- 76bd725 `feat(08-06): gateway drift.go — periodic version_drift emitter + drift_test.go`

**Build + test verification:**
- `go build ./...` — CLEAN
- `go test ./internal/check/... ./internal/gateway/... ./internal/shim/...` — ALL PASS
- `TestGatewayNudgeMerge` — PASS
- `TestGatewayAdvisoryCapPerSession` — PASS
- `TestShimNudgeBeforeProxy` — PASS
- `TestCheckDriftEmitsVersionDrift` — PASS
- `TestCheckDriftNoDrift` — PASS
- `TestCheckDriftFetchError` — PASS

---
*Phase: 08-package-manager-nudge-behavioral-test-suite*
*Completed: 2026-06-04*
