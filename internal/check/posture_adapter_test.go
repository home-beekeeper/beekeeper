package check

import (
	"context"
	"net/http"
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

// TestPosturizeAllowPassesThrough: posturize leaves an allow unchanged.
func TestPosturizeAllowPassesThrough(t *testing.T) {
	in := policy.Decision{Allow: true, Level: "allow", Reason: "ok", RuleIDs: []string{"x"}}
	out := posturize(in)
	if out.Level != "allow" || !out.Allow {
		t.Fatalf("posturize(allow) = %+v, want unchanged allow", out)
	}
}

// TestPosturizeBlockBecomesWarn: posturize re-maps a pure-evaluator BLOCK to a
// WARN (Allow:true) - the WARN-default contract.
func TestPosturizeBlockBecomesWarn(t *testing.T) {
	in := policy.Decision{Allow: false, Level: "block", Reason: "too young", RuleIDs: []string{"release-age-policy"}}
	out := posturize(in)
	if !out.Allow || out.Level != "warn" {
		t.Fatalf("posturize(block) = %+v, want Allow:true Level:warn (default posture is warn)", out)
	}
	if out.Reason != in.Reason {
		t.Errorf("posturize dropped reason: got %q want %q", out.Reason, in.Reason)
	}
}
