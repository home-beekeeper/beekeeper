package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

type panelKind string

const (
	panelAlerts     panelKind = "alerts"
	panelQuarantine panelKind = "quarantine"
	panelCatalogs   panelKind = "catalogs"
	panelPolicy     panelKind = "policy"
	panelSettings   panelKind = "settings"
	panelAudit      panelKind = "audit"
	panelScan       panelKind = "scan"
	panelHelp       panelKind = "help"
)

// PanelContent is implemented by each panel type.
type PanelContent interface {
	// Update handles panel-mode messages.
	Update(tea.Msg) (PanelContent, tea.Cmd)
	// Title is shown in the panelhead.
	Title() string
	// Count is shown right-aligned in the panelhead.
	Count() string
	// Padded returns true for text-based panels (catalogs, policy, audit, help).
	Padded() bool
	// Body renders the inner content without borders.
	Body(width, height int) string
	// Footer renders the key hints row.
	Footer() string
	// Critical returns true if this panel uses red border (alerts in critical mode).
	Critical() bool
}

// PanelModel wraps a PanelContent and renders the full panel overlay.
type PanelModel struct {
	content PanelContent
	kind    panelKind
}

// NewPanelModel creates a PanelModel wrapping the given content.
func NewPanelModel(kind panelKind, content PanelContent) PanelModel {
	return PanelModel{kind: kind, content: content}
}

// Update delegates messages to the wrapped content.
func (p PanelModel) Update(msg tea.Msg) (PanelModel, tea.Cmd) {
	if p.content == nil {
		return p, nil
	}
	var cmd tea.Cmd
	p.content, cmd = p.content.Update(msg)
	return p, cmd
}

// View renders the full panel overlay at the given dimensions.
func (p PanelModel) View(width, height int) string {
	if p.content == nil {
		return ""
	}
	// Choose border color
	border := stylePanelBorder
	if p.content.Critical() {
		border = stylePanelBorderRed
	}

	// Panelhead: title left, count right
	headStyle := lipgloss.NewStyle().Background(colorPanel2).Foreground(colorWhite).Bold(true)
	countStyle := lipgloss.NewStyle().Background(colorPanel2).Foreground(colorDimmer)
	innerWidth := width - 4 // account for border padding
	titleStr := headStyle.Render(" " + p.content.Title())
	countStr := countStyle.Render(p.content.Count() + " ")
	padLen := innerWidth - lipgloss.Width(titleStr) - lipgloss.Width(countStr)
	if padLen < 0 {
		padLen = 0
	}
	head := titleStr + strings.Repeat(" ", padLen) + countStr

	// Panelbody
	bodyHeight := height - 6 // head + foot + border rows
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	bodyPad := ""
	if p.content.Padded() {
		bodyPad = "  "
	}
	body := bodyPad + p.content.Body(innerWidth, bodyHeight)

	// Panelfoot
	footStyle := lipgloss.NewStyle().Background(colorPanel2).Foreground(colorDim)
	foot := footStyle.Render(" " + p.content.Footer())

	combined := head + "\n" + body + "\n" + foot
	return border.Width(width - 2).Render(combined)
}
