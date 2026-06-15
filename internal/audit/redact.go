// Package audit — redact.go implements sensitive field redaction for audit records.
//
// Redaction is applied before every audit record is written to disk (in writeAuditWithAC
// in internal/check/handler.go). The default patterns cover the three most common
// credential leak vectors observed in agent tool outputs (T-04-05-02):
//
//  1. Authorization: Bearer <token> — HTTP auth headers
//  2. eyJ...  — JWT tokens embedded in any string
//  3. sk-proj-/sk-ant-/AKIA/ghp_/glpat- prefixes — common API key namespaces
//
// All patterns are non-backtracking character classes (no nested quantifiers) to
// prevent catastrophic backtracking (T-04-05-07). Each pattern is pre-compiled once
// via defaultRedactPatterns().
//
// applyRedaction is a pure function — it returns a new string, never modifies the
// input. RedactRecord returns a copy of the AuditRecord with sensitive fields replaced.
package audit

import (
	"fmt"
	"regexp"
	"sync"
)

// redactPattern is a compiled regex + replacement pair used by applyRedaction.
type redactPattern struct {
	regex       *regexp.Regexp
	replacement string
}

// defaultPatternsOnce guards compilation of the default redact patterns.
// WR-05: compile regexps once at package initialisation rather than on every
// DefaultRedactPatterns() call, which is called from writeAuditWithAC on
// every hook invocation and from gateway writeAudit on every request.
var (
	defaultPatternsOnce sync.Once
	defaultPatternsVal  []redactPattern
)

// DefaultRedactPatterns returns the default set of sensitive-field redaction
// patterns. Each pattern uses non-backtracking character classes (no nested
// quantifiers) to prevent catastrophic backtracking (T-04-05-07).
//
// Patterns:
//  1. Bearer tokens: Authorization: Bearer <token> → Authorization: Bearer [REDACTED]
//  2. JWT tokens: eyJ<header>.<payload>.<sig> → [JWT_REDACTED]
//  3. Common API key prefixes: sk-proj/sk-ant/AKIA/ghp_/glpat- → prefix[REDACTED]
//
// WR-05: patterns are compiled once via sync.Once and reused on subsequent
// calls. This eliminates per-call regexp compilation overhead in the audit
// hot path (check handler + gateway handler both call this per request).
//
// This function is exported so callers (check.writeAuditWithAC, gateway.writeAudit)
// can apply redaction at the single chokepoint before writing to disk.
func DefaultRedactPatterns() []redactPattern {
	defaultPatternsOnce.Do(func() {
		defaultPatternsVal = []redactPattern{
			{
				// Bearer token in Authorization header (T-04-05-02).
				// Non-capturing; \S+ matches any non-whitespace (no nested quantifiers).
				regex:       regexp.MustCompile(`(?i)Authorization:\s*Bearer\s+\S+`),
				replacement: "Authorization: Bearer [REDACTED]",
			},
			{
				// JWT token: three base64url segments separated by dots.
				// Character class [A-Za-z0-9_-]+ prevents backtracking.
				regex:       regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
				replacement: "[JWT_REDACTED]",
			},
			{
				// Common API key prefixes (T-04-05-02).
				// Alternation with fixed prefixes; character class suffix prevents backtracking.
				// Uses a capturing group ($1) so the prefix is preserved in the replacement.
				regex:       regexp.MustCompile(`(sk-proj-|sk-ant-|AKIA|ghp_|glpat-)[A-Za-z0-9_-]+`),
				replacement: "${1}[REDACTED]",
			},
		}
	})
	return defaultPatternsVal
}

// RedactPatternsWith compiles the caller-supplied custom regex pattern strings
// and APPENDS them to DefaultRedactPatterns(), returning the combined set.
//
// This is the chokepoint that makes config.GetRedactPatterns() (the
// `redact_patterns` config key) actually take effect. Without it, custom
// patterns were plumbed through config but silently inert — every redaction
// site used the hardcoded defaults (the LOW-severity "silently inert" finding).
// Callers that have config-supplied patterns should compile them through this
// helper and pass the result to RedactRecord:
//
//	patterns, err := audit.RedactPatternsWith(cfg.GetRedactPatterns())
//	if err != nil { /* surface the config error; fall back to defaults */ }
//	rec = audit.RedactRecord(rec, patterns)
//
// Security invariant: custom patterns are ALWAYS appended to — never replace —
// the defaults. A defender (or an attacker who can influence config) therefore
// cannot weaken the built-in Bearer/JWT/API-key redaction by supplying a
// narrower or empty pattern list; they can only add additional redaction.
//
// Each custom pattern is anchored with a custom replacement of "[REDACTED]".
// A pattern that fails to compile aborts with a wrapped error (the first bad
// pattern is reported); the caller decides whether to fall back to the
// defaults. The returned defaults slice is the shared sync.Once-compiled value;
// the combined slice is freshly allocated and safe for the caller to retain.
//
// When custom is empty, RedactPatternsWith returns DefaultRedactPatterns()
// verbatim (no allocation) with a nil error.
func RedactPatternsWith(custom []string) ([]redactPattern, error) {
	defaults := DefaultRedactPatterns()
	if len(custom) == 0 {
		return defaults, nil
	}
	combined := make([]redactPattern, len(defaults), len(defaults)+len(custom))
	copy(combined, defaults)
	for _, expr := range custom {
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("audit: invalid redact_patterns entry %q: %w", expr, err)
		}
		combined = append(combined, redactPattern{regex: re, replacement: "[REDACTED]"})
	}
	return combined, nil
}

// applyRedaction applies each pattern to s in order and returns the result.
// It is a pure function: s is never modified. patterns may be nil or empty,
// in which case s is returned unchanged.
func applyRedaction(s string, patterns []redactPattern) string {
	if s == "" || len(patterns) == 0 {
		return s
	}
	result := s
	for _, p := range patterns {
		result = p.regex.ReplaceAllString(result, p.replacement)
	}
	return result
}

// RedactRecord returns a copy of rec with sensitive string values replaced by
// redaction placeholders. The following fields are redacted:
//
//   - Reason: may contain credential snippets from policy engine messages
//   - OriginalCommand: the verbatim agent-supplied Bash command (Phase 8 nudge,
//     WR-01) — may carry a token/secret embedded in an install argument such as
//     `npm install --registry=https://x:Bearer ...@host/`.
//   - RewrittenCommand: the nudge-rewritten command (Phase 8) — derived from the
//     original and may inherit the same secrets.
//   - PMState: the flattened §9 PM-state string (Phase 8). It is structured
//     detection metadata today, but it is redacted defensively so that no
//     attacker-influenced data path bypasses redaction (WR-01).
//   - SentryProcessExe: the path of the monitored process. May embed credential
//     paths or tokens in argv-derived exe strings on some platforms (TM-D-03).
//   - SentryCorrelatedExt: the extension ID correlated with a sentry alert. May
//     contain publisher-supplied strings that embed tokens (TM-D-03).
//   - SentryFilesAccessed: slice of file paths observed by the Sentry — may
//     contain credential paths or secrets embedded in paths (TM-D-03).
//   - SentryNetworkDests: slice of network destinations — may contain bearer
//     tokens embedded in URL query strings or authority components (TM-D-03).
//   - CatalogProvenance: struct slice — Reason and EntryID fields within each
//     match may carry attacker-controlled strings from the catalog (TM-D-03).
//
// Non-sensitive structural fields (RecordType, Decision, Timestamp, RuleIDs,
// boolean flags, numeric counters) are NOT redacted.
//
// ToolInput is a map[string]any in policy.ToolCall (not in AuditRecord directly);
// string values would be redacted at the ToolCall layer if exposed here.
//
// RedactRecord always returns a new AuditRecord — it never mutates the receiver.
func RedactRecord(rec AuditRecord, patterns []redactPattern) AuditRecord {
	if len(patterns) == 0 {
		return rec
	}
	// Shallow copy — all value-type fields are copied verbatim. Slice fields
	// that need redaction are replaced with freshly allocated slices below.
	out := rec
	out.Reason = applyRedaction(rec.Reason, patterns)
	// WR-01: the Phase-8 nudge fields carry attacker-influenced raw command
	// input. Apply the same credential redaction used for Reason so a forensic
	// log cannot become a credential-exfil target.
	out.OriginalCommand = applyRedaction(rec.OriginalCommand, patterns)
	out.RewrittenCommand = applyRedaction(rec.RewrittenCommand, patterns)
	out.PMState = applyRedaction(rec.PMState, patterns)

	// TM-D-03: redact Sentry string fields that may carry credential-adjacent
	// data (process paths, network destinations, correlated extension IDs).
	out.SentryProcessExe = applyRedaction(rec.SentryProcessExe, patterns)
	out.SentryCorrelatedExt = applyRedaction(rec.SentryCorrelatedExt, patterns)
	out.SentryFilesAccessed = RedactStringSlice(rec.SentryFilesAccessed, patterns)
	out.SentryNetworkDests = RedactStringSlice(rec.SentryNetworkDests, patterns)

	// TM-D-03: redact CatalogProvenance fields that carry attacker-influenced
	// strings (Package, EntryID). Structural/provenance fields (CatalogSource,
	// Ecosystem, Severity, Version, boolean flags) are not redacted.
	if len(rec.CatalogMatches) > 0 {
		redactedMatches := make([]CatalogProvenance, len(rec.CatalogMatches))
		for i, cm := range rec.CatalogMatches {
			redactedMatches[i] = cm
			redactedMatches[i].Package = applyRedaction(cm.Package, patterns)
			redactedMatches[i].EntryID = applyRedaction(cm.EntryID, patterns)
		}
		out.CatalogMatches = redactedMatches
	}

	return out
}

// RedactRecordWithDefaults returns a copy of rec with all default sensitive
// patterns applied. This is the corpus store's cross-package redaction
// entrypoint (Phase 23 prerequisite — research Finding 7 / T-22-02).
//
// The unexported redactPattern type used by RedactRecord and DefaultRedactPatterns
// cannot cross package boundaries, so internal/corpus cannot call
// RedactRecord(rec, DefaultRedactPatterns()) directly. This wrapper exposes a
// cross-package-safe signature that takes only AuditRecord and returns
// AuditRecord, with no unexported types in the signature.
//
// RedactRecordWithDefaults never mutates the input rec — it delegates to
// RedactRecord which always returns a new AuditRecord (copy semantics).
func RedactRecordWithDefaults(rec AuditRecord) AuditRecord {
	return RedactRecord(rec, DefaultRedactPatterns())
}

// RedactString is a convenience helper for single-string redaction using the
// default patterns. It is used in tests and for ad-hoc redaction outside of
// audit records.
func RedactString(s string) string {
	return applyRedaction(s, DefaultRedactPatterns())
}

// HasSensitiveData reports whether s matches any default redaction pattern.
// It is used in tests to verify that redaction is applied correctly.
func HasSensitiveData(s string) bool {
	for _, p := range DefaultRedactPatterns() {
		if p.regex.MatchString(s) {
			return true
		}
	}
	return false
}

// RedactStringSlice applies redaction to each element of ss and returns a new slice.
// Elements that do not match any pattern are returned unchanged.
func RedactStringSlice(ss []string, patterns []redactPattern) []string {
	if len(ss) == 0 || len(patterns) == 0 {
		return ss
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = applyRedaction(s, patterns)
	}
	return out
}
