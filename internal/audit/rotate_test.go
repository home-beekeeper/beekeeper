package audit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestRotateCreatesNumberedArchive verifies that when the audit log exceeds
// maxBytes, Rotate renames it to .1 and creates a new empty log.
func TestRotateCreatesNumberedArchive(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "beekeeper.ndjson")

	// Write 100 bytes
	if err := os.WriteFile(auditPath, make([]byte, 100), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Rotate(auditPath, 50, 30); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// beekeeper.ndjson.1 must exist
	if _, err := os.Stat(auditPath + ".1"); err != nil {
		t.Errorf("expected beekeeper.ndjson.1 to exist: %v", err)
	}

	// beekeeper.ndjson must exist and be empty
	info, err := os.Stat(auditPath)
	if err != nil {
		t.Errorf("expected beekeeper.ndjson to exist: %v", err)
	} else if info.Size() != 0 {
		t.Errorf("expected beekeeper.ndjson to be empty, got size=%d", info.Size())
	}

	// On Unix verify 0600 permissions.
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected 0600 perms, got %o", info.Mode().Perm())
		}
	}
}

// TestRotateShiftsExistingArchives verifies that existing .1 is shifted to .2
// before the current log becomes .1.
func TestRotateShiftsExistingArchives(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "beekeeper.ndjson")

	// Current log: 100 bytes
	if err := os.WriteFile(auditPath, make([]byte, 100), 0600); err != nil {
		t.Fatal(err)
	}
	// Existing archive .1: 5 bytes
	if err := os.WriteFile(auditPath+".1", make([]byte, 5), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Rotate(auditPath, 50, 30); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// .2 must exist (was .1)
	if _, err := os.Stat(auditPath + ".2"); err != nil {
		t.Errorf("expected beekeeper.ndjson.2 to exist: %v", err)
	}
	// .1 must exist (was current)
	if _, err := os.Stat(auditPath + ".1"); err != nil {
		t.Errorf("expected beekeeper.ndjson.1 to exist: %v", err)
	}
	// current must be empty
	info, err := os.Stat(auditPath)
	if err != nil {
		t.Errorf("expected beekeeper.ndjson to exist: %v", err)
	} else if info.Size() != 0 {
		t.Errorf("expected beekeeper.ndjson to be empty, got size=%d", info.Size())
	}
}

// TestRotateDeletesOldArchives verifies that archives older than retentionDays
// are removed before shifting, so the old content is gone and the rotated
// current log lands at .1.
func TestRotateDeletesOldArchives(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "beekeeper.ndjson")

	// Old archive .1: 5 bytes, mtime set 40 days in the past.
	oldArchive := auditPath + ".1"
	if err := os.WriteFile(oldArchive, make([]byte, 5), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(oldArchive, old, old); err != nil {
		t.Fatal(err)
	}

	// Current log: 100 bytes (exceeds maxBytes=50).
	if err := os.WriteFile(auditPath, make([]byte, 100), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Rotate(auditPath, 50, 30); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// After rotation:
	// - Old .1 (5 bytes, 40 days old) was deleted.
	// - Current log (100 bytes) became the new .1.
	// - No .2 should exist (old .1 was not shifted, it was deleted).
	if _, err := os.Stat(auditPath + ".2"); !os.IsNotExist(err) {
		t.Errorf("expected no beekeeper.ndjson.2 (old .1 was deleted, not shifted)")
	}

	// New .1 must exist and have the old current-log size (100 bytes).
	info, err := os.Stat(auditPath + ".1")
	if err != nil {
		t.Fatalf("expected new beekeeper.ndjson.1 to exist: %v", err)
	}
	if info.Size() != 100 {
		t.Errorf("expected new .1 to have 100 bytes (was current log), got %d", info.Size())
	}

	// Current log must exist and be empty (freshly created).
	currInfo, err := os.Stat(auditPath)
	if err != nil {
		t.Errorf("expected beekeeper.ndjson to exist: %v", err)
	} else if currInfo.Size() != 0 {
		t.Errorf("expected empty current log, got size=%d", currInfo.Size())
	}
}

// TestRotateNoOpWhenSmall verifies that when the log is smaller than maxBytes
// Rotate is a no-op.
func TestRotateNoOpWhenSmall(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "beekeeper.ndjson")

	if err := os.WriteFile(auditPath, make([]byte, 10), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Rotate(auditPath, 1000, 30); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// No archive must exist
	if _, err := os.Stat(auditPath + ".1"); !os.IsNotExist(err) {
		t.Errorf("expected beekeeper.ndjson.1 not to exist after no-op rotate")
	}

	// Original log must still have 10 bytes
	info, err := os.Stat(auditPath)
	if err != nil {
		t.Errorf("expected beekeeper.ndjson to exist: %v", err)
	} else if info.Size() != 10 {
		t.Errorf("expected 10 bytes, got %d", info.Size())
	}
}
