package baseline

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
)

func TestLoadMissingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baselines", "project.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	bc, err := s.Load()
	if err != nil {
		t.Fatalf("Load on missing file returned error: %v", err)
	}
	if bc.Counts == nil {
		t.Fatal("Load: Counts is nil, want initialized empty map")
	}
	if len(bc.Counts) != 0 {
		t.Fatalf("Load: Counts has %d entries, want 0", len(bc.Counts))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	original := policy.BaselineCounters{
		Counts: map[string][]int64{
			"Bash::npm install": {1716676800, 1716763200, 1716849600},
		},
		WindowDays: 7,
	}

	if err := s.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}

	if loaded.WindowDays != original.WindowDays {
		t.Errorf("WindowDays = %d, want %d", loaded.WindowDays, original.WindowDays)
	}

	key := "Bash::npm install"
	if len(loaded.Counts[key]) != 3 {
		t.Errorf("Counts[%q] has %d entries, want 3", key, len(loaded.Counts[key]))
	}
	for i, ts := range loaded.Counts[key] {
		if ts != original.Counts[key][i] {
			t.Errorf("Counts[%q][%d] = %d, want %d", key, i, ts, original.Counts[key][i])
		}
	}
}

func TestSaveEnforcesOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		// On Windows the DACL-based check is platform-specific.
		// SetOwnerOnly still runs on Windows but mode bits are not POSIX.
		t.Skip("skipping Unix file mode assertion on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	bc := policy.BaselineCounters{Counts: map[string][]int64{}}
	if err := s.Save(bc); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat after Save: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestNewStoreCreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	// The parent "baselines" directory does not exist yet.
	path := filepath.Join(dir, "baselines", "subdir", "project.json")

	_, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore should create parent dirs but got: %v", err)
	}

	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	if err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", parentDir)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	// Verify that the temp file does not survive after Save returns (rename succeeded).
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	bc := policy.BaselineCounters{Counts: map[string][]int64{"k": {1}}}
	if err := s.Save(bc); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The rename should have succeeded: no temp files in dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "project.json" {
			t.Errorf("unexpected file in baselines dir: %q (expected only project.json)", e.Name())
		}
	}
}
