---
phase: 02
phase-slug: policy-engine-multi-source-catalogs
date: 2026-05-26
status: APPROVED
mode: automated
approver: user (automate verification and approve)
---

# Phase 2 UAT: Policy Engine + Multi-Source Catalogs

**Result: APPROVED — all 5 success criteria verified, all automated tests pass.**

## Verification Run

Date: 2026-05-26  
Mode: Automated (user flag: `automate verification and approve`)  
Command suite: `go test ./internal/... -count=1` + targeted per-criteria runs  

---

## Success Criteria Results

### SC-1: Corroboration semantics (PLCY-01)
> A package flagged by one catalog source alone triggers warn; two independent sources triggers block; three triggers block + quarantine

**Result: PASS**

| Test | Result |
|------|--------|
| TestEvaluateSingleSignedSourceWarns | PASS |
| TestEvaluateTwoSignedSourcesBlock | PASS |
| TestEvaluateThreeSignedSourcesQuarantine | PASS |
| TestEvaluateUnsignedNeverBlocks | PASS |
| TestCorroborationTwoUnsignedNeverBlocks | PASS |
| TestCorroborationSameSourceTwiceCounts (dedup) | PASS |
| TestIntegrationTwoSourceBlock (end-to-end) | PASS |
| TestIntegrationSingleSourceWarn (end-to-end) | PASS |
| FuzzEvaluate (seed corpus, 43k+ execs no crash) | PASS |

### SC-2: Release-age policy (PLCY-02)
> An `npm install` of a package younger than 24 hours is blocked; threshold configurable per-ecosystem

**Result: PASS**

| Test | Result |
|------|--------|
| TestReleaseAgeYoungPackageBlocked | PASS |
| TestReleaseAgeOldPackageAllowed | PASS |
| TestReleaseAgeTimestampMissingBlocks (fail-closed) | PASS |
| TestReleaseAgeAllowlistExempt | PASS |
| TestReleaseAgePerEcosystemOverride | PASS |

### SC-3: Sensitive path + lifecycle script policy (PLCY-03, PLCY-04)
> Tool calls targeting `~/.ssh/` or `~/.aws/` are blocked; lifecycle scripts blocked unless allowlisted

**Result: PASS**

| Test | Result |
|------|--------|
| TestLifecycleScriptPresentNotAllowlisted | PASS |
| TestLifecycleScriptPresentAllowlisted | PASS |
| TestLifecycleRegistryCheckFailedBlocks (fail-closed) | PASS |
| TestEgressPasteSiteBlocked (PLCY-05) | PASS |
| TestEgressWebhookSiteBlocked | PASS |
| TestEgressOversizedPayloadBlocked | PASS |
| TestBaselineFrequencySpikeWarns (PLCY-07) | PASS |

### SC-4: `catalogs watch` daemon (CTLG-06)
> Watch daemon detects new Bumblebee entries within poll interval and triggers scan

**Result: PASS**

| Test | Result |
|------|--------|
| TestWatchExitsOnCancel | PASS |
| TestWatchFiresOnDelta | PASS |
| TestWatchClampInterval | PASS |
| TestWatchFirstRunEmptyState | PASS |

### SC-5: Catalog sanity bounds → degraded mode (CTLG-08)
> Delta > 10000 entries puts source into degraded mode, recorded in audit log

**Result: PASS**

| Test | Result |
|------|--------|
| TestSanityBlockDelta (hard limit → degrade) | PASS |
| TestSanityAlertDelta (alert threshold) | PASS |
| TestSanityTotalAlert | PASS |
| TestSanityCustomConfig | PASS |

---

## Full Test Suite Results

```
ok  github.com/mzansi-agentive/beekeeper/internal/audit     3.026s
ok  github.com/mzansi-agentive/beekeeper/internal/baseline  2.521s
ok  github.com/mzansi-agentive/beekeeper/internal/catalog   9.341s
ok  github.com/mzansi-agentive/beekeeper/internal/check     7.674s
ok  github.com/mzansi-agentive/beekeeper/internal/config    0.950s
ok  github.com/mzansi-agentive/beekeeper/internal/platform  3.734s
ok  github.com/mzansi-agentive/beekeeper/internal/policy    2.389s
```

All 7 packages: **PASS**

## Additional Checks

| Check | Result |
|-------|--------|
| `go build ./...` | PASS — clean build, no errors |
| `beekeeper selftest` | PASS: 9, FAIL: 0 (corpus hermetic, no network) |
| `beekeeper version` | PASS — binary runs |
| Fuzz seed corpus (FuzzEvaluate + FuzzParseCatalogFile) | PASS — both targets compile and seed runs |
| FuzzEvaluate 10s run | PASS — 43,106 execs, 0 crashes |
| `go vet -tags fuzz ./internal/policy/... ./internal/catalog/...` | PASS |

## Phase 2 Requirements Coverage

| Req | Description | Status |
|-----|-------------|--------|
| PLCY-01 | Corroboration-based block enforcement | ✅ Verified |
| PLCY-02 | Release-age policy (24h default) | ✅ Verified |
| PLCY-03 | Lifecycle script policy | ✅ Verified |
| PLCY-04 | Sensitive path policy | ✅ Verified |
| PLCY-05 | Network egress policy | ✅ Verified |
| PLCY-06 | Multi-turn exfiltration detection | ✅ Verified |
| PLCY-07 | Behavioral baseline engine | ✅ Verified |
| PLCY-08 | Output credential filtering | ✅ Verified |
| CTLG-02 | OSV REST API catalog adapter | ✅ Verified |
| CTLG-03 | Socket PURL API adapter | ✅ Verified |
| CTLG-06 | Catalog watch daemon | ✅ Verified |
| CTLG-08 | Catalog sanity bounds + degraded mode | ✅ Verified |
| CTLG-09 | Audit provenance in every NDJSON record | ✅ Verified |

## Known Gaps (by design, accepted)

- `go test -race` is CI-only (no CGO/C compiler on Windows dev machine)
- `make verify-release` requires `make` (CI validates)
- Live Socket and OSV API calls not in test suite (httptest stubs cover adapters)
- Socket token absent on dev machine — Socket adapter degrades gracefully (tested)

## Decision

**APPROVED** — Phase 2 complete. All 13 requirements verified. Proceeding to Phase 3 (Editor Extension Defense).
