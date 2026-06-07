package check

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/nudge"
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

	// SPATH-01/02/03 + HARDEN-01: sensitive-path evaluation — after overlay and
	// before NUDGE, so a path block is never downgraded by the overlay
	// (CR-02 ordering mirrors production runCheck). Iterates BOTH canonical forms
	// via canonicalizePathForms (HARDEN-01) and blocks on any form, in lockstep
	// with the handler.go SPATH loop so the test mirror stays faithful to production.
	spathCfg := policy.DefaultSensitivePaths()
	for _, rawPath := range extractPathTargets(toolCall) {
		for _, resolved := range canonicalizePathForms(rawPath) {
			if resolved == "" {
				continue
			}
			pathDecision := policy.EvaluatePath(resolved, spathCfg)
			decision = mergeDecisions(decision, pathDecision)
		}
	}

	// SELF-PROTECTION mirror (lockstep with handler.go) — state dir (read+write) +
	// binary (write) + content-aware hook-entry guard + CLI-mutation guard.
	selfCfg := buildSelfProtectConfig()
	for _, t := range extractTypedTargets(toolCall) {
		for _, resolved := range canonicalizePathForms(t.path) {
			if resolved == "" {
				continue
			}
			decision = mergeDecisions(decision, policy.EvaluateSelfPath(resolved, t.isWrite, selfCfg))
		}
	}
	decision = mergeDecisions(decision, evaluateHookGuard(toolCall))
	decision = mergeDecisions(decision, evaluateCLIGuard(toolCall))

	// NUDGE-03/04/08: package-manager nudge evaluation — mirrors the production
	// runCheck nudge block (handler.go) so runCheckWithIndex exercises the live
	// nudge wiring. Runs AFTER overlay and SPATH (CR-02 ordering). Detection is
	// resolved via nudge.DetectStateFn (the EXPORTED seam) so BTEST-02 tests can
	// inject a synthetic PMState via defer-restore across the package boundary.
	nudgeCfgValue := config.DefaultNudgeConfig()
	if cfg.Nudge != nil {
		nudgeCfgValue = *cfg.Nudge
	}
	if nudgeDecision, nudgeRec, nudgeOK := evaluateNudge(ctx, toolCall, nudgeCfgValue); nudgeOK {
		decision = mergeDecisions(decision, nudgeDecision)
		writeNudgeAuditRecord(auditPath, nudgeRec)
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

// readAuditRecordByType reads all NDJSON records from auditPath and returns the
// last record whose RecordType matches wantType. Returns the zero AuditRecord
// (RecordType=="") if no matching record is found.
func readAuditRecordByType(t *testing.T, auditPath, wantType string) audit.AuditRecord {
	t.Helper()
	f, err := os.Open(auditPath)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()

	var matched audit.AuditRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec audit.AuditRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("parse audit NDJSON line: %v", err)
		}
		if rec.RecordType == wantType {
			matched = rec
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit file: %v", err)
	}
	return matched
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

// TestIntegrationNudgePnpmAddEvilPkg verifies that a pnpm add of a catalog-malicious
// package (keyed "npm::evil-pkg") is BLOCKED end-to-end through the live runCheckWithIndex
// nudge path (BTEST-02 case (a) / F3 end-to-end / NUDGE-01 SC1).
//
// pnpm/bun/yarn install commands map to ecosystem "npm" in pkgparse (F3/SC1) so
// LookupAll("npm", "evil-pkg") matches the catalog entry. No DetectStateFn injection
// is needed — the block comes from the catalog match, not nudge detection.
func TestIntegrationNudgePnpmAddEvilPkg(t *testing.T) {
	// Seed the fake index with two signed sources at "npm::evil-pkg" to trigger a
	// two-source corroboration block (pnpm maps ecosystem to "npm" — SC1/F3).
	idx := &mapMultiIndex{
		matchesByKey: map[string][]policy.CatalogMatch{
			"npm::evil-pkg": {
				{
					CatalogSource: "bumblebee",
					EntryID:       "bb-evil-pkg-001",
					Ecosystem:     "npm",
					Package:       "evil-pkg",
					Version:       "1.0.0",
					Severity:      "critical",
					Signed:        true,
				},
				{
					CatalogSource: "osv",
					EntryID:       "osv-evil-pkg-001",
					Ecosystem:     "npm",
					Package:       "evil-pkg",
					Version:       "1.0.0",
					Severity:      "critical",
					Signed:        true,
				},
			},
		},
	}

	toolCallJSON := `{"agent_name":"test-agent","tool_name":"Bash","tool_input":{"command":"pnpm add evil-pkg"}}`
	auditPath := auditPathIn(t)

	res := runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON), closedConfig(), idx, auditPath)

	// Two signed sources must block (PLCY-01 / F3 end-to-end).
	if res.ExitCode == exitAllow {
		t.Errorf("ExitCode = %d (allow), want non-zero; pnpm add evil-pkg with 2 catalog sources must block (F3/SC1)", res.ExitCode)
	}
	if res.Decision.Level != "block" {
		t.Errorf("Level = %q, want %q", res.Decision.Level, "block")
	}
	if res.Decision.CorroborationCount < 2 {
		t.Errorf("CorroborationCount = %d, want >= 2; both signed sources must be counted", res.Decision.CorroborationCount)
	}

	// Verify the final audit record carries block decision.
	rec := readLastAuditRecord(t, auditPath)
	if rec.Decision != "block" {
		t.Errorf("audit Decision = %q, want %q", rec.Decision, "block")
	}
}

// TestIntegrationNudgeSoftAdvisory verifies the soft advisory path for a pnpm install
// command when pnpm >= 11 is injected via the EXPORTED nudge.DetectStateFn seam
// (BTEST-02 case (b) / NUDGE-03 live wiring).
//
// The DetectStateFn is swapped with a stub that returns a synthetic pnpm-11 PMState;
// the original is defer-restored. This is the cross-package injection pattern —
// the unexported pnpmVersionFn/nodeVersionFn CANNOT be assigned from package check.
//
// Expected: exit 0 (soft advisory = warn level), audit record_type "nudge",
// nudge_action "advise", reason_code "pnpm-available-soft".
func TestIntegrationNudgeSoftAdvisory(t *testing.T) {
	// Inject a synthetic pnpm-11 PMState via the EXPORTED seam (T-08-10b).
	orig := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.5.1",
			NodeVersion:   "22.5.0",
			PnpmHardened:  true,
		}
	}
	defer func() { nudge.DetectStateFn = orig }()

	// Empty catalog index: catalog match does not block; only nudge fires.
	idx := &mapMultiIndex{matchesByKey: map[string][]policy.CatalogMatch{}}

	// npm install foo — an UNHARDENED install command; pnpm is installed and
	// hardened → soft mode → Advise / pnpm-available-soft. (A `pnpm install`
	// command would instead Proceed/already-hardened-pm — you don't nudge
	// pnpm→pnpm — so the advisory case must use an npm/yarn command.)
	toolCallJSON := `{"agent_name":"test-agent","tool_name":"Bash","tool_input":{"command":"npm install foo"}}`
	auditPath := auditPathIn(t)

	res := runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON), closedConfig(), idx, auditPath)

	// Soft advisory: exit 0 (warn level does not block — NUDGE-03).
	if res.ExitCode != exitAllow {
		t.Errorf("ExitCode = %d, want 0; soft advisory must not block (NUDGE-03 / BTEST-02 case (b))", res.ExitCode)
	}

	// The nudge audit record must carry record_type "nudge" and the closed §9 reason.
	nudgeRec := readAuditRecordByType(t, auditPath, "nudge")
	if nudgeRec.RecordType == "" {
		t.Fatalf("no record_type=nudge audit record found; nudge wiring must write §9 record (BTEST-02 case (b))")
	}
	if nudgeRec.NudgeAction != "advise" {
		t.Errorf("nudge_action = %q, want %q (pnpm-available-soft → Advise)", nudgeRec.NudgeAction, "advise")
	}
	wantReason := nudge.ReasonPnpmAvailableSoft
	if nudgeRec.ReasonCode != wantReason {
		t.Errorf("reason_code = %q, want %q", nudgeRec.ReasonCode, wantReason)
	}
}

// TestIntegrationNudgeNonInstallSkipped verifies that a non-install Bash command
// (e.g. "npm ls") does NOT emit a nudge audit record (BTEST-02 case (c) / §10-7).
//
// Non-install commands must NEVER trigger nudge detection or write a nudge record.
// No DetectStateFn injection needed — the skip happens before detection.
func TestIntegrationNudgeNonInstallSkipped(t *testing.T) {
	// No catalog entries; non-install commands should not be nudged.
	idx := &mapMultiIndex{matchesByKey: map[string][]policy.CatalogMatch{}}

	// "npm ls" is NOT an install command (§10-7); nudge must not fire.
	toolCallJSON := `{"agent_name":"test-agent","tool_name":"Bash","tool_input":{"command":"npm ls"}}`
	auditPath := auditPathIn(t)

	res := runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON), closedConfig(), idx, auditPath)

	// Non-install must allow (no catalog match, no nudge block).
	if res.ExitCode != exitAllow {
		t.Errorf("ExitCode = %d, want 0; non-install npm ls must not block", res.ExitCode)
	}

	// Confirm no nudge audit record was written (§10-7 / Pitfall 2).
	rec := readAuditRecordByType(t, auditPath, "nudge")
	if rec.RecordType == "nudge" {
		t.Errorf("unexpected record_type=nudge record found for non-install command (§10-7 violation)")
	}
}

// TestIntegrationAncestorSymlinkCredentialBlocks is the HARDEN-01 end-to-end
// regression: a credential read THROUGH an ancestor-directory symlink must block
// through the live check pipeline (runCheckWithIndex), proving the dual-form
// canonicalizePathForms wiring landed at the call site (Plan 03 / HARDEN-01).
//
// Attack shape: plant a symlink L -> realDir, then `cat L/.aws/credentials`.
// With the pre-fix single-form canonicalizePath, EvalSymlinks resolves the L
// ancestor and can rewrite the path under realDir (stripping the matchable
// /.aws/ shape); the lexical form preserves /.aws/ so a block on EITHER form
// blocks. This test asserts the BLOCK reaches the Result/exit code, not just the
// helper (which is unit-tested in paths_test.go).
//
// Empty catalog index → no catalog match; the block must come SOLELY from the
// SPATH dual-form evaluation. Skips cleanly when os.Symlink is unprivileged
// (unprivileged Windows dev box); CI Linux/macOS exercise it.
func TestIntegrationAncestorSymlinkCredentialBlocks(t *testing.T) {
	// Real target directory with a real .aws/credentials so EvalSymlinks can fully
	// resolve the ancestor symlink (making the single-form path lose /.aws/ shape).
	realDir := t.TempDir()
	awsDir := filepath.Join(realDir, ".aws")
	if err := os.MkdirAll(awsDir, 0o755); err != nil {
		t.Fatalf("mkdir .aws: %v", err)
	}
	if err := os.WriteFile(filepath.Join(awsDir, "credentials"), []byte("[default]\n"), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	linkPath := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realDir, linkPath); err != nil {
		if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "privilege") {
			t.Skipf("os.Symlink requires privilege on this host, skipping: %v", err)
		}
		t.Fatalf("os.Symlink: %v", err)
	}

	// Read the credential file THROUGH the ancestor symlink via a Bash read verb.
	target := filepath.ToSlash(linkPath) + "/.aws/credentials"
	toolCallJSON := `{"agent_name":"test-agent","tool_name":"Bash","tool_input":{"command":"cat ` + target + `"}}`

	// Empty index — the block must come from the SPATH dual-form evaluation alone.
	idx := &mapMultiIndex{matchesByKey: map[string][]policy.CatalogMatch{}}
	auditPath := auditPathIn(t)

	res := runCheckWithIndex(context.Background(), strings.NewReader(toolCallJSON), closedConfig(), idx, auditPath)

	if res.Decision.Allow {
		t.Errorf("Allow = true, want false — ancestor-symlink credential read must block end-to-end (HARDEN-01)")
	}
	if res.Decision.Level != "block" {
		t.Errorf("Level = %q, want block — dual-form SPATH must block the symlink-ancestor credential read (HARDEN-01)", res.Decision.Level)
	}
	if res.ExitCode == exitAllow {
		t.Errorf("ExitCode = %d (allow), want non-zero (HARDEN-01 block must exit 1)", res.ExitCode)
	}

	// The final audit record must reflect the block.
	rec := readLastAuditRecord(t, auditPath)
	if rec.Decision != "block" {
		t.Errorf("audit Decision = %q, want block (HARDEN-01 end-to-end regression)", rec.Decision)
	}
}
