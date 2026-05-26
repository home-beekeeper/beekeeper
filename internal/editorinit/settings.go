package editorinit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/tidwall/jsonc"
)

// PatchSettings reads the JSON (or JSONC) file at path, sets key to value, and
// writes the result back using an atomic rename. The function is idempotent:
// calling it multiple times with the same key/value is safe and will not
// duplicate the key.
//
// NOTE: existing // and /* */ comments in the file are removed on write
// (tidwall/jsonc has no comment-preserving writer). The caller's consent
// prompt must warn the user.
//
// If the file does not exist it is created with just the single key/value pair.
// Parent directories are created with mode 0o755 if absent.
func PatchSettings(path, key string, value any) error {
	// Step 1: read existing file; treat ErrNotExist as an empty object.
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		data = []byte("{}")
	}

	// Step 2: strip JSONC comments so json.Unmarshal can handle the input.
	stripped := jsonc.ToJSON(data)

	// Step 3: unmarshal into a generic map.
	var settings map[string]any
	if err := json.Unmarshal(stripped, &settings); err != nil {
		// If the file is corrupt/empty, start fresh rather than failing.
		settings = nil
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	// Step 4: set the key (idempotent — overwrites if already present).
	settings[key] = value

	// Step 5: re-marshal with 4-space indent.
	out, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return err
	}

	// Step 6: ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Step 7: atomic write — write to a temp file then rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// ReadSettings reads the JSON or JSONC file at path and returns its content as
// a map[string]any. JSONC comments are stripped before unmarshalling so that
// files with comments (legal in Claude Code's settings.json format) are parsed
// correctly. If the file does not exist, ReadSettings returns an empty map and
// no error (consistent with PatchSettings behaviour on non-existent files).
//
// This is the JSONC-safe read helper that uninstall paths must use instead of
// a bare json.Unmarshal (WR-01).
func ReadSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]any), nil
		}
		return nil, err
	}

	stripped := jsonc.ToJSON(data)
	var settings map[string]any
	if err := json.Unmarshal(stripped, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// DisableExtensionAutoUpdate sets "extensions.autoUpdate" to false in the
// editor settings file at settingsPath. It delegates to PatchSettings and
// inherits the same JSONC-comment-stripping and atomic-write semantics.
func DisableExtensionAutoUpdate(settingsPath string) error {
	return PatchSettings(settingsPath, "extensions.autoUpdate", false)
}
