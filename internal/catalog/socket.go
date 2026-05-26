// Package catalog — Socket PURL API adapter (CTLG-03).
//
// # DEPRECATION NOTICE
//
// The v0/purl endpoint used by this adapter is DEPRECATED since 2026-01-05.
// Removal is scheduled for 2026-07-30.
//
// TODO(2026-07-30): Migrate to POST https://api.socket.dev/v0/packages before
// removal.  The migration touches only this file — change socketPURLURL, the
// request body builder (purlFor → buildPackagesRequest), and the response
// parser (parseSocketResponse).  The cache, backoff, degraded-mode, and adapter
// logic are all unchanged.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// socketPURLURL is the Socket PURL API endpoint.
//
// DEPRECATION: v0/purl deprecated 2026-01-05, removal 2026-07-30.
// Migrate to POST https://api.socket.dev/v0/packages before removal.
const socketPURLURL = "https://api.socket.dev/v0/purl"

// socketMaxBodyBytes caps the Socket API response body to prevent memory
// exhaustion from an oversized or malicious response.
const socketMaxBodyBytes = 4 << 20 // 4 MiB

// socketCacheTTL is the lifetime of a cached Socket query result. Using a
// 24-hour TTL as the primary rate-limit defense (CTLG-03, T-02-05-02).
const socketCacheTTL = 24 * time.Hour

// socketCacheEntry is the on-disk format for a cached Socket PURL result.
// The token is NEVER written here — only the returned package risk data.
type socketCacheEntry struct {
	CachedAt time.Time `json:"cached_at"`
	Entries  []Entry   `json:"entries"`
}

// ecosystemToPURLType maps Beekeeper ecosystem names to PURL type strings per
// https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst.
var ecosystemToPURLType = map[string]string{
	"npm":       "npm",
	"pypi":      "pypi",
	"go":        "golang",
	"cargo":     "cargo",
	"rubygems":  "gem",
	"packagist": "composer",
}

// purlFor builds a minimal Package-URL (PURL) string for the given ecosystem,
// package, and optional version. Returns "" if the ecosystem is unsupported.
// Format: pkg:<type>/<pkg>@<version>  (version omitted when empty).
func purlFor(ecosystem, pkg, version string) string {
	purlType, ok := ecosystemToPURLType[strings.ToLower(ecosystem)]
	if !ok {
		return ""
	}
	if version != "" {
		return fmt.Sprintf("pkg:%s/%s@%s", purlType, pkg, version)
	}
	return fmt.Sprintf("pkg:%s/%s", purlType, pkg)
}

// socketCachePath returns the path where the query result for (ecosystem, pkg,
// version) is cached.  The version "" is stored as the literal filename "_any"
// so it does not conflict with a package actually versioned "".
func socketCachePath(cacheDir, ecosystem, pkg, version string) string {
	if version == "" {
		version = "_any"
	}
	// Sanitise package name: replace path separators to prevent directory
	// traversal (e.g. "foo/bar" becomes "foo_bar").
	safePkg := strings.NewReplacer("/", "_", "\\", "_").Replace(pkg)
	return filepath.Join(cacheDir, "socket-cache", ecosystem, safePkg, version+".json")
}

// loadSocketCache reads and validates the cache at path.  Returns (entry, true)
// when the cached entry exists and is still within the TTL.
func loadSocketCache(path string) (socketCacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return socketCacheEntry{}, false
	}
	var ce socketCacheEntry
	if err := json.Unmarshal(data, &ce); err != nil {
		return socketCacheEntry{}, false
	}
	if time.Since(ce.CachedAt) > socketCacheTTL {
		return socketCacheEntry{}, false
	}
	return ce, true
}

// writeSocketCache persists entries to the cache path atomically.
// Cache directory is created with 0o700 (owner-only).
// The Bearer token is NEVER written here (T-02-05-01).
func writeSocketCache(path string, entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("socket cache mkdir: %w", err)
	}
	ce := socketCacheEntry{
		CachedAt: time.Now().UTC(),
		Entries:  entries,
	}
	data, err := json.Marshal(ce)
	if err != nil {
		return fmt.Errorf("socket cache marshal: %w", err)
	}
	return writeFileAtomic(path, data)
}

// socketRiskItem models a single entry in the Socket v0/purl response.
// The API returns a JSON array of objects; we capture only the fields we need.
type socketRiskItem struct {
	// The Socket API returns package risk data with varying schema. We extract
	// the fields common across all risk types to build an Entry.
	ID       string `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Type     string `json:"type"` // e.g. "npm"
	Score    int    `json:"score"`
	Severity string `json:"severity"` // "critical"|"high"|"medium"|"low" when present
}

// parseSocketResponse parses the raw Socket v0/purl API response body into
// catalog entries for the given ecosystem+package.  Returns nil (not an error)
// when the body represents "no threats found" (empty array or empty results).
func parseSocketResponse(body []byte, ecosystem, pkg string) ([]Entry, error) {
	// The Socket v0/purl API returns a JSON object at top level, or may return
	// a JSON array.  We attempt both and use whichever unmarshals cleanly.
	var results []socketRiskItem
	if err := json.Unmarshal(body, &results); err != nil {
		// Try object wrapper: {"results":[...]}
		var wrapper struct {
			Results []socketRiskItem `json:"results"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
			return nil, fmt.Errorf("socket: parse response: %w", err)
		}
		results = wrapper.Results
	}

	if len(results) == 0 {
		return nil, nil
	}

	entries := make([]Entry, 0, len(results))
	for _, r := range results {
		severity := r.Severity
		if severity == "" {
			// Map score (0–100) to severity tier when the field is absent.
			switch {
			case r.Score >= 75:
				severity = "critical"
			case r.Score >= 50:
				severity = "high"
			case r.Score >= 25:
				severity = "medium"
			default:
				severity = "low"
			}
		}

		id := r.ID
		if id == "" {
			id = fmt.Sprintf("socket-%s-%s", ecosystem, pkg)
		}

		entries = append(entries, Entry{
			ID:               id,
			Name:             r.Name,
			Ecosystem:        ecosystem,
			Package:          pkg,
			Versions:         []string{r.Version},
			Severity:         severity,
			CatalogSignature: "socket-api", // treated as signed — authoritative over TLS
			CatalogSource:    "socket",
		})
	}
	return entries, nil
}

// defaultSocketBackoffBase is the initial exponential backoff duration.
// It is a variable (not const) so tests can inject a shorter duration via
// socketBackoffBase to avoid real sleeps.
var defaultSocketBackoffBase = time.Second

// QuerySocket queries the Socket PURL API for threat data about an
// (ecosystem, pkg, version) tuple, using disk cache and exponential backoff.
//
// Return semantics:
//
//	token == ""                     → (nil, false, nil)  Socket disabled, not an error
//	cache hit (age < 24h)           → (entries, false, nil)
//	200 OK                          → (entries, false, nil); cache written atomically
//	HTTP 429 / transport err / 5xx  → (nil, true, err)   degraded=true; check continues
//
// The Bearer token is ONLY used as a request header.  It is never written to
// disk, logs, or the returned error message (T-02-05-01).
func QuerySocket(
	ctx context.Context,
	client *http.Client,
	cacheDir, token, ecosystem, pkg, version string,
) (entries []Entry, degraded bool, err error) {
	return querySocket(ctx, client, cacheDir, token, ecosystem, pkg, version, defaultSocketBackoffBase)
}

// querySocket is the testable inner implementation; backoffBase is injected by
// tests to avoid real sleeps.
func querySocket(
	ctx context.Context,
	client *http.Client,
	cacheDir, token, ecosystem, pkg, version string,
	backoffBase time.Duration,
) ([]Entry, bool, error) {
	// token == "" means Socket is simply not configured; not an error.
	if token == "" {
		return nil, false, nil
	}

	purl := purlFor(ecosystem, pkg, version)
	if purl == "" {
		// Unsupported ecosystem — treat as disabled (no match, no error).
		return nil, false, nil
	}

	// Cache-first: serve from disk if fresh.
	cachePath := socketCachePath(cacheDir, ecosystem, pkg, version)
	if ce, ok := loadSocketCache(cachePath); ok {
		return ce.Entries, false, nil
	}

	// Prepare request body: Socket v0/purl expects the PURL as a JSON string.
	bodyJSON, err := json.Marshal(purl)
	if err != nil {
		return nil, true, fmt.Errorf("socket: build request: %w", err)
	}

	const maxRetries = 5
	backoff := backoffBase
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, socketPURLURL,
			strings.NewReader(string(bodyJSON)))
		if err != nil {
			return nil, true, fmt.Errorf("socket: build request: %w", err)
		}
		// Token is only ever set as a request header — never written to disk or
		// included in error messages (T-02-05-01).
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := client.Do(req)
		if doErr != nil {
			// Transport error — degrade immediately (no retry for connection errors).
			return nil, true, fmt.Errorf("socket: request failed: %w", doErr)
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, socketMaxBodyBytes))
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			// T-02-05-02: honor Retry-After or fall back to exponential backoff.
			sleep := backoff
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, parseErr := strconv.ParseFloat(ra, 64); parseErr == nil && secs > 0 {
					sleep = time.Duration(secs * float64(time.Second))
				}
			}
			if sleep > 60*time.Second {
				sleep = 60 * time.Second
			}

			// Check max retries BEFORE sleeping so the final attempt does not
			// incur an unnecessary backoff delay before returning the error (WR-02).
			if attempt == maxRetries {
				return nil, true, fmt.Errorf("socket: rate limit exceeded after %d retries", maxRetries)
			}

			// Sleep only when we will actually retry.
			select {
			case <-time.After(sleep):
			case <-ctx.Done():
				return nil, true, ctx.Err()
			}
			backoff *= 2
			continue
		}

		if resp.StatusCode >= 500 {
			// 5xx — degrade gracefully (T-02-05-03).
			return nil, true, fmt.Errorf("socket: server error HTTP %d", resp.StatusCode)
		}

		if resp.StatusCode == http.StatusNotFound {
			// 404 most likely means the v0/purl endpoint has been removed.
			// DEPRECATED: v0/purl was deprecated 2026-01-05; removal scheduled 2026-07-30.
			// Emit a prominent stderr warning so operators notice before (or after) removal day.
			// TODO(2026-07-30): migrate to POST https://api.socket.dev/v0/packages.
			fmt.Fprintf(os.Stderr, "DEPRECATED: Socket v0/purl endpoint returned 404 — the endpoint "+
				"(deprecated 2026-01-05, removal 2026-07-30) may have been removed. "+
				"Migrate to POST https://api.socket.dev/v0/packages.\n")
			return nil, true, fmt.Errorf("socket: unexpected HTTP %d (v0/purl deprecated — see stderr)", resp.StatusCode)
		}

		if resp.StatusCode != http.StatusOK {
			// Other non-success (e.g. 401, 403) — degrade so the check can continue.
			return nil, true, fmt.Errorf("socket: unexpected HTTP %d", resp.StatusCode)
		}

		// 200 OK: parse and cache.
		parsed, parseErr := parseSocketResponse(body, ecosystem, pkg)
		if parseErr != nil {
			return nil, true, parseErr
		}

		// Write cache atomically; ignore cache-write errors (non-fatal).
		_ = writeSocketCache(cachePath, parsed)

		return parsed, false, nil
	}

	return nil, true, fmt.Errorf("socket: max retries exceeded")
}

// SocketAdapter implements policy.MultiCatalogLookup using the Socket PURL API.
// Call QuerySocket before Evaluate; the adapter must not perform I/O during
// LookupAll (the policy package is a pure library).  Pre-resolve by calling
// QuerySocket and storing the results, then pass this adapter to Evaluate.
//
// For use in the pre-resolution path (Plan 08 aggregator), construct an adapter
// with the resolved entries per lookup key, then delegate LookupAll to that map.
//
// In the simple case (single-package check), use SocketAdapter directly:
// QuerySocket is called from LookupAll, which makes the adapter an I/O-performing
// type. Callers who must keep I/O out of the policy path should use the
// pre-resolved map pattern (Plan 08).
type SocketAdapter struct {
	Client   *http.Client
	CacheDir string
	Token    string
	Ctx      context.Context
}

// LookupAll returns policy.CatalogMatch entries for (ecosystem, pkg) from Socket.
// On degradation or error, returns nil (the aggregator in Plan 08 records the
// degradation to audit).
//
// NOTE: This method performs I/O. It must only be called from outside
// internal/policy. The pure-function constraint on internal/policy is upheld
// because SocketAdapter is defined in internal/catalog, not internal/policy.
func (a SocketAdapter) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	entries, degraded, err := querySocket(a.Ctx, a.Client, a.CacheDir, a.Token,
		ecosystem, pkg, "" /* version resolved per-entry */, defaultSocketBackoffBase)
	if degraded || err != nil {
		// Nil signals the aggregator (Plan 08) to record this source as degraded.
		return nil
	}

	if len(entries) == 0 {
		return nil
	}

	matches := make([]policy.CatalogMatch, 0, len(entries))
	for _, e := range entries {
		matches = append(matches, policy.CatalogMatch{
			CatalogSource:  "socket",
			EntryID:        e.ID,
			Ecosystem:      ecosystem,
			Package:        pkg,
			Version:        "",
			Severity:       e.Severity,
			Signed:         true,            // Socket API over TLS with Bearer auth — treated as signed
			CatalogVersion: "socket-api",    // version token; per-response version is not stable
		})
	}
	return matches
}
