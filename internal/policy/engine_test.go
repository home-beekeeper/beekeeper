package policy

import (
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// fakeMultiCatalog implements MultiCatalogLookup with canned matches keyed by
// "ecosystem::package". It performs no I/O and is safe for pure unit tests of
// Evaluate. Replaces the Phase 1 fakeCatalog for Phase 2 tests.
type fakeMultiCatalog struct {
	matchesByKey map[string][]CatalogMatch
}

func (f fakeMultiCatalog) LookupAll(ecosystem, pkg string) []CatalogMatch {
	return f.matchesByKey[ecosystem+"::"+pkg]
}

// newFakeMultiWithMatches builds a fakeMultiCatalog from a variadic list of
// (key, match) pairs where key is "ecosystem::package".
func newFakeMulti(matchesByKey map[string][]CatalogMatch) fakeMultiCatalog {
	return fakeMultiCatalog{matchesByKey: matchesByKey}
}

// nxConsoleMatch returns a single CatalogMatch for the Nx Console advisory.
func nxConsoleMatch(source string, signed bool) CatalogMatch {
	return CatalogMatch{
		CatalogSource: source,
		EntryID:       "advisory-2026-nx-console",
		Ecosystem:     "editor-extension",
		Package:       "nrwl.angular-console",
		Version:       "18.95.0",
		Severity:      "critical",
		Signed:        signed,
	}
}

// ─── Phase 2 multi-source corroboration tests ───────────────────────────────

// TestEvaluateSingleSignedSourceWarns: one signed match via fakeMultiCatalog →
// Level "warn", Allow true, CorroborationCount 1.
func TestEvaluateSingleSignedSourceWarns(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", true),
		},
	})
	tc := ToolCall{
		AgentName: "test-agent",
		ToolName:  "Install",
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (single source → warn, not block)")
	}
	if d.CorroborationCount != 1 {
		t.Errorf("CorroborationCount = %d, want 1", d.CorroborationCount)
	}
	if len(d.SourcesAgreed) != 1 || d.SourcesAgreed[0] != "bumblebee" {
		t.Errorf("SourcesAgreed = %v, want [bumblebee]", d.SourcesAgreed)
	}
	if d.Quarantine {
		t.Error("Quarantine = true, want false")
	}
}

// TestEvaluateTwoSignedSourcesBlock: two signed sources for same pkg →
// Level "block", Allow false, CorroborationCount 2, SourcesAgreed length 2.
func TestEvaluateTwoSignedSourcesBlock(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::lodash": {
			{CatalogSource: "bumblebee", EntryID: "bumblebee-lodash", Ecosystem: "npm", Package: "lodash", Signed: true, Severity: "high"},
			{CatalogSource: "osv", EntryID: "osv-lodash", Ecosystem: "npm", Package: "lodash", Signed: true, Severity: "high"},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install lodash@4.17.20"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (block decision)")
	}
	if d.CorroborationCount != 2 {
		t.Errorf("CorroborationCount = %d, want 2", d.CorroborationCount)
	}
	if len(d.SourcesAgreed) != 2 {
		t.Errorf("len(SourcesAgreed) = %d, want 2; got %v", len(d.SourcesAgreed), d.SourcesAgreed)
	}
	if d.Quarantine {
		t.Error("Quarantine = true, want false (only 2 sources)")
	}
}

// TestEvaluateThreeSignedSourcesQuarantine: three signed sources →
// Level "block", Allow false, Quarantine true.
func TestEvaluateThreeSignedSourcesQuarantine(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::malicious-pkg": {
			{CatalogSource: "bumblebee", Ecosystem: "npm", Package: "malicious-pkg", Signed: true},
			{CatalogSource: "osv", Ecosystem: "npm", Package: "malicious-pkg", Signed: true},
			{CatalogSource: "socket", Ecosystem: "npm", Package: "malicious-pkg", Signed: true},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install malicious-pkg@1.0.0"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false")
	}
	if !d.Quarantine {
		t.Error("Quarantine = false, want true (three signed sources → quarantine)")
	}
	if d.CorroborationCount != 3 {
		t.Errorf("CorroborationCount = %d, want 3", d.CorroborationCount)
	}
}

// TestEvaluateNoMatchAllows: empty LookupAll result → Level "allow", Allow true.
func TestEvaluateNoMatchAllows(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{}) // empty catalog
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install express@4.18.2"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
	if len(d.CatalogMatches) != 0 {
		t.Errorf("len(CatalogMatches) = %d, want 0", len(d.CatalogMatches))
	}
}

// TestEvaluateUnsignedNeverBlocks: two unsigned sources → Level "warn", Allow true.
func TestEvaluateUnsignedNeverBlocks(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::suspect-pkg": {
			{CatalogSource: "sourceA", Ecosystem: "npm", Package: "suspect-pkg", Signed: false},
			{CatalogSource: "sourceB", Ecosystem: "npm", Package: "suspect-pkg", Signed: false},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install suspect-pkg@1.0.0"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (unsigned sources must never block)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (unsigned sources → warn, not block)")
	}
}

// ─── Migrated Phase 1 tests (using fakeMultiCatalog) ────────────────────────

// TestCatalogMatchProducesWarn: single signed source → warn, Allow true.
func TestCatalogMatchProducesWarn(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", false), // unsigned
		},
	})
	tc := ToolCall{
		AgentName: "test-agent",
		ToolName:  "Install",
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn does not block)")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	m := d.CatalogMatches[0]
	if m.EntryID != "advisory-2026-nx-console" {
		t.Errorf("EntryID = %q, want %q", m.EntryID, "advisory-2026-nx-console")
	}
	if m.Ecosystem != "editor-extension" || m.Package != "nrwl.angular-console" || m.Version != "18.95.0" {
		t.Errorf("match identity = %+v, want editor-extension/nrwl.angular-console/18.95.0", m)
	}
	if len(d.RuleIDs) == 0 {
		t.Errorf("RuleIDs = %v, want at least one rule ID", d.RuleIDs)
	}
}

// TestUnsignedCatalogIsWarnOnly: unsigned match → warn, Allow true, Signed false.
func TestUnsignedCatalogIsWarnOnly(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", false), // unsigned
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true — unsigned match must never block (CTLG-07)")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	if d.CatalogMatches[0].Signed {
		t.Errorf("Signed = true, want false for unsigned match")
	}
}

// TestSignedCatalogStillWarnWithSingleSource: one signed source → warn (block needs 2).
func TestSignedCatalogStillWarnWithSingleSource(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			nxConsoleMatch("bumblebee", true), // signed but single source
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (single signed source → warn)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (block escalation requires 2 signed sources)")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	if !d.CatalogMatches[0].Signed {
		t.Errorf("Signed = false, want true for signed match")
	}
}

// TestNoMatchAllows: no entry in catalog → allow.
func TestNoMatchAllows(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{}) // empty
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install express@4.18.2"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
	if len(d.CatalogMatches) != 0 {
		t.Errorf("len(CatalogMatches) = %d, want 0", len(d.CatalogMatches))
	}
	if d.Reason != "no catalog match" {
		t.Errorf("Reason = %q, want %q", d.Reason, "no catalog match")
	}
}

// TestRemediatedVersionAllows: lookup returns nothing (version not matched by key).
func TestRemediatedVersionAllows(t *testing.T) {
	// Only the affected version key is in the catalog.
	idx := newFakeMulti(map[string][]CatalogMatch{
		"editor-extension::nrwl.angular-console": {
			// Adversarial: version 18.100.0 is safe (entry only covers 18.95.0 semantics
			// but for fakeMultiCatalog, the key already resolved — return empty to simulate no match)
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.100.0",
		},
	}
	// The fakeMultiCatalog key is "editor-extension::nrwl.angular-console" but matches are empty.
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestNpmInstallCommandExtraction: npm install command extracts ecosystem+pkg+version.
// In Phase 2, the version on the CatalogMatch is pre-set by the adapter; the fake
// sets it to match what the command extracts.
func TestNpmInstallCommandExtraction(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::some-pkg": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-npm", Ecosystem: "npm", Package: "some-pkg", Version: "1.0.0", Severity: "high", Signed: false},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install some-pkg@1.0.0"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	m := d.CatalogMatches[0]
	if m.Ecosystem != "npm" || m.Package != "some-pkg" || m.Version != "1.0.0" {
		t.Errorf("extracted match = %+v, want npm/some-pkg/1.0.0", m)
	}
}

// TestVersionlessCommandStillWarns: no explicit version still warns (defense-favoring).
func TestVersionlessCommandStillWarns(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::bad-pkg": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-npm-allver", Ecosystem: "npm", Package: "bad-pkg", Severity: "high", Signed: false},
		},
	})
	tc := ToolCall{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "npm install bad-pkg"},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestNoPackageIdentifiedAllows: tool call with no extractable package → allow.
func TestNoPackageIdentifiedAllows(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{})
	tc := ToolCall{
		ToolName:  "Read",
		ToolInput: map[string]any{},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
	if d.Reason != "no package identified" {
		t.Errorf("Reason = %q, want %q", d.Reason, "no package identified")
	}
	if len(d.CatalogMatches) != 0 {
		t.Errorf("len(CatalogMatches) = %d, want 0", len(d.CatalogMatches))
	}
}

// TestPackageNameNormalizedBeforeLookup: mixed-case + padded input still hits.
func TestPackageNameNormalizedBeforeLookup(t *testing.T) {
	idx := newFakeMulti(map[string][]CatalogMatch{
		"npm::mixedcase": {
			{CatalogSource: "bumblebee", EntryID: "advisory-2026-case", Ecosystem: "npm", Package: "mixedcase", Severity: "medium", Signed: false},
		},
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "npm",
			"package":   "  MixedCase  ",
			"version":   "2.0.0",
		},
	}
	d := Evaluate(tc, idx, DefaultCorroborationThresholds())

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (package name must be normalized before lookup)", d.Level, "warn")
	}
}

// TestEngineImportsArePure enforces the purity contract: engine.go must not
// import any package that performs I/O, concurrency, or wall-clock access.
func TestEngineImportsArePure(t *testing.T) {
	const enginePath = "engine.go"
	src, err := os.ReadFile(enginePath)
	if err != nil {
		t.Fatalf("reading %s: %v", enginePath, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, enginePath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", enginePath, err)
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
			t.Errorf("engine.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
