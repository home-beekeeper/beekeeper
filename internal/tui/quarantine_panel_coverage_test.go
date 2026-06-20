package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	quarantine "github.com/home-beekeeper/beekeeper/internal/quarantine"
)

// seedQuarantine creates a quarantine dir under a temp root and quarantines one
// editor extension so the panel has a real item to load/restore/purge.
func seedQuarantine(t *testing.T) (qDir string, srcRoot string) {
	t.Helper()
	root := t.TempDir()
	qDir = filepath.Join(root, "quarantine")
	srcRoot = filepath.Join(root, "src")
	extPath := filepath.Join(srcRoot, "acme.evil-linter")
	if err := os.MkdirAll(extPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extPath, "extension.js"), []byte("// evil"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := quarantine.Manifest{
		ArtifactType: quarantine.ArtifactTypeEditorExtension,
		Publisher:    "acme",
		Name:         "evil-linter",
		Version:      "1.0.0",
		Reason:       "catalog corroborated block",
	}
	if _, err := quarantine.MoveTyped(qDir, extPath, m); err != nil {
		t.Fatalf("MoveTyped seed: %v", err)
	}
	return qDir, srcRoot
}

// TestQuarantineLoadItemsRealDir proves loadItems reads a real quarantine dir and
// that the panel's Count/Body reflect the held item.
func TestQuarantineLoadItemsRealDir(t *testing.T) {
	qDir, _ := seedQuarantine(t)
	p := &QuarantinePanel{quarantineDir: qDir, adminMode: true}
	p.loadItems()
	if len(p.items) != 1 {
		t.Fatalf("loadItems should find 1 held item, got %d", len(p.items))
	}
	if !strings.Contains(p.Count(), "1 item held") {
		t.Errorf("Count = %q, want '1 item held'", p.Count())
	}
	body := p.Body(100, 20)
	if !strings.Contains(body, "evil-linter") {
		t.Errorf("Body should list the held item, got: %q", body)
	}
}

// TestQuarantineCountEmpty covers the empty Count branch.
func TestQuarantineCountEmpty(t *testing.T) {
	p := &QuarantinePanel{quarantineDir: t.TempDir()}
	p.loadItems()
	if p.Count() != "empty" {
		t.Errorf("empty Count = %q, want 'empty'", p.Count())
	}
}

// TestQuarantineContract covers the small PanelContent surface.
func TestQuarantineContract(t *testing.T) {
	admin := &QuarantinePanel{adminMode: true}
	if admin.Title() != "Quarantine" {
		t.Errorf("Title = %q", admin.Title())
	}
	if admin.Padded() {
		t.Error("quarantine panel is not padded")
	}
	if admin.Critical() {
		t.Error("quarantine panel is not critical")
	}
	if !strings.Contains(admin.Footer(), "restore") {
		t.Errorf("admin Footer should mention restore: %q", admin.Footer())
	}
	nonAdmin := &QuarantinePanel{adminMode: false}
	if strings.Contains(nonAdmin.Footer(), "restore") {
		t.Errorf("non-admin Footer must not expose restore: %q", nonAdmin.Footer())
	}
	if !strings.Contains(nonAdmin.Footer(), "details") {
		t.Errorf("non-admin Footer should mention details: %q", nonAdmin.Footer())
	}
}

// TestQuarantineNavigation proves j/k move and clamp for both admin and
// non-admin panels.
func TestQuarantineNavigation(t *testing.T) {
	mkItems := func() []quarantine.Manifest {
		return []quarantine.Manifest{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	}
	for _, admin := range []bool{true, false} {
		p := &QuarantinePanel{adminMode: admin, items: mkItems()}
		p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		if p.selIdx != 1 {
			t.Errorf("admin=%v down should move to 1, got %d", admin, p.selIdx)
		}
		p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"}) // clamp at 2
		if p.selIdx != 2 {
			t.Errorf("admin=%v down should clamp at 2, got %d", admin, p.selIdx)
		}
		p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		if p.selIdx != 1 {
			t.Errorf("admin=%v up should move to 1, got %d", admin, p.selIdx)
		}
	}
}

// TestQuarantineRestoreCommand proves pressing 'r' as admin dispatches a real
// restore command that, when run, restores the artifact and removes it from the
// quarantine dir.
func TestQuarantineRestoreCommand(t *testing.T) {
	qDir, _ := seedQuarantine(t)
	p := &QuarantinePanel{quarantineDir: qDir, adminMode: true}
	p.loadItems()
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("'r' as admin with items should dispatch a restore command")
	}
	msg := cmd()
	qa, ok := msg.(QuarantineActionMsg)
	if !ok || qa.Kind != "restore" {
		t.Fatalf("restore cmd produced %#v, want a restore QuarantineActionMsg", msg)
	}
	if qa.Err != nil {
		t.Fatalf("restore failed: %v", qa.Err)
	}
	// The item is gone from quarantine after restore.
	items, _ := quarantine.List(qDir)
	if len(items) != 0 {
		t.Errorf("after restore, quarantine should be empty, got %d", len(items))
	}
}

// TestQuarantinePurgeConfirmFlow drives the destructive purge confirmation: 'p'
// arms the confirm prompt, then 'y' dispatches a real purge.
func TestQuarantinePurgeConfirmFlow(t *testing.T) {
	qDir, _ := seedQuarantine(t)
	p := &QuarantinePanel{quarantineDir: qDir, adminMode: true}
	p.loadItems()

	// 'p' arms the confirmation (no command yet).
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if cmd != nil || !p.confirmPurge {
		t.Fatalf("'p' should arm confirmPurge without dispatching (cmd=%v, confirm=%v)", cmd, p.confirmPurge)
	}
	if !strings.Contains(p.Body(100, 20), "Purge ALL") {
		t.Error("confirm body should show the purge prompt")
	}

	// 'y' confirms and dispatches the real purge.
	_, cmd = p.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("'y' should dispatch the purge command")
	}
	if p.confirmPurge {
		t.Error("'y' should clear confirmPurge")
	}
	qa, ok := cmd().(QuarantineActionMsg)
	if !ok || qa.Kind != "purge" {
		t.Fatalf("purge cmd produced %#v, want a purge QuarantineActionMsg", cmd())
	}
	items, _ := quarantine.List(qDir)
	if len(items) != 0 {
		t.Errorf("after purge, quarantine should be empty, got %d", len(items))
	}
}

// TestQuarantinePurgeCancel proves any non-y key cancels the purge confirmation.
func TestQuarantinePurgeCancel(t *testing.T) {
	p := &QuarantinePanel{adminMode: true, items: []quarantine.Manifest{{ID: "a"}}, confirmPurge: true}
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if cmd != nil {
		t.Error("cancel should not dispatch a command")
	}
	if p.confirmPurge {
		t.Error("a non-y key should cancel the purge confirmation")
	}
}

// TestQuarantineNonAdminCannotRestore proves r/p are no-ops without --admin.
func TestQuarantineNonAdminCannotRestore(t *testing.T) {
	p := &QuarantinePanel{adminMode: false, items: []quarantine.Manifest{{ID: "a"}}}
	if _, cmd := p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}); cmd != nil {
		t.Error("non-admin 'r' must be a no-op")
	}
	if _, cmd := p.Update(tea.KeyPressMsg{Code: 'p', Text: "p"}); cmd != nil {
		t.Error("non-admin 'p' must be a no-op")
	}
	if p.confirmPurge {
		t.Error("non-admin 'p' must not arm the purge confirmation")
	}
}

// TestQuarantineStateTickReloads proves a stateTick triggers loadItems (picks up
// a newly-quarantined artifact).
func TestQuarantineStateTickReloads(t *testing.T) {
	qDir, _ := seedQuarantine(t)
	p := &QuarantinePanel{quarantineDir: qDir, adminMode: true}
	if len(p.items) != 0 {
		t.Fatal("panel should start empty before any load")
	}
	p.Update(stateTick{})
	if len(p.items) != 1 {
		t.Errorf("stateTick should reload items, got %d", len(p.items))
	}
}

// TestQuarantineActionMsgReloads proves a completed action message reloads items.
func TestQuarantineActionMsgReloads(t *testing.T) {
	qDir, _ := seedQuarantine(t)
	p := &QuarantinePanel{quarantineDir: qDir, adminMode: true}
	p.Update(QuarantineActionMsg{Kind: "restore", Target: "x"})
	if len(p.items) != 1 {
		t.Errorf("action msg should reload items, got %d", len(p.items))
	}
}

// TestNewQuarantinePanelHermetic covers the public constructor under an isolated
// BEEKEEPER_HOME so it never touches the real state dir.
func TestNewQuarantinePanelHermetic(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())
	p := NewQuarantinePanel(true)
	if !p.adminMode {
		t.Error("adminMode not propagated")
	}
	if p.quarantineDir == "" {
		t.Error("quarantineDir should be resolved from the state dir")
	}
}
