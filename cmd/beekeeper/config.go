// config.go — beekeeper config subcommands.
//
// Provides a `config set <key> <value>` subcommand. Every successful config
// change is logged to the audit trail (PRD §5.2).
//
// History: this command previously set the package-manager nudge.* keys. The
// nudge feature was removed in v1.1.0, so there are currently no settable keys —
// `config set` rejects every key fail-closed. The command and its audit-logging
// plumbing are retained so a future setting can be wired in without re-adding the
// surface. The audit record_type stays "config_change".
//
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
	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// newConfigCmd groups the config management subcommands.
func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Beekeeper configuration",
		Long: `Manage the Beekeeper user configuration (~/.beekeeper/config.json).

Config changes are validated fail-closed (invalid values are rejected with no
write) and logged to the audit trail (PRD §5.2).`,
	}
	configCmd.AddCommand(newConfigSetCmd())
	return configCmd
}

// errNoSettableKeys is returned for every key because there are currently no
// settable config keys via `config set` (the nudge.* keys were removed in
// v1.1.0). It is fail-closed: no write happens on an unknown key.
func unknownConfigKeyError(key string) error {
	return fmt.Errorf("unknown key %q: there are no settable config keys in this version", key)
}

// newConfigSetCmd implements `beekeeper config set <key> <value>`.
//
// There are currently no settable keys, so it rejects every key fail-closed
// (no write). The audit/record plumbing is retained for a future setting.
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a Beekeeper configuration value by key.

There are currently no settable keys via this command — an unknown key is
rejected fail-closed with a non-nil error and no write. (The package-manager
nudge.* keys were removed in v1.1.0.)`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			// No settable keys exist — reject fail-closed, no write.
			return fmt.Errorf("config set: %w", unknownConfigKeyError(key))
		},
	}
}

// parseBool parses "true"/"false"/"1"/"0"/"yes"/"no" case-insensitively.
// Retained for a future settable bool key.
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
// Retained for a future settable key: an explicit operator action surfaces the
// write error to the caller (PRD §5.2).
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
		// Reason encodes the old→new transition for forensic audit.
		Reason:     fmt.Sprintf("%s changed from %q to %q", key, oldValue, newValue),
		ReasonCode: key,
	}

	return w.Write(rec)
}

// configAuditPath resolves the audit log path for a config change.
// Retained for a future settable key.
func configAuditPath() (string, error) {
	auditDir, err := platform.AuditDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(auditDir, "beekeeper.ndjson"), nil
}

// ensureConfigDir is a helper for tests that need to stage a real home directory.
// Not called from production code.
func ensureConfigDir(dir string) error {
	return os.MkdirAll(dir, 0700)
}
