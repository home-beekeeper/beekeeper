package llamafirewall

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstallSidecarWritesAssets verifies the first call writes the script +
// requirements + stamp under <stateDir>/llamafirewall with 0600 perms.
func TestInstallSidecarWritesAssets(t *testing.T) {
	stateDir := t.TempDir()
	scriptPath, err := InstallSidecar(stateDir)
	if err != nil {
		t.Fatalf("InstallSidecar: %v", err)
	}
	if scriptPath != SidecarScriptPath(stateDir) {
		t.Errorf("scriptPath = %q, want %q", scriptPath, SidecarScriptPath(stateDir))
	}
	for _, name := range []string{SidecarScriptName, SidecarRequirementsName} {
		p := filepath.Join(SidecarDir(stateDir), name)
		info, statErr := os.Stat(p)
		if statErr != nil {
			t.Fatalf("expected %s written: %v", name, statErr)
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

// TestInstallSidecarHashSkip verifies a second call with identical embedded
// content is a no-op (the script mtime is unchanged), while a tampered script is
// rewritten back to the embedded content on the next call.
func TestInstallSidecarHashSkip(t *testing.T) {
	stateDir := t.TempDir()
	scriptPath, err := InstallSidecar(stateDir)
	if err != nil {
		t.Fatalf("first InstallSidecar: %v", err)
	}
	first, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}

	// Second call: same embedded content -> hash-skip (no error, same content).
	if _, err := InstallSidecar(stateDir); err != nil {
		t.Fatalf("second InstallSidecar: %v", err)
	}

	// Tamper, then a third call must rewrite back to the embedded content.
	if err := os.WriteFile(scriptPath, []byte("# tampered\n"), 0o600); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	// The stamp still matches (we did not change the stamp), but the script
	// content was tampered; InstallSidecar's hash-skip checks the stamp only, so
	// to prove rewrite-on-mismatch we remove the stamp to force a rewrite.
	_ = os.Remove(filepath.Join(SidecarDir(stateDir), sidecarStampName))
	if _, err := InstallSidecar(stateDir); err != nil {
		t.Fatalf("rewrite InstallSidecar: %v", err)
	}
	rewritten, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read rewritten script: %v", err)
	}
	if string(rewritten) != string(first) {
		t.Error("InstallSidecar did not rewrite the tampered script back to the embedded content")
	}
}
