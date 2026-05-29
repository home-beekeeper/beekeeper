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

// TestSelfQuarantineState_Persistence verifies that a WatchState with an active
// SelfQuarantineState round-trips through SaveState → LoadState without data loss.
// It also verifies backward compatibility: a WatchState without SelfQuarantine
// round-trips correctly (field remains nil after load).
func TestSelfQuarantineState_Persistence(t *testing.T) {
	t.Run("with self quarantine", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")

		original := WatchState{
			Sources: map[string]SourceState{
				"bumblebee": {Hash: "abc123", Count: 100},
			},
			SelfQuarantine: &SelfQuarantineState{
				Version: "v0.4.2",
				EntryID: "beekeeper-self-2026-001",
				Reason:  "Beekeeper v0.4.2 release pipeline compromise",
				FiredAt: "2026-05-29T12:00:00Z",
			},
		}

		if err := SaveState(path, original); err != nil {
			t.Fatalf("SaveState: %v", err)
		}

		loaded, err := LoadState(path)
		if err != nil {
			t.Fatalf("LoadState: %v", err)
		}

		if loaded.SelfQuarantine == nil {
			t.Fatal("SelfQuarantine must not be nil after round-trip")
		}
		if loaded.SelfQuarantine.Version != "v0.4.2" {
			t.Errorf("Version: want %q, got %q", "v0.4.2", loaded.SelfQuarantine.Version)
		}
		if loaded.SelfQuarantine.EntryID != "beekeeper-self-2026-001" {
			t.Errorf("EntryID: want %q, got %q", "beekeeper-self-2026-001", loaded.SelfQuarantine.EntryID)
		}
		if loaded.SelfQuarantine.Reason != "Beekeeper v0.4.2 release pipeline compromise" {
			t.Errorf("Reason: want %q, got %q", "Beekeeper v0.4.2 release pipeline compromise", loaded.SelfQuarantine.Reason)
		}
		if loaded.SelfQuarantine.FiredAt != "2026-05-29T12:00:00Z" {
			t.Errorf("FiredAt: want %q, got %q", "2026-05-29T12:00:00Z", loaded.SelfQuarantine.FiredAt)
		}
		// Ensure Sources also survived.
		if len(loaded.Sources) != 1 {
			t.Errorf("Sources len: want 1, got %d", len(loaded.Sources))
		}
	})

	t.Run("backward compatible — no self quarantine field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")

		// Write state.json without a self_quarantine key — simulates an existing
		// pre-Phase-9 state.json that was written before this field existed.
		rawJSON := []byte(`{"sources":{"bumblebee":{"hash":"abc","count":10}}}`)
		if err := writeFileAtomic(path, rawJSON); err != nil {
			t.Fatalf("writeFileAtomic: %v", err)
		}

		loaded, err := LoadState(path)
		if err != nil {
			t.Fatalf("LoadState: %v", err)
		}
		if loaded.SelfQuarantine != nil {
			t.Errorf("SelfQuarantine must be nil for pre-Phase-9 state.json, got %+v", loaded.SelfQuarantine)
		}
		if loaded.Sources["bumblebee"].Hash != "abc" {
			t.Errorf("Sources[bumblebee].Hash: want %q, got %q", "abc", loaded.Sources["bumblebee"].Hash)
		}
	})

	t.Run("no self quarantine field omitted on marshal", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")

		st := WatchState{
			Sources:        map[string]SourceState{},
			SelfQuarantine: nil, // should be omitted
		}
		if err := SaveState(path, st); err != nil {
			t.Fatalf("SaveState: %v", err)
		}

		loaded, err := LoadState(path)
		if err != nil {
			t.Fatalf("LoadState: %v", err)
		}
		if loaded.SelfQuarantine != nil {
			t.Errorf("nil SelfQuarantine must round-trip as nil, got %+v", loaded.SelfQuarantine)
		}
	})
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
