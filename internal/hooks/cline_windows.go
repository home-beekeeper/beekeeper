//go:build windows

package hooks

// Package hooks — cline_windows.go
//
// Cline hooks are macOS/Linux only. On Windows, Cline does not support the
// executable PreToolUse hook mechanism (RESEARCH.md row 4). This stub returns
// a clear error so users receive an explicit message rather than a silent
// no-op.

import (
	"fmt"
	"io"
)

// clineCheckSuffix is the stable suffix for the Cline beekeeper hook command.
// Declared here so the hooks.go dispatch and HookConfigFiles can reference it
// without build errors on Windows. The macOS/Linux implementation is in cline.go.
const clineCheckSuffix = "check --hook cline"

// clineHooksDir returns a sentinel path on Windows. The real implementation
// (cline.go, !windows) returns ~/Documents/Cline/Rules/Hooks.
func clineHooksDir(_ string) string {
	return "~\\Documents\\Cline\\Rules\\Hooks"
}

// installCline returns a clear error on Windows: Cline hooks are macOS/Linux
// only. Users on Windows should use a different harness (e.g. Claude Code,
// Cursor) or the MCP gateway.
func installCline(_ string, _ bool, out io.Writer) error {
	fmt.Fprintf(out, "Cline hooks are macOS/Linux only (not supported on Windows).\n")
	fmt.Fprintf(out, "Use --target claude-code, --target cursor, or --target opencode on Windows.\n")
	return fmt.Errorf("cline hooks are macOS/Linux only (not supported on Windows)")
}

// uninstallCline returns a clear error on Windows.
func uninstallCline(_ string, _ bool, out io.Writer) error {
	fmt.Fprintf(out, "Cline hooks are macOS/Linux only (not supported on Windows) — nothing to uninstall.\n")
	return fmt.Errorf("cline hooks are macOS/Linux only (not supported on Windows)")
}

// containsClineCommand reports whether content contains a beekeeper Cline hook
// marker (stub for Windows; the real implementation is in cline.go !windows).
func containsClineCommand(content string) bool {
	_ = content
	return false
}
