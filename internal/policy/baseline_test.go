package policy

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// nowEpoch is a fixed "now" for deterministic tests.
// 2026-05-26T00:00:00Z in Unix seconds.
const nowEpoch int64 = 1748217600

func TestBaselineEmptyCountersAllow(t *testing.T) {
	counters := BaselineCounters{
		Counts:     map[string][]int64{},
		WindowDays: 7,
	}
	cfg := DefaultBaselineConfig()
	d := EvaluateBaseline("bash::npm install", nowEpoch, counters, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (empty counters → no baseline → allow)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

func TestBaselineWithinNormalFrequencyAllows(t *testing.T) {
	// 7 days of once-per-day occurrences → stable baseline; current is also once → allow
	cfg := DefaultBaselineConfig()
	counters := BaselineCounters{
		Counts:     map[string][]int64{},
		WindowDays: 7,
	}
	key := "bash::npm install"
	// One occurrence per day for the last 7 days (including today)
	for i := int64(6); i >= 0; i-- {
		ts := nowEpoch - i*86400 + 3600 // 1 hour into each day
		counters.Counts[key] = append(counters.Counts[key], ts)
	}
	// Current event: one occurrence today is within the window
	d := EvaluateBaseline(key, nowEpoch, counters, cfg)
	if d.Level == "warn" {
		// Warn would require current day count to exceed mean+3*stddev.
		// With all 7 days having count=1, mean=1, stddev=0 — should allow.
		t.Errorf("Level = %q, want %q (stable frequency at 1/day should allow)", d.Level, "allow")
	}
}

func TestBaselineFrequencySpikeWarns(t *testing.T) {
	cfg := DefaultBaselineConfig()
	counters := BaselineCounters{
		Counts:     map[string][]int64{},
		WindowDays: 7,
	}
	key := "bash::npm install"
	// Historical: 1 occurrence per day for 6 days (not today)
	for i := int64(6); i >= 1; i-- {
		dayStart := nowEpoch - i*86400
		counters.Counts[key] = append(counters.Counts[key], dayStart+3600)
	}
	// Today: a massive spike — 20 occurrences (historical mean is ~1, stddev is ~0)
	todayStart := nowEpoch - (nowEpoch % 86400)
	for j := 0; j < 20; j++ {
		counters.Counts[key] = append(counters.Counts[key], todayStart+int64(j*60))
	}
	d := EvaluateBaseline(key, nowEpoch, counters, cfg)
	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (20x spike beyond 3σ should warn)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn does not block)")
	}
}

func TestBaselineOldTimestampsExcluded(t *testing.T) {
	cfg := DefaultBaselineConfig()
	counters := BaselineCounters{
		Counts:     map[string][]int64{},
		WindowDays: 7,
	}
	key := "bash::pip install"
	// Timestamps older than 7 days (outside the rolling window)
	for i := int64(30); i >= 8; i-- {
		ts := nowEpoch - i*86400
		counters.Counts[key] = append(counters.Counts[key], ts)
	}
	// Only these old timestamps — none within the window → insufficient history → allow
	d := EvaluateBaseline(key, nowEpoch, counters, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (timestamps outside window excluded; insufficient history → allow)", d.Level, "allow")
	}
}

func TestBaselineInsufficientHistoryAllows(t *testing.T) {
	cfg := DefaultBaselineConfig()
	counters := BaselineCounters{
		Counts:     map[string][]int64{},
		WindowDays: 7,
	}
	key := "bash::cargo add"
	// Only 1 day of data — can't compute meaningful stddev with < 2 populated days
	counters.Counts[key] = []int64{nowEpoch - 3600} // 1 hour ago
	d := EvaluateBaseline(key, nowEpoch, counters, cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (single day of history → insufficient → allow)", d.Level, "allow")
	}
}

func TestBaselineCountersJSONRoundTrip(t *testing.T) {
	original := BaselineCounters{
		Counts: map[string][]int64{
			"bash::npm install":  {1748217600, 1748131200, 1748044800},
			"bash::pip install":  {1748217600},
			"webfetch::github.com": {1748131200, 1748044800},
		},
		WindowDays: 7,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored BaselineCounters
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.WindowDays != original.WindowDays {
		t.Errorf("WindowDays = %d, want %d", restored.WindowDays, original.WindowDays)
	}
	if len(restored.Counts) != len(original.Counts) {
		t.Errorf("len(Counts) = %d, want %d", len(restored.Counts), len(original.Counts))
	}
	for key, orig := range original.Counts {
		got, ok := restored.Counts[key]
		if !ok {
			t.Errorf("key %q missing after round-trip", key)
			continue
		}
		if len(got) != len(orig) {
			t.Errorf("key %q: len = %d, want %d", key, len(got), len(orig))
			continue
		}
		for i, v := range orig {
			if got[i] != v {
				t.Errorf("key %q [%d]: got %d, want %d", key, i, got[i], v)
			}
		}
	}
}

func TestBaselineZeroWindowDaysDefaultsTo7(t *testing.T) {
	cfg := DefaultBaselineConfig()
	// WindowDays == 0 should default to 7 internally
	counters := BaselineCounters{
		Counts:     map[string][]int64{},
		WindowDays: 0, // zero — should be treated as 7
	}
	key := "bash::gem install"
	// Add timestamps 8+ days ago — with default 7d window they should be excluded
	counters.Counts[key] = []int64{
		nowEpoch - 10*86400,
		nowEpoch - 9*86400,
		nowEpoch - 8*86400,
	}
	d := EvaluateBaseline(key, nowEpoch, counters, cfg)
	// All timestamps are outside a 7-day window, so insufficient history → allow
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (old timestamps with default 7d window → allow)", d.Level, "allow")
	}
}

func TestBaselineImportsArePure(t *testing.T) {
	const filePath = "baseline.go"
	src, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading %s: %v", filePath, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", filePath, err)
	}

	forbidden := map[string]bool{
		"os":       true,
		"net":      true,
		"net/http": true,
		"io":       true,
		"sync":     true,
		"time":     true,
		"context":  true,
	}

	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("baseline.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
