package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLayerConfig writes a JSON config file into dir with the given name and
// content, returning the full path.
func writeLayerConfig(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write layer config %q: %v", path, err)
	}
	return path
}

// TestLoadLayered_PrecedenceOrder verifies the five-row layered config test
// matrix from 09-RESEARCH.md "Layered config test matrix":
//
//	user overrides system; project overrides user.
func TestLoadLayered_PrecedenceOrder(t *testing.T) {
	tests := []struct {
		name        string
		system      string // JSON content or "" to skip
		user        string // JSON content (always written)
		project     string // JSON content or "" to skip
		wantMode    string
		wantAPIToken string
	}{
		{
			name:     "baseline: user absent layers produce default",
			user:     `{}`,
			wantMode: FailModeClosed,
		},
		{
			name:     "user overrides system",
			system:   `{"fail_mode":"closed"}`,
			user:     `{"fail_mode":"open"}`,
			wantMode: FailModeOpen,
		},
		{
			name:     "project overrides user",
			user:     `{"fail_mode":"open"}`,
			project:  `{"fail_mode":"closed"}`,
			wantMode: FailModeClosed,
		},
		{
			name:        "project overrides user - api token preserved",
			user:        `{"fail_mode":"open","socket":{"api_token":"tok_user"}}`,
			project:     `{"fail_mode":"closed"}`,
			wantMode:    FailModeClosed,
			wantAPIToken: "tok_user", // project did not set socket; user value survives
		},
		{
			name:        "project api token overrides user api token",
			user:        `{"socket":{"api_token":"tok_user"}}`,
			project:     `{"socket":{"api_token":"tok_project"}}`,
			wantMode:    FailModeClosed,
			wantAPIToken: "tok_project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			opts := LayerOpts{
				UserPath: writeLayerConfig(t, dir, "user.json", tc.user),
			}
			if tc.system != "" {
				opts.SystemPath = writeLayerConfig(t, dir, "system.json", tc.system)
			}
			if tc.project != "" {
				opts.ProjectPath = writeLayerConfig(t, dir, "project.json", tc.project)
			}

			cfg, err := LoadLayered(opts)
			if err != nil {
				t.Fatalf("LoadLayered returned error: %v", err)
			}
			if cfg.FailMode != tc.wantMode {
				t.Errorf("FailMode = %q, want %q", cfg.FailMode, tc.wantMode)
			}
			if tc.wantAPIToken != "" && cfg.Socket.APIToken != tc.wantAPIToken {
				t.Errorf("Socket.APIToken = %q, want %q", cfg.Socket.APIToken, tc.wantAPIToken)
			}
		})
	}
}

// TestLoadLayered_MissingOptionalLayers verifies that absent system and project
// layers are silently skipped (not errors).
func TestLoadLayered_MissingOptionalLayers(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"open"}`)

	opts := LayerOpts{
		SystemPath:  filepath.Join(dir, "nonexistent-system.json"), // absent
		UserPath:    userPath,
		ProjectPath: filepath.Join(dir, "nonexistent-project.json"), // absent
	}

	cfg, err := LoadLayered(opts)
	if err != nil {
		t.Fatalf("LoadLayered returned error when optional layers absent: %v", err)
	}
	if cfg.FailMode != FailModeOpen {
		t.Errorf("FailMode = %q, want %q (user layer should win when optionals absent)", cfg.FailMode, FailModeOpen)
	}
}

// TestMerge_ZeroValuePreservation verifies that a project config that only sets
// one field does NOT reset unrelated user-layer fields to their zero values.
// This is the core guarantee of the "src wins only if non-zero" merge rule.
func TestMerge_ZeroValuePreservation(t *testing.T) {
	dir := t.TempDir()

	// User sets fail_mode=open and a socket token.
	userPath := writeLayerConfig(t, dir, "user.json",
		`{"fail_mode":"open","socket":{"api_token":"tok_abc"}}`)
	// Project sets ONLY one unrelated field (redact_patterns); FailMode is absent/zero.
	projectPath := writeLayerConfig(t, dir, "project.json",
		`{"redact_patterns":["MY_SECRET"]}`)

	opts := LayerOpts{
		UserPath:    userPath,
		ProjectPath: projectPath,
	}

	cfg, err := LoadLayered(opts)
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	// User's fail_mode must survive; project's absent fail_mode must NOT reset it.
	if cfg.FailMode != FailModeOpen {
		t.Errorf("FailMode = %q, want %q — zero-value project field must not reset user value",
			cfg.FailMode, FailModeOpen)
	}
	// User's socket token must also survive.
	if cfg.Socket.APIToken != "tok_abc" {
		t.Errorf("Socket.APIToken = %q, want %q — zero-value project field must not reset user token",
			cfg.Socket.APIToken, "tok_abc")
	}
	// Project's redact_patterns must be applied.
	if len(cfg.RedactPatterns) != 1 || cfg.RedactPatterns[0] != "MY_SECRET" {
		t.Errorf("RedactPatterns = %v, want [MY_SECRET]", cfg.RedactPatterns)
	}
}

// ---- Task 2: TDD tests for applyEnvVars + applyFlagOverrides ----

// TestLoadLayered_EnvVarOverride verifies that BEEKEEPER_FAIL_MODE=open in
// opts.Environ overrides a JSON file fail_mode "closed".
func TestLoadLayered_EnvVarOverride(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"closed"}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_FAIL_MODE=open"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeOpen {
		t.Errorf("FailMode = %q, want %q (BEEKEEPER_FAIL_MODE env must override JSON)", cfg.FailMode, FailModeOpen)
	}
}

// TestLoadLayered_EnvOverridesProject verifies that env vars beat the project layer.
func TestLoadLayered_EnvOverridesProject(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"closed"}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"fail_mode":"closed"}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath:    userPath,
		ProjectPath: projectPath,
		Environ:     []string{"BEEKEEPER_FAIL_MODE=warn"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeWarn {
		t.Errorf("FailMode = %q, want %q (env must beat project layer)", cfg.FailMode, FailModeWarn)
	}
}

// TestLoadLayered_FlagOverridesEnv verifies that CLI flag overrides beat env vars.
func TestLoadLayered_FlagOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath:      userPath,
		Environ:       []string{"BEEKEEPER_FAIL_MODE=warn"},
		FlagOverrides: map[string]string{"fail_mode": "open"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeOpen {
		t.Errorf("FailMode = %q, want %q (CLI flag must beat env var)", cfg.FailMode, FailModeOpen)
	}
}

// TestLoadLayered_UnknownEnvIgnored verifies that an unknown BEEKEEPER_* env var
// (or any non-BEEKEEPER_ var) does not error and does not alter Config.
func TestLoadLayered_UnknownEnvIgnored(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"warn"}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ: []string{
			"BEEKEEPER_NONSENSE=x",     // unknown BEEKEEPER_ key
			"TOTALLY_UNRELATED=abc",    // not even BEEKEEPER_
			"BEEKEEPER_FAIL_MODE=warn", // known — but same value, harmless
		},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeWarn {
		t.Errorf("FailMode = %q, want %q — unknown env must not alter config", cfg.FailMode, FailModeWarn)
	}
}

// TestLoadLayered_EnvAuditSinks verifies BEEKEEPER_AUDIT_SINKS (comma-split).
func TestLoadLayered_EnvAuditSinks(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_AUDIT_SINKS=file,syslog"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if len(cfg.Audit.Sinks) != 2 || cfg.Audit.Sinks[0] != "file" || cfg.Audit.Sinks[1] != "syslog" {
		t.Errorf("Audit.Sinks = %v, want [file syslog]", cfg.Audit.Sinks)
	}
}

// TestLoadLayered_EnvSelfCatalogURL verifies BEEKEEPER_SELF_CATALOG_URL mapping.
func TestLoadLayered_EnvSelfCatalogURL(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_SELF_CATALOG_URL=https://example.com/beekeeper-self.json"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.SelfCatalog.URL != "https://example.com/beekeeper-self.json" {
		t.Errorf("SelfCatalog.URL = %q, want https://example.com/beekeeper-self.json", cfg.SelfCatalog.URL)
	}
}

// TestLoadLayered_EnvSocketAPIToken verifies BEEKEEPER_SOCKET_API_TOKEN mapping.
func TestLoadLayered_EnvSocketAPIToken(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_SOCKET_API_TOKEN=tok_env_xyz"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Socket.APIToken != "tok_env_xyz" {
		t.Errorf("Socket.APIToken = %q, want tok_env_xyz", cfg.Socket.APIToken)
	}
}

// TestLoadLayered_EnvLlamaFirewallEnabled verifies BEEKEEPER_LLAMAFIREWALL_ENABLED.
func TestLoadLayered_EnvLlamaFirewallEnabled(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_LLAMAFIREWALL_ENABLED=true"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if !cfg.LlamaFirewall.Enabled {
		t.Error("LlamaFirewall.Enabled = false, want true from BEEKEEPER_LLAMAFIREWALL_ENABLED=true")
	}
}
