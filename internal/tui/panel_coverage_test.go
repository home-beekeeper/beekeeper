package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	version "github.com/home-beekeeper/beekeeper/internal/version"
)

// --- PanelModel wrapper (panel.go) ---

// TestPanelModelViewRendersHeadBodyFoot proves the wrapper composes the head
// (title left, count right), body, and footer of the wrapped content into a
// single bordered overlay.
func TestPanelModelViewRendersHeadBodyFoot(t *testing.T) {
	pm := NewPanelModel(panelHelp, NewHelpPanel())
	view := pm.View(100, 40)
	if !strings.Contains(view, "Help") {
		t.Errorf("panel view missing the content Title: %q", view)
	}
	if !strings.Contains(view, "beekeeper "+version.Version) {
		t.Errorf("panel view missing the content Count (version): %q", view)
	}
	// Body's NAVIGATION section and the footer's "close" hint must both appear.
	if !strings.Contains(view, "NAVIGATION") {
		t.Errorf("panel view missing the content Body: %q", view)
	}
	if !strings.Contains(view, "close") {
		t.Errorf("panel view missing the content Footer: %q", view)
	}
}

// TestPanelModelNilContent proves a zero-value wrapper renders nothing and its
// Update is a safe no-op (the App sets PanelModel{} on close).
func TestPanelModelNilContent(t *testing.T) {
	var pm PanelModel
	if got := pm.View(80, 24); got != "" {
		t.Errorf("nil-content View = %q, want empty", got)
	}
	out, cmd := pm.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd != nil {
		t.Errorf("nil-content Update should return a nil cmd, got %v", cmd)
	}
	if out.content != nil {
		t.Errorf("nil-content Update should keep content nil")
	}
}

// TestPanelModelUpdateDelegates proves keys reach the wrapped content: a 'j'
// keypress in an alerts panel moves its selection through the wrapper.
func TestPanelModelUpdateDelegates(t *testing.T) {
	ap := NewAlertsPanel(false)
	ap.rows = []AlertRow{{Label: "a"}, {Label: "b"}, {Label: "c"}}
	pm := NewPanelModel(panelAlerts, ap)
	pm, _ = pm.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got := pm.content.(*AlertsPanel)
	if got.selIdx != 1 {
		t.Errorf("Update did not delegate the keypress to the content (selIdx=%d, want 1)", got.selIdx)
	}
}

// TestPanelModelCriticalBorder proves a critical content selects the red border
// (the rendered output differs from the same content rendered non-critical).
func TestPanelModelCriticalBorder(t *testing.T) {
	crit := NewPanelModel(panelAlerts, NewAlertsPanel(true)).View(100, 40)
	calm := NewPanelModel(panelAlerts, NewAlertsPanel(false)).View(100, 40)
	if crit == calm {
		t.Error("critical and non-critical panels rendered identically — red border not applied")
	}
}

// TestPanelModelTinyDimensions exercises the negative-padding / tiny-height
// clamps in View (padLen<0 and bodyHeight<1 branches) without panicking.
func TestPanelModelTinyDimensions(t *testing.T) {
	pm := NewPanelModel(panelHelp, NewHelpPanel())
	if got := pm.View(6, 3); got == "" {
		t.Error("View at tiny dimensions should still render the clamped frame")
	}
}

// TestPanelModelNonPaddedBody proves a non-padded content (alerts) does not get
// the two-space body indent applied (Padded()==false branch).
func TestPanelModelNonPaddedBody(t *testing.T) {
	// HelpPanel is Padded()==true; AlertsPanel is Padded()==false. Both must render.
	if NewPanelModel(panelAlerts, NewAlertsPanel(false)).View(100, 40) == "" {
		t.Error("non-padded panel rendered empty")
	}
}

// --- HelpPanel (help_panel.go) ---

// TestHelpPanelContract covers the small PanelContent surface of HelpPanel.
func TestHelpPanelContract(t *testing.T) {
	p := NewHelpPanel()
	if p.Title() != "Help" {
		t.Errorf("Title = %q, want Help", p.Title())
	}
	if !strings.Contains(p.Count(), version.Version) {
		t.Errorf("Count = %q, want it to contain the real build version", p.Count())
	}
	if !p.Padded() {
		t.Error("help panel should be padded")
	}
	if p.Critical() {
		t.Error("help panel must not be critical")
	}
	if !strings.Contains(p.Footer(), "close") {
		t.Errorf("Footer = %q, want a close hint", p.Footer())
	}
	// Update is a static no-op that returns itself and a nil cmd.
	out, cmd := p.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if cmd != nil {
		t.Errorf("help Update should return nil cmd, got %v", cmd)
	}
	if _, ok := out.(*HelpPanel); !ok {
		t.Errorf("help Update should return *HelpPanel, got %T", out)
	}
}

// --- PaletteModel (palette.go) ---

// keyText is a small helper to build a printable-character keypress.
func keyText(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// TestPaletteFilterAndSelection drives typed input and verifies the filtered set
// narrows and Selected() tracks the current row.
func TestPaletteFilterAndSelection(t *testing.T) {
	var p PaletteModel
	// No query: full command list, first row selected.
	if len(p.filtered()) != len(commands) {
		t.Fatalf("empty-query filtered() = %d, want %d", len(p.filtered()), len(commands))
	}
	sel := p.Selected()
	if sel == nil || sel.Name != commands[0].Name {
		t.Fatalf("Selected() at idx 0 = %v, want %q", sel, commands[0].Name)
	}

	// Type "history" → only "scan history" matches (substring on Name).
	for _, ch := range "history" {
		p, _ = p.Update(keyText(string(ch)))
	}
	got := p.filtered()
	if len(got) != 1 || got[0].Name != "scan history" {
		t.Fatalf("filtered() after typing 'history' = %+v, want [scan history]", got)
	}
	// selIdx reset to 0 on each printable keystroke.
	if s := p.Selected(); s == nil || s.Name != "scan history" {
		t.Fatalf("Selected() = %v, want scan history", s)
	}

	// Backspace removes a char; "histor" still matches only scan history, but the
	// selIdx clamp branch runs.
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if p.query != "histor" {
		t.Errorf("backspace query = %q, want histor", p.query)
	}
}

// TestPaletteNavigationClamps proves up/down move and clamp within the filtered
// list bounds.
func TestPaletteNavigationClamps(t *testing.T) {
	var p PaletteModel
	// up at the top is a no-op.
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.selIdx != 0 {
		t.Errorf("up at top moved to %d, want 0", p.selIdx)
	}
	// down moves through to the last command and clamps.
	for range commands {
		p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if p.selIdx != len(commands)-1 {
		t.Errorf("down past bottom = %d, want %d", p.selIdx, len(commands)-1)
	}
	// up then moves back.
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.selIdx != len(commands)-2 {
		t.Errorf("up = %d, want %d", p.selIdx, len(commands)-2)
	}
}

// TestPaletteBackspaceOnEmptyQuery proves backspace with no query is a no-op.
func TestPaletteBackspaceOnEmptyQuery(t *testing.T) {
	var p PaletteModel
	p, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if p.query != "" {
		t.Errorf("backspace on empty query set query=%q", p.query)
	}
}

// TestPaletteNonKeyMsgIgnored proves Update ignores non-key messages.
func TestPaletteNonKeyMsgIgnored(t *testing.T) {
	p := PaletteModel{query: "scan", selIdx: 1}
	out, cmd := p.Update(stateTick{})
	if cmd != nil {
		t.Errorf("non-key msg should produce nil cmd")
	}
	if out.query != "scan" || out.selIdx != 1 {
		t.Errorf("non-key msg mutated palette state: %+v", out)
	}
}

// TestPaletteSelectedOutOfRange proves Selected() returns nil when selIdx is
// beyond the filtered list (e.g. a query that matches nothing).
func TestPaletteSelectedOutOfRange(t *testing.T) {
	p := PaletteModel{query: "zzz-no-match", selIdx: 0}
	if p.Selected() != nil {
		t.Error("Selected() with no matches should be nil")
	}
}

// TestPaletteViewRenders covers the View render path including the grouped rows,
// the selected-row highlight, and the empty-result branch.
func TestPaletteViewRenders(t *testing.T) {
	// Populated list: groups + a selected row.
	full := PaletteModel{selIdx: 3}.View(80, 24)
	for _, want := range []string{"SCAN", "INVESTIGATE", "alerts"} {
		if !strings.Contains(full, want) {
			t.Errorf("palette View missing %q:\n%s", want, full)
		}
	}
	// Empty-result branch.
	empty := PaletteModel{query: "zzz-no-match"}.View(80, 24)
	if !strings.Contains(empty, "no matching commands") {
		t.Errorf("empty-query palette View missing the no-match line:\n%s", empty)
	}
}
