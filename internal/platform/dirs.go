// Package platform provides cross-platform primitives for locating the
// Beekeeper state directory and enforcing owner-only file permissions.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// StateDir returns the Beekeeper state directory:
//
//	BEEKEEPER_HOME set: $BEEKEEPER_HOME/beekeeper  (hermetic E2E / test isolation)
//	Windows (default): %APPDATA%\beekeeper          (via os.UserConfigDir)
//	Unix (default):    ~/.beekeeper                 (via os.UserHomeDir)
//
// The BEEKEEPER_HOME env override is checked first so the live-binary E2E test
// suite (BTEST-03) can redirect state/audit/catalogs to a temp directory without
// modifying the real user state. When unset or empty, the prior OS-specific
// resolution is completely unchanged.
//
// Because CatalogDir, AuditDir, and ConfigPath all derive from StateDir, setting
// BEEKEEPER_HOME redirects all of them together — exactly what hermetic E2E runs
// need (Wave 0 blocker A2 closed).
//
// On Unix we deliberately use os.UserHomeDir rather than os.UserConfigDir:
// os.UserConfigDir would resolve to ~/.config/beekeeper, which is NOT the
// ~/.beekeeper convention the project mandates.
func StateDir() (string, error) {
	// Wave 0 (BTEST-03): BEEKEEPER_HOME overrides the OS default so hermetic
	// E2E runs can redirect the entire state tree to a temp directory.
	if h := os.Getenv("BEEKEEPER_HOME"); h != "" {
		return filepath.Join(h, "beekeeper"), nil
	}

	if runtime.GOOS == "windows" {
		base, err := os.UserConfigDir() // %APPDATA% on Windows
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "beekeeper"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".beekeeper"), nil
}

// CatalogDir returns the cached-catalog directory under the state directory.
func CatalogDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "catalogs"), nil
}

// AuditDir returns the NDJSON audit-log directory under the state directory.
func AuditDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "audit"), nil
}

// ConfigPath returns the path to the Beekeeper config file under the state
// directory.
func ConfigPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}
