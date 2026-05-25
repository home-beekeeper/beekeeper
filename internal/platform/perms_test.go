package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSetOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.ndjson")

	if err := os.WriteFile(path, []byte("sensitive\n"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if err := SetOwnerOnly(path); err != nil {
		t.Fatalf("SetOwnerOnly(%q) returned error: %v", path, err)
	}

	if runtime.GOOS == "windows" {
		// DACL content assertion is out of scope for this unit test; a nil
		// error is the contract here. Behavioral DACL verification is covered
		// by the audit plan.
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) returned error: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("after SetOwnerOnly, mode = %#o, want %#o", perm, 0600)
	}
}
