package sentry

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

// BaselineState describes the learning-mode window for a Sentry deployment.
// During the baseline window the correlation rules fire alerts exactly as they
// would in enforcement mode but QuarantineRec is suppressed — operators can
// tune thresholds against real traffic before enabling quarantine.
//
// DurationDays semantics:
//
//	 0  — baseline never active; immediate enforcement (default after first install)
//	-1  — baseline active indefinitely (never transitions to enforcement)
//	 N  — baseline active for N days from StartedAt, then enforcement
type BaselineState struct {
	StartedAt    time.Time `json:"started_at"`
	DurationDays int       `json:"duration_days"`
}

// IsBaselineActive reports whether the baseline window is currently open. The
// now argument is passed explicitly so the function remains pure and testable.
func IsBaselineActive(state BaselineState, now time.Time) bool {
	switch {
	case state.DurationDays == 0:
		return false
	case state.DurationDays < 0:
		return true
	default:
		deadline := state.StartedAt.Add(time.Duration(state.DurationDays) * 24 * time.Hour)
		return now.Before(deadline)
	}
}

// LoadBaseline reads the BaselineState persisted at path. If the file does not
// exist a fresh 7-day baseline starting now (UTC) is returned so that new
// deployments begin in learning mode automatically. Any other I/O error is
// returned as-is.
func LoadBaseline(path string) (BaselineState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return BaselineState{
				StartedAt:    time.Now().UTC(),
				DurationDays: 7,
			}, nil
		}
		return BaselineState{}, err
	}

	var state BaselineState
	if err := json.Unmarshal(data, &state); err != nil {
		return BaselineState{}, err
	}
	return state, nil
}

// SaveBaseline atomically persists state to path using a write-then-rename
// strategy so that a crash mid-write cannot leave a corrupt file.
func SaveBaseline(path string, state BaselineState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
