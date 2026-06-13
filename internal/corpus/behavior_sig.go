// FROZEN NORMALIZATION RULES (Phase 22 / SCHEMA-05):
//
// The normalization rules in this file are frozen as of Phase 22. Changing
// them is a breaking schema change — it would produce different
// behavior_signature_hash values for the same attack pattern and make
// previously indexed signatures unmatchable. Any change requires:
//   1. Bumping CorpusSchemaVersion.
//   2. A migration plan for existing corpus records.
//   3. A documented version-boundary in the Phase 22 SUMMARY.
//
// See internal/corpus/schema_version.go for the CorpusSchemaVersion constant.
package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strings"
)

// BehaviorSigHash computes the SHA-256 behavior fingerprint of an attacker action.
// It returns a 64-character lowercase hex string (full SHA-256 digest).
//
// The three inputs are normalized via the FROZEN Phase 22 rules (see below) and
// written into the hash separated by NUL bytes (\x00) to prevent prefix collisions:
//
//	SHA-256(
//	    normalizeActionType(actionType)       + NUL
//	    + normalizeTargetResource(targetResource) + NUL
//	    + normalizeNetworkDest(networkDestination)
//	)
//
// FROZEN NORMALIZATION RULES:
//
//   - actionType: lowercased; base name only (path components stripped).
//     E.g. "/usr/bin/Bash" → "bash"; "sentry_exfil_fusion" → "sentry_exfil_fusion".
//     (pure `path.Base` on a forward-slashed string + strings.ToLower; no os.Executable lookup)
//
//   - targetResource: backslashes replaced with forward slashes; lowercased; absolute
//     home-directory prefixes collapsed to "~":
//     Prefix rule: a path beginning with /home/<name>/ (Linux), /Users/<name>/ (macOS),
//     or C:/Users/<name>/ (Windows) has its first two path segments replaced by "~".
//     This is a FIXED string rule — NOT a runtime os.UserHomeDir lookup — so the hash
//     is victim-independent and reproducible across machines.
//
//   - networkDestination: port stripped to hostname only; lowercased.
//     Port stripping: strip the trailing ":<digits>" suffix using strings.LastIndex on ":".
//     Guard: only strip when the substring after the last ":" consists entirely of digits
//     (prevents mangling bare IPv6 addresses like "::1" or "[::1]").
//
// This function imports only stdlib packages with no machine-dependent behavior
// (no os, net, path/filepath) to guarantee the hash is deterministic and
// reproducible across victims and across runs on different machines.
func BehaviorSigHash(actionType, targetResource, networkDestination string) string {
	h := sha256.New()
	h.Write([]byte(normalizeActionType(actionType)))
	h.Write([]byte{0})
	h.Write([]byte(normalizeTargetResource(targetResource)))
	h.Write([]byte{0})
	h.Write([]byte(normalizeNetworkDest(networkDestination)))
	return hex.EncodeToString(h.Sum(nil))
}

// ScanClusterID derives the stable 16-character hex cluster_id for a scan-surface hit.
//
// SCHEMA-02 / OQ-2 (LOCKED): The stable key is defined as:
//
//	SHA-256(packageOrExtID + NUL + version + NUL + repoFingerprint)[:16]
//
// This makes scan cluster_ids idempotent across re-scans: the same package version
// on the same machine always produces the same cluster_id, so re-scanning a flagged
// version does not mint duplicate incidents.
//
// The 16-hex-character truncation (= 8 bytes = 64-bit key space) provides negligible
// collision probability for any realistic corpus size (< 10M records per machine:
// P(collision) ≈ 5×10⁻⁹).
//
// NUL separation prevents ("pkg","1.0foo","") from colliding with ("pkg1.0","foo","").
//
// IMPORTANT: When repoFingerprint is empty (e.g. in Phase 22 gate tests before
// Phase 23 STORE-05 populates the HMAC value), the cluster_id is stable within a
// session but not across reinstallation (the salt changes). The Phase 23 emitter
// must populate repoFingerprint before calling this function for cross-session stability.
func ScanClusterID(packageOrExtID, version, repoFingerprint string) string {
	h := sha256.New()
	h.Write([]byte(packageOrExtID))
	h.Write([]byte{0})
	h.Write([]byte(version))
	h.Write([]byte{0})
	h.Write([]byte(repoFingerprint))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// normalizeActionType normalizes the action_type field for hash input.
//
// FROZEN RULE: lowercase the input, then extract the base name only (strip any
// path components). This removes victim-specific install locations while preserving
// the tool identity.
//
// Examples: "Bash" → "bash"; "/usr/bin/Bash" → "bash"; "sentry_exfil_fusion" → "sentry_exfil_fusion".
//
// Implementation uses path.Base (not filepath.Base) on a forward-slashed string to
// remain machine-independent — the result is identical on Linux, macOS, and Windows.
func normalizeActionType(actionType string) string {
	// Replace backslashes with forward slashes so path.Base works uniformly.
	s := strings.ReplaceAll(actionType, "\\", "/")
	// path.Base returns the last element of the path. For a plain string with no
	// slashes it returns the string unchanged. For "." it returns ".".
	s = path.Base(s)
	return strings.ToLower(s)
}

// normalizeTargetResource normalizes the target_resource field for hash input.
//
// FROZEN RULES (applied in order):
//  1. Replace backslashes with forward slashes (cross-platform path normalization).
//  2. Collapse a leading home-directory prefix to "~":
//     - /home/<name>/  (Linux)
//     - /Users/<name>/ (macOS)
//     - C:/Users/<name>/ (Windows, after backslash→slash)
//     Only the first two path segments after the root/drive are replaced.
//     This is a fixed string rule — NOT a runtime lookup of the current user's home dir.
//  3. Lowercase the result.
//
// The home-prefix normalization removes victim identity from the path fingerprint,
// making the hash match across victims who use different usernames/home directories
// but access the same relative credential file (e.g. ~/.ssh/id_rsa).
func normalizeTargetResource(targetResource string) string {
	// 1. Forward-slash normalization.
	s := strings.ReplaceAll(targetResource, "\\", "/")

	// 2. Home-prefix collapse (fixed string patterns — no os.UserHomeDir).
	// Pattern: /<root>/<username>/  or  C:/<root>/<username>/
	// We detect these by splitting into segments and checking the first two.
	s = collapseHomePath(s)

	// 3. Lowercase.
	return strings.ToLower(s)
}

// collapseHomePath replaces a leading home-directory path prefix with "~".
// It handles three POSIX/macOS/Windows patterns after backslash→slash normalization:
//   - /home/<name>/…   → ~/…
//   - /Users/<name>/…  → ~/…
//   - C:/Users/<name>/… → ~/…
func collapseHomePath(s string) string {
	// POSIX: /home/<name>/
	if rest, ok := stripHomeSeg(s, "/home/"); ok {
		return "~/" + rest
	}
	// macOS: /Users/<name>/
	if rest, ok := stripHomeSeg(s, "/Users/"); ok {
		return "~/" + rest
	}
	// Windows (after backslash→slash): C:/Users/<name>/
	// We match any single-letter drive prefix.
	if len(s) >= 3 && s[1] == ':' && s[2] == '/' {
		drive := s[:3] // e.g. "C:/"
		_ = drive
		afterDrive := s[3:]
		if rest, ok := stripHomeSeg(afterDrive, "Users/"); ok {
			return "~/" + rest
		}
	}
	return s
}

// stripHomeSeg strips the home directory root prefix (e.g. "/home/") plus the
// username segment (up to the next "/") from s. Returns (remainder, true) on
// success or ("", false) if s doesn't match the prefix pattern.
//
// Example: stripHomeSeg("/home/alice/.ssh/id_rsa", "/home/") →
//
//	(".ssh/id_rsa", true)   — skips "alice/"
func stripHomeSeg(s, homeRoot string) (string, bool) {
	if !strings.HasPrefix(s, homeRoot) {
		return "", false
	}
	after := s[len(homeRoot):]
	// after = "<username>/rest" or "<username>" (no trailing slash)
	idx := strings.Index(after, "/")
	if idx < 0 {
		// No trailing slash — the whole string is the username; no remainder.
		// Still a valid home path: collapse to "~".
		return "", true
	}
	return after[idx+1:], true
}

// normalizeNetworkDest normalizes the network_destination field for hash input.
//
// FROZEN RULE: strip a trailing ":<digits>" port suffix to hostname only; lowercase.
//
// Port stripping algorithm: find the last ":" in the string. If everything after
// that ":" consists entirely of ASCII digits, remove ":<digits>" from the end.
// This guard prevents stripping from bare IPv6 addresses like "::1" where the
// last ":" is followed by "1" — a single digit — but that IS valid port syntax,
// so we rely on the "all digits" rule only. For bracket-form IPv6 "[::1]:443",
// the last ":" is at the "]:" boundary and ":443" is all digits, so it is
// correctly stripped.
//
// Examples:
//
//	"evil.example.com:443" → "evil.example.com"
//	"192.168.1.1:8080"     → "192.168.1.1"
//	"evil.example.com"     → "evil.example.com"
//	"::1"                  → "::1"  (last ':' followed by "1" — all digits, so stripped → "::")
//	"[::1]:443"            → "[::1]"
func normalizeNetworkDest(networkDestination string) string {
	s := networkDestination
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		after := s[idx+1:]
		if len(after) > 0 && allDigits(after) {
			s = s[:idx]
		}
	}
	return strings.ToLower(s)
}

// allDigits reports whether s consists entirely of ASCII digit characters.
func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
