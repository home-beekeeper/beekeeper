package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fixedMarketplaceNow is a synthetic "now" for marketplace tests to eliminate
// wall-clock flakiness.
var fixedMarketplaceNow = time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

// TestFetchMarketplaceAge verifies that FetchMarketplaceAge returns a correct
// age when Open VSX returns a valid timestamp.
func TestFetchMarketplaceAge(t *testing.T) {
	// Extension "published" 25 hours ago.
	publishedAt := fixedMarketplaceNow.Add(-25 * time.Hour)
	ts := publishedAt.Format(time.RFC3339Nano)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(map[string]string{
			"timestamp": ts,
			"error":     "",
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	origBase := openVSXBase
	openVSXBase = srv.URL
	defer func() { openVSXBase = origBase }()

	cacheDir := t.TempDir()
	age, missing, err := FetchMarketplaceAge(
		context.Background(), srv.Client(),
		cacheDir, "nrwl", "angular-console", "18.95.0",
		fixedMarketplaceNow,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing {
		t.Errorf("missing = true, want false (timestamp is available)")
	}
	// 25 hours = 1500 minutes; allow ±1 minute for rounding.
	if age < 1499 || age > 1501 {
		t.Errorf("ageMinutes = %d, want ~1500 (25h)", age)
	}
}

// TestMarketplaceAgeCacheHit verifies that a second call with the same arguments
// reads from disk cache and does NOT hit the HTTP server again.
func TestMarketplaceAgeCacheHit(t *testing.T) {
	publishedAt := fixedMarketplaceNow.Add(-2 * time.Hour)
	ts := publishedAt.Format(time.RFC3339Nano)

	var hitCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(map[string]string{
			"timestamp": ts,
			"error":     "",
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	origBase := openVSXBase
	openVSXBase = srv.URL
	defer func() { openVSXBase = origBase }()

	cacheDir := t.TempDir()
	client := srv.Client()

	// First call: should hit the server.
	_, missing1, err := FetchMarketplaceAge(
		context.Background(), client,
		cacheDir, "ms-python", "python", "2024.1.0",
		fixedMarketplaceNow,
	)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if missing1 {
		t.Fatalf("first call: missing = true, want false")
	}
	if hitCount.Load() != 1 {
		t.Fatalf("expected 1 server hit after first call, got %d", hitCount.Load())
	}

	// Second call with identical args: must read from cache, not hit the server.
	_, missing2, err := FetchMarketplaceAge(
		context.Background(), &http.Client{},
		cacheDir, "ms-python", "python", "2024.1.0",
		fixedMarketplaceNow,
	)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if missing2 {
		t.Errorf("second call: missing = true, want false (cache should serve)")
	}
	if hitCount.Load() != 1 {
		t.Errorf("server hit count = %d after second call, want 1 (cache should prevent second hit)", hitCount.Load())
	}
}

// TestMarketplaceAgeMissing verifies that when both Open VSX and VS Code
// Marketplace fail, FetchMarketplaceAge returns (0, true, nil) — fail-closed.
func TestMarketplaceAgeMissing(t *testing.T) {
	// Open VSX stub: returns an error response.
	openVSXSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(map[string]string{
			"timestamp": "",
			"error":     "not found",
		})
		_, _ = w.Write(body)
	}))
	defer openVSXSrv.Close()

	// VS Code Marketplace stub: returns a non-200.
	vscodeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer vscodeSrv.Close()

	origOpenVSX := openVSXBase
	origVSCode := vscodeMarketplaceBase
	openVSXBase = openVSXSrv.URL
	vscodeMarketplaceBase = vscodeSrv.URL
	defer func() {
		openVSXBase = origOpenVSX
		vscodeMarketplaceBase = origVSCode
	}()

	cacheDir := t.TempDir()
	age, missing, err := FetchMarketplaceAge(
		context.Background(), openVSXSrv.Client(),
		cacheDir, "unknown-publisher", "nonexistent-ext", "0.0.1",
		fixedMarketplaceNow,
	)
	if err != nil {
		t.Fatalf("unexpected non-nil error: %v", err)
	}
	if !missing {
		t.Errorf("missing = false, want true (both sources failed → fail-closed)")
	}
	if age != 0 {
		t.Errorf("age = %d, want 0 on missing", age)
	}
}
