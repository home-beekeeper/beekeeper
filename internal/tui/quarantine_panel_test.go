package tui

import (
	"strings"
	"testing"

	quarantine "github.com/home-beekeeper/beekeeper/internal/quarantine"
)

func TestQuarantinePanelEmpty(t *testing.T) {
	p := &QuarantinePanel{
		adminMode:     false,
		quarantineDir: "/nonexistent",
		items:         []quarantine.Manifest{},
	}
	body := p.Body(100, 20)
	if !strings.Contains(body, "quarantine empty") {
		t.Errorf("expected 'quarantine empty' in body, got: %q", body)
	}
}

func TestQuarantinePanelPurgeConfirm(t *testing.T) {
	p := &QuarantinePanel{
		adminMode:    true,
		items:        []quarantine.Manifest{{ID: "test-001", Name: "angular-console", Publisher: "nrwl", Version: "18.95.0"}},
		confirmPurge: true,
	}
	body := p.Body(100, 20)
	if !strings.Contains(body, "Purge ALL") {
		t.Errorf("expected 'Purge ALL' in body, got: %q", body)
	}
	if !strings.Contains(body, "[y/N]") {
		t.Errorf("expected '[y/N]' in body, got: %q", body)
	}
}

func TestQuarantinePanelHeld(t *testing.T) {
	p := &QuarantinePanel{
		adminMode: true,
		items: []quarantine.Manifest{
			{ID: "test-001", Publisher: "nrwl", Name: "angular-console", Version: "18.95.0"},
		},
		confirmPurge: false,
	}
	body := p.Body(100, 20)
	if !strings.Contains(body, "HELD") {
		t.Errorf("expected 'HELD' badge in body, got: %q", body)
	}
}
