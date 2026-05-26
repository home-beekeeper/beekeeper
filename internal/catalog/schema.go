// Package catalog implements Bumblebee threat-intel catalog parsing, signature
// presence checks, a memory-mappable binary index for O(log n) lookup at hook
// evaluation time, and the `beekeeper catalogs sync` orchestration.
//
// The schema types in this file model the verified Bumblebee catalog format
// (schema_version "0.1.0") plus Beekeeper-specific extension fields
// (source_url, catalog_signature, catalog_source) per CTLG-01.
package catalog

// SupportedSchemaVersion is the only Bumblebee catalog schema_version Beekeeper
// accepts in Phase 1. Unknown versions are rejected (not silently parsed) so a
// breaking upstream schema change is detected immediately rather than producing
// malformed entries. See ValidateSchemaVersion.
const SupportedSchemaVersion = "0.1.0"

// Entry is a single threat-intel catalog record. The first six fields mirror
// the verified Bumblebee entry schema; the trailing three are Beekeeper
// extensions used by the policy engine and provenance tracking.
type Entry struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Ecosystem string   `json:"ecosystem"` // npm|pypi|go|rubygems|packagist|cargo|editor-extension
	Package   string   `json:"package"`
	Versions  []string `json:"versions"`
	Severity  string   `json:"severity"` // critical|high|medium|low

	// Beekeeper extensions.
	SourceURL        string `json:"source_url"`        // upstream advisory URL
	CatalogSignature string `json:"catalog_signature"` // empty => unsigned (warn-only in Phase 1)
	CatalogSource    string `json:"catalog_source"`    // provenance, e.g. "bumblebee"
}

// CatalogFile is a parsed Bumblebee catalog JSON document. Bumblebee requires
// an object with a top-level entries[] array; a bare JSON array is invalid.
type CatalogFile struct {
	SchemaVersion string  `json:"schema_version"`
	Entries       []Entry `json:"entries"`
}
