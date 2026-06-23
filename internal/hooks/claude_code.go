package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/home-beekeeper/beekeeper/internal/editorinit"
)

// Beekeeper's hook command stable suffixes, used both to build the entries
// (via beekeeperCmd) and to detect their presence for idempotent install,
// migration, and targeted uninstall via matchesBeekeeperCommand.
//
// These suffixes are stable invariants: the installed command is always
// "<quoted-abspath> <suffix>" (or "beekeeper <suffix>" on fallback), so
// matching on the suffix covers BOTH old bare-name and new abspath forms.
const (
	claudeCheckSuffix  = "check --hook claude-code"
	claudeAuditSuffix  = "audit-record"
)

// beekeeperClaudePreEntry returns the canonical PreToolUse entry for Claude Code
// (matcher ".*" → beekeeperCmd(claudeCheckSuffix)). Schema verified from
// code.claude.com/docs/en/hooks.
func beekeeperClaudePreEntry() map[string]any {
	return map[string]any{
		"matcher": ".*",
		"hooks": []any{
			map[string]any{"type": "command", "command": beekeeperCmd(claudeCheckSuffix)},
		},
	}
}

// beekeeperClaudePostEntry returns the canonical PostToolUse entry for Claude Code.
func beekeeperClaudePostEntry() map[string]any {
	return map[string]any{
		"matcher": ".*",
		"hooks": []any{
			map[string]any{"type": "command", "command": beekeeperCmd(claudeAuditSuffix)},
		},
	}
}

// claudeHookConfig builds the beekeeper hooks block shown in dry-run output.
func claudeHookConfig() map[string]any {
	return map[string]any{
		"PreToolUse":  []any{beekeeperClaudePreEntry()},
		"PostToolUse": []any{beekeeperClaudePostEntry()},
	}
}

// claudeEntriesContainCommand reports whether any entry in a Claude event array
// has an inner "hooks" command that matches the given stable suffix.
// Uses matchesBeekeeperCommand so BOTH old bare-name and new abspath forms are
// detected — idempotency and migration both work on mixed-form installs.
// Tolerates the loosely-typed map[string]any/[]any shapes that json.Unmarshal produces.
func claudeEntriesContainCommand(entries []any, suffix string) bool {
	for _, e := range entries {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := em["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if c, _ := hm["command"].(string); matchesBeekeeperCommand(c, suffix) {
				return true
			}
		}
	}
	return false
}

// mergeClaudeHookEntry appends entry to an existing Claude event array,
// preserving every existing non-beekeeper entry.
//
// Migration: if an existing entry already matches the suffix (old bare-name OR
// stale abspath), it is REPLACED in place with the freshly-built abspath command
// from entry (exactly one beekeeper entry remains). This self-heals stale entries
// without duplication.
//
// CRITICAL (clobber fix): this MERGES rather than overwrites so that a user's
// pre-existing Claude Code hooks (e.g. a project's own PreToolUse guards) are
// never destroyed by `beekeeper hooks install`. Before this change the installer
// replaced the entire "hooks" key, silently wiping every non-beekeeper hook.
func mergeClaudeHookEntry(existing any, suffix string, entry map[string]any) []any {
	arr, _ := existing.([]any)

	// Extract the freshly-built command from entry for migration comparison.
	newCmd := ""
	if hooksArr, ok := entry["hooks"].([]any); ok && len(hooksArr) > 0 {
		if hm, ok := hooksArr[0].(map[string]any); ok {
			newCmd, _ = hm["command"].(string)
		}
	}

	found := false
	merged := make([]any, 0, len(arr)+1)
	for _, e := range arr {
		em, ok := e.(map[string]any)
		if !ok {
			merged = append(merged, e)
			continue
		}
		inner, ok := em["hooks"].([]any)
		if !ok {
			merged = append(merged, e)
			continue
		}
		// Check if any inner command matches our suffix (covers both bare and abspath).
		matchedInner := false
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if c, _ := hm["command"].(string); matchesBeekeeperCommand(c, suffix) {
				matchedInner = true
				break
			}
		}
		if matchedInner {
			found = true
			if newCmd == "" {
				// Fallback: keep as-is if we couldn't extract the new command.
				merged = append(merged, e)
				continue
			}
			// Check if migration is needed (old bare-name or stale abspath).
			needsMigration := false
			for _, h := range inner {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				if c, _ := hm["command"].(string); c != newCmd {
					needsMigration = true
					break
				}
			}
			if needsMigration {
				// Replace in place with the freshly-built abspath entry.
				merged = append(merged, entry)
			} else {
				// Already up to date.
				merged = append(merged, e)
			}
		} else {
			merged = append(merged, e)
		}
	}
	if !found {
		merged = append(merged, entry)
	}
	return merged
}

// removeClaudeHookEntry returns existing with every entry whose inner hooks
// match the given suffix dropped, plus the number removed.
// Non-beekeeper entries are preserved. existing may be nil.
func removeClaudeHookEntry(existing any, suffix string) ([]any, int) {
	arr, _ := existing.([]any)
	filtered := make([]any, 0, len(arr))
	removed := 0
	for _, e := range arr {
		if claudeEntriesContainCommand([]any{e}, suffix) {
			removed++
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, removed
}

// installClaudeCode merges Beekeeper's PreToolUse and PostToolUse hooks into the
// Claude Code settings.json at settingsPath WITHOUT disturbing any pre-existing
// hooks (other PreToolUse/PostToolUse entries, SessionStart, etc.) or other
// top-level settings keys.
//
// The installed commands embed the running binary's absolute path (resolved via
// os.Executable at install time) so the hook resolves regardless of PATH drift.
// Re-running performs migration: if an old bare-name entry exists, it is replaced
// in place with the new abspath command (no duplication).
//
// It reads via editorinit.ReadSettings (JSONC-safe) to compute the merged hooks
// value, then writes via editorinit.PatchSettings (atomic, MkdirAll, sets only
// the "hooks" key so all sibling keys are preserved). The function is idempotent:
// re-running does not duplicate beekeeper's entries.
//
// A backup of the existing file is created before any modification.
func installClaudeCode(settingsPath string, dryRun bool, out io.Writer) error {
	if dryRun {
		data, _ := json.MarshalIndent(claudeHookConfig(), "", "    ")
		fmt.Fprintf(out, "[dry-run] Would merge into %s (hooks key — existing hooks preserved):\n%s\n", settingsPath, string(data))
		return nil
	}

	// Read existing settings to MERGE into (ReadSettings returns an empty map on
	// ErrNotExist, so a non-nil error here is a genuine read/parse failure).
	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("install claude-code: parse %q: %w", settingsPath, err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	// Merge (append-if-absent, migrate-if-stale) — never overwrite sibling hooks.
	hooks["PreToolUse"] = mergeClaudeHookEntry(hooks["PreToolUse"], claudeCheckSuffix, beekeeperClaudePreEntry())
	hooks["PostToolUse"] = mergeClaudeHookEntry(hooks["PostToolUse"], claudeAuditSuffix, beekeeperClaudePostEntry())

	if err := backupSettings(settingsPath); err != nil {
		return err
	}

	// PatchSettings sets ONLY the "hooks" key (preserving statusLine, enabledPlugins,
	// theme, …) and handles MkdirAll + atomic write.
	return editorinit.PatchSettings(settingsPath, "hooks", hooks)
}

// uninstallClaudeCode removes ONLY Beekeeper's entries from the Claude Code
// settings.json, preserving all other hooks. If an event array becomes empty it
// is dropped; if the whole hooks map becomes empty the "hooks" key is removed.
// Other top-level settings keys are always preserved. A backup is created first.
//
// Uninstall uses suffix matching so it removes BOTH old bare-name entries and new
// abspath entries.
//
// WR-01: editorinit.ReadSettings strips JSONC comments before unmarshalling so
// settings.json files containing // or /* */ comments are parsed correctly.
func uninstallClaudeCode(settingsPath string, dryRun bool, out io.Writer) error {
	settings, err := editorinit.ReadSettings(settingsPath)
	if err != nil {
		// ReadSettings returns (emptyMap, nil) for ErrNotExist, so a non-nil error
		// here is a genuine read/parse failure.
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Claude Code settings.json found at %s — nothing to uninstall.\n", settingsPath)
			return nil
		}
		return fmt.Errorf("uninstall claude-code: parse %q: %w", settingsPath, err)
	}

	if len(settings) == 0 {
		fmt.Fprintf(out, "No Claude Code settings.json found at %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok || hooks == nil {
		fmt.Fprintf(out, "No hooks key found in %s — nothing to uninstall.\n", settingsPath)
		return nil
	}

	preArr, removedPre := removeClaudeHookEntry(hooks["PreToolUse"], claudeCheckSuffix)
	postArr, removedPost := removeClaudeHookEntry(hooks["PostToolUse"], claudeAuditSuffix)
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

	// Reassign the filtered arrays; drop an event key when its array is empty.
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

	// Remove the hooks key entirely only if nothing else remains under it.
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	out2, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return fmt.Errorf("uninstall claude-code: marshal: %w", err)
	}

	return writeFileAtomic(settingsPath, out2)
}
