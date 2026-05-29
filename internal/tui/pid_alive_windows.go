//go:build windows

package tui

import (
	"golang.org/x/sys/windows"
)

// pidAlive reports whether the process identified by pid is currently running.
// On Windows, os.FindProcess always succeeds for any numeric PID, so we use
// OpenProcess with the SYNCHRONIZE access right to confirm the process exists.
// A non-nil handle means the process is live; ERROR_INVALID_PARAMETER or
// ERROR_NOT_FOUND means it is not. Any other error is treated as false (fail-soft).
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(handle)
	return true
}
