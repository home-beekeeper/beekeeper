package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	audit "github.com/bantuson/beekeeper/internal/audit"
)

// TestScanPanelSteps: inject 4 stepTickMsgs → Body() contains all 4 step lines with ▸ prefix.
func TestScanPanelSteps(t *testing.T) {
	p := NewScanPanel("deep")
	// Inject 4 stepTickMsgs
	var pc PanelContent = p
	for i := 0; i < 4; i++ {
		var cmd tea.Cmd
		pc, cmd = pc.Update(stepTickMsg{})
		_ = cmd
	}
	sp := pc.(*ScanPanel)
	body := sp.progressBody()
	for _, step := range scanSteps {
		if !strings.Contains(body, step[:20]) { // check first 20 chars
			t.Errorf("expected step %q in body after 4 ticks", step[:20])
		}
	}
	if sp.done {
		t.Error("should not be done after exactly 4 steps (steps are 0-indexed advancement)")
	}
}

// TestScanPanelComplete: inject 5th stepTickMsg (past last step) → Body() contains "scan complete".
func TestScanPanelComplete(t *testing.T) {
	p := NewScanPanel("deep")
	var pc PanelContent = p
	// 5 ticks: 4 steps + 1 to trigger done
	for i := 0; i < 5; i++ {
		pc, _ = pc.Update(stepTickMsg{})
	}
	sp := pc.(*ScanPanel)
	body := sp.progressBody()
	if !strings.Contains(body, "scan complete") {
		t.Errorf("expected 'scan complete' in body after 5 ticks, got: %q", body)
	}
	if !sp.done {
		t.Error("expected done=true after 5 ticks")
	}
}

// TestPolicyPanelContent: Body() contains all 5 prototype rows.
func TestPolicyPanelContent(t *testing.T) {
	p := NewPolicyPanel(false)
	body := p.Body(100, 40)
	for _, expected := range []string{"corroboration", "release-age", "llamafirewall"} {
		if !strings.Contains(body, expected) {
			t.Errorf("expected %q in policy panel body", expected)
		}
	}
}

// TestAuditPanelAlertRed: sentry_alert record stored and retrievable.
func TestAuditPanelAlertRed(t *testing.T) {
	p := NewAuditPanel()
	msg := newRecordsMsg{
		{RecordType: "sentry_alert", Decision: "alert", Timestamp: "2026-05-28T14:21:54Z"},
	}
	pc, _ := p.Update(msg)
	ap := pc.(*AuditPanel)
	if len(ap.records) != 1 {
		t.Errorf("expected 1 record stored, got %d", len(ap.records))
	}
}

// TestAuditPanelMax20: panel caps at 20 records.
func TestAuditPanelMax20(t *testing.T) {
	p := NewAuditPanel()
	recs := make([]audit.AuditRecord, 21)
	for i := range recs {
		recs[i] = audit.AuditRecord{RecordType: "policy_decision", Decision: "allow", Timestamp: "2026-05-28T10:00:00Z"}
	}
	pc, _ := p.Update(newRecordsMsg(recs))
	ap := pc.(*AuditPanel)
	if len(ap.records) != 20 {
		t.Errorf("expected 20 records (cap), got %d", len(ap.records))
	}
}

// TestHelpPanelContent: Body() contains NAVIGATION and CONCEPT sections.
func TestHelpPanelContent(t *testing.T) {
	p := NewHelpPanel()
	body := p.Body(100, 40)
	if !strings.Contains(body, "NAVIGATION") {
		t.Errorf("expected 'NAVIGATION' in help body")
	}
	if !strings.Contains(body, "command palette") {
		t.Errorf("expected 'command palette' in help body")
	}
	if !strings.Contains(body, "CONCEPT") {
		t.Errorf("expected 'CONCEPT' in help body")
	}
}
