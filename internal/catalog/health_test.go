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

// TestResolveHealthyOSVDegraded verifies that a sanity-degraded OSV source
// causes ResolveHealthy to return false (TM-B-01 conservative fix).
//
// Before this fix ResolveHealthy only checked the "bumblebee" source. A
// compromised critical-severity match from a single degraded OSV source could
// still drive a single-source block via the per-severity override path.
func TestResolveHealthyOSVDegraded(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "catalogs")
	stateFile := filepath.Join(dir, "state.json")

	writeStateJSON(t, stateFile, WatchState{
		Sources: map[string]SourceState{
			"bumblebee": {
				Hash:  "bumblebee-hash",
				Count: 5000,
				// Healthy — NOT degraded.
			},
			"osv": {
				Hash:           "osv-poisoned-hash",
				Count:          200000,
				Degraded:       true,
				DegradedReason: "total 200000 exceeds alert threshold 100000",
			},
		},
	})

	if ResolveHealthy(cacheDir) {
		t.Error("ResolveHealthy with degraded OSV source = true, want false (TM-B-01)")
	}
}

// TestResolveHealthySocketDegraded verifies that a sanity-degraded Socket source
// causes ResolveHealthy to return false (TM-B-01 conservative fix).
func TestResolveHealthySocketDegraded(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "catalogs")
	stateFile := filepath.Join(dir, "state.json")

	writeStateJSON(t, stateFile, WatchState{
		Sources: map[string]SourceState{
			"bumblebee": {
				Hash:  "bumblebee-hash",
				Count: 5000,
				// Healthy.
			},
			"socket": {
				Hash:           "socket-spiked-hash",
				Count:          120000,
				Degraded:       true,
				DegradedReason: "delta 15000 exceeds hard limit 10000",
			},
		},
	})

	if ResolveHealthy(cacheDir) {
		t.Error("ResolveHealthy with degraded Socket source = true, want false (TM-B-01)")
	}
}

// TestResolveHealthyAllSourcesDegraded verifies that when all sources are
// degraded the result is still false (not accidentally healthy).
func TestResolveHealthyAllSourcesDegraded(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "catalogs")
	stateFile := filepath.Join(dir, "state.json")

	writeStateJSON(t, stateFile, WatchState{
		Sources: map[string]SourceState{
			"bumblebee": {Degraded: true, DegradedReason: "delta exceeded"},
			"osv":       {Degraded: true, DegradedReason: "total exceeded"},
			"socket":    {Degraded: true, DegradedReason: "delta exceeded"},
		},
	})

	if ResolveHealthy(cacheDir) {
		t.Error("ResolveHealthy with all sources degraded = true, want false")
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
