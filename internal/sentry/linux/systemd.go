//go:build linux

package linux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"text/template"
	"time"
)

const unitFileTemplate = `[Unit]
Description=Beekeeper Sentry Daemon
After=network.target
StartLimitIntervalSec=0

[Service]
Type=notify
ExecStart={{.BinPath}} sentry
Restart=on-failure
RestartSec=5s
User=root
CapabilityBoundingSet=CAP_SYS_ADMIN CAP_BPF CAP_NET_ADMIN CAP_DAC_READ_SEARCH
AmbientCapabilities=CAP_SYS_ADMIN CAP_BPF CAP_NET_ADMIN CAP_DAC_READ_SEARCH
SecureBits=keep-caps
ProtectSystem=strict
ProtectHome=read-only
NoNewPrivileges=false
RuntimeDirectory=beekeeper

[Install]
WantedBy=multi-user.target
`

// unitFilePath is the default systemd unit file path. Overridable in tests.
var unitFilePath = "/etc/systemd/system/beekeeper-sentry.service"

// IsSystemdRunning reports whether systemd is the init system by checking
// for the presence of /run/systemd/system.
func IsSystemdRunning() bool {
	fi, err := os.Stat("/run/systemd/system")
	return err == nil && fi.IsDir()
}

// WriteUnitFile renders the systemd unit template with binPath as ExecStart
// and writes it to unitFilePath (default: /etc/systemd/system/beekeeper-sentry.service).
// Returns the path written.
func WriteUnitFile(binPath string) (string, error) {
	tmpl, err := template.New("unit").Parse(unitFileTemplate)
	if err != nil {
		return "", fmt.Errorf("parse unit template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ BinPath string }{binPath}); err != nil {
		return "", fmt.Errorf("render unit template: %w", err)
	}
	if err := os.WriteFile(unitFilePath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("write unit file: %w", err)
	}
	return unitFilePath, nil
}

// SystemctlDaemonReload runs "systemctl daemon-reload".
func SystemctlDaemonReload(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "systemctl", "daemon-reload").CombinedOutput()
	if err != nil {
		return fmt.Errorf("daemon-reload: %w: %s", err, out)
	}
	return nil
}

// SystemctlEnableNow runs "systemctl enable --now <unit>".
func SystemctlEnableNow(ctx context.Context, unit string) error {
	out, err := exec.CommandContext(ctx, "systemctl", "enable", "--now", unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("enable --now %s: %w: %s", unit, err, out)
	}
	return nil
}

// SystemctlDisableNow runs "systemctl disable --now <unit>".
func SystemctlDisableNow(ctx context.Context, unit string) error {
	out, err := exec.CommandContext(ctx, "systemctl", "disable", "--now", unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("disable --now %s: %w: %s", unit, err, out)
	}
	return nil
}

// SystemctlIsActive reports whether a systemd unit is active.
// Returns (true, nil) if active, (false, nil) if inactive, or (false, err) on
// unexpected errors.
func SystemctlIsActive(ctx context.Context, unit string) (bool, error) {
	err := exec.CommandContext(ctx, "systemctl", "is-active", unit).Run()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, err
}

// WaitForSocket polls path at 200 ms intervals until the socket file appears or
// timeout elapses. Returns a non-nil error if the deadline is exceeded.
func WaitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for socket %s", path)
}
