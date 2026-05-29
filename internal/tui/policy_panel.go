package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	platform "github.com/mzansi-agentive/beekeeper/internal/platform"
)

// PolicyPanel implements PanelContent for the policy & thresholds overlay.
// It loads real policy rules from ~/.beekeeper/policies/tui_rules.json,
// renders each rule as a drill-down row with its enabled/disabled state,
// and — when adminMode is true — allows toggling individual rules via e/t.
type PolicyPanel struct {
	rules       []PolicyRule
	selIdx      int
	adminMode   bool
	policiesDir string
}

// NewPolicyPanel creates a PolicyPanel. When adminMode is true the panel
// exposes the e/t toggle affordance; otherwise those keys are no-ops.
func NewPolicyPanel(adminMode bool) *PolicyPanel {
	stateDir, err := platform.StateDir()
	if err != nil {
		stateDir = "policies"
	}
	dir := filepath.Join(stateDir, "policies")
	p := &PolicyPanel{
		adminMode:   adminMode,
		policiesDir: dir,
	}
	p.rules = LoadPolicyRules(dir)
	return p
}

// Update implements PanelContent. Navigation (j/k/up/down) always works.
// The e/t toggle is gated on adminMode — mirroring QuarantinePanel's r/p gate.
func (p *PolicyPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case stateTick:
		// Reload rules so external edits surface.
		p.rules = LoadPolicyRules(p.policiesDir)
		// Clamp selIdx against the (possibly shorter) new slice.
		if p.selIdx >= len(p.rules) {
			p.selIdx = len(p.rules) - 1
		}
		if p.selIdx < 0 {
			p.selIdx = 0
		}

	case tea.KeyPressMsg:
		k := msg.String()

		if !p.adminMode {
			// Non-admin: only j/k navigation; e/t are no-ops.
			switch k {
			case "j", "down":
				if p.selIdx < len(p.rules)-1 {
					p.selIdx++
				}
			case "k", "up":
				if p.selIdx > 0 {
					p.selIdx--
				}
			}
			return p, nil
		}

		// Admin path: navigation + toggle.
		switch k {
		case "j", "down":
			if p.selIdx < len(p.rules)-1 {
				p.selIdx++
			}
		case "k", "up":
			if p.selIdx > 0 {
				p.selIdx--
			}
		case "e", "E", "t", "T":
			if p.selIdx >= 0 && p.selIdx < len(p.rules) {
				p.rules[p.selIdx].Enabled = !p.rules[p.selIdx].Enabled
				_ = ToggleRule(p.policiesDir, p.rules[p.selIdx].ID, p.rules[p.selIdx].Enabled)
				// Reload to reflect persisted state, then re-clamp.
				p.rules = LoadPolicyRules(p.policiesDir)
				if p.selIdx >= len(p.rules) {
					p.selIdx = len(p.rules) - 1
				}
			}
		}
	}
	return p, nil
}

// Title implements PanelContent.
func (p *PolicyPanel) Title() string { return "Policy & thresholds" }

// Count implements PanelContent. Reports N rules · M enabled.
func (p *PolicyPanel) Count() string {
	total := len(p.rules)
	enabled := 0
	for _, r := range p.rules {
		if r.Enabled {
			enabled++
		}
	}
	return fmt.Sprintf("%d rules · %d enabled", total, enabled)
}

// Padded implements PanelContent.
func (p *PolicyPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *PolicyPanel) Critical() bool { return false }

// Body implements PanelContent. Renders each rule as a drill-down row with
// an explicit ON/OFF state indicator; the selected row is highlighted.
func (p *PolicyPanel) Body(width, height int) string {
	if len(p.rules) == 0 {
		return "\n  " + styleDim.Render("(no policy rules)")
	}

	var lines []string
	lines = append(lines, "")

	for i, r := range p.rules {
		var stateToken string
		if r.Enabled {
			stateToken = styleGreen.Render("ON ")
		} else {
			stateToken = styleDimmer.Render("OFF")
		}

		label := styleDim.Render(fmt.Sprintf("%-18s", r.Label))
		detail := styleDimmer.Render(r.Detail)
		line := "  " + stateToken + "  " + label + detail

		if i == p.selIdx {
			line = styleSelRow.Render(strings.TrimRight(line, " "))
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, "  "+styleDimmer.Render("Declarative JSON, version-controlled, testable with"))
	lines = append(lines, "  "+styleTeal.Render("beekeeper policy test <file>"))

	return strings.Join(lines, "\n")
}

// Footer implements PanelContent. Shows admin keys only when adminMode is true.
func (p *PolicyPanel) Footer() string {
	if p.adminMode {
		return styleTeal.Render("e/t") + styleDim.Render(" toggle · ") +
			styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
			styleTeal.Render("esc") + styleDim.Render(" close")
	}
	return styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
		styleTeal.Render("esc") + styleDim.Render(" close · ") +
		styleDimmer.Render("--admin to toggle")
}
