package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	audit "github.com/home-beekeeper/beekeeper/internal/audit"
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

// TestScanPanelTicksNeverComplete: the animation alone must NOT mark the scan
// done or fabricate a completion line — completion comes only from a real result.
func TestScanPanelTicksNeverComplete(t *testing.T) {
	p := NewScanPanel("deep")
	var pc PanelContent = p
	for i := 0; i < 6; i++ {
		pc, _ = pc.Update(stepTickMsg{})
	}
	sp := pc.(*ScanPanel)
	if sp.done {
		t.Error("animation ticks must not set done — only a real scanResultMsg may")
	}
	if strings.Contains(sp.progressBody(), "scan complete") {
		t.Error("animation must not render a fabricated 'scan complete' line")
	}
}

// TestScanPanelComplete: a real scanResultMsg → Body() shows the REAL counts.
func TestScanPanelComplete(t *testing.T) {
	p := NewScanPanel("deep")
	var pc PanelContent = p
	pc, _ = pc.Update(scanResultMsg{res: scanResult{packages: 312, findings: 47, threats: 0}})
	sp := pc.(*ScanPanel)
	if !sp.done {
		t.Error("expected done=true after scanResultMsg")
	}
	body := sp.progressBody()
	if !strings.Contains(body, "scan complete") {
		t.Errorf("expected 'scan complete' in body, got: %q", body)
	}
	if !strings.Contains(body, "312 packages") || !strings.Contains(body, "47 findings") {
		t.Errorf("expected real counts in completion line, got: %q", body)
	}
	if !strings.Contains(body, "no threats matched") {
		t.Errorf("expected 'no threats matched' for zero threats, got: %q", body)
	}
}

// TestScanPanelThreatsAndError: result rendering reflects real threats and errors.
func TestScanPanelThreatsAndError(t *testing.T) {
	p := NewScanPanel("quick")
	pc, _ := PanelContent(p).Update(scanResultMsg{res: scanResult{packages: 10, findings: 2, threats: 2}})
	if body := pc.(*ScanPanel).progressBody(); !strings.Contains(body, "2 threats flagged") {
		t.Errorf("expected threat count in body, got: %q", body)
	}

	p2 := NewScanPanel("deep")
	pc2, _ := PanelContent(p2).Update(scanResultMsg{err: errInjectedScan})
	if body := pc2.(*ScanPanel).progressBody(); !strings.Contains(body, "scan failed") {
		t.Errorf("expected error surfaced in body, got: %q", body)
	}
}

var errInjectedScan = errors.New("boom")

// TestPolicyPanelContent: Body() renders the editable sections and the honest
// read-only rows. Uses a temp-dir panel so it never touches the real policies dir.
func TestPolicyPanelContent(t *testing.T) {
	p, _ := newTestPolicyPanel(t, false)
	body := p.Body(100, 40)
	for _, expected := range []string{"Corroboration", "block at", "Package allowlist", "Sensitive paths", "release-age", "llamafirewall"} {
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
