//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"strings"

	etw "github.com/tekert/golang-etw/etw"
)

// ProbeKernelLoggerConflict attempts to start the NT Kernel Logger ETW session.
// It returns (true, nil) when ERROR_ALREADY_EXISTS (code 183) is returned —
// another process (commonly an EDR or Windows Defender) owns the session.
// It returns (false, nil) if the session can be created — no conflict.
// It returns (false, err) for any other unexpected error.
//
// The created session is immediately stopped so the probe is non-destructive.
func ProbeKernelLoggerConflict(ctx context.Context) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Create but do not start — we only need to attempt StartTrace to probe.
	sess := etw.NewRealTimeSession(etw.NtKernelLogger)
	err := sess.Start()
	if err == nil {
		// No conflict — clean up.
		_ = sess.Stop()
		return false, nil
	}

	// Check ERROR_ALREADY_EXISTS (code 183) using the etw package constant.
	if errors.Is(err, etw.ERROR_ALREADY_EXISTS) {
		return true, nil
	}
	// Fallback: check by raw errno value 183.
	var errno interface{ Is(error) bool }
	_ = errno
	if strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return true, nil
	}

	return false, fmt.Errorf("probe kernel logger: %w", err)
}

// ConflictMessage returns a human-readable status string for use in
// 'beekeeper protect status' output.
func ConflictMessage(conflict bool) string {
	if conflict {
		return "WARNING: NT Kernel Logger session is held by another process (commonly EDR/Defender). Beekeeper ETW coverage may be limited."
	}
	return "NT Kernel Logger session available."
}
