package catalog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// makeOverlayEntry returns a minimal catalog Entry suitable for overlay tests.
// CatalogSignature is always "" (unsigned — overlay entries are warn-only per CTLG-07).
// CatalogSource is always "local-overlay".
func makeOverlayEntry(ecosystem, pkg string) Entry {
	return Entry{
		ID:               "local-overlay-" + ecosystem + "-" + pkg,
		Name:             pkg,
		Ecosystem:        ecosystem,
		Package:          pkg,
		Versions:         []string{"17.3.0"},
		Severity:         "critical",
		SourceURL:        "",
		CatalogSignature: "", // unsigned — local-overlay entries are warn-only per CTLG-07
		CatalogSource:    "local-overlay",
	}
}

// TestLocalOverlaySurvivesSync verifies that after AddLocalOverlayEntry writes
// local-overlay.json and local-overlay.idx, simulating a SyncConditional by
// writing only bumblebee.json and bumblebee.idx leaves both overlay files
// byte-unchanged.
func TestLocalOverlaySurvivesSync(t *testing.T) {
	dir := t.TempDir()
	e := makeOverlayEntry("npm", "@nrwl/nx-console")

	// Add the overlay entry.
	if err := AddLocalOverlayEntry(dir, e); err != nil {
		t.Fatalf("AddLocalOverlayEntry: %v", err)
	}

	// Read overlay files before simulated sync.
	jsonPath := filepath.Join(dir, "local-overlay.json")
	idxPath := filepath.Join(dir, "local-overlay.idx")

	jsonBefore, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read local-overlay.json before sync: %v", err)
	}
	idxBefore, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read local-overlay.idx before sync: %v", err)
	}

	// Simulate SyncConditional: write only bumblebee.json and bumblebee.idx.
	// SyncConditional only writes these two files (VERIFIED: sync.go lines 138-143).
	if err := os.WriteFile(filepath.Join(dir, "bumblebee.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatalf("simulate bumblebee.json write: %v", err)
	}
	bbEntry := Entry{
		ID:            "bb-sync-test",
		Ecosystem:     "npm",
		Package:       "some-other-pkg",
		CatalogSource: "bumblebee",
	}
	if err := BuildIndex(filepath.Join(dir, "bumblebee.idx"), []Entry{bbEntry}); err != nil {
		t.Fatalf("simulate bumblebee.idx rebuild: %v", err)
	}

	// Verify overlay files are byte-unchanged after the simulated sync.
	jsonAfter, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read local-overlay.json after sync: %v", err)
	}
	idxAfter, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read local-overlay.idx after sync: %v", err)
	}

	if string(jsonBefore) != string(jsonAfter) {
		t.Error("local-overlay.json was modified by simulated SyncConditional")
	}
	if string(idxBefore) != string(idxAfter) {
		t.Error("local-overlay.idx was modified by simulated SyncConditional")
	}
}

// TestLocalOverlayFilePermissions verifies that after AddLocalOverlayEntry both
// local-overlay.json and local-overlay.idx are 0600 on non-Windows systems.
// On Windows, SetOwnerOnly applies a DACL (not a POSIX mode), so the POSIX mode
// check is skipped with a structured reason.
func TestLocalOverlayFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows owner-DACL not a POSIX mode; SetOwnerOnly covers it")
	}

	dir := t.TempDir()
	e := makeOverlayEntry("npm", "@nrwl/nx-console")
	if err := AddLocalOverlayEntry(dir, e); err != nil {
		t.Fatalf("AddLocalOverlayEntry: %v", err)
	}

	for _, name := range []string{"local-overlay.json", "local-overlay.idx"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		mode := info.Mode().Perm()
		if mode != 0o600 {
			t.Errorf("%s permissions = %04o, want 0600", name, mode)
		}
	}
}

// TestMultiIndexQueriesOverlay verifies that a MultiIndex built via
// NewMultiIndexWithOverlay returns at least one match with CatalogSource
// "local-overlay" when the overlay contains the queried package.
func TestMultiIndexQueriesOverlay(t *testing.T) {
	dir := t.TempDir()
	e := makeOverlayEntry("npm", "@nrwl/nx-console")
	if err := AddLocalOverlayEntry(dir, e); err != nil {
		t.Fatalf("AddLocalOverlayEntry: %v", err)
	}

	overlayPath := filepath.Join(dir, "local-overlay.idx")
	// nil bumblebee — overlay is the only source.
	mi := NewMultiIndexWithOverlay(nil, nil, nil, overlayPath)
	if mi == nil {
		t.Fatal("NewMultiIndexWithOverlay returned nil")
	}
	defer mi.Close()

	got := mi.LookupAll("npm", "@nrwl/nx-console")
	if len(got) == 0 {
		t.Fatal("LookupAll returned no matches; want >= 1 from local-overlay")
	}
	found := false
	for _, m := range got {
		if m.CatalogSource == "local-overlay" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no match with CatalogSource==\"local-overlay\" in %+v", got)
	}
}

// TestLocalOverlayUnsignedIsWarnTier verifies that the overlay Entry written by
// AddLocalOverlayEntry carries CatalogSignature="" (unsigned), making it
// warn-only per CTLG-07. A single unsigned overlay source alone yields
// source_count:1 (not enough for enforce).
func TestLocalOverlayUnsignedIsWarnTier(t *testing.T) {
	dir := t.TempDir()
	e := makeOverlayEntry("npm", "@nrwl/nx-console")
	if err := AddLocalOverlayEntry(dir, e); err != nil {
		t.Fatalf("AddLocalOverlayEntry: %v", err)
	}

	// Load the overlay back and confirm CatalogSignature is empty.
	entries, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("LoadLocalOverlay returned empty; want 1 entry")
	}
	for _, entry := range entries {
		if entry.CatalogSignature != "" {
			t.Errorf("overlay entry CatalogSignature = %q, want \"\" (unsigned → warn-only)", entry.CatalogSignature)
		}
	}

	// Verify via MultiIndex: single overlay source = no signed source → source_count
	// in corroborate is 0 signed sources = warn tier only. We verify the LookupAll
	// result has Signed=false (the policy engine uses this to determine signed count).
	overlayPath := filepath.Join(dir, "local-overlay.idx")
	mi := NewMultiIndexWithOverlay(nil, nil, nil, overlayPath)
	defer mi.Close()

	got := mi.LookupAll("npm", "@nrwl/nx-console")
	if len(got) == 0 {
		t.Fatal("LookupAll returned no matches")
	}
	for _, m := range got {
		if m.CatalogSource == "local-overlay" && m.Signed {
			t.Errorf("overlay match should have Signed=false (unsigned entry), got Signed=true")
		}
	}
}

// TestLocalOverlayPlusBumblebeeIsEnforce verifies that when both bumblebee and
// the local overlay contain the same package, LookupAll returns matches from
// both "bumblebee" and "local-overlay" CatalogSource names. The corroboration
// engine will count these as 2 distinct sources → enforce tier. This test
// asserts the two-source shape at the MultiIndex layer only.
func TestLocalOverlayPlusBumblebeeIsEnforce(t *testing.T) {
	dir := t.TempDir()

	// Build bumblebee index with the same package.
	bbEntry := Entry{
		ID:               "bb-nx-001",
		Name:             "@nrwl/nx-console",
		Ecosystem:        "npm",
		Package:          "@nrwl/nx-console",
		Versions:         []string{"17.3.0"},
		Severity:         "critical",
		CatalogSignature: "sig-abc123", // signed
		CatalogSource:    "bumblebee",
	}
	bbIdx := buildTestIndexWithEntry(t, bbEntry)

	// Build local overlay with the same package.
	e := makeOverlayEntry("npm", "@nrwl/nx-console")
	if err := AddLocalOverlayEntry(dir, e); err != nil {
		t.Fatalf("AddLocalOverlayEntry: %v", err)
	}

	overlayPath := filepath.Join(dir, "local-overlay.idx")
	mi := NewMultiIndexWithOverlay(bbIdx, nil, nil, overlayPath)
	defer mi.Close()

	got := mi.LookupAll("npm", "@nrwl/nx-console")

	sources := map[string]bool{}
	for _, m := range got {
		if !m.Dissented {
			sources[m.CatalogSource] = true
		}
	}
	if !sources["bumblebee"] {
		t.Error("expected bumblebee match; not found in LookupAll result")
	}
	if !sources["local-overlay"] {
		t.Error("expected local-overlay match; not found in LookupAll result")
	}
	if len(sources) < 2 {
		t.Errorf("want >= 2 distinct sources (bumblebee + local-overlay), got %v", sources)
	}
}

// TestLocalOverlayIdempotentAdd verifies that calling AddLocalOverlayEntry twice
// with the same ecosystem+package results in exactly one entry in LoadLocalOverlay.
func TestLocalOverlayIdempotentAdd(t *testing.T) {
	dir := t.TempDir()
	e := makeOverlayEntry("npm", "@nrwl/nx-console")

	if err := AddLocalOverlayEntry(dir, e); err != nil {
		t.Fatalf("first AddLocalOverlayEntry: %v", err)
	}
	if err := AddLocalOverlayEntry(dir, e); err != nil {
		t.Fatalf("second AddLocalOverlayEntry: %v", err)
	}

	entries, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 entry after idempotent add, got %d: %+v", len(entries), entries)
	}
}
