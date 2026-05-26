package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// watchPollMin is the minimum allowed PollInterval. An interval shorter
	// than this would cause excessive disk reads and thrash the Bumblebee CDN.
	watchPollMin = 5 * time.Minute

	// watchPollMax is the maximum allowed PollInterval.
	watchPollMax = 24 * time.Hour

	// watchPollDefault is the production default when PollInterval is zero.
	watchPollDefault = time.Hour

	// bumblebeeSource is the canonical source name for the Bumblebee catalog.
	bumblebeeSource = "bumblebee"
)

// SnapshotFunc is a function that returns the current entry count and content
// hash of a catalog source without triggering a network sync. The injectable
// seam exists so the watch loop is testable without a real network; production
// sets it to readBumblebeeSnapshot.
//
// It must be fast (reading from disk or mmap, never from the network).
type SnapshotFunc func(ctx context.Context) (count int, hash string, err error)

// WatchConfig controls the catalog watch daemon parameters.
type WatchConfig struct {
	// PollInterval is how often the watch loop checks for catalog changes.
	// Clamped to [5m, 24h]; zero becomes 1h.
	PollInterval time.Duration

	// CatalogDir is the directory where bumblebee.json and bumblebee.idx live.
	// Defaults to the production catalog directory if empty.
	CatalogDir string

	// StateFile is the path to state.json for delta state persistence.
	// Defaults to ~/.beekeeper/state.json if empty.
	StateFile string

	// Client is the HTTP client used for future network operations.
	// May be nil; Watch does not make network calls itself (network calls are
	// made by Sync, which the caller triggers via the onDelta callback).
	Client *http.Client

	// Sanity holds the alert and hard-block thresholds for delta validation.
	// Zero value is replaced with DefaultSanityConfig() on first use.
	Sanity SanityConfig

	// Snapshot is the injectable function that reads the current catalog state
	// from disk without network I/O. Production leaves this nil and Watch
	// sets it to readBumblebeeSnapshot using CatalogDir. Tests inject a fake.
	Snapshot SnapshotFunc

	// minPollInterval overrides watchPollMin for testing. Zero means use the
	// production default (5m). Tests set this to a short duration (e.g. 10ms)
	// to avoid multi-minute waits. Never set in production code.
	minPollInterval time.Duration
}

// clampInterval returns interval clamped to [minPoll, watchPollMax].
// A zero interval becomes watchPollDefault. minPoll defaults to watchPollMin
// (5m) when zero — this keeps the production guard active.
func clampInterval(interval, minPoll time.Duration) time.Duration {
	if minPoll == 0 {
		minPoll = watchPollMin
	}
	if interval == 0 {
		return watchPollDefault
	}
	if interval < minPoll {
		return minPoll
	}
	if interval > watchPollMax {
		return watchPollMax
	}
	return interval
}

// readBumblebeeSnapshot reads bumblebee.json from catalogDir and returns the
// entry count and SHA-256 hex digest of the raw file bytes. It is the
// production implementation of SnapshotFunc: no network, no mmap open/close
// overhead, just a stat + read of the raw cache.
//
// If bumblebee.json does not exist (catalog not yet synced), it returns
// count=0 and hash="" with a nil error so the watch loop treats it as an
// "unchanged empty state" until the first sync runs.
func readBumblebeeSnapshot(catalogDir string) SnapshotFunc {
	return func(ctx context.Context) (int, string, error) {
		path := filepath.Join(catalogDir, "bumblebee.json")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return 0, "", nil
			}
			return 0, "", fmt.Errorf("read bumblebee.json: %w", err)
		}

		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])

		// Count entries by opening the mmap index, which is the authoritative
		// source of truth. If the index does not exist yet, fall back to 0.
		idxPath := filepath.Join(catalogDir, "bumblebee.idx")
		idx, err := OpenIndex(idxPath)
		if err != nil {
			// Index missing or corrupt — return 0 count but valid hash.
			return 0, hash, nil
		}
		count := idx.Count()
		_ = idx.Close()

		return count, hash, nil
	}
}

// computeDelta reads the current catalog snapshot, compares it to the prior
// persisted state, runs CheckSanity on any delta, updates the WatchState
// (setting Degraded if sanity breaches), and persists the new state via
// SaveState.
//
// It returns the CatalogDelta (which may have HasChanges()==false), the updated
// WatchState, and the SanityResult for the current tick.
//
// On error, the prior state is unchanged on disk and WatchState{} is returned.
func computeDelta(ctx context.Context, cfg WatchConfig, prev WatchState) (CatalogDelta, WatchState, SanityResult, error) {
	snapshotFn := cfg.Snapshot
	if snapshotFn == nil {
		snapshotFn = readBumblebeeSnapshot(cfg.CatalogDir)
	}

	newCount, newHash, err := snapshotFn(ctx)
	if err != nil {
		return CatalogDelta{}, WatchState{}, SanityResult{}, fmt.Errorf("snapshot: %w", err)
	}

	prevSS := prev.Sources[bumblebeeSource]

	delta := CatalogDelta{
		Source:     bumblebeeSource,
		PrevHash:   prevSS.Hash,
		NewHash:    newHash,
		PrevCount:  prevSS.Count,
		NewCount:   newCount,
		DeltaCount: newCount - prevSS.Count,
	}

	sanity := cfg.Sanity
	if sanity.AlertDeltaEntries == 0 && sanity.BlockDeltaEntries == 0 {
		sanity = DefaultSanityConfig()
	}

	result := CheckSanity(prevSS.Count, newCount, sanity)

	// Build the new SourceState.
	newSS := SourceState{
		Hash:  newHash,
		Count: newCount,
	}

	// A Block or Alert from CheckSanity degrades the source.
	// Once degraded, keep the source degraded until it is explicitly cleared
	// (that is the job of `catalogs verify` in Plan 08). However, we update
	// the hash and count so subsequent polls compare against the new baseline
	// rather than amplifying the original shock delta every tick.
	if result.Block {
		newSS.Degraded = true
		newSS.DegradedReason = result.Reason
	} else if result.Alert {
		newSS.Degraded = true
		newSS.DegradedReason = result.Reason
	} else if prevSS.Degraded {
		// Preserve an existing degradation across ticks — it must be explicitly
		// cleared by the operator (Plan 08 `catalogs verify --clear-degraded`).
		newSS.Degraded = true
		newSS.DegradedReason = prevSS.DegradedReason
	}

	newState := WatchState{
		Sources: make(map[string]SourceState, len(prev.Sources)+1),
	}
	for k, v := range prev.Sources {
		newState.Sources[k] = v
	}
	newState.Sources[bumblebeeSource] = newSS

	if err := SaveState(cfg.StateFile, newState); err != nil {
		return delta, newState, result, fmt.Errorf("save state: %w", err)
	}

	return delta, newState, result, nil
}

// Watch runs the catalog watch daemon. It polls the Bumblebee catalog source on
// cfg.PollInterval, detects deltas against the persisted WatchState, runs the
// sanity check, and calls onDelta when the catalog has changed or a sanity
// threshold is breached.
//
// Watch is the production daemon entry point wired by `beekeeper catalogs watch`
// (Plan 08). It mirrors the tailAuditLog ticker pattern in cmd/beekeeper/main.go
// lines 200–242.
//
// Lifecycle:
//   - Watch blocks until ctx is cancelled (SIGTERM/SIGINT) — returns nil.
//   - On poll error, logs to stderr and continues on the next tick (degraded,
//     not fatal) — never busy-loops (T-02-07-02, T-02-07-03).
//   - PollInterval is clamped to [5m, 24h]; zero → 1h default (T-02-07-02).
//
// The onDelta callback is called from the Watch goroutine's own context. It
// must not block for longer than the poll interval or it will delay the next
// tick. Audit recording and re-scan triggering (Plan 08) should be done in the
// callback.
func Watch(ctx context.Context, cfg WatchConfig, onDelta func(CatalogDelta, SanityResult)) error {
	cfg.PollInterval = clampInterval(cfg.PollInterval, cfg.minPollInterval)

	st, err := LoadState(cfg.StateFile)
	if err != nil {
		// A corrupt state file is non-fatal: start with empty state.
		fmt.Fprintf(os.Stderr, "beekeeper watch: failed to load state %q: %v; starting fresh\n", cfg.StateFile, err)
		st = WatchState{Sources: make(map[string]SourceState)}
	}

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			delta, newState, sanityResult, saveErr := computeDelta(ctx, cfg, st)

			// Always update the in-memory state so degraded flags persist across
			// ticks even when the disk write (SaveState) failed (CR-06).
			// newState.Sources == nil only when computeDelta failed before building
			// any state (snapshot error), in which case we keep the prior st.
			if newState.Sources != nil {
				st = newState
			}

			// Fire the callback for any meaningful event regardless of save error
			// so a valid delta is never silently swallowed (WR-06).
			if delta.HasChanges() || sanityResult.Alert || sanityResult.Block {
				if onDelta != nil {
					onDelta(delta, sanityResult)
				}
			}

			if saveErr != nil {
				fmt.Fprintf(os.Stderr, "beekeeper watch: state save error: %v\n", saveErr)
			}
		}
	}
}
