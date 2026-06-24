//go:build !windows

package hooks

// Package hooks — cline.go (macOS/Linux only, !windows build constraint)
//
// Cline hooks are macOS/Linux only. The hook mechanism is an EXECUTABLE FILE
// named "PreToolUse" (no extension) in:
//   - Project-local:  .clinerules/hooks/PreToolUse
//   - Global (used by this installer): ~/Documents/Cline/Rules/Hooks/PreToolUse
//
// The deny contract (RESEARCH.md row 4):
//   - stdout {"cancel":true,"errorMessage":"..."} + exit 0, OR
//   - exit 2
//
// RenderDeny(HarnessCline, d) handles the JSON output.
// The PreToolUse file must be executable (mode 0o755) or Cline will not run it.
//
// Threat T-10-20: installCline backs up and refuses to silently destroy a
// foreign PreToolUse script that does not contain a beekeeper hook marker.
// uninstallCline verifies the beekeeper marker before removing.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// clineCheckSuffix is the stable suffix for the Cline beekeeper hook command.
// Used for idempotent install, migration, and targeted uninstall.
// Matches BOTH old bare-name and new abspath forms via matchesBeekeeperCommand.
const clineCheckSuffix = "check --hook cline"

// clineHooksDir returns the path to the global Cline hooks directory.
// The project-local alternative (.clinerules/hooks/) is documented here but
// not targeted by this installer — users who want project-scoped hooks should
// place the script there manually.
func clineHooksDir(homeDir string) string {
	// Global hooks directory: ~/Documents/Cline/Rules/Hooks/
	// Project-local alternative: <project>/.clinerules/hooks/PreToolUse (manual)
	return filepath.Join(homeDir, "Documents", "Cline", "Rules", "Hooks")
}

// clinePreToolUsePath returns the path to the PreToolUse executable file.
func clinePreToolUsePath(hooksDir string) string {
	return filepath.Join(hooksDir, "PreToolUse")
}

// clineScript returns the content written to the PreToolUse executable.
// It is a POSIX shell script that invokes beekeeper with its absolute path
// (resolved at install time) so the hook does not fail with exit 127 when
// beekeeper was installed after Cline captured its PATH.
func clineScript() string {
	return "#!/bin/sh\n" + beekeeperCmd(clineCheckSuffix) + "\n"
}

// installCline writes an executable PreToolUse script to hooksDir.
// Behaviour:
//   - If the file does not exist: create it with mode 0o755.
//   - If the file already contains a current abspath beekeeper command: no-op.
//   - If the file contains a stale beekeeper command (bare-name or stale abspath):
//     migrate in place (rewrite with current abspath command).
//   - If the file exists but contains a FOREIGN script: back it up and report;
//     do NOT silently overwrite (T-10-20: preserves the user's existing hook).
func installCline(hooksDir string, dryRun bool, out io.Writer) error {
	hookPath := clinePreToolUsePath(hooksDir)
	newScript := clineScript()

	existing, err := os.ReadFile(hookPath)
	switch {
	case err == nil:
		// File exists — check its content.
		content := string(existing)
		if containsClineCommand(content) {
			// Already has a beekeeper hook marker. Check if migration needed.
			if content == newScript {
				fmt.Fprintf(out, "Cline PreToolUse hook already has current beekeeper command — no change.\n")
				return nil
			}
			// Stale bare-name or stale-abspath form — migrate in place.
			if dryRun {
				fmt.Fprintf(out, "[dry-run] Would migrate Cline PreToolUse at %s to abspath form\n", hookPath)
				return nil
			}
			if err := backupSettings(hookPath); err != nil {
				return err
			}
			if err := os.WriteFile(hookPath, []byte(newScript), 0o755); err != nil {
				return fmt.Errorf("cline migrate: write %q: %w", hookPath, err)
			}
			fmt.Fprintf(out, "Migrated Cline PreToolUse hook to abspath form at %s (mode 0755)\n", hookPath)
			return nil
		}
		// Foreign script: back up and report, but still install.
		if dryRun {
			fmt.Fprintf(out, "[dry-run] Would back up existing foreign PreToolUse to %s.beekeeper-backup-* and overwrite with beekeeper script at %s\n", hookPath, hookPath)
			return nil
		}
		if err := backupSettings(hookPath); err != nil {
			return err
		}
		fmt.Fprintf(out, "WARNING: Backed up existing PreToolUse script (foreign tool). Installing Beekeeper script.\n")

	case errors.Is(err, os.ErrNotExist):
		if dryRun {
			fmt.Fprintf(out, "[dry-run] Would create executable PreToolUse script at %s\n", hookPath)
			return nil
		}
		// Fall through to write.

	default:
		return fmt.Errorf("cline: read %q: %w", hookPath, err)
	}

	if dryRun {
		// Already handled the foreign-script dryRun path above; this catches
		// the ErrNotExist path where dryRun was set but we still reach here.
		fmt.Fprintf(out, "[dry-run] Would create executable PreToolUse script at %s\n", hookPath)
		return nil
	}

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("cline: mkdir %q: %w", hooksDir, err)
	}

	if err := os.WriteFile(hookPath, []byte(newScript), 0o755); err != nil {
		return fmt.Errorf("cline: write %q: %w", hookPath, err)
	}

	fmt.Fprintf(out, "Installed Cline PreToolUse hook at %s (mode 0755)\n", hookPath)
	return nil
}

// uninstallCline removes the beekeeper PreToolUse script from hooksDir.
// It only removes the script if it contains the beekeeper hook marker (suffix);
// foreign scripts are preserved (T-10-20). Suffix matching covers BOTH old
// bare-name and new abspath forms.
func uninstallCline(hooksDir string, dryRun bool, out io.Writer) error {
	hookPath := clinePreToolUsePath(hooksDir)

	data, err := os.ReadFile(hookPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "No Cline PreToolUse script found at %s — nothing to uninstall.\n", hookPath)
			return nil
		}
		return fmt.Errorf("cline uninstall: read %q: %w", hookPath, err)
	}

	if !containsClineCommand(string(data)) {
		fmt.Fprintf(out, "PreToolUse at %s is not a beekeeper script — not removed (foreign hook preserved).\n", hookPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(out, "[dry-run] Would remove beekeeper PreToolUse script at %s\n", hookPath)
		return nil
	}

	if err := os.Remove(hookPath); err != nil {
		return fmt.Errorf("cline uninstall: remove %q: %w", hookPath, err)
	}

	fmt.Fprintf(out, "Removed Cline PreToolUse beekeeper hook at %s\n", hookPath)
	return nil
}

// containsClineCommand reports whether content contains a beekeeper hook marker.
// Matches BOTH old bare-name ("beekeeper check --hook cline") and new abspath
// forms (e.g. '"/path/to/beekeeper" check --hook cline') via stable-suffix matching.
func containsClineCommand(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if matchesBeekeeperCommand(line, clineCheckSuffix) {
			return true
		}
	}
	return false
}
