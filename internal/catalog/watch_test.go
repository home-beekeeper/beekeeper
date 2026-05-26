package catalog

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// testPollInterval is used in watch tests that exercise the Watch loop.
// It is small enough that tests complete quickly, but set via minPollInterval
// so it bypasses the production 5m floor enforced by clampInterval.
const testPollInterval = 30 * time.Millisecond

// makeSnapshot returns a SnapshotFunc that always returns the given count and hash.
func makeSnapshot(count int, hash string) SnapshotFunc {
	return func(ctx context.Context) (int, string, error) {
		return count, hash, nil
	}
}

// testWatchConfig returns a WatchConfig pre-filled for tests: short poll
// interval, temp CatalogDir and StateFile, and production-default Sanity.
func testWatchConfig(dir, stateFile string, snapshot SnapshotFunc) WatchConfig {
	return WatchConfig{
		PollInterval:    testPollInterval,
		minPollInterval: testPollInterval, // bypass 5m production floor
		CatalogDir:      dir,
		StateFile:       stateFile,
		Snapshot:        snapshot,
		Sanity:          DefaultSanityConfig(),
	}
}

// TestComputeDeltaDetectsChange verifies that computeDelta returns HasChanges()==true
// when the injected Snapshot returns a new hash vs the prior persisted state.
func TestComputeDeltaDetectsChange(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	priorState := WatchState{
		Sources: map[string]SourceState{
			bumblebeeSource: {Hash: "old-hash", Count: 100},
		},
	}

	cfg := testWatchConfig(dir, stateFile, makeSnapshot(105, "new-hash"))

	delta, newState, sanity, err := computeDelta(context.Background(), cfg, priorState)
	if err != nil {
		t.Fatalf("computeDelta: %v", err)
	}

	if !delta.HasChanges() {
		t.Errorf("expected HasChanges()=true, got false (PrevHash=%q NewHash=%q)", delta.PrevHash, delta.NewHash)
	}
	if delta.PrevHash != "old-hash" {
		t.Errorf("PrevHash: want %q, got %q", "old-hash", delta.PrevHash)
	}
	if delta.NewHash != "new-hash" {
		t.Errorf("NewHash: want %q, got %q", "new-hash", delta.NewHash)
	}
	if delta.PrevCount != 100 {
		t.Errorf("PrevCount: want 100, got %d", delta.PrevCount)
	}
	if delta.NewCount != 105 {
		t.Errorf("NewCount: want 105, got %d", delta.NewCount)
	}
	if delta.DeltaCount != 5 {
		t.Errorf("DeltaCount: want 5, got %d", delta.DeltaCount)
	}
	if sanity.Alert || sanity.Block {
		t.Errorf("expected no sanity issue for delta=5, got Alert=%v Block=%v Reason=%q", sanity.Alert, sanity.Block, sanity.Reason)
	}

	ss := newState.Sources[bumblebeeSource]
	if ss.Hash != "new-hash" {
		t.Errorf("persisted Hash: want %q, got %q", "new-hash", ss.Hash)
	}
	if ss.Count != 105 {
		t.Errorf("persisted Count: want 105, got %d", ss.Count)
	}
}

// TestComputeDeltaNoChange verifies that computeDelta returns HasChanges()==false
// when the snapshot matches the prior persisted state.
func TestComputeDeltaNoChange(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	priorState := WatchState{
		Sources: map[string]SourceState{
			bumblebeeSource: {Hash: "same-hash", Count: 654},
		},
	}

	cfg := testWatchConfig(dir, stateFile, makeSnapshot(654, "same-hash"))

	delta, _, sanity, err := computeDelta(context.Background(), cfg, priorState)
	if err != nil {
		t.Fatalf("computeDelta: %v", err)
	}
	if delta.HasChanges() {
		t.Errorf("expected HasChanges()=false for same hash, got true")
	}
	if sanity.Alert || sanity.Block {
		t.Errorf("expected no sanity issue for no-change, got Alert=%v Block=%v", sanity.Alert, sanity.Block)
	}
}

// TestComputeDeltaSanityBlock verifies that a +12000 entry jump degrades the source
// (Block=true, Degraded=true in persisted state).
func TestComputeDeltaSanityBlock(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	priorState := WatchState{
		Sources: map[string]SourceState{
			bumblebeeSource: {Hash: "old-hash", Count: 1000},
		},
	}

	cfg := testWatchConfig(dir, stateFile, makeSnapshot(13000, "poison-hash")) // delta = 12000

	delta, newState, sanity, err := computeDelta(context.Background(), cfg, priorState)
	if err != nil {
		t.Fatalf("computeDelta: %v", err)
	}

	if !sanity.Block {
		t.Errorf("expected SanityResult.Block=true for delta 12000, got Block=false (Alert=%v Reason=%q)", sanity.Alert, sanity.Reason)
	}
	if sanity.Alert && !sanity.Block {
		t.Errorf("Block should take priority over Alert; got Alert=true Block=false")
	}

	ss := newState.Sources[bumblebeeSource]
	if !ss.Degraded {
		t.Errorf("expected source marked Degraded=true after Block, got Degraded=false")
	}
	if ss.DegradedReason == "" {
		t.Errorf("expected non-empty DegradedReason, got empty string")
	}

	// Verify it is persisted to disk.
	loaded, err := LoadState(stateFile)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	persistedSS := loaded.Sources[bumblebeeSource]
	if !persistedSS.Degraded {
		t.Errorf("degraded state must be persisted to disk: Degraded=false after LoadState")
	}

	if !delta.HasChanges() {
		t.Errorf("expected HasChanges()=true even when sanity blocked, got false")
	}
}

// TestComputeDeltaSanityAlert verifies that a delta of 1500 triggers Alert but not Block.
func TestComputeDeltaSanityAlert(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	priorState := WatchState{
		Sources: map[string]SourceState{
			bumblebeeSource: {Hash: "old-hash", Count: 1000},
		},
	}

	cfg := testWatchConfig(dir, stateFile, makeSnapshot(2500, "new-hash")) // delta = 1500

	_, newState, sanity, err := computeDelta(context.Background(), cfg, priorState)
	if err != nil {
		t.Fatalf("computeDelta: %v", err)
	}
	if !sanity.Alert {
		t.Errorf("expected Alert=true for delta 1500, got Alert=false")
	}
	if sanity.Block {
		t.Errorf("expected Block=false for delta 1500, got Block=true")
	}
	if !newState.Sources[bumblebeeSource].Degraded {
		t.Errorf("alert should degrade the source to warning-only: Degraded=false")
	}
}

// TestComputeDeltaDegradationPreserved verifies that a previously-degraded source
// remains degraded even when subsequent polls produce no sanity breach.
func TestComputeDeltaDegradationPreserved(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	priorState := WatchState{
		Sources: map[string]SourceState{
			bumblebeeSource: {
				Hash:           "poison-hash",
				Count:          13000,
				Degraded:       true,
				DegradedReason: "delta 12000 exceeds hard limit 10000",
			},
		},
	}

	cfg := testWatchConfig(dir, stateFile, makeSnapshot(13001, "new-hash"))

	_, newState, sanity, err := computeDelta(context.Background(), cfg, priorState)
	if err != nil {
		t.Fatalf("computeDelta: %v", err)
	}
	if sanity.Alert || sanity.Block {
		t.Errorf("expected clean sanity for delta=1, got Alert=%v Block=%v", sanity.Alert, sanity.Block)
	}
	if !newState.Sources[bumblebeeSource].Degraded {
		t.Error("prior degradation must persist across clean ticks")
	}
}

// TestWatchExitsOnCancel verifies that Watch returns nil promptly when its
// context is cancelled (no goroutine leak, no panic).
func TestWatchExitsOnCancel(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	ctx, cancel := context.WithCancel(context.Background())

	cfg := testWatchConfig(dir, stateFile, makeSnapshot(100, "hash-a"))

	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, cfg, nil)
	}()

	// Cancel almost immediately.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch returned non-nil error after cancel: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Watch did not exit within 500ms of context cancellation (goroutine leak?)")
	}
}

// TestWatchFiresOnDelta verifies that the onDelta callback is invoked when the
// injected snapshot changes. Uses a buffered channel for synchronisation rather
// than ctx.Done() to avoid scheduler-induced timing issues on Windows.
func TestWatchFiresOnDelta(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	var callCount atomic.Int32

	// snapshot returns hash-a on the first call, hash-b on subsequent calls.
	// State is pre-seeded with hash-a, so the first tick is a no-change tick
	// and the second tick fires onDelta.
	var ticksSeen atomic.Int32
	snapshot := func(ctx context.Context) (int, string, error) {
		n := ticksSeen.Add(1)
		if n <= 1 {
			return 100, "hash-a", nil
		}
		return 105, "hash-b", nil
	}

	if err := SaveState(stateFile, WatchState{
		Sources: map[string]SourceState{
			bumblebeeSource: {Hash: "hash-a", Count: 100},
		},
	}); err != nil {
		t.Fatalf("SaveState seed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := testWatchConfig(dir, stateFile, snapshot)

	// Use a buffered channel to receive the callback notification. This avoids
	// blocking on ctx.Done() in the main goroutine, which can starve the Watch
	// goroutine on Windows (the scheduler treats ctx.Done() channel receives
	// differently from other channel receives in some scenarios).
	called := make(chan struct{}, 1)

	go Watch(ctx, cfg, func(d CatalogDelta, s SanityResult) {
		callCount.Add(1)
		if !d.HasChanges() && !s.Alert && !s.Block {
			t.Errorf("onDelta called without meaningful event: HasChanges=%v Alert=%v Block=%v",
				d.HasChanges(), s.Alert, s.Block)
		}
		select {
		case called <- struct{}{}:
		default:
		}
		cancel()
	})

	select {
	case <-called:
		// Callback fired as expected.
	case <-time.After(2 * time.Second):
		t.Errorf("onDelta was never called within 2 seconds; ticksSeen=%d", ticksSeen.Load())
	}

	if callCount.Load() == 0 {
		t.Error("expected onDelta to be called at least once, got 0 calls")
	}
}

// TestWatchClampInterval verifies the poll interval clamping behavior using the
// production minimum (watchPollMin = 5m).
func TestWatchClampInterval(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want time.Duration
	}{
		{0, watchPollDefault},
		{time.Second, watchPollMin},
		{time.Minute, watchPollMin},
		{5 * time.Minute, watchPollMin},
		{time.Hour, time.Hour},
		{12 * time.Hour, 12 * time.Hour},
		{24 * time.Hour, watchPollMax},
		{48 * time.Hour, watchPollMax},
	}

	for _, tc := range tests {
		// Zero minPoll → uses production default (watchPollMin = 5m).
		got := clampInterval(tc.in, 0)
		if got != tc.want {
			t.Errorf("clampInterval(%v, 0): want %v, got %v", tc.in, tc.want, got)
		}
	}
}

// TestWatchFirstRunEmptyState verifies that Watch handles the first-run case
// (no prior state.json) without panicking and fires onDelta on the first tick
// (empty → non-empty is a change).
func TestWatchFirstRunEmptyState(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	// Do NOT create state.json — simulate first-run.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshot := func(ctx context.Context) (int, string, error) {
		return 654, "catalog-hash-v1", nil
	}

	cfg := testWatchConfig(dir, stateFile, snapshot)

	called := make(chan struct{}, 1)
	done := make(chan error, 1)

	go func() {
		done <- Watch(ctx, cfg, func(d CatalogDelta, s SanityResult) {
			select {
			case called <- struct{}{}:
			default:
			}
			cancel()
		})
	}()

	select {
	case <-called:
		// Good — first-run delta fired.
	case <-time.After(2 * time.Second):
		// If state was pre-populated as empty with hash="", the first real
		// snapshot (hash-v1) should still be a change. If nothing fired, fail.
		cancel()
		t.Error("Watch did not fire onDelta on first run within 2 seconds")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch returned error on first run: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Watch did not exit after cancel")
	}
}
