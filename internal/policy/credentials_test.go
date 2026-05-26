package policy

import (
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// TestFilterCredentials exercises the output credential filtering policy (PLCY-08).
func TestFilterCredentials(t *testing.T) {
	cfg := CredentialFilterConfig{}

	tests := []struct {
		name              string
		input             string
		wantRedacted      bool   // if true, Redacted != input
		wantType          string // at least this type in DetectedTypes
		wantContainsCreds bool
	}{
		{
			name:              "AWS access key detected and redacted",
			input:             "Access key: AKIAIOSFODNN7EXAMPLE remaining text",
			wantRedacted:      true,
			wantType:          "aws-access-key",
			wantContainsCreds: true,
		},
		{
			name:              "JWT token detected and redacted",
			input:             "Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			wantRedacted:      true,
			wantType:          "jwt",
			wantContainsCreds: true,
		},
		{
			name:              "Bearer token detected and redacted",
			input:             "Authorization: Bearer abc123def456ghi789jkl",
			wantRedacted:      true,
			wantType:          "bearer",
			wantContainsCreds: true,
		},
		{
			name:              "GitHub PAT detected and redacted",
			input:             "GitHub token: ghp_abcdefghijklmnopqrstuvwxyz0123456789abcd",
			wantRedacted:      true,
			wantType:          "github-token",
			wantContainsCreds: true,
		},
		{
			name:              "npm token detected and redacted",
			input:             "npm token: npm_abcdefghijklmnopqrstuvwxyz0123456789abcd",
			wantRedacted:      true,
			wantType:          "npm-token",
			wantContainsCreds: true,
		},
		{
			name:              "OpenAI key detected and redacted",
			input:             "key: sk-abcdefghijklmnopqrstuvwxyz01234567890123",
			wantRedacted:      true,
			wantType:          "openai-key",
			wantContainsCreds: true,
		},
		{
			name:              "clean output passes through unchanged",
			input:             "hello world",
			wantRedacted:      false,
			wantContainsCreds: false,
		},
		{
			name:              "empty string passes through",
			input:             "",
			wantRedacted:      false,
			wantContainsCreds: false,
		},
		{
			name:              "GitHub org token (gho_) detected",
			input:             "org token: gho_abcdefghijklmnopqrstuvwxyz0123456789abcd",
			wantRedacted:      true,
			wantType:          "github-token",
			wantContainsCreds: true,
		},
		{
			name:              "GitHub user token (ghu_) detected",
			input:             "user token: ghu_abcdefghijklmnopqrstuvwxyz0123456789abcd",
			wantRedacted:      true,
			wantType:          "github-token",
			wantContainsCreds: true,
		},
		{
			name:              "GitHub server token (ghs_) detected",
			input:             "server token: ghs_abcdefghijklmnopqrstuvwxyz0123456789abcd",
			wantRedacted:      true,
			wantType:          "github-token",
			wantContainsCreds: true,
		},
		{
			name:              "GitHub refresh token (ghr_) detected",
			input:             "refresh token: ghr_abcdefghijklmnopqrstuvwxyz0123456789abcd",
			wantRedacted:      true,
			wantType:          "github-token",
			wantContainsCreds: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterCredentials(tt.input, cfg)

			if tt.wantContainsCreds {
				if !result.ContainsCredentials {
					t.Errorf("ContainsCredentials = false, want true")
				}
				if tt.wantRedacted && result.Redacted == tt.input {
					t.Errorf("Redacted = input unchanged, want credential to be replaced")
				}
				if tt.wantType != "" && !containsType(result.DetectedTypes, tt.wantType) {
					t.Errorf("DetectedTypes = %v, want to contain %q", result.DetectedTypes, tt.wantType)
				}
			} else {
				if result.ContainsCredentials {
					t.Errorf("ContainsCredentials = true, want false for clean input")
				}
				if result.Redacted != tt.input {
					t.Errorf("Redacted = %q, want unchanged input %q", result.Redacted, tt.input)
				}
				if len(result.DetectedTypes) != 0 {
					t.Errorf("DetectedTypes = %v, want empty for clean input", result.DetectedTypes)
				}
			}
		})
	}
}

// TestFilterCredentialsRedactionFormat checks that redacted values use the
// [REDACTED:<type>] format required by PLCY-08.
func TestFilterCredentialsRedactionFormat(t *testing.T) {
	cfg := CredentialFilterConfig{}

	result := FilterCredentials("AKIAIOSFODNN7EXAMPLE", cfg)
	if result.Redacted != "[REDACTED:aws-access-key]" {
		t.Errorf("Redacted = %q, want %q", result.Redacted, "[REDACTED:aws-access-key]")
	}
}

// TestFilterCredentialsCustomPattern checks that additional regex patterns
// supplied via config are applied and redact matching content.
func TestFilterCredentialsCustomPattern(t *testing.T) {
	cfg := CredentialFilterConfig{
		AdditionalPatterns: []string{`MYTOKEN-[A-Z0-9]{8}`},
	}

	result := FilterCredentials("secret: MYTOKEN-ABCD1234 here", cfg)
	if !result.ContainsCredentials {
		t.Errorf("ContainsCredentials = false, want true")
	}
	if result.Redacted == "secret: MYTOKEN-ABCD1234 here" {
		t.Errorf("custom pattern not redacted")
	}
	if !containsType(result.DetectedTypes, "custom") {
		t.Errorf("DetectedTypes = %v, want to contain 'custom'", result.DetectedTypes)
	}
}

// TestFilterCredentialsDeduplication checks that the same credential type
// appearing multiple times in the output is only reported once in DetectedTypes.
func TestFilterCredentialsDeduplication(t *testing.T) {
	cfg := CredentialFilterConfig{}
	// Two AWS access keys in the same output.
	result := FilterCredentials("k1: AKIAIOSFODNN7EXAMPLE k2: AKIAJ3HFWZK2Y4JQOVSC", cfg)
	if !result.ContainsCredentials {
		t.Errorf("ContainsCredentials = false")
	}
	count := 0
	for _, dt := range result.DetectedTypes {
		if dt == "aws-access-key" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("DetectedTypes contains 'aws-access-key' %d times, want exactly 1 (deduped)", count)
	}
}

// TestCredentialsImportsArePure enforces the purity contract for credentials.go.
func TestCredentialsImportsArePure(t *testing.T) {
	const filePath = "credentials.go"
	src, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading %s: %v", filePath, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", filePath, err)
	}

	forbidden := map[string]bool{
		"os":       true,
		"net":      true,
		"net/http": true,
		"io":       true,
		"sync":     true,
		"time":     true,
		"context":  true,
	}

	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("credentials.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}

// containsType reports whether types contains target.
func containsType(types []string, target string) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}
