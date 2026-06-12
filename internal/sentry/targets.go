package sentry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// TargetEntry is a single entry in the TargetList, recording a flagged and
// installed artifact that the Sentry correlation engine should track.
//
// DETECTION-ONLY: the TargetList only tightens correlation thresholds for
// matching process subtrees. It NEVER triggers kill, isolate, or network-cut.
type TargetEntry struct {
	// Name is the package/extension name from the scan hit.
	Name string `json:"name"`
	// Path is the on-disk path of the artifact (may be "" for pending entries).
	Path string `json:"path,omitempty"`
	// ExpectedProcess is the base-name of the process expected to execute this
	// artifact (e.g. "node" for npm packages, "python" for pip packages).
	// Used by MatchesPID to walk the process tree for tightening decisions.
	ExpectedProcess string `json:"expected_process,omitempty"`
}

// TargetList holds the in-memory list of catalog-flagged installed artifacts
// that the Sentry daemon consults to tighten correlation thresholds.
//
// The type is pure: AddTarget and MatchesPID perform no I/O. LoadTargets and
// SaveTargets are the only I/O helpers; the correlation engine hot path is I/O-free.
type TargetList struct {
	Entries []TargetEntry `json:"targets"`
}

// AddTarget appends an entry to the target list.
// Duplicate names are silently skipped (idempotent — re-running a scan after a
// catalog delta must not cause duplicate target entries).
func (tl *TargetList) AddTarget(name, path, expectedProcess string) {
	if tl == nil {
		return
	}
	for _, e := range tl.Entries {
		if e.Name == name {
			return // already tracked
		}
	}
	tl.Entries = append(tl.Entries, TargetEntry{
		Name:            name,
		Path:            path,
		ExpectedProcess: expectedProcess,
	})
}

// MatchesPID reports whether the given PID (or any ancestor up to 32 hops)
// matches a target entry, either by exe base-name matching ExpectedProcess OR
// by the executable path matching the target's Path.
//
// Returns true when any target's ExpectedProcess matches any exe in the
// PID's ancestor chain (using the existing isDescendantOf walk), OR when
// the PID's own exe matches a target Path.
//
// DETECTION-ONLY: this only returns a boolean used to decide whether to apply
// tightened thresholds; no kill/isolate/network-cut is performed.
func (tl *TargetList) MatchesPID(pid uint32, tree map[uint32]ProcessNode) bool {
	if tl == nil || len(tl.Entries) == 0 {
		return false
	}

	// Build a per-call set of ExpectedProcess names from all target entries.
	exeSet := make(map[string]bool, len(tl.Entries))
	for _, entry := range tl.Entries {
		if entry.ExpectedProcess != "" {
			exeSet[entry.ExpectedProcess] = true
		}
	}

	// Use existing isDescendantOf walk: if any ancestor exe matches an
	// expected-process, the PID is in scope for tightened thresholds.
	if len(exeSet) > 0 && isDescendantOf(pid, tree, exeSet) {
		return true
	}

	// Also match by direct exe path (if the process itself IS the flagged
	// artifact's expected process).
	if node, ok := tree[pid]; ok {
		for _, entry := range tl.Entries {
			if entry.Path != "" && filepath.Clean(node.Exe) == filepath.Clean(entry.Path) {
				return true
			}
		}
	}

	return false
}

// LoadTargets reads a sentry-targets.json from path and returns a populated
// TargetList. If the file does not exist, an empty TargetList is returned
// (not an error — a fresh install has no targets yet). Any other read/parse
// error is returned to the caller (fail-closed).
//
// The engine hot path (EvaluateEvent) never calls this; only the daemon calls
// it once at startup and after each first-responder run (I/O stays outside
// the hot path, mirroring catalog.LoadState/SaveState).
func LoadTargets(path string) (*TargetList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &TargetList{}, nil
		}
		return nil, fmt.Errorf("sentry: load targets %q: %w", path, err)
	}
	var tl TargetList
	if err := json.Unmarshal(data, &tl); err != nil {
		return nil, fmt.Errorf("sentry: parse targets %q: %w", path, err)
	}
	return &tl, nil
}

// SaveTargets writes the TargetList to path as indented JSON with 0600
// permissions. The directory is created if it does not exist.
// Called by the first-responder after each scan hit update and by the daemon
// after any AddTarget mutation.
func SaveTargets(path string, tl *TargetList) error {
	if tl == nil {
		tl = &TargetList{}
	}
	data, err := json.MarshalIndent(tl, "", "  ")
	if err != nil {
		return fmt.Errorf("sentry: marshal targets: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("sentry: mkdir for targets: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("sentry: write targets %q: %w", path, err)
	}
	return nil
}
