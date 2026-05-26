package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// testdataDir resolves the repo-root testdata/catalog directory from within the
// internal/catalog package (two levels up: internal/catalog -> internal -> root).
func testdataDir() string {
	return filepath.Join("..", "..", "testdata", "catalog")
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

func TestCatalogParse(t *testing.T) {
	cf, err := ParseCatalogFile(readFixture(t, "nx-console.json"))
	if err != nil {
		t.Fatalf("ParseCatalogFile: unexpected error: %v", err)
	}
	if cf.SchemaVersion != SupportedSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", cf.SchemaVersion, SupportedSchemaVersion)
	}
	if len(cf.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(cf.Entries))
	}
	e := cf.Entries[0]
	if e.Ecosystem != "editor-extension" {
		t.Errorf("Ecosystem = %q, want %q", e.Ecosystem, "editor-extension")
	}
	if e.Package != "nrwl.angular-console" {
		t.Errorf("Package = %q, want %q", e.Package, "nrwl.angular-console")
	}
	if len(e.Versions) != 1 || e.Versions[0] != "18.95.0" {
		t.Errorf("Versions = %v, want [18.95.0]", e.Versions)
	}
	if e.Severity != "critical" {
		t.Errorf("Severity = %q, want %q", e.Severity, "critical")
	}
}

func TestUnknownSchemaVersion(t *testing.T) {
	data := []byte(`{"schema_version":"0.2.0","entries":[]}`)
	if _, err := ParseCatalogFile(data); err == nil {
		t.Fatal("ParseCatalogFile(schema_version 0.2.0): expected error, got nil")
	}
}

func TestRejectBareArray(t *testing.T) {
	data := []byte(`[{"id":"x","package":"y"}]`)
	if _, err := ParseCatalogFile(data); err == nil {
		t.Fatal("ParseCatalogFile(bare array): expected error, got nil")
	}
}

func TestDefaultCatalogSource(t *testing.T) {
	cf, err := ParseCatalogFile(readFixture(t, "nx-console.json"))
	if err != nil {
		t.Fatalf("ParseCatalogFile: unexpected error: %v", err)
	}
	if got := cf.Entries[0].CatalogSource; got != "bumblebee" {
		t.Errorf("CatalogSource = %q, want %q (defaulted)", got, "bumblebee")
	}
}
