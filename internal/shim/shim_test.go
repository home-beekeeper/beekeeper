package shim_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/shim"
)

// TestShimInstallUnix verifies that Install creates an executable shell script
// for npm containing the exec keyword (signal preservation) and the real binary path.
func TestShimInstallUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shim test skipped on Windows")
	}
	shimDir := t.TempDir()

	// Create a real binary in a separate temp dir so LookPath can find it.
	realBinDir := t.TempDir()
	realNpm := filepath.Join(realBinDir, "npm")
	if err := os.WriteFile(realNpm, []byte("#!/bin/sh\necho npm"), 0755); err != nil {
		t.Fatalf("setup real npm: %v", err)
	}

	// Prepend realBinDir to PATH so findRealBinary can locate npm.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", realBinDir+string(os.PathListSeparator)+origPath)

	var out bytes.Buffer
	if err := shim.Install(shimDir, []string{"npm"}, &out); err != nil {
		t.Fatalf("Install: %v", err)
	}

	shimFile := filepath.Join(shimDir, "npm")
	data, err := os.ReadFile(shimFile)
	if err != nil {
		t.Fatalf("shim file not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "exec ") {
		t.Errorf("shim script must contain 'exec ' for signal preservation; got:\n%s", content)
	}
	if !strings.Contains(content, realNpm) {
		t.Errorf("shim script must embed real binary path %q; got:\n%s", realNpm, content)
	}

	// Verify executable bit is set.
	info, err := os.Stat(shimFile)
	if err != nil {
		t.Fatalf("stat shim: %v", err)
	}
	if info.Mode()&0100 == 0 {
		t.Errorf("shim file must be executable; got mode %v", info.Mode())
	}
}

// TestShimInstallWindows verifies that Install creates a .cmd batch file with
// CRLF line endings and a quoted real binary path.
func TestShimInstallWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows shim test skipped on non-Windows")
	}
	shimDir := t.TempDir()

	// On Windows, we need a real npm in PATH. Use the system npm if available;
	// otherwise skip.
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm not found in PATH; skipping Windows shim install test")
	}
	_ = npmPath

	var out bytes.Buffer
	if err := shim.Install(shimDir, []string{"npm"}, &out); err != nil {
		t.Fatalf("Install: %v", err)
	}

	shimFile := filepath.Join(shimDir, "npm.cmd")
	data, err := os.ReadFile(shimFile)
	if err != nil {
		t.Fatalf("shim .cmd file not created: %v", err)
	}

	// Verify CRLF line endings.
	if !bytes.Contains(data, []byte("\r\n")) {
		t.Errorf("Windows .cmd shim must use CRLF line endings; content:\n%s", data)
	}

	// Verify quoted real binary path (double-quote around path in :run section).
	content := string(data)
	if !strings.Contains(content, `"`) {
		t.Errorf("Windows .cmd shim must quote the real binary path; got:\n%s", content)
	}
}

// TestShimRealBinary verifies that findRealBinary (exercised via Install) excludes
// the shim directory from PATH so that a fake shim binary is not resolved as the
// "real" binary.
func TestShimRealBinary(t *testing.T) {
	// On Windows, exec.LookPath requires a file to have a recognized executable
	// extension (.exe, .cmd, .bat). We use ".cmd" so the test works on Windows
	// without requiring .exe compilation, while ".sh" provides a recognizable
	// script on Unix. The tool name used in FindRealBinary must match the base
	// name without extension (LookPath appends PATHEXT on Windows automatically).
	var ext, content string
	if runtime.GOOS == "windows" {
		ext = ".cmd"
		content = "@echo off\r\necho fake\r\n"
	} else {
		ext = ""
		content = "#!/bin/sh\necho fake\n"
	}

	toolName := "testbinshim204" // unlikely to exist in PATH

	// Create a shim dir with a fake binary inside.
	shimDir := t.TempDir()
	fakeInShim := filepath.Join(shimDir, toolName+ext)
	if err := os.WriteFile(fakeInShim, []byte(content), 0755); err != nil {
		t.Fatalf("setup fake shim binary: %v", err)
	}

	// Create a separate dir with the "real" binary.
	realBinDir := t.TempDir()
	realBin := filepath.Join(realBinDir, toolName+ext)
	var realContent string
	if runtime.GOOS == "windows" {
		realContent = "@echo off\r\necho real\r\n"
	} else {
		realContent = "#!/bin/sh\necho real\n"
	}
	if err := os.WriteFile(realBin, []byte(realContent), 0755); err != nil {
		t.Fatalf("setup real binary: %v", err)
	}

	// Set PATH so shimDir comes first, then realBinDir.
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+realBinDir)

	// findRealBinary should skip shimDir and return the binary from realBinDir.
	// On Windows we look up toolName (LookPath will append .cmd from PATHEXT).
	found, err := shim.FindRealBinary(shimDir, toolName)
	if err != nil {
		t.Fatalf("FindRealBinary: %v", err)
	}

	// found must NOT be inside shimDir.
	if strings.HasPrefix(filepath.Clean(found), filepath.Clean(shimDir)) {
		t.Errorf("FindRealBinary returned path inside shimDir %q; must exclude shimDir; got %q", shimDir, found)
	}
	// found must be inside realBinDir.
	if !strings.HasPrefix(filepath.Clean(found), filepath.Clean(realBinDir)) {
		t.Errorf("FindRealBinary: want path inside %q, got %q", realBinDir, found)
	}
}

// TestShimUninstall verifies that Uninstall removes all files in the shim directory.
func TestShimUninstall(t *testing.T) {
	shimDir := t.TempDir()

	// Create two fake shim files.
	for _, name := range []string{"npm", "pip"} {
		if err := os.WriteFile(filepath.Join(shimDir, name), []byte("fake"), 0644); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}

	if err := shim.Uninstall(shimDir); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	entries, err := os.ReadDir(shimDir)
	if err != nil {
		t.Fatalf("ReadDir after uninstall: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Uninstall should remove all shim files; %d files remain", len(entries))
	}
}

// TestShimIdempotent verifies that calling Install twice does not error and
// results in exactly one shim file (second call overwrites the first).
func TestShimIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Idempotent test uses Unix shim; skipped on Windows")
	}
	shimDir := t.TempDir()

	realBinDir := t.TempDir()
	realNpm := filepath.Join(realBinDir, "npm")
	if err := os.WriteFile(realNpm, []byte("#!/bin/sh\necho npm"), 0755); err != nil {
		t.Fatalf("setup real npm: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", realBinDir+string(os.PathListSeparator)+origPath)

	var out bytes.Buffer
	for i := 0; i < 2; i++ {
		if err := shim.Install(shimDir, []string{"npm"}, &out); err != nil {
			t.Fatalf("Install call %d: %v", i+1, err)
		}
	}

	entries, err := os.ReadDir(shimDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("after two Install calls, want 1 shim file, got %d", len(entries))
	}
}

// TestShimStatus verifies that Status reports "shimmed" for existing shim files
// and "not shimmed" for missing ones.
func TestShimStatus(t *testing.T) {
	shimDir := t.TempDir()

	// Create a fake npm shim (extension depends on OS).
	var shimName string
	if runtime.GOOS == "windows" {
		shimName = "npm.cmd"
	} else {
		shimName = "npm"
	}
	if err := os.WriteFile(filepath.Join(shimDir, shimName), []byte("fake"), 0644); err != nil {
		t.Fatalf("setup shim file: %v", err)
	}

	var out bytes.Buffer
	if err := shim.Status(shimDir, []string{"npm", "pip"}, &out); err != nil {
		t.Fatalf("Status: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "npm") || !strings.Contains(output, "shimmed") {
		t.Errorf("Status output must report npm as shimmed; got:\n%s", output)
	}
	// pip shim not created, must report not shimmed.
	if !strings.Contains(output, "pip") || !strings.Contains(output, "not shimmed") {
		t.Errorf("Status output must report pip as not shimmed; got:\n%s", output)
	}
}

// TestShimToolNotFound verifies that Install silently skips tools not found in PATH
// and reports them in the output without returning an error.
func TestShimToolNotFound(t *testing.T) {
	shimDir := t.TempDir()

	var out bytes.Buffer
	// "nonexistent-beekeeper-tool" is guaranteed not to be in PATH.
	err := shim.Install(shimDir, []string{"nonexistent-beekeeper-tool"}, &out)
	if err != nil {
		t.Fatalf("Install must not error when tool is not in PATH; got: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "not found") {
		t.Errorf("Install must report not-found tools; got:\n%s", output)
	}

	// No shim file should have been created.
	entries, _ := os.ReadDir(shimDir)
	if len(entries) != 0 {
		t.Errorf("Install must not create shim for missing tool; found %d files", len(entries))
	}
}

// TestShimInstallCreatesDir verifies that Install creates shimDir if it does not exist.
func TestShimInstallCreatesDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Dir creation test uses Unix shim; skipped on Windows")
	}
	// Use a non-existent subdir within a temp dir.
	base := t.TempDir()
	shimDir := filepath.Join(base, "newshims")

	realBinDir := t.TempDir()
	realNpm := filepath.Join(realBinDir, "npm")
	if err := os.WriteFile(realNpm, []byte("#!/bin/sh\necho npm"), 0755); err != nil {
		t.Fatalf("setup real npm: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", realBinDir+string(os.PathListSeparator)+origPath)

	var out bytes.Buffer
	if err := shim.Install(shimDir, []string{"npm"}, &out); err != nil {
		t.Fatalf("Install with new shimDir: %v", err)
	}

	if _, err := os.Stat(shimDir); os.IsNotExist(err) {
		t.Errorf("Install must create shimDir; %q does not exist", shimDir)
	}
}

// TestShimPathInstructions verifies that Install prints PATH instructions after creating shims.
func TestShimPathInstructions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH instructions test uses Unix shim; skipped on Windows")
	}
	shimDir := t.TempDir()

	realBinDir := t.TempDir()
	realNpm := filepath.Join(realBinDir, "npm")
	if err := os.WriteFile(realNpm, []byte("#!/bin/sh\necho npm"), 0755); err != nil {
		t.Fatalf("setup real npm: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", realBinDir+string(os.PathListSeparator)+origPath)

	var out bytes.Buffer
	if err := shim.Install(shimDir, []string{"npm"}, &out); err != nil {
		t.Fatalf("Install: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "PATH") {
		t.Errorf("Install must print PATH instructions; got:\n%s", output)
	}
}
