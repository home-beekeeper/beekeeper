package policy

import (
	"fmt"
	"math"
	"sort"
)

// ruleBaselineAnomaly is the rule ID for behavioral baseline deviation decisions.
const ruleBaselineAnomaly = "baseline-anomaly"

// BaselineCounters is the exported on-disk contract for per-project frequency data.
// The I/O layer (internal/baseline/store.go) loads and persists this type;
// the policy engine receives only the pre-resolved counters.
//
// Counts is keyed by "tool_name::target_pattern" and each value is a slice of
// Unix-second timestamps recording when that tool+target combination occurred.
// WindowDays controls the rolling window for the statistical calculation.
type BaselineCounters struct {
	Counts     map[string][]int64 `json:"counts"`
	WindowDays int                `json:"window_days"`
}

// BaselineConfig holds parameters for the deviation detection calculation.
type BaselineConfig struct {
	DeviationSigma float64 // number of standard deviations above mean to trigger warn
}

// DefaultBaselineConfig returns the default behavioral baseline configuration.
// DeviationSigma of 3.0 means warn when current_frequency > mean + 3*stddev.
func DefaultBaselineConfig() BaselineConfig {
	return BaselineConfig{
		DeviationSigma: 3.0,
	}
}

// EvaluateBaseline returns a warn Decision if the frequency for the given key
// exceeds mean + cfg.DeviationSigma*stddev over the rolling window. Pure function;
// no time.Now() — the caller supplies nowUnix.
//
// The key is a "tool_name::target_pattern" string matching the Counts map keys.
//
// Decision rules:
//   - No timestamps for key in window → allow (no baseline yet)
//   - Fewer than 2 populated days in window → allow (insufficient history for stddev)
//   - Current day frequency > mean + N*stddev → warn
//   - Otherwise → allow
func EvaluateBaseline(key string, nowUnix int64, counters BaselineCounters, cfg BaselineConfig) Decision {
	windowDays := counters.WindowDays
	if windowDays <= 0 {
		windowDays = 7 // default to 7-day rolling window
	}

	// Window cutoff: only count timestamps on or after this Unix time.
	cutoff := nowUnix - int64(windowDays)*86400

	// Filter the key's timestamps to those within the rolling window.
	timestamps, ok := counters.Counts[key]
	if !ok || len(timestamps) == 0 {
		return Decision{
			Allow:   true,
			Level:   "allow",
			Reason:  "no baseline for " + key,
			RuleIDs: []string{ruleBaselineAnomaly},
		}
	}

	var inWindow []int64
	for _, ts := range timestamps {
		if ts >= cutoff {
			inWindow = append(inWindow, ts)
		}
	}
	if len(inWindow) == 0 {
		return Decision{
			Allow:   true,
			Level:   "allow",
			Reason:  "no baseline within window for " + key,
			RuleIDs: []string{ruleBaselineAnomaly},
		}
	}

	// Bucket timestamps by day (relative to nowUnix).
	// dayBucket maps dayIndex → count, where dayIndex = floor(ts / 86400).
	dayBucket := make(map[int64]int64)
	for _, ts := range inWindow {
		day := ts / 86400
		dayBucket[day]++
	}

	// Determine "current day" (the day containing nowUnix).
	currentDay := nowUnix / 86400

	// Compute daily counts as a sorted slice.
	days := make([]int64, 0, len(dayBucket))
	for d := range dayBucket {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })

	// Build the per-day frequency series from HISTORICAL days only (excluding today).
	// The deviation check compares today's count against historical statistics.
	var historicalFreqs []float64
	var currentFreq float64
	for _, d := range days {
		f := float64(dayBucket[d])
		if d == currentDay {
			currentFreq = f
		} else {
			historicalFreqs = append(historicalFreqs, f)
		}
	}

	// Require at least 2 historical days (not counting today) for a meaningful stddev.
	if len(historicalFreqs) < 2 {
		return Decision{
			Allow:   true,
			Level:   "allow",
			Reason:  "insufficient historical days for " + key,
			RuleIDs: []string{ruleBaselineAnomaly},
		}
	}

	// Calculate mean and stddev from historical days only.
	mean := meanFloat(historicalFreqs)
	stddev := stddevFloat(historicalFreqs, mean)

	// Detect anomaly:
	// - When stddev > 0: current > mean + N*stddev is the statistical threshold.
	// - When stddev == 0 (perfectly uniform history): any count above mean is anomalous
	//   (the pattern was constant; any deviation is a spike).
	var anomalous bool
	if stddev > 0 {
		anomalous = currentFreq > mean+cfg.DeviationSigma*stddev
	} else {
		// Uniform history: flag any count strictly above mean.
		anomalous = currentFreq > mean
	}
	if anomalous {
		return Decision{
			Allow:   true,
			Level:   "warn",
			Reason:  fmt.Sprintf("baseline deviation for %s: %.0f vs mean %.2f", key, currentFreq, mean),
			RuleIDs: []string{ruleBaselineAnomaly},
		}
	}

	return Decision{
		Allow:   true,
		Level:   "allow",
		Reason:  "",
		RuleIDs: []string{ruleBaselineAnomaly},
	}
}

// meanFloat computes the arithmetic mean of a slice of float64 values.
// Returns 0 for an empty slice.
func meanFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// stddevFloat computes the sample standard deviation of vals given their mean,
// using Bessel's correction (n-1 denominator). Sample stddev is appropriate here
// because historicalFreqs is a sample of past daily counts; population stddev
// would underestimate the true spread and make the anomaly threshold too low for
// small sample sizes (WR-07).
// Returns 0 for an empty slice or a single value.
func stddevFloat(vals []float64, mean float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	var sumSq float64
	for _, v := range vals {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(vals)-1))
}
