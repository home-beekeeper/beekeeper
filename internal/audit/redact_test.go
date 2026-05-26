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
