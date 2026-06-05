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

// clinePreCommand is the command written into the PreToolUse executable script
// (macOS/Linux only). Declared here so the hooks.go dispatch can reference it
// without build errors on Windows.
const clinePreCommand = "beekeeper check --hook cline"

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
