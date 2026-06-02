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

	"github.com/bantuson/beekeeper/internal/catalog"
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

func TestScanWindowsShapedRecord(t *testing.T) {
	old := runBumblebeeFn
	defer func() { runBumblebeeFn = old }()

	// Windows-shaped Pollen NDJSON record:
	//   - project_path and source_file use backslash separators + drive letter
	//   - endpoint.os = "windows", endpoint.uid = "" (WPATH-02)
	// JSON-encoded backslashes: C:\Users\fana → C:\\Users\\fana in the raw string.
	windowsRecord := `{"record_type":"package","record_id":"package:abc123",` +
		`"schema_version":"0.1.0","scanner_name":"pollen",` +
		`"endpoint":{"hostname":"WIN-BOX","os":"windows","arch":"amd64",` +
		`"username":"fana","uid":""},"ecosystem":"npm",` +
		`"normalized_name":"left-pad","version":"1.3.0",` +
		`"project_path":"C:\\Users\\fana\\code\\web-app",` +
		`"source_type":"npm-lockfile",` +
		`"source_file":"C:\\Users\\fana\\code\\web-app\\package-lock.json",` +
		`"confidence":"high","has_lifecycle_scripts":false}`

	runBumblebeeFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, 1)
		ch <- []byte(windowsRecord)
		close(ch)
		return ch, true
	}

	var buf bytes.Buffer
	if err := Scan(context.Background(), Config{}, &buf); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	out := buf.String()
	// Assert: record passed through — NOT rewritten as scan_error.
	if strings.Contains(out, `"record_type":"scan_error"`) {
		t.Errorf("Windows-shaped record rejected as malformed: %s", out)
	}
	if !strings.Contains(out, `"os":"windows"`) {
		t.Errorf("endpoint.os=windows not preserved in passthrough: %s", out)
	}
	if !strings.Contains(out, `"uid":""`) {
		t.Errorf("empty uid not preserved in passthrough: %s", out)
	}
	// Backslash paths survive JSON round-trip (JSON doubles them: C:\ → C:\\ in encoded form).
	if !strings.Contains(out, `C:\\`) {
		t.Errorf("Windows drive+backslash path not preserved in passthrough: %s", out)
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
