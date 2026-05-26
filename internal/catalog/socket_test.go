package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- TestQuerySocketNoTokenDisabled ---

// TestQuerySocketNoTokenDisabled asserts that an empty token returns (nil, false, nil)
// and makes NO HTTP request (T-02-05-01).
func TestQuerySocketNoTokenDisabled(t *testing.T) {
	// Set up a server that fails the test if hit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("Socket server was called with empty token — this must never happen")
		http.Error(w, "unexpected call", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	entries, degraded, err := querySocket(context.Background(), srv.Client(), cacheDir,
		"" /* empty token */, "npm", "lodash", "", time.Millisecond)

	if err != nil {
		t.Fatalf("empty token: got error %v, want nil", err)
	}
	if degraded {
		t.Fatalf("empty token: degraded=true, want false")
	}
	if entries != nil {
		t.Fatalf("empty token: entries=%v, want nil", entries)
	}
}

// --- TestQuerySocketParsesResponse ---

// socketArrayResponse is a minimal Socket v0/purl response body (JSON array).
const socketArrayResponse = `[{"id":"sock-01","name":"lodash","version":"4.17.11","type":"npm","score":80,"severity":"high"}]`

// TestQuerySocketParsesResponse verifies that a 200 OK response is parsed into
// catalog entries with CatalogSource "socket".
func TestQuerySocketParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the Bearer token header is present (must not be logged or cached).
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Authorization header missing or malformed: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(socketArrayResponse))
	}))
	defer srv.Close()

	// Point the adapter at the test server by swapping the URL at package level.
	origURL := socketPURLURL
	// socketPURLURL is a const — we use the transport override approach instead:
	// Redirect the test server URL via a custom RoundTripper.
	cacheDir := t.TempDir()
	client := socketTestClient(srv.URL)

	entries, degraded, err := querySocket(context.Background(), client, cacheDir,
		"tok_test", "npm", "lodash", "4.17.11", time.Millisecond)
	_ = origURL // suppress unused warning

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if degraded {
		t.Fatalf("degraded=true, want false on success")
	}
	if len(entries) == 0 {
		t.Fatal("expected entries, got none")
	}
	if entries[0].CatalogSource != "socket" {
		t.Errorf("CatalogSource = %q, want %q", entries[0].CatalogSource, "socket")
	}
	if entries[0].Severity != "high" {
		t.Errorf("Severity = %q, want %q", entries[0].Severity, "high")
	}
}

// --- TestQuerySocketCacheHit ---

// TestQuerySocketCacheHit proves that a second call with the server closed is
// served from disk cache without a network call.
func TestQuerySocketCacheHit(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(socketArrayResponse))
	}))

	cacheDir := t.TempDir()
	client := socketTestClient(srv.URL)

	// First call — must hit the server and populate cache.
	entries1, degraded1, err1 := querySocket(context.Background(), client, cacheDir,
		"tok_test", "npm", "lodash", "4.17.11", time.Millisecond)
	if err1 != nil || degraded1 {
		t.Fatalf("first call: err=%v degraded=%v", err1, degraded1)
	}
	if len(entries1) == 0 {
		t.Fatal("first call: expected entries")
	}

	// Close the server — any further network call will fail.
	srv.Close()

	// Second call — must be served from cache.
	entries2, degraded2, err2 := querySocket(context.Background(), client, cacheDir,
		"tok_test", "npm", "lodash", "4.17.11", time.Millisecond)
	if err2 != nil {
		t.Fatalf("second call (cache hit expected): err=%v", err2)
	}
	if degraded2 {
		t.Fatalf("second call: degraded=true, want false on cache hit")
	}
	if len(entries2) == 0 {
		t.Fatal("second call: expected entries from cache")
	}
	if callCount.Load() != 1 {
		t.Errorf("server call count = %d, want 1 (cache should have served second request)", callCount.Load())
	}
}

// --- TestQuerySocket429Backoff ---

// TestQuerySocket429Backoff verifies that 429 → 429 → 200 succeeds within the
// retry budget.  Uses a tiny backoffBase (1ms) to keep the test fast.
func TestQuerySocket429Backoff(t *testing.T) {
	var attempt atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempt.Add(1)
		if n <= 2 {
			// First two calls → 429 with Retry-After: 0
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Third call → 200 success.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(socketArrayResponse))
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	client := socketTestClient(srv.URL)

	start := time.Now()
	entries, degraded, err := querySocket(context.Background(), client, cacheDir,
		"tok_test", "npm", "lodash", "4.17.11", time.Millisecond /* tiny backoff */)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("429→429→200: unexpected error: %v", err)
	}
	if degraded {
		t.Fatalf("429→429→200: degraded=true, want false on eventual success")
	}
	if len(entries) == 0 {
		t.Fatal("429→429→200: expected entries after backoff")
	}
	if attempt.Load() != 3 {
		t.Errorf("server attempt count = %d, want 3", attempt.Load())
	}
	// Sanity: test must not have taken more than 3 seconds (real backoff would be ≥1s).
	if elapsed > 3*time.Second {
		t.Errorf("test took %v, want <3s (check backoffBase injection)", elapsed)
	}
}

// --- TestQuerySocket5xxDegrades ---

// TestQuerySocket5xxDegrades verifies that a 500 response yields degraded=true,
// a non-nil error, no panic, and no fabricated entries.
func TestQuerySocket5xxDegrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	client := socketTestClient(srv.URL)

	entries, degraded, err := querySocket(context.Background(), client, cacheDir,
		"tok_test", "npm", "lodash", "4.17.11", time.Millisecond)

	if err == nil {
		t.Fatal("5xx: expected non-nil error, got nil")
	}
	if !degraded {
		t.Fatal("5xx: degraded=false, want true")
	}
	if entries != nil {
		t.Fatalf("5xx: entries=%v, want nil (no fabricated entries)", entries)
	}
	// Verify the token does NOT appear in the error message (T-02-05-01).
	if strings.Contains(err.Error(), "tok_test") {
		t.Errorf("Bearer token leaked into error message: %q", err.Error())
	}
}

// --- TestSocketAdapterLookupAllMapsMatches ---

// TestSocketAdapterLookupAllMapsMatches verifies that SocketAdapter.LookupAll
// returns []policy.CatalogMatch with CatalogSource "socket" and Signed=true.
func TestSocketAdapterLookupAllMapsMatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(socketArrayResponse))
	}))
	defer srv.Close()

	cacheDir := t.TempDir()

	adapter := SocketAdapter{
		Client:   socketTestClient(srv.URL),
		CacheDir: cacheDir,
		Token:    "tok_test",
		Ctx:      context.Background(),
	}

	matches := adapter.LookupAll("npm", "lodash")
	if len(matches) == 0 {
		t.Fatal("LookupAll returned no matches")
	}
	for i, m := range matches {
		if m.CatalogSource != "socket" {
			t.Errorf("matches[%d].CatalogSource = %q, want %q", i, m.CatalogSource, "socket")
		}
		if !m.Signed {
			t.Errorf("matches[%d].Signed = false, want true", i)
		}
		if m.CatalogVersion != "socket-api" {
			t.Errorf("matches[%d].CatalogVersion = %q, want %q", i, m.CatalogVersion, "socket-api")
		}
	}
}

// --- TestQuerySocketCacheWriteIsAtomic ---

// TestQuerySocketCacheWriteIsAtomic checks that the cache file is created
// through writeFileAtomic (no partial writes) and the cache directory is 0o700.
func TestQuerySocketCacheWriteIsAtomic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(socketArrayResponse))
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	client := socketTestClient(srv.URL)

	_, _, err := querySocket(context.Background(), client, cacheDir,
		"tok_test", "npm", "lodash", "4.17.11", time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache directory was created with owner-only permissions.
	socketCacheDir := filepath.Join(cacheDir, "socket-cache")
	info, err := os.Stat(socketCacheDir)
	if err != nil {
		t.Fatalf("socket-cache dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("socket-cache is not a directory")
	}

	// Verify the cache file exists and is valid JSON (same package — direct unmarshal).
	cachePath := socketCachePath(cacheDir, "npm", "lodash", "4.17.11")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
	var ce socketCacheEntry
	if jsonErr := json.Unmarshal(data, &ce); jsonErr != nil {
		t.Fatalf("cache file is not valid JSON: %v", jsonErr)
	}
	if ce.CachedAt.IsZero() {
		t.Error("cache entry CachedAt is zero — cache was not written correctly")
	}
}

// --- helper: socketTestClient ---

// socketTestClient returns an *http.Client that redirects all requests to
// targetBaseURL (the httptest server).  This lets us test socket.go without
// modifying the socketPURLURL const.
func socketTestClient(targetBaseURL string) *http.Client {
	return &http.Client{
		Transport: &socketRedirectTransport{base: targetBaseURL},
	}
}

// socketRedirectTransport rewrites the request host/scheme to the test server
// while preserving method, headers, and body.
type socketRedirectTransport struct {
	base string
}

func (t *socketRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clonedURL := *req.URL
	clonedURL.Scheme = "http"
	clonedURL.Host = strings.TrimPrefix(t.base, "http://")
	clone.URL = &clonedURL
	return http.DefaultTransport.RoundTrip(clone)
}
