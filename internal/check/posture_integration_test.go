package check

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/catalog"
	"github.com/home-beekeeper/beekeeper/internal/config"
)

// closedBlockReleaseAgeConfig is a fail-closed config that opts the release-age
// posture rule UP to block (IPOVR-03). Used to prove the LIVE hook blocks a
// definite release-age violation while the unknown path stays fail-soft warn.
func closedBlockReleaseAgeConfig() config.Config {
	return config.Config{
		FailMode: config.FailModeClosed,
		Posture: &config.PostureConfig{
			ReleaseAge: config.PostureRuleConfig{Action: config.PostureActionBlock},
		},
	}
}

// buildBlockingNPMIndex writes an index with a SIGNED critical entry for an npm
// package so that a Bash `npm install <pkg>` triggers a genuine catalog BLOCK
// (signedCount:1 satisfies critical.BlockAt:1). This lets the integration test
// prove a posture WARN cannot downgrade a catalog block on the live hook path.
//
// The package name is fictional so a live OSV query (npm IS an OSV ecosystem)
// returns no extra match - the block comes from the signed bumblebee entry alone.
func buildBlockingNPMIndex(t *testing.T, dir, pkg string) string {
	t.Helper()
	entries := []catalog.Entry{
		{
			ID:               "beekeeper-test-2026-06-21-npm-posture-block",
			Name:             "posture integration blocking entry",
			Ecosystem:        "npm",
			Package:          pkg,
			Versions:         []string{"1.0.0"},
			Severity:         "critical",
			CatalogSource:    "bumblebee",
			CatalogSignature: "test-sig-present", // non-empty → Signed:true → block at critical.BlockAt:1
		},
	}
	idxPath := filepath.Join(dir, "bumblebee-npm-blocking.idx")
	if err := catalog.BuildIndex(idxPath, entries); err != nil {
		t.Fatalf("BuildIndex (npm blocking): %v", err)
	}
	return idxPath
}

// readPolicyDecisionRecord scans the NDJSON audit log and returns the first
// policy_decision record as a generic map.
func readPolicyDecisionRecord(t *testing.T, auditPath string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log not written: %v", err)
	}
	for _, raw := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			t.Fatalf("audit record not valid JSON: %v\nline: %s", err, raw)
		}
		if rec["record_type"] == "policy_decision" {
			return rec
		}
	}
	t.Fatalf("no policy_decision record in audit log:\n%s", string(data))
	return nil
}

// TestRunCheckPostureFreshPackageWarns is the live-path proof (test the PATH not
// the component): a Bash `npm install <fresh-pkg>` drives the real RunCheck with
// the posture fetchers stubbed to a <24h package + no lifecycle scripts. The
// returned decision AND the audit record must be a WARN with exit code 0 - a
// warn does not block.
func TestRunCheckPostureFreshPackageWarns(t *testing.T) {
	// <24h-old package, no lifecycle scripts. No network: seams short-circuit the
	// registry. (Posture fetchers are the only registry I/O on this path; OSV for
	// a fictional npm name returns no match.)
	stubPostureFetchers(t, 30, false, nil, nil, false, nil)

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir) // unsigned editor-extension entry → no npm match
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-posture-fresh-xyz-not-real@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (a posture warn does not block)", res.ExitCode, exitAllow)
	}
	if !res.Decision.Allow {
		t.Fatalf("Allow = false, want true (warn); decision: %+v", res.Decision)
	}
	if res.Decision.Level != "warn" {
		t.Fatalf("Level = %q, want warn (fresh package); decision: %+v", res.Decision.Level, res.Decision)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "warn" {
		t.Fatalf("audit decision = %v, want warn; record: %+v", rec["decision"], rec)
	}
}

// TestRunCheckPostureMissingTimestampWarnsNotBlock proves the fail-soft
// divergence on the LIVE path: a missing publish timestamp must produce a WARN
// (exit 0), NOT a fail-closed block, even under FailModeClosed.
func TestRunCheckPostureMissingTimestampWarnsNotBlock(t *testing.T) {
	stubPostureFetchers(t, 0, true, nil, nil, false, nil) // timestamp missing

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-posture-missing-xyz-not-real@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (missing timestamp must WARN, not block)", res.ExitCode, exitAllow)
	}
	if !res.Decision.Allow {
		t.Fatalf("Allow = false, want true (fail-soft warn); decision: %+v", res.Decision)
	}
	if res.Decision.Level == "block" {
		t.Fatalf("Level = block, want warn (fail-soft on missing timestamp); decision: %+v", res.Decision)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] == "block" {
		t.Fatalf("audit decision = block, want a non-block warn (fail-soft); record: %+v", rec)
	}
}

// TestRunCheckPostureCannotDowngradeCatalogBlock proves the most-restrictive
// merge on the LIVE path: a catalog-blocked npm package still BLOCKS even though
// the posture rules (stubbed fresh + lifecycle scripts) would WARN. Posture can
// never downgrade a block.
func TestRunCheckPostureCannotDowngradeCatalogBlock(t *testing.T) {
	const blockedPkg = "beekeeper-posture-blocked-pkg-not-real"
	// Stub posture to "fired" results (fresh + lifecycle scripts) - these are
	// warns. If posture could downgrade, the catalog block would become a warn.
	stubPostureFetchers(t, 10, false, nil, []string{"postinstall"}, false, nil)

	dir := t.TempDir()
	idxPath := buildBlockingNPMIndex(t, dir, blockedPkg)
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install ` + blockedPkg + `@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitBlock {
		t.Fatalf("ExitCode = %d, want %d (catalog block must survive a posture warn)", res.ExitCode, exitBlock)
	}
	if res.Decision.Allow {
		t.Fatalf("Allow = true, want false (catalog block); decision: %+v", res.Decision)
	}
	if res.Decision.Level != "block" {
		t.Fatalf("Level = %q, want block (posture warn cannot downgrade); decision: %+v", res.Decision.Level, res.Decision)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "block" {
		t.Fatalf("audit decision = %v, want block; record: %+v", rec["decision"], rec)
	}
}

// TestRunCheckPostureBlockModeBlocksFreshPackage is the IPOVR-03 live-path proof
// (test the PATH not the component): with release-age opted UP to block, a Bash
// `npm install <fresh-pkg>` drives the real RunCheck and must BLOCK (exit non-zero)
// on a DEFINITE <24h violation. This is the new opt-up capability.
func TestRunCheckPostureBlockModeBlocksFreshPackage(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // <24h, definite violation

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir) // unsigned editor entry -> no npm catalog match
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-posture-blockmode-xyz-not-real@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedBlockReleaseAgeConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitBlock {
		t.Fatalf("ExitCode = %d, want %d (release-age opted up to block must block a <24h package)", res.ExitCode, exitBlock)
	}
	if res.Decision.Allow {
		t.Fatalf("Allow = true, want false (block mode); decision: %+v", res.Decision)
	}
	if res.Decision.Level != "block" {
		t.Fatalf("Level = %q, want block; decision: %+v", res.Decision.Level, res.Decision)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "block" {
		t.Fatalf("audit decision = %v, want block; record: %+v", rec["decision"], rec)
	}
}

// TestRunCheckPostureBlockModeMissingTimestampStillWarns proves the IPOVR-03
// fail-soft invariant on the LIVE path: EVEN with release-age opted up to block, a
// MISSING publish timestamp (unknown input) must WARN (exit 0), not block. A
// registry outage cannot break a build even under block mode.
func TestRunCheckPostureBlockModeMissingTimestampStillWarns(t *testing.T) {
	stubPostureFetchers(t, 0, true, nil, nil, false, nil) // timestamp missing (unknown)

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-posture-blockmode-missing-xyz-not-real@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedBlockReleaseAgeConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (unknown stays fail-soft warn even under block mode)", res.ExitCode, exitAllow)
	}
	if !res.Decision.Allow {
		t.Fatalf("Allow = false, want true (fail-soft warn); decision: %+v", res.Decision)
	}
	if res.Decision.Level == "block" {
		t.Fatalf("Level = block, want warn (unknown stays fail-soft even under block mode); decision: %+v", res.Decision)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] == "block" {
		t.Fatalf("audit decision = block, want a non-block warn (fail-soft unknown); record: %+v", rec)
	}
}

// TestRunCheckPostureBlockModeCannotDowngradeCatalogBlock proves most-restrictive
// merge still holds with block mode active: a catalog block + a posture block both
// fire; the result is a block whose reason is the CATALOG reason (the catalog block
// is merged first and a posture block cannot downgrade it). Asserts both block and
// the catalog rule survives.
func TestRunCheckPostureBlockModeCannotDowngradeCatalogBlock(t *testing.T) {
	const blockedPkg = "beekeeper-posture-blockmode-catalog-not-real"
	stubPostureFetchers(t, 10, false, nil, nil, false, nil) // fresh -> posture release-age fires (block under this cfg)

	dir := t.TempDir()
	idxPath := buildBlockingNPMIndex(t, dir, blockedPkg)
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install ` + blockedPkg + `@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedBlockReleaseAgeConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitBlock {
		t.Fatalf("ExitCode = %d, want %d (catalog block must hold)", res.ExitCode, exitBlock)
	}
	if res.Decision.Allow || res.Decision.Level != "block" {
		t.Fatalf("decision = %+v, want a block (catalog wins, posture cannot downgrade)", res.Decision)
	}
	// The catalog block is merged BEFORE posture; most-restrictive merge keeps the
	// first block's reason, so the catalog block (not the posture block) is surfaced.
	if !strings.Contains(strings.ToLower(res.Decision.Reason), "catalog") &&
		!strings.Contains(strings.ToLower(res.Decision.Reason), "corrobor") &&
		!strings.Contains(strings.ToLower(res.Decision.Reason), "block") {
		t.Errorf("block reason = %q, expected the catalog block reason to win", res.Decision.Reason)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "block" {
		t.Fatalf("audit decision = %v, want block; record: %+v", rec["decision"], rec)
	}
}

// TestRunCheckShimShapeEnforcesPosture proves the shim now actually enforces.
// A shim invocation (beekeeper check --tool npm --args install <pkg>) is
// reconstructed by buildShimToolCall (cmd/beekeeper) into exactly this shape:
// tool_name "Bash" with the FULL command string. This test drives that exact
// shape through the live RunCheck and asserts posture fires (a fresh package
// warns), closing the latent gap where the old {tool_name:"execute",
// command:"npm", args:[...]} shape parsed no install and allowed everything.
func TestRunCheckShimShapeEnforcesPosture(t *testing.T) {
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // <24h, no lifecycle scripts

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)
	// The exact JSON buildShimToolCall produces for: shim npm install <pkg>.
	stdin := strings.NewReader(`{"agent_name":"shim","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-shim-fresh-xyz-not-real@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (shim posture warn does not block)", res.ExitCode, exitAllow)
	}
	if res.Decision.Level != "warn" {
		t.Fatalf("Level = %q, want warn (shim-intercepted fresh install must be evaluated, not allowed silently)", res.Decision.Level)
	}
}
