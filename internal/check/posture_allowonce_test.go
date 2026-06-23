package check

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestAllowOnceAddThenConsumeThenGone proves the one-shot lifecycle: a recorded
// token is consumed (returns true) by the FIRST matching install and is gone for
// the SECOND (returns false) -- the token is consumed exactly once.
func TestAllowOnceAddThenConsumeThenGone(t *testing.T) {
	stateDir := t.TempDir()
	if err := AddAllowOnce(stateDir, "npm", "left-pad", "trying it"); err != nil {
		t.Fatalf("AddAllowOnce: %v", err)
	}
	if !ConsumeAllowOnce(stateDir, "npm", "left-pad") {
		t.Fatal("first ConsumeAllowOnce = false, want true (the token should be consumed once)")
	}
	if ConsumeAllowOnce(stateDir, "npm", "left-pad") {
		t.Fatal("second ConsumeAllowOnce = true, want false (the one-shot token was already consumed)")
	}
}

// TestAllowOnceEcosystemEmptyMatchesAny proves an entry with an empty ecosystem
// matches an install in any ecosystem (package name alone is the key).
func TestAllowOnceEcosystemEmptyMatchesAny(t *testing.T) {
	stateDir := t.TempDir()
	if err := AddAllowOnce(stateDir, "", "any-eco-pkg", ""); err != nil {
		t.Fatalf("AddAllowOnce: %v", err)
	}
	if !ConsumeAllowOnce(stateDir, "pypi", "any-eco-pkg") {
		t.Fatal("ConsumeAllowOnce = false, want true (empty-ecosystem token matches any ecosystem)")
	}
}

// TestAllowOnceNonMatchNotConsumed proves a different package is NOT consumed and
// the recorded token survives for its real match.
func TestAllowOnceNonMatchNotConsumed(t *testing.T) {
	stateDir := t.TempDir()
	if err := AddAllowOnce(stateDir, "npm", "wanted", ""); err != nil {
		t.Fatalf("AddAllowOnce: %v", err)
	}
	if ConsumeAllowOnce(stateDir, "npm", "other") {
		t.Fatal("ConsumeAllowOnce(other) = true, want false (a non-matching install must not consume the token)")
	}
	if !ConsumeAllowOnce(stateDir, "npm", "wanted") {
		t.Fatal("ConsumeAllowOnce(wanted) = false, want true (the token survives a non-matching install)")
	}
}

// TestAllowOnceCorruptStoreFailOpen proves the store is FAIL-OPEN on a read error:
// a corrupt store yields no match (false) rather than an error that could block.
func TestAllowOnceCorruptStoreFailOpen(t *testing.T) {
	stateDir := t.TempDir()
	path := allowOncePath(stateDir)
	if err := os.WriteFile(path, []byte("{ this is not valid json"), 0o600); err != nil {
		t.Fatalf("seed corrupt store: %v", err)
	}
	// A corrupt store must not panic or error; ConsumeAllowOnce returns false.
	if ConsumeAllowOnce(stateDir, "npm", "anything") {
		t.Fatal("ConsumeAllowOnce on a corrupt store = true, want false (fail-open: no token)")
	}
}

// TestAllowOnceMissingStoreNoMatch proves an absent store yields no match (the
// common first-run path: no tokens recorded yet).
func TestAllowOnceMissingStoreNoMatch(t *testing.T) {
	stateDir := t.TempDir()
	if ConsumeAllowOnce(stateDir, "npm", "anything") {
		t.Fatal("ConsumeAllowOnce on an absent store = true, want false")
	}
}

// TestAllowOnceStoreOwnerOnly proves the store is created owner-only (0600 on
// Unix). On Windows the bit check does not apply (owner-only is enforced via the
// DACL by platform.SetOwnerOnly), so the mode assertion is Unix-only.
func TestAllowOnceStoreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("owner-only is enforced via the Windows DACL, not the mode bits")
	}
	stateDir := t.TempDir()
	if err := AddAllowOnce(stateDir, "npm", "pkg", "r"); err != nil {
		t.Fatalf("AddAllowOnce: %v", err)
	}
	fi, err := os.Stat(allowOncePath(stateDir))
	if err != nil {
		t.Fatalf("stat allow-once store: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("allow-once store perm = %#o, want 0600 (owner-only)", perm)
	}
}

// TestPostureStateDirFromCacheDir proves postureStateDir derives the state dir as
// the parent of the catalogs cacheDir (production: <stateDir>/catalogs).
func TestPostureStateDirFromCacheDir(t *testing.T) {
	cacheDir := filepath.Join("some", "state", "catalogs")
	if got := postureStateDir(cacheDir); got != filepath.Join("some", "state") {
		t.Fatalf("postureStateDir(%q) = %q, want the parent dir", cacheDir, got)
	}
	if got := postureStateDir(""); got != "" {
		t.Fatalf("postureStateDir(\"\") = %q, want \"\"", got)
	}
}
