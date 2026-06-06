package catalog

import "path/filepath"

// ResolveHealthy reads the catalog watch state and returns false when the
// bumblebee catalog source is marked Degraded by the watch daemon.
//
// cacheDir is the path to the catalogs cache directory (e.g. ~/.beekeeper/catalogs).
// state.json lives one level up: filepath.Dir(cacheDir)/state.json.
//
// Returns true (healthy) when:
//   - cacheDir is empty (e.g. test with no configured cache directory)
//   - state.json is missing (first run — no degradation recorded yet)
//   - state.json is unreadable (corrupt/locked — cannot confirm degradation)
//   - the bumblebee source is absent or not Degraded
//
// Returns false when the bumblebee source has Degraded == true, which is written
// by the watch daemon when catalog.CheckSanity reports Alert or Block at sync
// time (CORR-02).
//
// Why bumblebee-only (intentional — TM-B-01 reassessed 2026-06-06): bumblebee is
// the canonical, locally-SYNCED catalog and the ONLY source the watch daemon
// sanity-checks and records in state.Sources (see watch.go — the sole write is
// state.Sources[bumblebeeSource]). The OSV and Socket sources are per-request
// network adapters built fresh in multi.go; they are never synced or
// sanity-tracked, never carry a Degraded flag in state.json, and on error they
// degrade to NO-MATCH (returning nil) — so a degraded OSV/Socket contributes no
// match and therefore cannot drive a per-severity escalation in the first place.
// A blunt "any source degraded ⇒ suppress escalation" would let transient
// OSV/Socket unavailability (common offline / on Windows) suppress bumblebee's
// legitimate critical escalation, weakening true-positive detection. Escalation
// is correctly gated on the health of the canonical synced feed.
//
// Rationale for defaulting to true on any read failure: inability to read the
// state file is NOT evidence of catalog degradation. Degradation is a
// positively-asserted flag written by the watch daemon. Defaulting healthy means
// severity escalation applies in the absence of evidence; only confirmed
// degradation (bumblebee.Degraded == true) suppresses escalation.
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
	if src, ok := state.Sources[bumblebeeSource]; ok {
		return !src.Degraded // Degraded=true iff the canonical feed's sanity check failed at last sync
	}
	return true // bumblebee not in state yet (first run) → assume healthy
}
