---
phase: "09"
plan: "09-06"
subsystem: policy-overlay, self-catalog, latency
tags: [gap-closure, policy-as-code, self-defense, latency-tracking]
dependency_graph:
  requires: [09-01-PLAN, 09-03-PLAN, 09-04-PLAN, 09-05-PLAN]
  provides: [CODE-01, CTLG-04, SFDF-06, CODE-06]
  affects: [internal/policyloader, internal/check, internal/catalog, cmd/beekeeper, internal/llamafirewall]
tech_stack:
  added: []
  patterns:
    - Pure overlay function (most-restrictive-wins with allowlist escape hatch)
    - Nearest-rank percentile formula (math.Ceil(p*n)-1 clamped)
    - Ed25519 PubKeyOverride with fail-closed decode guard
key_files:
  created:
    - internal/policyloader/enforce.go
    - internal/policyloader/enforce_test.go
  modified:
    - internal/policyloader/test.go
    - internal/check/handler.go
    - internal/check/handler_test.go
    - internal/catalog/selfcatalog.go
    - internal/catalog/selfcatalog_test.go
    - cmd/beekeeper/selfquarantine.go
    - cmd/beekeeper/selfquarantine_test.go
    - internal/llamafirewall/latency.go
    - internal/llamafirewall/latency_test.go
    - docs/THREAT-MODEL.md
decisions:
  - "Policy overlay (ApplyPolicyOverlay) lives in internal/policyloader/enforce.go — internal/policy is unchanged (zero diff)"
  - "Overlay combination: block-first, then allow-escape-hatch for allowlist, then most-restrictive-wins for warn, then base unchanged"
  - "release_age and lifecycle_script_allowlist not enforced by overlay v1 — documented in enforce.go and THREAT-MODEL.md §9"
  - "PubKeyOverride exported (was pubKeyOverride); misconfigured key fails closed with clear error (T-09-32)"
  - "Nearest-rank percentile: math.Ceil(p*n)-1 shared helper percentile() called by P95 and P99"
  - "THREAT-MODEL.md §9 added: allowlist override escape hatch + overlay limitations"
metrics:
  duration: "~45 min"
  completed_date: "2026-05-30"
  tasks_completed: 3
  files_changed: 11
---

# Phase 09 Plan 06: Gap Closure — Live Policy Enforcement + Self-Hosted Key + Percentile Fix Summary

Three gaps from Phase 9 verification and code review were closed in this plan.

## What Was Built

**Task 1 (CODE-01): Pure policy overlay enforcing declarative policy files against live check decisions**

`internal/policyloader/enforce.go` adds `ApplyPolicyOverlay(files []PolicyFile, tc policy.ToolCall, base policy.Decision) policy.Decision` — a pure function with no I/O that evaluates `package_allowlist` and `sensitive_path` rules from policy files and combines with the engine decision via most-restrictive-wins with an explicit allowlist allow escape hatch (T-09-31). A `LoadPolicyDir` helper loads all valid `*.json` files from a directory, skipping individually invalid files with a warning (T-09-33).

`internal/policyloader/test.go` was updated so `runPolicyTestWithCatalog` applies the overlay after `policy.Evaluate`, ensuring `beekeeper policy test` output matches live enforcement (CODE-01).

`internal/check/handler.go` loads `~/.beekeeper/policies/*.json` via `LoadPolicyDir` and applies `ApplyPolicyOverlay` to the engine decision before `finalizeWithAC`. A wholesale unreadable policies directory honors `fail_mode` (T-09-33). `internal/policy` was not touched — confirmed by zero diff on `internal/policy/*`.

**Task 2 (CR-01): Self-hosted beekeeper-self public key wired through PubKeyOverride**

`internal/catalog/selfcatalog.go` exports `pubKeyOverride` as `PubKeyOverride ed25519.PublicKey`. `CheckSelfCatalog` prefers `PubKeyOverride` when non-nil, falling back to the embedded `SelfCatalogPublicKey`.

`cmd/beekeeper/selfquarantine.go` hex-decodes `cfg.SelfCatalog.PubKey` and sets `opts.PubKeyOverride` when configured. A present-but-invalid key (wrong length after hex decode) fails closed immediately with a clear error rather than silently falling back to the embedded key (T-09-32).

**Task 3 (WR-03): Nearest-rank percentile for LatencyTracker**

`internal/llamafirewall/latency.go` replaces `idx := int(float64(n)*p)` with a shared `percentile(buf []int64, p float64) int64` helper using `idx := int(math.Ceil(p*float64(n))) - 1`, clamped to `[0, n-1]`. P95 of 1..100 now returns 95 (was 96); P99 of 1..100 now returns 99 (was 100); P95 of n=20 returns 19 (not the max 20).

**docs/THREAT-MODEL.md**: Section 9 added documenting the allowlist-override escape hatch (T-09-31) and the `release_age`/`lifecycle_script_allowlist` overlay limitation.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 654cd69 | feat(09-06): implement pure policy overlay + wire into check and policy test |
| 2 | 14d0eac | feat(09-06): wire self-hosted beekeeper-self public key through PubKeyOverride |
| 3 | c656654 | fix(09-06): correct LatencyTracker P95/P99 to nearest-rank percentile |

## Test Results

All 23 packages pass: `go test ./... -count=1` EXIT=0. Both `go build ./...` and `GOOS=windows go build ./...` exit 0.

Key new tests:
- `enforce_test.go`: 13 cases covering all combination rules (block, warn, allow downgrade, non-matching, corroboration no double-apply, command-shape extraction, multi-ecosystem)
- `TestPolicyOverlayBlocksViaDir` in handler_test.go: proves live beekeeper check applies policies-dir block rule
- `TestSelfCatalog_CustomKeyVerifiesAndEmbeddedFails`: custom-signed feed verifies with PubKeyOverride=customPub, fails with embedded key
- `TestEnforceSelfQuarantine_InvalidPubKeyFailsClosed`: short key causes fail-closed error before checkSelfCatalogFn
- `TestP95NinetyFifthPercentile`, `TestP95SmallNDoesNotCollapseToMax`: P95 of 1..100 = 95; n=20 P95 = 19 (not 20)

## Deviations from Plan

None — plan executed exactly as written. All PINNED design semantics followed.

## Known Stubs

None. All policy enforcement is wired to real decisions.

## Threat Flags

No new network endpoints, auth paths, or trust boundaries introduced. All changes are within existing surfaces:
- `internal/policyloader/enforce.go`: pure function, no I/O, no network
- `SelfCatalogOpts.PubKeyOverride`: tightens existing trust anchor, no new surface
- `percentile()`: purely mathematical, no security surface

## Self-Check: PASSED

- `internal/policyloader/enforce.go`: exists
- `internal/policyloader/enforce_test.go`: exists
- Commits 654cd69, 14d0eac, c656654: verified in git log
- `internal/policy/*` unchanged: confirmed zero diff
- `go test ./...`: all 23 packages pass
- `go build ./...` and `GOOS=windows go build ./...`: exit 0
- `docs/THREAT-MODEL.md` §9: added (allowlist escape hatch + overlay limitation)
