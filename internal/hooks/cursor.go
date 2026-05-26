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
// Schema verified from cursor.com/docs/hooks.
type cursorHooksFile struct {
	Version int                      `json:"version"`
	Hooks   map[string][]cursorHook  `json:"hooks"`
}

// cursorHook is a single hook entry in Cursor's preToolUse array.
// Note: the key is "preToolUse" (camelCase) — different from Claude Code's
// "PreToolUse" (PascalCase).
type cursorHook struct {
	Command    string `json:"command"`
	Type       string `json:"type"`
	Timeout    int    `json:"timeout,omitempty"`
	Matcher    string `json:"matcher,omitempty"`
	FailClosed bool   `json:"failClosed,omitempty"`
}

// containsCursorHookByCommand reports whether any hook in the slice has
// Command equal to cmd. Used to prevent duplicate entries (idempotency).
func containsCursorHookByCommand(hooks []cursorHook, cmd string) bool {
	for _, h := range hooks {
		if h.Command == cmd {
			return true
		}
	}
	return false
}

// installCursor merges Beekeeper's preToolUse hook into ~/.cursor/hooks.json.
//
// The function is idempotent: it only appends the hook if "beekeeper check" is
// not already present in the preToolUse array. Existing hooks from other tools
// are preserved.
//
// CRITICAL: failClosed must be true. If the hook crashes or times out, Cursor
// must block the tool call (fail-closed), not allow it (fail-open).
// The path written to is ALWAYS ~/.cursor/hooks.json — never the editor preferences
// file. See RESEARCH Pattern 2 and Anti-Patterns section for rationale.
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

	beekeeperHook := cursorHook{
		Command:    "beekeeper check",
		Type:       "command",
		Timeout:    10,
		Matcher:    ".*",
		FailClosed: true, // REQUIRED: fail-closed on hook failure
	}

	// Idempotent: only append if the beekeeper check hook is not already present.
	if !containsCursorHookByCommand(existing.Hooks["preToolUse"], "beekeeper check") {
		existing.Hooks["preToolUse"] = append(existing.Hooks["preToolUse"], beekeeperHook)
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

// uninstallCursor removes the "beekeeper check" entry from ~/.cursor/hooks.json's
// preToolUse array. Other hooks are preserved. A backup is created first.
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

	preToolUse := existing.Hooks["preToolUse"]
	filtered := make([]cursorHook, 0, len(preToolUse))
	removed := 0
	for _, h := range preToolUse {
		if h.Command == "beekeeper check" {
			removed++
			continue
		}
		filtered = append(filtered, h)
	}

	if removed == 0 {
		fmt.Fprintf(out, "No beekeeper check hooks found in %s — nothing to uninstall.\n", hooksPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove %d beekeeper check hook(s) from preToolUse in %s\n", removed, hooksPath)
		return nil
	}

	if err := backupSettings(hooksPath); err != nil {
		return err
	}

	existing.Hooks["preToolUse"] = filtered

	out2, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("cursor uninstall: marshal: %w", err)
	}

	return writeFileAtomic(hooksPath, out2)
}
