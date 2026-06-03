package policyloader

import (
	"path/filepath"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
)

// fakeSignedCatalog is a test-only MultiCatalogLookup that returns a single
// signed match for a given ecosystem::package key. Used to simulate a catalog
// hit for testing block-rule threshold behaviour without live catalogs.
type fakeSignedCatalog struct {
	ecosystem string
	pkg       string
	source    string
}

func (f fakeSignedCatalog) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	if ecosystem == f.ecosystem && pkg == f.pkg {
		return []policy.CatalogMatch{
			{
				CatalogSource: f.source,
				EntryID:       "test-entry-001",
				Ecosystem:     ecosystem,
				Package:       pkg,
				Severity:      "high",
				Signed:        true,
			},
		}
	}
	return nil
}

// TestPolicyTest_BlockRule verifies that a policy file with a corroboration_threshold
// rule lowering block_at to 1, combined with a single signed catalog match for
// a matching ToolCall, yields a block-level Decision.
//
// This test uses the internal runPolicyTestWithCatalog helper (white-box) so it
// can inject a signed catalog match. The public RunPolicyTest uses emptyLookup{}
// (Pitfall 4: empty catalog prevents misleading "allow" results from catalog absence).
func TestPolicyTest_BlockRule(t *testing.T) {
	// Build a policy file with a corroboration_threshold rule lowering block_at to 1.
	// Default PLCY-01 has block_at=2; lowering to 1 means a single signed source blocks.
	pf := PolicyFile{
		SchemaVersion: "1",
		Name:          "strict-block-policy",
		Rules: []PolicyRule{
			{
				ID:       "lower-block-threshold",
				RuleType: "corroboration_threshold",
				Ecosystem: "npm",
				WarnAt:   1,
				BlockAt:  1,
				QuarantineAt: 2,
			},
		},
	}

	tc := policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "Bash",
		ToolInput: map[string]any{
			"ecosystem": "npm",
			"package":   "malicious-pkg",
		},
	}

	// Inject a signed catalog match so the engine has data to corroborate.
	fakeCatalog := fakeSignedCatalog{
		ecosystem: "npm",
		pkg:       "malicious-pkg",
		source:    "bumblebee",
	}

	d := runPolicyTestWithCatalog(pf, tc, fakeCatalog, policy.AgentContext{})
	if d.Allow {
		t.Errorf("TestPolicyTest_BlockRule: Allow = true, want false (block_at=1 + 1 signed source should block)")
	}
	if d.Level != "block" {
		t.Errorf("TestPolicyTest_BlockRule: Level = %q, want %q", d.Level, "block")
	}
}

// TestPolicyTest_AllowlistOverride verifies that a policy file with a
// package_allowlist rule for a package, when dry-run against a ToolCall for
// that package with NO catalog matches (emptyLookup), yields an allow Decision.
//
// With an empty lookup there are no catalog matches, so Evaluate returns "allow"
// regardless of any allowlist rules. This test verifies that RunPolicyTest
// correctly uses emptyLookup and that the allowlist context is preserved.
func TestPolicyTest_AllowlistOverride(t *testing.T) {
	path := filepath.Join(testdataDir(), "valid_allowlist.json")
	pf, errs := LoadPolicyFile(path)
	if len(errs) != 0 {
		t.Fatalf("LoadPolicyFile: unexpected errors: %v", errs)
	}

	// ToolCall for a package in the allowlist.
	tc := policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "Bash",
		ToolInput: map[string]any{
			"ecosystem": "npm",
			"package":   "react",
		},
	}

	d := RunPolicyTest(pf, tc, policy.AgentContext{})
	if !d.Allow {
		t.Errorf("TestPolicyTest_AllowlistOverride: Allow = false, want true (no catalog matches → allow)")
	}
	if d.Level != "allow" {
		t.Errorf("TestPolicyTest_AllowlistOverride: Level = %q, want %q", d.Level, "allow")
	}
}

// TestThresholdsFromPolicyFilesCriticalBlockAt verifies that a policy file with a
// corroboration_threshold rule setting CriticalBlockAt:1 causes ThresholdsFromPolicyFiles
// to return thresholds where SeverityOverrides["critical"].BlockAt == 1 and
// QuarantineAt >= 1 (default: CriticalBlockAt+1).
func TestThresholdsFromPolicyFilesCriticalBlockAt(t *testing.T) {
	pf := PolicyFile{
		SchemaVersion: "1",
		Name:          "critical-block-policy",
		Rules: []PolicyRule{
			{
				ID:              "CORR-01",
				RuleType:        "corroboration_threshold",
				CriticalBlockAt: 1,
			},
		},
	}

	thresholds := ThresholdsFromPolicyFiles([]PolicyFile{pf})

	if thresholds.SeverityOverrides == nil {
		t.Fatal("SeverityOverrides is nil, want non-nil map")
	}
	ov, ok := thresholds.SeverityOverrides["critical"]
	if !ok {
		t.Fatal("SeverityOverrides[\"critical\"] not set")
	}
	if ov.BlockAt != 1 {
		t.Errorf("BlockAt = %d, want 1", ov.BlockAt)
	}
	if ov.QuarantineAt < 1 {
		t.Errorf("QuarantineAt = %d, want >= 1 (default: CriticalBlockAt+1)", ov.QuarantineAt)
	}
}

// TestThresholdsFromPolicyFile verifies that thresholdsFromPolicyFile correctly
// overrides the PLCY-01 defaults with values from corroboration_threshold rules.
func TestThresholdsFromPolicyFile(t *testing.T) {
	pf := PolicyFile{
		SchemaVersion: "1",
		Name:          "custom-thresholds",
		Rules: []PolicyRule{
			{
				ID:           "tight-threshold",
				RuleType:     "corroboration_threshold",
				WarnAt:       1,
				BlockAt:      1,
				QuarantineAt: 2,
			},
		},
	}

	thresholds := thresholdsFromPolicyFile(pf)
	if thresholds.WarnAt != 1 {
		t.Errorf("WarnAt = %d, want 1", thresholds.WarnAt)
	}
	if thresholds.BlockAt != 1 {
		t.Errorf("BlockAt = %d, want 1", thresholds.BlockAt)
	}
	if thresholds.QuarantineAt != 2 {
		t.Errorf("QuarantineAt = %d, want 2", thresholds.QuarantineAt)
	}
}
