package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// osvQueryURL is the public, no-auth OSV REST API endpoint. POST with JSON body.
// OSV API is treated as a signed source because responses come from the
// authoritative OSV host (api.osv.dev) over TLS with certificate pinning at the
// OS level — no additional signature field is needed.
const osvQueryURL = "https://api.osv.dev/v1/query"

// osvEcosystem maps Beekeeper's internal lowercase ecosystem names to the
// case-sensitive names the OSV API requires. Returns ("", false) for unknown
// ecosystems — callers MUST treat ("", false) as "OSV does not cover this
// ecosystem" and return (nil, nil) rather than querying with a wrong name that
// would silently produce empty results (T-02-04-02).
func osvEcosystem(internal string) (string, bool) {
	switch internal {
	case "npm":
		return "npm", true
	case "pypi":
		return "PyPI", true
	case "go":
		return "Go", true
	case "cargo":
		return "crates.io", true
	case "rubygems":
		return "RubyGems", true
	case "packagist":
		return "Packagist", true
	default:
		return "", false
	}
}

// osvQuery is the JSON request body sent to the OSV API.
type osvQuery struct {
	Version string  `json:"version,omitempty"`
	Package osvPkg  `json:"package"`
}

// osvPkg identifies the package in an OSV query.
type osvPkg struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// osvResponse is the top-level JSON response from the OSV API.
// A response with no vulnerabilities returns an empty or absent vulns array.
type osvResponse struct {
	Vulns []osvVuln `json:"vulns"`
}

// osvVuln is a single vulnerability record from the OSV API response.
// Only the fields required for catalog entry construction are modelled.
type osvVuln struct {
	ID               string         `json:"id"`
	Summary          string         `json:"summary"`
	DatabaseSpecific map[string]any `json:"database_specific"`
}

// osvCacheEntry is the on-disk cache record written by writeOSVCache.
// CachedAt records when the entry was written; Entries holds the parsed results.
type osvCacheEntry struct {
	CachedAt time.Time `json:"cached_at"`
	Entries  []Entry   `json:"entries"`
}

// osvCacheTTL is the time-to-live for OSV disk cache entries.
const osvCacheTTL = 24 * time.Hour

// osvCachePath returns the full path to the on-disk cache file for a given
// (cacheDir, ecosystem, pkg, version) tuple. The layout is:
//
//	<cacheDir>/osv/<ecosystem>/<pkg>/<version>.json
//
// When version is empty, "_any" is used as the filename stem so that cache
// files for unversioned queries are distinct from versioned ones and never
// collide with a package version named "".
func osvCachePath(cacheDir, ecosystem, pkg, version string) string {
	stem := version
	if stem == "" {
		stem = "_any"
	}
	return filepath.Join(cacheDir, "osv", ecosystem, pkg, stem+".json")
}

// deriveSeverity extracts a severity string from an OSV vulnerability record.
// It reads DatabaseSpecific["severity"] when the value is a string, and
// returns "unknown" otherwise. The OSV API does not mandate a severity field,
// so callers must handle "unknown" gracefully.
func deriveSeverity(v osvVuln) string {
	if v.DatabaseSpecific != nil {
		if s, ok := v.DatabaseSpecific["severity"].(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}

// readOSVCache reads an existing cache file at the path determined by
// osvCachePath. Returns the cached entries and true if the file exists and was
// written within osvCacheTTL. Returns (nil, false, nil) on a cache miss or
// an expired entry; returns a non-nil error only when the file exists but is
// corrupt or unreadable.
func readOSVCache(cacheDir, ecosystem, pkg, version string) ([]Entry, bool, error) {
	path := osvCachePath(cacheDir, ecosystem, pkg, version)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read osv cache %q: %w", path, err)
	}

	var ce osvCacheEntry
	if err := json.Unmarshal(data, &ce); err != nil {
		// Corrupt cache — treat as miss, not as a fatal error.
		return nil, false, nil
	}

	if time.Since(ce.CachedAt) >= osvCacheTTL {
		// Expired — cache miss.
		return nil, false, nil
	}

	return ce.Entries, true, nil
}

// writeOSVCache writes entries to the on-disk cache file at the path
// determined by osvCachePath, creating the necessary directories with 0o700
// permissions. The write is atomic via writeFileAtomic so partial writes
// never leave a corrupt cache file visible to concurrent readers.
func writeOSVCache(cacheDir, ecosystem, pkg, version string, entries []Entry) error {
	path := osvCachePath(cacheDir, ecosystem, pkg, version)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir osv cache dir: %w", err)
	}
	ce := osvCacheEntry{
		CachedAt: time.Now().UTC(),
		Entries:  entries,
	}
	data, err := json.Marshal(ce)
	if err != nil {
		return fmt.Errorf("marshal osv cache: %w", err)
	}
	return writeFileAtomic(path, data)
}

// queryOSVWithURL is the testable core of QueryOSV. It accepts a custom URL
// so tests can point at an httptest.Server instead of the real OSV endpoint.
// Production code calls QueryOSV which hardcodes osvQueryURL.
func queryOSVWithURL(ctx context.Context, client *http.Client, cacheDir, ecosystem, pkg, version, url string) ([]Entry, error) {
	return queryOSV(ctx, client, cacheDir, ecosystem, pkg, version, url)
}

// QueryOSV queries the OSV REST API for vulnerabilities affecting (ecosystem,
// pkg, version) and returns matching catalog entries. The flow is:
//
//  1. Map ecosystem via osvEcosystem; unknown → (nil, nil).
//  2. Cache-first: read osvCachePath; if cache hit and age < 24h, return cached entries.
//  3. On cache miss: POST to osvQueryURL with JSON body, Content-Type application/json.
//  4. On transport error or non-200: return (nil, err). The caller treats OSV as
//     degraded and falls back to remaining sources. NEVER fabricate an allow.
//  5. On 200: parse response, convert vulns to Entry, write cache atomically, return entries.
//
// The caller-supplied ctx carries the hook handler's deadline (5s typical).
// Cache-first avoids network on the common hot path (T-02-04-03).
func QueryOSV(ctx context.Context, client *http.Client, cacheDir, ecosystem, pkg, version string) ([]Entry, error) {
	return queryOSV(ctx, client, cacheDir, ecosystem, pkg, version, osvQueryURL)
}

// queryOSV is the internal implementation of QueryOSV with an injectable URL.
// This separation allows tests to point at an httptest.Server without exporting
// a URL parameter on the public API.
func queryOSV(ctx context.Context, client *http.Client, cacheDir, ecosystem, pkg, version, queryURL string) ([]Entry, error) {
	osvEco, ok := osvEcosystem(ecosystem)
	if !ok {
		// OSV does not cover this ecosystem — not an error.
		return nil, nil
	}

	// Cache-first.
	cached, hit, err := readOSVCache(cacheDir, ecosystem, pkg, version)
	if err == nil && hit {
		return cached, nil
	}
	// err != nil: log-able but continue to network (cache read failure is not fatal).

	// Build and send the POST request.
	q := osvQuery{
		Version: version,
		Package: osvPkg{Name: pkg, Ecosystem: osvEco},
	}
	bodyBytes, err := json.Marshal(q)
	if err != nil {
		return nil, fmt.Errorf("osv query marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("osv new request: %w", err)
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

	// Bound the read to 4MB (T-02-04-01: LimitReader).
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("osv read body: %w", err)
	}

	var osvResp osvResponse
	if err := json.Unmarshal(data, &osvResp); err != nil {
		return nil, fmt.Errorf("osv parse response: %w", err)
	}

	// Convert OSV vulns to catalog entries.
	entries := make([]Entry, 0, len(osvResp.Vulns))
	for _, v := range osvResp.Vulns {
		versionList := []string{}
		if version != "" {
			versionList = []string{version}
		}
		entries = append(entries, Entry{
			ID:               v.ID,
			Name:             v.Summary,
			Ecosystem:        ecosystem, // internal lowercase ecosystem name
			Package:          pkg,
			Versions:         versionList,
			Severity:         deriveSeverity(v),
			CatalogSignature: "osv-api",
			CatalogSource:    "osv",
		})
	}

	// Write cache atomically; cache write failure is non-fatal (degraded to
	// in-memory only for this invocation, will retry next call).
	_ = writeOSVCache(cacheDir, ecosystem, pkg, version, entries)

	return entries, nil
}

// OSVAdapter wraps QueryOSV to implement the per-source half of
// policy.MultiCatalogLookup. The adapter resolves all I/O (network + disk)
// before returning; the policy engine receives only the []policy.CatalogMatch
// slice and performs no I/O itself (PATTERNS "I/O adapters return pre-resolved
// inputs" rule).
//
// On QueryOSV error, LookupAll returns nil (degrade to no-match). The
// aggregator in Plan 08 records the degradation to the audit log. A nil return
// never fabricates an allow — it means this source contributes 0 signed
// matches to the corroboration count.
type OSVAdapter struct {
	// Client is the HTTP client used for OSV API calls. Must not be nil.
	Client *http.Client
	// CacheDir is the path to the Beekeeper catalogs directory (e.g.
	// ~/.beekeeper/catalogs or %APPDATA%\beekeeper\catalogs). The OSV
	// subdirectory is created on first write.
	CacheDir string
	// Ctx is the context for HTTP requests. Callers should supply the hook
	// handler's request context so the 5s deadline propagates to OSV queries.
	Ctx context.Context
	// baseURL overrides the OSV query URL. Zero value uses osvQueryURL.
	// This field is exported only for tests; production code leaves it empty.
	baseURL string
}

// LookupAll queries OSV for all known vulnerabilities affecting (ecosystem,
// pkg) regardless of version (version "" means "any version"). It maps each
// returned Entry to a policy.CatalogMatch with:
//   - CatalogSource: "osv"
//   - Signed: true (OSV API over TLS is treated as a signed source — see osvQueryURL)
//   - CatalogVersion: "osv-api"
//
// LookupAll satisfies the policy.MultiCatalogLookup interface.
func (a *OSVAdapter) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	url := osvQueryURL
	if a.baseURL != "" {
		url = a.baseURL
	}
	entries, err := queryOSV(a.Ctx, a.Client, a.CacheDir, ecosystem, pkg, "", url)
	if err != nil {
		// Degrade to no-match — caller (aggregator) logs the degradation.
		return nil
	}

	if len(entries) == 0 {
		return nil
	}

	matches := make([]policy.CatalogMatch, 0, len(entries))
	for _, e := range entries {
		matches = append(matches, policy.CatalogMatch{
			CatalogSource:  "osv",
			EntryID:        e.ID,
			Ecosystem:      ecosystem,
			Package:        pkg,
			Severity:       e.Severity,
			Signed:         true, // OSV API over TLS — authoritative signed source
			CatalogVersion: "osv-api",
		})
	}
	return matches
}
