package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// HelpPanel implements PanelContent for the help overlay.
// Content is static (LOCKED from prototype).
type HelpPanel struct{}

// NewHelpPanel creates a HelpPanel.
func NewHelpPanel() *HelpPanel { return &HelpPanel{} }

// Update implements PanelContent.
func (p *HelpPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) { return p, nil }

// Title implements PanelContent.
func (p *HelpPanel) Title() string { return "Help" }

// Count implements PanelContent.
func (p *HelpPanel) Count() string { return "beekeeper v0.6.0" }

// Padded implements PanelContent.
func (p *HelpPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *HelpPanel) Critical() bool { return false }

// Body implements PanelContent.
// NAVIGATION section label in colorDim uppercase.
// Key chars in colorTeal. Explanation in colorDimmer.
// CONCEPT section follows. (LOCKED from prototype)
func (p *HelpPanel) Body(width, height int) string {
	label := func(s string) string {
		return styleDim.Render(s)
	}
	key := func(s string) string {
		return styleTeal.Render(fmt.Sprintf("%-4s", s))
	}
	explain := func(s string) string {
		return styleDimmer.Render(s)
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  " + label("NAVIGATION") + "\n")
	sb.WriteString("  " + key(":") + explain("open command palette (do anything)") + "\n")
	sb.WriteString("  " + key("!") + explain("jump straight to alerts") + "\n")
	sb.WriteString("  " + key("g") + explain("go-to menu") + "\n")
	sb.WriteString("  " + key("esc") + explain("close any overlay") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + label("CONCEPT") + "\n")
	sb.WriteString("  " + explain("Calm mode stays quiet by design. When Sentry detects a") + "\n")
	sb.WriteString("  " + explain("credential exfiltration pattern, the base screen escalates on") + "\n")
	sb.WriteString("  " + explain("its own — the incident card appears with containment actions") + "\n")
	sb.WriteString("  " + explain("ready. You don't hunt for the problem; it comes to you.") + "\n")
	return sb.String()
}

// Footer implements PanelContent.
func (p *HelpPanel) Footer() string {
	return styleTeal.Render("esc") + styleDim.Render(" close")
}
