package hooks

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// codexCheckSuffix / codexAuditSuffix are the stable suffixes for Codex hooks.
const (
	codexCheckSuffix = "check --hook codex"
	codexAuditSuffix = "audit-record"
)

// containsCodexHookByCommand reports whether any hook in the given entry array
// already contains a command matching the given stable suffix.
// Matches BOTH old bare-name and new abspath forms via matchesBeekeeperCommand.
func containsCodexHookByCommand(entries []codexHookEntry, suffix string) bool {
	for _, entry := range entries {
		for _, h := range entry.Hooks {
			if matchesBeekeeperCommand(h.Command, suffix) {
				return true
			}
		}
	}
	return false
}

// beekeeperCodexPreToolUse returns the canonical PreToolUse entry for Codex.
// Command uses the absolute binary path via beekeeperCmd so that Codex hook
// invocations emit exit 2 + hookSpecificOutput deny JSON on block (HPC-02).
func beekeeperCodexPreToolUse() codexHookEntry {
	return codexHookEntry{
		Matcher: ".*",
		Hooks: []codexHookCmd{
			{
				Type:    "command",
				Command: beekeeperCmd(codexCheckSuffix),
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
				Command: beekeeperCmd(codexAuditSuffix),
			},
		},
	}
}

// codexConfigPath returns the path to Codex's config.toml.
func codexConfigPath(homeDir string) string {
	return homeDir + "/.codex/config.toml"
}

// ensureCodexFeaturesFlag idempotently ensures that ~/.codex/config.toml
// contains a [features] section with hooks=true. This flag is required for
// Codex to execute hooks at all (Codex PR #18385 gate — without it hooks are
// silently ignored regardless of hooks.json contents).
//
// The function uses targeted string patching to avoid adding a TOML library
// dependency: it reads the existing file line-by-line, finds or appends the
// [features] section, and ensures hooks = true is present within it.
// Existing content (other sections, other feature flags) is always preserved.
//
// Backup of config.toml is taken by the caller before this function runs.
func ensureCodexFeaturesFlag(configPath string, out io.Writer) error {
	// Read existing content; treat ErrNotExist as empty.
	data, err := os.ReadFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("codex config: read %q: %w", configPath, err)
	}
	content := string(data)

	// Fast path: already correct.
	if hasCodexHooksTrue(content) {
		fmt.Fprintf(out, "Codex config.toml already has [features] hooks = true — no change.\n")
		return nil
	}

	updated := patchCodexFeaturesFlag(content)

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("codex config: mkdir %q: %w", filepath.Dir(configPath), err)
	}
	return writeFileAtomic(configPath, []byte(updated))
}

// hasCodexHooksTrue reports whether content already contains hooks = true (or
// hooks=true) inside a [features] section. It does a simple line scan — this
// is not a general TOML parser, but is sufficient for the single-flag case.
func hasCodexHooksTrue(content string) bool {
	inFeatures := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[features]" {
			inFeatures = true
			continue
		}
		// Any new top-level section ends the [features] block.
		if strings.HasPrefix(line, "[") && line != "[features]" {
			inFeatures = false
		}
		if inFeatures {
			// Accept both "hooks = true" and "hooks=true".
			normalized := strings.ReplaceAll(line, " ", "")
			if normalized == "hooks=true" {
				return true
			}
		}
	}
	return false
}

// patchCodexFeaturesFlag returns a new TOML string that has [features]\nhooks=true
// added. If a [features] section already exists without hooks=true, the key is
// inserted after the [features] header. If no [features] section exists, one
// is appended at the end of the file.
func patchCodexFeaturesFlag(content string) string {
	lines := splitLines(content)

	// Find an existing [features] section.
	featuresIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "[features]" {
			featuresIdx = i
			break
		}
	}

	if featuresIdx >= 0 {
		// Insert "hooks = true" right after the [features] header line.
		newLines := make([]string, 0, len(lines)+1)
		for i, line := range lines {
			newLines = append(newLines, line)
			if i == featuresIdx {
				newLines = append(newLines, "hooks = true")
			}
		}
		return strings.Join(newLines, "\n")
	}

	// No [features] section: append one.
	// Ensure the file doesn't already end with two newlines.
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return "[features]\nhooks = true\n"
	}
	return trimmed + "\n\n[features]\nhooks = true\n"
}

// splitLines splits content by "\n" preserving empty lines.
func splitLines(content string) []string {
	return strings.Split(content, "\n")
}

// mergeCodexHookEntry merges beekeeper entries into a codex event array,
// migrating stale bare-name or stale-abspath entries in place.
func mergeCodexHookEntry(arr []codexHookEntry, suffix string, newEntry codexHookEntry) []codexHookEntry {
	newCmd := ""
	if len(newEntry.Hooks) > 0 {
		newCmd = newEntry.Hooks[0].Command
	}

	found := false
	merged := make([]codexHookEntry, 0, len(arr)+1)
	for _, entry := range arr {
		matched := false
		for _, h := range entry.Hooks {
			if matchesBeekeeperCommand(h.Command, suffix) {
				matched = true
				break
			}
		}
		if matched {
			found = true
			// Check if migration needed.
			needsMigration := false
			for _, h := range entry.Hooks {
				if matchesBeekeeperCommand(h.Command, suffix) && h.Command != newCmd {
					needsMigration = true
					break
				}
			}
			if needsMigration && newCmd != "" {
				merged = append(merged, newEntry)
			} else {
				merged = append(merged, entry)
			}
		} else {
			merged = append(merged, entry)
		}
	}
	if !found {
		merged = append(merged, newEntry)
	}
	return merged
}

// installCodex merges Beekeeper's PreToolUse and PostToolUse hooks into
// ~/.codex/hooks.json, then ensures [features] hooks=true in config.toml.
// The installed commands embed the running binary's absolute path. Re-running
// migrates stale entries to the new abspath form in place (no duplicate).
//
// After writing the hooks file, it prints the Codex trust reminder.
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

	// Merge/migrate PreToolUse and PostToolUse.
	existing.Hooks["PreToolUse"] = mergeCodexHookEntry(existing.Hooks["PreToolUse"], codexCheckSuffix, beekeeperCodexPreToolUse())
	existing.Hooks["PostToolUse"] = mergeCodexHookEntry(existing.Hooks["PostToolUse"], codexAuditSuffix, beekeeperCodexPostToolUse())

	if dryRun {
		data, _ := json.MarshalIndent(existing, "", "    ")
		fmt.Fprintf(out, "[dry-run] Would write to %s:\n%s\n", hooksPath, string(data))
		// Also describe what would happen to config.toml.
		homeDir, _ := os.UserHomeDir()
		configPath := codexConfigPath(homeDir)
		fmt.Fprintf(out, "[dry-run] Would ensure [features] hooks = true in %s\n", configPath)
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

	// Print the trust reminder after a successful hooks.json write.
	fmt.Fprint(out, codexTrustReminder)

	// Ensure [features] hooks=true in config.toml (Codex PR #18385 gate).
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("codex: could not determine home directory: %w", err)
	}
	configPath := codexConfigPath(homeDir)
	if err := backupSettings(configPath); err != nil {
		return err
	}
	return ensureCodexFeaturesFlag(configPath, out)
}

// uninstallCodex removes beekeeper entries from PreToolUse and PostToolUse in
// ~/.codex/hooks.json. Suffix matching covers BOTH old bare-name and new abspath
// forms. Other hooks are preserved. A backup is created first.
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

	// Remove beekeeper check --hook codex entries from PreToolUse.
	preToolUse := existing.Hooks["PreToolUse"]
	filtered := make([]codexHookEntry, 0, len(preToolUse))
	for _, entry := range preToolUse {
		if containsCodexHookByCommand([]codexHookEntry{entry}, codexCheckSuffix) {
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
		if containsCodexHookByCommand([]codexHookEntry{entry}, codexAuditSuffix) {
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
