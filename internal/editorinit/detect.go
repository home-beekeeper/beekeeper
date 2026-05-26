// Package editorinit provides editor detection and settings patching for the
// beekeeper editor-extension defense subsystem (Phase 3).
package editorinit

import (
	"os"
	"path/filepath"
	"runtime"
)

// Editor describes a detected code editor installation.
type Editor struct {
	// Name is the human-readable editor name (e.g. "VS Code", "Cursor").
	Name string
	// Executable is the resolved path returned by lookPath, or "" if the
	// executable was not found on PATH (editor was detected via extension dir).
	Executable string
	// ExecutableFound is true when lookPath succeeded for at least one alias.
	ExecutableFound bool
	// ExtensionDir is the platform-specific extension directory path.
	ExtensionDir string
	// SettingsPath is the platform-specific user settings.json path.
	SettingsPath string
}

// Injectable stubs for testing — default to real OS functions.
var lookPath = defaultLookPath
var statFunc = os.Stat

// defaultLookPath wraps os.Stat-based resolution via exec.LookPath; declared
// as a named wrapper so tests can replace lookPath without importing os/exec.
func defaultLookPath(name string) (string, error) {
	// Delegate to the real exec.LookPath via the thin wrapper below.
	return execLookPath(name)
}

// homeDir returns the current user's home directory. Falls back to "~" on error
// so path construction never panics.
func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "~"
	}
	return h
}

// configBase returns the platform-appropriate base directory for application
// configuration files:
//
//	Windows: %APPDATA%  (via os.UserConfigDir)
//	Unix:    ~/.config  (XDG convention)
func configBase() string {
	if runtime.GOOS == "windows" {
		base, err := os.UserConfigDir() // %APPDATA% on Windows
		if err != nil {
			return filepath.Join(homeDir(), "AppData", "Roaming")
		}
		return base
	}
	return filepath.Join(homeDir(), ".config")
}

// editorDescriptor is a static entry in the known-editors table.
type editorDescriptor struct {
	name         string
	executables  []string
	extensionDir string
	settingsPath string
}

// knownEditors returns the static table of supported editors with
// platform-aware paths. It is evaluated at call time (not package init) so
// homeDir() and configBase() always reflect the actual runtime environment.
func knownEditors() []editorDescriptor {
	home := homeDir()
	cfg := configBase()

	return []editorDescriptor{
		{
			name:        "VS Code",
			executables: []string{"code", "code-insiders", "codium"},
			extensionDir: filepath.Join(home, ".vscode", "extensions"),
			settingsPath: filepath.Join(cfg, "Code", "User", "settings.json"),
		},
		{
			// Assumption A1 (LOW confidence): mirrors VS Code convention; needs empirical validation
			name:        "Cursor",
			executables: []string{"cursor"},
			extensionDir: filepath.Join(home, ".cursor", "extensions"),
			settingsPath: filepath.Join(cfg, "Cursor", "User", "settings.json"),
		},
		{
			name:        "Windsurf",
			executables: []string{"windsurf"},
			extensionDir: filepath.Join(home, ".windsurf", "extensions"),
			settingsPath: filepath.Join(cfg, "Windsurf", "User", "settings.json"),
		},
	}
}

// DetectEditors probes the current system for installed code editors and
// returns one Editor entry for each editor that is detected either by
// finding its executable on PATH or by finding its extension directory on
// disk. An editor absent from both probes is excluded from the results.
//
// The lookPath and statFunc variables are package-level and may be swapped
// in tests for hermetic behaviour.
func DetectEditors() ([]Editor, error) {
	descriptors := knownEditors()
	var result []Editor

	for _, d := range descriptors {
		var execPath string
		execFound := false

		// Try each executable alias; use the first one that resolves.
		for _, alias := range d.executables {
			p, err := lookPath(alias)
			if err == nil {
				execPath = p
				execFound = true
				break
			}
		}

		// Check whether the extension directory exists on disk.
		dirExists := false
		if _, err := statFunc(d.extensionDir); err == nil {
			dirExists = true
		}

		// Include the editor if either the executable or extension dir is found.
		if !execFound && !dirExists {
			continue
		}

		result = append(result, Editor{
			Name:            d.name,
			Executable:      execPath,
			ExecutableFound: execFound,
			ExtensionDir:    d.extensionDir,
			SettingsPath:    d.settingsPath,
		})
	}

	return result, nil
}
