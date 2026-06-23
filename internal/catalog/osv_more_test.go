package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestOSVEcosystemMapping verifies the internal→OSV ecosystem name table and the
// unmapped-ecosystem ("",false) contract.
func TestOSVEcosystemMapping(t *testing.T) {
	mapped := map[string]string{
		"npm":       "npm",
		"pypi":      "PyPI",
		"go":        "Go",
		"cargo":     "crates.io",
		"rubygems":  "RubyGems",
		"packagist": "Packagist",
	}
	for internal, want := range mapped {
		got, ok := osvEcosystem(internal)
		if !ok || got != want {
			t.Errorf("osvEcosystem(%q) = (%q,%v), want (%q,true)", internal, got, ok, want)
		}
	}
	if got, ok := osvEcosystem("debian"); ok || got != "" {
		t.Errorf("osvEcosystem(debian) = (%q,%v), want (\"\",false)", got, ok)
	}
}

// TestDeriveSeverity covers the present-string, empty-string, absent-map, and
// non-string cases.
func TestDeriveSeverity(t *testing.T) {
	cases := []struct {
		name string
		v    osvVuln
		want string
	}{
		{"present", osvVuln{DatabaseSpecific: map[string]any{"severity": "high"}}, "high"},
		{"empty-string", osvVuln{DatabaseSpecific: map[string]any{"severity": ""}}, "unknown"},
		{"non-string", osvVuln{DatabaseSpecific: map[string]any{"severity": 7}}, "unknown"},
		{"nil-map", osvVuln{}, "unknown"},
	}
	for _, tc := range cases {
		if got := deriveSeverity(tc.v); got != tc.want {
			t.Errorf("%s: deriveSeverity = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestQueryOSVPublicWrapper exercises the exported QueryOSV entry point on its
// no-op path (unmapped ecosystem → nil,nil, no network).
func TestQueryOSVPublicWrapper(t *testing.T) {
	entries, err := QueryOSV(context.Background(), http.DefaultClient, t.TempDir(), "debian", "libc", "2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("want nil for unmapped ecosystem, got %v", entries)
	}
}

// TestReadOSVCacheCorrupt: a corrupt cache file is a miss, not a fatal error.
func TestReadOSVCacheCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := osvCachePath(dir, "npm", "corrupt", "1.0.0")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{bad json`), 0o600); err != nil {
		t.Fatalf("seed corrupt cache: %v", err)
	}
	entries, hit, err := readOSVCache(dir, "npm", "corrupt", "1.0.0")
	if err != nil {
		t.Fatalf("corrupt cache must not error: %v", err)
	}
	if hit || entries != nil {
		t.Errorf("corrupt cache: hit=%v entries=%v, want miss", hit, entries)
	}
}

// TestOSVCachePathTraversalSanitised: path components are filepath.Base'd so a
// traversal attempt cannot escape the cache dir, and "_any" is used for empty version.
func TestOSVCachePathTraversalSanitised(t *testing.T) {
	p := osvCachePath("/cache", "../../etc", "../passwd", "")
	// Each segment must be a base name; no ".." allowed.
	for _, seg := range []string{"etc", "passwd", "_any.json"} {
		if !contains(p, seg) {
			t.Errorf("path %q missing sanitised segment %q", p, seg)
		}
	}
	if contains(p, "..") {
		t.Errorf("path %q still contains traversal sequence", p)
	}
}

// TestOSVAdapterDegradedReturnsNil: a 500 from OSV makes the adapter return nil.
func TestOSVAdapterDegradedReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	adapter := &OSVAdapter{
		Client:   srv.Client(),
		CacheDir: t.TempDir(),
		Ctx:      context.Background(),
		baseURL:  srv.URL + "/v1/query",
	}
	if got := adapter.LookupAll("npm", "p"); got != nil {
		t.Errorf("want nil on degraded OSV, got %v", got)
	}
}

// TestOSVAdapterDefaultURLNoMatchForUnmapped: with baseURL unset, an unmapped
// ecosystem returns nil without hitting the real network (queryOSV early-returns).
func TestOSVAdapterDefaultURLNoMatchForUnmapped(t *testing.T) {
	adapter := &OSVAdapter{
		Client:   http.DefaultClient,
		CacheDir: t.TempDir(),
		Ctx:      context.Background(),
	}
	if got := adapter.LookupAll("debian", "libc"); got != nil {
		t.Errorf("want nil for unmapped ecosystem, got %v", got)
	}
}

// contains is a tiny substring helper to avoid importing strings in this file.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
