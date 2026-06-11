// Package coveragegate is the Phase-21 VAL-01 coverage gate: it walks every Go
// production file under internal/ and cmd/ and classifies each as
// package-tested, reason-coded-allowlisted, or UNACCOUNTED. The gate fails
// (TestCoverageManifest) on any UNACCOUNTED file, so a new production file in a
// test-less package cannot ship without either a test or a justified allowlist
// entry.
//
// Linkage is PACKAGE-LEVEL (a directory containing >=1 _test.go accounts every
// production file in it), not same-name-sibling: ~70/184 production files have
// no same-name _test.go yet are package-tested, so sibling linkage would drown
// the real gaps in false positives (RESEARCH Pitfall 1). The escape hatch is the
// fail-closed reason-coded allowlist in allowlist.go (VAL-08 self-defense).
//
// Enumeration is filesystem-based + go/parser-validated (NOT a regex //go:build
// scan), so the gate gives the SAME answer on every OS — a linux-only file is
// counted on the Windows dev box too, keeping coverage-allowlist.txt stable
// across platforms.
package coveragegate

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Status is the accounting classification of a production .go file.
type Status string

const (
	// StatusTested means the file's directory contains at least one _test.go.
	StatusTested Status = "package-tested"
	// StatusAllowlisted means the file has a valid reason-coded allowlist entry.
	StatusAllowlisted Status = "allowlisted"
	// StatusUnaccounted means the file is neither package-tested nor allowlisted.
	StatusUnaccounted Status = "UNACCOUNTED"
)

// FileStatus is the classification of a single production file. Path is
// module-root-relative with forward slashes (stable across OSes).
type FileStatus struct {
	Path   string
	Status Status
	Reason string // allowlist reason code, set only when Status == StatusAllowlisted
}

// DefaultSubdirs are the source roots the gate accounts. cmd/ and internal/
// hold all first-party Go; everything else (module root files, generated
// artifacts outside these trees) is out of scope.
var DefaultSubdirs = []string{"internal", "cmd"}

// ModuleRoot walks up from start until it finds the directory containing go.mod.
func ModuleRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("coveragegate: no go.mod found walking up from %s", start)
		}
		dir = parent
	}
}

// Walk classifies every non-test .go file under the given subdirs (relative to
// moduleRoot) using package-level test linkage and the allowlist. A nil
// allowlist accounts nothing via the allowlist (every non-package-tested file is
// UNACCOUNTED) — the fail-closed default.
func Walk(moduleRoot string, subdirs []string, al *Allowlist) ([]FileStatus, error) {
	testedDirs := map[string]bool{}
	var prodFiles []string
	fset := token.NewFileSet()

	for _, sub := range subdirs {
		rootDir := filepath.Join(moduleRoot, sub)
		if _, err := os.Stat(rootDir); err != nil {
			// A configured subdir that does not exist is a hard error: the gate
			// must not silently skip a tree it was told to account.
			return nil, fmt.Errorf("coveragegate: subdir %q: %w", sub, err)
		}
		walkErr := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "testdata", "vendor", "node_modules":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			dir := filepath.Dir(path)
			if strings.HasSuffix(path, "_test.go") {
				testedDirs[dir] = true
				return nil
			}
			// Validate it is parseable Go (uses go/parser, not a regex scan).
			if _, perr := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly); perr != nil {
				return fmt.Errorf("coveragegate: parsing %s: %w", path, perr)
			}
			prodFiles = append(prodFiles, path)
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}

	out := make([]FileStatus, 0, len(prodFiles))
	for _, p := range prodFiles {
		rel, err := relSlash(moduleRoot, p)
		if err != nil {
			return nil, err
		}
		switch {
		case testedDirs[filepath.Dir(p)]:
			out = append(out, FileStatus{Path: rel, Status: StatusTested})
		case al != nil && al.Has(rel):
			out = append(out, FileStatus{Path: rel, Status: StatusAllowlisted, Reason: al.Reason(rel)})
		default:
			out = append(out, FileStatus{Path: rel, Status: StatusUnaccounted})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Unaccounted returns just the UNACCOUNTED paths from a Walk result.
func Unaccounted(statuses []FileStatus) []string {
	var u []string
	for _, s := range statuses {
		if s.Status == StatusUnaccounted {
			u = append(u, s.Path)
		}
	}
	return u
}

func relSlash(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}
