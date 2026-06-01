package check

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bantuson/beekeeper/internal/catalog"
)

// TestEventsLost_NonWindows verifies that eventsLost() returns 0 on non-Windows
// builds. On Windows, diag_windows.go provides the real counter; on all other
// platforms, diag_other.go provides this stub (T-09-17 mitigation: ETW stub
// returns 0, not an error).
//
// This test is compiled on all platforms. On Windows it tests the actual counter
// behavior (zero at startup, before any ETW events arrive); on other platforms it
// tests the stub. Both must return 0 in a clean test environment.
func TestEventsLost_NonWindows(t *testing.T) {
	got := eventsLost()
	if got != 0 {
		t.Fatalf("eventsLost() = %d, want 0 (ETW has no events in test environment)", got)
	}
}

// TestCollectDiag verifies that CollectDiag assembles a DiagReport whose
// CatalogSources slice contains the seeded sources in sorted order with the
// correct Degraded and Count fields.
func TestCollectDiag(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	ringPath := filepath.Join(dir, hookLatencyFile)

	// Seed state.json with two sources: "osv" (degraded) and "bumblebee" (healthy).
	ws := catalog.WatchState{
		Sources: map[string]catalog.SourceState{
			"bumblebee": {Hash: "sha256:aabbcc", Count: 654, Degraded: false},
			"osv":       {Hash: "sha256:ddeeff", Count: 1203, Degraded: true, DegradedReason: "sanity: entry drop > 20%"},
		},
	}
	data, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("json.Marshal WatchState: %v", err)
	}
	if err := os.WriteFile(stateFile, data, 0o600); err != nil {
		t.Fatalf("WriteFile state.json: %v", err)
	}

	// Seed the ring with some latency samples.
	for _, ms := range []int64{10, 20, 30, 40, 50} {
		appendHookLatency(ringPath, ms)
	}

	report := CollectDiag(stateFile, ringPath)

	// Verify catalog sources are sorted alphabetically.
	if len(report.CatalogSources) != 2 {
		t.Fatalf("CatalogSources: got %d sources, want 2", len(report.CatalogSources))
	}
	if report.CatalogSources[0].Name != "bumblebee" {
		t.Errorf("CatalogSources[0].Name = %q, want \"bumblebee\"", report.CatalogSources[0].Name)
	}
	if report.CatalogSources[1].Name != "osv" {
		t.Errorf("CatalogSources[1].Name = %q, want \"osv\"", report.CatalogSources[1].Name)
	}

	// Verify bumblebee source fields.
	bb := report.CatalogSources[0]
	if bb.Degraded {
		t.Errorf("bumblebee.Degraded = true, want false")
	}
	if bb.Count != 654 {
		t.Errorf("bumblebee.Count = %d, want 654", bb.Count)
	}
	if bb.Hash != "sha256:aabbcc" {
		t.Errorf("bumblebee.Hash = %q, want \"sha256:aabbcc\"", bb.Hash)
	}

	// Verify osv source fields (degraded).
	osv := report.CatalogSources[1]
	if !osv.Degraded {
		t.Errorf("osv.Degraded = false, want true")
	}
	if osv.Count != 1203 {
		t.Errorf("osv.Count = %d, want 1203", osv.Count)
	}

	// Verify hook latency is non-zero (ring was seeded).
	if report.HookLatencyP95MS == 0 {
		t.Error("HookLatencyP95MS = 0, want non-zero (ring seeded with samples)")
	}
	if report.HookLatencyP99MS == 0 {
		t.Error("HookLatencyP99MS = 0, want non-zero (ring seeded with samples)")
	}

	// Verify P99 >= P95 invariant.
	if report.HookLatencyP99MS < report.HookLatencyP95MS {
		t.Errorf("HookLatencyP99MS (%d) < HookLatencyP95MS (%d), want >= ",
			report.HookLatencyP99MS, report.HookLatencyP95MS)
	}
}

// TestCollectDiag_MissingStateFile verifies that CollectDiag returns a valid
// (zero-value catalog sources) DiagReport when the state file does not exist.
// This mirrors the missing-file-is-OK pattern used by catalog.LoadState.
func TestCollectDiag_MissingStateFile(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "nonexistent-state.json")
	ringPath := filepath.Join(dir, hookLatencyFile)

	report := CollectDiag(stateFile, ringPath)

	// CatalogSources must be nil or empty — not a panic.
	if len(report.CatalogSources) != 0 {
		t.Errorf("CatalogSources: got %d entries for missing state file, want 0",
			len(report.CatalogSources))
	}

	// ETW field must always be present (0 on non-Windows).
	// No assertion on value — just verify no panic.
	_ = report.ETWEventsLost
}

// TestCollectDiag_SortedSources verifies that CollectDiag returns catalog sources
// in alphabetical order even when the state.json map iteration would produce a
// different order. Go maps have non-deterministic iteration; this test catches any
// regression in the sort.Strings(names) call.
func TestCollectDiag_SortedSources(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	ringPath := filepath.Join(dir, hookLatencyFile)

	// Three sources: "socket", "bumblebee", "osv" — alphabetical order: bumblebee, osv, socket.
	ws := catalog.WatchState{
		Sources: map[string]catalog.SourceState{
			"socket":    {Hash: "sha256:111", Count: 0},
			"bumblebee": {Hash: "sha256:aaa", Count: 654},
			"osv":       {Hash: "sha256:bbb", Count: 1203},
		},
	}
	data, _ := json.Marshal(ws)
	if err := os.WriteFile(stateFile, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	report := CollectDiag(stateFile, ringPath)

	want := []string{"bumblebee", "osv", "socket"}
	if len(report.CatalogSources) != len(want) {
		t.Fatalf("CatalogSources: got %d, want %d", len(report.CatalogSources), len(want))
	}
	for i, w := range want {
		if report.CatalogSources[i].Name != w {
			t.Errorf("CatalogSources[%d].Name = %q, want %q", i, report.CatalogSources[i].Name, w)
		}
	}
}
