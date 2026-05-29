// Package check implements hook-latency persistence for beekeeper diag.
//
// beekeeper check is a one-shot process (one invocation per tool call), so
// the in-memory LatencyTracker resets on every run. This file persists each
// hook latency sample to a small JSON ring file (~/.beekeeper/hook-latency.json)
// that accumulates the last 100 samples across restarts. beekeeper diag reads
// this ring to compute p95/p99 from real production check invocations.
//
// All writes are best-effort: a write error is silently ignored and never
// alters the Result returned by runCheck (T-09-14 mitigation).
package check

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	// hookLatencyRingSize is the maximum number of samples kept in the ring file.
	hookLatencyRingSize = 100
	// hookLatencyFile is the file name under the beekeeper home directory.
	hookLatencyFile = "hook-latency.json"
)

// hookLatencyRing is the JSON representation of the persisted ring file.
type hookLatencyRing struct {
	Samples []int64 `json:"samples"`
}

// appendHookLatency appends a latency sample (in milliseconds) to the persisted
// ring at ringPath, keeping at most hookLatencyRingSize samples. The write is
// atomic (temp file + rename) so a crash never leaves a partial file.
//
// Best-effort: if ringPath is unwritable or any I/O error occurs, the error is
// silently discarded. runCheck's Result is never affected.
func appendHookLatency(ringPath string, ms int64) {
	existing := loadHookLatency(ringPath)
	existing = append(existing, ms)
	if len(existing) > hookLatencyRingSize {
		existing = existing[len(existing)-hookLatencyRingSize:]
	}
	ring := hookLatencyRing{Samples: existing}
	data, err := json.Marshal(ring)
	if err != nil {
		return // best-effort
	}
	_ = writeRingAtomic(ringPath, data)
}

// loadHookLatency reads the persisted ring at ringPath and returns the samples
// slice. A missing or corrupt file is treated as an empty sample set (T-09-15
// mitigation): diag will report 0 for p95/p99 rather than crashing.
func loadHookLatency(ringPath string) []int64 {
	data, err := os.ReadFile(ringPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // first run — no ring yet
		}
		return nil // corrupt / unreadable — treat as empty
	}
	var ring hookLatencyRing
	if err := json.Unmarshal(data, &ring); err != nil {
		return nil // corrupt JSON — treat as empty
	}
	return ring.Samples
}

// writeRingAtomic writes data to a temp file in the same directory as path,
// then renames it over path atomically. Parent directories are created with
// 0o700 permissions.
func writeRingAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
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
