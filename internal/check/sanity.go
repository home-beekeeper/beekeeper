package check

import (
	"path/filepath"

	"github.com/bantuson/beekeeper/internal/catalog"
)

// resolveCatalogHealthy reads the catalog watch state and returns false when
// the bumblebee source is marked Degraded by the watch daemon.
//
// catalog.LoadState takes a full path to state.json (not a directory).
// The state file lives at filepath.Dir(cacheDir)/state.json — e.g. if cacheDir
// is ~/.beekeeper/catalogs, state.json is at ~/.beekeeper/state.json.
//
// Returns true (healthy) when:
//   - cacheDir is empty (e.g. test with no configured cache directory)
//   - state file is missing (first run — no degradation recorded yet)
//   - state file is unreadable (corrupt/locked — cannot confirm degradation)
//   - bumblebee source is not present in state.Sources (first sync not yet complete)
//   - bumblebee.Degraded is false (explicit healthy flag)
//
// Returns false only on confirmed degradation (bumblebee.Degraded == true),
// which is written by the watch daemon when catalog.CheckSanity reports Alert
// or Block at sync time.
//
// Rationale for defaulting to true: inability to read the state file is NOT
// evidence of catalog degradation. Degradation is a positively-asserted flag.
// Defaulting healthy means escalation applies in the absence of evidence; only
// confirmed degradation suppresses severity escalation (CORR-02).
//
// This function performs I/O and therefore lives in the caller tier (internal/check),
// NOT in internal/policy (which is a pure function library with no I/O).
func resolveCatalogHealthy(cacheDir string) bool {
	if cacheDir == "" {
		return true
	}
	statePath := filepath.Join(filepath.Dir(cacheDir), "state.json")
	state, err := catalog.LoadState(statePath)
	if err != nil {
		return true // missing/unreadable state → assume healthy
	}
	if src, ok := state.Sources["bumblebee"]; ok {
		return !src.Degraded // Degraded=true iff sanity check failed at last sync
	}
	return true // bumblebee not in state yet (first run) → assume healthy
}
