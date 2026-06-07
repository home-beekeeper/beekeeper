package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	audit "github.com/bantuson/beekeeper/internal/audit"
	platform "github.com/bantuson/beekeeper/internal/platform"
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
	toast     ToastModel
	health    HealthState
	status    string
	clock     time.Time
	width     int
	height    int
	adminMode bool
	auditPath string
}

// NewApp constructs the initial App model.
func NewApp(adminMode bool) App {
	auditDir, _ := platform.AuditDir()
	return App{
		mode:      modeCalm,
		adminMode: adminMode,
		auditPath: filepath.Join(auditDir, "beekeeper.ndjson"),
		status:    "all systems nominal · protecting 4 agents · 0 open criticals today",
		clock:     time.Now(),
		health: HealthState{
			HooksOK:         true,
			GatewayOK:       true,
			SentryOK:        true,
			CatalogsOK:      true,
			LlamaFirewallOK: true,
			LastBlock:       "last block 6m ago",
		},
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
		// Check for critical sentry alert.
		for _, rec := range []audit.AuditRecord(msg) {
			if rec.RecordType == "sentry_alert" && rec.SentrySeverity == "critical" && !a.critical {
				a.critical = true
				a.status = "⚠ 1 CRITICAL — credential exfiltration pattern detected"
				a.incident = DefaultIncident()
				a.health.LastBlock = "sentry firing"
				a.health.SentryOK = false
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
		return a, healthTickCmd()

	case quarantineAlertMsg:
		// Close the alerts panel and show toast (prototype: q/Q in alerts panel).
		a.mode = modeCalm
		a.panelM = PanelModel{}
		var qCmd tea.Cmd
		a.toast, qCmd = a.toast.Show("item sent to quarantine", toastOK)
		return a, qCmd

	case syncCatalogsMsg:
		// Show sync toast (prototype: s in catalogs panel shows "Syncing all sources…").
		var sCmd tea.Cmd
		a.toast, sCmd = a.toast.Show("Syncing all sources…", toastOK)
		return a, sCmd

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
		case "Q", "q":
			a.critical = false
			a.status = "contained · 1 item quarantined · rotate creds recommended"
			a.incident = IncidentModel{}
			a.health.LastBlock = "last block just now"
			a.health.SentryOK = true
			var cmd tea.Cmd
			a.toast, cmd = a.toast.Show("extension quarantined", toastOK)
			return a, cmd
		case "I", "i":
			a.critical = false
			a.status = "contained · process isolated · rotate creds recommended"
			a.incident = IncidentModel{}
			a.health.LastBlock = "last block just now"
			a.health.SentryOK = true
			var cmd tea.Cmd
			a.toast, cmd = a.toast.Show("process isolated", toastOK)
			return a, cmd
		case "d", "D":
			// Opens full record panel WITHOUT resolving critical mode (prototype behaviour).
			return a.openPanel(panelAlerts, NewAlertsPanel(a.critical))
		case "up", "left", "down", "right":
			var cmd tea.Cmd
			a.incident, cmd = a.incident.Update(msg)
			return a, cmd
		case "enter":
			return a.doIncidentAction()
		}
	}

	// Calm mode global key bindings (LOCKED from prototype).
	switch k {
	case ":":
		a.mode = modePalette
		a.palette = PaletteModel{}
	case "!":
		return a.openPanel(panelAlerts, NewAlertsPanel(a.critical))
	case "?":
		return a.openPanel(panelHelp, NewHelpPanel())
	case "g", "G":
		a.mode = modePalette
		a.palette = PaletteModel{query: "go"}
	case "x", "X":
		a.critical = true
		a.status = "⚠ 1 CRITICAL — credential exfiltration pattern detected"
		a.incident = DefaultIncident()
		a.health.LastBlock = "sentry firing"
		a.health.SentryOK = false
	case "q", "ctrl+c":
		return a, tea.Quit
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
	case "Q":
		a.critical = false
		a.status = "contained · 1 item quarantined · rotate creds recommended"
		a.incident = IncidentModel{}
		a.health.LastBlock = "last block just now"
		a.health.SentryOK = true
		var cmd tea.Cmd
		a.toast, cmd = a.toast.Show("extension quarantined", toastOK)
		return a, cmd
	case "I":
		a.critical = false
		a.status = "contained · process isolated · rotate creds recommended"
		a.incident = IncidentModel{}
		a.health.LastBlock = "last block just now"
		a.health.SentryOK = true
		var cmd tea.Cmd
		a.toast, cmd = a.toast.Show("process isolated", toastOK)
		return a, cmd
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
	// ScanPanel requires an initial tick to start the step animation.
	if kind == panelScan {
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

	case "catalogs":
		m, _ := a.openPanel(panelCatalogs, NewCatalogsPanel())
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
		newToast, _ := a.toast.Show("protect mode already active", toastOK)
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

// viewString is a test helper to inspect model state without rendering ANSI.
func (a App) viewString() string {
	return fmt.Sprintf("mode=%d critical=%v panel=%s status=%s clock=%s",
		a.mode, a.critical, a.panel, a.status, a.clock.Format("15:04:05"))
}
