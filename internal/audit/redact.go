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
//
// ToolInput is a map[string]any in policy.ToolCall (not in AuditRecord directly);
// string values would be redacted at the ToolCall layer if exposed here.
//
// RedactRecord always returns a new AuditRecord — it never mutates the receiver.
func RedactRecord(rec AuditRecord, patterns []redactPattern) AuditRecord {
	if len(patterns) == 0 {
		return rec
	}
	// Shallow copy — all fields are value types or immutable (slices are not
	// modified by redaction since we only redact string fields).
	out := rec
	out.Reason = applyRedaction(rec.Reason, patterns)
	// WR-01: the Phase-8 nudge fields carry attacker-influenced raw command
	// input. Apply the same credential redaction used for Reason so a forensic
	// log cannot become a credential-exfil target.
	out.OriginalCommand = applyRedaction(rec.OriginalCommand, patterns)
	out.RewrittenCommand = applyRedaction(rec.RewrittenCommand, patterns)
	out.PMState = applyRedaction(rec.PMState, patterns)
	return out
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

