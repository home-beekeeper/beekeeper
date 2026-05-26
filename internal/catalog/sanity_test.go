package catalog

import (
	"strings"
	"testing"
)

func TestSanityWithinBounds(t *testing.T) {
	cfg := DefaultSanityConfig()
	result := CheckSanity(1000, 1500, cfg) // delta=500 — within alert threshold
	if result.Alert {
		t.Errorf("expected Alert=false for delta 500, got Alert=true (reason: %q)", result.Reason)
	}
	if result.Block {
		t.Errorf("expected Block=false for delta 500, got Block=true (reason: %q)", result.Reason)
	}
	if result.Reason != "" {
		t.Errorf("expected empty Reason for within-bounds delta, got %q", result.Reason)
	}
}

func TestSanityAlertDelta(t *testing.T) {
	cfg := DefaultSanityConfig()
	// delta = 1500: above alert threshold (1000) but below block threshold (10000)
	result := CheckSanity(1000, 2500, cfg)
	if !result.Alert {
		t.Errorf("expected Alert=true for delta 1500, got Alert=false")
	}
	if result.Block {
		t.Errorf("expected Block=false for delta 1500, got Block=true")
	}
	if result.Reason == "" {
		t.Errorf("expected non-empty Reason for alert, got empty string")
	}
}

func TestSanityAlertDeltaNegative(t *testing.T) {
	cfg := DefaultSanityConfig()
	// delta = -1500 (shrink): abs value still triggers alert
	result := CheckSanity(5000, 3500, cfg)
	if !result.Alert {
		t.Errorf("expected Alert=true for delta -1500 (shrink), got Alert=false")
	}
	if result.Block {
		t.Errorf("expected Block=false for delta -1500 (shrink), got Block=true")
	}
}

func TestSanityBlockDelta(t *testing.T) {
	cfg := DefaultSanityConfig()
	// delta = 12000: above hard-block threshold (10000)
	result := CheckSanity(1000, 13000, cfg)
	if !result.Block {
		t.Errorf("expected Block=true for delta 12000, got Block=false")
	}
	if result.Alert {
		t.Errorf("expected Alert=false when Block=true (Block takes priority), got Alert=true")
	}
	if result.Reason == "" {
		t.Errorf("expected non-empty Reason for block, got empty string")
	}
	if !strings.Contains(result.Reason, "12000") {
		t.Errorf("expected Reason to contain the delta count 12000, got %q", result.Reason)
	}
}

func TestSanityTotalAlert(t *testing.T) {
	cfg := DefaultSanityConfig()
	// delta = 500 (within bounds) but total = 150000 (above AlertTotalEntries=100000)
	result := CheckSanity(149500, 150000, cfg)
	if !result.Alert {
		t.Errorf("expected Alert=true for total 150000 > AlertTotalEntries 100000, got Alert=false")
	}
	if result.Block {
		t.Errorf("expected Block=false for total-only alert, got Block=true")
	}
	if result.Reason == "" {
		t.Errorf("expected non-empty Reason for total alert, got empty string")
	}
}

func TestSanityBlockPriorityOverTotal(t *testing.T) {
	cfg := DefaultSanityConfig()
	// delta exceeds block AND total exceeds alert — block takes priority
	result := CheckSanity(0, 150000, cfg) // delta=150000, total=150000
	if !result.Block {
		t.Errorf("expected Block=true (takes priority over total alert), got Block=false")
	}
}

func TestSanityZeroToZero(t *testing.T) {
	cfg := DefaultSanityConfig()
	result := CheckSanity(0, 0, cfg)
	if result.Alert || result.Block {
		t.Errorf("expected no alert/block for 0→0, got Alert=%v Block=%v", result.Alert, result.Block)
	}
}

func TestSanityCustomConfig(t *testing.T) {
	cfg := SanityConfig{
		AlertDeltaEntries: 10,
		BlockDeltaEntries: 100,
		AlertTotalEntries: 500,
	}
	// Test custom alert threshold (delta=20 > 10)
	result := CheckSanity(0, 20, cfg)
	if !result.Alert {
		t.Errorf("expected Alert=true with custom AlertDeltaEntries=10 and delta=20")
	}
	// Test custom block threshold (delta=200 > 100)
	result = CheckSanity(0, 200, cfg)
	if !result.Block {
		t.Errorf("expected Block=true with custom BlockDeltaEntries=100 and delta=200")
	}
}

func TestDefaultSanityConfig(t *testing.T) {
	cfg := DefaultSanityConfig()
	if cfg.AlertDeltaEntries != 1000 {
		t.Errorf("AlertDeltaEntries: want 1000, got %d", cfg.AlertDeltaEntries)
	}
	if cfg.BlockDeltaEntries != 10000 {
		t.Errorf("BlockDeltaEntries: want 10000, got %d", cfg.BlockDeltaEntries)
	}
	if cfg.AlertTotalEntries != 100000 {
		t.Errorf("AlertTotalEntries: want 100000, got %d", cfg.AlertTotalEntries)
	}
	if cfg.AlertVersionsPerPkg != 1000 {
		t.Errorf("AlertVersionsPerPkg: want 1000, got %d", cfg.AlertVersionsPerPkg)
	}
}
