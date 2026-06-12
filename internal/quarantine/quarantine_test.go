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

// TestQuarantineRestoreTamperedOriginalPath verifies TM-D-05: Restore rejects
// a manifest whose OriginalPath has been tampered with to attempt:
//   - a relative path with ".." traversal components
//   - an absolute path that resolves inside the quarantine directory itself
//
// Both cases must return a non-nil error and leave the quarantine entry intact.
func TestQuarantineRestoreTamperedOriginalPath(t *testing.T) {
	tests := []struct {
		name         string
		originalPath func(quarantineDir string) string
	}{
		{
			name: "relative traversal",
			// e.g. "../../etc/cron.d" — would escape the extensions root
			originalPath: func(_ string) string { return "../../etc/cron.d" },
		},
		{
			name: "dotdot in absolute path",
			// absolute but contains ".." component
			originalPath: func(q string) string {
				// Build a path like /tmp/quarantine/../../../etc/passwd
				return filepath.Join(q, "..", "..", "etc", "passwd")
			},
		},
		{
			name: "restore into quarantine dir itself",
			// OriginalPath resolves inside quarantineDir — restore-to-quarantine cycle
			originalPath: func(q string) string {
				return filepath.Join(q, "planted-file")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quarantineDir := t.TempDir()
			extDir := quarantine.ExtensionsDir(quarantineDir)

			// Craft a quarantine entry with a tampered manifest manually, since
			// Move() always sets OriginalPath from the actual extension path.
			const entryID = "evil.pkg-1.0.0-99999999999"
			entryDir := filepath.Join(extDir, entryID)
			if err := os.MkdirAll(entryDir, 0o700); err != nil {
				t.Fatalf("mkdir entry: %v", err)
			}

			tamperedPath := tt.originalPath(quarantineDir)
			m := quarantine.Manifest{
				ID:           entryID,
				Publisher:    "evil",
				Name:         "pkg",
				Version:      "1.0.0",
				OriginalPath: tamperedPath,
			}
			data, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				t.Fatalf("marshal manifest: %v", err)
			}
			if err := os.WriteFile(filepath.Join(entryDir, "beekeeper-manifest.json"), data, 0o600); err != nil {
				t.Fatalf("write manifest: %v", err)
			}

			// Restore must be rejected.
			err = quarantine.Restore(quarantineDir, entryID)
			if err == nil {
				t.Errorf("[%s] Restore with tampered original_path %q should return error, got nil", tt.name, tamperedPath)
			} else {
				t.Logf("[%s] correctly rejected: %v", tt.name, err)
			}

			// The quarantine entry must still be intact (nothing was moved).
			if _, statErr := os.Stat(entryDir); statErr != nil {
				t.Errorf("[%s] quarantine entry dir was removed even though restore was rejected", tt.name)
			}
		})
	}
}

// --- NEW TESTS FOR TYPE-AWARE QUARANTINE (Task 1 / C1) ---

// TestMoveTypedEditorExtensionBackCompat verifies that a MoveTyped call with
// ArtifactType="editor-extension" places the entry under extensions/ (back-compat
// with the existing layout, so EDXT-03 callers are unchanged).
func TestMoveTypedEditorExtensionBackCompat(t *testing.T) {
	quarantineDir := t.TempDir()

	extPath := filepath.Join(t.TempDir(), "my-extension")
	if err := os.MkdirAll(extPath, 0o700); err != nil {
		t.Fatalf("mkdir ext: %v", err)
	}

	m := quarantine.Manifest{
		Publisher:    "pub",
		Name:         "myext",
		Version:      "1.0.0",
		ArtifactType: quarantine.ArtifactTypeEditorExtension,
		Reason:       "test",
	}

	id, err := quarantine.MoveTyped(quarantineDir, extPath, m)
	if err != nil {
		t.Fatalf("MoveTyped error: %v", err)
	}

	// Must live under extensions/ subdir.
	extDir := quarantine.ExtensionsDir(quarantineDir)
	entryDir := filepath.Join(extDir, id)
	if _, statErr := os.Stat(entryDir); statErr != nil {
		t.Errorf("editor-extension entry not found under extensions/: %v", statErr)
	}
}

// TestMoveTypedLanguagePackageRoundTrip verifies that a language-package can be
// moved into quarantine (under packages/) and fully restored to OriginalPath.
func TestMoveTypedLanguagePackageRoundTrip(t *testing.T) {
	quarantineDir := t.TempDir()

	// Create a fake npm package directory.
	pkgPath := filepath.Join(t.TempDir(), "node_modules", "left-pad")
	if err := os.MkdirAll(pkgPath, 0o700); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	sentinelPath := filepath.Join(pkgPath, "index.js")
	if err := os.WriteFile(sentinelPath, []byte("module.exports = leftPad;"), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	m := quarantine.Manifest{
		Publisher:    "npm",
		Name:         "left-pad",
		Version:      "1.3.0",
		ArtifactType: quarantine.ArtifactTypeLanguagePackage,
		Reason:       "catalog match",
	}

	id, err := quarantine.MoveTyped(quarantineDir, pkgPath, m)
	if err != nil {
		t.Fatalf("MoveTyped error: %v", err)
	}

	// Package dir should no longer exist at original location.
	if _, statErr := os.Stat(pkgPath); !os.IsNotExist(statErr) {
		t.Errorf("pkgPath %q still exists after MoveTyped, want gone", pkgPath)
	}

	// Entry should be under packages/ subdir.
	pkgDir := quarantine.PackagesDir(quarantineDir)
	entryDir := filepath.Join(pkgDir, id)
	if _, statErr := os.Stat(entryDir); statErr != nil {
		t.Fatalf("language-package entry not found under packages/: %v", statErr)
	}

	// Restore must put it back byte-identical.
	if err := quarantine.Restore(quarantineDir, id); err != nil {
		t.Fatalf("Restore error: %v", err)
	}
	if _, statErr := os.Stat(pkgPath); statErr != nil {
		t.Errorf("pkgPath not restored: %v", statErr)
	}
	if _, statErr := os.Stat(sentinelPath); statErr != nil {
		t.Errorf("sentinel file not found after Restore: %v", statErr)
	}
}

// TestListBothSubdirs verifies that List enumerates entries from BOTH extensions/
// and packages/ subdirectories.
func TestListBothSubdirs(t *testing.T) {
	quarantineDir := t.TempDir()

	// Create one extension entry.
	ext := filepath.Join(t.TempDir(), "myext")
	if err := os.MkdirAll(ext, 0o700); err != nil {
		t.Fatalf("mkdir ext: %v", err)
	}
	if _, err := quarantine.MoveTyped(quarantineDir, ext, quarantine.Manifest{
		Publisher:    "pub",
		Name:         "myext",
		Version:      "1.0.0",
		ArtifactType: quarantine.ArtifactTypeEditorExtension,
		Reason:       "test",
	}); err != nil {
		t.Fatalf("MoveTyped ext: %v", err)
	}

	// Create one package entry.
	pkg := filepath.Join(t.TempDir(), "mypkg")
	if err := os.MkdirAll(pkg, 0o700); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if _, err := quarantine.MoveTyped(quarantineDir, pkg, quarantine.Manifest{
		Publisher:    "npm",
		Name:         "mypkg",
		Version:      "2.0.0",
		ArtifactType: quarantine.ArtifactTypeLanguagePackage,
		Reason:       "test",
	}); err != nil {
		t.Fatalf("MoveTyped pkg: %v", err)
	}

	manifests, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("List returned %d entries, want 2 (one ext + one pkg)", len(manifests))
	}
}

// TestPurgeBothSubdirs verifies that Purge removes entries from both subdirs.
func TestPurgeBothSubdirs(t *testing.T) {
	quarantineDir := t.TempDir()

	ext := filepath.Join(t.TempDir(), "myext")
	if err := os.MkdirAll(ext, 0o700); err != nil {
		t.Fatalf("mkdir ext: %v", err)
	}
	if _, err := quarantine.MoveTyped(quarantineDir, ext, quarantine.Manifest{
		Publisher:    "pub",
		Name:         "myext",
		Version:      "1.0.0",
		ArtifactType: quarantine.ArtifactTypeEditorExtension,
		Reason:       "test",
	}); err != nil {
		t.Fatalf("MoveTyped ext: %v", err)
	}

	pkg := filepath.Join(t.TempDir(), "mypkg")
	if err := os.MkdirAll(pkg, 0o700); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if _, err := quarantine.MoveTyped(quarantineDir, pkg, quarantine.Manifest{
		Publisher:    "npm",
		Name:         "mypkg",
		Version:      "2.0.0",
		ArtifactType: quarantine.ArtifactTypeLanguagePackage,
		Reason:       "test",
	}); err != nil {
		t.Fatalf("MoveTyped pkg: %v", err)
	}

	purged, err := quarantine.Purge(quarantineDir)
	if err != nil {
		t.Fatalf("Purge error: %v", err)
	}
	if len(purged) != 2 {
		t.Errorf("Purge returned %d ids, want 2", len(purged))
	}

	remaining, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("List after Purge: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("List after Purge returned %d entries, want 0", len(remaining))
	}
}

// TestMoveTypedTraversalGuardPackages verifies that path-traversal attacks on
// publisher/name/version fields for language-package entries are sanitized.
func TestMoveTypedTraversalGuardPackages(t *testing.T) {
	quarantineDir := t.TempDir()

	pkg := filepath.Join(t.TempDir(), "mypkg")
	if err := os.MkdirAll(pkg, 0o700); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}

	// Attacker-controlled publisher/name with traversal components.
	m := quarantine.Manifest{
		Publisher:    "../../etc",
		Name:         "../passwd",
		Version:      "1.0.0",
		ArtifactType: quarantine.ArtifactTypeLanguagePackage,
		Reason:       "test",
	}

	// MoveTyped must not place files outside the packages/ subdir.
	id, err := quarantine.MoveTyped(quarantineDir, pkg, m)
	if err != nil {
		// Guard triggered — acceptable.
		t.Logf("MoveTyped correctly returned error on traversal attempt: %v", err)
		return
	}

	// If it succeeded, the entry must be inside packages/ (not escaped).
	pkgDir := quarantine.PackagesDir(quarantineDir)
	entryDir := filepath.Join(pkgDir, id)
	if _, statErr := os.Stat(entryDir); statErr != nil {
		t.Errorf("entry should be under packages/ if MoveTyped succeeded, but not found: %v", statErr)
	}
}

// TestMoveWrapperDefaultsToEditorExtension verifies that the legacy Move wrapper
// sets ArtifactType="editor-extension" and places entries under extensions/.
func TestMoveWrapperDefaultsToEditorExtension(t *testing.T) {
	quarantineDir := t.TempDir()

	extPath := filepath.Join(t.TempDir(), "legacy-ext")
	if err := os.MkdirAll(extPath, 0o700); err != nil {
		t.Fatalf("mkdir ext: %v", err)
	}

	m := quarantine.Manifest{
		Publisher: "ms",
		Name:      "pylance",
		Version:   "2026.1.0",
		Reason:    "catalog match",
	}

	id, err := quarantine.Move(quarantineDir, extPath, m)
	if err != nil {
		t.Fatalf("Move error: %v", err)
	}

	// Must land under extensions/.
	extDir := quarantine.ExtensionsDir(quarantineDir)
	entryDir := filepath.Join(extDir, id)
	if _, statErr := os.Stat(entryDir); statErr != nil {
		t.Errorf("Move wrapper: entry not under extensions/: %v", statErr)
	}

	// List must include it.
	manifests, err := quarantine.List(quarantineDir)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(manifests) != 1 {
		t.Errorf("List returned %d entries, want 1", len(manifests))
	}
	if manifests[0].ArtifactType != quarantine.ArtifactTypeEditorExtension {
		t.Errorf("ArtifactType = %q, want %q", manifests[0].ArtifactType, quarantine.ArtifactTypeEditorExtension)
	}
}
