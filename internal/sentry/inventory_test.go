package sentry

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestInventoryStoreRecordAndSnapshot verifies that RecordInstall and Snapshot
// return the recorded extension within the TTL window and prune it after.
func TestInventoryStoreRecordAndSnapshot(t *testing.T) {
	store := NewInventoryStore()
	store.InventoryTTL = 30 * time.Minute

	now := time.Now().UTC()
	store.RecordInstall("ms-python.python", now.Add(-5*time.Minute)) // within window
	store.RecordInstall("old-ext.old", now.Add(-60*time.Minute))     // outside window

	snap := store.Snapshot(now)
	if _, ok := snap.RecentExtensions["ms-python.python"]; !ok {
		t.Error("expected ms-python.python in snapshot (installed 5 min ago)")
	}
	if _, ok := snap.RecentExtensions["old-ext.old"]; ok {
		t.Error("expected old-ext.old pruned from snapshot (installed 60 min ago)")
	}
}

// TestInventoryStoreScanDirs verifies that ScanDirs picks up extension
// directories modified within the TTL window and ignores older ones.
func TestInventoryStoreScanDirs(t *testing.T) {
	dir := t.TempDir()

	now := time.Now().UTC()

	// Fresh extension directory (modified 2 minutes ago within 30 min TTL).
	freshDir := filepath.Join(dir, "ms-python.python-2026.4.0")
	if err := os.MkdirAll(freshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Set modtime to 2 minutes ago.
	freshTime := now.Add(-2 * time.Minute)
	if err := os.Chtimes(freshDir, freshTime, freshTime); err != nil {
		t.Fatal(err)
	}

	// Old extension directory (modified 2 hours ago — outside 30 min TTL).
	oldDir := filepath.Join(dir, "GitHub.copilot-1.0.0")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatal(err)
	}
	oldTime := now.Add(-2 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	store := NewInventoryStore()
	store.InventoryTTL = 30 * time.Minute
	store.ScanDirs([]string{dir}, now)

	snap := store.Snapshot(now)
	if _, ok := snap.RecentExtensions["ms-python.python"]; !ok {
		t.Error("expected ms-python.python in snapshot after ScanDirs")
	}
	if _, ok := snap.RecentExtensions["github.copilot"]; ok {
		t.Error("expected github.copilot pruned from snapshot (too old)")
	}
}

// TestInventoryStoreExtensionIDFromDirName exercises extensionIDFromDirName
// with a variety of directory name patterns.
func TestInventoryStoreExtensionIDFromDirName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"normal VS Code ext", "ms-python.python-2026.4.0", "ms-python.python"},
		{"mixed-case", "GitHub.copilot-1.5.0", "github.copilot"},
		{"no version suffix", "publisher.name", "publisher.name"},
		{"dot prefix (skip)", ".obsolete", ""},
		{"no dot (skip)", "extensions", ""},
		{"empty string", "", ""},
		{"only dot", ".", ""},
		{"dot at start", ".beekeeper", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extensionIDFromDirName(tc.in)
			if got != tc.want {
				t.Errorf("extensionIDFromDirName(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSENTRY004FiresViaInventoryStore verifies that SENTRY-004 fires in the
// production evaluation path when an InventoryStore (not direct test injection)
// provides the fresh-extension signal.
//
// This is the TM-RS-01 regression test: in the unfixed code, the production
// correlationEngineLoop always passed sentry.InventorySnapshot{} (empty), so
// SENTRY-004 could never fire. With the fix, the store is seeded via ScanDirs
// and Snapshot() is called on each EvaluateEvent call.
func TestSENTRY004FiresViaInventoryStore(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	// Create a fresh extension dir (modified 5 minutes ago).
	extDir := filepath.Join(dir, "evil-ext.malicious-1.0.0")
	if err := os.MkdirAll(extDir, 0o700); err != nil {
		t.Fatal(err)
	}
	freshTime := now.Add(-5 * time.Minute)
	if err := os.Chtimes(extDir, freshTime, freshTime); err != nil {
		t.Fatal(err)
	}

	// Build the InventoryStore as the daemon would.
	invStore := NewInventoryStore()
	invStore.ScanDirs([]string{dir}, now)

	// Use the same evaluation path as the production daemon loop.
	tree := editorTree()
	state := NewRuleState()

	var allAlerts []SentryAlert
	for i, path := range []string{"/home/user/.ssh/id_rsa", "/home/user/.aws/credentials"} {
		t2 := now.Add(time.Duration(i) * time.Second)
		ev := SentryEvent{
			Kind:     EventFileAccess,
			PID:      100,
			PPID:     1,
			Exe:      "/usr/bin/some-tool",
			FilePath: path,
			WallTime: t2,
		}
		// Production path: call Snapshot() on each evaluation — mirrors the fixed daemon loop.
		alerts := EvaluateEvent(ev, state, tree, invStore.Snapshot(t2), RuleConfig{}, noBaseline(), t2)
		allAlerts = append(allAlerts, alerts...)
	}

	if !hasAlert(allAlerts, "SENTRY-004") {
		t.Errorf("TM-RS-01: SENTRY-004 did not fire via InventoryStore production path; got alerts: %v", allAlerts)
	}
}

// TestSENTRY005FiresViaInventoryStore verifies that SENTRY-005 fires in the
// production evaluation path when an InventoryStore provides the inventory.
func TestSENTRY005FiresViaInventoryStore(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Now().UTC()

	// Create a fresh extension dir (modified 1 minute ago).
	extDir := filepath.Join(dir, "evil-ext.exfil-1.0.0")
	if err := os.MkdirAll(extDir, 0o700); err != nil {
		t.Fatal(err)
	}
	freshTime := t0.Add(-1 * time.Minute)
	if err := os.Chtimes(extDir, freshTime, freshTime); err != nil {
		t.Fatal(err)
	}

	invStore := NewInventoryStore()
	invStore.ScanDirs([]string{dir}, t0)

	tree := editorTree()
	state := NewRuleState()

	// Step 1: sensitive file read (adds to CredAccessByPID but doesn't trigger SENTRY-001 alone).
	ev1 := SentryEvent{
		Kind:     EventFileAccess,
		PID:      100,
		PPID:     1,
		Exe:      "/usr/bin/some-tool",
		FilePath: "/home/user/.ssh/id_rsa",
		WallTime: t0,
	}
	EvaluateEvent(ev1, state, tree, invStore.Snapshot(t0), RuleConfig{}, noBaseline(), t0)

	// Step 2: network connection at T+2min — triggers SENTRY-005 (exfil fusion).
	t2 := t0.Add(2 * time.Minute)
	ev2 := SentryEvent{
		Kind:    EventNetworkConnect,
		PID:     100,
		PPID:    1,
		Exe:     "/usr/bin/some-tool",
		DstAddr: net.ParseIP("5.6.7.8"),
		DstPort: 80,
		WallTime: t2,
	}
	alerts := EvaluateEvent(ev2, state, tree, invStore.Snapshot(t2), RuleConfig{}, noBaseline(), t2)

	if !hasAlert(alerts, "SENTRY-005") {
		t.Errorf("TM-RS-01: SENTRY-005 did not fire via InventoryStore production path; got: %v", alerts)
	}
}
