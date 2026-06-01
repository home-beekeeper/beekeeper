package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestLayeredConfigProjectOverridesUser verifies CODE-05 SC2 closure:
// a project-level .beekeeper/config.json overrides user config for enforcement
// decisions WITHOUT any BEEKEEPER_* env var set.
//
// This is the core acceptance criterion for Task 11-01-02: enforcement commands
// must now use resolveConfig (layered) not config.Load (single-file).
func TestLayeredConfigProjectOverridesUser(t *testing.T) {
	dir := t.TempDir()
	userConfigPath := filepath.Join(dir, "user-config.json")
	projectConfigPath := filepath.Join(dir, "project-config.json")

	// User config: fail_mode = "open"
	userCfg := map[string]any{"fail_mode": "open"}
	writeJSON(t, userConfigPath, userCfg)

	// Project config: fail_mode = "closed" (overrides user)
	projCfg := map[string]any{"fail_mode": "closed"}
	writeJSON(t, projectConfigPath, projCfg)

	// Use resolveConfigWithPaths so we don't touch real platform paths or os.Environ().
	cfg, err := resolveConfigWithPaths(userConfigPath, projectConfigPath, nil)
	if err != nil {
		t.Fatalf("resolveConfigWithPaths error: %v", err)
	}

	// The project layer must win: fail_mode should be "closed".
	if cfg.FailMode != "closed" {
		t.Errorf("fail_mode = %q, want closed (project must override user)", cfg.FailMode)
	}
}

// TestLayeredConfigUserAppliedWhenNoProject verifies that when no project config
// exists, the user config's values are applied correctly.
func TestLayeredConfigUserAppliedWhenNoProject(t *testing.T) {
	dir := t.TempDir()
	userConfigPath := filepath.Join(dir, "user-config.json")

	// User config: fail_mode = "warn"
	userCfg := map[string]any{"fail_mode": "warn"}
	writeJSON(t, userConfigPath, userCfg)

	// No project config path (empty string → layer skipped).
	cfg, err := resolveConfigWithPaths(userConfigPath, "", nil)
	if err != nil {
		t.Fatalf("resolveConfigWithPaths error: %v", err)
	}

	if cfg.FailMode != "warn" {
		t.Errorf("fail_mode = %q, want warn (user config must apply when no project config)", cfg.FailMode)
	}
}

// TestLayeredConfigCorruptUserFails verifies fail-closed behavior: a corrupt
// user config file must return an error so enforcement never proceeds with
// partial/unknown settings (CODE-05 SC2 fail-closed requirement).
func TestLayeredConfigCorruptUserFails(t *testing.T) {
	dir := t.TempDir()
	userConfigPath := filepath.Join(dir, "bad-config.json")

	// Write corrupt JSON.
	if err := os.WriteFile(userConfigPath, []byte("not valid json{{{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := resolveConfigWithPaths(userConfigPath, "", nil)
	if err == nil {
		t.Error("expected error for corrupt user config, got nil (fail-closed violation)")
	}
}

// TestLayeredConfigEnvOverridesProject verifies that BEEKEEPER_* env vars
// override the project config (env layer has higher precedence than project).
func TestLayeredConfigEnvOverridesProject(t *testing.T) {
	dir := t.TempDir()
	userConfigPath := filepath.Join(dir, "user-config.json")
	projectConfigPath := filepath.Join(dir, "project-config.json")

	// User: fail_mode = open
	writeJSON(t, userConfigPath, map[string]any{"fail_mode": "open"})
	// Project: fail_mode = warn
	writeJSON(t, projectConfigPath, map[string]any{"fail_mode": "warn"})

	// Env: BEEKEEPER_FAIL_MODE = closed (highest precedence below flags).
	environ := []string{"BEEKEEPER_FAIL_MODE=closed"}

	cfg, err := resolveConfigWithPaths(userConfigPath, projectConfigPath, environ)
	if err != nil {
		t.Fatalf("resolveConfigWithPaths error: %v", err)
	}

	if cfg.FailMode != "closed" {
		t.Errorf("fail_mode = %q, want closed (env must override project)", cfg.FailMode)
	}
}

// TestDiscoverProjectConfig verifies that discoverProjectConfig finds the
// .beekeeper/config.json in a temp directory (simulating a project root) when
// given a userPath from a different location.
func TestDiscoverProjectConfig(t *testing.T) {
	dir := t.TempDir()
	beekeeperDir := filepath.Join(dir, ".beekeeper")
	if err := os.MkdirAll(beekeeperDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	projectCfg := filepath.Join(beekeeperDir, "config.json")
	if err := os.WriteFile(projectCfg, []byte(`{"fail_mode":"closed"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Simulate cwd = dir.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	// userPath is somewhere else so discoverProjectConfig does not skip it.
	result := discoverProjectConfig("/some/other/path/.beekeeper/config.json")
	if result != projectCfg {
		t.Errorf("discoverProjectConfig = %q, want %q", result, projectCfg)
	}
}

// writeJSON is a test helper that marshals v into JSON and writes it to path.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile %q: %v", path, err)
	}
}
