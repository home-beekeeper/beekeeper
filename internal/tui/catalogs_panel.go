package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	catalog "github.com/bantuson/beekeeper/internal/catalog"
	config "github.com/bantuson/beekeeper/internal/config"
	platform "github.com/bantuson/beekeeper/internal/platform"
)

// syncCatalogsMsg is the in-progress signal: when the user presses s the panel
// batches it alongside the real async runSyncCmd so App shows a "Syncing…" toast
// immediately while the sync runs. The result arrives as syncDoneMsg.
type syncCatalogsMsg struct{}

// syncDoneMsg reports the outcome of a manual TUI catalog sync (handled by App).
type syncDoneMsg struct {
	err         error
	count       int
	notModified bool
}

// catalogScheduleOptions is the admin-gated background-sync cadence selector:
// 2h / 5h / 10h / 24h tighten or relax the interval; "off" disables background
// sync. Each is persisted via validate-before-write (ValidateCatalogSyncConfig).
var catalogScheduleOptions = []struct {
	Label    string
	Interval string
	Enabled  bool
}{
	{Label: "2h", Interval: "2h", Enabled: true},
	{Label: "5h", Interval: "5h", Enabled: true},
	{Label: "10h", Interval: "10h", Enabled: true},
	{Label: "24h", Interval: "24h", Enabled: true},
	{Label: "off", Interval: "", Enabled: false},
}

// knownSources lists the 4 catalog sources always shown in the panel (LOCKED order from prototype).
// Default is the honest sync line shown ONLY when no local index exists for the
// source — never an unverified status claim. socket is a live-query API with no
// persistent cache, so "live query" is its accurate idle state.
var knownSources = []struct {
	Name    string
	Type    string
	Default string // honest sync info when no local index is present
}{
	{Name: "bumblebee", Type: "threat_intel", Default: "never synced"},
	{Name: "osv", Type: "offline db", Default: "never synced"},
	{Name: "socket", Type: "public api", Default: "live query"},
	{Name: "self", Type: "beekeeper-self", Default: "not synced"},
}

// catalogExplainerText is the fixed explanatory text from the prototype.
const catalogExplainerText = `
Enforcement requires 2 of 3 independent sources to agree.
A single source can warn but cannot block — the 2FA principle
applied to threat intelligence.`

// pipColor returns the color for a freshness pip, keyed off the recorded
// SourceState (Phase 20). A degraded source is amber; a source whose last sync
// ATTEMPT is newer than its last SUCCESS is amber too — a failed sync never
// renders "fresh" green. Freshness otherwise comes from LastSuccess, falling
// back to the index mtime for sources that don't track sync timestamps
// (osv/socket/self).
func pipColor(ss catalog.SourceState, mtime time.Time) color.Color {
	if ss.Degraded {
		return colorAmber
	}
	if !ss.LastAttempt.IsZero() && ss.LastAttempt.After(ss.LastSuccess) {
		// A sync was attempted but did not succeed — surface amber, not fresh.
		return colorAmber
	}
	ref := ss.LastSuccess
	if ref.IsZero() {
		ref = mtime
	}
	if ref.IsZero() {
		return colorDim // unknown — never synced
	}
	age := time.Since(ref)
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
	configPath  string
	scheduleIdx int    // index into catalogScheduleOptions for the displayed cadence
	bodyCache   string // pre-rendered body
}

// NewCatalogsPanel creates a CatalogsPanel, loading paths from platform.
func NewCatalogsPanel() *CatalogsPanel {
	stateDir, _ := platform.StateDir()
	catalogDir, _ := platform.CatalogDir()
	configPath, _ := platform.ConfigPath()
	p := &CatalogsPanel{
		stateFile:   filepath.Join(stateDir, "state.json"),
		catalogDir:  catalogDir,
		configPath:  configPath,
		indexMtimes: make(map[string]time.Time),
	}
	p.scheduleIdx = p.currentScheduleIdx()
	p.refresh()
	return p
}

// currentScheduleIdx resolves the displayed cadence from the live config so the
// selector starts on the active interval (or "off" when disabled).
func (p *CatalogsPanel) currentScheduleIdx() int {
	if p.configPath == "" {
		return 1 // default display: 10h-ish midpoint when config is unavailable
	}
	cfg, err := config.LoadLayered(config.LayerOpts{UserPath: p.configPath, Environ: os.Environ()})
	if err != nil {
		return 1
	}
	if !cfg.CatalogSyncEnabled() {
		return len(catalogScheduleOptions) - 1 // "off"
	}
	want := cfg.CatalogSyncInterval()
	for i, opt := range catalogScheduleOptions {
		if opt.Enabled {
			if d, derr := time.ParseDuration(opt.Interval); derr == nil && d == want {
				return i
			}
		}
	}
	return 1
}

func (p *CatalogsPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch m := msg.(type) {
	case stateTick:
		p.refresh()
	case tea.KeyPressMsg:
		switch m.String() {
		case "s":
			// Manual sync is an explicit force-sync (bypasses the interval gate).
			// Batch the in-progress toast with the real async sync command.
			p.refresh()
			return p, tea.Batch(
				p.runSyncCmd(),
				func() tea.Msg { return syncCatalogsMsg{} },
			)
		case "i":
			// Cycle the background-sync cadence and persist via validate-before-write.
			p.scheduleIdx = (p.scheduleIdx + 1) % len(catalogScheduleOptions)
			opt := catalogScheduleOptions[p.scheduleIdx]
			cmd := p.persistSchedule(opt.Interval, opt.Enabled)
			p.refresh()
			return p, cmd
		}
	}
	return p, nil
}

// runSyncCmd performs a REAL catalog sync asynchronously (mirrors `catalogs sync
// --force`: manual is explicit, so it bypasses the interval gate) and records
// the bumblebee freshness fields in state.json. It returns a syncDoneMsg for the
// App to render as a result toast. Mirrors ScanPanel.runScanCmd.
func (p *CatalogsPanel) runSyncCmd() tea.Cmd {
	stateFile := p.stateFile
	catalogDir := p.catalogDir
	prevETag := p.watchState.Sources[catalogSyncSourceName].ETag
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client := &http.Client{Timeout: 30 * time.Second}
		res, err := catalog.SyncConditional(ctx, client, catalogDir, prevETag)

		// Record freshness fields. Reload fresh state so we never clobber a
		// concurrent watch-daemon write; preserve the source's other fields.
		if st, lerr := catalog.LoadState(stateFile); lerr == nil {
			if st.Sources == nil {
				st.Sources = make(map[string]catalog.SourceState)
			}
			ss := st.Sources[catalogSyncSourceName]
			ss.LastAttempt = time.Now()
			if err != nil {
				ss.LastError = err.Error()
			} else {
				ss.LastSuccess = ss.LastAttempt
				ss.LastError = ""
				ss.ETag = res.ETag
				if !res.NotModified {
					ss.Count = res.Count
				}
			}
			st.Sources[catalogSyncSourceName] = ss
			_ = catalog.SaveState(stateFile, st)
		}
		return syncDoneMsg{err: err, count: res.Count, notModified: res.NotModified}
	}
}

// persistSchedule validates and writes the background-sync cadence to the user
// config, reusing the policy editor's validate-before-write discipline: on a
// rejected interval it emits policyEditErrMsg and leaves disk untouched; on
// success it emits policySavedMsg.
func (p *CatalogsPanel) persistSchedule(interval string, enabled bool) tea.Cmd {
	if err := persistCatalogSyncInterval(p.configPath, interval, enabled); err != nil {
		m := err.Error()
		return func() tea.Msg { return policyEditErrMsg{msg: "catalog schedule rejected: " + m} }
	}
	label := "off"
	if enabled {
		label = interval
	}
	ok := fmt.Sprintf("catalog sync schedule set to %s", label)
	return func() tea.Msg { return policySavedMsg{msg: ok} }
}

// catalogSyncSourceName is the bumblebee SourceState key (matches the CLI).
const catalogSyncSourceName = "bumblebee"

// persistCatalogSyncInterval validates interval+enabled via the EXPORTED
// ValidateCatalogSyncConfig and, only if valid, writes the catalog_sync block to
// the user config at cfgPath preserving all other keys (validate-before-write).
// An invalid interval returns an error WITHOUT touching disk. Empty interval is
// allowed (used by "off", which sets enabled:false and leaves interval as-is).
func persistCatalogSyncInterval(cfgPath, interval string, enabled bool) error {
	if cfgPath == "" {
		return fmt.Errorf("config path unavailable")
	}
	if err := config.ValidateCatalogSyncConfig(config.CatalogSyncConfig{Enabled: enabled, Interval: interval}); err != nil {
		return err
	}
	cfg := map[string]any{}
	if data, rerr := os.ReadFile(cfgPath); rerr == nil {
		if jerr := json.Unmarshal(data, &cfg); jerr != nil {
			return fmt.Errorf("existing config is unparseable: %w", jerr)
		}
		if cfg == nil {
			cfg = map[string]any{}
		}
	}
	cs, _ := cfg["catalog_sync"].(map[string]any)
	if cs == nil {
		cs = map[string]any{}
	}
	cs["enabled"] = enabled
	if interval != "" {
		cs["interval"] = interval
	}
	cfg["catalog_sync"] = cs

	data, merr := json.MarshalIndent(cfg, "", "  ")
	if merr != nil {
		return merr
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0o600)
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
		pip := lipgloss.NewStyle().Foreground(pipColor(ss, mtime)).Render("●")

		nameStr := styleWhite.Render(fmt.Sprintf("%-12s", src.Name))
		typeStr := styleDim.Render(fmt.Sprintf("%-16s", src.Type))

		// Sync info is driven by the source's REAL state — degraded flag and the
		// index mtime/entry count from WatchState — for every source uniformly.
		// The honest per-source Default is shown only when no local index exists.
		var syncInfo string
		switch {
		case ss.Degraded:
			syncInfo = "degraded"
		case mtime.IsZero():
			syncInfo = src.Default
		default:
			age := fmtAge(mtime)
			if ss.Count > 0 {
				syncInfo = fmt.Sprintf("synced %s · %d entries", age, ss.Count)
			} else {
				syncInfo = fmt.Sprintf("synced %s", age)
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
	// Count sources that are not degraded and have a local index (fresh).
	fresh := 0
	for _, src := range knownSources {
		ss := p.watchState.Sources[src.Name]
		if !ss.Degraded && !p.indexMtimes[src.Name].IsZero() {
			fresh++
		}
	}
	return fmt.Sprintf("%d sources · %d fresh", total, fresh)
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
	sched := catalogScheduleOptions[p.scheduleIdx].Label
	return styleTeal.Render("s") + styleDim.Render(" sync all · ") +
		styleTeal.Render("i") + styleDim.Render(" schedule ("+sched+") · ") +
		styleTeal.Render("esc") + styleDim.Render(" close")
}
