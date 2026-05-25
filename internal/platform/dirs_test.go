package platform

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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
