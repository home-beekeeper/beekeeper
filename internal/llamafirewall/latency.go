// Package llamafirewall provides the supervisor, client, and latency tracking
// for the LlamaFirewall Python sidecar process.
package llamafirewall

import (
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
	head   int        // next write position (0..99)
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
	idx := int(float64(n) * 0.95)
	if idx >= n {
		idx = n - 1
	}
	return buf[idx]
}

// P99 returns the 99th-percentile latency (in milliseconds) over the last 100
// samples, or 0 if no samples have been recorded yet. It uses the same ring
// buffer as P95 (p95buf holds the last 100 samples regardless of percentile).
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
	idx := int(float64(n) * 0.99)
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
