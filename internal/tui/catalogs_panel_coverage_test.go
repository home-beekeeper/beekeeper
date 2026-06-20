package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	catalog "github.com/home-beekeeper/beekeeper/internal/catalog"
)

// TestCatalogsPanelContract covers the small PanelContent surface and the
// fresh-source count.
func TestCatalogsPanelContract(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.indexMtimes["bumblebee"] = time.Now().Add(-30 * time.Minute) // one fresh source
	p.bodyCache = p.buildBody()

	if p.Title() != "Catalog sources" {
		t.Errorf("Title = %q", p.Title())
	}
	if !p.Padded() {
		t.Error("catalogs panel should be padded")
	}
	if p.Critical() {
		t.Error("catalogs panel must not be critical")
	}
	count := p.Count()
	if !strings.Contains(count, "4 sources") {
		t.Errorf("Count = %q, want '4 sources'", count)
	}
	if !strings.Contains(count, "1 fresh") {
		t.Errorf("Count = %q, want '1 fresh' (bumblebee has a recent index)", count)
	}
	if !strings.Contains(p.Footer(), "sync all") {
		t.Errorf("Footer = %q, want a 'sync all' hint", p.Footer())
	}
}

// TestCatalogsPanelBodyDegradedAndNeverSynced exercises the buildBody branches:
// a degraded source, a never-synced source (Default text), and a synced source
// with an entry count.
func TestCatalogsPanelBodyVariants(t *testing.T) {
	p := newCatalogsPanelForTest()
	// bumblebee: synced with a count.
	p.watchState.Sources["bumblebee"] = catalog.SourceState{Count: 1234}
	p.indexMtimes["bumblebee"] = time.Now().Add(-10 * time.Minute)
	// osv: degraded.
	p.watchState.Sources["osv"] = catalog.SourceState{Degraded: true}
	p.indexMtimes["osv"] = time.Now().Add(-5 * time.Minute)
	// socket: never synced (no index) → shows its Default ("live query").
	p.indexMtimes["socket"] = time.Time{}
	p.bodyCache = p.buildBody()

	body := p.Body(100, 40)
	if !strings.Contains(body, "1234 entries") {
		t.Errorf("body should show the synced entry count: %q", body)
	}
	if !strings.Contains(body, "degraded") {
		t.Errorf("body should show the degraded source: %q", body)
	}
	if !strings.Contains(body, "live query") {
		t.Errorf("body should show the never-synced Default for socket: %q", body)
	}
	if !strings.Contains(body, "2 of 3 independent sources") {
		t.Errorf("body should include the corroboration explainer: %q", body)
	}
}

// TestCatalogsPanelBodyLazyBuild proves Body builds the cache on demand when it
// is empty.
func TestCatalogsPanelBodyLazyBuild(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.bodyCache = "" // force the lazy-build branch
	if got := p.Body(80, 24); got == "" {
		t.Error("Body should lazily build the body cache when empty")
	}
}

// TestFmtAge covers the just-now / minutes / hours / unknown branches.
func TestFmtAge(t *testing.T) {
	if got := fmtAge(time.Time{}); got != "unknown" {
		t.Errorf("zero time = %q, want unknown", got)
	}
	if got := fmtAge(time.Now().Add(-30 * time.Second)); got != "just now" {
		t.Errorf("30s ago = %q, want 'just now'", got)
	}
	if got := fmtAge(time.Now().Add(-30 * time.Minute)); !strings.Contains(got, "m ago") {
		t.Errorf("30m ago = %q, want minutes", got)
	}
	if got := fmtAge(time.Now().Add(-3 * time.Hour)); !strings.Contains(got, "h ago") {
		t.Errorf("3h ago = %q, want hours", got)
	}
}

// TestPipColorNeverSynced covers the unknown (never-synced, no mtime) branch.
func TestPipColorNeverSynced(t *testing.T) {
	if c := pipColor(catalog.SourceState{}, time.Time{}); c != colorDim {
		t.Errorf("never-synced pip = %v, want colorDim", c)
	}
}

// TestSourceIndexPath covers the per-source filename mapping including the
// default branch.
func TestSourceIndexPath(t *testing.T) {
	dir := "/cat"
	cases := map[string]string{
		"bumblebee": "bumblebee.idx",
		"osv":       "osv.json",
		"socket":    "socket.json",
		"self":      "beekeeper-self.json",
		"other":     "other.json", // default branch
	}
	for name, want := range cases {
		got := sourceIndexPath(dir, name)
		if filepath.Base(got) != want {
			t.Errorf("sourceIndexPath(%q) = %q, want base %q", name, got, want)
		}
	}
}

// TestCatalogsScheduleSelectorCycles proves pressing 'i' cycles the cadence and
// persists it through the validate-before-write path; the footer reflects the
// new label.
func TestCatalogsScheduleSelectorCycles(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	p := &CatalogsPanel{
		stateFile:   filepath.Join(dir, "state.json"),
		catalogDir:  dir,
		configPath:  cfgPath,
		indexMtimes: make(map[string]time.Time),
		scheduleIdx: 0, // start at "2h"
	}
	p.watchState = catalog.WatchState{Sources: make(map[string]catalog.SourceState)}

	_, cmd := p.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	if cmd == nil {
		t.Fatal("'i' should dispatch a schedule-persist command")
	}
	if p.scheduleIdx != 1 {
		t.Errorf("'i' should advance the schedule index, got %d", p.scheduleIdx)
	}
	// The persisted command is a success message (5h is a valid interval).
	if _, isErr := cmd().(policyEditErrMsg); isErr {
		t.Errorf("advancing to a valid cadence should not be rejected")
	}
	// Footer reflects the new label.
	if !strings.Contains(p.Footer(), catalogScheduleOptions[1].Label) {
		t.Errorf("footer should show the new cadence label %q: %q", catalogScheduleOptions[1].Label, p.Footer())
	}
	// Config was written.
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("schedule cycle should write the config: %v", err)
	}
}

// TestPersistScheduleRejectsBadConfigPath proves persistSchedule surfaces a
// rejection (policyEditErrMsg) when the config path is unavailable.
func TestPersistScheduleRejectsBadConfigPath(t *testing.T) {
	p := &CatalogsPanel{configPath: ""} // unavailable → persistCatalogSyncInterval errors
	cmd := p.persistSchedule("5h", true)
	if cmd == nil {
		t.Fatal("persistSchedule with no config path should emit a command")
	}
	if _, isErr := cmd().(policyEditErrMsg); !isErr {
		t.Fatalf("an unavailable config path should be rejected with policyEditErrMsg, got %T", cmd())
	}
}

// TestPersistScheduleOff proves the "off" cadence emits a saved message labelled
// off.
func TestPersistScheduleSuccessLabel(t *testing.T) {
	dir := t.TempDir()
	p := &CatalogsPanel{configPath: filepath.Join(dir, "config.json")}
	cmd := p.persistSchedule("", false) // "off"
	saved, ok := cmd().(policySavedMsg)
	if !ok {
		t.Fatalf("disabling sync should emit a policySavedMsg, got %T", cmd())
	}
	if !strings.Contains(saved.msg, "off") {
		t.Errorf("off cadence message = %q, want it to mention off", saved.msg)
	}
}

// TestCatalogsStateTickRefreshes proves a stateTick refreshes the body cache from
// the (missing) state file without panicking.
func TestCatalogsStateTickRefreshes(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.Update(stateTick{})
	if p.bodyCache == "" {
		t.Error("stateTick should rebuild the body cache")
	}
}

// TestCurrentScheduleIdx covers the config-driven cadence resolver: no path,
// disabled (off), and an explicit interval.
func TestCurrentScheduleIdx(t *testing.T) {
	// No config path → default midpoint index 1.
	noPath := &CatalogsPanel{}
	if noPath.currentScheduleIdx() != 1 {
		t.Errorf("no-path currentScheduleIdx = %d, want 1", noPath.currentScheduleIdx())
	}

	// Disabled sync → "off" (last index).
	dir := t.TempDir()
	offPath := filepath.Join(dir, "off.json")
	if err := os.WriteFile(offPath, []byte(`{"catalog_sync":{"enabled":false}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	off := &CatalogsPanel{configPath: offPath}
	if got := off.currentScheduleIdx(); got != len(catalogScheduleOptions)-1 {
		t.Errorf("disabled sync currentScheduleIdx = %d, want %d (off)", got, len(catalogScheduleOptions)-1)
	}

	// Enabled with a 5h interval → index of the "5h" option.
	onPath := filepath.Join(dir, "on.json")
	if err := os.WriteFile(onPath, []byte(`{"catalog_sync":{"enabled":true,"interval":"5h"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	on := &CatalogsPanel{configPath: onPath}
	want := -1
	for i, opt := range catalogScheduleOptions {
		if opt.Label == "5h" {
			want = i
		}
	}
	if got := on.currentScheduleIdx(); got != want {
		t.Errorf("5h interval currentScheduleIdx = %d, want %d", got, want)
	}
}

// TestNewCatalogsPanelHermetic covers the public constructor under an isolated
// home so it resolves real paths without touching the user's state dir.
func TestNewCatalogsPanelHermetic(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())
	p := NewCatalogsPanel()
	if p.catalogDir == "" || p.stateFile == "" {
		t.Error("constructor should resolve catalogDir + stateFile")
	}
	if p.Body(80, 24) == "" {
		t.Error("a freshly-constructed panel should render a body")
	}
}
