// Package llamafirewall provides the supervisor, client, and latency tracking
// for the LlamaFirewall Python sidecar process.
package llamafirewall

import (
	"math"
	"sort"
	"sync"
)

// LatencyTracker tracks call latencies using a fixed-size ring buffer for P95
// computation and a running sum for mean computation. Thread-safe.
type LatencyTracker struct {
	mu     sync.Mutex
	count  int64
	sumMS  int64
	p95buf [100]int64 // ring buffer for last 100 latencies
	head   int        // monotonic write counter; slot index is head%100
	filled bool       // true once all 100 slots written at least once
}

// Record adds a latency sample (in milliseconds) to the tracker.
func (t *LatencyTracker) Record(ms int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count++
	t.sumMS += ms
	t.p95buf[t.head%100] = ms
	t.head++
	if t.head >= 100 {
		t.filled = true
	}
}

// P95 returns the 95th-percentile latency (in milliseconds) over the last 100
// samples, or 0 if no samples have been recorded yet.
//
// WR-03: uses nearest-rank (ceil(p*n)-1, clamped to [0,n-1]) so that
// P95 of 100 ascending samples returns the 95th value (95), not 96.
func (t *LatencyTracker) P95() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count == 0 {
		return 0
	}
	n := 100
	if !t.filled {
		n = t.head
	}
	buf := make([]int64, n)
	copy(buf, t.p95buf[:n])
	sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })
	return percentile(buf, 0.95)
}

// P99 returns the 99th-percentile latency (in milliseconds) over the last 100
// samples, or 0 if no samples have been recorded yet. It uses the same ring
// buffer as P95 (p95buf holds the last 100 samples regardless of percentile).
//
// WR-03: uses nearest-rank (ceil(p*n)-1, clamped to [0,n-1]).
func (t *LatencyTracker) P99() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count == 0 {
		return 0
	}
	n := 100
	if !t.filled {
		n = t.head
	}
	buf := make([]int64, n)
	copy(buf, t.p95buf[:n])
	sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })
	return percentile(buf, 0.99)
}

// percentile returns the value at the given fraction p (0.0–1.0) in the
// already-sorted slice buf using the nearest-rank formula (WR-03):
//
//	idx = ceil(p * n) - 1, clamped to [0, n-1]
//
// This ensures P95 of 100 ascending samples returns the 95th value (95),
// not 96 (the old off-by-one), and small-n does not collapse to the maximum
// unless the percentile genuinely falls there.
func percentile(buf []int64, p float64) int64 {
	n := len(buf)
	if n == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return buf[idx]
}

// Mean returns the arithmetic mean latency (in milliseconds) over all recorded
// samples, or 0.0 if no samples have been recorded.
func (t *LatencyTracker) Mean() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count == 0 {
		return 0
	}
	return float64(t.sumMS) / float64(t.count)
}

// GlobalLatencyTracker accumulates per-invocation LlamaFirewall sidecar latency
// samples for beekeeper diag. Initialized once at package level; Record() is
// called from the supervisor's Scan method after each sidecar call completes.
var GlobalLatencyTracker = &LatencyTracker{}
