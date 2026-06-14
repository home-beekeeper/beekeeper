package corpus

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestFingerprintNonReversibility verifies STORE-05:
//
//   - Same repo path + same salt → same fingerprint (stable).
//   - Same repo path + different salt → different fingerprint (non-reversible:
//     two installs of the same repo produce different fingerprints).
//   - Same inputs to FleetNodeID are stable; different salts differ.
//   - An empty/invalid salt is rejected with an error (WR-04) rather than
//     silently downgrading to a raw-string/empty HMAC key.
func TestFingerprintNonReversibility(t *testing.T) {
	saltA := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	saltB := "99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa"
	repoPath := "/home/user/projects/myrepo"

	mustFP := func(fp string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("fingerprint returned error: %v", err)
		}
		return fp
	}

	t.Run("RepoFingerprint stable with same salt", func(t *testing.T) {
		fp1 := mustFP(RepoFingerprint(repoPath, saltA))
		fp2 := mustFP(RepoFingerprint(repoPath, saltA))
		if fp1 == "" {
			t.Fatal("RepoFingerprint returned empty string")
		}
		if fp1 != fp2 {
			t.Errorf("RepoFingerprint not stable: same inputs gave %q != %q", fp1, fp2)
		}
	})

	t.Run("RepoFingerprint differs with different salt (non-reversibility)", func(t *testing.T) {
		fpA := mustFP(RepoFingerprint(repoPath, saltA))
		fpB := mustFP(RepoFingerprint(repoPath, saltB))
		if fpA == fpB {
			t.Errorf("RepoFingerprint: same repo path under different salts should produce different fingerprints, got %q for both", fpA)
		}
	})

	t.Run("RepoFingerprint returns 64-char hex", func(t *testing.T) {
		fp := mustFP(RepoFingerprint(repoPath, saltA))
		if len(fp) != 64 {
			t.Errorf("RepoFingerprint: expected 64-char hex; got %d chars: %q", len(fp), fp)
		}
	})

	t.Run("RepoFingerprint rejects empty salt (WR-04)", func(t *testing.T) {
		if _, err := RepoFingerprint(repoPath, ""); err == nil {
			t.Error("RepoFingerprint: expected error on empty salt, got nil (silent weak-crypto downgrade)")
		}
	})

	t.Run("RepoFingerprint rejects non-hex salt (WR-04)", func(t *testing.T) {
		if _, err := RepoFingerprint(repoPath, "not-hex-salt-zzzz"); err == nil {
			t.Error("RepoFingerprint: expected error on non-hex salt, got nil")
		}
	})

	t.Run("FleetNodeID stable with same salt", func(t *testing.T) {
		id1 := mustFP(FleetNodeID("myhost", "linux", saltA))
		id2 := mustFP(FleetNodeID("myhost", "linux", saltA))
		if id1 == "" {
			t.Fatal("FleetNodeID returned empty string")
		}
		if id1 != id2 {
			t.Errorf("FleetNodeID not stable: same inputs gave %q != %q", id1, id2)
		}
	})

	t.Run("FleetNodeID differs with different salt", func(t *testing.T) {
		idA := mustFP(FleetNodeID("myhost", "linux", saltA))
		idB := mustFP(FleetNodeID("myhost", "linux", saltB))
		if idA == idB {
			t.Errorf("FleetNodeID: same hostname+goos under different salts should produce different values, got %q for both", idA)
		}
	})

	t.Run("FleetNodeID NUL-separated (hostname+goos no prefix collision)", func(t *testing.T) {
		// FleetNodeID("ab", "c", salt) must differ from FleetNodeID("a", "bc", salt)
		id1 := mustFP(FleetNodeID("ab", "c", saltA))
		id2 := mustFP(FleetNodeID("a", "bc", saltA))
		if id1 == id2 {
			t.Errorf("FleetNodeID prefix collision: NUL separation broken — (\"ab\",\"c\") == (\"a\",\"bc\")")
		}
	})

	t.Run("FleetNodeID rejects empty salt (WR-04)", func(t *testing.T) {
		if _, err := FleetNodeID("myhost", "linux", ""); err == nil {
			t.Error("FleetNodeID: expected error on empty salt, got nil")
		}
	})
}

// TestLoadOrCreateSalt verifies the per-install salt persistence in its own
// dedicated owner-only file (CR-01 / STORE-05):
//
//   - First call on a fresh state dir generates a non-empty 64-char hex salt.
//   - Second call returns the SAME salt (idempotent).
//   - The salt lives at stateDir/corpus/salt, NOT in state.json.
//   - state.json is never touched by the salt path.
func TestLoadOrCreateSalt(t *testing.T) {
	dir := t.TempDir()

	// First call: salt must be generated and persisted to its own file.
	salt1, err := LoadOrCreateSalt(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (first call): %v", err)
	}
	if salt1 == "" {
		t.Fatal("LoadOrCreateSalt returned empty salt on first call")
	}
	if len(salt1) != 64 {
		t.Errorf("LoadOrCreateSalt: expected 64-char hex salt; got %d chars: %q", len(salt1), salt1)
	}

	// The salt must live in the dedicated file, not state.json.
	saltPath := filepath.Join(dir, "corpus", "salt")
	if _, err := os.Stat(saltPath); err != nil {
		t.Errorf("salt file not created at %q: %v", saltPath, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); !os.IsNotExist(err) {
		t.Errorf("LoadOrCreateSalt must NOT create or touch state.json (CR-01); stat err=%v", err)
	}

	// Second call: must return the SAME salt (idempotent round-trip).
	salt2, err := LoadOrCreateSalt(dir)
	if err != nil {
		t.Fatalf("LoadOrCreateSalt (second call): %v", err)
	}
	if salt1 != salt2 {
		t.Errorf("LoadOrCreateSalt not idempotent: first=%q, second=%q", salt1, salt2)
	}
}

// TestLoadOrCreateSaltConcurrentFirstRun proves CR-01: N concurrent first-run
// creators converge on exactly ONE salt (first-writer-wins via O_CREATE|O_EXCL),
// never last-writer-wins rotation. state.json is never touched.
func TestLoadOrCreateSaltConcurrentFirstRun(t *testing.T) {
	dir := t.TempDir()

	const n = 32
	var wg sync.WaitGroup
	results := make([]string, n)
	errs := make([]error, n)
	start := make(chan struct{})

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // line everyone up to maximize the create race
			results[idx], errs[idx] = LoadOrCreateSalt(dir)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: LoadOrCreateSalt: %v", i, err)
		}
	}

	first := results[0]
	if len(first) != 64 {
		t.Fatalf("salt is not 64-char hex: %q", first)
	}
	for i, got := range results {
		if got != first {
			t.Errorf("salt divergence: goroutine %d got %q, want %q (first-writer-wins broken)", i, got, first)
		}
	}

	// state.json must remain untouched by the concurrent salt creation.
	if _, err := os.Stat(filepath.Join(dir, "state.json")); !os.IsNotExist(err) {
		t.Errorf("concurrent salt creation must NOT touch state.json (CR-01); stat err=%v", err)
	}
}
