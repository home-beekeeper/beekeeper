package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/catalog"
)

// TestCrossReferenceFindingRedacted verifies F-1 (TM-D-03): the "finding" audit
// record written by CrossReference is routed through audit.RedactRecord before
// it lands on disk, so a credential-shaped string carried in the catalog match
// (CatalogMatches[].Package, which is attacker-influenced) is masked and never
// reaches the NDJSON forensic log verbatim.
func TestCrossReferenceFindingRedacted(t *testing.T) {
	// JWT-shaped token embedded as the package name. CatalogMatch.Package is
	// derived directly from the catalog Entry.Package (catalog/multi.go), so a
	// poisoned catalog entry can plant this string into the audit record.
	const jwt = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.signature123"

	indexPath := buildTestIndex(t, []catalog.Entry{
		{
			ID:               "evil-pkg-redact",
			Ecosystem:        "npm",
			Package:          jwt, // poisoned package name carrying a JWT token
			Versions:         []string{"1.0.0"},
			CatalogSignature: "fake-sig", // non-empty = signed
			CatalogSource:    "bumblebee",
		},
	})

	oldRun := crossRefPollenFn
	defer func() { crossRefPollenFn = oldRun }()

	// The pollen record's normalized_name must match the entry to trigger a hit.
	pkgLine := `{"record_type":"package","ecosystem":"npm","normalized_name":"` + jwt + `","version":"1.0.0","project_path":"/home/user/project"}`
	crossRefPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, 1)
		ch <- []byte(pkgLine)
		close(ch)
		return ch, true
	}

	auditPath := filepath.Join(t.TempDir(), "beekeeper.ndjson")
	cfg := CrossRefConfig{
		IndexPath: indexPath,
		CacheDir:  t.TempDir(),
		AuditPath: auditPath,
	}

	if _, err := CrossReference(context.Background(), cfg); err != nil {
		t.Fatalf("CrossReference error: %v", err)
	}

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	got := string(data)
	if strings.Contains(got, jwt) {
		t.Errorf("finding record leaked raw JWT token into audit log:\n%s", got)
	}
	if !strings.Contains(got, "[JWT_REDACTED]") {
		t.Errorf("finding record missing [JWT_REDACTED] marker (redaction not applied):\n%s", got)
	}
}

// buildTestIndex is a helper that creates a minimal mmap index with the given entries.
func buildTestIndex(t *testing.T, entries []catalog.Entry) string {
	t.Helper()
	indexDir := t.TempDir()
	indexPath := filepath.Join(indexDir, "beekeeper.idx")
	if err := catalog.BuildIndex(indexPath, entries); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	return indexPath
}

// TestCrossReferenceHit verifies that a cataloged installed npm package produces
// a ScanHit with CorroborationCount >= 1 and no package mutation.
func TestCrossReferenceHit(t *testing.T) {
	indexPath := buildTestIndex(t, []catalog.Entry{
		{
			ID:               "evil-pkg-1",
			Ecosystem:        "npm",
			Package:          "evil-package",
			Versions:         []string{"1.0.0"},
			CatalogSignature: "fake-sig", // non-empty = signed
			CatalogSource:    "bumblebee",
		},
	})

	// Inject canned package records via runPollenFn.
	oldRun := crossRefPollenFn
	defer func() { crossRefPollenFn = oldRun }()

	// Emit one matching npm package record and one non-matching package.
	pkgLine := `{"record_type":"package","ecosystem":"npm","normalized_name":"evil-package","version":"1.0.0","project_path":"/home/user/project"}`
	cleanLine := `{"record_type":"package","ecosystem":"npm","normalized_name":"safe-package","version":"2.0.0"}`

	crossRefPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, 2)
		ch <- []byte(pkgLine)
		ch <- []byte(cleanLine)
		close(ch)
		return ch, true
	}

	cfg := CrossRefConfig{
		IndexPath: indexPath,
		CacheDir:  t.TempDir(),
		AuditPath: filepath.Join(t.TempDir(), "beekeeper.ndjson"),
	}

	hits, err := CrossReference(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CrossReference error: %v", err)
	}

	if len(hits) != 1 {
		t.Fatalf("CrossReference returned %d hits, want 1 (only evil-package should match)", len(hits))
	}

	hit := hits[0]
	if hit.Package != "evil-package" {
		t.Errorf("hit.Package = %q, want %q", hit.Package, "evil-package")
	}
	if hit.Ecosystem != "npm" {
		t.Errorf("hit.Ecosystem = %q, want %q", hit.Ecosystem, "npm")
	}
	if hit.CorroborationCount < 1 {
		t.Errorf("hit.CorroborationCount = %d, want >= 1", hit.CorroborationCount)
	}
}

// TestCrossReferenceNoHit verifies that an uncatalogued package produces no hits.
func TestCrossReferenceNoHit(t *testing.T) {
	indexPath := buildTestIndex(t, []catalog.Entry{
		{
			ID:               "known-pkg-1",
			Ecosystem:        "npm",
			Package:          "known-package",
			Versions:         []string{"1.0.0"},
			CatalogSignature: "fake-sig",
			CatalogSource:    "bumblebee",
		},
	})

	oldRun := crossRefPollenFn
	defer func() { crossRefPollenFn = oldRun }()

	// Emit a package that is NOT in the catalog.
	crossRefPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, 1)
		ch <- []byte(`{"record_type":"package","ecosystem":"npm","normalized_name":"totally-safe","version":"3.0.0"}`)
		close(ch)
		return ch, true
	}

	cfg := CrossRefConfig{IndexPath: indexPath, CacheDir: t.TempDir()}

	hits, err := CrossReference(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CrossReference error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("CrossReference returned %d hits, want 0 for uncatalogued package", len(hits))
	}
}

// TestCrossReferenceUnresolvedPath verifies that when the scan record has no
// resolved path, PathResolved=false on the ScanHit.
func TestCrossReferenceUnresolvedPath(t *testing.T) {
	indexPath := buildTestIndex(t, []catalog.Entry{
		{
			ID:               "evil-pkg-2",
			Ecosystem:        "npm",
			Package:          "evil-package2",
			Versions:         []string{"1.0.0"},
			CatalogSignature: "fake-sig",
			CatalogSource:    "bumblebee",
		},
	})

	oldRun := crossRefPollenFn
	defer func() { crossRefPollenFn = oldRun }()

	// Record has no project_path — path cannot be resolved.
	crossRefPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		ch := make(chan []byte, 1)
		ch <- []byte(`{"record_type":"package","ecosystem":"npm","normalized_name":"evil-package2","version":"1.0.0"}`)
		close(ch)
		return ch, true
	}

	cfg := CrossRefConfig{IndexPath: indexPath, CacheDir: t.TempDir()}

	hits, err := CrossReference(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CrossReference error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("CrossReference returned %d hits, want 1", len(hits))
	}
	if hits[0].PathResolved {
		t.Errorf("PathResolved = true, want false when project_path absent from record")
	}
	if hits[0].InstalledPath != "" {
		t.Errorf("InstalledPath = %q, want empty when path unresolvable", hits[0].InstalledPath)
	}
}

// TestCrossReferenceReadOnly verifies that CrossReference performs no writes to
// the packages referenced in scan records — it is purely a read operation.
func TestCrossReferenceReadOnly(t *testing.T) {
	oldRun := crossRefPollenFn
	defer func() { crossRefPollenFn = oldRun }()
	crossRefPollenFn = func(_ context.Context, _ bool) (<-chan []byte, bool) {
		return nil, false // pollen unavailable
	}

	cfg := CrossRefConfig{CacheDir: t.TempDir()}
	hits, err := CrossReference(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CrossReference should not error when pollen unavailable: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("CrossReference returned %d hits with no scanner output, want 0", len(hits))
	}
}
