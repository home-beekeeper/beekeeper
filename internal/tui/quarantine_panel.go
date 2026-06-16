package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	platform "github.com/home-beekeeper/beekeeper/internal/platform"
	quarantine "github.com/home-beekeeper/beekeeper/internal/quarantine"
)

// QuarantineActionMsg is sent back to App when a quarantine action completes.
type QuarantineActionMsg struct {
	Kind   string // "restore" | "purge"
	Target string // item ID for restore; "all" for purge
	Err    error
}

// QuarantinePanel implements PanelContent for the quarantine overlay.
type QuarantinePanel struct {
	items         []quarantine.Manifest
	selIdx        int
	adminMode     bool
	confirmPurge  bool
	quarantineDir string
}

// NewQuarantinePanel creates a QuarantinePanel.
func NewQuarantinePanel(adminMode bool) *QuarantinePanel {
	stateDir, _ := platform.StateDir()
	p := &QuarantinePanel{
		adminMode:     adminMode,
		quarantineDir: filepath.Join(stateDir, "quarantine"),
	}
	p.loadItems()
	return p
}

func (p *QuarantinePanel) loadItems() {
	items, err := quarantine.List(p.quarantineDir)
	if err == nil {
		p.items = items
	}
	if p.selIdx >= len(p.items) && len(p.items) > 0 {
		p.selIdx = len(p.items) - 1
	}
}

func (p *QuarantinePanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case stateTick:
		p.loadItems()

	case QuarantineActionMsg:
		p.loadItems()

	case tea.KeyPressMsg:
		k := msg.String()

		// Confirmation prompt handling
		if p.confirmPurge {
			switch k {
			case "y", "Y":
				p.confirmPurge = false
				return p, p.doPurge()
			default:
				p.confirmPurge = false
			}
			return p, nil
		}

		if !p.adminMode {
			// Non-admin: r/p keys are no-ops (admin flag required)
			switch k {
			case "j", "down":
				if p.selIdx < len(p.items)-1 {
					p.selIdx++
				}
			case "k", "up":
				if p.selIdx > 0 {
					p.selIdx--
				}
			}
			return p, nil
		}

		switch k {
		case "j", "down":
			if p.selIdx < len(p.items)-1 {
				p.selIdx++
			}
		case "k", "up":
			if p.selIdx > 0 {
				p.selIdx--
			}
		case "r", "R":
			if len(p.items) > 0 {
				return p, p.doRestore(p.items[p.selIdx].ID)
			}
		case "p", "P":
			if len(p.items) > 0 {
				p.confirmPurge = true
			}
		}
	}
	return p, nil
}

func (p *QuarantinePanel) doRestore(id string) tea.Cmd {
	dir := p.quarantineDir
	return func() tea.Msg {
		err := quarantine.Restore(dir, id)
		return QuarantineActionMsg{Kind: "restore", Target: id, Err: err}
	}
}

func (p *QuarantinePanel) doPurge() tea.Cmd {
	dir := p.quarantineDir
	return func() tea.Msg {
		_, err := quarantine.Purge(dir)
		return QuarantineActionMsg{Kind: "purge", Target: "all", Err: err}
	}
}

func (p *QuarantinePanel) Title() string { return "Quarantine" }

func (p *QuarantinePanel) Count() string {
	n := len(p.items)
	if n == 0 {
		return "empty"
	}
	return fmt.Sprintf("%d item held", n)
}

func (p *QuarantinePanel) Padded() bool { return false }

func (p *QuarantinePanel) Critical() bool { return false }

func (p *QuarantinePanel) Body(width, height int) string {
	if p.confirmPurge {
		return fmt.Sprintf("\n  Purge ALL %d items? [y/N]", len(p.items))
	}
	if len(p.items) == 0 {
		return "\n  " + styleDim.Render("(quarantine empty)")
	}

	var lines []string
	lines = append(lines, "")
	for i, m := range p.items {
		ts := m.QuarantinedAt.Format("15:04:05")
		badge := BadgeHeld()
		name := fmt.Sprintf("%s.%s %s", m.Publisher, m.Name, m.Version)
		meta := m.Reason
		if meta == "" {
			meta = "editor-extension"
		}

		timeStr := styleDim.Render(fmt.Sprintf("%-8s", ts))
		nameStr := lipgloss.NewStyle().Foreground(colorFg).Render(fmt.Sprintf("%-32s", name))
		metaStr := styleDimmer.Render(meta)

		line := "  " + timeStr + "  " + badge + "  " + nameStr + "  " + metaStr
		if i == p.selIdx {
			line = styleSelRow.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (p *QuarantinePanel) Footer() string {
	if !p.adminMode {
		return styleTeal.Render("enter") + styleDim.Render(" details · ") +
			styleTeal.Render("esc") + styleDim.Render(" close")
	}
	return styleTeal.Render("r") + styleDim.Render(" restore · ") +
		styleTeal.Render("p") + styleDim.Render(" purge · ") +
		styleTeal.Render("enter") + styleDim.Render(" details · ") +
		styleTeal.Render("esc") + styleDim.Render(" close")
}
