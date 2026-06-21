package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
	platform "github.com/home-beekeeper/beekeeper/internal/platform"
)

// --- NewAuditPanel + AuditPanel contract (audit_panel.go) ---

// TestNewAuditPanelHermetic constructs the public audit panel under an isolated
// home seeded with a real audit log, proving it loads history on open and
// exercising the Title/Count/Footer/Padded/Critical contract surface.
func TestNewAuditPanelHermetic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", home)
	auditDir, err := platform.AuditDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(auditDir, "beekeeper.ndjson")
	recs := []audit.AuditRecord{
		{RecordType: "policy_decision", RecordID: "r1", Decision: "block", ToolName: "Bash", Timestamp: "2026-06-18T09:00:00Z"},
		{RecordType: "sentry_alert", RecordID: "r2", SentrySeverity: "critical", SentryProcessExe: "node", Timestamp: "2026-06-18T09:01:00Z"},
	}
	var sb strings.Builder
	for _, r := range recs {
		w, _ := audit.NewWriter(path)
		_ = w.Write(r)
		_ = w.Close()
		_ = sb
	}

	p := NewAuditPanel()
	if len(p.records) == 0 {
		t.Fatal("NewAuditPanel should seed history from the audit tail")
	}
	if p.Title() != "Audit log" {
		t.Errorf("Title = %q", p.Title())
	}
	if !p.Padded() {
		t.Error("audit panel should be padded")
	}
	if p.Critical() {
		t.Error("audit panel must not be critical")
	}
	if !strings.Contains(p.Count(), "records") {
		t.Errorf("Count = %q, want it to mention records", p.Count())
	}
	if !strings.Contains(p.Footer(), "filter") {
		t.Errorf("Footer = %q, want a filter hint", p.Footer())
	}
	// Enforced filter footer + count branch.
	p.enforcedOnly = true
	if !strings.Contains(p.Footer(), "enforced") {
		t.Errorf("enforced Footer = %q, want it to read 'enforced'", p.Footer())
	}
	if !strings.Contains(p.Count(), "enforced") {
		t.Errorf("enforced Count = %q, want it to read 'enforced'", p.Count())
	}
}

// TestAuditPanelExpandedNavAndFilterKey covers the expanded-view key handling and
// the 'f' filter toggle plus the empty-filtered body branch.
func TestAuditPanelExpandedNavAndFilterKey(t *testing.T) {
	p := newTestAuditPanel(
		rec("policy_decision", audit.AuditRecord{ToolName: "a", Decision: "allow"}),
	)
	// enter expands, then left/backspace collapse (expanded-view branch).
	p.handleKey("enter")
	if !p.expanded {
		t.Fatal("enter should expand")
	}
	p.handleKey("left")
	if p.expanded {
		t.Fatal("left should collapse")
	}
	p.handleKey("enter")
	p.handleKey("backspace")
	if p.expanded {
		t.Fatal("backspace should collapse")
	}

	// 'f' enables the enforced filter; with only an allow record the filtered body
	// shows the enforced-empty message.
	p.handleKey("f")
	if !p.enforcedOnly {
		t.Fatal("f should toggle enforcedOnly on")
	}
	if !strings.Contains(p.Body(80, 20), "no blocks, warns, or alerts") {
		t.Errorf("filtered-empty body should show the enforced-empty message: %q", p.Body(80, 20))
	}
}

// TestAuditTsShortPlaceholder covers the unparseable-timestamp branch of tsShort.
func TestAuditTsShortPlaceholder(t *testing.T) {
	if got := tsShort("not-a-time"); got != "--:--:--" {
		t.Errorf("tsShort(bad) = %q, want placeholder", got)
	}
	if got := tsShort("2026-06-18T09:00:00Z"); got == "--:--:--" {
		t.Errorf("tsShort(valid) should format a time, got placeholder")
	}
}

// TestAuditBadgeForExtraBranches covers the clean-llmf, alert, and
// unknown-type badge branches not hit elsewhere. The retired nudge/version_drift
// record types (removed in v1.1.0) fall through to the decision-based badge.
func TestAuditBadgeForExtraBranches(t *testing.T) {
	cases := []struct {
		rec  audit.AuditRecord
		want string
	}{
		{rec("llmf_alert", audit.AuditRecord{LLMFResult: "clean"}), "LLMF"},
		{rec("policy_decision", audit.AuditRecord{Decision: "alert"}), "ALERT"},
		{rec("unknown_type", audit.AuditRecord{}), "EVENT"},
		// Retired record types with no decision fall through to EVENT.
		{rec("version_drift", audit.AuditRecord{}), "EVENT"},
		{rec("nudge", audit.AuditRecord{}), "EVENT"},
	}
	for _, c := range cases {
		if got, _ := badgeFor(c.rec); got != c.want {
			t.Errorf("badgeFor(%+v) = %q, want %q", c.rec, got, c.want)
		}
	}
}

// --- runPaletteSelection: drive every command branch (model.go) ---

// TestRunPaletteSelectionAllCommands proves each palette command dispatches to
// the correct next model (panel/toast). A temp home isolates the panels that
// seed config/policy files on construction.
func TestRunPaletteSelectionAllCommands(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())

	// Map command name → expected resulting panel (or "" for the toast-only case).
	wantPanel := map[string]panelKind{
		"alerts":      panelAlerts,
		"quarantine":  panelQuarantine,
		"audit tail":  panelAudit,
		"policy edit": panelPolicy,
		"settings":    panelSettings,
		"catalogs":    panelCatalogs,
		"scan now":    panelScan,
		"scan --quick": panelScan,
		"scan history": panelScan,
		"help":        panelHelp,
	}

	for i, c := range commands {
		a := NewApp(true)
		a.mode = modePalette
		a.palette = PaletteModel{selIdx: i}

		fn := a.runPaletteSelection()
		switch c.Name {
		case "quit":
			if fn != nil {
				t.Errorf("'quit' selection should return a nil dispatch fn")
			}
			continue
		case "protect install":
			if fn == nil {
				t.Fatalf("'protect install' should return a dispatch fn")
			}
			res := fn().(App)
			if !res.toast.visible || !strings.Contains(res.toast.msg, "protect install") {
				t.Errorf("'protect install' should surface a CLI-directing toast, got %q", res.toast.msg)
			}
			continue
		}

		want, ok := wantPanel[c.Name]
		if !ok {
			t.Fatalf("unhandled command in test map: %q", c.Name)
		}
		if fn == nil {
			t.Fatalf("%q should return a dispatch fn", c.Name)
		}
		res := fn().(App)
		if res.mode != modePanel || res.panel != want {
			t.Errorf("%q dispatched to mode=%d panel=%s, want panel %s", c.Name, res.mode, res.panel, want)
		}
	}
}

// TestRunPaletteSelectionNilWhenNoSelection proves a palette with no selectable
// command returns a nil dispatch fn.
func TestRunPaletteSelectionNilWhenNoSelection(t *testing.T) {
	a := NewApp(false)
	a.palette = PaletteModel{query: "zzz-no-match"}
	if a.runPaletteSelection() != nil {
		t.Error("runPaletteSelection with no selection should be nil")
	}
}

// --- alerts panel Update: filter mode + quarantine key (alerts_panel.go) ---

// TestAlertsFilterModeTyping drives the filter input sub-mode: '/' enters it,
// printable keys build the query, backspace trims, Esc clears.
func TestAlertsFilterModeTyping(t *testing.T) {
	p := NewAlertsPanel(false)
	p.rows = []AlertRow{{Label: "exfil", Meta: "net"}, {Label: "install", Meta: "pkg"}}

	p.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !p.filterMode {
		t.Fatal("'/' should enter filter mode")
	}
	for _, ch := range "exf" {
		p.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
	}
	if p.filterQuery != "exf" {
		t.Fatalf("filter query = %q, want exf", p.filterQuery)
	}
	if got := p.filteredRows(); len(got) != 1 || got[0].Label != "exfil" {
		t.Errorf("filter 'exf' should match only exfil, got %+v", got)
	}
	// Filter bar renders in the body while filtering.
	if !strings.Contains(p.Body(100, 20), "match") {
		t.Error("filter-mode body should render the match-count bar")
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if p.filterQuery != "ex" {
		t.Errorf("backspace query = %q, want ex", p.filterQuery)
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if p.filterMode || p.filterQuery != "" {
		t.Errorf("Esc should clear filter mode + query (mode=%v q=%q)", p.filterMode, p.filterQuery)
	}
}

// TestAlertsQuarantineKey proves 'q' on a selected row dispatches a
// quarantineAlertMsg carrying the row's RecordID.
func TestAlertsQuarantineKey(t *testing.T) {
	p := NewAlertsPanel(false)
	p.rows = []AlertRow{{Label: "exfil", RecordID: "rec-1"}}
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("'q' on a row should dispatch a quarantine command")
	}
	qa, ok := cmd().(quarantineAlertMsg)
	if !ok || qa.RecordID != "rec-1" {
		t.Fatalf("'q' produced %#v, want quarantineAlertMsg{rec-1}", cmd())
	}
}

// TestAlertsNavigationCollapsesDetail proves j/k navigation collapses an expanded
// detail view.
func TestAlertsNavigationCollapsesDetail(t *testing.T) {
	p := NewAlertsPanel(false)
	p.rows = []AlertRow{{Label: "a"}, {Label: "b"}}
	p.expanded = true
	p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if p.expanded {
		t.Error("down should collapse the detail view")
	}
	if p.selIdx != 1 {
		t.Errorf("down should advance selection, got %d", p.selIdx)
	}
	p.expanded = true
	p.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if p.expanded {
		t.Error("up should collapse the detail view")
	}
}

// TestAlertsEnterToggleExpand proves enter toggles the expanded detail view.
func TestAlertsEnterToggleExpand(t *testing.T) {
	p := NewAlertsPanel(false)
	p.rows = []AlertRow{{Label: "a"}}
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !p.expanded {
		t.Fatal("enter should expand")
	}
	p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if p.expanded {
		t.Fatal("enter again should collapse")
	}
}

// TestAlertsFooterStates covers the filter-mode and expanded-mode footer
// variants.
func TestAlertsFooterStates(t *testing.T) {
	p := NewAlertsPanel(false)
	p.filterMode = true
	if !strings.Contains(p.Footer(), "filter") {
		t.Errorf("filter-mode footer = %q", p.Footer())
	}
	p.filterMode = false
	p.expanded = true
	if !strings.Contains(p.Footer(), "collapse") {
		t.Errorf("expanded footer = %q, want a collapse hint", p.Footer())
	}
}

// TestAlertsCountActiveCritical covers the active-critical Count branch.
func TestAlertsCountActiveCritical(t *testing.T) {
	p := NewAlertsPanel(false)
	p.rows = []AlertRow{{IsCritical: true}, {IsCritical: false}}
	if !strings.Contains(p.Count(), "active critical") {
		t.Errorf("Count with a critical row = %q, want 'active critical'", p.Count())
	}
}

// TestRecordToRowSentryVariants covers the sentry severity high/medium badge
// branches and the files-only meta fallback.
func TestRecordToRowSentryVariants(t *testing.T) {
	p := NewAlertsPanel(false)

	// high severity → WARN badge; meta falls back to files when no ext/net.
	row, ok := p.recordToRow(audit.AuditRecord{
		RecordType: "sentry_alert", SentrySeverity: "high", SentryRuleID: "R-1",
		SentryFilesAccessed: []string{"~/.aws/credentials", "~/.ssh/id", "x", "y"},
		Timestamp:           "2026-06-18T09:00:00Z",
	})
	if !ok {
		t.Fatal("high-severity sentry alert should produce a row")
	}
	if row.Label != "R-1" {
		t.Errorf("label should fall back to rule ID, got %q", row.Label)
	}
	if !strings.Contains(row.Meta, "~/.aws/credentials") {
		t.Errorf("meta should fall back to the first files, got %q", row.Meta)
	}

	// medium severity with a correlated extension + network dest → joined meta.
	row2, _ := p.recordToRow(audit.AuditRecord{
		RecordType: "sentry_alert", SentrySeverity: "medium", SentryRuleName: "rule-x",
		SentryCorrelatedExt: "acme.evil", SentryNetworkDests: []string{"1.2.3.4:443"},
		Timestamp: "2026-06-18T09:00:00Z",
	})
	if !strings.Contains(row2.Meta, "acme.evil") || !strings.Contains(row2.Meta, "1.2.3.4:443") {
		t.Errorf("meta should join ext + first network dest, got %q", row2.Meta)
	}

	// A non-alert, non-decision record is dropped.
	if _, ok := p.recordToRow(audit.AuditRecord{RecordType: "package"}); ok {
		t.Error("a package record should not become an alert row")
	}
}

// TestMinMaxHelpers covers both branches of minInt and max.
func TestMinMaxHelpers(t *testing.T) {
	if minInt(2, 5) != 2 || minInt(5, 2) != 2 {
		t.Error("minInt branches")
	}
	if max(2, 5) != 5 || max(5, 2) != 5 {
		t.Error("max branches")
	}
}

// TestIncidentModelNavBothDirections covers the up/left branch of the incident
// action selector (the down/right branch is covered elsewhere).
func TestIncidentModelNavBothDirections(t *testing.T) {
	m := IncidentFromRecord(sampleSentryRecord())
	m.SelAction = 1
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.SelAction != 0 {
		t.Errorf("left should decrement SelAction, got %d", m.SelAction)
	}
	// up at 0 clamps.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.SelAction != 0 {
		t.Errorf("up at 0 should clamp, got %d", m.SelAction)
	}
}

// --- StartResizePoller (resize_windows.go on this host) ---

// TestStartResizePoller proves the poller launches without panicking. It sends
// WindowSizeMsg to a real headless program; we cancel via the program context
// after confirming the goroutine started (the goroutine is detached and exits
// when the process does — we only assert the launch is safe).
func TestStartResizePoller(t *testing.T) {
	got := make(chan struct{}, 1)
	model := resizeProbeModel{got: got}
	p := tea.NewProgram(model,
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
		tea.WithInput(nil),
	)
	StartResizePoller(p)

	done := make(chan struct{})
	go func() { _, _ = p.Run(); close(done) }()

	select {
	case <-got:
		// Received at least one resize tick — the poller is alive.
	case <-time.After(10 * time.Second):
		p.Quit()
		<-done
		t.Fatal("resize poller did not deliver a WindowSizeMsg within the deadline")
	}
	p.Quit()
	<-done
}

type resizeProbeModel struct{ got chan struct{} }

func (m resizeProbeModel) Init() tea.Cmd { return nil }
func (m resizeProbeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		select {
		case m.got <- struct{}{}:
		default:
		}
		return m, tea.Quit
	}
	return m, nil
}
func (m resizeProbeModel) View() tea.View { return tea.NewView("") }
