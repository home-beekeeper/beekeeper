package policy

import (
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// TestEvaluatePath exercises the sensitive-path policy (PLCY-04).
// resolvedPath is always pre-resolved by the caller (no "~" in these inputs).
func TestEvaluatePath(t *testing.T) {
	cfg := DefaultSensitivePaths()

	tests := []struct {
		name        string
		path        string
		wantLevel   string
		wantAllow   bool
		wantPattern string // if non-empty, Reason must contain this string
	}{
		{
			name:        "ssh key blocked",
			path:        "/home/u/.ssh/id_rsa",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: "/.ssh/",
		},
		{
			name:        "aws credentials blocked",
			path:        "/home/u/.aws/credentials",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: "/.aws/",
		},
		{
			name:        "gnupg blocked",
			path:        "/home/u/.gnupg/pubring.kbx",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: "/.gnupg/",
		},
		{
			name:        ".env file blocked",
			path:        "/home/u/project/.env",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".env",
		},
		{
			name:        ".env.local file blocked",
			path:        "/home/u/project/.env.local",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".env",
		},
		{
			name:        ".env.production blocked by .env.* glob",
			path:        "/home/u/project/.env.production",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".env",
		},
		{
			name:      "clean go source file allowed",
			path:      "/home/u/project/src/main.go",
			wantLevel: "allow",
			wantAllow: true,
		},
		{
			name:      "readme allowed",
			path:      "/home/u/project/README.md",
			wantLevel: "allow",
			wantAllow: true,
		},
		{
			name:        "netrc blocked",
			path:        "/home/u/.netrc",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".netrc",
		},
		{
			name:        "npmrc blocked",
			path:        "/home/u/.npmrc",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".npmrc",
		},
		{
			name:        "pypirc blocked",
			path:        "/home/u/.pypirc",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".pypirc",
		},
		{
			name:        "cargo credentials blocked",
			path:        "/home/u/.cargo/credentials.toml",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: "/.cargo/credentials.toml",
		},
		{
			name:        "1Password CLI config blocked",
			path:        "/home/u/.config/op/config",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: "/.config/op/",
		},
		{
			name:        "GitHub CLI config blocked",
			path:        "/home/u/.config/gh/hosts.yml",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: "/.config/gh/",
		},
		{
			name:        "Claude config blocked",
			path:        "/home/u/.config/Claude/settings.json",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: "/.config/Claude/",
		},
		// HARDEN-02 / IN-02: Windows ADS + trailing-dot/space basename evasion.
		// These exercise the BASENAME branch of matchesBlockPattern via the pure
		// normalizeBasename helper (OS-agnostic — no //go:build windows needed).
		{
			name:        ".env ADS stream blocked (basename branch)",
			path:        "/home/u/project/.env:stream",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".env",
		},
		{
			name:        ".env trailing dot blocked",
			path:        "/home/u/project/.env.",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".env",
		},
		{
			name:        ".env trailing space blocked",
			path:        "/home/u/project/.env ",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".env",
		},
		{
			name:        "netrc ADS stream blocked (basename branch)",
			path:        "/home/u/.netrc:hidden",
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".netrc",
		},
		{
			name:        "npmrc trailing dot blocked",
			path:        `C:\Users\u\.npmrc.`,
			wantLevel:   "block",
			wantAllow:   false,
			wantPattern: ".npmrc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := EvaluatePath(tt.path, cfg)
			if d.Level != tt.wantLevel {
				t.Errorf("Level = %q, want %q", d.Level, tt.wantLevel)
			}
			if d.Allow != tt.wantAllow {
				t.Errorf("Allow = %v, want %v", d.Allow, tt.wantAllow)
			}
			if tt.wantPattern != "" && !containsStr(d.Reason, tt.wantPattern) {
				t.Errorf("Reason = %q, want it to contain %q", d.Reason, tt.wantPattern)
			}
		})
	}
}

// TestEvaluatePathAllowlistOverride verifies that an explicit AllowPattern
// overrides the blocklist — even for a normally-blocked path.
func TestEvaluatePathAllowlistOverride(t *testing.T) {
	cfg := DefaultSensitivePaths()
	// Explicitly allowlist a path that would normally be blocked.
	cfg.AllowPatterns = append(cfg.AllowPatterns, "/home/u/.aws/credentials")

	d := EvaluatePath("/home/u/.aws/credentials", cfg)
	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (allowlist must override blocklist)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true")
	}
}

// TestEvaluatePathWindowsStyle checks that Windows-style paths (with backslash
// segments) are handled. The caller normalizes the path before calling.
func TestEvaluatePathWindowsStyle(t *testing.T) {
	cfg := DefaultSensitivePaths()

	// Windows-style path with .ssh segment
	d := EvaluatePath(`C:\Users\u\.ssh\id_rsa`, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q for Windows .ssh path", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false")
	}
}

// TestEvaluatePathWindowsEnv checks that a Windows .env file path is blocked.
func TestEvaluatePathWindowsEnv(t *testing.T) {
	cfg := DefaultSensitivePaths()

	d := EvaluatePath(`C:\Users\u\project\.env`, cfg)
	if d.Level != "block" {
		t.Errorf("Level = %q, want %q for Windows .env path", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false")
	}
}

// TestEvaluatePathRuleID checks that block decisions carry the correct rule ID.
func TestEvaluatePathRuleID(t *testing.T) {
	cfg := DefaultSensitivePaths()
	d := EvaluatePath("/home/u/.ssh/id_rsa", cfg)
	if len(d.RuleIDs) == 0 || d.RuleIDs[0] != "sensitive-path-policy" {
		t.Errorf("RuleIDs = %v, want [sensitive-path-policy]", d.RuleIDs)
	}
}

// TestPathImportsArePure enforces the purity contract for path.go: it must not
// import any package that performs I/O, concurrency, or wall-clock access.
func TestPathImportsArePure(t *testing.T) {
	const filePath = "path.go"
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
			t.Errorf("path.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}

// TestEvaluatePathBasenameAllowlist verifies that the .env.example, .env.test,
// and .env.schema safe lookalikes are allowed by the basename allowlist, while
// .env.production and .env (blocked patterns) remain blocked.
func TestEvaluatePathBasenameAllowlist(t *testing.T) {
	cfg := DefaultSensitivePaths()

	tests := []struct {
		name      string
		path      string
		wantAllow bool
		wantLevel string
	}{
		{
			name:      ".env.example allowed via basename allowlist",
			path:      "/home/u/project/.env.example",
			wantAllow: true,
			wantLevel: "allow",
		},
		{
			name:      ".env.test allowed via basename allowlist",
			path:      "/home/u/project/.env.test",
			wantAllow: true,
			wantLevel: "allow",
		},
		{
			name:      ".env.schema allowed via basename allowlist",
			path:      "/home/u/project/.env.schema",
			wantAllow: true,
			wantLevel: "allow",
		},
		{
			name:      ".env.production still blocked by .env.* glob",
			path:      "/home/u/project/.env.production",
			wantAllow: false,
			wantLevel: "block",
		},
		{
			name:      ".env still blocked",
			path:      "/home/u/project/.env",
			wantAllow: false,
			wantLevel: "block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := EvaluatePath(tt.path, cfg)
			if d.Allow != tt.wantAllow {
				t.Errorf("Allow = %v, want %v", d.Allow, tt.wantAllow)
			}
			if d.Level != tt.wantLevel {
				t.Errorf("Level = %q, want %q", d.Level, tt.wantLevel)
			}
		})
	}
}

// TestNormalizeBasename unit-tests the pure ADS/trailing-dot normalizer
// (HARDEN-02 / IN-02). It strips a trailing :streamname ADS suffix and trims
// trailing dots/spaces; it must leave a clean basename untouched.
func TestNormalizeBasename(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"id_rsa", "id_rsa"},
		{"id_rsa:fakestream", "id_rsa"},
		{"id_rsa:$DATA", "id_rsa"},
		{".env:stream", ".env"},
		{"credentials.", "credentials"},
		{"credentials ", "credentials"},
		{"credentials.  ", "credentials"},
		{".env.example:stream", ".env.example"},
		{".env.example.", ".env.example"},
		{"", ""},
		{":onlystream", ""},
		// First-colon cut: any ':' is an ADS separator at the segment level.
		{"a:b:c", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeBasename(tt.in); got != tt.want {
				t.Errorf("normalizeBasename(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestEvaluatePathBasenameADSBlock proves that an ADS-suffixed or trailing-dot
// basename matches a custom basename BlockPattern exactly as the clean basename
// would (the basename branch, e.g. "id_rsa", which is not a default pattern but
// represents the general HARDEN-02 case). IN-02.
func TestEvaluatePathBasenameADSBlock(t *testing.T) {
	cfg := SensitivePathConfig{BlockPatterns: []string{"id_rsa"}}

	for _, p := range []string{
		"/home/u/.ssh/id_rsa",
		"/home/u/.ssh/id_rsa:fakestream",
		`C:\Users\u\.ssh\id_rsa:$DATA`,
		"/home/u/.ssh/id_rsa.",
		"/home/u/.ssh/id_rsa ",
	} {
		t.Run(p, func(t *testing.T) {
			d := EvaluatePath(p, cfg)
			if d.Level != "block" {
				t.Errorf("EvaluatePath(%q).Level = %q, want block (HARDEN-02 basename branch)", p, d.Level)
			}
		})
	}
}

// TestEvaluatePathAllowlistNormalizationAligned verifies that the allowlist
// normalization mirrors the blocklist: ".env.example:stream" and ".env.example."
// stay ALLOWED (the ADS/trailing-dot form of a safe lookalike must not be
// un-allowlisted), while ".env:stream" and ".env." (block lookalikes) stay
// BLOCKED. IN-02 / HARDEN-02.
func TestEvaluatePathAllowlistNormalizationAligned(t *testing.T) {
	cfg := DefaultSensitivePaths()

	tests := []struct {
		name      string
		path      string
		wantAllow bool
		wantLevel string
	}{
		{".env.example:stream stays allowed", "/home/u/project/.env.example:stream", true, "allow"},
		{".env.example. stays allowed", "/home/u/project/.env.example.", true, "allow"},
		{".env.example space stays allowed", "/home/u/project/.env.example ", true, "allow"},
		{".env:stream stays blocked", "/home/u/project/.env:stream", false, "block"},
		{".env. stays blocked", "/home/u/project/.env.", false, "block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := EvaluatePath(tt.path, cfg)
			if d.Allow != tt.wantAllow {
				t.Errorf("EvaluatePath(%q).Allow = %v, want %v", tt.path, d.Allow, tt.wantAllow)
			}
			if d.Level != tt.wantLevel {
				t.Errorf("EvaluatePath(%q).Level = %q, want %q", tt.path, d.Level, tt.wantLevel)
			}
		})
	}
}

// TestEvaluatePathCursorMCPBlocked verifies that Cursor MCP config paths are
// blocked by the /.cursor/ fragment pattern (D-02).
func TestEvaluatePathCursorMCPBlocked(t *testing.T) {
	cfg := DefaultSensitivePaths()

	paths := []string{
		"/home/u/.cursor/mcp.json",
		"/home/u/.cursor/settings.json",
		`C:\Users\u\.cursor\mcp.json`,
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			d := EvaluatePath(p, cfg)
			if d.Level != "block" {
				t.Errorf("EvaluatePath(%q).Level = %q, want %q", p, d.Level, "block")
			}
			if d.Allow {
				t.Errorf("EvaluatePath(%q).Allow = true, want false", p)
			}
		})
	}
}

// TestEvaluatePathWindsurfMCPBlocked verifies that Windsurf MCP config paths
// are blocked by the /.windsurf/ fragment pattern (D-02).
func TestEvaluatePathWindsurfMCPBlocked(t *testing.T) {
	cfg := DefaultSensitivePaths()

	paths := []string{
		"/home/u/.windsurf/mcp.json",
		"/home/u/.windsurf/settings.json",
		`C:\Users\u\.windsurf\mcp.json`,
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			d := EvaluatePath(p, cfg)
			if d.Level != "block" {
				t.Errorf("EvaluatePath(%q).Level = %q, want %q", p, d.Level, "block")
			}
			if d.Allow {
				t.Errorf("EvaluatePath(%q).Allow = true, want false", p)
			}
		})
	}
}

// TestEvaluatePathCargoCredentialsBlocked verifies that both the bare
// /.cargo/credentials file and the .toml variant are blocked (D-02).
func TestEvaluatePathCargoCredentialsBlocked(t *testing.T) {
	cfg := DefaultSensitivePaths()

	tests := []struct {
		name string
		path string
	}{
		{"bare credentials (pre-2022 format)", "/home/u/.cargo/credentials"},
		{"credentials.toml (current format)", "/home/u/.cargo/credentials.toml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := EvaluatePath(tt.path, cfg)
			if d.Level != "block" {
				t.Errorf("EvaluatePath(%q).Level = %q, want %q", tt.path, d.Level, "block")
			}
			if d.Allow {
				t.Errorf("EvaluatePath(%q).Allow = true, want false", tt.path)
			}
		})
	}
}

// containsStr reports whether s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
