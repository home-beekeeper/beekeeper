package config

import (
	"path/filepath"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// Per-rule install-posture severity config tests (IPOVR-03, Plan 29-01).
//
// Default is warn for every rule (the shipped Phase 27 default). A user may opt a
// rule UP to block via a trusted layer; a bogus action is rejected fail-closed at
// load time; the accessor is nil-safe. The tighten-only untrusted merge invariant
// (warn->block applied, block->warn ignored) lives in layered_posture_test.go.
// ---------------------------------------------------------------------------

// TestDefaultPostureConfigAllWarn verifies the documented default: every rule warns.
func TestDefaultPostureConfigAllWarn(t *testing.T) {
	def := DefaultPostureConfig()
	if def.ReleaseAge.Action != PostureActionWarn ||
		def.Lifecycle.Action != PostureActionWarn ||
		def.RemoteSource.Action != PostureActionWarn {
		t.Fatalf("DefaultPostureConfig = %+v, want every rule action %q", def, PostureActionWarn)
	}
}

// TestValidatePostureConfigAccepts verifies the legal actions ("", warn, block)
// pass validation for every rule.
func TestValidatePostureConfigAccepts(t *testing.T) {
	for _, action := range []string{"", PostureActionWarn, PostureActionBlock} {
		pc := PostureConfig{
			ReleaseAge:   PostureRuleConfig{Action: action},
			Lifecycle:    PostureRuleConfig{Action: action},
			RemoteSource: PostureRuleConfig{Action: action},
		}
		if err := ValidatePostureConfig(pc); err != nil {
			t.Errorf("ValidatePostureConfig(action=%q) = %v, want nil", action, err)
		}
	}
}

// TestValidatePostureConfigRejectsBogus verifies a bogus action is rejected
// fail-closed (one rule per case so the error names the offending rule).
func TestValidatePostureConfigRejectsBogus(t *testing.T) {
	cases := []PostureConfig{
		{ReleaseAge: PostureRuleConfig{Action: "off"}},
		{Lifecycle: PostureRuleConfig{Action: "deny"}},
		{RemoteSource: PostureRuleConfig{Action: "BLOCK"}}, // case-sensitive: not "block"
	}
	for i, pc := range cases {
		if err := ValidatePostureConfig(pc); err == nil {
			t.Errorf("case %d: ValidatePostureConfig(%+v) = nil, want a fail-closed rejection", i, pc)
		}
	}
}

// TestPostureRuleActionNilSafe verifies a nil Posture block (absent config) resolves
// every rule to the warn default, and an unknown rule name also resolves to warn.
func TestPostureRuleActionNilSafe(t *testing.T) {
	var c Config // Posture is nil
	for _, rule := range []string{PostureRuleReleaseAge, PostureRuleLifecycle, PostureRuleRemoteSource, "bogus-rule"} {
		if got := c.PostureRuleAction(rule); got != PostureActionWarn {
			t.Errorf("PostureRuleAction(%q) on nil Posture = %q, want %q", rule, got, PostureActionWarn)
		}
	}
}

// TestPostureRuleActionReturnsConfigured verifies a configured block is returned and
// an empty/unset rule action resolves to warn.
func TestPostureRuleActionReturnsConfigured(t *testing.T) {
	c := Config{Posture: &PostureConfig{
		ReleaseAge:   PostureRuleConfig{Action: PostureActionBlock},
		Lifecycle:    PostureRuleConfig{Action: ""}, // unset -> warn
		RemoteSource: PostureRuleConfig{Action: PostureActionWarn},
	}}
	if got := c.PostureRuleAction(PostureRuleReleaseAge); got != PostureActionBlock {
		t.Errorf("PostureRuleAction(release-age) = %q, want %q", got, PostureActionBlock)
	}
	if got := c.PostureRuleAction(PostureRuleLifecycle); got != PostureActionWarn {
		t.Errorf("PostureRuleAction(lifecycle) = %q, want %q (unset resolves to warn)", got, PostureActionWarn)
	}
	if got := c.PostureRuleAction(PostureRuleRemoteSource); got != PostureActionWarn {
		t.Errorf("PostureRuleAction(git-remote) = %q, want %q", got, PostureActionWarn)
	}
}

// TestLoadRejectsBogusPostureAction verifies Load wires ValidatePostureConfig: a
// config file with a bogus posture action fails to load fail-closed.
func TestLoadRejectsBogusPostureAction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"posture":{"release_age":{"action":"off"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load with a bogus posture action = nil error, want a fail-closed rejection")
	}
}

// TestLoadAcceptsPostureBlock verifies a valid block action loads and the accessor
// returns it.
func TestLoadAcceptsPostureBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"posture":{"release_age":{"action":"block"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.PostureRuleAction(PostureRuleReleaseAge); got != PostureActionBlock {
		t.Errorf("PostureRuleAction(release-age) = %q, want %q after load", got, PostureActionBlock)
	}
}
