package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
)

func TestScanWithBumblebee(t *testing.T) {
	old := runBumblebeeFn
	defer func() { runBumblebeeFn = old }()

	line1 := `{"record_type":"package","name":"test-package"}`
	line2 := `{"record_type":"finding","severity":"high"}`
	runBumblebeeFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, 2)
		ch <- []byte(line1)
		ch <- []byte(line2)
		close(ch)
		return ch, true
	}

	var buf bytes.Buffer
	cfg := Config{} // no ExtensionDirs → beekeeper scan skipped
	if err := Scan(context.Background(), cfg, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"record_type":"package"`) {
		t.Errorf("want record_type:package in output; got:\n%s", out)
	}
	if !strings.Contains(out, `"record_type":"finding"`) {
		t.Errorf("want record_type:finding in output; got:\n%s", out)
	}
}

func TestScanBumblebeeUnavailable(t *testing.T) {
	old := runBumblebeeFn
	defer func() { runBumblebeeFn = old }()
	runBumblebeeFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		return nil, false
	}

	// Build a minimal mmap index that does NOT contain the test extension.
	indexDir := t.TempDir()
	indexPath := filepath.Join(indexDir, "beekeeper.idx")
	if err := catalog.BuildIndex(indexPath, []catalog.Entry{{
		ID:        "unrelated",
		Ecosystem: "editor-extension",
		Package:   "evil.package",
	}}); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	// Create a clean extension directory.
	extRoot := t.TempDir()
	extDir := filepath.Join(extRoot, "ms-python.python-2026.4.0")
	if err := os.MkdirAll(extDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pkgJSON := []byte(`{"publisher":"ms-python","name":"python","version":"2026.4.0","displayName":"Python"}`)
	if err := os.WriteFile(filepath.Join(extDir, "package.json"), pkgJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	// Pre-seed marketplace cache so FetchMarketplaceAge avoids network calls.
	// Extension is 2 days old (> 1440-minute threshold) → release-age allows.
	cacheDir := t.TempDir()
	testNow := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	publishedAt := testNow.Add(-48 * time.Hour)
	mktDir := filepath.Join(cacheDir, "marketplace-cache", "ms-python", "python")
	if err := os.MkdirAll(mktDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cacheEntry := map[string]any{
		"published_at": publishedAt.UTC().Format(time.RFC3339),
		"cached_at":    testNow.UTC().Format(time.RFC3339),
		"missing":      false,
	}
	cacheBytes, _ := json.Marshal(cacheEntry)
	if err := os.WriteFile(filepath.Join(mktDir, "2026.4.0.json"), cacheBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	cfg := Config{
		ExtensionDirs: []string{extRoot},
		IndexPath:     indexPath,
		CacheDir:      cacheDir,
		HTTPClient:    &http.Client{Timeout: 4 * time.Second},
		Now:           func() time.Time { return testNow },
	}
	if err := Scan(context.Background(), cfg, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"bumblebee_unavailable":true`) {
		t.Errorf("want bumblebee_unavailable:true in output; got:\n%s", out)
	}
	if !strings.Contains(out, `"record_type":"finding"`) {
		t.Errorf("want record_type:finding in output (beekeeper-own scan ran); got:\n%s", out)
	}
}
