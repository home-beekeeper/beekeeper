package main

import (
	"testing"
	"time"
)

// TestCatalogSyncGate proves the interval gate with an injected clock: the
// OS-scheduled hourly heartbeat must NO-OP unless the configured interval has
// elapsed since the last success (D-T1-interval), with --force bypassing it.
func TestCatalogSyncGate(t *testing.T) {
	base := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	interval := 12 * time.Hour

	tests := []struct {
		name        string
		lastSuccess time.Time
		now         time.Time
		force       bool
		wantDue     bool
	}{
		{"never synced -> due", time.Time{}, base, false, true},
		{"just synced -> not due", base, base.Add(time.Minute), false, false},
		{"half interval -> not due", base, base.Add(6 * time.Hour), false, false},
		{"one minute short -> not due", base, base.Add(interval - time.Minute), false, false},
		{"exactly interval -> due", base, base.Add(interval), false, true},
		{"past interval -> due", base, base.Add(13 * time.Hour), false, true},
		{"force bypasses gate (not due) -> due", base, base.Add(time.Minute), true, true},
		{"force bypasses gate (never synced) -> due", time.Time{}, base, true, true},
	}
	for _, tt := range tests {
		if got := catalogSyncDue(tt.lastSuccess, interval, tt.now, tt.force); got != tt.wantDue {
			t.Errorf("%s: catalogSyncDue(lastSuccess=%v, interval=%s, now=%v, force=%v) = %v, want %v",
				tt.name, tt.lastSuccess, interval, tt.now, tt.force, got, tt.wantDue)
		}
	}
}

// TestCatalogDaemonStatusNoError verifies the status probe never errors when the
// daemon is not registered (it reports not-installed rather than failing). The
// OS-specific query tool is shelled out; the test only asserts the contract that
// "absent" is not an error.
func TestCatalogDaemonStatusNoError(t *testing.T) {
	installed, detail, err := catalogDaemonStatus()
	if err != nil {
		t.Fatalf("catalogDaemonStatus() error = %v, want nil (absent must not be an error)", err)
	}
	if detail == "" {
		t.Error("catalogDaemonStatus() detail is empty, want a human-readable state string")
	}
	_ = installed // value is environment-dependent; only the no-error contract is asserted
}
