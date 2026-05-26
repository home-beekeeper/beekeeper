package catalog

import (
	"path/filepath"
	"testing"
)

func TestLoadStateMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "state.json")

	st, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState missing file: unexpected error: %v", err)
	}
	if st.Sources == nil {
		t.Fatal("LoadState missing file: Sources map must not be nil")
	}
	if len(st.Sources) != 0 {
		t.Fatalf("LoadState missing file: expected empty Sources map, got %d entries", len(st.Sources))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := WatchState{
		Sources: map[string]SourceState{
			"bumblebee": {
				Hash:  "abc123",
				Count: 654,
			},
			"osv": {
				Hash:           "def456",
				Count:          1200,
				Degraded:       true,
				DegradedReason: "delta 12000 exceeds hard limit 10000",
			},
		},
	}

	if err := SaveState(path, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if loaded.Sources == nil {
		t.Fatal("loaded Sources must not be nil")
	}
	if len(loaded.Sources) != len(original.Sources) {
		t.Fatalf("loaded Sources len: want %d, got %d", len(original.Sources), len(loaded.Sources))
	}

	bumblebee := loaded.Sources["bumblebee"]
	if bumblebee.Hash != "abc123" {
		t.Errorf("bumblebee Hash: want %q, got %q", "abc123", bumblebee.Hash)
	}
	if bumblebee.Count != 654 {
		t.Errorf("bumblebee Count: want 654, got %d", bumblebee.Count)
	}
	if bumblebee.Degraded {
		t.Errorf("bumblebee Degraded: want false, got true")
	}
	if bumblebee.DegradedReason != "" {
		t.Errorf("bumblebee DegradedReason: want empty, got %q", bumblebee.DegradedReason)
	}

	osv := loaded.Sources["osv"]
	if osv.Hash != "def456" {
		t.Errorf("osv Hash: want %q, got %q", "def456", osv.Hash)
	}
	if osv.Count != 1200 {
		t.Errorf("osv Count: want 1200, got %d", osv.Count)
	}
	if !osv.Degraded {
		t.Errorf("osv Degraded: want true, got false")
	}
	if osv.DegradedReason == "" {
		t.Errorf("osv DegradedReason: want non-empty, got empty")
	}
}

func TestSaveLoadRoundTripDegradedSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := WatchState{
		Sources: map[string]SourceState{
			"bumblebee": {
				Hash:           "poison-hash",
				Count:          50000,
				Degraded:       true,
				DegradedReason: "delta 12345 exceeds hard limit 10000",
			},
		},
	}

	if err := SaveState(path, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	ss := loaded.Sources["bumblebee"]
	if !ss.Degraded {
		t.Error("Degraded state must survive round-trip: want Degraded=true, got false")
	}
	if ss.DegradedReason == "" {
		t.Error("DegradedReason must survive round-trip: want non-empty, got empty")
	}
	if ss.Hash != "poison-hash" {
		t.Errorf("Hash: want %q, got %q", "poison-hash", ss.Hash)
	}
}

func TestSaveCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	// State file nested in non-existent subdirectories.
	path := filepath.Join(dir, "nested", "deeply", "state.json")

	st := WatchState{Sources: map[string]SourceState{}}
	if err := SaveState(path, st); err != nil {
		t.Fatalf("SaveState should create parent dirs: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState after nested save: %v", err)
	}
	if loaded.Sources == nil {
		t.Error("loaded Sources must not be nil")
	}
}

func TestLoadStateNilSourcesRepaired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Manually write a state.json with null sources to simulate a corrupt or
	// manually-edited file.
	rawJSON := []byte(`{"sources":null}`)
	if err := writeFileAtomic(path, rawJSON); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	st, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.Sources == nil {
		t.Error("LoadState must repair nil Sources to a non-nil map")
	}
}

func TestCatalogDeltaHasChanges(t *testing.T) {
	tests := []struct {
		name     string
		delta    CatalogDelta
		wantTrue bool
	}{
		{
			name: "hash changed",
			delta: CatalogDelta{
				Source:     "bumblebee",
				PrevHash:   "aaa",
				NewHash:    "bbb",
				PrevCount:  100,
				NewCount:   105,
				DeltaCount: 5,
			},
			wantTrue: true,
		},
		{
			name: "hash unchanged",
			delta: CatalogDelta{
				Source:     "bumblebee",
				PrevHash:   "aaa",
				NewHash:    "aaa",
				PrevCount:  100,
				NewCount:   100,
				DeltaCount: 0,
			},
			wantTrue: false,
		},
		{
			name: "first run (both empty)",
			delta: CatalogDelta{
				Source:     "bumblebee",
				PrevHash:   "",
				NewHash:    "abc123",
				PrevCount:  0,
				NewCount:   654,
				DeltaCount: 654,
			},
			wantTrue: true,
		},
		{
			name: "both empty (no-op)",
			delta: CatalogDelta{
				Source:     "bumblebee",
				PrevHash:   "",
				NewHash:    "",
				PrevCount:  0,
				NewCount:   0,
				DeltaCount: 0,
			},
			wantTrue: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.delta.HasChanges()
			if got != tc.wantTrue {
				t.Errorf("HasChanges(): want %v, got %v", tc.wantTrue, got)
			}
		})
	}
}
