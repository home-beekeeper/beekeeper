//go:build linux

package linux

import "testing"

func TestProbeTierReturnsDegradationTier(t *testing.T) {
	tier := ProbeTier()
	if tier != Tier0 && tier != Tier1 && tier != Tier2 {
		t.Errorf("ProbeTier() returned unexpected value %d", tier)
	}
}

func TestTierString(t *testing.T) {
	for _, tier := range []DegradationTier{Tier0, Tier1, Tier2} {
		s := TierString(tier)
		if s == "" {
			t.Errorf("TierString(%d) returned empty string", tier)
		}
	}
}
