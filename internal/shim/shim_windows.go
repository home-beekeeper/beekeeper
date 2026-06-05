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
// CR-04 / TM-A-04 fix: pass %1 as the --args flag value and %2..%9 as
// individual positional arguments. cobra.ArbitraryArgs (main.go) collects
// those positional args and appends them to allArgs, so a multi-word install
//   npm install left-pad react
// produces toolArgs=["install"] + positional=["left-pad","react"] →
// "args": ["install","left-pad","react"] in the ToolCall JSON.
//
// Supports up to 9 arguments per invocation (cmd.exe %1..%9 limit).
// Unset %N tokens expand to empty and are skipped by cobra's arg parser.
// The tool name is a fixed string set at install time (safe to embed).
//
// Template: RESEARCH Pattern 9 (VERIFIED — INTG-06).
func writeShellScript(shimDir, tool, realBin string) error {
	// Build content using "\r\n" as line separator (NOT "\n").
	// The quoted realBin path mitigates T-04-04-02 (spaces in path).
	//
	// TM-A-04: %1 is passed as the --args flag value; %2 through %9 become
	// cobra positional args (cobra.ArbitraryArgs) and are appended to allArgs
	// on the Go side before json.Marshal. Empty %N tokens (when fewer than 9
	// args are provided) produce no tokens — cmd.exe skips whitespace-only
	// portions of the command line. The :run section forwards the original
	// full argv (%*) to the real binary unchanged.
	content := fmt.Sprintf(
		"@echo off\r\n"+
			"beekeeper check --tool \"%s\" --args %%1 %%2 %%3 %%4 %%5 %%6 %%7 %%8 %%9\r\n"+
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
