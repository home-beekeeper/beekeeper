//go:build windows

package windows

import (
	"context"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestServiceNameConstant(t *testing.T) {
	if ServiceName != "BeekeeperSentry" {
		t.Errorf("ServiceName = %q; want %q", ServiceName, "BeekeeperSentry")
	}
}

func TestServiceConfigUsesLocalService(t *testing.T) {
	cfg := defaultServiceConfig(`C:\Program Files\Beekeeper\beekeeper.exe`)

	if cfg.ServiceStartName != `NT AUTHORITY\LocalService` {
		t.Errorf("ServiceStartName = %q; want %q", cfg.ServiceStartName, `NT AUTHORITY\LocalService`)
	}
	if cfg.StartType != mgr.StartAutomatic {
		t.Errorf("StartType = %v; want mgr.StartAutomatic (%v)", cfg.StartType, mgr.StartAutomatic)
	}
	if cfg.ServiceType != windows.SERVICE_WIN32_OWN_PROCESS {
		t.Errorf("ServiceType = %v; want SERVICE_WIN32_OWN_PROCESS (%v)", cfg.ServiceType, windows.SERVICE_WIN32_OWN_PROCESS)
	}
}

func TestQueryServiceWhenNotInstalled(t *testing.T) {
	// Querying the SCM requires administrator privileges on Windows.
	// Skip gracefully if access is denied (non-admin dev environment).
	running, statusText, err := QueryService(context.Background())
	if err != nil {
		// "Access is denied" means we don't have SCM access — skip the assertion.
		t.Skipf("QueryService() returned error (likely non-admin): %v", err)
	}
	if running {
		t.Skip("BeekeeperSentry service is installed and running on this machine; skipping not-installed assertion")
	}
	// statusText should be "not_installed" unless the service is installed but stopped.
	if statusText != "not_installed" && statusText != "stopped" {
		t.Errorf("statusText = %q; want not_installed or stopped", statusText)
	}
}

func TestWaitForPipeTimesOut(t *testing.T) {
	// The Beekeeper Sentry service is not running during unit tests, so
	// WaitForPipe should time out quickly.
	err := WaitForPipe(100 * time.Millisecond)
	if err == nil {
		t.Fatal("WaitForPipe(100ms) returned nil; expected timeout error")
	}
	if len(err.Error()) == 0 {
		t.Error("WaitForPipe returned non-nil error with empty message")
	}
}

func TestUninstallServiceIsIdempotent(t *testing.T) {
	// Calling UninstallService when the service does not exist must return nil.
	// If the service IS installed, skip to avoid disrupting a real installation.
	_, statusText, _ := QueryService(context.Background())
	if statusText != "not_installed" {
		t.Skipf("BeekeeperSentry service exists (status=%s); skipping idempotent-uninstall test", statusText)
	}

	err := UninstallService()
	if err != nil {
		t.Errorf("UninstallService() on non-existent service returned error: %v", err)
	}
}
