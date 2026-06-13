package policy

import (
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// TestCorroborationZeroMatches: no matches → level "allow", quarantine false, count 0.
func TestCorroborationZeroMatches(t *testing.T) {
	level, quarantine, count, agreed, dissented := corroborate(nil, DefaultCorroborationThresholds())
	if level != "allow" {
		t.Errorf("level = %q, want %q", level, "allow")
	}
	if quarantine {
		t.Error("quarantine = true, want false")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if len(agreed) != 0 {
		t.Errorf("agreed = %v, want empty", agreed)
	}
	if len(dissented) != 0 {
		t.Errorf("dissented = %v, want empty", dissented)
	}
}

// TestCorroborationOneSignedSource: one signed source → level "warn", quarantine false,
// CorroborationCount 1, SourcesAgreed ["bumblebee"].
//
// WR-03: Explicitly use Severity "high" (not "critical" and not the zero value "")
// to verify the GLOBAL threshold path, NOT the SeverityOverrides["critical"] override
// path. DefaultCorroborationThresholds() includes SeverityOverrides["critical"]{BlockAt:1},
// so a match with Severity "critical" would take the override path and produce "block"
// at signedCount=1. Using "high" keeps this test on the global path (effectiveBlockAt=2)
// where signedCount=1 correctly yields "warn". A future developer adding Severity
// "critical" here should recognise this test would then exercise a different code path.
func TestCorroborationOneSignedSource(t *testing.T) {
	matches := []CatalogMatch{
		// Severity "high" is intentional: exercises the global-threshold path,
		// not the SeverityOverrides["critical"] override path (WR-03).
		{CatalogSource: "bumblebee", Severity: "high", Signed: true},
	}
	level, quarantine, count, agreed, dissented := corroborate(matches, DefaultCorroborationThresholds())
	if level != "warn" {
		t.Errorf("level = %q, want %q", level, "warn")
	}
	if quarantine {
		t.Error("quarantine = true, want false")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if len(agreed) != 1 || agreed[0] != "bumblebee" {
		t.Errorf("agreed = %v, want [bumblebee]", agreed)
	}
	if len(dissented) != 0 {
		t.Errorf("dissented = %v, want empty", dissented)
	}
}

// TestCorroborationTwoSignedSources: two independent signed sources → level "block",
// quarantine false, CorroborationCount 2.
func TestCorroborationTwoSignedSources(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true},
		{CatalogSource: "osv", Signed: true},
	}
	level, quarantine, count, agreed, dissented := corroborate(matches, DefaultCorroborationThresholds())
	if level != "block" {
		t.Errorf("level = %q, want %q", level, "block")
	}
	if quarantine {
		t.Error("quarantine = true, want false (two sources don't quarantine)")
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if len(agreed) != 2 {
		t.Errorf("len(agreed) = %d, want 2; agreed = %v", len(agreed), agreed)
	}
	if len(dissented) != 0 {
		t.Errorf("dissented = %v, want empty", dissented)
	}
}

// TestCorroborationThreeSignedSources: three independent signed sources →
// level "block", quarantine TRUE, CorroborationCount 3.
func TestCorroborationThreeSignedSources(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true},
		{CatalogSource: "osv", Signed: true},
		{CatalogSource: "socket", Signed: true},
	}
	level, quarantine, count, _, _ := corroborate(matches, DefaultCorroborationThresholds())
	if level != "block" {
		t.Errorf("level = %q, want %q", level, "block")
	}
	if !quarantine {
		t.Error("quarantine = false, want true (three signed sources → quarantine)")
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// TestCorroborationTwoUnsignedNeverBlocks: two UNSIGNED sources, zero signed →
// level "warn" (unsigned sources can never alone reach block).
func TestCorroborationTwoUnsignedNeverBlocks(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "sourceA", Signed: false},
		{CatalogSource: "sourceB", Signed: false},
	}
	level, quarantine, count, _, _ := corroborate(matches, DefaultCorroborationThresholds())
	if level != "warn" {
		t.Errorf("level = %q, want %q (unsigned sources must never block)", level, "warn")
	}
	if quarantine {
		t.Error("quarantine = true, want false")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (unsigned do not contribute to signed count)", count)
	}
}

// TestCorroborationSameSourceTwiceCounts: same source appearing in two matches
// (bumblebee + bumblebee) counts as ONE independent source → level "warn", count 1.
func TestCorroborationSameSourceTwiceCounts(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true},
		{CatalogSource: "bumblebee", Signed: true}, // duplicate source — must not count twice
	}
	level, quarantine, count, agreed, _ := corroborate(matches, DefaultCorroborationThresholds())
	if level != "warn" {
		t.Errorf("level = %q, want %q (same source twice = one independent source)", level, "warn")
	}
	if quarantine {
		t.Error("quarantine = true, want false")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (deduplication by CatalogSource)", count)
	}
	if len(agreed) != 1 {
		t.Errorf("len(agreed) = %d, want 1; agreed = %v", len(agreed), agreed)
	}
}

// TestCorroborationOneSignedOneUnsigned: one signed + one unsigned →
// signed count 1, level "warn" (block requires 2 signed-weight; unsigned is 0.5).
func TestCorroborationOneSignedOneUnsigned(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true},
		{CatalogSource: "unofficial", Signed: false},
	}
	level, quarantine, count, agreed, _ := corroborate(matches, DefaultCorroborationThresholds())
	if level != "warn" {
		t.Errorf("level = %q, want %q (one signed + one unsigned stays warn)", level, "warn")
	}
	if quarantine {
		t.Error("quarantine = true, want false")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (only signed sources counted)", count)
	}
	// agreed should include both sources (signed and unsigned both matched)
	if len(agreed) < 1 {
		t.Errorf("agreed = %v, want at least [bumblebee]", agreed)
	}
}

// TestCorroborationSourcesDissented verifies CTLG-09 gap closure: when a multi-
// source lookup provides dissent sentinels (Dissented=true), corroborate populates
// SourcesDissented with the sources that did NOT flag the package.
func TestCorroborationSourcesDissented(t *testing.T) {
	// Bumblebee agreed (signed match), OSV dissented (no match → sentinel).
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true},
		{CatalogSource: "osv", Dissented: true},
	}
	level, _, count, agreed, dissented := corroborate(matches, DefaultCorroborationThresholds())

	// One signed source → warn.
	if level != "warn" {
		t.Errorf("level = %q, want warn (one signed source)", level)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	if len(agreed) != 1 || agreed[0] != "bumblebee" {
		t.Errorf("agreed = %v, want [bumblebee]", agreed)
	}
	// OSV must be in SourcesDissented (was queried, found nothing).
	if len(dissented) != 1 || dissented[0] != "osv" {
		t.Errorf("dissented = %v, want [osv] (CTLG-09 not populated)", dissented)
	}
}

// TestCorroborationDissentDoesNotAffectDecision verifies that a dissenting source
// sentinel does not change the corroboration level — it is forensic provenance only.
func TestCorroborationDissentDoesNotAffectDecision(t *testing.T) {
	// Two signed sources agree, socket dissented.
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true},
		{CatalogSource: "osv", Signed: true},
		{CatalogSource: "socket", Dissented: true},
	}
	level, quarantine, count, agreed, dissented := corroborate(matches, DefaultCorroborationThresholds())

	// Two signed sources → block.
	if level != "block" {
		t.Errorf("level = %q, want block (2 signed sources)", level)
	}
	if quarantine {
		t.Error("quarantine = true, want false (only 2 signed)")
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if len(agreed) != 2 {
		t.Errorf("len(agreed) = %d, want 2 (bumblebee+osv); got %v", len(agreed), agreed)
	}
	// Socket is in dissented.
	if len(dissented) != 1 || dissented[0] != "socket" {
		t.Errorf("dissented = %v, want [socket]", dissented)
	}
}

// TestCorroborationShaiHuludCriticalBlock: bumblebee (unsigned, critical) +
// OSV (signed, severity "unknown") → single signed source + critical severity override →
// effectiveBlockAt=1 → block.
func TestCorroborationShaiHuludCriticalBlock(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Severity: "critical", Signed: false},
		{CatalogSource: "osv", Severity: "unknown", Signed: true},
	}
	thresholds := DefaultCorroborationThresholds() // includes CatalogHealthy:true + SeverityOverrides
	level, quarantine, count, agreed, _ := corroborate(matches, thresholds)
	if level != "block" {
		t.Errorf("level = %q, want %q (critical single-signed-source must block)", level, "block")
	}
	if quarantine {
		t.Error("quarantine should be false (signedCount=1 < QuarantineAt=2)")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (only OSV is signed)", count)
	}
	if len(agreed) != 2 {
		t.Errorf("agreed = %v, want [bumblebee osv] (both matched)", agreed)
	}
}

// TestCorroborationDegradedCatalogNoEscalation: CatalogHealthy=false suppresses
// critical-severity override → same fixture as Shai-Hulud → warn-only.
func TestCorroborationDegradedCatalogNoEscalation(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Severity: "critical", Signed: false},
		{CatalogSource: "osv", Severity: "unknown", Signed: true},
	}
	thresholds := DefaultCorroborationThresholds()
	thresholds.CatalogHealthy = false // simulate >1000 delta entries injected
	level, _, _, _, _ := corroborate(matches, thresholds)
	if level != "warn" {
		t.Errorf("level = %q, want warn (degraded catalog must not escalate)", level)
	}
}

// TestCorroborationAllVersionsCriticalWildcardStaysWarn: versions:["*"] critical
// entry with one signed source must NOT block even with SeverityOverrides active.
func TestCorroborationAllVersionsCriticalWildcardStaysWarn(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Severity: "critical", Version: "*", Signed: false},
		{CatalogSource: "osv", Severity: "unknown", Version: "*", Signed: true},
	}
	thresholds := DefaultCorroborationThresholds() // CatalogHealthy:true
	level, _, _, _, _ := corroborate(matches, thresholds)
	if level != "warn" {
		t.Errorf("level = %q, want warn (all-versions wildcard must require 2 sources)", level)
	}
}

// TestCorroborationMixedWildcardDoesNotDowngradeVersionedCritical (CR-01 regression):
// when a package is matched by BOTH an injected all-versions wildcard entry AND a real
// version-specific signed critical advisory, the wildcard must NOT suppress escalation —
// otherwise one poisoned wildcard entry could downgrade a legitimate critical block→warn.
// The all-versions guard suppresses only when EVERY non-dissented match is a wildcard.
func TestCorroborationMixedWildcardDoesNotDowngradeVersionedCritical(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Severity: "critical", Version: "*", Signed: false},     // injected wildcard
		{CatalogSource: "osv", Severity: "critical", Version: "1.2.3", Signed: true},        // real version-specific critical
	}
	thresholds := DefaultCorroborationThresholds() // CatalogHealthy:true, critical→BlockAt:1
	level, _, _, _, _ := corroborate(matches, thresholds)
	if level != "block" {
		t.Errorf("level = %q, want block — a co-occurring wildcard must not downgrade a version-specific signed critical (CR-01)", level)
	}
}

// TestValidateCorroborationThresholdsRejectsBlockAtZero: BlockAt<1 override fails closed.
func TestValidateCorroborationThresholdsRejectsBlockAtZero(t *testing.T) {
	thresholds := DefaultCorroborationThresholds()
	thresholds.SeverityOverrides["critical"] = SeverityThreshold{BlockAt: 0, QuarantineAt: 1}
	// validateCorroborationThresholds should return non-nil error.
	if err := validateCorroborationThresholds(thresholds); err == nil {
		t.Error("want error for BlockAt=0, got nil")
	}
	// And corroborate should fail closed (block) not silently allow.
	matches := []CatalogMatch{{CatalogSource: "bumblebee", Severity: "critical", Signed: false}}
	level, _, _, _, _ := corroborate(matches, thresholds)
	if level != "block" {
		t.Errorf("level = %q, want block (misconfigured thresholds → fail closed)", level)
	}
}

// TestValidateCorroborationThresholdsRejectsLooserOverride: override BlockAt > global BlockAt → error.
func TestValidateCorroborationThresholdsRejectsLooserOverride(t *testing.T) {
	thresholds := DefaultCorroborationThresholds() // global BlockAt=2
	thresholds.SeverityOverrides["critical"] = SeverityThreshold{BlockAt: 3, QuarantineAt: 4}
	if err := validateCorroborationThresholds(thresholds); err == nil {
		t.Error("want error for override BlockAt(3) > global BlockAt(2), got nil")
	}
}

// TestValidateCorroborationThresholdsRejectsEqualQuarantine (CR-02 regression):
// an override where QuarantineAt == BlockAt collapses the two protection tiers
// (every block also quarantines). It must be rejected — QuarantineAt must be
// STRICTLY above BlockAt.
func TestValidateCorroborationThresholdsRejectsEqualQuarantine(t *testing.T) {
	thresholds := DefaultCorroborationThresholds()
	thresholds.SeverityOverrides["critical"] = SeverityThreshold{BlockAt: 2, QuarantineAt: 2}
	if err := validateCorroborationThresholds(thresholds); err == nil {
		t.Error("want error for override QuarantineAt(2) == BlockAt(2) (tier collapse), got nil")
	}
}

// TestDefaultThresholdsIncludeSeverityOverrides: DefaultCorroborationThresholds() has
// CatalogHealthy==true and SeverityOverrides["critical"].BlockAt==1.
func TestDefaultThresholdsIncludeSeverityOverrides(t *testing.T) {
	thresholds := DefaultCorroborationThresholds()
	if !thresholds.CatalogHealthy {
		t.Error("CatalogHealthy = false, want true (default must be healthy)")
	}
	if thresholds.SeverityOverrides == nil {
		t.Fatal("SeverityOverrides is nil, want non-nil map with critical entry")
	}
	ov, ok := thresholds.SeverityOverrides["critical"]
	if !ok {
		t.Fatal("SeverityOverrides[\"critical\"] not present in defaults")
	}
	if ov.BlockAt != 1 {
		t.Errorf("SeverityOverrides[\"critical\"].BlockAt = %d, want 1", ov.BlockAt)
	}
}

// TestCorroborateOutcome verifies the exported CorroborateOutcome wrapper.
//
// CRITICAL: confidence_tier is derived from count >= t.BlockAt, NEVER from
// level == "block" (Pitfall 4 / 2FA invariant). A single-source critical-severity
// block has level "block" but count 1 (< BlockAt=2) and must return "watch".
func TestCorroborateOutcome(t *testing.T) {
	t.Run("two signed independent sources returns enforce", func(t *testing.T) {
		// Two independent signed sources (count=2, default BlockAt=2) → enforce.
		matches := []CatalogMatch{
			{CatalogSource: "bumblebee", Signed: true},
			{CatalogSource: "osv", Signed: true},
		}
		got := CorroborateOutcome(matches, DefaultCorroborationThresholds())
		if got.SourceCount != 2 {
			t.Errorf("SourceCount = %d, want 2", got.SourceCount)
		}
		if got.ConfidenceTier != "enforce" {
			t.Errorf("ConfidenceTier = %q, want %q (count=%d >= BlockAt=%d)",
				got.ConfidenceTier, "enforce", got.SourceCount, DefaultCorroborationThresholds().BlockAt)
		}
	})

	t.Run("single-source critical block returns watch (Pitfall 4 / 2FA invariant)", func(t *testing.T) {
		// One signed source (OSV) with a critical-severity match from bumblebee
		// (unsigned). The default SeverityOverrides["critical"].BlockAt=1 causes
		// corroborate() to return level="block" but count=1 (only OSV is signed).
		// CorroborateOutcome must derive tier from count < BlockAt=2, NOT from
		// level="block" — so it must return "watch" (the 2FA invariant).
		matches := []CatalogMatch{
			{CatalogSource: "bumblebee", Severity: "critical", Signed: false},
			{CatalogSource: "osv", Severity: "unknown", Signed: true},
		}
		thresholds := DefaultCorroborationThresholds() // critical override BlockAt:1
		got := CorroborateOutcome(matches, thresholds)
		if got.SourceCount != 1 {
			t.Errorf("SourceCount = %d, want 1 (only OSV is signed)", got.SourceCount)
		}
		// CRITICAL: must be "watch", NOT "enforce" — tier from count, not level.
		if got.ConfidenceTier != "watch" {
			t.Errorf("ConfidenceTier = %q, want %q — single-source critical block must be watch (Pitfall 4)",
				got.ConfidenceTier, "watch")
		}
	})

	t.Run("no matches returns watch with zero source count", func(t *testing.T) {
		got := CorroborateOutcome(nil, DefaultCorroborationThresholds())
		if got.SourceCount != 0 {
			t.Errorf("SourceCount = %d, want 0", got.SourceCount)
		}
		if got.ConfidenceTier != "watch" {
			t.Errorf("ConfidenceTier = %q, want %q", got.ConfidenceTier, "watch")
		}
	})
}

// TestCorroborationImportsArePure enforces the purity contract: corroboration.go
// must not import any package that performs I/O, concurrency, or wall-clock
// access. Replicates the pattern from TestEngineImportsArePure.
func TestCorroborationImportsArePure(t *testing.T) {
	const filePath = "corroboration.go"
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
		// imp.Path.Value includes surrounding quotes.
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("corroboration.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
