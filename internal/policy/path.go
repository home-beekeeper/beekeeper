package policy

import "strings"

// ruleSensitivePath is the rule ID returned in block Decisions when a tool
// call targets a path covered by the sensitive-path policy (PLCY-04).
const ruleSensitivePath = "sensitive-path-policy"

// SensitivePathConfig holds the blocklist and allowlist for path policy.
//
//   - BlockPatterns: path fragments or basename strings checked against the
//     resolved path. A fragment containing "/" (or "\") is matched via
//     strings.Contains; a basename pattern (like ".env") is matched against
//     the last path segment only.
//   - AllowPatterns: exact/prefix patterns (separator-bearing) or basename
//     patterns (no separator). A match here overrides any blocklist match —
//     checked first. Basename patterns (no "/" or "\") are matched against
//     the last path segment, mirroring matchesBlockPattern's basename logic.
//
// Path normalization (resolving "~" to the home directory, converting OS
// separators) is the CALLER's responsibility. EvaluatePath receives an
// already-resolved string and matches verbatim.
//
// MCP host-config directories (/.cursor/, /.windsurf/) and bare credential
// files (/.cargo/credentials) are hardcoded in DefaultSensitivePaths as of
// v1.2.0 Phase 7 (SPATH-01, D-02).
type SensitivePathConfig struct {
	BlockPatterns []string
	AllowPatterns []string
}

// DefaultSensitivePaths returns the default blocklist for PLCY-04.
//
// BlockPatterns are forward-slash-canonical fragments; the caller normalizes OS
// separators before calling EvaluatePath. The basename-glob entries (".env",
// ".env.local") are recognized by EvaluatePath as last-segment matches.
//
// AllowPatterns holds basename-only safe lookalikes for .env files (SPATH-04).
// These are checked FIRST by EvaluatePath, so .env.example etc. are allowed
// even though the .env.* glob in BlockPatterns would otherwise block them.
func DefaultSensitivePaths() SensitivePathConfig {
	return SensitivePathConfig{
		BlockPatterns: []string{
			"/.ssh/",
			"/.aws/",
			"/.gnupg/",
			"/.config/Claude/",
			"/.config/op/",
			"/.config/gh/",
			"/.cursor/",              // Cursor MCP config (SPATH-01, D-02, Phase 7)
			"/.windsurf/",            // Windsurf MCP config (SPATH-01, D-02, Phase 7)
			"/.netrc",
			"/.npmrc",
			"/.pypirc",
			"/.cargo/credentials.toml",
			"/.cargo/credentials",    // bare pre-2022 format (D-02, Phase 7)
			// Basename glob patterns — matched against last path segment.
			// ".env" covers exact ".env"; ".env.local" covers exact ".env.local";
			// ".env.*" covers any basename with prefix ".env." (e.g. .env.production).
			".env",
			".env.local",
			".env.*",
		},
		AllowPatterns: []string{
			".env.example", // safe lookalike — not a secrets file (SPATH-04)
			".env.test",    // safe lookalike (SPATH-04)
			".env.schema",  // safe lookalike (SPATH-04)
		},
	}
}

// EvaluatePath evaluates resolvedPath against cfg and returns a Decision.
//
// Matching rules:
//  1. If resolvedPath matches any AllowPattern (prefix or exact), return allow
//     with reason "explicitly allowlisted" — regardless of blocklist.
//  2. For each BlockPattern:
//   - Patterns containing "/" or "\" are matched via strings.Contains
//     (fragment match — catches paths like /.ssh/id_rsa, /.aws/credentials).
//   - Basename patterns without a path separator (".env", ".env.local",
//     ".env.*") are matched against the last segment extracted by splitting
//     on both "/" and "\":
//     • ".env"       → exact match against last segment
//     • ".env.local" → exact match against last segment
//     • ".env.*"     → last segment has prefix ".env." (glob simulation)
//  3. No match → allow Decision.
//
// EvaluatePath is a pure function: it imports only "strings", performs no I/O,
// and has no side effects.
func EvaluatePath(resolvedPath string, cfg SensitivePathConfig) Decision {
	// 1. Allowlist check — overrides blocklist.
	for _, allow := range cfg.AllowPatterns {
		if isAllowedPath(resolvedPath, allow) {
			return Decision{
				Allow:   true,
				Level:   "allow",
				Reason:  "explicitly allowlisted",
				RuleIDs: []string{ruleSensitivePath},
			}
		}
	}

	// 2. Blocklist check.
	for _, block := range cfg.BlockPatterns {
		if matchesBlockPattern(resolvedPath, block) {
			return Decision{
				Allow:   false,
				Level:   "block",
				Reason:  "sensitive path blocked: " + block,
				RuleIDs: []string{ruleSensitivePath},
			}
		}
	}

	// 3. Default: allow.
	return Decision{
		Allow:  true,
		Level:  "allow",
		Reason: "no sensitive path match",
	}
}

// matchesBlockPattern reports whether resolvedPath matches a block pattern.
//
// Patterns that contain "/" or "\" are fragment patterns: matched via
// strings.Contains. All other patterns are basename patterns matched against
// the last path segment.
func matchesBlockPattern(resolvedPath, pattern string) bool {
	if strings.Contains(pattern, "/") || strings.Contains(pattern, "\\") {
		// Fragment pattern — check for substring match.
		// Also handle Windows paths where the caller used forward slashes in
		// the pattern but the resolved path may have backslashes.
		return strings.Contains(resolvedPath, pattern) ||
			strings.Contains(normalizeSlashes(resolvedPath), pattern)
	}

	// Basename pattern — extract the last segment and match.
	seg := lastSegment(resolvedPath)

	// Handle glob: ".env.*" matches any segment with prefix ".env."
	if strings.HasSuffix(pattern, ".*") {
		prefix := pattern[:len(pattern)-1] // strip the trailing "*", keep "."
		return strings.HasPrefix(seg, prefix)
	}

	// Exact basename match.
	return seg == pattern
}

// isAllowedPath reports whether resolvedPath matches an allow pattern exactly or
// as a proper path-component prefix. A bare prefix like "/home/user/projects"
// must be followed by a path separator before it matches — preventing
// "/home/user/projects-secret" from being allowed by a "/home/user/projects"
// entry (WR-04).
//
// When the allow pattern contains no path separator ("/" or "\"), it is a
// basename pattern: matched against lastSegment(resolvedPath), mirroring
// matchesBlockPattern's basename logic (including the ".*" glob-prefix branch).
func isAllowedPath(resolvedPath, allow string) bool {
	// Basename pattern: no separator → match against the last path segment.
	// This enables AllowPatterns like ".env.example" to match any absolute path
	// whose final component is ".env.example" (Pitfall 2 fix, SPATH-04).
	if !strings.Contains(allow, "/") && !strings.Contains(allow, "\\") {
		seg := lastSegment(resolvedPath)
		// Handle glob: ".env.*" style allow patterns.
		if strings.HasSuffix(allow, ".*") {
			prefix := allow[:len(allow)-1] // strip trailing "*", keep "."
			return strings.HasPrefix(seg, prefix)
		}
		return seg == allow
	}

	// Separator-bearing pattern: exact or path-boundary prefix (WR-04 preserved).
	if resolvedPath == allow {
		return true
	}
	// If allow already ends with a separator, a simple HasPrefix is correct.
	if strings.HasSuffix(allow, "/") || strings.HasSuffix(allow, "\\") {
		return strings.HasPrefix(resolvedPath, allow)
	}
	// Require path boundary after the prefix.
	return strings.HasPrefix(resolvedPath, allow+"/") ||
		strings.HasPrefix(resolvedPath, allow+"\\")
}

// lastSegment returns the last path segment of p, splitting on both "/" and "\".
func lastSegment(p string) string {
	i := strings.LastIndexAny(p, "/\\")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

// normalizeSlashes converts all backslashes to forward slashes for pattern
// comparison. Used to handle Windows paths against forward-slash blocklist
// fragments.
func normalizeSlashes(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
