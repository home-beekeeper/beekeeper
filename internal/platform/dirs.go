// Package platform provides cross-platform primitives for locating the
// Beekeeper state directory and enforcing owner-only file permissions.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
	//
	// SECURITY (remediation 260615, finding #1): BEEKEEPER_HOME relocates the
	// ENTIRE trust root — catalogs (the block-decision index), policies/,
	// state.json, and the audit log. If a production binary honored it, anything
	// that can influence the hook process's environment (a poisoned shell profile
	// sourced by the harness, etc.) could silently repoint enforcement at an
	// attacker-controlled tree: an empty index makes every package "allow", and a
	// planted policies/ dir can flip fail_mode to open. The override is therefore
	// honored ONLY in test binaries (testing.Testing()) or when the
	// `beekeeperhomeoverride` build tag is compiled in — which the live-binary E2E
	// harness does. A normal production build ignores BEEKEEPER_HOME and uses the
	// OS default below, so the relocation primitive is not available at runtime.
	if h := os.Getenv("BEEKEEPER_HOME"); h != "" && homeOverrideAllowed() {
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

// homeOverrideAllowed reports whether the BEEKEEPER_HOME trust-root override may
// be honored in the current binary. It is true under `go test` (in-process unit
// tests that set BEEKEEPER_HOME via t.Setenv) and when the binary is built with
// the `beekeeperhomeoverride` build tag (the live-binary E2E harness). Production
// builds return false so BEEKEEPER_HOME cannot repoint enforcement. See StateDir.
func homeOverrideAllowed() bool {
	return testing.Testing() || buildTagHomeOverride
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

// LogDir returns the daemon-log directory under the state directory. The
// background catalog-sync run (`catalogs sync --background`) tees its
// human-readable output here so a scheduled run is never invisible — on Windows
// the scheduler used to flash a console that discarded the output, and macOS
// launchd discards stdout entirely.
func LogDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
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
