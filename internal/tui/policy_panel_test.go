package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/policy"
	policyloader "github.com/home-beekeeper/beekeeper/internal/policyloader"
)

// newTestPolicyPanel builds a panel bound to a temp policies dir (bypassing
// platform.StateDir) and seeds the managed file, mirroring NewPolicyPanel.
func newTestPolicyPanel(t *testing.T, admin bool) (*PolicyPanel, string) {
	t.Helper()
	dir := t.TempDir()
	p := &PolicyPanel{adminMode: admin, policiesDir: dir, editMode: editView}
	p.reload()
	return p, dir
}

// effectiveThresholds loads the managed dir the way beekeeper check does and
// returns the enforced corroboration thresholds.
func effectiveThresholds(t *testing.T, dir string) policy.CorroborationThresholds {
	t.Helper()
	files, err := policyloader.LoadPolicyDir(dir)
	if err != nil {
		t.Fatalf("LoadPolicyDir: %v", err)
	}
	return policyloader.ThresholdsFromPolicyFiles(files)
}

// TestPolicyPanelSeedsManagedFile verifies a fresh panel seeds the real,
// enforceable managed policy file and builds the corroboration rows.
func TestPolicyPanelSeedsManagedFile(t *testing.T) {
	p, dir := newTestPolicyPanel(t, false)

	if _, err := os.Stat(policyloader.ManagedPolicyPath(dir)); err != nil {
		t.Fatalf("managed policy file must be seeded on first load: %v", err)
	}
	// The seeded file must be a VALID, enforceable policy file (not the foreign
	// tui_rules.json schema the engine skipped).
	files, lerr := policyloader.LoadPolicyDir(dir)
	if lerr != nil || len(files) != 1 {
		t.Fatalf("seeded file must load as exactly 1 valid policy file, got %d (err %v)", len(files), lerr)
	}
	// 4 corroboration rows + addAllow + addSens + 2 info rows.
	if len(p.rows) < 6 || p.rows[0].corrField != "warn" || p.rows[1].corrField != "block" {
		t.Fatalf("unexpected row model: %+v", p.rows)
	}
	if got := effectiveThresholds(t, dir); got.WarnAt != 1 || got.BlockAt != 2 || got.QuarantineAt != 3 {
		t.Errorf("seeded thresholds = %+v, want warn 1/block 2/quar 3", got)
	}
}

// TestPolicyPanelAdjustThresholdEnforced is the end-to-end proof for numeric
// edits: a +/- in the TUI changes the file beekeeper check actually enforces.
func TestPolicyPanelAdjustThresholdEnforced(t *testing.T) {
	p, dir := newTestPolicyPanel(t, true)

	// Select the "block at" row and decrement 2 -> 1.
	p.selIdx = 1
	if p.rows[p.selIdx].corrField != "block" {
		t.Fatalf("selIdx 1 is not the block row: %+v", p.rows[p.selIdx])
	}
	if cmd := p.handleKey("-"); cmd != nil {
		// success path is silent (nil); a non-nil cmd here would be an error toast.
		if _, isErr := cmd().(policyEditErrMsg); isErr {
			t.Fatalf("valid block-at decrement was rejected")
		}
	}

	// On-disk file is valid AND the enforced threshold reflects the edit.
	if _, errs := policyloader.LoadPolicyFile(policyloader.ManagedPolicyPath(dir)); len(errs) > 0 {
		t.Fatalf("managed file invalid after edit: %v", errs)
	}
	if got := effectiveThresholds(t, dir); got.BlockAt != 1 {
		t.Errorf("enforced block_at = %d after TUI edit, want 1", got.BlockAt)
	}
}

// TestPolicyPanelAddAllowlistOverridesBlock is the end-to-end proof for list
// edits: adding an allowlist entry in the TUI makes beekeeper check's overlay
// downgrade a catalog block to allow for that package.
func TestPolicyPanelAddAllowlistOverridesBlock(t *testing.T) {
	p, dir := newTestPolicyPanel(t, true)

	// Navigate to the "+ add allowlist entry" row and drive the text-input flow.
	addIdx := -1
	for i, r := range p.rows {
		if r.kind == rowAddAllow {
			addIdx = i
			break
		}
	}
	if addIdx < 0 {
		t.Fatal("no add-allowlist row found")
	}
	p.selIdx = addIdx
	p.handleKey("a") // enter add mode
	if p.editMode != editAddAllow {
		t.Fatalf("expected editAddAllow, got %v", p.editMode)
	}
	for _, ch := range "npm:evil-pkg" {
		p.handleKey(string(ch))
	}
	if p.buf != "npm:evil-pkg" {
		t.Fatalf("text capture wrong: %q", p.buf)
	}
	if cmd := p.handleKey("enter"); cmd != nil {
		if _, isErr := cmd().(policyEditErrMsg); isErr {
			t.Fatalf("valid allowlist add was rejected")
		}
	}

	// Enforcement: a catalog-block base for `npm install evil-pkg` is overridden.
	files, err := policyloader.LoadPolicyDir(dir)
	if err != nil {
		t.Fatalf("LoadPolicyDir: %v", err)
	}
	tc := policy.ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install evil-pkg"},
	}
	base := policy.Decision{Allow: false, Level: "block", Reason: "catalog corroborated block"}
	dec := policyloader.ApplyPolicyOverlay(files, tc, base)
	if !dec.Allow || dec.Level != "allow" {
		t.Errorf("TUI allowlist entry did not override the block: got Allow=%v Level=%q", dec.Allow, dec.Level)
	}
}

// TestPolicyPanelLastGateRejectsInvalid proves an invalid edit is rejected in
// the TUI and the on-disk file is left valid and unchanged.
func TestPolicyPanelLastGateRejectsInvalid(t *testing.T) {
	p, dir := newTestPolicyPanel(t, true)
	p.selIdx = 0 // warn at

	// warn 1 -> 2 is valid (warn <= block).
	if cmd := p.handleKey("+"); cmd != nil {
		if _, isErr := cmd().(policyEditErrMsg); isErr {
			t.Fatal("warn 1->2 should be valid")
		}
	}
	if got := effectiveThresholds(t, dir); got.WarnAt != 2 {
		t.Fatalf("warn_at = %d, want 2 after first increment", got.WarnAt)
	}

	// warn 2 -> 3 would make warn > block (2): must be REJECTED.
	cmd := p.handleKey("+")
	if cmd == nil {
		t.Fatal("warn 2->3 (warn > block) must be rejected with an error cmd")
	}
	if _, isErr := cmd().(policyEditErrMsg); !isErr {
		t.Fatalf("expected policyEditErrMsg, got %T", cmd())
	}
	// Disk unchanged: still warn 2, still a valid file.
	if got := effectiveThresholds(t, dir); got.WarnAt != 2 {
		t.Errorf("rejected edit changed disk: warn_at = %d, want 2", got.WarnAt)
	}
	if _, errs := policyloader.LoadPolicyFile(policyloader.ManagedPolicyPath(dir)); len(errs) > 0 {
		t.Errorf("managed file must remain valid after a rejected edit: %v", errs)
	}
}

// TestPolicyPanelDeleteEntry verifies a sensitive-path entry can be added then
// removed, and the change persists.
func TestPolicyPanelDeleteEntry(t *testing.T) {
	p, dir := newTestPolicyPanel(t, true)

	// Add a sensitive path.
	for i, r := range p.rows {
		if r.kind == rowAddSens {
			p.selIdx = i
			break
		}
	}
	p.handleKey("a")
	for _, ch := range "**/.env" {
		p.handleKey(string(ch))
	}
	p.handleKey("enter")

	files, _ := policyloader.LoadPolicyDir(dir)
	if countRuleType(files, "sensitive_path") != 1 {
		t.Fatalf("expected 1 sensitive_path rule after add, got %d", countRuleType(files, "sensitive_path"))
	}

	// Select the entry row and delete it.
	delIdx := -1
	for i, r := range p.rows {
		if r.kind == rowSens {
			delIdx = i
			break
		}
	}
	if delIdx < 0 {
		t.Fatal("no sensitive_path entry row found after add")
	}
	p.selIdx = delIdx
	p.handleKey("d")

	files, _ = policyloader.LoadPolicyDir(dir)
	if countRuleType(files, "sensitive_path") != 0 {
		t.Errorf("expected 0 sensitive_path rules after delete, got %d", countRuleType(files, "sensitive_path"))
	}
}

// TestPolicyPanelNonAdminCannotEdit verifies the admin gate: a non-admin panel
// processes navigation only and never mutates the policy file.
func TestPolicyPanelNonAdminCannotEdit(t *testing.T) {
	p, dir := newTestPolicyPanel(t, false)
	p.selIdx = 1 // block at

	if cmd := p.handleKey("-"); cmd != nil {
		t.Fatal("non-admin edit must be a no-op (nil cmd)")
	}
	if got := effectiveThresholds(t, dir); got.BlockAt != 2 {
		t.Errorf("non-admin must not change block_at: got %d, want 2", got.BlockAt)
	}

	// Navigation still works for non-admin.
	p.selIdx = 0
	p.handleKey("j")
	if p.selIdx != 1 {
		t.Errorf("non-admin navigation broken: selIdx = %d, want 1", p.selIdx)
	}
}

// TestPolicyPanelCancelInput verifies esc abandons an in-progress entry without
// writing anything.
func TestPolicyPanelCancelInput(t *testing.T) {
	p, dir := newTestPolicyPanel(t, true)
	for i, r := range p.rows {
		if r.kind == rowAddAllow {
			p.selIdx = i
			break
		}
	}
	p.handleKey("a")
	p.handleKey("n")
	p.handleKey("p")
	p.handleKey("backspace")
	p.handleKey("esc")
	if p.editMode != editView || p.buf != "" {
		t.Fatalf("esc must reset edit mode/buffer: mode=%v buf=%q", p.editMode, p.buf)
	}
	files, _ := policyloader.LoadPolicyDir(dir)
	if countRuleType(files, "package_allowlist") != 0 {
		t.Errorf("cancelled input must not persist an entry, got %d", countRuleType(files, "package_allowlist"))
	}
}

// TestPolicyPanelDoesNotClobberOtherPolicyFiles verifies the TUI writes only its
// own managed file and leaves other user policy files in the dir untouched.
func TestPolicyPanelDoesNotClobberOtherPolicyFiles(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "user.json")
	otherPF := policyloader.PolicyFile{
		SchemaVersion: policyloader.SupportedSchemaVersion,
		Name:          "user-handwritten",
		Rules: []policyloader.PolicyRule{
			{ID: "trust-react", RuleType: "package_allowlist", Ecosystem: "npm", Packages: []string{"react"}, Action: "allow"},
		},
	}
	if errs := policyloader.SavePolicyFile(other, otherPF); len(errs) > 0 {
		t.Fatalf("seed other policy file: %v", errs)
	}
	beforeBytes, _ := os.ReadFile(other)

	p := &PolicyPanel{adminMode: true, policiesDir: dir, editMode: editView}
	p.reload()
	p.selIdx = 1 // block
	p.handleKey("-")

	afterBytes, _ := os.ReadFile(other)
	if string(afterBytes) != string(beforeBytes) {
		t.Error("TUI edit must not modify other policy files in the dir")
	}
	// Both files load and the managed file is distinct from the user file.
	files, _ := policyloader.LoadPolicyDir(dir)
	if len(files) != 2 {
		t.Errorf("expected 2 policy files (user + managed), got %d", len(files))
	}
}

// TestPolicyPanelStateTickReload verifies the panel reloads on stateTick, but
// not while a text entry is in progress (so the tick can't clobber input).
func TestPolicyPanelStateTickReload(t *testing.T) {
	p, dir := newTestPolicyPanel(t, true)

	// External edit: tighten block to 1 directly on disk.
	pf := policyloader.DefaultManagedPolicy()
	pf.Rules[0].BlockAt = 1
	if errs := policyloader.SavePolicyFile(policyloader.ManagedPolicyPath(dir), pf); len(errs) > 0 {
		t.Fatalf("external edit failed: %v", errs)
	}

	// In view mode, stateTick reloads and surfaces the external change.
	p.Update(stateTick(time.Time{}))
	if p.rows[1].value != "1" {
		t.Errorf("stateTick should surface external block_at=1, row shows %q", p.rows[1].value)
	}

	// In input mode, stateTick must NOT reload (would clobber the buffer).
	p.editMode = editAddAllow
	p.buf = "npm:lod"
	p.Update(stateTick(time.Time{}))
	if p.buf != "npm:lod" {
		t.Errorf("stateTick clobbered in-progress input: %q", p.buf)
	}
}

// TestNewPolicyPanelHermetic covers the public constructor without touching the
// real state dir by redirecting platform.StateDir via BEEKEEPER_HOME.
func TestNewPolicyPanelHermetic(t *testing.T) {
	t.Setenv("BEEKEEPER_HOME", t.TempDir())
	p := NewPolicyPanel(true)
	if !p.adminMode {
		t.Error("adminMode not propagated")
	}
	if _, err := os.Stat(policyloader.ManagedPolicyPath(p.policiesDir)); err != nil {
		t.Fatalf("NewPolicyPanel must seed the managed file under StateDir: %v", err)
	}
	if len(p.rows) == 0 {
		t.Error("rows must be built after construction")
	}
}

// TestPolicyPanelCount verifies the header count reflects allow/path entries.
func TestPolicyPanelCount(t *testing.T) {
	p, _ := newTestPolicyPanel(t, true)
	p.pf.Rules = append(p.pf.Rules,
		policyloader.PolicyRule{ID: "a", RuleType: "package_allowlist", Ecosystem: "npm", Packages: []string{"x"}, Action: "allow"},
		policyloader.PolicyRule{ID: "s", RuleType: "sensitive_path", PathPatterns: []string{"y"}, Action: "block"},
	)
	if c := p.Count(); !strings.Contains(c, "1 allow") || !strings.Contains(c, "1 path") {
		t.Errorf("Count = %q, want it to report 1 allow and 1 path", c)
	}
}

// TestPolicyPanelRenderSmoke exercises the render/PanelContent methods.
func TestPolicyPanelRenderSmoke(t *testing.T) {
	p, _ := newTestPolicyPanel(t, true)
	if p.Title() == "" || p.Footer() == "" {
		t.Error("Title/Footer must be non-empty")
	}
	if !p.Padded() {
		t.Error("policy panel should be padded")
	}
	if p.Critical() {
		t.Error("policy panel must not be critical")
	}
	_ = p.Count()
	if p.Body(80, 24) == "" {
		t.Error("Body must render content")
	}
	// Input-mode render path.
	p.editMode = editAddSens
	p.buf = "**/.env"
	if p.Body(80, 24) == "" {
		t.Error("Body (input mode) must render content")
	}
	// Non-admin footer path.
	p.editMode = editView
	p.adminMode = false
	if p.Footer() == "" {
		t.Error("non-admin Footer must be non-empty")
	}
}

func countRuleType(files []policyloader.PolicyFile, ruleType string) int {
	n := 0
	for _, pf := range files {
		for _, r := range pf.Rules {
			if r.RuleType == ruleType {
				n++
			}
		}
	}
	return n
}
