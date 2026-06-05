// Package quarantine manages the Beekeeper quarantine directory where
// flagged or malicious editor extensions are moved for isolation.
//
// The quarantine layout under quarantineDir is:
//
//	quarantineDir/
//	  extensions/
//	    <id>/               # each quarantined extension
//	      beekeeper-manifest.json
//	      <original extension content>
//
// All path inputs are sanitized with filepath.Base and validated against
// the ExtensionsDir prefix to prevent directory traversal attacks.
package quarantine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bantuson/beekeeper/internal/platform"
)

// manifestFileName is the name of the per-quarantine-entry metadata file
// written inside each quarantined extension directory.
const manifestFileName = "beekeeper-manifest.json"

// CatalogMatchSummary is a trimmed representation of a catalog match,
// recorded in the quarantine manifest for audit purposes. The quarantine
// package deliberately does NOT import internal/policy to avoid import cycles;
// the caller maps policy.CatalogMatch into CatalogMatchSummary before calling Move.
type CatalogMatchSummary struct {
	CatalogSource string `json:"catalog_source"`
	EntryID       string `json:"entry_id"`
	Severity      string `json:"severity"`
}

// Manifest is the metadata file written alongside each quarantined extension.
// It records the provenance, reason for quarantine, and audit trail needed
// to restore or investigate the extension later.
type Manifest struct {
	ID             string                `json:"id"`
	Publisher      string                `json:"publisher"`
	Name           string                `json:"name"`
	Version        string                `json:"version"`
	DisplayName    string                `json:"display_name"`
	OriginalPath   string                `json:"original_path"`
	QuarantinedAt  time.Time             `json:"quarantined_at"`
	Reason         string                `json:"reason"`
	RuleIDs        []string              `json:"rule_ids"`
	AuditRecordID  string                `json:"audit_record_id"`
	CatalogMatches []CatalogMatchSummary `json:"catalog_matches,omitempty"`
}

// ExtensionsDir returns the directory under quarantineDir where individual
// quarantined extension entries are stored.
func ExtensionsDir(quarantineDir string) string {
	return filepath.Join(quarantineDir, "extensions")
}

// Move quarantines the extension at extensionPath by moving it into the
// quarantine directory and writing a beekeeper-manifest.json alongside it.
//
// The returned id can be used with Restore or to display the entry in List.
//
// Cross-device moves (extensionPath and quarantineDir on different filesystems)
// are not supported in Phase 3; os.Rename returns a cross-device error which
// is propagated to the caller. The caller should surface this as a user-visible
// error instructing manual quarantine.
//
// All path components from m.Publisher, m.Name, and m.Version are sanitized
// with filepath.Base to prevent directory traversal in the generated id.
func Move(quarantineDir, extensionPath string, m Manifest) (id string, err error) {
	extDir := ExtensionsDir(quarantineDir)

	// Sanitize attacker-controlled fields before composing the entry id.
	pub := filepath.Base(m.Publisher)
	name := filepath.Base(m.Name)
	ver := filepath.Base(m.Version)
	id = fmt.Sprintf("%s.%s-%s-%d", pub, name, ver, time.Now().UnixNano())

	destDir := filepath.Join(extDir, id)

	// PATH TRAVERSAL GUARD: verify the resolved destDir is under ExtensionsDir.
	cleanDest := filepath.Clean(destDir)
	cleanExt := filepath.Clean(extDir)
	if !strings.HasPrefix(cleanDest, cleanExt+string(filepath.Separator)) {
		return "", fmt.Errorf("quarantine: destination %q is outside extensions dir %q", cleanDest, cleanExt)
	}

	if err := os.MkdirAll(extDir, 0o700); err != nil {
		return "", fmt.Errorf("quarantine: mkdir extensions dir: %w", err)
	}

	// Populate manifest fields.
	m.ID = id
	m.OriginalPath = extensionPath

	// Move the extension directory into quarantine.
	if err := os.Rename(extensionPath, destDir); err != nil {
		return "", fmt.Errorf("quarantine: move %q → %q: %w", extensionPath, destDir, err)
	}

	// Write the manifest file.
	manifestPath := filepath.Join(destDir, manifestFileName)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("quarantine: marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		return "", fmt.Errorf("quarantine: write manifest: %w", err)
	}

	// Enforce owner-only permissions on the manifest.
	if err := platform.SetOwnerOnly(manifestPath); err != nil {
		return "", fmt.Errorf("quarantine: set manifest permissions: %w", err)
	}

	return id, nil
}

// List returns all valid quarantine entries under quarantineDir.
// Entries whose directory is missing a readable beekeeper-manifest.json are
// silently skipped; they may be partial moves or externally introduced.
// If the extensions directory does not exist, an empty slice is returned
// (not an error — the quarantine is simply empty).
func List(quarantineDir string) ([]Manifest, error) {
	extDir := ExtensionsDir(quarantineDir)
	entries, err := os.ReadDir(extDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Manifest{}, nil
		}
		return nil, fmt.Errorf("quarantine: read extensions dir: %w", err)
	}

	var manifests []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(extDir, e.Name(), manifestFileName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			// Missing or unreadable manifest — skip this entry.
			continue
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			// Invalid manifest JSON — skip.
			continue
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// Restore moves a quarantined extension back to its original location.
// The id is sanitized with filepath.Base to prevent traversal attacks.
// Returns an error if the manifest has an empty OriginalPath, if the
// quarantine entry does not exist, or if the manifest OriginalPath
// would resolve outside the expected extensions root (TM-D-05).
//
// OriginalPath validation mirrors the entry-id guard discipline:
//   - Absolute paths are accepted when they resolve outside the quarantine
//     directory itself (restoring to the original install root is normal).
//   - Paths that are NOT absolute are checked for ".." traversal components
//     that would escape the parent of the extensions root.
//   - Any path that would resolve INSIDE quarantineDir is rejected to prevent
//     restore-to-quarantine cycles.
func Restore(quarantineDir, id string) error {
	// Strip any directory component from the caller-supplied id.
	safeID := filepath.Base(id)
	extDir := ExtensionsDir(quarantineDir)
	entryDir := filepath.Join(extDir, safeID)

	// PATH TRAVERSAL GUARD: verify entryDir is under ExtensionsDir.
	cleanEntry := filepath.Clean(entryDir)
	cleanExt := filepath.Clean(extDir)
	if !strings.HasPrefix(cleanEntry, cleanExt+string(filepath.Separator)) {
		return fmt.Errorf("quarantine: entry %q is outside extensions dir %q", cleanEntry, cleanExt)
	}

	// Verify the entry exists.
	if _, err := os.Stat(entryDir); err != nil {
		return fmt.Errorf("quarantine: entry %q not found: %w", safeID, err)
	}

	// Read the manifest to get the original path.
	manifestPath := filepath.Join(entryDir, manifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("quarantine: read manifest for %q: %w", safeID, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("quarantine: parse manifest for %q: %w", safeID, err)
	}
	if m.OriginalPath == "" {
		return fmt.Errorf("quarantine: manifest for %q has empty original_path", safeID)
	}

	// TM-D-05: validate OriginalPath before restoring to it.
	// A tampered manifest could set OriginalPath to an absolute attacker-chosen
	// location (e.g. /etc/... or C:\Windows\...) or a relative path with ".."
	// traversal components (e.g. ../../etc/cron.d).
	//
	// Rules applied (mirror entry-id guard discipline):
	//   1. Reject any path whose CLEAN form sits inside quarantineDir — this
	//      prevents restore-to-quarantine cycles.
	//   2. Reject any path whose individual components contain ".." — this
	//      catches relative traversals regardless of whether the path is
	//      absolute or relative.
	cleanOriginal := filepath.Clean(m.OriginalPath)
	cleanQuarantine := filepath.Clean(quarantineDir)

	// Rule 1: restoring into the quarantine directory itself is always wrong.
	if cleanOriginal == cleanQuarantine ||
		strings.HasPrefix(cleanOriginal, cleanQuarantine+string(filepath.Separator)) {
		return fmt.Errorf("quarantine: manifest original_path %q resolves inside quarantine dir %q — refusing restore", cleanOriginal, cleanQuarantine)
	}

	// Rule 2: reject ".." traversal components in the manifest-supplied path.
	// filepath.Clean normalises "a/../b" → "b", so we must check for ".."
	// segments in the RAW path (before cleaning), not in cleanOriginal.
	for _, part := range strings.FieldsFunc(m.OriginalPath, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return fmt.Errorf("quarantine: manifest original_path %q contains path traversal — refusing restore", m.OriginalPath)
		}
	}

	if err := os.Rename(entryDir, m.OriginalPath); err != nil {
		return fmt.Errorf("quarantine: restore %q → %q: %w", entryDir, m.OriginalPath, err)
	}
	return nil
}

// Purge removes all entries from the quarantine directory unconditionally.
// The CLI layer is responsible for any user-facing confirmation prompt;
// Purge itself performs no confirmation.
// Returns the list of purged entry IDs and any error encountered.
// On partial failure, already-purged IDs are still returned.
func Purge(quarantineDir string) (purged []string, err error) {
	extDir := ExtensionsDir(quarantineDir)
	entries, readErr := os.ReadDir(extDir)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("quarantine: read extensions dir for purge: %w", readErr)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		entryDir := filepath.Join(extDir, e.Name())
		if removeErr := os.RemoveAll(entryDir); removeErr != nil {
			err = fmt.Errorf("quarantine: purge %q: %w", e.Name(), removeErr)
			// Continue purging remaining entries; surface first error at the end.
			continue
		}
		purged = append(purged, e.Name())
	}
	return purged, err
}
