package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/llamafirewall"
)

// TestAppendAndLoadHookLatency verifies the basic round-trip: appendHookLatency
// writes samples to a ring file that loadHookLatency reads back in order.
func TestAppendAndLoadHookLatency(t *testing.T) {
	dir := t.TempDir()
	ringPath := filepath.Join(dir, hookLatencyFile)

	// Initially empty — missing file must return nil, not an error.
	samples := loadHookLatency(ringPath)
	if samples != nil {
		t.Fatalf("loadHookLatency on missing file: got %v, want nil", samples)
	}

	// Append three samples.
	appendHookLatency(ringPath, 10)
	appendHookLatency(ringPath, 20)
	appendHookLatency(ringPath, 30)

	samples = loadHookLatency(ringPath)
	if len(samples) != 3 {
		t.Fatalf("loadHookLatency: got %d samples, want 3", len(samples))
	}
	if samples[0] != 10 || samples[1] != 20 || samples[2] != 30 {
		t.Fatalf("loadHookLatency: got %v, want [10 20 30]", samples)
	}
}

// TestHookLatencyRingRotation verifies that the ring drops the oldest samples
// once hookLatencyRingSize samples have been accumulated.
func TestHookLatencyRingRotation(t *testing.T) {
	dir := t.TempDir()
	ringPath := filepath.Join(dir, hookLatencyFile)

	// Write hookLatencyRingSize + 10 samples. The ring must retain only the
	// last hookLatencyRingSize samples.
	total := hookLatencyRingSize + 10
	for i := 0; i < total; i++ {
		appendHookLatency(ringPath, int64(i+1))
	}

	samples := loadHookLatency(ringPath)
	if len(samples) != hookLatencyRingSize {
		t.Fatalf("ring rotation: got %d samples, want %d", len(samples), hookLatencyRingSize)
	}
	// After rotation the first sample should be value 11 (the 11th recorded).
	wantFirst := int64(total - hookLatencyRingSize + 1)
	if samples[0] != wantFirst {
		t.Fatalf("ring rotation: first sample = %d, want %d", samples[0], wantFirst)
	}
}

// TestLoadHookLatency_CorruptFile verifies that a corrupt ring file is treated
// as an empty sample set (T-09-15: must not crash diag or runCheck).
func TestLoadHookLatency_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	ringPath := filepath.Join(dir, hookLatencyFile)

	if err := os.WriteFile(ringPath, []byte("this is not JSON{{{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	samples := loadHookLatency(ringPath)
	if samples != nil {
		t.Fatalf("loadHookLatency(corrupt): got %v, want nil", samples)
	}
}

// TestGlobalHookTracker verifies that multiple simulated check latencies
// accumulated in the persisted ring produce non-zero p95/p99 values that are
// ordered p99 >= p95.
func TestGlobalHookTracker(t *testing.T) {
	dir := t.TempDir()
	ringPath := filepath.Join(dir, hookLatencyFile)

	// Simulate 20 check invocations writing latency samples to the ring.
	simulated := []int64{10, 15, 20, 25, 30, 35, 40, 45, 50, 55,
		60, 65, 70, 75, 80, 85, 90, 95, 99, 100}
	for _, ms := range simulated {
		appendHookLatency(ringPath, ms)
	}

	// Load the samples and feed them into a fresh LatencyTracker.
	samples := loadHookLatency(ringPath)
	if len(samples) == 0 {
		t.Fatal("TestGlobalHookTracker: no samples loaded from ring file")
	}

	var lt llamafirewall.LatencyTracker
	for _, ms := range samples {
		lt.Record(ms)
	}

	p95 := lt.P95()
	p99 := lt.P99()

	if p95 == 0 {
		t.Fatal("TestGlobalHookTracker: p95 == 0, want non-zero")
	}
	if p99 == 0 {
		t.Fatal("TestGlobalHookTracker: p99 == 0, want non-zero")
	}
	if p99 < p95 {
		t.Fatalf("TestGlobalHookTracker: p99 (%d) < p95 (%d), want p99 >= p95", p99, p95)
	}
}

// TestAppendHookLatency_InvalidPath verifies that appendHookLatency with an
// unwritable path is silent: no panic, no returned error (best-effort contract,
// T-09-14). The test simulates the fail-closed contract by checking GlobalHookTracker
// still works normally.
func TestAppendHookLatency_InvalidPath(t *testing.T) {
	// Attempt to write to a path where the parent cannot be created.
	// On Windows this is a path with an invalid character in it.
	invalidPath := filepath.Join(t.TempDir(), "sub\x00dir", hookLatencyFile)

	// Must not panic.
	appendHookLatency(invalidPath, 42)

	// GlobalHookTracker itself is unaffected.
	var lt llamafirewall.LatencyTracker
	lt.Record(42)
	if got := lt.P95(); got != 42 {
		t.Fatalf("LatencyTracker.P95() after ring-write failure = %d, want 42", got)
	}
}
