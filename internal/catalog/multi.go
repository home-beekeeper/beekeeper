// Package catalog provides the multi-source catalog aggregator for Plan 08.
// MultiIndex aggregates Bumblebee (mmap index), OSV, and Socket adapters into a
// single policy.MultiCatalogLookup implementation passed to policy.Evaluate.
package catalog

import "github.com/bantuson/beekeeper/internal/policy"

// MultiIndex aggregates the three independent threat-intel sources into a single
// policy.MultiCatalogLookup. It is the concrete aggregator wired by the hook
// handler to satisfy CTLG-09 (per-source provenance in every audit record) and
// PLCY-01 (corroboration-based block enforcement).
//
// Nil sub-adapters are silently skipped — Socket is nil when no API token is
// configured; OSV is nil in test stubs or when explicitly disabled.
// Overlay is nil when no local-overlay.idx has been built yet (first run).
type MultiIndex struct {
	// Bumblebee is the mmap-backed local index (the only source in Phase 1).
	// It is wrapped internally as a bumblebeeAdapter for LookupAll semantics.
	Bumblebee *Index

	// Overlay is the local catalog overlay index (local-overlay.idx).
	// Nil when absent (first run or overlay not yet written). Added in Phase 24
	// (FRB-05). Queried after Bumblebee with CatalogSource "local-overlay".
	Overlay *Index

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

// NewMultiIndexWithOverlay constructs a MultiIndex that additionally queries
// a local catalog overlay index at overlayPath. If overlayPath is empty or the
// index cannot be opened, Overlay stays nil (silently degraded — no overlay
// match is no block, consistent with fail-closed). Existing callers that use
// NewMultiIndex are unaffected (additive extension, mirrors Phase 23
// NewMultiSinkWithCorpus pattern).
//
// RED stub: Overlay is always nil until Task 3 implements this fully.
func NewMultiIndexWithOverlay(bumblebee *Index, osv, socket policy.MultiCatalogLookup, overlayPath string) *MultiIndex {
	m := NewMultiIndex(bumblebee, osv, socket)
	// RED stub — overlayPath ignored; Overlay remains nil. Task 3 implements this.
	_ = overlayPath
	return m
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
//
// CTLG-09 dissent tracking: when a configured (non-nil) source finds no match
// for the queried package, a dissent sentinel (CatalogMatch{Dissented: true}) is
// appended. The policy engine's corroborate() filters these into SourcesDissented
// so forensic provenance can trace which sources explicitly did NOT flag a package.
func (m *MultiIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	var matches []policy.CatalogMatch

	if m.Bumblebee != nil {
		adapter := &bumblebeeMultiAdapter{idx: m.Bumblebee}
		got := adapter.LookupAll(ecosystem, pkg)
		if len(got) > 0 {
			matches = append(matches, got...)
		} else {
			// Bumblebee was queried but found no match — record as dissenting.
			matches = append(matches, policy.CatalogMatch{
				CatalogSource: "bumblebee",
				Dissented:     true,
			})
		}
	}

	if m.OSV != nil {
		got := m.OSV.LookupAll(ecosystem, pkg)
		if len(got) > 0 {
			matches = append(matches, got...)
		} else {
			// OSV was queried but found no match — record as dissenting.
			matches = append(matches, policy.CatalogMatch{
				CatalogSource: "osv",
				Dissented:     true,
			})
		}
	}

	if m.Socket != nil {
		got := m.Socket.LookupAll(ecosystem, pkg)
		if len(got) > 0 {
			matches = append(matches, got...)
		} else {
			// Socket was queried but found no match — record as dissenting.
			matches = append(matches, policy.CatalogMatch{
				CatalogSource: "socket",
				Dissented:     true,
			})
		}
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
