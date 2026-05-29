package tui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	catalog "github.com/mzansi-agentive/beekeeper/internal/catalog"
	platform "github.com/mzansi-agentive/beekeeper/internal/platform"
)

// syncCatalogsMsg is sent to App when the user presses s in the catalogs panel.
// App handles it by showing a "Syncing all sources..." toast.
type syncCatalogsMsg struct{}

// knownSources lists the 4 catalog sources always shown in the panel (LOCKED order from prototype).
var knownSources = []struct {
	Name    string
	Type    string
	Default string // default sync info when state unknown
}{
	{Name: "bumblebee", Type: "threat_intel", Default: "never synced"},
	{Name: "osv", Type: "offline db", Default: "never synced"},
	{Name: "socket", Type: "public api", Default: "live query · rate-limited"},
	{Name: "self", Type: "beekeeper-self", Default: "clean · this build not flagged"},
}

// catalogExplainerText is the fixed explanatory text from the prototype.
const catalogExplainerText = `
Enforcement requires 2 of 3 independent sources to agree.
A single source can warn but cannot block — the 2FA principle
applied to threat intelligence.`

// pipColor returns the color for a freshness pip.
func pipColor(mtime time.Time, degraded bool) color.Color {
	if degraded {
		return colorAmber
	}
	if mtime.IsZero() {
		return colorDim // unknown
	}
	age := time.Since(mtime)
	if age > 24*time.Hour {
		return colorRed
	}
	if age > 2*time.Hour {
		return colorAmber
	}
	return colorGreen
}

// fmtAge formats a duration as a human-readable "Xm ago" / "Xh ago" string.
func fmtAge(mtime time.Time) string {
	if mtime.IsZero() {
		return "unknown"
	}
	age := time.Since(mtime)
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	}
}

// sourceIndexPath returns the catalog index file path used as a last-sync proxy.
func sourceIndexPath(catalogDir, name string) string {
	switch name {
	case "bumblebee":
		return filepath.Join(catalogDir, "bumblebee.idx")
	case "osv":
		return filepath.Join(catalogDir, "osv.json")
	case "socket":
		return filepath.Join(catalogDir, "socket.json")
	case "self":
		return filepath.Join(catalogDir, "beekeeper-self.json")
	default:
		return filepath.Join(catalogDir, name+".json")
	}
}

// CatalogsPanel implements PanelContent for the catalog sources overlay.
type CatalogsPanel struct {
	watchState  catalog.WatchState
	indexMtimes map[string]time.Time
	stateFile   string
	catalogDir  string
	bodyCache   string // pre-rendered body
}

// NewCatalogsPanel creates a CatalogsPanel, loading paths from platform.
func NewCatalogsPanel() *CatalogsPanel {
	stateDir, _ := platform.StateDir()
	catalogDir, _ := platform.CatalogDir()
	p := &CatalogsPanel{
		stateFile:   filepath.Join(stateDir, "state.json"),
		catalogDir:  catalogDir,
		indexMtimes: make(map[string]time.Time),
	}
	p.refresh()
	return p
}

func (p *CatalogsPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg.(type) {
	case stateTick:
		p.refresh()
	case tea.KeyPressMsg:
		if msg.(tea.KeyPressMsg).String() == "s" {
			p.refresh()
			return p, func() tea.Msg { return syncCatalogsMsg{} }
		}
	}
	return p, nil
}

func (p *CatalogsPanel) refresh() {
	// Load WatchState from state.json.
	ws, err := catalog.LoadState(p.stateFile)
	if err == nil {
		p.watchState = ws
	}
	if p.watchState.Sources == nil {
		p.watchState.Sources = make(map[string]catalog.SourceState)
	}

	// Read index file mtimes.
	for _, src := range knownSources {
		idxPath := sourceIndexPath(p.catalogDir, src.Name)
		if info, err := os.Stat(idxPath); err == nil {
			p.indexMtimes[src.Name] = info.ModTime()
		} else {
			p.indexMtimes[src.Name] = time.Time{}
		}
	}

	p.bodyCache = p.buildBody()
}

func (p *CatalogsPanel) buildBody() string {
	var sb strings.Builder
	sb.WriteString("\n")
	for _, src := range knownSources {
		ss := p.watchState.Sources[src.Name]
		mtime := p.indexMtimes[src.Name]
		pip := lipgloss.NewStyle().Foreground(pipColor(mtime, ss.Degraded)).Render("●")

		nameStr := styleWhite.Render(fmt.Sprintf("%-12s", src.Name))
		typeStr := styleDim.Render(fmt.Sprintf("%-16s", src.Type))

		var syncInfo string
		switch src.Name {
		case "socket":
			syncInfo = "live query · rate-limited"
		case "self":
			syncInfo = "clean · this build not flagged"
		default:
			if mtime.IsZero() {
				syncInfo = "never synced"
			} else {
				age := fmtAge(mtime)
				if ss.Count > 0 {
					syncInfo = fmt.Sprintf("synced %s · %d entries", age, ss.Count)
				} else {
					syncInfo = fmt.Sprintf("synced %s", age)
				}
			}
		}
		syncStr := styleDimmer.Render(syncInfo)

		sb.WriteString("  " + pip + " " + nameStr + "  " + typeStr + "  " + syncStr + "\n")
	}
	sb.WriteString(styleDimmer.Render(catalogExplainerText))
	sb.WriteString("\n")
	return sb.String()
}

func (p *CatalogsPanel) Title() string { return "Catalog sources" }

func (p *CatalogsPanel) Count() string {
	total := len(knownSources)
	// Count sources that are not degraded and have been synced.
	enforcing := 0
	for _, src := range knownSources {
		ss := p.watchState.Sources[src.Name]
		if !ss.Degraded && !p.indexMtimes[src.Name].IsZero() {
			enforcing++
		}
	}
	return fmt.Sprintf("%d sources · %d/%d to enforce", total, enforcing, 3)
}

func (p *CatalogsPanel) Padded() bool { return true }

func (p *CatalogsPanel) Critical() bool { return false }

func (p *CatalogsPanel) Body(width, height int) string {
	if p.bodyCache == "" {
		p.bodyCache = p.buildBody()
	}
	return p.bodyCache
}

func (p *CatalogsPanel) Footer() string {
	return styleTeal.Render("s") + styleDim.Render(" sync all · ") +
		styleTeal.Render("esc") + styleDim.Render(" close")
}
