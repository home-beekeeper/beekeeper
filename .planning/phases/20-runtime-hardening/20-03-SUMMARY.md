---
phase: 20-runtime-hardening
plan: 03
subsystem: testing
tags: [sentry, correlation-rules, watchlist, exfil-detection, purity-test]

requires:
  - phase: 08-tui-dashboard
    provides: Sentry correlation engine (EvaluateEvent, SENTRY-001..005, RuleState, BaselineState)
provides:
  - expanded defaultSensitivePaths + daemonSensitivePaths (cloud harvesters + .claude/)
  - agentExes + isAgentDescendant + unified isMonitoredDescendant (editor OR agent)
  - SENTRY-006 (agent credential-access cluster, no double-fire with 001 on integrated terminals)
  - isExternalDest (precomputed private nets, IPv4-mapped-IPv6 normalized) + SENTRY-003 external gate
  - SENTRY-007 (generalized exfil fusion: monitored-descendant + recent cred-read/persistence-write + external outbound)
  - RuleState.PersistWriteByPID (the SENTRY-007 persistence-write extension point for 20-04)
  - TestRulesImportsArePure (engine purity gate)
affects: [20-04, 20-05]

tech-stack:
  added: []
  patterns:
    - "Unified ancestry gate (isDescendantOf helper + editor/agent variants) so rules cover both editor trojans and standalone agents"
    - "agent-not-also-editor gate (D-T3-gate) prevents double-firing the agent rule on integrated terminals"
    - "precomputed []*net.IPNet at package init for O(1) external-dest classification (keeps the rule layer pure)"

key-files:
  created:
    - internal/sentry/imports_test.go
  modified:
    - internal/sentry/types.go
    - internal/sentry/rules.go
    - internal/sentry/rules_test.go
    - internal/sentry/linux/daemon.go

key-decisions:
  - "SENTRY-006 reuses the CredAccessByPID window populated by SENTRY-001 (no second append) and reads it after 001 in the EventFileAccess dispatch — avoids a parallel window and read-ordering bugs."
  - "Bare-terminal agents may fire BOTH SENTRY-001 (now isMonitoredDescendant-gated) and SENTRY-006; the explicit no-double-fire requirement is only for integrated terminals (editor-descended), which fire exactly SENTRY-001. Matches the plan's D-T3-gate intent and the asserted tests."
  - "SENTRY-007 persistence-write input wired against a new RuleState.PersistWriteByPID field that stays empty until 20-04 populates it (clean extension point, no behavioral gap)."
  - "Sentry purity set is looser than the policy engine's: net+time ALLOWED (the engine correlates IPs and time windows); os/net-http/io/sync/context forbidden."

patterns-established:
  - "isExternalDest normalizes IPv4-mapped IPv6 via To4() before range tests so ::ffff:10.0.0.1 is correctly private"

requirements-completed: [SENT-01, SENT-02, SENT-03, SENT-04]

duration: ~50 min
completed: 2026-06-10
---

# Phase 20 Plan 03: Sentry Tier-3 W1 Rule Wins (SENT-01..04) Summary

**Expanded cloud-credential watchlist, a unified editor-OR-agent ancestry gate with agent-specific SENTRY-006, external-destination-gated SENTRY-003, generalized exfil-fusion SENTRY-007, and a purity test locking the correlation engine I/O-free.**

## Performance

- **Duration:** ~50 min
- **Tasks:** 3
- **Files modified:** 4 modified + 1 created

## Accomplishments
- Watchlist expanded with `.config/gcloud`/`.azure`/`.kube/config`/`.docker/config.json`/`.claude/` in both the canonical `defaultSensitivePaths` and the `daemonSensitivePaths` fanotify mirror; a 2-cloud-cred read now trips SENTRY-001.
- `agentExes` + `isAgentDescendant` + unified `isMonitoredDescendant` (via a shared `isDescendantOf` helper); the four SENTRY-001/002/003/005 guards now fire for standalone-terminal/CI/SSH agents, not just editor extensions.
- `SENTRY-006` (agent credential-access cluster) fires for agent-descended-but-not-editor processes with a self-config-read allowlist, and does not double-fire with SENTRY-001 on integrated terminals (proven both directions in tests).
- `isExternalDest` (precomputed loopback/RFC1918/link-local/ULA/CGNAT nets, IPv4-mapped-IPv6 normalized) gates SENTRY-003 and powers `SENTRY-007` (generalized exfil fusion, warn-first in baseline); `TestRulesImportsArePure` forbids os/net-http/io/sync/context.

## Task Commits

1. **Tasks 1+2: watchlist + agent gate + SENTRY-006/007 + external-dest** - `0e6b5f1` (feat) — shared `rules.go`; SENTRY-007 (Task 2) builds on isMonitoredDescendant (Task 1) so they committed together.
2. **Task 3: TestRulesImportsArePure** - `3e5b3a2` (test)

## Decisions Made
See `key-decisions` frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Refactor] Extracted isDescendantOf shared helper**
- **Found during:** Task 1
- **Issue:** Adding isAgentDescendant as a near-copy of isEditorDescendant would duplicate the 32-hop/cycle-guard walk.
- **Fix:** Extracted `isDescendantOf(pid, tree, exes)`; both editor/agent variants delegate to it.
- **Verification:** Existing SENTRY-001..005 tests still green.
- **Committed in:** `0e6b5f1`

---

**Total deviations:** 1 auto-fixed (1 refactor)
**Impact on plan:** DRY cleanup, no behavior change; all prior Sentry tests pass unchanged.

## Issues Encountered
None.

## Next Phase Readiness
- Wave 1 complete (20-01 CSYNC + 20-03 SENT W1). Wave 2 plans 20-02 (LlamaFirewall) and 20-04 (Tier 3 W2 file-write) can proceed.
- 20-04 will append `EventFileWrite` to the EventKind iota and populate `RuleState.PersistWriteByPID`, closing the SENTRY-007 persistence-write extension point left here.
- `go build ./...` + `GOOS=linux go build ./...` + `go test ./internal/sentry/...` + `go vet ./internal/sentry/...` all green.

---
*Phase: 20-runtime-hardening*
*Completed: 2026-06-10*
