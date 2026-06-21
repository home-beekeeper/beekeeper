package check

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/catalog"
)

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
