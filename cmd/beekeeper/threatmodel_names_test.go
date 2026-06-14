package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestThreatModelNamesResidualGaps is a grep tripwire that fails if any of the
// three LAUNCH-04 residual-gap names (or the §13 section header) is removed from
// docs/THREAT-MODEL.md. Naming the three gaps verbatim is a correctness requirement
// (LAUNCH-04): understating residual gaps is an information-disclosure / false-
// confidence threat. This test turns red the moment any verbatim name disappears.
func TestThreatModelNamesResidualGaps(t *testing.T) {
	// Resolve the path to docs/THREAT-MODEL.md relative to this test file.
	// This test file is at cmd/beekeeper/, so the doc is at ../../docs/THREAT-MODEL.md.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot resolve test file path")
	}
	docPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "THREAT-MODEL.md")
	docPath = filepath.Clean(docPath)

	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("LAUNCH-04: could not read docs/THREAT-MODEL.md at %s: %v", docPath, err)
	}
	content := string(data)

	wantStrings := []string{
		"## 13. Adjudicated Corpus (Local Loop)",
		"SENTRY-008 CI-runner OIDC theft",
		"GitHub API dead-drop exfil",
		"DNS-tunnel ingested-but-undetected",
	}

	for _, s := range wantStrings {
		if !strings.Contains(content, s) {
			t.Errorf(
				"LAUNCH-04: docs/THREAT-MODEL.md must name residual gap %q verbatim (honest-docs correctness requirement)",
				s,
			)
		}
	}
}
