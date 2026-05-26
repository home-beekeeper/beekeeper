// Package watch implements the beekeeper file-watcher daemon (Layer 2 of the
// editor extension defense). It monitors extension installation directories,
// parses extension manifests, evaluates new extensions against the catalog, and
// quarantines malicious ones.
package watch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ExtensionManifest holds the fields from an extension's package.json that
// Beekeeper needs for catalog lookup and quarantine provenance.
type ExtensionManifest struct {
	Publisher   string `json:"publisher"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	DisplayName string `json:"displayName"`
}

// ErrNoManifest is returned when extensionDir does not contain a valid
// extension manifest. This is the normal case for non-extension directories
// (e.g. the `.obsolete` marker, `extensions.json`) and is treated as a
// no-op by the handler.
var ErrNoManifest = errors.New("no extension manifest")

// maxManifestSize is the maximum accepted size for a package.json file.
// Files larger than this are rejected without parsing to bound memory use.
const maxManifestSize = 1 << 20 // 1 MiB

// ParseManifest reads and parses the package.json from extensionDir.
//
// Returns ErrNoManifest when:
//   - the file does not exist
//   - the parsed Publisher or Name field is empty (filters non-extension entries
//     like `.obsolete` and `extensions.json`)
//
// Returns an error (not ErrNoManifest) when:
//   - the file cannot be read for other reasons
//   - the file exceeds 1 MiB
//   - the file contains invalid JSON
func ParseManifest(extensionDir string) (ExtensionManifest, error) {
	path := filepath.Join(extensionDir, "package.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ExtensionManifest{}, ErrNoManifest
		}
		return ExtensionManifest{}, err
	}

	if len(data) > maxManifestSize {
		return ExtensionManifest{}, errors.New("watch: package.json exceeds 1 MiB limit")
	}

	var m ExtensionManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ExtensionManifest{}, err
	}

	if m.Publisher == "" || m.Name == "" {
		return ExtensionManifest{}, ErrNoManifest
	}

	return m, nil
}
