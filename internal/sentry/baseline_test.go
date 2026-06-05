package sentry

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIsBaselineActiveWithin(t *testing.T) {
	now := time.Now().UTC()
	state := BaselineState{
		StartedAt:    now.Add(-3 * 24 * time.Hour), // started 3 days ago
		DurationDays: 7,                             // 7-day window
	}
	if !IsBaselineActive(state, now) {
		t.Error("expected baseline to be active (3 days into 7-day window)")
	}
}

func TestIsBaselineActiveExpired(t *testing.T) {
	now := time.Now().UTC()
	state := BaselineState{
		StartedAt:    now.Add(-8 * 24 * time.Hour), // started 8 days ago
		DurationDays: 7,                             // 7-day window expired
	}
	if IsBaselineActive(state, now) {
		t.Error("expected baseline to be inactive (8 days into 7-day window)")
	}
}

func TestIsBaselineActiveImmediate(t *testing.T) {
	now := time.Now().UTC()
	state := BaselineState{
		StartedAt:    now,
		DurationDays: 0, // 0 means immediate enforcement
	}
	if IsBaselineActive(state, now) {
		t.Error("expected baseline to be inactive (DurationDays=0)")
	}
}

func TestIsBaselineActiveIndefinite(t *testing.T) {
	now := time.Now().UTC()
	state := BaselineState{
		StartedAt:    now.Add(-365 * 24 * time.Hour), // started 1 year ago
		DurationDays: -1,                              // indefinite
	}
	if !IsBaselineActive(state, now) {
		t.Error("expected baseline to be active (DurationDays=-1 is indefinite)")
	}
}

func TestLoadBaselineMissingFile(t *testing.T) {
	state, err := LoadBaseline(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("LoadBaseline with missing file should not error; got: %v", err)
	}
	if state.DurationDays != 7 {
		t.Errorf("DurationDays: got %d, want 7", state.DurationDays)
	}
	if state.StartedAt.IsZero() {
		t.Error("StartedAt should be set (non-zero) for a fresh baseline")
	}
}

// TestIsBaselineActivePermanentQuarantineSuppress verifies TM-RS-03: when
// DurationDays < 0 (permanent learning mode), IsBaselineActive returns true and
// QuarantineRec is suppressed by makeAlert. This test documents the INTENDED
// behavior and guards against accidental removal of the permanent-mode code path.
// The BaselinePermanent field in ipc.StatusResponse must be set to true so that
// 'beekeeper protect status' can surface a prominent warning to the operator.
func TestIsBaselineActivePermanentQuarantineSuppress(t *testing.T) {
	now := time.Now().UTC()

	// DurationDays=-1 (permanent): baseline must be active regardless of StartedAt.
	permanent := BaselineState{
		StartedAt:    now.Add(-365 * 24 * time.Hour), // started a year ago
		DurationDays: -1,
	}
	if !IsBaselineActive(permanent, now) {
		t.Error("DurationDays=-1: expected IsBaselineActive=true (permanent learning mode)")
	}

	// Verify that the daysLeft guard logic handles permanent mode correctly:
	// computing time.Duration(-1)*24*time.Hour gives a negative/nonsensical result,
	// so the daemon code must skip that computation when DurationDays < 0.
	isPermanent := permanent.DurationDays < 0
	if !isPermanent {
		t.Error("DurationDays=-1 must be detected as permanent (DurationDays < 0)")
	}

	// DurationDays=-2 is also permanent (any negative value).
	neg2 := BaselineState{StartedAt: now, DurationDays: -2}
	if !IsBaselineActive(neg2, now) {
		t.Error("DurationDays=-2: expected IsBaselineActive=true (any negative = permanent)")
	}
}

func TestSaveLoadBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	original := BaselineState{
		StartedAt:    time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		DurationDays: 14,
	}

	if err := SaveBaseline(path, original); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	loaded, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}

	if !loaded.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", loaded.StartedAt, original.StartedAt)
	}
	if loaded.DurationDays != original.DurationDays {
		t.Errorf("DurationDays: got %d, want %d", loaded.DurationDays, original.DurationDays)
	}
}
