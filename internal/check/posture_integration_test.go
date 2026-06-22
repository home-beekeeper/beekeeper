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

// closedBlockLifecycleConfig opts the lifecycle posture rule UP to block.
func closedBlockLifecycleConfig() config.Config {
	return config.Config{
		FailMode: config.FailModeClosed,
		Posture: &config.PostureConfig{
			Lifecycle: config.PostureRuleConfig{Action: config.PostureActionBlock},
		},
	}
}

// closedBlockRemoteSourceConfig opts the git-remote posture rule UP to block.
func closedBlockRemoteSourceConfig() config.Config {
	return config.Config{
		FailMode: config.FailModeClosed,
		Posture: &config.PostureConfig{
			RemoteSource: config.PostureRuleConfig{Action: config.PostureActionBlock},
		},
	}
}

// TestRunCheckPostureBlockModeBlocksLifecycle is the IPOVR-03 live-path proof for
// the lifecycle rule (test the PATH not the component): with lifecycle opted UP to
// block, a Bash `npm install <pkg>` whose package carries a postinstall lifecycle
// script drives the real RunCheck and must BLOCK (exit 1) with a lifecycle reason +
// audit decision "block". The package is old (release-age clean) so the block is
// attributable to the lifecycle rule alone.
func TestRunCheckPostureBlockModeBlocksLifecycle(t *testing.T) {
	// Old (release-age clean) + a postinstall lifecycle script -> lifecycle fires.
	stubPostureFetchers(t, 100000, false, nil, []string{"postinstall"}, false, nil)

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir) // unsigned editor entry -> no npm catalog match
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-posture-lifecycle-xyz-not-real@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedBlockLifecycleConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitBlock {
		t.Fatalf("ExitCode = %d, want %d (lifecycle opted up to block must block a package with a postinstall script)", res.ExitCode, exitBlock)
	}
	if res.Decision.Allow || res.Decision.Level != "block" {
		t.Fatalf("decision = %+v, want Allow:false Level:block (lifecycle opt-up)", res.Decision)
	}
	if !strings.Contains(strings.ToLower(res.Decision.Reason), "lifecycle") &&
		!strings.Contains(strings.ToLower(res.Decision.Reason), "postinstall") {
		t.Errorf("block reason = %q, want it to mention the lifecycle script", res.Decision.Reason)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "block" {
		t.Fatalf("audit decision = %v, want block; record: %+v", rec["decision"], rec)
	}
}

// TestRunCheckPostureBlockModeBlocksRemoteSource is the IPOVR-03 live-path proof
// for the git-remote rule: with git-remote opted UP to block, a Bash
// `npm install git+https://...` drives the real RunCheck and must BLOCK (exit 1)
// with a remote-source reason + audit decision "block". The git-remote rule is
// parsed entirely from the command string (no registry fetch), so this is
// deterministic with no network. The fetchers are stubbed to fail-soft values to
// prove they are not even consulted for a remote install.
func TestRunCheckPostureBlockModeBlocksRemoteSource(t *testing.T) {
	// Fetchers stubbed to "would warn" values; a remote install never consults them.
	stubPostureFetchers(t, 0, true, nil, nil, true, nil)

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir) // unsigned editor entry -> no npm catalog match
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install git+https://github.com/evil/pkg.git"}}`)

	res := RunCheck(context.Background(), stdin, closedBlockRemoteSourceConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitBlock {
		t.Fatalf("ExitCode = %d, want %d (git-remote opted up to block must block a git install)", res.ExitCode, exitBlock)
	}
	if res.Decision.Allow || res.Decision.Level != "block" {
		t.Fatalf("decision = %+v, want Allow:false Level:block (git-remote opt-up)", res.Decision)
	}
	if !strings.Contains(strings.ToLower(res.Decision.Reason), "git") &&
		!strings.Contains(strings.ToLower(res.Decision.Reason), "source") {
		t.Errorf("block reason = %q, want it to mention the remote/git source", res.Decision.Reason)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "block" {
		t.Fatalf("audit decision = %v, want block; record: %+v", rec["decision"], rec)
	}
}

// TestRunCheckPostureRemoteSourceDefaultWarnsNotBlock is the attribution sibling
// for the git-remote block above: the SAME git install under the DEFAULT (warn)
// config must WARN (exit 0), NOT block. This proves the block in
// TestRunCheckPostureBlockModeBlocksRemoteSource is attributable to the opt-up,
// not to the git-remote rule firing a block on its own (the pure evaluator warns).
func TestRunCheckPostureRemoteSourceDefaultWarnsNotBlock(t *testing.T) {
	stubPostureFetchers(t, 0, true, nil, nil, true, nil)

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install git+https://github.com/evil/pkg.git"}}`)

	// closedConfig() is FailModeClosed with NO Posture block -> git-remote defaults to warn.
	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (a git install warns by default, does not block)", res.ExitCode, exitAllow)
	}
	if !res.Decision.Allow || res.Decision.Level != "warn" {
		t.Fatalf("decision = %+v, want Allow:true Level:warn (default git-remote posture is warn)", res.Decision)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "warn" {
		t.Fatalf("audit decision = %v, want warn; record: %+v", rec["decision"], rec)
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

// allowAlwaysConfig returns a fail-closed config with a posture-scoped
// allow-always entry for pkg (all rules, npm) -- mirrors `beekeeper posture allow
// <pkg> --always`. SECURITY: this is config.Posture.Allow, NOT package_allowlist.
func allowAlwaysConfig(pkg string) config.Config {
	return config.Config{
		FailMode: config.FailModeClosed,
		Posture: &config.PostureConfig{
			Allow: []config.PostureAllow{{Ecosystem: "npm", Package: pkg, Reason: "vetted by operator"}},
		},
	}
}

// TestRunCheckPostureAllowAlwaysAllowsFreshPackage is the IPOVR-02 live-path proof
// (test the PATH not the component): a posture allow-always for a fresh (<24h)
// package makes the LIVE hook ALLOW it (no posture warn), while a non-allowlisted
// fresh package still warns.
func TestRunCheckPostureAllowAlwaysAllowsFreshPackage(t *testing.T) {
	const pkg = "beekeeper-posture-allowalways-fresh-not-real"
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // <24h, no lifecycle scripts

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir) // unsigned editor entry -> no npm catalog match
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install ` + pkg + `@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, allowAlwaysConfig(pkg), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (a posture allow-always for the package must allow)", res.ExitCode, exitAllow)
	}
	if !res.Decision.Allow || res.Decision.Level != "allow" {
		t.Fatalf("decision = %+v, want Allow:true Level:allow (allow-always exempts the fresh package)", res.Decision)
	}
}

// TestRunCheckPostureAllowAlwaysDoesNotBypassCatalogBlock is THE load-bearing
// security-distinction test (T-09-31): a posture allow-always for package X makes
// a <24h X install ALLOW on the posture rules, BUT a CATALOG-blocked X still
// BLOCKS. The posture-scoped allow-always never bypasses malware enforcement.
//
// If allow-always had (wrongly) reused package_allowlist, ApplyPolicyOverlay would
// treat the entry as a user-trust override and DOWNGRADE the catalog block to an
// allow -- this test would then fail (exit 0). It must stay a block (exit non-zero).
func TestRunCheckPostureAllowAlwaysDoesNotBypassCatalogBlock(t *testing.T) {
	const pkg = "beekeeper-posture-allowalways-catalog-not-real"
	// Fresh package: WITHOUT the catalog block, posture would warn; WITH the
	// allow-always, posture allows. The catalog block must still win regardless.
	stubPostureFetchers(t, 10, false, nil, nil, false, nil)

	dir := t.TempDir()
	idxPath := buildBlockingNPMIndex(t, dir, pkg) // signed critical npm entry -> catalog block
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install ` + pkg + `@1.0.0"}}`)

	// allow-always for the SAME package X.
	res := RunCheck(context.Background(), stdin, allowAlwaysConfig(pkg), idxPath, auditPath, t.TempDir())

	if res.ExitCode != exitBlock {
		t.Fatalf("ExitCode = %d, want %d -- a posture allow-always must NOT bypass a catalog malware block (T-09-31)", res.ExitCode, exitBlock)
	}
	if res.Decision.Allow || res.Decision.Level != "block" {
		t.Fatalf("decision = %+v, want a block (catalog enforcement is untouched by posture allow-always)", res.Decision)
	}

	rec := readPolicyDecisionRecord(t, auditPath)
	if rec["decision"] != "block" {
		t.Fatalf("audit decision = %v, want block; record: %+v", rec["decision"], rec)
	}
}

// TestRunCheckPostureAllowOnceConsumedThenWarns is the IPOVR-01 live-path proof: a
// recorded one-shot token allows the NEXT matching install on the live hook, and
// the SUBSEQUENT identical install warns again (token consumed). A shared cacheDir
// across both RunCheck calls exercises the real on-disk consume.
func TestRunCheckPostureAllowOnceConsumedThenWarns(t *testing.T) {
	const pkg = "beekeeper-posture-allowonce-not-real"
	stubPostureFetchers(t, 30, false, nil, nil, false, nil) // fresh package -> would warn

	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)

	stateDir := t.TempDir()
	cacheDir := filepath.Join(stateDir, "catalogs")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatalf("mkdir cacheDir: %v", err)
	}
	if err := AddAllowOnce(stateDir, "npm", pkg, "trying once"); err != nil {
		t.Fatalf("AddAllowOnce: %v", err)
	}

	cmdJSON := `{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install ` + pkg + `@1.0.0"}}`

	// First install consumes the one-shot token -> allow.
	res1 := RunCheck(context.Background(), strings.NewReader(cmdJSON), closedConfig(), idxPath, auditPathIn(t), cacheDir)
	if res1.ExitCode != exitAllow {
		t.Fatalf("first install ExitCode = %d, want %d (one-shot token allows)", res1.ExitCode, exitAllow)
	}
	if res1.Decision.Level != "allow" {
		t.Fatalf("first install Level = %q, want allow (one-shot token consumed); decision: %+v", res1.Decision.Level, res1.Decision)
	}

	// Second install: token gone -> the fresh package warns again.
	res2 := RunCheck(context.Background(), strings.NewReader(cmdJSON), closedConfig(), idxPath, auditPathIn(t), cacheDir)
	if res2.ExitCode != exitAllow {
		t.Fatalf("second install ExitCode = %d, want %d (warn does not block)", res2.ExitCode, exitAllow)
	}
	if res2.Decision.Level != "warn" {
		t.Fatalf("second install Level = %q, want warn (token already consumed); decision: %+v", res2.Decision.Level, res2.Decision)
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
