package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// defaultCatalogSource is applied to entries whose catalog_source is absent in
// the source JSON. Phase 1 only ingests Bumblebee, so this is the provenance
// recorded for all synced entries (CTLG-01).
const defaultCatalogSource = "bumblebee"

// ValidateSchemaVersion returns an error if v is not the schema version
// Beekeeper supports. Rejecting unknown versions (rather than best-effort
// parsing) means an upstream Bumblebee schema bump fails loudly at sync time.
func ValidateSchemaVersion(v string) error {
	if v != SupportedSchemaVersion {
		return fmt.Errorf("unsupported catalog schema_version %q: expected %q", v, SupportedSchemaVersion)
	}
	return nil
}

// ParseCatalogFile decodes a Bumblebee catalog JSON document into a typed
// CatalogFile. It rejects a bare top-level JSON array (Bumblebee requires an
// object with entries[]) and rejects unknown schema versions. Entries missing
// catalog_source are defaulted to "bumblebee".
func ParseCatalogFile(data []byte) (CatalogFile, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	var cf CatalogFile
	if err := dec.Decode(&cf); err != nil {
		// A bare array (e.g. `[...]`) fails to decode into the struct and is
		// surfaced here with context rather than silently accepted.
		return CatalogFile{}, fmt.Errorf("decode catalog file: %w", err)
	}

	if err := ValidateSchemaVersion(cf.SchemaVersion); err != nil {
		return CatalogFile{}, err
	}

	for i := range cf.Entries {
		if cf.Entries[i].CatalogSource == "" {
			cf.Entries[i].CatalogSource = defaultCatalogSource
		}
	}

	return cf, nil
}
