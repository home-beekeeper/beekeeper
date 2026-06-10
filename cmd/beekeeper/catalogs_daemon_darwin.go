//go:build darwin

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Unprivileged launchd LaunchAgent for the background catalog sync. Written to
// ~/Library/LaunchAgents and loaded into the per-user gui domain via
// `launchctl bootstrap gui/$(id -u)` (no root, unlike protect_darwin.go's
// /Library/LaunchDaemons). StartInterval fires every hour; the interval gate in
// `catalogs sync` enforces the configured cadence (D-T1-interval).
const catalogLaunchdLabel = "com.mzansi.beekeeper.catalog-sync"

func catalogPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", catalogLaunchdLabel+".plist"), nil
}

func installCatalogDaemon(out io.Writer, selfPath string) error {
	plistPath, err := catalogPlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>catalogs</string>
    <string>sync</string>
  </array>
  <key>StartInterval</key><integer>3600</integer>
  <key>RunAtLoad</key><true/>
</dict>
</plist>
`, catalogLaunchdLabel, selfPath)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	ctx := context.Background()
	domain := "gui/" + strconv.Itoa(os.Getuid())
	// bootout first so a re-install is idempotent (ignore error if not loaded).
	_ = exec.CommandContext(ctx, "launchctl", "bootout", domain, plistPath).Run()
	if outB, err := exec.CommandContext(ctx, "launchctl", "bootstrap", domain, plistPath).CombinedOutput(); err != nil {
		// Fallback to the legacy load path for older macOS.
		if outL, lerr := exec.CommandContext(ctx, "launchctl", "load", "-w", plistPath).CombinedOutput(); lerr != nil {
			return fmt.Errorf("launchctl bootstrap/load failed: %s / %s: %w",
				strings.TrimSpace(string(outB)), strings.TrimSpace(string(outL)), lerr)
		}
	}

	fmt.Fprintf(out, "Catalog sync daemon installed (LaunchAgent, StartInterval 3600s).\n")
	fmt.Fprintf(out, "  Plist: %s\n", plistPath)
	return nil
}

func uninstallCatalogDaemon(out io.Writer) error {
	plistPath, err := catalogPlistPath()
	if err != nil {
		return err
	}
	ctx := context.Background()
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.CommandContext(ctx, "launchctl", "bootout", domain, plistPath).Run()
	_ = exec.CommandContext(ctx, "launchctl", "unload", plistPath).Run()
	_ = os.Remove(plistPath)
	fmt.Fprintln(out, "Catalog sync daemon uninstalled.")
	return nil
}

func catalogDaemonStatus() (bool, string, error) {
	out, err := exec.Command("launchctl", "list").CombinedOutput()
	if err != nil {
		return false, "launchctl unavailable", nil
	}
	if strings.Contains(string(out), catalogLaunchdLabel) {
		return true, "loaded (launchd LaunchAgent)", nil
	}
	return false, "not loaded", nil
}
