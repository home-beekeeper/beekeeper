package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestDiagCmd_Output verifies that the diag command prints all four required
// sections to stdout and returns nil (exit 0) when no state files exist.
//
// The test exercises the Cobra wiring and output formatting only; the underlying
// CollectDiag and per-platform eventsLost are unit-tested in internal/check.
// When stateFile and hookLatencyRingPath don't exist, CollectDiag gracefully
// returns zero-value fields — the sections must still appear in the output.
func TestDiagCmd_Output(t *testing.T) {
	cmd := newDiagCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	// RunE will resolve platform.StateDir() and platform.ConfigPath() on the
	// test machine. On CI/Windows without a beekeeper state dir, these calls
	// succeed (StateDir is based on %APPDATA% or ~/); missing files are handled
	// gracefully by CollectDiag. We assert on output section headings.
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("diag RunE returned unexpected error: %v (stderr: %q)", err, errOut.String())
	}

	output := out.String()

	// All four section headings must be present.
	requiredSections := []string{
		"Hook Handler",
		"LlamaFirewall Sidecar",
		"Catalog Sources",
		"ETW Event Loss",
	}
	for _, section := range requiredSections {
		if !strings.Contains(output, section) {
			t.Errorf("diag output missing section %q\nFull output:\n%s", section, output)
		}
	}

	// The key latency labels should be present.
	if !strings.Contains(output, "p95 latency") {
		t.Errorf("diag output missing 'p95 latency'\nFull output:\n%s", output)
	}
	if !strings.Contains(output, "p99 latency") {
		t.Errorf("diag output missing 'p99 latency'\nFull output:\n%s", output)
	}
	if !strings.Contains(output, "events lost") {
		t.Errorf("diag output missing 'events lost'\nFull output:\n%s", output)
	}
}
