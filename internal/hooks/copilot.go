package hooks

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/home-beekeeper/beekeeper/internal/editorinit"
)

// Beekeeper's hook command string for GitHub Copilot.
const copilotPreCommand = "beekeeper check --hook copilot"

// copilotSettingsPath returns the path to Copilot's settings.json.
// Copilot uses ~/.copilot/settings.json by default; the COPILOT_HOME
// environment variable can override the parent directory but the installer
// uses the default path. Project-local hooks (.github/hooks/*.json) are an
// alternative, but the global user-level file is the canonical install target.
func copilotSettingsPath(homeDir string) string {
	return homeDir + "/.copilot/settings.json"
}

// installCopilot merges Beekeeper's preToolUse hook into the Copilot
// settings.json at settingsPath WITHOUT disturbing any pre-existing hooks or
// other top-level settings keys.
//
// Copilot's event key is "preToolUse" (camelCase). This is correct for
// Copilot — do not confuse with the Cursor bug (Cursor has different event
// names: beforeShellExecution etc.). Copilot uses exactly "preToolUse".
//
// The deny JSON itself is rendered at runtime by RenderDeny(HarnessCopilot)
// as the FLAT permissionDecision form:
//
//	{"permissionDecision":"deny","permissionDecisionReason":"..."}
//
// The installer only wires the command; the runtime deny shape is delegated
// to RenderDeny. Copilot also honors exit 2 as a deny.
//
// The merge-not-clobber trinity (mergeClaudeHookEntry, removeClaudeHookEntry,
// claudeEntriesContainCommand) is reused because Copilot uses the same
// settings.json array-of-entries schema.
//
// A backup of the existing file is created before any modification.
func installCopilot(settingsPath string, dryRun bool, out io.Writer) error {
	if dryRun {
		hooksConfig := map[string]any{
			"preToolUse": []any{beekeeperClaudePreEntryWith(copilotPreCommand)},
		}
		data, _ := json.MarshalIndent(hooksConfig, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would merge into %s (hooks key — existing hooks preserved):\n%s\n", settingsPath, string(data))
		return nil
	}

	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("install copilot: parse %q: %w", settingsPath, err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	// Copilot event key is "preToolUse" (camelCase — correct for Copilot).
	hooks["preToolUse"] = mergeClaudeHookEntry(hooks["preToolUse"], copilotPreCommand, beekeeperClaudePreEntryWith(copilotPreCommand))

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	// PatchSettings sets ONLY the "hooks" key, preserving all other top-level
	// settings keys.
	return editorinit.PatchSettings(settingsPath, "hooks", hooks)
}

// uninstallCopilot removes ONLY Beekeeper's entries from the Copilot
// settings.json, preserving all other hooks and top-level keys.
// A backup is created before modification.
func uninstallCopilot(settingsPath string, dryRun bool, out io.Writer) error {
	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("uninstall copilot: parse %q: %w", settingsPath, err)
	}

	if len(settings) == 0 {
		fmt.Fprintf(out, "No Copilot settings.json found at %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok || hooks == nil {
		fmt.Fprintf(out, "No hooks key found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	preArr, removedPre := removeClaudeHookEntry(hooks["preToolUse"], copilotPreCommand)
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
		delete(hooks, "preToolUse")
	} else {
		hooks["preToolUse"] = preArr
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	out2, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return fmt.Errorf("uninstall copilot: marshal: %w", err)
	}

	return writeFileAtomic(settingsPath, out2)
}
