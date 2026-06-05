package sentry

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// InventoryStore maintains a live InventorySnapshot of recently-installed
// editor extensions. It is safe for concurrent use and is designed to be
// embedded in the Sentry daemon loop so SENTRY-004 and SENTRY-005 have a
// real inventory to evaluate against (TM-RS-01).
//
// The store is populated two ways:
//  1. ScanDirs: a full walk of all watch directories, called at daemon startup
//     and on a periodic refresh cadence (default 30 s).
//  2. RecordInstall: called immediately when a new extension directory is
//     detected by an fsnotify-backed watcher, if one is wired to the daemon.
//
// Any extension whose recorded install time is within InventoryTTL of the
// current wall-clock time is included in Snapshot(). Extensions older than
// InventoryTTL are pruned on the next ScanDirs or RecordInstall call.
type InventoryStore struct {
	mu           sync.RWMutex
	extensions   map[string]time.Time // extID (publisher.name) → install time
	InventoryTTL time.Duration        // how far back SENTRY-004/005 look; defaults to 30 min
}

// NewInventoryStore returns an InventoryStore with TTL defaulting to 30 minutes
// (matching RuleConfig.FreshExtWindowMin default in applyDefaults).
func NewInventoryStore() *InventoryStore {
	return &InventoryStore{
		extensions:   make(map[string]time.Time),
		InventoryTTL: 30 * time.Minute,
	}
}

// RecordInstall records that an extension with the given ID was installed at t.
// extID should be "publisher.name" (lower-cased) for consistency with the rule
// engine's InventorySnapshot.RecentExtensions key convention.
func (s *InventoryStore) RecordInstall(extID string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extensions[extID] = t
}

// Snapshot returns an immutable InventorySnapshot containing only extensions
// whose install time is within InventoryTTL of now. Older entries are pruned
// from the store in the same pass to bound memory use.
func (s *InventoryStore) Snapshot(now time.Time) InventorySnapshot {
	cutoff := now.Add(-s.InventoryTTL)
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := make(map[string]time.Time, len(s.extensions))
	for id, t := range s.extensions {
		if t.Before(cutoff) {
			delete(s.extensions, id)
			continue
		}
		snap[id] = t
	}
	return InventorySnapshot{RecentExtensions: snap}
}

// ScanDirs walks each directory in dirs, parses the modification time of each
// immediate child directory whose name follows the "publisher.name[-version]"
// extension layout, and records it as an install event. Directories that
// cannot be read are silently skipped (best-effort; daemon must not fail-open
// on a missing extension directory).
//
// ScanDirs is intended to be called at daemon startup and then on a periodic
// refresh cadence (e.g. every 30 s) so that extensions installed between
// refreshes are picked up without requiring a live fsnotify watcher inside
// the Sentry daemon process.
func (s *InventoryStore) ScanDirs(dirs []string, now time.Time) {
	cutoff := now.Add(-s.InventoryTTL)
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Derive extID from directory name. VS Code extension directories
			// are named "publisher.name-version" (e.g. "ms-python.python-2026.4.0").
			// We extract "publisher.name" by taking everything before the first
			// "-" that appears AFTER a "." (to avoid splitting "ms-python.python").
			extID := extensionIDFromDirName(entry.Name())
			if extID == "" {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}
			modTime := info.ModTime()
			if modTime.Before(cutoff) {
				// Extension predates the TTL window — skip (and prune if present).
				delete(s.extensions, extID)
				continue
			}
			// Record the most-recent known install time for this extID.
			if existing, ok := s.extensions[extID]; !ok || modTime.After(existing) {
				s.extensions[extID] = modTime
			}
		}
	}
}

// extensionIDFromDirName extracts a "publisher.name" identifier from a VS Code
// extension directory name of the form "publisher.name-version". It returns ""
// if the name does not contain a dot, i.e. is not a plausible extension dir.
//
// Examples:
//
//	"ms-python.python-2026.4.0"    → "ms-python.python"
//	"GitHub.copilot-1.5.0"         → "github.copilot"   (lowercased)
//	"extensions.json"               → ""                 (no dot → skip)
//	".obsolete"                     → ""                 (dot-prefixed → skip)
func extensionIDFromDirName(name string) string {
	if len(name) == 0 || name[0] == '.' {
		return "" // hidden / dot-file
	}
	dotIdx := strings.Index(name, ".")
	if dotIdx < 0 {
		return "" // no dot → not an extension directory
	}
	// publisher portion is everything before the dot
	publisher := name[:dotIdx]
	rest := name[dotIdx+1:]

	// The name portion ends at the first "-" that comes after the publisher.name
	// separator.  If there is no "-", the whole rest is the name (no version suffix).
	nameOnly := rest
	if dashIdx := strings.Index(rest, "-"); dashIdx >= 0 {
		nameOnly = rest[:dashIdx]
	}
	if publisher == "" || nameOnly == "" {
		return ""
	}
	return strings.ToLower(publisher) + "." + strings.ToLower(nameOnly)
}

// expandHome replaces a leading "~" with the OS home directory. Mirrors the
// same helper in internal/watch/watcher.go; duplicated here to keep the sentry
// package free of a watch import (avoids circular dependency).
func expandHome(dir string) string {
	if len(dir) == 0 || dir[0] != '~' {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	return filepath.Join(home, dir[1:])
}

// ExpandedWatchDirs expands "~" in each element of dirs and returns the result.
// Used by daemons to normalise cfg.WatchDirectories() before passing to ScanDirs.
func ExpandedWatchDirs(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, expandHome(d))
	}
	return out
}
