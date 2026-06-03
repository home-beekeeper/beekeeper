package watch

import (
	"path/filepath"

	"github.com/bantuson/beekeeper/internal/catalog"
)

// resolveCatalogHealthy reads the catalog watch state and returns false when
// the bumblebee source is marked Degraded by the watch daemon.
//
// catalog.LoadState takes a full path to state.json (not a directory).
// The state file lives at filepath.Dir(cacheDir)/state.json.
//
// Returns true (healthy) when: cacheDir is empty, state file is missing,
// state file is unreadable, bumblebee is absent from state.Sources, or
// bumblebee.Degraded is false.
// Returns false only on confirmed degradation (bumblebee.Degraded == true).
//
// Rationale: inability to read state is NOT evidence of degradation.
// Escalation applies in the absence of evidence; only confirmed degradation
// suppresses severity escalation (CORR-02).
//
// This function performs I/O and therefore lives in the caller tier (internal/watch),
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
