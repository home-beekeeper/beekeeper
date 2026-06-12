// Package quarantine manages the Beekeeper quarantine directory where
// flagged or malicious editor extensions and language packages are moved
// for isolation.
//
// The quarantine layout under quarantineDir is:
//
//	quarantineDir/
//	  extensions/
//	    <id>/               # each quarantined editor extension
//	      beekeeper-manifest.json
//	      <original extension content>
//	  packages/
//	    <id>/               # each quarantined language package
//	      beekeeper-manifest.json
//	      <original package content>
//
// All path inputs are sanitized with filepath.Base and validated against
// the per-type subdir prefix to prevent directory traversal attacks.
package quarantine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bantuson/beekeeper/internal/platform"
)

// manifestFileName is the name of the per-quarantine-entry metadata file
// written inside each quarantined artifact directory.
const manifestFileName = "beekeeper-manifest.json"

// ArtifactType classifies the type of quarantined artifact.
// Valid values are ArtifactTypeEditorExtension and ArtifactTypeLanguagePackage.
type ArtifactType = string

const (
	// ArtifactTypeEditorExtension is the type for editor extensions (VS Code, Cursor, etc.).
	// Entries land under extensions/ to preserve back-compat with the EDXT-03 layout.
	ArtifactTypeEditorExtension ArtifactType = "editor-extension"
	// ArtifactTypeLanguagePackage is the type for language packages (npm, pip, cargo, etc.).
	// Entries land under packages/.
	ArtifactTypeLanguagePackage ArtifactType = "language-package"
)

// CatalogMatchSummary is a trimmed representation of a catalog match,
// recorded in the quarantine manifest for audit purposes. The quarantine
// package deliberately does NOT import internal/policy to avoid import cycles;
// the caller maps policy.CatalogMatch into CatalogMatchSummary before calling Move.
type CatalogMatchSummary struct {
	CatalogSource string `json:"catalog_source"`
	EntryID       string `json:"entry_id"`
	Severity      string `json:"severity"`
}

// Manifest is the metadata file written alongside each quarantined artifact.
// It records the provenance, reason for quarantine, and audit trail needed
// to restore or investigate the artifact later.
type Manifest struct {
	ID             string                `json:"id"`
	ArtifactType   ArtifactType          `json:"artifact_type,omitempty"`
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
// quarantined editor-extension entries are stored.
func ExtensionsDir(quarantineDir string) string {
	return filepath.Join(quarantineDir, "extensions")
}

// PackagesDir returns the directory under quarantineDir where individual
// quarantined language-package entries are stored.
func PackagesDir(quarantineDir string) string {
	return filepath.Join(quarantineDir, "packages")
}

// subdirForType returns the per-type subdir root (extensions/ or packages/)
// for the given ArtifactType. Defaults to extensions/ for unknown/empty types
// to preserve back-compat.
func subdirForType(quarantineDir string, at ArtifactType) string {
	switch at {
	case ArtifactTypeLanguagePackage:
		return PackagesDir(quarantineDir)
	default:
		return ExtensionsDir(quarantineDir)
	}
}

// MoveTyped quarantines the artifact at artifactPath by moving it into the
// quarantine directory under the per-type subdir and writing a
// beekeeper-manifest.json alongside it.
//
// The returned id can be used with Restore or to display the entry in List.
//
// Cross-device moves (artifactPath and quarantineDir on different filesystems)
// are not supported; os.Rename returns a cross-device error which is propagated
// to the caller. The caller should surface this as a user-visible error
// instructing manual quarantine.
//
// All path components from m.Publisher, m.Name, and m.Version are sanitized
// with filepath.Base to prevent directory traversal in the generated id.
func MoveTyped(quarantineDir, artifactPath string, m Manifest) (id string, err error) {
	typeDir := subdirForType(quarantineDir, m.ArtifactType)

	// Sanitize attacker-controlled fields before composing the entry id.
	pub := filepath.Base(m.Publisher)
	name := filepath.Base(m.Name)
	ver := filepath.Base(m.Version)
	id = fmt.Sprintf("%s.%s-%s-%d", pub, name, ver, time.Now().UnixNano())

	destDir := filepath.Join(typeDir, id)

	// PATH TRAVERSAL GUARD: verify the resolved destDir is under the type subdir.
	cleanDest := filepath.Clean(destDir)
	cleanTypeDir := filepath.Clean(typeDir)
	if !strings.HasPrefix(cleanDest, cleanTypeDir+string(filepath.Separator)) {
		return "", fmt.Errorf("quarantine: destination %q is outside type dir %q", cleanDest, cleanTypeDir)
	}

	if err := os.MkdirAll(typeDir, 0o700); err != nil {
		return "", fmt.Errorf("quarantine: mkdir type dir: %w", err)
	}

	// Populate manifest fields.
	m.ID = id
	m.OriginalPath = artifactPath
	if m.ArtifactType == "" {
		m.ArtifactType = ArtifactTypeEditorExtension
	}

	// F-2: refuse to move a symlink/junction source. os.Rename on a path that
	// resolves through a reparse point would relocate the redirected target (a
	// sibling project or shared cache the install never owned) into quarantine.
	// Fail closed and leave the artifact in place, mirroring the cross-device
	// refusal contract above.
	if isLink, lerr := isReparsePoint(artifactPath); lerr != nil {
		return "", fmt.Errorf("quarantine: stat source %q: %w", artifactPath, lerr)
	} else if isLink {
		return "", fmt.Errorf("quarantine: refusing to move %q: source is a symlink or reparse point (junction)", artifactPath)
	}

	// Move the artifact directory into quarantine.
	if err := os.Rename(artifactPath, destDir); err != nil {
		return "", fmt.Errorf("quarantine: move %q -> %q: %w", artifactPath, destDir, err)
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

// Move quarantines the extension at extensionPath by moving it into the
// quarantine directory and writing a beekeeper-manifest.json alongside it.
// It is a thin wrapper around MoveTyped that defaults ArtifactType to
// "editor-extension" for back-compat with EDXT-03 callers.
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
	if m.ArtifactType == "" {
		m.ArtifactType = ArtifactTypeEditorExtension
	}
	return MoveTyped(quarantineDir, extensionPath, m)
}

// listSubdir reads valid manifest entries from a single quarantine subdir.
// Missing or malformed manifests are silently skipped.
func listSubdir(subdir string) ([]Manifest, error) {
	entries, err := os.ReadDir(subdir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Manifest{}, nil
		}
		return nil, fmt.Errorf("quarantine: read dir %q: %w", subdir, err)
	}

	var manifests []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(subdir, e.Name(), manifestFileName)
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

// List returns all valid quarantine entries under quarantineDir, spanning
// both extensions/ and packages/ subdirectories.
// Entries whose directory is missing a readable beekeeper-manifest.json are
// silently skipped; they may be partial moves or externally introduced.
// If neither subdir exists, an empty slice is returned (not an error).
func List(quarantineDir string) ([]Manifest, error) {
	var all []Manifest

	for _, subdir := range []string{ExtensionsDir(quarantineDir), PackagesDir(quarantineDir)} {
		ms, err := listSubdir(subdir)
		if err != nil {
			return nil, err
		}
		all = append(all, ms...)
	}
	return all, nil
}

// Restore moves a quarantined artifact back to its original location.
// The id is sanitized with filepath.Base to prevent traversal attacks.
// The manifest's ArtifactType determines which subdir is searched first;
// if not found there, the other subdir is tried (handles manifests without
// an explicit ArtifactType written by the old Move).
//
// Returns an error if the manifest has an empty OriginalPath, if the
// quarantine entry does not exist, or if the manifest OriginalPath
// would resolve outside the expected path rules (TM-D-05).
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

	// Try each subdir in order: extensions first (back-compat), then packages.
	subdirs := []string{ExtensionsDir(quarantineDir), PackagesDir(quarantineDir)}
	var entryDir string
	var foundSubdir string
	for _, subdir := range subdirs {
		candidate := filepath.Join(subdir, safeID)
		// PATH TRAVERSAL GUARD.
		cleanEntry := filepath.Clean(candidate)
		cleanSub := filepath.Clean(subdir)
		if !strings.HasPrefix(cleanEntry, cleanSub+string(filepath.Separator)) {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			entryDir = candidate
			foundSubdir = subdir
			break
		}
	}

	if entryDir == "" {
		// Entry not found in either subdir — return the extensions-subdir error
		// for consistency with the original error message format.
		extDir := ExtensionsDir(quarantineDir)
		cleanEntry := filepath.Clean(filepath.Join(extDir, safeID))
		cleanExt := filepath.Clean(extDir)
		if !strings.HasPrefix(cleanEntry, cleanExt+string(filepath.Separator)) {
			return fmt.Errorf("quarantine: entry %q is outside extensions dir %q", cleanEntry, cleanExt)
		}
		return fmt.Errorf("quarantine: entry %q not found", safeID)
	}
	_ = foundSubdir

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
	//
	// validationPath is the form used for the containment + ".." checks below.
	// On Windows it is the extended-length-prefix-stripped form so the prefix
	// containment check cannot be evaded with a \\?\ prefix; on POSIX it is the
	// raw OriginalPath.
	validationPath := m.OriginalPath

	// F-3: Windows-complete canonicalization. These path forms are Windows-only
	// (filepath.VolumeName is "" and absolute paths start with "/" on POSIX), so
	// guard the whole block on GOOS to avoid disturbing POSIX behavior.
	if runtime.GOOS == "windows" {
		// Reject drive-relative paths like "C:foo" (no separator after the
		// volume). These resolve against the current directory on drive C:, not
		// the drive root, and contain no ".." — filepath.IsAbs is false for them.
		if !filepath.IsAbs(m.OriginalPath) {
			return fmt.Errorf("quarantine: manifest original_path %q is not absolute (drive-relative or relative) — refusing restore", m.OriginalPath)
		}

		// Reject extended-length / UNC prefixes (\\?\ and \\?\UNC\). filepath.Clean
		// does not strip \\?\, so a prefixed absolute path could otherwise dodge
		// the HasPrefix(quarantineDir) containment check while still naming an
		// arbitrary location. A restored artifact's OriginalPath has no legitimate
		// reason to carry an extended-length prefix, so reject outright (and also
		// strip it for the residual checks below, belt-and-suspenders).
		stripped := m.OriginalPath
		if strings.HasPrefix(stripped, `\\?\UNC\`) || strings.HasPrefix(stripped, `\\?\`) {
			return fmt.Errorf("quarantine: manifest original_path %q uses an extended-length (\\\\?\\) prefix — refusing restore", m.OriginalPath)
		}
		validationPath = stripped

		// Reject Alternate Data Stream syntax: a ":" appearing AFTER the volume
		// name (e.g. C:\x\y.txt:ads). The volume colon (index 1) is legitimate.
		vol := filepath.VolumeName(stripped)
		rest := stripped[len(vol):]
		if strings.Contains(rest, ":") {
			return fmt.Errorf("quarantine: manifest original_path %q contains an alternate-data-stream ':' — refusing restore", m.OriginalPath)
		}

		// Reject any path component with a trailing dot or space. Windows strips
		// these silently at the filesystem layer, so "C:\x " and "C:\x." can name
		// an unintended target while comparing unequal byte-wise.
		for _, part := range strings.FieldsFunc(stripped, func(r rune) bool {
			return r == '/' || r == '\\'
		}) {
			if part == "" {
				continue
			}
			if last := part[len(part)-1]; last == '.' || last == ' ' {
				return fmt.Errorf("quarantine: manifest original_path component %q has a trailing dot or space — refusing restore", part)
			}
		}
	}

	cleanOriginal := filepath.Clean(validationPath)
	cleanQuarantine := filepath.Clean(quarantineDir)

	// Rule 1: restoring into the quarantine directory itself is always wrong.
	if cleanOriginal == cleanQuarantine ||
		strings.HasPrefix(cleanOriginal, cleanQuarantine+string(filepath.Separator)) {
		return fmt.Errorf("quarantine: manifest original_path %q resolves inside quarantine dir %q — refusing restore", cleanOriginal, cleanQuarantine)
	}

	// Rule 2: reject ".." traversal components in the manifest-supplied path.
	// filepath.Clean normalises "a/../b" -> "b", so we must check for ".."
	// segments in the RAW path (before cleaning), not in cleanOriginal. Use the
	// prefix-stripped validationPath so a \\?\-prefixed traversal is also caught.
	for _, part := range strings.FieldsFunc(validationPath, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return fmt.Errorf("quarantine: manifest original_path %q contains path traversal — refusing restore", m.OriginalPath)
		}
	}

	// F-2: refuse to restore through a symlink/junction. Reject if the quarantine
	// entry source itself is a reparse point.
	if isLink, lerr := isReparsePoint(entryDir); lerr != nil {
		return fmt.Errorf("quarantine: stat entry %q: %w", entryDir, lerr)
	} else if isLink {
		return fmt.Errorf("quarantine: refusing to restore %q: quarantine entry is a symlink or reparse point", safeID)
	}

	// F-2: re-assert the RESOLVED parent of the destination is not inside the
	// quarantine dir. A junctioned restore target (parent of OriginalPath is a
	// junction pointing back into quarantine, or the destination parent is a
	// symlink) would otherwise route the rename to an operator-unexpected
	// location. EvalSymlinks may fail when the parent does not yet exist; in
	// that case fall back to the lexical parent (already validated above), which
	// preserves the fail-closed posture.
	destParent := filepath.Dir(m.OriginalPath)
	if resolvedParent, evalErr := filepath.EvalSymlinks(destParent); evalErr == nil {
		cleanResolvedParent := filepath.Clean(resolvedParent)
		if cleanResolvedParent == cleanQuarantine ||
			strings.HasPrefix(cleanResolvedParent, cleanQuarantine+string(filepath.Separator)) {
			return fmt.Errorf("quarantine: resolved restore parent %q is inside quarantine dir %q — refusing restore", cleanResolvedParent, cleanQuarantine)
		}
	}

	if err := os.Rename(entryDir, m.OriginalPath); err != nil {
		return fmt.Errorf("quarantine: restore %q -> %q: %w", entryDir, m.OriginalPath, err)
	}
	return nil
}

// purgeSubdir removes all entries in subdir and returns purged entry IDs.
func purgeSubdir(subdir string) (purged []string, err error) {
	entries, readErr := os.ReadDir(subdir)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("quarantine: read dir %q for purge: %w", subdir, readErr)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		entryDir := filepath.Join(subdir, e.Name())
		if removeErr := os.RemoveAll(entryDir); removeErr != nil {
			err = fmt.Errorf("quarantine: purge %q: %w", e.Name(), removeErr)
			// Continue purging remaining entries; surface first error at the end.
			continue
		}
		purged = append(purged, e.Name())
	}
	return purged, err
}

// Purge removes all entries from the quarantine directory unconditionally,
// spanning both extensions/ and packages/ subdirectories.
// The CLI layer is responsible for any user-facing confirmation prompt;
// Purge itself performs no confirmation.
// Returns the list of purged entry IDs and any error encountered.
// On partial failure, already-purged IDs are still returned.
func Purge(quarantineDir string) (purged []string, err error) {
	for _, subdir := range []string{ExtensionsDir(quarantineDir), PackagesDir(quarantineDir)} {
		ids, subErr := purgeSubdir(subdir)
		purged = append(purged, ids...)
		if subErr != nil && err == nil {
			err = subErr
		}
	}
	return purged, err
}
