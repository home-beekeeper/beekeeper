package tui

import (
	"strings"
	"testing"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
)

func rec(t string, fields audit.AuditRecord) audit.AuditRecord {
	fields.RecordType = t
	if fields.Timestamp == "" {
		fields.Timestamp = "2026-06-17T09:00:00Z"
	}
	return fields
}

// newTestAuditPanel builds a panel from in-memory records (no file read).
func newTestAuditPanel(records ...audit.AuditRecord) *AuditPanel {
	return &AuditPanel{records: records, seen: map[string]bool{}}
}

func TestAuditBadgeFor(t *testing.T) {
	cases := []struct {
		rec  audit.AuditRecord
		want string
	}{
		{rec("policy_decision", audit.AuditRecord{Decision: "block"}), "BLOCK"},
		{rec("policy_decision", audit.AuditRecord{Decision: "warn"}), "WARN"},
		{rec("policy_decision", audit.AuditRecord{Decision: "allow"}), "ALLOW"},
		{rec("sentry_alert", audit.AuditRecord{}), "ALERT"},
		{rec("config_change", audit.AuditRecord{}), "CONFIG"},
		{rec("nudge", audit.AuditRecord{NudgeAction: "block"}), "NUDGE"},
		{rec("llmf_alert", audit.AuditRecord{LLMFResult: "injection"}), "LLMF"},
	}
	for _, c := range cases {
		if got, _ := badgeFor(c.rec); got != c.want {
			t.Errorf("badgeFor(%+v) = %q, want %q", c.rec, got, c.want)
		}
	}
}

func TestAuditDisplayNewestFirstAndFilter(t *testing.T) {
	p := newTestAuditPanel(
		rec("policy_decision", audit.AuditRecord{ToolName: "old-allow", Decision: "allow"}),
		rec("policy_decision", audit.AuditRecord{ToolName: "mid-block", Decision: "block"}),
		rec("sentry_alert", audit.AuditRecord{SentryProcessExe: "new-alert"}),
	)
	disp := p.display()
	if len(disp) != 3 || disp[0].RecordType != "sentry_alert" {
		t.Fatalf("display should be newest-first; got %d, first=%q", len(disp), disp[0].RecordType)
	}
	p.enforcedOnly = true
	disp = p.display()
	if len(disp) != 2 {
		t.Fatalf("enforced filter should drop the allow; got %d", len(disp))
	}
	for _, r := range disp {
		if r.Decision == "allow" {
			t.Errorf("enforced filter leaked an allow record")
		}
	}
}

// TestAuditSanitizesFields proves attacker-influenceable fields are sanitized
// before rendering: a bidi-override and a BEL embedded in a package name / reason
// never reach the rendered Body (those bytes are added by no styling, only data).
func TestAuditSanitizesFields(t *testing.T) {
	p := newTestAuditPanel(rec("policy_decision", audit.AuditRecord{
		Decision: "block",
		Reason:   "evil\x07reason",
		CatalogMatches: []audit.CatalogProvenance{
			{CatalogSource: "bumblebee", Ecosystem: "npm", Package: "‮elive-pkg", Version: "1.0.0", Severity: "critical", Signed: true},
		},
	}))
	body := p.Body(120, 20)
	if strings.ContainsRune(body, '‮') {
		t.Error("rendered body contains a raw bidi-override rune from a package name")
	}
	if strings.ContainsRune(body, '\x07') {
		t.Error("rendered body contains a raw BEL byte from a reason field")
	}
	// The printable remainder is still shown.
	if !strings.Contains(body, "elive-pkg") {
		t.Errorf("sanitized package name should still render its printable text:\n%s", body)
	}
}

func TestAuditNavigationAndDetail(t *testing.T) {
	p := newTestAuditPanel(
		rec("policy_decision", audit.AuditRecord{ToolName: "a", Decision: "allow"}),
		rec("policy_decision", audit.AuditRecord{ToolName: "b", Decision: "block"}),
	)
	// j moves down within the display slice; clamps at the end.
	p.handleKey("j")
	if p.selIdx != 1 {
		t.Fatalf("selIdx after j = %d, want 1", p.selIdx)
	}
	p.handleKey("j")
	if p.selIdx != 1 {
		t.Fatalf("selIdx should clamp at len-1, got %d", p.selIdx)
	}
	// enter expands; h collapses.
	p.handleKey("enter")
	if !p.expanded {
		t.Fatal("enter should expand detail")
	}
	p.handleKey("h")
	if p.expanded {
		t.Fatal("h should collapse detail")
	}
}

func TestAuditDetailShowsDecisionLogic(t *testing.T) {
	r := rec("policy_decision", audit.AuditRecord{
		Decision:           "block",
		Reason:             "corroborated malicious",
		RuleIDs:            []string{"CTLG-02"},
		CorroborationCount: 2,
		SourcesAgreed:      []string{"bumblebee", "socket"},
		SourcesDissented:   []string{"osv"},
		CatalogMatches: []audit.CatalogProvenance{
			{CatalogSource: "bumblebee", Ecosystem: "npm", Package: "evil", Version: "9.9.9", Severity: "critical", Signed: true},
		},
	})
	lines := strings.Join(detailLines(r), "\n")
	for _, want := range []string{"corroboration", "bumblebee", "socket", "osv", "CTLG-02", "evil@9.9.9", "critical", "signed"} {
		if !strings.Contains(lines, want) {
			t.Errorf("detail logic missing %q:\n%s", want, lines)
		}
	}
}

func TestAuditFilterToggleAndCount(t *testing.T) {
	p := newTestAuditPanel(
		rec("policy_decision", audit.AuditRecord{Decision: "allow"}),
		rec("policy_decision", audit.AuditRecord{Decision: "block"}),
	)
	if got := p.Count(); !strings.Contains(got, "2 records") {
		t.Errorf("Count = %q, want it to mention 2 records", got)
	}
	p.handleKey("f")
	if !p.enforcedOnly {
		t.Fatal("f should toggle the enforced filter on")
	}
	if got := p.Count(); !strings.Contains(got, "enforced") {
		t.Errorf("Count under filter = %q, want it to mention enforced", got)
	}
}

func TestAuditLiveAppendDedupes(t *testing.T) {
	p := newTestAuditPanel()
	r := rec("policy_decision", audit.AuditRecord{RecordID: "abc", Decision: "block"})
	p.Update(newRecordsMsg([]audit.AuditRecord{r}))
	p.Update(newRecordsMsg([]audit.AuditRecord{r})) // same RecordID again
	if len(p.records) != 1 {
		t.Errorf("duplicate RecordID should not double-append; got %d", len(p.records))
	}
}

func TestAuditEmptyBody(t *testing.T) {
	p := newTestAuditPanel()
	if !strings.Contains(p.Body(80, 20), "no audit records") {
		t.Error("empty panel should show the no-records message")
	}
}

func TestAuditRenderNoPanicAllRows(t *testing.T) {
	// Render every row selected + expanded across record types — must not panic.
	recs := []audit.AuditRecord{
		rec("policy_decision", audit.AuditRecord{Decision: "block", CatalogMatches: []audit.CatalogProvenance{{CatalogSource: "osv", Ecosystem: "pypi", Package: "p", Version: "1"}}}),
		rec("nudge", audit.AuditRecord{NudgeAction: "rewrite", OriginalCommand: "npm i x", RewrittenCommand: "pnpm add x"}),
		rec("sentry_alert", audit.AuditRecord{SentryRuleName: "exfil", SentrySeverity: "critical", SentryProcessExe: "node", SentryNetworkDests: []string{"1.2.3.4:443"}}),
		rec("config_change", audit.AuditRecord{ReasonCode: "auto_quarantine.enabled", Reason: "changed"}),
	}
	p := newTestAuditPanel(recs...)
	for i := range recs {
		p.selIdx = i
		p.expanded = false
		_ = p.Body(100, 12)
		p.expanded = true
		_ = p.Body(100, 12)
	}
}
