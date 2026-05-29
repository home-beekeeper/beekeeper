package tui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// renderBase renders the calm base screen. Called by App.View() before
// overlaying the palette or panel if those modes are active.
func renderBase(a App) string {
	w := a.width
	if w < 40 {
		w = 40
	}

	// -- Titlebar --
	dots := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f57")).Render("●") + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#febc2e")).Render("●") + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#28c840")).Render("●")
	titleStr := styleDim.Render("  beekeeper dashboard — calm mode")
	clockStr := styleDimmer.Render(a.clock.Format("15:04:05") + " ")
	titlePad := w - lipgloss.Width(dots) - lipgloss.Width(titleStr) - lipgloss.Width(clockStr) - 2
	if titlePad < 0 {
		titlePad = 0
	}
	titlebar := styleTitlebar.Width(w).Render(
		" " + dots + titleStr + strings.Repeat(" ", titlePad) + clockStr)

	// -- Status line --
	brandStr := styleBrand.Render("BEEKEEPER")
	var statusStr string
	if a.critical {
		statusStr = "  " + styleRed.Render(a.status)
	} else {
		statusStr = "  " + styleDim.Render(a.status)
	}
	statusLine := "\n  " + brandStr + statusStr + "\n"

	// -- Health pips --
	pip := func(ok bool, label string) string {
		dot := styleGreen.Render("●")
		if !ok {
			dot = styleRed.Render("●")
		}
		return dot + " " + styleDim.Render(label)
	}
	// Sentry pip pulses (alternates red/dimmer) every second during critical mode.
	sentryPip := pip(a.health.SentryOK, "sentry")
	if a.critical && !a.health.SentryOK {
		if a.clock.Second()%2 == 0 {
			sentryPip = styleRed.Render("●") + " " + styleDim.Render("sentry")
		} else {
			sentryPip = styleDimmer.Render("●") + " " + styleDim.Render("sentry")
		}
	}
	lastBlockStr := styleDimmer.Render(a.health.LastBlock)
	healthRow := "  " + pip(a.health.HooksOK, "hooks") + "  " +
		pip(a.health.GatewayOK, "gateway") + "  " +
		sentryPip + "  " +
		pip(a.health.CatalogsOK, "catalogs fresh") + "  " +
		pip(a.health.LlamaFirewallOK, "llamafirewall") + "  " +
		lastBlockStr + "\n"

	// -- Incident slot --
	incidentSlot := ""
	if a.critical {
		incidentSlot = "\n" + a.incident.View(w) + "\n"
	}

	// -- Horizontal rule --
	rule := "\n  " + lipgloss.NewStyle().Foreground(colorBorderDim).
		Render(strings.Repeat("─", w-4)) + "\n"

	// -- Tagline + single-row hint --
	// Tagline sits directly under the rule; the shortcut labels render as one
	// compact `·`-separated row beneath it (no separate bottom keybar — that was
	// a duplicate of these same labels).
	key := func(k string) string { return styleTeal.Render(k) }
	hint := "\n  " +
		styleDimmer.Render("Beekeeper stays quiet until something needs you. Press : to do anything.") + "\n  " +
		key(":") + styleDim.Render(" command palette") + styleDimmer.Render(" · ") +
		key("!") + styleDim.Render(" jump to alerts") + styleDimmer.Render(" · ") +
		key("g") + styleDim.Render(" go to…") + styleDimmer.Render(" · ") +
		key("?") + styleDim.Render(" help") + styleDimmer.Render(" · ") +
		key("q") + styleDim.Render(" quit") + "\n"

	body := statusLine + healthRow + incidentSlot + rule + hint
	full := titlebar + "\n" + body

	return full
}

// renderBaseDimmed wraps renderBase output in a dimmer foreground for overlay modes.
func renderBaseDimmed(a App) string {
	base := renderBase(a)
	return lipgloss.NewStyle().Foreground(colorDimmer).Render(base)
}
