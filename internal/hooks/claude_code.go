package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mzansi-agentive/beekeeper/internal/editorinit"
)

// claudeHookConfig builds the hooks map for Claude Code's settings.json.
// The schema is verified from code.claude.com/docs/en/hooks.
func claudeHookConfig() map[string]any {
	return map[string]any{
		"PreToolUse": []any{
			map[string]any{
				"matcher": ".*",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "beekeeper check",
					},
				},
			},
		},
		"PostToolUse": []any{
			map[string]any{
				"matcher": ".*",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "beekeeper audit-record",
					},
				},
			},
		},
	}
}

// installClaudeCode merges Beekeeper's PreToolUse and PostToolUse hooks into
// the Claude Code settings.json at settingsPath.
//
// It uses editorinit.PatchSettings which is JSONC-safe (strips comments on
// read, atomic write). The function is idempotent: PatchSettings overwrites the
// "hooks" key with the canonical value — re-running produces identical output.
//
// A backup of the existing file is created before any modification.
func installClaudeCode(settingsPath string, dryRun bool, out io.Writer) error {
	hookConfig := claudeHookConfig()

	if dryRun {
		data, _ := json.MarshalIndent(hookConfig, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would write to %s (hooks key):\n%s\n", settingsPath, string(data))
		return nil
	}

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	return editorinit.PatchSettings(settingsPath, "hooks", hookConfig)
}

// uninstallClaudeCode removes the "hooks" key from the Claude Code settings.json.
// Other keys in the file are preserved. A backup is created before modification.
func uninstallClaudeCode(settingsPath string, dryRun bool, out io.Writer) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Claude Code settings.json found at %s — nothing to uninstall.\n", settingsPath)
			return nil
		}
		return fmt.Errorf("uninstall claude-code: read %q: %w", settingsPath, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("uninstall claude-code: parse %q: %w", settingsPath, err)
	}

	if _, ok := settings["hooks"]; !ok {
		fmt.Fprintf(out, "No hooks key found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove hooks key from %s\n", settingsPath)
		return nil
	}

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	delete(settings, "hooks")

	out2, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return fmt.Errorf("uninstall claude-code: marshal: %w", err)
	}

	return writeFileAtomic(settingsPath, out2)
}
