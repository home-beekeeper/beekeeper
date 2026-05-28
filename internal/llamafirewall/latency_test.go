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
