# Phase 2: Policy Engine + Multi-Source Catalogs - Research

**Researched:** 2026-05-26
**Domain:** Go policy engine extension, OSV offline DB, Socket public API, registry timestamp APIs, fuzz testing, catalog watch daemon, behavioral baseline
**Confidence:** HIGH (most topics verified via live API calls and official docs)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Corroboration Semantics (PLCY-01):**
- 1 independent source match → warn (Phase 1 behavior preserved)
- 2 independent sources agree → block
- 3 independent sources agree → block + quarantine recommendation
- "Independent" = different `catalog_source` values; same source in two catalog files does NOT count as two sources
- Thresholds configurable per-ecosystem in config
- Full `catalog_matches` provenance MUST appear in every audit record
- Unsigned catalog sources count as 0.5 toward threshold; at least one signed source required for enforcement

**OSV Database (CTLG-02):**
- Use `github.com/google/osv-scanner/v2` Go library — NOT CLI exec per-call
- OSV DB synced offline via ecosystem ZIPs from `osv-vulnerabilities.storage.googleapis.com`
- Stored in `~/.beekeeper/catalogs/osv/` (Windows: `%APPDATA%\beekeeper\catalogs\osv\`)
- Sync as part of `beekeeper catalogs sync`
- Default ecosystems: npm, pypi, go, cargo, rubygems, packagist (configurable)
- OSV query results cached in-memory per `beekeeper check` invocation; cache lives outside `internal/policy`
- Results pre-resolved into `CatalogMatch` structs before reaching the policy engine

**Socket Public API (CTLG-03):**
- Endpoint: `POST https://api.socket.dev/v0/purl` (see NOTE below — authentication IS required)
- Results cached by package+version with 24h TTL on disk in `~/.beekeeper/catalogs/socket-cache/`
- Exponential backoff on HTTP 429: base 1s, max 60s, up to 5 retries
- Cache-first: age < 24h → use without network call
- Socket adapter resolves to `CatalogMatch` before reaching policy engine
- If Socket API unavailable: degrade to warn-only; log degradation to audit

**Release-Age Policy (PLCY-02):**
- Ecosystems: npm, PyPI, Cargo, RubyGems, Composer, Go modules
- Default minimum age: 1440 minutes (24h)
- Configurable per-ecosystem, per-package allowlist
- Fail closed if timestamp unavailable: block with reason "publish timestamp unavailable"
- Registry timestamp results cached 24h in `~/.beekeeper/catalogs/age-cache/`

**Lifecycle Script Policy (PLCY-03):** Default deny for preinstall/postinstall/install scripts; allowlist in `~/.beekeeper/policies/lifecycle.json`

**Sensitive Path Policy (PLCY-04):** Blocklist as specified in CONTEXT.md; path-aware resolution on Windows

**Network Egress Policy (PLCY-05):** Per-tool egress allowlists; outbound size limits; default package registry domains allowed

**Multi-turn Exfiltration Detection (PLCY-06):** Rolling entropy + base64 detection across turns; counter-based, no ML; baseline engine co-location

**Behavioral Baseline Engine (PLCY-07):** Per-project frequency counters in `~/.beekeeper/baselines/<project-hash>.json`; 7-day rolling window; 3-sigma deviation threshold; owner-only 0600 permissions

**Output Credential Filtering (PLCY-08):** Redact API key prefixes, JWT, Bearer, GitHub/npm tokens from PostToolUse outputs; full original in audit with `contains_credential_patterns: true`

**`beekeeper catalogs watch` Daemon (CTLG-06):** Polls all enabled catalog sources (default 1h); detects new Bumblebee `threat_intel/` entries; triggers targeted scan; SIGHUP/SIGTERM signal handling; foreground process model (full daemon management Phase 4)

**Catalog Sanity Bounds (CTLG-08):** Alert threshold 1000 new entries per sync; hard block threshold 10000; exceeding hard limit → entire source degraded to warning-only; state in `~/.beekeeper/state.json`

**Catalog Provenance in Audit Records (CTLG-09):** Full `catalog_matches` field in every NDJSON audit record with source, version, corroboration status; `corroboration_count`, `sources_agreed`, `sources_dissented` fields

**Pure-Function Constraint on `internal/policy`:** Absolute — no I/O, no goroutines, no side effects introduced in Phase 2. All catalog resolution happens in adapters that pass `CatalogMatch` slices to the policy engine.

### Claude's Discretion
- Exact registry API endpoints and response parsing for each ecosystem's publish timestamp
- In-memory cache architecture for OSV query results within a single `beekeeper check` invocation
- Socket PURL API request body format (beyond public API docs)
- Specific entropy algorithm for multi-turn exfiltration detection
- State file format for baseline counters and catalog watch delta state
- Daemon process management details for `catalogs watch`
- Error message formatting for each new policy block reason
- Whether to expose `beekeeper catalogs diff` command in Phase 2

### Deferred Ideas (OUT OF SCOPE)
- Editor extension defense (EDXT-01 through EDXT-06) — Phase 3
- Hook installation (INTG-01, INTG-02) — Phase 4
- MCP gateway — Phase 4
- Sentry daemon — Phase 5+
- LlamaFirewall — Phase 6
- Full audit sinks: syslog, OTLP, HTTPS POST — Phase 6
- `beekeeper audit query` / `beekeeper audit export` — Phase 6
- TUI dashboard — Phase 8
- Policy as code — Phase 9
- `beekeeper-self` catalog — Phase 9
- SLSA Level 3 provenance — Phase 7
- Desktop notifications for catalog watch — Phase 3
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PLCY-01 | Corroboration-based catalog matching: 1 source → warn, 2 → block, 3 → block+quarantine | `internal/policy.Evaluate` extended to accept multi-source `[]CatalogMatch`; pure corroboration counter logic; configurable thresholds |
| PLCY-02 | Release-age policy: 6 ecosystems, 24h default, configurable per-ecosystem/package | Registry timestamp APIs confirmed for all 6 ecosystems; caching pattern established |
| PLCY-03 | Lifecycle script policy: allowlist-only deny for preinstall/postinstall/install | npm registry `scripts` field confirmed; similar pattern for other ecosystems |
| PLCY-04 | Sensitive path policy: blocklist with cross-platform path resolution | Pure function implementation; platform-aware `~` expansion |
| PLCY-05 | Network egress policy: per-tool allowlists, outbound size limits | Config-driven allow/deny lists; request body size tracking |
| PLCY-06 | Multi-turn exfiltration detection: rolling entropy, base64 accumulation | Shannon entropy (math/bits, stdlib); rolling window counter in baseline engine |
| PLCY-07 | Behavioral baseline engine: per-project frequency counters, 3-sigma deviation | JSON counter file at 0600; rolling window; stddev calculation |
| PLCY-08 | Output credential filtering: redact known credential patterns from tool outputs | Regex patterns; PostToolUse hook target; audit records with redacted flag |
| CTLG-02 | OSV database offline sync and query | `osv-scanner/v2` v2.3.8 library; OSV REST API alternative; offline DB at GCS |
| CTLG-03 | Socket PURL API with 24h cache and exponential backoff | Socket API requires Bearer token auth; free tier 500 quota/hour; 24h cache is essential |
| CTLG-06 | `beekeeper catalogs watch` foreground daemon | fsnotify v1.10.1 for `~/.beekeeper/catalogs/` watch; SIGTERM/SIGHUP via `os/signal`; polling loop |
| CTLG-08 | Catalog sanity bounds and degraded mode | Delta counting during sync; `state.json` degraded-mode flag per source |
| CTLG-09 | Full catalog provenance in every NDJSON audit record | `AuditRecord` struct extension; `corroboration_count` + `sources_agreed/dissented` fields |
</phase_requirements>

---

## Summary

Phase 2 extends the Phase 1 policy engine foundation into a full multi-source corroboration system. The core architectural challenge is keeping `internal/policy` pure while integrating OSV and Socket catalog sources that require I/O, network calls, and caching. The solution is a two-layer architecture: catalog adapters (`internal/catalog/osv/`, `internal/catalog/socket/`) perform all I/O and resolve results into `CatalogMatch` slices, which are passed into the existing `policy.Evaluate()` function. The policy engine adds corroboration counting logic but remains free of I/O.

The critical empirical finding from this research is that the Socket API **requires Bearer token authentication even for basic queries** — the CONTEXT.md description of "no key required for basic queries" is incorrect based on live testing (`{"error":{"message":"Unauthorized"}}` on unauthenticated POST). The free tier provides 500 quota units/hour with a token. This means Beekeeper requires users to provide a Socket API token; the 24h TTL disk cache makes this manageable in practice. Alternatively, the OSV REST API (`https://api.osv.dev/v1/query`) is fully public, requires no authentication, and can serve as the reliable second source in many cases.

For OSV, the `github.com/google/osv-scanner/v2` library's offline database mechanism uses `internal/clients/clientimpl/localmatcher` which is an **unexported package** (not importable externally). The practical approach is either: (a) call the public OSV REST API (`api.osv.dev/v1/query`) with a 24h cache — simple, no large DB download required; or (b) import `pkg/osvscanner.DoScan()` with a synthetic lockfile approach for offline mode. The REST API approach is significantly simpler and recommended for Phase 2.

All six registry timestamp APIs are verified. The pattern is consistent: query a per-version endpoint, extract the publish timestamp, cache 24h. The npm `time` field in the full package metadata (`registry.npmjs.org/<pkg>`) provides version-keyed timestamps — the simplest pattern.

**Primary recommendation:** Implement OSV as an HTTP client against `api.osv.dev/v1/query` (24h cache, no library dependency), Socket as an HTTP client against `api.socket.dev/v0/purl` (24h cache, requires Bearer token), and registry timestamp lookups as per-ecosystem HTTP clients. All sources resolved outside `internal/policy` via adapter pattern extending the existing `CatalogLookup` interface.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Corroboration counting + decision | `internal/policy` (pure lib) | — | Must stay pure; receives pre-resolved `[]CatalogMatch` from adapters |
| OSV catalog adapter (HTTP/cache) | `internal/catalog/osv` | Called by check handler | I/O adapter; not in policy engine |
| Socket catalog adapter (HTTP/cache) | `internal/catalog/socket` | Called by check handler | I/O adapter; requires token from config |
| Bumblebee mmap adapter | `internal/catalog` (existing) | — | Already exists; implements `CatalogLookup` |
| Registry timestamp lookup | `internal/registry` (new) | Per-ecosystem HTTP clients | One package, 6 sub-implementations |
| Lifecycle script inspection | `internal/registry` (new) | Reads registry package metadata | npm: `scripts` field in `/pkg/ver`; others similar |
| Release-age policy rule | `internal/policy` (pure) | Calls registry adapter result | Pre-resolved timestamp passed in |
| Sensitive path policy rule | `internal/policy` (pure) | — | Pure string/path matching |
| Network egress policy rule | `internal/policy` (pure) | — | Config-driven allowlist matching |
| Behavioral baseline counter | `internal/baseline` (new) | File I/O for counter persistence | Counter update is outside policy; deviation check can be pure |
| Credential redaction | `internal/filter` (new) | PostToolUse output path | Regex-based redaction |
| `catalogs watch` daemon | `cmd/beekeeper` (Cobra cmd) | `internal/catalog` sync loop | Foreground process with signal handling |
| Catalog sanity bounds check | `internal/catalog` (extended) | During sync path | Delta counting in `catalog.Sync()` |
| Audit record provenance fields | `internal/audit` (extended) | Called by check handler | `AuditRecord` struct gets `CorroborationCount`, `SourcesAgreed`, `SourcesDissented` |
| State file (degraded mode) | `internal/state` (new) | Written during sync | JSON file at `~/.beekeeper/state.json` |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` | stdlib | OSV API, Socket API, registry timestamp queries | Already in project; no external dep for HTTP clients |
| `encoding/json` | stdlib | All API response parsing, cache serialization, state.json | Consistent with Phase 1 choices |
| `regexp` | stdlib | Credential pattern matching (PLCY-08), sensitive path glob-like matching | No external regex lib needed for fixed patterns |
| `math` | stdlib | Shannon entropy calculation (PLCY-06) | Shannon entropy is a `math.Log2` formula; no external lib |
| `os/signal` | stdlib | SIGTERM/SIGHUP handling in `catalogs watch` daemon | Standard Unix signal handling pattern |
| `github.com/fsnotify/fsnotify` | v1.10.1 | Filesystem watcher for `catalogs watch` delta detection on `~/.beekeeper/catalogs/` | Cross-platform (Windows/Linux/macOS); already the locked library for Phase 3 |

[VERIFIED: fsnotify v1.10.1 published 2026-05-04 via proxy.golang.org]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/google/osv-scanner/v2` | v2.3.8 | OSV DB offline mode (CTLG-02 future) | Import `pkg/osvscanner.DoScan` only if REST API approach proves insufficient; `localmatcher` package is internal and NOT importable |
| `golang.org/x/sys` | already in go.sum | Platform-specific ops (already Phase 1 dep) | Signal handling edge cases on Windows |
| `sync` | stdlib | In-memory cache for OSV query results per `beekeeper check` invocation | `sync.Map` or `sync.Mutex`-guarded `map` |

[VERIFIED: osv-scanner v2.3.8 published 2026-05-08 via proxy.golang.org]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| OSV REST API (`api.osv.dev/v1/query`) | `osv-scanner/v2` library | Library `localmatcher` is internal; `DoScan` requires lockfile input not single-package query; REST API is simpler and public |
| Shannon entropy via `math.Log2` | `github.com/chrisjchandler/entropy` | External dep for 8 lines of code; not worth it; stdlib math is sufficient |
| `os/signal` for daemon signal handling | `github.com/oklog/run` or `golang.org/x/sync/errgroup` | For a simple foreground daemon with 2 signals, `os/signal.NotifyContext` is sufficient without additional deps |
| `regexp` stdlib for credential patterns | `github.com/dlclark/regexp2` (PCRE) | PCRE2 needed only for lookbehind/lookahead; stdlib `regexp` is sufficient for prefix-match patterns (AKIA*, sk-*, etc.) |

**Installation:**
```bash
go get github.com/fsnotify/fsnotify@v1.10.1
# OSV library (only if lockfile-based scan approach chosen):
# go get github.com/google/osv-scanner/v2@v2.3.8
```

---

## Architecture Patterns

### System Architecture Diagram

```
                    PHASE 2 POLICY EVALUATION FLOW
                    ──────────────────────────────────

Agent tool call → beekeeper check (fresh subprocess)
        │
        ▼
[io.LimitReader + json.Decode] → ToolCall{ecosystem, pkg, version}
        │
        ├──────────────────────────────────────────────────────┐
        │  CATALOG RESOLUTION (I/O — outside internal/policy)  │
        │                                                       │
        ├→ [internal/catalog] Bumblebee mmap Index.Lookup()    │
        │        └→ []CatalogMatch{source:"bumblebee"}         │
        │                                                       │
        ├→ [internal/catalog/osv] OSVAdapter.Lookup()          │
        │        └→ Check 24h disk cache (osv-cache/)          │
        │        └→ if miss: POST api.osv.dev/v1/query         │
        │        └→ []CatalogMatch{source:"osv"}               │
        │                                                       │
        ├→ [internal/catalog/socket] SocketAdapter.Lookup()    │
        │        └→ Check 24h disk cache (socket-cache/)       │
        │        └→ if miss: POST api.socket.dev/v0/purl       │
        │              Bearer token from config                 │
        │        └→ if unavailable: degrade (warn-only)        │
        │        └→ []CatalogMatch{source:"socket"}            │
        │                                                       │
        └──────────────────────────────────────────────────────┘
                        │
                        ▼  all CatalogMatch slices merged
        [internal/policy.Evaluate(tc ToolCall, allMatches []CatalogMatch, cfg PolicyConfig)]
                        │
        ┌───────────────┴───────────────────────────────────────┐
        │  PURE POLICY RULES (no I/O)                           │
        │                                                       │
        ├→ CorroborationRule: count unique catalog_source values │
        │        1 source (or unsigned) → warn                  │
        │        2 sources (≥1 signed) → block                  │
        │        3 sources              → block + quarantine_rec │
        │                                                       │
        ├→ ReleaseAgeRule: compare pre-resolved publish_time    │
        │        age < threshold → block                        │
        │                                                       │
        ├→ LifecycleScriptRule: check pre-resolved script flags │
        │        has_lifecycle_script AND not in allowlist → block│
        │                                                       │
        ├→ SensitivePathRule: path matching against blocklist   │
        │        match → block                                  │
        │                                                       │
        ├→ NetworkEgressRule: URL matching against allowlist    │
        │        not in allowlist → block                       │
        │                                                       │
        ├→ BaselineDeviationRule: pre-resolved deviation flag   │
        │        deviation > 3σ → warn                          │
        │                                                       │
        └→ CredentialFilterRule: regex match on tool OUTPUT     │
                post-tool-use only                              │
                        │
                        ▼
                Decision{Allow, Level, Reason, CatalogMatches,
                         CorroborationCount, SourcesAgreed, SourcesDissented}
                        │
                        ▼
        [internal/audit] Write extended NDJSON audit record
                        │
                        ▼
              exit 0 (allow/warn) or exit 1 (block)
```

### Recommended Project Structure (Phase 2 additions)

```
internal/
  catalog/
    osv/
      adapter.go      # OSVAdapter: HTTP client + disk cache for api.osv.dev
      adapter_test.go
    socket/
      adapter.go      # SocketAdapter: HTTP client + disk cache for api.socket.dev/v0/purl
      adapter_test.go
  registry/
    npm.go            # npm registry publish timestamp + lifecycle scripts
    pypi.go           # PyPI JSON API publish timestamp
    cargo.go          # crates.io API publish timestamp
    rubygems.go       # rubygems.org API v2 publish timestamp
    packagist.go      # packagist.org API publish timestamp
    gomod.go          # proxy.golang.org /@v/<ver>.info timestamp
    cache.go          # 24h TTL disk cache shared across registry lookups
    registry_test.go  # table-driven tests with httptest.Server stubs
  policy/
    types.go          # EXTENDED: PolicyInput struct, corroboration fields in Decision
    engine.go         # EXTENDED: Evaluate() accepts multi-source inputs
    engine_test.go    # extended fuzz + unit tests
    fuzz_test.go      # FuzzPolicyEngine (release-gating fuzz target)
  baseline/
    engine.go         # Frequency counter read/write, deviation calculation
    engine_test.go
  filter/
    credential.go     # Regex-based credential redaction (PLCY-08)
    credential_test.go
  state/
    state.go          # state.json read/write for degraded-mode flags, catalog watch delta
    state_test.go
  catalog/
    index.go          # unchanged
    schema.go         # unchanged
    sync.go           # EXTENDED: sanity bounds, degraded-mode trigger
    fuzz_test.go      # FuzzCatalogParser (release-gating fuzz target)
  audit/
    types.go          # EXTENDED: CorroborationCount, SourcesAgreed, SourcesDissented
    writer.go         # unchanged
~/.beekeeper/
  catalogs/
    osv/              # Ecosystem zip files from osv-vulnerabilities.storage.googleapis.com
      npm/all.zip
      PyPI/all.zip
      ...
    osv-cache/        # 24h TTL disk cache: {sha256(ecosystem+pkg+ver)}.json
    socket-cache/     # 24h TTL disk cache: {sha256(purl)}.json
    age-cache/        # 24h TTL registry timestamp cache: {sha256(ecosystem+pkg+ver)}.json
  baselines/
    <project-hash>.json  # per-project frequency counters
  state.json          # degraded-mode flags, catalog watch state
```

### Pattern 1: Extended CatalogLookup Interface and Multi-Source Resolution

**What:** The existing `CatalogLookup` interface (one source, one lookup) must be extended to support multi-source aggregation. The policy engine receives all matches from all sources pre-aggregated.

**When to use:** All Phase 2 catalog resolution.

```go
// Source: [ASSUMED] — design based on Phase 1 CatalogLookup + CONTEXT.md decisions

// MultiSourceLookup aggregates results from all enabled catalog sources.
// Lives in internal/catalog/multi.go — NOT in internal/policy (pure lib).
type MultiSourceLookup struct {
    Bumblebee CatalogLookup   // existing mmap index (Phase 1)
    OSV       *osv.Adapter    // new Phase 2
    Socket    *socket.Adapter // new Phase 2
}

// LookupAll queries all sources and returns all matches with source attribution.
// This is I/O — it MUST NOT be called from internal/policy.Evaluate.
func (m *MultiSourceLookup) LookupAll(ecosystem, pkg, version string) []policy.CatalogMatch {
    var matches []policy.CatalogMatch
    // Bumblebee (mmap, synchronous, no I/O from policy's perspective)
    if e, ok := m.Bumblebee.Lookup(ecosystem, pkg); ok {
        matches = append(matches, catalogEntryToMatch(e, version))
    }
    // OSV (HTTP with cache)
    matches = append(matches, m.OSV.LookupAll(ecosystem, pkg, version)...)
    // Socket (HTTP with cache)
    matches = append(matches, m.Socket.LookupAll(ecosystem, pkg, version)...)
    return matches
}

// In internal/policy/engine.go — pure function, receives pre-resolved matches:
func Evaluate(tc ToolCall, preResolvedMatches []CatalogMatch, cfg PolicyConfig) Decision {
    // Corroboration counting
    uniqueSources := uniqueSignedSources(preResolvedMatches)
    corrobCount := len(uniqueSources)
    // ... apply thresholds
}
```

**Key design principle:** `internal/policy.Evaluate` signature changes from `(tc ToolCall, idx CatalogLookup)` to `(tc ToolCall, inputs PolicyInput)` where `PolicyInput` contains pre-resolved data from all sources. This preserves purity.

### Pattern 2: OSV REST API Client (Recommended Approach)

**What:** Query the public OSV REST API for CVE data. No authentication required. Fully public.

**Endpoint:** `POST https://api.osv.dev/v1/query`

**Request format** (verified via live API call):
```json
{
  "package": {
    "name": "lodash",
    "ecosystem": "npm"
  },
  "version": "4.17.20"
}
```

**Response:** `{"vulns":[{"id":"GHSA-...","summary":"...","affected":[...]}]}`

**Ecosystem name mapping (case-sensitive — OSV is strict):**
| Beekeeper ecosystem | OSV ecosystem string |
|---------------------|---------------------|
| `npm` | `npm` |
| `pypi` | `PyPI` |
| `go` | `Go` |
| `cargo` | `crates.io` |
| `rubygems` | `RubyGems` |
| `packagist` | `Packagist` |

[VERIFIED: live OSV API call returning lodash vulnerabilities; ecosystem list from osv.dev docs]

```go
// Source: [VERIFIED: api.osv.dev live test + google.github.io/osv.dev/post-v1-query/]
// internal/catalog/osv/adapter.go

type OSVQuery struct {
    Package OSVPackage `json:"package"`
    Version string     `json:"version"`
}
type OSVPackage struct {
    Name      string `json:"name"`
    Ecosystem string `json:"ecosystem"` // case-sensitive; see mapping table above
}
type OSVResponse struct {
    Vulns []OSVVuln `json:"vulns"`
}
type OSVVuln struct {
    ID      string `json:"id"`
    Summary string `json:"summary"`
    // ...
}

func (a *OSVAdapter) Lookup(ecosystem, pkg, version string) ([]policy.CatalogMatch, error) {
    cacheKey := cacheKeyFor("osv", ecosystem, pkg, version)
    if cached, ok := a.cache.Get(cacheKey); ok {
        return cached, nil
    }

    osvEco := mapToOSVEcosystem(ecosystem) // e.g., "pypi" → "PyPI"
    body, _ := json.Marshal(OSVQuery{
        Package: OSVPackage{Name: pkg, Ecosystem: osvEco},
        Version: version,
    })
    resp, err := a.client.Post("https://api.osv.dev/v1/query", "application/json", bytes.NewReader(body))
    // ... parse, convert to []CatalogMatch, cache 24h
}
```

### Pattern 3: Socket PURL API Client (Requires Bearer Token)

**What:** Query Socket's PURL endpoint for supply chain risk alerts. REQUIRES Bearer token authentication.

**CRITICAL FINDING:** Live testing confirms Socket API returns `{"error":{"message":"Unauthorized"}}` for unauthenticated requests. The CONTEXT.md statement "no key required for basic queries" is INCORRECT. Beekeeper must require users to configure a Socket API token. The free tier provides **500 quota units/hour** with a registered token.

**Endpoint:** `POST https://api.socket.dev/v0/purl?alerts=true`
**Auth:** `Authorization: Bearer <token>`
**Request format:**
```json
{
  "components": [
    { "purl": "pkg:npm/[email protected]" }
  ]
}
```

[VERIFIED: live API test; docs.socket.dev/pricing free tier 500 quota/hour; unauthorized response confirmed]

```go
// Source: [VERIFIED: live API test; docs.socket.dev/reference/batchpackagefetch]
// internal/catalog/socket/adapter.go

func (a *SocketAdapter) Lookup(ecosystem, pkg, version string) ([]policy.CatalogMatch, error) {
    if a.token == "" {
        // No token configured — degrade gracefully; log warning
        return nil, nil
    }
    purl := toPURL(ecosystem, pkg, version) // e.g., "pkg:npm/[email protected]"
    // ... 24h cache check, then HTTP POST with Bearer token
}

// If Socket token is unconfigured OR network error → degrade (warn-only for this source)
// NOT fail-closed — Socket is one of multiple sources; losing it degrades corroboration
// but does not prevent evaluation from completing with remaining sources
```

**PURL format per ecosystem:**
| Ecosystem | PURL format |
|-----------|-------------|
| npm | `pkg:npm/<name>@<version>` |
| pypi | `pkg:pypi/<name>@<version>` |
| cargo | `pkg:cargo/<name>@<version>` |
| rubygems | `pkg:gem/<name>@<version>` |
| packagist | `pkg:composer/<vendor>/<name>@<version>` |
| go | `pkg:golang/<module>@<version>` |

### Pattern 4: Registry Timestamp APIs (Verified)

**npm:** `GET https://registry.npmjs.org/<pkg>` → `response.time["<version>"]` is ISO 8601 string.
```
"4.17.20":"2020-08-13T16:53:54.152Z"   ← confirmed via live test
```

**PyPI:** `GET https://pypi.org/pypi/<pkg>/<version>/json` → `response.urls[0].upload_time_iso_8601`
```
"upload_time_iso_8601": "2017-06-14T17:51:25.096686Z"   ← confirmed via pypi.org/pypi/requests/json
```

**Cargo:** `GET https://crates.io/api/v1/crates/<name>/<version>` with `User-Agent` header (required) → `response.version.created_at`
```
"created_at":"2017-04-20T15:26:44.055136Z"   ← confirmed via live test
```
Note: crates.io requires `User-Agent` header with contact info; requests without it return 403.

**RubyGems:** `GET https://rubygems.org/api/v2/rubygems/<gem>/versions/<version>.json` → `response.version_created_at`
```
"version_created_at":"2021-12-15T23:45:57.959Z"   ← confirmed via live test
```

**Packagist:** `GET https://repo.packagist.org/p2/<vendor>/<package>.json` → per-version `time` field
```
"time":"2026-05-20T11:46:02+00:00"   ← confirmed via live test
```

**Go modules:** `GET https://proxy.golang.org/<module>/@v/<version>.info` → `response.Time`
```
{"Version":"v2.3.8","Time":"2026-05-08T04:54:35Z",...}   ← confirmed via live test
```

[VERIFIED: all 6 registry APIs confirmed via live HTTP calls on 2026-05-26]

### Pattern 5: Corroboration Logic (Pure Function)

**What:** Count unique, independent, signed catalog sources from pre-resolved matches. Apply configurable thresholds.

```go
// Source: [ASSUMED] — design from CONTEXT.md decisions
// internal/policy/engine.go (extended)

// PolicyInput bundles all pre-resolved inputs so Evaluate stays pure.
type PolicyInput struct {
    ToolCall       ToolCall
    CatalogMatches []CatalogMatch  // from all sources, pre-aggregated
    PublishAge     *time.Duration  // nil if unavailable (triggers fail-closed)
    HasLifecycle   bool            // whether package has lifecycle scripts
    IsPathBlocked  bool            // for sensitive path policy
    // ... other pre-resolved inputs
}

func (d Decision) corroborate(matches []CatalogMatch, cfg CorroborationConfig) Decision {
    // Count unique source values. Same catalog_source in two files = 1 source.
    sourceSet := make(map[string]bool)
    signedCount := 0
    for _, m := range matches {
        sourceSet[m.CatalogSource] = true
        if m.Signed {
            signedCount++
        }
    }
    uniqueCount := len(sourceSet)

    // Unsigned sources count as 0.5: require at least 1 signed source for enforcement.
    // Rule: if all sources are unsigned and count >= 2, still only warn.
    hasSignedSource := signedCount > 0

    switch {
    case uniqueCount >= 3 && hasSignedSource:
        return Decision{Allow: false, Level: "block", QuarantineRecommended: true, ...}
    case uniqueCount >= 2 && hasSignedSource:
        return Decision{Allow: false, Level: "block", ...}
    default:
        return Decision{Allow: true, Level: "warn", ...}
    }
}
```

### Pattern 6: Fuzz Targets (Phase 2 Release-Gating)

**What:** Go native fuzzing (`go test -fuzz`) targets for catalog parser and policy engine. Phase 2 is when these become CI release gates.

```go
// Source: [VERIFIED: go.dev/doc/security/fuzz/]
// internal/policy/fuzz_test.go

//go:build !nofuzz

func FuzzPolicyEvaluate(f *testing.F) {
    // Seed corpus: minimal valid tool call JSON
    f.Add([]byte(`{"tool_name":"bash","tool_input":{"command":"npm install lodash@4.17.20"}}`))
    f.Add([]byte(`{}`))
    f.Add([]byte(`{"tool_name":"write_file","tool_input":{"path":"~/.ssh/id_rsa"}}`))

    f.Fuzz(func(t *testing.T, data []byte) {
        var tc policy.ToolCall
        if err := json.Unmarshal(data, &tc); err != nil {
            return // not a valid tool call — that's expected; skip
        }
        // Must not panic, must return a valid Decision
        d := policy.Evaluate(tc, policy.PolicyInput{ToolCall: tc})
        if d.Level != "allow" && d.Level != "warn" && d.Level != "block" {
            t.Fatalf("invalid decision level %q", d.Level)
        }
    })
}

// internal/catalog/fuzz_test.go
func FuzzCatalogParser(f *testing.F) {
    f.Add([]byte(`{"schema_version":"0.1.0","entries":[]}`))
    f.Fuzz(func(t *testing.T, data []byte) {
        _, _ = catalog.ParseCatalogFile(data)
        // Must not panic
    })
}
```

**CI gating pattern:** CI runs `go test ./... -fuzz=FuzzPolicyEvaluate -fuzztime=30s` on release tag push. Any failure → block release. Seed corpus failures (running fuzz as unit test in regular CI) always gate.

**Build tag:** The `//go:build !nofuzz` pattern allows teams to skip fuzz tests in environments without Go 1.18+ (though Go 1.25 is required, this just documents the convention). Fuzz tests in regular `go test ./...` run seed corpus only.

### Pattern 7: `beekeeper catalogs watch` Foreground Daemon

**What:** Long-running foreground process that polls catalog sources, detects deltas, triggers scans.

```go
// Source: [ASSUMED] — standard Go daemon pattern
// cmd/beekeeper: catalogs watch RunE

func runWatch(ctx context.Context, cfg WatchConfig) error {
    ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
    defer stop()

    watcher, err := fsnotify.NewWatcher()
    // ... watch ~/.beekeeper/catalogs/ for new files post-sync

    ticker := time.NewTicker(cfg.PollInterval) // default 1h
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil // clean shutdown on SIGTERM/Interrupt
        case <-ticker.C:
            if err := runSyncCycle(ctx, cfg); err != nil {
                // log error, continue — don't crash the daemon
                log.Printf("sync cycle error: %v", err)
            }
        case event := <-watcher.Events:
            if event.Op&fsnotify.Create != 0 {
                // New file in catalogs/ — possible delta
                handleCatalogDelta(event.Name)
            }
        }
    }
}
```

**Signal handling:** `os/signal.NotifyContext` (Go 1.16+) provides clean context cancellation on SIGTERM/SIGINT. This is the idiomatic Go pattern.

**SIGHUP:** On Unix, SIGHUP can trigger immediate re-sync (similar to nginx -s reload). On Windows, SIGHUP is not supported — document this limitation.

### Anti-Patterns to Avoid

- **Anti-pattern: I/O in `internal/policy.Evaluate`.** Any HTTP call, file read, or time.Now() inside `policy.Evaluate` breaks the pure-function contract and prevents use in the MCP gateway (Phase 4) and Sentry correlation (Phase 5). Always resolve catalog data in adapters before calling Evaluate. [CITED: CLAUDE.md + CONTEXT.md]

- **Anti-pattern: Importing `internal/clients/clientimpl/localmatcher` from osv-scanner.** This is an unexported internal package and cannot be imported by external modules. [VERIFIED: pkg.go.dev — package is listed under `internal/`]

- **Anti-pattern: Treating Socket API as unauthenticated.** Live testing confirms `{"error":{"message":"Unauthorized"}}` for unauthenticated requests. Beekeeper must document the Socket token requirement and degrade gracefully when unconfigured. [VERIFIED: live API test 2026-05-26]

- **Anti-pattern: Counting the same `catalog_source` in two catalog files as two independent sources.** The corroboration check must deduplicate by `catalog_source` string value, not by number of catalog files or number of entries. [CITED: CONTEXT.md corroboration semantics]

- **Anti-pattern: fsnotify recursive watching on Windows.** fsnotify v1.10.1 does NOT support recursive directory watching. Use explicit `watcher.Add()` per directory. For `catalogs watch`, this is straightforward (watching `~/.beekeeper/catalogs/` directly). [CITED: CLAUDE.md + fsnotify docs]

- **Anti-pattern: Failing closed when Socket API is unavailable.** Socket is one of three optional sources. If Socket is unavailable (no token, network error, 429), the correct behavior is to degrade to warn-only for packages that would have needed Socket's corroboration, not to block all checks. Only Bumblebee mmap must always be available (existing Phase 1 behavior). [CITED: CONTEXT.md Socket decision]

- **Anti-pattern: Storing credentials in the age-cache or socket-cache files.** Cache files store raw package metadata. Never include the Socket API token or registry credentials in cache file content. [ASSUMED — standard security hygiene]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Filesystem watching | `inotify`/`ReadDirectoryChangesW` syscall wrapper | `fsnotify` v1.10.1 | Cross-platform; handles Windows NTFS quirks; tested on Windows/Linux/macOS |
| PURL format construction | Custom string formatter | Well-known PURL spec (type/namespace/name@version) | Standard PURL format is documented; see pattern table above |
| Shannon entropy | Third-party entropy library | `math.Log2` in stdlib | 8 lines of code; no external dep justified |
| OSV ecosystem name mapping | Hardcoded switch statement | Verified table from OSV docs | OSV is case-sensitive; wrong case = no results (e.g., "pypi" instead of "PyPI") |
| HTTP retry logic with backoff | Custom retry loop | Standard pattern with `time.Sleep` + backoff formula | Simple enough to hand-roll correctly; no lib needed for exponential backoff with 5 retries |
| File-based TTL cache | BoltDB, Redis, SQLite | Simple JSON files with timestamp in filename or in content | No daemon mode in Phase 2; SQLite is overkill; JSON files are debuggable |

**Key insight:** The catalog adapter layer (OSV + Socket) involves network I/O and disk caching, but the individual components (HTTP client, JSON parsing, file-based cache) are all simple enough to implement with stdlib. The complexity budget is saved for the corroboration logic and policy rule integration.

---

## Common Pitfalls

### Pitfall 1: OSV Ecosystem Names Are Case-Sensitive

**What goes wrong:** Querying `api.osv.dev/v1/query` with `"ecosystem": "pypi"` returns 0 vulnerabilities even for known-vulnerable packages.

**Why it happens:** OSV API is case-sensitive. The correct string is `"PyPI"` not `"pypi"`, `"RubyGems"` not `"rubygems"`, `"crates.io"` not `"cargo"`. The OSV ecosystem name for Cargo is `"crates.io"` (not `"cargo"`).

**How to avoid:** Use a hardcoded mapping table at the OSV adapter layer. Never pass Beekeeper's internal ecosystem string directly to the OSV API.

**Warning signs:** OSV queries return `{"vulns":[]}` for packages known to have vulnerabilities.

### Pitfall 2: Socket API Requires Bearer Token (Breaks "No Key Required" Assumption)

**What goes wrong:** `beekeeper check` silently receives 401/Unauthorized from Socket API on every call if no token is configured, degrading all packages to warn-only via Socket (single-source Bumblebee-only decisions).

**Why it happens:** CONTEXT.md incorrectly states "no key required for basic queries" — live testing shows Socket returns 401 without a Bearer token. Socket free tier (500 quota/hour) requires account registration and token creation.

**How to avoid:** (1) Check for Socket token at startup; log a warning if absent (Socket source disabled). (2) Document token requirement clearly in `beekeeper init` output. (3) Graceful degrade: if Socket token absent, treat Socket as "unavailable" (not as a failure) and proceed with Bumblebee + OSV only.

**Warning signs:** All decisions are single-source warn even for packages in Socket's database; audit records show `sources_agreed: ["bumblebee"]` never `["bumblebee","socket"]`.

### Pitfall 3: crates.io API Requires User-Agent Header

**What goes wrong:** Registry timestamp lookup for Cargo packages returns HTTP 403.

**Why it happens:** crates.io enforces a `User-Agent` header policy. Requests without it are rejected. The User-Agent must contain contact info per crates.io terms.

**How to avoid:** Set `User-Agent: beekeeper/<version> (contact: https://github.com/mzansi-agentive/beekeeper)` on all crates.io API requests.

**Warning signs:** `cargo add` tool calls produce "publish timestamp unavailable" failures even when the package is valid.

### Pitfall 4: Corroboration Count Using Number of Entries Instead of Unique Sources

**What goes wrong:** If Bumblebee catalog has 5 entries for a package (from 5 separate catalog files), the corroboration count is reported as 5, triggering a block+quarantine on a single-source basis.

**Why it happens:** Naive implementation counts `len(matches)` instead of `len(unique(matches[i].CatalogSource))`.

**How to avoid:** Corroboration counter must deduplicate by `CatalogSource` string value before counting. Add a unit test: build 5 `CatalogMatch` structs all with `CatalogSource: "bumblebee"` and assert corroboration count == 1.

**Warning signs:** `corroboration_count` > 3 in audit records for packages that should be single-source.

### Pitfall 5: OSV Zip Ecosystem Database Sizes

**What goes wrong:** `beekeeper catalogs sync` takes 30+ minutes to download OSV DB for all ecosystems; user abandons or runs it repeatedly.

**Why it happens:** OSV ecosystem zip sizes vary significantly. npm and PyPI in particular are large (hundreds of MB each). Downloading all 43 ecosystems is impractical.

**How to avoid:** Default to only the 6 ecosystems specified in CONTEXT.md (npm, pypi, go, cargo, rubygems, packagist). Make ecosystem list configurable. Add progress output to `catalogs sync` so user sees per-ecosystem progress. Use `If-Modified-Since` or `ETag` caching for re-syncs.

**Warning signs:** `catalogs sync` appears to hang with no output; disk usage spike of >1GB in `~/.beekeeper/catalogs/osv/`.

### Pitfall 6: Packagist API Version Timestamps Are Per-Package Object, Not Per-Version

**What goes wrong:** Packagist `p2` endpoint returns a large JSON with all versions' metadata nested differently than npm.

**Why it happens:** Packagist's `repo.packagist.org/p2/<vendor>/<package>.json` returns an array of version objects each with a `time` field, but the format differs from other registries.

**How to avoid:** Parse the Packagist response as `{"packages":{"vendor/name":[{"version":"x","time":"..."}]}}`. Iterate the version array to find the exact version match.

**Warning signs:** Packagist release-age check always returns "timestamp unavailable"; Composer packages always trigger fail-closed blocks.

### Pitfall 7: `catalogs watch` On Windows With SIGHUP

**What goes wrong:** Sending SIGHUP to force immediate re-sync on Windows silently fails; the signal is ignored.

**Why it happens:** SIGHUP is a Unix signal. Windows does not support it. `signal.Notify(ch, syscall.SIGHUP)` may compile but never fires on Windows.

**How to avoid:** Use a build-tag-guarded SIGHUP handler: `//go:build !windows`. On Windows, document that "immediate re-sync is triggered by restarting `beekeeper catalogs watch`". Alternatively, use a no-op implementation on Windows.

**Warning signs:** Attempting to send SIGHUP on Windows produces no visible error but no re-sync occurs.

### Pitfall 8: Baseline File Permissions on Windows

**What goes wrong:** `~/.beekeeper/baselines/<project-hash>.json` is world-readable on Windows after creation with standard `os.WriteFile`.

**Why it happens:** Same Windows DACL issue as the audit log (Phase 1 Pitfall 5). `os.Create` inherits parent directory permissions.

**How to avoid:** Apply `platform.SetOwnerOnly(path)` after writing baseline files, same pattern as audit log. [CITED: Phase 1 Pitfall 5 + `hectane/go-acl`]

---

## Code Examples

### OSV API Query (Verified Working)

```go
// Source: [VERIFIED: live api.osv.dev/v1/query test, 2026-05-26]

type osvQuery struct {
    Package osvPkg `json:"package"`
    Version string `json:"version"`
}
type osvPkg struct {
    Name      string `json:"name"`
    Ecosystem string `json:"ecosystem"` // CASE-SENSITIVE
}

var osvEcosystemMap = map[string]string{
    "npm":       "npm",
    "pypi":      "PyPI",
    "go":        "Go",
    "cargo":     "crates.io",  // NOT "Cargo"
    "rubygems":  "RubyGems",
    "packagist": "Packagist",
}

func queryOSV(ctx context.Context, ecosystem, pkg, version string) ([]OSVVuln, error) {
    osvEco, ok := osvEcosystemMap[ecosystem]
    if !ok {
        return nil, nil // unsupported ecosystem
    }
    body, _ := json.Marshal(osvQuery{Package: osvPkg{Name: pkg, Ecosystem: osvEco}, Version: version})
    req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.osv.dev/v1/query", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var out struct{ Vulns []OSVVuln `json:"vulns"` }
    _ = json.NewDecoder(resp.Body).Decode(&out)
    return out.Vulns, nil
}
```

### npm Registry Publish Timestamp (Verified Working)

```go
// Source: [VERIFIED: registry.npmjs.org/lodash returning version timestamps, 2026-05-26]

func npmPublishTime(ctx context.Context, pkg, version string) (time.Time, error) {
    // Use the full-metadata endpoint — `time` object has per-version timestamps
    url := "https://registry.npmjs.org/" + url.PathEscape(pkg)
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Accept", "application/json")
    resp, err := http.DefaultClient.Do(req)
    // ...
    var meta struct {
        Time map[string]string `json:"time"` // version → "2020-08-13T16:53:54.152Z"
    }
    json.NewDecoder(resp.Body).Decode(&meta)
    ts, ok := meta.Time[version]
    if !ok {
        return time.Time{}, errors.New("version timestamp not found")
    }
    return time.Parse(time.RFC3339Nano, ts)
}
```

### Shannon Entropy (8 lines, stdlib only)

```go
// Source: [ASSUMED] — standard Shannon entropy implementation using math.Log2

import "math"

// Entropy returns the Shannon entropy (bits per symbol) of data.
// High entropy (> 4.5 bits/byte) indicates compressed, encrypted, or base64 data.
func Entropy(data []byte) float64 {
    if len(data) == 0 {
        return 0
    }
    freq := make([]int, 256)
    for _, b := range data {
        freq[b]++
    }
    var h float64
    n := float64(len(data))
    for _, c := range freq {
        if c > 0 {
            p := float64(c) / n
            h -= p * math.Log2(p)
        }
    }
    return h
}
```

### state.json Schema for Degraded Mode

```go
// Source: [ASSUMED] — design from CONTEXT.md catalog sanity bounds decision

// internal/state/state.go
type CatalogState struct {
    DegradedMode   bool      `json:"degraded_mode"`
    DegradedSince  time.Time `json:"degraded_since,omitempty"`
    DegradedReason string    `json:"degraded_reason,omitempty"`
    LastEntryCount int       `json:"last_entry_count"`
    LastSyncHash   string    `json:"last_sync_hash"` // fingerprint for delta detection
    LastSyncTime   time.Time `json:"last_sync_time"`
}

type State struct {
    Catalogs map[string]CatalogState `json:"catalogs"` // key: catalog source name
}
```

---

## Runtime State Inventory

This is not a rename/refactor phase. No existing runtime state needs migration. However, Phase 2 introduces NEW state categories that the planner must account for as separate tasks (not just code edits):

| Category | New State Introduced | Task Type |
|----------|---------------------|-----------|
| Disk cache directories | `~/.beekeeper/catalogs/osv-cache/`, `socket-cache/`, `age-cache/` | Created on first use by adapter code; `beekeeper init` should create them |
| OSV database files | `~/.beekeeper/catalogs/osv/<Ecosystem>/all.zip` | Downloaded by `catalogs sync`; first sync for new users |
| Baseline counter files | `~/.beekeeper/baselines/<project-hash>.json` | Created on first allow/warn decision per project |
| state.json | `~/.beekeeper/state.json` (new file) | Created by catalog watch/sync; may not exist on Phase 1 installations |
| Socket API token | Config field `socket.api_token` in `~/.beekeeper/config.json` | User must add manually; `beekeeper init` should prompt |

**Nothing found requiring data migration from Phase 1 state** — Phase 1 creates only `bumblebee.idx`, `bumblebee.json`, and `beekeeper.ndjson`. All Phase 2 state is new.

---

## Open Questions (RESOLVED)

1. **Socket API token distribution model** — RESOLVED: Require individual user registration. Socket source is opt-in (disabled when `socket.api_token` is absent in config). `beekeeper init` prompts for token but does not require it. No shared project token. This decision is locked in CONTEXT.md (CTLG-03 section: "If Socket token is absent or empty: treat Socket source as disabled").

2. **OSV offline DB vs. REST API for CTLG-02** — RESOLVED: Use the public OSV REST API (`POST https://api.osv.dev/v1/query`, no auth). The `github.com/google/osv-scanner/v2` library's `localmatcher` package is internal and not importable; `DoScan()` requires lockfile input, not single-package queries. CONTEXT.md locked decision (OSV Database Integration section) confirms the REST API approach with 24h TTL cache. REQUIREMENTS.md CTLG-02 has been updated to reflect this.

3. **Config schema extension for Phase 2** — RESOLVED: Extend the single-level user `config.json` only (consistent with Phase 1). Full layered merge (CODE-05) is deferred to Phase 9 per ROADMAP. All Phase 2 config fields (socket token, corroboration thresholds, release-age settings, egress allowlists, baseline config) are added to `internal/config/config.go`.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `api.osv.dev/v1/query` | CTLG-02 (REST API) | ✓ (public, no auth) | — | Offline OSV zip DB (complex) |
| `api.socket.dev/v0/purl` | CTLG-03 | ✓ (requires token) | — | Degrade Socket source to disabled |
| Go toolchain | All builds | ✓ (Phase 1 verified) | 1.25 | — |
| registry.npmjs.org | PLCY-02 npm timestamps | ✓ (public) | — | Fail-closed (block) |
| pypi.org API | PLCY-02 PyPI timestamps | ✓ (public) | — | Fail-closed (block) |
| crates.io API | PLCY-02 Cargo timestamps | ✓ (public, needs User-Agent) | — | Fail-closed (block) |
| rubygems.org API | PLCY-02 RubyGems timestamps | ✓ (public) | — | Fail-closed (block) |
| packagist.org API | PLCY-02 Packagist timestamps | ✓ (public) | — | Fail-closed (block) |
| proxy.golang.org | PLCY-02 Go module timestamps | ✓ (public) | — | Fail-closed (block) |
| fsnotify v1.10.1 | CTLG-06 catalogs watch | ✓ (via `go get`) | v1.10.1 | — |

**Missing dependencies with no fallback:** None (all critical dependencies either public or gracefully degradable).

**Key constraint:** Socket API token is user-supplied; without it, Socket source is disabled. This means corroboration with only Bumblebee + OSV (2 sources) is still possible for blocks, but Socket's supply chain insights are unavailable. This reduces detection coverage but does not break the system.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + `go test` |
| Config file | None — `go test ./...` discovers tests automatically |
| Quick run command | `go test ./internal/... -count=1` |
| Full suite command | `go test -race -count=1 ./...` (CI-only; requires CGO) |
| Fuzz run command | `go test ./internal/policy/... -fuzz=FuzzPolicyEvaluate -fuzztime=30s` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLCY-01 | 1 source → warn, 2 sources → block, 3 → block+quarantine | unit | `go test ./internal/policy/... -run TestCorroboration -v` | ❌ Wave 0 |
| PLCY-01 | Unsigned sources count 0.5; two unsigned still only warn | unit | `go test ./internal/policy/... -run TestUnsignedCorroboration` | ❌ Wave 0 |
| PLCY-01 | Same source in two catalog files = 1 independent source | unit | `go test ./internal/policy/... -run TestSameSourceDedup` | ❌ Wave 0 |
| PLCY-02 | Package younger than threshold → block with reason | unit | `go test ./internal/policy/... -run TestReleaseAgeBlock` | ❌ Wave 0 |
| PLCY-02 | Timestamp unavailable → fail-closed block | unit | `go test ./internal/policy/... -run TestReleaseAgeFailClosed` | ❌ Wave 0 |
| PLCY-03 | Package with lifecycle script + not in allowlist → block | unit | `go test ./internal/policy/... -run TestLifecycleScriptBlock` | ❌ Wave 0 |
| PLCY-04 | Tool call targeting `~/.ssh/id_rsa` → block | unit | `go test ./internal/policy/... -run TestSensitivePathBlock` | ❌ Wave 0 |
| PLCY-05 | Outbound to pastebin.com → block | unit | `go test ./internal/policy/... -run TestEgressBlock` | ❌ Wave 0 |
| PLCY-06 | High entropy output → warn decision | unit | `go test ./internal/baseline/... -run TestEntropyDetection` | ❌ Wave 0 |
| PLCY-07 | Baseline deviation > 3σ → warn | unit | `go test ./internal/baseline/... -run TestBaselineDeviation` | ❌ Wave 0 |
| PLCY-08 | Tool output with `AKIA*` pattern → redacted in returned output | unit | `go test ./internal/filter/... -run TestCredentialRedact` | ❌ Wave 0 |
| CTLG-02 | OSV adapter returns matches for known-vulnerable package+version | unit (httptest) | `go test ./internal/catalog/osv/... -run TestOSVLookup` | ❌ Wave 0 |
| CTLG-03 | Socket adapter handles 401 (no token) → returns nil matches | unit | `go test ./internal/catalog/socket/... -run TestSocketNoToken` | ❌ Wave 0 |
| CTLG-03 | Socket adapter respects 24h TTL cache | unit | `go test ./internal/catalog/socket/... -run TestSocketCacheHit` | ❌ Wave 0 |
| CTLG-06 | `catalogs watch` exits cleanly on SIGTERM | integration | `go test ./internal/... -run TestWatchShutdown` | ❌ Wave 0 |
| CTLG-08 | Delta > 10000 entries → source degraded in state.json | unit | `go test ./internal/catalog/... -run TestSanityBoundsHardLimit` | ❌ Wave 0 |
| CTLG-09 | Audit record includes catalog_matches with provenance | unit | `go test ./internal/audit/... -run TestProvenanceFields` | ❌ Wave 0 |
| CTLG-02 | FuzzCatalogParser — must not panic on arbitrary input | fuzz | `go test ./internal/catalog/... -fuzz=FuzzCatalogParser -fuzztime=30s` | ❌ Wave 0 |
| PLCY-01 | FuzzPolicyEvaluate — must not panic on arbitrary tool call | fuzz | `go test ./internal/policy/... -fuzz=FuzzPolicyEvaluate -fuzztime=30s` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/... -count=1` (< 30 seconds on Windows)
- **Per wave merge:** `go test -race -count=1 ./...` (CI-only — race detector requires CGO)
- **Phase gate (fuzz):** `go test -fuzz=FuzzPolicyEvaluate -fuzztime=30s` + `go test -fuzz=FuzzCatalogParser -fuzztime=30s` in CI release job
- **Phase gate (full):** Full test suite green on all 3 platforms before `/gsd-verify-work`

### Wave 0 Gaps (must be created before implementation)
- [ ] `internal/policy/engine_test.go` — extended: TestCorroboration, TestUnsignedCorroboration, TestSameSourceDedup, TestReleaseAgeBlock, TestReleaseAgeFailClosed, TestLifecycleScriptBlock, TestSensitivePathBlock, TestEgressBlock
- [ ] `internal/policy/fuzz_test.go` — FuzzPolicyEvaluate
- [ ] `internal/catalog/osv/adapter_test.go` — TestOSVLookup (httptest.Server stub)
- [ ] `internal/catalog/socket/adapter_test.go` — TestSocketNoToken, TestSocketCacheHit
- [ ] `internal/catalog/fuzz_test.go` — FuzzCatalogParser
- [ ] `internal/baseline/engine_test.go` — TestEntropyDetection, TestBaselineDeviation
- [ ] `internal/filter/credential_test.go` — TestCredentialRedact
- [ ] `internal/audit/types_test.go` — TestProvenanceFields
- [ ] `internal/state/state_test.go` — TestSanityBoundsHardLimit, TestDegradedModeRoundtrip

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Single-user local binary |
| V3 Session Management | No | Stateless; baseline counters not session data |
| V4 Access Control | Yes — baseline files, cache files | `0600` Unix + DACL Windows via `hectane/go-acl` (same as audit log) |
| V5 Input Validation | Yes — OSV/Socket API responses, registry responses | `json.Decoder` strict parsing; bounds checking on entry counts (CTLG-08) |
| V6 Cryptography | No new crypto — catalog signature verification already Phase 1 | — |
| V7 Error Handling | Yes — all new I/O paths | Fail-closed on registry unavailability (PLCY-02); degrade-not-fail for Socket |

### Known Threat Patterns for Phase 2 Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malicious OSV API response inflating vuln list | Tampering | Rate-limit trust: OSV hit alone → warn only; response bounds checked |
| Socket catalog poisoning (compromised Socket source) | Tampering | Corroboration semantics: Socket alone → warn only; 2+ sources for block |
| Coordinated false-positive poisoning (all 3 sources simultaneously) | Tampering | Explicitly documented threat in PRD §9; configurable thresholds; user reviews degraded mode |
| Baseline file tampering to suppress deviation warnings | Tampering | `0600` file permissions; deviation is warn-not-block so manipulation aids attacker not defender |
| Socket API token exfiltration from config.json | Information Disclosure | Audit log must NOT log the Socket token; config file has `0600` permissions |
| OSV cache poisoning (replacing cache files) | Tampering | Cache files are read-only after write; agent that can write `~/.beekeeper/` has game over already |
| Catalog sanity bound bypass via slow drip | Tampering | Per-sync delta bounds only; cumulative count check also in CTLG-08 |
| Multi-turn credential accumulation via base64 | Exfiltration | Rolling window base64 detection in PLCY-06 baseline engine |

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| OSV query via CLI subprocess | OSV REST API (`api.osv.dev/v1/query`) | Phase 2 — simpler than library | No GB of downloads; no subprocess; public API |
| Socket "no auth needed" assumption | Socket requires Bearer token (500 quota/hr free tier) | Discovered 2026-05-26 via live test | Users must register; Beekeeper must degrade gracefully without token |
| Catalog from single source | Corroboration from N sources; 2FA analogy | Phase 2 design | False positive rate decreases dramatically |
| Lockfile scanning for OSV | Per-package-per-call REST API query | Phase 2 design | No lockfile required; integrates at tool-call layer |

**Deprecated/outdated:**
- `DoScan()` with synthetic lockfiles: not recommended when REST API is available and simpler
- `localmatcher` import: not possible (internal package); do not reference in any plan task

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Socket `v0/purl` endpoint (marked deprecated since 2026-01-05, removal 2026-07-30) will remain available through Phase 2 execution | Pattern 3 / Socket API | If removed before Phase 2 completes, must use new Socket packages endpoint (`/v0/packages`) |
| A2 | OSV REST API `api.osv.dev/v1/query` has no authentication requirement and no rate limiting for reasonable single-package queries | Pattern 2 / OSV | If rate limits are introduced, need to add caching and backoff (recommended anyway) |
| A3 | Socket free tier 500 quota/hour is sufficient for development-rate tool calls with 24h cache | Socket design | If quota is per-call not per-unique-package, high-velocity dev sessions could exhaust quota |
| A4 | crates.io requires `User-Agent` with contact info; using a proper user agent will not be rate-limited | Pitfall 3 | If crates.io further restricts access, Cargo timestamp checks will fail |
| A5 | `osv-vulnerabilities.storage.googleapis.com/<Ecosystem>/all.zip` format is stable for npm, PyPI, Go, crates.io, RubyGems, Packagist | CTLG-02 (offline path) | If GCS bucket structure changes, offline sync breaks |
| A6 | fsnotify v1.10.1 Create events on Windows correctly detect new `.zip` files in `~/.beekeeper/catalogs/osv/` without junction-point interference | CTLG-06 | If junction points interfere, fall back to polling-only approach in `catalogs watch` |

**If these assumptions are wrong:** A1 requires Socket endpoint update (breaking change, track deprecation). A2-A4 require adding backoff/retry (non-breaking). A5 requires offline DB sync logic update. A6 requires removing fsnotify from `catalogs watch` and using poll-only.

---

## Project Constraints (from CLAUDE.md)

The following CLAUDE.md directives constrain Phase 2 planning and implementation:

- `internal/policy` must remain a pure function library — no I/O, no goroutines, no side effects. All network calls and file reads are in adapter packages before calling `policy.Evaluate`.
- Single static binary, CGO-free. All new packages (`internal/catalog/osv`, `internal/catalog/socket`, `internal/registry`, `internal/baseline`, `internal/filter`, `internal/state`) must use pure Go.
- Fail closed by default. Registry timestamp unavailability → fail closed (block). Socket unavailability → degrade (catalog source disabled) because Socket is optional corroboration, not a required gate.
- Windows primary dev machine. All new code must compile on Windows without WSL. No `syscall.SIGHUP` without `//go:build !windows` guard.
- Fuzz testing in CI: Phase 2 is explicitly when fuzz tests become release-gating (CLAUDE.md "Phase 2: Corroboration sanity bounds + catalog signature verification" and PRD §12.4). `FuzzPolicyEvaluate` and `FuzzCatalogParser` must be in CI release gate by end of Phase 2.
- No WSL integration tests — any network-dependent tests must use `httptest.Server` stubs, not live API calls, for local dev.

---

## Sources

### Primary (HIGH confidence)
- `api.osv.dev/v1/query` — verified via live POST request returning lodash vulnerabilities (2026-05-26)
- `storage.googleapis.com/osv-vulnerabilities/ecosystems.txt` — verified 43 ecosystems list (2026-05-26)
- `registry.npmjs.org/lodash` — verified `time["4.17.20"]` publish timestamp format (2026-05-26)
- `pypi.org/pypi/requests/json` — verified `upload_time_iso_8601` field structure (2026-05-26)
- `crates.io/api/v1/crates/serde/1.0.0` — verified `created_at` field with User-Agent header (2026-05-26)
- `rubygems.org/api/v2/rubygems/rails/versions/7.0.0.json` — verified `version_created_at` field (2026-05-26)
- `repo.packagist.org/p2/laravel/framework.json` — verified per-version `time` field (2026-05-26)
- `proxy.golang.org/github.com/google/osv-scanner/v2/@v/v2.3.8.info` — verified `Time` field format (2026-05-26)
- `proxy.golang.org/github.com/fsnotify/fsnotify/@latest` — verified v1.10.1 (2026-05-26)
- `api.socket.dev/v0/purl` live test — verified authentication required; returns 401 Unauthorized without Bearer token (2026-05-26)
- `socket.dev/pricing` — verified free tier: 500 API quota/hour, 1 API token (2026-05-26)
- `go.dev/doc/security/fuzz/` — verified fuzz test patterns: `FuzzXxx(*testing.F)`, corpus format, CI integration (fetched 2026-05-26)
- `pkg.go.dev/github.com/google/osv-scanner/v2/pkg/osvscanner#ScannerActions` — verified `LocalDBPath`, `CompareOffline`, `DownloadDatabases` fields; `localmatcher` is internal (2026-05-26)
- Phase 1 implementation files — `internal/policy/types.go`, `engine.go`, `internal/catalog/index.go`, `internal/audit/types.go` — verified existing interfaces (2026-05-26)

### Secondary (MEDIUM confidence)
- `google.github.io/osv.dev/post-v1-query/` — OSV API request/response format; case-sensitivity documented
- `docs.socket.dev/reference/batchpackagefetch` — Socket PURL endpoint auth requirement; batch size limit
- `docs.socket.dev/reference/rate-limits` — rate limit 600/minute (endpoint-level)
- `docs.socket.dev/reference/quota` — per-token quota model; 429 + Retry-After pattern

### Tertiary (LOW confidence / ASSUMED)
- OSV ecosystem string casing (`crates.io` not `cargo`) — inferred from OSV data at storage.googleapis.com; not independently verified against osv.dev ecosystem docs
- Socket PURL request body `{"components":[{"purl":"..."}]}` format — inferred from deprecation docs; exact format for active replacement endpoint not verified
- Shannon entropy threshold of 4.5 bits/byte for exfiltration detection — standard security literature value; not OSS project-specific

---

## Metadata

**Confidence breakdown:**
- Standard stack (stdlib + fsnotify): HIGH — verified versions via proxy.golang.org
- Registry timestamp APIs (all 6): HIGH — verified via live HTTP calls
- OSV REST API: HIGH — verified via live API call with real response
- Socket API authentication requirement: HIGH — verified via live test (401 Unauthorized)
- Socket free tier quota: HIGH — verified via socket.dev/pricing page
- Architecture patterns (multi-source adapter, pure policy extension): MEDIUM — design from CONTEXT.md + Phase 1 patterns; not yet implemented
- Corroboration logic: MEDIUM — designed from CONTEXT.md; implementation details are Claude's discretion
- Fuzz testing integration: MEDIUM — patterns from official docs; specific CI gate integration is Claude's discretion
- OSV ecosystem name mapping: MEDIUM — partially verified; `crates.io` ecosystem name is LOW confidence

**Research date:** 2026-05-26
**Valid until:** 2026-06-25 (30 days; registry APIs are stable; Socket API v0/purl deprecation deadline 2026-07-30 is within window — monitor)

**CRITICAL ALERT for planner:** The Socket API `v0/purl` endpoint is marked deprecated since 2026-01-05 with removal announced for 2026-07-30. Phase 2 planning should use this endpoint while available but include a note to migrate to the replacement endpoint before the July 30 deadline. The replacement appears to be `POST /v0/packages` (batch packages endpoint) based on Socket docs.
