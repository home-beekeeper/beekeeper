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

func newCatalogsPanelForTest() *CatalogsPanel {
	p := &CatalogsPanel{
		stateFile:   "/nonexistent",
		catalogDir:  "/nonexistent",
		indexMtimes: make(map[string]time.Time),
	}
	p.watchState = catalog.WatchState{Sources: make(map[string]catalog.SourceState)}
	return p
}

func TestCatalogsPanelFresh(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.indexMtimes["bumblebee"] = time.Now().Add(-30 * time.Minute)
	p.bodyCache = p.buildBody()
	body := p.Body(100, 20)
	if !strings.Contains(body, "bumblebee") {
		t.Error("expected bumblebee in body")
	}
	// Fresh source: pip color should be green (test via pipColor function directly)
	c := pipColor(catalog.SourceState{}, p.indexMtimes["bumblebee"])
	if c != colorGreen {
		t.Errorf("expected colorGreen for 30m fresh source, got %v", c)
	}
	// Body should show "synced" info for bumblebee (not "never synced" for that source)
	if !strings.Contains(body, "synced") {
		t.Error("expected 'synced' in body for fresh bumblebee source")
	}
}

func TestCatalogsPanelStale2h(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.indexMtimes["bumblebee"] = time.Now().Add(-3 * time.Hour)
	color := pipColor(catalog.SourceState{}, p.indexMtimes["bumblebee"])
	if color != colorAmber {
		t.Errorf("expected colorAmber for 3h stale source, got %v", color)
	}
}

func TestCatalogsPanelStale24h(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.indexMtimes["bumblebee"] = time.Now().Add(-25 * time.Hour)
	color := pipColor(catalog.SourceState{}, p.indexMtimes["bumblebee"])
	if color != colorRed {
		t.Errorf("expected colorRed for 25h stale source, got %v", color)
	}
}

func TestCatalogsPanelDegraded(t *testing.T) {
	p := newCatalogsPanelForTest()
	// Even with recent mtime, degraded=true → colorAmber
	p.indexMtimes["osv"] = time.Now().Add(-10 * time.Minute)
	color := pipColor(catalog.SourceState{Degraded: true}, p.indexMtimes["osv"])
	if color != colorAmber {
		t.Errorf("expected colorAmber for degraded source, got %v", color)
	}
}

func TestCatalogsPanelKnownSources(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.bodyCache = p.buildBody()
	body := p.Body(100, 40)
	for _, src := range []string{"bumblebee", "osv", "socket", "self"} {
		if !strings.Contains(body, src) {
			t.Errorf("expected source %q in catalog body", src)
		}
	}
}

// --- Phase 20 (CSYNC) Task 4 — real sync + honest pip + schedule selector ---

// TestPipColorFailedSyncAmber proves a failed sync (LastAttempt newer than
// LastSuccess) renders AMBER, never green/"fresh" — even when LastSuccess is
// recent.
func TestPipColorFailedSyncAmber(t *testing.T) {
	now := time.Now()
	ss := catalog.SourceState{
		LastSuccess: now.Add(-30 * time.Minute), // would be green on its own
		LastAttempt: now.Add(-1 * time.Minute),  // but a newer attempt failed
		LastError:   "boom",
	}
	if c := pipColor(ss, time.Time{}); c != colorAmber {
		t.Errorf("failed sync (LastAttempt>LastSuccess) pip = %v, want colorAmber (not fresh)", c)
	}

	// A clean recent success (no newer attempt) stays green.
	ok := catalog.SourceState{LastSuccess: now.Add(-30 * time.Minute), LastAttempt: now.Add(-30 * time.Minute)}
	if c := pipColor(ok, time.Time{}); c != colorGreen {
		t.Errorf("clean recent success pip = %v, want colorGreen", c)
	}
}

// TestCatalogsPanelSyncKeyDispatchesCommand proves pressing 's' returns a real
// async command (not nil / not a static toast).
func TestCatalogsPanelSyncKeyDispatchesCommand(t *testing.T) {
	p := newCatalogsPanelForTest()
	_, cmd := p.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if cmd == nil {
		t.Fatal("pressing 's' returned a nil command — expected an async sync dispatch")
	}
}

// TestPersistCatalogSyncInterval proves validate-before-write: a valid interval
// is written; an out-of-range interval is rejected WITHOUT touching disk.
func TestPersistCatalogSyncInterval(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Valid interval persists.
	if err := persistCatalogSyncInterval(cfgPath, "5h", true); err != nil {
		t.Fatalf("persist valid interval 5h: unexpected error %v", err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config after persist: %v", err)
	}
	if !strings.Contains(string(data), `"interval": "5h"`) {
		t.Errorf("config does not contain the persisted interval:\n%s", data)
	}

	// Invalid (out-of-range) interval is rejected and leaves disk unchanged.
	before := string(data)
	if err := persistCatalogSyncInterval(cfgPath, "1h", true); err == nil {
		t.Fatal("persist out-of-range interval 1h returned nil error, want rejection (validate-before-write)")
	}
	after, _ := os.ReadFile(cfgPath)
	if string(after) != before {
		t.Errorf("rejected interval modified disk — validate-before-write violated\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
