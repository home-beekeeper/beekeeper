package nudge

// PURE — imports only "strconv" and "strings". No os/net/io/time/sync/context.
// Floor comparisons use simple major.minor integer comparison (A3: floors are
// major.minor gates only; full semver lib is not needed and would add
// supply-chain footprint to a security tool).

import (
	"strconv"
	"strings"
)

// meetsFloor reports whether have is at or above floor using a major.minor
// integer comparison. Patch versions are compared only as a tiebreaker when
// major and minor are equal (a version with a larger patch always meets a
// floor with a smaller patch at the same major.minor).
//
// A malformed version string (non-integer component) returns false so an
// empty or broken detection result never silently passes the floor check.
// An empty floor string returns true (no restriction).
func meetsFloor(have, floor string) bool {
	if floor == "" {
		return true // no floor configured
	}
	if have == "" {
		return false // nothing detected
	}

	hParts := parseParts(have)
	fParts := parseParts(floor)
	if hParts == nil || fParts == nil {
		return false // malformed
	}

	// Compare major, then minor, then patch.
	for i := 0; i < 3; i++ {
		if hParts[i] > fParts[i] {
			return true
		}
		if hParts[i] < fParts[i] {
			return false
		}
	}
	return true // exactly equal
}

// isMajorDrift reports whether latest has a strictly higher major version than
// floor. Used for the weekly drift check (PRD §7.1, §10-15):
// isMajorDrift("12.0.0", "11.0.0") → true.
func isMajorDrift(latest, floor string) bool {
	if latest == "" || floor == "" {
		return false
	}
	lParts := parseParts(latest)
	fParts := parseParts(floor)
	if lParts == nil || fParts == nil {
		return false
	}
	return lParts[0] > fParts[0]
}

// IsMajorDrift is the EXPORTED wrapper over isMajorDrift. It exists so package
// gateway (Plan 06 drift.go) can call it across the package boundary. The
// lowercase private form is uncallable from another package and would re-block
// §10-15 at compile time (BLOCKER 2).
//
// IsMajorDrift(latest, floor string) returns true when the latest released
// version has a strictly higher major version than the configured floor,
// indicating that a new major version is available and a drift check should
// be recorded.
func IsMajorDrift(latest, floor string) bool {
	return isMajorDrift(latest, floor)
}

// parseParts parses a version string into a [3]int array: [major, minor, patch].
// Returns nil when any component is not a non-negative integer.
func parseParts(v string) *[3]int {
	// Strip a leading "v" (e.g. "v22.0.0").
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 4) // stop at 4 to detect malformed extra dots
	if len(parts) < 2 || len(parts) > 3 {
		return nil
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil
		}
		out[i] = n
	}
	return &out
}
