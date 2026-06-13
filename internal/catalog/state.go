package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SourceState holds the persisted per-source delta state. It is stored under
// the "sources" key in state.json, keyed by source name (e.g. "bumblebee").
//
// This is the canonical state record for the watch daemon: it survives process
// restart and allows Watch to compute a delta against the prior sync even after
// `beekeeper` is restarted between poll cycles (CTLG-06).
type SourceState struct {
	// Hash is the content hash of the last-seen catalog snapshot. The watch
	// loop uses SHA-256 of the raw catalog file to detect changes without
	// loading the entire entry set.
	Hash string `json:"hash"`

	// Count is the number of entries in the last-seen catalog snapshot.
	Count int `json:"count"`

	// Degraded is true when a sanity check has caused the source to be
	// downgraded. A degraded source's matches count at most 0.5 toward
	// corroboration (warning-only), not the full 1.0 (CTLG-08).
	Degraded bool `json:"degraded"`

	// DegradedReason is the human-readable reason for degradation.
	// Empty when Degraded is false.
	DegradedReason string `json:"degraded_reason,omitempty"`

	// LastSuccess is the time of the most recent SUCCESSFUL sync (200 fetch+
	// rebuild OR a 304 not-modified confirmation). The interval gate keys off
	// this: `catalogs sync` no-ops unless time.Since(LastSuccess) >= the
	// configured interval (Phase 20, CSYNC). Zero value = never synced.
	LastSuccess time.Time `json:"last_success,omitempty"`

	// LastAttempt is the time of the most recent sync attempt regardless of
	// outcome. LastAttempt > LastSuccess means the last attempt failed — the TUI
	// renders that amber rather than "fresh".
	LastAttempt time.Time `json:"last_attempt,omitempty"`

	// LastError is the error string from the most recent failed attempt, cleared
	// on the next success. Empty when the last attempt succeeded.
	LastError string `json:"last_error,omitempty"`

	// ETag is the GitHub Contents-list ETag from the last successful 200, sent
	// as If-None-Match on the next sync so an unchanged upstream returns 304
	// (skip fetch + rebuild). Empty when never captured.
	ETag string `json:"etag,omitempty"`
}

// WatchState is the complete persisted watch-daemon state, written atomically
// to ~/.beekeeper/state.json after every poll cycle that produces a delta.
type WatchState struct {
	// Sources maps source name (e.g. "bumblebee") to its per-source state.
	Sources map[string]SourceState `json:"sources"`

	// SelfQuarantine is set when CheckSelfCatalog determines the running
	// binary version is listed in the beekeeper-self compromise feed.
	// The omitempty tag ensures backward compatibility: existing state.json
	// files without this field parse cleanly (field reads as nil), and
	// re-written state.json files without an active quarantine omit the key.
	SelfQuarantine *SelfQuarantineState `json:"self_quarantine,omitempty"`

	// CorpusLocalSalt is the per-installation HMAC key for repo fingerprinting
	// (Phase 23, STORE-05). Generated once via crypto/rand on first corpus store
	// init and persisted here so that RepoFingerprint and FleetNodeID are stable
	// across process restarts for a given installation, but differ between
	// installations of the same repo. The omitempty tag ensures backward
	// compatibility: existing state.json files without this field parse cleanly.
	CorpusLocalSalt string `json:"corpus_local_salt,omitempty"`
}

// SelfQuarantineState records the details of an active self-quarantine event
// as persisted to state.json. It is read on every startup BEFORE any network
// fetch so that a previous quarantine decision is honored offline.
type SelfQuarantineState struct {
	// Version is the beekeeper version string that triggered quarantine
	// (e.g. "v0.4.2"). Stored for display purposes and to allow the startup
	// check to match the currently running version.
	Version string `json:"version"`

	// EntryID is the beekeeper-self catalog entry ID that matched
	// (e.g. "beekeeper-self-2026-001").
	EntryID string `json:"entry_id"`

	// Reason is the human-readable description from the catalog entry
	// (e.g. "Beekeeper v0.4.2 release pipeline compromise").
	Reason string `json:"reason"`

	// FiredAt is the RFC3339 timestamp of when self-quarantine was triggered.
	FiredAt string `json:"fired_at"`
}

// LoadState reads the watch state from path.
//
// A missing file is normal — it means this is a first run — so it returns an
// empty WatchState with a non-nil Sources map and a nil error. This mirrors the
// config.Load missing-file-is-OK pattern (see internal/config/config.go).
func LoadState(path string) (WatchState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return WatchState{Sources: make(map[string]SourceState)}, nil
		}
		return WatchState{}, fmt.Errorf("read state %q: %w", path, err)
	}

	var st WatchState
	if err := json.Unmarshal(data, &st); err != nil {
		return WatchState{}, fmt.Errorf("parse state %q: %w", path, err)
	}

	// Ensure Sources is never nil — callers must be able to do map reads safely.
	if st.Sources == nil {
		st.Sources = make(map[string]SourceState)
	}

	return st, nil
}

// SaveState atomically writes the watch state to path. It creates parent
// directories with owner-only permissions (0o700) before writing.
//
// Writes are performed via writeFileAtomic (temp file + rename) so a crash
// during the write never leaves a partially-written state.json that could mask
// a prior Degraded mark.
func SaveState(path string, st WatchState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state directory %q: %w", dir, err)
	}

	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := writeFileAtomic(path, data); err != nil {
		return fmt.Errorf("write state %q: %w", path, err)
	}

	return nil
}

// CatalogDelta records the before/after state of a single catalog sync cycle
// for delta detection and audit provenance (CTLG-09).
type CatalogDelta struct {
	// Source is the catalog source name (e.g. "bumblebee").
	Source string

	// PrevHash is the content hash from the prior SourceState (empty on first run).
	PrevHash string

	// NewHash is the content hash from the current snapshot.
	NewHash string

	// PrevCount is the entry count from the prior SourceState (0 on first run).
	PrevCount int

	// NewCount is the entry count from the current snapshot.
	NewCount int

	// DeltaCount is NewCount - PrevCount (signed).
	DeltaCount int
}

// HasChanges reports whether the catalog content has changed since the last
// persisted snapshot. Hash comparison is used rather than count comparison
// because the count can stay the same while entries change (e.g. a severity
// update replaces one entry with another).
func (d CatalogDelta) HasChanges() bool {
	return d.PrevHash != d.NewHash
}
