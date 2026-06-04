package nudge

import "testing"

// TestMeetsFloor covers the major.minor floor comparison behavior.
func TestMeetsFloor(t *testing.T) {
	tests := []struct {
		have  string
		floor string
		want  bool
	}{
		{"11.5.1", "11.0.0", true},
		{"10.9.0", "11.0.0", false},
		{"1.3.14", "1.3.0", true},
		{"1.2.99", "1.3.0", false},
		{"21.0.0", "22.0.0", false},
		{"22.0.0", "22.0.0", true},
		{"22.1.0", "22.0.0", true},
		{"23.0.0", "22.0.0", true},
		// Edge cases
		{"", "11.0.0", false},  // empty "have" → does not meet floor
		{"11.0.0", "", true},   // empty floor → always meets (no restriction)
		{"bad", "11.0.0", false},
	}
	for _, tc := range tests {
		got := meetsFloor(tc.have, tc.floor)
		if got != tc.want {
			t.Errorf("meetsFloor(%q, %q) = %v, want %v", tc.have, tc.floor, got, tc.want)
		}
	}
}

// TestIsMajorDrift covers the private and exported drift predicates.
func TestIsMajorDrift(t *testing.T) {
	tests := []struct {
		latest string
		floor  string
		want   bool
	}{
		{"12.0.0", "11.0.0", true},  // §10-15
		{"11.5.1", "11.0.0", false}, // same major
		{"11.0.0", "11.0.0", false}, // exact equal
		{"10.0.0", "11.0.0", false}, // latest below floor — not a forward drift
		{"2.0.0", "1.3.0", true},
		{"1.9.0", "1.3.0", false},
		{"", "11.0.0", false},
		{"12.0.0", "", false},
	}
	for _, tc := range tests {
		// Private function
		got := isMajorDrift(tc.latest, tc.floor)
		if got != tc.want {
			t.Errorf("isMajorDrift(%q, %q) = %v, want %v", tc.latest, tc.floor, got, tc.want)
		}
		// Exported wrapper must return identically
		gotExp := IsMajorDrift(tc.latest, tc.floor)
		if gotExp != got {
			t.Errorf("IsMajorDrift(%q, %q) = %v, private returned %v — wrapper mismatch", tc.latest, tc.floor, gotExp, got)
		}
	}
}
