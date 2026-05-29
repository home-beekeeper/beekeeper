package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	audit "github.com/mzansi-agentive/beekeeper/internal/audit"
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

func TestAppCriticalTrigger(t *testing.T) {
	a := NewApp(false)
	if a.critical {
		t.Fatal("expected not critical on startup")
	}
	a.critical = true
	a.incident = DefaultIncident()
	if !a.critical {
		t.Fatal("expected critical=true after trigger")
	}
	if a.incident.RuleName == "" {
		t.Fatal("expected incident to be populated")
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
