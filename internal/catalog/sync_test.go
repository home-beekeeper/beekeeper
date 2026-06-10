package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// validCatalogBody is a minimal but schema-valid Bumblebee catalog file with a
// single entry (unsigned → warn-only; the warning to stderr is expected).
const validCatalogBody = `{"schema_version":"0.1.0","entries":[{"id":"t-1","name":"Test","ecosystem":"npm","package":"evil-pkg","versions":["1.0.0"],"severity":"critical"}]}`

// withContentsURL points bumblebeeContentsURL at u for the duration of the test
// and restores it afterwards.
func withContentsURL(t *testing.T, u string) {
	t.Helper()
	prev := bumblebeeContentsURL
	bumblebeeContentsURL = u
	t.Cleanup(func() { bumblebeeContentsURL = prev })
}

// TestSyncConditional304SkipsFetchAndRebuild proves the ETag conditional path:
// a 304 on the list call does NO raw fetch and NO index rebuild, and signals
// NotModified.
func TestSyncConditional304SkipsFetchAndRebuild(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "") // deterministic: no auth header

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// Only the list call should ever be made on the 304 path.
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()
	withContentsURL(t, srv.URL)

	dir := t.TempDir()
	idxPath := filepath.Join(dir, "bumblebee.idx")

	res, err := SyncConditional(context.Background(), srv.Client(), dir, `"prev-etag"`)
	if err != nil {
		t.Fatalf("SyncConditional(304) returned error: %v", err)
	}
	if !res.NotModified {
		t.Error("NotModified = false, want true on a 304")
	}
	if res.ETag != `"prev-etag"` {
		t.Errorf("ETag = %q, want the echoed prev ETag", res.ETag)
	}
	if requests != 1 {
		t.Errorf("server saw %d requests, want exactly 1 (list only; zero raw fetches)", requests)
	}
	if _, statErr := os.Stat(idxPath); statErr == nil {
		t.Error("bumblebee.idx was created on a 304 — no rebuild should occur")
	}
}

// TestSyncConditional200ThenETag304RoundTrip proves the full conditional cycle:
// a 200 fetches + rebuilds + captures the ETag; passing that ETag back yields a
// 304 with no further raw fetch.
func TestSyncConditional200ThenETag304RoundTrip(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	var rawFetches int
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/raw/test.json":
			rawFetches++
			_, _ = w.Write([]byte(validCatalogBody))
		default: // the Contents LIST call
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write([]byte(`[{"name":"test.json","type":"file","download_url":"` + srvURL + `/raw/test.json"}]`))
		}
	}))
	defer srv.Close()
	srvURL = srv.URL
	withContentsURL(t, srv.URL)

	dir := t.TempDir()

	// First call: no prior ETag → 200 → fetch + rebuild + capture ETag.
	res, err := SyncConditional(context.Background(), srv.Client(), dir, "")
	if err != nil {
		t.Fatalf("SyncConditional(200) returned error: %v", err)
	}
	if res.NotModified {
		t.Error("NotModified = true, want false on a 200")
	}
	if res.Count != 1 {
		t.Errorf("Count = %d, want 1", res.Count)
	}
	if res.ETag != `"v1"` {
		t.Errorf("ETag = %q, want %q (captured from response)", res.ETag, `"v1"`)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "bumblebee.idx")); statErr != nil {
		t.Errorf("bumblebee.idx not built on 200: %v", statErr)
	}
	if rawFetches != 1 {
		t.Errorf("rawFetches = %d after 200, want 1", rawFetches)
	}

	// Second call: pass the captured ETag → server replies 304 → no extra fetch.
	res2, err := SyncConditional(context.Background(), srv.Client(), dir, res.ETag)
	if err != nil {
		t.Fatalf("SyncConditional(round-trip 304) returned error: %v", err)
	}
	if !res2.NotModified {
		t.Error("second call NotModified = false, want true (ETag matched)")
	}
	if rawFetches != 1 {
		t.Errorf("rawFetches = %d after 304, want still 1 (no extra fetch)", rawFetches)
	}
}

// TestSyncConditionalErrorLeavesIndexIntact proves last-good safety: an error
// during sync never destroys the existing on-disk index.
func TestSyncConditionalErrorLeavesIndexIntact(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	withContentsURL(t, srv.URL)

	dir := t.TempDir()
	idxPath := filepath.Join(dir, "bumblebee.idx")
	sentinel := []byte("LAST-GOOD-INDEX")
	if err := os.WriteFile(idxPath, sentinel, 0o600); err != nil {
		t.Fatalf("seed index: %v", err)
	}

	if _, err := SyncConditional(context.Background(), srv.Client(), dir, ""); err == nil {
		t.Fatal("SyncConditional(500) returned nil error, want non-nil")
	}

	got, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read index after failed sync: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("index was modified on failed sync: got %q, want %q (last-good must survive)", got, sentinel)
	}
}
