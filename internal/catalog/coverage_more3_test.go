package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// --- registry.go: client.Do transport-error path (shared via fetchRegistryJSON) ---

// TestFetchRegistryJSONTransportError: pointing a fetcher at a closed server
// exercises the client.Do error branch in fetchRegistryJSON.
func TestFetchRegistryJSONTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused for subsequent requests

	var dst struct{}
	if err := fetchRegistryJSON(context.Background(), &http.Client{}, url, &dst); err == nil {
		t.Error("want transport error against a closed server, got nil")
	}
}

// TestFetchNPMPublishTimeTransportError ensures the npm fetcher wraps the
// transport error from a closed server.
func TestFetchNPMPublishTimeTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	withBase(t, &npmRegistryBase, srv.URL)
	srv.Close()

	if _, err := fetchNPMPublishTime(context.Background(), &http.Client{}, "pkg", "1.0.0"); err == nil {
		t.Error("want transport error, got nil")
	}
}

// --- socket.go: Retry-After header honored on a successful retry ---

// TestQuerySocketRetryAfterHonored: a 429 with a tiny numeric Retry-After then a
// 200 succeeds, exercising the Retry-After parse branch.
func TestQuerySocketRetryAfterHonored(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", "0.001") // valid float > 0 → parse branch
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(socketArrayResponse))
	}))
	defer srv.Close()

	entries, degraded, err := querySocket(context.Background(), socketTestClient(srv.URL), t.TempDir(),
		"tok", "npm", "lodash", "4.17.11", time.Hour /* ignored: Retry-After wins */)
	if err != nil || degraded {
		t.Fatalf("err=%v degraded=%v, want success", err, degraded)
	}
	if len(entries) == 0 {
		t.Fatal("expected entries after Retry-After backoff")
	}
}

// --- selfcatalog.go: fetchSelfFeed non-200 + WarnContinue with stale cache ---

// TestFetchSelfFeedNon200: a non-200 feed response → error.
func TestFetchSelfFeedNon200(t *testing.T) {
	srv := serveFeed(t, []byte("nope"), http.StatusInternalServerError)
	if _, err := fetchSelfFeed(srv.Client(), srv.URL); err == nil {
		t.Error("want error on non-200 feed, got nil")
	}
}

// TestCheckSelfCatalogStaleCacheWarnContinue: the network fails AND the only cache
// is stale (older than the TTL) → WarnContinue (don't brick).
func TestCheckSelfCatalogStaleCacheWarnContinue(t *testing.T) {
	cacheDir := t.TempDir()
	// Seed a stale cache entry directly on disk.
	cachePath := selfCatalogCachePath(cacheDir)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := selfCatalogCacheEntry{
		CachedAt: time.Now().Add(-selfCatalogCacheTTL - time.Hour),
		FeedData: []byte(`{"schema_version":"1","entries":[]}`),
	}
	data, _ := json.Marshal(stale)
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatalf("seed stale cache: %v", err)
	}

	// Point the feed URL at a closed server so the fetch fails.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	res := CheckSelfCatalog(SelfCatalogOpts{
		FeedURL:        deadURL,
		CacheDir:       cacheDir,
		Client:         &http.Client{Timeout: time.Second},
		Version:        "v0.0.0-test",
		StatePath:      filepath.Join(t.TempDir(), "state.json"),
		PubKeyOverride: SelfCatalogPublicKey,
	})
	if res.Outcome != SelfCatalogWarnContinue {
		t.Errorf("Outcome = %v, want SelfCatalogWarnContinue (stale cache + network down)", res.Outcome)
	}
}

// --- marketplace.go: future-timestamp on the cache-hit path ---

// TestFetchMarketplaceAgeFutureCacheServed: a fresh cache entry with a future
// PublishedAt is treated as missing on the cache-hit branch.
func TestFetchMarketplaceAgeFutureCacheServed(t *testing.T) {
	dir := t.TempDir()
	path := marketplaceCachePath(dir, "pub", "ext", "3.0.0")
	entry := ageCacheEntry{
		PublishedAt: fixedMarketplaceNow.Add(10 * time.Hour), // future
		CachedAt:    fixedMarketplaceNow.Add(-1 * time.Hour), // fresh
	}
	data, _ := json.Marshal(entry)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	age, missing, err := FetchMarketplaceAge(context.Background(), &http.Client{}, dir, "pub", "ext", "3.0.0", fixedMarketplaceNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !missing || age != 0 {
		t.Errorf("(age=%d,missing=%v), want (0,true)", age, missing)
	}
}

// --- state.go: SaveState mkdir failure (a file occupies the parent dir path) ---

// TestSaveStateMkdirFails: when the parent path is a regular file, MkdirAll fails
// and SaveState returns an error.
func TestSaveStateMkdirFails(t *testing.T) {
	dir := t.TempDir()
	// Create a file named "blocker"; then ask SaveState to write under blocker/state.json.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker file: %v", err)
	}
	target := filepath.Join(blocker, "state.json") // parent "blocker" is a file → MkdirAll fails
	if err := SaveState(target, WatchState{Sources: map[string]SourceState{}}); err == nil {
		t.Error("SaveState into a file-as-dir path = nil error, want error")
	}
}

// --- index.go: OpenIndex bad-version + truncated header ---

// TestOpenIndexBadVersion: a header with correct magic but wrong version → error.
func TestOpenIndexBadVersion(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.idx")
	if err := BuildIndex(good, nxEntries()); err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Corrupt the version field (bytes 4..8) to an unsupported value.
	data[4] = 0xFF
	data[5] = 0xFF
	bad := filepath.Join(dir, "badver.idx")
	if err := os.WriteFile(bad, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := OpenIndex(bad); err == nil {
		t.Error("OpenIndex(bad version) = nil error, want error")
	}
}

// TestOpenIndexTruncatedRecords: a valid header claiming records but a truncated
// body → error (implausible count or truncation guard).
func TestOpenIndexTruncatedRecords(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.idx")
	if err := BuildIndex(good, nxEntries()); err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Keep the header but drop the body so the records region is truncated.
	truncated := data[:headerSize+1]
	bad := filepath.Join(dir, "trunc.idx")
	if err := os.WriteFile(bad, truncated, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := OpenIndex(bad); err == nil {
		t.Error("OpenIndex(truncated records) = nil error, want error")
	}
}
