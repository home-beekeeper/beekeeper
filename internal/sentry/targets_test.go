package sentry

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadTargetsCorruptFileReturnsError verifies the contract F-5 relies on:
// a corrupt sentry-targets.json yields (nil, err) so the daemon startup path
// can detect the parse failure, log it, and emit a targets_load_error audit
// record rather than silently disabling detection tightening.
func TestLoadTargetsCorruptFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sentry-targets.json")
	if err := os.WriteFile(path, []byte("{ this is not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	tl, err := LoadTargets(path)
	if err == nil {
		t.Fatal("LoadTargets on corrupt file returned nil error, want a parse error")
	}
	if tl != nil {
		t.Errorf("LoadTargets on corrupt file returned non-nil list %+v, want nil", tl)
	}
}

// TestTargetListAddTargetIdempotent verifies that AddTarget is idempotent:
// adding the same name twice results in only one entry.
func TestTargetListAddTargetIdempotent(t *testing.T) {
	tl := &TargetList{}
	tl.AddTarget("evil-package", "/home/user/node_modules/evil-package", "node")
	tl.AddTarget("evil-package", "/home/user/node_modules/evil-package", "node")

	if len(tl.Entries) != 1 {
		t.Errorf("AddTarget idempotent: got %d entries, want 1", len(tl.Entries))
	}
}

// TestTargetListMatchesPIDByExpectedProcess verifies that MatchesPID returns true
// when any ancestor exe matches the target's ExpectedProcess.
func TestTargetListMatchesPIDByExpectedProcess(t *testing.T) {
	tl := &TargetList{}
	tl.AddTarget("evil-package", "", "node")

	// Build a simple process tree: PID 100 (node) -> PID 200 (evil-child).
	tree := map[uint32]ProcessNode{
		100: {PID: 100, PPID: 1, Exe: "node"},
		200: {PID: 200, PPID: 100, Exe: "evil-child"},
	}

	// PID 200 descends from "node" which is in the target list.
	if !tl.MatchesPID(200, tree) {
		t.Error("MatchesPID(200, tree) = false, want true (ancestor exe matches ExpectedProcess)")
	}
}

// TestTargetListMatchesPIDNoMatch verifies that MatchesPID returns false for
// a PID whose ancestor chain does not match any target.
func TestTargetListMatchesPIDNoMatch(t *testing.T) {
	tl := &TargetList{}
	tl.AddTarget("evil-package", "", "node")

	// Build a process tree: PID 300 (python) — not a node descendant.
	tree := map[uint32]ProcessNode{
		300: {PID: 300, PPID: 1, Exe: "python"},
	}

	if tl.MatchesPID(300, tree) {
		t.Error("MatchesPID(300, tree) = true, want false (no match for python)")
	}
}

// TestTargetListMatchesPIDNilEmpty verifies that nil/empty TargetList always
// returns false (no spurious tightening).
func TestTargetListMatchesPIDNilEmpty(t *testing.T) {
	tree := map[uint32]ProcessNode{
		100: {PID: 100, PPID: 1, Exe: "node"},
	}

	var nilTL *TargetList
	if nilTL.MatchesPID(100, tree) {
		t.Error("nil TargetList.MatchesPID = true, want false")
	}

	emptyTL := &TargetList{}
	if emptyTL.MatchesPID(100, tree) {
		t.Error("empty TargetList.MatchesPID = true, want false")
	}
}

// TestLoadSaveTargetsRoundTrip verifies LoadTargets/SaveTargets round-trip.
func TestLoadSaveTargetsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sentry-targets.json")

	original := &TargetList{}
	original.AddTarget("evil-pkg", "/usr/lib/node_modules/evil-pkg", "node")
	original.AddTarget("bad-cargo", "/home/.cargo/registry/src/bad-cargo", "cargo")

	if err := SaveTargets(path, original); err != nil {
		t.Fatalf("SaveTargets error: %v", err)
	}

	loaded, err := LoadTargets(path)
	if err != nil {
		t.Fatalf("LoadTargets error: %v", err)
	}

	if len(loaded.Entries) != 2 {
		t.Fatalf("LoadTargets returned %d entries, want 2", len(loaded.Entries))
	}

	found := make(map[string]bool)
	for _, e := range loaded.Entries {
		found[e.Name] = true
	}
	if !found["evil-pkg"] {
		t.Error("evil-pkg not found after round-trip")
	}
	if !found["bad-cargo"] {
		t.Error("bad-cargo not found after round-trip")
	}
}

// TestLoadTargetsMissingFileReturnsEmpty verifies that LoadTargets on a missing
// file returns an empty list, not an error.
func TestLoadTargetsMissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")

	tl, err := LoadTargets(path)
	if err != nil {
		t.Fatalf("LoadTargets on missing file returned error: %v", err)
	}
	if tl == nil {
		t.Fatal("LoadTargets returned nil, want empty list")
	}
	if len(tl.Entries) != 0 {
		t.Errorf("LoadTargets on missing file returned %d entries, want 0", len(tl.Entries))
	}
}
