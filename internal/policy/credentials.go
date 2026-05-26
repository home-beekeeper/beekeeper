package policy

import "regexp"

// ruleCredentialOutput is the rule ID used when credential patterns are
// detected in tool output (PLCY-08).
const ruleCredentialOutput = "credential-output-filter"

// credPattern pairs a credential type name with its compiled regexp.
type credPattern struct {
	name string
	re   *regexp.Regexp
}

// credPatterns holds the built-in compiled regexps for credential detection.
// Compiled once at package init (package-level var), never per-call.
//
// Pattern design notes (PLCY-08):
//   - All patterns use character-class repeats with no nested quantifiers.
//     Go's RE2 engine is linear-time by construction — no catastrophic
//     backtracking is possible (T-02-02-04 accept disposition).
//   - aws-access-key: AKIA prefix + 16 uppercase alphanum chars (exact AWS format).
//   - jwt: three base64url segments separated by dots (minimal length enforced
//     by the {2,} quantifier on each segment).
//   - bearer: case-insensitive "Bearer" followed by whitespace + token chars.
//     The entire match (including "Bearer ") is replaced with [REDACTED:bearer].
//   - github-token: gh[pousr]_ prefix + ≥36 alphanumeric chars covers all
//     current GitHub token types (PAT, OAuth, user, server, refresh).
//   - npm-token: npm_ prefix + exactly 36 alphanumeric chars.
//   - openai-key: sk- prefix + ≥20 alphanumeric chars.
var credPatterns = []credPattern{
	{
		name: "aws-access-key",
		re:   regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	},
	{
		name: "jwt",
		re:   regexp.MustCompile(`eyJ[A-Za-z0-9_-]{2,}\.[A-Za-z0-9_-]{2,}\.[A-Za-z0-9_-]{2,}`),
	},
	{
		name: "bearer",
		re:   regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._-]+`),
	},
	{
		name: "github-token",
		re:   regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`),
	},
	{
		name: "npm-token",
		re:   regexp.MustCompile(`npm_[A-Za-z0-9]{36}`),
	},
	{
		name: "openai-key",
		re:   regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
	},
}

// CredentialFilterConfig carries optional runtime configuration for
// FilterCredentials.
//
// AdditionalPatterns is a slice of regular expression strings (not compiled
// regexps) provided by the caller. Each is compiled per-call — this is
// acceptable because the config set is small and caller-controlled; built-in
// patterns remain package-level.
type CredentialFilterConfig struct {
	// AdditionalPatterns is a list of regex strings (not compiled) for
	// site-specific credential formats. Matches are labeled "custom" in
	// DetectedTypes.
	AdditionalPatterns []string
}

// CredentialFilterResult is the output of FilterCredentials.
type CredentialFilterResult struct {
	// Redacted is a copy of the original output with all detected credential
	// patterns replaced by [REDACTED:<type>] placeholders.
	Redacted string

	// DetectedTypes lists the unique credential type names found (e.g.
	// "aws-access-key", "jwt"). Each type appears at most once regardless of
	// how many matches were found (deduplication). Order is unspecified.
	DetectedTypes []string

	// ContainsCredentials is true if at least one credential was detected.
	ContainsCredentials bool
}

// FilterCredentials scans output for known credential patterns and returns a
// CredentialFilterResult with a redacted copy of the output and the list of
// detected credential types.
//
// It is a pure function: it imports only "regexp" and "strings", performs no
// I/O, accesses no globals (except reading package-level compiled regexps),
// and has no side effects.
//
// Built-in pattern matching uses package-level compiled regexps (credPatterns).
// AdditionalPatterns in cfg are compiled per-call (acceptable for small sets).
func FilterCredentials(output string, cfg CredentialFilterConfig) CredentialFilterResult {
	redacted := output
	seenTypes := make(map[string]bool)

	// Apply built-in patterns.
	for _, cp := range credPatterns {
		replacement := "[REDACTED:" + cp.name + "]"
		replaced := cp.re.ReplaceAllString(redacted, replacement)
		if replaced != redacted {
			redacted = replaced
			seenTypes[cp.name] = true
		}
	}

	// Apply additional (caller-supplied) patterns.
	for _, rawPat := range cfg.AdditionalPatterns {
		re, err := regexp.Compile(rawPat)
		if err != nil {
			// Invalid pattern — skip silently; do not panic.
			continue
		}
		replacement := "[REDACTED:custom]"
		replaced := re.ReplaceAllString(redacted, replacement)
		if replaced != redacted {
			redacted = replaced
			seenTypes["custom"] = true
		}
	}

	// Build deduplicated DetectedTypes slice in a stable order (built-ins
	// first, then "custom").
	var detectedTypes []string
	for _, cp := range credPatterns {
		if seenTypes[cp.name] {
			detectedTypes = append(detectedTypes, cp.name)
		}
	}
	if seenTypes["custom"] {
		detectedTypes = append(detectedTypes, "custom")
	}

	// Ensure DetectedTypes is nil (not empty slice) when nothing was found,
	// matching the zero-value contract expected by callers.
	if len(detectedTypes) == 0 {
		detectedTypes = nil
	}

	return CredentialFilterResult{
		Redacted:            redacted,
		DetectedTypes:       detectedTypes,
		ContainsCredentials: len(detectedTypes) > 0,
	}
}
