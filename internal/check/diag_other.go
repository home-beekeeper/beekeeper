//go:build !windows

package check

// eventsLost returns 0 on non-Windows platforms. ETW (Event Tracing for
// Windows) is a Windows-only kernel tracing mechanism; it does not exist on
// Linux or macOS. The Windows implementation lives in diag_windows.go and
// reads the real atomic counter from internal/sentry/windows.EventsLost.
func eventsLost() uint64 { return 0 }
