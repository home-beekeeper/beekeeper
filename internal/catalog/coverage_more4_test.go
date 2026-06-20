package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSyncErrorPropagates: the public Sync wrapper surfaces a SyncConditional
// error (here a 500 on the list call).
func TestSyncErrorPropagates(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	withContentsURL(t, srv.URL)

	count, err := Sync(context.Background(), srv.Client(), t.TempDir())
	if err == nil {
		t.Fatal("Sync(500 list) = nil error, want error")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 on error", count)
	}
}

// TestSyncConditionalParseError: a raw file with invalid catalog JSON → parse error.
func TestSyncConditionalParseError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/raw/bad.json" {
			// Wrong schema_version → ParseCatalogFile rejects it.
			_, _ = w.Write([]byte(`{"schema_version":"9.9.9","entries":[]}`))
			return
		}
		_, _ = w.Write([]byte(`[{"name":"bad.json","type":"file","download_url":"` + srvURL + `/raw/bad.json"}]`))
	}))
	defer srv.Close()
	srvURL = srv.URL
	withContentsURL(t, srv.URL)

	if _, err := SyncConditional(context.Background(), srv.Client(), t.TempDir(), ""); err == nil {
		t.Fatal("want parse error on bad raw catalog, got nil")
	}
}

// TestWriteOSVCacheMkdirFails: when the OSV cache dir cannot be created (a parent
// path component is a file), writeOSVCache returns an error.
func TestWriteOSVCacheMkdirFails(t *testing.T) {
	dir := t.TempDir()
	// Make the "osv" path a regular file so MkdirAll under it fails.
	if err := os.WriteFile(filepath.Join(dir, "osv"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	if err := writeOSVCache(dir, "npm", "p", "1.0.0", []Entry{{ID: "x"}}); err == nil {
		t.Error("writeOSVCache with blocked dir = nil error, want error")
	}
}

// TestWriteSocketCacheMkdirFails: a blocked socket-cache parent path → error.
func TestWriteSocketCacheMkdirFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "socket-cache"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	path := socketCachePath(dir, "npm", "p", "1.0.0")
	if err := writeSocketCache(path, []Entry{{ID: "x"}}); err == nil {
		t.Error("writeSocketCache with blocked dir = nil error, want error")
	}
}

// TestWatchRecoversFromCorruptState: Watch starts fresh when the state file is
// corrupt (non-fatal), runs at least one tick, and exits on context cancel.
func TestWatchRecoversFromCorruptState(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	if err := os.WriteFile(stateFile, []byte(`{corrupt`), 0o600); err != nil {
		t.Fatalf("seed corrupt state: %v", err)
	}
	catalogDir := t.TempDir() // no bumblebee.json → snapshot returns (0,"",nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, WatchConfig{
			PollInterval:    time.Millisecond, // tiny so a tick fires fast
			minPollInterval: time.Millisecond, // allow the tiny interval through clamp
			CatalogDir:      catalogDir,
			StateFile:       stateFile,
		}, nil)
	}()

	// Give the loop a moment to tick at least once, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch returned %v, want nil on cancel", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not exit within 2s of cancel")
	}
}

// TestAddLocalOverlayEntryWriteFails: a nonexistent catalogDir makes the atomic
// JSON write fail (CreateTemp in a missing dir), surfacing an error.
func TestAddLocalOverlayEntryWriteFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	e := Entry{Ecosystem: "npm", Package: "evil", Versions: []string{"1.0.0"}, CatalogSource: "local-overlay"}
	if err := AddLocalOverlayEntry(missing, e); err == nil {
		t.Error("AddLocalOverlayEntry into a missing dir = nil error, want write error")
	}
}
