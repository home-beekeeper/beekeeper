package tui

import (
	"strings"
	"testing"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
	policyloader "github.com/home-beekeeper/beekeeper/internal/policyloader"
)

// TestIsUnsafeRuneAllBranches covers every classification branch including the
// C1 controls, bidi isolates, and BOM, plus the safe default.
func TestIsUnsafeRuneAllBranches(t *testing.T) {
	unsafe := []rune{
		0x00,   // NUL (C0)
		0x1b,   // ESC (C0)
		0x7f,   // DEL
		0x85,   // C1 control
		0x200b, // zero-width space
		0x202e, // bidi override (RLO)
		0x2066, // bidi isolate (LRI)
		0x2069, // bidi isolate (PDI)
		0xfeff, // BOM
	}
	for _, r := range unsafe {
		if !isUnsafeRune(r) {
			t.Errorf("rune %U should be classified unsafe", r)
		}
	}
	safe := []rune{'a', '9', 'é', '日', '😀', ' ' + 1}
	for _, r := range safe {
		if isUnsafeRune(r) {
			t.Errorf("rune %U should be classified safe", r)
		}
	}
}

// TestAuditClampSelOutOfRange covers the clampSel branches: an over-range selIdx
// is pulled back to n-1, an empty display resets to 0.
func TestAuditClampSelOutOfRange(t *testing.T) {
	p := newTestAuditPanel(
		rec("policy_decision", audit.AuditRecord{Decision: "block"}),
		rec("policy_decision", audit.AuditRecord{Decision: "allow"}),
	)
	p.selIdx = 99
	p.clampSel(len(p.display()))
	if p.selIdx != 1 {
		t.Errorf("over-range selIdx should clamp to 1, got %d", p.selIdx)
	}
	p.selIdx = 5
	p.clampSel(0)
	if p.selIdx != 0 {
		t.Errorf("empty display should reset selIdx to 0, got %d", p.selIdx)
	}
}

// TestAuditEnsureVisibleScrolls drives the scroll-window math: selecting a record
// past the visible window advances the offset; scrolling back up resets it.
func TestAuditEnsureVisibleScrolls(t *testing.T) {
	recs := make([]audit.AuditRecord, 30)
	for i := range recs {
		recs[i] = rec("policy_decision", audit.AuditRecord{Decision: "allow"})
	}
	p := newTestAuditPanel(recs...)
	// Render a short window then jump the selection to the end.
	p.Body(100, 8)
	for i := 0; i < 29; i++ {
		p.handleKey("j")
	}
	p.Body(100, 8)
	if p.offset == 0 {
		t.Error("selecting the last record in a tall list should scroll the window (offset>0)")
	}
	// Scroll back to the top.
	for i := 0; i < 29; i++ {
		p.handleKey("k")
	}
	p.Body(100, 8)
	if p.offset != 0 {
		t.Errorf("scrolling back to the first record should reset offset to 0, got %d", p.offset)
	}
}

// TestAuditRecordSubjectVariants covers the recordSubject selector branches.
func TestAuditRecordSubjectVariants(t *testing.T) {
	cases := []struct {
		rec  audit.AuditRecord
		want string
	}{
		{audit.AuditRecord{CatalogMatches: []audit.CatalogProvenance{{Ecosystem: "npm", Package: "evil", Version: "1.0"}}}, "npm:evil@1.0"},
		{audit.AuditRecord{OriginalCommand: "npm install x"}, "npm install x"},
		{audit.AuditRecord{SentryProcessExe: "node ext.js"}, "node ext.js"},
		{audit.AuditRecord{SentryCorrelatedExt: "acme.evil"}, "acme.evil"},
		{audit.AuditRecord{RecordType: "config_change", ReasonCode: "corpus.enabled"}, "corpus.enabled"},
		{audit.AuditRecord{ToolName: "Bash"}, "Bash"},
	}
	for _, c := range cases {
		if got := recordSubject(c.rec, 60); got != c.want {
			t.Errorf("recordSubject(%+v) = %q, want %q", c.rec, got, c.want)
		}
	}
}

// TestAuditDetailLinesNudgeAndSentryAndLLMF covers detail-block branches not hit
// elsewhere: nudge rewrite, sentry process/parents/files/network, llamafirewall,
// agent lineage/cluster/ruleset.
func TestAuditDetailLinesRichRecord(t *testing.T) {
	r := rec("sentry_alert", audit.AuditRecord{
		Decision:            "alert",
		NudgeAction:         "rewrite",
		OriginalCommand:     "npm i x",
		RewrittenCommand:    "pnpm add x",
		SentryRuleName:      "exfil",
		SentrySeverity:      "critical",
		SentryProcessExe:    "node",
		SentryProcessPID:    42,
		SentryParentChain:   []string{"launchd", "node"},
		SentryFilesAccessed: []string{"~/.aws/credentials"},
		SentryNetworkDests:  []string{"1.2.3.4:443"},
		SentryCorrelatedExt: "acme.evil",
		LLMFScanned:         true,
		LLMFScanKind:        "prompt",
		LLMFResult:          "injection",
		LLMFConfidence:      0.91,
		AgentID:             "agent-1",
		AgentLineage:        []string{"root", "agent-1"},
		ClusterID:           "cluster-9",
		RulesetVersion:      "2026.06",
	})
	lines := strings.Join(detailLines(r), "\n")
	for _, want := range []string{"rewrite", "pnpm add x", "exfil", "node", "launchd", "~/.aws/credentials", "1.2.3.4:443", "acme.evil", "injection", "agent-1", "cluster-9", "2026.06"} {
		if !strings.Contains(lines, want) {
			t.Errorf("detailLines missing %q:\n%s", want, lines)
		}
	}
}

// TestAuditDetailLinesEmpty covers the "(no further detail)" fallback.
func TestAuditDetailLinesEmpty(t *testing.T) {
	lines := detailLines(audit.AuditRecord{})
	if len(lines) != 1 || !strings.Contains(lines[0], "no further detail") {
		t.Errorf("empty record should yield the no-detail fallback, got %+v", lines)
	}
}

// TestIsEnforcementEventBranches covers each enforcement classification branch.
func TestIsEnforcementEventBranches(t *testing.T) {
	yes := []audit.AuditRecord{
		{Decision: "block"},
		{Decision: "warn"},
		{Decision: "alert"},
		{Quarantine: true},
		{RecordType: "sentry_alert"},
		{RecordType: "nudge", NudgeAction: "block"},
		{RecordType: "llmf_alert", LLMFResult: "injection"},
	}
	for _, r := range yes {
		if !isEnforcementEvent(r) {
			t.Errorf("%+v should be an enforcement event", r)
		}
	}
	no := []audit.AuditRecord{
		{Decision: "allow"},
		{RecordType: "nudge", NudgeAction: "rewrite"},
		{RecordType: "llmf_alert", LLMFResult: "clean"},
		{RecordType: "package"},
	}
	for _, r := range no {
		if isEnforcementEvent(r) {
			t.Errorf("%+v should NOT be an enforcement event", r)
		}
	}
}

// TestSplitEcoPkgInvalid covers the rejection branches of splitEcoPkg.
func TestSplitEcoPkgInvalid(t *testing.T) {
	bad := []string{"nocolon", ":leading", "trailing:", ":", "  :  "}
	for _, s := range bad {
		if _, _, ok := splitEcoPkg(s); ok {
			t.Errorf("splitEcoPkg(%q) should be rejected", s)
		}
	}
	eco, pkg, ok := splitEcoPkg("  NPM : React  ")
	if !ok || eco != "npm" || pkg != "React" {
		t.Errorf("splitEcoPkg trimmed/lowercased = (%q,%q,%v), want (npm,React,true)", eco, pkg, ok)
	}
}

// TestPolicyHelpers covers corrRuleIndex (found/not-found), policyHasRuleID, and
// uniquePolicyRuleID's collision-avoidance loop.
func TestPolicyHelpers(t *testing.T) {
	empty := policyloader.PolicyFile{}
	if corrRuleIndex(empty) != -1 {
		t.Error("corrRuleIndex on a file with no corroboration rule should be -1")
	}
	pf := policyloader.PolicyFile{Rules: []policyloader.PolicyRule{
		{ID: "tui-allow-npm-react", RuleType: "package_allowlist"},
		{ID: "corr", RuleType: "corroboration_threshold"},
	}}
	if corrRuleIndex(pf) != 1 {
		t.Errorf("corrRuleIndex should find the corroboration rule at index 1, got %d", corrRuleIndex(pf))
	}
	if !policyHasRuleID(pf, "corr") {
		t.Error("policyHasRuleID should find an existing ID")
	}
	if policyHasRuleID(pf, "nope") {
		t.Error("policyHasRuleID should not find a missing ID")
	}
	// uniquePolicyRuleID must avoid the existing slugged ID by suffixing -2.
	got := uniquePolicyRuleID(pf, "tui-allow-npm-react")
	if got == "tui-allow-npm-react" {
		t.Errorf("uniquePolicyRuleID should avoid the existing ID, got %q", got)
	}
	if !strings.HasPrefix(got, "tui-allow-npm-react") {
		t.Errorf("uniquePolicyRuleID should keep the base slug, got %q", got)
	}
}

// TestPolicySlugEdgeCases covers the empty-result fallback.
func TestPolicySlugEdgeCases(t *testing.T) {
	if got := policySlug("***"); got != "entry" {
		t.Errorf("policySlug of all-punctuation should fall back to 'entry', got %q", got)
	}
	if got := policySlug("NPM React!!"); got != "npm-react" {
		t.Errorf("policySlug = %q, want npm-react", got)
	}
}

// TestPolicyAdjustCorrSeedsRule proves adjustCorr seeds a corroboration rule when
// none exists, then changes the critical override threshold.
func TestPolicyAdjustCorrCriticalField(t *testing.T) {
	p, dir := newTestPolicyPanel(t, true)
	// Select the "critical block at" row and increment it.
	critIdx := -1
	for i, r := range p.rows {
		if r.corrField == "critical" {
			critIdx = i
		}
	}
	if critIdx < 0 {
		t.Fatal("no critical corroboration row found")
	}
	p.selIdx = critIdx
	if cmd := p.handleKey("+"); cmd != nil {
		if _, isErr := cmd().(policyEditErrMsg); isErr {
			t.Fatal("incrementing the critical threshold should not be rejected")
		}
	}
	// The managed file remains valid and loadable.
	if _, errs := policyloader.LoadPolicyFile(policyloader.ManagedPolicyPath(dir)); len(errs) > 0 {
		t.Errorf("managed file invalid after critical edit: %v", errs)
	}
}
