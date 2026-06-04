package check

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/policyloader"
)

// mapMultiIndex implements catalogIndex for hermetic integration tests. It
// returns pre-canned policy.CatalogMatch slices keyed by "ecosystem::pkg"
// without any disk or network access, allowing full end-to-end testing of the
// check handler with controlled multi-source corroboration scenarios.
type mapMultiIndex struct {
	matchesByKey map[string][]policy.CatalogMatch
}

func (f *mapMultiIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	return f.matchesByKey[ecosystem+"::"+pkg]
}

func (f *mapMultiIndex) Close() error { return nil }

// runCheckWithIndex is the testable variant of runCheck that accepts a
// pre-built catalogIndex instead of opening one from disk and wiring network
// adapters. The stdin decoding, size cap, timeout, fail-closed panic guard,
// and finalize path are identical to the production runCheck path.
func runCheckWithIndex(ctx context.Context, stdin io.Reader, cfg config.Config, idx catalogIndex, auditPath string) (result Result) {
	debug.SetMemoryLimit(memLimit)

	var toolCall policy.ToolCall

	defer func() {
		if r := recover(); r != nil {
			d := failDecision(cfg, "internal error (fail-closed)")
			result = finalize(d, cfg, toolCall, auditPath)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	limited := &io.LimitedReader{R: stdin, N: maxStdin + 1}
	dec := json.NewDecoder(limited)
	if err := dec.Decode(&toolCall); err != nil {
		if limited.N <= 0 {
			return finalize(failDecision(cfg, "stdin exceeds 1MB cap (fail-closed)"), cfg, toolCall, auditPath)
		}
		return finalize(failDecision(cfg, "invalid tool call JSON (fail-closed)"), cfg, toolCall, auditPath)
	}
	if limited.N <= 0 {
		return finalize(failDecision(cfg, "stdin exceeds 1MB cap (fail-closed)"), cfg, toolCall, auditPath)
	}
	if ctx.Err() != nil {
		return finalize(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath)
	}

	decision := policy.Evaluate(toolCall, idx, policy.DefaultCorroborationThresholds(), policy.AgentContext{})

	// Apply overlay with empty policyFiles (no-op here; mirrors production ordering
	// from runCheck so that runCheckWithIndex runs overlay before path evaluation,
	// preventing a package_allowlist allow from downgrading a later path block).
	// Tests that need a real overlay use RunCheck directly (CR-02 regression).
	decision = policyloader.ApplyPolicyOverlay(nil, toolCall, decision)

	// SPATH-01/02/03: sensitive-path evaluation — LAST, after overlay,
	// so a path block is the final word (CR-02 fix mirrors production runCheck).
	spathCfg := policy.DefaultSensitivePaths()
	for _, rawPath := range extractPathTargets(toolCall) {
		resolved := canonicalizePath(rawPath)
		if resolved == "" {
			continue
		}
		pathDecision := policy.EvaluatePath(resolved, spathCfg)
		decision = mergeDecisions(decision, pathDecision)
	}

	if ctx.Err() != nil {
		return finalize(failDecision(cfg, "execution timeout (fail-closed)"), cfg, toolCall, auditPath)
	}

	return finalize(decision, cfg, toolCall, auditPath)
}

// readLastAuditRecord reads and returns the last NDJSON record from auditPath.
func readLastAuditRecord(t *testing.T, auditPath string) audit.AuditRecord {
	t.Helper()
	f, err := os.Open(auditPath)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()

	var last audit.AuditRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &last); err != nil {
			t.Fatalf("parse audit NDJSON line: %v", err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit file: %v", err)
	}
	return last
}

// TestIntegrationTwoSourceBlock verifies the end-to-end two-source corroboration
// block path: when two independent signed catalog sources agree on a package,
// the check handler must exit non-zero and record corroboration_count >= 2 in
// the audit NDJSON (PLCY-01, CTLG-09).
func TestIntegrationTwoSourceBlock(t *testing.T) {
	idx := &mapMultiIndex{
		matchesByKey: map[string][]policy.CatalogMatch{
			"npm::evil-pkg-integration-test-xyz": {
				{
					CatalogSource: "bumblebee",
					EntryID:       "bb-evil-xyz",
					Ecosystem:     "npm",
					Package:       "evil-pkg-integration-test-xyz",
					Version:       "1.0.0",
					Severity:      "critical",
					Signed:        true,
				},
				{
					CatalogSource: "osv",
					EntryID:       "osv-evil-xyz",
					Ecosystem:     "npm",
					Package:       "evil-pkg-integration-test-xyz",
					Version:       "1.0.0",
					Severity:      "critical",
					Signed:        true,
				},
			},
		},
	}

	toolCallJSON := `{"agent_name":"test-agent","tool_name":"Bash","tool_input":{"command":"npm install evil-pkg-integration-test-xyz@1.0.0"}}`
	auditPath := auditPathIn(t)

	res := runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON), closedConfig(), idx, auditPath)

	if res.ExitCode == exitAllow {
		t.Errorf("ExitCode = %d (allow), want non-zero; two signed sources must block (PLCY-01)", res.ExitCode)
	}
	if res.Decision.Level != "block" {
		t.Errorf("Level = %q, want %q", res.Decision.Level, "block")
	}
	if res.Decision.CorroborationCount < 2 {
		t.Errorf("CorroborationCount = %d, want >= 2; both signed sources must be counted", res.Decision.CorroborationCount)
	}

	// Verify the audit NDJSON record carries the full corroboration provenance (CTLG-09).
	rec := readLastAuditRecord(t, auditPath)
	if rec.CorroborationCount < 2 {
		t.Errorf("audit CorroborationCount = %d, want >= 2", rec.CorroborationCount)
	}
	wantSources := map[string]bool{"bumblebee": true, "osv": true}
	for _, src := range rec.SourcesAgreed {
		delete(wantSources, src)
	}
	if len(wantSources) > 0 {
		t.Errorf("audit SourcesAgreed = %v; missing expected sources: %v", rec.SourcesAgreed, wantSources)
	}
}

// TestIntegrationSingleSourceWarn verifies the single-source warn path: one
// signed source produces a warn-level decision that exits 0 (allow) and
// records corroboration_count == 1 (PLCY-01, CTLG-09).
func TestIntegrationSingleSourceWarn(t *testing.T) {
	idx := &mapMultiIndex{
		matchesByKey: map[string][]policy.CatalogMatch{
			"npm::semi-sus-pkg-integration-test-xyz": {
				{
					CatalogSource: "bumblebee",
					EntryID:       "bb-semi-sus-xyz",
					Ecosystem:     "npm",
					Package:       "semi-sus-pkg-integration-test-xyz",
					Version:       "1.0.0",
					Severity:      "high",
					Signed:        true,
				},
			},
		},
	}

	toolCallJSON := `{"agent_name":"test-agent","tool_name":"Bash","tool_input":{"command":"npm install semi-sus-pkg-integration-test-xyz@1.0.0"}}`
	auditPath := auditPathIn(t)

	res := runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON), closedConfig(), idx, auditPath)

	if res.ExitCode != exitAllow {
		t.Errorf("ExitCode = %d, want 0; single signed source is warn (exit 0, PLCY-01)", res.ExitCode)
	}
	if res.Decision.Level != "warn" {
		t.Errorf("Level = %q, want %q", res.Decision.Level, "warn")
	}
	if res.Decision.CorroborationCount != 1 {
		t.Errorf("CorroborationCount = %d, want 1", res.Decision.CorroborationCount)
	}

	// Verify the audit NDJSON record.
	rec := readLastAuditRecord(t, auditPath)
	if rec.CorroborationCount != 1 {
		t.Errorf("audit CorroborationCount = %d, want 1", rec.CorroborationCount)
	}
	if len(rec.SourcesAgreed) != 1 || rec.SourcesAgreed[0] != "bumblebee" {
		t.Errorf("audit SourcesAgreed = %v, want [bumblebee]", rec.SourcesAgreed)
	}
}
