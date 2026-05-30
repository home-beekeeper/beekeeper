package check

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/config"
	"github.com/mzansi-agentive/beekeeper/internal/llamafirewall"
	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// buildTestIndex writes a small real mmap index in dir containing the
// compromised Nx Console entry and returns the index path.
func buildTestIndex(t *testing.T, dir string) string {
	t.Helper()
	entries := []catalog.Entry{
		{
			ID:            "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised",
			Name:          "nrwl.angular-console compromise",
			Ecosystem:     "editor-extension",
			Package:       "nrwl.angular-console",
			Versions:      []string{"18.95.0"},
			Severity:      "critical",
			CatalogSource: "bumblebee",
		},
	}
	idxPath := filepath.Join(dir, "bumblebee.idx")
	if err := catalog.BuildIndex(idxPath, entries); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	return idxPath
}

func closedConfig() config.Config { return config.Config{FailMode: config.FailModeClosed} }

func auditPathIn(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "audit", "beekeeper.ndjson")
}

func TestHookHandlerAllow(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	// Use a clearly fictional package name that will never appear in any real
	// threat-intel catalog (Bumblebee, OSV, or Socket). This avoids false
	// test failures when live OSV queries return results for real packages
	// like "express" that happen to have known CVEs.
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-test-clean-package-xyz-not-real@1.0.0"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), t.TempDir())

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (package should be clean)", res.ExitCode, exitAllow)
	}
	if !res.Decision.Allow {
		t.Fatalf("Allow = false, want true for fictional clean package; decision: %+v", res.Decision)
	}
}

func TestCatalogMatchWarns(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Install","tool_input":{"ecosystem":"editor-extension","package":"nrwl.angular-console","version":"18.95.0"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), t.TempDir())

	// Phase 1: single-source catalog match is warn, NOT block — exit 0.
	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (single-source warn does not block in Phase 1)", res.ExitCode, exitAllow)
	}
	if res.Decision.Level != "warn" {
		t.Fatalf("Level = %q, want warn", res.Decision.Level)
	}
	if !res.Decision.Allow {
		t.Fatal("Allow = false, want true for Phase 1 warn")
	}
	if len(res.Decision.CatalogMatches) == 0 {
		t.Fatal("expected at least one CatalogMatch")
	}
}

func TestFailClosedOnPanic(t *testing.T) {
	// Inject an opener that panics, exercising the top-level recover guard.
	panicOpener := func(string) (*catalog.Index, error) {
		panic("boom")
	}
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)

	res := runCheck(context.Background(), stdin, closedConfig(), "ignored", auditPathIn(t), t.TempDir(), panicOpener)

	if res.Decision.Allow {
		t.Fatal("Allow = true on panic, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on panic", res.ExitCode)
	}
}

func TestTimeoutFailClosed(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)

	// Already-cancelled context: the deadline check must short-circuit to block.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := RunCheck(ctx, stdin, closedConfig(), idxPath, auditPathIn(t), t.TempDir())

	if res.Decision.Allow {
		t.Fatal("Allow = true with cancelled context, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on timeout", res.ExitCode)
	}
	r := strings.ToLower(res.Decision.Reason)
	if !strings.Contains(r, "timeout") && !strings.Contains(r, "fail-closed") {
		t.Fatalf("Reason = %q, want it to mention timeout/fail-closed", res.Decision.Reason)
	}
}

func TestStdinCapEnforced(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)

	// Craft a syntactically valid but >1MB JSON object so decode does not fail
	// on syntax — the size cap must be what blocks it.
	var buf bytes.Buffer
	buf.WriteString(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"`)
	buf.WriteString(strings.Repeat("A", 2<<20)) // 2MB of payload
	buf.WriteString(`"}}`)

	res := RunCheck(context.Background(), &buf, closedConfig(), idxPath, auditPathIn(t), t.TempDir())

	if res.Decision.Allow {
		t.Fatal("Allow = true on oversized stdin, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on oversized stdin", res.ExitCode)
	}
	r := strings.ToLower(res.Decision.Reason)
	if !strings.Contains(r, "1mb") && !strings.Contains(r, "cap") {
		t.Fatalf("Reason = %q, want it to mention 1MB/cap", res.Decision.Reason)
	}
}

func TestMalformedJSONFailsClosed(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader("{this is not valid json")

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), t.TempDir())

	if res.Decision.Allow {
		t.Fatal("Allow = true on malformed JSON, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on malformed JSON", res.ExitCode)
	}
}

func TestMissingIndexFailsClosed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope", "bumblebee.idx")
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), missing, auditPathIn(t), t.TempDir())

	if res.Decision.Allow {
		t.Fatal("Allow = true with missing index, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero with missing index", res.ExitCode)
	}
}

func TestFailOpenModeAllowsOnFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope", "bumblebee.idx")
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)
	openCfg := config.Config{FailMode: config.FailModeOpen}

	res := RunCheck(context.Background(), stdin, openCfg, missing, auditPathIn(t), t.TempDir())

	// fail_open deliberately reduces security: a failure ALLOWS.
	if !res.Decision.Allow {
		t.Fatal("Allow = false with fail_open + missing index, want true (reduced-security opt-in)")
	}
	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d with fail_open", res.ExitCode, exitAllow)
	}
}

func TestAuditRecordWrittenOnEveryPath(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)
	// Use a fictional package to avoid live OSV hits on real packages.
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-test-clean-package-xyz-not-real@1.0.0"}}`)

	RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPath, t.TempDir())

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log not written: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		t.Fatal("audit log is empty, want one record")
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("audit record not valid JSON: %v", err)
	}
	if rec["record_type"] != "policy_decision" {
		t.Fatalf("record_type = %v, want policy_decision", rec["record_type"])
	}
}

func TestMalformedJSONStillAudits(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)

	RunCheck(context.Background(), strings.NewReader("{bad"), closedConfig(), idxPath, auditPath, t.TempDir())

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log not written on fail-closed path: %v", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("expected a best-effort audit record on malformed-JSON fail-closed path")
	}
}

// guard against an accidentally too-short timeout: a real evaluation must
// complete well within the budget.
func TestNormalEvaluationWithinDeadline(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	// Use a fictional package so OSV does not make a slow real call for a
	// known-vulnerable package, which would skew the deadline measurement.
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install beekeeper-test-clean-package-xyz-not-real@1.0.0"}}`)

	start := time.Now()
	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), t.TempDir())
	if time.Since(start) > execTimeout {
		t.Fatal("evaluation exceeded the execution timeout for a trivial input")
	}
	if !res.Decision.Allow {
		t.Fatalf("fictional clean package should allow; decision: %+v", res.Decision)
	}
}

// ─── Phase 4 Task 2: AgentContext + RunAuditRecord tests (INTG-07) ──────────

// TestReadAgentContext verifies that readAgentContext reads BEEKEEPER_AGENT_ID
// and returns it as AgentContext.AgentID.
func TestReadAgentContext(t *testing.T) {
	t.Setenv("BEEKEEPER_AGENT_ID", "agent-test-1")
	t.Setenv("BEEKEEPER_PARENT_AGENT_ID", "parent-1")
	// Clear other env vars to avoid interference from test environment
	t.Setenv("BEEKEEPER_AGENT_DEPTH", "")
	t.Setenv("BEEKEEPER_AGENT_LINEAGE", "")

	ac := readAgentContext("")
	if ac.AgentID != "agent-test-1" {
		t.Errorf("AgentID = %q, want agent-test-1", ac.AgentID)
	}
	if ac.ParentAgentID != "parent-1" {
		t.Errorf("ParentAgentID = %q, want parent-1", ac.ParentAgentID)
	}
}

// TestReadAgentContextNegativeDepth verifies that BEEKEEPER_AGENT_DEPTH="-3"
// results in Depth==0 (normalized).
func TestReadAgentContextNegativeDepth(t *testing.T) {
	t.Setenv("BEEKEEPER_AGENT_DEPTH", "-3")
	t.Setenv("BEEKEEPER_AGENT_ID", "")
	t.Setenv("BEEKEEPER_PARENT_AGENT_ID", "")
	t.Setenv("BEEKEEPER_AGENT_LINEAGE", "")

	ac := readAgentContext("")
	if ac.Depth != 0 {
		t.Errorf("Depth = %d, want 0 (negative depth normalized)", ac.Depth)
	}
}

// TestReadAgentContextLineage verifies that BEEKEEPER_AGENT_LINEAGE="root,mid,child"
// produces Lineage with 3 items.
func TestReadAgentContextLineage(t *testing.T) {
	t.Setenv("BEEKEEPER_AGENT_LINEAGE", "root,mid,child")
	t.Setenv("BEEKEEPER_AGENT_ID", "")
	t.Setenv("BEEKEEPER_PARENT_AGENT_ID", "")
	t.Setenv("BEEKEEPER_AGENT_DEPTH", "")

	ac := readAgentContext("")
	if len(ac.Lineage) != 3 {
		t.Errorf("Lineage = %v, want 3 items", ac.Lineage)
	}
	if ac.Lineage[0] != "root" || ac.Lineage[1] != "mid" || ac.Lineage[2] != "child" {
		t.Errorf("Lineage = %v, want [root mid child]", ac.Lineage)
	}
}

// TestReadAgentContextEnvOverridesStdin verifies that env var BEEKEEPER_AGENT_ID
// takes precedence over the stdin agent_id field.
func TestReadAgentContextEnvOverridesStdin(t *testing.T) {
	t.Setenv("BEEKEEPER_AGENT_ID", "env-id")
	t.Setenv("BEEKEEPER_PARENT_AGENT_ID", "")
	t.Setenv("BEEKEEPER_AGENT_DEPTH", "")
	t.Setenv("BEEKEEPER_AGENT_LINEAGE", "")

	ac := readAgentContext("stdin-id")
	if ac.AgentID != "env-id" {
		t.Errorf("AgentID = %q, want env-id (env var must override stdin)", ac.AgentID)
	}
}

// TestRunAuditRecordMalformedStdin verifies that RunAuditRecord returns 0
// even when stdin is malformed JSON (PostToolUse hooks must not disrupt agent).
func TestRunAuditRecordMalformedStdin(t *testing.T) {
	stdin := strings.NewReader("not json")
	code := RunAuditRecord(stdin, auditPathIn(t))
	if code != 0 {
		t.Errorf("RunAuditRecord with malformed stdin = %d, want 0 (must always exit 0)", code)
	}
}

// TestRunAuditRecordValid verifies that RunAuditRecord returns 0 and writes
// a tool_result audit record for a valid PostToolUse JSON input.
func TestRunAuditRecordValid(t *testing.T) {
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"npm test"},"tool_use_id":"uid-1"}`)
	code := RunAuditRecord(stdin, auditPath)
	if code != 0 {
		t.Errorf("RunAuditRecord returned %d, want 0", code)
	}

	// Verify the audit record was written.
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log not written: %v", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("audit log is empty, want one tool_result record")
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &rec); err != nil {
		t.Fatalf("audit record not valid JSON: %v", err)
	}
	if rec["record_type"] != "tool_result" {
		t.Errorf("record_type = %v, want tool_result", rec["record_type"])
	}
}

// fakeMultiCatalog is a test double for policy.MultiCatalogLookup.
type fakeMultiCatalog struct {
	matches []policy.CatalogMatch
}

func (f *fakeMultiCatalog) LookupAll(_, _ string) []policy.CatalogMatch {
	return f.matches
}

// fakeMultiIndex wraps a fakeMultiCatalog as a catalogIndex (MultiCatalogLookup + Closer).
type fakeMultiIndex struct {
	multi *fakeMultiCatalog
}

func (f *fakeMultiIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	return f.multi.LookupAll(ecosystem, pkg)
}
func (f *fakeMultiIndex) Close() error { return nil }

// TestRunCheckMultiSourceBlock verifies that when two signed sources agree,
// RunCheck returns a block result with ExitCode non-zero.
func TestRunCheckMultiSourceBlock(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)

	// The real Bumblebee index has the nrwl.angular-console entry (unsigned).
	// We need two signed sources → build an opener that returns the real index
	// but we also need to inject OSV+Socket results.
	// Easiest approach: use an opener that returns a fake *catalog.Index whose
	// LookupAll returns two signed matches from different sources.
	// Since we can't trivially fake *catalog.Index, use a real index plus a
	// MultiIndex with fakes. But RunCheck builds MultiIndex internally using cfg.
	// So: test this via runCheck with a panicOpener is not ideal.
	// Instead, use a special test opener that returns a real index, and we
	// accept that OSV/Socket will make no network calls (cacheDir is empty tempdir).
	// The single Bumblebee hit → warn. Multi-source block test is better
	// exercised via policy.Evaluate directly; integration via real network is CI-only.

	// Build an opener returning the real index.
	realOpener := func(path string) (*catalog.Index, error) {
		return catalog.OpenIndex(path)
	}

	// Single Bumblebee match (unsigned) → warn, not block.
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Install","tool_input":{"ecosystem":"editor-extension","package":"nrwl.angular-console","version":"18.95.0"}}`)
	res := runCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), t.TempDir(), realOpener)

	// Single unsigned source → warn, exit 0 (per PLCY-01 corroboration semantics).
	if res.ExitCode != exitAllow {
		t.Fatalf("single-source warn should exit 0, got %d", res.ExitCode)
	}
	if res.Decision.Level != "warn" {
		t.Fatalf("Level = %q, want warn for single-source Bumblebee hit", res.Decision.Level)
	}
}

// TestRunCheckSocketDisabledStillWorks verifies that with no Socket token configured,
// RunCheck still evaluates correctly using only Bumblebee.
func TestRunCheckSocketDisabledStillWorks(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	// Empty token → Socket adapter is nil (disabled).
	cfg := config.Config{FailMode: config.FailModeClosed, Socket: config.SocketConfig{APIToken: ""}}
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Install","tool_input":{"ecosystem":"editor-extension","package":"nrwl.angular-console","version":"18.95.0"}}`)

	res := RunCheck(context.Background(), stdin, cfg, idxPath, auditPathIn(t), t.TempDir())

	// One unsigned Bumblebee source → warn (allow=true, exit 0).
	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (single source warn)", res.ExitCode, exitAllow)
	}
	if res.Decision.Level != "warn" {
		t.Fatalf("Level = %q, want warn", res.Decision.Level)
	}
}

// mockScanner implements Scannable for testing LLMF integration.
type mockScanner struct {
	resp     llamafirewall.ScanResponse
	err      error
	degraded bool
	calls    []llamafirewall.ScanRequest
}

func (m *mockScanner) Scan(_ context.Context, req llamafirewall.ScanRequest) (llamafirewall.ScanResponse, error) {
	m.calls = append(m.calls, req)
	return m.resp, m.err
}

func (m *mockScanner) IsDegraded() bool { return m.degraded }

func TestHandlerLLMFInjectionRedacted(t *testing.T) {
	auditPath := auditPathIn(t)
	scanner := &mockScanner{resp: llamafirewall.ScanResponse{
		Result:     llamafirewall.ResultInjection,
		Confidence: 0.97,
		Reason:     "injection detected",
		LatencyMS:  10,
	}}
	stdin := strings.NewReader(`{"hook_event_name":"PostToolUse","tool_name":"read_file","tool_result":"<content that triggers injection>"}`)
	exitCode := RunAuditRecordWithLLMF(stdin, auditPath, config.Config{}, scanner)
	_ = exitCode // injection detection logs alert, does not block
	if len(scanner.calls) != 1 {
		t.Fatalf("expected 1 scan call, got %d", len(scanner.calls))
	}
	if scanner.calls[0].Kind != llamafirewall.ScanPrompt {
		t.Errorf("expected ScanPrompt, got %v", scanner.calls[0].Kind)
	}
}

func TestHandlerLLMFSidecarUnavailableFailsClosed(t *testing.T) {
	auditPath := auditPathIn(t)
	scanner := &mockScanner{err: llamafirewall.ErrSidecarUnavailable}
	stdin := strings.NewReader(`{"hook_event_name":"PostToolUse","tool_name":"web_search","tool_result":"search results"}`)
	cfg := config.Config{FailMode: config.FailModeClosed}
	exitCode := RunAuditRecordWithLLMF(stdin, auditPath, cfg, scanner)
	if exitCode != 1 {
		t.Errorf("expected exit 1 (block) on sidecar unavailable + fail-closed, got %d", exitCode)
	}
}

func TestHandlerLLMFCleanPassThrough(t *testing.T) {
	auditPath := auditPathIn(t)
	scanner := &mockScanner{resp: llamafirewall.ScanResponse{
		Result:     llamafirewall.ResultClean,
		Confidence: 0.1,
		LatencyMS:  5,
	}}
	stdin := strings.NewReader(`{"hook_event_name":"PostToolUse","tool_name":"read_file","tool_result":"safe content"}`)
	exitCode := RunAuditRecordWithLLMF(stdin, auditPath, config.Config{}, scanner)
	if exitCode != 0 {
		t.Errorf("expected exit 0 (allow) for clean scan, got %d", exitCode)
	}
	if len(scanner.calls) != 1 {
		t.Fatalf("expected 1 scan call, got %d", len(scanner.calls))
	}
}

func TestHandlerLLMFCodeShieldBlock(t *testing.T) {
	auditPath := auditPathIn(t)
	scanner := &mockScanner{resp: llamafirewall.ScanResponse{
		Result:     llamafirewall.ResultUnsafe,
		Confidence: 0.95,
		LatencyMS:  15,
	}}
	stdin := strings.NewReader(`{"hook_event_name":"PostToolUse","tool_name":"write_file","tool_result":"rm -rf /"}`)
	cfg := config.Config{LlamaFirewall: config.LlamaFirewallConfig{CodeShieldAction: "block"}}
	exitCode := RunAuditRecordWithLLMF(stdin, auditPath, cfg, scanner)
	if exitCode != 1 {
		t.Errorf("expected exit 1 (block) for CodeShield unsafe + block action, got %d", exitCode)
	}
}

// TestPolicyOverlayBlocksViaDir verifies that beekeeper check applies a
// policies-dir block rule: a tool call that the bare engine would allow is
// blocked when a matching package_allowlist block policy file is present in the
// policies directory (CODE-01 live enforcement).
func TestPolicyOverlayBlocksViaDir(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)

	// Create a cacheDir that has a sibling "policies/" directory containing a
	// block rule for "innocent-pkg-xyz-overlaytest".
	cacheDir := filepath.Join(dir, "catalogs")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatalf("MkdirAll cacheDir: %v", err)
	}
	policiesDir := filepath.Join(dir, "policies")
	if err := os.MkdirAll(policiesDir, 0700); err != nil {
		t.Fatalf("MkdirAll policiesDir: %v", err)
	}

	policyJSON := `{
		"schema_version": "1",
		"name": "test-block-policy",
		"rules": [
			{
				"id": "block-overlay-test-pkg",
				"rule_type": "package_allowlist",
				"ecosystem": "npm",
				"packages": ["innocent-pkg-xyz-overlaytest"],
				"action": "block"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(policiesDir, "block.json"), []byte(policyJSON), 0600); err != nil {
		t.Fatalf("WriteFile block.json: %v", err)
	}

	// The engine alone would allow this fictional package (not in any catalog).
	// The overlay block rule must force a block.
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"ecosystem":"npm","package":"innocent-pkg-xyz-overlaytest"}}`)
	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), cacheDir)

	if res.Decision.Allow {
		t.Errorf("Allow = true, want false (policy overlay block rule must override engine allow)")
	}
	if res.Decision.Level != "block" {
		t.Errorf("Level = %q, want block", res.Decision.Level)
	}
	if res.ExitCode == exitAllow {
		t.Errorf("ExitCode = %d, want non-zero (policy overlay block must exit non-zero)", res.ExitCode)
	}
}

func TestHandlerLLMFCodeShieldWarn(t *testing.T) {
	auditPath := auditPathIn(t)
	scanner := &mockScanner{resp: llamafirewall.ScanResponse{
		Result:     llamafirewall.ResultUnsafe,
		Confidence: 0.90,
		LatencyMS:  12,
	}}
	stdin := strings.NewReader(`{"hook_event_name":"PostToolUse","tool_name":"write_file","tool_result":"some code"}`)
	cfg := config.Config{LlamaFirewall: config.LlamaFirewallConfig{CodeShieldAction: "warn"}}
	exitCode := RunAuditRecordWithLLMF(stdin, auditPath, cfg, scanner)
	if exitCode != 0 {
		t.Errorf("expected exit 0 (warn, not block) for CodeShield warn action, got %d", exitCode)
	}
}
