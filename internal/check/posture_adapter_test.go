package check

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// postureNow is a synthetic wall-clock for posture unit tests.
var postureNow = time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)

// stubPostureFetchers swaps the package-level posture fetch seams for the
// duration of a test and restores them on cleanup. ageMinutes/ageMissing/ageErr
// drive the release-age path; scripts/lifeFailed/lifeErr drive the lifecycle path.
func stubPostureFetchers(
	t *testing.T,
	ageMinutes int64, ageMissing bool, ageErr error,
	scripts []string, lifeFailed bool, lifeErr error,
) {
	t.Helper()
	origAge := posturePublishAgeFn
	origLife := postureLifecycleFn
	posturePublishAgeFn = func(_ context.Context, _ *http.Client, _, _, _, _ string, _ time.Time) (int64, bool, error) {
		return ageMinutes, ageMissing, ageErr
	}
	postureLifecycleFn = func(_ context.Context, _ *http.Client, _, _, _, _ string, _ time.Time) ([]string, bool, error) {
		return scripts, lifeFailed, lifeErr
	}
	t.Cleanup(func() {
		posturePublishAgeFn = origAge
		postureLifecycleFn = origLife
	})
}

// bashInstall builds a Bash install ToolCall.
func bashInstall(cmd string) policy.ToolCall {
	return policy.ToolCall{ToolName: "Bash", ToolInput: map[string]any{"command": cmd}}
}

func runPosture(t *testing.T, tc policy.ToolCall) (policy.Decision, bool) {
	t.Helper()
	return evaluatePosture(context.Background(), tc, config.Config{}, &http.Client{}, t.TempDir(), postureNow)
}

// runPostureCfg runs evaluatePosture with an explicit config (IPOVR-03 per-rule
// action overrides).
func runPostureCfg(t *testing.T, tc policy.ToolCall, cfg config.Config) (policy.Decision, bool) {
	t.Helper()
	return evaluatePosture(context.Background(), tc, cfg, &http.Client{}, t.TempDir(), postureNow)
}

// blockReleaseAgeCfg returns a config that opts the release-age rule UP to block.
func blockReleaseAgeCfg() config.Config {
	return config.Config{Posture: &config.PostureConfig{
		ReleaseAge: config.PostureRuleConfig{Action: config.PostureActionBlock},
	}}
}

// TestPostureBlockModeBlocksFreshPackage: with release-age opted up to block, a
// <24h package on a DEFINITE violation BLOCKS (Allow:false, Level:block).
func TestPostureBlockModeBlocksFreshPackage(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // 30 min old, definite violation
	dec, ok := runPostureCfg(t, bashInstall("npm install left-pad@1.0.0"), blockReleaseAgeCfg())
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if dec.Allow || dec.Level != "block" {
		t.Fatalf("decision = %+v, want Allow:false Level:block (release-age opted up to block on a <24h package)", dec)
	}
}

// TestPostureBlockModeMissingTimestampStillWarns: the IPOVR-03 fail-soft invariant
// at the adapter level. With release-age opted up to block, a MISSING timestamp
// (unknown input) still WARNS, never blocks -- a registry outage cannot turn into a
// blocked install even under block mode.
func TestPostureBlockModeMissingTimestampStillWarns(t *testing.T) {
	stubPostureFetchers(t, 0, true, nil, nil, false, nil) // timestamp missing (unknown)
	dec, ok := runPostureCfg(t, bashInstall("npm install left-pad@1.0.0"), blockReleaseAgeCfg())
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level == "block" {
		t.Fatalf("decision = %+v, want a non-block warn (unknown stays fail-soft even under block mode)", dec)
	}
}

// TestPostureBlockModeRegistryErrorStillWarns: a registry error under block mode
// still warns (fail-soft), never blocks.
func TestPostureBlockModeRegistryErrorStillWarns(t *testing.T) {
	stubPostureFetchers(t, 0, false, context.DeadlineExceeded, nil, false, nil)
	dec, ok := runPostureCfg(t, bashInstall("npm install left-pad@1.0.0"), blockReleaseAgeCfg())
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level == "block" {
		t.Fatalf("decision = %+v, want a non-block warn on registry error even under block mode", dec)
	}
}

// TestPostureBlockModeOldCleanPackageAllows: block mode does NOT block a clean
// install -- only a fired rule. An old, script-free package still allows.
func TestPostureBlockModeOldCleanPackageAllows(t *testing.T) {
	stubPostureFetchers(t, 100000, false, nil, nil, false, nil) // old, clean
	dec, ok := runPostureCfg(t, bashInstall("npm install lodash@4.17.21"), blockReleaseAgeCfg())
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level != "allow" {
		t.Fatalf("decision = %+v, want Allow:true Level:allow (block mode never blocks a clean install)", dec)
	}
}

// TestPostureDefaultNoConfigStillWarns: with no Posture config at all, a fresh
// package still WARNS (the default is unchanged by this plan).
func TestPostureDefaultNoConfigStillWarns(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil)
	dec, ok := runPostureCfg(t, bashInstall("npm install left-pad@1.0.0"), config.Config{})
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level != "warn" {
		t.Fatalf("decision = %+v, want Allow:true Level:warn (default unchanged: fresh package warns)", dec)
	}
}

// TestPostureBlockModeOnlyAffectsOptedRule: with ONLY lifecycle opted up to block, a
// fresh-package release-age violation still WARNS (release-age left at warn).
func TestPostureBlockModeOnlyAffectsOptedRule(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // fresh (release-age fires), no lifecycle scripts
	cfg := config.Config{Posture: &config.PostureConfig{
		Lifecycle: config.PostureRuleConfig{Action: config.PostureActionBlock},
	}}
	dec, ok := runPostureCfg(t, bashInstall("npm install left-pad@1.0.0"), cfg)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level != "warn" {
		t.Fatalf("decision = %+v, want Allow:true Level:warn (only lifecycle is opted up; release-age stays warn)", dec)
	}
}

// TestPostureFreshPackageWarns: a <24h package produces a WARN (age below
// minimum), Allow:true (exit 0, does not block).
func TestPostureFreshPackageWarns(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // 30 min old, no lifecycle scripts
	dec, ok := runPosture(t, bashInstall("npm install left-pad@1.0.0"))
	if !ok {
		t.Fatal("ok = false, want true for an install command")
	}
	if !dec.Allow || dec.Level != "warn" {
		t.Fatalf("decision = %+v, want Allow:true Level:warn (fresh package warns, does not block)", dec)
	}
}

// TestPostureOldCleanPackageAllows: an old package with no lifecycle scripts and
// no remote source produces an allow.
func TestPostureOldCleanPackageAllows(t *testing.T) {
	stubPostureFetchers(t, 100000, false, nil, nil, false, nil) // ~69 days old, clean
	dec, ok := runPosture(t, bashInstall("npm install left-pad@1.0.0"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level != "allow" {
		t.Fatalf("decision = %+v, want Allow:true Level:allow", dec)
	}
}

// TestPostureLifecycleScriptsWarn: an old package that carries a postinstall
// lifecycle script warns (does not block).
func TestPostureLifecycleScriptsWarn(t *testing.T) {
	stubPostureFetchers(t, 100000, false, nil, []string{"postinstall"}, false, nil)
	dec, ok := runPosture(t, bashInstall("npm install some-pkg@2.0.0"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level != "warn" {
		t.Fatalf("decision = %+v, want Allow:true Level:warn (lifecycle scripts warn)", dec)
	}
}

// TestPostureRemoteSourceWarns: a git install warns via the remote-source rule
// WITHOUT any registry fetch (the fetchers must not even be consulted - assert
// by stubbing them to block-equivalent results that, if used, would surface).
func TestPostureRemoteSourceWarns(t *testing.T) {
	// If the registry fetchers were (incorrectly) consulted for a git install,
	// these would still only warn (fail-soft), so the assertion below relies on
	// the decision being a warn either way; the key invariant is no block.
	stubPostureFetchers(t, 0, true, nil, nil, true, nil)
	for _, cmd := range []string{
		"npm install git+https://github.com/evil/pkg.git",
		"pip install https://example.com/pkg.tar.gz",
		"npm install ./local-pkg",
	} {
		dec, ok := runPosture(t, bashInstall(cmd))
		if !ok {
			t.Fatalf("%q: ok = false, want true", cmd)
		}
		if !dec.Allow || dec.Level != "warn" {
			t.Fatalf("%q: decision = %+v, want Allow:true Level:warn", cmd, dec)
		}
	}
}

// TestPostureMissingTimestampWarnsNotBlock: a missing publish timestamp produces
// a WARN-unknown (NOT a block) - the deliberate fail-soft divergence from the
// pure evaluator's fail-closed block.
func TestPostureMissingTimestampWarnsNotBlock(t *testing.T) {
	stubPostureFetchers(t, 0, true, nil, nil, false, nil) // timestamp missing
	dec, ok := runPosture(t, bashInstall("npm install left-pad@1.0.0"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow {
		t.Fatalf("decision = %+v, want Allow:true (missing timestamp must WARN, not block)", dec)
	}
	if dec.Level == "block" {
		t.Fatalf("decision = %+v, want warn not block on missing timestamp (fail-soft)", dec)
	}
}

// TestPostureRegistryErrorWarnsNotBlock: a registry error on the age fetch
// produces a warn-unknown, not a block.
func TestPostureRegistryErrorWarnsNotBlock(t *testing.T) {
	stubPostureFetchers(t, 0, false, context.DeadlineExceeded, nil, false, nil)
	dec, ok := runPosture(t, bashInstall("npm install left-pad@1.0.0"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level == "block" {
		t.Fatalf("decision = %+v, want a non-block warn on registry error (fail-soft)", dec)
	}
}

// TestPostureLifecycleUnsupportedWarnsNotBlock: an unsupported-ecosystem
// lifecycle failure warns, not blocks.
func TestPostureLifecycleUnsupportedWarnsNotBlock(t *testing.T) {
	stubPostureFetchers(t, 100000, false, nil, nil, true, nil) // lifecycle failed (unsupported)
	dec, ok := runPosture(t, bashInstall("pip install requests@2.0.0"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level == "block" {
		t.Fatalf("decision = %+v, want a non-block warn on lifecycle-unknown (fail-soft)", dec)
	}
}

// TestPostureCleanRegistryInstallAllows: an old, script-free registry install
// allows.
func TestPostureCleanRegistryInstallAllows(t *testing.T) {
	stubPostureFetchers(t, 100000, false, nil, []string{}, false, nil)
	dec, ok := runPosture(t, bashInstall("npm install lodash@4.17.21"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level != "allow" {
		t.Fatalf("decision = %+v, want Allow:true Level:allow", dec)
	}
}

// TestPostureSkipsNonBash: a non-Bash tool call is skipped (ok=false).
func TestPostureSkipsNonBash(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil)
	_, ok := evaluatePosture(context.Background(), policy.ToolCall{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": "/tmp/x"},
	}, config.Config{}, &http.Client{}, t.TempDir(), postureNow)
	if ok {
		t.Fatal("ok = true, want false for a non-Bash tool call")
	}
}

// TestPostureSkipsNonInstall: a Bash command that is not an install is skipped.
func TestPostureSkipsNonInstall(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil)
	for _, cmd := range []string{"npm run build", "ls -la", "git status", ""} {
		_, ok := runPosture(t, bashInstall(cmd))
		if ok {
			t.Fatalf("%q: ok = true, want false (non-install command must skip posture)", cmd)
		}
	}
}

// TestPosturizeAllowPassesThrough: posturizeWithAction leaves an allow unchanged,
// even under the block action (an allow is never lifted).
func TestPosturizeAllowPassesThrough(t *testing.T) {
	in := policy.Decision{Allow: true, Level: "allow", Reason: "ok", RuleIDs: []string{"x"}}
	for _, action := range []string{config.PostureActionWarn, config.PostureActionBlock, ""} {
		out := posturizeWithAction(in, action)
		if out.Level != "allow" || !out.Allow {
			t.Fatalf("posturizeWithAction(allow, %q) = %+v, want unchanged allow", action, out)
		}
	}
}

// TestPosturizeBlockBecomesWarn: with the default warn action, posturizeWithAction
// re-maps a pure-evaluator BLOCK to a WARN (Allow:true) - the WARN-default contract.
func TestPosturizeBlockBecomesWarn(t *testing.T) {
	in := policy.Decision{Allow: false, Level: "block", Reason: "too young", RuleIDs: []string{"release-age-policy"}}
	out := posturizeWithAction(in, config.PostureActionWarn)
	if !out.Allow || out.Level != "warn" {
		t.Fatalf("posturizeWithAction(block, warn) = %+v, want Allow:true Level:warn (default posture is warn)", out)
	}
	if out.Reason != in.Reason {
		t.Errorf("posturizeWithAction dropped reason: got %q want %q", out.Reason, in.Reason)
	}
}

// TestPosturizeBlockActionKeepsBlock: with action=block, posturizeWithAction keeps a
// fired rule a BLOCK (Allow:false), preserving the evaluator reason and rule IDs.
// This is the IPOVR-03 opt-up path.
func TestPosturizeBlockActionKeepsBlock(t *testing.T) {
	in := policy.Decision{Allow: false, Level: "block", Reason: "too young", RuleIDs: []string{"release-age-policy"}}
	out := posturizeWithAction(in, config.PostureActionBlock)
	if out.Allow || out.Level != "block" {
		t.Fatalf("posturizeWithAction(block, block) = %+v, want Allow:false Level:block (opted up)", out)
	}
	if out.Reason != in.Reason {
		t.Errorf("posturizeWithAction(block) dropped reason: got %q want %q", out.Reason, in.Reason)
	}
	if len(out.RuleIDs) != 1 || out.RuleIDs[0] != "release-age-policy" {
		t.Errorf("posturizeWithAction(block) dropped rule IDs: got %v", out.RuleIDs)
	}
}

// TestPosturizeWarnActionKeepsRemoteWarn: a remote-source rule fires as a warn
// (not a block) from the pure evaluator; under the warn action it stays a warn.
func TestPosturizeWarnActionKeepsRemoteWarn(t *testing.T) {
	in := policy.Decision{Allow: true, Level: "warn", Reason: "remote source", RuleIDs: []string{"remote-source-policy"}}
	out := posturizeWithAction(in, config.PostureActionWarn)
	if !out.Allow || out.Level != "warn" {
		t.Fatalf("posturizeWithAction(remote-warn, warn) = %+v, want Allow:true Level:warn", out)
	}
}

// TestPosturizeBlockActionLiftsRemoteWarn: a fired remote-source warn is lifted to
// a BLOCK when that rule is opted up to block (a fired rule, definite violation).
func TestPosturizeBlockActionLiftsRemoteWarn(t *testing.T) {
	in := policy.Decision{Allow: true, Level: "warn", Reason: "remote source", RuleIDs: []string{"remote-source-policy"}}
	out := posturizeWithAction(in, config.PostureActionBlock)
	if out.Allow || out.Level != "block" {
		t.Fatalf("posturizeWithAction(remote-warn, block) = %+v, want Allow:false Level:block (opted up)", out)
	}
}

// ---------------------------------------------------------------------------
// Scoped allow-always (IPOVR-02): a posture-scoped Allow entry feeds the per-rule
// Exclude of the pure evaluator so the package stops firing THAT posture rule.
// ---------------------------------------------------------------------------

// allowAlwaysCfg returns a config with a posture-scoped allow-always entry for pkg
// (all rules, npm) -- mirrors what `beekeeper posture allow --always` records.
func allowAlwaysCfg(pkg string) config.Config {
	return config.Config{Posture: &config.PostureConfig{
		Allow: []config.PostureAllow{{Ecosystem: "npm", Package: pkg, Reason: "vetted"}},
	}}
}

// TestPostureAllowAlwaysExemptsFreshPackage: a fresh (<24h) package that is
// posture-allowlisted ALLOWS (no warn) while a non-allowlisted fresh package still
// WARNS. Proves the Exclude wiring reaches the release-age evaluator.
func TestPostureAllowAlwaysExemptsFreshPackage(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // fresh, no lifecycle scripts

	// Allowlisted fresh package -> allow.
	dec, ok := runPostureCfg(t, bashInstall("npm install vetted-fresh@1.0.0"), allowAlwaysCfg("vetted-fresh"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec.Allow || dec.Level != "allow" {
		t.Fatalf("allowlisted fresh package decision = %+v, want Allow:true Level:allow (posture allow-always exempts it)", dec)
	}

	// A DIFFERENT fresh package (not allowlisted) still warns under the same config.
	dec2, ok := runPostureCfg(t, bashInstall("npm install other-fresh@1.0.0"), allowAlwaysCfg("vetted-fresh"))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !dec2.Allow || dec2.Level != "warn" {
		t.Fatalf("non-allowlisted fresh package decision = %+v, want Allow:true Level:warn (only the allowlisted package is exempt)", dec2)
	}
}

// TestPostureAllowAlwaysRuleScoped: a release-age-scoped allow-always exempts the
// release-age rule but NOT the lifecycle rule -- a fresh package with lifecycle
// scripts still warns on the lifecycle rule.
func TestPostureAllowAlwaysRuleScoped(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, []string{"postinstall"}, false, nil) // fresh AND lifecycle scripts
	cfg := config.Config{Posture: &config.PostureConfig{
		Allow: []config.PostureAllow{{Ecosystem: "npm", Package: "scoped-pkg", Rule: config.PostureRuleReleaseAge, Reason: "r"}},
	}}
	dec, ok := runPostureCfg(t, bashInstall("npm install scoped-pkg@1.0.0"), cfg)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	// release-age is exempted, but lifecycle scripts still fire -> warn (not allow).
	if !dec.Allow || dec.Level != "warn" {
		t.Fatalf("decision = %+v, want Allow:true Level:warn (release-age exempt but lifecycle still warns)", dec)
	}
}

// TestPostureAllowOnceConsumedThenWarns drives the allow-once path through the
// adapter: with a one-shot token recorded, the first matching install ALLOWS, and
// (the token consumed) the next identical install WARNS again. The store lives
// under the parent of cacheDir, so a shared cacheDir across both calls exercises
// the real consume.
func TestPostureAllowOnceConsumedThenWarns(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // fresh package -> would warn

	stateDir := t.TempDir()
	cacheDir := filepath.Join(stateDir, "catalogs")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("mkdir cacheDir: %v", err)
	}
	if err := AddAllowOnce(stateDir, "npm", "once-pkg", "trying once"); err != nil {
		t.Fatalf("AddAllowOnce: %v", err)
	}

	tc := bashInstall("npm install once-pkg@1.0.0")

	// First install consumes the token -> allow.
	dec1, ok := evaluatePosture(context.Background(), tc, config.Config{}, &http.Client{}, cacheDir, postureNow)
	if !ok {
		t.Fatal("first install: ok = false, want true")
	}
	if !dec1.Allow || dec1.Level != "allow" {
		t.Fatalf("first install decision = %+v, want Allow:true Level:allow (one-shot token consumed)", dec1)
	}

	// Second install: token gone -> the fresh package warns again.
	dec2, ok := evaluatePosture(context.Background(), tc, config.Config{}, &http.Client{}, cacheDir, postureNow)
	if !ok {
		t.Fatal("second install: ok = false, want true")
	}
	if !dec2.Allow || dec2.Level != "warn" {
		t.Fatalf("second install decision = %+v, want Allow:true Level:warn (token already consumed, warns again)", dec2)
	}
}
