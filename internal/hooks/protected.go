package hooks

// Package hooks — protected.go
//
// Exposes the set of harness hook-config files and the Beekeeper hook markers so
// the self-protection guard (internal/check) can detect when an agent tool call
// would remove or disable Beekeeper's own hook entry. This is read-only metadata;
// no install/uninstall behavior lives here.

import "path/filepath"

// BeekeeperHookMarkers returns the substrings that identify a Beekeeper hook
// entry inside a harness hook-config file. A file that contains any of these
// is considered to have Beekeeper installed.
//
// Two forms are supported:
//
//   - Bare-name form (pre-v1.5.0): the installed command is the literal
//     "beekeeper check --hook <harness>" / "beekeeper audit-record". The raw
//     bytes contain "beekeeper check" / "beekeeper audit-record".
//
//   - Abspath form (v1.5.0+): the installed command is
//     '"<absolute-path>" check --hook <harness>'. JSON-encoded, the raw bytes
//     contain 'beekeeper\" check' (escaped inner quote). YAML/shell raw bytes
//     contain 'beekeeper" check'. Both forms contain "check --hook" which is
//     the stable cross-encoding marker.
//
// The markers include both forms so the self-protection guard (internal/check)
// recognises hook-config files regardless of which installer generation wrote
// the entry. "beekeeper check" / "beekeeper audit-record" remain in the set
// for backward compatibility with bare-name form entries already on disk.
func BeekeeperHookMarkers() []string {
	return []string{
		// Bare-name form (pre-v1.5.0 installs):
		"beekeeper check",
		"beekeeper audit-record",
		// Abspath form (v1.5.0+ installs) — invariant suffixes present in raw
		// bytes of EVERY format (JSON-escaped, YAML, shell script):
		"check --hook",
		"audit-record",
	}
}

// HookConfigFiles returns the set of hook-config file paths that Beekeeper may
// install its hook into, rooted at homeDir. The content-aware self-protection
// guard uses this list to decide whether a tool-call target is a hook-config
// file worth inspecting. Paths are returned for every supported harness
// regardless of whether it is installed (a path that does not exist simply never
// matches a real tool call). All helpers used here are cross-platform; the Cline
// PreToolUse path is composed from the cross-platform clineHooksDir to avoid the
// !windows-only clinePreToolUsePath.
func HookConfigFiles(homeDir string) []string {
	return []string{
		claudeSettingsPath(homeDir),
		cursorHooksPath(homeDir),
		codexHooksPath(homeDir),
		codexConfigPath(homeDir),
		augmentSettingsPath(homeDir),
		codebuddySettingsPath(homeDir),
		qwenSettingsPath(homeDir),
		copilotSettingsPath(homeDir),
		antigravitySettingsPath(homeDir),
		geminiSettingsPath(homeDir),
		windsurfHooksPath(homeDir),
		hermesConfigPath(homeDir),
		filepath.Join(clineHooksDir(homeDir), "PreToolUse"),
		openCodePluginPath(openCodePluginDir(homeDir)),
	}
}
