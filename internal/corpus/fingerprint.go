package corpus

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/bantuson/beekeeper/internal/catalog"
)

// RepoFingerprint returns an HMAC-SHA256 hex fingerprint of repoPath keyed by
// salt.
//
// The fingerprint is NON-REVERSIBLE: a dictionary attack on repo paths requires
// knowledge of the per-install salt (T-23-02). Using bare SHA-256 without a
// salt would be reversible by an attacker who has the fingerprint and a
// dictionary of common repo paths. HMAC prevents this.
//
// salt must be a non-empty hex-encoded string (64 hex chars = 32 bytes).
// An empty salt is a programming error — it produces a stable but unsalted
// value and MUST NOT be used in production. LoadOrCreateSalt guarantees a
// non-empty salt at corpus store init.
//
// Returns a 64-char lowercase hex string (full HMAC-SHA256 output).
func RepoFingerprint(repoPath, salt string) string {
	return hmacHex([]byte(repoPath), salt)
}

// FleetNodeID returns an HMAC-SHA256 hex fingerprint of (hostname, goos) keyed
// by salt.
//
// The two inputs are concatenated with a NUL byte separator to prevent prefix
// collisions (consistent with the NUL-separation rule in behavior_sig.go):
// FleetNodeID("ab","c",salt) != FleetNodeID("a","bc",salt).
//
// Same non-reversibility guarantee as RepoFingerprint: requires the per-install
// salt to reverse (T-23-02).
//
// Returns a 64-char lowercase hex string.
func FleetNodeID(hostname, goos, salt string) string {
	// NUL separator prevents prefix collision.
	message := hostname + "\x00" + goos
	return hmacHex([]byte(message), salt)
}

// hmacHex computes HMAC-SHA256(key=saltBytes, message=message) and returns the
// result as a lowercase hex string. salt is decoded from hex; if the hex is
// invalid the key falls back to the raw salt bytes (still non-empty and keyed).
func hmacHex(message []byte, salt string) string {
	// Decode hex salt to raw bytes. If the salt is not valid hex (e.g. the raw
	// string passed directly), use the string bytes as the key — still keyed
	// and non-reversible, though callers should always pass hex-encoded salts.
	saltBytes, err := hex.DecodeString(salt)
	if err != nil || len(saltBytes) == 0 {
		// Fallback: use salt as raw bytes. This handles the case where the salt
		// is not hex-encoded (should not happen in production).
		saltBytes = []byte(salt)
	}

	mac := hmac.New(sha256.New, saltBytes)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}

// LoadOrCreateSalt loads the per-install corpus salt from stateFile. If the
// salt is absent (first run or migrated state.json), it generates 32 random
// bytes via crypto/rand, persists the hex-encoded salt under
// WatchState.CorpusLocalSalt in stateFile, and returns the new salt.
//
// The salt is generated once at process/sink init — never per-record. Subsequent
// calls return the persisted value (idempotent).
//
// The returned salt is a 64-char lowercase hex string (32 bytes).
//
// stateFile is the path to state.json; the caller passes the resolved path so
// the corpus package remains testable without touching the real StateDir.
func LoadOrCreateSalt(stateFile string) (string, error) {
	st, err := catalog.LoadState(stateFile)
	if err != nil {
		return "", fmt.Errorf("load state for corpus salt: %w", err)
	}

	if st.CorpusLocalSalt != "" {
		return st.CorpusLocalSalt, nil
	}

	// Generate 32 random bytes for a new per-install salt.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate corpus salt: %w", err)
	}
	salt := hex.EncodeToString(raw)

	st.CorpusLocalSalt = salt
	if err := catalog.SaveState(stateFile, st); err != nil {
		return "", fmt.Errorf("persist corpus salt to state: %w", err)
	}

	return salt, nil
}
