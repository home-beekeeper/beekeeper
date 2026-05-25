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
//	Windows: %APPDATA%\beekeeper  (via os.UserConfigDir)
//	Unix:    ~/.beekeeper         (via os.UserHomeDir)
//
// On Unix we deliberately use os.UserHomeDir rather than os.UserConfigDir:
// os.UserConfigDir would resolve to ~/.config/beekeeper, which is NOT the
// ~/.beekeeper convention the project mandates.
func StateDir() (string, error) {
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
