# Phase 2: Policy Engine + Multi-Source Catalogs - Pattern Map

**Mapped:** 2026-05-26
**Files analyzed:** 14 new/modified files
**Analogs found:** 14 / 14

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/policy/types.go` | model | request-response | `internal/policy/types.go` (extend) | exact |
| `internal/policy/engine.go` | service (pure) | request-response | `internal/policy/engine.go` (extend) | exact |
| `internal/policy/release_age.go` | service (pure) | request-response | `internal/policy/engine.go` | exact |
| `internal/policy/lifecycle.go` | service (pure) | request-response | `internal/policy/engine.go` | exact |
| `internal/policy/path.go` | service (pure) | request-response | `internal/policy/engine.go` | exact |
| `internal/policy/egress.go` | service (pure) | request-response | `internal/policy/engine.go` | exact |
| `internal/policy/exfil.go` | service (pure) | request-response | `internal/policy/engine.go` | exact |
| `internal/policy/baseline.go` | service (pure) | request-response | `internal/policy/engine.go` | exact |
| `internal/policy/credentials.go` | service (pure) | transform | `internal/policy/engine.go` | role-match |
| `internal/catalog/osv.go` | service | request-response | `internal/catalog/sync.go` | exact |
| `internal/catalog/socket.go` | service | request-response | `internal/catalog/sync.go` | exact |
| `internal/catalog/age_cache.go` | service | CRUD | `internal/catalog/sync.go` | role-match |
| `internal/catalog/registry.go` | service | request-response | `internal/catalog/sync.go` | role-match |
| `internal/catalog/watch.go` | service | event-driven | `internal/catalog/sync.go` | role-match |
| `internal/catalog/sanity.go` | utility | transform | `internal/catalog/loader.go` | role-match |
| `internal/baseline/store.go` | service | CRUD | `internal/audit/writer.go` | role-match |
| `internal/audit/types.go` | model | request-response | `internal/audit/types.go` (extend) | exact |
| `cmd/beekeeper/main.go` | config | request-response | `cmd/beekeeper/main.go` (extend) | exact |

---

## Pattern Assignments

### `internal/policy/types.go` (model — extend existing)

**Analog:** `internal/policy/types.go` (lines 1–59)

**Current struct shapes to extend:**

```go
// CatalogMatch — add Corroborated and Dissented booleans (CTLG-09)
type CatalogMatch struct {
    CatalogSource string
    EntryID       string
    Ecosystem     string
    Package       string
    Version       string
    Severity      string
    Signed        bool   // present in Phase 1
    // Phase 2 additions:
    Corroborated  bool   // true when this source contributed to a block/quarantine
    Dissented     bool   // true when this source disagreed with corroboration
    CatalogVersion string // hash or timestamp of the catalog at evaluation time
}

// Decision — add corroboration fields (CTLG-09)
type Decision struct {
    Allow              bool
    Level              string
    Reason             string
    RuleIDs            []string
    CatalogMatches     []CatalogMatch
    // Phase 2 additions:
    CorroborationCount int      // number of independent sources that matched
    SourcesAgreed      []string // e.g. ["bumblebee", "osv"]
    SourcesDissented   []string // e.g. []
    Quarantine         bool     // true when 3+ sources agree
}
```

**CatalogLookup interface — extend for multi-source (lines 57–59):**

```go
// Phase 1 (single-source, keep for backward compat):
type CatalogLookup interface {
    Lookup(ecosystem, pkg string) (catalog.Entry, bool)
}

// Phase 2 — new multi-source interface:
// MultiCatalogLookup wraps multiple CatalogLookup implementations and
// returns matches from ALL sources with their provenance. Pure; no I/O.
type MultiCatalogLookup interface {
    LookupAll(ecosystem, pkg string) []CatalogMatch
}
```

---

### `internal/policy/engine.go` (service, pure — extend existing)

**Analog:** `internal/policy/engine.go` (full file, lines 1–189)

**Package declaration + import constraint pattern** (lines 1–7):
```go
package policy

import (
    "strings"
    // Phase 2 MAY add: "math" for entropy calculation in exfil.go
    // MUST NOT add: os, net, net/http, io, sync, time, context
    "github.com/mzansi-agentive/beekeeper/internal/catalog"
)
```

**Purity enforcement test pattern** (`engine_test.go` lines 295–328) — replicate in every new `internal/policy/*.go` file's test with the same forbidden-import list:
```go
forbidden := map[string]bool{
    "os": true, "net": true, "net/http": true,
    "io": true, "sync": true, "time": true, "context": true,
}
```

**Evaluate function signature — extend, not replace** (line 35):
```go
// Phase 2: Evaluate receives a MultiCatalogLookup (wrapping all sources) instead
// of the single CatalogLookup. The concrete *catalog.MultiIndex satisfies both.
func Evaluate(tc ToolCall, idx MultiCatalogLookup) Decision {
```

**Corroboration logic pattern (new for Phase 2):**
```go
// corroborate counts independent source hits and escalates level.
// "Independent" = distinct CatalogSource values. Same source in two files = 1.
// Unsigned sources count as 0.5 (require at least one signed for enforcement).
func corroborate(matches []CatalogMatch, thresholds CorroborationThresholds) (level string, quarantine bool) {
    signedSources := map[string]bool{}
    unsignedSources := map[string]bool{}
    for _, m := range matches {
        if m.Signed {
            signedSources[m.CatalogSource] = true
        } else {
            unsignedSources[m.CatalogSource] = true
        }
    }
    signedCount := len(signedSources)
    // Unsigned count as 0.5 — require at least one signed for enforcement.
    hasSignedSource := signedCount >= 1
    totalWeight := signedCount // unsigned: warn-only (0.5 weight, never block alone)

    switch {
    case totalWeight >= thresholds.QuarantineAt && hasSignedSource:
        return "block", true
    case totalWeight >= thresholds.BlockAt && hasSignedSource:
        return "block", false
    case totalWeight >= thresholds.WarnAt || len(unsignedSources) > 0:
        return "warn", false
    default:
        return "allow", false
    }
}
```

**Rule ID constant pattern** (line 11):
```go
const (
    ruleBumblebeeCatalogMatch = "bumblebee-catalog-match"
    // Phase 2 additions — one constant per rule file:
    ruleOSVCatalogMatch   = "osv-catalog-match"
    ruleSocketCatalogMatch = "socket-catalog-match"
    ruleReleaseAge         = "release-age-policy"
    ruleLifecycleScript    = "lifecycle-script-policy"
    ruleSensitivePath      = "sensitive-path-policy"
    ruleNetworkEgress      = "network-egress-policy"
    ruleExfiltration       = "multi-turn-exfiltration"
    ruleBaselineAnomaly    = "baseline-anomaly"
    ruleCredentialOutput   = "credential-output-filter"
)
```

**Decision construction pattern** (lines 68–74) — all rule files follow this shape:
```go
return Decision{
    Allow:          false,          // or true for warn-level
    Level:          "block",        // "allow" | "warn" | "block"
    Reason:         "reason text: " + detail,
    RuleIDs:        []string{ruleXxx},
    CatalogMatches: matches,        // []CatalogMatch, may be nil/empty
    // Phase 2:
    CorroborationCount: count,
    SourcesAgreed:      agreed,
    SourcesDissented:   dissented,
    Quarantine:         quarantine,
}
```

---

### `internal/policy/release_age.go` (service, pure)

**Analog:** `internal/policy/engine.go` — pure function, same package, same constraints.

**Function signature pattern:**
```go
package policy

// ReleaseAgeInput carries the caller-resolved publish timestamp for a package.
// The I/O adapter (internal/catalog/age_cache.go) fetches and caches it; the
// policy function receives only the resolved duration (pure, no I/O).
type ReleaseAgeInput struct {
    Ecosystem        string
    Package          string
    AgeMinutes       int64  // time.Since(publishedAt).Minutes() — computed by caller
    TimestampMissing bool   // true when registry returned no data (fail closed)
}

// EvaluateReleaseAge is a pure function: given resolved age data and
// per-ecosystem thresholds, it returns a Decision. No I/O. No time.Now().
func EvaluateReleaseAge(input ReleaseAgeInput, cfg ReleaseAgeConfig) Decision {
    if input.TimestampMissing {
        return Decision{
            Allow:   false,
            Level:   "block",
            Reason:  "publish timestamp unavailable (fail-closed)",
            RuleIDs: []string{ruleReleaseAge},
        }
    }
    // ...threshold comparison...
}
```

---

### `internal/policy/lifecycle.go` (service, pure)

**Analog:** `internal/policy/engine.go` — pure function.

**Function signature pattern:**
```go
package policy

// LifecycleInput carries the caller-resolved script fields for a package.
// The I/O adapter (internal/catalog/registry.go) fetches registry metadata;
// the policy function receives pre-resolved booleans.
type LifecycleInput struct {
    Ecosystem          string
    Package            string
    ScriptsPresent     []string // e.g. ["preinstall", "postinstall"]
    RegistryCheckFailed bool    // true when registry fetch failed (fail closed)
}

// EvaluateLifecycle returns block if any lifecycle script is present and not
// in the allowlist. Pure function; no I/O.
func EvaluateLifecycle(input LifecycleInput, allowlist []string) Decision {
    if input.RegistryCheckFailed {
        return Decision{
            Allow:   false,
            Level:   "block",
            Reason:  "lifecycle script check unavailable (fail-closed)",
            RuleIDs: []string{ruleLifecycleScript},
        }
    }
    // ...allowlist check...
}
```

---

### `internal/policy/path.go` (service, pure)

**Analog:** `internal/policy/engine.go` — pure function; uses `strings` for path prefix matching.

**Function signature pattern:**
```go
package policy

import "strings"

// SensitivePathConfig holds the default + user-extended blocklist and allowlist.
type SensitivePathConfig struct {
    BlockPatterns    []string // prefix patterns; "~" already resolved by caller
    AllowPatterns    []string
}

// EvaluatePath returns block if resolvedPath matches any block pattern and is
// not explicitly allowed. resolvedPath has "~" already substituted by the
// caller (the platform layer). Pure function; no filepath.Abs, no os.Stat.
func EvaluatePath(resolvedPath string, cfg SensitivePathConfig) Decision {
    // ...
}
```

---

### `internal/policy/egress.go` (service, pure)

**Analog:** `internal/policy/engine.go` — pure function.

**Function signature pattern:**
```go
package policy

// EgressInput carries the caller-resolved outbound request attributes.
type EgressInput struct {
    ToolName    string
    TargetURL   string   // normalized by caller
    PayloadSize int64    // bytes
}

// EvaluateEgress checks the target URL against the per-tool allowlist/blocklist
// and payload size limits. Pure function; no net.Dial, no DNS.
func EvaluateEgress(input EgressInput, cfg EgressConfig) Decision {
    // ...
}
```

---

### `internal/policy/exfil.go` (service, pure)

**Analog:** `internal/policy/engine.go` — pure function; uses `math` for Shannon entropy.

**Function signature pattern:**
```go
package policy

import "math"

// ExfilWindow carries the rolling window of recent tool outputs (pre-collected
// by the caller from the baseline store; pure inputs only).
type ExfilWindow struct {
    Outputs     []string // last N tool outputs
    Base64Bytes int64    // accumulated base64-encoded byte count across outputs
}

// EvaluateExfil computes Shannon entropy over the window and detects base64
// accumulation. Pure function; no I/O, no wall clock.
func EvaluateExfil(window ExfilWindow, cfg ExfilConfig) Decision {
    entropy := shannonEntropy(window.Outputs)
    // ...
}

// shannonEntropy computes Shannon entropy H over the character distribution of
// all outputs concatenated. Uses math.Log2; no I/O.
func shannonEntropy(outputs []string) float64 {
    // freq count, then H = -sum(p * log2(p))
    _ = math.Log2  // ensure import used
    // ...
    return 0
}
```

---

### `internal/policy/baseline.go` (service, pure)

**Analog:** `internal/policy/engine.go` — pure function.

**Function signature pattern:**
```go
package policy

// BaselineCounters is the pure in-memory view of per-project frequency data.
// The I/O layer (internal/baseline/store.go) loads and persists it; the policy
// function receives only the resolved counters.
type BaselineCounters struct {
    // Keyed by "tool_name::target_pattern"
    Counts  map[string][]int64 // each int64 is a Unix timestamp of an occurrence
    WindowDays int             // rolling window size in days
}

// EvaluateBaseline returns warn if the current event's frequency exceeds
// mean + N*stddev over the rolling window. Pure function; no time.Now() —
// caller provides nowUnix.
func EvaluateBaseline(key string, nowUnix int64, counters BaselineCounters, cfg BaselineConfig) Decision {
    // ...
}
```

---

### `internal/policy/credentials.go` (service, pure — transform)

**Analog:** `internal/policy/engine.go` — pure function; uses `strings` and `regexp`.

**Function signature pattern:**
```go
package policy

import "regexp"

// CredentialFilterResult is the output of FilterCredentials.
type CredentialFilterResult struct {
    Redacted             string   // original with credential patterns replaced
    DetectedTypes        []string // e.g. ["aws-access-key", "jwt"]
    ContainsCredentials  bool
}

// FilterCredentials scans output for known credential patterns and returns a
// redacted copy. Pure function; no I/O. Compiled regexps must be package-level
// vars (not compiled per-call).
func FilterCredentials(output string, cfg CredentialFilterConfig) CredentialFilterResult {
    // ...
}

// credPatterns holds compiled regexps. Compiled once at package init, not per call.
var credPatterns = []*credPattern{
    {name: "aws-access-key", re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
    {name: "jwt",            re: regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)},
    // ...
}
```

---

### `internal/catalog/osv.go` (service, I/O — HTTP adapter)

**Analog:** `internal/catalog/sync.go` (full file, lines 1–157)

**Package + imports pattern** (lines 1–12):
```go
package catalog

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "time"
)
```

**HTTP POST with context pattern** (lines 106–131 adapted):
```go
// SyncOSV queries the OSV REST API for a package and version, caches the result
// in <cacheDir>/osv/<ecosystem>/<pkg>/<version>.json with 24h TTL, and returns
// the matching entries as []Entry. Context timeout mirrors sync.go (30s caller).
//
// OSV ecosystem names are CASE-SENSITIVE: npm→"npm", pypi→"PyPI", go→"Go",
// cargo→"crates.io", rubygems→"RubyGems", packagist→"Packagist".
func SyncOSV(ctx context.Context, client *http.Client, cacheDir, ecosystem, pkg, version string) ([]Entry, error) {
    // cache-first: check cacheDir/osv/<key>.json age before network call
    cached, ok, err := readOSVCache(cacheDir, ecosystem, pkg, version)
    if err == nil && ok {
        return cached, nil
    }

    // POST https://api.osv.dev/v1/query
    body := osvQueryBody(ecosystem, pkg, version)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvQueryURL, body)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("osv query: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("osv query: HTTP %d", resp.StatusCode)
    }

    data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
    // ... parse, cache, return entries ...
}
```

**Atomic cache write pattern** — copy `writeFileAtomic` from `index.go` (lines 118–139):
```go
// writeOSVCache atomically writes parsed OSV results to disk.
// Uses same writeFileAtomic helper already in catalog package.
func writeOSVCache(cacheDir, ecosystem, pkg, version string, entries []Entry) error {
    path := osvCachePath(cacheDir, ecosystem, pkg, version)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
        return err
    }
    data, err := json.Marshal(entries)
    if err != nil {
        return err
    }
    return writeFileAtomic(path, data)
}
```

**CatalogLookup implementation pattern** — OSV adapter must satisfy `policy.CatalogLookup` (or the new `MultiCatalogLookup`):
```go
// OSVAdapter implements policy.MultiCatalogLookup by resolving OSV results
// from cache or network. It wraps the I/O; the policy engine sees only the
// returned []policy.CatalogMatch.
type OSVAdapter struct {
    Client   *http.Client
    CacheDir string
}
```

---

### `internal/catalog/socket.go` (service, I/O — HTTP adapter)

**Analog:** `internal/catalog/sync.go` (full file, lines 1–157)

**Bearer token auth pattern** (extends fetch pattern from lines 134–156):
```go
// fetchSocket performs a POST to the Socket PURL API with exponential backoff.
// Token is read from config; if empty, Socket source is disabled (not an error).
//
// Deprecation: v0/purl deprecated 2026-01-05, removal 2026-07-30.
// Phase 2 uses this endpoint; migration to POST /v0/packages planned before removal.
func fetchSocket(ctx context.Context, client *http.Client, token, purlStr string) ([]byte, error) {
    if token == "" {
        return nil, nil // Socket disabled — not an error
    }

    const maxRetries = 5
    backoff := time.Second
    for attempt := 0; attempt <= maxRetries; attempt++ {
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, socketPURLURL, strings.NewReader(purlStr))
        if err != nil {
            return nil, err
        }
        req.Header.Set("Authorization", "Bearer "+token)
        req.Header.Set("Content-Type", "application/json")

        resp, err := client.Do(req)
        if err != nil {
            return nil, fmt.Errorf("socket purl: %w", err)
        }
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
        resp.Body.Close()

        if resp.StatusCode == http.StatusTooManyRequests {
            // Respect Retry-After; fall back to exponential backoff.
            if ra := resp.Header.Get("Retry-After"); ra != "" {
                // parse ra as seconds
            }
            if backoff > 60*time.Second {
                return nil, fmt.Errorf("socket purl: rate limit exceeded after %d retries", maxRetries)
            }
            // sleep(backoff); backoff *= 2
            continue
        }
        if resp.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("socket purl: HTTP %d", resp.StatusCode)
        }
        return body, nil
    }
    return nil, fmt.Errorf("socket purl: max retries exceeded")
}
```

**Degraded-mode pattern** — Socket unavailable degrades to warn-only, never blocks:
```go
// QuerySocket returns (entries, degraded, error).
// degraded=true when Socket is unavailable but the call should continue with
// reduced corroboration rather than fail-closing the whole check.
func QuerySocket(ctx context.Context, client *http.Client, cacheDir, token, ecosystem, pkg, version string) ([]Entry, bool, error) {
    // cache-first (same TTL pattern as osv.go)
    // on network error: return nil, true (degraded), nil — log to caller
    // on token empty: return nil, false, nil — Socket simply disabled
}
```

---

### `internal/catalog/age_cache.go` (service, I/O — CRUD)

**Analog:** `internal/catalog/sync.go` (HTTP fetch + disk cache) + `internal/audit/writer.go` (owner-only perms)

**Cache file pattern:**
```go
package catalog

import (
    "context"
    "encoding/json"
    "net/http"
    "os"
    "path/filepath"
    "time"
)

// ageCacheEntry is the on-disk format stored in <cacheDir>/age-cache/<key>.json.
type ageCacheEntry struct {
    PublishedAt time.Time `json:"published_at"`
    CachedAt    time.Time `json:"cached_at"`
    Missing     bool      `json:"missing"` // true = registry returned no data
}

// FetchPublishAge queries the relevant registry API for a package's publish
// timestamp, caches the result with 24h TTL in cacheDir/age-cache/, and
// returns (ageMinutes, timestampMissing, error).
// Fail closed: if the registry returns an error or no timestamp, missing=true.
func FetchPublishAge(ctx context.Context, client *http.Client, cacheDir, ecosystem, pkg, version string) (ageMinutes int64, missing bool, err error) {
    // cache check: read ageCacheEntry, check CachedAt+24h > now
    // on miss: dispatch to ecosystem-specific registry fetcher
    // on fetch error: return 0, true (missing), nil — caller blocks
    // write cache atomically using writeFileAtomic
}
```

---

### `internal/catalog/registry.go` (service, I/O)

**Analog:** `internal/catalog/sync.go` (HTTP fetch pattern, lines 104–156)

**Pattern — ecosystem dispatch for lifecycle and publish-age queries:**
```go
package catalog

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

// fetchNPMPublishTime returns the publish timestamp for pkg@version from the
// npm registry JSON API: GET https://registry.npmjs.org/<pkg>/<version>
// On any error, returns zero time and an error (caller treats as missing).
func fetchNPMPublishTime(ctx context.Context, client *http.Client, pkg, version string) (string, error) {
    url := "https://registry.npmjs.org/" + pkg + "/" + version
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    // ... same response handling as sync.go fetch() ...
}

// npmHasLifecycleScripts returns the list of lifecycle script keys present in
// pkg's package.json via the npm registry response.
func npmHasLifecycleScripts(ctx context.Context, client *http.Client, pkg, version string) ([]string, error) {
    // same HTTP pattern; parse "scripts" field
}
```

---

### `internal/catalog/watch.go` (service, event-driven)

**Analog:** `cmd/beekeeper/main.go` tailAuditLog (lines 200–242) — foreground loop with context cancellation + `internal/catalog/sync.go` for the sync trigger.

**Daemon loop pattern** (from tailAuditLog lines 214–242):
```go
package catalog

import (
    "context"
    "net/http"
    "time"
)

// WatchConfig controls the catalog watch daemon parameters.
type WatchConfig struct {
    PollInterval time.Duration // default 1h; range 5m–24h
    CatalogDir   string
    StateFile    string        // ~/.beekeeper/state.json for delta state
    Client       *http.Client
}

// Watch runs the catalog watch daemon: polls all enabled catalog sources on
// PollInterval, detects deltas, triggers targeted re-scans, and writes delta
// events to the audit log. Blocks until ctx is cancelled (SIGTERM/SIGINT).
func Watch(ctx context.Context, cfg WatchConfig, onDelta func(delta CatalogDelta)) error {
    ticker := time.NewTicker(cfg.PollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            delta, err := checkDelta(ctx, cfg)
            if err != nil {
                // log to stderr; do not exit — degraded mode
                continue
            }
            if delta.HasChanges() {
                onDelta(delta)
            }
        }
    }
}
```

**State file pattern** (mirroring `config.go` Load/Save):
```go
// CatalogDelta records the before/after state of a catalog sync for audit
// provenance (CTLG-09).
type CatalogDelta struct {
    Source       string
    PrevHash     string
    NewHash      string
    PrevCount    int
    NewCount     int
    DeltaCount   int
}

func (d CatalogDelta) HasChanges() bool { return d.PrevHash != d.NewHash }
```

**Cobra cmd wiring pattern** (from `main.go` newCatalogsCmd, lines 141–172):
```go
// In cmd/beekeeper/main.go newCatalogsCmd(), add:
catalogs.AddCommand(&cobra.Command{
    Use:   "watch",
    Short: "Poll catalog sources and trigger re-scans on delta (Ctrl+C to stop)",
    Args:  cobra.NoArgs,
    RunE: func(cmd *cobra.Command, _ []string) error {
        dir, err := platform.CatalogDir()
        // ... resolve paths ...
        client := &http.Client{Timeout: 30 * time.Second}
        cfg := catalog.WatchConfig{
            PollInterval: time.Hour,
            CatalogDir:   dir,
            Client:       client,
        }
        return catalog.Watch(cmd.Context(), cfg, func(d catalog.CatalogDelta) {
            fmt.Fprintf(cmd.OutOrStdout(), "catalog delta: %+v\n", d)
        })
    },
})
```

---

### `internal/catalog/sanity.go` (utility, transform)

**Analog:** `internal/catalog/loader.go` (lines 1–49) — parse-time validation, same package, returns typed errors.

**Validation function pattern** (mirrors `ValidateSchemaVersion`, lines 17–22):
```go
package catalog

import "fmt"

// SanityConfig holds the configurable alert and hard-block thresholds.
type SanityConfig struct {
    AlertDeltaEntries  int // default 1000
    BlockDeltaEntries  int // default 10000
    AlertTotalEntries  int // default 100000
    AlertVersionsPerPkg int // default 1000
}

// SanityResult is the outcome of a catalog sanity check.
type SanityResult struct {
    Alert   bool   // true when alert threshold exceeded — source → warn-only
    Block   bool   // true when hard threshold exceeded — source → degraded
    Reason  string
}

// CheckSanity validates catalog delta and total sizes against configured
// thresholds. Returns SanityResult; no I/O — all inputs resolved by caller.
func CheckSanity(prevCount, newCount int, cfg SanityConfig) SanityResult {
    delta := newCount - prevCount
    if delta < 0 {
        delta = -delta
    }
    switch {
    case delta > cfg.BlockDeltaEntries:
        return SanityResult{Block: true, Reason: fmt.Sprintf("delta %d exceeds hard limit %d", delta, cfg.BlockDeltaEntries)}
    case delta > cfg.AlertDeltaEntries:
        return SanityResult{Alert: true, Reason: fmt.Sprintf("delta %d exceeds alert threshold %d", delta, cfg.AlertDeltaEntries)}
    case newCount > cfg.AlertTotalEntries:
        return SanityResult{Alert: true, Reason: fmt.Sprintf("total %d exceeds alert threshold %d", newCount, cfg.AlertTotalEntries)}
    default:
        return SanityResult{}
    }
}
```

---

### `internal/baseline/store.go` (service, CRUD — I/O)

**Analog:** `internal/audit/writer.go` (lines 1–75) — owner-only perms, atomic JSON write; `internal/config/config.go` (lines 45–72) — JSON load with missing-file-is-OK pattern.

**Store struct + Load/Save pattern:**
```go
package baseline

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"

    "github.com/mzansi-agentive/beekeeper/internal/platform"
    "github.com/mzansi-agentive/beekeeper/internal/policy"
)

// Store persists per-project behavioral baseline counters to disk.
// File is owner-only (0600) — contains frequency data about developer patterns.
type Store struct {
    path string
}

// NewStore returns a Store backed by path (under ~/.beekeeper/baselines/).
func NewStore(path string) (*Store, error) {
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
        return nil, fmt.Errorf("create baselines directory: %w", err)
    }
    return &Store{path: path}, nil
}

// Load reads the baseline counters. Missing file returns empty counters (normal
// first-run case), mirroring config.Load's missing-file-is-OK pattern.
func (s *Store) Load() (policy.BaselineCounters, error) {
    data, err := os.ReadFile(s.path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return policy.BaselineCounters{Counts: map[string][]int64{}}, nil
        }
        return policy.BaselineCounters{}, fmt.Errorf("read baseline %q: %w", s.path, err)
    }
    var bc policy.BaselineCounters
    if err := json.Unmarshal(data, &bc); err != nil {
        return policy.BaselineCounters{}, fmt.Errorf("parse baseline %q: %w", s.path, err)
    }
    return bc, nil
}

// Save atomically persists counters to disk and enforces owner-only permissions.
// Uses writeFileAtomic pattern (same as catalog/index.go) to avoid partial writes.
func (s *Store) Save(bc policy.BaselineCounters) error {
    data, err := json.Marshal(bc)
    if err != nil {
        return fmt.Errorf("marshal baseline: %w", err)
    }
    if err := writeBaselineAtomic(s.path, data); err != nil {
        return err
    }
    return platform.SetOwnerOnly(s.path)
}
```

**Atomic write** — same pattern as `catalog/index.go` writeFileAtomic (lines 118–139):
```go
func writeBaselineAtomic(path string, data []byte) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
    if err != nil {
        return err
    }
    tmpName := tmp.Name()
    defer os.Remove(tmpName)
    if _, err := tmp.Write(data); err != nil { tmp.Close(); return err }
    if err := tmp.Sync(); err != nil { tmp.Close(); return err }
    if err := tmp.Close(); err != nil { return err }
    return os.Rename(tmpName, path)
}
```

---

### `internal/audit/types.go` (model — extend existing)

**Analog:** `internal/audit/types.go` (full file, lines 1–77) — extend, do not replace.

**CatalogProvenance — add Phase 2 fields** (lines 36–44):
```go
type CatalogProvenance struct {
    CatalogSource  string `json:"catalog_source"`
    EntryID        string `json:"entry_id"`
    Ecosystem      string `json:"ecosystem"`
    Package        string `json:"package"`
    Version        string `json:"version"`
    Severity       string `json:"severity"`
    Signed         bool   `json:"signed"`
    // Phase 2 additions (CTLG-09):
    Corroborated   bool   `json:"corroborated"`
    Dissented      bool   `json:"dissented"`
    CatalogVersion string `json:"catalog_version"`
}
```

**AuditRecord — add Phase 2 corroboration fields** (lines 18–30):
```go
type AuditRecord struct {
    // ... all Phase 1 fields unchanged ...
    // Phase 2 additions (CTLG-09):
    CorroborationCount int      `json:"corroboration_count"`
    SourcesAgreed      []string `json:"sources_agreed"`
    SourcesDissented   []string `json:"sources_dissented"`
    Quarantine         bool     `json:"quarantine,omitempty"`
    // Policy-specific fields for non-catalog decisions:
    ReleaseAgeMinutes  int64    `json:"release_age_minutes,omitempty"`
    CredentialTypes    []string `json:"credential_types,omitempty"`
}
```

**FromDecision mapping — extend** (lines 50–77):
```go
// FromDecision must be extended to map the new Decision fields to AuditRecord.
// The function signature stays identical; the mapping block is extended inline.
func FromDecision(tc policy.ToolCall, d policy.Decision, recordID, timestamp string) AuditRecord {
    // ... existing CatalogMatches mapping ...
    // Add Phase 2 fields:
    rec.CorroborationCount = d.CorroborationCount
    rec.SourcesAgreed = d.SourcesAgreed
    rec.SourcesDissented = d.SourcesDissented
    rec.Quarantine = d.Quarantine
    return rec
}
```

---

## Shared Patterns

### Module Path
**Source:** `go.mod` line 1
**Apply to:** All new files
```go
module github.com/mzansi-agentive/beekeeper
```
All internal imports use `github.com/mzansi-agentive/beekeeper/internal/<pkg>`.

---

### Pure Function Purity Enforcement (CRITICAL)
**Source:** `internal/policy/engine_test.go` lines 295–328
**Apply to:** Every `internal/policy/*.go` file; each must have a corresponding `_test.go` with the forbidden-import test.

```go
// TestXxxImportsArePure — replicate for each new policy file.
func TestXxxImportsArePure(t *testing.T) {
    const filePath = "xxx.go"
    // ... parse imports, assert not in forbidden set ...
    forbidden := map[string]bool{
        "os": true, "net": true, "net/http": true,
        "io": true, "sync": true, "time": true, "context": true,
    }
}
```

---

### Owner-Only File Permissions
**Source:** `internal/audit/writer.go` lines 26–43; `internal/platform/perms_unix.go` + `perms_windows.go`
**Apply to:** `internal/baseline/store.go`, any new file written under `~/.beekeeper/`

```go
import "github.com/mzansi-agentive/beekeeper/internal/platform"

// After creating or writing a sensitive file:
if err := platform.SetOwnerOnly(path); err != nil {
    return fmt.Errorf("enforce owner-only permissions on %q: %w", path, err)
}
// Open with 0600:
f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
```

---

### Atomic File Write
**Source:** `internal/catalog/index.go` writeFileAtomic, lines 118–139
**Apply to:** `internal/catalog/osv.go`, `internal/catalog/socket.go`, `internal/catalog/age_cache.go`, `internal/baseline/store.go`

```go
func writeFileAtomic(path string, data []byte) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
    if err != nil { return err }
    tmpName := tmp.Name()
    defer os.Remove(tmpName)
    if _, err := tmp.Write(data); err != nil { tmp.Close(); return err }
    if err := tmp.Sync(); err != nil { tmp.Close(); return err }
    if err := tmp.Close(); err != nil { return err }
    return os.Rename(tmpName, path)
}
```
Note: `writeFileAtomic` is already in `internal/catalog` package scope — OSV/Socket/age_cache can call it directly without redeclaring.

---

### HTTP Client Pattern (Context + Body Limit + Error on non-200)
**Source:** `internal/catalog/sync.go` fetch(), lines 134–156
**Apply to:** `internal/catalog/osv.go`, `internal/catalog/socket.go`, `internal/catalog/registry.go`, `internal/catalog/age_cache.go`

```go
func fetch(ctx context.Context, client *http.Client, url, token string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil { return nil, err }
    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }
    resp, err := client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
    }
    return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}
```

---

### Missing-File-is-OK Load Pattern
**Source:** `internal/config/config.go` Load(), lines 45–57
**Apply to:** `internal/baseline/store.go` Load(), `internal/catalog/watch.go` loadState()

```go
data, err := os.ReadFile(path)
if err != nil {
    if errors.Is(err, os.ErrNotExist) {
        return defaultValue, nil  // missing file = first run
    }
    return zero, fmt.Errorf("read %q: %w", path, err)
}
```

---

### Fail-Closed Default in I/O Adapters
**Source:** `internal/check/handler.go` failDecision(), lines 159–178
**Apply to:** All I/O adapters (`osv.go`, `socket.go`, `age_cache.go`, `registry.go`) — on any network or parse failure, the adapter must return a result that causes the policy engine to fail closed, NOT silently allow.

Convention:
- Return `(nil, true, nil)` for "degraded/missing, caller must treat as blocking input" — not `(nil, false, err)` which would hide the failure.
- The policy functions receive a `*Missing bool` / `degraded bool` input and explicitly block on it.

---

### Cobra Command Wiring (thin — no business logic)
**Source:** `cmd/beekeeper/main.go` newCatalogsCmd(), lines 141–172
**Apply to:** `beekeeper catalogs watch` sub-command addition and `beekeeper catalogs verify` addition

```go
// Pattern: subcommand is a &cobra.Command{} literal added via catalogs.AddCommand().
// Business logic lives in internal/catalog — cmd just resolves paths, builds
// client, and calls the internal function.
catalogs.AddCommand(&cobra.Command{
    Use:   "watch",
    Short: "...",
    Args:  cobra.NoArgs,
    RunE: func(cmd *cobra.Command, _ []string) error {
        dir, err := platform.CatalogDir()
        if err != nil { return fmt.Errorf("resolve catalog directory: %w", err) }
        // ... minimal wiring only ...
        return catalog.Watch(cmd.Context(), cfg, handler)
    },
})
```

---

### Test Helper: Fixture-Based Table Tests
**Source:** `internal/policy/engine_test.go` full file; `internal/catalog/loader_test.go`
**Apply to:** All new `internal/policy/*_test.go` and `internal/catalog/*_test.go`

```go
// Use package-internal test (same package, no _test suffix) for white-box testing:
package policy

// Use fakeCatalog pattern for injecting test data without I/O:
type fakeMultiCatalog struct {
    matchesByKey map[string][]CatalogMatch
}
func (f fakeMultiCatalog) LookupAll(ecosystem, pkg string) []CatalogMatch {
    return f.matchesByKey[ecosystem+"::"+pkg]
}

// Table-driven test pattern:
tests := []struct{
    name  string
    input XxxInput
    want  Decision
}{
    {"allow on no match", ...},
    {"warn on 1 source", ...},
    {"block on 2 signed sources", ...},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got := EvaluateXxx(tt.input, defaultConfig)
        if got.Level != tt.want.Level { t.Errorf(...) }
    })
}
```

---

### Fuzz Target Pattern (Phase 2 release gate)
**Source:** Phase 2 requirement (CLAUDE.md, CONTEXT.md §Fuzz Testing)
**Apply to:** `internal/policy/engine_test.go` (policy fuzz), `internal/catalog/loader_test.go` (catalog parser fuzz)

```go
//go:build fuzz

package policy

import "testing"

func FuzzEvaluate(f *testing.F) {
    // Seed corpus from existing test cases:
    f.Add(`{"tool_name":"Bash","tool_input":{"command":"npm install express"}}`)
    f.Fuzz(func(t *testing.T, data string) {
        // Must not panic; decision must always be valid:
        // Panics are caught by the fuzzer as failures.
    })
}
```

---

### Platform Directory Helpers
**Source:** `internal/platform/dirs.go` (full file)
**Apply to:** `internal/catalog/osv.go`, `internal/catalog/socket.go`, `internal/catalog/age_cache.go`, `internal/baseline/store.go`, `internal/catalog/watch.go`

```go
import "github.com/mzansi-agentive/beekeeper/internal/platform"

// Resolve cache paths via platform helpers — not hardcoded strings:
catalogDir, err := platform.CatalogDir()
// Results in ~/.beekeeper/catalogs/ (Unix) or %APPDATA%\beekeeper\catalogs\ (Windows)
// Subdirectories: osv/, socket-cache/, age-cache/ created via os.MkdirAll(..., 0700)
```

---

## No Analog Found

All Phase 2 files have close analogs in the existing Phase 1 codebase. No files require falling back to RESEARCH.md patterns exclusively.

The following Phase 2 files have no direct analog but are covered by the role-match analogs listed:

| File | Role | Data Flow | Why No Exact Analog |
|---|---|---|---|
| `internal/policy/exfil.go` | service (pure) | transform | Shannon entropy math — no existing entropy computation; `math` stdlib suffices |
| `internal/catalog/watch.go` | service | event-driven | First event-driven component; `tailAuditLog` in `main.go` is the closest polling-loop analog |
| `internal/baseline/store.go` | service | CRUD | First per-project keyed store; `audit/writer.go` + `config/config.go` together cover the pattern |

---

## Metadata

**Analog search scope:** `internal/policy/`, `internal/catalog/`, `internal/audit/`, `internal/config/`, `internal/check/`, `internal/platform/`, `cmd/beekeeper/`
**Files scanned:** 18 source files + 11 test files
**Pattern extraction date:** 2026-05-26
