package policyloader

import (
	"os"
	"path/filepath"
	"testing"
)

// testdataDir resolves the local testdata/ directory for the policyloader package.
// Since testdata is local to this package (not at the repo root), no "../.." traversal
// is needed.
func testdataDir() string {
	return filepath.Join("testdata")
}

// readFixture reads a named fixture file from the testdata directory.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

// TestLoadPolicyFile verifies that valid policy files round-trip into typed rules.
func TestLoadPolicyFile(t *testing.T) {
	t.Run("valid_release_age", func(t *testing.T) {
		path := filepath.Join(testdataDir(), "valid_release_age.json")
		pf, errs := LoadPolicyFile(path)
		if len(errs) != 0 {
			t.Fatalf("LoadPolicyFile(%q): unexpected errors: %v", path, errs)
		}
		if pf.SchemaVersion != "1" {
			t.Errorf("SchemaVersion = %q, want %q", pf.SchemaVersion, "1")
		}
		if pf.Name != "test-release-age-policy" {
			t.Errorf("Name = %q, want %q", pf.Name, "test-release-age-policy")
		}
		if len(pf.Rules) != 1 {
			t.Fatalf("len(Rules) = %d, want 1", len(pf.Rules))
		}
		r := pf.Rules[0]
		if r.ID != "block-fresh-npm" {
			t.Errorf("Rule.ID = %q, want %q", r.ID, "block-fresh-npm")
		}
		if r.RuleType != "release_age" {
			t.Errorf("Rule.RuleType = %q, want %q", r.RuleType, "release_age")
		}
		if r.MinAgeHours != 48 {
			t.Errorf("Rule.MinAgeHours = %d, want 48", r.MinAgeHours)
		}
		if r.Action != "block" {
			t.Errorf("Rule.Action = %q, want %q", r.Action, "block")
		}
	})

	t.Run("valid_allowlist", func(t *testing.T) {
		path := filepath.Join(testdataDir(), "valid_allowlist.json")
		pf, errs := LoadPolicyFile(path)
		if len(errs) != 0 {
			t.Fatalf("LoadPolicyFile(%q): unexpected errors: %v", path, errs)
		}
		if len(pf.Rules) != 1 {
			t.Fatalf("len(Rules) = %d, want 1", len(pf.Rules))
		}
		r := pf.Rules[0]
		if r.RuleType != "package_allowlist" {
			t.Errorf("Rule.RuleType = %q, want %q", r.RuleType, "package_allowlist")
		}
		if r.Ecosystem != "npm" {
			t.Errorf("Rule.Ecosystem = %q, want %q", r.Ecosystem, "npm")
		}
		if len(r.Packages) != 2 {
			t.Errorf("len(Rule.Packages) = %d, want 2", len(r.Packages))
		}
	})

	t.Run("missing_file", func(t *testing.T) {
		_, errs := LoadPolicyFile(filepath.Join(testdataDir(), "does_not_exist.json"))
		if len(errs) == 0 {
			t.Fatal("LoadPolicyFile(missing): expected errors, got none")
		}
	})
}

// TestListPolicyFiles_MissingDir verifies that a missing policies/ directory
// yields an empty list and no error (Pitfall 3).
func TestListPolicyFiles_MissingDir(t *testing.T) {
	summaries, err := ListPolicyFiles(filepath.Join(testdataDir(), "nonexistent_dir"))
	if err != nil {
		t.Fatalf("ListPolicyFiles(missing dir): unexpected error: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("ListPolicyFiles(missing dir): expected empty list, got %d entries", len(summaries))
	}
}

// TestListPolicyFiles_ValidDir verifies that a directory with valid policy files
// returns correct summaries.
func TestListPolicyFiles_ValidDir(t *testing.T) {
	// The testdata directory contains both valid and invalid fixtures.
	// ListPolicyFiles silently skips invalid files.
	summaries, err := ListPolicyFiles(testdataDir())
	if err != nil {
		t.Fatalf("ListPolicyFiles(testdata): unexpected error: %v", err)
	}
	// Should find valid_release_age.json and valid_allowlist.json (2 valid fixtures).
	validCount := 0
	for _, s := range summaries {
		if s.RuleCount > 0 {
			validCount++
		}
	}
	if validCount < 2 {
		t.Errorf("expected at least 2 valid policy summaries, got %d (summaries: %v)", validCount, summaries)
	}
}
