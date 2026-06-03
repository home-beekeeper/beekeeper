package catalog

import "path/filepath"

// ResolveHealthy reads the catalog watch state and returns false when the
// bumblebee source is marked Degraded by the watch daemon.
//
// cacheDir is the path to the catalogs cache directory (e.g. ~/.beekeeper/catalogs).
// state.json lives one level up: filepath.Dir(cacheDir)/state.json.
//
// Returns true (healthy) when:
//   - cacheDir is empty (e.g. test with no configured cache directory)
//   - state.json is missing (first run — no degradation recorded yet)
//   - state.json is unreadable (corrupt/locked — cannot confirm degradation)
//   - bumblebee source is not present in state.Sources (first sync not yet complete)
//   - bumblebee.Degraded is false (explicit healthy flag)
//
// Returns false only on confirmed degradation (bumblebee.Degraded == true),
// which is written by the watch daemon when catalog.CheckSanity reports Alert
// or Block at sync time.
//
// Rationale for defaulting to true on any read failure: inability to read the
// state file is NOT evidence of catalog degradation. Degradation is a
// positively-asserted flag written by the watch daemon. Defaulting healthy means
// severity escalation applies in the absence of evidence; only confirmed
// degradation (bumblebee.Degraded == true) suppresses escalation (CORR-02).
//
// Security note: this function defaults to healthy=true on any read failure
// (missing file, permissions error, parse error). An attacker who can make
// state.json unreadable (e.g. by removing read permission) will suppress
// degradation detection and re-enable severity escalation. This is a conscious
// trade-off: the watch daemon and the check handler run under the same user
// account, so an attacker with permission to modify state.json already has
// file-system access broader than Beekeeper's trust boundary. Verify that
// ~/.beekeeper/ has owner-only permissions (0o700) on installation.
//
// This function performs I/O and therefore lives in internal/catalog (the I/O
// tier), NOT in internal/policy (which is a pure function library with no I/O).
// All caller-tier packages (internal/check, internal/gateway, internal/watch,
// internal/scan) call this single implementation to avoid divergence.
func ResolveHealthy(cacheDir string) bool {
	if cacheDir == "" {
		return true
	}
	statePath := filepath.Join(filepath.Dir(cacheDir), "state.json")
	state, err := LoadState(statePath)
	if err != nil {
		return true // missing/unreadable state → assume healthy (fail open on read, not on security)
	}
	if src, ok := state.Sources["bumblebee"]; ok {
		return !src.Degraded // Degraded=true iff sanity check failed at last sync
	}
	return true // bumblebee not in state yet (first run) → assume healthy
}
