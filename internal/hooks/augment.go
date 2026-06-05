package hooks

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/bantuson/beekeeper/internal/editorinit"
)

// Beekeeper's hook command strings for Augment.
const (
	augmentPreCommand  = "beekeeper check --hook augment"
	augmentPostCommand = "beekeeper audit-record"
)

// augmentSettingsPath returns the path to Augment's settings.json.
func augmentSettingsPath(homeDir string) string {
	return homeDir + "/.augment/settings.json"
}

// installAugment merges Beekeeper's PreToolUse and PostToolUse hooks into the
// Augment settings.json at settingsPath WITHOUT disturbing any pre-existing
// hooks or other top-level settings keys.
//
// Augment uses the same nested hookSpecificOutput schema as Claude Code:
// settings.json with a "hooks" key containing "PreToolUse" and "PostToolUse"
// event arrays. The merge-not-clobber trinity (mergeClaudeHookEntry,
// removeClaudeHookEntry, claudeEntriesContainCommand) is reused directly.
//
// A backup of the existing file is created before any modification.
func installAugment(settingsPath string, dryRun bool, out io.Writer) error {
	if dryRun {
		hooksConfig := map[string]any{
			"PreToolUse":  []any{beekeeperClaudePreEntryWith(augmentPreCommand)},
			"PostToolUse": []any{beekeeperClaudePostEntryWith(augmentPostCommand)},
		}
		data, _ := json.MarshalIndent(hooksConfig, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would merge into %s (hooks key — existing hooks preserved):\n%s\n", settingsPath, string(data))
		return nil
	}

	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("install augment: parse %q: %w", settingsPath, err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	hooks["PreToolUse"] = mergeClaudeHookEntry(hooks["PreToolUse"], augmentPreCommand, beekeeperClaudePreEntryWith(augmentPreCommand))
	hooks["PostToolUse"] = mergeClaudeHookEntry(hooks["PostToolUse"], augmentPostCommand, beekeeperClaudePostEntryWith(augmentPostCommand))

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	return editorinit.PatchSettings(settingsPath, "hooks", hooks)
}

// uninstallAugment removes ONLY Beekeeper's entries from Augment's
// settings.json, preserving all other hooks and top-level keys.
// A backup is created before modification.
func uninstallAugment(settingsPath string, dryRun bool, out io.Writer) error {
	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("uninstall augment: parse %q: %w", settingsPath, err)
	}

	if len(settings) == 0 {
		fmt.Fprintf(out, "No Augment settings.json found at %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok || hooks == nil {
		fmt.Fprintf(out, "No hooks key found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	preArr, removedPre := removeClaudeHookEntry(hooks["PreToolUse"], augmentPreCommand)
	postArr, removedPost := removeClaudeHookEntry(hooks["PostToolUse"], augmentPostCommand)
	removed := removedPre + removedPost

	if removed == 0 {
		fmt.Fprintf(out, "No beekeeper hooks found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove %d beekeeper hook entry(ies) from %s (other hooks preserved)\n", removed, settingsPath)
		return nil
	}

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	if len(preArr) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = preArr
	}
	if len(postArr) == 0 {
		delete(hooks, "PostToolUse")
	} else {
		hooks["PostToolUse"] = postArr
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	out2, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return fmt.Errorf("uninstall augment: marshal: %w", err)
	}

	return writeFileAtomic(settingsPath, out2)
}

// beekeeperClaudePreEntryWith returns a PreToolUse hook entry using cmd as
// the command string. Reuses the Claude hook schema for harnesses that share it.
func beekeeperClaudePreEntryWith(cmd string) map[string]any {
	return map[string]any{
		"matcher": ".*",
		"hooks": []any{
			map[string]any{"type": "command", "command": cmd},
		},
	}
}

// beekeeperClaudePostEntryWith returns a PostToolUse hook entry using cmd as
// the command string.
func beekeeperClaudePostEntryWith(cmd string) map[string]any {
	return map[string]any{
		"matcher": ".*",
		"hooks": []any{
			map[string]any{"type": "command", "command": cmd},
		},
	}
}
