// Package check - posture allow-once store (IPOVR-01, Plan 29-02).
//
// A one-shot exception: the operator runs `beekeeper posture allow <pkg> --once`,
// which records a token here; the NEXT matching install at the pre-exec hook
// consumes that token (allow + remove) so the install proceeds once, and the
// install after that warns again. This is a convenience, NOT a security gate:
//
//   - The store is owner-only (0600 / owner-only DACL, mirroring the corpus salt
//     file and the audit log) so another local user cannot pre-seed exemptions.
//   - It is FAIL-OPEN on a READ error: a missing, empty, or corrupt store yields
//     "no token" (the install warns/blocks per the normal rules) rather than an
//     error that would break the build. A corrupt convenience store must never
//     turn into a blocked install.
//   - A successful CONSUME is a DURABLE atomic rewrite (temp file + rename), so a
//     consumed one-shot token is genuinely gone and cannot be replayed.
//
// SECURITY: allow-once allows the WHOLE posture evaluation for one install of a
// matching (ecosystem, package). It feeds ONLY the posture adapter and NEVER the
// catalog/corroboration path, so it can silence a posture warn but can never
// bypass a malware block - exactly like the scoped allow-always list. The adapter
// merges the catalog decision most-restrictive, so a catalog block still wins.
package check

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// allowOnceFileName is the basename of the owner-only allow-once store under the
// state directory (next to the catalogs/ and audit/ dirs).
const allowOnceFileName = "posture-allow-once.json"

// maxAllowOnceTokens bounds the store so a runaway or hostile writer cannot grow
// it without limit. AddAllowOnce trims oldest-first when the cap is exceeded.
const maxAllowOnceTokens = 256

// allowOnceToken is a single one-shot posture exemption. Ecosystem may be empty
// (matches any ecosystem for the package name). Reason is the operator-recorded
// justification; CreatedAt is an RFC3339 timestamp for forensic ordering and the
// oldest-first trim.
type allowOnceToken struct {
	Ecosystem string `json:"ecosystem,omitempty"`
	Package   string `json:"package"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// allowOncePath returns the path to the allow-once store under stateDir.
func allowOncePath(stateDir string) string {
	return filepath.Join(stateDir, allowOnceFileName)
}

// postureStateDir derives the state directory from the cacheDir the handler
// threads into evaluatePosture. In production cacheDir is <stateDir>/catalogs
// (platform.CatalogDir), so the parent is the state dir; the allow-once store
// lives there, next to catalogs/ and audit/. In tests cacheDir is a t.TempDir, so
// the store is created under that temp parent and never touches real user state.
func postureStateDir(cacheDir string) string {
	if cacheDir == "" {
		return ""
	}
	return filepath.Dir(cacheDir)
}

// consumeAllowOnceFn is the package-level seam (mirroring posturePublishAgeFn) so
// unit tests can drive the allow-once branch without a real on-disk store.
// Production assigns ConsumeAllowOnce.
var consumeAllowOnceFn = ConsumeAllowOnce

// readAllowOnce reads and parses the store at path. A missing file returns an
// empty slice with a nil error (first run / no tokens). A corrupt or unreadable
// file ALSO returns an empty slice with a nil error logged to stderr: the store
// is fail-open, so a malformed convenience store must never surface an error that
// could block an install.
func readAllowOnce(path string) []allowOnceToken {
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "beekeeper posture: allow-once store unreadable, treating as empty: %v\n", err)
		}
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	var tokens []allowOnceToken
	if err := json.Unmarshal([]byte(trimmed), &tokens); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper posture: allow-once store malformed, treating as empty: %v\n", err)
		return nil
	}
	return tokens
}

// writeAllowOnce atomically rewrites the store at path with tokens. It writes a
// sibling temp file (fsynced, owner-only) then renames over the target, so a
// concurrent reader can only ever observe the old complete file or the new
// complete file - never a truncated one (mirrors config.Save and the corpus salt
// publish). The parent directory is created owner-only if absent.
func writeAllowOnce(path string, tokens []allowOnceToken) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create allow-once dir: %w", err)
	}

	// Always marshal a non-nil slice so an emptied store serializes as "[]" rather
	// than "null" (a consumed-to-empty store stays a well-formed JSON array).
	if tokens == nil {
		tokens = []allowOnceToken{}
	}
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal allow-once tokens: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".allow-once-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp allow-once file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp allow-once file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp allow-once file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp allow-once file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp allow-once file over %q: %w", path, err)
	}
	// Enforce owner-only permissions (0600 on Unix; owner-only DACL on Windows),
	// mirroring the audit log and corpus salt file.
	if err := platform.SetOwnerOnly(path); err != nil {
		return fmt.Errorf("enforce owner-only permissions on allow-once store: %w", err)
	}
	return nil
}

// allowOnceMatches reports whether a stored token matches an install of pkg in
// ecosystem. The token's empty Ecosystem matches any ecosystem; a non-empty
// Ecosystem must match exactly. Package always matches exactly.
func allowOnceMatches(tok allowOnceToken, ecosystem, pkg string) bool {
	if tok.Package != pkg {
		return false
	}
	if tok.Ecosystem != "" && tok.Ecosystem != ecosystem {
		return false
	}
	return true
}

// AddAllowOnce records a one-shot allow token for an install of pkg in ecosystem
// with the operator-recorded reason. It is used by `beekeeper posture allow
// <pkg> --once`. The store is read, the token appended, oldest-first-trimmed to
// maxAllowOnceTokens, and atomically rewritten owner-only.
//
// An unreadable existing store is treated as empty (fail-open read) so a corrupt
// store never prevents recording a new token; the WRITE, by contrast, surfaces
// its error so the operator knows the token was not durably persisted.
func AddAllowOnce(stateDir, ecosystem, pkg, reason string) error {
	if pkg == "" {
		return errors.New("allow-once: package is required")
	}
	if stateDir == "" {
		return errors.New("allow-once: state dir is required")
	}
	path := allowOncePath(stateDir)
	tokens := readAllowOnce(path)
	tokens = append(tokens, allowOnceToken{
		Ecosystem: ecosystem,
		Package:   pkg,
		Reason:    reason,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if len(tokens) > maxAllowOnceTokens {
		tokens = tokens[len(tokens)-maxAllowOnceTokens:]
	}
	return writeAllowOnce(path, tokens)
}

// ConsumeAllowOnce returns true and durably removes the FIRST matching one-shot
// token for an install of pkg in ecosystem; it returns false when no token
// matches. A removed token is gone after an atomic rewrite, so the next matching
// install no longer finds it (the one-shot is consumed).
//
// Fail-open: an unreadable/corrupt store yields no match (false) so a convenience
// store never blocks an install. A rewrite error after a match is logged but the
// function still returns true - the operator's intent (allow this install once)
// is honored even if the durable removal failed; the worst case is the token is
// consumed again on a retry, which is strictly safer than blocking.
func ConsumeAllowOnce(stateDir, ecosystem, pkg string) bool {
	if pkg == "" || stateDir == "" {
		return false
	}
	path := allowOncePath(stateDir)
	tokens := readAllowOnce(path)
	if len(tokens) == 0 {
		return false
	}
	idx := -1
	for i, tok := range tokens {
		if allowOnceMatches(tok, ecosystem, pkg) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	// Remove the matched token (consume) and atomically rewrite.
	remaining := append(tokens[:idx:idx], tokens[idx+1:]...)
	if err := writeAllowOnce(path, remaining); err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper posture: allow-once consume could not be persisted: %v\n", err)
	}
	return true
}
