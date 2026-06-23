package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// npmLifecycleStub returns an httptest.Server that responds to any GET with an
// npm per-version document whose .scripts object contains the given keys. The
// returned server must be closed by the caller.
func npmLifecycleStub(t *testing.T, scripts map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, _ := json.Marshal(map[string]any{"scripts": scripts})
		_, _ = w.Write(body)
	}))
}

// TestFetchLifecycleScriptsPresent: registry stub returns a postinstall script →
// the adapter returns the present scripts, failed=false.
func TestFetchLifecycleScriptsPresent(t *testing.T) {
	srv := npmLifecycleStub(t, map[string]string{
		"postinstall": "node build.js",
		"test":        "jest", // non-lifecycle key, must be filtered out
	})
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	cacheDir := t.TempDir()
	scripts, failed, err := FetchLifecycleScripts(context.Background(), srv.Client(), cacheDir, "npm", "evil-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failed {
		t.Errorf("failed = true, want false")
	}
	if len(scripts) != 1 || scripts[0] != "postinstall" {
		t.Errorf("scripts = %v, want [postinstall]", scripts)
	}
}

// TestFetchLifecycleScriptsNone: a package with no lifecycle scripts returns an
// empty list, failed=false.
func TestFetchLifecycleScriptsNone(t *testing.T) {
	srv := npmLifecycleStub(t, map[string]string{"test": "jest"})
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	cacheDir := t.TempDir()
	scripts, failed, err := FetchLifecycleScripts(context.Background(), srv.Client(), cacheDir, "npm", "clean-pkg", "2.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failed {
		t.Errorf("failed = true, want false")
	}
	if len(scripts) != 0 {
		t.Errorf("scripts = %v, want empty", scripts)
	}
}

// TestFetchLifecycleScriptsCacheHit: after a successful fetch, the server is
// closed; a second call with the same cacheDir+now should be served from cache.
func TestFetchLifecycleScriptsCacheHit(t *testing.T) {
	srv := npmLifecycleStub(t, map[string]string{"preinstall": "x", "postinstall": "y"})

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL

	cacheDir := t.TempDir()
	client := srv.Client()

	scripts1, failed1, err := FetchLifecycleScripts(context.Background(), client, cacheDir, "npm", "cached-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if failed1 {
		t.Fatalf("first call: failed = true, want false")
	}

	// Close the server - subsequent real network calls will fail.
	srv.Close()
	npmRegistryBase = origBase

	scripts2, failed2, err := FetchLifecycleScripts(context.Background(), &http.Client{}, cacheDir, "npm", "cached-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if failed2 {
		t.Errorf("second call: failed = true, want false (cache should serve)")
	}
	sort.Strings(scripts1)
	sort.Strings(scripts2)
	if len(scripts1) != len(scripts2) {
		t.Fatalf("scripts1=%v scripts2=%v: cache must match fresh fetch", scripts1, scripts2)
	}
	for i := range scripts1 {
		if scripts1[i] != scripts2[i] {
			t.Errorf("scripts1=%v scripts2=%v: cache must match fresh fetch", scripts1, scripts2)
		}
	}
}

// TestFetchLifecycleScriptsUnsupportedEcosystem: a non-npm ecosystem fails
// (ErrEcosystemLifecycleUnsupported) → failed=true, and the failure is cached.
func TestFetchLifecycleScriptsUnsupportedEcosystem(t *testing.T) {
	cacheDir := t.TempDir()
	scripts, failed, err := FetchLifecycleScripts(context.Background(), &http.Client{}, cacheDir, "pypi", "requests", "2.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected non-nil error: %v", err)
	}
	if !failed {
		t.Errorf("failed = false, want true (unsupported ecosystem)")
	}
	if scripts != nil {
		t.Errorf("scripts = %v, want nil on failure", scripts)
	}

	// Verify a Failed:true cache entry was written.
	cachePath := lifecycleCachePath(cacheDir, "pypi", "requests", "2.0.0")
	data, readErr := os.ReadFile(cachePath)
	if readErr != nil {
		t.Fatalf("expected cache entry at %s, got error: %v", cachePath, readErr)
	}
	var entry lifecycleCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal cache entry: %v", err)
	}
	if !entry.Failed {
		t.Errorf("cache entry Failed = false, want true")
	}
}

// TestFetchLifecycleScriptsErrorCached: a registry 500 returns failed=true and
// writes a Failed:true cache entry served on a subsequent call.
func TestFetchLifecycleScriptsErrorCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	cacheDir := t.TempDir()
	_, failed, err := FetchLifecycleScripts(context.Background(), srv.Client(), cacheDir, "npm", "flaky-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !failed {
		t.Errorf("failed = false, want true (registry 500)")
	}

	// Second call served from the cached failure (no network).
	_, failed2, err := FetchLifecycleScripts(context.Background(), &http.Client{}, cacheDir, "npm", "flaky-pkg", "1.0.0", fixedNow)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if !failed2 {
		t.Errorf("second call: failed = false, want true (cached failure)")
	}
}

// TestFetchLifecycleScriptsStaleRefetched: a stale cache entry (>24h old)
// triggers a fresh registry fetch.
func TestFetchLifecycleScriptsStaleRefetched(t *testing.T) {
	srv := npmLifecycleStub(t, map[string]string{"install": "make"})
	defer srv.Close()

	origBase := npmRegistryBase
	npmRegistryBase = srv.URL
	defer func() { npmRegistryBase = origBase }()

	cacheDir := t.TempDir()
	cachePath := lifecycleCachePath(cacheDir, "npm", "stale-life-pkg", "3.0.0")

	// Pre-write a stale Failed:true entry (older than 24h).
	staleEntry := lifecycleCacheEntry{
		CachedAt: fixedNow.Add(-25 * time.Hour),
		Failed:   true,
	}
	data, _ := json.Marshal(staleEntry)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		t.Fatalf("write stale cache: %v", err)
	}

	scripts, failed, err := FetchLifecycleScripts(context.Background(), srv.Client(), cacheDir, "npm", "stale-life-pkg", "3.0.0", fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failed {
		t.Errorf("failed = true, want false (fresh data available, stale entry refetched)")
	}
	if len(scripts) != 1 || scripts[0] != "install" {
		t.Errorf("scripts = %v, want [install]", scripts)
	}
}
