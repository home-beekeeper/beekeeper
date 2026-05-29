package tui

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

// Color palette — all 16 hex values match the prototype CSS exactly (LOCKED).
var (
	colorBg        = lipgloss.Color("#0b0f14")  // --bg
	colorScreen    = lipgloss.Color("#0d1117")  // --screen
	colorPanel     = lipgloss.Color("#11161d")  // --panel
	colorPanel2    = lipgloss.Color("#161b22")  // --panel2 (titlebar, keybar, panelhead)
	colorBorder    = lipgloss.Color("#2b3543")  // --border
	colorBorderDim = lipgloss.Color("#1c242e")  // --border-dim (rule/hr)
	colorFg        = lipgloss.Color("#c9d1d9")  // --fg
	colorDim       = lipgloss.Color("#6e7681")  // --dim
	colorDimmer    = lipgloss.Color("#454d57")  // --dimmer
	colorWhite     = lipgloss.Color("#ffffff")  // --white
	colorRed       = lipgloss.Color("#f85149")  // --red
	colorCoral     = lipgloss.Color("#f0883e")  // --coral
	colorAmber     = lipgloss.Color("#e3b341")  // --amber
	colorGreen     = lipgloss.Color("#3fb950")  // --green
	colorTeal      = lipgloss.Color("#39c5cf")  // --teal
	colorSelbg     = lipgloss.Color("#11233f")  // --selbg
)

// renderBadge renders a filled color badge chip with dark fg for contrast.
func renderBadge(text string, bg color.Color) string {
	return lipgloss.NewStyle().
		Background(bg).
		Foreground(lipgloss.Color("#0d1117")).
		Bold(true).
		Padding(0, 1).
		Render(text)
}

// Badge helper functions.
func BadgeCrit() string  { return renderBadge("CRIT", colorRed) }
func BadgeBlock() string { return renderBadge("BLOCK", colorCoral) }
func BadgeWarn() string  { return renderBadge("WARN", colorAmber) }
func BadgeOK() string    { return renderBadge("OK", colorGreen) }
func BadgeHeld() string  { return renderBadge("HELD", colorCoral) }

// Common styles declared as package-level vars.
var (
	styleTitlebar = lipgloss.NewStyle().Background(colorPanel2).Foreground(colorDim)
	styleKeybar   = lipgloss.NewStyle().Background(colorPanel2).Foreground(colorDim)
	styleBrand    = lipgloss.NewStyle().Foreground(colorAmber).Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(colorDim)
	styleDimmer   = lipgloss.NewStyle().Foreground(colorDimmer)
	styleTeal     = lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
	styleWhite    = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	styleRed      = lipgloss.NewStyle().Foreground(colorRed)
	styleCoral    = lipgloss.NewStyle().Foreground(colorCoral)
	styleAmber    = lipgloss.NewStyle().Foreground(colorAmber)
	styleGreen    = lipgloss.NewStyle().Foreground(colorGreen)

	// Panel border styles
	stylePanelBorder     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder)
	stylePanelBorderTeal = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorTeal)
	stylePanelBorderRed  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorRed)

	// Incident card border
	styleIncidentBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorRed)

	// Selected row in row-based panels
	styleSelRow = lipgloss.NewStyle().Background(colorSelbg).BorderLeft(true).BorderForeground(colorTeal)
)
