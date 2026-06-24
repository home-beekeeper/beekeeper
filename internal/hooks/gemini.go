package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// geminiHooksFile is the top-level structure of the Gemini CLI settings.json.
// Gemini CLI stores hooks in the "hooks" array within ~/.gemini/settings.json.
// Schema verified from geminicli.com/docs/hooks/reference.
type geminiHooksFile struct {
	Hooks []geminiHookEntry `json:"hooks"`
}

// geminiHookEntry is a single hook entry in the Gemini CLI hooks array.
// The Event field specifies when the hook fires (e.g. "BeforeTool"), the
// Matcher is a regex matched against the tool name, and the Command is the
// shell command to execute.
type geminiHookEntry struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Command string `json:"command"`
}

// geminiCheckSuffix is the stable suffix for Gemini CLI's beekeeper hook command.
// Used for idempotent install, migration, and targeted uninstall.
const geminiCheckSuffix = "check --hook gemini"

// geminiSettingsPath returns the path to Gemini CLI's settings.json.
// Gemini CLI stores hooks under the "hooks" array in this file.
func geminiSettingsPath(homeDir string) string {
	return homeDir + "/.gemini/settings.json"
}

// containsGeminiHookByCommand reports whether any entry in the given hook
// array has a Command matching the given stable suffix. Used for idempotent
// install and targeted uninstall.
// Matches BOTH old bare-name and new abspath forms via matchesBeekeeperCommand.
func containsGeminiHookByCommand(hooks []geminiHookEntry, suffix string) bool {
	for _, h := range hooks {
		if matchesBeekeeperCommand(h.Command, suffix) {
			return true
		}
	}
	return false
}

// beekeeperGeminiEntry returns the canonical BeforeTool hook entry for
// Gemini CLI. The Matcher ".*" matches all tool names.
// Command uses the absolute binary path via beekeeperCmd (resolved at install
// time) so that Gemini CLI hook invocations are fail-closed on PATH miss.
func beekeeperGeminiEntry() geminiHookEntry {
	return geminiHookEntry{
		Event:   "BeforeTool",
		Matcher: ".*",
		Command: beekeeperCmd(geminiCheckSuffix),
	}
}

// installGemini appends a BeforeTool hook entry for beekeeper check --hook gemini
// to the Gemini CLI settings.json at settingsPath. The function is idempotent:
// it only appends the entry if no matching beekeeper entry (bare or abspath) is
// already present. Re-running migrates a stale bare-name entry to the new
// abspath form in place (no duplicate).
//
// Gemini CLI honors either exit 2 OR stdout {"decision":"deny","reason":"..."}.
// The deny JSON is rendered at runtime by RenderDeny(HarnessGemini). The
// installer only wires the command + event.
//
// Existing hooks (other BeforeTool entries, other event types) are preserved.
// A backup of the existing file is created before any modification.
func installGemini(settingsPath string, dryRun bool, out io.Writer) error {
	existing := geminiHooksFile{}

	if data, err := os.ReadFile(settingsPath); err == nil {
		// Tolerate parse errors — start with empty hooks if the file is malformed.
		_ = json.Unmarshal(data, &existing)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("gemini: read %q: %w", settingsPath, err)
	}

	newCmd := beekeeperCmd(geminiCheckSuffix)

	if dryRun {
		entry := beekeeperGeminiEntry()
		preview := geminiHooksFile{Hooks: append(existing.Hooks, entry)}
		data, _ := json.MarshalIndent(preview, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would write to %s:\n%s\n", settingsPath, string(data))
		return nil
	}

	// Merge/migrate: replace stale bare-name or stale-abspath entry in place.
	found := false
	migrated := make([]geminiHookEntry, 0, len(existing.Hooks)+1)
	for _, h := range existing.Hooks {
		if matchesBeekeeperCommand(h.Command, geminiCheckSuffix) {
			found = true
			if h.Command != newCmd {
				// Migrate stale entry to new abspath form.
				migrated = append(migrated, beekeeperGeminiEntry())
			} else {
				migrated = append(migrated, h)
			}
		} else {
			migrated = append(migrated, h)
		}
	}
	if !found {
		migrated = append(migrated, beekeeperGeminiEntry())
	}
	existing.Hooks = migrated

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	// Ensure the parent directory exists (e.g., ~/.gemini/ may not exist yet).
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("gemini: create dir %q: %w", filepath.Dir(settingsPath), err)
	}

	data, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("gemini: marshal hooks: %w", err)
	}

	return writeFileAtomic(settingsPath, data)
}

// uninstallGemini removes beekeeper check --hook gemini entries from the
// Gemini CLI settings.json hooks array. Suffix matching covers BOTH old
// bare-name and new abspath forms. Other hook entries are preserved.
// A backup is created before modification.
func uninstallGemini(settingsPath string, dryRun bool, out io.Writer) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Gemini CLI settings.json found at %s — nothing to uninstall.\n", settingsPath)
			return nil
		}
		return fmt.Errorf("gemini uninstall: read %q: %w", settingsPath, err)
	}

	var existing geminiHooksFile
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("gemini uninstall: parse %q: %w", settingsPath, err)
	}

	filtered := make([]geminiHookEntry, 0, len(existing.Hooks))
	removed := 0
	for _, h := range existing.Hooks {
		if matchesBeekeeperCommand(h.Command, geminiCheckSuffix) {
			removed++
			continue
		}
		filtered = append(filtered, h)
	}

	if removed == 0 {
		fmt.Fprintf(out, "No beekeeper hooks found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove %d beekeeper hook entry(ies) from %s\n", removed, settingsPath)
		return nil
	}

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	existing.Hooks = filtered

	out2, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("gemini uninstall: marshal: %w", err)
	}

	return writeFileAtomic(settingsPath, out2)
}
