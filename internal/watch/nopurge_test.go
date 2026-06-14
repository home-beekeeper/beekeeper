package watch_test

// TestCorpusPathHasNoPurgeCall is the FRB-02 STATIC GATE.
//
// It reads the source of internal/watch/firstresponder.go and
// internal/corpus/reader.go and asserts that neither file contains a call to
// quarantine.Purge (nor a bare .Purge() call) on any non-comment source line.
//
// This mirrors the project's grep-hygiene convention: comment lines (lines
// whose trimmed prefix is "//") are excluded from the check so a doc-comment
// that mentions "Purge" does not falsely trip the gate.
//
// Rationale (FRB-02): the corpus adjudication path from ReadMaliciousRecords
// through RunFirstResponder must only call quarantine.MoveTyped (reversible).
// quarantine.Purge is irreversible and must remain exclusively TUI-keyboard-
// gated (p key → y key confirmation). Any corpus-path Purge call would
// eliminate the restore path and violate the FRB-02 non-auto-purge invariant.
//
// The behavioral half of FRB-02 is TestFirstResponderCorpusNoPurge (which
// asserts that a quarantined artifact remains reversible after the corpus path
// runs). This static gate is the complementary compile-time/CI check.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCorpusPathHasNoPurgeCall asserts that the corpus code path source files
// contain no call to quarantine.Purge (or bare .Purge() on a non-comment line).
func TestCorpusPathHasNoPurgeCall(t *testing.T) {
	// Resolve the repo root from the test file's location.
	// runtime.Caller(0) returns the current file path at compile time.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot resolve repo root")
	}
	// thisFile is internal/watch/nopurge_test.go; walk up two directories.
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

	targetFiles := []string{
		filepath.Join(repoRoot, "internal", "watch", "firstresponder.go"),
		filepath.Join(repoRoot, "internal", "corpus", "reader.go"),
	}

	for _, srcPath := range targetFiles {
		t.Run(filepath.Base(filepath.Dir(srcPath))+"/"+filepath.Base(srcPath), func(t *testing.T) {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatalf("read source file %s: %v", srcPath, err)
			}

			lines := strings.Split(string(data), "\n")
			for lineNum, line := range lines {
				trimmed := strings.TrimSpace(line)
				// Skip blank lines and comment lines. Comment lines whose
				// trimmed prefix is "//" are documentation — a doc-comment
				// that mentions "Purge" must not trip this gate.
				if trimmed == "" || strings.HasPrefix(trimmed, "//") {
					continue
				}
				// Check for any Purge call on a non-comment source line.
				// We check for both the fully-qualified "quarantine.Purge(" and
				// the bare ".Purge(" to catch any indirect reference.
				if strings.Contains(line, "quarantine.Purge(") || strings.Contains(line, ".Purge(") {
					t.Errorf("FRB-02 violation: %s line %d contains a Purge call on the corpus code path (non-comment line):\n  %s",
						srcPath, lineNum+1, strings.TrimSpace(line))
				}
			}
		})
	}
}
