// Package hooks implements the beekeeper hook installer and uninstaller.
// It writes PreToolUse/PostToolUse hooks into the correct settings files for
// Claude Code, Cursor, Codex, Augment, CodeBuddy, and Qwen. Gateway-based
// targets (Continue, OpenCode, OpenClaw) receive printed configuration
// guidance rather than a file write.
package hooks

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Supported target names.
const (
	TargetClaudeCode  = "claude-code"
	TargetCursor      = "cursor"
	TargetCodex       = "codex"
	TargetAugment     = "augment"
	TargetCodeBuddy   = "codebuddy"
	TargetQwen        = "qwen"
	TargetCopilot     = "copilot"
	TargetAntigravity = "antigravity"
	TargetGemini      = "gemini"
	TargetWindsurf    = "windsurf"
	TargetContinue    = "continue"
	TargetOpenCode    = "opencode"
	TargetOpenClaw    = "openclaw"
)

// gatewayTargets is the set of targets that receive a printed guide rather than
// a file write.
var gatewayTargets = map[string]bool{
	TargetContinue: true,
	TargetOpenCode: true,
	TargetOpenClaw: true,
}

// fileTargets is the ordered list of targets that write files.
var fileTargets = []string{
	TargetClaudeCode, TargetCursor, TargetCodex,
	TargetAugment, TargetCodeBuddy, TargetQwen,
	TargetCopilot, TargetAntigravity, TargetGemini, TargetWindsurf,
}

// allTargets is the complete list of supported targets.
var allTargets = []string{
	TargetClaudeCode, TargetCursor, TargetCodex,
	TargetAugment, TargetCodeBuddy, TargetQwen,
	TargetCopilot, TargetAntigravity, TargetGemini, TargetWindsurf,
	TargetContinue, TargetOpenCode, TargetOpenClaw,
}

// Install installs Beekeeper hooks for the given target.
//
//   - dryRun: print what would be written without touching any file.
//   - force: not currently used; reserved for future idempotency override.
//
// For file-writing targets (claude-code, cursor, codex), the target settings
// file is backed up before modification.
// For gateway targets (continue, opencode, openclaw), a configuration guide is
// printed to os.Stdout.
// Returns a descriptive error for unknown targets.
func Install(target string, dryRun bool, force bool) error {
	return InstallTo(target, dryRun, force, os.Stdout)
}

// InstallTo is the testable variant of Install that accepts an io.Writer for
// output (guides and dry-run messages).
func InstallTo(target string, dryRun bool, force bool, out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("hooks: could not determine home directory: %w", err)
	}

	switch target {
	case TargetClaudeCode:
		settingsPath := claudeSettingsPath(homeDir)
		return installClaudeCode(settingsPath, dryRun, out)

	case TargetCursor:
		hooksPath := cursorHooksPath(homeDir)
		return installCursor(hooksPath, dryRun, out)

	case TargetCodex:
		hooksPath := codexHooksPath(homeDir)
		return installCodex(hooksPath, dryRun, out)

	case TargetAugment:
		settingsPath := augmentSettingsPath(homeDir)
		return installAugment(settingsPath, dryRun, out)

	case TargetCodeBuddy:
		settingsPath := codebuddySettingsPath(homeDir)
		return installCodeBuddy(settingsPath, dryRun, out)

	case TargetQwen:
		settingsPath := qwenSettingsPath(homeDir)
		return installQwen(settingsPath, dryRun, out)

	case TargetCopilot:
		settingsPath := copilotSettingsPath(homeDir)
		return installCopilot(settingsPath, dryRun, out)

	case TargetAntigravity:
		settingsPath := antigravitySettingsPath(homeDir)
		return installAntigravity(settingsPath, dryRun, out)

	case TargetGemini:
		settingsPath := geminiSettingsPath(homeDir)
		return installGemini(settingsPath, dryRun, out)

	case TargetWindsurf:
		hooksPath := windsurfHooksPath(homeDir)
		return installWindsurf(hooksPath, dryRun, out)

	case TargetContinue, TargetOpenCode, TargetOpenClaw:
		return printGatewayGuide(target, out)

	default:
		return fmt.Errorf(
			"hooks: unknown target %q; valid targets: claude-code, cursor, codex, augment, codebuddy, qwen, copilot, antigravity, gemini, windsurf, continue, opencode, openclaw",
			target,
		)
	}
}

// Uninstall removes Beekeeper hooks for the given target.
//
//   - dryRun: print what would be removed without touching any file.
//
// Gateway targets (continue, opencode, openclaw) do not write files, so
// Uninstall is a no-op for them (prints a message).
func Uninstall(target string, dryRun bool) error {
	return UninstallTo(target, dryRun, os.Stdout)
}

// UninstallTo is the testable variant of Uninstall that accepts an io.Writer.
func UninstallTo(target string, dryRun bool, out io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("hooks: could not determine home directory: %w", err)
	}

	switch target {
	case TargetClaudeCode:
		settingsPath := claudeSettingsPath(homeDir)
		return uninstallClaudeCode(settingsPath, dryRun, out)

	case TargetCursor:
		hooksPath := cursorHooksPath(homeDir)
		return uninstallCursor(hooksPath, dryRun, out)

	case TargetCodex:
		hooksPath := codexHooksPath(homeDir)
		return uninstallCodex(hooksPath, dryRun, out)

	case TargetAugment:
		settingsPath := augmentSettingsPath(homeDir)
		return uninstallAugment(settingsPath, dryRun, out)

	case TargetCodeBuddy:
		settingsPath := codebuddySettingsPath(homeDir)
		return uninstallCodeBuddy(settingsPath, dryRun, out)

	case TargetQwen:
		settingsPath := qwenSettingsPath(homeDir)
		return uninstallQwen(settingsPath, dryRun, out)

	case TargetCopilot:
		settingsPath := copilotSettingsPath(homeDir)
		return uninstallCopilot(settingsPath, dryRun, out)

	case TargetAntigravity:
		settingsPath := antigravitySettingsPath(homeDir)
		return uninstallAntigravity(settingsPath, dryRun, out)

	case TargetGemini:
		settingsPath := geminiSettingsPath(homeDir)
		return uninstallGemini(settingsPath, dryRun, out)

	case TargetWindsurf:
		hooksPath := windsurfHooksPath(homeDir)
		return uninstallWindsurf(hooksPath, dryRun, out)

	case TargetContinue, TargetOpenCode, TargetOpenClaw:
		fmt.Fprintf(out, "No files were written for %s — nothing to uninstall.\n", target)
		fmt.Fprintf(out, "Remove the Beekeeper MCP server entry from your %s configuration manually.\n", target)
		return nil

	default:
		return fmt.Errorf(
			"hooks: unknown target %q; valid targets: claude-code, cursor, codex, augment, codebuddy, qwen, copilot, antigravity, gemini, windsurf, continue, opencode, openclaw",
			target,
		)
	}
}

// backupSettings copies the file at path to path + ".beekeeper-backup-<timestamp>".
// If the file does not exist, backupSettings returns nil (not an error — the
// installer will create the file fresh).
//
// WR-06: backup files are written with 0o600 (not 0o644) so sensitive
// settings content (which may include tokens or credentials) is not
// world-readable on multi-user systems.
func backupSettings(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("hooks: backup read %q: %w", path, err)
	}
	ts := time.Now().Format("20060102-150405")
	backupPath := path + ".beekeeper-backup-" + ts
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return fmt.Errorf("hooks: backup write %q: %w", backupPath, err)
	}
	return nil
}

// writeFileAtomic writes data to a temp file in the same directory then
// renames it over path so readers never observe a partially-written file.
//
// CR-05: use os.CreateTemp with a randomised suffix so concurrent invocations
// do not collide on a fixed ".beekeeper-tmp" name. A deferred os.Remove
// cleans up the temp file if the rename fails (no-op if rename succeeds since
// the name is changed).
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful Rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// printDryRun prints a message showing what would be written to path.
func printDryRun(path string, label string, value any, out io.Writer) {
	fmt.Fprintf(out, "[dry-run] Would write to %s (%s):\n%v\n", path, label, value)
}

// claudeSettingsPath returns the path to Claude Code's settings.json.
func claudeSettingsPath(homeDir string) string {
	return homeDir + "/.claude/settings.json"
}

// cursorHooksPath returns the path to Cursor's hooks.json.
func cursorHooksPath(homeDir string) string {
	return homeDir + "/.cursor/hooks.json"
}

// codexHooksPath returns the path to Codex CLI's hooks.json.
func codexHooksPath(homeDir string) string {
	return homeDir + "/.codex/hooks.json"
}
