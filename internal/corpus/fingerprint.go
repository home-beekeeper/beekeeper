package corpus

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// errInvalidSalt is returned by the fingerprint helpers when the salt is empty
// or not decodable hex. With the dedicated salt file (LoadOrCreateSalt) the salt
// is always a 64-char hex string, so this is a should-never-happen guard that
// turns a programming error into a loud, propagated failure instead of a silent
// weak-crypto downgrade (WR-04).
var errInvalidSalt = errors.New("corpus fingerprint: salt must be non-empty hex (programming error)")

// RepoFingerprint returns an HMAC-SHA256 hex fingerprint of repoPath keyed by
// salt.
//
// The fingerprint is NON-REVERSIBLE: a dictionary attack on repo paths requires
// knowledge of the per-install salt (T-23-02). Using bare SHA-256 without a
// salt would be reversible by an attacker who has the fingerprint and a
// dictionary of common repo paths. HMAC prevents this.
//
// salt must be a non-empty hex-encoded string (64 hex chars = 32 bytes).
// An empty or undecodable salt is a programming error: RepoFingerprint returns
// an error (WR-04) rather than silently keying the HMAC with attacker-guessable
// bytes. LoadOrCreateSalt guarantees a non-empty 64-char hex salt at corpus
// store init, so callers on the normal path never hit the error branch.
//
// Returns a 64-char lowercase hex string (full HMAC-SHA256 output).
func RepoFingerprint(repoPath, salt string) (string, error) {
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
// salt to reverse (T-23-02). An empty or undecodable salt returns an error
// (WR-04).
//
// Returns a 64-char lowercase hex string.
func FleetNodeID(hostname, goos, salt string) (string, error) {
	// NUL separator prevents prefix collision.
	message := hostname + "\x00" + goos
	return hmacHex([]byte(message), salt)
}

// hmacHex computes HMAC-SHA256(key=saltBytes, message=message) and returns the
// result as a lowercase hex string. salt MUST be a non-empty hex string; an
// empty or undecodable salt returns errInvalidSalt rather than falling back to a
// raw-string/empty HMAC key (WR-04). Keying HMAC with attacker-guessable bytes
// would make the fingerprint reversible (the exact T-23-02 threat this design
// exists to prevent), so we fail loudly instead.
func hmacHex(message []byte, salt string) (string, error) {
	saltBytes, err := hex.DecodeString(salt)
	if err != nil || len(saltBytes) == 0 {
		return "", errInvalidSalt
	}

	mac := hmac.New(sha256.New, saltBytes)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// saltFileName is the basename of the dedicated per-install corpus salt file,
// stored under StateDir()/corpus/ alongside the corpus NDJSON.
const saltFileName = "salt"

// corpusSaltPath returns the path to the dedicated salt file under stateDir.
func corpusSaltPath(stateDir string) string {
	return filepath.Join(stateDir, "corpus", saltFileName)
}

// LoadOrCreateSalt loads the per-install corpus salt from its OWN dedicated
// owner-only file at stateDir/corpus/salt. If the file is absent (first run), it
// generates 32 random bytes via crypto/rand, hex-encodes them, and creates the
// file atomically with O_CREATE|O_EXCL so the FIRST writer wins. Concurrent
// first-run creators that lose the O_EXCL race fall back to reading the
// now-existing file (first-writer-wins, never last-writer-wins rotation).
//
// CR-01 fix: the salt is no longer stored in the shared state.json, so the
// concurrent `beekeeper check` hot path NEVER read-modify-writes state.json for
// the salt. That eliminates both the salt-rotation race and the watch-daemon
// Degraded-clobber that the prior state.json round-trip caused.
//
// The salt is generated once per install and never per-record. Subsequent calls
// return the persisted value (idempotent).
//
// The returned salt is a 64-char lowercase hex string (32 bytes).
//
// stateDir is the resolved beekeeper state directory; the caller passes it so
// the corpus package remains testable with t.TempDir().
func LoadOrCreateSalt(stateDir string) (string, error) {
	path := corpusSaltPath(stateDir)

	// Fast path: salt file already exists — just read it.
	if salt, err := readSaltFile(path); err == nil {
		return salt, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	// First run: ensure the corpus directory exists (owner-only).
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create corpus directory for salt: %w", err)
	}

	// Generate 32 random bytes for a new per-install salt.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate corpus salt: %w", err)
	}
	salt := hex.EncodeToString(raw)

	// Write the candidate salt to a temp file in the same directory and fully
	// flush it BEFORE publishing, so the published salt file is never observed
	// mid-write. A prior version created the final file with O_CREATE|O_EXCL and
	// then wrote into it: O_EXCL elected one *creator*, but the file was empty
	// between create and write, so a concurrent loser that hit fs.ErrExist could
	// read it in that window and see 0 hex chars ("malformed salt" — a flaky
	// first-run race under -race). Publishing a fully-written temp file via
	// os.Link closes that window: Link fails with fs.ErrExist if the destination
	// already exists, preserving atomic FIRST-writer-wins (vs. rename's
	// last-writer-wins, which would rotate the salt), and the linked file always
	// has complete content.
	tmp, err := os.CreateTemp(dir, saltFileName+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp corpus salt file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // best-effort; harmless if already unlinked by Link

	if _, werr := tmp.WriteString(salt); werr != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write corpus salt: %w", werr)
	}
	if serr := tmp.Sync(); serr != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("sync corpus salt: %w", serr)
	}
	if cerr := tmp.Close(); cerr != nil {
		return "", fmt.Errorf("close temp corpus salt file: %w", cerr)
	}

	// Atomic first-writer-wins publish. If a concurrent creator already published
	// (fs.ErrExist), the winner's salt is authoritative and fully written.
	if lerr := os.Link(tmpName, path); lerr != nil {
		if errors.Is(lerr, fs.ErrExist) {
			existing, rerr := readSaltFile(path)
			if rerr != nil {
				return "", fmt.Errorf("read concurrently-created corpus salt: %w", rerr)
			}
			return existing, nil
		}
		return "", fmt.Errorf("publish corpus salt file: %w", lerr)
	}

	// Enforce owner-only permissions (Windows DACL; mirrors StoreSink/T-23-03).
	if err := platform.SetOwnerOnly(path); err != nil {
		return "", fmt.Errorf("enforce owner-only permissions on corpus salt: %w", err)
	}

	return salt, nil
}

// readSaltFile reads and validates the salt file at path. The salt must be a
// non-empty 64-char hex string; anything else (truncated/corrupt write) is
// rejected so a malformed salt never silently keys the HMAC. A missing file
// returns an error wrapping fs.ErrNotExist so the caller can detect first run.
func readSaltFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err // may wrap fs.ErrNotExist — caller checks
	}
	salt := strings.TrimSpace(string(data))
	if len(salt) != 64 {
		return "", fmt.Errorf("corpus salt file %q is malformed (expected 64 hex chars, got %d)", path, len(salt))
	}
	if _, err := hex.DecodeString(salt); err != nil {
		return "", fmt.Errorf("corpus salt file %q is not valid hex: %w", path, err)
	}
	return salt, nil
}
