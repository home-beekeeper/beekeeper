package nudge

// detect.go — IMPURE (os/exec/context/time/sync/strings — the ONLY nudge file
// that may import these packages). This is the impure-adapter half of the
// locked pure-decision-over-resolved-input pattern.
//
// Fail-OPEN-by-design contract (NOT a fail-closed violation):
//   Detection timeout/error → treat PM as "not installed", proceed. This is
//   the documented soft-nudge exception to the catalog/path fail-closed rule.
//   A slow/absent PM must never block the agent (PRD §10 criterion 12,
//   T-08-11). Contrast with catalog/path decisions which fail CLOSED — this
//   file is the ONE place the feature is intentionally fail-open.
//
// Cross-package injection seam:
//   var DetectStateFn = DetectState is the EXPORTED package-level var that the
//   check adapter (Plan 06) calls and the Plan 07 behavioral test swaps with a
//   defer-restore. The unexported version-fns (pnpmVersionFn, bunVersionFn,
//   nodeVersionFn) are the internal default implementation inside DetectState
//   — a cross-package test cannot assign them; it assigns DetectStateFn instead
//   (research Pattern 3 / shim.go osLookPath idiom — T-08-10b).
//
// Gateway-only Cache:
//   Constructed ONCE by the gateway; NEVER by the one-shot check hook.
//   A one-shot process gets no cache hits — flag 2 Position B. The gateway
//   wraps DetectStateFn in this cache; the check path calls DetectStateFn
//   fresh.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// detectionTimeout is the hard deadline for each PM version exec call (§10-12,
// T-08-11). Documented as a package-level var so tests can tighten it and so
// callers (gateway, check hook) see the same tunable value.
//
// Pitfall 1 (Windows corepack timing): corepack-shimmed pnpm on Windows may be
// slower on first call due to Node bootstrap. The 2s timeout is intentional —
// on timeout the PM is treated as "not installed" and the agent proceeds (never
// blocks). The gateway 60s Cache amortizes the cost for long-lived sessions.
var detectionTimeout = 2 * time.Second

// pnpmVersionFn, bunVersionFn, nodeVersionFn are the injectable package-level
// vars for the PM version exec calls. Tests substitute slow/erroring fakes to
// prove the timeout/error fallback without a real binary (research Pattern 3,
// shim.go osLookPath idiom). Real implementations use exec.CommandContext with
// the detectionTimeout hard deadline. T-08-10: executes FIXED argv only —
// "pnpm"/"bun"/"node", "--version". No user-controlled path or argument is
// ever passed.

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
// with a 2s hard timeout and, when cfg.CheckSocketScanner is true, scans
// bunfig.toml / pnpm-workspace.yaml.
//
// Fail-open contract: on any timeout or error a PM is treated as "not installed"
// (graceful fallback — never blocks on detection failure, PRD §10 criterion 12).
//
// The check hook calls DetectStateFn (which defaults to DetectState) directly on
// every invocation with no cache. The gateway wraps DetectStateFn in a 60s Cache.
//
// T-08-10: executes fixed argv ("pnpm"/"bun"/"node", "--version") only.
// T-08-11: 2s hard timeout per exec; timeout → PM not installed, proceed.
func DetectState(ctx context.Context, cfg Config) PMState {
	var state PMState

	// Detect pnpm.
	if ver, err := pnpmVersionFn(ctx); err == nil && ver != "" {
		state.PnpmInstalled = true
		state.PnpmVersion = ver

		// Compute hardening: version floor AND pnpm-workspace hardening scan.
		hardeningOK := meetsFloor(ver, cfg.VersionFloors.Pnpm)
		if hardeningOK {
			// Check for explicit hardening weaknesses in pnpm-workspace.yaml.
			wsPath := pnpmWorkspacePath()
			hr := DetectPnpmHardening(wsPath)
			// Hardened stays true unless the workspace explicitly removes hardening.
			// Weakness is logged but does not flip hardened (§10-16).
			_ = hr.WeaknessLogged // weakness logging is handled by the caller/audit layer
			state.PnpmHardened = hr.Hardened
		}
		// If version does not meet floor, PnpmHardened stays false.
	}
	// On pnpmVersionFn error/timeout: PnpmInstalled=false, proceed (fail-open).

	// Detect bun (bun may be absent on dev machine — tests use injected fns).
	if ver, err := bunVersionFn(ctx); err == nil && ver != "" {
		state.BunInstalled = true
		state.BunVersion = ver

		// BunScannerOK: checked only when cfg.CheckSocketScanner is true.
		if cfg.CheckSocketScanner {
			paths := bunfigPaths()
			state.BunScannerOK = DetectBunScanner(paths)
		}
	}
	// On bunVersionFn error/timeout: BunInstalled=false, proceed (fail-open).

	// Detect node.
	if ver, err := nodeVersionFn(ctx); err == nil && ver != "" {
		state.NodeVersion = ver
	}
	// On nodeVersionFn error/timeout: NodeVersion="", proceed (fail-open).

	return state
}

// DetectStateFn is the EXPORTED swappable detection seam.
//
// The check adapter (Plan 06) and the per-request gateway path resolve PMState
// through DetectStateFn, NOT DetectState directly, so a behavioral test in
// another package — e.g. package check (Plan 07) — can inject a synthetic
// PMState without a real pnpm/bun on PATH. The unexported pnpmVersionFn /
// bunVersionFn / nodeVersionFn remain the internal default implementation
// reachable only from package nudge; cross-package tests swap DetectStateFn
// and defer-restore it (T-08-10b).
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
// ONCE by the gateway at startup and reused across requests.
//
// NEVER constructed in the one-shot check hook (a one-shot process gets no
// cache hits — Flag 2 Position B). The gateway wraps DetectStateFn in this
// cache; the check path calls DetectStateFn fresh.
//
// The now field is injectable for tests (§10-11 TTL test via fake clock).
// In production code NewCache sets now to time.Now.
type Cache struct {
	mu      sync.Mutex
	detect  func(context.Context, Config) PMState
	ttl     time.Duration
	now     func() time.Time
	state   PMState
	have    bool
	expires time.Time
}

// NewCache creates a new Cache wrapping the given detect function with the
// given TTL. In production callers pass DetectStateFn and 60*time.Second.
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
// underlying detect function to refresh it. Concurrent callers are serialized
// by the mutex — only one refresh happens per TTL window.
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
