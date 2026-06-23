package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestQuerySocketUnsupportedEcosystemDisabled: an ecosystem with no PURL type
// returns (nil,false,nil) and makes no network call (treated as disabled).
func TestQuerySocketUnsupportedEcosystemDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for an unsupported ecosystem")
	}))
	defer srv.Close()

	entries, degraded, err := querySocket(context.Background(), socketTestClient(srv.URL), t.TempDir(),
		"tok", "unknown-eco", "pkg", "", time.Millisecond)
	if err != nil || degraded || entries != nil {
		t.Fatalf("unsupported eco: got entries=%v degraded=%v err=%v, want nil/false/nil", entries, degraded, err)
	}
}

// TestQuerySocket404Deprecated: a 404 yields degraded=true + error (deprecation path).
func TestQuerySocket404Deprecated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	entries, degraded, err := querySocket(context.Background(), socketTestClient(srv.URL), t.TempDir(),
		"tok", "npm", "lodash", "4.0.0", time.Millisecond)
	if err == nil {
		t.Fatal("want error on 404, got nil")
	}
	if !degraded {
		t.Error("want degraded=true on 404")
	}
	if entries != nil {
		t.Errorf("want nil entries on 404, got %v", entries)
	}
}

// TestQuerySocket401Degrades: an auth failure (401) degrades with an error.
func TestQuerySocket401Degrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, degraded, err := querySocket(context.Background(), socketTestClient(srv.URL), t.TempDir(),
		"tok_secret", "npm", "lodash", "4.0.0", time.Millisecond)
	if err == nil {
		t.Fatal("want error on 401, got nil")
	}
	if !degraded {
		t.Error("want degraded=true on 401")
	}
	if strings.Contains(err.Error(), "tok_secret") {
		t.Error("token leaked into error message")
	}
}

// TestQuerySocketTransportError: a connection failure degrades immediately.
func TestQuerySocketTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // close immediately → connection refused

	_, degraded, err := querySocket(context.Background(), socketTestClient(url), t.TempDir(),
		"tok", "npm", "lodash", "4.0.0", time.Millisecond)
	if err == nil {
		t.Fatal("want transport error, got nil")
	}
	if !degraded {
		t.Error("want degraded=true on transport error")
	}
}

// TestQuerySocket429ExhaustsRetries: persistent 429s exhaust the retry budget and
// return a rate-limit error (uses tiny backoff to stay fast).
func TestQuerySocket429ExhaustsRetries(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, degraded, err := querySocket(context.Background(), socketTestClient(srv.URL), t.TempDir(),
		"tok", "npm", "lodash", "4.0.0", time.Microsecond)
	if err == nil {
		t.Fatal("want rate-limit error after exhausting retries, got nil")
	}
	if !degraded {
		t.Error("want degraded=true after retries exhausted")
	}
	// maxRetries=5 → 6 attempts total (0..5).
	if hits != 6 {
		t.Errorf("server hits = %d, want 6 (1 initial + 5 retries)", hits)
	}
}

// TestQuerySocketCtxCancelledDuringBackoff: a cancelled context during the 429
// backoff returns the context error.
func TestQuerySocketCtxCancelledDuringBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Retry-After → falls back to backoffBase, giving the cancel time to land.
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call so the backoff select picks ctx.Done()

	_, degraded, err := querySocket(ctx, socketTestClient(srv.URL), t.TempDir(),
		"tok", "npm", "lodash", "4.0.0", time.Hour /* long backoff so ctx wins */)
	if err == nil {
		t.Fatal("want ctx error, got nil")
	}
	if !degraded {
		t.Error("want degraded=true when ctx cancelled")
	}
}

// TestQuerySocketPublicWrapper exercises the exported QuerySocket entry point with
// an empty token (disabled), proving the public surface returns (nil,false,nil).
func TestQuerySocketPublicWrapper(t *testing.T) {
	entries, degraded, err := QuerySocket(context.Background(), http.DefaultClient, t.TempDir(),
		"" /* no token */, "npm", "lodash", "1.0.0")
	if err != nil || degraded || entries != nil {
		t.Fatalf("QuerySocket(no token): entries=%v degraded=%v err=%v, want nil/false/nil", entries, degraded, err)
	}
}

// TestParseSocketResponseObjectWrapper covers the {"results":[...]} object form
// and the score→severity tier mapping when the severity field is absent.
func TestParseSocketResponseObjectWrapper(t *testing.T) {
	body := []byte(`{"results":[{"id":"r1","name":"p","version":"1.0.0","score":60}]}`)
	entries, err := parseSocketResponse(body, "npm", "p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	// score 60 is in [50,75) → "high".
	if entries[0].Severity != "high" {
		t.Errorf("severity = %q, want high (score 60)", entries[0].Severity)
	}
}

// TestParseSocketResponseScoreTiers verifies all score→severity tiers and the
// synthesized ID when the id field is empty.
func TestParseSocketResponseScoreTiers(t *testing.T) {
	body := []byte(`[{"score":80},{"score":50},{"score":25},{"score":0}]`)
	entries, err := parseSocketResponse(body, "npm", "tierpkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"critical", "high", "medium", "low"}
	if len(entries) != len(want) {
		t.Fatalf("want %d entries, got %d", len(want), len(entries))
	}
	for i, e := range entries {
		if e.Severity != want[i] {
			t.Errorf("entries[%d].Severity = %q, want %q", i, e.Severity, want[i])
		}
		if e.ID != "socket-npm-tierpkg" {
			t.Errorf("entries[%d].ID = %q, want synthesized socket-npm-tierpkg", i, e.ID)
		}
	}
}

// TestParseSocketResponseEmpty: empty array → (nil, nil) (no threats found).
func TestParseSocketResponseEmpty(t *testing.T) {
	entries, err := parseSocketResponse([]byte(`[]`), "npm", "p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("want nil for empty results, got %v", entries)
	}
}

// TestParseSocketResponseInvalid: unparseable body → error.
func TestParseSocketResponseInvalid(t *testing.T) {
	if _, err := parseSocketResponse([]byte(`{{not json`), "npm", "p"); err == nil {
		t.Error("want parse error, got nil")
	}
}

// TestQuerySocketParseErrorDegrades: a 200 with an unparseable body degrades.
func TestQuerySocketParseErrorDegrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{{garbage`))
	}))
	defer srv.Close()

	_, degraded, err := querySocket(context.Background(), socketTestClient(srv.URL), t.TempDir(),
		"tok", "npm", "lodash", "4.0.0", time.Millisecond)
	if err == nil {
		t.Fatal("want parse error, got nil")
	}
	if !degraded {
		t.Error("want degraded=true on parse error")
	}
}

// TestSocketAdapterDegradedReturnsNil: when QuerySocket degrades, the adapter
// returns nil (no fabricated matches).
func TestSocketAdapterDegradedReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	adapter := SocketAdapter{
		Client:   socketTestClient(srv.URL),
		CacheDir: t.TempDir(),
		Token:    "tok",
		Ctx:      context.Background(),
	}
	if got := adapter.LookupAll("npm", "lodash"); got != nil {
		t.Errorf("want nil on degraded source, got %v", got)
	}
}

// TestPurlForUnsupported: unsupported ecosystem → "".
func TestPurlForUnsupported(t *testing.T) {
	if got := purlFor("unknown", "p", "1.0.0"); got != "" {
		t.Errorf("purlFor(unknown) = %q, want empty", got)
	}
	// Supported, no version → no @version suffix.
	if got := purlFor("npm", "lodash", ""); got != "pkg:npm/lodash" {
		t.Errorf("purlFor(npm,lodash,'') = %q, want pkg:npm/lodash", got)
	}
}

// TestLoadSocketCacheStale: an entry older than the TTL is reported as a miss.
func TestLoadSocketCacheStale(t *testing.T) {
	dir := t.TempDir()
	path := socketCachePath(dir, "npm", "stale", "1.0.0")
	if err := writeSocketCache(path, []Entry{{ID: "x"}}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	// Manually rewrite with a stale CachedAt.
	stale := socketCacheEntry{CachedAt: time.Now().Add(-socketCacheTTL - time.Hour), Entries: []Entry{{ID: "x"}}}
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := writeFileAtomic(path, data); err != nil {
		t.Fatalf("rewrite cache: %v", err)
	}
	if _, ok := loadSocketCache(path); ok {
		t.Error("loadSocketCache returned ok=true for a stale entry, want false")
	}
}

// TestLoadSocketCacheCorrupt: a non-JSON cache file is a miss, not a crash.
func TestLoadSocketCacheCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := socketCachePath(dir, "npm", "corrupt", "1.0.0")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`not json`), 0o600); err != nil {
		t.Fatalf("seed corrupt cache: %v", err)
	}
	if _, ok := loadSocketCache(path); ok {
		t.Error("loadSocketCache returned ok=true for corrupt JSON, want false")
	}
}
