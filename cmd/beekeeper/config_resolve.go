// config_resolve.go — shared layered config resolver for all enforcement commands.
//
// resolveConfig, systemConfigPath, and discoverProjectConfig are used by
// newCheckCmd, newGatewayCmd, newWatchCmd, and newScanCmd to ensure the full
// CODE-05 layer set (system→user→project→BEEKEEPER_*→flags) applies to every
// enforcement decision — not just diag. Previously these functions lived in
// diag.go and were only invoked by the diag command (CODE-05 SC2 gap).
//
// Architecture constraint: this file contains only thin wiring helpers. All
// config business logic lives in internal/config.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mzansi-agentive/beekeeper/internal/config"
	"github.com/mzansi-agentive/beekeeper/internal/platform"
)

// resolveConfig builds LayerOpts for LoadLayered from platform paths plus the
// caller's environment and applies all five CODE-05 config layers. It is shared
// by all enforcement commands (check, gateway, watch, scan) so a project-level
// .beekeeper/config.json overrides user config WITHOUT requiring BEEKEEPER_* env
// vars (SC2).
//
// Fail-closed semantics: a corrupt user config returns an error (caller must
// propagate it — RunCheck, gateway.Start, etc. are never reached with a broken
// config).
func resolveConfig(cmd *cobra.Command) (config.Config, error) {
	userPath, err := platform.ConfigPath()
	if err != nil {
		return config.Config{}, fmt.Errorf("resolve config path: %w", err)
	}

	// Full CODE-05 layer set: system → user → project → BEEKEEPER_* env → flags.
	// SystemPath and ProjectPath are skipped silently by LoadLayered when absent,
	// so the project layer overrides the user layer WITHOUT requiring env vars
	// (SC2). Environ MUST be os.Environ() in production or the BEEKEEPER_* layer
	// is silently dead.
	opts := config.LayerOpts{
		SystemPath:  systemConfigPath(),
		UserPath:    userPath,
		ProjectPath: discoverProjectConfig(userPath),
		Environ:     os.Environ(),
	}

	cfg, err := config.LoadLayered(opts)
	if err != nil {
		return config.Config{}, fmt.Errorf("load layered config: %w", err)
	}
	return cfg, nil
}

// resolveConfigWithPaths builds LayerOpts for LoadLayered from the provided
// userPath and optional projectPath, plus the given environ slice. This variant
// is intended for testing only — it avoids touching real platform paths or
// os.Environ() so tests are deterministic.
func resolveConfigWithPaths(userPath, projectPath string, environ []string) (config.Config, error) {
	opts := config.LayerOpts{
		SystemPath:  systemConfigPath(),
		UserPath:    userPath,
		ProjectPath: projectPath,
		Environ:     environ,
	}
	cfg, err := config.LoadLayered(opts)
	if err != nil {
		return config.Config{}, fmt.Errorf("load layered config: %w", err)
	}
	return cfg, nil
}

// systemConfigPath returns the system-wide config path per PRD §9
// (/etc/beekeeper/config.json). On platforms without /etc the file simply does
// not exist and LoadLayered skips the layer silently.
func systemConfigPath() string {
	return filepath.Join("/etc", "beekeeper", "config.json")
}

// discoverProjectConfig walks up from the current working directory (git-style)
// looking for a .beekeeper/config.json, returning the first match. Returns ""
// when none is found, which makes LoadLayered skip the project layer. This is
// what lets a project-level config override user config without env vars (SC2).
//
// The user-level config (userPath, typically ~/.beekeeper/config.json) is never
// returned as a project config, and the walk stops at the home directory so it
// does not climb into other users' homes or the filesystem root.
func discoverProjectConfig(userPath string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	home, _ := os.UserHomeDir()
	for {
		candidate := filepath.Join(dir, ".beekeeper", "config.json")
		if candidate != userPath {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate
			}
		}
		if home != "" && dir == home {
			return "" // do not search at or above the user's home directory
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root without a match
		}
		dir = parent
	}
}
