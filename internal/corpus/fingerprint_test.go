package corpus

import (
	"path/filepath"
	"testing"
)

// TestFingerprintNonReversibility verifies STORE-05:
//
//   - Same repo path + same salt → same fingerprint (stable).
//   - Same repo path + different salt → different fingerprint (non-reversible:
//     two installs of the same repo produce different fingerprints).
//   - Same inputs to FleetNodeID are stable; different salts differ.
func TestFingerprintNonReversibility(t *testing.T) {
	saltA := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	saltB := "99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa"
	repoPath := "/home/user/projects/myrepo"

	t.Run("RepoFingerprint stable with same salt", func(t *testing.T) {
		fp1 := RepoFingerprint(repoPath, saltA)
		fp2 := RepoFingerprint(repoPath, saltA)
		if fp1 == "" {
			t.Fatal("RepoFingerprint returned empty string")
		}
		if fp1 != fp2 {
			t.Errorf("RepoFingerprint not stable: same inputs gave %q != %q", fp1, fp2)
		}
	})

	t.Run("RepoFingerprint differs with different salt (non-reversibility)", func(t *testing.T) {
		fpA := RepoFingerprint(repoPath, saltA)
		fpB := RepoFingerprint(repoPath, saltB)
		if fpA == fpB {
			t.Errorf("RepoFingerprint: same repo path under different salts should produce different fingerprints, got %q for both", fpA)
		}
	})

	t.Run("RepoFingerprint returns 64-char hex", func(t *testing.T) {
		fp := RepoFingerprint(repoPath, saltA)
		if len(fp) != 64 {
			t.Errorf("RepoFingerprint: expected 64-char hex; got %d chars: %q", len(fp), fp)
		}
	})

	t.Run("FleetNodeID stable with same salt", func(t *testing.T) {
		id1 := FleetNodeID("myhost", "linux", saltA)
		id2 := FleetNodeID("myhost", "linux", saltA)
		if id1 == "" {
			t.Fatal("FleetNodeID returned empty string")
		}
		if id1 != id2 {
			t.Errorf("FleetNodeID not stable: same inputs gave %q != %q", id1, id2)
		}
	})

	t.Run("FleetNodeID differs with different salt", func(t *testing.T) {
		idA := FleetNodeID("myhost", "linux", saltA)
		idB := FleetNodeID("myhost", "linux", saltB)
		if idA == idB {
			t.Errorf("FleetNodeID: same hostname+goos under different salts should produce different values, got %q for both", idA)
		}
	})

	t.Run("FleetNodeID NUL-separated (hostname+goos no prefix collision)", func(t *testing.T) {
		// FleetNodeID("ab", "c", salt) must differ from FleetNodeID("a", "bc", salt)
		id1 := FleetNodeID("ab", "c", saltA)
		id2 := FleetNodeID("a", "bc", saltA)
		if id1 == id2 {
			t.Errorf("FleetNodeID prefix collision: NUL separation broken — (\"ab\",\"c\") == (\"a\",\"bc\")")
		}
	})
}

// TestLoadOrCreateSalt verifies the per-install salt persistence (OQ-3 / STORE-05):
//
//   - First call on a missing state file generates a non-empty hex salt.
//   - Second call on the same state file returns the SAME salt (idempotent).
func TestLoadOrCreateSalt(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	// First call: salt must be generated and persisted.
	salt1, err := LoadOrCreateSalt(stateFile)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (first call): %v", err)
	}
	if salt1 == "" {
		t.Fatal("LoadOrCreateSalt returned empty salt on first call")
	}
	if len(salt1) != 64 {
		t.Errorf("LoadOrCreateSalt: expected 64-char hex salt; got %d chars: %q", len(salt1), salt1)
	}

	// Second call: must return the SAME salt (idempotent round-trip).
	salt2, err := LoadOrCreateSalt(stateFile)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (second call): %v", err)
	}
	if salt1 != salt2 {
		t.Errorf("LoadOrCreateSalt not idempotent: first=%q, second=%q", salt1, salt2)
	}
}
