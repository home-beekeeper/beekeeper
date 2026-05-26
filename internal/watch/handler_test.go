package watch

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/notify"
	"github.com/mzansi-agentive/beekeeper/internal/quarantine"
)

func TestHandleNewExtensionCatalogHit(t *testing.T) {
	ctx := context.Background()

	// 1. Build a temp mmap index containing the Nx Console entry.
	indexDir := t.TempDir()
	indexPath := filepath.Join(indexDir, "beekeeper.idx")
	entries := []catalog.Entry{
		{
			ID:            "test-entry",
			Name:          "Nx Console",
			Ecosystem:     "editor-extension",
			Package:       "nrwl.angular-console",
			Versions:      []string{"18.95.0"},
			Severity:      "critical",
			CatalogSource: "bumblebee",
		},
	}
	if err := catalog.BuildIndex(indexPath, entries); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	// 2. Set up temp directories.
	cacheDir := t.TempDir()
	quarantineDir := t.TempDir()
	auditDir := t.TempDir()
	auditPath := filepath.Join(auditDir, "audit.ndjson")

	// 3. Create a copy of the malicious extension in a temp dir that the handler
	//    can actually move (os.Rename requires same filesystem as quarantine dir).
	watchRoot := t.TempDir()
	extDir := filepath.Join(watchRoot, "nrwl.angular-console-18.95.0")
	if err := os.MkdirAll(extDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pkgJSON := []byte(`{"publisher":"nrwl","name":"angular-console","version":"18.95.0","displayName":"Nx Console"}`)
	if err := os.WriteFile(filepath.Join(extDir, "package.json"), pkgJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	// 4. Pre-seed the marketplace cache so FetchMarketplaceAge returns a very
	//    recent publish time (10 minutes ago), triggering the release-age block
	//    (threshold is 1440 minutes / 24h). No HTTP calls needed.
	//    Path: <cacheDir>/marketplace-cache/<publisher>/<name>/<version>.json
	testNow := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	publishedAt := testNow.Add(-10 * time.Minute) // only 10 minutes old → blocked

	mktCacheDir := filepath.Join(cacheDir, "marketplace-cache", "nrwl", "angular-console")
	if err := os.MkdirAll(mktCacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cacheEntry := map[string]interface{}{
		"published_at": publishedAt.UTC().Format(time.RFC3339),
		"cached_at":    testNow.UTC().Format(time.RFC3339),
		"missing":      false,
	}
	cacheBytes, _ := json.Marshal(cacheEntry)
	if err := os.WriteFile(filepath.Join(mktCacheDir, "18.95.0.json"), cacheBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	// 5. Build and call the handler.
	now := func() time.Time { return testNow }
	handler := NewHandler(
		indexPath, cacheDir, quarantineDir, auditPath,
		notify.Config{Enabled: false},
		"", // no socket token
		&http.Client{Timeout: 4 * time.Second},
		now,
		[]string{watchRoot},
	)

	handler.HandleNewExtension(ctx, extDir)

	// 6. Assert: original path no longer exists.
	if _, err := os.Stat(extDir); !os.IsNotExist(err) {
		t.Errorf("extension should have been moved to quarantine; stat err = %v", err)
	}

	// 7. Assert: quarantine ExtensionsDir contains exactly one entry with a
	//    non-empty original_path manifest.
	manifests, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("quarantine.List: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("want 1 quarantine entry, got %d", len(manifests))
	}
	if manifests[0].OriginalPath == "" {
		t.Error("quarantine manifest original_path is empty")
	}

	// 8. Assert: audit log contains a sentry_alert record.
	f, err := os.Open(auditPath)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()

	var foundSentryAlert bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec["record_type"] == "sentry_alert" {
			foundSentryAlert = true
			break
		}
	}
	if !foundSentryAlert {
		t.Error("audit log does not contain a sentry_alert record")
	}
}
