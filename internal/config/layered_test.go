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
//
// Uses two LOCAL sink names: as of finding #12 the env layer strips REMOTE sinks
// (otlp/http/https/syslog), so this test exercises the comma-split mechanics with
// non-remote names. The remote-strip behavior is covered by
// TestLoadLayered_EnvAuditSinksRemoteRefused.
func TestLoadLayered_EnvAuditSinks(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_AUDIT_SINKS=file,stderr"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if len(cfg.Audit.Sinks) != 2 || cfg.Audit.Sinks[0] != "file" || cfg.Audit.Sinks[1] != "stderr" {
		t.Errorf("Audit.Sinks = %v, want [file stderr]", cfg.Audit.Sinks)
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

// ---- Phase 24: mergeCorpus + mergeAutoQuarantine unit tests ----
//
// These tests regression-lock the Phase-24 bug fix: before the fix, merge()
// did not call mergeCorpus / mergeAutoQuarantine, so cfg.Corpus.Enabled and
// cfg.AutoQuarantine were always the zero value regardless of what a higher
// layer set. The tests below directly call the merge helpers so any regression
// in the helper implementations is caught at the unit level (not just by the
// E2E Nx Console gate).

// TestMergeCorpus_ZeroSrcLeavesBase verifies that a zero-value src CorpusConfig
// does not reset a populated dst (Pitfall 5: absent higher-layer field must not
// clobber lower-layer non-zero value).
//
// Note: Scope is carried through as-is (mergeCorpus does not process it, so
// dst.Scope is always preserved verbatim regardless of src.Scope).
func TestMergeCorpus_ZeroSrcLeavesBase(t *testing.T) {
	dst := CorpusConfig{
		Enabled:             true,
		Path:                "/data/corpus.ndjson",
		DownstreamCleanDays: 30,
	}
	src := CorpusConfig{} // all zero

	got := mergeCorpus(dst, src)

	if !got.Enabled {
		t.Error("Enabled = false, want true — zero src must not clobber dst")
	}
	if got.Path != "/data/corpus.ndjson" {
		t.Errorf("Path = %q, want /data/corpus.ndjson", got.Path)
	}
	if got.DownstreamCleanDays != 30 {
		t.Errorf("DownstreamCleanDays = %d, want 30", got.DownstreamCleanDays)
	}
}

// TestMergeCorpus_NonZeroSrcWins verifies that a fully-populated src wins over
// every field of dst that mergeCorpus handles: Enabled, Path,
// DownstreamCleanDays. Note: Scope is intentionally NOT merged by mergeCorpus
// (it is not wired into the merge helper; only Enabled, Path, and
// DownstreamCleanDays are). This test documents the actual behaviour.
func TestMergeCorpus_NonZeroSrcWins(t *testing.T) {
	dst := CorpusConfig{
		Enabled:             false,
		Path:                "/old/corpus.ndjson",
		DownstreamCleanDays: 10,
	}
	src := CorpusConfig{
		Enabled:             true,
		Path:                "/new/corpus.ndjson",
		DownstreamCleanDays: 90,
	}

	got := mergeCorpus(dst, src)

	if !got.Enabled {
		t.Error("Enabled = false, want true — src.Enabled=true must win")
	}
	if got.Path != "/new/corpus.ndjson" {
		t.Errorf("Path = %q, want /new/corpus.ndjson", got.Path)
	}
	if got.DownstreamCleanDays != 90 {
		t.Errorf("DownstreamCleanDays = %d, want 90", got.DownstreamCleanDays)
	}
}

// TestMergeCorpus_PartialSrcMergesFieldByField verifies that a partial src
// (only Path and DownstreamCleanDays set) merges only those fields without
// clobbering dst.Enabled (the zero-value false cannot beat a true).
func TestMergeCorpus_PartialSrcMergesFieldByField(t *testing.T) {
	dst := CorpusConfig{
		Enabled:             true,
		Path:                "/keep/corpus.ndjson",
		DownstreamCleanDays: 30,
	}
	// src sets only Path and DownstreamCleanDays.
	src := CorpusConfig{
		Path:                "/override/corpus.ndjson",
		DownstreamCleanDays: 60,
	}

	got := mergeCorpus(dst, src)

	// Enabled: src.Enabled == false (zero) — must NOT clobber dst.Enabled.
	if !got.Enabled {
		t.Error("Enabled = false, want true — partial src must not clobber dst.Enabled")
	}
	if got.Path != "/override/corpus.ndjson" {
		t.Errorf("Path = %q, want /override/corpus.ndjson", got.Path)
	}
	if got.DownstreamCleanDays != 60 {
		t.Errorf("DownstreamCleanDays = %d, want 60", got.DownstreamCleanDays)
	}
}

// TestMergeCorpus_EnabledFalseCannotDisable verifies the asymmetry in
// mergeCorpus: because Go JSON unmarshalling cannot distinguish absent=false
// from explicit=false for booleans, a src.Enabled==false NEVER clobbers a
// dst.Enabled==true. This prevents a higher trusted layer from accidentally
// disabling corpus monitoring by omitting the field.
func TestMergeCorpus_EnabledFalseCannotDisable(t *testing.T) {
	dst := CorpusConfig{Enabled: true, Path: "/corpus.ndjson"}
	src := CorpusConfig{Enabled: false} // cannot distinguish absent from explicit

	got := mergeCorpus(dst, src)
	if !got.Enabled {
		t.Error("Enabled = false, want true — false cannot clobber true (absent-vs-explicit ambiguity)")
	}
}

// TestMergeCorpus_BothZeroStaysZero verifies the zero-dst + zero-src case:
// the result is also zero (no panic, no unexpected mutation).
func TestMergeCorpus_BothZeroStaysZero(t *testing.T) {
	got := mergeCorpus(CorpusConfig{}, CorpusConfig{})
	if got.Enabled {
		t.Error("Enabled = true, want false — zero merge must stay zero")
	}
	if got.Path != "" || got.Scope != "" || got.DownstreamCleanDays != 0 {
		t.Errorf("unexpected non-zero fields on zero+zero merge: %+v", got)
	}
}

// TestMergeAutoQuarantine_ZeroSrcLeavesBase verifies that a zero-value src
// AutoQuarantineConfig does not reset a populated dst (Pitfall 5).
func TestMergeAutoQuarantine_ZeroSrcLeavesBase(t *testing.T) {
	dst := AutoQuarantineConfig{
		Enabled:   true,
		DryRun:    false, // explicit non-default
		Threshold: 3,
	}
	src := AutoQuarantineConfig{} // all zero

	got := mergeAutoQuarantine(dst, src)

	if !got.Enabled {
		t.Error("Enabled = false, want true — zero src must not clobber dst")
	}
	// DryRun: src.DryRun==false cannot be distinguished from absent — must NOT
	// clobber dst.DryRun==false (it was already false, so the value is preserved).
	if got.DryRun != false {
		t.Errorf("DryRun = %v, want false", got.DryRun)
	}
	if got.Threshold != 3 {
		t.Errorf("Threshold = %d, want 3", got.Threshold)
	}
}

// TestMergeAutoQuarantine_NonZeroSrcWins verifies that a fully-populated src
// wins over every field of dst where src is non-zero.
func TestMergeAutoQuarantine_NonZeroSrcWins(t *testing.T) {
	dst := AutoQuarantineConfig{
		Enabled:   false,
		DryRun:    false,
		Threshold: 1,
	}
	src := AutoQuarantineConfig{
		Enabled:   true,
		DryRun:    true,
		Threshold: 3,
	}

	got := mergeAutoQuarantine(dst, src)

	if !got.Enabled {
		t.Error("Enabled = false, want true — src.Enabled=true must win")
	}
	if !got.DryRun {
		t.Error("DryRun = false, want true — src.DryRun=true must win")
	}
	if got.Threshold != 3 {
		t.Errorf("Threshold = %d, want 3", got.Threshold)
	}
}

// TestMergeAutoQuarantine_PartialSrcMergesFieldByField verifies that a partial
// src (only Threshold set) merges only that field without clobbering the rest.
func TestMergeAutoQuarantine_PartialSrcMergesFieldByField(t *testing.T) {
	dst := AutoQuarantineConfig{
		Enabled:   true,
		DryRun:    true,
		Threshold: 2,
	}
	src := AutoQuarantineConfig{
		Threshold: 3, // only override Threshold
	}

	got := mergeAutoQuarantine(dst, src)

	if !got.Enabled {
		t.Error("Enabled = false, want true — partial src must not clobber dst.Enabled")
	}
	// DryRun: src.DryRun==false (zero) — must NOT clobber dst.DryRun==true.
	// Note: mergeAutoQuarantine uses "if src.DryRun { dst.DryRun = true }" so
	// a false src.DryRun can never override a true dst.DryRun.
	if !got.DryRun {
		t.Error("DryRun = false, want true — zero-value false cannot clobber dst true")
	}
	if got.Threshold != 3 {
		t.Errorf("Threshold = %d, want 3", got.Threshold)
	}
}

// TestMergeAutoQuarantine_EnabledFalseCannotDisable verifies the bool asymmetry:
// src.Enabled==false NEVER clobbers a dst.Enabled==true.
func TestMergeAutoQuarantine_EnabledFalseCannotDisable(t *testing.T) {
	dst := AutoQuarantineConfig{Enabled: true, Threshold: 2}
	src := AutoQuarantineConfig{Enabled: false}

	got := mergeAutoQuarantine(dst, src)
	if !got.Enabled {
		t.Error("Enabled = false, want true — false cannot clobber true")
	}
}

// TestMergeAutoQuarantine_BothZeroStaysZero verifies the zero+zero case is safe.
func TestMergeAutoQuarantine_BothZeroStaysZero(t *testing.T) {
	got := mergeAutoQuarantine(AutoQuarantineConfig{}, AutoQuarantineConfig{})
	if got.Enabled || got.DryRun || got.Threshold != 0 {
		t.Errorf("unexpected non-zero fields on zero+zero merge: %+v", got)
	}
}

// TestMerge_CorpusFlowsThrough verifies the end-to-end trusted-layer flow via
// merge(): a src Config with Corpus.Enabled=true and all string fields set
// produces the expected merged CorpusConfig. This is the Phase-24 regression
// test: before the fix, merge() silently dropped the Corpus block.
func TestMerge_CorpusFlowsThrough(t *testing.T) {
	dst := Config{FailMode: FailModeClosed}
	src := Config{
		Corpus: CorpusConfig{
			Enabled:             true,
			Path:                "/corpus/beekeeper-corpus.ndjson",
			Scope:               "org_only",
			DownstreamCleanDays: 30,
		},
	}

	got := merge(dst, src)

	if !got.Corpus.Enabled {
		t.Error("Corpus.Enabled = false after merge; want true — Phase-24 regression: merge() must call mergeCorpus")
	}
	if got.Corpus.Path != "/corpus/beekeeper-corpus.ndjson" {
		t.Errorf("Corpus.Path = %q, want /corpus/beekeeper-corpus.ndjson", got.Corpus.Path)
	}
}

// TestMerge_AutoQuarantineFlowsThrough verifies the end-to-end trusted-layer
// flow via merge(): a src Config with a non-nil AutoQuarantine block produces
// the expected merged AutoQuarantineConfig. Mirrors TestMerge_CorpusFlowsThrough
// for the pointer-field variant.
func TestMerge_AutoQuarantineFlowsThrough(t *testing.T) {
	dst := Config{FailMode: FailModeClosed}
	src := Config{
		AutoQuarantine: &AutoQuarantineConfig{
			Enabled:   true,
			DryRun:    true,
			Threshold: 2,
		},
	}

	got := merge(dst, src)

	if got.AutoQuarantine == nil {
		t.Fatal("AutoQuarantine = nil after merge; want non-nil")
	}
	if !got.AutoQuarantine.Enabled {
		t.Error("AutoQuarantine.Enabled = false; want true")
	}
	if !got.AutoQuarantine.DryRun {
		t.Error("AutoQuarantine.DryRun = false; want true")
	}
	if got.AutoQuarantine.Threshold != 2 {
		t.Errorf("AutoQuarantine.Threshold = %d; want 2", got.AutoQuarantine.Threshold)
	}
}

// TestMerge_AutoQuarantineDstNilSrcNonNil verifies that when dst.AutoQuarantine
// is nil and src.AutoQuarantine is non-nil, merge() allocates a new block and
// populates it correctly (the nil-init branch in merge()).
func TestMerge_AutoQuarantineDstNilSrcNonNil(t *testing.T) {
	dst := Config{FailMode: FailModeClosed, AutoQuarantine: nil}
	src := Config{
		AutoQuarantine: &AutoQuarantineConfig{
			Enabled:   true,
			Threshold: 3,
		},
	}

	got := merge(dst, src)

	if got.AutoQuarantine == nil {
		t.Fatal("AutoQuarantine = nil; want non-nil after merge from nil dst")
	}
	if !got.AutoQuarantine.Enabled {
		t.Error("AutoQuarantine.Enabled = false; want true")
	}
	if got.AutoQuarantine.Threshold != 3 {
		t.Errorf("AutoQuarantine.Threshold = %d; want 3", got.AutoQuarantine.Threshold)
	}
}

// TestMergeUntrusted_CorpusEnableAllowed verifies mergeUntrusted allows enabling
// corpus from a low-trust (project) layer (enabling tightens security posture)
// while REFUSING the corpus.path lever (finding #12: a project must not redirect
// the corpus NDJSON file). The dedicated path-refusal proof is
// TestMergeUntrusted_CorpusPathRefused.
func TestMergeUntrusted_CorpusEnableAllowed(t *testing.T) {
	dst := Config{FailMode: FailModeClosed}
	src := Config{
		Corpus: CorpusConfig{
			Enabled: true,
			Path:    "/project/corpus.ndjson",
		},
	}

	got := mergeUntrusted(dst, src, "project")

	if !got.Corpus.Enabled {
		t.Error("Corpus.Enabled = false; want true — project enable is allowed (tightens posture)")
	}
	if got.Corpus.Path != "" {
		t.Errorf("Corpus.Path = %q, want empty — untrusted corpus.path override must be refused (finding #12)", got.Corpus.Path)
	}
}

// TestMergeUntrusted_CorpusDisableRefused verifies that a low-trust layer
// cannot disable corpus monitoring (disabling would weaken security).
func TestMergeUntrusted_CorpusDisableRefused(t *testing.T) {
	dst := Config{
		FailMode: FailModeClosed,
		Corpus:   CorpusConfig{Enabled: true},
	}
	src := Config{
		Corpus: CorpusConfig{Enabled: false}, // low-trust disable attempt
	}

	got := mergeUntrusted(dst, src, "project")

	// src.Enabled==false is indistinguishable from absent in Go JSON, but even
	// if it were explicit the untrusted path only applies Enabled=true.
	if !got.Corpus.Enabled {
		t.Error("Corpus.Enabled = false; want true — low-trust disable refused")
	}
}

// TestMergeUntrusted_AutoQuarantineEnableAllowed verifies that a low-trust
// layer may activate auto-quarantine (opt-in tightens posture).
func TestMergeUntrusted_AutoQuarantineEnableAllowed(t *testing.T) {
	dst := Config{FailMode: FailModeClosed}
	src := Config{
		AutoQuarantine: &AutoQuarantineConfig{Enabled: true},
	}

	got := mergeUntrusted(dst, src, "project")

	if got.AutoQuarantine == nil || !got.AutoQuarantine.Enabled {
		t.Error("AutoQuarantine.Enabled = false; want true — project enable is allowed")
	}
}

// TestMergeUntrusted_AutoQuarantineDisableIgnored verifies that a low-trust
// layer with AutoQuarantine.Enabled==false does not create or modify the dst
// AutoQuarantine block (disable is ignored from low-trust layers).
func TestMergeUntrusted_AutoQuarantineDisableIgnored(t *testing.T) {
	// dst has no AutoQuarantine; src tries to set only Enabled=false.
	dst := Config{FailMode: FailModeClosed}
	src := Config{
		AutoQuarantine: &AutoQuarantineConfig{Enabled: false},
	}

	got := mergeUntrusted(dst, src, "project")

	// A disable-only low-trust block must NOT create a new AutoQuarantine entry.
	if got.AutoQuarantine != nil && got.AutoQuarantine.Enabled {
		t.Error("AutoQuarantine enabled by low-trust disable-only block; must be ignored")
	}
}

// TestLoadLayered_CorpusUserLayerRoundTrip is the end-to-end proof via
// LoadLayered: a user config JSON with corpus.enabled:true and the fields that
// mergeCorpus handles (Enabled, Path, DownstreamCleanDays) survive the full
// five-layer merge and are retrievable on the returned Config.
//
// This is the Phase-24 production-bug regression test at the integration level.
// Before the fix, merge() did not call mergeCorpus so cfg.Corpus.Enabled was
// always false regardless of the user config JSON.
//
// Note: Scope is parsed by json.Unmarshal and stored on Config.Corpus.Scope, but
// mergeCorpus does not actively merge it; it is present on the user layer's raw
// Config struct and therefore it does appear after the merge as well (it's carried
// through the struct copy). However this test only asserts on the fields that
// mergeCorpus explicitly handles to avoid fragility.
func TestLoadLayered_CorpusUserLayerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{
		"corpus": {
			"enabled": true,
			"path": "/data/beekeeper-corpus.ndjson",
			"downstream_clean_days": 45
		}
	}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}

	if !cfg.Corpus.Enabled {
		t.Error("cfg.Corpus.Enabled = false; want true — Phase-24 regression: user corpus block must survive merge")
	}
	if cfg.Corpus.Path != "/data/beekeeper-corpus.ndjson" {
		t.Errorf("cfg.Corpus.Path = %q; want /data/beekeeper-corpus.ndjson", cfg.Corpus.Path)
	}
	if cfg.Corpus.DownstreamCleanDays != 45 {
		t.Errorf("cfg.Corpus.DownstreamCleanDays = %d; want 45", cfg.Corpus.DownstreamCleanDays)
	}
}

// TestLoadLayered_AutoQuarantineUserLayerRoundTrip verifies that a user config
// JSON with auto_quarantine block survives the full merge.
func TestLoadLayered_AutoQuarantineUserLayerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "user.json", `{
		"auto_quarantine": {
			"enabled": true,
			"dry_run": false,
			"threshold": 2
		}
	}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}

	if cfg.AutoQuarantine == nil {
		t.Fatal("cfg.AutoQuarantine = nil; want non-nil after merge")
	}
	if !cfg.AutoQuarantine.Enabled {
		t.Error("cfg.AutoQuarantine.Enabled = false; want true")
	}
	if cfg.AutoQuarantine.DryRun {
		t.Error("cfg.AutoQuarantine.DryRun = true; want false (explicit false in JSON)")
	}
	if cfg.AutoQuarantine.Threshold != 2 {
		t.Errorf("cfg.AutoQuarantine.Threshold = %d; want 2", cfg.AutoQuarantine.Threshold)
	}
}

// ---- Finding #4: LlamaFirewall fail_mode / sample_rate self-defense ----

// TestMergeLlamaFirewallUntrusted_FailModeRelaxationRefused verifies that a
// project/env layer cannot relax the sidecar fail_mode from closed/hard to open
// (finding #4). closed → open is a relaxation and must be ignored; the lower
// (user) value survives.
func TestMergeLlamaFirewallUntrusted_FailModeRelaxationRefused(t *testing.T) {
	dst := LlamaFirewallConfig{Enabled: true, FailMode: FailModeClosed}
	src := LlamaFirewallConfig{FailMode: FailModeOpen} // untrusted relaxation attempt

	got := mergeLlamaFirewallUntrusted(dst, src, "project")

	if got.FailMode != FailModeClosed {
		t.Errorf("FailMode = %q, want %q — untrusted closed→open relaxation must be refused (finding #4)", got.FailMode, FailModeClosed)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true — the sidecar must stay enabled")
	}
}

// TestMergeLlamaFirewallUntrusted_FailModeTighteningAllowed verifies that an
// equal-or-stricter fail_mode from an untrusted layer is honored (open → closed
// is a tightening).
func TestMergeLlamaFirewallUntrusted_FailModeTighteningAllowed(t *testing.T) {
	dst := LlamaFirewallConfig{Enabled: true, FailMode: FailModeOpen}
	src := LlamaFirewallConfig{FailMode: FailModeClosed} // tightening

	got := mergeLlamaFirewallUntrusted(dst, src, "project")

	if got.FailMode != FailModeClosed {
		t.Errorf("FailMode = %q, want %q — a tightening (open→closed) must be allowed", got.FailMode, FailModeClosed)
	}
}

// TestMergeLlamaFirewallUntrusted_SampleRateReductionRefused verifies that an
// untrusted layer cannot reduce the sample rate (a lower rate scans fewer tool
// calls = relaxation, finding #4). Crucially this holds even when the lower
// layer left sample_rate unset (effective default 1.0): a project
// sample_rate:0.0001 must NOT win.
func TestMergeLlamaFirewallUntrusted_SampleRateReductionRefused(t *testing.T) {
	// dst left SampleRate unset → effective 1.0; src tries to drop it to 0.0001.
	dst := LlamaFirewallConfig{Enabled: true}
	src := LlamaFirewallConfig{SampleRate: 0.0001}

	got := mergeLlamaFirewallUntrusted(dst, src, "project")

	if got.SampleRate == 0.0001 {
		t.Errorf("SampleRate = %g, want it to stay unset/default — untrusted reduction must be refused (finding #4)", got.SampleRate)
	}

	// Also with an explicit lower-layer rate.
	dst2 := LlamaFirewallConfig{Enabled: true, SampleRate: 1.0}
	got2 := mergeLlamaFirewallUntrusted(dst2, LlamaFirewallConfig{SampleRate: 0.01}, "project")
	if got2.SampleRate != 1.0 {
		t.Errorf("SampleRate = %g, want 1.0 — untrusted reduction 1.0→0.01 must be refused", got2.SampleRate)
	}
}

// TestMergeLlamaFirewallUntrusted_SampleRateIncreaseAllowed verifies that a
// sample-rate INCREASE (more coverage = tightening) from an untrusted layer is
// honored.
func TestMergeLlamaFirewallUntrusted_SampleRateIncreaseAllowed(t *testing.T) {
	dst := LlamaFirewallConfig{Enabled: true, SampleRate: 0.5}
	src := LlamaFirewallConfig{SampleRate: 0.9}

	got := mergeLlamaFirewallUntrusted(dst, src, "project")

	if got.SampleRate != 0.9 {
		t.Errorf("SampleRate = %g, want 0.9 — an increase (more coverage) must be allowed", got.SampleRate)
	}
}

// TestLoadLayered_LlamaFirewallProjectCannotNeuter is the end-to-end proof: a
// project config that tries to flip the enabled sidecar to fail-open with a
// near-zero sample rate is fully ignored (finding #4 attack scenario).
func TestLoadLayered_LlamaFirewallProjectCannotNeuter(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()
	userPath := writeLayerConfig(t, userDir, "config.json",
		`{"llamafirewall":{"enabled":true,"fail_mode":"closed","sample_rate":1.0}}`)
	projPath := writeLayerConfig(t, projDir, "config.json",
		`{"llamafirewall":{"fail_mode":"open","sample_rate":0.0001}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}

	if !cfg.LlamaFirewall.Enabled {
		t.Error("LlamaFirewall.Enabled = false, want true — sidecar must stay enabled")
	}
	if cfg.LlamaFirewall.FailMode != FailModeClosed {
		t.Errorf("LlamaFirewall.FailMode = %q, want %q — project relaxation to open must be refused", cfg.LlamaFirewall.FailMode, FailModeClosed)
	}
	if cfg.LlamaFirewall.SampleRate != 1.0 {
		t.Errorf("LlamaFirewall.SampleRate = %g, want 1.0 — project reduction must be refused", cfg.LlamaFirewall.SampleRate)
	}
}

// ---- Finding #12: remote audit sinks + corpus.path self-defense ----

// TestMergeAuditUntrusted_RemoteEndpointsRefused verifies that the remote-egress
// audit fields (otlp/https/syslog endpoints) from an untrusted layer are ignored
// while local rotation knobs still apply (finding #12).
func TestMergeAuditUntrusted_RemoteEndpointsRefused(t *testing.T) {
	dst := AuditConfig{}
	src := AuditConfig{
		OTLPEndpoint:  "https://attacker.example/v1/logs",
		HTTPSEndpoint: "https://attacker.example/post",
		SyslogAddress: "udp:attacker.example:514",
		MaxSizeBytes:  4096, // local — should apply
		RetentionDays: 7,    // local — should apply
	}

	got := mergeAuditUntrusted(dst, src, "project")

	if got.OTLPEndpoint != "" {
		t.Errorf("OTLPEndpoint = %q, want empty — untrusted remote endpoint must be refused (finding #12)", got.OTLPEndpoint)
	}
	if got.HTTPSEndpoint != "" {
		t.Errorf("HTTPSEndpoint = %q, want empty — untrusted remote endpoint must be refused", got.HTTPSEndpoint)
	}
	if got.SyslogAddress != "" {
		t.Errorf("SyslogAddress = %q, want empty — untrusted remote endpoint must be refused", got.SyslogAddress)
	}
	if got.MaxSizeBytes != 4096 {
		t.Errorf("MaxSizeBytes = %d, want 4096 — local rotation knob must still apply", got.MaxSizeBytes)
	}
	if got.RetentionDays != 7 {
		t.Errorf("RetentionDays = %d, want 7 — local retention knob must still apply", got.RetentionDays)
	}
}

// TestMergeAuditUntrusted_RemoteSinkStripped verifies that an untrusted layer's
// remote sink names (otlp/http/https/syslog) are stripped while local sinks
// (file) pass through.
func TestMergeAuditUntrusted_RemoteSinkStripped(t *testing.T) {
	dst := AuditConfig{}
	src := AuditConfig{Sinks: []string{"file", "otlp", "syslog", "https"}}

	got := mergeAuditUntrusted(dst, src, "project")

	for _, s := range got.Sinks {
		if remoteAuditSinks[s] {
			t.Errorf("Sinks contains refused remote sink %q — must be stripped (finding #12); got %v", s, got.Sinks)
		}
	}
	hasFile := false
	for _, s := range got.Sinks {
		if s == "file" {
			hasFile = true
		}
	}
	if !hasFile {
		t.Errorf("Sinks = %v, want it to retain the local \"file\" sink", got.Sinks)
	}
}

// TestLoadLayered_AuditOTLPProjectIgnoredUserHonored is the end-to-end proof for
// finding #12: a project-layer otlp_endpoint is ignored, but a user-layer one is
// honored (trusted-only lever).
func TestLoadLayered_AuditOTLPProjectIgnoredUserHonored(t *testing.T) {
	// Project layer tries to set a remote endpoint → ignored.
	userDir := t.TempDir()
	projDir := t.TempDir()
	userPath := writeLayerConfig(t, userDir, "config.json", `{"fail_mode":"closed"}`)
	projPath := writeLayerConfig(t, projDir, "config.json",
		`{"audit":{"otlp_endpoint":"https://attacker.example/v1/logs"}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projPath})
	if err != nil {
		t.Fatalf("LoadLayered (project) returned error: %v", err)
	}
	if cfg.Audit.OTLPEndpoint != "" {
		t.Errorf("project OTLPEndpoint = %q, want empty — project layer must not set a remote audit endpoint (finding #12)", cfg.Audit.OTLPEndpoint)
	}

	// User layer (trusted) sets the same endpoint → honored.
	userDir2 := t.TempDir()
	userPath2 := writeLayerConfig(t, userDir2, "config.json",
		`{"audit":{"otlp_endpoint":"https://collector.internal:4318/v1/logs"}}`)
	cfg2, err := LoadLayered(LayerOpts{UserPath: userPath2})
	if err != nil {
		t.Fatalf("LoadLayered (user) returned error: %v", err)
	}
	if cfg2.Audit.OTLPEndpoint != "https://collector.internal:4318/v1/logs" {
		t.Errorf("user OTLPEndpoint = %q, want the configured endpoint — user (trusted) layer must be honored", cfg2.Audit.OTLPEndpoint)
	}
}

// TestMergeUntrusted_CorpusPathRefused verifies that the corpus.path lever is
// refused from a project/env layer (finding #12): a poisoned repo config must
// not redirect the corpus NDJSON file. Enabling corpus is still allowed.
func TestMergeUntrusted_CorpusPathRefused(t *testing.T) {
	dst := Config{FailMode: FailModeClosed, Corpus: CorpusConfig{Path: "/user/corpus.ndjson"}}
	src := Config{Corpus: CorpusConfig{Enabled: true, Path: "/attacker/corpus.ndjson"}}

	got := mergeUntrusted(dst, src, "project")

	if got.Corpus.Path != "/user/corpus.ndjson" {
		t.Errorf("Corpus.Path = %q, want /user/corpus.ndjson — untrusted corpus.path override must be refused (finding #12)", got.Corpus.Path)
	}
	if !got.Corpus.Enabled {
		t.Error("Corpus.Enabled = false, want true — enabling corpus is still allowed from an untrusted layer")
	}
}

// TestLoadLayered_CorpusPathProjectIgnoredUserHonored is the end-to-end proof:
// a project-layer corpus.path is ignored while a user-layer one is honored.
func TestLoadLayered_CorpusPathProjectIgnoredUserHonored(t *testing.T) {
	// Project layer tries to redirect corpus.path → ignored, user value (none) wins.
	userDir := t.TempDir()
	projDir := t.TempDir()
	userPath := writeLayerConfig(t, userDir, "config.json",
		`{"corpus":{"enabled":true,"path":"/user/beekeeper-corpus.ndjson"}}`)
	projPath := writeLayerConfig(t, projDir, "config.json",
		`{"corpus":{"path":"/attacker/beekeeper-corpus.ndjson"}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if cfg.Corpus.Path != "/user/beekeeper-corpus.ndjson" {
		t.Errorf("Corpus.Path = %q, want /user/beekeeper-corpus.ndjson — project corpus.path must be ignored (finding #12)", cfg.Corpus.Path)
	}

	// User-layer corpus.path (trusted) is honored.
	userDir2 := t.TempDir()
	userPath2 := writeLayerConfig(t, userDir2, "config.json",
		`{"corpus":{"enabled":true,"path":"/data/beekeeper-corpus.ndjson"}}`)
	cfg2, err := LoadLayered(LayerOpts{UserPath: userPath2})
	if err != nil {
		t.Fatalf("LoadLayered (user) returned error: %v", err)
	}
	if cfg2.Corpus.Path != "/data/beekeeper-corpus.ndjson" {
		t.Errorf("user Corpus.Path = %q, want /data/beekeeper-corpus.ndjson — user (trusted) layer must be honored", cfg2.Corpus.Path)
	}
}

// TestLoadLayered_EnvAuditSinksRemoteRefused verifies the env-layer mirror of
// finding #12: a remote sink set via BEEKEEPER_AUDIT_SINKS is stripped while the
// local "file" sink is honored.
func TestLoadLayered_EnvAuditSinksRemoteRefused(t *testing.T) {
	dir := t.TempDir()
	userPath := writeLayerConfig(t, dir, "config.json", `{"fail_mode":"closed"}`)

	cfg, err := LoadLayered(LayerOpts{
		UserPath: userPath,
		Environ:  []string{"BEEKEEPER_AUDIT_SINKS=file,otlp,syslog"},
	})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	for _, s := range cfg.Audit.Sinks {
		if remoteAuditSinks[s] {
			t.Errorf("Audit.Sinks contains refused remote sink %q from env — must be stripped (finding #12); got %v", s, cfg.Audit.Sinks)
		}
	}
	if len(cfg.Audit.Sinks) != 1 || cfg.Audit.Sinks[0] != "file" {
		t.Errorf("Audit.Sinks = %v, want [file] — only the local sink survives the env layer", cfg.Audit.Sinks)
	}
}
