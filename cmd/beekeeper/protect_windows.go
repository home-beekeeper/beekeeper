//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"

	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/ipc"
	"github.com/bantuson/beekeeper/internal/platform"
	winsentry "github.com/bantuson/beekeeper/internal/sentry/windows"
)

// installPath is the canonical location for the installed Beekeeper binary on Windows.
const installPath = `C:\Program Files\Beekeeper\beekeeper.exe`

// isElevated returns true when the current process is running with a UAC
// elevated token (Administrator). Uses GetCurrentProcessToken().IsElevated()
// from golang.org/x/sys/windows — no unsafe pointer arithmetic needed.
func isElevated() bool {
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}

// copyFileWindows copies src to dst with the given mode. Defined locally on
// windows to avoid duplicate symbol with protect_linux.go's copyFile.
func copyFileWindows(src, dst string, mode os.FileMode) error {
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

func runProtectInstall(cmd *cobra.Command, _ []string) error {
	if !isElevated() {
		return errors.New("beekeeper protect install must be run as Administrator")
	}
	ctx := cmd.Context()

	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	// Create the installation directory and copy the binary.
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}
	if selfPath != installPath {
		if err := copyFileWindows(selfPath, installPath, 0755); err != nil {
			return fmt.Errorf("copy binary: %w", err)
		}
	}

	// Install and start the Windows Service.
	if err := winsentry.InstallService(installPath); err != nil {
		return fmt.Errorf("install service: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Waiting for IPC named pipe...")
	if err := winsentry.WaitForPipe(10 * time.Second); err != nil {
		return fmt.Errorf("wait for pipe: %w", err)
	}

	// SWIN-03: surface NT Kernel Logger conflict status at install time.
	conflict, probeErr := winsentry.ProbeKernelLoggerConflict(ctx)
	if probeErr != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "NT Kernel Logger probe error: %v\n", probeErr)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), winsentry.ConflictMessage(conflict))
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Beekeeper Sentry installed and running.")
	return nil
}

func runProtectUninstall(cmd *cobra.Command, _ []string) error {
	if !isElevated() {
		return errors.New("beekeeper protect uninstall must be run as Administrator")
	}
	_ = winsentry.UninstallService() // idempotent
	fmt.Fprintln(cmd.OutOrStdout(), "Beekeeper Sentry uninstalled.")
	return nil
}

func runProtectStatus(cmd *cobra.Command, _ []string) error {
	conn, err := ipc.Connect("", 3*time.Second)
	if err != nil {
		// IPC unavailable — fall back to SCM query.
		running, statusText, _ := winsentry.QueryService(cmd.Context())
		if running {
			fmt.Fprintln(cmd.OutOrStdout(), "Beekeeper Sentry — Active (Windows Service; IPC unavailable)")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Beekeeper Sentry — %s\n", statusText)
		}
		// SWIN-03: always surface conflict status even without IPC.
		conflict, _ := winsentry.ProbeKernelLoggerConflict(cmd.Context())
		fmt.Fprintln(cmd.OutOrStdout(), winsentry.ConflictMessage(conflict))
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
	// SWIN-04 surfacing: "lost" is the Windows-canonical term for EventsLost.
	fmt.Fprintf(out, "Events:     %d processed, %d lost\n", sr.EventsProcessed, sr.EventsDropped)
	fmt.Fprintf(out, "IPC pipe:   %s\n", sr.SockPath)
	// TM-RS-03: baseline status must be surfaced prominently so permanent
	// learn-only mode (quarantine suppressed) cannot mask enforcement silently.
	if sr.BaselineActive {
		if sr.BaselinePermanent {
			fmt.Fprintf(out, "Baseline:   PERMANENT LEARNING MODE — quarantine suppressed indefinitely (duration_days<0); set duration_days>=0 to enable quarantine\n")
		} else {
			fmt.Fprintf(out, "Baseline:   active (%d day(s) remaining) — quarantine suppressed during learning window\n", sr.BaselineDaysLeft)
		}
	} else {
		fmt.Fprintf(out, "Baseline:   inactive — full enforcement (quarantine enabled)\n")
	}
	fmt.Fprintln(out)

	// SWIN-03 surfacing: re-probe at status-time to capture transient state.
	conflict, _ := winsentry.ProbeKernelLoggerConflict(cmd.Context())
	fmt.Fprintln(out, winsentry.ConflictMessage(conflict))

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
	return winsentry.RunDaemon(ctx, &cfg, auditPath)
}

func runSentryRulesList(cmd *cobra.Command, _ []string) error {
	// ipc.Connect("", ...) on Windows resolves to ipc.PipePath internally.
	conn, err := ipc.Connect("", 3*time.Second)
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
	conn, err := ipc.Connect("", 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = ipc.SendCommand(conn, ipc.IPCCommand{Kind: ipc.CmdRulesEnableRequest, RuleID: args[0]}, 3*time.Second)
	fmt.Fprintf(cmd.OutOrStdout(), "Rule %s enabled.\n", args[0])
	return nil
}

func runSentryRulesDisable(cmd *cobra.Command, args []string) error {
	conn, err := ipc.Connect("", 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = ipc.SendCommand(conn, ipc.IPCCommand{Kind: ipc.CmdRulesDisableRequest, RuleID: args[0]}, 3*time.Second)
	fmt.Fprintf(cmd.OutOrStdout(), "Rule %s disabled.\n", args[0])
	return nil
}
