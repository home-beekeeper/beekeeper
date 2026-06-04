package check

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bantuson/beekeeper/internal/policy"
)

// ---------------------------------------------------------------------------
// TestExpandWinEnvVars
// ---------------------------------------------------------------------------

func TestExpandWinEnvVars(t *testing.T) {
	t.Run("expands USERPROFILE", func(t *testing.T) {
		t.Setenv("USERPROFILE", `C:\Users\testuser`)
		result := expandWinEnvVars(`%USERPROFILE%\.ssh\id_rsa`)
		if !strings.Contains(result, `C:\Users\testuser`) {
			t.Errorf("expected USERPROFILE expanded, got %q", result)
		}
	})

	t.Run("expands HOMEPATH", func(t *testing.T) {
		t.Setenv("HOMEPATH", `\Users\testuser`)
		result := expandWinEnvVars(`%HOMEPATH%\.aws\credentials`)
		if !strings.Contains(result, `\Users\testuser`) {
			t.Errorf("expected HOMEPATH expanded, got %q", result)
		}
	})

	t.Run("case-insensitive var name", func(t *testing.T) {
		t.Setenv("USERPROFILE", `C:\Users\testuser`)
		result := expandWinEnvVars(`%userprofile%\.ssh\id_rsa`)
		if !strings.Contains(result, `C:\Users\testuser`) {
			t.Errorf("expected case-insensitive expansion, got %q", result)
		}
	})

	t.Run("fail-closed on unresolved var", func(t *testing.T) {
		t.Setenv("USERPROFILE", "")
		// On Windows USERPROFILE is usually set; on CI it may be set. Unset it.
		result := expandWinEnvVars(`%UNRESOLVABLE_VAR_XYZ%\.ssh\id_rsa`)
		// Fail-closed: the raw %VAR% token must be preserved, not removed.
		if !strings.Contains(result, "%UNRESOLVABLE_VAR_XYZ%") {
			t.Errorf("fail-closed: expected raw %%VAR%% preserved, got %q", result)
		}
	})

	t.Run("no-op on path without percent vars", func(t *testing.T) {
		input := `~/.aws/credentials`
		result := expandWinEnvVars(input)
		if result != input {
			t.Errorf("expected unchanged, got %q", result)
		}
	})

	t.Run("does not use os.ExpandEnv dollar form", func(t *testing.T) {
		t.Setenv("HOME", "/expanded-home")
		result := expandWinEnvVars("$HOME/.aws/credentials")
		// os.ExpandEnv would expand $HOME; we must NOT do that.
		if strings.Contains(result, "/expanded-home") {
			t.Errorf("should not expand dollar-sign vars, got %q", result)
		}
	})

	t.Run("substituted value containing % is not re-expanded (WR-01 single-pass)", func(t *testing.T) {
		// If A expands to a value containing %, the old multi-pass loop could
		// treat that stray % as the start of a new variable reference. The
		// single-pass Builder rewrite must not re-scan already-emitted content.
		t.Setenv("MYVAR_WR01_TEST", "val%with%percent")
		result := expandWinEnvVars("%MYVAR_WR01_TEST%\\rest")
		// The value must appear verbatim; the "%" characters in it must not
		// trigger further expansion (e.g. pairing with the trailing \ to form
		// a new %VAR% pattern).
		if !strings.Contains(result, "val%with%percent") {
			t.Errorf("expected substituted value verbatim in result, got %q", result)
		}
	})
}

// ---------------------------------------------------------------------------
// TestCanonicalizePath
// ---------------------------------------------------------------------------

func TestCanonicalizePath(t *testing.T) {
	t.Run("empty returns empty", func(t *testing.T) {
		if got := canonicalizePath(""); got != "" {
			t.Errorf("canonicalizePath(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("tilde expands to absolute path containing credential fragment", func(t *testing.T) {
		got := canonicalizePath("~/.aws/credentials")
		if got == "" {
			t.Fatal("canonicalizePath returned empty for tilde path")
		}
		if !filepath_has_prefix(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
		if !strings.Contains(got, "/.aws/credentials") {
			t.Errorf("expected /.aws/credentials in result, got %q", got)
		}
		if strings.Contains(got, "~") {
			t.Errorf("tilde should be expanded, got %q", got)
		}
	})

	t.Run("dot-dot traversal resolves to absolute path (SPATH-02)", func(t *testing.T) {
		got := canonicalizePath("../../.aws/credentials")
		if got == "" {
			t.Fatal("canonicalizePath returned empty for traversal path")
		}
		if !filepath_has_prefix(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
		if strings.Contains(got, "..") {
			t.Errorf("traversal should be resolved, got %q", got)
		}
	})

	t.Run("non-existent path still resolves (Pitfall 3 — EvalSymlinks fallback to Abs)", func(t *testing.T) {
		// ~/.aws/credentials may not exist on the test machine.
		// It must NOT return "" — EvalSymlinks error falls back to Abs.
		got := canonicalizePath("~/.aws/credentials")
		if got == "" {
			t.Error("non-existent credential path must not return empty (Pitfall 3)")
		}
		// The /.aws/ fragment must appear so EvaluatePath can block it.
		if !strings.Contains(got, "/.aws/") {
			t.Errorf("expected /.aws/ fragment in result even for non-existent path, got %q", got)
		}
	})

	t.Run("USERPROFILE env-var expansion (D-01)", func(t *testing.T) {
		t.Setenv("USERPROFILE", "/home/testuser")
		got := canonicalizePath(`%USERPROFILE%\.ssh\id_rsa`)
		if got == "" {
			t.Fatal("canonicalizePath returned empty for USERPROFILE path")
		}
		if !strings.Contains(got, "/.ssh/") {
			t.Errorf("expected /.ssh/ fragment after USERPROFILE expansion, got %q", got)
		}
	})

	t.Run("uses forward slashes (ToSlash normalization)", func(t *testing.T) {
		got := canonicalizePath("~/.aws/credentials")
		if strings.Contains(got, "\\") {
			t.Errorf("expected forward slashes only, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// TestCanonicalizePathForms (HARDEN-01 / IN-01)
// ---------------------------------------------------------------------------

func TestCanonicalizePathForms(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		if got := canonicalizePathForms(""); got != nil {
			t.Errorf("canonicalizePathForms(\"\") = %v, want nil", got)
		}
	})

	t.Run("non-existent credential path yields a /.aws/ form (Pitfall 3 fallback preserved)", func(t *testing.T) {
		forms := canonicalizePathForms("~/.aws/credentials")
		if len(forms) == 0 {
			t.Fatal("canonicalizePathForms returned no forms for tilde credential path")
		}
		if !anyFormContains(forms, "/.aws/") {
			t.Errorf("expected at least one form containing /.aws/, got %v", forms)
		}
	})

	t.Run("ancestor symlink: sensitive fragment survives in at least one form (HARDEN-01)", func(t *testing.T) {
		// Plant an ancestor symlink: real dir T (t.TempDir), symlink L -> T.
		// Request canonicalizePathForms(L + "/.aws/credentials").
		// The pre-fix single-form canonicalizePath would EvalSymlinks the L
		// ancestor and produce a path under T — but because the /.aws/ subdir
		// does not exist under T, EvalSymlinks of the FULL path errors and the
		// fallback Abs path is the *symlink* path (L/.aws/credentials), which
		// happens to retain /.aws/. To make the regression bite even when the
		// fragment dir DOES exist (EvalSymlinks succeeds end-to-end), we create
		// the .aws/credentials structure under T so EvalSymlinks fully resolves
		// L -> T; the resolved form then carries T's real path. The LEXICAL form
		// (no EvalSymlinks) keeps the L/.aws/ shape. We assert at least one form
		// carries /.aws/ regardless — which is the whole point of dual-form.
		realDir := t.TempDir()
		awsDir := filepath.Join(realDir, ".aws")
		if err := os.MkdirAll(awsDir, 0o755); err != nil {
			t.Fatalf("mkdir .aws: %v", err)
		}
		credFile := filepath.Join(awsDir, "credentials")
		if err := os.WriteFile(credFile, []byte("[default]\n"), 0o600); err != nil {
			t.Fatalf("write credentials: %v", err)
		}

		linkPath := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(realDir, linkPath); err != nil {
			// os.Symlink fails on Windows without the SeCreateSymbolicLink
			// privilege (Developer Mode off). Skip cleanly so the unprivileged
			// Windows dev box stays green; CI Linux/macOS exercise this path.
			if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "privilege") {
				t.Skipf("os.Symlink requires privilege on this host, skipping: %v", err)
			}
			t.Fatalf("os.Symlink: %v", err)
		}

		// Request the credential read THROUGH the ancestor symlink.
		target := linkPath + "/.aws/credentials"
		forms := canonicalizePathForms(target)
		if len(forms) == 0 {
			t.Fatal("canonicalizePathForms returned no forms for symlink-ancestor path")
		}

		// The crux of HARDEN-01: at least one returned form must still carry the
		// /.aws/ fragment so a downstream block matches. The lexical form (no
		// EvalSymlinks) guarantees this even when EvalSymlinks rewrites the
		// ancestor away.
		if !anyFormContains(forms, "/.aws/") {
			t.Errorf("HARDEN-01: no returned form carries /.aws/ — ancestor symlink dodged the blocklist; forms=%v", forms)
		}

		// Sanity: a downstream policy.EvaluatePath on the surviving form blocks.
		cfg := policy.DefaultSensitivePaths()
		blocked := false
		for _, f := range forms {
			if d := policy.EvaluatePath(f, cfg); d.Level == "block" {
				blocked = true
				break
			}
		}
		if !blocked {
			t.Errorf("HARDEN-01: no form blocked by EvaluatePath; forms=%v", forms)
		}
	})

	t.Run("de-duplicates identical forms", func(t *testing.T) { //nolint:thelper
		// An absolute, non-symlink, existing path: lexical and resolved forms are
		// identical and must be collapsed to a single entry.
		dir := t.TempDir()
		f := filepath.Join(dir, "plain.txt")
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		forms := canonicalizePathForms(f)
		if len(forms) != 1 {
			t.Errorf("expected 1 de-duplicated form for a non-symlink existing path, got %d: %v", len(forms), forms)
		}
	})
}

// ---------------------------------------------------------------------------
// TestCanonicalizeEvaluateADS (HARDEN-02 / IN-02) — Windows-gated adapter test
// ---------------------------------------------------------------------------

// TestCanonicalizeEvaluateADS exercises the END-TO-END adapter path
// (canonicalize -> EvaluatePath) for Windows ADS and trailing-dot basename
// evasion forms. It is Windows-gated: %USERPROFILE% expansion + backslash
// separators are Windows-shaped, so non-Windows hosts skip. The pure layer is
// covered OS-agnostically in internal/policy/path_test.go.
func TestCanonicalizeEvaluateADS(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-shaped ADS/trailing-dot adapter test; pure coverage lives in internal/policy")
	}

	t.Setenv("USERPROFILE", `C:\Users\testuser`)
	cfg := policy.DefaultSensitivePaths()

	cases := []struct {
		name string
		raw  string
	}{
		{"ssh ADS stream blocks", `%USERPROFILE%\.ssh\id_rsa:fakestream`},
		{"credentials trailing dot blocks", `%USERPROFILE%\.ssh\credentials.`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Use the dual-form helper (HARDEN-01) and block on ANY form, mirroring
			// the wiring Plan 03 lands in handler.go.
			forms := canonicalizePathForms(tc.raw)
			if len(forms) == 0 {
				t.Fatalf("no forms for %q", tc.raw)
			}
			blocked := false
			for _, f := range forms {
				if d := policy.EvaluatePath(f, cfg); d.Level == "block" {
					blocked = true
					break
				}
			}
			if !blocked {
				t.Errorf("HARDEN-02: %q (%q) was not blocked; forms=%v", tc.name, tc.raw, forms)
			}
		})
	}
}

// anyFormContains reports whether any string in forms contains sub.
func anyFormContains(forms []string, sub string) bool {
	for _, f := range forms {
		if strings.Contains(f, sub) {
			return true
		}
	}
	return false
}

// filepath_has_prefix returns true if the path looks absolute (forward-slash or
// drive-letter form). A simple heuristic for test assertions.
func filepath_has_prefix(p string) bool {
	if len(p) == 0 {
		return false
	}
	// Unix: starts with /
	if p[0] == '/' {
		return true
	}
	// Windows drive letter: "C:/"
	if len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// TestExtractPathTargets
// ---------------------------------------------------------------------------

func TestExtractPathTargets(t *testing.T) {
	t.Run("extracts file_path key (Claude Code Read/Write/Edit)", func(t *testing.T) {
		tc := policy.ToolCall{
			ToolName:  "Read",
			ToolInput: map[string]any{"file_path": "~/.aws/credentials"},
		}
		got := extractPathTargets(tc)
		if len(got) == 0 {
			t.Fatal("expected non-empty result for file_path")
		}
		if got[0] != "~/.aws/credentials" {
			t.Errorf("expected ~/.aws/credentials, got %q", got[0])
		}
	})

	t.Run("extracts path key (legacy fallback compat)", func(t *testing.T) {
		tc := policy.ToolCall{
			ToolInput: map[string]any{"path": "/legacy"},
		}
		got := extractPathTargets(tc)
		if len(got) == 0 {
			t.Fatal("expected non-empty result for path key")
		}
		found := false
		for _, p := range got {
			if p == "/legacy" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected /legacy in result, got %v", got)
		}
	})

	t.Run("non-file non-Bash tool returns nil", func(t *testing.T) {
		tc := policy.ToolCall{
			ToolName:  "WebFetch",
			ToolInput: map[string]any{"url": "http://example.com"},
		}
		got := extractPathTargets(tc)
		if len(got) != 0 {
			t.Errorf("expected nil for WebFetch, got %v", got)
		}
	})

	t.Run("Bash echo command returns nil (no credential verb)", func(t *testing.T) {
		tc := policy.ToolCall{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": "echo hi"},
		}
		got := extractPathTargets(tc)
		if len(got) != 0 {
			t.Errorf("expected nil for echo, got %v", got)
		}
	})

	t.Run("nil ToolInput returns nil without panic", func(t *testing.T) {
		tc := policy.ToolCall{ToolInput: nil}
		got := extractPathTargets(tc)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("Bash cat command extracts path", func(t *testing.T) {
		tc := policy.ToolCall{
			ToolName:  "Bash",
			ToolInput: map[string]any{"command": "cat ~/.aws/credentials"},
		}
		got := extractPathTargets(tc)
		if len(got) == 0 {
			t.Fatal("expected path from Bash cat command")
		}
		found := false
		for _, p := range got {
			if p == "~/.aws/credentials" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected ~/.aws/credentials in result, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMergeDecisions
// ---------------------------------------------------------------------------

func TestMergeDecisions(t *testing.T) {
	allow := policy.Decision{Allow: true, Level: "allow", Reason: "no match"}
	warn := policy.Decision{Allow: true, Level: "warn", Reason: "warn match"}
	block := policy.Decision{Allow: false, Level: "block", Reason: "sensitive path blocked: /.ssh/", RuleIDs: []string{"sensitive-path-policy"}}

	t.Run("block overlays allow → block wins", func(t *testing.T) {
		got := mergeDecisions(allow, block)
		if got.Level != "block" {
			t.Errorf("expected block, got %q", got.Level)
		}
	})

	t.Run("allow overlays block → block stays", func(t *testing.T) {
		got := mergeDecisions(block, allow)
		if got.Level != "block" {
			t.Errorf("expected block to persist, got %q", got.Level)
		}
	})

	t.Run("warn overlays allow → warn wins", func(t *testing.T) {
		got := mergeDecisions(allow, warn)
		if got.Level != "warn" {
			t.Errorf("expected warn, got %q", got.Level)
		}
	})

	t.Run("block overlays warn → block wins", func(t *testing.T) {
		got := mergeDecisions(warn, block)
		if got.Level != "block" {
			t.Errorf("expected block, got %q", got.Level)
		}
	})

	t.Run("allow overlays allow → allow", func(t *testing.T) {
		got := mergeDecisions(allow, allow)
		if got.Level != "allow" {
			t.Errorf("expected allow, got %q", got.Level)
		}
	})

	t.Run("base block with warn overlay → block preserved", func(t *testing.T) {
		got := mergeDecisions(block, warn)
		if got.Level != "block" {
			t.Errorf("expected block to be preserved over warn overlay, got %q", got.Level)
		}
	})

	t.Run("rule IDs preserved from most-restrictive decision", func(t *testing.T) {
		got := mergeDecisions(allow, block)
		if len(got.RuleIDs) == 0 {
			t.Error("expected rule IDs from block decision to be preserved")
		}
		if got.RuleIDs[0] != "sensitive-path-policy" {
			t.Errorf("expected sensitive-path-policy rule ID, got %v", got.RuleIDs)
		}
	})
}

// ---------------------------------------------------------------------------
// TestExtractBashCredentialPaths
// ---------------------------------------------------------------------------

func TestExtractBashCredentialPaths(t *testing.T) {
	t.Run("cat with tilde path", func(t *testing.T) {
		got := extractBashCredentialPaths("cat ~/.aws/credentials")
		assertContains(t, got, "~/.aws/credentials")
	})

	t.Run("Get-Content PowerShell", func(t *testing.T) {
		got := extractBashCredentialPaths("Get-Content ~/.npmrc")
		assertContains(t, got, "~/.npmrc")
	})

	t.Run("gc PowerShell alias", func(t *testing.T) {
		got := extractBashCredentialPaths("gc ~/.ssh/id_rsa")
		assertContains(t, got, "~/.ssh/id_rsa")
	})

	t.Run("quoted path strips surrounding double-quotes", func(t *testing.T) {
		got := extractBashCredentialPaths(`cat "~/.aws/credentials"`)
		assertContains(t, got, "~/.aws/credentials")
	})

	t.Run("quoted path strips surrounding single-quotes", func(t *testing.T) {
		got := extractBashCredentialPaths("cat '~/.npmrc'")
		assertContains(t, got, "~/.npmrc")
	})

	t.Run("type command with USERPROFILE Windows form (D-01 / SC2)", func(t *testing.T) {
		// extractBashCredentialPaths returns the raw %USERPROFILE%\.ssh\id_rsa token.
		// canonicalizePath later expands %USERPROFILE% (D-01).
		got := extractBashCredentialPaths(`type %USERPROFILE%\.ssh\id_rsa`)
		assertContains(t, got, `%USERPROFILE%\.ssh\id_rsa`)
	})

	t.Run("verb found mid-string after command chaining", func(t *testing.T) {
		got := extractBashCredentialPaths("ls && cat ~/.aws/credentials")
		assertContains(t, got, "~/.aws/credentials")
	})

	t.Run("no read verb returns nil", func(t *testing.T) {
		got := extractBashCredentialPaths("echo hello")
		if len(got) != 0 {
			t.Errorf("expected nil for non-read command, got %v", got)
		}
	})

	t.Run("empty command returns nil", func(t *testing.T) {
		got := extractBashCredentialPaths("")
		if len(got) != 0 {
			t.Errorf("expected nil for empty command, got %v", got)
		}
	})

	t.Run("head command", func(t *testing.T) {
		got := extractBashCredentialPaths("head ~/.ssh/id_rsa")
		assertContains(t, got, "~/.ssh/id_rsa")
	})

	t.Run("tail command", func(t *testing.T) {
		got := extractBashCredentialPaths("tail ~/.aws/credentials")
		assertContains(t, got, "~/.aws/credentials")
	})

	t.Run("chained cat: second occurrence of verb extracts credential path (CR-01)", func(t *testing.T) {
		// cat appears TWICE: first read is benign, second reads a credential.
		// The old strings.Index only found the first occurrence and missed the
		// credential read. The fixed multi-occurrence loop must find BOTH.
		got := extractBashCredentialPaths("cat banner.txt && cat ~/.ssh/id_rsa")
		assertContains(t, got, "~/.ssh/id_rsa")
		// The benign file should also be present.
		assertContains(t, got, "banner.txt")
	})

	t.Run("leading flag token skipped to reach path (CR-01)", func(t *testing.T) {
		// "cat -n ~/.ssh/id_rsa": the old firstShellToken returned "-n" as the
		// first token, dropping the credential path. The fixed loop skips
		// leading flags and returns "~/.ssh/id_rsa".
		got := extractBashCredentialPaths("cat -n ~/.ssh/id_rsa")
		assertContains(t, got, "~/.ssh/id_rsa")
	})
}

// ---------------------------------------------------------------------------
// TestFirstShellToken
// ---------------------------------------------------------------------------

func TestFirstShellToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "leading whitespace + double-quoted path + extra token",
			input: `  "~/.aws/credentials" extra`,
			want:  "~/.aws/credentials",
		},
		{
			name:  "simple unquoted path",
			input: "~/.ssh/id_rsa",
			want:  "~/.ssh/id_rsa",
		},
		{
			name:  "single-quoted path",
			input: `'~/.npmrc' -n 1`,
			want:  "~/.npmrc",
		},
		{
			name:  "path with leading whitespace",
			input: "   /etc/passwd",
			want:  "/etc/passwd",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "",
		},
		{
			name:  "Windows backslash path unquoted",
			input: `%USERPROFILE%\.ssh\id_rsa`,
			want:  `%USERPROFILE%\.ssh\id_rsa`,
		},
		{
			name:  "unclosed quote returns remainder",
			input: `"~/.aws/credentials`,
			want:  "~/.aws/credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstShellToken(tt.input)
			if got != tt.want {
				t.Errorf("firstShellToken(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertContains(t *testing.T, paths []string, want string) {
	t.Helper()
	for _, p := range paths {
		if p == want {
			return
		}
	}
	t.Errorf("expected %q in %v", want, paths)
}
