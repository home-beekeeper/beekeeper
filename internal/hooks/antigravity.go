package hooks

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/home-beekeeper/beekeeper/internal/editorinit"
)

// Beekeeper's hook command string for Antigravity (Google).
const antigravityPreCommand = "beekeeper check --hook antigravity"

// antigravitySettingsPath returns the path to Antigravity's hooks.json.
// The primary location is ~/.gemini/antigravity/hooks.json.
// An alternative project-local location is .agents/hooks.json (project root),
// and some documentation also references ~/.gemini/config/hooks.json.
// This installer targets the global user-level file.
func antigravitySettingsPath(homeDir string) string {
	return homeDir + "/.gemini/antigravity/hooks.json"
}

// installAntigravity merges Beekeeper's PreToolUse hook into the Antigravity
// hooks.json at settingsPath WITHOUT disturbing any pre-existing hooks or
// other top-level settings keys.
//
// Antigravity uses the PreToolUse event. The deny-field name is MED-confidence
// (Antigravity docs are ambiguous between "decision":"deny",
// "permissionDecision":"deny"+denyReason, and SDK allow:false+denyReason).
// To handle all variants, the runtime RenderDeny(HarnessAntigravity) emits
// BOTH the decision:"deny" AND the permissionDecision:"deny"+denyReason fields
// defensively. Exit 2 also blocks. The installer only wires the command.
//
// The merge-not-clobber trinity from claude_code.go is reused because
// Antigravity uses the same settings.json array-of-entries schema as Claude Code.
//
// A backup of the existing file is created before any modification.
func installAntigravity(settingsPath string, dryRun bool, out io.Writer) error {
	if dryRun {
		hooksConfig := map[string]any{
			"PreToolUse": []any{beekeeperClaudePreEntryWith(antigravityPreCommand)},
		}
		data, _ := json.MarshalIndent(hooksConfig, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would merge into %s (hooks key — existing hooks preserved):\n%s\n", settingsPath, string(data))
		return nil
	}

	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("install antigravity: parse %q: %w", settingsPath, err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	hooks["PreToolUse"] = mergeClaudeHookEntry(hooks["PreToolUse"], antigravityPreCommand, beekeeperClaudePreEntryWith(antigravityPreCommand))

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	// PatchSettings sets ONLY the "hooks" key, preserving all other top-level
	// settings keys.
	return editorinit.PatchSettings(settingsPath, "hooks", hooks)
}

// uninstallAntigravity removes ONLY Beekeeper's entries from the Antigravity
// hooks.json, preserving all other hooks and top-level keys.
// A backup is created before modification.
func uninstallAntigravity(settingsPath string, dryRun bool, out io.Writer) error {
	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("uninstall antigravity: parse %q: %w", settingsPath, err)
	}

	if len(settings) == 0 {
		fmt.Fprintf(out, "No Antigravity hooks.json found at %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok || hooks == nil {
		fmt.Fprintf(out, "No hooks key found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	preArr, removedPre := removeClaudeHookEntry(hooks["PreToolUse"], antigravityPreCommand)
	removed := removedPre

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

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	out2, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return fmt.Errorf("uninstall antigravity: marshal: %w", err)
	}

	return writeFileAtomic(settingsPath, out2)
}
