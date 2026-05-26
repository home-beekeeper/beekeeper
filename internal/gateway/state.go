package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
)

// GatewayState is the persisted gateway daemon state. It is written atomically
// to ~/.beekeeper/state.json under the "gateway" key at startup and cleared on
// clean shutdown. It allows CLI commands (beekeeper gateway token, gateway
// status) to read the current token and port without connecting to the daemon.
//
// GatewayToken is a 64-char hex string (32 random bytes from crypto/rand).
// Permissions: state.json is written with 0o600 (owner-read-only) to protect
// the token from other local processes (T-04-03-01).
type GatewayState struct {
	GatewayToken string `json:"gateway_token"` // 64-char hex; rotates on each gateway restart
	BoundAddr    string `json:"bound_addr"`    // e.g. "127.0.0.1"
	BoundPort    int    `json:"bound_port"`
	StartedAt    string `json:"started_at"` // RFC3339
	PID          int    `json:"pid"`        // os.Getpid()
}

// topLevelState is the top-level structure of ~/.beekeeper/state.json.
// It preserves the existing "sources" key written by catalog.SaveState alongside
// the new "gateway" key. The two keys evolve independently; json.Unmarshal
// ignores unknown fields so adding "gateway" here is backward-compatible
// (Assumption A6 in RESEARCH.md confirmed: Go ignores unknown fields).
type topLevelState struct {
	Sources map[string]catalog.SourceState `json:"sources,omitempty"`
	Gateway *GatewayState                  `json:"gateway,omitempty"`
}

// LoadGatewayState reads the gateway state from state.json at path.
//
// A missing file is normal (first run or after clean shutdown) — it returns a
// zero-value GatewayState and a nil error, following the catalog.LoadState
// pattern.
func LoadGatewayState(path string) (GatewayState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GatewayState{}, nil
		}
		return GatewayState{}, fmt.Errorf("read state %q: %w", path, err)
	}

	var top topLevelState
	if err := json.Unmarshal(data, &top); err != nil {
		return GatewayState{}, fmt.Errorf("parse state %q: %w", path, err)
	}

	if top.Gateway == nil {
		return GatewayState{}, nil
	}
	return *top.Gateway, nil
}

// SaveGatewayState atomically writes gw into the "gateway" key in state.json
// at path, preserving any existing "sources" content.
//
// File permissions are 0o600 (owner-read-only) to protect the gateway token
// from other local processes (T-04-03-01, T-04-03-06 mitigations). The parent
// directory is created with 0o700 if it does not exist.
//
// Writes are performed via writeStateFileAtomic (temp-file + rename) so a crash
// during the write never leaves a partially-written state.json.
func SaveGatewayState(path string, gw GatewayState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state directory %q: %w", dir, err)
	}

	// Load existing state to preserve the "sources" key.
	data, _ := os.ReadFile(path)
	var top topLevelState
	_ = json.Unmarshal(data, &top) // tolerate missing/corrupt file

	top.Gateway = &gw

	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gateway state: %w", err)
	}

	return writeStateFileAtomic(path, out)
}

// ClearGatewayState removes the "gateway" key from state.json at path,
// preserving other keys. Called on clean shutdown so stale tokens are not
// left in state.json after the daemon exits.
func ClearGatewayState(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // nothing to clear
		}
		return fmt.Errorf("read state %q: %w", path, err)
	}

	var top topLevelState
	if err := json.Unmarshal(data, &top); err != nil {
		// If we can't parse it, leave it alone — don't corrupt state.
		return nil
	}

	top.Gateway = nil

	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	return writeStateFileAtomic(path, out)
}

// writeStateFileAtomic writes data to a temp file in the same directory then
// renames it over path with 0o600 permissions. The 0o600 permission is set on
// the temp file before rename so the target is never world-readable at any
// point during the write (T-04-03-01).
func writeStateFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	// Set 0o600 before writing so the file is never world-readable.
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod state temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write state temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync state temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close state temp file: %w", err)
	}

	return os.Rename(tmpName, path)
}
