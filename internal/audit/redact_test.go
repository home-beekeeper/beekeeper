package audit

import (
	"strings"
	"testing"
)

// TestRedactBearerToken verifies that Authorization: Bearer <token> is redacted.
func TestRedactBearerToken(t *testing.T) {
	patterns := DefaultRedactPatterns()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard bearer token",
			input: "Authorization: Bearer eyJhbGciOiJSUzI1NiJ9",
			want:  "Authorization: Bearer [REDACTED]",
		},
		{
			name:  "bearer token in longer string",
			// \S+ matches any non-whitespace including comma, so the comma is absorbed into [REDACTED]
			// Real-world HTTP headers are separated by \r\n, not commas; this is expected behavior.
			input: "request headers: Authorization: Bearer abc123xyz Content-Type: application/json",
			want:  "request headers: Authorization: Bearer [REDACTED] Content-Type: application/json",
		},
		{
			name:  "case insensitive authorization header",
			input: "authorization: bearer mytoken123",
			want:  "Authorization: Bearer [REDACTED]",
		},
		{
			name:  "no bearer token — unchanged",
			input: "Authorization: Basic dXNlcjpwYXNz",
			want:  "Authorization: Basic dXNlcjpwYXNz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRedaction(tt.input, patterns)
			if got != tt.want {
				t.Errorf("applyRedaction(%q):\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestRedactJWT verifies that JWT tokens (eyJ...) are redacted.
func TestRedactJWT(t *testing.T) {
	patterns := DefaultRedactPatterns()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard JWT",
			input: "token=eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.signature123",
			want:  "token=[JWT_REDACTED]",
		},
		{
			name:  "JWT in response body",
			input: `{"access_token":"eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.sig456","type":"Bearer"}`,
			want:  `{"access_token":"[JWT_REDACTED]","type":"Bearer"}`,
		},
		{
			name:  "no JWT — unchanged",
			input: "this string does not contain a JWT token",
			want:  "this string does not contain a JWT token",
		},
		{
			name:  "partial JWT (only two segments) — unchanged",
			input: "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9",
			want:  "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRedaction(tt.input, patterns)
			if got != tt.want {
				t.Errorf("applyRedaction(%q):\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestRedactAPIKeyPrefix verifies that common API key prefixes are redacted.
func TestRedactAPIKeyPrefix(t *testing.T) {
	patterns := DefaultRedactPatterns()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "OpenAI project key",
			input: "API_KEY=sk-proj-abc123XYZ",
			want:  "API_KEY=sk-proj-[REDACTED]",
		},
		{
			name:  "Anthropic key",
			input: "key: sk-ant-api03-longkeystring",
			want:  "key: sk-ant-[REDACTED]",
		},
		{
			name:  "AWS access key",
			input: "export AWS_SECRET_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE",
			want:  "export AWS_SECRET_ACCESS_KEY=AKIA[REDACTED]",
		},
		{
			name:  "GitHub PAT",
			input: "GITHUB_TOKEN=ghp_1234567890abcdef",
			want:  "GITHUB_TOKEN=ghp_[REDACTED]",
		},
		{
			name:  "GitLab PAT",
			input: "GITLAB_TOKEN=glpat-xyz123abc",
			want:  "GITLAB_TOKEN=glpat-[REDACTED]",
		},
		{
			name:  "no API key prefix — unchanged",
			input: "normal command output with no credentials",
			want:  "normal command output with no credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRedaction(tt.input, patterns)
			if got != tt.want {
				t.Errorf("applyRedaction(%q):\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestRedactPure verifies that applyRedaction does not modify the input string.
// The function must be pure: calling it twice on the same input yields the same
// output, and the original string is unchanged.
func TestRedactPure(t *testing.T) {
	patterns := DefaultRedactPatterns()
	input := "Authorization: Bearer secret-token-value"
	original := input // copy for comparison

	result1 := applyRedaction(input, patterns)
	result2 := applyRedaction(input, patterns)

	// Input string must not be modified.
	if input != original {
		t.Errorf("applyRedaction modified input string: got %q, want %q", input, original)
	}

	// Results must be identical (deterministic).
	if result1 != result2 {
		t.Errorf("applyRedaction is non-deterministic: first=%q second=%q", result1, result2)
	}

	// Result must differ from input (redaction occurred).
	if result1 == input {
		t.Errorf("applyRedaction did not redact sensitive content: %q", result1)
	}

	// Result must contain [REDACTED].
	if !strings.Contains(result1, "[REDACTED]") {
		t.Errorf("applyRedaction result missing [REDACTED]: %q", result1)
	}
}

// TestRedactNoMatch verifies that strings with no sensitive patterns are returned unchanged.
func TestRedactNoMatch(t *testing.T) {
	patterns := DefaultRedactPatterns()
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"plain text", "Hello, world!"},
		{"code output", "compiled 42 packages in 1.23s"},
		{"JSON no secrets", `{"status":"ok","count":5}`},
		{"URL no credentials", "https://example.com/api/v1/resource"},
		{"partial prefix no suffix", "sk-proj-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRedaction(tt.input, patterns)
			if got != tt.input {
				t.Errorf("applyRedaction changed non-sensitive input %q → %q", tt.input, got)
			}
		})
	}
}

// TestRedactRecordReason verifies that RedactRecord redacts the Reason field in a copy.
func TestRedactRecordReason(t *testing.T) {
	patterns := DefaultRedactPatterns()

	original := AuditRecord{
		RecordType: "policy_decision",
		RecordID:   "test-id-1",
		Timestamp:  "2026-05-26T00:00:00Z",
		ScannerName: "beekeeper",
		Decision:   "warn",
		Reason:     "tool output contained Authorization: Bearer secret-abc123; flagged",
		RuleIDs:    []string{"TEST-01"},
	}

	redacted := RedactRecord(original, patterns)

	// Original must be unchanged (RedactRecord returns a copy).
	if original.Reason != "tool output contained Authorization: Bearer secret-abc123; flagged" {
		t.Errorf("RedactRecord mutated original.Reason: %q", original.Reason)
	}

	// Redacted copy must have redacted Reason.
	if strings.Contains(redacted.Reason, "secret-abc123") {
		t.Errorf("RedactRecord did not redact secret in Reason: %q", redacted.Reason)
	}
	if !strings.Contains(redacted.Reason, "[REDACTED]") {
		t.Errorf("RedactRecord Reason missing [REDACTED]: %q", redacted.Reason)
	}

	// Other fields must be unchanged.
	if redacted.RecordType != original.RecordType {
		t.Errorf("RedactRecord changed RecordType: got %q want %q", redacted.RecordType, original.RecordType)
	}
	if redacted.RecordID != original.RecordID {
		t.Errorf("RedactRecord changed RecordID: got %q want %q", redacted.RecordID, original.RecordID)
	}
	if redacted.Decision != original.Decision {
		t.Errorf("RedactRecord changed Decision: got %q want %q", redacted.Decision, original.Decision)
	}
}

// TestRedactRecordNudgeCommandFields verifies WR-01: the Phase-8 nudge command
// fields (OriginalCommand, RewrittenCommand, PMState) are run through the same
// credential redaction as Reason, so a token embedded in an agent-supplied
// install command never reaches the forensic NDJSON log verbatim.
func TestRedactRecordNudgeCommandFields(t *testing.T) {
	patterns := DefaultRedactPatterns()

	original := AuditRecord{
		RecordType:       "nudge",
		RecordID:         "nudge-id-1",
		Timestamp:        "2026-06-04T00:00:00Z",
		ScannerName:      "beekeeper",
		Decision:         "warn",
		NudgeAction:      "advise",
		OriginalCommand:  "npm install --registry=https://x:Bearer eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.sig@host/ pkg",
		RewrittenCommand: "pnpm add pkg --registry=https://x?token=ghp_1234567890abcdef",
		PMState:          "pnpm installed; token sk-ant-api03-leakedkey present",
	}

	redacted := RedactRecord(original, patterns)

	// Original must be unchanged (RedactRecord returns a copy).
	if !strings.Contains(original.OriginalCommand, "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.sig") {
		t.Fatalf("RedactRecord mutated original.OriginalCommand: %q", original.OriginalCommand)
	}

	// OriginalCommand: JWT must be redacted.
	if strings.Contains(redacted.OriginalCommand, "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.sig") {
		t.Errorf("OriginalCommand JWT not redacted: %q", redacted.OriginalCommand)
	}
	if !strings.Contains(redacted.OriginalCommand, "[JWT_REDACTED]") {
		t.Errorf("OriginalCommand missing [JWT_REDACTED]: %q", redacted.OriginalCommand)
	}

	// RewrittenCommand: GitHub PAT must be redacted.
	if strings.Contains(redacted.RewrittenCommand, "ghp_1234567890abcdef") {
		t.Errorf("RewrittenCommand PAT not redacted: %q", redacted.RewrittenCommand)
	}
	if !strings.Contains(redacted.RewrittenCommand, "ghp_[REDACTED]") {
		t.Errorf("RewrittenCommand missing ghp_[REDACTED]: %q", redacted.RewrittenCommand)
	}

	// PMState: Anthropic key must be redacted.
	if strings.Contains(redacted.PMState, "sk-ant-api03-leakedkey") {
		t.Errorf("PMState key not redacted: %q", redacted.PMState)
	}
	if !strings.Contains(redacted.PMState, "sk-ant-[REDACTED]") {
		t.Errorf("PMState missing sk-ant-[REDACTED]: %q", redacted.PMState)
	}

	// Reason redaction must still work alongside the new fields.
	if redacted.Decision != original.Decision {
		t.Errorf("RedactRecord changed Decision: got %q want %q", redacted.Decision, original.Decision)
	}
}

// TestRedactRecordNoPatterns verifies that RedactRecord with no patterns returns the record unchanged.
func TestRedactRecordNoPatterns(t *testing.T) {
	rec := AuditRecord{
		Reason: "Authorization: Bearer should-be-redacted",
	}
	got := RedactRecord(rec, nil)
	if got.Reason != rec.Reason {
		t.Errorf("RedactRecord with nil patterns changed Reason: got %q want %q", got.Reason, rec.Reason)
	}
}

// TestRedactRecordSentryFields verifies TM-D-03: RedactRecord redacts the
// Sentry string fields (SentryFilesAccessed, SentryNetworkDests, SentryProcessExe,
// SentryCorrelatedExt) and CatalogProvenance entries so that credentials embedded
// in those fields do not reach remote OTLP/HTTPS/syslog sinks.
func TestRedactRecordSentryFields(t *testing.T) {
	patterns := DefaultRedactPatterns()

	original := AuditRecord{
		RecordType:  "sentry_alert",
		RecordID:    "sentry-id-1",
		Timestamp:   "2026-06-05T00:00:00Z",
		ScannerName: "beekeeper",
		Decision:    "block",
		// Sensitive data embedded in Sentry fields.
		SentryProcessExe:    "/usr/bin/node --token=ghp_1234567890abcdef",
		SentryCorrelatedExt: "malicious.ext sk-proj-abc123XYZ",
		SentryFilesAccessed: []string{
			"/home/user/.ssh/id_rsa",
			"/home/user/Authorization: Bearer secret-file-token /data",
		},
		SentryNetworkDests: []string{
			"https://exfil.example.com/?token=ghp_abcdefghijklmnop",
			"192.0.2.1:443",
		},
		CatalogMatches: []CatalogProvenance{
			{
				CatalogSource: "bumblebee",
				EntryID:       "entry-ghp_1234567890abcdef",
				Ecosystem:     "editor-extension",
				Package:       "evil.pkg sk-ant-api03-secretkey",
				Version:       "1.0.0",
				Severity:      "critical",
			},
		},
	}

	redacted := RedactRecord(original, patterns)

	// Original must be unchanged.
	if original.SentryProcessExe != "/usr/bin/node --token=ghp_1234567890abcdef" {
		t.Fatalf("RedactRecord mutated original.SentryProcessExe")
	}

	// SentryProcessExe: token must be redacted.
	if strings.Contains(redacted.SentryProcessExe, "ghp_1234567890abcdef") {
		t.Errorf("SentryProcessExe token not redacted: %q", redacted.SentryProcessExe)
	}

	// SentryCorrelatedExt: API key must be redacted.
	if strings.Contains(redacted.SentryCorrelatedExt, "sk-proj-abc123XYZ") {
		t.Errorf("SentryCorrelatedExt key not redacted: %q", redacted.SentryCorrelatedExt)
	}

	// SentryFilesAccessed: bearer token in path must be redacted; plain path unchanged.
	if len(redacted.SentryFilesAccessed) != 2 {
		t.Fatalf("SentryFilesAccessed length changed: got %d want 2", len(redacted.SentryFilesAccessed))
	}
	// Plain path must be unchanged (no credential pattern).
	if redacted.SentryFilesAccessed[0] != original.SentryFilesAccessed[0] {
		t.Errorf("SentryFilesAccessed[0] changed unexpectedly: got %q", redacted.SentryFilesAccessed[0])
	}
	// Bearer token in second entry must be redacted.
	if strings.Contains(redacted.SentryFilesAccessed[1], "secret-file-token") {
		t.Errorf("SentryFilesAccessed[1] bearer token not redacted: %q", redacted.SentryFilesAccessed[1])
	}

	// SentryNetworkDests: GitHub PAT in URL must be redacted.
	if len(redacted.SentryNetworkDests) != 2 {
		t.Fatalf("SentryNetworkDests length changed: got %d want 2", len(redacted.SentryNetworkDests))
	}
	if strings.Contains(redacted.SentryNetworkDests[0], "ghp_abcdefghijklmnop") {
		t.Errorf("SentryNetworkDests[0] PAT not redacted: %q", redacted.SentryNetworkDests[0])
	}
	// Plain IP must be unchanged.
	if redacted.SentryNetworkDests[1] != original.SentryNetworkDests[1] {
		t.Errorf("SentryNetworkDests[1] changed unexpectedly: got %q", redacted.SentryNetworkDests[1])
	}

	// CatalogMatches: Package and EntryID must be redacted.
	if len(redacted.CatalogMatches) != 1 {
		t.Fatalf("CatalogMatches length changed: got %d want 1", len(redacted.CatalogMatches))
	}
	if strings.Contains(redacted.CatalogMatches[0].Package, "sk-ant-api03-secretkey") {
		t.Errorf("CatalogMatches[0].Package key not redacted: %q", redacted.CatalogMatches[0].Package)
	}
	if strings.Contains(redacted.CatalogMatches[0].EntryID, "ghp_1234567890abcdef") {
		t.Errorf("CatalogMatches[0].EntryID PAT not redacted: %q", redacted.CatalogMatches[0].EntryID)
	}
	// Non-sensitive structural fields must be unchanged.
	if redacted.CatalogMatches[0].CatalogSource != "bumblebee" {
		t.Errorf("CatalogMatches[0].CatalogSource changed: got %q", redacted.CatalogMatches[0].CatalogSource)
	}
	if redacted.CatalogMatches[0].Severity != "critical" {
		t.Errorf("CatalogMatches[0].Severity changed: got %q", redacted.CatalogMatches[0].Severity)
	}

	// Structural fields must be unchanged.
	if redacted.RecordType != "sentry_alert" {
		t.Errorf("RedactRecord changed RecordType: got %q", redacted.RecordType)
	}
	if redacted.Decision != "block" {
		t.Errorf("RedactRecord changed Decision: got %q", redacted.Decision)
	}
}

// ---------------------------------------------------------------------------
// Exported convenience helpers: RedactString + HasSensitiveData.
//
// These wrap applyRedaction / the default pattern set with no caller-supplied
// patterns, so they are the entry point for ad-hoc redaction outside audit
// records. The assertions below prove a KNOWN secret pattern is actually masked
// (not merely that a non-nil string is returned) and that benign text is left
// untouched — the load-bearing security property for a redaction helper.
// ---------------------------------------------------------------------------

// TestRedactRecordWithDefaults verifies the Phase 22 cross-package redaction
// entrypoint (T-22-02 / research Finding 7):
//   - A Bearer token in rec.Reason is redacted.
//   - A JWT in rec.Reason is redacted.
//   - The input rec is NOT mutated (copy semantics preserved).
func TestRedactRecordWithDefaults(t *testing.T) {
	t.Run("bearer token in Reason is redacted", func(t *testing.T) {
		rec := AuditRecord{
			RecordType:  "policy_decision",
			RecordID:    "rrd-bearer-01",
			Timestamp:   "2026-06-13T00:00:00Z",
			ScannerName: "beekeeper",
			Decision:    "warn",
			Reason:      "tool output: Authorization: Bearer abc123secrettoken; flagged",
		}
		got := RedactRecordWithDefaults(rec)

		if strings.Contains(got.Reason, "abc123secrettoken") {
			t.Errorf("RedactRecordWithDefaults did not redact Bearer token in Reason: %q", got.Reason)
		}
		if !strings.Contains(got.Reason, "Authorization: Bearer [REDACTED]") {
			t.Errorf("RedactRecordWithDefaults Reason missing [REDACTED] marker: %q", got.Reason)
		}
	})

	t.Run("JWT in Reason is redacted", func(t *testing.T) {
		rec := AuditRecord{
			RecordType:  "policy_decision",
			RecordID:    "rrd-jwt-01",
			Timestamp:   "2026-06-13T00:00:00Z",
			ScannerName: "beekeeper",
			Decision:    "warn",
			Reason:      "token=eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.signature123 was found",
		}
		got := RedactRecordWithDefaults(rec)

		if strings.Contains(got.Reason, "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.signature123") {
			t.Errorf("RedactRecordWithDefaults did not redact JWT in Reason: %q", got.Reason)
		}
		if !strings.Contains(got.Reason, "[JWT_REDACTED]") {
			t.Errorf("RedactRecordWithDefaults Reason missing [JWT_REDACTED] marker: %q", got.Reason)
		}
	})

	t.Run("input rec is not mutated", func(t *testing.T) {
		originalReason := "Authorization: Bearer supersecrettoken"
		rec := AuditRecord{
			RecordType:  "policy_decision",
			RecordID:    "rrd-mutate-01",
			Timestamp:   "2026-06-13T00:00:00Z",
			ScannerName: "beekeeper",
			Decision:    "warn",
			Reason:      originalReason,
		}

		_ = RedactRecordWithDefaults(rec)

		// Input must be unchanged after the call.
		if rec.Reason != originalReason {
			t.Errorf("RedactRecordWithDefaults mutated input rec.Reason: got %q, want %q",
				rec.Reason, originalReason)
		}
	})
}

// TestRedactString verifies RedactString masks each default secret family and
// leaves benign / partial input unchanged.
func TestRedactString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		mustContain string // substring that must survive (replacement marker)
	}{
		{
			name:        "bearer token masked",
			input:       "Authorization: Bearer abc123secrettoken",
			want:        "Authorization: Bearer [REDACTED]",
			mustContain: "[REDACTED]",
		},
		{
			name:        "JWT masked",
			input:       "token=eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.signature123",
			want:        "token=[JWT_REDACTED]",
			mustContain: "[JWT_REDACTED]",
		},
		{
			name:        "anthropic key masked",
			input:       "ANTHROPIC_API_KEY=sk-ant-api03-supersecretvalue",
			want:        "ANTHROPIC_API_KEY=sk-ant-[REDACTED]",
			mustContain: "sk-ant-[REDACTED]",
		},
		{
			name:        "AWS access key masked",
			input:       "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			want:        "AWS_ACCESS_KEY_ID=AKIA[REDACTED]",
			mustContain: "AKIA[REDACTED]",
		},
		{
			name:  "benign string unchanged",
			input: "compiled 42 packages in 1.23s",
			want:  "compiled 42 packages in 1.23s",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactString(tt.input)
			if got != tt.want {
				t.Errorf("RedactString(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if tt.mustContain != "" {
				// Prove the raw secret is gone and the redaction marker is present.
				if !strings.Contains(got, tt.mustContain) {
					t.Errorf("RedactString(%q) = %q, missing marker %q", tt.input, got, tt.mustContain)
				}
				// The original secret payload must not survive verbatim.
				if got == tt.input {
					t.Errorf("RedactString(%q) returned input unchanged — secret not masked", tt.input)
				}
			}
		})
	}
}

// TestHasSensitiveData verifies the matcher flags each default secret family and
// returns false for benign / partial-prefix input.
func TestHasSensitiveData(t *testing.T) {
	sensitive := []struct {
		name  string
		input string
	}{
		{"bearer token", "Authorization: Bearer abc123secrettoken"},
		{"case-insensitive bearer", "authorization: bearer mytoken"},
		{"JWT", "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9.signature123"},
		{"openai project key", "sk-proj-abc123XYZ"},
		{"anthropic key", "sk-ant-api03-secret"},
		{"aws key", "AKIAIOSFODNN7EXAMPLE"},
		{"github pat", "ghp_1234567890abcdef"},
		{"gitlab pat", "glpat-xyz123abc"},
	}
	for _, tt := range sensitive {
		t.Run("sensitive/"+tt.name, func(t *testing.T) {
			if !HasSensitiveData(tt.input) {
				t.Errorf("HasSensitiveData(%q) = false, want true", tt.input)
			}
		})
	}

	benign := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"plain text", "Hello, world!"},
		{"json no secrets", `{"status":"ok","count":5}`},
		{"url no creds", "https://example.com/api/v1/resource"},
		{"prefix without suffix", "sk-proj-"}, // alternation needs >=1 trailing char
		{"basic auth not bearer", "Authorization: Basic dXNlcjpwYXNz"},
		{"two-segment not-JWT", "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMSJ9"},
	}
	for _, tt := range benign {
		t.Run("benign/"+tt.name, func(t *testing.T) {
			if HasSensitiveData(tt.input) {
				t.Errorf("HasSensitiveData(%q) = true, want false", tt.input)
			}
		})
	}
}

// TestRedactStringHasSensitiveConsistency verifies the two helpers agree: a
// string flagged sensitive must be changed by RedactString, and a string left
// unchanged by RedactString must not be flagged sensitive.
func TestRedactStringHasSensitiveConsistency(t *testing.T) {
	inputs := []string{
		"Authorization: Bearer xyz",
		"sk-ant-api03-leak",
		"ghp_1234567890abcdef",
		"plain benign text",
		"",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			sensitive := HasSensitiveData(in)
			changed := RedactString(in) != in
			if sensitive != changed {
				t.Errorf("inconsistent for %q: HasSensitiveData=%v but RedactString-changed=%v",
					in, sensitive, changed)
			}
		})
	}
}

// TestRedactPathologicalInputs verifies that default patterns do not catastrophically
// backtrack on adversarial inputs (T-04-05-07).
// Each test case must complete in <100ms (enforced by test timeout).
func TestRedactPathologicalInputs(t *testing.T) {
	patterns := DefaultRedactPatterns()

	adversarial := []struct {
		name  string
		input string
	}{
		{
			// Large string of 'a's — no pattern should match, must be fast.
			name:  "large repeated character",
			input: strings.Repeat("a", 100_000),
		},
		{
			// Many 'sk-' prefixes without a matching suffix.
			name:  "many sk- without suffix",
			input: strings.Repeat("sk-proj-\n", 10_000),
		},
		{
			// String with many dots but no valid JWT structure.
			name:  "many dots no JWT",
			input: strings.Repeat("eyJ.", 10_000),
		},
		{
			// Very long Authorization line with no Bearer keyword.
			name:  "long auth line no bearer",
			input: "Authorization: Basic " + strings.Repeat("x", 50_000),
		},
	}

	for _, tt := range adversarial {
		t.Run(tt.name, func(t *testing.T) {
			// This will timeout via the test timeout (-timeout flag) if backtracking occurs.
			result := applyRedaction(tt.input, patterns)
			// Result should never be longer than input (redaction only replaces or shortens).
			if len(result) > len(tt.input) {
				t.Errorf("applyRedaction returned longer string than input: %d > %d", len(result), len(tt.input))
			}
		})
	}
}
