package tui

import (
	"strings"
	"testing"
	"time"

	catalog "github.com/mzansi-agentive/beekeeper/internal/catalog"
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
	c := pipColor(p.indexMtimes["bumblebee"], false)
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
	color := pipColor(p.indexMtimes["bumblebee"], false)
	if color != colorAmber {
		t.Errorf("expected colorAmber for 3h stale source, got %v", color)
	}
}

func TestCatalogsPanelStale24h(t *testing.T) {
	p := newCatalogsPanelForTest()
	p.indexMtimes["bumblebee"] = time.Now().Add(-25 * time.Hour)
	color := pipColor(p.indexMtimes["bumblebee"], false)
	if color != colorRed {
		t.Errorf("expected colorRed for 25h stale source, got %v", color)
	}
}

func TestCatalogsPanelDegraded(t *testing.T) {
	p := newCatalogsPanelForTest()
	// Even with recent mtime, degraded=true → colorAmber
	p.indexMtimes["osv"] = time.Now().Add(-10 * time.Minute)
	color := pipColor(p.indexMtimes["osv"], true)
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
