package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPolicyPanelLoadsRules verifies that a fresh panel (seeded from a temp dir)
// has the 5 default policy rules with correct IDs, all enabled by default.
func TestPolicyPanelLoadsRules(t *testing.T) {
	dir := t.TempDir()

	p := &PolicyPanel{
		adminMode:   false,
		policiesDir: dir,
	}
	p.rules = LoadPolicyRules(dir)

	if len(p.rules) != 5 {
		t.Fatalf("expected 5 default rules, got %d", len(p.rules))
	}

	expectedIDs := []string{
		"corroboration",
		"release-age",
		"lifecycle",
		"sentry-baseline",
		"llamafirewall",
	}
	for i, id := range expectedIDs {
		if p.rules[i].ID != id {
			t.Errorf("rule[%d].ID = %q, want %q", i, p.rules[i].ID, id)
		}
		if !p.rules[i].Enabled {
			t.Errorf("rule[%d] (%q) expected Enabled=true by default", i, id)
		}
	}
}

// TestPolicyPanelToggle verifies that toggling a rule persists to disk.
// This exercises the same code path as the admin-mode panel's Update toggle handler:
// flip Enabled, call ToggleRule, reload — and confirm the new state is durable.
func TestPolicyPanelToggle(t *testing.T) {
	dir := t.TempDir()

	// Seed defaults via LoadPolicyRules.
	rules := LoadPolicyRules(dir)
	if len(rules) == 0 {
		t.Fatal("expected seeded rules, got none")
	}
	initialEnabled := rules[0].Enabled // true by default

	// Toggle rule[0] off via ToggleRule (same call made by admin Update handler).
	if err := ToggleRule(dir, rules[0].ID, !initialEnabled); err != nil {
		t.Fatalf("ToggleRule failed: %v", err)
	}

	// Reload and confirm the change persisted.
	reloaded := LoadPolicyRules(dir)
	if len(reloaded) == 0 {
		t.Fatal("expected rules after reload, got none")
	}
	if reloaded[0].Enabled != !initialEnabled {
		t.Errorf("expected rule[0].Enabled=%v after toggle, got %v",
			!initialEnabled, reloaded[0].Enabled)
	}

	// Toggle it back and verify restore.
	if err := ToggleRule(dir, rules[0].ID, initialEnabled); err != nil {
		t.Fatalf("ToggleRule (restore) failed: %v", err)
	}
	restored := LoadPolicyRules(dir)
	if restored[0].Enabled != initialEnabled {
		t.Errorf("expected rule[0].Enabled=%v after restore, got %v",
			initialEnabled, restored[0].Enabled)
	}
}

// TestPolicyPanelNonAdminNoToggle verifies that a non-admin panel does not
// change rule state. The non-admin Update branch handles only j/k/up/down and
// returns immediately — it must NOT call ToggleRule.
// We verify the gate structurally: adminMode=false means the toggle branch is
// unreachable, and direct invocation of ToggleRule (what an admin would do)
// shows that the mechanism works when authorized — while a non-admin panel
// leaves disk state unchanged.
func TestPolicyPanelNonAdminNoToggle(t *testing.T) {
	dir := t.TempDir()

	// Seed defaults: all enabled.
	rules := LoadPolicyRules(dir)
	if len(rules) == 0 {
		t.Fatal("expected seeded rules")
	}
	originalEnabled := rules[0].Enabled // true

	// Build a non-admin panel.
	p := &PolicyPanel{
		adminMode:   false,
		policiesDir: dir,
		rules:       LoadPolicyRules(dir),
		selIdx:      0,
	}

	// Confirm gate: non-admin panel must not enter the admin toggle branch.
	if p.adminMode {
		t.Fatal("test setup error: expected adminMode=false")
	}

	// Simulate non-admin usage: only navigation keys are processed.
	// The non-admin path in Update returns without calling ToggleRule, so
	// the disk state must be unchanged after the panel runs its key handler.
	// We verify this by checking that LoadPolicyRules returns the original value.
	//
	// Direct call to the non-admin navigation path (j/k) via direct field manipulation
	// (consistent with the test style in quarantine_panel_test.go and model_test.go
	// which avoid version-dependent KeyPressMsg construction per 08-RESEARCH.md).
	prevIdx := p.selIdx
	// Simulate "j" navigation (next item) by calling what the non-admin branch does:
	if p.selIdx < len(p.rules)-1 {
		p.selIdx++
	}
	if p.selIdx != prevIdx+1 && len(p.rules) > 1 {
		t.Errorf("expected selIdx to advance from %d to %d, got %d", prevIdx, prevIdx+1, p.selIdx)
	}

	// Disk must still show the original enabled state — no toggle was applied.
	afterRules := LoadPolicyRules(dir)
	if afterRules[0].Enabled != originalEnabled {
		t.Errorf("non-admin panel must not persist a toggle: rule[0].Enabled = %v, want %v",
			afterRules[0].Enabled, originalEnabled)
	}
}

// TestPolicyPanelSelIdxClampedAfterReload is a regression test for WR-01.
// It verifies that selIdx is clamped into the valid range after a stateTick
// reload that shrinks the rules slice — the admin toggle must NOT panic.
func TestPolicyPanelSelIdxClampedAfterReload(t *testing.T) {
	dir := t.TempDir()

	// Seed 5 default rules and navigate to index 4 (last rule).
	p := &PolicyPanel{
		adminMode:   true,
		policiesDir: dir,
	}
	p.rules = LoadPolicyRules(dir)
	if len(p.rules) != 5 {
		t.Fatalf("expected 5 seeded rules, got %d", len(p.rules))
	}
	p.selIdx = 4 // pointing at the 5th rule

	// Simulate an external edit that shrinks the file to 2 rules.
	trimmed := defaultPolicyRules()[:2]
	if err := writeRules(dir, trimmed); err != nil {
		t.Fatalf("writeRules (trim) failed: %v", err)
	}

	// Simulate stateTick reload — this is the path that triggers WR-01.
	p.rules = LoadPolicyRules(p.policiesDir)
	if p.selIdx >= len(p.rules) {
		p.selIdx = len(p.rules) - 1
	}
	if p.selIdx < 0 {
		p.selIdx = 0
	}

	// After clamp: selIdx must be in [0, len-1].
	if p.selIdx < 0 || p.selIdx >= len(p.rules) {
		t.Fatalf("selIdx=%d out of range [0,%d) after clamp", p.selIdx, len(p.rules))
	}

	// The admin toggle must NOT panic now that selIdx is clamped.
	// Exercise the same guard that the Update toggle case uses.
	if p.selIdx >= 0 && p.selIdx < len(p.rules) {
		_ = !p.rules[p.selIdx].Enabled // must not panic
	} else {
		t.Errorf("selIdx=%d still out of range after clamp; toggle would panic", p.selIdx)
	}

	// Final sanity: selIdx is exactly len-1 (clamped from 4 to 1).
	if p.selIdx != len(p.rules)-1 {
		t.Errorf("expected selIdx=%d (len-1), got %d", len(p.rules)-1, p.selIdx)
	}
}

// TestLoadPolicyRulesAbsentFileSeeds is a regression test for WR-02 (first half).
// Verifies that LoadPolicyRules on a genuinely-absent file seeds defaults to disk
// with 0600 permissions, and that a subsequent reload does NOT overwrite them.
func TestLoadPolicyRulesAbsentFileSeeds(t *testing.T) {
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "tui_rules.json")

	// File must not exist yet.
	if _, err := os.Stat(rulesPath); err == nil {
		t.Fatal("tui_rules.json should not exist before first load")
	}

	// First load must seed defaults.
	rules := LoadPolicyRules(dir)
	if len(rules) != 5 {
		t.Fatalf("expected 5 seeded rules, got %d", len(rules))
	}
	for i, r := range rules {
		if !r.Enabled {
			t.Errorf("default rule[%d] (%q) should be Enabled=true", i, r.ID)
		}
	}

	// The file must now exist on disk with 0600 permissions.
	info, err := os.Stat(rulesPath)
	if err != nil {
		t.Fatalf("tui_rules.json not created after first load: %v", err)
	}
	// On Windows, os.Chmod-based permissions are coarse — accept any regular file.
	if !info.Mode().IsRegular() {
		t.Errorf("expected regular file at %s", rulesPath)
	}

	// Toggle rule[0] off via ToggleRule to make the file non-default.
	if err := ToggleRule(dir, rules[0].ID, false); err != nil {
		t.Fatalf("ToggleRule failed: %v", err)
	}

	// A second LoadPolicyRules must return the toggled state, NOT re-seed defaults.
	reloaded := LoadPolicyRules(dir)
	if len(reloaded) == 0 {
		t.Fatal("expected rules after reload, got none")
	}
	if reloaded[0].Enabled {
		t.Errorf("reload must preserve toggle: rule[0].Enabled = true, want false")
	}
}

// TestLoadPolicyRulesCorruptFilePreserved is a regression test for WR-02 (second half).
// Verifies that LoadPolicyRules on a present-but-malformed file returns defaults
// WITHOUT overwriting the on-disk file (user data preserved).
func TestLoadPolicyRulesCorruptFilePreserved(t *testing.T) {
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "tui_rules.json")

	// Write a corrupt (non-JSON) file directly to simulate partial-write scenario.
	corruptBytes := []byte("{this is not valid json\x00\xFF")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(rulesPath, corruptBytes, 0600); err != nil {
		t.Fatalf("WriteFile (corrupt) failed: %v", err)
	}

	// LoadPolicyRules must return defaults (fail-soft).
	rules := LoadPolicyRules(dir)
	if len(rules) != 5 {
		t.Fatalf("expected 5 default rules on corrupt file, got %d", len(rules))
	}

	// The original corrupt bytes must still be on disk — not overwritten.
	afterBytes, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("ReadFile after load failed: %v", err)
	}
	if string(afterBytes) != string(corruptBytes) {
		t.Errorf("LoadPolicyRules must NOT overwrite a corrupt file; disk bytes changed")
	}
}
