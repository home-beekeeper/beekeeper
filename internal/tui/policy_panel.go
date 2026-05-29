package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// PolicyPanel implements PanelContent for the policy & thresholds overlay.
// Content is static for Phase 8 — Phase 9 will add live config reading.
type PolicyPanel struct{}

// NewPolicyPanel creates a PolicyPanel.
func NewPolicyPanel() *PolicyPanel { return &PolicyPanel{} }

// Update implements PanelContent.
func (p *PolicyPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) { return p, nil }

// Title implements PanelContent.
func (p *PolicyPanel) Title() string { return "Policy & thresholds" }

// Count implements PanelContent.
func (p *PolicyPanel) Count() string { return "editing default profile" }

// Padded implements PanelContent.
func (p *PolicyPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *PolicyPanel) Critical() bool { return false }

// Body implements PanelContent.
// Lead labels in colorDim, values in colorWhite, secondary in colorDimmer.
// Matches prototype EXACTLY (LOCKED).
func (p *PolicyPanel) Body(width, height int) string {
	row := func(label, value, secondary string) string {
		l := styleDim.Render(fmt.Sprintf("%-18s", label))
		v := styleWhite.Render(value)
		s := ""
		if secondary != "" {
			s = "  " + styleDimmer.Render(secondary)
		}
		return "  " + l + v + s
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(row("corroboration", "single → warn", "two → enforce  three → quarantine") + "\n")
	sb.WriteString(row("release-age", "1440 min (24h)", "· npm pypi cargo gem composer") + "\n")
	sb.WriteString(row("lifecycle", "deny by default", "· allowlist 3 pkgs") + "\n")
	sb.WriteString(row("sentry baseline", "day 3 of 7", "· audit-only until day 7") + "\n")
	sb.WriteString(row("llamafirewall", "enabled", "· sample 1.0") + "\n")
	sb.WriteString("\n")
	sb.WriteString(styleDimmer.Render("Declarative JSON, version-controlled, testable with") + "\n")
	sb.WriteString(styleTeal.Render("beekeeper policy test <file>") + "\n")
	return sb.String()
}

// Footer implements PanelContent.
// e/t are visual-only in Phase 8 (wired to $EDITOR and policy test in Phase 9).
func (p *PolicyPanel) Footer() string {
	return styleTeal.Render("e") + styleDim.Render(" open in $EDITOR · ") +
		styleTeal.Render("t") + styleDim.Render(" test · ") +
		styleTeal.Render("esc") + styleDim.Render(" close")
}
