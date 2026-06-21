package posture

// PURE — imports only "strconv" and "strings". No os/net/io/time/sync/context.
// Floor comparisons use simple major.minor.patch integer comparison (floors are
// major.minor gates only; a full semver lib is not needed and would add
// supply-chain footprint to a security tool).
//
// History: the steering-only drift predicate (isMajorDrift/IsMajorDrift) was
// removed with the nudge in v1.1.0; only the floor comparison the posture
// "hardened" flag needs survives here.

import (
	"strconv"
	"strings"
)

// meetsFloor reports whether have is at or above floor using a major.minor.patch
// integer comparison. A malformed version string (non-integer component) returns
// false so an empty or broken detection result never silently passes the floor
// check. An empty floor string returns true (no restriction).
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
