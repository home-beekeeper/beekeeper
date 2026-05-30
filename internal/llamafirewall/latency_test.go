package llamafirewall

import (
	"testing"
)

// TestLatencyTrackerP95Empty verifies that P95() returns 0 on an empty tracker.
func TestLatencyTrackerP95Empty(t *testing.T) {
	var lt LatencyTracker
	if got := lt.P95(); got != 0 {
		t.Fatalf("P95() on empty tracker = %d, want 0", got)
	}
}

// TestLatencyTrackerP95Single records one value (100ms) and verifies P95() == 100.
func TestLatencyTrackerP95Single(t *testing.T) {
	var lt LatencyTracker
	lt.Record(100)
	if got := lt.P95(); got != 100 {
		t.Fatalf("P95() after single Record(100) = %d, want 100", got)
	}
}

// TestLatencyTrackerP95RingBuffer records 200 values: first 100 are 1ms, next 100
// are 99ms. Because the ring buffer holds only 100 samples the oldest 100 are
// evicted, so P95() is computed entirely over the last 100 samples (all 99ms) and
// must return 99.
func TestLatencyTrackerP95RingBuffer(t *testing.T) {
	var lt LatencyTracker
	for i := 0; i < 100; i++ {
		lt.Record(1)
	}
	for i := 0; i < 100; i++ {
		lt.Record(99)
	}
	got := lt.P95()
	if got != 99 {
		t.Fatalf("P95() after ring-buffer eviction = %d, want 99", got)
	}
}

// TestLatencyTrackerMean records [10, 20, 30, 40] and verifies Mean() == 25.0.
func TestLatencyTrackerMean(t *testing.T) {
	var lt LatencyTracker
	for _, v := range []int64{10, 20, 30, 40} {
		lt.Record(v)
	}
	got := lt.Mean()
	if got != 25.0 {
		t.Fatalf("Mean() = %f, want 25.0", got)
	}
}

// TestP99Empty verifies that P99() returns 0 on an empty tracker.
func TestP99Empty(t *testing.T) {
	var lt LatencyTracker
	if got := lt.P99(); got != 0 {
		t.Fatalf("P99() on empty tracker = %d, want 0", got)
	}
}

// TestP99Single records one value and verifies P99() == that value.
func TestP99Single(t *testing.T) {
	var lt LatencyTracker
	lt.Record(42)
	if got := lt.P99(); got != 42 {
		t.Fatalf("P99() after single Record(42) = %d, want 42", got)
	}
}

// TestP99GreaterOrEqualP95 records a known distribution and verifies
// P99() >= P95() (the 99th percentile must be at least the 95th percentile).
func TestP99GreaterOrEqualP95(t *testing.T) {
	var lt LatencyTracker
	// Record a monotonically increasing distribution 1..50.
	for i := int64(1); i <= 50; i++ {
		lt.Record(i)
	}
	p95 := lt.P95()
	p99 := lt.P99()
	if p99 < p95 {
		t.Fatalf("P99() = %d < P95() = %d; P99 must be >= P95", p99, p95)
	}
}

// TestP99NinetyNinthPercentile records 100 values (1..100) and verifies that
// P99() returns the 99th value (99) using nearest-rank (WR-03 correction).
//
// WR-03 correction: the old formula (int(n * 0.99) = 99 → index 99 → value 100)
// was off by one. Nearest-rank: ceil(0.99 * 100) - 1 = 99 - 1 = 98 → value 99.
func TestP99NinetyNinthPercentile(t *testing.T) {
	var lt LatencyTracker
	for i := int64(1); i <= 100; i++ {
		lt.Record(i)
	}
	got := lt.P99()
	// WR-03: nearest-rank gives the 99th element (99), not the maximum (100).
	if got != 99 {
		t.Fatalf("P99() with 1..100 distribution = %d, want 99 (nearest-rank WR-03)", got)
	}
}

// TestP95NinetyFifthPercentile records 100 values (1..100) and verifies that
// P95() returns the 95th value (95) using nearest-rank (WR-03).
//
// WR-03: ceil(0.95 * 100) - 1 = 95 - 1 = 94 → buf[94] = 95 in sorted [1..100].
func TestP95NinetyFifthPercentile(t *testing.T) {
	var lt LatencyTracker
	for i := int64(1); i <= 100; i++ {
		lt.Record(i)
	}
	got := lt.P95()
	if got != 95 {
		t.Fatalf("P95() with 1..100 distribution = %d, want 95 (nearest-rank WR-03)", got)
	}
}

// TestP95SmallNDoesNotCollapseToMax verifies that P95 of a small sample set
// does not return the maximum unless the percentile genuinely lands there.
// With n=20 samples (1..20), ceil(0.95 * 20) - 1 = ceil(19.0) - 1 = 18 → buf[18] = 19.
// The maximum is 20, so the result must not be 20.
func TestP95SmallNDoesNotCollapseToMax(t *testing.T) {
	var lt LatencyTracker
	for i := int64(1); i <= 20; i++ {
		lt.Record(i)
	}
	got := lt.P95()
	if got == 20 {
		t.Errorf("P95() with n=20 = 20 (maximum), want 19 (nearest-rank must not collapse to max)")
	}
	if got != 19 {
		t.Errorf("P95() with n=20 = %d, want 19", got)
	}
}
