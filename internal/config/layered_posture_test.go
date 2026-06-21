package config

import "testing"

// ---------------------------------------------------------------------------
// Tighten-only untrusted posture merge (IPOVR-03 self-defense).
//
// An untrusted (project/env) layer may RAISE a rule warn->block (a tightening);
// a block->warn loosening from an untrusted layer is REFUSED. This mirrors the
// FailMode (TM-D-01) and LlamaFirewall.FailMode untrusted gates. A trusted layer
// may set either direction.
// ---------------------------------------------------------------------------

// TestMergePostureUntrustedAllowsTighten verifies an untrusted layer raising
// release-age warn->block is applied (tightening is always safe).
func TestMergePostureUntrustedAllowsTighten(t *testing.T) {
	dst := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: PostureActionWarn}}
	src := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: PostureActionBlock}}
	out := mergePostureUntrusted(dst, src, "project")
	if out == nil {
		t.Fatal("merged Posture = nil, want non-nil")
	}
	if out.ReleaseAge.Action != PostureActionBlock {
		t.Errorf("ReleaseAge.Action = %q, want %q -- an untrusted tighten warn->block is allowed (IPOVR-03)",
			out.ReleaseAge.Action, PostureActionBlock)
	}
}

// TestMergePostureUntrustedRefusesLoosen verifies an untrusted layer lowering
// release-age block->warn is IGNORED (the tighten-only invariant). The block set
// by the trusted lower layer survives.
func TestMergePostureUntrustedRefusesLoosen(t *testing.T) {
	dst := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: PostureActionBlock}}
	src := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: PostureActionWarn}}
	out := mergePostureUntrusted(dst, src, "project")
	if out.ReleaseAge.Action != PostureActionBlock {
		t.Errorf("ReleaseAge.Action = %q, want %q -- an untrusted loosen block->warn must be refused (IPOVR-03)",
			out.ReleaseAge.Action, PostureActionBlock)
	}
}

// TestMergePostureUntrustedEmptySrcKeepsDst verifies an absent (empty) src action
// leaves the lower layer authoritative.
func TestMergePostureUntrustedEmptySrcKeepsDst(t *testing.T) {
	dst := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: PostureActionBlock}}
	src := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: ""}}
	out := mergePostureUntrusted(dst, src, "env")
	if out.ReleaseAge.Action != PostureActionBlock {
		t.Errorf("ReleaseAge.Action = %q, want %q -- an absent src action keeps the lower layer value",
			out.ReleaseAge.Action, PostureActionBlock)
	}
}

// TestMergePostureTrustedMayLoosen verifies the TRUSTED merge may lower a rule
// block->warn (a trusted user/global layer is unrestricted, unlike untrusted).
func TestMergePostureTrustedMayLoosen(t *testing.T) {
	dst := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: PostureActionBlock}}
	src := &PostureConfig{ReleaseAge: PostureRuleConfig{Action: PostureActionWarn}}
	out := mergePosture(dst, src)
	if out.ReleaseAge.Action != PostureActionWarn {
		t.Errorf("ReleaseAge.Action = %q, want %q -- a trusted layer may loosen", out.ReleaseAge.Action, PostureActionWarn)
	}
}

// TestLoadLayeredPostureProjectCanTightenNotLoosen is the end-to-end proof through
// LoadLayered: a project layer can raise lifecycle warn->block, but cannot lower a
// user-set release-age block->warn.
func TestLoadLayeredPostureProjectCanTightenNotLoosen(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()
	// User (trusted) blocks release-age; leaves lifecycle at warn default.
	userPath := writeLayerConfig(t, userDir, "config.json",
		`{"posture":{"release_age":{"action":"block"},"lifecycle":{"action":"warn"}}}`)
	// Project (untrusted) tries to loosen release-age->warn (refused) and tighten
	// lifecycle->block (allowed).
	projPath := writeLayerConfig(t, projDir, "config.json",
		`{"posture":{"release_age":{"action":"warn"},"lifecycle":{"action":"block"}}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	if got := cfg.PostureRuleAction(PostureRuleReleaseAge); got != PostureActionBlock {
		t.Errorf("release-age = %q, want %q -- project must not loosen a user block (IPOVR-03)", got, PostureActionBlock)
	}
	if got := cfg.PostureRuleAction(PostureRuleLifecycle); got != PostureActionBlock {
		t.Errorf("lifecycle = %q, want %q -- project tightening warn->block is allowed", got, PostureActionBlock)
	}
}

// TestLoadLayeredPostureProjectOnlyDefaultsWarn verifies that with no posture block
// anywhere, every rule resolves to warn (the absent-everywhere default).
func TestLoadLayeredPostureProjectOnlyDefaultsWarn(t *testing.T) {
	userDir := t.TempDir()
	userPath := writeLayerConfig(t, userDir, "config.json", `{}`)
	cfg, err := LoadLayered(LayerOpts{UserPath: userPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	for _, rule := range []string{PostureRuleReleaseAge, PostureRuleLifecycle, PostureRuleRemoteSource} {
		if got := cfg.PostureRuleAction(rule); got != PostureActionWarn {
			t.Errorf("%s = %q, want %q (absent everywhere defaults to warn)", rule, got, PostureActionWarn)
		}
	}
}

// ---------------------------------------------------------------------------
// Scoped allow-always list (IPOVR-01/02): adding an allow LOOSENS the posture, so
// Allow entries flow from TRUSTED layers only. mergePostureUntrusted DROPS
// src.Allow. These tests lock that invariant.
// ---------------------------------------------------------------------------

// TestMergePostureTrustedCarriesAllow verifies a TRUSTED layer's Allow entries are
// appended to the lower layer's entries (a trusted layer may add an exemption).
func TestMergePostureTrustedCarriesAllow(t *testing.T) {
	dst := &PostureConfig{Allow: []PostureAllow{{Package: "lower", Rule: ""}}}
	src := &PostureConfig{Allow: []PostureAllow{{Package: "upper", Rule: "release-age"}}}
	out := mergePosture(dst, src)
	if len(out.Allow) != 2 {
		t.Fatalf("Allow len = %d, want 2 (trusted layer appends its allow entries); got %+v", len(out.Allow), out.Allow)
	}
}

// TestMergePostureUntrustedDropsAllow verifies an UNTRUSTED layer's Allow entries
// are DROPPED when there is a trusted baseline (adding an allow loosens; an
// untrusted layer may not add an exemption).
func TestMergePostureUntrustedDropsAllow(t *testing.T) {
	dst := &PostureConfig{Allow: []PostureAllow{{Package: "trusted-entry"}}}
	src := &PostureConfig{Allow: []PostureAllow{{Package: "untrusted-injected"}}}
	out := mergePostureUntrusted(dst, src, "project")
	if len(out.Allow) != 1 || out.Allow[0].Package != "trusted-entry" {
		t.Fatalf("Allow = %+v, want only the trusted baseline entry (untrusted Allow must be dropped)", out.Allow)
	}
}

// TestMergePostureUntrustedDropsAllowNilDst verifies that even with NO trusted
// baseline (dst == nil), an untrusted layer's Allow entries are dropped: an
// untrusted layer never contributes a standing exemption.
func TestMergePostureUntrustedDropsAllowNilDst(t *testing.T) {
	src := &PostureConfig{
		ReleaseAge: PostureRuleConfig{Action: PostureActionBlock}, // a tightening, allowed
		Allow:      []PostureAllow{{Package: "untrusted-injected"}},
	}
	out := mergePostureUntrusted(nil, src, "project")
	if out == nil {
		t.Fatal("merged Posture = nil, want non-nil")
	}
	if len(out.Allow) != 0 {
		t.Errorf("Allow = %+v, want empty (untrusted layer cannot add an allow even with no baseline)", out.Allow)
	}
	// The tightening action still applies (only the Allow list is dropped).
	if out.ReleaseAge.Action != PostureActionBlock {
		t.Errorf("ReleaseAge.Action = %q, want %q (an untrusted tightening still applies)", out.ReleaseAge.Action, PostureActionBlock)
	}
}

// TestLoadLayeredPostureProjectCannotAddAllow is the end-to-end proof through
// LoadLayered: a project (untrusted) PostureAllow entry is ignored, while a user
// (trusted) entry is honored. PostureRuleExcludes reflects only the trusted entry.
func TestLoadLayeredPostureProjectCannotAddAllow(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()
	userPath := writeLayerConfig(t, userDir, "config.json",
		`{"posture":{"allow":[{"package":"user-trusted-pkg","ecosystem":"npm"}]}}`)
	projPath := writeLayerConfig(t, projDir, "config.json",
		`{"posture":{"allow":[{"package":"project-injected-pkg","ecosystem":"npm"}]}}`)

	cfg, err := LoadLayered(LayerOpts{UserPath: userPath, ProjectPath: projPath})
	if err != nil {
		t.Fatalf("LoadLayered returned error: %v", err)
	}
	excludes := cfg.PostureRuleExcludes(PostureRuleReleaseAge, "npm")
	if len(excludes) != 1 || excludes[0] != "user-trusted-pkg" {
		t.Fatalf("PostureRuleExcludes = %v, want only [user-trusted-pkg] (a project allow entry must be ignored)", excludes)
	}
	for _, e := range excludes {
		if e == "project-injected-pkg" {
			t.Fatal("project-injected-pkg leaked into the posture excludes -- an untrusted layer added an allow")
		}
	}
}

// TestPostureRuleExcludesScoping verifies the rule/ecosystem scoping of
// PostureRuleExcludes: an all-rules entry matches every rule; a rule-scoped entry
// matches only its rule; an ecosystem-scoped entry matches only its ecosystem.
func TestPostureRuleExcludesScoping(t *testing.T) {
	c := Config{Posture: &PostureConfig{Allow: []PostureAllow{
		{Package: "all-rules-pkg"},                                  // matches every rule, any eco
		{Package: "age-only-pkg", Rule: PostureRuleReleaseAge},      // only release-age
		{Package: "npm-only-pkg", Ecosystem: "npm"},                 // only npm
		{Package: "", Rule: PostureRuleReleaseAge},                  // empty package -> ignored
	}}}

	ageNpm := c.PostureRuleExcludes(PostureRuleReleaseAge, "npm")
	if !contains(ageNpm, "all-rules-pkg") || !contains(ageNpm, "age-only-pkg") || !contains(ageNpm, "npm-only-pkg") {
		t.Errorf("release-age/npm excludes = %v, want all three matching entries", ageNpm)
	}
	lifePypi := c.PostureRuleExcludes(PostureRuleLifecycle, "pypi")
	if contains(lifePypi, "age-only-pkg") {
		t.Errorf("lifecycle/pypi excludes = %v, must NOT contain the release-age-only entry", lifePypi)
	}
	if contains(lifePypi, "npm-only-pkg") {
		t.Errorf("lifecycle/pypi excludes = %v, must NOT contain the npm-only entry", lifePypi)
	}
	if !contains(lifePypi, "all-rules-pkg") {
		t.Errorf("lifecycle/pypi excludes = %v, must contain the all-rules entry", lifePypi)
	}
	for _, e := range ageNpm {
		if e == "" {
			t.Error("an empty-package allow entry must be skipped, not returned as an exclude")
		}
	}
}

// contains reports whether s is in xs.
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
