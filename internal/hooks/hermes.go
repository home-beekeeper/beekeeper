package hooks

// Package hooks — hermes.go
//
// Hermes (NousResearch/hermes-agent) is a FAIL-OPEN harness: non-zero exit
// codes, timeouts, and malformed stdout do NOT block the tool call.  The ONLY
// deny path is a JSON object on stdout whose action or decision field is
// "block"/"deny" AND whose message field is non-empty.
//
// Concretely, RenderDeny(HarnessHermes, d) produces:
//   - ExitCode: 0   (Hermes ignores non-zero; emit 0 so it does not log a hook error)
//   - Stdout:   {"action":"block","message":"<reason>"}  — REQUIRED non-empty message
//   - Stderr:   <reason>  (best-effort human-readable)
//
// A missing or empty message field silently allows the tool.  Plan 01 ensures
// the message is always non-empty by substituting "blocked by beekeeper policy"
// when d.Reason is empty.
//
// Config format: ~/.hermes/config.yaml (YAML).  This installer does NOT add a
// gopkg.in/yaml.v3 dependency — it uses targeted string/section patching instead
// (CLAUDE.md constraint: no new module deps).

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// hermesPreCommand is the command written into the Hermes pre_tool_call hook.
// The --hook hermes flag causes beekeeper check to emit RenderDeny output
// (exit 0 + {"action":"block","message":"..."} on block) rather than the
// default exit-1 path used by the shim/gateway.
const hermesPreCommand = "beekeeper check --hook hermes"

// hermesConfigPath returns the path to the Hermes config file.
func hermesConfigPath(homeDir string) string {
	return homeDir + "/.hermes/config.yaml"
}

// hasHermesBeekeeperHook reports whether content already has a pre_tool_call
// entry pointing at hermesPreCommand. It does a simple line scan — sufficient
// for the single-command case and avoids a YAML parser dependency.
func hasHermesBeekeeperHook(content string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, hermesPreCommand) {
			return true
		}
	}
	return false
}

// hermesHookBlock returns the YAML block to append for the beekeeper
// pre_tool_call hook entry. The block includes the required `hooks:` and
// `pre_tool_call:` nesting.
//
// Example output (appended at end of config):
//
//	hooks:
//	  pre_tool_call:
//	    - command: beekeeper check --hook hermes
const hermesHookBlock = `
hooks:
  pre_tool_call:
    - command: ` + hermesPreCommand + `
`

// patchHermesConfig returns a new YAML string that idempotently ensures a
// pre_tool_call beekeeper entry is present. It handles three cases:
//
//  1. A `hooks:` section with a `pre_tool_call:` sub-key already exists
//     → insert the command entry under the existing pre_tool_call block.
//  2. A `hooks:` section exists but has no `pre_tool_call:` sub-key
//     → append `pre_tool_call:` under the `hooks:` section.
//  3. Neither section exists
//     → append the full hermesHookBlock.
//
// Pre-condition: hasHermesBeekeeperHook(content) must be false (caller checks).
func patchHermesConfig(content string) string {
	lines := strings.Split(content, "\n")

	// Locate `hooks:` section.
	hooksIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "hooks:" {
			hooksIdx = i
			break
		}
	}

	if hooksIdx < 0 {
		// No hooks section — append the full block.
		trimmed := strings.TrimRight(content, "\n")
		if trimmed == "" {
			return hermesHookBlock
		}
		return trimmed + "\n" + hermesHookBlock
	}

	// Find a `pre_tool_call:` sub-key directly under `hooks:`.
	preIdx := -1
	for i := hooksIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		// Any non-indented line (other than empty) after hooks: means we left the section.
		if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		if strings.TrimSpace(line) == "pre_tool_call:" {
			preIdx = i
			break
		}
	}

	if preIdx < 0 {
		// hooks: exists but no pre_tool_call: — insert it after hooks: line.
		newLines := make([]string, 0, len(lines)+2)
		for i, line := range lines {
			newLines = append(newLines, line)
			if i == hooksIdx {
				newLines = append(newLines,
					"  pre_tool_call:",
					"    - command: "+hermesPreCommand,
				)
			}
		}
		return strings.Join(newLines, "\n")
	}

	// pre_tool_call: exists — insert the command entry after it.
	newLines := make([]string, 0, len(lines)+1)
	for i, line := range lines {
		newLines = append(newLines, line)
		if i == preIdx {
			newLines = append(newLines, "    - command: "+hermesPreCommand)
		}
	}
	return strings.Join(newLines, "\n")
}

// installHermes installs the Beekeeper pre_tool_call hook into
// ~/.hermes/config.yaml. The edit is idempotent: if hermesPreCommand is
// already present the function no-ops (and prints a message). Existing
// content is always preserved. A backup is created before any modification.
//
// Deny contract reminder: Hermes ignores exit codes. The actual block is
// carried ONLY by RenderDeny(HarnessHermes)'s stdout JSON with a guaranteed
// non-empty message field.  The installer wires the JSON-emitting command so
// that contract is satisfied on every invocation.
func installHermes(configPath string, dryRun bool, out io.Writer) error {
	// Read existing config; treat ErrNotExist as empty.
	data, err := os.ReadFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("hermes: read %q: %w", configPath, err)
	}
	content := string(data)

	if hasHermesBeekeeperHook(content) {
		fmt.Fprintf(out, "Hermes config.yaml already has a beekeeper pre_tool_call entry — no change.\n")
		return nil
	}

	updated := patchHermesConfig(content)

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would write to %s (pre_tool_call beekeeper entry added):\n%s\n", configPath, updated)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("hermes: mkdir %q: %w", filepath.Dir(configPath), err)
	}

	if err := backupSettings(configPath); err != nil {
		return err
	}

	if err := writeFileAtomic(configPath, []byte(updated)); err != nil {
		return err
	}

	fmt.Fprintf(out, "Installed Hermes pre_tool_call hook in %s\n", configPath)
	fmt.Fprintf(out, "Note: Hermes is fail-OPEN — exit codes are ignored. The block is carried\n")
	fmt.Fprintf(out, "      by the JSON stdout from 'beekeeper check --hook hermes'.\n")
	return nil
}

// uninstallHermes removes the beekeeper pre_tool_call line/block from
// ~/.hermes/config.yaml. Other hooks and all other content are preserved.
// If the command is not found, uninstallHermes is a no-op.
func uninstallHermes(configPath string, dryRun bool, out io.Writer) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Hermes config.yaml found at %s — nothing to uninstall.\n", configPath)
			return nil
		}
		return fmt.Errorf("hermes uninstall: read %q: %w", configPath, err)
	}
	content := string(data)

	if !hasHermesBeekeeperHook(content) {
		fmt.Fprintf(out, "No beekeeper hook found in %s — nothing to uninstall.\n", configPath)
		return nil
	}

	updated := removeHermesBeekeeperHook(content)

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove beekeeper pre_tool_call entry from %s\n", configPath)
		return nil
	}

	if err := backupSettings(configPath); err != nil {
		return err
	}

	if err := writeFileAtomic(configPath, []byte(updated)); err != nil {
		return err
	}

	fmt.Fprintf(out, "Removed Hermes pre_tool_call beekeeper hook from %s\n", configPath)
	return nil
}

// removeHermesBeekeeperHook returns a new YAML string with the beekeeper
// command entry removed. It removes the `    - command: hermesPreCommand` line
// and, if the surrounding pre_tool_call block becomes empty, the surrounding
// structure (empty pre_tool_call: + empty hooks:) is also cleaned up.
func removeHermesBeekeeperHook(content string) string {
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "- command: "+hermesPreCommand {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

