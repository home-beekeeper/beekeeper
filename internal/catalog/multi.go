// Package catalog provides the multi-source catalog aggregator for Plan 08.
// MultiIndex aggregates Bumblebee (mmap index), OSV, and Socket adapters into a
// single policy.MultiCatalogLookup implementation passed to policy.Evaluate.
package catalog

import "github.com/mzansi-agentive/beekeeper/internal/policy"

// MultiIndex aggregates the three independent threat-intel sources into a single
// policy.MultiCatalogLookup. It is the concrete aggregator wired by the hook
// handler to satisfy CTLG-09 (per-source provenance in every audit record) and
// PLCY-01 (corroboration-based block enforcement).
//
// Nil sub-adapters are silently skipped — Socket is nil when no API token is
// configured; OSV is nil in test stubs or when explicitly disabled.
type MultiIndex struct {
	// Bumblebee is the mmap-backed local index (the only source in Phase 1).
	// It is wrapped internally as a bumblebeeAdapter for LookupAll semantics.
	Bumblebee *Index

	// OSV is the OSV REST API adapter. Nil when OSV is unavailable or disabled.
	OSV policy.MultiCatalogLookup

	// Socket is the Socket PURL API adapter. Nil when the API token is absent.
	Socket policy.MultiCatalogLookup
}

// NewMultiIndex constructs a MultiIndex from the three independent catalog sources.
// bumblebee must not be nil (it is always required). osv and socket may be nil
// (they are skipped when absent).
func NewMultiIndex(bumblebee *Index, osv, socket policy.MultiCatalogLookup) *MultiIndex {
	return &MultiIndex{
		Bumblebee: bumblebee,
		OSV:       osv,
		Socket:    socket,
	}
}

// LookupAll satisfies policy.MultiCatalogLookup. It queries each non-nil source
// in turn and returns the concatenated results:
//
//  1. Bumblebee via the internal bumblebeeMultiAdapter (mmap, O(log n), no I/O).
//  2. OSV (pre-resolved by the caller's adapter; no network call here).
//  3. Socket (pre-resolved; nil when disabled).
//
// Nil sub-adapters are skipped without error — degraded sources simply contribute
// zero matches rather than blocking the entire check.
func (m *MultiIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	var matches []policy.CatalogMatch

	if m.Bumblebee != nil {
		adapter := &bumblebeeMultiAdapter{idx: m.Bumblebee}
		matches = append(matches, adapter.LookupAll(ecosystem, pkg)...)
	}

	if m.OSV != nil {
		matches = append(matches, m.OSV.LookupAll(ecosystem, pkg)...)
	}

	if m.Socket != nil {
		matches = append(matches, m.Socket.LookupAll(ecosystem, pkg)...)
	}

	return matches
}

// Close releases the underlying Bumblebee mmap index. It satisfies io.Closer so
// the hook handler can defer idx.Close() on the MultiIndex directly.
func (m *MultiIndex) Close() error {
	if m.Bumblebee != nil {
		return m.Bumblebee.Close()
	}
	return nil
}

// bumblebeeMultiAdapter wraps *Index for use inside MultiIndex.LookupAll.
// It is distinct from the bumblebeeAdapter in selftest.go so the two can evolve
// independently; this adapter lives in package catalog (no cross-package dep).
type bumblebeeMultiAdapter struct {
	idx *Index
}

// LookupAll maps an Index.Lookup result to []policy.CatalogMatch with
// CatalogSource "bumblebee". Returns nil on miss. One match per version in
// Entry.Versions so the policy engine can filter by the extracted version.
func (a *bumblebeeMultiAdapter) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	e, ok := a.idx.Lookup(ecosystem, pkg)
	if !ok {
		return nil
	}

	signed := e.CatalogSignature != ""
	// CatalogVersion carries the source name from the entry as the catalog
	// version identifier (Phase 1 entries set this to "bumblebee").
	catalogVersion := e.CatalogSource
	if catalogVersion == "" {
		catalogVersion = "bumblebee"
	}

	if len(e.Versions) == 0 {
		return []policy.CatalogMatch{{
			CatalogSource:  "bumblebee",
			EntryID:        e.ID,
			Ecosystem:      e.Ecosystem,
			Package:        e.Package,
			Severity:       e.Severity,
			Signed:         signed,
			CatalogVersion: catalogVersion,
		}}
	}

	out := make([]policy.CatalogMatch, 0, len(e.Versions))
	for _, v := range e.Versions {
		out = append(out, policy.CatalogMatch{
			CatalogSource:  "bumblebee",
			EntryID:        e.ID,
			Ecosystem:      e.Ecosystem,
			Package:        e.Package,
			Version:        v,
			Severity:       e.Severity,
			Signed:         signed,
			CatalogVersion: catalogVersion,
		})
	}
	return out
}
