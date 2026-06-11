package coveragegate

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCoverageManifest is the VAL-01 gate: every production .go file under
// internal/ and cmd/ must be package-tested or carry a reason-coded allowlist
// entry. A single UNACCOUNTED file fails this test (and therefore `go test
// ./...` and CI). Parsing the allowlist itself fails closed — a tampered
// allowlist breaks the gate rather than silently lowering the bar.
func TestCoverageManifest(t *testing.T) {
	root, err := ModuleRoot(".")
	if err != nil {
		t.Fatalf("locating module root: %v", err)
	}

	al, err := ParseAllowlistFile(filepath.Join(root, "coverage-allowlist.txt"))
	if err != nil {
		t.Fatalf("coverage-allowlist.txt failed to parse (fail-closed): %v", err)
	}

	statuses, err := Walk(root, DefaultSubdirs, al)
	if err != nil {
		t.Fatalf("walking source tree: %v", err)
	}

	unaccounted := Unaccounted(statuses)
	if len(unaccounted) > 0 {
		t.Errorf("%d production file(s) are UNACCOUNTED — add a test to the package OR a reason-coded entry to coverage-allowlist.txt (valid codes: %v):", len(unaccounted), ValidReasonCodes())
		for _, p := range unaccounted {
			t.Errorf("  UNACCOUNTED: %s", p)
		}
	}
}

// TestCoverageManifestNegative proves the gate actually flags a test-less
// package: a synthetic production file in a package with no _test.go and no
// allowlist entry is reported UNACCOUNTED, and adding a reason-coded entry
// flips it to allowlisted.
func TestCoverageManifestNegative(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module synth\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(root, "internal", "synthpkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "foo.go"), []byte("package synthpkg\n\nfunc Foo() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No allowlist entry, no _test.go -> UNACCOUNTED.
	statuses, err := Walk(root, []string{"internal"}, &Allowlist{entries: map[string]string{}})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if got := Unaccounted(statuses); len(got) != 1 || got[0] != "internal/synthpkg/foo.go" {
		t.Fatalf("expected exactly [internal/synthpkg/foo.go] UNACCOUNTED, got %v", got)
	}

	// With a valid reason-coded allowlist entry -> allowlisted, not UNACCOUNTED.
	al := &Allowlist{entries: map[string]string{"internal/synthpkg/foo.go": "type-only"}}
	statuses, err = Walk(root, []string{"internal"}, al)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if got := Unaccounted(statuses); len(got) != 0 {
		t.Fatalf("expected zero UNACCOUNTED after allowlisting, got %v", got)
	}
	if statuses[0].Status != StatusAllowlisted || statuses[0].Reason != "type-only" {
		t.Fatalf("expected allowlisted/type-only, got %s/%s", statuses[0].Status, statuses[0].Reason)
	}
}

// TestCoverageManifestNegativePackageTested proves a sibling _test.go accounts
// the whole package (package-level linkage, not same-name-sibling).
func TestCoverageManifestNegativePackageTested(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module synth\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(root, "internal", "synthpkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A production file with NO same-name sibling test, plus an unrelated test
	// file in the same package. Package-level linkage must account foo.go.
	if err := os.WriteFile(filepath.Join(pkgDir, "foo.go"), []byte("package synthpkg\n\nfunc Foo() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "bar_test.go"), []byte("package synthpkg\n\nimport \"testing\"\n\nfunc TestBar(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statuses, err := Walk(root, []string{"internal"}, &Allowlist{entries: map[string]string{}})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if got := Unaccounted(statuses); len(got) != 0 {
		t.Fatalf("package-level linkage should account foo.go via bar_test.go, got UNACCOUNTED %v", got)
	}
	if statuses[0].Status != StatusTested {
		t.Fatalf("expected package-tested, got %s", statuses[0].Status)
	}
}
