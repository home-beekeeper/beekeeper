package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeStateJSON writes a WatchState as JSON to stateFile, creating parent dirs.
func writeStateJSON(t *testing.T, stateFile string, st WatchState) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o700); err != nil {
		t.Fatalf("writeStateJSON: mkdir: %v", err)
	}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("writeStateJSON: marshal: %v", err)
	}
	if err := os.WriteFile(stateFile, data, 0o600); err != nil {
		t.Fatalf("writeStateJSON: write: %v", err)
	}
}

// TestResolveHealthyEmptyCacheDir verifies that an empty cacheDir returns true.
func TestResolveHealthyEmptyCacheDir(t *testing.T) {
	if !ResolveHealthy("") {
		t.Error("ResolveHealthy(\"\") = false, want true")
	}
}

// TestResolveHealthyMissingStateFile verifies that a missing state.json returns true.
func TestResolveHealthyMissingStateFile(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "catalogs")
	// state.json is not created — simulates first run.
	if !ResolveHealthy(cacheDir) {
		t.Error("ResolveHealthy with missing state.json = false, want true (first-run default)")
	}
}

// TestResolveHealthyBumblebeeDegraded verifies that a degraded bumblebee source
// causes ResolveHealthy to return false.
func TestResolveHealthyBumblebeeDegraded(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "catalogs")
	stateFile := filepath.Join(dir, "state.json")

	writeStateJSON(t, stateFile, WatchState{
		Sources: map[string]SourceState{
			"bumblebee": {
				Hash:           "abc123",
				Count:          50000,
				Degraded:       true,
				DegradedReason: "delta 12000 exceeds hard limit 10000",
			},
		},
	})

	if ResolveHealthy(cacheDir) {
		t.Error("ResolveHealthy with degraded bumblebee = true, want false")
	}
}

// TestResolveHealthyBumblebeeHealthy verifies that a healthy bumblebee source
// (and no other sources) returns true.
func TestResolveHealthyBumblebeeHealthy(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "catalogs")
	stateFile := filepath.Join(dir, "state.json")

	writeStateJSON(t, stateFile, WatchState{
		Sources: map[string]SourceState{
			"bumblebee": {
				Hash:  "abc123",
				Count: 5000,
			},
		},
	})

	if !ResolveHealthy(cacheDir) {
		t.Error("ResolveHealthy with healthy bumblebee = false, want true")
	}
}

// TestResolveHealthyNoSources verifies that an empty state.Sources map
// returns true (no positively-asserted degradation).
func TestResolveHealthyNoSources(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "catalogs")
	stateFile := filepath.Join(dir, "state.json")

	writeStateJSON(t, stateFile, WatchState{
		Sources: map[string]SourceState{},
	})

	if !ResolveHealthy(cacheDir) {
		t.Error("ResolveHealthy with empty state.Sources = false, want true (no degradation recorded)")
	}
}
