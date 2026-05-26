package catalog

import "fmt"

// SanityConfig holds the configurable alert and hard-block thresholds for
// catalog delta and total entry counts. A catalog source that suddenly balloons
// by thousands of entries is treated as suspect and degraded to warning-only
// rather than trusted to block (CTLG-08, CLAUDE.md Phase 2 self-defense).
type SanityConfig struct {
	// AlertDeltaEntries is the delta threshold above which the source is marked
	// warning-only (Alert). Default: 1000.
	AlertDeltaEntries int

	// BlockDeltaEntries is the hard delta threshold above which the source is
	// degraded (Block). Default: 10000.
	BlockDeltaEntries int

	// AlertTotalEntries is the total-entry threshold above which the source is
	// marked warning-only (Alert). Default: 100000.
	AlertTotalEntries int

	// AlertVersionsPerPkg is reserved for future per-package version count
	// checks. Default: 1000.
	AlertVersionsPerPkg int
}

// DefaultSanityConfig returns the recommended production threshold set.
// These values are documented in CLAUDE.md Phase 2 self-defense non-negotiables:
// alert >1000 deltas, hard-block >10000 deltas, alert >100000 total entries.
func DefaultSanityConfig() SanityConfig {
	return SanityConfig{
		AlertDeltaEntries:   1000,
		BlockDeltaEntries:   10000,
		AlertTotalEntries:   100000,
		AlertVersionsPerPkg: 1000,
	}
}

// SanityResult is the outcome of a catalog sanity check.
type SanityResult struct {
	// Alert is true when an alert threshold is exceeded — source is downgraded
	// to warn-only but not fully degraded.
	Alert bool

	// Block is true when a hard threshold is exceeded — source is degraded and
	// its matches count at most 0.5 toward corroboration (warning-only).
	Block bool

	// Reason is a human-readable explanation for the alert or block decision.
	// Empty when both Alert and Block are false.
	Reason string
}

// CheckSanity validates catalog delta and total sizes against configured
// thresholds. It is a pure function — no I/O — so it is safe to call from the
// watch loop, the hook handler, or tests without side effects.
//
// Decision table (evaluated in priority order):
//  1. abs(newCount - prevCount) > BlockDeltaEntries → Block=true
//  2. abs(newCount - prevCount) > AlertDeltaEntries → Alert=true
//  3. newCount > AlertTotalEntries                  → Alert=true
//  4. otherwise                                     → SanityResult{}
func CheckSanity(prevCount, newCount int, cfg SanityConfig) SanityResult {
	delta := newCount - prevCount
	if delta < 0 {
		delta = -delta
	}

	switch {
	case delta > cfg.BlockDeltaEntries:
		return SanityResult{
			Block:  true,
			Reason: fmt.Sprintf("delta %d exceeds hard limit %d", delta, cfg.BlockDeltaEntries),
		}
	case delta > cfg.AlertDeltaEntries:
		return SanityResult{
			Alert:  true,
			Reason: fmt.Sprintf("delta %d exceeds alert threshold %d", delta, cfg.AlertDeltaEntries),
		}
	case newCount > cfg.AlertTotalEntries:
		return SanityResult{
			Alert:  true,
			Reason: fmt.Sprintf("total %d exceeds alert threshold %d", newCount, cfg.AlertTotalEntries),
		}
	default:
		return SanityResult{}
	}
}
