package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixedNow is a synthetic "now" used across cache tests to eliminate wall-clock
// flakiness. All tests should use this or derive from it.
var fixedNow = time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

// npmPublishStub returns an httptest.Server that responds to any GET with a npm
// full-package document whose .time map contains pkg@version with the given
// timestamp. The returned server must be closed by the caller.
func npmPublishStub(t *testing.T, pkg, version, timestamp string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(map[string]any{
			"name": pkg,
			"time": map[string]string{
				version: timestamp,
			},
		})
		_, _ = w.Write(body)
	}))
}

// TestFetchPublishAgeFresh: registry stub returns a timestamp 30 min before
// fixedNow → ageMinutes ~30, missing false.
func TestFetchPublishAgeFresh(t *testing.T) {
	publishedAt := fixedNow.Add(-30 * time.Minute)
	srv := npmPublishStub(t, "lodash", "4.17.21", publishedAt.Format(time.RFC3339))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	cacheDir := t.TempDir()
	age, missing, err := FetchPublishAge(context.Background(), srv.Client(), cacheDir, "npm", "lodash", "4.17.21", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing {
		t.Errorf("missing = true, want false (timestamp is available)")
	}
	// Age should be approximately 30 minutes (allow ±1 minute for rounding).
	if age < 29 || age > 31 {
		t.Errorf("ageMinutes = %d, want ~30", age)
	}
}

// TestFetchPublishAgeCacheHit: after a successful fetch, the server is closed;
// a second call with the same cacheDir+now should be served from cache without
// hitting the (now-closed) server.
func TestFetchPublishAgeCacheHit(t *testing.T) {
	publishedAt := fixedNow.Add(-2 * time.Hour)
	srv := npmPublishStub(t, "express", "4.18.2", publishedAt.Format(time.RFC3339))

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL

	cacheDir := t.TempDir()
	client := srv.Client()

	// First call: populates cache.
	age1, missing1, err := FetchPublishAge(context.Background(), client, cacheDir, "npm", "express", "4.18.2", fixedNow)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if missing1 {
		t.Fatalf("first call: missing = true, want false")
	}

	// Close the server — all subsequent real network calls will fail.
	srv.Close()
	npmRegistryBase = origBase // restore to real URL (will fail if hit)

	// Second call: must succeed from cache (server is closed).
	age2, missing2, err := FetchPublishAge(context.Background(), &http.Client{}, cacheDir, "npm", "express", "4.18.2", fixedNow)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if missing2 {
		t.Errorf("second call: missing = true, want false (cache should serve)")
	}
	if age1 != age2 {
		t.Errorf("age1=%d age2=%d: cache should return same age as fresh fetch", age1, age2)
	}
}

// TestFetchPublishAgeMissingOnError: when the registry returns a non-200 error,
// FetchPublishAge must return (0, true, nil) AND write a Missing:true cache entry
// so repeated failures don't hammer the registry within the TTL window.
func TestFetchPublishAgeMissingOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	cacheDir := t.TempDir()
	age, missing, err := FetchPublishAge(context.Background(), srv.Client(), cacheDir, "npm", "failing-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected non-nil error: %v", err)
	}
	if !missing {
		t.Errorf("missing = false, want true (registry error → fail-closed)")
	}
	if age != 0 {
		t.Errorf("age = %d, want 0 on missing", age)
	}

	// Verify a Missing:true cache entry was written.
	cachePath := ageCachePath(cacheDir, "npm", "failing-pkg", "1.0.0")
	data, readErr := os.ReadFile(cachePath)
	if readErr != nil {
		t.Fatalf("expected cache entry at %s, got error: %v", cachePath, readErr)
	}
	var entry ageCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal cache entry: %v", err)
	}
	if !entry.Missing {
		t.Errorf("cache entry Missing = false, want true")
	}
}

// TestFetchPublishAgeMissingCacheServed: a pre-written Missing:true fresh cache
// entry is served without network access, returning (0, true, nil).
func TestFetchPublishAgeMissingCacheServed(t *testing.T) {
	cacheDir := t.TempDir()
	cachePath := ageCachePath(cacheDir, "npm", "cached-missing-pkg", "2.0.0")

	// Pre-write a Missing:true cache entry that is still fresh.
	preEntry := ageCacheEntry{
		CachedAt: fixedNow.Add(-1 * time.Hour), // 1h old — within 24h TTL
		Missing:  true,
	}
	data, _ := json.Marshal(preEntry)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	// Call should read from cache — any network call would fail (no server running).
	age, missing, err := FetchPublishAge(context.Background(), &http.Client{}, cacheDir, "npm", "cached-missing-pkg", "2.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing {
		t.Errorf("missing = false, want true (pre-written Missing:true cache entry)")
	}
	if age != 0 {
		t.Errorf("age = %d, want 0", age)
	}
}

// TestFetchPublishAgeStaleEntryRefetched: a stale cache entry (>24h old) triggers
// a fresh registry fetch even if the entry exists.
func TestFetchPublishAgeStaleEntryRefetched(t *testing.T) {
	newPublishedAt := fixedNow.Add(-45 * time.Minute) // 45 min ago — used in fresh stub

	srv := npmPublishStub(t, "stale-pkg", "3.0.0", newPublishedAt.Format(time.RFC3339))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	cacheDir := t.TempDir()
	cachePath := ageCachePath(cacheDir, "npm", "stale-pkg", "3.0.0")

	// Pre-write a stale cache entry (older than 24h).
	oldPublishedAt := fixedNow.Add(-72 * time.Hour) // 3 days ago
	staleEntry := ageCacheEntry{
		PublishedAt: oldPublishedAt,
		CachedAt:    fixedNow.Add(-25 * time.Hour), // stale (>24h old)
	}
	data, _ := json.Marshal(staleEntry)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatalf("write stale cache: %v", err)
	}

	age, missing, err := FetchPublishAge(context.Background(), srv.Client(), cacheDir, "npm", "stale-pkg", "3.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing {
		t.Errorf("missing = true, want false (fresh registry data available)")
	}
	// Age should be ~45 minutes from newPublishedAt, not 72 hours from staleEntry.
	if age < 44 || age > 46 {
		t.Errorf("ageMinutes = %d, want ~45 (stale entry should have been refetched)", age)
	}
}
