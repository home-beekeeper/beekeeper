//go:build !windows

package tui

import (
	"os"
	"syscall"
)

// pidAlive reports whether the process identified by pid is currently running.
// On Unix it uses kill(pid, 0) semantics: signal 0 does not kill the process
// but returns an error if the PID does not exist or is not accessible.
// Any error (ESRCH = no such process, EPERM = no permission but process exists
// treated as alive) is handled: EPERM → true (process exists, we lack permission
// to signal it); ESRCH → false; other errors → false (fail-soft).
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists but we cannot signal it — treat as alive.
	if err == syscall.EPERM {
		return true
	}
	return false
}
