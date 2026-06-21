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
