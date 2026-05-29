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
// P99() returns a value that is at or near the 99th index of the sorted buffer.
// With n=100 samples, idx = int(100 * 0.99) = 99, so the sorted buffer at
// index 99 is 100 (the maximum). P99() must equal 100.
func TestP99NinetyNinthPercentile(t *testing.T) {
	var lt LatencyTracker
	for i := int64(1); i <= 100; i++ {
		lt.Record(i)
	}
	got := lt.P99()
	if got != 100 {
		t.Fatalf("P99() with 1..100 distribution = %d, want 100", got)
	}
}
