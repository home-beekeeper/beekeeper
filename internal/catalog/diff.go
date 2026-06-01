package catalog

import (
	"context"
	"fmt"
	"net/http"
	"sort"
)

// DiffResult holds the computed delta for a single catalog source.
// "Added" and "Removed" are entry count deltas; "Changed" indicates the content
// hash changed (true even if entry count is the same — e.g. severity updates).
type DiffResult struct {
	// Source is the catalog source name (e.g. "bumblebee").
	Source string

	// PrevCount is the persisted entry count from the last sync.
	PrevCount int

	// CurrentCount is the entry count from the current on-disk snapshot.
	CurrentCount int

	// Added is the number of new entries (positive delta).
	Added int

	// Removed is the number of removed entries (negative delta becomes positive here).
	Removed int

	// Changed is true when the content hash has changed since the last sync,
	// regardless of whether the count delta is zero.
	Changed bool

	// PrevHash is the content hash from the persisted state.
	PrevHash string

	// CurrentHash is the content hash from the current on-disk snapshot.
	CurrentHash string

	// Degraded mirrors SourceState.Degraded — included for display convenience.
	Degraded bool

	// DegradedReason is the human-readable degradation reason when Degraded is true.
	DegradedReason string
}

// HasChanges reports whether this source has any diff vs the persisted state.
func (d DiffResult) HasChanges() bool {
	return d.Changed || d.Added != 0 || d.Removed != 0
}

// Diff computes the delta between the current on-disk catalog state and the
// last-synced (persisted) state in stateFile. It is read-only — no mutation,
// no network calls, no enforcement side effects (PRD §10).
//
// For each source recorded in stateFile, Diff reads the current snapshot via
// a SnapshotFunc (production: readBumblebeeSnapshot). Sources in stateFile that
// have no snapshot function are included with CurrentCount=0 (indicating the
// catalog file may be absent or not yet generated).
//
// client is accepted for API compatibility but is not used in the current
// implementation (all reads are from local disk snapshots). It may be used in
// a future OSV/Socket diff implementation.
func Diff(ctx context.Context, stateFile, catalogDir string, client *http.Client) ([]DiffResult, error) {
	st, err := LoadState(stateFile)
	if err != nil {
		return nil, fmt.Errorf("diff: load state %q: %w", stateFile, err)
	}

	// Map of known per-source snapshot readers. Add additional sources here when
	// OSV/Socket diff support is added.
	snapshotReaders := map[string]SnapshotFunc{
		bumblebeeSource: readBumblebeeSnapshot(catalogDir),
	}

	// Sort sources for deterministic output.
	sources := make([]string, 0, len(st.Sources))
	for src := range st.Sources {
		sources = append(sources, src)
	}
	// Also include any snapshot sources not yet in state.
	for src := range snapshotReaders {
		if _, seen := st.Sources[src]; !seen {
			sources = append(sources, src)
		}
	}
	sort.Strings(sources)

	results := make([]DiffResult, 0, len(sources))
	for _, src := range sources {
		prevState := st.Sources[src] // zero value if absent

		var currentCount int
		var currentHash string

		if snapshotFn, ok := snapshotReaders[src]; ok {
			count, hash, snapErr := snapshotFn(ctx)
			if snapErr != nil {
				// Non-fatal: include a result with zero current state and note the error.
				currentHash = "(read error: " + snapErr.Error() + ")"
			} else {
				currentCount = count
				currentHash = hash
			}
		}

		added := 0
		removed := 0
		delta := currentCount - prevState.Count
		if delta > 0 {
			added = delta
		} else if delta < 0 {
			removed = -delta
		}

		changed := prevState.Hash != currentHash && currentHash != "" && !startsWith(currentHash, "(read error")

		results = append(results, DiffResult{
			Source:         src,
			PrevCount:      prevState.Count,
			CurrentCount:   currentCount,
			Added:          added,
			Removed:        removed,
			Changed:        changed,
			PrevHash:       prevState.Hash,
			CurrentHash:    currentHash,
			Degraded:       prevState.Degraded,
			DegradedReason: prevState.DegradedReason,
		})
	}

	return results, nil
}

// startsWith is a minimal helper to avoid importing strings in this pure-data file.
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
