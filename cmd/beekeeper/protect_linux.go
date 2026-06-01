//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/ipc"
	"github.com/bantuson/beekeeper/internal/platform"
	linux "github.com/bantuson/beekeeper/internal/sentry/linux"
)

func runProtectInstall(cmd *cobra.Command, _ []string) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("beekeeper protect install must be run as root (use sudo)")
	}
	if !linux.IsSystemdRunning() {
		return fmt.Errorf("systemd is not running on this system")
	}
	ctx := cmd.Context()
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}
	const installPath = "/usr/local/bin/beekeeper"
	if selfPath != installPath {
		if err := copyFile(selfPath, installPath, 0755); err != nil {
			return fmt.Errorf("copy binary: %w", err)
		}
	}
	if _, err := linux.WriteUnitFile(installPath); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	if err := linux.SystemctlDaemonReload(ctx); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}
	if err := linux.SystemctlEnableNow(ctx, "beekeeper-sentry"); err != nil {
		return fmt.Errorf("enable --now: %w", err)
	}
	stateDir, err := platform.StateDir()
	if err != nil {
		return err
	}
	sockPath := filepath.Join(stateDir, "sentry.sock")
	fmt.Fprintln(cmd.OutOrStdout(), "Waiting for sentry socket...")
	if err := linux.WaitForSocket(sockPath, 10*time.Second); err != nil {
		return fmt.Errorf("wait for socket: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Beekeeper Sentry installed and running.")
	return nil
}

func runProtectUninstall(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	_ = linux.SystemctlDisableNow(ctx, "beekeeper-sentry")
	_ = os.Remove("/etc/systemd/system/beekeeper-sentry.service")
	_ = linux.SystemctlDaemonReload(ctx)
	stateDir, _ := platform.StateDir()
	if stateDir != "" {
		_ = os.Remove(filepath.Join(stateDir, "sentry.sock"))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Beekeeper Sentry uninstalled.")
	return nil
}

func runProtectStatus(cmd *cobra.Command, _ []string) error {
	stateDir, err := platform.StateDir()
	if err != nil {
		return err
	}
	sockPath := filepath.Join(stateDir, "sentry.sock")
	conn, err := ipc.Connect(sockPath, 3*time.Second)
	if err != nil {
		// Fallback: systemctl is-active
		active, _ := linux.SystemctlIsActive(cmd.Context(), "beekeeper-sentry")
		if active {
			fmt.Fprintln(cmd.OutOrStdout(), "Beekeeper Sentry — Active (systemd; IPC unavailable)")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Beekeeper Sentry — Inactive")
		}
		return nil
	}
	defer conn.Close()
	if err := ipc.SendCommand(conn, ipc.IPCCommand{Kind: ipc.CmdStatusRequest}, 3*time.Second); err != nil {
		return fmt.Errorf("send command: %w", err)
	}
	resp, err := ipc.ReadResponse(conn, 3*time.Second)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var sr ipc.StatusResponse
	if err := json.Unmarshal(resp.Payload, &sr); err != nil {
		return fmt.Errorf("unmarshal status: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Beekeeper Sentry — Active (PID %d, uptime %s)\n", sr.DaemonPID, sr.Uptime)
	fmt.Fprintf(out, "Tier:       %s\n", sr.TierReason)
	fmt.Fprintf(out, "Rules:      %d/5 active\n", sr.RulesActive)
	fmt.Fprintf(out, "Events:     %d processed, %d dropped\n", sr.EventsProcessed, sr.EventsDropped)
	fmt.Fprintf(out, "IPC socket: %s\n", sr.SockPath)
	return nil
}

func runSentryDaemon(cmd *cobra.Command, _ []string) error {
	configPath, err := platform.ConfigPath()
	if err != nil {
		return fmt.Errorf("config path: %w", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	auditDir, err := platform.AuditDir()
	if err != nil {
		return err
	}
	auditPath := filepath.Join(auditDir, "beekeeper.ndjson")
	return linux.RunDaemon(ctx, &cfg, auditPath)
}

func runSentryRulesList(cmd *cobra.Command, _ []string) error {
	stateDir, err := platform.StateDir()
	if err != nil {
		return err
	}
	conn, err := ipc.Connect(filepath.Join(stateDir, "sentry.sock"), 3*time.Second)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	if err := ipc.SendCommand(conn, ipc.IPCCommand{Kind: ipc.CmdRulesListRequest}, 3*time.Second); err != nil {
		return err
	}
	resp, err := ipc.ReadResponse(conn, 3*time.Second)
	if err != nil {
		return err
	}
	var rl ipc.RulesListResponse
	if err := json.Unmarshal(resp.Payload, &rl); err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%-12s %-30s %-8s %s\n", "ID", "Name", "Enabled", "Severity")
	for _, r := range rl.Rules {
		fmt.Fprintf(out, "%-12s %-30s %-8v %s\n", r.ID, r.Name, r.Enabled, r.Severity)
	}
	return nil
}

func runSentryRulesEnable(cmd *cobra.Command, args []string) error {
	stateDir, err := platform.StateDir()
	if err != nil {
		return err
	}
	conn, err := ipc.Connect(filepath.Join(stateDir, "sentry.sock"), 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = ipc.SendCommand(conn, ipc.IPCCommand{Kind: ipc.CmdRulesEnableRequest, RuleID: args[0]}, 3*time.Second)
	fmt.Fprintf(cmd.OutOrStdout(), "Rule %s enabled.\n", args[0])
	return nil
}

func runSentryRulesDisable(cmd *cobra.Command, args []string) error {
	stateDir, err := platform.StateDir()
	if err != nil {
		return err
	}
	conn, err := ipc.Connect(filepath.Join(stateDir, "sentry.sock"), 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = ipc.SendCommand(conn, ipc.IPCCommand{Kind: ipc.CmdRulesDisableRequest, RuleID: args[0]}, 3*time.Second)
	fmt.Fprintf(cmd.OutOrStdout(), "Rule %s disabled.\n", args[0])
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
