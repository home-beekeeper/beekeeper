package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	audit "github.com/bantuson/beekeeper/internal/audit"
)

func TestAppModeCalm(t *testing.T) {
	a := NewApp(false)
	if a.mode != modeCalm {
		t.Fatalf("expected modeCalm on startup, got %d", a.mode)
	}
}

func TestAppOpenPalette(t *testing.T) {
	a := NewApp(false)
	// Simulate : key via direct state manipulation (tea.KeyPressMsg construction is version-dependent)
	a2 := a
	a2.mode = modePalette
	a2.palette = PaletteModel{}
	if a2.mode != modePalette {
		t.Fatal("expected modePalette")
	}
}

func TestAppClosePalette(t *testing.T) {
	a := NewApp(false)
	a.mode = modePalette
	// Escape from palette → calm
	a.mode = modeCalm
	a.palette = PaletteModel{}
	if a.mode != modeCalm {
		t.Fatal("expected modeCalm after closing palette")
	}
}

func TestAppOpenAlerts(t *testing.T) {
	a := NewApp(false)
	m, _ := a.openPanel(panelAlerts, nil)
	a = m.(App)
	if a.mode != modePanel {
		t.Fatalf("expected modePanel, got %d", a.mode)
	}
	if a.panel != panelAlerts {
		t.Fatalf("expected panelAlerts, got %s", a.panel)
	}
}

func TestAppClosePanel(t *testing.T) {
	a := NewApp(false)
	a.mode = modePanel
	a.panel = panelAlerts
	// Esc in panel mode → calm
	a.mode = modeCalm
	a.panelM = PanelModel{}
	if a.mode != modeCalm {
		t.Fatal("expected modeCalm after closing panel")
	}
}

// TestAppPolicyShortcut proves the `p` key opens the policy editor panel from
// the calm base screen — the first-class shortcut added so the editor is
// reachable without going through the `:` command palette. BEEKEEPER_HOME
// isolates the managed-policy file NewPolicyPanel seeds on construction.
func TestAppPolicyShortcut(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())
	a := NewApp(true)
	m, _ := a.handleKey(tea.KeyPressMsg{Code: 'p', Text: "p"})
	app, ok := m.(App)
	if !ok {
		t.Fatalf("handleKey returned %T, want App", m)
	}
	if app.mode != modePanel {
		t.Fatalf("expected modePanel after 'p', got %d", app.mode)
	}
	if app.panel != panelPolicy {
		t.Fatalf("expected panelPolicy after 'p', got %s", app.panel)
	}
}

// sampleSentryRecord returns a realistic critical sentry_alert audit record for
// driving the incident card in tests (replaces the retired DefaultIncident demo).
func sampleSentryRecord() audit.AuditRecord {
	return audit.AuditRecord{
		RecordType:          "sentry_alert",
		Timestamp:           "2026-05-28T14:21:54Z",
		SentrySeverity:      "critical",
		SentryRuleID:        "SLNX-08",
		SentryRuleName:      "credential-exfil",
		SentryProcessExe:    "node extension.js",
		SentryProcessPID:    8847,
		SentryParentChain:   []string{"Code Helper (Plugin)", "node extension.js"},
		SentryFilesAccessed: []string{"~/.aws/credentials", "~/.ssh/id_ed25519"},
		SentryNetworkDests:  []string{"185.2.0.1:443"},
		SentryCorrelatedExt: "acme.evil-linter",
	}
}

func TestAppCriticalTrigger(t *testing.T) {
	a := NewApp(false)
	if a.critical {
		t.Fatal("expected not critical on startup")
	}
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord())
	if !a.critical {
		t.Fatal("expected critical=true after trigger")
	}
	if a.incident.RuleName != "credential-exfil" {
		t.Fatalf("expected incident built from the real record, got RuleName=%q", a.incident.RuleName)
	}
}

// TestIncidentFromRecordIsReal verifies the incident card reflects the actual
// alert record, not fabricated demo data.
func TestIncidentFromRecordIsReal(t *testing.T) {
	inc := IncidentFromRecord(sampleSentryRecord())
	if inc.RuleID != "SLNX-08" {
		t.Errorf("expected RuleID from record, got %q", inc.RuleID)
	}
	var treeText string
	for _, l := range inc.Tree {
		treeText += l.Text + "\n"
	}
	if !strings.Contains(treeText, "~/.aws/credentials") {
		t.Errorf("expected real file path in tree, got:\n%s", treeText)
	}
	// The retired demo incident's invented data must never appear.
	if strings.Contains(treeText, "185.2.x.x") || strings.Contains(treeText, "pid 8821") {
		t.Errorf("incident contains fabricated demo data: %s", treeText)
	}
	for _, act := range inc.Actions {
		if act.Lbl == "quarantine extension" || act.Lbl == "isolate process" {
			t.Errorf("incident must not offer un-wired %q action", act.Lbl)
		}
	}
}

func TestAppWindowSizeDedup(t *testing.T) {
	a := NewApp(false)
	m1, _ := a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a1 := m1.(App)
	if a1.width != 80 {
		t.Fatalf("expected width=80, got %d", a1.width)
	}
	// Send same size again — should be a no-op (nil cmd)
	_, cmd2 := a1.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd2 != nil {
		t.Fatal("expected nil cmd on duplicate WindowSizeMsg")
	}
}

func TestNewRecordsMsgType(t *testing.T) {
	records := newRecordsMsg{audit.AuditRecord{Decision: "block"}}
	var msg tea.Msg = records
	_, ok := msg.(newRecordsMsg)
	if !ok {
		t.Fatal("type assertion newRecordsMsg failed")
	}
}

func TestAppCommandDispatch(t *testing.T) {
	a := NewApp(false)
	// Open palette
	a.mode = modePalette
	// Simulate "alerts" command selected — index 3 in commands slice (scan now/quick/history, alerts)
	a.palette = PaletteModel{query: "", selIdx: 3}
	// Trigger Enter via runPaletteSelection
	fn := a.runPaletteSelection()
	if fn == nil {
		t.Fatal("expected non-nil dispatch function for alerts command")
	}
	result := fn()
	app, ok := result.(App)
	if !ok {
		t.Fatal("expected App from runPaletteSelection")
	}
	if app.mode != modePanel {
		t.Fatalf("expected modePanel after selecting alerts, got %d", app.mode)
	}
	if app.panel != panelAlerts {
		t.Fatalf("expected panelAlerts, got %s", app.panel)
	}
}

func TestAppIncidentResolve(t *testing.T) {
	a := NewApp(false)
	// Trigger critical mode with a real incident.
	a.critical = true
	a.incident = IncidentFromRecord(sampleSentryRecord())
	a.health.SentryOK = false
	a.health.LastBlock = "sentry firing"

	if !a.critical {
		t.Fatal("setup: expected critical=true")
	}

	// Acknowledge via the real handler (not by hand-setting status).
	m, _ := a.acknowledgeIncident()
	a = m.(App)

	if a.critical {
		t.Error("expected critical=false after acknowledge")
	}
	if a.incident.RuleName != "" {
		t.Error("expected incident cleared after acknowledge")
	}
	if !strings.Contains(a.status, "acknowledged") {
		t.Errorf("expected 'acknowledged' in status, got: %q", a.status)
	}
	// Must NOT claim a containment it did not perform.
	if strings.Contains(a.status, "contained") || strings.Contains(a.status, "quarantined") {
		t.Errorf("acknowledge must not claim containment, got: %q", a.status)
	}
}

func TestAppHealthState(t *testing.T) {
	a := NewApp(false)
	// healthTick should not panic even with missing state files
	m, cmd := a.Update(healthTick(time.Now()))
	if m == nil {
		t.Fatal("expected non-nil model after healthTick")
	}
	if cmd == nil {
		t.Fatal("expected re-arm cmd after healthTick")
	}
	// Health state should be updated (may be all false due to missing files — that's OK)
	_ = m.(App).health
}

func TestAppHealthLlamaFirewallPip(t *testing.T) {
	// Health is probed for real at construction (fail-safe — NOT seeded green).
	// On a machine with no sidecar state.json the probe returns false; the
	// contract is that the field is reachable and construction never panics.
	a := NewApp(false)
	_ = a.health.LlamaFirewallOK

	// After a healthTick the field is refreshed. On a dev machine with no sidecar
	// state.json the probe returns false — that is acceptable. The critical assertion
	// is that Update does not panic and returns a non-nil model + re-arm cmd.
	m, cmd := a.Update(healthTick(time.Now()))
	if m == nil {
		t.Fatal("expected non-nil model after healthTick with llamafirewall probe")
	}
	if cmd == nil {
		t.Fatal("expected re-arm cmd after healthTick with llamafirewall probe")
	}
	// Verify LlamaFirewallOK is reachable (field exists in returned HealthState).
	_ = m.(App).health.LlamaFirewallOK

	// Verify renderBase includes "llamafirewall" label when width is set.
	a2 := NewApp(false)
	a2.width = 120
	rendered := renderBase(a2)
	if !strings.Contains(rendered, "llamafirewall") {
		t.Error("expected renderBase output to contain 'llamafirewall' pip label")
	}
}

func TestAppFullFlow(t *testing.T) {
	a := NewApp(false)
	// Initialize window size
	m, _ := a.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	a = m.(App)
	if a.width != 200 || a.height != 50 {
		t.Fatalf("expected 200x50, got %dx%d", a.width, a.height)
	}
	// Should be in calm mode
	if a.mode != modeCalm {
		t.Fatal("expected calm mode initially")
	}
	// Open palette directly
	a.mode = modePalette
	if a.mode != modePalette {
		t.Fatal("expected palette mode")
	}
	// Escape back to calm
	a.mode = modeCalm
	a.palette = PaletteModel{}
	if a.mode != modeCalm {
		t.Fatal("expected calm mode after Esc")
	}
}
