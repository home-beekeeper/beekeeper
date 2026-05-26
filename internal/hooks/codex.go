package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// codexHooksFile is the top-level structure of ~/.codex/hooks.json.
// Schema verified from developers.openai.com/codex/hooks.
// The nested structure differs from Cursor: each event key maps to an array of
// codexHookEntry values, each of which has its own inner "hooks" command array.
type codexHooksFile struct {
	Hooks map[string][]codexHookEntry `json:"hooks"`
}

// codexHookEntry is a single hook configuration block in Codex's hooks file.
// It contains a matcher regex and an inner array of command definitions.
type codexHookEntry struct {
	Matcher string         `json:"matcher"`
	Hooks   []codexHookCmd `json:"hooks"`
}

// codexHookCmd is a single command within a codexHookEntry.
type codexHookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// codexTrustReminder is the message printed after writing ~/.codex/hooks.json.
// Codex requires the user to explicitly trust new hooks on first run.
const codexTrustReminder = `
Codex requires you to trust this hook. Run Codex once and approve the hook
prompt before Beekeeper interception takes effect.
`

// containsCodexHookByCommand reports whether any hook in the given entry array
// already contains a command matching cmd. Used to prevent duplicates.
func containsCodexHookByCommand(entries []codexHookEntry, cmd string) bool {
	for _, entry := range entries {
		for _, h := range entry.Hooks {
			if h.Command == cmd {
				return true
			}
		}
	}
	return false
}

// beekeeperCodexPreToolUse returns the canonical PreToolUse entry for Codex.
func beekeeperCodexPreToolUse() codexHookEntry {
	return codexHookEntry{
		Matcher: ".*",
		Hooks: []codexHookCmd{
			{
				Type:    "command",
				Command: "beekeeper check",
				Timeout: 10,
			},
		},
	}
}

// beekeeperCodexPostToolUse returns the canonical PostToolUse entry for Codex.
func beekeeperCodexPostToolUse() codexHookEntry {
	return codexHookEntry{
		Matcher: ".*",
		Hooks: []codexHookCmd{
			{
				Type:    "command",
				Command: "beekeeper audit-record",
			},
		},
	}
}

// installCodex merges Beekeeper's PreToolUse and PostToolUse hooks into
// ~/.codex/hooks.json. The function is idempotent: it only appends the entries
// if "beekeeper check" / "beekeeper audit-record" are not already present.
//
// After writing the file, it prints the Codex trust reminder.
func installCodex(hooksPath string, dryRun bool, out io.Writer) error {
	existing := codexHooksFile{
		Hooks: make(map[string][]codexHookEntry),
	}

	if data, err := os.ReadFile(hooksPath); err == nil {
		// Tolerate parse errors — start fresh if the file is malformed.
		_ = json.Unmarshal(data, &existing)
		if existing.Hooks == nil {
			existing.Hooks = make(map[string][]codexHookEntry)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("codex: read %q: %w", hooksPath, err)
	}

	// Idempotent: only append PreToolUse if beekeeper check is not already present.
	if !containsCodexHookByCommand(existing.Hooks["PreToolUse"], "beekeeper check") {
		existing.Hooks["PreToolUse"] = append(existing.Hooks["PreToolUse"], beekeeperCodexPreToolUse())
	}

	// Idempotent: only append PostToolUse if beekeeper audit-record is not already present.
	if !containsCodexHookByCommand(existing.Hooks["PostToolUse"], "beekeeper audit-record") {
		existing.Hooks["PostToolUse"] = append(existing.Hooks["PostToolUse"], beekeeperCodexPostToolUse())
	}

	if dryRun {
		data, _ := json.MarshalIndent(existing, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would write to %s:\n%s\n", hooksPath, string(data))
		return nil
	}

	if err := backupSettings(hooksPath); err != nil {
		return err
	}

	// Ensure the parent directory exists (e.g., ~/.codex/ may not exist yet).
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return fmt.Errorf("codex: create dir %q: %w", filepath.Dir(hooksPath), err)
	}

	data, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("codex: marshal hooks: %w", err)
	}

	if err := writeFileAtomic(hooksPath, data); err != nil {
		return err
	}

	// Print the trust reminder after a successful write.
	fmt.Fprint(out, codexTrustReminder)
	return nil
}

// uninstallCodex removes beekeeper entries from PreToolUse and PostToolUse in
// ~/.codex/hooks.json. Other hooks are preserved. A backup is created first.
func uninstallCodex(hooksPath string, dryRun bool, out io.Writer) error {
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Codex hooks.json found at %s — nothing to uninstall.\n", hooksPath)
			return nil
		}
		return fmt.Errorf("codex uninstall: read %q: %w", hooksPath, err)
	}

	var existing codexHooksFile
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("codex uninstall: parse %q: %w", hooksPath, err)
	}
	if existing.Hooks == nil {
		existing.Hooks = make(map[string][]codexHookEntry)
	}

	removed := 0

	// Remove beekeeper check entries from PreToolUse.
	preToolUse := existing.Hooks["PreToolUse"]
	filtered := make([]codexHookEntry, 0, len(preToolUse))
	for _, entry := range preToolUse {
		if containsCodexHookByCommand([]codexHookEntry{entry}, "beekeeper check") {
			removed++
			continue
		}
		filtered = append(filtered, entry)
	}
	existing.Hooks["PreToolUse"] = filtered

	// Remove beekeeper audit-record entries from PostToolUse.
	postToolUse := existing.Hooks["PostToolUse"]
	filteredPost := make([]codexHookEntry, 0, len(postToolUse))
	for _, entry := range postToolUse {
		if containsCodexHookByCommand([]codexHookEntry{entry}, "beekeeper audit-record") {
			removed++
			continue
		}
		filteredPost = append(filteredPost, entry)
	}
	existing.Hooks["PostToolUse"] = filteredPost

	if removed == 0 {
		fmt.Fprintf(out, "No beekeeper hooks found in %s — nothing to uninstall.\n", hooksPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove %d beekeeper hook entry(ies) from %s\n", removed, hooksPath)
		return nil
	}

	if err := backupSettings(hooksPath); err != nil {
		return err
	}

	out2, err := json.MarshalIndent(existing, "", "    ")
	if err != nil {
		return fmt.Errorf("codex uninstall: marshal: %w", err)
	}

	return writeFileAtomic(hooksPath, out2)
}
