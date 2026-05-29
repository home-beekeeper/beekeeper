package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// IncidentAction is a containment action button shown on the incident card.
type IncidentAction struct {
	Key string // "Q", "I", "d"
	Cls string // "danger", "warn", "info"
	Lbl string
}

// TreeLine is a single line in the process tree display.
type TreeLine struct {
	Prefix      string
	PrefixStyle lipgloss.Style // style for the connector/verb prefix
	Style       lipgloss.Style // style for the main content text
	Text        string
}

// IncidentModel holds the data and state for the critical incident card.
type IncidentModel struct {
	RuleID    string
	RuleName  string
	Timestamp string
	Desc      string
	Tree      []TreeLine
	Actions   []IncidentAction
	SelAction int
}

// DefaultIncident returns the prototype "exfil-signature-fusion" demo incident.
func DefaultIncident() IncidentModel {
	return IncidentModel{
		RuleID:    "R5",
		RuleName:  "exfil-signature-fusion",
		Timestamp: "14:21:54",
		Desc: lipgloss.NewStyle().Foreground(colorFg).Render(
			"A process from the VS Code extension host read three credential\n"+
				"files and opened an outbound connection, within 4 minutes of installing\n"+
				"an extension flagged by ") +
			lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("2 of 3 catalogs") +
			lipgloss.NewStyle().Foreground(colorFg).Render("."),
		Tree: []TreeLine{
			{Prefix: "", PrefixStyle: styleDim, Style: styleDim, Text: "Code Helper (Plugin) pid 8821"},
			{Prefix: "└─ ", PrefixStyle: styleDim, Style: styleCoral, Text: "node extension.js pid 8847"},
			{Prefix: "   ├─ read ", PrefixStyle: styleDim, Style: styleRed, Text: "~/.aws/credentials"},
			{Prefix: "   ├─ read ", PrefixStyle: styleDim, Style: styleRed, Text: "~/.config/op/config"},
			{Prefix: "   ├─ read ", PrefixStyle: styleDim, Style: styleRed, Text: "~/.ssh/id_ed25519"},
			{Prefix: "   └─ ", PrefixStyle: styleDim, Style: styleRed, Text: "POST 185.2.x.x:443  4.2KB"},
		},
		Actions: []IncidentAction{
			{Key: "Q", Cls: "danger", Lbl: "quarantine extension"},
			{Key: "I", Cls: "warn", Lbl: "isolate process"},
			{Key: "d", Cls: "info", Lbl: "full record"},
		},
		SelAction: 0,
	}
}

// Update handles key navigation across action buttons.
func (m IncidentModel) Update(msg tea.Msg) (IncidentModel, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.String() {
		case "up", "left":
			if m.SelAction > 0 {
				m.SelAction--
			}
		case "down", "right":
			if m.SelAction < len(m.Actions)-1 {
				m.SelAction++
			}
		}
	}
	return m, nil
}

// View renders the red-bordered incident card at the given width.
func (m IncidentModel) View(width int) string {
	var sb strings.Builder

	// Head
	badge := BadgeCrit()
	title := styleWhite.Render(" " + m.RuleName)
	ts := styleDim.Render("  " + m.Timestamp + " · sentry · rule " + m.RuleID + " ")
	sb.WriteString(badge + title + ts + "\n\n")

	// Description — pre-rendered with inline ANSI
	sb.WriteString(m.Desc + "\n\n")

	// Process tree
	sb.WriteString(styleDim.Render("PROCESS TREE") + "\n")
	for _, line := range m.Tree {
		sb.WriteString(line.PrefixStyle.Render(line.Prefix) + line.Style.Render(line.Text) + "\n")
	}
	sb.WriteString("\n")

	// Action buttons
	var btns []string
	for i, act := range m.Actions {
		var keyStyle lipgloss.Style
		switch act.Cls {
		case "danger":
			keyStyle = styleCoral
		case "warn":
			keyStyle = styleAmber
		default:
			keyStyle = styleTeal
		}
		btnContent := keyStyle.Render("["+act.Key+"]") + " " + act.Lbl
		var btnStyle lipgloss.Style
		if i == m.SelAction {
			btnStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorTeal).
				Background(colorSelbg).
				Foreground(colorWhite).
				Padding(0, 1)
		} else {
			btnStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Background(colorPanel).
				Padding(0, 1)
		}
		btns = append(btns, btnStyle.Render(btnContent))
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, btns...) + "\n")
	sb.WriteString(lipgloss.PlaceHorizontal(width-4, lipgloss.Right, styleDim.Render("rotate exposed creds after containment")) + "\n")

	body := sb.String()
	return styleIncidentBorder.Width(width - 4).Render(body)
}
