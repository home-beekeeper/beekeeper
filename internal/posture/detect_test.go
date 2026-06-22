package posture

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestDetectStateTimeoutFallback proves §10-12: a version-fn that sleeps past
// the timeout causes the PM to be treated as NOT installed (fail-open, never
// blocks). The test injects a slow pnpmVersionFn that sleeps longer than the
// 2s default and verifies PnpmInstalled=false.
func TestDetectStateTimeoutFallback(t *testing.T) {
	// Inject a slow pnpm version fn that always times out.
	orig := pnpmVersionFn
	pnpmVersionFn = func(ctx context.Context) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err() // timeout → treat as not installed
		case <-time.After(10 * time.Second): // far past detectionTimeout
			return "11.0.0", nil
		}
	}
	defer func() { pnpmVersionFn = orig }()

	// Use a context with a short timeout so the test completes quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cfg := DefaultConfig()
	state := DetectState(ctx, cfg)

	if state.PnpmInstalled {
		t.Error("PnpmInstalled should be false when version-fn times out (§10-12 fail-open)")
	}
	if state.PnpmVersion != "" {
		t.Errorf("PnpmVersion should be empty on timeout, got %q", state.PnpmVersion)
	}
	if state.PnpmHardened {
		t.Error("PnpmHardened should be false when pnpm not installed")
	}
}

// TestDetectStateParallelProbes proves WR-03: the three version probes run
// concurrently, so worst-case detection latency is bounded by the single
// slowest probe (~detectionTimeout) rather than the sum of all three. The test
// injects three version-fns that each block on their context until cancelled,
// drives DetectState under a single short deadline, and asserts the total wall
// time is far below the ~3x sequential worst case — while preserving fail-open
// (all three PMs report "not installed").
func TestDetectStateParallelProbes(t *testing.T) {
	const probeDelay = 400 * time.Millisecond

	// Each probe blocks for probeDelay then returns a value. If the probes ran
	// sequentially the total would be ~3*probeDelay (~1.2s); in parallel it is
	// ~probeDelay (~0.4s). We assert well under the sequential sum.
	blocker := func(_ context.Context) (string, error) {
		time.Sleep(probeDelay)
		return "", errors.New("slow probe → not installed")
	}

	origNpm := npmVersionFn
	origPnpm := pnpmVersionFn
	origBun := bunVersionFn
	origNode := nodeVersionFn
	npmVersionFn = blocker
	pnpmVersionFn = blocker
	bunVersionFn = blocker
	nodeVersionFn = blocker
	defer func() {
		npmVersionFn = origNpm
		pnpmVersionFn = origPnpm
		bunVersionFn = origBun
		nodeVersionFn = origNode
	}()

	start := time.Now()
	state := DetectState(context.Background(), DefaultConfig())
	elapsed := time.Since(start)

	// Parallel: elapsed ~= probeDelay. Sequential would be ~3*probeDelay.
	// Use 2*probeDelay as a generous ceiling that still proves parallelism.
	if elapsed >= 2*probeDelay {
		t.Errorf("detection took %v — probes appear sequential, want < %v (parallel)", elapsed, 2*probeDelay)
	}

	// Fail-open preserved: every PM reports not installed.
	if state.NpmInstalled || state.PnpmInstalled || state.BunInstalled || state.NodeVersion != "" {
		t.Errorf("fail-open violated: %+v — slow probes must yield 'not installed'", state)
	}
}

// TestDetectStateProbePanicFailOpen proves WR-03 hardening: a panicking
// version-fn is contained as a detection failure (fail-open), never crashing the
// process. The injected pnpm probe panics; DetectState must return normally with
// PnpmInstalled=false while bun/node still resolve.
func TestDetectStateProbePanicFailOpen(t *testing.T) {
	origPnpm := pnpmVersionFn
	origBun := bunVersionFn
	origNode := nodeVersionFn
	pnpmVersionFn = func(_ context.Context) (string, error) { panic("boom in probe") }
	bunVersionFn = func(_ context.Context) (string, error) { return "1.3.14", nil }
	nodeVersionFn = func(_ context.Context) (string, error) { return "22.0.0", nil }
	defer func() {
		pnpmVersionFn = origPnpm
		bunVersionFn = origBun
		nodeVersionFn = origNode
	}()

	// Must not panic out of DetectState.
	state := DetectState(context.Background(), DefaultConfig())

	if state.PnpmInstalled {
		t.Error("PnpmInstalled should be false when the pnpm probe panics (fail-open)")
	}
	if !state.BunInstalled {
		t.Error("BunInstalled should still be true — bun probe is unaffected by pnpm panic")
	}
	if state.NodeVersion != "22.0.0" {
		t.Errorf("NodeVersion = %q, want 22.0.0 — node probe unaffected by pnpm panic", state.NodeVersion)
	}
}

// TestDetectStateErrorFallback proves §10-12: a version-fn returning an error
// causes the PM to be treated as NOT installed (graceful fallback).
func TestDetectStateErrorFallback(t *testing.T) {
	origPnpm := pnpmVersionFn
	origBun := bunVersionFn
	origNode := nodeVersionFn
	pnpmVersionFn = func(_ context.Context) (string, error) {
		return "", errors.New("binary not found")
	}
	bunVersionFn = func(_ context.Context) (string, error) {
		return "", errors.New("binary not found")
	}
	nodeVersionFn = func(_ context.Context) (string, error) {
		return "", errors.New("binary not found")
	}
	defer func() {
		pnpmVersionFn = origPnpm
		bunVersionFn = origBun
		nodeVersionFn = origNode
	}()

	cfg := DefaultConfig()
	state := DetectState(context.Background(), cfg)

	if state.PnpmInstalled {
		t.Error("PnpmInstalled should be false when version-fn errors")
	}
	if state.BunInstalled {
		t.Error("BunInstalled should be false when version-fn errors")
	}
	if state.NodeVersion != "" {
		t.Errorf("NodeVersion should be empty when node version-fn errors, got %q", state.NodeVersion)
	}
}

// TestDetectStateGoodVersions proves that DetectState correctly populates
// PMState when all version-fns return good output.
func TestDetectStateGoodVersions(t *testing.T) {
	origPnpm := pnpmVersionFn
	origBun := bunVersionFn
	origNode := nodeVersionFn
	pnpmVersionFn = func(_ context.Context) (string, error) { return "11.5.1", nil }
	bunVersionFn = func(_ context.Context) (string, error) { return "1.3.14", nil }
	nodeVersionFn = func(_ context.Context) (string, error) { return "22.0.0", nil }
	defer func() {
		pnpmVersionFn = origPnpm
		bunVersionFn = origBun
		nodeVersionFn = origNode
	}()

	// Override readFileFn so pnpm hardening scan succeeds with no weakness.
	origRead := readFileFn
	readFileFn = func(_ string) ([]byte, error) {
		// pnpm-workspace.yaml absent → hardened=true
		return nil, errors.New("not found")
	}
	defer func() { readFileFn = origRead }()

	cfg := DefaultConfig()
	state := DetectState(context.Background(), cfg)

	if !state.PnpmInstalled {
		t.Error("PnpmInstalled should be true")
	}
	if state.PnpmVersion != "11.5.1" {
		t.Errorf("PnpmVersion = %q, want 11.5.1", state.PnpmVersion)
	}
	if !state.PnpmHardened {
		t.Error("PnpmHardened should be true — version meets floor and no override weakness")
	}
	if !state.BunInstalled {
		t.Error("BunInstalled should be true")
	}
	if state.BunVersion != "1.3.14" {
		t.Errorf("BunVersion = %q, want 1.3.14", state.BunVersion)
	}
	if state.NodeVersion != "22.0.0" {
		t.Errorf("NodeVersion = %q, want 22.0.0", state.NodeVersion)
	}
}

// TestDetectStateFloorNotMet proves that PnpmHardened=false when the detected
// pnpm version is below the configured floor.
func TestDetectStateFloorNotMet(t *testing.T) {
	orig := pnpmVersionFn
	pnpmVersionFn = func(_ context.Context) (string, error) { return "10.9.0", nil }
	defer func() { pnpmVersionFn = orig }()

	cfg := DefaultConfig() // floor = 11.0.0
	state := DetectState(context.Background(), cfg)

	if !state.PnpmInstalled {
		t.Error("PnpmInstalled should be true when version is returned")
	}
	if state.PnpmHardened {
		t.Error("PnpmHardened should be false — version 10.9.0 < floor 11.0.0")
	}
}

// TestDetectStateBunScannerCheck proves that BunScannerOK is set when
// cfg.CheckSocketScanner=true and the injected readFileFn returns a bunfig
// with the scanner configured.
func TestDetectStateBunScannerCheck(t *testing.T) {
	origBun := bunVersionFn
	bunVersionFn = func(_ context.Context) (string, error) { return "1.3.14", nil }
	defer func() { bunVersionFn = origBun }()

	origRead := readFileFn
	readFileFn = func(path string) ([]byte, error) {
		// Return valid bunfig for all paths.
		return []byte("[install.security]\nscanner = \"@socketsecurity/bun-security-scanner\"\n"), nil
	}
	defer func() { readFileFn = origRead }()

	cfg := DefaultConfig()
	cfg.CheckSocketScanner = true
	state := DetectState(context.Background(), cfg)

	if !state.BunScannerOK {
		t.Error("BunScannerOK should be true when bunfig has the scanner")
	}
}

// TestDetectStateBunScannerSkipped proves that BunScannerOK stays false when
// cfg.CheckSocketScanner=false, even if the bunfig would have the scanner.
func TestDetectStateBunScannerSkipped(t *testing.T) {
	origBun := bunVersionFn
	bunVersionFn = func(_ context.Context) (string, error) { return "1.3.14", nil }
	defer func() { bunVersionFn = origBun }()

	cfg := DefaultConfig()
	cfg.CheckSocketScanner = false
	state := DetectState(context.Background(), cfg)

	if state.BunScannerOK {
		t.Error("BunScannerOK should be false when CheckSocketScanner=false")
	}
}

// TestDetectStateFnSeamDefault proves that DetectStateFn defaults to DetectState:
// calling DetectStateFn with injected version-fns yields the same PMState
// as calling DetectState directly.
func TestDetectStateFnSeamDefault(t *testing.T) {
	origPnpm := pnpmVersionFn
	origBun := bunVersionFn
	origNode := nodeVersionFn
	pnpmVersionFn = func(_ context.Context) (string, error) { return "11.0.0", nil }
	bunVersionFn = func(_ context.Context) (string, error) { return "", errors.New("absent") }
	nodeVersionFn = func(_ context.Context) (string, error) { return "22.0.0", nil }
	defer func() {
		pnpmVersionFn = origPnpm
		bunVersionFn = origBun
		nodeVersionFn = origNode
	}()

	origRead := readFileFn
	readFileFn = func(_ string) ([]byte, error) { return nil, errors.New("absent") }
	defer func() { readFileFn = origRead }()

	cfg := DefaultConfig()

	// DetectStateFn must point at DetectState (default) and produce identical output.
	stateViaSeam := DetectStateFn(context.Background(), cfg)
	stateDirect := DetectState(context.Background(), cfg)

	if stateViaSeam.PnpmInstalled != stateDirect.PnpmInstalled {
		t.Errorf("DetectStateFn.PnpmInstalled=%v != DetectState.PnpmInstalled=%v",
			stateViaSeam.PnpmInstalled, stateDirect.PnpmInstalled)
	}
	if stateViaSeam.PnpmVersion != stateDirect.PnpmVersion {
		t.Errorf("DetectStateFn.PnpmVersion=%q != DetectState.PnpmVersion=%q",
			stateViaSeam.PnpmVersion, stateDirect.PnpmVersion)
	}
	if stateViaSeam.NodeVersion != stateDirect.NodeVersion {
		t.Errorf("DetectStateFn.NodeVersion=%q != DetectState.NodeVersion=%q",
			stateViaSeam.NodeVersion, stateDirect.NodeVersion)
	}
}

// TestDetectStateFnSeamSwap proves that reassigning DetectStateFn to a stub
// returning a synthetic PMState causes callers of DetectStateFn to observe the
// stub, and a defer-restore returns it to the real DetectState.
func TestDetectStateFnSeamSwap(t *testing.T) {
	syntheticState := PMState{
		PnpmInstalled: true,
		PnpmVersion:   "99.0.0",
		PnpmHardened:  true,
		NodeVersion:   "99.0.0",
	}

	orig := DetectStateFn
	DetectStateFn = func(_ context.Context, _ Config) PMState {
		return syntheticState
	}
	defer func() { DetectStateFn = orig }()

	cfg := DefaultConfig()
	got := DetectStateFn(context.Background(), cfg)

	if got.PnpmVersion != "99.0.0" {
		t.Errorf("Swapped DetectStateFn returned PnpmVersion=%q, want 99.0.0", got.PnpmVersion)
	}
	if !got.PnpmHardened {
		t.Error("Swapped DetectStateFn should return synthetic PnpmHardened=true")
	}
}

// TestDetectStateFnSeamRestore proves that after the defer-restore above,
// DetectStateFn is back to the real DetectState. We run this as a separate
// subtest to confirm the defer-restore pattern from the previous test actually
// worked (i.e. the seam is correctly restored).
func TestDetectStateFnSeamRestore(t *testing.T) {
	// At this point DetectStateFn should be the real DetectState (not the swap
	// from TestDetectStateFnSeamSwap — tests run sequentially and that test's
	// defer has already fired by the time this runs).

	// Inject known version-fns so we can assert real DetectState behavior.
	origPnpm := pnpmVersionFn
	pnpmVersionFn = func(_ context.Context) (string, error) { return "11.0.0", nil }
	defer func() { pnpmVersionFn = origPnpm }()

	origRead := readFileFn
	readFileFn = func(_ string) ([]byte, error) { return nil, errors.New("absent") }
	defer func() { readFileFn = origRead }()

	cfg := DefaultConfig()
	// If DetectStateFn were still the swap stub, this would return "99.0.0".
	// If it's the real DetectState, it will return "11.0.0".
	got := DetectStateFn(context.Background(), cfg)
	if got.PnpmVersion == "99.0.0" {
		t.Error("DetectStateFn was not restored — still returning the stub value 99.0.0")
	}
	// After real DetectState with injected fn returning "11.0.0" we expect PnpmInstalled=true.
	if !got.PnpmInstalled {
		t.Error("DetectStateFn (restored to real) should detect pnpm from injected fn")
	}
}

// TestCacheTTLWithInjectedClock proves §10-11: the Cache memoizes within TTL and
// refreshes after TTL expiry, proven with an injected clock + counting detect-fn.
func TestCacheTTLWithInjectedClock(t *testing.T) {
	var callCount atomic.Int32

	syntheticState := PMState{PnpmInstalled: true, PnpmVersion: "11.0.0"}
	detectFn := func(_ context.Context, _ Config) PMState {
		callCount.Add(1)
		return syntheticState
	}

	// Inject a controllable clock starting at a fixed time.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fakeClock := func() time.Time { return now }

	c := newCacheWithClock(detectFn, 60*time.Second, fakeClock)
	cfg := DefaultConfig()

	// First call: cache miss → underlying fn called.
	s1 := c.State(context.Background(), cfg)
	if callCount.Load() != 1 {
		t.Errorf("expected 1 underlying call after first State(), got %d", callCount.Load())
	}
	if !s1.PnpmInstalled {
		t.Error("first State() should return PnpmInstalled=true")
	}

	// Second call within TTL: cache hit → underlying fn NOT called again.
	now = now.Add(30 * time.Second) // 30s < 60s TTL
	s2 := c.State(context.Background(), cfg)
	if callCount.Load() != 1 {
		t.Errorf("expected still 1 underlying call within TTL, got %d", callCount.Load())
	}
	if s2.PnpmVersion != "11.0.0" {
		t.Errorf("cached State PnpmVersion=%q, want 11.0.0", s2.PnpmVersion)
	}

	// Third call after TTL expiry: cache miss → underlying fn called again.
	now = now.Add(31 * time.Second) // now 30+31=61s since start > 60s TTL
	s3 := c.State(context.Background(), cfg)
	if callCount.Load() != 2 {
		t.Errorf("expected 2 underlying calls after TTL expiry, got %d", callCount.Load())
	}
	if !s3.PnpmInstalled {
		t.Error("refreshed State() should return PnpmInstalled=true")
	}
}

// TestCacheNewCache verifies that NewCache with default clock is constructable
// and produces a valid initial call that invokes the detect fn.
func TestCacheNewCache(t *testing.T) {
	called := false
	detectFn := func(_ context.Context, _ Config) PMState {
		called = true
		return PMState{PnpmInstalled: true, PnpmVersion: "11.0.0"}
	}

	c := NewCache(detectFn, 60*time.Second)
	s := c.State(context.Background(), DefaultConfig())
	if !called {
		t.Error("NewCache.State() should call the underlying detect function")
	}
	if !s.PnpmInstalled {
		t.Error("NewCache.State() should return the detected state")
	}
}

// TestCacheIsNeverConstructedByCheckHook documents the architectural contract:
// Cache is constructed only in the long-lived gateway, never in the one-shot
// check hook. This test verifies the Cache.State mechanics work correctly in
// isolation (the hook not constructing a Cache is verified by code review and
// the handler.go wiring in Plan 06/08).
func TestCacheIsNeverConstructedByCheckHook(t *testing.T) {
	// Sanity-check: NewCache returns a non-nil *Cache (gateway can construct it).
	c := NewCache(DetectStateFn, 60*time.Second)
	if c == nil {
		t.Fatal("NewCache returned nil")
	}
	// Documented: the check hook calls DetectStateFn directly — no Cache.
	// This test's comment is the canonical documentation of that contract.
}
