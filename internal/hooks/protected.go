package hooks

// Package hooks — protected.go
//
// Exposes the set of harness hook-config files and the Beekeeper hook markers so
// the self-protection guard (internal/check) can detect when an agent tool call
// would remove or disable Beekeeper's own hook entry. This is read-only metadata;
// no install/uninstall behavior lives here.

import "path/filepath"

// BeekeeperHookMarkers returns the substrings that identify a Beekeeper hook
// entry inside a harness hook-config file. Every per-harness PreToolUse command
// is "beekeeper check --hook <harness>", so "beekeeper check" matches all of
// them; "beekeeper audit-record" matches the PostToolUse entry. A file that
// contains any of these is considered to have Beekeeper installed.
func BeekeeperHookMarkers() []string {
	return []string{"beekeeper check", "beekeeper audit-record"}
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
