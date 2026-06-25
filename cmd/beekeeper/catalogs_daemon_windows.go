//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Unprivileged scheduled task for the background catalog sync. Created with
// schtasks for the CURRENT user — RESEARCH Tier-1 pitfall: a standard user can
// create a task ONLY when the run-as-SYSTEM and highest-run-level flags are both
// omitted. We omit both, so no elevation is required (unlike the Windows service
// in protect_windows.go). HOURLY heartbeat; the interval gate in `catalogs sync`
// enforces the configured cadence (D-T1-interval).
const catalogTaskName = "BeekeeperCatalogSync"

func installCatalogDaemon(out io.Writer, selfPath string) error {
	ctx := context.Background()
	// /f overwrites an existing task (idempotent re-install). The run-as-SYSTEM
	// and highest-run-level flags are deliberately omitted → the task runs as the
	// current standard user with no elevation.
	//
	// --background hides the console (the binary self-hides via ShowWindow) and
	// tees output to <state>/logs/sync.log so the hourly heartbeat no longer
	// flashes a blank window. When conhost supports --headless (Windows 11) we run
	// the binary under it for TRUE zero-flash (no window object is ever created);
	// otherwise we fall back to the plain self-hiding form.
	tr := fmt.Sprintf(`"%s" catalogs sync --background`, selfPath)
	if conhost := conhostHeadlessPath(ctx); conhost != "" {
		tr = fmt.Sprintf(`"%s" --headless "%s" catalogs sync --background`, conhost, selfPath)
	}
	args := []string{"/create", "/tn", catalogTaskName, "/tr", tr, "/sc", "HOURLY", "/f"}
	if outB, err := exec.CommandContext(ctx, "schtasks", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks /create failed: %s: %w", strings.TrimSpace(string(outB)), err)
	}
	fmt.Fprintf(out, "Catalog sync daemon installed (schtasks current-user task %q, hourly silent heartbeat).\n", catalogTaskName)
	fmt.Fprintln(out, "  Output is logged to <state>/logs/sync.log; run `beekeeper catalogs status` for the last result.")
	return nil
}

// conhostHeadlessPath returns the path to conhost.exe when it supports the
// --headless flag (Windows 11), else "". Headless conhost runs a console app
// with no window object at all, giving true zero-flash scheduling. Best-effort:
// any probe failure returns "" so the installer falls back to self-hide.
func conhostHeadlessPath(ctx context.Context) string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	conhost := filepath.Join(systemRoot, "System32", "conhost.exe")
	if _, err := os.Stat(conhost); err != nil {
		return ""
	}
	// `conhost.exe --headless --help` exits 0 on builds that support the flag and
	// errors on older builds that do not. Run a trivial no-op under it to probe.
	probe := exec.CommandContext(ctx, conhost, "--headless", "cmd.exe", "/c", "exit")
	if err := probe.Run(); err != nil {
		return ""
	}
	return conhost
}

func uninstallCatalogDaemon(out io.Writer) error {
	ctx := context.Background()
	// Idempotent: ignore the error when the task does not exist.
	_, _ = exec.CommandContext(ctx, "schtasks", "/delete", "/tn", catalogTaskName, "/f").CombinedOutput()
	fmt.Fprintln(out, "Catalog sync daemon uninstalled.")
	return nil
}

func catalogDaemonStatus() (bool, string, error) {
	// schtasks /query exits non-zero when the task is absent; that is "not
	// registered", not an error to surface.
	if _, err := exec.Command("schtasks", "/query", "/tn", catalogTaskName).CombinedOutput(); err != nil {
		return false, "not registered", nil
	}
	return true, "registered (schtasks current-user task)", nil
}
