package platform

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestStateDirHonorsBeekeeperHomeOverride verifies the Wave 0 / BTEST-03
// hermetic E2E requirement: when BEEKEEPER_HOME is set, StateDir and all
// derived dirs resolve under it rather than the OS default.
func TestStateDirHonorsBeekeeperHomeOverride(t *testing.T) {
	base := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", base)

	stateDir, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir() returned error: %v", err)
	}
	wantState := filepath.Join(base, "beekeeper")
	if stateDir != wantState {
		t.Fatalf("StateDir() = %q, want %q", stateDir, wantState)
	}

	// AuditDir must resolve under the overridden state dir.
	auditDir, err := AuditDir()
	if err != nil {
		t.Fatalf("AuditDir() returned error: %v", err)
	}
	wantAudit := filepath.Join(base, "beekeeper", "audit")
	if auditDir != wantAudit {
		t.Fatalf("AuditDir() = %q, want %q", auditDir, wantAudit)
	}

	// CatalogDir must also resolve under the overridden state dir.
	catalogDir, err := CatalogDir()
	if err != nil {
		t.Fatalf("CatalogDir() returned error: %v", err)
	}
	wantCatalog := filepath.Join(base, "beekeeper", "catalogs")
	if catalogDir != wantCatalog {
		t.Fatalf("CatalogDir() = %q, want %q", catalogDir, wantCatalog)
	}

	// ConfigPath must also resolve under the overridden state dir.
	cfgPath, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() returned error: %v", err)
	}
	wantConfig := filepath.Join(base, "beekeeper", "config.json")
	if cfgPath != wantConfig {
		t.Fatalf("ConfigPath() = %q, want %q", cfgPath, wantConfig)
	}
}

func TestStateDirReturnsExpectedSuffix(t *testing.T) {
	dir, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir() returned error: %v", err)
	}
	if dir == "" {
		t.Fatal("StateDir() returned empty path")
	}

	base := filepath.Base(dir)
	if runtime.GOOS == "windows" {
		// %APPDATA%\beekeeper — final element must be "beekeeper".
		if base != "beekeeper" {
			t.Fatalf("on windows StateDir() should end with %q, got %q (full: %q)",
				"beekeeper", base, dir)
		}
	} else {
		// ~/.beekeeper — final element must be ".beekeeper" and NOT under
		// an XDG ~/.config path.
		if base != ".beekeeper" {
			t.Fatalf("on %s StateDir() should end with %q, got %q (full: %q)",
				runtime.GOOS, ".beekeeper", base, dir)
		}
		if strings.Contains(dir, filepath.Join(".config", "beekeeper")) {
			t.Fatalf("StateDir() should not resolve under ~/.config on %s: %q",
				runtime.GOOS, dir)
		}
	}
}

func TestCatalogDirUnderStateDir(t *testing.T) {
	state, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir() returned error: %v", err)
	}
	catalog, err := CatalogDir()
	if err != nil {
		t.Fatalf("CatalogDir() returned error: %v", err)
	}

	want := filepath.Join(state, "catalogs")
	if catalog != want {
		t.Fatalf("CatalogDir() = %q, want %q", catalog, want)
	}
}

func TestAuditDirUnderStateDir(t *testing.T) {
	state, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir() returned error: %v", err)
	}
	audit, err := AuditDir()
	if err != nil {
		t.Fatalf("AuditDir() returned error: %v", err)
	}

	want := filepath.Join(state, "audit")
	if audit != want {
		t.Fatalf("AuditDir() = %q, want %q", audit, want)
	}
}

func TestConfigPathUnderStateDir(t *testing.T) {
	state, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir() returned error: %v", err)
	}
	cfg, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() returned error: %v", err)
	}

	want := filepath.Join(state, "config.json")
	if cfg != want {
		t.Fatalf("ConfigPath() = %q, want %q", cfg, want)
	}
}
