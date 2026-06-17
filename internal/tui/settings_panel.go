package tui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
	config "github.com/home-beekeeper/beekeeper/internal/config"
	platform "github.com/home-beekeeper/beekeeper/internal/platform"
)

// settingsEditErrMsg is emitted when a settings edit is rejected by the
// validation gate (config.Validate*), refused (unreadable config), or the save
// path fails. The App shows it as a warn toast; the on-disk config.json is left
// unchanged. It also carries the "saved, but the audit record failed" partial
// outcome (a warn, because the change DID apply).
type settingsEditErrMsg struct{ msg string }

// settingsSavedMsg is emitted after a settings edit persists successfully.
type settingsSavedMsg struct{ msg string }

func settingsErr(m string) tea.Cmd { return func() tea.Msg { return settingsEditErrMsg{msg: m} } }
func settingsOK(m string) tea.Cmd  { return func() tea.Msg { return settingsSavedMsg{msg: m} } }

type settingsRowKind int

const (
	setRowToggle settingsRowKind = iota // bool knob (on/off)
	setRowInt                           // numeric knob (+/- adjust)
	setRowInfo                          // read-only display
)

type settingsField string

const (
	fieldAQEnabled        settingsField = "aq_enabled"
	fieldAQDryRun         settingsField = "aq_dryrun"
	fieldAQThreshold      settingsField = "aq_threshold"
	fieldCorpusEnabled    settingsField = "corpus_enabled"
	fieldCorpusDownstream settingsField = "corpus_downstream"
)

// corpusDownstreamMin / Max bound the TUI cursor for the downstream-clean window.
// This is a friendlier subset of config's accepted range (0 or 1..3650): the
// panel offers a sane day-slider, while config.ValidateCorpusConfig is the real
// fail-closed gate on the load and save paths.
const (
	corpusDownstreamMin = 1
	corpusDownstreamMax = 365
)

// settingsRow is one rendered/navigable line in the editor.
type settingsRow struct {
	kind   settingsRowKind
	header string // section header rendered above this row when non-empty
	label  string
	field  settingsField
	value  string // setRowInt / setRowInfo display value
	on     bool   // setRowToggle state
}

// settingsChange describes one persisted edit for the audit trail (parity with
// `beekeeper config set`, which records every mutation).
type settingsChange struct {
	key   string // audit ReasonCode, e.g. "auto_quarantine.enabled"
	old   string
	new   string
	toast string // human-facing success message
}

// SettingsPanel implements PanelContent for the first-responder settings overlay.
//
// It edits the REAL user config (platform.ConfigPath, ~/.beekeeper/config.json —
// the same file `beekeeper config set` writes) for the two first-responder knobs
// that previously had no CLI or TUI surface and required hand-editing JSON:
//
//   - auto_quarantine: enabled, dry_run, threshold
//   - corpus:          enabled, downstream_clean_days
//
// Every edit is validated (config.ValidateAutoQuarantineConfig +
// config.ValidateCorpusConfig) and written through the atomic config.Save BEFORE
// it takes effect, so an invalid edit is rejected in the TUI and never persisted,
// and every successful change is recorded to the audit log. Editing is
// admin-gated (mirrors PolicyPanel / QuarantinePanel); navigation works without
// --admin. The destructive purge and the scope/path knobs stay config.json-only
// by design — see the read-only rows.
type SettingsPanel struct {
	adminMode  bool
	configPath string
	auditPath  string
	cfg        config.Config
	loadErr    bool // last config.Load failed → edits are refused (fail-closed write)
	rows       []settingsRow
	selIdx     int
}

// NewSettingsPanel constructs a panel bound to the user config path. A missing
// config is normal (config.Load returns documented defaults), so the panel opens
// against defaults and the first edit materializes the file.
func NewSettingsPanel(adminMode bool) *SettingsPanel {
	path, err := platform.ConfigPath()
	if err != nil {
		path = "config.json"
	}
	auditPath := ""
	if dir, derr := platform.AuditDir(); derr == nil {
		auditPath = filepath.Join(dir, "beekeeper.ndjson")
	}
	p := &SettingsPanel{adminMode: adminMode, configPath: path, auditPath: auditPath}
	p.reload()
	return p
}

// reload reads the user config and rebuilds the row model. On a load error
// (e.g. a corrupt or out-of-range config.json) it keeps the current in-memory
// cfg — fail-soft, never panics — AND records loadErr so persist() refuses to
// overwrite the unparsed file (fail-closed write): a malformed-but-recoverable
// config must not be silently clobbered by the panel's defaults.
func (p *SettingsPanel) reload() {
	cfg, err := config.Load(p.configPath)
	p.loadErr = err != nil
	if err == nil {
		p.cfg = cfg
	}
	p.rows = buildSettingsRows(p.cfg)
	p.clampSel()
}

func (p *SettingsPanel) clampSel() {
	if p.selIdx >= len(p.rows) {
		p.selIdx = len(p.rows) - 1
	}
	if p.selIdx < 0 {
		p.selIdx = 0
	}
}

// buildSettingsRows derives the navigable/rendered rows from cfg. The displayed
// values are the EFFECTIVE values (via the config accessors), so an absent block
// shows its documented default rather than a blank.
func buildSettingsRows(cfg config.Config) []settingsRow {
	return []settingsRow{
		{kind: setRowToggle, header: "Auto-quarantine  (reversible first-responder move)", label: "enabled", field: fieldAQEnabled, on: cfg.AutoQuarantineEnabled()},
		{kind: setRowToggle, label: "dry-run (audit only, no move)", field: fieldAQDryRun, on: cfg.AutoQuarantineDryRun()},
		{kind: setRowInt, label: "corroboration threshold", field: fieldAQThreshold, value: strconv.Itoa(cfg.AutoQuarantineThreshold())},

		{kind: setRowToggle, header: "Corpus  (local confirmed-incident record)", label: "enabled", field: fieldCorpusEnabled, on: cfg.Corpus.Enabled},
		{kind: setRowInt, label: "downstream-clean window (days)", field: fieldCorpusDownstream, value: strconv.Itoa(cfg.CorpusDownstreamCleanDays())},

		{kind: setRowInfo, header: "Read-only here  (edit config.json)", label: "corpus scope", value: corpusScopeDisplay(cfg)},
		{kind: setRowInfo, label: "corpus path", value: corpusPathDisplay(cfg)},
		{kind: setRowInfo, label: "purge", value: "always human-gated · never automatic"},
	}
}

func corpusScopeDisplay(cfg config.Config) string {
	if s := cfg.Corpus.Scope; s != "" && s != "org_only" {
		return s + " (reserved · no effect)"
	}
	return "org_only · community_shareable reserved"
}

func corpusPathDisplay(cfg config.Config) string {
	if cfg.Corpus.Path == "" {
		return "default (<state-dir>/corpus/beekeeper-corpus.ndjson)"
	}
	return cfg.Corpus.Path
}

// Update implements PanelContent. Navigation always works; edits are admin-gated.
func (p *SettingsPanel) Update(msg tea.Msg) (PanelContent, tea.Cmd) {
	switch msg := msg.(type) {
	case stateTick:
		// Reload unconditionally so external edits (CLI `config set`, a hand-edit)
		// surface. This panel has no buffered text-entry mode (unlike PolicyPanel),
		// so there is no in-flight input for a reload to clobber.
		p.reload()
	case tea.KeyPressMsg:
		return p, p.handleKey(msg.String())
	}
	return p, nil
}

// handleKey is the version-independent key dispatch (keyed on the string form so
// it is unit-testable without constructing a tea.KeyPressMsg). It returns a
// tea.Cmd carrying a toast message when an edit succeeds or is rejected.
func (p *SettingsPanel) handleKey(k string) tea.Cmd {
	// Navigation (available to all users, even when the config is unreadable).
	switch k {
	case "j", "down":
		if p.selIdx < len(p.rows)-1 {
			p.selIdx++
		}
		return nil
	case "k", "up":
		if p.selIdx > 0 {
			p.selIdx--
		}
		return nil
	}

	// Edits require admin mode.
	if !p.adminMode {
		return nil
	}

	r := p.curRow()
	if r == nil {
		return nil
	}

	switch r.kind {
	case setRowToggle:
		// `space` toggles (matches the footer hint and PolicyPanel, which reserves
		// `enter` for commit/open flows); +/- set the explicit on/off state.
		switch k {
		case "space":
			return p.setToggle(r.field, !r.on)
		case "+", "=", "l", "right":
			return p.setToggle(r.field, true)
		case "-", "_", "h", "left":
			return p.setToggle(r.field, false)
		}
	case setRowInt:
		switch k {
		case "+", "=", "l", "right":
			return p.adjustInt(r.field, 1)
		case "-", "_", "h", "left":
			return p.adjustInt(r.field, -1)
		}
	}
	return nil
}

func (p *SettingsPanel) curRow() *settingsRow {
	if p.selIdx < 0 || p.selIdx >= len(p.rows) {
		return nil
	}
	return &p.rows[p.selIdx]
}

// cloneConfigForEdit returns a candidate config with the AutoQuarantine pointer
// deep-copied so a failed persist never mutates the panel's in-memory cfg. Corpus
// is a value field, copied by the shallow struct copy.
func cloneConfigForEdit(c config.Config) config.Config {
	out := c
	if c.AutoQuarantine != nil {
		aq := *c.AutoQuarantine
		out.AutoQuarantine = &aq
	}
	return out
}

// setToggle flips a boolean knob and persists.
func (p *SettingsPanel) setToggle(field settingsField, on bool) tea.Cmd {
	cand := cloneConfigForEdit(p.cfg)
	var key, label string
	var old bool
	switch field {
	case fieldAQEnabled:
		old = p.cfg.AutoQuarantineEnabled()
		ensureAQ(&cand).Enabled = on
		key, label = "auto_quarantine.enabled", "auto-quarantine"
	case fieldAQDryRun:
		old = p.cfg.AutoQuarantineDryRun()
		ensureAQ(&cand).DryRun = on
		key, label = "auto_quarantine.dry_run", "auto-quarantine dry-run"
	case fieldCorpusEnabled:
		old = p.cfg.Corpus.Enabled
		cand.Corpus.Enabled = on
		key, label = "corpus.enabled", "corpus"
	default:
		return nil
	}
	return p.persist(cand, settingsChange{
		key:   key,
		old:   onOff(old),
		new:   onOff(on),
		toast: fmt.Sprintf("%s %s", label, onOff(on)),
	})
}

// adjustInt changes a numeric knob by delta, clamped to its valid range, and
// persists. The base value is read from the CANDIDATE (not p.cfg) so the delta is
// robust to any future buffered-edit / reload-race wrinkle (mirrors PolicyPanel).
func (p *SettingsPanel) adjustInt(field settingsField, delta int) tea.Cmd {
	cand := cloneConfigForEdit(p.cfg)
	switch field {
	case fieldAQThreshold:
		old := cand.AutoQuarantineThreshold()
		v := clampInt(old+delta, config.AutoQuarantineThresholdMin, config.AutoQuarantineThresholdMax)
		ensureAQ(&cand).Threshold = v
		return p.persist(cand, settingsChange{
			key:   "auto_quarantine.threshold",
			old:   strconv.Itoa(old),
			new:   strconv.Itoa(v),
			toast: fmt.Sprintf("threshold → %d", v),
		})
	case fieldCorpusDownstream:
		old := cand.CorpusDownstreamCleanDays()
		v := clampInt(old+delta, corpusDownstreamMin, corpusDownstreamMax)
		cand.Corpus.DownstreamCleanDays = v
		return p.persist(cand, settingsChange{
			key:   "corpus.downstream_clean_days",
			old:   strconv.Itoa(old),
			new:   strconv.Itoa(v),
			toast: fmt.Sprintf("downstream-clean → %d days", v),
		})
	}
	return nil
}

// ensureAQ returns the candidate's AutoQuarantine block, materializing it from
// the documented defaults if absent so a first edit writes a complete block.
func ensureAQ(cand *config.Config) *config.AutoQuarantineConfig {
	if cand.AutoQuarantine == nil {
		d := config.DefaultAutoQuarantineConfig()
		cand.AutoQuarantine = &d
	}
	return cand.AutoQuarantine
}

// persist is the single write path / last gate. It refuses to write when the
// current file is unreadable (fail-closed write — never clobber a recoverable
// config with defaults), validates the WHOLE candidate (auto_quarantine AND
// corpus), writes it via the atomic config.Save, reloads, and records the change
// to the audit log. On rejection it emits settingsEditErrMsg and leaves disk
// unchanged; on success it emits settingsSavedMsg. A failed audit write does not
// undo the saved change, so it is surfaced as a warn (the change applied, the
// trail did not), not a false success.
func (p *SettingsPanel) persist(cand config.Config, ch settingsChange) tea.Cmd {
	if p.loadErr {
		return settingsErr("config.json is unreadable — fix it before editing from the dashboard")
	}
	if cand.AutoQuarantine != nil {
		if err := config.ValidateAutoQuarantineConfig(*cand.AutoQuarantine); err != nil {
			return settingsErr(err.Error())
		}
	}
	if err := config.ValidateCorpusConfig(cand.Corpus); err != nil {
		return settingsErr(err.Error())
	}
	if err := config.Save(p.configPath, cand); err != nil {
		return settingsErr(err.Error())
	}
	p.reload()
	if auditErr := p.writeConfigChange(ch); auditErr != nil {
		return settingsErr(ch.toast + " · saved, but the audit record failed to write")
	}
	if ch.toast != "" {
		return settingsOK(ch.toast)
	}
	return nil
}

// writeConfigChange appends a config_change audit record for a successful edit,
// mirroring cmd/beekeeper/config.go writeConfigChangeRecord so dashboard edits
// are as forensically visible as `beekeeper config set`. A panel with no resolved
// audit sink (auditPath == "") or an empty key is a no-op.
func (p *SettingsPanel) writeConfigChange(ch settingsChange) error {
	if p.auditPath == "" || ch.key == "" {
		return nil
	}
	w, err := audit.NewWriter(p.auditPath)
	if err != nil {
		return err
	}
	defer w.Close()

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		copy(raw[:], []byte(fmt.Sprintf("%016x", time.Now().UnixNano())))
	}
	rec := audit.AuditRecord{
		RecordType:      "config_change",
		RecordID:        hex.EncodeToString(raw[:]),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ScannerName:     "beekeeper",
		Endpoint:        "dashboard",
		OriginalCommand: fmt.Sprintf("dashboard settings: %s=%s", ch.key, ch.new),
		Reason:          fmt.Sprintf("%s changed from %q to %q (dashboard)", ch.key, ch.old, ch.new),
		ReasonCode:      ch.key,
	}
	return w.Write(rec)
}

// Title implements PanelContent.
func (p *SettingsPanel) Title() string { return "First-responder settings" }

// Count implements PanelContent.
func (p *SettingsPanel) Count() string {
	return fmt.Sprintf("auto-q %s · corpus %s",
		onOff(p.cfg.AutoQuarantineEnabled()), onOff(p.cfg.Corpus.Enabled))
}

// Padded implements PanelContent.
func (p *SettingsPanel) Padded() bool { return true }

// Critical implements PanelContent.
func (p *SettingsPanel) Critical() bool { return false }

// Body implements PanelContent. width/height are unused (the panel frame sizes
// the overlay); rows are short, fixed lines like the policy and help panels.
func (p *SettingsPanel) Body(width, height int) string {
	lines := []string{""}
	if p.loadErr {
		lines = append(lines,
			"  "+styleRed.Render("config.json is unreadable — fix it before editing here"), "")
	}
	for i, row := range p.rows {
		if row.header != "" {
			if i != 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "  "+styleDimmer.Render(row.header))
		}
		lines = append(lines, p.renderRow(i, row))
	}
	lines = append(lines, "",
		"  "+styleDimmer.Render("Edits ~/.beekeeper/config.json · validated + audited before saving · enforced by the catalog daemon"))
	return strings.Join(lines, "\n")
}

func (p *SettingsPanel) renderRow(i int, row settingsRow) string {
	var text string
	switch row.kind {
	case setRowToggle:
		state := styleDim.Render("off")
		if row.on {
			state = styleGreen.Render("on")
		}
		text = "  " + styleDim.Render(fmt.Sprintf("%-32s", row.label)) + state
	case setRowInt:
		text = "  " + styleDim.Render(fmt.Sprintf("%-32s", row.label)) + styleTeal.Render(row.value)
	case setRowInfo:
		text = "  " + styleDimmer.Render(fmt.Sprintf("%-16s", row.label)+row.value)
	}
	if i == p.selIdx {
		text = styleSelRow.Render(strings.TrimRight(text, " "))
	}
	return text
}

// Footer implements PanelContent.
func (p *SettingsPanel) Footer() string {
	if p.adminMode {
		return styleTeal.Render("space") + styleDim.Render(" toggle · ") +
			styleTeal.Render("+/-") + styleDim.Render(" adjust · ") +
			styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
			styleTeal.Render("esc") + styleDim.Render(" close")
	}
	return styleTeal.Render("↑↓") + styleDim.Render(" select · ") +
		styleTeal.Render("esc") + styleDim.Render(" close · ") +
		styleDimmer.Render("--admin to edit")
}

// --- helpers ---

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
