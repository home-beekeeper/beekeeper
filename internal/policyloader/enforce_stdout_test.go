package policyloader

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadPolicyDirNeverWritesStdout is the regression guard for the hook-protocol
// corruption bug: beekeeper check runs LoadPolicyDir on every call, so the
// "skipping invalid policy file" warning MUST go to stderr, never stdout. A
// foreign file in policies/ (e.g. the retired tui_rules.json) must not emit a
// single byte on stdout that could corrupt the hook's JSON/deny output.
func TestLoadPolicyDirNeverWritesStdout(t *testing.T) {
	dir := t.TempDir()

	// A foreign/invalid file that LoadPolicyDir will skip (this is what triggered
	// the stray warning).
	if err := os.WriteFile(filepath.Join(dir, "tui_rules.json"),
		[]byte(`[{"id":"corroboration","enabled":false}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	// A valid file that must still load.
	if errs := SavePolicyFile(ManagedPolicyPath(dir), DefaultManagedPolicy()); len(errs) > 0 {
		t.Fatalf("seed valid file: %v", errs)
	}

	// Capture stdout across the LoadPolicyDir call.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	files, loadErr := LoadPolicyDir(dir)

	_ = w.Close()
	os.Stdout = old
	var captured bytes.Buffer
	_, _ = io.Copy(&captured, r)

	if loadErr != nil {
		t.Fatalf("LoadPolicyDir error: %v", loadErr)
	}
	if captured.Len() != 0 {
		t.Errorf("LoadPolicyDir wrote %d bytes to stdout (hook-protocol corruption): %q",
			captured.Len(), captured.String())
	}
	// The valid managed file must still have loaded despite the foreign sibling.
	if len(files) != 1 {
		t.Errorf("expected 1 valid policy file loaded, got %d", len(files))
	}
}
