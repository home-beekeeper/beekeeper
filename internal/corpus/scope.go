package corpus

import (
	"encoding/json"
	"errors"
)

// CorpusScope is the sharing-scope tag on every CorpusRecord. The Go zero value
// (empty string "") MUST serialize as "org_only" to prevent scope-tag leakage:
// a CorpusRecord{} (zero value) must never serialize as community_shareable.
//
// The MarshalJSON method is the SCOPE-01 guarantee — it is the only mechanism
// that enforces the zero-value constraint regardless of how the CorpusRecord
// is constructed. Constructor enforcement alone is insufficient because test
// code and zero-value struct literals can bypass it.
type CorpusScope string

const (
	// ScopeOrgOnly is the default and safe scope. Records stay within the local
	// machine in v1 and within the originating organization in v1.1+. This is
	// the zero-value sentinel: CorpusScope("") serializes identically to
	// CorpusScope("org_only") via MarshalJSON.
	ScopeOrgOnly CorpusScope = "org_only"

	// ScopeCommunityShareable indicates the record has been explicitly promoted
	// and anonymized for cross-organization sharing. Promotion requires the v2.0
	// anonymization gate; PromoteScope always returns an error in v1.
	ScopeCommunityShareable CorpusScope = "community_shareable"
)

// MarshalJSON implements json.Marshaler. The zero value ("") serializes as
// "org_only" rather than "", preventing scope-tag leakage from uninitialized
// CorpusRecord structs. A zero-value CorpusRecord{} therefore produces
// "scope":"org_only" in JSON output.
//
// This is the SCOPE-01 type-level guarantee (Pitfall 4 in PITFALLS.md).
func (s CorpusScope) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte(`"org_only"`), nil
	}
	return json.Marshal(string(s))
}

// PromoteScope returns an error in v1. Scope promotion to community_shareable
// requires an anonymization gate that strips all re-identifiable fields
// (repo_fingerprint, fleet_node_id, agent lineage). This gate is a v2.0
// deliverable and is not available in v1.
//
// PromoteScope is the ONLY code path that may change a CorpusRecord's scope.
// The function is a stub in v1 so the call site exists and the error is
// explicit from day one — any caller must handle the error, and there is no
// silent automatic promotion path (SCOPE-02).
//
// PromoteScope mutates nothing; r.Scope is unchanged on return.
func PromoteScope(r *CorpusRecord) error {
	return errors.New("corpus: scope promotion requires anonymization gate (v2.0); not available in v1")
}
