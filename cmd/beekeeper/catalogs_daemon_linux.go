//go:build linux

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

// Unprivileged systemd --user units for the background catalog sync. Modeled on
// protect_linux.go's privileged unit handling but written to the user systemd
// directory and enabled with `systemctl --user` (no root). Per RESEARCH Tier-1,
// user timers stop at logout unless lingering is enabled — acceptable for v1
// (the agent only runs while the user is active).
const (
	catalogServiceUnit = "beekeeper-catalog-sync.service"
	catalogTimerUnit   = "beekeeper-catalog-sync.timer"
)

func userSystemdDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func runUserSystemctl(ctx context.Context, args ...string) error {
	full := append([]string{"--user"}, args...)
	out, err := exec.CommandContext(ctx, "systemctl", full...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

func installCatalogDaemon(out io.Writer, selfPath string) error {
	dir, err := userSystemdDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create systemd user dir %q: %w", dir, err)
	}

	service := fmt.Sprintf(`[Unit]
Description=Beekeeper background catalog sync

[Service]
Type=oneshot
ExecStart=%s catalogs sync --background
`, selfPath)
	timer := `[Unit]
Description=Beekeeper hourly catalog-sync heartbeat

[Timer]
OnBootSec=2min
OnUnitActiveSec=1h
Persistent=true

[Install]
WantedBy=timers.target
`
	if err := os.WriteFile(filepath.Join(dir, catalogServiceUnit), []byte(service), 0o644); err != nil {
		return fmt.Errorf("write service unit: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, catalogTimerUnit), []byte(timer), 0o644); err != nil {
		return fmt.Errorf("write timer unit: %w", err)
	}

	ctx := context.Background()
	if err := runUserSystemctl(ctx, "daemon-reload"); err != nil {
		return err
	}
	if err := runUserSystemctl(ctx, "enable", "--now", catalogTimerUnit); err != nil {
		return err
	}

	fmt.Fprintf(out, "Catalog sync daemon installed (systemd --user timer, hourly heartbeat).\n")
	fmt.Fprintf(out, "  Units: %s\n", dir)
	fmt.Fprintf(out, "  Note: user timers stop at logout unless lingering is enabled (loginctl enable-linger).\n")
	return nil
}

func uninstallCatalogDaemon(out io.Writer) error {
	ctx := context.Background()
	_ = runUserSystemctl(ctx, "disable", "--now", catalogTimerUnit)
	if dir, err := userSystemdDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, catalogTimerUnit))
		_ = os.Remove(filepath.Join(dir, catalogServiceUnit))
	}
	_ = runUserSystemctl(ctx, "daemon-reload")
	fmt.Fprintln(out, "Catalog sync daemon uninstalled.")
	return nil
}

func catalogDaemonStatus() (bool, string, error) {
	// `systemctl --user is-enabled` exits non-zero when the unit is not enabled;
	// that is "not installed", not an error to surface.
	out, _ := exec.Command("systemctl", "--user", "is-enabled", catalogTimerUnit).CombinedOutput()
	state := strings.TrimSpace(string(out))
	if state == "enabled" || state == "enabled-runtime" {
		return true, state, nil
	}
	if state == "" {
		state = "disabled"
	}
	return false, state, nil
}
