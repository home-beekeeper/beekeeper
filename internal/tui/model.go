package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
	platform "github.com/home-beekeeper/beekeeper/internal/platform"
)

type mode int

const (
	modeCalm    mode = iota
	modePalette      // command palette overlay open
	modePanel        // a panel overlay is open
)

// HealthState holds health pip states read from IPC + audit.
type HealthState struct {
	HooksOK         bool
	GatewayOK       bool
	SentryOK        bool
	CatalogsOK      bool
	LlamaFirewallOK bool
	LastBlock       string // e.g. "6m ago", "just now", "sentry firing"
}

// App is the top-level Bubble Tea model.
type App struct {
	mode      mode
	critical  bool
	panel     panelKind
	palette   PaletteModel
	panelM    PanelModel
	incident  IncidentModel
	// quarantineAlert + quarantineIncident hold the catalog-quarantine card raised
	// by a BACKGROUND sync hit (FRSP-01/02). It is lower precedence than a sentry
	// critical: a.critical is checked first in both rendering and key handling, so
	// a live sentry incident always preempts the quarantine card.
	quarantineAlert    bool
	quarantineIncident IncidentModel
	toast              ToastModel
	health    HealthState
	status    string
	clock     time.Time
	width     int
	height    int
	adminMode bool
	auditPath string
}

// NewApp constructs the initial App model.
//
// Health and the status line are computed from REAL probes at construction time
// (not seeded optimistically): a security dashboard must not paint every pip
// green and claim activity counts before it has checked anything. refreshHealthState
// fails safe — any component it cannot reach reads as degraded, not healthy — and
// computeStatus summarises today's real audit activity (or "monitoring" when the
// log is empty).
func NewApp(adminMode bool) App {
	auditDir, _ := platform.AuditDir()
	auditPath := filepath.Join(auditDir, "beekeeper.ndjson")
	stateDir, _ := platform.StateDir()
	return App{
		mode:      modeCalm,
		adminMode: adminMode,
		auditPath: auditPath,
		status:    computeStatus(auditPath),
		clock:     time.Now(),
		health:    refreshHealthState(stateDir),
	}
}

// Init starts the background timers.
func (a App) Init() tea.Cmd {
	return tea.Batch(clockCmd(), stateTickCmd(), healthTickCmd())
}

// Update is the Bubble Tea message handler.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		// Dedup: no-op if dimensions unchanged (resize poller fires every 500ms).
		if msg.Width == a.width && msg.Height == a.height {
			return a, nil
		}
		a.width, a.height = msg.Width, msg.Height
		return a, nil

	case clockMsg:
		a.clock = time.Time(msg)
		return a, clockCmd()

	case toastHideMsg:
		var cmd tea.Cmd
		a.toast, cmd = a.toast.Update(msg)
		return a, cmd

	case newRecordsMsg:
		// Distribute to open panel if applicable.
		if a.mode == modePanel {
			a.panelM, _ = a.panelM.Update(msg)
		}
		// Check for critical sentry alert. The incident card and status line are
		// built from the REAL record — never a canned demo incident — so an
		// operator never sees fabricated forensic detail during a live event.
		for _, rec := range []audit.AuditRecord(msg) {
			if rec.RecordType == "sentry_alert" && rec.SentrySeverity == "critical" && !a.critical {
				a.critical = true
				a.incident = IncidentFromRecord(rec)
				a.status = "⚠ 1 CRITICAL: " + a.incident.RuleName
				a.health.LastBlock = "sentry firing"
				a.health.SentryOK = false
				// A sentry critical preempts a background quarantine card.
				a.quarantineAlert = false
				a.quarantineIncident = IncidentModel{}
				continue
			}
			// Background catalog-quarantine from a scheduled sync (FRSP-01/02): raise
			// the catalog-quarantine card so an operator with the TUI open sees it.
			// Skip when a sentry critical is already showing (it takes precedence).
			if !a.critical && !a.quarantineAlert &&
				(rec.RecordType == "catalog_quarantine" || rec.RecordType == "pending-quarantine") {
				pending := rec.RecordType == "pending-quarantine"
				a.quarantineAlert = true
				a.quarantineIncident = CatalogQuarantineIncidentFromRecord(rec, pending)
				pkg := rec.ToolName
				if pkg == "" {
					pkg = "package"
				}
				if pending {
					a.status = "⚠ scan hit pending quarantine: " + pkg
				} else {
					a.status = "⚠ package quarantined: " + pkg
				}
			}
		}
		return a, nil

	case stateTick:
		if a.mode == modePanel {
			a.panelM, _ = a.panelM.Update(msg)
		}
		return a, stateTickCmd()

	case healthTick:
		if a.mode == modePanel {
			a.panelM, _ = a.panelM.Update(msg)
		}
		stateDir, _ := platform.StateDir()
		a.health = refreshHealthState(stateDir)
		// Refresh the calm-mode status from real audit activity. Never overwrite a
		// live critical banner — that is owned by the incident flow.
		if !a.critical {
			a.status = computeStatus(a.auditPath)
		}
		return a, healthTickCmd()

	case stepTickMsg:
		// Scan progress animation tick — route to the open panel and re-arm.
		if a.mode == modePanel {
			var cmd tea.Cmd
			a.panelM, cmd = a.panelM.Update(msg)
			return a, cmd
		}
		return a, nil

	case scanResultMsg:
		// Real scan completed — deliver the result to the open scan panel.
		if a.mode == modePanel {
			var cmd tea.Cmd
			a.panelM, cmd = a.panelM.Update(msg)
			return a, cmd
		}
		return a, nil

	case quarantineAlertMsg:
		// Close the alerts panel and show toast (prototype: q/Q in alerts panel).
		a.mode = modeCalm
		a.panelM = PanelModel{}
		var qCmd tea.Cmd
		a.toast, qCmd = a.toast.Show("item sent to quarantine", toastOK)
		return a, qCmd

	case syncCatalogsMsg:
		// In-progress feedback: the catalogs panel batches this alongside the real
		// async runSyncCmd, so the toast shows WHILE the sync runs. The outcome
		// arrives as syncDoneMsg below (Phase 20 — no longer a no-op toast).
		var sCmd tea.Cmd
		a.toast, sCmd = a.toast.Show("Syncing all sources…", toastOK)
		return a, sCmd

	case syncDoneMsg:
		// Real catalog-sync result toast (Phase 20): success / unchanged / failure.
		var dCmd tea.Cmd
		switch {
		case msg.err != nil:
			emsg := msg.err.Error()
			if len(emsg) > 60 {
				emsg = emsg[:57] + "..."
			}
			a.toast, dCmd = a.toast.Show("Catalog sync failed: "+emsg, toastWarn)
		case msg.notModified:
			a.toast, dCmd = a.toast.Show("Catalogs already fresh (unchanged)", toastOK)
		default:
			a.toast, dCmd = a.toast.Show(fmt.Sprintf("Synced %d catalog entries", msg.count), toastOK)
		}
		return a, dCmd

	case policyEditErrMsg:
		// A policy edit was rejected by the validation gate — surface why; the
		// on-disk policy file is unchanged.
		var eCmd tea.Cmd
		a.toast, eCmd = a.toast.Show(msg.msg, toastWarn)
		return a, eCmd

	case policySavedMsg:
		// A policy edit persisted successfully.
		var pCmd tea.Cmd
		a.toast, pCmd = a.toast.Show(msg.msg, toastOK)
		return a, pCmd

	case settingsEditErrMsg:
		// A settings edit was rejected by the validation gate — surface why; the
		// on-disk config.json is unchanged.
		var seCmd tea.Cmd
		a.toast, seCmd = a.toast.Show(msg.msg, toastWarn)
		return a, seCmd

	case settingsSavedMsg:
		// A first-responder settings edit persisted successfully.
		var ssCmd tea.Cmd
		a.toast, ssCmd = a.toast.Show(msg.msg, toastOK)
		return a, ssCmd

	case tea.KeyPressMsg:
		return a.handleKey(msg)
	}

	return a, nil
}

func (a App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Palette mode captures all keys.
	if a.mode == modePalette {
		switch k {
		case "esc":
			a.mode = modeCalm
			a.palette = PaletteModel{}
			return a, nil
		case "enter":
			sel := a.palette.Selected()
			if sel != nil && sel.Name == "quit" {
				return a, tea.Quit
			}
			if fn := a.runPaletteSelection(); fn != nil {
				m := fn()
				if app, ok := m.(App); ok {
					return app, nil
				}
			}
			a.mode = modeCalm
			return a, nil
		default:
			var cmd tea.Cmd
			a.palette, cmd = a.palette.Update(msg)
			return a, cmd
		}
	}

	// Panel mode captures all keys except Esc.
	if a.mode == modePanel {
		if k == "esc" {
			a.mode = modeCalm
			a.panelM = PanelModel{}
			return a, nil
		}
		// Propagate the panel's command so panel-emitted messages (e.g. the policy
		// editor's save/reject toasts) actually fire. Previously discarded.
		var pcmd tea.Cmd
		a.panelM, pcmd = a.panelM.Update(msg)
		return a, pcmd
	}

	// Critical incident card keys — only active when in calm base screen.
	if a.critical {
		switch k {
		case "a", "A":
			return a.acknowledgeIncident()
		case "d", "D":
			// Opens full record panel WITHOUT resolving critical mode.
			return a.openPanel(panelAlerts, NewAlertsPanel(a.critical))
		case "up", "left", "down", "right":
			var cmd tea.Cmd
			a.incident, cmd = a.incident.Update(msg)
			return a, cmd
		case "enter":
			return a.doIncidentAction()
		}
	}

	// Background catalog-quarantine card keys (lower precedence than critical).
	if a.quarantineAlert {
		switch k {
		case "a", "A":
			return a.acknowledgeQuarantineAlert()
		case "r", "R", "p", "P":
			// Restore/purge are performed in the quarantine panel (admin-gated, with
			// confirmation). Open it rather than claiming a direct action on the card.
			a.quarantineAlert = false
			a.quarantineIncident = IncidentModel{}
			return a.openPanel(panelQuarantine, NewQuarantinePanel(a.adminMode))
		case "up", "left", "down", "right":
			var cmd tea.Cmd
			a.quarantineIncident, cmd = a.quarantineIncident.Update(msg)
			return a, cmd
		case "enter":
			return a.doQuarantineIncidentAction()
		}
	}

	// Calm mode global key bindings. Prototype-locked set, intentionally extended
	// 2026-06-10 with `p` so the real policy editor has a first-class shortcut
	// (it was previously reachable only via the `:` palette → "policy edit").
	switch k {
	case ":":
		a.mode = modePalette
		a.palette = PaletteModel{}
	case "!":
		return a.openPanel(panelAlerts, NewAlertsPanel(a.critical))
	case "p", "P":
		return a.openPanel(panelPolicy, NewPolicyPanel(a.adminMode))
	case "s", "S":
		return a.openPanel(panelSettings, NewSettingsPanel(a.adminMode))
	case "?":
		return a.openPanel(panelHelp, NewHelpPanel())
	case "g", "G":
		a.mode = modePalette
		a.palette = PaletteModel{query: "go"}
	case "q", "ctrl+c":
		return a, tea.Quit
	}
	return a, nil
}

// acknowledgeIncident clears a live critical as an explicit operator
// acknowledgement. It does NOT claim any automated containment: the dashboard
// has no in-process quarantine/isolate primitive, so it tells the operator how
// to remediate (alert log + CLI) rather than fabricating a "contained" result.
func (a App) acknowledgeIncident() (tea.Model, tea.Cmd) {
	a.critical = false
	a.incident = IncidentModel{}
	a.status = "acknowledged · review the alert log (!) and rotate any exposed credentials"
	a.health.LastBlock = "last block just now"
	a.health.SentryOK = true
	var cmd tea.Cmd
	a.toast, cmd = a.toast.Show("incident acknowledged: no automated containment; remediate via CLI", toastWarn)
	return a, cmd
}

// acknowledgeQuarantineAlert dismisses the background catalog-quarantine card.
// It claims no action beyond dismissal: the artifact is already (reversibly)
// quarantined and persists in the quarantine panel for restore/purge.
func (a App) acknowledgeQuarantineAlert() (tea.Model, tea.Cmd) {
	a.quarantineAlert = false
	a.quarantineIncident = IncidentModel{}
	a.status = "acknowledged · review held items via : → quarantine"
	var cmd tea.Cmd
	a.toast, cmd = a.toast.Show("quarantine acknowledged", toastOK)
	return a, cmd
}

// doQuarantineIncidentAction executes the selected action on the background
// catalog-quarantine card. Restore/purge route to the quarantine panel (where
// they run admin-gated with confirmation); acknowledge dismisses the card.
func (a App) doQuarantineIncidentAction() (tea.Model, tea.Cmd) {
	if len(a.quarantineIncident.Actions) == 0 {
		return a, nil
	}
	sel := a.quarantineIncident.Actions[a.quarantineIncident.SelAction]
	switch sel.Key {
	case "a":
		return a.acknowledgeQuarantineAlert()
	case "r", "p":
		a.quarantineAlert = false
		a.quarantineIncident = IncidentModel{}
		return a.openPanel(panelQuarantine, NewQuarantinePanel(a.adminMode))
	}
	return a, nil
}

// doIncidentAction executes the currently selected incident action button.
func (a App) doIncidentAction() (tea.Model, tea.Cmd) {
	if len(a.incident.Actions) == 0 {
		return a, nil
	}
	sel := a.incident.Actions[a.incident.SelAction]
	switch sel.Key {
	case "a":
		return a.acknowledgeIncident()
	case "d":
		return a.openPanel(panelAlerts, NewAlertsPanel(a.critical))
	}
	return a, nil
}

// openPanel transitions to modePanel.
func (a App) openPanel(kind panelKind, content PanelContent) (tea.Model, tea.Cmd) {
	a.mode = modePanel
	a.panel = kind
	if content != nil {
		a.panelM = NewPanelModel(kind, content)
	}
	// ScanPanel starts the step animation AND kicks off a real scan (deep/quick).
	// History mode reads past audit records and needs neither.
	if kind == panelScan {
		if sp, ok := content.(*ScanPanel); ok && sp.scanMode != "history" {
			return a, tea.Batch(stepTickCmd(), sp.runScanCmd())
		}
		return a, stepTickCmd()
	}
	return a, nil
}

// runPaletteSelection dispatches the selected palette command and returns the next model.
// Returns nil if no command is selected or if the command is "quit".
func (a App) runPaletteSelection() func() interface{} {
	sel := a.palette.Selected()
	if sel == nil {
		return nil
	}
	a.mode = modeCalm
	a.palette = PaletteModel{}

	switch sel.Name {
	case "alerts":
		m, _ := a.openPanel(panelAlerts, NewAlertsPanel(a.critical))
		return func() interface{} { return m }

	case "quarantine":
		m, _ := a.openPanel(panelQuarantine, NewQuarantinePanel(a.adminMode))
		return func() interface{} { return m }

	case "audit tail":
		m, _ := a.openPanel(panelAudit, NewAuditPanel())
		return func() interface{} { return m }

	case "policy edit":
		m, _ := a.openPanel(panelPolicy, NewPolicyPanel(a.adminMode))
		return func() interface{} { return m }

	case "settings":
		m, _ := a.openPanel(panelSettings, NewSettingsPanel(a.adminMode))
		return func() interface{} { return m }

	case "catalogs":
		m, _ := a.openPanel(panelCatalogs, NewCatalogsPanel())
		return func() interface{} { return m }

	case "posture":
		m, _ := a.openPanel(panelPosture, NewPosturePanel())
		return func() interface{} { return m }

	case "scan now":
		m, _ := a.openPanel(panelScan, NewScanPanel("deep"))
		return func() interface{} { return m }

	case "scan --quick":
		m, _ := a.openPanel(panelScan, NewScanPanel("quick"))
		return func() interface{} { return m }

	case "scan history":
		m, _ := a.openPanel(panelScan, NewScanPanel("history"))
		return func() interface{} { return m }

	case "help":
		m, _ := a.openPanel(panelHelp, NewHelpPanel())
		return func() interface{} { return m }

	case "protect install":
		// The dashboard cannot install elevated protection itself (it requires a
		// privileged, out-of-band step), so it directs the operator to the CLI
		// rather than falsely claiming protection is already active.
		newToast, _ := a.toast.Show("run `beekeeper protect install` in a terminal to enable elevated protection", toastWarn)
		a.toast = newToast
		return func() interface{} { return a }

	case "quit":
		return nil
	}
	return func() interface{} { return a }
}

// View returns the rendered Bubble Tea view with AltScreen enabled.
func (a App) View() tea.View {
	var content string

	switch a.mode {
	case modeCalm:
		content = renderBase(a)

	case modePalette:
		dimmed := renderBaseDimmed(a)
		_ = dimmed
		pView := a.palette.View(a.width*3/4, a.height)
		screenBg := lipgloss.NewStyle().Background(colorScreen)
		content = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Top,
			pView,
			lipgloss.WithWhitespaceStyle(screenBg),
		)

	case modePanel:
		dimmed := renderBaseDimmed(a)
		_ = dimmed
		panelView := a.panelM.View(a.width*3/4, a.height*3/4)
		screenBg := lipgloss.NewStyle().Background(colorScreen)
		content = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			panelView,
			lipgloss.WithWhitespaceStyle(screenBg),
		)
	}

	// Append toast at bottom if visible.
	if a.toast.visible {
		content += "\n" + a.toast.View(a.width)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// Run creates and starts the Bubble Tea program.
func Run(ctx context.Context, adminMode bool) error {
	_ = ctx
	m := NewApp(adminMode)
	p := tea.NewProgram(m)
	StartResizePoller(p)
	go watchAuditLog(p, m.auditPath)
	_, err := p.Run()
	return err
}

// computeStatus summarises today's REAL audit activity for the calm-mode status
// line: distinct agents seen, block decisions, and critical sentry alerts. It
// reads the audit tail (best-effort) and never fabricates counts — an empty or
// unreadable log yields an honest "monitoring" line rather than invented numbers.
func computeStatus(auditPath string) string {
	recs := recentAuditRecords(auditPath)
	if len(recs) == 0 {
		return "monitoring · press : to act"
	}
	now := time.Now()
	agents := make(map[string]struct{})
	blocks, criticals := 0, 0
	for _, rec := range recs {
		t, terr := time.Parse(time.RFC3339, rec.Timestamp)
		if terr != nil || !sameDay(t, now) {
			continue
		}
		if rec.AgentName != "" {
			agents[rec.AgentName] = struct{}{}
		}
		if rec.Decision == "block" {
			blocks++
		}
		if rec.RecordType == "sentry_alert" && rec.SentrySeverity == "critical" {
			criticals++
		}
	}
	parts := make([]string, 0, 3)
	if len(agents) > 0 {
		parts = append(parts, fmt.Sprintf("%d agent%s today", len(agents), plural(len(agents))))
	}
	parts = append(parts,
		fmt.Sprintf("%d block%s today", blocks, plural(blocks)),
		fmt.Sprintf("%d critical%s today", criticals, plural(criticals)),
	)
	return strings.Join(parts, " · ")
}

// recentAuditRecords returns the audit records from the TAIL of the log, bounded
// to the last statusScanBytes. The audit log can grow to tens of megabytes, so
// reading it whole on startup and on every health tick would make the dashboard
// re-parse the entire file repeatedly. Seeking near the end and letting tailFrom
// discard the (malformed) partial first line keeps this O(window), and the most
// recent activity — today's records and the last block — lives at the tail anyway.
func recentAuditRecords(auditPath string) []audit.AuditRecord {
	const statusScanBytes = 512 * 1024
	info, err := os.Stat(auditPath)
	if err != nil {
		return nil
	}
	var offset int64
	if info.Size() > statusScanBytes {
		offset = info.Size() - statusScanBytes
	}
	recs, _ := tailFrom(auditPath, offset)
	return recs
}

// sameDay reports whether two times fall on the same calendar day in local time.
func sameDay(a, b time.Time) bool {
	a, b = a.Local(), b.Local()
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// viewString is a test helper to inspect model state without rendering ANSI.
func (a App) viewString() string {
	return fmt.Sprintf("mode=%d critical=%v panel=%s status=%s clock=%s",
		a.mode, a.critical, a.panel, a.status, a.clock.Format("15:04:05"))
}
