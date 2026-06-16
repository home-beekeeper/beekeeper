package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/config"
)

// TestConfigSetCmd_HardMode drives `config set nudge.mode hard` against a temp
// BEEKEEPER_HOME and asserts:
//  (a) the saved config has Nudge.Mode == "hard"
//  (b) an audit record naming nudge.mode is written (§10-17)
func TestConfigSetCmd_HardMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	// Pre-create the beekeeper state directory so ConfigPath resolves correctly.
	stateDir := filepath.Join(dir, "beekeeper")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	cmd := newConfigSetCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, []string{"nudge.mode", "hard"})
	if err != nil {
		t.Fatalf("RunE returned unexpected error: %v (stderr: %q)", err, errOut.String())
	}

	// (a) Verify saved config has Nudge.Mode == "hard".
	configPath := filepath.Join(stateDir, "config.json")
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config file: %v", readErr)
	}
	var savedCfg config.Config
	if err := json.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if savedCfg.Nudge == nil {
		t.Fatal("saved config has nil Nudge block")
	}
	if savedCfg.Nudge.Mode != "hard" {
		t.Errorf("expected Nudge.Mode=%q, got %q", "hard", savedCfg.Nudge.Mode)
	}

	// (b) Verify a config-change audit record was written (§10-17).
	auditPath := filepath.Join(stateDir, "audit", "beekeeper.ndjson")
	auditData, auditErr := os.ReadFile(auditPath)
	if auditErr != nil {
		t.Fatalf("read audit log: %v", auditErr)
	}

	// Scan NDJSON lines for a config_change record referencing nudge.mode.
	found := false
	for _, line := range strings.Split(string(auditData), "\n") {
		if line == "" {
			continue
		}
		var rec audit.AuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.RecordType == "config_change" && strings.Contains(rec.Reason, "nudge.mode") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected config_change audit record for nudge.mode, not found in audit log:\n%s", string(auditData))
	}
}

// TestConfigSetCmd_InvalidValue verifies that an invalid value (e.g.
// nudge.mode=aggressive) is rejected by config.ValidateNudgeConfig and
// does NOT write the config (§10-17 fail-closed).
func TestConfigSetCmd_InvalidValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	stateDir := filepath.Join(dir, "beekeeper")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	cmd := newConfigSetCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, []string{"nudge.mode", "aggressive"})
	if err == nil {
		t.Error("RunE: expected non-nil error for invalid nudge.mode=aggressive, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected 'validation failed' in error, got: %v", err)
	}

	// The config file must NOT have been written.
	configPath := filepath.Join(stateDir, "config.json")
	if _, statErr := os.Stat(configPath); statErr == nil {
		t.Error("config file was written despite validation failure — fail-closed violated")
	}
}

// TestConfigSetCmd_UnknownKey verifies that an unknown key returns an error.
func TestConfigSetCmd_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	stateDir := filepath.Join(dir, "beekeeper")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	cmd := newConfigSetCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, []string{"nudge.unknown_field", "value"})
	if err == nil {
		t.Error("RunE: expected non-nil error for unknown key, got nil")
	}
}

// TestConfigSetCmd_EnabledFalse verifies that nudge.enabled=false persists and
// is not silently reset to true.
func TestConfigSetCmd_EnabledFalse(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	stateDir := filepath.Join(dir, "beekeeper")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}

	cmd := newConfigSetCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, []string{"nudge.enabled", "false"})
	if err != nil {
		t.Fatalf("RunE returned unexpected error: %v", err)
	}

	configPath := filepath.Join(stateDir, "config.json")
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config file: %v", readErr)
	}
	var savedCfg config.Config
	if err := json.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if savedCfg.Nudge == nil {
		t.Fatal("saved config has nil Nudge block")
	}
	if savedCfg.Nudge.Enabled != false {
		t.Errorf("expected Nudge.Enabled=false, got %v", savedCfg.Nudge.Enabled)
	}
}
