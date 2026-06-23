package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/home-beekeeper/beekeeper/internal/config"
)

// newTestSettingsPanel builds a panel bound to a temp config path (bypassing
// platform.ConfigPath), mirroring NewSettingsPanel. The file does not exist yet,
// so the panel opens against documented defaults. auditPath is left empty, so
// writeConfigChange is a no-op and the tests do not pollute a real audit log
// (the audit path is exercised explicitly in TestSettingsPanelWritesAuditRecord).
func newTestSettingsPanel(t *testing.T, admin bool) (*SettingsPanel, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	p := &SettingsPanel{adminMode: admin, configPath: path}
	p.reload()
	return p, path
}

// rowByField returns the index of the row editing the given field.
func (p *SettingsPanel) rowByField(field settingsField) int {
	for i, r := range p.rows {
		if r.field == field {
			return i
		}
	}
	return -1
}

// rowByKind returns the index of the first row of the given kind.
func (p *SettingsPanel) rowByKind(kind settingsRowKind) int {
	for i, r := range p.rows {
		if r.kind == kind {
			return i
		}
	}
	return -1
}

// loadSaved reads the on-disk config the way the daemon does.
func loadSaved(t *testing.T, path string) config.Config {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", path, err)
	}
	return cfg
}

// TestSettingsPanelDefaults verifies a fresh panel shows the documented defaults
// (auto-quarantine off + dry-run on + threshold 2, corpus off + window 30) and
// writes nothing until an edit happens.
func TestSettingsPanelDefaults(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a fresh panel must not write config.json before any edit (stat err=%v)", err)
	}
	if p.rows[p.rowByField(fieldAQEnabled)].on {
		t.Errorf("auto-quarantine should default off")
	}
	if !p.rows[p.rowByField(fieldAQDryRun)].on {
		t.Errorf("dry-run should default on")
	}
	if got := p.rows[p.rowByField(fieldAQThreshold)].value; got != "2" {
		t.Errorf("threshold default = %q, want 2", got)
	}
	if p.rows[p.rowByField(fieldCorpusEnabled)].on {
		t.Errorf("corpus should default off")
	}
	if got := p.rows[p.rowByField(fieldCorpusDownstream)].value; got != "30" {
		t.Errorf("downstream-clean default = %q, want 30", got)
	}
}

// TestSettingsPanelEnableAutoQuarantineEnforced is the end-to-end proof: a toggle
// in the TUI changes the config.json the catalog daemon actually loads.
func TestSettingsPanelEnableAutoQuarantineEnforced(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)

	p.selIdx = p.rowByField(fieldAQEnabled)
	cmd := p.handleKey("space")
	if cmd == nil {
		t.Fatal("toggle should emit a saved-toast cmd")
	}
	if _, isErr := cmd().(settingsEditErrMsg); isErr {
		t.Fatalf("valid enable toggle was rejected")
	}
	if !loadSaved(t, path).AutoQuarantineEnabled() {
		t.Errorf("enforced auto_quarantine.enabled = false after TUI toggle, want true")
	}
	if !p.rows[p.rowByField(fieldAQEnabled)].on {
		t.Errorf("panel row did not refresh after enable")
	}
}

// TestSettingsPanelDryRunOff proves disabling dry-run persists.
func TestSettingsPanelDryRunOff(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	p.selIdx = p.rowByField(fieldAQDryRun)
	p.handleKey("-") // set false
	if loadSaved(t, path).AutoQuarantineDryRun() {
		t.Errorf("dry_run still true after TUI set-off")
	}
}

// TestSettingsPanelThresholdClampedInRange proves the cursor stays inside the
// validator's accepted [1,3] band, so every saved value loads cleanly.
func TestSettingsPanelThresholdClampedInRange(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	p.selIdx = p.rowByField(fieldAQThreshold)

	p.handleKey("+")
	p.handleKey("+")
	if got := loadSaved(t, path).AutoQuarantineThreshold(); got != 3 {
		t.Fatalf("threshold after two increments = %d, want 3 (clamped)", got)
	}
	p.handleKey("-")
	p.handleKey("-")
	p.handleKey("-")
	if got := loadSaved(t, path).AutoQuarantineThreshold(); got != 1 {
		t.Fatalf("threshold after flooring = %d, want 1", got)
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("config.json invalid after threshold edits: %v", err)
	}
}

// TestSettingsPanelCorpusKnobs proves the corpus enable toggle and the
// downstream-clean window both persist and clamp.
func TestSettingsPanelCorpusKnobs(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)

	p.selIdx = p.rowByField(fieldCorpusEnabled)
	p.handleKey("space")
	if !loadSaved(t, path).Corpus.Enabled {
		t.Errorf("corpus.enabled false after TUI toggle, want true")
	}

	p.selIdx = p.rowByField(fieldCorpusDownstream)
	p.handleKey("-") // 30 -> 29
	if got := loadSaved(t, path).CorpusDownstreamCleanDays(); got != 29 {
		t.Errorf("downstream-clean = %d after one decrement, want 29", got)
	}
}

// TestSettingsPanelCorpusOnlyEditDoesNotMaterializeAutoQuarantine proves a
// corpus-only edit on a fresh config leaves auto_quarantine absent (nil), so the
// panel does not spuriously write an auto_quarantine block.
func TestSettingsPanelCorpusOnlyEditDoesNotMaterializeAutoQuarantine(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	p.selIdx = p.rowByField(fieldCorpusEnabled)
	p.handleKey("space")
	saved := loadSaved(t, path)
	if saved.AutoQuarantine != nil {
		t.Errorf("corpus-only edit materialized an auto_quarantine block: %+v", *saved.AutoQuarantine)
	}
	if !saved.Corpus.Enabled {
		t.Errorf("corpus edit did not persist")
	}
}

// TestSettingsPanelDownstreamFloor proves the window cannot go below its floor.
func TestSettingsPanelDownstreamFloor(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	p.cfg.Corpus.DownstreamCleanDays = corpusDownstreamMin
	p.rows = buildSettingsRows(p.cfg)
	p.selIdx = p.rowByField(fieldCorpusDownstream)
	p.handleKey("-")
	if got := loadSaved(t, path).CorpusDownstreamCleanDays(); got != corpusDownstreamMin {
		t.Errorf("downstream-clean = %d, want floor %d", got, corpusDownstreamMin)
	}
}

// TestSettingsPanelNonAdminCannotEdit proves edits are admin-gated: without
// --admin a toggle is a no-op and config.json is never written.
func TestSettingsPanelNonAdminCannotEdit(t *testing.T) {
	p, path := newTestSettingsPanel(t, false)
	p.selIdx = p.rowByField(fieldAQEnabled)
	if cmd := p.handleKey("space"); cmd != nil {
		t.Errorf("non-admin toggle should be a no-op (got a cmd)")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("non-admin must not write config.json (stat err=%v)", err)
	}
}

// TestSettingsPanelNonAdminCanNavigate proves navigation works without --admin
// (only edits are gated).
func TestSettingsPanelNonAdminCanNavigate(t *testing.T) {
	p, _ := newTestSettingsPanel(t, false)
	p.selIdx = 0
	p.handleKey("j")
	if p.selIdx != 1 {
		t.Errorf("non-admin down should move cursor to 1, got %d", p.selIdx)
	}
	p.handleKey("k")
	if p.selIdx != 0 {
		t.Errorf("non-admin up should move cursor to 0, got %d", p.selIdx)
	}
}

// TestSettingsPanelNavigation proves j/k movement and clamping at the ends.
func TestSettingsPanelNavigation(t *testing.T) {
	p, _ := newTestSettingsPanel(t, true)
	p.selIdx = 0
	p.handleKey("k") // already at top, stays
	if p.selIdx != 0 {
		t.Errorf("up at top moved cursor to %d", p.selIdx)
	}
	for range p.rows {
		p.handleKey("j")
	}
	if p.selIdx != len(p.rows)-1 {
		t.Errorf("down past bottom = %d, want %d", p.selIdx, len(p.rows)-1)
	}
}

// TestSettingsPanelEnterDoesNotToggle pins the WR-03 fix: enter is NOT a toggle
// (only space is), matching the footer hint and PolicyPanel.
func TestSettingsPanelEnterDoesNotToggle(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	p.selIdx = p.rowByField(fieldAQEnabled)
	if cmd := p.handleKey("enter"); cmd != nil {
		t.Errorf("enter on a toggle row should be a no-op, got a cmd")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("enter must not write config.json (stat err=%v)", err)
	}
}

// TestSettingsPanelInfoRowNoOp proves an edit key on a read-only row is a no-op.
func TestSettingsPanelInfoRowNoOp(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	p.selIdx = p.rowByKind(setRowInfo)
	for _, k := range []string{"+", "-", "space", "enter"} {
		if cmd := p.handleKey(k); cmd != nil {
			t.Errorf("key %q on an info row should be a no-op, got a cmd", k)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("info-row keys must not write config.json (stat err=%v)", err)
	}
}

// TestSettingsPanelValidationRejectsAndLeavesDiskUnchanged drives the persist
// gate directly with an out-of-range threshold (unreachable through the clamped
// UI) and proves it is rejected with no write.
func TestSettingsPanelValidationRejectsAndLeavesDiskUnchanged(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	p.selIdx = p.rowByField(fieldAQThreshold)
	p.handleKey("+") // -> 3, a valid save so the file exists
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected a file after a valid edit: %v", err)
	}

	cand := cloneConfigForEdit(p.cfg)
	ensureAQ(&cand).Threshold = 5 // outside [1,3] — validator must reject
	cmd := p.persist(cand, settingsChange{key: "auto_quarantine.threshold", toast: "should-not-save"})
	if cmd == nil {
		t.Fatal("persist of an invalid threshold should emit an error cmd")
	}
	if _, isErr := cmd().(settingsEditErrMsg); !isErr {
		t.Fatalf("invalid threshold was not rejected with settingsEditErrMsg")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Errorf("config.json changed despite a rejected edit")
	}
}

// TestSettingsPanelCorpusValidationRejects proves the corpus block is now part of
// the persist gate (the WR-01 hole): an out-of-range / bad-scope candidate is
// rejected with no write, even though the AutoQuarantine block is valid.
func TestSettingsPanelCorpusValidationRejects(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	cand := cloneConfigForEdit(p.cfg)
	cand.Corpus.Scope = "world_readable" // not a legal scope
	cmd := p.persist(cand, settingsChange{key: "corpus.scope", toast: "x"})
	if cmd == nil {
		t.Fatal("persist of an invalid corpus scope should emit an error cmd")
	}
	if _, isErr := cmd().(settingsEditErrMsg); !isErr {
		t.Fatalf("invalid corpus scope was not rejected")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a rejected corpus edit must not write config.json (stat err=%v)", err)
	}
}

// TestSettingsPanelRefusesWriteWhenConfigUnreadable pins the WR-02 fix: a corrupt
// config.json sets loadErr, surfaces a banner, and an edit is REFUSED rather than
// overwriting the recoverable file with the panel's defaults.
func TestSettingsPanelRefusesWriteWhenConfigUnreadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	corrupt := []byte("{ this is : not valid json ]\n")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	p := &SettingsPanel{adminMode: true, configPath: path}
	p.reload()
	if !p.loadErr {
		t.Fatal("a corrupt config.json must set loadErr")
	}
	if !strings.Contains(p.Body(80, 24), "unreadable") {
		t.Error("Body should surface the unreadable-config banner")
	}
	p.selIdx = p.rowByField(fieldAQEnabled)
	cmd := p.handleKey("space")
	if cmd == nil {
		t.Fatal("an edit against an unreadable config should emit an error cmd")
	}
	if _, isErr := cmd().(settingsEditErrMsg); !isErr {
		t.Fatalf("edit against unreadable config was not refused")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(corrupt) {
		t.Errorf("the recoverable corrupt config was overwritten:\n%s", after)
	}
}

// TestSettingsPanelSaveErrorSurfaced exercises the config.Save failure branch: a
// config path whose parent directory does not exist loads as defaults (missing =
// defaults, loadErr false) but cannot be written, so persist surfaces the error.
func TestSettingsPanelSaveErrorSurfaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "config.json")
	p := &SettingsPanel{adminMode: true, configPath: path}
	p.reload()
	if p.loadErr {
		t.Fatal("a missing file (parent absent) should load as defaults, not loadErr")
	}
	p.selIdx = p.rowByField(fieldAQEnabled)
	cmd := p.handleKey("space")
	if cmd == nil {
		t.Fatal("a failed save should emit an error cmd")
	}
	if _, isErr := cmd().(settingsEditErrMsg); !isErr {
		t.Fatalf("a config.Save failure was not surfaced as settingsEditErrMsg")
	}
}

// TestSettingsPanelWritesAuditRecord proves a successful edit appends a
// config_change record to the audit log (parity with `beekeeper config set`).
func TestSettingsPanelWritesAuditRecord(t *testing.T) {
	dir := t.TempDir()
	p := &SettingsPanel{
		adminMode:  true,
		configPath: filepath.Join(dir, "config.json"),
		auditPath:  filepath.Join(dir, "beekeeper.ndjson"),
	}
	p.reload()
	p.selIdx = p.rowByField(fieldAQEnabled)
	if cmd := p.handleKey("space"); cmd != nil {
		if _, isErr := cmd().(settingsEditErrMsg); isErr {
			t.Fatalf("valid enable toggle was rejected")
		}
	}
	data, err := os.ReadFile(p.auditPath)
	if err != nil {
		t.Fatalf("audit log not written: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"record_type":"config_change"`) {
		t.Errorf("audit record is not a config_change record:\n%s", s)
	}
	if !strings.Contains(s, "auto_quarantine.enabled") {
		t.Errorf("audit record does not name the changed key:\n%s", s)
	}
}

// TestSettingsPanelReloadSurfacesExternalEdit proves a stateTick reload reflects
// a change made outside the panel (CLI `config set`, a hand-edit).
func TestSettingsPanelReloadSurfacesExternalEdit(t *testing.T) {
	p, path := newTestSettingsPanel(t, true)
	external := config.Config{
		FailMode:       config.FailModeClosed,
		AutoQuarantine: &config.AutoQuarantineConfig{Enabled: true, DryRun: false, Threshold: 3},
	}
	if err := config.Save(path, external); err != nil {
		t.Fatalf("seed external config: %v", err)
	}
	p.Update(stateTick{})
	if !p.rows[p.rowByField(fieldAQEnabled)].on {
		t.Errorf("panel did not surface the externally-enabled auto-quarantine")
	}
}

// TestSettingsPanelRender exercises Title/Count/Body/Footer/renderRow for both
// admin and non-admin so the render paths are covered and stay panic-free.
func TestSettingsPanelRender(t *testing.T) {
	for _, admin := range []bool{true, false} {
		p, _ := newTestSettingsPanel(t, admin)
		if p.Title() == "" {
			t.Error("empty Title")
		}
		if !strings.Contains(p.Count(), "auto-q") {
			t.Errorf("Count missing summary: %q", p.Count())
		}
		if !p.Padded() || p.Critical() {
			t.Error("expected Padded=true, Critical=false")
		}
		for i := range p.rows {
			p.selIdx = i
			if body := p.Body(80, 24); body == "" {
				t.Errorf("empty Body at selIdx %d", i)
			}
		}
		if foot := p.Footer(); foot == "" {
			t.Error("empty Footer")
		}
	}
}

// TestSettingsDisplayHelpers covers the scope/path display variants and onOff.
func TestSettingsDisplayHelpers(t *testing.T) {
	def := config.Config{}
	if !strings.Contains(corpusScopeDisplay(def), "org_only") {
		t.Errorf("default scope display = %q", corpusScopeDisplay(def))
	}
	custom := config.Config{Corpus: config.CorpusConfig{Scope: "community_shareable", Path: "/tmp/c.ndjson"}}
	if !strings.Contains(corpusScopeDisplay(custom), "reserved") {
		t.Errorf("non-default scope must be flagged reserved: %q", corpusScopeDisplay(custom))
	}
	if corpusPathDisplay(def) == corpusPathDisplay(custom) {
		t.Errorf("custom path should differ from default-path display")
	}
	if onOff(true) != "on" || onOff(false) != "off" {
		t.Errorf("onOff mapping wrong")
	}
}

// TestSettingsCloneIsolation proves cloneConfigForEdit deep-copies the
// AutoQuarantine pointer so a rejected edit cannot mutate the live cfg.
func TestSettingsCloneIsolation(t *testing.T) {
	orig := config.Config{AutoQuarantine: &config.AutoQuarantineConfig{Threshold: 2}}
	clone := cloneConfigForEdit(orig)
	ensureAQ(&clone).Threshold = 3
	if orig.AutoQuarantine.Threshold != 2 {
		t.Errorf("mutating the clone changed the original (got %d)", orig.AutoQuarantine.Threshold)
	}
}
