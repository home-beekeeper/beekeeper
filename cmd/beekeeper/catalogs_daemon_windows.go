//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
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
	tr := fmt.Sprintf(`"%s" catalogs sync`, selfPath)
	args := []string{"/create", "/tn", catalogTaskName, "/tr", tr, "/sc", "HOURLY", "/f"}
	if outB, err := exec.CommandContext(ctx, "schtasks", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks /create failed: %s: %w", strings.TrimSpace(string(outB)), err)
	}
	fmt.Fprintf(out, "Catalog sync daemon installed (schtasks current-user task %q, hourly heartbeat).\n", catalogTaskName)
	return nil
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
