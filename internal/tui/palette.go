package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// Command represents a single entry in the command palette.
type Command struct {
	Grp  string
	Name string
	Desc string
}

// commands is the LOCKED command list from the prototype (order and grouping are fixed).
var commands = []Command{
	{Grp: "scan", Name: "scan now", Desc: "run a deep bumblebee sweep"},
	{Grp: "scan", Name: "scan --quick", Desc: "lockfiles + extensions only"},
	{Grp: "scan", Name: "scan history", Desc: "view past scan results"},
	{Grp: "investigate", Name: "alerts", Desc: "open the sentry alert log"},
	{Grp: "investigate", Name: "quarantine", Desc: "review held items"},
	{Grp: "investigate", Name: "audit tail", Desc: "stream the raw event log"},
	{Grp: "configure", Name: "policy edit", Desc: "tune rules & thresholds"},
	{Grp: "configure", Name: "settings", Desc: "auto-quarantine & corpus knobs"},
	{Grp: "configure", Name: "catalogs", Desc: "source status & sync"},
	{Grp: "configure", Name: "protect install", Desc: "enable privileged sentry daemon"},
	{Grp: "system", Name: "help", Desc: "keybindings & concepts"},
	{Grp: "system", Name: "quit", Desc: "exit dashboard"},
}

// PaletteModel is the command palette overlay model.
type PaletteModel struct {
	query  string
	selIdx int
}

// filtered returns commands matching the current query (case-insensitive substring).
func (p PaletteModel) filtered() []Command {
	if p.query == "" {
		return commands
	}
	q := strings.ToLower(p.query)
	var out []Command
	for _, c := range commands {
		if strings.Contains(strings.ToLower(c.Name), q) ||
			strings.Contains(strings.ToLower(c.Desc), q) {
			out = append(out, c)
		}
	}
	return out
}

// Selected returns the currently selected command, or nil if none.
func (p PaletteModel) Selected() *Command {
	list := p.filtered()
	if p.selIdx >= 0 && p.selIdx < len(list) {
		c := list[p.selIdx]
		return &c
	}
	return nil
}

// Update handles key events in palette mode.
func (p PaletteModel) Update(msg tea.Msg) (PaletteModel, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	list := p.filtered()
	switch kp.String() {
	case "up":
		if p.selIdx > 0 {
			p.selIdx--
		}
	case "down":
		if p.selIdx < len(list)-1 {
			p.selIdx++
		}
	case "backspace":
		if len(p.query) > 0 {
			p.query = p.query[:len(p.query)-1]
			if p.selIdx >= len(p.filtered()) {
				p.selIdx = len(p.filtered()) - 1
			}
			if p.selIdx < 0 {
				p.selIdx = 0
			}
		}
	default:
		s := kp.String()
		// Append printable single characters to filter query.
		if len(s) == 1 && s[0] >= 0x20 && s[0] < 0x7f {
			p.query += s
			p.selIdx = 0
		}
	}
	return p, nil
}

// View renders the teal-bordered palette overlay.
func (p PaletteModel) View(width, height int) string {
	list := p.filtered()

	// Input row
	prompt := styleTeal.Render(":") + " " + lipgloss.NewStyle().Foreground(colorFg).Render(p.query) + "▌"
	inputRow := lipgloss.NewStyle().Background(colorPanel2).Padding(0, 1).Render(prompt)

	var lines []string
	lines = append(lines, inputRow)
	lines = append(lines, lipgloss.NewStyle().Foreground(colorBorderDim).Render(strings.Repeat("─", width-6)))

	if len(list) == 0 {
		emptyLine := "\n  " + styleDimmer.Render("no matching commands") + "\n"
		body := strings.Join([]string{inputRow, lipgloss.NewStyle().Foreground(colorBorderDim).Render(strings.Repeat("─", width-6)), emptyLine}, "\n")
		return stylePanelBorderTeal.Width(width - 4).Render(body)
	}

	currentGrp := ""
	for i, cmd := range list {
		if cmd.Grp != currentGrp {
			currentGrp = cmd.Grp
			lines = append(lines, styleDim.Render("  "+strings.ToUpper(currentGrp)))
		}
		arrow := styleDimmer.Render("▸")
		nameW := 22
		nameStr := cmd.Name
		if len(nameStr) < nameW {
			nameStr = nameStr + strings.Repeat(" ", nameW-len(nameStr))
		}
		descStr := styleDim.Render(cmd.Desc)

		var row string
		if i == p.selIdx {
			nameRend := styleWhite.Render(nameStr)
			arrowRend := styleTeal.Render("▸")
			row = styleSelRow.Render("  " + arrowRend + " " + nameRend + "  " + descStr)
		} else {
			row = "  " + arrow + " " + lipgloss.NewStyle().Foreground(colorFg).Render(nameStr) + "  " + descStr
		}
		lines = append(lines, row)
	}

	body := strings.Join(lines, "\n")
	return stylePanelBorderTeal.Width(width - 4).Render(body)
}
