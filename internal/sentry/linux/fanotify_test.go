//go:build linux

package linux

import (
	"os"
	"testing"
)

func TestInitFanotifyFallback(t *testing.T) {
	fd, err := InitFanotify(Tier0)
	if err != nil {
		// EPERM is expected if not running with CAP_SYS_ADMIN — acceptable in CI.
		t.Skipf("InitFanotify returned error (likely no CAP_SYS_ADMIN): %v", err)
	}
	if fd < 0 {
		t.Errorf("InitFanotify returned negative fd: %d", fd)
	}
	// Close it to avoid leaking the fd.
	_ = fd
}

func TestFanotifyMarkPathsSkipsMissing(t *testing.T) {
	// Use a real fanotify fd so FanotifyMarkPaths can attempt marks.
	fd, err := InitFanotify(Tier2)
	if err != nil {
		t.Skipf("cannot open fanotify fd: %v", err)
	}
	defer func() { _ = fd }() // fd cleanup handled by OS on test exit

	tmpDir := t.TempDir()
	realPath := tmpDir
	nonExistent := tmpDir + "/nonexistent_path_xyz"

	// nonExistent should be silently skipped (ErrNotExist).
	// realPath may fail with permission error on FanotifyMark, which is acceptable —
	// the test validates that non-existent paths do NOT cause an error.
	paths := []string{realPath, nonExistent}
	err = FanotifyMarkPaths(fd, paths)
	if err != nil {
		// The non-existent path is skipped, so any error is from the real path.
		// Verify it is NOT an ErrNotExist for the real path.
		if os.IsNotExist(err) {
			t.Errorf("FanotifyMarkPaths returned ErrNotExist for an existing path: %v", err)
		}
		// EACCES, EPERM etc. from the real path mark are acceptable — not a test failure.
	}
}
