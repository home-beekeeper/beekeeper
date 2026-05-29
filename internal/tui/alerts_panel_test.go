package tui

import (
	"strings"
	"testing"
	"time"

	audit "github.com/mzansi-agentive/beekeeper/internal/audit"
)

func makeTS(offset int) string {
	return time.Now().Add(time.Duration(offset) * time.Minute).UTC().Format(time.RFC3339)
}

func TestAlertsPanelSentryAlert(t *testing.T) {
	p := NewAlertsPanel(false)
	msg := newRecordsMsg{
		{
			RecordType:     "sentry_alert",
			SentrySeverity: "critical",
			SentryRuleName: "exfil-signature-fusion",
			Timestamp:      makeTS(0),
		},
	}
	pc, _ := p.Update(msg)
	ap := pc.(*AlertsPanel)
	body := ap.Body(100, 40)
	if !strings.Contains(body, "CRIT") {
		t.Errorf("expected CRIT badge text in body, got: %q", body)
	}
}

func TestAlertsPanelPolicyBlock(t *testing.T) {
	p := NewAlertsPanel(false)
	msg := newRecordsMsg{
		{
			RecordType: "policy_decision",
			Decision:   "block",
			ToolName:   "Bash",
			Reason:     "release-age 4h < 24h",
			Timestamp:  makeTS(0),
		},
	}
	pc, _ := p.Update(msg)
	ap := pc.(*AlertsPanel)
	body := ap.Body(100, 40)
	if !strings.Contains(body, "BLOCK") {
		t.Errorf("expected BLOCK badge text in body, got: %q", body)
	}
}

func TestAlertsPanelPolicyWarn(t *testing.T) {
	p := NewAlertsPanel(false)
	msg := newRecordsMsg{
		{
			RecordType: "policy_decision",
			Decision:   "warn",
			ToolName:   "npm",
			Reason:     "1-source only",
			Timestamp:  makeTS(0),
		},
	}
	pc, _ := p.Update(msg)
	ap := pc.(*AlertsPanel)
	body := ap.Body(100, 40)
	if !strings.Contains(body, "WARN") {
		t.Errorf("expected WARN badge text in body, got: %q", body)
	}
}

func TestAlertsPanelRecordFilter(t *testing.T) {
	p := NewAlertsPanel(false)
	msg := newRecordsMsg{
		{
			RecordType: "policy_decision",
			Decision:   "allow",
			ToolName:   "Read",
			Timestamp:  makeTS(0),
		},
	}
	pc, _ := p.Update(msg)
	ap := pc.(*AlertsPanel)
	if len(ap.rows) != 0 {
		t.Errorf("expected 0 rows for allow decision, got %d", len(ap.rows))
	}
}

func TestAlertsPanelMaxRows(t *testing.T) {
	p := NewAlertsPanel(false)
	recs := make([]audit.AuditRecord, 201)
	for i := range recs {
		recs[i] = audit.AuditRecord{
			RecordType:     "sentry_alert",
			SentrySeverity: "critical",
			Timestamp:      makeTS(i),
		}
	}
	pc, _ := p.Update(newRecordsMsg(recs))
	ap := pc.(*AlertsPanel)
	if len(ap.rows) != maxAlertRows {
		t.Errorf("expected %d rows (cap), got %d", maxAlertRows, len(ap.rows))
	}
}

func TestAlertsPanelCriticalBorder(t *testing.T) {
	p := NewAlertsPanel(true)
	if !p.Critical() {
		t.Error("expected Critical()=true for panel created with critical=true")
	}
	p2 := NewAlertsPanel(false)
	if p2.Critical() {
		t.Error("expected Critical()=false for panel created with critical=false")
	}
}

func TestAlertsPanelFilterMatch(t *testing.T) {
	p := NewAlertsPanel(false)
	msg := newRecordsMsg{
		{RecordType: "sentry_alert", SentrySeverity: "critical", SentryRuleName: "exfil-signature-fusion", Timestamp: makeTS(0)},
		{RecordType: "policy_decision", Decision: "block", ToolName: "npm install chalk", Reason: "release-age", Timestamp: makeTS(-1)},
	}
	pc, _ := p.Update(msg)
	ap := pc.(*AlertsPanel)
	if len(ap.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(ap.rows))
	}
	// Apply filter that matches only the sentry alert
	ap.filterQuery = "exfil"
	visible := ap.filteredRows()
	if len(visible) != 1 {
		t.Errorf("expected 1 match for 'exfil', got %d", len(visible))
	}
	if visible[0].Label != "exfil-signature-fusion" {
		t.Errorf("unexpected label: %q", visible[0].Label)
	}
}

func TestAlertsPanelFilterNoMatch(t *testing.T) {
	p := NewAlertsPanel(false)
	msg := newRecordsMsg{
		{RecordType: "sentry_alert", SentrySeverity: "critical", SentryRuleName: "exfil-signature-fusion", Timestamp: makeTS(0)},
	}
	pc, _ := p.Update(msg)
	ap := pc.(*AlertsPanel)
	ap.filterQuery = "zzznomatch"
	visible := ap.filteredRows()
	if len(visible) != 0 {
		t.Errorf("expected 0 matches for 'zzznomatch', got %d", len(visible))
	}
	body := ap.Body(100, 20)
	if !strings.Contains(body, "no matches") {
		t.Errorf("expected 'no matches' in body for empty filter result")
	}
}
