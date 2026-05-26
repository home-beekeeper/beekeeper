package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// osvVulnResponse builds a minimal valid OSV API response body with one vuln.
func osvVulnResponse(id, summary string) string {
	return `{"vulns":[{"id":"` + id + `","summary":"` + summary + `","database_specific":{"severity":"critical"}}]}`
}

// TestQueryOSVParsesVulns verifies that a successful OSV API response is parsed
// into a catalog Entry with CatalogSource "osv" and the correct fields.
func TestQueryOSVParsesVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(osvVulnResponse("GHSA-xxxx-yyyy-zzzz", "Test vuln")))
	}))
	defer srv.Close()

	// Patch the query URL to point at the test server.
	origURL := osvQueryURL
	const patchedURLField = osvQueryURL // compile-time check that the const is accessible
	_ = patchedURLField
	// Since osvQueryURL is a const, we use a client that rewrites the URL via
	// a custom transport, or we call a variant. For simplicity, we patch via
	// an httptest.Server-aware client using a test helper.
	client := srv.Client()

	// We can't patch the const directly. Use a helper that accepts a base URL.
	// The real QueryOSV hardcodes osvQueryURL. We test via the server URL directly.
	_ = origURL

	entries, err := queryOSVWithURL(context.Background(), client, t.TempDir(), "npm", "lodash", "4.17.20", srv.URL+"/v1/query")
	if err != nil {
		t.Fatalf("QueryOSV: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.CatalogSource != "osv" {
		t.Errorf("CatalogSource = %q, want osv", e.CatalogSource)
	}
	if e.ID != "GHSA-xxxx-yyyy-zzzz" {
		t.Errorf("ID = %q, want GHSA-xxxx-yyyy-zzzz", e.ID)
	}
	if e.Severity != "critical" {
		t.Errorf("Severity = %q, want critical", e.Severity)
	}
	if e.Ecosystem != "npm" {
		t.Errorf("Ecosystem = %q, want npm", e.Ecosystem)
	}
}

// TestQueryOSVCacheHit proves that a second lookup with the httptest server
// shut down still returns the cached result — confirming cache-first behaviour.
func TestQueryOSVCacheHit(t *testing.T) {
	cacheDir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(osvVulnResponse("GHSA-cache-hit-0001", "Cached vuln")))
	}))

	client := srv.Client()
	url := srv.URL + "/v1/query"

	// First call: cache miss → network hit → populate cache.
	entries1, err := queryOSVWithURL(context.Background(), client, cacheDir, "npm", "semver", "7.5.2", url)
	if err != nil {
		t.Fatalf("first QueryOSV: %v", err)
	}
	if len(entries1) != 1 {
		t.Fatalf("expected 1 entry on first call, got %d", len(entries1))
	}

	// Tear down the server — subsequent network calls will fail.
	srv.Close()

	// Second call: cache hit → no network call → same result.
	entries2, err := queryOSVWithURL(context.Background(), client, cacheDir, "npm", "semver", "7.5.2", url)
	if err != nil {
		t.Fatalf("second QueryOSV (cache hit): %v", err)
	}
	if len(entries2) != 1 {
		t.Fatalf("expected 1 entry on cache hit, got %d", len(entries2))
	}
	if entries2[0].ID != entries1[0].ID {
		t.Errorf("cache hit returned different ID: got %q, want %q", entries2[0].ID, entries1[0].ID)
	}
}

// TestQueryOSVUnmappedEcosystem verifies that an unknown ecosystem returns
// (nil, nil) without making a network call or returning an error.
func TestQueryOSVUnmappedEcosystem(t *testing.T) {
	// Server should never be called.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vulns":[]}`))
	}))
	defer srv.Close()

	entries, err := queryOSVWithURL(context.Background(), srv.Client(), t.TempDir(), "deb", "libc6", "2.36", srv.URL+"/v1/query")
	if err != nil {
		t.Fatalf("unexpected error for unmapped ecosystem: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for unmapped ecosystem, got %v", entries)
	}
	if called {
		t.Error("server was called for an unmapped ecosystem — should have returned early")
	}
}

// TestQueryOSVNon200Degrades verifies that a non-200 HTTP response from the
// OSV API returns a non-nil error and does not panic or fabricate entries.
func TestQueryOSVNon200Degrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	entries, err := queryOSVWithURL(context.Background(), srv.Client(), t.TempDir(), "npm", "express", "4.18.2", srv.URL+"/v1/query")
	if err == nil {
		t.Fatal("expected non-nil error for HTTP 500, got nil")
	}
	if entries != nil {
		t.Errorf("expected nil entries on error, got %v (fabricated entries forbidden)", entries)
	}
}

// TestOSVAdapterLookupAllMapsMatches verifies that OSVAdapter.LookupAll returns
// []policy.CatalogMatch with CatalogSource "osv" and Signed true.
func TestOSVAdapterLookupAllMapsMatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(osvVulnResponse("GHSA-adapter-0001", "Adapter test vuln")))
	}))
	defer srv.Close()

	adapter := &OSVAdapter{
		Client:   srv.Client(),
		CacheDir: t.TempDir(),
		Ctx:      context.Background(),
		baseURL:  srv.URL + "/v1/query",
	}

	matches := adapter.LookupAll("npm", "vulnerable-pkg")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if m.CatalogSource != "osv" {
		t.Errorf("CatalogSource = %q, want osv", m.CatalogSource)
	}
	if !m.Signed {
		t.Error("Signed = false, want true (OSV API over TLS is a signed source)")
	}
	if m.CatalogVersion != "osv-api" {
		t.Errorf("CatalogVersion = %q, want osv-api", m.CatalogVersion)
	}
	if m.EntryID != "GHSA-adapter-0001" {
		t.Errorf("EntryID = %q, want GHSA-adapter-0001", m.EntryID)
	}
}

// TestQueryOSVEmptyResponse verifies that an OSV API response with no vulns
// returns an empty (non-nil) slice without error.
func TestQueryOSVEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vulns":[]}`))
	}))
	defer srv.Close()

	entries, err := queryOSVWithURL(context.Background(), srv.Client(), t.TempDir(), "npm", "safe-pkg", "1.0.0", srv.URL+"/v1/query")
	if err != nil {
		t.Fatalf("unexpected error for empty vulns response: %v", err)
	}
	// entries may be nil or empty — both are acceptable for a no-vuln response.
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for no-vuln response, got %d", len(entries))
	}
}

// TestOSVCacheExpiry verifies that a cache entry older than 24h is not served.
func TestOSVCacheExpiry(t *testing.T) {
	cacheDir := t.TempDir()

	// Write a stale cache entry (cached 25h ago).
	staleEntries := []Entry{{
		ID:            "GHSA-stale-0001",
		CatalogSource: "osv",
	}}
	stale := osvCacheEntry{
		CachedAt: time.Now().Add(-25 * time.Hour),
		Entries:  staleEntries,
	}
	data, _ := json.Marshal(stale)
	path := osvCachePath(cacheDir, "npm", "stale-pkg", "1.0.0")
	if err := writeFileAtomic(path, data); err != nil {
		// Can't create the path directly since the dir may not exist; use writeOSVCache helper.
		_ = err
	}
	// Use writeOSVCache to ensure dirs are created, then manually overwrite.
	_ = writeOSVCache(cacheDir, "npm", "stale-pkg", "1.0.0", staleEntries)
	// Re-write with stale timestamp.
	data, _ = json.Marshal(stale)
	p := osvCachePath(cacheDir, "npm", "stale-pkg", "1.0.0")
	_ = writeFileAtomic(p, data)

	// A fresh server should be called (cache is expired).
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"vulns":[]}`))
	}))
	defer srv.Close()

	_, _ = queryOSVWithURL(context.Background(), srv.Client(), cacheDir, "npm", "stale-pkg", "1.0.0", srv.URL+"/v1/query")
	if !called {
		t.Error("expected network call for expired cache, but server was not called")
	}
}
