package catalog

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func nxEntries() []Entry {
	return []Entry{
		{
			ID:            "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised",
			Name:          "nrwl.angular-console",
			Ecosystem:     "editor-extension",
			Package:       "nrwl.angular-console",
			Versions:      []string{"18.95.0"},
			Severity:      "critical",
			CatalogSource: "bumblebee",
		},
		{
			ID:            "beekeeper-test-clean-npm-pkg",
			Name:          "some-internal-test-pkg",
			Ecosystem:     "npm",
			Package:       "some-internal-test-pkg",
			Versions:      []string{"1.0.0"},
			Severity:      "low",
			CatalogSource: "bumblebee",
		},
	}
}

func TestIndexBuildAndOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.idx")
	entries := nxEntries()
	if err := BuildIndex(path, entries); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	idx, err := OpenIndex(path)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer idx.Close()

	if idx.Count() != len(entries) {
		t.Errorf("Count() = %d, want %d", idx.Count(), len(entries))
	}
}

func TestIndexLookupHit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.idx")
	if err := BuildIndex(path, nxEntries()); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	idx, err := OpenIndex(path)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer idx.Close()

	e, ok := idx.Lookup("editor-extension", "nrwl.angular-console")
	if !ok {
		t.Fatal("Lookup(editor-extension, nrwl.angular-console): ok=false, want true")
	}
	if e.Severity != "critical" {
		t.Errorf("Severity = %q, want critical", e.Severity)
	}
	if len(e.Versions) != 1 || e.Versions[0] != "18.95.0" {
		t.Errorf("Versions = %v, want [18.95.0]", e.Versions)
	}

	// Case-insensitive package match.
	if _, ok := idx.Lookup("editor-extension", "NRWL.Angular-Console"); !ok {
		t.Error("Lookup with mixed-case package: ok=false, want true (case-insensitive)")
	}
}

func TestIndexLookupMiss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.idx")
	if err := BuildIndex(path, nxEntries()); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	idx, err := OpenIndex(path)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer idx.Close()

	if _, ok := idx.Lookup("npm", "express"); ok {
		t.Error("Lookup(npm, express): ok=true, want false")
	}
	// Right package, wrong ecosystem must miss (key includes ecosystem).
	if _, ok := idx.Lookup("npm", "nrwl.angular-console"); ok {
		t.Error("Lookup(npm, nrwl.angular-console): ok=true, want false (wrong ecosystem)")
	}
}

func TestIndexBinarySearchManyEntries(t *testing.T) {
	const n = 250
	entries := make([]Entry, n)
	for i := 0; i < n; i++ {
		entries[i] = Entry{
			ID:        fmt.Sprintf("id-%04d", i),
			Ecosystem: "npm",
			Package:   fmt.Sprintf("pkg-%04d", i),
			Versions:  []string{"1.0.0"},
			Severity:  "high",
		}
	}

	path := filepath.Join(t.TempDir(), "many.idx")
	if err := BuildIndex(path, entries); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	idx, err := OpenIndex(path)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	defer idx.Close()

	if idx.Count() != n {
		t.Fatalf("Count() = %d, want %d", idx.Count(), n)
	}

	rng := rand.New(rand.NewSource(42))
	// Random hits.
	for i := 0; i < 50; i++ {
		want := rng.Intn(n)
		pkg := fmt.Sprintf("pkg-%04d", want)
		e, ok := idx.Lookup("npm", pkg)
		if !ok {
			t.Fatalf("Lookup(npm, %s): ok=false, want true", pkg)
		}
		if e.ID != fmt.Sprintf("id-%04d", want) {
			t.Fatalf("Lookup(npm, %s).ID = %q, want id-%04d", pkg, e.ID, want)
		}
	}
	// Random misses.
	for i := 0; i < 50; i++ {
		pkg := fmt.Sprintf("missing-%06d", rng.Intn(1_000_000))
		if _, ok := idx.Lookup("npm", pkg); ok {
			t.Fatalf("Lookup(npm, %s): ok=true, want false", pkg)
		}
	}
}

// TestIndexOpenDoesNotReadJSON proves HOOK-02: the read path mmaps only the
// .idx file and never touches the source catalog JSON. We build the index,
// delete every JSON file in the directory, and assert OpenIndex + Lookup still
// resolve correctly.
func TestIndexOpenDoesNotReadJSON(t *testing.T) {
	dir := t.TempDir()
	idxPath := filepath.Join(dir, "bumblebee.idx")
	jsonPath := filepath.Join(dir, "bumblebee.json")

	if err := os.WriteFile(jsonPath, []byte(`{"schema_version":"0.1.0","entries":[]}`), 0o600); err != nil {
		t.Fatalf("write source json: %v", err)
	}
	if err := BuildIndex(idxPath, nxEntries()); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	// Remove the source JSON entirely.
	if err := os.Remove(jsonPath); err != nil {
		t.Fatalf("remove source json: %v", err)
	}

	idx, err := OpenIndex(idxPath)
	if err != nil {
		t.Fatalf("OpenIndex after deleting source json: %v", err)
	}
	defer idx.Close()

	if _, ok := idx.Lookup("editor-extension", "nrwl.angular-console"); !ok {
		t.Fatal("Lookup after deleting source json: ok=false, want true (index is self-contained)")
	}
}

func TestOpenIndexRejectsBadMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.idx")
	// 16-byte header with wrong magic.
	bad := make([]byte, headerSize)
	if err := os.WriteFile(path, bad, 0o600); err != nil {
		t.Fatalf("write corrupt index: %v", err)
	}
	if _, err := OpenIndex(path); err == nil {
		t.Fatal("OpenIndex(bad magic): expected error, got nil")
	}
}
