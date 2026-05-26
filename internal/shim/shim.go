// Package shim creates and manages OS-native wrapper scripts placed in
// ~/.beekeeper/shims/ that intercept package manager and toolchain invocations
// by invoking beekeeper check before forwarding to the real binary.
//
// Purpose: INTG-06. PATH-prepended shims provide universal coverage for any
// agent that executes package managers directly (npm, pip, cargo, etc.) without
// going through a hook or MCP gateway. This completes the defense-in-depth
// story: native hooks cover hook-enabled agents, the MCP gateway covers MCP
// clients, and shims cover everything else.
//
// CONCURRENCY NOTE: findRealBinary temporarily modifies os.Getenv("PATH") via
// os.Setenv. This is not goroutine-safe. It is only called from shim install,
// which is a synchronous single-goroutine CLI command. If called from a
// concurrent context in future, protect with a sync.Mutex.
package shim

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultTools is the canonical list of package managers and toolchains that
// beekeeper shims by default (INTG-06 — locked).
var DefaultTools = []string{
	"npm", "pnpm", "pip", "cargo", "go", "gem", "composer", "npx", "pipx",
}

// osLookPath is the exec.LookPath function used by findRealBinary. It is a
// package-level variable so tests can substitute a fake without a real binary
// in PATH.
var osLookPath = exec.LookPath

// Install creates wrapper scripts in shimDir for each tool in tools. The shim
// directory is created if it does not already exist. Tools not found in PATH
// are reported to out and silently skipped — this is not an error.
//
// Install is idempotent: re-running overwrites existing shim files with the
// current real binary path.
//
// After creating shims, Install prints PATH instructions for bash, zsh, fish,
// and PowerShell to out.
func Install(shimDir string, tools []string, out io.Writer) error {
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return fmt.Errorf("shim: create shim directory: %w", err)
	}

	for _, tool := range tools {
		realBin, err := findRealBinary(shimDir, tool)
		if err != nil {
			// Tool not in PATH — silently skip; not an error.
			fmt.Fprintf(out, "  %s: not found, skipping\n", tool)
			continue
		}

		if err := writeShellScript(shimDir, tool, realBin); err != nil {
			return fmt.Errorf("shim: write script for %s: %w", tool, err)
		}
		fmt.Fprintf(out, "  %s: shimmed → %s\n", tool, realBin)
	}

	printPathInstructions(shimDir, out)
	return nil
}

// Uninstall removes all files in shimDir. If shimDir does not exist, Uninstall
// returns nil (idempotent). It returns the first file-removal error encountered.
func Uninstall(shimDir string) error {
	entries, err := os.ReadDir(shimDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("shim: read shim directory: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(shimDir, entry.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("shim: remove %s: %w", path, err)
		}
	}
	return nil
}

// Status reports, for each tool in tools, whether a shim file exists in shimDir.
// On Unix the shim file has no extension (e.g. "npm"); on Windows it has the
// ".cmd" extension (e.g. "npm.cmd").
func Status(shimDir string, tools []string, out io.Writer) error {
	for _, tool := range tools {
		shimFile := shimFilePath(shimDir, tool)
		if _, err := os.Stat(shimFile); err == nil {
			fmt.Fprintf(out, "  %s: shimmed\n", tool)
		} else {
			fmt.Fprintf(out, "  %s: not shimmed\n", tool)
		}
	}
	return nil
}

// FindRealBinary finds the absolute path of tool on PATH, excluding shimDir.
// This prevents a shim from resolving to itself and causing an infinite loop.
// It temporarily removes shimDir from PATH, calls exec.LookPath, then restores
// the original PATH.
//
// CONCURRENCY NOTE: This function is not goroutine-safe (os.Setenv is
// process-wide). It is safe to call from a single-goroutine CLI command.
func FindRealBinary(shimDir, tool string) (string, error) {
	return findRealBinary(shimDir, tool)
}

// findRealBinary is the internal implementation called by Install and FindRealBinary.
func findRealBinary(shimDir, tool string) (string, error) {
	origPath := os.Getenv("PATH")
	filteredPath := filterPathEntries(origPath, shimDir)

	os.Setenv("PATH", filteredPath) //nolint:errcheck
	defer os.Setenv("PATH", origPath) //nolint:errcheck

	return osLookPath(tool)
}

// filterPathEntries returns the PATH with all entries equal to exclude removed.
// Path separator is os.PathListSeparator (":" on Unix, ";" on Windows).
// Comparison is case-insensitive on Windows via filepath.Clean.
func filterPathEntries(path, exclude string) string {
	sep := string(os.PathListSeparator)
	parts := strings.Split(path, sep)
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if filepath.Clean(p) != filepath.Clean(exclude) {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, sep)
}

// printPathInstructions writes shell-specific PATH prepend instructions for
// bash, zsh, fish, and PowerShell to out.
func printPathInstructions(shimDir string, out io.Writer) {
	fmt.Fprintf(out, "\nAdd the shim directory to the beginning of your PATH:\n\n")
	fmt.Fprintf(out, "  bash/zsh:   export PATH=%q:$PATH\n", shimDir)
	fmt.Fprintf(out, "  fish:       fish_add_path --prepend %q\n", shimDir)
	fmt.Fprintf(out, "  PowerShell: $env:PATH = %q + ';' + $env:PATH\n", shimDir)
	fmt.Fprintf(out, "\nAdd the appropriate line to your shell RC file (~/.bashrc, ~/.zshrc, etc.)\n")
}
