---
phase: 09-policy-as-code-self-defense-capstone
plan: "03"
subsystem: catalog/self-defense
tags: [self-quarantine, beekeeper-self, ed25519, fail-closed, catalog, state]
dependency_graph:
  requires:
    - internal/catalog/state.go (WatchState, LoadState, SaveState, writeFileAtomic)
    - internal/catalog/multi.go (MultiIndex, policy.MultiCatalogLookup)
    - internal/version/version.go (version.Version)
    - crypto/ed25519 (stdlib)
  provides:
    - SelfQuarantineState (state.go extension — backward-compatible)
    - WatchState.SelfQuarantine field (omitempty)
    - SelfCatalogPublicKey (compile-time embedded Ed25519 key)
    - CheckSelfCatalog(SelfCatalogOpts) SelfCatalogResult
    - errIntegrity sentinel (fail-closed integrity failure)
    - errNetwork sentinel (warn-continue network failure)
    - selfCatalogAdapter (policy.MultiCatalogLookup for "beekeeper" ecosystem)
  affects:
    - internal/catalog/state.go (WatchState extended)
    - internal/catalog/multi.go (Phase 9 comment: BeeKeeperSelf field ready)
tech_stack:
  added: []
  patterns:
    - Ed25519 signature verification via crypto/ed25519 stdlib
    - Typed error sentinels (errIntegrity, errNetwork) for fail-closed vs warn branching
    - HTTP fetch + disk cache + 24h TTL (mirrors osv.go pattern)
    - Atomic state writes via writeFileAtomic (temp file + rename)
    - TDD red/green/refactor cycle
key_files:
  created:
    - internal/catalog/selfkey.go
    - internal/catalog/selfcatalog.go
    - internal/catalog/selfcatalog_test.go
    - internal/catalog/testdata/selfcatalog_match.json
    - internal/catalog/testdata/selfcatalog_no_match.json
    - internal/catalog/testdata/selfcatalog_invalid_sig.json
  modified:
    - internal/catalog/state.go (SelfQuarantine field + SelfQuarantineState type)
    - internal/catalog/state_test.go (TestSelfQuarantineState_Persistence)
decisions:
  - "Ed25519 signature over canonical JSON of entries array (not full feed — avoids circular signature)"
  - "errIntegrity and errNetwork are distinct package-level sentinel errors — never collapsed"
  - "Offline persistence check happens BEFORE any network fetch — honors quarantine offline"
  - "Cache stores raw signed bytes (not parsed) so signature verification re-runs on cached data"
  - "SelfCatalogPublicKey populated via init() from compile-time hex constant — panics on malformed constant (programmer error, not runtime error)"
metrics:
  duration_minutes: 35
  completed_date: "2026-05-29"
  tasks_completed: 2
  tasks_total: 2
  files_created: 6
  files_modified: 2
---

# Phase 09 Plan 03: beekeeper-self Catalog Client Summary

**One-liner:** Ed25519-verified self-quarantine feed client with typed fail-closed/network-error sentinels, offline state persistence, and separate embedded signing key.

## What Was Built

### Task 1: SelfQuarantineState on WatchState + separate signing key (55514c7)

Extended `internal/catalog/state.go` with a new `SelfQuarantineState` struct and an optional `SelfQuarantine *SelfQuarantineState` field on `WatchState`. The omitempty pointer ensures full backward compatibility: pre-Phase-9 `state.json` files parse without changes, and new files only carry the field when quarantine is active.

Created `internal/catalog/selfkey.go` with `SelfCatalogPublicKey` — a compile-time embedded Ed25519 public key for verifying the beekeeper-self feed. This key is INDEPENDENT of the Sigstore/cosign release-signing identity (T-09-12: defeating both requires two independent compromises).

Added `TestSelfQuarantineState_Persistence` covering three sub-cases: full round-trip with quarantine data, backward-compatible load of pre-Phase-9 JSON, and nil field omitted on marshal.

### Task 2: CheckSelfCatalog — RED/GREEN TDD (95a1e66 + d891c2d)

**RED:** Created `selfcatalog_test.go` with 7 tests + 3 signed JSON fixtures. Tests verified to fail (build failure) before implementation existed.

**GREEN:** Implemented `internal/catalog/selfcatalog.go`:

- `CheckSelfCatalog(SelfCatalogOpts) SelfCatalogResult` — the primary entry point
- `SelfCatalogOutcome` enum: `Continue`, `Quarantine`, `FailClosed`, `WarnContinue`
- `errIntegrity` sentinel: invalid Ed25519 signature → `SelfCatalogFailClosed` — never degrades to warn
- `errNetwork` sentinel: fetch/timeout failure → `SelfCatalogWarnContinue` — never bricks the tool
- Offline persistence: reads `state.json.SelfQuarantine` BEFORE any network fetch; no network call if version already quarantined
- Real `ed25519.Verify` against `SelfCatalogPublicKey` (not the Phase 1 presence-only `VerifySignature`)
- Cache stores raw signed bytes; cache TTL is 24h (mirrors osv.go)
- `selfCatalogAdapter` implementing `policy.MultiCatalogLookup` for the "beekeeper" ecosystem

### Full 6-Row Behaviour Table Coverage

| Condition | Outcome | Test |
|-----------|---------|------|
| Offline state has quarantine for running version | Quarantine (no fetch) | TestSelfCatalog_OfflinePersistence |
| Fetch OK, sig valid, version matches | Quarantine + persist | TestSelfCatalog_VersionMatch |
| Fetch OK, sig valid, no match | Continue | TestSelfCatalog_NoMatch |
| Fetch OK, sig INVALID | FailClosed (errIntegrity) | TestSelfCatalog_InvalidSignature |
| Fetch fails, cache fresh (<24h) | Continue (using cache) | TestSelfCatalog_NetworkError_FreshCache |
| Fetch fails, no cache | WarnContinue (errNetwork) | TestSelfCatalog_NetworkError_NoCache |

## Deviations from Plan

### Auto-added Functionality

**TestSelfCatalog_OfflinePersistence and TestSelfCatalogAdapter_LookupAll** — The plan specified 5 behavior tests; I added 2 more to ensure complete coverage of the offline-persistence branch and the MultiCatalogLookup adapter. Both are mentioned in the plan's behavior description but not explicitly enumerated as named tests.

No Rule 4 architectural deviations. Plan executed exactly as written with the addition of full coverage tests.

## Verification Results

```
go build ./internal/catalog/...   → OK (no errors)
go vet ./internal/catalog/...     → OK (clean)
go test ./internal/catalog/... -count=1 → PASS (full suite including new tests)
```

Specific named test results:
- `TestSelfQuarantineState_Persistence` → PASS (3 subtests)
- `TestSelfCatalog_VersionMatch` → PASS
- `TestSelfCatalog_InvalidSignature` → PASS
- `TestSelfCatalog_NetworkError_NoCache` → PASS
- `TestSelfCatalog_NetworkError_FreshCache` → PASS
- `TestSelfCatalog_NoMatch` → PASS
- `TestSelfCatalog_OfflinePersistence` → PASS
- `TestSelfCatalogAdapter_LookupAll` → PASS

## Source Assertions (Acceptance Criteria)

```
grep -n "SelfCatalogPublicKey" internal/catalog/selfkey.go   → 3 matches (decl + init)
grep -c "http" internal/catalog/selfkey.go                   → 0  (no runtime fetch)
grep -c "errIntegrity\|errNetwork" internal/catalog/selfcatalog.go → 15 (both used distinctly)
grep -c "ed25519" internal/catalog/selfcatalog.go            → 4  (real verification)
```

## Threat Surface Scan

No new network endpoints. The `selfcatalog.go` module fetches from a configurable HTTPS URL (default: `https://beekeeper-self.mzansi-agentive.io/beekeeper-self.json`) — this is the intended trust boundary documented in the plan's threat model (T-09-09 through T-09-13). All STRIDE mitigations from the plan are implemented.

## Known Stubs

None. The implementation is complete for the client side. The live external hosting endpoint (`https://beekeeper-self.mzansi-agentive.io/beekeeper-self.json`) is an ops deliverable tracked in 09-VALIDATION.md as a manual gate — this is expected and documented in the plan's success criteria.

## Self-Check: PASSED

Files created/exist:
- internal/catalog/selfkey.go ✓
- internal/catalog/selfcatalog.go ✓
- internal/catalog/selfcatalog_test.go ✓
- internal/catalog/testdata/selfcatalog_match.json ✓
- internal/catalog/testdata/selfcatalog_no_match.json ✓
- internal/catalog/testdata/selfcatalog_invalid_sig.json ✓

Commits exist:
- 55514c7 (Task 1: SelfQuarantineState + selfkey.go) ✓
- 95a1e66 (Task 2 RED: failing tests) ✓
- d891c2d (Task 2 GREEN: implementation) ✓
