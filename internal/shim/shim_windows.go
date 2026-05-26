//go:build windows

package shim

import (
	"fmt"
	"os"
	"path/filepath"
)

// shimFilePath returns the path to the shim file for tool on this OS.
// On Windows, shim files use the ".cmd" extension (e.g. %APPDATA%\beekeeper\shims\npm.cmd).
func shimFilePath(shimDir, tool string) string {
	return filepath.Join(shimDir, tool+".cmd")
}

// writeShellScript creates a Windows .cmd batch file at shimDir/tool.cmd.
// The batch file invokes beekeeper check with the tool name and arguments as
// separate command-line flags rather than embedding them in an echo pipe. This
// avoids cmd.exe command injection via %* expanding to arguments containing
// |, >, <, &, or " characters (CR-04).
//
// CRITICAL: Line endings MUST be CRLF (\r\n) for cmd.exe compatibility
// (T-04-04-05 — Pitfall 8 from RESEARCH). Any LF-only line will cause
// silent exec failure under cmd.exe.
//
// CR-04: beekeeper check --tool <name> --args %* passes arguments as separate
// parameters to beekeeper, which constructs the JSON with json.Marshal in Go
// code rather than embedding raw %* in a shell-interpolated JSON string.
// The tool name is a fixed string set at install time (safe to embed).
//
// Template: RESEARCH Pattern 9 (VERIFIED — INTG-06).
func writeShellScript(shimDir, tool, realBin string) error {
	// Build content using "\r\n" as line separator (NOT "\n").
	// The quoted realBin path mitigates T-04-04-02 (spaces in path).
	content := fmt.Sprintf(
		"@echo off\r\n"+
			"beekeeper check --tool \"%s\" --args %%*\r\n"+
			"if %%ERRORLEVEL%% EQU 0 goto :run\r\n"+
			"exit /b %%ERRORLEVEL%%\r\n"+
			":run\r\n"+
			"\"%s\" %%*\r\n",
		tool, realBin,
	)

	shimPath := filepath.Join(shimDir, tool+".cmd")
	// Mode 0644 — Windows ignores Unix permission bits; cmd.exe executes .cmd files
	// based on extension, not permission bits.
	if err := os.WriteFile(shimPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write shim %s: %w", shimPath, err)
	}
	return nil
}
