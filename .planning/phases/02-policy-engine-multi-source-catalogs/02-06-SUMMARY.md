---
phase: 02-policy-engine-multi-source-catalogs
plan: "06"
subsystem: policy-engine + catalog-adapters
tags: [policy, release-age, lifecycle-scripts, registry, cache, pure-function, tdd, fail-closed]
dependency_graph:
  requires: [02-01]
  provides: [EvaluateReleaseAge, EvaluateLifecycle, FetchPublishAge, fetchPublishTime, fetchLifecycleScripts]
  affects: [internal/policy, internal/catalog]
tech_stack:
  added: []
  patterns: [pure-function-purity-test, cache-first-disk-cache, httptest-overridable-base-urls, fail-closed-on-missing]
key_files:
  created:
    - internal/policy/release_age.go
    - internal/policy/release_age_test.go
    - internal/policy/lifecycle.go
    - internal/policy/lifecycle_test.go
    - internal/catalog/registry.go
    - internal/catalog/registry_test.go
    - internal/catalog/age_cache.go
    - internal/catalog/age_cache_test.go
  modified: []
decisions:
  - "Allowlist check in EvaluateReleaseAge runs BEFORE missing-timestamp check — allowlisted packages exempt from all age checks including fail-closed"
  - "ErrEcosystemLifecycleUnsupported returned for all non-npm ecosystems; caller sets RegistryCheckFailed:true → EvaluateLifecycle blocks (documented escape hatch: lifecycle.json allowlist)"
  - "FetchPublishAge takes caller-supplied now time.Time so TTL math and age math are testable without wall-clock flakiness"
  - "Missing:true cache entry written on any registry failure — prevents registry hammering within 24h TTL window (T-02-06-02)"
  - "npm full-package document endpoint (GET /pkg) used for publish timestamps; per-version doc lacks .time map"
metrics:
  duration: "~7 min"
  completed: "2026-05-26T09:41:00Z"
  tasks_completed: 3
  files_modified: 8
---

# Phase 2 Plan 06: Release-Age + Lifecycle Policies Summary

Delivers PLCY-02 (release-age policy) and PLCY-03 (lifecycle-script policy) using the pure/adapter split: two pure policy functions in `internal/policy` plus two I/O adapters in `internal/catalog` (registry HTTP fetchers + a 24h publish-timestamp cache).

## What Was Built

### Function Signatures

**`internal/policy/release_age.go`:**
```go
type ReleaseAgeInput struct {
    Ecosystem        string
    Package          string
    AgeMinutes       int64  // time.Since(publishedAt).Minutes() — computed by caller
    TimestampMissing bool   // true when registry returned no data (fail closed)
}

type ReleaseAgeConfig struct {
    DefaultMinutes      int64
    PerEcosystemMinutes map[string]int64
    Exclude             []string
}

func DefaultReleaseAgeConfig() ReleaseAgeConfig  // DefaultMinutes: 1440 (24h)
func EvaluateReleaseAge(input ReleaseAgeInput, cfg ReleaseAgeConfig) Decision
```

**`internal/policy/lifecycle.go`:**
```go
type LifecycleInput struct {
    Ecosystem           string
    Package             string
    ScriptsPresent      []string  // e.g. ["preinstall", "postinstall"]
    RegistryCheckFailed bool      // true when registry fetch failed (fail closed)
}

func EvaluateLifecycle(input LifecycleInput, allowlist []string) Decision
```

**`internal/catalog/registry.go`:**
```go
var ErrEcosystemLifecycleUnsupported = errors.New("lifecycle script inspection not supported for this ecosystem")

func fetchPublishTime(ctx, client, ecosystem, pkg, version) (string, error)
func fetchLifecycleScripts(ctx, client, ecosystem, pkg, version) ([]string, error)
```

**`internal/catalog/age_cache.go`:**
```go
func FetchPublishAge(
    ctx context.Context, client *http.Client,
    cacheDir, ecosystem, pkg, version string,
    now time.Time,
) (ageMinutes int64, missing bool, err error)
```

### Registry Endpoint Table

| Ecosystem | Publish Timestamp Endpoint | Field |
|-----------|---------------------------|-------|
| npm | `GET registry.npmjs.org/<pkg>` (full doc) | `.time[<version>]` |
| pypi | `GET pypi.org/pypi/<pkg>/<version>/json` | `.urls[0].upload_time_iso_8601` |
| cargo | `GET crates.io/api/v1/crates/<pkg>/<version>` | `.version.created_at` |
| rubygems | `GET rubygems.org/api/v1/versions/<pkg>.json` | `[].built_at` (version match) |
| go | `GET proxy.golang.org/<module>/@v/<version>.info` | `.Time` |
| packagist | `GET repo.packagist.org/p2/<pkg>.json` | `packages[pkg][].time` (version match) |

Lifecycle script inspection: npm only (version doc `.scripts` → `{preinstall,install,postinstall}`). All other ecosystems return `ErrEcosystemLifecycleUnsupported`.

### Fail-Closed Missing-Timestamp Contract

When the registry cannot provide a publish timestamp (any error, 4xx, 5xx, or network failure):

1. `fetchPublishTime` returns `("", error)`.
2. `FetchPublishAge` writes a `Missing:true` cache entry (24h TTL — prevents registry hammering on repeated calls).
3. `FetchPublishAge` returns `(0, true, nil)`.
4. The hook-handler caller builds `ReleaseAgeInput{TimestampMissing: true}`.
5. `EvaluateReleaseAge` blocks with reason "publish timestamp unavailable (fail-closed)".

Same pattern for lifecycle: any `fetchLifecycleScripts` error → `RegistryCheckFailed:true` → `EvaluateLifecycle` blocks.

### npm-Primary Lifecycle Inspection Coverage

npm lifecycle script inspection is the only supported ecosystem in Phase 2. For PyPI, Cargo, RubyGems, Composer, and Go modules, `fetchLifecycleScripts` returns `ErrEcosystemLifecycleUnsupported`. The caller MUST treat this as `RegistryCheckFailed:true`, which causes `EvaluateLifecycle` to block. Users with legitimate non-npm packages that have build hooks (e.g., a Cargo crate with a build.rs) must add them to `~/.beekeeper/policies/lifecycle.json` (the allowlist escape hatch). This is documented in a code comment in registry.go.

## Decision Logic

### EvaluateReleaseAge (PLCY-02)

1. Package in `cfg.Exclude` → allow ("release-age allowlisted") — BEFORE missing check
2. `TimestampMissing` → block ("publish timestamp unavailable (fail-closed)")
3. Resolve threshold: `cfg.PerEcosystemMinutes[ecosystem]` if present, else `cfg.DefaultMinutes`
4. `AgeMinutes < threshold` → block ("package age Xm below minimum Ym")
5. Otherwise → allow

### EvaluateLifecycle (PLCY-03)

1. `RegistryCheckFailed` → block ("lifecycle script check unavailable (fail-closed)")
2. `len(ScriptsPresent) == 0` → allow ("no lifecycle scripts")
3. Package in allowlist → allow ("lifecycle allowlisted")
4. Block ("lifecycle scripts present (preinstall,postinstall); add package to allowlist")

## Tests

| Test | File | Verifies |
|------|------|---------|
| TestReleaseAgeYoungPackageBlocked | release_age_test.go | 30-min pkg < 1440-min threshold → block |
| TestReleaseAgeOldPackageAllowed | release_age_test.go | 2000-min pkg ≥ 1440-min threshold → allow |
| TestReleaseAgeTimestampMissingBlocks | release_age_test.go | TimestampMissing → fail-closed block |
| TestReleaseAgeAllowlistExempt | release_age_test.go | Exclude list → allow regardless of age |
| TestReleaseAgePerEcosystemOverride | release_age_test.go | npm 60-min override → 90-min pkg allows |
| TestReleaseAgeAllowlistBeforeMissing | release_age_test.go | Allowlist takes precedence over missing |
| TestReleaseAgeImportsArePure | release_age_test.go | AST purity gate on release_age.go |
| TestLifecycleScriptPresentNotAllowlisted | lifecycle_test.go | postinstall + not allowlisted → block |
| TestLifecycleScriptPresentAllowlisted | lifecycle_test.go | Allowlisted pkg → allow |
| TestLifecycleNoScriptsAllowed | lifecycle_test.go | Empty scripts → allow |
| TestLifecycleRegistryCheckFailedBlocks | lifecycle_test.go | Registry failure → fail-closed block |
| TestLifecycleNilScriptsAllowed | lifecycle_test.go | Nil scripts → allow |
| TestLifecycleMultipleScriptsReason | lifecycle_test.go | Multiple scripts named in reason |
| TestLifecycleImportsArePure | lifecycle_test.go | AST purity gate on lifecycle.go |
| TestFetchNPMPublishTime | registry_test.go | npm .time map → parsed timestamp |
| TestFetchNPMLifecycleScripts | registry_test.go | npm .scripts → ["postinstall"] |
| TestFetchPublishTimeUnsupportedEcosystem | registry_test.go | Unknown eco → error |
| TestFetchNon200IsError | registry_test.go | 404 → non-nil error (fail-closed) |
| TestFetchLifecycleScriptsNonNpmReturnsUnsupported | registry_test.go | Cargo → ErrEcosystemLifecycleUnsupported |
| TestFetchNPMPublishTimeMissingVersion | registry_test.go | Missing version in .time → error |
| TestFetchNPMLifecycleNoScripts | registry_test.go | No lifecycle keys → empty slice, nil error |
| TestFetchLifecycleScriptsPyPIUnsupported | registry_test.go | PyPI → ErrEcosystemLifecycleUnsupported |
| TestFetchPublishAgeFresh | age_cache_test.go | 30-min-old pkg → ageMinutes ~30, missing false |
| TestFetchPublishAgeCacheHit | age_cache_test.go | Cache hit served with server closed |
| TestFetchPublishAgeMissingOnError | age_cache_test.go | Registry error → (0, true, nil) + Missing:true cache |
| TestFetchPublishAgeMissingCacheServed | age_cache_test.go | Pre-written Missing:true cache → served without network |
| TestFetchPublishAgeStaleEntryRefetched | age_cache_test.go | >24h-old cache entry triggers fresh fetch |

All 27 tests pass. All purity tests pass (no os/net/io/sync/time/context in release_age.go or lifecycle.go).

## TDD Gate Compliance

- Task 1 RED: commit `f2327aa` (test) — tests fail with undefined symbol errors
- Task 1 GREEN: commit `7c2a247` (feat) — all 14 tests pass
- Tasks 2 and 3: not TDD-flagged in plan; tests written concurrently with implementation

## Threat Mitigations Implemented

| Threat ID | Mitigation |
|-----------|-----------|
| T-02-06-02 | Missing:true cache entry written on any registry failure — prevents repeated registry hammering within 24h TTL window; block is correct per PLCY-02 but not self-inflicted DoS |
| T-02-06-03 | TestReleaseAgeImportsArePure + TestLifecycleImportsArePure enforce AST-level import gating on both pure policy files |
| T-02-06-04 | EvaluateLifecycle: default-deny preinstall/install/postinstall unless allowlisted; RegistryCheckFailed fails closed |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. All policy logic and adapters are fully implemented and test-covered. The `now` parameter in `FetchPublishAge` is not a stub — it is the testability design; the production caller passes `time.Now().UTC()`.

## Threat Flags

None. No new network endpoints exposed, no new auth paths, no schema changes at trust boundaries. Registry calls are outbound from the daemon (not inbound).

## Commits

| Task | Type | Hash | Description |
|------|------|------|-------------|
| 1 RED | test | f2327aa | add failing tests for release-age + lifecycle policies |
| 1 GREEN | feat | 7c2a247 | implement pure release-age + lifecycle policy functions |
| 2 | feat | 710b65f | registry HTTP fetchers for publish timestamps + lifecycle scripts |
| 3 | feat | 2be1b39 | publish-timestamp 24h cache + FetchPublishAge adapter |

## Self-Check: PASSED

- `internal/policy/release_age.go`: FOUND
- `internal/policy/release_age_test.go`: FOUND
- `internal/policy/lifecycle.go`: FOUND
- `internal/policy/lifecycle_test.go`: FOUND
- `internal/catalog/registry.go`: FOUND
- `internal/catalog/registry_test.go`: FOUND
- `internal/catalog/age_cache.go`: FOUND
- `internal/catalog/age_cache_test.go`: FOUND
- Commit f2327aa: FOUND (RED tests)
- Commit 7c2a247: FOUND (GREEN implementation)
- Commit 710b65f: FOUND (registry fetchers)
- Commit 2be1b39: FOUND (age cache)
- `go test ./internal/policy/... -run 'TestReleaseAge|TestLifecycle' -count=1`: PASS
- `go test ./internal/catalog/... -run 'TestFetch|TestFetchPublishAge' -count=1`: PASS
- `go vet ./internal/policy/... ./internal/catalog/...`: PASS
