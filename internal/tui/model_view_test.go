package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
)

// renderView is a small helper that extracts the rendered string from App.View.
func renderView(a App) string {
	v := a.View()
	return v.Content
}

// TestViewCalmMode proves the calm base screen renders through View and that
// AltScreen is requested.
func TestViewCalmMode(t *testing.T) {
	a := NewApp(false)
	a.width, a.height = 120, 40
	v := a.View()
	if !v.AltScreen {
		t.Error("View should enable AltScreen")
	}
	out := v.Content
	if !strings.Contains(out, "BEEKEEPER") {
		t.Errorf("calm View missing brand:\n%s", out)
	}
}

// TestViewPaletteMode proves the palette overlay renders over the dimmed base.
func TestViewPaletteMode(t *testing.T) {
	a := NewApp(false)
	a.width, a.height = 120, 40
	a.mode = modePalette
	a.palette = PaletteModel{}
	out := renderView(a)
	// The palette lists the locked command groups.
	if !strings.Contains(out, "SCAN") {
		t.Errorf("palette View missing command groups:\n%s", out)
	}
}

// TestViewPanelMode proves a panel overlay renders through View.
func TestViewPanelMode(t *testing.T) {
	a := NewApp(false)
	a.width, a.height = 120, 40
	m, _ := a.openPanel(panelHelp, NewHelpPanel())
	out := renderView(m.(App))
	if !strings.Contains(out, "NAVIGATION") {
		t.Errorf("panel-mode View missing help body:\n%s", out)
	}
}

// TestViewWithToast proves an armed toast is appended to the view.
func TestViewWithToast(t *testing.T) {
	a := NewApp(false)
	a.width, a.height = 120, 40
	a.toast, _ = a.toast.Show("hello-toast", toastOK)
	out := renderView(a)
	if !strings.Contains(out, "hello-toast") {
		t.Errorf("View did not append the visible toast:\n%s", out)
	}
}

// TestViewCriticalIncident proves the incident card renders in the calm base
// screen when critical, and that the red status line is used.
func TestViewCriticalIncident(t *testing.T) {
	a := NewApp(false)
	a.width, a.height = 140, 50
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord())
	a.status = "⚠ 1 CRITICAL: credential-exfil"
	out := renderView(a)
	if !strings.Contains(out, "credential-exfil") {
		t.Errorf("critical View missing the incident rule name:\n%s", out)
	}
}

// TestInitReturnsBatch proves Init wires up the background timer commands.
func TestInitReturnsBatch(t *testing.T) {
	a := NewApp(false)
	if cmd := a.Init(); cmd == nil {
		t.Fatal("Init must return a non-nil batch of timer commands")
	}
}

// TestViewStringHelper covers the viewString test-introspection helper.
func TestViewStringHelper(t *testing.T) {
	a := NewApp(false)
	a.mode = modePanel
	a.panel = panelAudit
	a.status = "monitoring"
	s := a.viewString()
	for _, want := range []string{"mode=2", "panel=audit", "status=monitoring"} {
		if !strings.Contains(s, want) {
			t.Errorf("viewString = %q, missing %q", s, want)
		}
	}
}

// --- handleKey branch coverage (model.go) ---

// TestHandleKeyCalmShortcuts drives each calm-mode global key and asserts the
// resulting mode/panel transition.
func TestHandleKeyCalmShortcuts(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())

	cases := []struct {
		key       string
		wantMode  mode
		wantPanel panelKind
	}{
		{":", modePalette, ""},
		{"!", modePanel, panelAlerts},
		{"s", modePanel, panelSettings},
		{"?", modePanel, panelHelp},
	}
	for _, c := range cases {
		a := NewApp(true)
		m, _ := a.handleKey(keyText(c.key))
		app := m.(App)
		if app.mode != c.wantMode {
			t.Errorf("key %q: mode = %d, want %d", c.key, app.mode, c.wantMode)
		}
		if c.wantPanel != "" && app.panel != c.wantPanel {
			t.Errorf("key %q: panel = %s, want %s", c.key, app.panel, c.wantPanel)
		}
	}
}

// TestHandleKeyGoToMenu proves 'g' opens the palette pre-seeded with the "go"
// query.
func TestHandleKeyGoToMenu(t *testing.T) {
	a := NewApp(false)
	m, _ := a.handleKey(keyText("g"))
	app := m.(App)
	if app.mode != modePalette {
		t.Fatalf("'g' should open palette, got mode %d", app.mode)
	}
	if app.palette.query != "go" {
		t.Errorf("'g' should seed query 'go', got %q", app.palette.query)
	}
}

// TestHandleKeyQuit proves 'q' returns the tea.Quit command.
func TestHandleKeyQuit(t *testing.T) {
	a := NewApp(false)
	_, cmd := a.handleKey(keyText("q"))
	if cmd == nil {
		t.Fatal("'q' should return the Quit command")
	}
}

// TestHandleKeyPaletteEscClosesAndQuit covers palette-mode key dispatch: Esc
// closes back to calm, and enter on the "quit" command returns tea.Quit.
func TestHandleKeyPaletteEscAndQuit(t *testing.T) {
	a := NewApp(false)
	a.mode = modePalette
	a.palette = PaletteModel{}
	m, _ := a.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.(App).mode != modeCalm {
		t.Errorf("Esc in palette should return to calm")
	}

	// Select the "quit" command and press enter.
	a2 := NewApp(false)
	a2.mode = modePalette
	a2.palette = PaletteModel{selIdx: len(commands) - 1} // "quit" is last
	if commands[len(commands)-1].Name != "quit" {
		t.Fatalf("test assumption broken: last command is %q", commands[len(commands)-1].Name)
	}
	_, cmd := a2.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter on 'quit' should return the Quit command")
	}
}

// TestHandleKeyPaletteEnterOpensPanel proves enter on a non-quit selection
// dispatches the command (opening the chosen panel).
func TestHandleKeyPaletteEnterOpensPanel(t *testing.T) {
	a := NewApp(false)
	a.mode = modePalette
	a.palette = PaletteModel{selIdx: 3} // "alerts"
	m, _ := a.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	app := m.(App)
	if app.mode != modePanel || app.panel != panelAlerts {
		t.Errorf("enter on 'alerts' should open the alerts panel; got mode=%d panel=%s", app.mode, app.panel)
	}
}

// TestHandleKeyPaletteTypingFilters proves a printable key in palette mode is
// forwarded to the palette filter.
func TestHandleKeyPaletteTypingFilters(t *testing.T) {
	a := NewApp(false)
	a.mode = modePalette
	a.palette = PaletteModel{}
	m, _ := a.handleKey(keyText("a"))
	if m.(App).palette.query != "a" {
		t.Errorf("typing in palette should append to the query, got %q", m.(App).palette.query)
	}
}

// TestHandleKeyPanelEscCloses proves Esc in panel mode returns to calm and clears
// the panel model.
func TestHandleKeyPanelEscCloses(t *testing.T) {
	a := NewApp(false)
	m, _ := a.openPanel(panelHelp, NewHelpPanel())
	a = m.(App)
	m2, _ := a.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m2.(App).mode != modeCalm {
		t.Error("Esc in panel mode should return to calm")
	}
}

// TestHandleKeyPanelForwardsToContent proves a non-Esc key in panel mode is
// delegated to the wrapped panel content.
func TestHandleKeyPanelForwardsToContent(t *testing.T) {
	a := NewApp(false)
	ap := NewAlertsPanel(false)
	ap.rows = []AlertRow{{Label: "a"}, {Label: "b"}}
	m, _ := a.openPanel(panelAlerts, ap)
	a = m.(App)
	m2, _ := a.handleKey(keyText("j"))
	got := m2.(App).panelM.content.(*AlertsPanel)
	if got.selIdx != 1 {
		t.Errorf("panel-mode key should reach the content (selIdx=%d, want 1)", got.selIdx)
	}
}

// --- critical-incident keys (model.go handleKey + doIncidentAction) ---

// TestHandleKeyCriticalAcknowledge proves 'a' in critical mode clears the
// incident.
func TestHandleKeyCriticalAcknowledge(t *testing.T) {
	a := NewApp(false)
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord())
	m, _ := a.handleKey(keyText("a"))
	if m.(App).critical {
		t.Error("'a' in critical mode should acknowledge and clear critical")
	}
}

// TestHandleKeyCriticalDetail proves 'd' in critical mode opens the alerts panel
// WITHOUT resolving critical.
func TestHandleKeyCriticalDetail(t *testing.T) {
	a := NewApp(false)
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord())
	m, _ := a.handleKey(keyText("d"))
	app := m.(App)
	if !app.critical {
		t.Error("'d' must keep critical mode active")
	}
	if app.mode != modePanel || app.panel != panelAlerts {
		t.Errorf("'d' should open the alerts panel; got mode=%d panel=%s", app.mode, app.panel)
	}
}

// TestHandleKeyCriticalNavigation proves arrow keys in critical mode are routed
// to the incident's action selector.
func TestHandleKeyCriticalNavigation(t *testing.T) {
	a := NewApp(false)
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord()) // 2 actions
	m, _ := a.handleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.(App).incident.SelAction != 1 {
		t.Errorf("right in critical mode should advance the incident action selector, got %d", m.(App).incident.SelAction)
	}
}

// TestDoIncidentActionAcknowledge proves enter on the "acknowledge" action
// (SelAction 0) clears critical.
func TestDoIncidentActionAcknowledge(t *testing.T) {
	a := NewApp(false)
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord())
	a.incident.SelAction = 0 // "a" acknowledge
	m, _ := a.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.(App).critical {
		t.Error("enter on the acknowledge action should clear critical")
	}
}

// TestDoIncidentActionFullRecord proves enter on the "full record" action (d)
// opens the alerts panel.
func TestDoIncidentActionFullRecord(t *testing.T) {
	a := NewApp(false)
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord())
	a.incident.SelAction = 1 // "d" full record
	m, _ := a.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	app := m.(App)
	if app.mode != modePanel || app.panel != panelAlerts {
		t.Errorf("enter on full-record should open alerts; got mode=%d panel=%s", app.mode, app.panel)
	}
}

// TestDoIncidentActionNoActions proves doIncidentAction is a safe no-op when the
// incident carries no actions.
func TestDoIncidentActionNoActions(t *testing.T) {
	a := NewApp(false)
	a.critical = true
	a.incident = IncidentModel{} // no actions
	m, cmd := a.doIncidentAction()
	if cmd != nil {
		t.Error("doIncidentAction with no actions should return nil cmd")
	}
	if !m.(App).critical {
		t.Error("doIncidentAction with no actions should not change state")
	}
}

// --- Update message-type coverage (model.go) ---

// TestUpdateScanResultRoutedToPanel proves a scanResultMsg in panel mode is
// delivered to the open panel.
func TestUpdateScanMessagesRouted(t *testing.T) {
	a := NewApp(false)
	sp := NewScanPanel("deep")
	m, _ := a.openPanel(panelScan, sp)
	a = m.(App)

	// stepTickMsg routes and re-arms.
	m2, cmd := a.Update(stepTickMsg{})
	if cmd == nil {
		t.Error("stepTickMsg in panel mode should re-arm the step ticker")
	}
	a = m2.(App)

	// scanResultMsg marks the panel done.
	m3, _ := a.Update(scanResultMsg{res: scanResult{packages: 1}})
	got := m3.(App).panelM.content.(*ScanPanel)
	if !got.done {
		t.Error("scanResultMsg should mark the open scan panel done")
	}
}

// TestUpdateScanMessagesIgnoredInCalm proves scan animation/result messages are
// ignored when no panel is open.
func TestUpdateScanMessagesIgnoredInCalm(t *testing.T) {
	a := NewApp(false)
	if _, cmd := a.Update(stepTickMsg{}); cmd != nil {
		t.Error("stepTickMsg in calm mode should be a no-op")
	}
	if _, cmd := a.Update(scanResultMsg{}); cmd != nil {
		t.Error("scanResultMsg in calm mode should be a no-op")
	}
}

// TestUpdateToastMessages covers the toast-emitting message branches.
func TestUpdateToastMessages(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.Msg
		want string
	}{
		{"sync-progress", syncCatalogsMsg{}, "Syncing"},
		{"sync-done-ok", syncDoneMsg{count: 7}, "Synced 7"},
		{"sync-done-unchanged", syncDoneMsg{notModified: true}, "already fresh"},
		{"policy-saved", policySavedMsg{msg: "saved-x"}, "saved-x"},
		{"policy-err", policyEditErrMsg{msg: "rejected-x"}, "rejected-x"},
		{"settings-saved", settingsSavedMsg{msg: "ssaved"}, "ssaved"},
		{"settings-err", settingsEditErrMsg{msg: "serr"}, "serr"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := NewApp(false)
			a.width = 100
			m, cmd := a.Update(c.msg)
			if cmd == nil {
				t.Errorf("%s should emit a toast cmd", c.name)
			}
			if !m.(App).toast.visible {
				t.Errorf("%s should make a toast visible", c.name)
			}
			if !strings.Contains(m.(App).toast.msg, c.want) {
				t.Errorf("%s toast = %q, want it to contain %q", c.name, m.(App).toast.msg, c.want)
			}
		})
	}
}

// TestUpdateSyncDoneErrorTruncates proves a long sync error is truncated into the
// toast.
func TestUpdateSyncDoneErrorTruncates(t *testing.T) {
	a := NewApp(false)
	long := strings.Repeat("x", 120)
	m, _ := a.Update(syncDoneMsg{err: &stubErr{long}})
	got := m.(App).toast.msg
	if !strings.Contains(got, "Catalog sync failed") {
		t.Errorf("expected failure prefix, got %q", got)
	}
	if strings.Contains(got, long) {
		t.Errorf("long error should be truncated, got %q", got)
	}
	if !strings.Contains(got, "...") {
		t.Errorf("truncated error should end with ..., got %q", got)
	}
}

type stubErr struct{ s string }

func (e *stubErr) Error() string { return e.s }

// TestUpdateQuarantineAlertClosesPanel proves the quarantineAlertMsg closes the
// panel and shows a toast.
func TestUpdateQuarantineAlertClosesPanel(t *testing.T) {
	a := NewApp(false)
	m0, _ := a.openPanel(panelAlerts, NewAlertsPanel(false))
	a = m0.(App)
	m, cmd := a.Update(quarantineAlertMsg{RecordID: "abc"})
	if cmd == nil {
		t.Error("quarantineAlertMsg should emit a toast cmd")
	}
	app := m.(App)
	if app.mode != modeCalm {
		t.Error("quarantineAlertMsg should close the panel (return to calm)")
	}
	if !app.toast.visible {
		t.Error("quarantineAlertMsg should show a toast")
	}
}

// TestUpdateNewRecordsTriggersCritical proves a critical sentry_alert in a
// newRecordsMsg flips the App into critical mode with an incident built from the
// real record.
func TestUpdateNewRecordsTriggersCritical(t *testing.T) {
	a := NewApp(false)
	rec := sampleSentryRecord()
	m, _ := a.Update(newRecordsMsg{rec})
	app := m.(App)
	if !app.critical {
		t.Fatal("a critical sentry_alert should put the App in critical mode")
	}
	if app.incident.RuleName != "credential-exfil" {
		t.Errorf("incident should be built from the real record, got %q", app.incident.RuleName)
	}
	if app.health.SentryOK {
		t.Error("a live critical should mark SentryOK=false")
	}
}

// catalogQuarantineRecord builds an FRSP audit record for the Gap-4 TUI tests.
func catalogQuarantineRecord(recordType, pkg string) audit.AuditRecord {
	return audit.AuditRecord{
		RecordType: recordType,
		ToolName:   pkg,
		RuleIDs:    []string{"FRSP-01"},
		Timestamp:  makeTS(0),
		Reason:     "catalog match: 2 sources corroborated",
	}
}

// TestUpdateCatalogQuarantineRaisesCard proves a background catalog_quarantine
// record raises the quarantine card (with human-gated [r]/[p]/[a]) WITHOUT
// entering sentry-critical mode.
func TestUpdateCatalogQuarantineRaisesCard(t *testing.T) {
	a := NewApp(false)
	m, _ := a.Update(newRecordsMsg{catalogQuarantineRecord("catalog_quarantine", "evil-pkg")})
	app := m.(App)
	if app.critical {
		t.Error("a catalog_quarantine must not set sentry-critical")
	}
	if !app.quarantineAlert {
		t.Fatal("a catalog_quarantine should raise the quarantine card")
	}
	keys := map[string]bool{}
	for _, act := range app.quarantineIncident.Actions {
		keys[act.Key] = true
	}
	if !keys["r"] || !keys["p"] || !keys["a"] {
		t.Errorf("catalog-quarantine card should expose [r]/[p]/[a], got %+v", app.quarantineIncident.Actions)
	}
}

// TestUpdatePendingQuarantineAcknowledgeOnly proves a pending-quarantine record
// raises an acknowledge-only card (nothing to restore/purge yet).
func TestUpdatePendingQuarantineAcknowledgeOnly(t *testing.T) {
	a := NewApp(false)
	m, _ := a.Update(newRecordsMsg{catalogQuarantineRecord("pending-quarantine", "evil-wheel")})
	app := m.(App)
	if !app.quarantineAlert {
		t.Fatal("a pending-quarantine should raise the quarantine card")
	}
	for _, act := range app.quarantineIncident.Actions {
		if act.Key == "r" || act.Key == "p" {
			t.Errorf("pending-quarantine must not expose restore/purge, got %q", act.Key)
		}
	}
}

// TestSentryCriticalPreemptsQuarantine proves a sentry critical takes precedence
// over a showing background quarantine card.
func TestSentryCriticalPreemptsQuarantine(t *testing.T) {
	a := NewApp(false)
	m, _ := a.Update(newRecordsMsg{catalogQuarantineRecord("catalog_quarantine", "evil-pkg")})
	a = m.(App)
	if !a.quarantineAlert {
		t.Fatal("setup: quarantine card should be showing")
	}
	m2, _ := a.Update(newRecordsMsg{sampleSentryRecord()})
	app := m2.(App)
	if !app.critical {
		t.Fatal("sentry critical should preempt and set critical")
	}
	if app.quarantineAlert {
		t.Error("sentry critical should clear the lower-precedence quarantine card")
	}
}

// TestHandleKeyQuarantineAcknowledge proves 'a' dismisses the quarantine card.
func TestHandleKeyQuarantineAcknowledge(t *testing.T) {
	a := NewApp(false)
	a.quarantineAlert = true
	a.quarantineIncident = CatalogQuarantineIncidentFromRecord(catalogQuarantineRecord("catalog_quarantine", "evil-pkg"), false)
	m, _ := a.handleKey(keyText("a"))
	if m.(App).quarantineAlert {
		t.Error("'a' should acknowledge and clear the quarantine card")
	}
}

// TestHandleKeyQuarantineRestoreOpensPanel proves 'r' on the card opens the
// quarantine panel (where restore runs admin-gated) and clears the card.
func TestHandleKeyQuarantineRestoreOpensPanel(t *testing.T) {
	a := NewApp(false)
	a.quarantineAlert = true
	a.quarantineIncident = CatalogQuarantineIncidentFromRecord(catalogQuarantineRecord("catalog_quarantine", "evil-pkg"), false)
	m, _ := a.handleKey(keyText("r"))
	app := m.(App)
	if app.quarantineAlert {
		t.Error("'r' should clear the card")
	}
	if app.mode != modePanel || app.panel != panelQuarantine {
		t.Errorf("'r' should open the quarantine panel; got mode=%d panel=%s", app.mode, app.panel)
	}
}

// TestUpdateNewRecordsRoutedToPanel proves newRecordsMsg is also delivered to an
// open panel.
func TestUpdateNewRecordsRoutedToPanel(t *testing.T) {
	a := NewApp(false)
	m0, _ := a.openPanel(panelAlerts, NewAlertsPanel(false))
	a = m0.(App)
	m, _ := a.Update(newRecordsMsg{audit.AuditRecord{
		RecordType: "policy_decision", Decision: "block", ToolName: "Bash", Timestamp: makeTS(0),
	}})
	got := m.(App).panelM.content.(*AlertsPanel)
	if len(got.rows) != 1 {
		t.Errorf("newRecordsMsg should reach the open alerts panel, rows=%d", len(got.rows))
	}
}

// TestUpdateClockTick proves the clock tick updates the clock and re-arms.
func TestUpdateClockTick(t *testing.T) {
	a := NewApp(false)
	stamp := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	m, cmd := a.Update(clockMsg(stamp))
	if cmd == nil {
		t.Error("clock tick should re-arm")
	}
	if !m.(App).clock.Equal(stamp) {
		t.Errorf("clock not updated, got %v", m.(App).clock)
	}
}

// TestUpdateStateTickReArms proves the state tick re-arms (and routes to the
// panel when one is open).
func TestUpdateStateTickReArms(t *testing.T) {
	a := NewApp(false)
	if _, cmd := a.Update(stateTick(time.Now())); cmd == nil {
		t.Error("stateTick should re-arm")
	}
}

// TestUpdateToastHideRouted proves toastHideMsg is forwarded to the toast model.
func TestUpdateToastHideRouted(t *testing.T) {
	a := NewApp(false)
	a.toast, _ = a.toast.Show("x", toastOK)
	m, _ := a.Update(toastHideMsg{})
	if m.(App).toast.visible {
		t.Error("toastHideMsg should hide the toast")
	}
}

// --- base.go ---

// TestRenderBaseDimmed proves the dimmed wrapper still contains the base content
// (the overlay backdrop).
func TestRenderBaseDimmed(t *testing.T) {
	a := NewApp(false)
	a.width = 120
	out := renderBaseDimmed(a)
	if !strings.Contains(out, "BEEKEEPER") {
		t.Errorf("renderBaseDimmed lost the base content:\n%s", out)
	}
}

// TestRenderBaseCriticalSentryPulse exercises the critical sentry-pip pulse
// branch (both clock-second parities) and the red status line.
func TestRenderBaseCriticalSentryPulse(t *testing.T) {
	a := NewApp(false)
	a.width = 120
	a.critical = true
	a.health.SentryOK = false
	a.status = "⚠ critical"
	a.incident = IncidentFromRecord(sampleSentryRecord())

	a.clock = time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC) // even second
	even := renderBase(a)
	a.clock = time.Date(2026, 6, 18, 0, 0, 1, 0, time.UTC) // odd second
	odd := renderBase(a)
	if even == odd {
		t.Error("critical sentry pip should pulse between even/odd seconds")
	}
	if !strings.Contains(even, "credential-exfil") {
		t.Error("critical base should render the incident card")
	}
}
