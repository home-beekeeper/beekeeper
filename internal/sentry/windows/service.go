//go:build windows

package windows

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/bantuson/beekeeper/internal/ipc"
)

const (
	// ServiceName is the Windows Service Control Manager name for Beekeeper Sentry.
	ServiceName = "BeekeeperSentry"
	// ServiceDisplayName is the human-readable display name shown in services.msc.
	ServiceDisplayName = "Beekeeper Sentry Daemon"
	// ServiceDescription is the longer description shown in services.msc.
	ServiceDescription = "Beekeeper OS-level security monitoring (ETW event ingestion)"
)

// defaultServiceConfig returns the mgr.Config used by InstallService.
// Extracted as a function so service_test.go can assert the config fields
// without requiring admin (no SCM connection needed).
func defaultServiceConfig(exePath string) mgr.Config {
	return mgr.Config{
		ServiceType:      windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:        mgr.StartAutomatic,
		ErrorControl:     mgr.ErrorNormal,
		BinaryPathName:   exePath,
		ServiceStartName: `NT AUTHORITY\LocalService`,
		DisplayName:      ServiceDisplayName,
		Description:      ServiceDescription,
	}
}

// InstallService creates and starts the BeekeeperSentry Windows Service.
// The service runs the binary at exePath under NT AUTHORITY\LocalService with
// automatic startup. Returns an error if the service already exists.
// Requires administrator privileges (caller must check before calling).
func InstallService(exePath string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect SCM: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck

	// Detect if already installed (idempotent guard).
	s, err := m.OpenService(ServiceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %q already installed", ServiceName)
	}

	cfg := defaultServiceConfig(exePath)
	// CreateService accepts the config and optional arguments.
	// "sentry" is the Cobra subcommand that the service binary runs on start.
	s, err = m.CreateService(ServiceName, exePath, cfg, "sentry")
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	return s.Start()
}

// UninstallService stops and deletes the BeekeeperSentry Windows Service.
// The operation is idempotent: if the service does not exist, nil is returned.
// Requires administrator privileges (caller must check before calling).
func UninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect SCM: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck

	s, err := m.OpenService(ServiceName)
	if err != nil {
		// ERROR_SERVICE_DOES_NOT_EXIST (1060) — already uninstalled, treat as success.
		if isServiceNotExist(err) {
			return nil
		}
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()

	// Best-effort stop; ignore errors (service may already be stopped).
	status, _ := s.Query()
	if status.State != svc.Stopped {
		_, _ = s.Control(svc.Stop)
		time.Sleep(2 * time.Second)
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// QueryService returns the running state and a textual status of the
// BeekeeperSentry Windows Service. If the service is not installed,
// running is false and statusText is "not_installed".
func QueryService(ctx context.Context) (running bool, statusText string, err error) {
	_ = ctx // reserved for future cancellation support

	m, err := mgr.Connect()
	if err != nil {
		return false, "", fmt.Errorf("connect SCM: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck

	s, err := m.OpenService(ServiceName)
	if err != nil {
		if isServiceNotExist(err) {
			return false, "not_installed", nil
		}
		return false, "unknown", fmt.Errorf("open service: %w", err)
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return false, "unknown", fmt.Errorf("query service: %w", err)
	}

	switch status.State {
	case svc.Running:
		return true, "running", nil
	case svc.Stopped:
		return false, "stopped", nil
	case svc.StartPending:
		return false, "start_pending", nil
	case svc.StopPending:
		return false, "stop_pending", nil
	default:
		return false, fmt.Sprintf("state_%d", status.State), nil
	}
}

// WaitForPipe polls the Beekeeper Sentry named pipe until a connection
// succeeds or timeout elapses. Used by runProtectInstall after starting the
// service to confirm the daemon is ready for IPC.
func WaitForPipe(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := ipc.Connect("", 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for pipe %s", ipc.PipePath)
}

// isServiceNotExist returns true when the error indicates the service does not
// exist in the SCM (ERROR_SERVICE_DOES_NOT_EXIST = syscall.Errno(1060)).
func isServiceNotExist(err error) bool {
	return err != nil && err == windows.ERROR_SERVICE_DOES_NOT_EXIST
}
