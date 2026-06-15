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

// ---- Coverage additions for LoadLocalOverlay + AddLocalOverlayEntry ----

// TestLoadLocalOverlay_MissingFile verifies that LoadLocalOverlay returns
// (nil, nil) when the overlay JSON file does not exist. This is the "first run"
// case: a missing file is not an error.
func TestLoadLocalOverlay_MissingFile(t *testing.T) {
	dir := t.TempDir() // empty directory — local-overlay.json absent

	entries, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay on missing file returned error: %v", err)
	}
	if entries != nil {
		t.Errorf("LoadLocalOverlay on missing file returned non-nil entries: %+v", entries)
	}
}

// TestLoadLocalOverlay_MalformedJSON verifies that LoadLocalOverlay returns a
// parse error when the overlay file exists but contains invalid JSON.
func TestLoadLocalOverlay_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "local-overlay.json")
	if err := os.WriteFile(overlayPath, []byte(`not valid json`), 0o600); err != nil {
		t.Fatalf("write malformed overlay: %v", err)
	}

	_, err := LoadLocalOverlay(dir)
	if err == nil {
		t.Fatal("LoadLocalOverlay with malformed JSON returned nil error; want parse error")
	}
}

// TestLoadLocalOverlay_UnreadableFile covers the os.ReadFile error path that is
// NOT "not-exist" — specifically, a directory is placed at the overlay JSON path.
// On all supported OSes, os.ReadFile on a directory returns an error that is NOT
// os.ErrNotExist, exercising the "read failed, not absent" branch in
// LoadLocalOverlay.
func TestLoadLocalOverlay_UnreadableFile(t *testing.T) {
	dir := t.TempDir()
	overlayPath := filepath.Join(dir, "local-overlay.json")
	// Create a directory at the path where the JSON file is expected.
	// On Linux/macOS: returns EISDIR. On Windows: returns a non-ErrNotExist error.
	// Both are distinct from os.ErrNotExist and exercise the read-error branch.
	if err := os.Mkdir(overlayPath, 0o700); err != nil {
		t.Fatalf("mkdir at overlay path: %v", err)
	}

	_, err := LoadLocalOverlay(dir)
	if err == nil {
		t.Fatal("LoadLocalOverlay on unreadable path returned nil error; want read error")
	}
}

// TestLoadLocalOverlay_ValidLoad verifies that LoadLocalOverlay round-trips a
// file written by AddLocalOverlayEntry and returns all entries intact.
func TestLoadLocalOverlay_ValidLoad(t *testing.T) {
	dir := t.TempDir()

	entries := []Entry{
		makeOverlayEntry("npm", "evil-pkg-a"),
		makeOverlayEntry("pip", "evil-pkg-b"),
	}
	for _, e := range entries {
		if err := AddLocalOverlayEntry(dir, e); err != nil {
			t.Fatalf("AddLocalOverlayEntry(%s/%s): %v", e.Ecosystem, e.Package, err)
		}
	}

	got, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadLocalOverlay returned %d entries, want 2", len(got))
	}
	// Verify the ecosystems and packages were persisted correctly.
	ecosystems := map[string]bool{}
	for _, e := range got {
		ecosystems[e.Ecosystem] = true
	}
	if !ecosystems["npm"] || !ecosystems["pip"] {
		t.Errorf("missing expected ecosystems in loaded entries: %+v", got)
	}
}

// TestAddLocalOverlayEntry_LoadError verifies that AddLocalOverlayEntry returns
// the error from LoadLocalOverlay when the existing overlay file is malformed.
// This covers the "return err" branch at the top of AddLocalOverlayEntry.
func TestAddLocalOverlayEntry_LoadError(t *testing.T) {
	dir := t.TempDir()

	// Write a corrupt overlay file so LoadLocalOverlay inside AddLocalOverlayEntry fails.
	overlayPath := filepath.Join(dir, "local-overlay.json")
	if err := os.WriteFile(overlayPath, []byte(`{not valid`), 0o600); err != nil {
		t.Fatalf("write corrupt overlay: %v", err)
	}

	e := makeOverlayEntry("npm", "new-pkg")
	err := AddLocalOverlayEntry(dir, e)
	if err == nil {
		t.Fatal("AddLocalOverlayEntry with corrupt overlay returned nil error; want load error")
	}
}

// TestAddLocalOverlayEntry_UpdateExistingEntry verifies that calling
// AddLocalOverlayEntry with a different package in the same ecosystem is
// additive (the second entry is stored alongside the first, not replacing it).
// This also exercises the idempotency path where an entry with the same
// ecosystem+package is skipped on a case-insensitive comparison.
func TestAddLocalOverlayEntry_UpdateExistingEntry(t *testing.T) {
	dir := t.TempDir()

	// Add an entry.
	e1 := makeOverlayEntry("npm", "evil-pkg-one")
	if err := AddLocalOverlayEntry(dir, e1); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Add a second entry with the same ecosystem but a different package.
	e2 := makeOverlayEntry("npm", "evil-pkg-two")
	if err := AddLocalOverlayEntry(dir, e2); err != nil {
		t.Fatalf("second add: %v", err)
	}

	// Both entries must be present.
	entries, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d: %+v", len(entries), entries)
	}

	// Case-insensitive idempotency: adding the same package with different case
	// must NOT create a duplicate.
	e3 := makeOverlayEntry("NPM", "EVIL-PKG-ONE")
	if err := AddLocalOverlayEntry(dir, e3); err != nil {
		t.Fatalf("case-insensitive add: %v", err)
	}
	entries2, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay after case-insensitive add: %v", err)
	}
	if len(entries2) != 2 {
		t.Fatalf("case-insensitive idempotency failed: want 2 entries, got %d", len(entries2))
	}
}

// TestAddLocalOverlayEntry_CapEviction verifies the 1000-entry cap guard:
// adding an entry when the overlay already holds maxOverlayEntries is silently
// skipped (returns nil, logs a warning). The overlay remains at the cap and does
// not grow beyond it.
func TestAddLocalOverlayEntry_CapEviction(t *testing.T) {
	dir := t.TempDir()

	// Fill the overlay to the cap using unique packages.
	for i := 0; i < maxOverlayEntries; i++ {
		e := makeOverlayEntry("npm", filepath.Join("evil-pkg", string(rune('a'+(i%26)))+"-"+itoa(i)))
		if err := AddLocalOverlayEntry(dir, e); err != nil {
			t.Fatalf("fill entry %d: %v", i, err)
		}
	}

	// Verify we are exactly at the cap.
	entries, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay at cap: %v", err)
	}
	if len(entries) != maxOverlayEntries {
		t.Fatalf("want %d entries at cap, got %d", maxOverlayEntries, len(entries))
	}

	// Adding one more entry must be silently skipped (cap guard).
	overflow := makeOverlayEntry("npm", "overflow-entry")
	if err := AddLocalOverlayEntry(dir, overflow); err != nil {
		t.Fatalf("AddLocalOverlayEntry at cap returned unexpected error: %v", err)
	}

	// The overlay must remain at cap (overflow was discarded).
	afterCap, err := LoadLocalOverlay(dir)
	if err != nil {
		t.Fatalf("LoadLocalOverlay after cap overflow: %v", err)
	}
	if len(afterCap) != maxOverlayEntries {
		t.Fatalf("cap violated: want %d entries, got %d", maxOverlayEntries, len(afterCap))
	}
}

// itoa is a minimal int-to-string helper for TestAddLocalOverlayEntry_CapEviction,
// avoiding an import of strconv for a single-call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
