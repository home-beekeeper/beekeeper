//go:build !windows

package shim

import (
	"fmt"
	"os"
	"path/filepath"
)

// shimFilePath returns the path to the shim file for tool on this OS.
// On Unix, shim files have no extension (e.g. ~/.beekeeper/shims/npm).
func shimFilePath(shimDir, tool string) string {
	return filepath.Join(shimDir, tool)
}

// writeShellScript creates a POSIX shell wrapper script at shimDir/tool.
// The script invokes beekeeper check with the tool call JSON on stdin. If the
// check exits 0 (allow), the script uses exec to replace itself with the real
// binary, preserving signal handling and the real binary's exit code. The file
// is written with mode 0755 (executable by owner, group, and others).
//
// CR-03: arguments are passed to beekeeper check as separate positional
// parameters via --tool and --args flags rather than embedded in a heredoc
// JSON string. This avoids shell injection through $* expansion and JSON
// corruption from arguments containing quotes, backslashes, or newlines.
// The tool name is a fixed string set at install time (safe to embed).
// beekeeper check --tool/--args constructs the JSON with proper json.Marshal.
//
// Template: RESEARCH Pattern 9 (VERIFIED — INTG-06).
func writeShellScript(shimDir, tool, realBin string) error {
	content := fmt.Sprintf(`#!/bin/sh
# beekeeper shim for %s — auto-generated, do not edit
# Real binary: %s
# This file is managed by 'beekeeper shim install'. Modifying it is unsupported.
beekeeper check --tool "%s" --args "$@"
_bk_exit=$?
if [ $_bk_exit -eq 0 ]; then
    exec "%s" "$@"
fi
exit $_bk_exit
`, tool, realBin, tool, realBin)

	shimPath := filepath.Join(shimDir, tool)
	// Mode 0755 sets the executable bit — no separate Chmod call needed.
	if err := os.WriteFile(shimPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("write shim %s: %w", shimPath, err)
	}
	return nil
}
