package posture

// detect.go — IMPURE (os/exec/context/time/sync/strings — the ONLY posture file
// that may import these packages). This is the impure-adapter half of the
// pure-decision-over-resolved-input pattern: it resolves the local PM state by
// running each PM's --version and scanning its config files, then hands a
// read-only PMState to the posture view.
//
// Fail-OPEN-by-design contract (NOT a fail-closed violation):
//   Detection timeout/error → treat PM as "not installed", proceed. A slow or
//   absent PM must never block; the posture view simply reports it as not
//   installed. Contrast with catalog/path decisions which fail CLOSED — this
//   file is intentionally fail-open because it is a read-only inventory.
//
// Detection seam:
//   var DetectStateFn = DetectState is the EXPORTED package-level var that
//   consumers call so a test in another package can inject a synthetic PMState
//   without a real pnpm/bun on PATH. The unexported version-fns
//   (pnpmVersionFn, bunVersionFn, nodeVersionFn) are the internal default
//   implementation inside DetectState.
//
// Gateway Cache:
//   Constructed ONCE by a long-lived caller (e.g. the gateway); a one-shot
//   process gets no cache hits.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// detectionTimeout is the hard deadline for each PM version exec call.
// Documented as a package-level var so tests can tighten it and so callers see
// the same tunable value.
//
// Pitfall (Windows corepack timing): corepack-shimmed pnpm on Windows may be
// slower on first call due to Node bootstrap. Live measurement on a Windows dev
// box (pnpm 11.1.3 via the %APPDATA%\npm\pnpm.cmd shim) found `pnpm --version`
// averaging ~1.75s with a ~2.7s tail — so a 2s deadline tripped on ~17% of
// invocations, misreporting pnpm as "not installed". The deadline is 3s to
// cover the measured tail with margin. On timeout the PM is treated as "not
// installed" and detection proceeds (never blocks — fail-open). Unix probes
// return in well under 1s, so 3s only widens the Windows hang-cap.
var detectionTimeout = 3 * time.Second

// pnpmVersionFn, bunVersionFn, nodeVersionFn are the injectable package-level
// vars for the PM version exec calls. Tests substitute slow/erroring fakes to
// prove the timeout/error fallback without a real binary. Real implementations
// use exec.CommandContext with the detectionTimeout hard deadline. They execute
// FIXED argv only — "pnpm"/"bun"/"node", "--version". No user-controlled path or
// argument is ever passed.

var pnpmVersionFn = func(ctx context.Context) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, detectionTimeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, "pnpm", "--version").Output()
	if err != nil {
		return "", err // timeout (DeadlineExceeded) or not-found → "not installed"
	}
	return strings.TrimSpace(string(out)), nil
}

var bunVersionFn = func(ctx context.Context) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, detectionTimeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, "bun", "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var nodeVersionFn = func(ctx context.Context) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, detectionTimeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, "node", "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// DetectState resolves the local PM state. It runs pnpm/bun/node --version each
// with a hard timeout and, when cfg.CheckSocketScanner is true, scans
// bunfig.toml / pnpm-workspace.yaml.
//
// The three version probes are run CONCURRENTLY (they are independent), so
// worst-case detection latency is bounded by the single slowest probe
// (~detectionTimeout) rather than the sum.
//
// Fail-open contract: on any timeout or error a PM is treated as "not installed"
// (graceful fallback — never blocks on detection failure, never panics). A panic
// inside a probe goroutine would crash the process, so each probe is run
// defensively (a panicking version-fn is treated as "not installed"). The
// dependent file scans (pnpm-workspace hardening, bunfig scanner) run AFTER the
// probes complete.
func DetectState(ctx context.Context, cfg Config) PMState {
	var (
		wg                       sync.WaitGroup
		pnpmVer, bunVer, nodeVer string
		pnpmErr, bunErr, nodeErr error
	)
	probe := func(fn func(context.Context) (string, error), ver *string, errp *error) {
		defer wg.Done()
		// Defensive: a panic inside a probe must be contained as a detection
		// failure (fail-open), never crash the host process.
		defer func() {
			if r := recover(); r != nil {
				*ver = ""
				*errp = &probePanicError{value: r}
			}
		}()
		*ver, *errp = fn(ctx)
	}
	wg.Add(3)
	go probe(pnpmVersionFn, &pnpmVer, &pnpmErr)
	go probe(bunVersionFn, &bunVer, &bunErr)
	go probe(nodeVersionFn, &nodeVer, &nodeErr)
	wg.Wait()

	var state PMState

	// Detect pnpm.
	if pnpmErr == nil && pnpmVer != "" {
		state.PnpmInstalled = true
		state.PnpmVersion = pnpmVer

		// Compute hardening: version floor AND pnpm-workspace hardening scan.
		hardeningOK := meetsFloor(pnpmVer, cfg.VersionFloors.Pnpm)
		if hardeningOK {
			// Check for explicit hardening weaknesses in pnpm-workspace.yaml.
			wsPath := pnpmWorkspacePath()
			hr := DetectPnpmHardening(wsPath)
			// Hardened stays true unless the workspace explicitly removes hardening.
			// Weakness is logged but does not flip hardened.
			_ = hr.WeaknessLogged // weakness logging is handled by the caller/audit layer
			state.PnpmHardened = hr.Hardened
		}
		// If version does not meet floor, PnpmHardened stays false.
	}
	// On pnpmVersionFn error/timeout: PnpmInstalled=false, proceed (fail-open).

	// Detect bun (bun may be absent on dev machine — tests use injected fns).
	if bunErr == nil && bunVer != "" {
		state.BunInstalled = true
		state.BunVersion = bunVer

		// BunScannerOK: checked only when cfg.CheckSocketScanner is true.
		if cfg.CheckSocketScanner {
			paths := bunfigPaths()
			state.BunScannerOK = DetectBunScanner(paths)
		}
	}
	// On bunVersionFn error/timeout: BunInstalled=false, proceed (fail-open).

	// Detect node.
	if nodeErr == nil && nodeVer != "" {
		state.NodeVersion = nodeVer
	}
	// On nodeVersionFn error/timeout: NodeVersion="", proceed (fail-open).

	return state
}

// probePanicError wraps a value recovered from a panicking version-fn so that a
// panic is surfaced as an ordinary detection error (fail-open) rather than
// crashing the host process. It is never inspected beyond err != nil.
type probePanicError struct{ value any }

func (e *probePanicError) Error() string {
	return fmt.Sprintf("posture: version probe panicked: %v", e.value)
}

// DetectStateFn is the EXPORTED swappable detection seam.
//
// Consumers resolve PMState through DetectStateFn, NOT DetectState directly, so a
// behavioral test in another package can inject a synthetic PMState without a
// real pnpm/bun on PATH. The unexported pnpmVersionFn / bunVersionFn /
// nodeVersionFn remain the internal default implementation reachable only from
// package posture; cross-package tests swap DetectStateFn and defer-restore it.
//
// In production code DetectStateFn is never reassigned — this var is test-only
// infrastructure; the shipped binary always runs the real DetectState.
var DetectStateFn = DetectState

// pnpmWorkspacePath returns the path to pnpm-workspace.yaml in the current
// working directory. Returns empty string on error (detection proceeds safely).
func pnpmWorkspacePath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, "pnpm-workspace.yaml")
}

// bunfigPaths returns the ordered list of bunfig.toml paths to check:
//  1. <cwd>/bunfig.toml (project root)
//  2. <homedir>/.bunfig.toml (user home)
//
// Missing paths are skipped by DetectBunScanner — no error on absence.
func bunfigPaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "bunfig.toml"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".bunfig.toml"))
	}
	return paths
}

// Cache wraps a detect function with a TTL-based memoization. It is constructed
// ONCE by a long-lived caller (e.g. the gateway) at startup and reused across
// requests. NEVER constructed in a one-shot process (which gets no cache hits).
//
// The now field is injectable for tests. In production code NewCache sets now to
// time.Now.
type Cache struct {
	mu      sync.Mutex
	detect  func(context.Context, Config) PMState
	ttl     time.Duration
	now     func() time.Time
	state   PMState
	have    bool
	expires time.Time
}

// NewCache creates a new Cache wrapping the given detect function with the given
// TTL. In production callers pass DetectStateFn and 60*time.Second.
//
// The now function defaults to time.Now and is injectable for tests.
func NewCache(d func(context.Context, Config) PMState, ttl time.Duration) *Cache {
	return &Cache{
		detect: d,
		ttl:    ttl,
		now:    time.Now,
	}
}

// newCacheWithClock creates a Cache with an injectable clock. Used only in tests.
func newCacheWithClock(d func(context.Context, Config) PMState, ttl time.Duration, nowFn func() time.Time) *Cache {
	return &Cache{
		detect: d,
		ttl:    ttl,
		now:    nowFn,
	}
}

// State returns the cached PMState if it is still within the TTL, or calls the
// underlying detect function to refresh it. Concurrent callers are serialized by
// the mutex — only one refresh happens per TTL window.
func (c *Cache) State(ctx context.Context, cfg Config) PMState {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if c.have && now.Before(c.expires) {
		return c.state
	}

	// Cache miss or expired — refresh.
	c.state = c.detect(ctx, cfg)
	c.have = true
	c.expires = now.Add(c.ttl)
	return c.state
}
