// Package baseline provides per-project behavioral baseline counter persistence.
// Counters are stored as owner-only JSON (0600) and written atomically to
// prevent partial writes from corrupting the baseline state.
package baseline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bantuson/beekeeper/internal/platform"
	"github.com/bantuson/beekeeper/internal/policy"
)

// Store persists per-project behavioral baseline counters to disk.
// The file is owner-only (0600) — it contains frequency data about the
// developer's work patterns (T-02-08-04).
type Store struct {
	path string
}

// NewStore returns a Store backed by path (typically under
// ~/.beekeeper/baselines/<project-hash>.json). It creates the parent directory
// with 0o700 permissions if it does not exist.
func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create baselines directory: %w", err)
	}
	return &Store{path: path}, nil
}

// Load reads the baseline counters from disk.
//
// A missing file is normal (first-run case) and returns empty counters with a
// non-nil Counts map and a nil error, mirroring the config.Load missing-file-is-OK
// pattern. Any other I/O error is returned as-is.
func (s *Store) Load() (policy.BaselineCounters, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return policy.BaselineCounters{Counts: map[string][]int64{}}, nil
		}
		return policy.BaselineCounters{}, fmt.Errorf("read baseline %q: %w", s.path, err)
	}

	var bc policy.BaselineCounters
	if err := json.Unmarshal(data, &bc); err != nil {
		return policy.BaselineCounters{}, fmt.Errorf("parse baseline %q: %w", s.path, err)
	}

	// Ensure Counts is never nil — callers must be able to do map reads safely.
	if bc.Counts == nil {
		bc.Counts = map[string][]int64{}
	}

	return bc, nil
}

// Save atomically persists the baseline counters to disk and enforces
// owner-only permissions (0600 on Unix, owner-only DACL on Windows).
//
// The atomic write (temp file + rename) ensures that a crash during the write
// never leaves a partially-written baseline that could corrupt the counters.
func (s *Store) Save(bc policy.BaselineCounters) error {
	data, err := json.Marshal(bc)
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}
	if err := writeBaselineAtomic(s.path, data); err != nil {
		return err
	}
	return platform.SetOwnerOnly(s.path)
}

// writeBaselineAtomic writes data to a temp file in the same directory as path
// then renames it over path. This mirrors the catalog/index.go writeFileAtomic
// pattern — a partial write never leaves a corrupt baseline file.
func writeBaselineAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
