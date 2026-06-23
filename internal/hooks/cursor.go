package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// cursorHooksFile is the top-level structure of ~/.cursor/hooks.json.
// Schema verified from cursor.com/docs/hooks (v1.7+).
type cursorHooksFile struct {
	Version int                     `json:"version"`
	Hooks   map[string][]cursorHook `json:"hooks"`
}

// cursorHook is a single hook entry within a Cursor event array.
// Cursor v1.7+ uses three separate event keys (beforeShellExecution,
// beforeMCPExecution, beforeReadFile) — NOT the non-existent "preToolUse".
type cursorHook struct {
	Command    string `json:"command"`
	Type       string `json:"type"`
	Timeout    int    `json:"timeout,omitempty"`
	Matcher    string `json:"matcher,omitempty"`
	FailClosed bool   `json:"failClosed,omitempty"`
}

// cursorEvents lists the three real Cursor v1.7+ hook event names.
// "preToolUse" does NOT exist in Cursor — the installer must NEVER write that key.
var cursorEvents = []string{
	"beforeShellExecution",
	"beforeMCPExecution",
	"beforeReadFile",
}

// cursorCheckSuffix is the stable suffix for Cursor's beekeeper hook command.
// Used for idempotent install, migration, and targeted uninstall.
const cursorCheckSuffix = "check --hook cursor"

// containsCursorHookByCommand reports whether any hook in the slice has
// a Command that matches the beekeeper cursor stable suffix.
// Matches BOTH old bare-name ("beekeeper check --hook cursor") and new abspath
// forms via matchesBeekeeperCommand.
func containsCursorHookByCommand(hooks []cursorHook, suffix string) bool {
	for _, h := range hooks {
		if matchesBeekeeperCommand(h.Command, suffix) {
			return true
		}
	}
	return false
}

// installCursor merges Beekeeper's hooks into ~/.cursor/hooks.json for each
// of the three real Cursor v1.7+ hook events: beforeShellExecution,
// beforeMCPExecution, and beforeReadFile.
//
// The installed command embeds the running binary's absolute path (resolved via
// os.Executable at install time). Re-running migrates an old bare-name entry
// to the new abspath form in place (no duplicate).
//
// The function is idempotent: it only appends the hook to an event if no
// matching beekeeper entry (bare or abspath) is already present.
// Existing hooks from other tools are preserved.
//
// CRITICAL: FailClosed must be true. If the hook crashes or times out, Cursor
// must block the tool call (fail-closed), not allow it (fail-open). Cursor
// defaults to fail-OPEN, so this field is required to prevent silent bypass.
//
// The path written to is ALWAYS ~/.cursor/hooks.json — never the editor
// preferences file.
func installCursor(hooksPath string, dryRun bool, out io.Writer) error {
	existing := cursorHooksFile{
		Version: 1,
		Hooks:   make(map[string][]cursorHook),
	}

	if data, err := os.ReadFile(hooksPath); err == nil {
		// Tolerate parse errors — start fresh if the file is malformed.
		_ = json.Unmarshal(data, &existing)
		if existing.Hooks == nil {
			existing.Hooks = make(map[string][]cursorHook)
		}
		if existing.Version == 0 {
			existing.Version = 1
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cursor: read %q: %w", hooksPath, err)
	}

	// Build the freshly-resolved abspath command.
	newCmd := beekeeperCmd(cursorCheckSuffix)
	newHook := cursorHook{
		Command:    newCmd,
		Type:       "command",
		Timeout:    10,
		Matcher:    ".*",
		FailClosed: true, // REQUIRED: Cursor is fail-OPEN by default
	}

	// Append/migrate beekeeper hook to each of the three real Cursor v1.7+ events.
	for _, event := range cursorEvents {
		arr := existing.Hooks[event]
		found := false
		migrated := make([]cursorHook, 0, len(arr)+1)
		for _, h := range arr {
			if matchesBeekeeperCommand(h.Command, cursorCheckSuffix) {
				found = true
				if h.Command != newCmd {
					// Migrate stale bare-name or stale-abspath entry to new abspath.
					migrated = append(migrated, newHook)
				} else {
					migrated = append(migrated, h)
				}
			} else {
				migrated = append(migrated, h)
			}
		}
		if !found {
			migrated = append(migrated, newHook)
		}
		existing.Hooks[event] = migrated
	}

	if dryRun {
		data, _ := json.MarshalIndent(existing, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would write to %s:\n%s\n", hooksPath, string(data))
		return nil
	}

	if err := backupSettings(hooksPath); err != nil {
		return err
	}

	// Ensure the parent directory exists (e.g., ~/.cursor/ may not exist yet).
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return fmt.Errorf("cursor: create dir %q: %w", filepath.Dir(hooksPath), err)
	}

	data, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("cursor: marshal hooks: %w", err)
	}

	return writeFileAtomic(hooksPath, data)
}

// uninstallCursor removes beekeeper hook entries from ~/.cursor/hooks.json for
// all three Cursor event keys (beforeShellExecution, beforeMCPExecution,
// beforeReadFile). Suffix matching covers BOTH old bare-name and new abspath
// forms. Other hooks are preserved. A backup is created first.
func uninstallCursor(hooksPath string, dryRun bool, out io.Writer) error {
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Cursor hooks.json found at %s — nothing to uninstall.\n", hooksPath)
			return nil
		}
		return fmt.Errorf("cursor uninstall: read %q: %w", hooksPath, err)
	}

	var existing cursorHooksFile
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("cursor uninstall: parse %q: %w", hooksPath, err)
	}
	if existing.Hooks == nil {
		existing.Hooks = make(map[string][]cursorHook)
	}

	totalRemoved := 0

	// Iterate all three real Cursor event keys, filtering beekeeper entries by suffix.
	for _, event := range cursorEvents {
		arr := existing.Hooks[event]
		filtered := make([]cursorHook, 0, len(arr))
		for _, h := range arr {
			if matchesBeekeeperCommand(h.Command, cursorCheckSuffix) {
				totalRemoved++
				continue
			}
			filtered = append(filtered, h)
		}
		existing.Hooks[event] = filtered
	}

	if totalRemoved == 0 {
		fmt.Fprintf(out, "No beekeeper check hooks found in %s — nothing to uninstall.\n", hooksPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove %d beekeeper check hook(s) from all events in %s\n", totalRemoved, hooksPath)
		return nil
	}

	if err := backupSettings(hooksPath); err != nil {
		return err
	}

	out2, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("cursor uninstall: marshal: %w", err)
	}

	return writeFileAtomic(hooksPath, out2)
}
