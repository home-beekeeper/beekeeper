package catalog

import (
	"testing"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// fakeMultiLookup is a test double for policy.MultiCatalogLookup that returns
// a fixed slice of CatalogMatches keyed by "ecosystem::pkg".
type fakeMultiLookup struct {
	matches map[string][]policy.CatalogMatch
}

func (f fakeMultiLookup) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	return f.matches[ecosystem+"::"+pkg]
}

// buildTestIndexWithEntry creates a small real mmap index in t.TempDir()
// containing the given entry and returns the opened *Index. Caller must Close.
func buildTestIndexWithEntry(t *testing.T, e Entry) *Index {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/test.idx"
	if err := BuildIndex(path, []Entry{e}); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	idx, err := OpenIndex(path)
	if err != nil {
		t.Fatalf("OpenIndex: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })
	return idx
}

func TestMultiIndexAggregatesAllSources(t *testing.T) {
	// Bumblebee index with one entry for npm::evil-pkg@1.0.0
	bbEntry := Entry{
		ID:            "bb-001",
		Ecosystem:     "npm",
		Package:       "evil-pkg",
		Versions:      []string{"1.0.0"},
		Severity:      "critical",
		CatalogSource: "bumblebee",
	}
	bbIdx := buildTestIndexWithEntry(t, bbEntry)

	// Fake OSV and Socket adapters that both return a match for the same package.
	osvMatch := policy.CatalogMatch{
		CatalogSource:  "osv",
		EntryID:        "osv-001",
		Ecosystem:      "npm",
		Package:        "evil-pkg",
		Version:        "1.0.0",
		Severity:       "high",
		Signed:         true,
		CatalogVersion: "osv-api",
	}
	socketMatch := policy.CatalogMatch{
		CatalogSource:  "socket",
		EntryID:        "sock-001",
		Ecosystem:      "npm",
		Package:        "evil-pkg",
		Version:        "1.0.0",
		Severity:       "critical",
		Signed:         true,
		CatalogVersion: "socket-api",
	}

	osvAdapter := fakeMultiLookup{matches: map[string][]policy.CatalogMatch{
		"npm::evil-pkg": {osvMatch},
	}}
	socketAdapter := fakeMultiLookup{matches: map[string][]policy.CatalogMatch{
		"npm::evil-pkg": {socketMatch},
	}}

	mi := NewMultiIndex(bbIdx, osvAdapter, socketAdapter)

	got := mi.LookupAll("npm", "evil-pkg")

	if len(got) == 0 {
		t.Fatal("LookupAll returned no matches, want ≥3")
	}

	// Expect at least one match per source.
	sources := map[string]bool{}
	for _, m := range got {
		sources[m.CatalogSource] = true
	}
	if !sources["bumblebee"] {
		t.Error("missing bumblebee match")
	}
	if !sources["osv"] {
		t.Error("missing osv match")
	}
	if !sources["socket"] {
		t.Error("missing socket match")
	}
}

func TestMultiIndexSkipsNilSources(t *testing.T) {
	// Bumblebee index with one entry.
	bbEntry := Entry{
		ID:        "bb-002",
		Ecosystem: "npm",
		Package:   "evil-pkg",
		Versions:  []string{"2.0.0"},
		Severity:  "critical",
	}
	bbIdx := buildTestIndexWithEntry(t, bbEntry)

	osvMatch := policy.CatalogMatch{
		CatalogSource: "osv",
		EntryID:       "osv-002",
		Ecosystem:     "npm",
		Package:       "evil-pkg",
		Version:       "2.0.0",
		Signed:        true,
	}
	osvAdapter := fakeMultiLookup{matches: map[string][]policy.CatalogMatch{
		"npm::evil-pkg": {osvMatch},
	}}

	// Socket is nil — should be silently skipped.
	mi := NewMultiIndex(bbIdx, osvAdapter, nil)

	got := mi.LookupAll("npm", "evil-pkg")

	sources := map[string]bool{}
	for _, m := range got {
		sources[m.CatalogSource] = true
	}

	if !sources["bumblebee"] {
		t.Error("missing bumblebee match")
	}
	if !sources["osv"] {
		t.Error("missing osv match")
	}
	if sources["socket"] {
		t.Error("unexpected socket match — Socket was nil")
	}
}

func TestMultiIndexMissReturnsDiffentSentinel(t *testing.T) {
	// Index with a different package — lookup should return no real matches,
	// but CTLG-09: a configured source that found nothing returns a dissent
	// sentinel (CatalogMatch{CatalogSource: "bumblebee", Dissented: true}).
	bbEntry := Entry{
		ID:        "bb-003",
		Ecosystem: "npm",
		Package:   "other-pkg",
		Versions:  []string{"1.0.0"},
	}
	bbIdx := buildTestIndexWithEntry(t, bbEntry)

	mi := NewMultiIndex(bbIdx, nil, nil)

	got := mi.LookupAll("npm", "evil-pkg")

	// Expect exactly one dissent sentinel for bumblebee (no real matches).
	if len(got) != 1 {
		t.Fatalf("LookupAll returned %d matches, want 1 (dissent sentinel)", len(got))
	}
	if !got[0].Dissented {
		t.Error("Dissented = false, want true (no match from bumblebee → dissent)")
	}
	if got[0].CatalogSource != "bumblebee" {
		t.Errorf("CatalogSource = %q, want bumblebee", got[0].CatalogSource)
	}
}

func TestMultiIndexNilBumblebeeSkipped(t *testing.T) {
	osvMatch := policy.CatalogMatch{
		CatalogSource: "osv",
		EntryID:       "osv-004",
		Ecosystem:     "npm",
		Package:       "evil-pkg",
		Signed:        true,
	}
	osvAdapter := fakeMultiLookup{matches: map[string][]policy.CatalogMatch{
		"npm::evil-pkg": {osvMatch},
	}}

	// Bumblebee is nil — should not panic.
	mi := NewMultiIndex(nil, osvAdapter, nil)

	got := mi.LookupAll("npm", "evil-pkg")
	if len(got) != 1 || got[0].CatalogSource != "osv" {
		t.Errorf("got %v, want 1 osv match", got)
	}
}
