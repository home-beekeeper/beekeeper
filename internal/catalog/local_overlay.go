// Package catalog — local_overlay.go
//
// Local catalog overlay: an owner-only, sync-immune two-file overlay
// (local-overlay.json + local-overlay.idx) for confirmed-malicious packages.
//
// The overlay files are stored in <catalogDir> alongside bumblebee.json and
// bumblebee.idx, but are never touched by SyncConditional (which writes ONLY
// bumblebee.* — VERIFIED: sync.go lines 138-143). They therefore survive
// every catalogs sync automatically (FRB-05).
//
// Security properties (T-24-OVR-TAMPER):
//   - Both files are written via writeFileAtomic + platform.SetOwnerOnly
//     (0600 / Windows owner-DACL).
//   - Overlay entries carry CatalogSignature="" (unsigned) → warn-only per
//     CTLG-07. A single overlay source yields source_count:1; enforce requires
//     a second independent signed source.
//
// Import constraints: NO internal/tui, NO internal/corpus (import boundary
// ADJ-01 / Pitfall 1 from 24-RESEARCH.md).
package catalog

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// maxOverlayEntries is the maximum number of entries the local overlay will
// grow to. At the cap, AddLocalOverlayEntry logs a warning and returns nil
// without adding the entry (Pitfall 6 from 24-RESEARCH.md: O(n) rebuild
// per add is acceptable at v1 sizes; cap prevents pathological growth).
const maxOverlayEntries = 1000

// LoadLocalOverlay reads <catalogDir>/local-overlay.json and returns its
// entries. Missing file → (nil, nil) (first run — not an error). Malformed
// JSON → error.
func LoadLocalOverlay(catalogDir string) ([]Entry, error) {
	path := filepath.Join(catalogDir, "local-overlay.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // first run — not an error
		}
		return nil, fmt.Errorf("read local overlay: %w", err)
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse local overlay: %w", err)
	}
	return entries, nil
}

// AddLocalOverlayEntry appends e to the local catalog overlay and rebuilds the
// binary index (local-overlay.idx). It is idempotent: if an entry with the same
// ecosystem+package (case-insensitive) already exists, it returns nil without
// writing. Growth is capped at maxOverlayEntries — at the cap a warning is
// logged and nil is returned (no error, sync continues).
//
// Both overlay files (local-overlay.json, local-overlay.idx) are written via
// writeFileAtomic and hardened to owner-only via platform.SetOwnerOnly.
//
// e.CatalogSignature MUST be "" (unsigned — overlay entries are warn-only per
// CTLG-07). The caller is responsible for setting e.CatalogSource="local-overlay".
func AddLocalOverlayEntry(catalogDir string, e Entry) error {
	entries, err := LoadLocalOverlay(catalogDir)
	if err != nil {
		return err
	}

	// Idempotency: skip if an entry with the same ecosystem+package already exists.
	for _, existing := range entries {
		if strings.EqualFold(existing.Ecosystem, e.Ecosystem) &&
			strings.EqualFold(existing.Package, e.Package) {
			return nil // already present — idempotent
		}
	}

	// Cap guard (Pitfall 6): v1 is local-only; no fleet push. Log + return nil.
	if len(entries) >= maxOverlayEntries {
		log.Printf("beekeeper: local overlay: entry cap %d reached, skipping %s/%s",
			maxOverlayEntries, e.Ecosystem, e.Package)
		return nil
	}

	entries = append(entries, e)

	// Write JSON atomically (writeFileAtomic is package-private in index.go;
	// same package — call directly without export).
	jsonPath := filepath.Join(catalogDir, "local-overlay.json")
	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal local overlay: %w", err)
	}
	if err := writeFileAtomic(jsonPath, data); err != nil {
		return fmt.Errorf("write local overlay json: %w", err)
	}
	// Enforce owner-only on the JSON file (T-24-OVR-TAMPER).
	if err := platform.SetOwnerOnly(jsonPath); err != nil {
		return fmt.Errorf("enforce owner-only on local overlay json: %w", err)
	}

	// Rebuild the mmap binary index (BuildIndex is package-private in index.go;
	// same package — call directly). BuildIndex uses writeFileAtomic internally.
	idxPath := filepath.Join(catalogDir, "local-overlay.idx")
	if err := BuildIndex(idxPath, entries); err != nil {
		return fmt.Errorf("rebuild local overlay index: %w", err)
	}
	// Enforce owner-only on the index (T-24-OVR-TAMPER).
	if err := platform.SetOwnerOnly(idxPath); err != nil {
		return fmt.Errorf("enforce owner-only on local overlay index: %w", err)
	}

	return nil
}
