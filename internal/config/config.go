// Package config provides Beekeeper's user-level configuration loader.
//
// Phase 1 scope is intentionally minimal: a single user-level config file with
// the fail mode that governs how the hook handler behaves when it cannot reach
// a decision (crash, timeout, oversized input, missing catalog index). The full
// layered system→user→project→env→flag merge (CODE-05) lands in Phase 9 and is
// out of scope here.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Fail-mode values. "closed" is the secure default: any failure to reach a
// decision blocks the tool call. "open" and "warn" are explicit, documented
// opt-outs that allow on failure and therefore reduce security.
const (
	FailModeClosed = "closed"
	FailModeOpen   = "open"
	FailModeWarn   = "warn"
)

// Config is the user-level Beekeeper configuration.
type Config struct {
	// FailMode controls behavior when the hook handler cannot produce a real
	// policy decision (crash, timeout, oversized stdin, missing/corrupt index):
	//   "closed" (default) — failures BLOCK (fail-closed; secure default).
	//   "open"             — failures ALLOW. "open" reduces security: failures
	//                        allow instead of block.
	//   "warn"             — failures ALLOW but are surfaced as a warning.
	// Empty is treated as "closed".
	FailMode string `json:"fail_mode"`
}

// Load reads the config at path.
//
// A missing file is normal — absence means "use defaults" — so it returns
// Config{FailMode: "closed"} with a nil error. If the file exists it is read and
// unmarshaled; an empty fail_mode defaults to "closed", and any value other than
// "closed"/"open"/"warn" is rejected with a non-nil error so a typo cannot
// silently degrade to a less-secure mode.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{FailMode: FailModeClosed}, nil
		}
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	if cfg.FailMode == "" {
		cfg.FailMode = FailModeClosed
	}

	switch cfg.FailMode {
	case FailModeClosed, FailModeOpen, FailModeWarn:
		// valid
	default:
		return Config{}, fmt.Errorf("invalid fail_mode %q (want %q, %q, or %q)",
			cfg.FailMode, FailModeClosed, FailModeOpen, FailModeWarn)
	}

	return cfg, nil
}

// FailClosed reports whether failures should block. It returns true unless
// FailMode is explicitly "open" or "warn", so an empty or unrecognized mode is
// treated as fail-closed (the secure default).
func (c Config) FailClosed() bool {
	return c.FailMode != FailModeOpen && c.FailMode != FailModeWarn
}
