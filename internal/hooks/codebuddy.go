package hooks

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/home-beekeeper/beekeeper/internal/editorinit"
)

// Beekeeper's hook command strings for CodeBuddy.
const (
	codebuddyPreCommand  = "beekeeper check --hook codebuddy"
	codebuddyPostCommand = "beekeeper audit-record"
)

// codebuddySettingsPath returns the path to CodeBuddy's settings.json.
func codebuddySettingsPath(homeDir string) string {
	return homeDir + "/.codebuddy/settings.json"
}

// installCodeBuddy merges Beekeeper's PreToolUse and PostToolUse hooks into the
// CodeBuddy settings.json at settingsPath WITHOUT disturbing any pre-existing
// hooks or other top-level settings keys.
//
// CodeBuddy is a Claude Code clone that uses the same nested hookSpecificOutput
// schema: settings.json with a "hooks" key containing "PreToolUse" and
// "PostToolUse" event arrays. The merge-not-clobber trinity is reused directly.
//
// A backup of the existing file is created before any modification.
func installCodeBuddy(settingsPath string, dryRun bool, out io.Writer) error {
	if dryRun {
		hooksConfig := map[string]any{
			"PreToolUse":  []any{beekeeperClaudePreEntryWith(codebuddyPreCommand)},
			"PostToolUse": []any{beekeeperClaudePostEntryWith(codebuddyPostCommand)},
		}
		data, _ := json.MarshalIndent(hooksConfig, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would merge into %s (hooks key — existing hooks preserved):\n%s\n", settingsPath, string(data))
		return nil
	}

	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("install codebuddy: parse %q: %w", settingsPath, err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	hooks["PreToolUse"] = mergeClaudeHookEntry(hooks["PreToolUse"], codebuddyPreCommand, beekeeperClaudePreEntryWith(codebuddyPreCommand))
	hooks["PostToolUse"] = mergeClaudeHookEntry(hooks["PostToolUse"], codebuddyPostCommand, beekeeperClaudePostEntryWith(codebuddyPostCommand))

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	return editorinit.PatchSettings(settingsPath, "hooks", hooks)
}

// uninstallCodeBuddy removes ONLY Beekeeper's entries from CodeBuddy's
// settings.json, preserving all other hooks and top-level keys.
// A backup is created before modification.
func uninstallCodeBuddy(settingsPath string, dryRun bool, out io.Writer) error {
	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("uninstall codebuddy: parse %q: %w", settingsPath, err)
	}

	if len(settings) == 0 {
		fmt.Fprintf(out, "No CodeBuddy settings.json found at %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok || hooks == nil {
		fmt.Fprintf(out, "No hooks key found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	preArr, removedPre := removeClaudeHookEntry(hooks["PreToolUse"], codebuddyPreCommand)
	postArr, removedPost := removeClaudeHookEntry(hooks["PostToolUse"], codebuddyPostCommand)
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
		return fmt.Errorf("uninstall codebuddy: marshal: %w", err)
	}

	return writeFileAtomic(settingsPath, out2)
}
