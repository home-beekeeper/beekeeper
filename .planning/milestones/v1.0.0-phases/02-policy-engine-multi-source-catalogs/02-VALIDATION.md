---
phase: 02
phase-slug: policy-engine-multi-source-catalogs
date: 2026-05-26
---

# Phase 2 Validation Strategy

Extracted from `02-RESEARCH.md § Validation Architecture` — the formal artifact required by Nyquist Dimension 8.

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + `go test` |
| Quick run command | `go test ./internal/... -count=1` |
| Full suite command | `go test -race -count=1 ./...` (CI-only; race detector requires CGO) |
| Fuzz run command | `go test ./internal/policy/... -fuzz=FuzzPolicyEvaluate -fuzztime=30s` |

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command |
|--------|----------|-----------|-------------------|
| PLCY-01 | 1 source → warn, 2 sources → block, 3 → block+quarantine | unit | `go test ./internal/policy/... -run TestCorroboration -v` |
| PLCY-01 | Unsigned sources count 0.5; two unsigned still only warn | unit | `go test ./internal/policy/... -run TestUnsignedCorroboration` |
| PLCY-01 | Same source in two catalog files = 1 independent source | unit | `go test ./internal/policy/... -run TestSameSourceDedup` |
| PLCY-01 | FuzzPolicyEvaluate — must not panic on arbitrary tool call | fuzz | `go test ./internal/policy/... -fuzz=FuzzPolicyEvaluate -fuzztime=30s` |
| PLCY-02 | Package younger than threshold → block with reason | unit | `go test ./internal/policy/... -run TestReleaseAgeBlock` |
| PLCY-02 | Timestamp unavailable → fail-closed block | unit | `go test ./internal/policy/... -run TestReleaseAgeFailClosed` |
| PLCY-03 | Package with lifecycle script + not in allowlist → block | unit | `go test ./internal/policy/... -run TestLifecycleScriptBlock` |
| PLCY-03 | Non-npm ecosystem returns ErrEcosystemLifecycleUnsupported → block | unit | `go test ./internal/catalog/... -run TestFetchLifecycleScriptsNonNpmReturnsUnsupported` |
| PLCY-04 | Tool call targeting `~/.ssh/id_rsa` → block | unit | `go test ./internal/policy/... -run TestSensitivePathBlock` |
| PLCY-05 | Outbound to pastebin.com → block | unit | `go test ./internal/policy/... -run TestEgressBlock` |
| PLCY-06 | High entropy output → warn decision | unit | `go test ./internal/policy/... -run TestEntropyDetection` |
| PLCY-07 | Baseline deviation > 3σ → warn | unit | `go test ./internal/policy/... -run TestBaselineDeviation` |
| PLCY-08 | Tool output with `AKIA*` pattern → redacted | unit | `go test ./internal/policy/... -run TestCredentialRedact` |
| CTLG-02 | OSV adapter returns matches for known-vulnerable package | unit (httptest) | `go test ./internal/catalog/... -run TestOSVLookup` |
| CTLG-02 | FuzzCatalogParser — must not panic on arbitrary input | fuzz | `go test ./internal/catalog/... -fuzz=FuzzCatalogParser -fuzztime=30s` |
| CTLG-03 | Socket adapter gracefully disables when token absent | unit | `go test ./internal/catalog/... -run TestSocketNoToken` |
| CTLG-03 | Socket adapter respects 24h TTL cache | unit | `go test ./internal/catalog/... -run TestSocketCacheHit` |
| CTLG-06 | `catalogs watch` exits cleanly on shutdown signal | integration | `go test ./internal/catalog/... -run TestWatchShutdown` |
| CTLG-08 | Delta > 10000 entries → source degraded in state.json | unit | `go test ./internal/catalog/... -run TestSanityBoundsHardLimit` |
| CTLG-09 | Audit record includes catalog_matches with full provenance | unit | `go test ./internal/audit/... -run TestProvenanceFields` |

## Sampling Rate

- **Per task commit:** `go test ./internal/... -count=1` (< 30 seconds on Windows dev machine)
- **Per wave merge:** `go test -race -count=1 ./...` (CI-only — race detector requires CGO)
- **Phase gate (fuzz):** `FuzzPolicyEvaluate -fuzztime=30s` + `FuzzCatalogParser -fuzztime=30s` in CI release job behind `//go:build fuzz`
- **Phase gate (full):** Full test suite green on ubuntu-latest, macos-latest, windows-latest before `/gsd-verify-work`

## Wave 0 Gaps

Test files that must be created before or during Wave 1 execution (before they are needed by later waves):

- [ ] `internal/policy/corroboration_test.go` — TestCorroboration, TestUnsignedCorroboration, TestSameSourceDedup
- [ ] `internal/policy/release_age_test.go` — TestReleaseAgeBlock, TestReleaseAgeFailClosed
- [ ] `internal/policy/lifecycle_test.go` — TestLifecycleScriptBlock
- [ ] `internal/policy/path_test.go` — TestSensitivePathBlock
- [ ] `internal/policy/egress_test.go` — TestEgressBlock
- [ ] `internal/policy/exfil_test.go` — TestEntropyDetection
- [ ] `internal/policy/baseline_test.go` — TestBaselineDeviation
- [ ] `internal/policy/credentials_test.go` — TestCredentialRedact
- [ ] `internal/policy/fuzz_test.go` — FuzzPolicyEvaluate (Plan 09, `//go:build fuzz`)
- [ ] `internal/catalog/fuzz_test.go` — FuzzCatalogParser (Plan 09, `//go:build fuzz`)
- [ ] `internal/audit/types_test.go` — TestProvenanceFields
- [ ] `internal/catalog/registry_test.go` — TestFetchNPMLifecycleScripts, TestFetchLifecycleScriptsNonNpmReturnsUnsupported
- [ ] `internal/catalog/sanity_test.go` — TestSanityBoundsHardLimit
- [ ] `internal/catalog/watch_test.go` — TestWatchShutdown

## Known Gaps (by design)

- `go test -race` is CI-only (no CGO/C compiler on Windows dev machine; accepted in Phase 1, carried forward)
- `make verify-release` requires `make` (not installed on Windows dev machine; CI validates)
- Live Socket and OSV API calls are NOT part of the test suite — httptest.Server stubs cover the adapters; live validation happens during manual smoke testing after `beekeeper catalogs sync`
