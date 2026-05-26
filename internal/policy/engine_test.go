package policy

import (
	"go/parser"
	"go/token"
	"os"
	"testing"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
)

// fakeCatalog implements CatalogLookup with canned entries keyed by
// ecosystem + "::" + package. It performs no I/O and is safe for pure unit
// tests of Evaluate.
type fakeCatalog struct {
	entries map[string]catalog.Entry
}

func (f fakeCatalog) Lookup(ecosystem, pkg string) (catalog.Entry, bool) {
	e, ok := f.entries[ecosystem+"::"+pkg]
	return e, ok
}

func newFakeCatalog(entries ...catalog.Entry) fakeCatalog {
	m := make(map[string]catalog.Entry, len(entries))
	for _, e := range entries {
		m[e.Ecosystem+"::"+e.Package] = e
	}
	return fakeCatalog{entries: m}
}

// nxConsoleEntry is the Nx Console editor-extension advisory used across tests.
func nxConsoleEntry(signature string) catalog.Entry {
	return catalog.Entry{
		ID:               "advisory-2026-nx-console",
		Name:             "Nx Console malicious release",
		Ecosystem:        "editor-extension",
		Package:          "nrwl.angular-console",
		Versions:         []string{"18.95.0"},
		Severity:         "critical",
		SourceURL:        "https://example.test/advisory",
		CatalogSignature: signature,
		CatalogSource:    "bumblebee",
	}
}

func TestCatalogMatchProducesWarn(t *testing.T) {
	idx := newFakeCatalog(nxConsoleEntry(""))
	tc := ToolCall{
		AgentName: "test-agent",
		ToolName:  "Install",
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}

	d := Evaluate(tc, idx)

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (warn does not block in Phase 1)")
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
	if len(d.RuleIDs) != 1 || d.RuleIDs[0] != "bumblebee-catalog-match" {
		t.Errorf("RuleIDs = %v, want [bumblebee-catalog-match]", d.RuleIDs)
	}
}

func TestUnsignedCatalogIsWarnOnly(t *testing.T) {
	idx := newFakeCatalog(nxConsoleEntry("")) // empty signature => unsigned
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}

	d := Evaluate(tc, idx)

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
		t.Errorf("Signed = true, want false for empty signature")
	}
}

func TestSignedCatalogStillWarnInPhase1(t *testing.T) {
	idx := newFakeCatalog(nxConsoleEntry("sig-abc123"))
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.95.0",
		},
	}

	d := Evaluate(tc, idx)

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (no corroboration in Phase 1)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (block escalation is Phase 2)")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	if !d.CatalogMatches[0].Signed {
		t.Errorf("Signed = false, want true for non-empty signature")
	}
}

func TestNoMatchAllows(t *testing.T) {
	idx := newFakeCatalog(nxConsoleEntry(""))
	tc := ToolCall{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "npm install express@4.18.2",
		},
	}

	d := Evaluate(tc, idx)

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

func TestRemediatedVersionAllows(t *testing.T) {
	// Entry covers only 18.95.0; a later remediated version must not match.
	idx := newFakeCatalog(nxConsoleEntry(""))
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "editor-extension",
			"package":   "nrwl.angular-console",
			"version":   "18.100.0",
		},
	}

	d := Evaluate(tc, idx)

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

func TestNpmInstallCommandExtraction(t *testing.T) {
	// A catalog entry for an npm package; the tool call uses the command shape.
	idx := newFakeCatalog(catalog.Entry{
		ID:        "advisory-2026-npm",
		Ecosystem: "npm",
		Package:   "some-pkg",
		Versions:  []string{"1.0.0"},
		Severity:  "high",
	})
	tc := ToolCall{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "npm install some-pkg@1.0.0",
		},
	}

	d := Evaluate(tc, idx)

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

func TestVersionlessCommandStillWarns(t *testing.T) {
	// A command with no explicit @version; a version-less match is defense-favoring.
	idx := newFakeCatalog(catalog.Entry{
		ID:        "advisory-2026-npm-allver",
		Ecosystem: "npm",
		Package:   "bad-pkg",
		Versions:  nil, // entry applies to all versions
		Severity:  "high",
	})
	tc := ToolCall{
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": "npm install bad-pkg",
		},
	}

	d := Evaluate(tc, idx)

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (version-less match is defense-favoring)", d.Level, "warn")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
	if len(d.CatalogMatches) != 1 {
		t.Fatalf("len(CatalogMatches) = %d, want 1", len(d.CatalogMatches))
	}
	if d.CatalogMatches[0].Version != "" {
		t.Errorf("Version = %q, want empty string", d.CatalogMatches[0].Version)
	}
}

func TestNoPackageIdentifiedAllows(t *testing.T) {
	idx := newFakeCatalog(nxConsoleEntry(""))
	tc := ToolCall{
		ToolName:  "Read",
		ToolInput: map[string]any{},
	}

	d := Evaluate(tc, idx)

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

func TestPackageNameNormalizedBeforeLookup(t *testing.T) {
	// Index keys are lowercase/trimmed; mixed-case + padded input must still hit.
	idx := newFakeCatalog(catalog.Entry{
		ID:        "advisory-2026-case",
		Ecosystem: "npm",
		Package:   "mixedcase",
		Versions:  []string{"2.0.0"},
		Severity:  "medium",
	})
	tc := ToolCall{
		ToolInput: map[string]any{
			"ecosystem": "npm",
			"package":   "  MixedCase  ",
			"version":   "2.0.0",
		},
	}

	d := Evaluate(tc, idx)

	if d.Level != "warn" {
		t.Errorf("Level = %q, want %q (package name must be normalized before lookup)", d.Level, "warn")
	}
}

// TestEngineImportsArePure enforces the purity contract: engine.go must not
// import any package that performs I/O, concurrency, or wall-clock access. The
// test parses engine.go's import declarations and rejects the forbidden set.
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
		// imp.Path.Value includes surrounding quotes.
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("engine.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
