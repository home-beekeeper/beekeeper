// Package catalog — local_overlay.go
//
// Local catalog overlay: an owner-only, sync-immune two-file overlay
// (local-overlay.json + local-overlay.idx) that survives catalogs sync.
//
// Implementation is in Task 3 (feat(24-01): local catalog overlay + MultiIndex).
package catalog

// LoadLocalOverlay reads <catalogDir>/local-overlay.json and returns its
// entries. Missing file → (nil, nil) (first run — not an error).
// Malformed JSON → error.
//
// Implementation pending Task 3.
func LoadLocalOverlay(catalogDir string) ([]Entry, error) {
	// RED stub — always returns nil, nil. Tests will fail until Task 3 implements this.
	return nil, nil
}

// AddLocalOverlayEntry appends e to the local overlay and rebuilds
// local-overlay.idx. It is idempotent on ecosystem+package (EqualFold).
// Both overlay files are written owner-only (platform.SetOwnerOnly).
// Growth is capped at maxOverlayEntries=1000 (logged warning, not error).
//
// Implementation pending Task 3.
func AddLocalOverlayEntry(catalogDir string, e Entry) error {
	// RED stub — always returns nil without writing any files.
	// Tests that rely on files being present will fail until Task 3 implements this.
	return nil
}
