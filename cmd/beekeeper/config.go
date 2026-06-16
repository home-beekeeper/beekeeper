// config.go — beekeeper config subcommands (§10-17, Phase 8 Plan 08).
//
// Provides a `config set <key> <value>` subcommand scoped to nudge.* keys.
// Every config change is logged to the audit trail (PRD §5.2, §10-17).
//
// Security properties:
//  - ValidateNudgeConfig (EXPORTED from internal/config, Plan 05) is called
//    BEFORE any write — an invalid value is rejected fail-closed with no
//    partial write (T-08-26).
//  - A config-change audit record is written on every successful change so
//    operators can reconstruct when nudge settings were modified.
//
// Supported keys (nudge.*):
//  nudge.enabled          bool   (true|false)
//  nudge.mode             string (soft|hard)
//  nudge.require_hardened bool   (true|false)
//  nudge.preferred        string (pnpm|bun)
//  nudge.check_socket_scanner bool (true|false)
//
// All business logic delegates to internal/config and internal/audit.
// This file is thin Cobra wiring per the project architecture constraint.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// newConfigCmd groups the config management subcommands.
// Pattern: newPolicyCmd() grouped idiom.
func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Beekeeper configuration",
		Long: `Manage the Beekeeper user configuration (~/.beekeeper/config.json).

Changes to nudge.* settings are validated fail-closed (invalid values are
rejected with no write) and logged to the audit trail (PRD §5.2, §10-17).`,
	}
	configCmd.AddCommand(newConfigSetCmd())
	return configCmd
}

// newConfigSetCmd implements `beekeeper config set <key> <value>`.
// It loads the user config, applies the nudge.* key change, validates via the
// EXPORTED config.ValidateNudgeConfig (Plan 05), saves on success, and emits
// a config-change audit record (§10-17).
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value (nudge.* keys)",
		Long: `Set a Beekeeper configuration value by key.

Supported keys (nudge.*):
  nudge.enabled                 — bool (true|false)
  nudge.mode                    — string (soft|hard)
  nudge.require_hardened        — bool (true|false)
  nudge.preferred               — string (pnpm|bun)
  nudge.check_socket_scanner    — bool (true|false)

Config changes are validated fail-closed: an invalid value (e.g.
nudge.mode=aggressive) is rejected with a non-nil error and no write.
Every successful change is logged to the audit trail (PRD §5.2, §10-17).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			val := strings.TrimSpace(args[1])

			// (1) Resolve the user config path.
			userPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("config set: resolve config path: %w", err)
			}

			// (2) Load the current user config.
			cfg, err := config.Load(userPath)
			if err != nil {
				// A missing file is normal — Load returns defaults on ErrNotExist, so
				// a non-nil error here means the file exists but is corrupt/invalid.
				return fmt.Errorf("config set: load config: %w", err)
			}

			// Ensure Nudge block is populated (Load always fills it, but be safe).
			if cfg.Nudge == nil {
				d := config.DefaultNudgeConfig()
				cfg.Nudge = &d
			}

			// (3) Record the old value for the audit record before modification.
			oldValue := nudgeKeyCurrentValue(*cfg.Nudge, key)

			// (4) Apply the nudge.* key change into a candidate NudgeConfig nc.
			nc := *cfg.Nudge // copy
			if err := applyNudgeKey(&nc, key, val); err != nil {
				return fmt.Errorf("config set: %w", err)
			}

			// (5) Validate the resulting config via the EXPORTED validator BEFORE write
			//     (fail-closed §10-17 / T-08-26).
			if err := config.ValidateNudgeConfig(nc); err != nil {
				return fmt.Errorf("config set: validation failed (no write): %w", err)
			}

			// (6) Save the updated config.
			cfg.Nudge = &nc
			if err := config.Save(userPath, cfg); err != nil {
				return fmt.Errorf("config set: save config: %w", err)
			}

			// (7) Emit a config-change audit record (PRD §5.2, §10-17).
			//     This is an explicit operator action so we surface write errors.
			auditDir, aerr := platform.AuditDir()
			if aerr != nil {
				return fmt.Errorf("config set: resolve audit directory: %w", aerr)
			}
			if err := writeConfigChangeRecord(
				filepath.Join(auditDir, "beekeeper.ndjson"),
				key, oldValue, val,
			); err != nil {
				return fmt.Errorf("config set: write audit record: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "OK: %s set to %q\n", key, val)
			return nil
		},
	}
}

// applyNudgeKey applies the supported nudge.* key/value to nc.
// Returns a non-nil error for unknown keys — callers should not write nc on error.
func applyNudgeKey(nc *config.NudgeConfig, key, val string) error {
	switch key {
	case "nudge.enabled":
		b, err := parseBool(val)
		if err != nil {
			return fmt.Errorf("nudge.enabled: %w", err)
		}
		nc.Enabled = b
	case "nudge.mode":
		nc.Mode = val
	case "nudge.require_hardened":
		b, err := parseBool(val)
		if err != nil {
			return fmt.Errorf("nudge.require_hardened: %w", err)
		}
		nc.RequireHardened = b
	case "nudge.preferred":
		nc.Preferred = val
	case "nudge.check_socket_scanner":
		b, err := parseBool(val)
		if err != nil {
			return fmt.Errorf("nudge.check_socket_scanner: %w", err)
		}
		nc.CheckSocketScanner = b
	default:
		return fmt.Errorf("unknown key %q (supported: nudge.enabled, nudge.mode, nudge.require_hardened, nudge.preferred, nudge.check_socket_scanner)", key)
	}
	return nil
}

// nudgeKeyCurrentValue returns the current string representation of the given
// nudge key in nc, for audit record old-value capture.
func nudgeKeyCurrentValue(nc config.NudgeConfig, key string) string {
	switch key {
	case "nudge.enabled":
		return fmt.Sprintf("%v", nc.Enabled)
	case "nudge.mode":
		return nc.Mode
	case "nudge.require_hardened":
		return fmt.Sprintf("%v", nc.RequireHardened)
	case "nudge.preferred":
		return nc.Preferred
	case "nudge.check_socket_scanner":
		return fmt.Sprintf("%v", nc.CheckSocketScanner)
	default:
		return ""
	}
}

// parseBool parses "true"/"false"/"1"/"0"/"yes"/"no" case-insensitively.
func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("cannot parse %q as bool (want true|false|1|0|yes|no)", s)
	}
}

// writeConfigChangeRecord constructs and writes a config-change audit record.
// Mirrors writeLLMFAlertRecord in internal/check/handler.go — explicit operator
// action so the write error is surfaced to the caller (PRD §5.2).
func writeConfigChangeRecord(auditPath, key, oldValue, newValue string) error {
	w, err := audit.NewWriter(auditPath)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer w.Close()

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// Fallback: use a timestamp-derived ID if crypto/rand fails.
		copy(raw[:], []byte(fmt.Sprintf("%016x", time.Now().UnixNano())))
	}
	recordID := hex.EncodeToString(raw[:])

	rec := audit.AuditRecord{
		RecordType:      "config_change",
		RecordID:        recordID,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ScannerName:     "beekeeper",
		OriginalCommand: fmt.Sprintf("config set %s %s", key, newValue),
		// Reason encodes the old→new transition for forensic audit (§10-17).
		Reason:   fmt.Sprintf("%s changed from %q to %q", key, oldValue, newValue),
		ReasonCode: key,
	}

	return w.Write(rec)
}

// ensureNudgeDir is a helper for tests that need to stage a real home directory.
// Not called from production code.
func ensureNudgeDir(dir string) error {
	return os.MkdirAll(dir, 0700)
}
