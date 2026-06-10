package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
//	user overrides system; project tightening overrides user.
//
// Note: "user overrides system" with a relaxation (closed→open) is a TRUSTED
// layer transition and remains fully allowed (TM-D-01 only gates project/env).
// "project overrides user" with a tightening (open→closed) is also always
// allowed regardless of trust level.
func TestLoadLayered_PrecedenceOrder(t *testing.T) {
	tests := []struct {
		name         string
		system       string // JSON content or "" to skip
		user         string // JSON content (always written)
		project      string // JSON content or "" to skip
		wantMode     string
		wantAPIToken string
	}{
		{
			name:     "baseline: user absent layers produce default",
			user:     `{}`,
			wantMode: FailModeClosed,
		},
		{
			name:     "user overrides system (trusted layer relaxation allowed)",
			system:   `{"fail_mode":"closed"}`,
			user:     `{"fail_mode":"open"}`,
			wantMode: FailModeOpen,
		},
		{
			name:     "project tightens user (open→closed always allowed)",
			user:     `{"fail_mode":"open"}`,
			project:  `{"fail_mode":"closed"}`,
			wantMode: FailModeClosed,
		},
		{
			name:         "project overrides user - api token preserved",
			user:         `{"fail_mode":"open","socket":{"api_token":"tok_user"}}`,
			project:      `{"fail_mode":"closed"}`,
			wantMode:     FailModeClosed,
			wantAPIToken: "tok_user", // project did not set socket; user value survives
		},
		{
			name:         "project api token overrides user api token",
			user:         `{"socket":{"api_token":"tok_user"}}`,
			project:      `{"socket":{"api_token":"tok_project"}}`,
			wantMode:     FailModeClosed,
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

// ---- CLEAN-02: Nudge pointer merge in LoadLayered.merge() ----

// TestMerge_NudgeDefaultedAtLayeredRoot verifies that LoadLayered guarantees a
// non-nil, default-populated cfg.Nudge even when NO layer sets a nudge block.
// The user layer (Load) supplies defaults, but this asserts the merge carries
// the Nudge pointer through so it reaches the returned Config (CLEAN-02).
func TestMerge_NudgeDefaultedAtLayeredRoot(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil default-populated block (CLEAN-02)")
	}
	want := DefaultNudgeConfig()
	if *cfg.Nudge != want {
		t.Errorf("cfg.Nudge = %+v, want DefaultNudgeConfig() %+v", *cfg.Nudge, want)
	}
}

// TestMerge_NudgeProjectDisableRefused verifies TM-D-02: a project layer that
// sets ONLY nudge.enabled:false cannot disable a user-enabled nudge. The
// disable is refused from the low-trust project layer; Enabled stays true and
// all other nudge fields inherit defaults.
//
// NOTE: This test was previously named TestMerge_NudgeProjectDisableWins and
// asserted the opposite behavior (project disable wins per NUDGE-08/§11). The
// security hardening fix (TM-D-02) overrides that: project/env layers are
// low-trust and cannot relax security-enforcement switches including nudge.
func TestMerge_NudgeProjectDisableRefused(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"nudge":{"enabled":false}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil block")
	}
	// TM-D-02: project nudge.enabled:false must be REFUSED (not applied).
	if !cfg.Nudge.Enabled {
		t.Error("cfg.Nudge.Enabled = false, want true — project nudge disable is refused from low-trust layer (TM-D-02)")
	}
	// Other fields must still inherit defaults (partial project layer must not zero them — Pitfall 5).
	def := DefaultNudgeConfig()
	if cfg.Nudge.Mode != def.Mode {
		t.Errorf("cfg.Nudge.Mode = %q, want %q (inherited default, not zeroed)", cfg.Nudge.Mode, def.Mode)
	}
	if cfg.Nudge.Preferred != def.Preferred {
		t.Errorf("cfg.Nudge.Preferred = %q, want %q (inherited default)", cfg.Nudge.Preferred, def.Preferred)
	}
	if cfg.Nudge.VersionFloors != def.VersionFloors {
		t.Errorf("cfg.Nudge.VersionFloors = %+v, want %+v (floors must not be zeroed)", cfg.Nudge.VersionFloors, def.VersionFloors)
	}
}

// TestMerge_NudgeProjectModeOverride verifies a project nudge.mode override wins
// while Enabled stays true (inherited from the lower layer's default).
func TestMerge_NudgeProjectModeOverride(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"nudge":{"mode":"hard"}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil block")
	}
	if cfg.Nudge.Mode != "hard" {
		t.Errorf("cfg.Nudge.Mode = %q, want %q (project override)", cfg.Nudge.Mode, "hard")
	}
	if !cfg.Nudge.Enabled {
		t.Error("cfg.Nudge.Enabled = false, want true (inherited default — mode-only override must not disable)")
	}
}

// TestMerge_NudgeInvalidRejected verifies LoadLayered fails closed on an invalid
// merged nudge block (project nudge.mode:"aggressive"), mirroring how Load
// validates a non-nil Nudge.
func TestMerge_NudgeInvalidRejected(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"nudge":{"mode":"aggressive"}}`)

	_, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err == nil {
		t.Fatal("LoadLayered returned nil error for invalid merged nudge.mode:\"aggressive\"; want fail-closed rejection")
	}
}

// ---- CLEAN-02: layered-config Nudge assertions against LoadLayered output ----
//
// These three tests assert DIRECTLY on the LoadLayered return value with NO
// call to defaultNudgeConfigHelper or any consumer-side nil fix — proving the
// root-cause merge fix (mergeNudge, above) populates cfg.Nudge so consumer
// nil-guards are defense-in-depth, not load-bearing (CLEAN-02 / T-09-07).

// TestLoadLayeredNudgeDefaulting (Test A): LoadLayered with a user file that has
// NO nudge block and no project layer → cfg.Nudge deep-equals DefaultNudgeConfig().
func TestLoadLayeredNudgeDefaulting(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil defaults from LoadLayered (no consumer helper)")
	}
	if want := DefaultNudgeConfig(); *cfg.Nudge != want {
		t.Errorf("cfg.Nudge = %+v, want DefaultNudgeConfig() %+v", *cfg.Nudge, want)
	}
}

// TestLoadLayeredProjectNudgeDisableRefused (Test B / TM-D-02): a project file
// containing {"nudge":{"enabled":false}} over a defaulting user layer → Enabled
// stays TRUE because the disable is refused from the low-trust project layer.
// The other nudge fields equal the defaults (Mode "soft", Preferred "pnpm",
// floors intact).
//
// Previously this test asserted the project disable wins (NUDGE-08/§11 project-
// disable). The security fix (TM-D-02) supersedes that: project/env layers are
// low-trust and cannot relax security enforcement. To disable nudge, the operator
// must set nudge.enabled:false in the user config (~/.beekeeper/config.json).
func TestLoadLayeredProjectNudgeDisableRefused(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"nudge":{"enabled":false}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil block")
	}
	def := DefaultNudgeConfig()
	// TM-D-02: project nudge disable is refused; Enabled must stay true.
	if !cfg.Nudge.Enabled {
		t.Error("cfg.Nudge.Enabled = false, want true (project nudge disable refused from low-trust layer, TM-D-02)")
	}
	if cfg.Nudge.Mode != def.Mode {
		t.Errorf("cfg.Nudge.Mode = %q, want %q (default intact)", cfg.Nudge.Mode, def.Mode)
	}
	if cfg.Nudge.Preferred != def.Preferred {
		t.Errorf("cfg.Nudge.Preferred = %q, want %q (default intact)", cfg.Nudge.Preferred, def.Preferred)
	}
	if cfg.Nudge.VersionFloors != def.VersionFloors {
		t.Errorf("cfg.Nudge.VersionFloors = %+v, want %+v (floors intact)", cfg.Nudge.VersionFloors, def.VersionFloors)
	}
}

// TestLoadLayeredProjectNudgeModeOverride (Test C): a project file
// {"nudge":{"mode":"hard"}} → Mode "hard" and Enabled stays true (inherited).
func TestLoadLayeredProjectNudgeModeOverride(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"nudge":{"mode":"hard"}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil block")
	}
	if cfg.Nudge.Mode != "hard" {
		t.Errorf("cfg.Nudge.Mode = %q, want %q (project override)", cfg.Nudge.Mode, "hard")
	}
	if !cfg.Nudge.Enabled {
		t.Error("cfg.Nudge.Enabled = false, want true (inherited; mode-only override must not disable)")
	}
}

// ---- Task 2: TDD tests for applyEnvVars + applyFlagOverrides ----

// TestLoadLayered_EnvVarRelaxationRefused verifies that BEEKEEPER_FAIL_MODE=open
// in opts.Environ is REFUSED when the current merged value is "closed" (TM-D-01).
// The env layer is low-trust; a relaxation (closed→open) is not applied.
//
// Previously this test asserted that the env var override succeeds. The security
// fix (TM-D-01) reverses that for relaxations from the env layer: only tightening
// is allowed. To use fail_mode:open, the operator must set it in their user config
// (~/.beekeeper/config.json) or pass --fail-mode=open as a CLI flag.
func TestLoadLayered_EnvVarRelaxationRefused(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"closed"}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_FAIL_MODE=open"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	// TM-D-01: env relaxation (closed→open) must be refused; result stays closed.
	if cfg.FailMode != FailModeClosed {
		t.Errorf("FailMode = %q, want %q (env fail_mode relaxation must be refused, TM-D-01)", cfg.FailMode, FailModeClosed)
	}
}

// TestLoadLayered_EnvVarTighteningAllowed verifies that BEEKEEPER_FAIL_MODE=closed
// over a user warn is allowed (tightening from env is always safe).
func TestLoadLayered_EnvVarTighteningAllowed(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"warn"}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_FAIL_MODE=closed"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeClosed {
		t.Errorf("FailMode = %q, want %q (env tightening warn→closed must be allowed)", cfg.FailMode, FailModeClosed)
	}
}

// TestLoadLayered_EnvRelaxationOverProjectRefused verifies that BEEKEEPER_FAIL_MODE=warn
// is refused when the merged value after project layer is "closed" (TM-D-01).
// Both project and env are low-trust; neither can relax closed to warn.
func TestLoadLayered_EnvRelaxationOverProjectRefused(t *testing.T) {
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
	// TM-D-01: env relaxation (closed→warn) must be refused; result stays closed.
	if cfg.FailMode != FailModeClosed {
		t.Errorf("FailMode = %q, want %q (env relaxation from low-trust layer refused, TM-D-01)", cfg.FailMode, FailModeClosed)
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

// TestLoadLayered_EnvSelfCatalogURLRefused verifies that BEEKEEPER_SELF_CATALOG_URL
// is ignored from the env layer (TM-D-02). The self-catalog URL is a trust anchor;
// env-var injection must not redirect it to an attacker-controlled feed.
func TestLoadLayered_EnvSelfCatalogURLRefused(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_SELF_CATALOG_URL=https://attacker.example.com/evil.json"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	// TM-D-02: env self_catalog.url must be refused; URL stays empty (compiled-in default).
	if cfg.SelfCatalog.URL != "" {
		t.Errorf("SelfCatalog.URL = %q, want %q (env self_catalog.url refused from low-trust layer, TM-D-02)",
			cfg.SelfCatalog.URL, "")
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

// ---- TM-D-01 / TM-D-02: trust-aware merge acceptance tests ----

// TestTMD01_ProjectFailModeRelaxationRefused verifies that a project layer
// fail_mode:open over a user closed is refused (relaxation from low-trust layer).
func TestTMD01_ProjectFailModeRelaxationRefused(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"closed"}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"fail_mode":"open"}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeClosed {
		t.Errorf("FailMode = %q, want %q — project fail_mode:open must not relax user closed (TM-D-01)",
			cfg.FailMode, FailModeClosed)
	}
}

// TestTMD01_ProjectFailModeTighteningAllowed verifies that a project layer
// fail_mode:closed over a user open is honored (tightening is always safe).
func TestTMD01_ProjectFailModeTighteningAllowed(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"open"}`)
	projectPath := writeLayerConfig(t, dir, "project.json", `{"fail_mode":"closed"}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeClosed {
		t.Errorf("FailMode = %q, want %q — project tightening (open→closed) must be honored (TM-D-01)",
			cfg.FailMode, FailModeClosed)
	}
}

// TestTMD01_UserFailModeRelaxationHonored verifies that a USER layer fail_mode:open
// over a system closed is honored (user is a trusted layer).
func TestTMD01_UserFailModeRelaxationHonored(t *testing.T) {
	dir := t.TempDir()
	systemPath := writeLayerConfig(t, dir, "system.json", `{"fail_mode":"closed"}`)
	userPath := writeLayerConfig(t, dir, "user.json", `{"fail_mode":"open"}`)

	cfg, err := LoadLayered(LayerOpts{SystemPath: systemPath, UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.FailMode != FailModeOpen {
		t.Errorf("FailMode = %q, want %q — user (trusted) fail_mode:open must override system closed (TM-D-01)",
			cfg.FailMode, FailModeOpen)
	}
}

// TestTMD02_ProjectSelfCatalogIgnored verifies that project layer self_catalog.url
// and self_catalog.pub_key are ignored entirely (TM-D-02).
func TestTMD02_ProjectSelfCatalogIgnored(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json",
		`{"self_catalog":{"url":"https://official.example.com/feed.json","pub_key":"trustedkey=="}}`)
	projectPath := writeLayerConfig(t, dir, "project.json",
		`{"self_catalog":{"url":"https://attacker.example.com/evil.json","pub_key":"evilkey=="}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.SelfCatalog.URL != "https://official.example.com/feed.json" {
		t.Errorf("SelfCatalog.URL = %q, want official URL — project self_catalog.url refused (TM-D-02)",
			cfg.SelfCatalog.URL)
	}
	if cfg.SelfCatalog.PubKey != "trustedkey==" {
		t.Errorf("SelfCatalog.PubKey = %q, want trusted key — project self_catalog.pub_key refused (TM-D-02)",
			cfg.SelfCatalog.PubKey)
	}
}

// TestTMD02_UserSelfCatalogHonored verifies that a USER layer self_catalog override
// is honored (user is a trusted layer; operator intentionally reconfigured).
func TestTMD02_UserSelfCatalogHonored(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json",
		`{"self_catalog":{"url":"https://mirror.example.com/feed.json","pub_key":"mirrorkey=="}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.SelfCatalog.URL != "https://mirror.example.com/feed.json" {
		t.Errorf("SelfCatalog.URL = %q, want mirror URL — user (trusted) self_catalog.url must be honored (TM-D-02)",
			cfg.SelfCatalog.URL)
	}
	if cfg.SelfCatalog.PubKey != "mirrorkey==" {
		t.Errorf("SelfCatalog.PubKey = %q, want mirror key — user (trusted) self_catalog.pub_key must be honored (TM-D-02)",
			cfg.SelfCatalog.PubKey)
	}
}

// TestTMD02_ProjectLlamaFirewallDisableRefused verifies that a project layer
// llamafirewall.enabled:false over a user-enabled sidecar is refused (TM-D-02).
func TestTMD02_ProjectLlamaFirewallDisableRefused(t *testing.T) {
	dir := t.TempDir()
	// User enables LlamaFirewall with sample_rate so the "other fields" trigger
	// carries the enable intent unambiguously.
	userPath := writeLayerConfig(t, dir, "user.json",
		`{"llamafirewall":{"enabled":true,"sample_rate":1.0}}`)
	// Project tries to disable the sidecar.
	projectPath := writeLayerConfig(t, dir, "project.json",
		`{"llamafirewall":{"enabled":false}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if !cfg.LlamaFirewall.Enabled {
		t.Error("LlamaFirewall.Enabled = false, want true — project disable refused from low-trust layer (TM-D-02)")
	}
}

// TestTMD02_UserLlamaFirewallDisableHonored verifies that a USER layer
// llamafirewall.enabled:false is honored (trusted operator choice).
func TestTMD02_UserLlamaFirewallDisableHonored(t *testing.T) {
	dir := t.TempDir()
	systemPath := writeLayerConfig(t, dir, "system.json",
		`{"llamafirewall":{"enabled":true,"sample_rate":1.0}}`)
	userPath := writeLayerConfig(t, dir, "user.json",
		`{"llamafirewall":{"enabled":false,"sample_rate":0.5}}`)

	cfg, err := LoadLayered(LayerOpts{SystemPath: systemPath, UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.LlamaFirewall.Enabled {
		t.Error("LlamaFirewall.Enabled = true, want false — user (trusted) disable must be honored (TM-D-02)")
	}
}

// TestTMD02_ProjectNudgeDisableRefused is the canonical TM-D-02 nudge gate test:
// project nudge.enabled:false is refused when user has nudge enabled.
func TestTMD02_ProjectNudgeDisableRefused(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`) // defaults: nudge enabled
	projectPath := writeLayerConfig(t, dir, "project.json", `{"nudge":{"enabled":false}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projectPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil block")
	}
	if !cfg.Nudge.Enabled {
		t.Error("cfg.Nudge.Enabled = false, want true — project nudge disable refused from low-trust layer (TM-D-02)")
	}
}

// TestTMD02_UserNudgeDisableHonored verifies that a USER layer nudge.enabled:false
// is honored (trusted operator choice; operator opted out project-wide via ~/.beekeeper).
func TestTMD02_UserNudgeDisableHonored(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{"nudge":{"enabled":false}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Nudge == nil {
		t.Fatal("cfg.Nudge = nil, want non-nil block")
	}
	if cfg.Nudge.Enabled {
		t.Error("cfg.Nudge.Enabled = true, want false — user (trusted) nudge disable must be honored (TM-D-02)")
	}
}

// --- Phase 20 (CSYNC-04) — catalog_sync self-defense ---

// TestMergeCatalogSyncUntrustedRefusesDisable verifies a project/env layer
// cannot disable background catalog sync (disabling sync reduces security).
func TestMergeCatalogSyncUntrustedRefusesDisable(t *testing.T) {
	dst := &CatalogSyncConfig{Enabled: true, Interval: "12h"}
	src := &CatalogSyncConfig{Enabled: false} // untrusted disable attempt
	out := mergeCatalogSyncUntrusted(dst, src, "project")
	if out == nil {
		t.Fatal("merged CatalogSync = nil, want non-nil")
	}
	if !out.Enabled {
		t.Error("Enabled = false, want true — untrusted layer must not be able to disable sync (CSYNC-04)")
	}
}

// TestMergeCatalogSyncUntrustedAllowsEnable verifies an untrusted enable (a
// tightening) is honored even when the lower layer had sync disabled.
func TestMergeCatalogSyncUntrustedAllowsEnable(t *testing.T) {
	dst := &CatalogSyncConfig{Enabled: false, Interval: "12h"}
	src := &CatalogSyncConfig{Enabled: true}
	out := mergeCatalogSyncUntrusted(dst, src, "project")
	if !out.Enabled {
		t.Error("Enabled = false, want true — an untrusted enable (tightening) is allowed")
	}
}

// TestMergeCatalogSyncUntrustedInterval verifies a loosening (less-frequent)
// interval from an untrusted layer is refused, but a tightening is honored.
func TestMergeCatalogSyncUntrustedInterval(t *testing.T) {
	// Loosening 5h -> 24h is refused (kept at 5h).
	dst := &CatalogSyncConfig{Enabled: true, Interval: "5h"}
	loosen := &CatalogSyncConfig{Enabled: true, Interval: "24h"}
	if out := mergeCatalogSyncUntrusted(dst, loosen, "env"); out.Interval != "5h" {
		t.Errorf("Interval = %q, want 5h — untrusted loosening must be refused (CSYNC-04)", out.Interval)
	}
	// Tightening 24h -> 5h is honored.
	dst2 := &CatalogSyncConfig{Enabled: true, Interval: "24h"}
	tighten := &CatalogSyncConfig{Enabled: true, Interval: "5h"}
	if out := mergeCatalogSyncUntrusted(dst2, tighten, "env"); out.Interval != "5h" {
		t.Errorf("Interval = %q, want 5h — a tightening interval is allowed", out.Interval)
	}
}

// TestLoadLayeredCatalogSyncProjectCannotDisable is the end-to-end proof: a
// project layer with catalog_sync.enabled:false cannot disable a user-enabled
// sync.
func TestLoadLayeredCatalogSyncProjectCannotDisable(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()
	userPath := writeLayerConfig(t, userDir, "config.json", `{"catalog_sync":{"enabled":true,"interval":"6h"}}`)
	projPath := writeLayerConfig(t, projDir, "config.json", `{"catalog_sync":{"enabled":false}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.CatalogSync == nil || !cfg.CatalogSync.Enabled {
		t.Error("CatalogSync.Enabled = false, want true — project layer must not disable sync (CSYNC-04)")
	}
	if got := cfg.CatalogSyncInterval(); got != 6*time.Hour {
		t.Errorf("CatalogSyncInterval() = %s, want 6h (user value preserved)", got)
	}
}
