package baseline

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/policy"
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

// TestNewStoreMkdirAllError exercises the MkdirAll failure branch in NewStore.
// A component of the parent path is a regular file, so MkdirAll cannot create
// the directory tree and NewStore must return a wrapped error.
func TestNewStoreMkdirAllError(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file, then ask for a baseline path *underneath* it.
	// MkdirAll(filepath.Dir(path)) must fail because "afile" is not a directory.
	blocker := filepath.Join(dir, "afile")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	path := filepath.Join(blocker, "baselines", "project.json")

	s, err := NewStore(path)
	if err == nil {
		t.Fatalf("NewStore: expected error when parent path traverses a file, got nil (store=%v)", s)
	}
	if s != nil {
		t.Errorf("NewStore: expected nil Store on error, got %v", s)
	}
	if !strings.Contains(err.Error(), "create baselines directory") {
		t.Errorf("NewStore error = %q, want it to wrap %q", err.Error(), "create baselines directory")
	}
}

// TestLoadCorruptJSON exercises the json.Unmarshal failure branch in Load.
// A file containing invalid JSON must yield a wrapped parse error and empty
// counters (zero value), not a partially-populated struct.
func TestLoadCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := os.WriteFile(path, []byte("{ this is not valid json "), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	bc, err := s.Load()
	if err == nil {
		t.Fatal("Load: expected parse error on corrupt JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse baseline") {
		t.Errorf("Load error = %q, want it to wrap %q", err.Error(), "parse baseline")
	}
	// On error the zero-value BaselineCounters is returned (Counts left nil).
	if bc.Counts != nil {
		t.Errorf("Load on error: Counts = %v, want nil zero value", bc.Counts)
	}
}

// TestLoadNilCountsFieldInitialized verifies that valid JSON whose "counts"
// field is absent (or null) still yields a non-nil Counts map so callers can
// read from it safely. This covers the bc.Counts == nil reinitialization branch.
func TestLoadNilCountsFieldInitialized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Valid JSON with no "counts" key -> Counts unmarshals to nil.
	if err := os.WriteFile(path, []byte(`{"window_days":14}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	bc, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bc.Counts == nil {
		t.Fatal("Load: Counts is nil, want it reinitialized to an empty map")
	}
	if len(bc.Counts) != 0 {
		t.Errorf("Load: Counts has %d entries, want 0", len(bc.Counts))
	}
	if bc.WindowDays != 14 {
		t.Errorf("Load: WindowDays = %d, want 14", bc.WindowDays)
	}
}

// TestLoadNonNotExistReadError exercises the non-ErrNotExist branch of Load's
// read-error handling. Pointing the store at a directory makes os.ReadFile
// return an error that is NOT os.ErrNotExist, so Load must wrap and return it
// rather than treating it as the first-run (missing-file) case.
func TestLoadNonNotExistReadError(t *testing.T) {
	dir := t.TempDir()
	// Make the store path itself a directory.
	path := filepath.Join(dir, "project.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir at store path: %v", err)
	}

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	bc, err := s.Load()
	if err == nil {
		t.Fatal("Load: expected read error when path is a directory, got nil")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load error must not be ErrNotExist (that is the first-run case), got %v", err)
	}
	if !strings.Contains(err.Error(), "read baseline") {
		t.Errorf("Load error = %q, want it to wrap %q", err.Error(), "read baseline")
	}
	if bc.Counts != nil {
		t.Errorf("Load on error: Counts = %v, want nil zero value", bc.Counts)
	}
}

// TestSaveWriteAtomicError exercises the writeBaselineAtomic failure branch in
// Save. After NewStore creates the parent directory we remove it, so the
// os.CreateTemp inside writeBaselineAtomic fails and Save returns that error
// before it ever reaches SetOwnerOnly.
func TestSaveWriteAtomicError(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "baselines")
	path := filepath.Join(parent, "project.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Remove the parent directory so CreateTemp in the same dir fails.
	if err := os.RemoveAll(parent); err != nil {
		t.Fatalf("remove parent dir: %v", err)
	}

	bc := policy.BaselineCounters{Counts: map[string][]int64{"k": {1}}}
	if err := s.Save(bc); err == nil {
		t.Fatal("Save: expected error when temp dir does not exist, got nil")
	}

	// No file should have been created at the target path.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("Save failed but a file exists at %q (stat err=%v)", path, statErr)
	}
}

// TestWriteBaselineAtomicCreateTempError directly exercises writeBaselineAtomic
// against a directory that does not exist, asserting the CreateTemp error is
// surfaced and no target file is left behind.
func TestWriteBaselineAtomicCreateTempError(t *testing.T) {
	dir := t.TempDir()
	// Directory component does not exist.
	path := filepath.Join(dir, "missing", "project.json")

	if err := writeBaselineAtomic(path, []byte(`{"counts":{}}`)); err == nil {
		t.Fatal("writeBaselineAtomic: expected error for missing directory, got nil")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("writeBaselineAtomic failed but target exists at %q (stat err=%v)", path, statErr)
	}
}

// fakeTempFile is a test double for the createTemp seam. It writes to an
// in-memory buffer and lets each operation be forced to fail independently so
// the partial-write error branches in writeBaselineAtomic can be exercised
// deterministically without depending on filesystem error conditions.
type fakeTempFile struct {
	name     string
	writeErr error
	syncErr  error
	closeErr error

	wrote  []byte
	closed bool
	synced bool
}

func (f *fakeTempFile) Name() string { return f.name }

func (f *fakeTempFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.wrote = append(f.wrote, p...)
	return len(p), nil
}

func (f *fakeTempFile) Sync() error {
	f.synced = true
	return f.syncErr
}

func (f *fakeTempFile) Close() error {
	f.closed = true
	return f.closeErr
}

// withFakeCreateTemp swaps the package createTemp seam for one returning fake,
// restoring the original on test cleanup. It also creates the real temp file on
// disk under fake.name so the deferred os.Remove in writeBaselineAtomic is a
// harmless no-op (it does not error on a missing file anyway).
func withFakeCreateTemp(t *testing.T, fake *fakeTempFile) {
	t.Helper()
	orig := createTemp
	createTemp = func(_, _ string) (tempFile, error) {
		return fake, nil
	}
	t.Cleanup(func() { createTemp = orig })
}

// TestWriteBaselineAtomicWriteError covers the tmp.Write failure branch: the
// error is surfaced and the temp file is closed (not leaked open).
func TestWriteBaselineAtomicWriteError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")
	wantErr := errors.New("disk write boom")
	fake := &fakeTempFile{name: filepath.Join(dir, "project.json.tmp-fake"), writeErr: wantErr}
	withFakeCreateTemp(t, fake)

	err := writeBaselineAtomic(path, []byte(`{"counts":{}}`))
	if !errors.Is(err, wantErr) {
		t.Fatalf("writeBaselineAtomic error = %v, want %v", err, wantErr)
	}
	if !fake.closed {
		t.Error("temp file was not closed after Write error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("target file should not exist after Write error (stat err=%v)", statErr)
	}
}

// TestWriteBaselineAtomicSyncError covers the tmp.Sync failure branch: Write
// succeeds, Sync fails, the error is surfaced and the file is closed.
func TestWriteBaselineAtomicSyncError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")
	wantErr := errors.New("fsync boom")
	fake := &fakeTempFile{name: filepath.Join(dir, "project.json.tmp-fake"), syncErr: wantErr}
	withFakeCreateTemp(t, fake)

	err := writeBaselineAtomic(path, []byte(`{"counts":{}}`))
	if !errors.Is(err, wantErr) {
		t.Fatalf("writeBaselineAtomic error = %v, want %v", err, wantErr)
	}
	if len(fake.wrote) == 0 {
		t.Error("expected data to be written before Sync failed")
	}
	if !fake.synced {
		t.Error("Sync was not attempted")
	}
	if !fake.closed {
		t.Error("temp file was not closed after Sync error")
	}
}

// TestWriteBaselineAtomicCloseError covers the tmp.Close failure branch: Write
// and Sync succeed, Close fails, and that error is returned before the rename.
func TestWriteBaselineAtomicCloseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")
	wantErr := errors.New("close boom")
	fake := &fakeTempFile{name: filepath.Join(dir, "project.json.tmp-fake"), closeErr: wantErr}
	withFakeCreateTemp(t, fake)

	err := writeBaselineAtomic(path, []byte(`{"counts":{}}`))
	if !errors.Is(err, wantErr) {
		t.Fatalf("writeBaselineAtomic error = %v, want %v", err, wantErr)
	}
	if !fake.synced {
		t.Error("Sync should have succeeded before Close failed")
	}
	// Close failed, so the rename never ran: no target file.
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("target file should not exist after Close error (stat err=%v)", statErr)
	}
}

// TestSaveSurfacesWriteAtomicError verifies that an error from the atomic writer
// propagates out of Save (the writeBaselineAtomic != nil return branch) and that
// SetOwnerOnly is never reached, using the createTemp seam to force the failure.
func TestSaveSurfacesWriteAtomicError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")
	wantErr := errors.New("seam write failure")
	fake := &fakeTempFile{name: filepath.Join(dir, "project.json.tmp-fake"), writeErr: wantErr}
	withFakeCreateTemp(t, fake)

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if err := s.Save(policy.BaselineCounters{Counts: map[string][]int64{"k": {1}}}); !errors.Is(err, wantErr) {
		t.Fatalf("Save error = %v, want it to wrap %v", err, wantErr)
	}
}

// TestWriteBaselineAtomicRoundTrip verifies the happy path of the unexported
// atomic writer in isolation: data lands at the target and no temp file lingers.
func TestWriteBaselineAtomicRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")

	payload := []byte(`{"counts":{"Bash::ls":[1,2,3]},"window_days":7}`)
	if err := writeBaselineAtomic(path, payload); err != nil {
		t.Fatalf("writeBaselineAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after atomic write: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("written bytes = %q, want %q", got, payload)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "project.json" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("dir contents = %v, want only [project.json] (stale temp file?)", names)
	}
}
