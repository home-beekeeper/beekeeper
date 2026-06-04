package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileDefaultsClosed(t *testing.T) {
	// A path that does not exist must yield the secure default, not an error.
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file returned error: %v", err)
	}
	if cfg.FailMode != FailModeClosed {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeClosed)
	}
	if !cfg.FailClosed() {
		t.Fatal("FailClosed() = false, want true for default config")
	}
}

func TestLoadOpenMode(t *testing.T) {
	path := writeConfig(t, `{"fail_mode":"open"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FailMode != FailModeOpen {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeOpen)
	}
	if cfg.FailClosed() {
		t.Fatal("FailClosed() = true, want false for fail_mode=open")
	}
}

func TestLoadWarnMode(t *testing.T) {
	path := writeConfig(t, `{"fail_mode":"warn"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FailMode != FailModeWarn {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeWarn)
	}
	if cfg.FailClosed() {
		t.Fatal("FailClosed() = true, want false for fail_mode=warn")
	}
}

func TestLoadInvalidModeErrors(t *testing.T) {
	path := writeConfig(t, `{"fail_mode":"yolo"}`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load with invalid fail_mode returned nil error, want non-nil")
	}
}

func TestEmptyModeDefaultsClosed(t *testing.T) {
	// An empty/omitted fail_mode must default to the secure mode.
	path := writeConfig(t, `{}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FailMode != FailModeClosed {
		t.Fatalf("FailMode = %q, want %q", cfg.FailMode, FailModeClosed)
	}
	if !cfg.FailClosed() {
		t.Fatal("FailClosed() = false, want true for empty fail_mode")
	}
}

func TestLoadMalformedJSONErrors(t *testing.T) {
	path := writeConfig(t, `{not json}`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load with malformed JSON returned nil error, want non-nil")
	}
}

func TestSocketTokenLoads(t *testing.T) {
	// A config with socket.api_token set must load the token and still default
	// fail_mode to "closed".
	path := writeConfig(t, `{"socket":{"api_token":"tok_abc"}}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.SocketAPIToken(); got != "tok_abc" {
		t.Fatalf("SocketAPIToken() = %q, want %q", got, "tok_abc")
	}
	if cfg.FailMode != FailModeClosed {
		t.Fatalf("FailMode = %q, want %q (fail_mode must default to closed when omitted)", cfg.FailMode, FailModeClosed)
	}
	if !cfg.FailClosed() {
		t.Fatal("FailClosed() = false, want true when fail_mode is absent")
	}
}

func TestSocketTokenAbsentIsEmpty(t *testing.T) {
	// A config that only sets fail_mode and omits socket block must return ""
	// from SocketAPIToken() without error.
	path := writeConfig(t, `{"fail_mode":"closed"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.SocketAPIToken(); got != "" {
		t.Fatalf("SocketAPIToken() = %q, want \"\" when socket block absent", got)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Phase 8 (NUDGE-08): NudgeConfig load + ValidateNudgeConfig tests
// ---------------------------------------------------------------------------

// TestNudgeMissingBlockResolvesToDefaults verifies that a config file with no
// "nudge" key resolves to DefaultNudgeConfig() with all expected defaults.
func TestNudgeMissingBlockResolvesToDefaults(t *testing.T) {
	path := writeConfig(t, `{}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("Nudge is nil; want DefaultNudgeConfig()")
	}
	def := DefaultNudgeConfig()
	if !cfg.Nudge.Enabled {
		t.Errorf("Nudge.Enabled = false, want true (default)")
	}
	if cfg.Nudge.Mode != def.Mode {
		t.Errorf("Nudge.Mode = %q, want %q", cfg.Nudge.Mode, def.Mode)
	}
	if cfg.Nudge.Preferred != def.Preferred {
		t.Errorf("Nudge.Preferred = %q, want %q", cfg.Nudge.Preferred, def.Preferred)
	}
	if cfg.Nudge.VersionFloors.Pnpm != def.VersionFloors.Pnpm {
		t.Errorf("Nudge.VersionFloors.Pnpm = %q, want %q", cfg.Nudge.VersionFloors.Pnpm, def.VersionFloors.Pnpm)
	}
	if cfg.Nudge.VersionFloors.Bun != def.VersionFloors.Bun {
		t.Errorf("Nudge.VersionFloors.Bun = %q, want %q", cfg.Nudge.VersionFloors.Bun, def.VersionFloors.Bun)
	}
	if cfg.Nudge.VersionFloors.Node != def.VersionFloors.Node {
		t.Errorf("Nudge.VersionFloors.Node = %q, want %q", cfg.Nudge.VersionFloors.Node, def.VersionFloors.Node)
	}
	if cfg.Nudge.MajorDriftCheck.Interval != def.MajorDriftCheck.Interval {
		t.Errorf("Nudge.MajorDriftCheck.Interval = %q, want %q",
			cfg.Nudge.MajorDriftCheck.Interval, def.MajorDriftCheck.Interval)
	}
}

// TestNudgeExplicitEnabledFalsePreserved verifies that an explicit
// nudge.enabled:false in the config is preserved (not clobbered by the
// default true). This is the project .beekeeper.json layered-config win case
// (NUDGE-08, PRD §11).
func TestNudgeExplicitEnabledFalsePreserved(t *testing.T) {
	path := writeConfig(t, `{"nudge":{"enabled":false}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("Nudge is nil after loading explicit block")
	}
	if cfg.Nudge.Enabled {
		t.Error("Nudge.Enabled = true, want false (explicit project-level opt-out)")
	}
}

// TestNudgeInvalidModeErrors verifies that an unknown mode (e.g. "aggressive")
// is rejected with a non-nil error at load time (fail-closed).
func TestNudgeInvalidModeErrors(t *testing.T) {
	path := writeConfig(t, `{"nudge":{"enabled":true,"mode":"aggressive"}}`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load with invalid nudge mode returned nil error, want non-nil")
	}
}

// TestNudgeUnknownPreferredErrors verifies that an unknown preferred PM is
// rejected (fail-closed).
func TestNudgeUnknownPreferredErrors(t *testing.T) {
	path := writeConfig(t, `{"nudge":{"enabled":true,"preferred":"yarn"}}`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load with unknown nudge.preferred returned nil error, want non-nil")
	}
}

// TestNudgeMalformedVersionFloorErrors verifies that a malformed version floor
// is rejected (fail-closed).
func TestNudgeMalformedVersionFloorErrors(t *testing.T) {
	path := writeConfig(t, `{"nudge":{"enabled":true,"version_floors":{"pnpm":"not.a.version"}}}`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load with malformed version floor returned nil error, want non-nil")
	}
}

// TestNudgeValidExplicitBlockLoads verifies that a fully explicit valid nudge
// block loads without error.
func TestNudgeValidExplicitBlockLoads(t *testing.T) {
	path := writeConfig(t, `{
		"nudge": {
			"enabled": true,
			"mode": "hard",
			"preferred": "bun",
			"version_floors": {
				"pnpm": "11.0.0",
				"bun":  "1.3.0",
				"node": "22.0.0"
			},
			"major_drift_check": {
				"enabled": true,
				"interval": "168h"
			}
		}
	}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load with valid explicit nudge block returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("Nudge is nil after loading explicit block")
	}
	if cfg.Nudge.Mode != "hard" {
		t.Errorf("Nudge.Mode = %q, want hard", cfg.Nudge.Mode)
	}
	if cfg.Nudge.Preferred != "bun" {
		t.Errorf("Nudge.Preferred = %q, want bun", cfg.Nudge.Preferred)
	}
}

// TestValidateNudgeConfigExported is the direct unit test for the exported
// ValidateNudgeConfig function — the entry point cmd/beekeeper (Plan 08)
// consumes for the §10-17 config-set rejection test.
func TestValidateNudgeConfigExported(t *testing.T) {
	// Default config must be valid.
	if err := ValidateNudgeConfig(DefaultNudgeConfig()); err != nil {
		t.Errorf("ValidateNudgeConfig(DefaultNudgeConfig()) = %v, want nil", err)
	}

	// "aggressive" mode must be rejected.
	if err := ValidateNudgeConfig(NudgeConfig{Mode: "aggressive"}); err == nil {
		t.Error("ValidateNudgeConfig(mode:aggressive) returned nil, want non-nil error")
	}

	// "yarn" preferred must be rejected.
	if err := ValidateNudgeConfig(NudgeConfig{Preferred: "yarn"}); err == nil {
		t.Error("ValidateNudgeConfig(preferred:yarn) returned nil, want non-nil error")
	}

	// Malformed pnpm floor must be rejected.
	if err := ValidateNudgeConfig(NudgeConfig{
		VersionFloors: NudgeVersionFloors{Pnpm: "not-a-version"},
	}); err == nil {
		t.Error("ValidateNudgeConfig(pnpm floor:not-a-version) returned nil, want non-nil error")
	}

	// Malformed drift interval must be rejected.
	if err := ValidateNudgeConfig(NudgeConfig{
		MajorDriftCheck: NudgeMajorDriftCheck{Interval: "not-a-duration"},
	}); err == nil {
		t.Error("ValidateNudgeConfig(interval:not-a-duration) returned nil, want non-nil error")
	}

	// Valid "hard" mode must be accepted.
	if err := ValidateNudgeConfig(NudgeConfig{Mode: "hard", Preferred: "pnpm"}); err != nil {
		t.Errorf("ValidateNudgeConfig(mode:hard, preferred:pnpm) = %v, want nil", err)
	}

	// Valid "bun" preferred must be accepted.
	if err := ValidateNudgeConfig(NudgeConfig{Preferred: "bun"}); err != nil {
		t.Errorf("ValidateNudgeConfig(preferred:bun) = %v, want nil", err)
	}

	// Well-formed version floor must be accepted.
	if err := ValidateNudgeConfig(NudgeConfig{
		VersionFloors: NudgeVersionFloors{Pnpm: "11.0.0", Bun: "1.3.0", Node: "22.0.0"},
	}); err != nil {
		t.Errorf("ValidateNudgeConfig(valid floors) = %v, want nil", err)
	}
}

// TestNudgeMalformedDriftIntervalErrors verifies that a malformed
// major_drift_check.interval is rejected at load time (fail-closed).
func TestNudgeMalformedDriftIntervalErrors(t *testing.T) {
	path := writeConfig(t, `{"nudge":{"enabled":true,"major_drift_check":{"enabled":true,"interval":"not-a-duration"}}}`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load with malformed drift interval returned nil error, want non-nil")
	}
}
