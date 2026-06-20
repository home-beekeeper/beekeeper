package tui

import (
	"strings"
	"testing"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
)

// TestScanPanelTitleCountFooter covers the per-mode header/footer text.
func TestScanPanelTitleCountFooter(t *testing.T) {
	cases := []struct {
		mode      string
		wantTitle string
		wantCount string
	}{
		{"deep", "Bumblebee scan", "deep · all ecosystems"},
		{"quick", "Bumblebee scan", "quick · lockfiles + ext"},
		{"history", "Scan history", "past runs"},
	}
	for _, c := range cases {
		p := NewScanPanel(c.mode)
		if p.Title() != c.wantTitle {
			t.Errorf("%s Title = %q, want %q", c.mode, p.Title(), c.wantTitle)
		}
		if p.Count() != c.wantCount {
			t.Errorf("%s Count = %q, want %q", c.mode, p.Count(), c.wantCount)
		}
		if !p.Padded() {
			t.Errorf("%s scan panel should be padded", c.mode)
		}
		if p.Critical() {
			t.Errorf("%s scan panel must not be critical", c.mode)
		}
	}
}

// TestScanPanelFooterStates proves the footer reads "scanning…" while a non-history
// scan is in flight and switches to the close hint when done.
func TestScanPanelFooterStates(t *testing.T) {
	running := NewScanPanel("deep")
	if !strings.Contains(running.Footer(), "scanning") {
		t.Errorf("running scan Footer = %q, want a scanning hint", running.Footer())
	}
	done := NewScanPanel("deep")
	done.done = true
	if !strings.Contains(done.Footer(), "close") {
		t.Errorf("finished scan Footer = %q, want a close hint", done.Footer())
	}
	// History mode always shows the close hint.
	hist := NewScanPanel("history")
	if !strings.Contains(hist.Footer(), "close") {
		t.Errorf("history Footer = %q, want a close hint", hist.Footer())
	}
}

// TestScanPanelBodyRoutesByMode proves Body() dispatches to progressBody vs
// historyBody.
func TestScanPanelBodyRoutesByMode(t *testing.T) {
	deep := NewScanPanel("deep")
	if got := deep.Body(80, 24); got != deep.progressBody() {
		t.Error("deep mode Body should be the progress view")
	}
	hist := NewScanPanel("history")
	if !strings.Contains(hist.Body(80, 24), "no scan history") {
		t.Errorf("empty history Body should show the placeholder, got %q", hist.Body(80, 24))
	}
}

// TestScanPanelHistoryAppends proves history mode collects scan_status/finding
// records from newRecordsMsg and renders them; unrelated records are ignored.
func TestScanPanelHistoryAppends(t *testing.T) {
	p := NewScanPanel("history")
	pc, _ := p.Update(newRecordsMsg{
		{RecordType: "scan_status", Timestamp: "2026-06-18T09:00:00Z"},
		{RecordType: "finding", Timestamp: "2026-06-18T09:01:00Z"},
		{RecordType: "policy_decision", Timestamp: "2026-06-18T09:02:00Z"}, // ignored
	})
	sp := pc.(*ScanPanel)
	if len(sp.history) != 2 {
		t.Fatalf("history should keep only scan_status/finding records, got %d", len(sp.history))
	}
	body := sp.historyBody()
	if !strings.Contains(body, "2026-06-18T09:00:00Z") {
		t.Errorf("history body should render the record timestamps, got %q", body)
	}
}

// TestScanPanelHistoryIgnoresProgressTicks proves a history panel never advances
// the step animation.
func TestScanPanelHistoryIgnoresProgressTicks(t *testing.T) {
	p := NewScanPanel("history")
	pc, cmd := p.Update(stepTickMsg{})
	if cmd != nil {
		t.Error("history mode should not re-arm the step ticker")
	}
	if pc.(*ScanPanel).currentStep != 0 {
		t.Error("history mode must not advance the step animation")
	}
}

// TestScanPanelStepReArmsUntilLast proves the step ticker re-arms while steps
// remain and stops on the last step (without marking done).
func TestScanPanelStepReArmsUntilLast(t *testing.T) {
	p := NewScanPanel("deep")
	var pc PanelContent = p
	// First len-1 ticks re-arm.
	for i := 0; i < len(scanSteps)-1; i++ {
		var cmd interface{}
		pc, cmd = pc.Update(stepTickMsg{})
		if cmd == nil {
			t.Fatalf("step %d should re-arm the ticker", i)
		}
	}
	// The tick that reveals the final step must NOT re-arm.
	pc2, cmd := pc.Update(stepTickMsg{})
	if cmd != nil {
		t.Error("the final step tick should not re-arm the ticker")
	}
	if pc2.(*ScanPanel).done {
		t.Error("the animation must never set done")
	}
}

// TestScanPanelErrorAndPollenUnavailable proves the progress body surfaces an
// error and the pollen-unavailable note.
func TestScanPanelErrorAndPollenUnavailable(t *testing.T) {
	errP := NewScanPanel("deep")
	errP.done = true
	errP.err = errInjectedScan
	if !strings.Contains(errP.progressBody(), "scan failed") {
		t.Errorf("error body should report the failure: %q", errP.progressBody())
	}

	okP := NewScanPanel("deep")
	okP.done = true
	okP.result = &scanResult{packages: 5, findings: 1, threats: 1, pollenUnavailable: true}
	body := okP.progressBody()
	if !strings.Contains(body, "1 threat flagged") {
		t.Errorf("single-threat body should be singular: %q", body)
	}
	if !strings.Contains(body, "pollen unavailable") {
		t.Errorf("body should note pollen unavailability: %q", body)
	}
}

// TestScanPanelDoneNilResult proves a done panel with a nil result renders the
// zero-count completion line without panicking.
func TestScanPanelDoneNilResult(t *testing.T) {
	p := NewScanPanel("deep")
	p.done = true // result stays nil
	if !strings.Contains(p.progressBody(), "scan complete") {
		t.Error("a done panel with nil result should still render the completion line")
	}
}

// TestParseScanOutput proves the NDJSON tally distinguishes packages, findings,
// threats, and the pollen-unavailable status, and skips blank / malformed lines.
func TestParseScanOutput(t *testing.T) {
	data := strings.Join([]string{
		`{"record_type":"package"}`,
		`{"record_type":"package"}`,
		`{"record_type":"finding","decision":"allow"}`,
		`{"record_type":"finding","decision":"block"}`,
		`{"record_type":"scan_status","pollen_unavailable":true}`,
		``,             // blank line skipped
		`not-json`,     // malformed line skipped
	}, "\n")
	r := parseScanOutput([]byte(data))
	if r.packages != 2 {
		t.Errorf("packages = %d, want 2", r.packages)
	}
	if r.findings != 2 {
		t.Errorf("findings = %d, want 2", r.findings)
	}
	if r.threats != 1 {
		t.Errorf("threats = %d, want 1 (only the non-allow finding)", r.threats)
	}
	if !r.pollenUnavailable {
		t.Error("pollenUnavailable should be set from the scan_status record")
	}
}

// TestRunScanCmdReturnsCommand proves runScanCmd returns a non-nil command for a
// deep/quick panel. The closure is NOT executed here: invoking it would run a
// real scan (filesystem + Socket API), which is slow and non-deterministic; the
// scan engine itself is covered by internal/scan tests. buildScanConfig — the
// real logic this panel contributes — is covered separately below.
func TestRunScanCmdReturnsCommand(t *testing.T) {
	if NewScanPanel("deep").runScanCmd() == nil {
		t.Error("deep scan panel should return a runScanCmd command")
	}
	if NewScanPanel("quick").runScanCmd() == nil {
		t.Error("quick scan panel should return a runScanCmd command")
	}
}

// TestBuildScanConfigDeepFlag proves buildScanConfig threads the deep flag and
// resolves the expected catalog/audit paths under an isolated home.
func TestBuildScanConfigDeepFlag(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())
	cfg, ok := buildScanConfig(true)
	if !ok {
		t.Fatal("buildScanConfig should succeed with a resolvable home")
	}
	if !cfg.Deep {
		t.Error("deep flag not threaded into the scan config")
	}
	if !strings.HasSuffix(cfg.IndexPath, "bumblebee.idx") {
		t.Errorf("IndexPath = %q, want it to end in bumblebee.idx", cfg.IndexPath)
	}
	if !strings.HasSuffix(cfg.AuditPath, "beekeeper.ndjson") {
		t.Errorf("AuditPath = %q, want it to end in beekeeper.ndjson", cfg.AuditPath)
	}
}

var _ = audit.AuditRecord{}
