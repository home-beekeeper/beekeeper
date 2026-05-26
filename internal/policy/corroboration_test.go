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
func TestCorroborationOneSignedSource(t *testing.T) {
	matches := []CatalogMatch{
		{CatalogSource: "bumblebee", Signed: true},
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
