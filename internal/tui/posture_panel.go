package tui

// posture_panel.go - the read-only install-posture dashboard panel (Layer 2,
// IPVIEW-01). It mirrors the catalogs panel structure but is strictly read-only:
// no admin actions, no key handlers that mutate anything, and it never writes a
// package-manager config file. It reuses the SAME pure comparison model as the
// CLI (posture.BuildComparison / Comparison) so the CLI and TUI share one source
// of truth, and it shows the canonical enforcement-boundary statement
// (posture.BoundaryShort, IPBND-01) rather than re-typing the prose.

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	posture "github.com/home-beekeeper/beekeeper/internal/posture"
)

// postureDetectFn is the detection seam used by the panel so tests can inject a
// synthetic PMState without a real npm/pnpm/bun on PATH. Production code leaves it
// as posture.DetectStateFn (the real read-only detection).
var postureDetectFn = posture.DetectStateFn

// postureWeaknessFn is the pnpm-workspace weakness-note seam for the panel.
// Production code leaves it as the real read-only reader.
var postureWeaknessFn = posture.PnpmWeaknessNote

// PosturePanel implements PanelContent for the read-only install-posture overlay.
// It holds the pre-built comparison model and a cached rendered body. It has NO
// mutating actions - it only displays detected managers side-by-side with
// Beekeeper's enforced posture.
type PosturePanel struct {
	comparison posture.Comparison
	bodyCache  string
}

// NewPosturePanel creates a PosturePanel, resolving the read-only PMState once at
// construction (with a short bounded timeout) and building the pure comparison
// model. Detection failures fall back to "nothing detected" - the panel never
// blocks and never writes.
func NewPosturePanel() *PosturePanel {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	state := postureDetectFn(ctx, posture.DefaultConfig())
	weakness := postureWeaknessFn()
	p := &PosturePanel{
		comparison: posture.BuildComparison(state, posture.DefaultEnforced(), weakness),
	}
	p.bodyCache = p.buildBody()
	return p
}

// Update implements PanelContent. The panel is read-only, so it handles no keys
// that change state - it simply ignores input (esc-to-close is handled by the
// App). This is intentional: there is nothing to toggle, sync, or persist.
func (p *PosturePanel) Update(_ tea.Msg) (PanelContent, tea.Cmd) { return p, nil }

// Title implements PanelContent.
func (p *PosturePanel) Title() string { return "Install posture" }

// Count implements PanelContent: detected manager count and total gaps covered.
func (p *PosturePanel) Count() string {
	n := len(p.comparison.Managers)
	gaps := p.comparison.TotalGaps()
	return fmt.Sprintf("%d managers · %d gaps covered", n, gaps)
}

// Padded implements PanelContent.
func (p *PosturePanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *PosturePanel) Critical() bool { return false }

// buildBody renders the comparison body using the shared model. Color is applied
// here (TUI chrome) but the underlying copy comes from the same pure model the
// CLI prints, so the two never drift.
func (p *PosturePanel) buildBody() string {
	var sb strings.Builder
	sb.WriteString("\n")

	en := p.comparison.Enforced
	sb.WriteString("  " + styleDim.Render("BEEKEEPER ENFORCES AT THE HOOK") + "\n")
	sb.WriteString("  " + styleWhite.Render(
		fmt.Sprintf("%s · %s · %s", en.ReleaseAge, en.LifecycleScripts, en.RemoteSource)) +
		"  " + styleDimmer.Render("(warn by default)") + "\n")
	sb.WriteString("\n")

	if len(p.comparison.Managers) == 0 {
		sb.WriteString("  " + styleDimmer.Render("No package managers detected on this machine.") + "\n")
	} else {
		sb.WriteString("  " + styleDim.Render("DETECTED MANAGERS") + "\n")
		for _, m := range p.comparison.Managers {
			name := styleWhite.Render(fmt.Sprintf("%-6s", m.Manager))
			ver := styleDimmer.Render(versionLabel(m.Version))

			var status string
			if m.Aligned || m.GapCount() == 0 {
				status = styleGreen.Render("aligned, no gap")
			} else {
				status = styleAmber.Render(gapLabel(m.GapCount()) + ": " + strings.Join(m.Gaps, ", "))
			}
			sb.WriteString("  " + name + " " + ver + "  " + status + "\n")

			for _, line := range m.SelfPosture {
				sb.WriteString("      " + styleDimmer.Render(line) + "\n")
			}
		}
	}

	sb.WriteString("\n")
	// The canonical enforcement-boundary statement (single source of truth).
	sb.WriteString("  " + styleDimmer.Render(posture.BoundaryShort) + "\n")
	return sb.String()
}

// versionLabel formats the detected version for display.
func versionLabel(v string) string {
	if v == "" {
		return "detected"
	}
	return "v" + v
}

// gapLabel returns "covers N gap(s)" for a manager row.
func gapLabel(n int) string {
	noun := "gaps"
	if n == 1 {
		noun = "gap"
	}
	return fmt.Sprintf("covers %d %s", n, noun)
}

// Body implements PanelContent.
func (p *PosturePanel) Body(_, _ int) string {
	if p.bodyCache == "" {
		p.bodyCache = p.buildBody()
	}
	return p.bodyCache
}

// Footer implements PanelContent. Read-only: the only action is to close.
func (p *PosturePanel) Footer() string {
	return styleDim.Render("read-only · ") +
		styleTeal.Render("esc") + styleDim.Render(" close")
}
