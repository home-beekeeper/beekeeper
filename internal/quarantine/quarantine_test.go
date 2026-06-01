package quarantine_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/quarantine"
)

// writeManifest is a test helper that creates a quarantine entry directory
// containing a beekeeper-manifest.json with the given manifest.
func writeManifest(t *testing.T, extDir, id string, m quarantine.Manifest) {
	t.Helper()
	entryDir := filepath.Join(extDir, id)
	if err := os.MkdirAll(entryDir, 0o700); err != nil {
		t.Fatalf("mkdir entry dir: %v", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entryDir, "beekeeper-manifest.json"), data, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// TestQuarantineList verifies that List returns exactly the entries with valid
// manifests and silently skips directories that have no manifest.
func TestQuarantineList(t *testing.T) {
	quarantineDir := t.TempDir()
	extDir := quarantine.ExtensionsDir(quarantineDir)

	// Entry 1: valid manifest.
	writeManifest(t, extDir, "nrwl.angular-console-18.95.0-1", quarantine.Manifest{
		ID:        "nrwl.angular-console-18.95.0-1",
		Publisher: "nrwl",
		Name:      "angular-console",
		Version:   "18.95.0",
		Reason:    "malicious install script",
	})

	// Entry 2: valid manifest.
	writeManifest(t, extDir, "ms-python.python-2024.1.0-2", quarantine.Manifest{
		ID:        "ms-python.python-2024.1.0-2",
		Publisher: "ms-python",
		Name:      "python",
		Version:   "2024.1.0",
		Reason:    "suspicious network activity",
	})

	// Entry 3: directory WITHOUT a manifest — should be skipped.
	noManifestDir := filepath.Join(extDir, "broken-entry")
	if err := os.MkdirAll(noManifestDir, 0o700); err != nil {
		t.Fatalf("mkdir no-manifest dir: %v", err)
	}

	manifests, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("List returned %d entries, want 2 (entry without manifest must be skipped)", len(manifests))
	}
}

// TestQuarantineRestore verifies the full Move → Restore lifecycle:
// after Move, the extension is no longer at extensionPath; after Restore,
// it is back at extensionPath.
func TestQuarantineRestore(t *testing.T) {
	quarantineDir := t.TempDir()

	// Create a fake extension directory at extensionPath.
	extensionPath := filepath.Join(t.TempDir(), "angular-console")
	if err := os.MkdirAll(extensionPath, 0o700); err != nil {
		t.Fatalf("mkdir extension: %v", err)
	}
	// Put a sentinel file inside so we can verify the directory moved.
	sentinelPath := filepath.Join(extensionPath, "extension.vsixmanifest")
	if err := os.WriteFile(sentinelPath, []byte("sentinel"), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	m := quarantine.Manifest{
		Publisher:     "nrwl",
		Name:          "angular-console",
		Version:       "18.95.0",
		DisplayName:   "Nx Console",
		Reason:        "catalog match: high severity",
		RuleIDs:       []string{"EXTQ-001"},
		QuarantinedAt: time.Now().UTC(),
	}

	id, err := quarantine.Move(quarantineDir, extensionPath, m)
	if err != nil {
		t.Fatalf("Move error: %v", err)
	}
	if id == "" {
		t.Fatal("Move returned empty id")
	}

	// extensionPath should no longer exist.
	if _, statErr := os.Stat(extensionPath); !os.IsNotExist(statErr) {
		t.Errorf("extensionPath %q still exists after Move, want gone", extensionPath)
	}

	// The quarantine entry should exist.
	entryDir := filepath.Join(quarantine.ExtensionsDir(quarantineDir), id)
	if _, statErr := os.Stat(entryDir); statErr != nil {
		t.Fatalf("quarantine entry dir %q not found after Move: %v", entryDir, statErr)
	}

	// Restore it.
	if err := quarantine.Restore(quarantineDir, id); err != nil {
		t.Fatalf("Restore error: %v", err)
	}

	// extensionPath should be back.
	if _, statErr := os.Stat(extensionPath); statErr != nil {
		t.Errorf("extensionPath %q not restored: %v", extensionPath, statErr)
	}
	// Sentinel file should be inside.
	if _, statErr := os.Stat(sentinelPath); statErr != nil {
		t.Errorf("sentinel file %q not found after Restore: %v", sentinelPath, statErr)
	}

	// Quarantine entry should no longer exist.
	if _, statErr := os.Stat(entryDir); !os.IsNotExist(statErr) {
		t.Errorf("quarantine entry dir %q still exists after Restore, want gone", entryDir)
	}
}

// TestQuarantinePurge verifies that Purge removes all entries and returns
// their IDs.
func TestQuarantinePurge(t *testing.T) {
	quarantineDir := t.TempDir()

	// Create two extension source directories.
	ext1 := filepath.Join(t.TempDir(), "ext1")
	ext2 := filepath.Join(t.TempDir(), "ext2")
	for _, p := range []string{ext1, ext2} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
	}

	m1 := quarantine.Manifest{Publisher: "pub1", Name: "ext1", Version: "1.0.0", Reason: "test"}
	m2 := quarantine.Manifest{Publisher: "pub2", Name: "ext2", Version: "2.0.0", Reason: "test"}

	id1, err := quarantine.Move(quarantineDir, ext1, m1)
	if err != nil {
		t.Fatalf("Move ext1: %v", err)
	}
	id2, err := quarantine.Move(quarantineDir, ext2, m2)
	if err != nil {
		t.Fatalf("Move ext2: %v", err)
	}

	purged, err := quarantine.Purge(quarantineDir)
	if err != nil {
		t.Fatalf("Purge error: %v", err)
	}

	// Both IDs should be in the purged list.
	if len(purged) != 2 {
		t.Errorf("Purge returned %d ids, want 2 (got %v)", len(purged), purged)
	}
	purgedSet := make(map[string]bool, len(purged))
	for _, pid := range purged {
		purgedSet[pid] = true
	}
	if !purgedSet[id1] {
		t.Errorf("id1 %q not in purged list", id1)
	}
	if !purgedSet[id2] {
		t.Errorf("id2 %q not in purged list", id2)
	}

	// ExtensionsDir should now be empty.
	remaining, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("List after Purge error: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("List after Purge returned %d entries, want 0", len(remaining))
	}
}

// TestQuarantineRestorePathTraversal verifies that Restore rejects IDs that
// attempt to escape the quarantine root via path traversal.
func TestQuarantineRestorePathTraversal(t *testing.T) {
	quarantineDir := t.TempDir()

	err := quarantine.Restore(quarantineDir, "../../escape")
	if err == nil {
		t.Error("Restore with path-traversal id should return an error, got nil")
	}
}
