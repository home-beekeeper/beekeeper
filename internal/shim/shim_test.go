package shim_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/nudge"
	"github.com/home-beekeeper/beekeeper/internal/shim"
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

// TestShimNudgeBeforeProxy verifies that NudgeCheck (the nudge-before-proxy logic)
// correctly evaluates an npm install command and surfaces an advisory without
// blocking (soft mode). Uses the exported nudge.DetectStateFn seam to inject a
// synthetic pnpm-hardened PMState so no real pnpm binary is required.
//
// This test proves the shim calls nudge (via DetectStateFn) before proxying
// (NUDGE-03: advisory surfaces; NUDGE-04: soft mode does not block; T-08-10b:
// cross-package seam injection).
func TestShimNudgeBeforeProxy(t *testing.T) {
	// Inject synthetic PMState: pnpm installed and hardened.
	orig := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.5.0",
			PnpmHardened:  true,
			NodeVersion:   "22.5.0",
		}
	}
	defer func() { nudge.DetectStateFn = orig }()

	nc := config.DefaultNudgeConfig()
	// Soft mode: advise + proceed, never block.
	nc.Mode = "soft"
	nc.RequireHardened = false

	ctx := context.Background()
	result := shim.NudgeCheck(ctx, "npm install lodash", nc)

	// NudgeCheck must report Applicable=true for an install command.
	if !result.Applicable {
		t.Errorf("NudgeCheck.Applicable = false, want true for npm install")
	}

	// Soft mode with hardened pnpm → Advise, not Block.
	if result.ShouldBlock {
		t.Errorf("NudgeCheck.ShouldBlock = true, want false in soft mode")
	}

	// Decision must be "advise" (pnpm available, soft mode).
	if result.Decision != "advise" {
		t.Errorf("NudgeCheck.Decision = %q, want advise", result.Decision)
	}

	// Advisory message must be non-empty for Advise action.
	if result.Advisory == "" {
		t.Errorf("NudgeCheck.Advisory is empty, want non-empty advisory for Advise action")
	}
}

// TestShimNudgeNonInstallSkipped verifies that NudgeCheck returns Applicable=false
// for non-install commands (npm ls, npm run, etc.) — no exec triggered (Pitfall 2).
func TestShimNudgeNonInstallSkipped(t *testing.T) {
	// DetectStateFn should NOT be called for non-install commands.
	orig := nudge.DetectStateFn
	called := false
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		called = true
		return nudge.PMState{}
	}
	defer func() { nudge.DetectStateFn = orig }()

	nc := config.DefaultNudgeConfig()
	result := shim.NudgeCheck(context.Background(), "npm ls", nc)

	if result.Applicable {
		t.Errorf("NudgeCheck.Applicable = true for npm ls, want false (non-install)")
	}
	if called {
		t.Error("DetectStateFn was called for a non-install command, want skip")
	}
}

// TestShimMultiArgContent verifies TM-A-04: the generated shim script passes
// individual args so that multi-word installs (e.g. npm install left-pad react)
// round-trip correctly through cobra.ArbitraryArgs on the Go side.
//
// On Unix: the script must contain `--args "$@"` so the shell expands each
// argument separately (cobra.ArbitraryArgs collects the overflow as positional args).
// On Windows: the script must NOT use `--args %*` (which produces a single
// space-joined string); instead it must pass %1 as --args and %2..%9 as
// individual positional args.
func TestShimMultiArgContent(t *testing.T) {
	if runtime.GOOS == "windows" {
		shimDir := t.TempDir()

		// We need a real npm in PATH on Windows; use a fake .cmd binary.
		realBinDir := t.TempDir()
		fakeNpm := filepath.Join(realBinDir, "npm.cmd")
		if err := os.WriteFile(fakeNpm, []byte("@echo off\r\necho npm\r\n"), 0755); err != nil {
			t.Fatalf("setup fake npm: %v", err)
		}
		origPath := os.Getenv("PATH")
		t.Setenv("PATH", realBinDir+string(os.PathListSeparator)+origPath)

		var out bytes.Buffer
		if err := shim.Install(shimDir, []string{"npm"}, &out); err != nil {
			t.Fatalf("Install: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(shimDir, "npm.cmd"))
		if err != nil {
			t.Fatalf("read npm.cmd: %v", err)
		}

		// Must NOT use %* for the --args value (that collapses multi-arg into one string).
		// Must use %1 as the --args value and %2..%9 as positional args.
		if strings.Contains(string(content), "--args %*") {
			t.Error("Windows shim must not use '--args %*' (collapses all args into one string); use '--args %1 %2 ... %9' instead")
		}
		if !strings.Contains(string(content), "--args %1 %2") {
			t.Error("Windows shim must pass '--args %1 %2 ...' to enable multi-arg round-trip via cobra.ArbitraryArgs")
		}
	} else {
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

		content, err := os.ReadFile(filepath.Join(shimDir, "npm"))
		if err != nil {
			t.Fatalf("read npm shim: %v", err)
		}

		// Unix shim must use --args "$@" so the shell expands to separate words.
		// cobra.ArbitraryArgs then collects the overflow positional args on the Go side.
		if !strings.Contains(string(content), `--args "$@"`) {
			t.Errorf("Unix shim must use '--args \"$@\"' for correct multi-arg expansion; got:\n%s", string(content))
		}
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
